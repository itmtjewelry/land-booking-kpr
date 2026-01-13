package httpapi

import (
	"encoding/json"
	"net/http"
)

type HealthResponse struct {
	OK   bool        `json:"ok"`
	Data interface{} `json:"data"`
}

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	resp := HealthResponse{
		OK: true,
		Data: map[string]string{
			"status": "ok",
		},
	}
	_ = json.NewEncoder(w).Encode(resp)
}
