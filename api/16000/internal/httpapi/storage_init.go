package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/itmtjewelry/land-booking-kpr/internal/storagejson"
)

type StorageInitResponse struct {
	OK   bool                     `json:"ok"`
	Data storagejson.LayoutResult `json:"data"`
}

func StorageInitHandler(baseDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok":    false,
				"error": "method_not_allowed",
			})
			return
		}

		result, err := storagejson.EnsureLayout(baseDir)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok":    false,
				"error": err.Error(),
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(StorageInitResponse{OK: true, Data: result})
	}
}

func StorageStatusHandler(baseDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok":    false,
				"error": "method_not_allowed",
			})
			return
		}

		result, err := storagejson.CurrentLayout(baseDir)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok":    false,
				"error": err.Error(),
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(StorageInitResponse{OK: true, Data: result})
	}
}
