package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/itmtjewelry/land-booking-kpr/internal/app"
	corehttp "github.com/itmtjewelry/land-booking-kpr/internal/http"
	"github.com/itmtjewelry/land-booking-kpr/internal/httpapi"
	"github.com/itmtjewelry/land-booking-kpr/internal/logging"
	"github.com/itmtjewelry/land-booking-kpr/internal/storage"
)

// Stage 7 deps adapter (must satisfy internal/http/handlers.Stage7Deps)
type stage7Deps struct {
	ready  bool
	loaded map[string]storage.JSONFile // filename -> JSONFile{Meta, Items(raw)}
}

func (d stage7Deps) StorageReady() bool { return d.ready }

// GetItems returns the "items" object decoded into map[string]any.
// It is read-only and returns a fresh map (no shared references).
func (d stage7Deps) GetItems(filename string) map[string]any {
	if !d.ready || d.loaded == nil {
		return nil
	}

	jf, ok := d.loaded[filename]
	if !ok || jf.Items == nil {
		return nil
	}

	out := make(map[string]any, len(jf.Items))
	for id, raw := range jf.Items {
		// Each item is an object; decode it into map[string]any.
		var v any
		if err := json.Unmarshal(raw, &v); err != nil {
			// Strict storage validation already happened at startup; if an item fails
			// to decode here, we just skip it (read APIs should not crash server).
			continue
		}
		out[id] = v
	}
	return out
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
		os.Exit(1) // STRICT: do not start server
	}

	state := app.State{
		StorageReady: true,
		StorageDir:   loadRes.Dir,
		LoadedFiles:  loadRes.LoadedList,
	}

	// Stage 7 uses the already-loaded in-memory JSON (object-of-objects).
	deps := stage7Deps{
		ready:  true,
		loaded: loadRes.Loaded,
	}

	logger.Log("INFO", "storage_loaded", "", "storage", storageDir, fmt.Sprintf("loaded=%d", len(loadRes.LoadedList)))
	logger.Log("INFO", "startup", "", "service", service, "starting server")

	mux := http.NewServeMux()

	// Health endpoint (Stage 6)
	mux.HandleFunc("/api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		logger.Log("INFO", "health", "", "http", "/api/v1/health", fmt.Sprintf("%s %s", r.Method, r.URL.Path))
		httpapi.HealthHandler(w, r, state)
	})

	// Stage 7: Read-only GET APIs under /api/v1/
	// Important: mount BEFORE "/" fallback.
	mux.Handle("/api/v1/", corehttp.NewStage7Router(deps))

	// Basic root to avoid confusion
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
