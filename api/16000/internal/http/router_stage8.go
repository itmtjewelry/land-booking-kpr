package http

import (
	"net/http"
	"strings"

	"github.com/itmtjewelry/land-booking-kpr/internal/http/handlers"
)

func NewStage8Router(deps handlers.Stage8Deps) http.Handler {
	mux := http.NewServeMux()

	// SITES
	mux.HandleFunc("/api/v1/sites", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handlers.SitesHandler(deps)(w, r)
		case http.MethodPost:
			handlers.SitesWriteCollection(deps, w, r)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			_, _ = w.Write([]byte("method not allowed\n"))
		}
	})
	mux.HandleFunc("/api/v1/sites/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/api/v1/sites/"))
		if id == "" || strings.Contains(id, "/") {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("invalid id\n"))
			return
		}
		handlers.SitesWriteByID(deps, id, w, r)
	})

	// SUBSITES
	mux.HandleFunc("/api/v1/subsites", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handlers.SubsitesHandler(deps)(w, r)
		case http.MethodPost:
			handlers.SubsitesWriteCollection(deps, w, r)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			_, _ = w.Write([]byte("method not allowed\n"))
		}
	})
	mux.HandleFunc("/api/v1/subsites/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/api/v1/subsites/"))
		if id == "" || strings.Contains(id, "/") {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("invalid id\n"))
			return
		}
		handlers.SubsitesWriteByID(deps, id, w, r)
	})

	// ZONES
	mux.HandleFunc("/api/v1/zones", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handlers.ZonesHandler(deps)(w, r)
		case http.MethodPost:
			handlers.ZonesWriteCollection(deps, w, r)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			_, _ = w.Write([]byte("method not allowed\n"))
		}
	})
	mux.HandleFunc("/api/v1/zones/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/api/v1/zones/"))
		if id == "" || strings.Contains(id, "/") {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("invalid id\n"))
			return
		}
		handlers.ZonesWriteByID(deps, id, w, r)
	})

	// BOOKINGS (read guest-safe; write admin-only)
	mux.HandleFunc("/api/v1/bookings", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handlers.BookingsHandler(deps)(w, r)
		case http.MethodPost:
			handlers.BookingsWriteCollection(deps, w, r)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			_, _ = w.Write([]byte("method not allowed\n"))
		}
	})
	mux.HandleFunc("/api/v1/bookings/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/bookings/")
		path = strings.TrimSpace(path)
		if path == "" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("invalid path\n"))
			return
		}

		// /api/v1/bookings/{id}/cancel
		if strings.HasSuffix(path, "/cancel") {
			id := strings.TrimSuffix(path, "/cancel")
			id = strings.TrimSuffix(id, "/")
			id = strings.TrimSpace(id)
			if id == "" || strings.Contains(id, "/") {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte("invalid id\n"))
				return
			}
			handlers.BookingCancelByID(deps, id, w, r)
			return
		}

		// /api/v1/bookings/{id}
		id := path
		if id == "" || strings.Contains(id, "/") {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("invalid id\n"))
			return
		}
		handlers.BookingsWriteByID(deps, id, w, r)
	})

	// AVAILABILITY
	mux.HandleFunc("/api/v1/availability", func(w http.ResponseWriter, r *http.Request) {
		handlers.AvailabilityHandler(deps)(w, r)
	})

	return mux
}
