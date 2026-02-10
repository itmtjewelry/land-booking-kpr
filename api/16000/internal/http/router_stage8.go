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

	// BOOKINGS
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
		id := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/api/v1/bookings/"))
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

	// =========================
	// STAGE 10.3: KPR + INSTALLMENTS
	// =========================

	// KPR collection: GET by booking_id, POST create (admin)
	mux.HandleFunc("/api/v1/kpr", func(w http.ResponseWriter, r *http.Request) {
		handlers.KPRCollection(deps, w, r)
	})

	// KPR by id and actions:
	// - PUT /api/v1/kpr/{id}
	// - POST /api/v1/kpr/{id}/submit
	// - POST /api/v1/kpr/{id}/approve
	// - POST /api/v1/kpr/{id}/reject
	// - POST /api/v1/kpr/{id}/cancel
	mux.HandleFunc("/api/v1/kpr/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/api/v1/kpr/"))
		if path == "" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("invalid path\n"))
			return
		}

		if strings.HasSuffix(path, "/submit") {
			id := strings.TrimSpace(strings.TrimSuffix(path, "/submit"))
			id = strings.TrimSuffix(id, "/")
			handlers.KPRSubmit(deps, id, w, r)
			return
		}
		if strings.HasSuffix(path, "/approve") {
			id := strings.TrimSpace(strings.TrimSuffix(path, "/approve"))
			id = strings.TrimSuffix(id, "/")
			handlers.KPRApprove(deps, id, w, r)
			return
		}
		if strings.HasSuffix(path, "/reject") {
			id := strings.TrimSpace(strings.TrimSuffix(path, "/reject"))
			id = strings.TrimSuffix(id, "/")
			handlers.KPRReject(deps, id, w, r)
			return
		}
		if strings.HasSuffix(path, "/cancel") {
			id := strings.TrimSpace(strings.TrimSuffix(path, "/cancel"))
			id = strings.TrimSuffix(id, "/")
			handlers.KPRCancel(deps, id, w, r)
			return
		}

		// default: /api/v1/kpr/{id}
		id := strings.TrimSuffix(path, "/")
		if id == "" || strings.Contains(id, "/") {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("invalid id\n"))
			return
		}
		handlers.KPRByID(deps, id, w, r)
	})

	// INSTALLMENTS read: GET /api/v1/installments?kpr_id=...
	mux.HandleFunc("/api/v1/installments", handlers.InstallmentsRead(deps))

	// INSTALLMENTS generate: POST /api/v1/installments/{kpr_id}/generate
	mux.HandleFunc("/api/v1/installments/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/api/v1/installments/"))
		if path == "" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("invalid path\n"))
			return
		}
		if strings.HasSuffix(path, "/generate") {
			id := strings.TrimSpace(strings.TrimSuffix(path, "/generate"))
			id = strings.TrimSuffix(id, "/")
			handlers.InstallmentsGenerate(deps, id, w, r)
			return
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("not found\n"))
	})

	return mux
}
