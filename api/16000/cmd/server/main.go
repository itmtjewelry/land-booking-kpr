package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/itmtjewelry/land-booking-kpr/internal/httpapi"
	"github.com/itmtjewelry/land-booking-kpr/internal/logging"
)

func main() {
	addr := ":16000"
	logDir := "/var/api/16000/logs"
	service := "go-core"

	logger := logging.NewCSVLogger(logDir, service)
	logger.Log("INFO", "startup", "", "service", service, "starting server")

	mux := http.NewServeMux()

	// Health endpoint (Stage 1)
	mux.HandleFunc("/api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		logger.Log("INFO", "health", "", "http", "/api/v1/health", fmt.Sprintf("%s %s", r.Method, r.URL.Path))
		httpapi.HealthHandler(w, r)
	})

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

	// Start server in goroutine
	go func() {
		logger.Log("INFO", "listen", "", "server", addr, "listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Log("ERROR", "listen_error", "", "server", addr, err.Error())
			// Also print to stderr for visibility
			_, _ = fmt.Fprintln(os.Stderr, "server error:", err)
		}
	}()

	// Graceful shutdown
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
