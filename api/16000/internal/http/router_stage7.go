package http

import (
	"net/http"

	"github.com/itmtjewelry/land-booking-kpr/internal/http/handlers"
)

// Replace YOUR_MODULE_PATH with the module path from your go.mod.
// Example: github.com/itmtjewelry/land-booking-kpr

// NewStage7Router returns an http.Handler that serves Stage 7 read-only endpoints under /api/v1.
func NewStage7Router(deps handlers.Stage7Deps) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/v1/sites", handlers.SitesHandler(deps))
	mux.HandleFunc("/api/v1/subsites", handlers.SubsitesHandler(deps))
	mux.HandleFunc("/api/v1/zones", handlers.ZonesHandler(deps))

	return mux
}
