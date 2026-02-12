package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/itmtjewelry/land-booking-kpr/internal/app"
	corehttp "github.com/itmtjewelry/land-booking-kpr/internal/http"
	"github.com/itmtjewelry/land-booking-kpr/internal/httpapi"
	"github.com/itmtjewelry/land-booking-kpr/internal/logging"
	"github.com/itmtjewelry/land-booking-kpr/internal/storage"
)

type runtimeState struct {
	mu sync.RWMutex

	storageDir string
	ready      bool
	loaded     map[string]storage.JSONFile
	loadedList []string

	fileMu map[string]*sync.Mutex
}

func newRuntimeState(storageDir string, lr *storage.LoadResult) *runtimeState {
	return &runtimeState{
		storageDir: storageDir,
		ready:      true,
		loaded:     lr.Loaded,
		loadedList: lr.LoadedList,
		fileMu: map[string]*sync.Mutex{
			"sites.json":    &sync.Mutex{},
			"subsites.json": &sync.Mutex{},
			"zones.json":    &sync.Mutex{},
			"users.json":    &sync.Mutex{},
			"bookings.json": &sync.Mutex{},
			"domains.json":  &sync.Mutex{},
		},
	}
}

func (rs *runtimeState) StorageReady() bool {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return rs.ready
}

func (rs *runtimeState) StorageDir() string {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return rs.storageDir
}

func (rs *runtimeState) Loaded() map[string]storage.JSONFile {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	out := make(map[string]storage.JSONFile, len(rs.loaded))
	for k, v := range rs.loaded {
		out[k] = v
	}
	return out
}

func (rs *runtimeState) LockForFile(filename string) *sync.Mutex {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	if m, ok := rs.fileMu[filename]; ok {
		return m
	}
	return &sync.Mutex{}
}

func (rs *runtimeState) ReloadCore() error {
	rs.mu.RLock()
	dir := rs.storageDir
	rs.mu.RUnlock()

	lr, err := storage.LoadCore(dir)
	if err != nil {
		return err
	}

	rs.mu.Lock()
	rs.ready = true
	rs.loaded = lr.Loaded
	rs.loadedList = lr.LoadedList
	rs.mu.Unlock()
	return nil
}

func (rs *runtimeState) GetItems(filename string) map[string]any {
	rs.mu.RLock()
	jf, ok := rs.loaded[filename]
	rs.mu.RUnlock()
	if !ok || jf.Items == nil {
		return nil
	}

	out := make(map[string]any, len(jf.Items))
	for id, raw := range jf.Items {
		var v any
		if err := json.Unmarshal(raw, &v); err != nil {
			continue
		}
		out[id] = v
	}
	return out
}

func (rs *runtimeState) CurrentAppState() app.State {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return app.State{
		StorageReady: rs.ready,
		StorageDir:   rs.storageDir,
		LoadedFiles:  rs.loadedList,
	}
}

func main() {
	addr := ":16000"
	logDir := "/var/api/16000/logs"
	service := "go-core"

	logger := logging.NewCSVLogger(logDir, service)
	storageDir := os.Getenv("STORAGE_DIR")

	loadRes, err := storage.LoadCore(storageDir)
	if err != nil {
		logger.Log("ERROR", "storage_load_failed", "", "storage", storageDir, err.Error())
		_, _ = fmt.Fprintln(os.Stderr, "storage load failed:", err)
		os.Exit(1)
	}

	rs := newRuntimeState(loadRes.Dir, loadRes)

	logger.Log("INFO", "storage_loaded", "", "storage", storageDir, fmt.Sprintf("loaded=%d", len(loadRes.LoadedList)))
	logger.Log("INFO", "startup", "", "service", service, "starting server")

	mux := http.NewServeMux()

	mux.HandleFunc("/api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		logger.Log("INFO", "health", "", "http", "/api/v1/health", fmt.Sprintf("%s %s", r.Method, r.URL.Path))
		httpapi.HealthHandler(w, r, rs.CurrentAppState())
	})

	mux.Handle("/api/v1/", corehttp.NewStage8Router(rs))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("not found\n"))
	})

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Log("INFO", "listen", "", "server", addr, "listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Log("ERROR", "listen_error", "", "server", addr, err.Error())
			_, _ = fmt.Fprintln(os.Stderr, "server error:", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	<-stop
	logger.Log("INFO", "shutdown", "", "service", service, "shutdown signal received")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Log("ERROR", "shutdown_error", "", "service", service, err.Error())
	} else {
		logger.Log("INFO", "shutdown_ok", "", "service", service, "server stopped cleanly")
	}
}
