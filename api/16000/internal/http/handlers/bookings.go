package handlers

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/itmtjewelry/land-booking-kpr/internal/auth"
)

type bookingOut struct {
	ID            string  `json:"id"`
	SiteID        string  `json:"site_id"`
	SubsiteID     string  `json:"subsite_id"`
	ZoneID        string  `json:"zone_id"`
	CustomerName  string  `json:"customer_name"`
	CustomerPhone string  `json:"customer_phone,omitempty"`
	CustomerEmail string  `json:"customer_email,omitempty"`
	Status        string  `json:"status"`
	StartDate     string  `json:"start_date"`
	EndDate       string  `json:"end_date"`
	Price         float64 `json:"price,omitempty"`
	Notes         string  `json:"notes,omitempty"`
	CreatedAt     string  `json:"created_at,omitempty"`
	UpdatedAt     string  `json:"updated_at,omitempty"`
}

func BookingsHandler(deps Stage8Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		if !deps.StorageReady() {
			errJSON(w, http.StatusServiceUnavailable, "storage not ready")
			return
		}

		zoneID := strings.TrimSpace(r.URL.Query().Get("zone_id"))
		if zoneID == "" {
			errJSON(w, http.StatusBadRequest, "zone_id is required")
			return
		}

		admin := auth.IsAdmin(r)
		items := deps.GetItems("bookings.json")

		out := make([]bookingOut, 0, 32)
		for _, v := range items {
			m, ok := v.(map[string]any)
			if !ok {
				continue
			}
			zid, _ := m["zone_id"].(string)
			if zid != zoneID {
				continue
			}

			b := bookingOut{
				ID:           str(m["id"]),
				SiteID:       str(m["site_id"]),
				SubsiteID:    str(m["subsite_id"]),
				ZoneID:       str(m["zone_id"]),
				CustomerName: str(m["customer_name"]),
				Status:       str(m["status"]),
				StartDate:    str(m["start_date"]),
				EndDate:      str(m["end_date"]),
				Notes:        str(m["notes"]),
				CreatedAt:    str(m["created_at"]),
				UpdatedAt:    str(m["updated_at"]),
			}
			if p, ok := m["price"].(float64); ok {
				b.Price = p
			}
			if admin {
				b.CustomerPhone = str(m["customer_phone"])
				b.CustomerEmail = str(m["customer_email"])
			}
			out = append(out, b)
		}

		sort.Slice(out, func(i, j int) bool {
			return out[i].StartDate < out[j].StartDate
		})

		okData(w, out)
	}
}

func AvailabilityHandler(deps Stage8Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		if !deps.StorageReady() {
			errJSON(w, http.StatusServiceUnavailable, "storage not ready")
			return
		}

		zoneID := strings.TrimSpace(r.URL.Query().Get("zone_id"))
		fromS := strings.TrimSpace(r.URL.Query().Get("from"))
		toS := strings.TrimSpace(r.URL.Query().Get("to"))
		if zoneID == "" || fromS == "" || toS == "" {
			errJSON(w, http.StatusBadRequest, "zone_id, from, to are required")
			return
		}

		fromT, err := time.Parse("2006-01-02", fromS)
		if err != nil {
			errJSON(w, http.StatusBadRequest, "invalid from date (YYYY-MM-DD)")
			return
		}
		toT, err := time.Parse("2006-01-02", toS)
		if err != nil {
			errJSON(w, http.StatusBadRequest, "invalid to date (YYYY-MM-DD)")
			return
		}
		if toT.Before(fromT) {
			errJSON(w, http.StatusBadRequest, "to must be >= from")
			return
		}

		items := deps.GetItems("bookings.json")

		type block struct {
			BookingID string `json:"booking_id"`
			Status    string `json:"status"`
			StartDate string `json:"start_date"`
			EndDate   string `json:"end_date"`
		}

		blocks := make([]block, 0, 32)
		available := true

		for _, v := range items {
			m, ok := v.(map[string]any)
			if !ok {
				continue
			}
			if str(m["zone_id"]) != zoneID {
				continue
			}
			status := str(m["status"])
			if status == "cancelled" {
				continue
			}

			bs := str(m["start_date"])
			be := str(m["end_date"])
			sT, err1 := time.Parse("2006-01-02", bs)
			eT, err2 := time.Parse("2006-01-02", be)
			if err1 != nil || err2 != nil {
				continue
			}

			if rangesOverlap(fromT, toT, sT, eT) {
				available = false
				blocks = append(blocks, block{
					BookingID: str(m["id"]),
					Status:    status,
					StartDate: bs,
					EndDate:   be,
				})
			}
		}

		sort.Slice(blocks, func(i, j int) bool {
			return blocks[i].StartDate < blocks[j].StartDate
		})

		okData(w, map[string]any{
			"zone_id":   zoneID,
			"from":      fromS,
			"to":        toS,
			"available": available,
			"blocked":   blocks,
		})
	}
}

// helpers
func str(v any) string {
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

func rangesOverlap(aStart, aEnd, bStart, bEnd time.Time) bool {
	// overlap if aStart <= bEnd AND aEnd >= bStart
	return !aStart.After(bEnd) && !aEnd.Before(bStart)
}

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
