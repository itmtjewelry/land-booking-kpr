package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/itmtjewelry/land-booking-kpr/internal/app"
)

type HealthResponse struct {
	OK   bool        `json:"ok"`
	Data interface{} `json:"data"`
}

func HealthHandler(w http.ResponseWriter, r *http.Request, st app.State) {
	w.Header().Set("Content-Type", "application/json")

	resp := HealthResponse{
		OK: st.StorageReady, // strict mode: if server is running, this should be true
		Data: map[string]any{
			"status":        "ok",
			"storage_ready": st.StorageReady,
			"storage_dir":   st.StorageDir,
			"loaded_files":  st.LoadedFiles,
		},
	}

	_ = json.NewEncoder(w).Encode(resp)
}
