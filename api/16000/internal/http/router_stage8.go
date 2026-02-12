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

	mux.HandleFunc("/api/v1/kpr", func(w http.ResponseWriter, r *http.Request) {
		handlers.KPRCollection(deps, w, r)
	})
	mux.HandleFunc("/api/v1/kpr/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/api/v1/kpr/"))
		if path == "" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("invalid path\n"))
			return
		}

		if strings.HasSuffix(path, "/submit") {
			id := strings.TrimSuffix(path, "/submit")
			id = strings.TrimSuffix(id, "/")
			handlers.KPRSubmit(deps, id, w, r)
			return
		}
		if strings.HasSuffix(path, "/approve") {
			id := strings.TrimSuffix(path, "/approve")
			id = strings.TrimSuffix(id, "/")
			handlers.KPRApprove(deps, id, w, r)
			return
		}
		if strings.HasSuffix(path, "/reject") {
			id := strings.TrimSuffix(path, "/reject")
			id = strings.TrimSuffix(id, "/")
			handlers.KPRReject(deps, id, w, r)
			return
		}
		if strings.HasSuffix(path, "/cancel") {
			id := strings.TrimSuffix(path, "/cancel")
			id = strings.TrimSuffix(id, "/")
			handlers.KPRCancel(deps, id, w, r)
			return
		}

		id := strings.TrimSuffix(path, "/")
		if id == "" || strings.Contains(id, "/") {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("invalid id\n"))
			return
		}
		handlers.KPRByID(deps, id, w, r)
	})

	mux.HandleFunc("/api/v1/installments", handlers.InstallmentsRead(deps))
	mux.HandleFunc("/api/v1/installments/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/api/v1/installments/"))
		if path == "" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("invalid path\n"))
			return
		}
		if strings.HasSuffix(path, "/generate") {
			id := strings.TrimSuffix(path, "/generate")
			id = strings.TrimSuffix(id, "/")
			handlers.InstallmentsGenerate(deps, id, w, r)
			return
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("not found\n"))
	})

	// =========================
	// STAGE 10.4: PAYMENTS (ADMIN ONLY)
	// =========================
	mux.HandleFunc("/api/v1/payments", func(w http.ResponseWriter, r *http.Request) {
		handlers.PaymentsCollection(deps, w, r)
	})

	// =========================
	// STAGE 10.5: REPORTS (READ ONLY)
	// =========================
	mux.HandleFunc("/api/v1/reports/kpr-statement", handlers.ReportKPRStatement(deps))
	mux.HandleFunc("/api/v1/reports/zone-summary", handlers.ReportZoneSummary(deps))
	mux.HandleFunc("/api/v1/reports/portfolio", handlers.ReportPortfolio(deps))
	mux.HandleFunc("/api/v1/reports/penalties/preview", handlers.PenaltiesPreview(deps))

	// STAGE 11: penalties charge (ADMIN)
	mux.HandleFunc("/api/v1/penalties/charge", func(w http.ResponseWriter, r *http.Request) {
		handlers.PenaltiesCharge(deps, w, r)
	})

	return mux
}
