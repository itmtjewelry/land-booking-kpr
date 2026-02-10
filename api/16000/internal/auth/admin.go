package auth

import (
	"net/http"
	"os"
)

const AdminHeader = "X-Admin-Token"

// IsAdmin returns true if request has correct admin token.
// It does not write any response.
func IsAdmin(r *http.Request) bool {
	expected := os.Getenv("ADMIN_TOKEN")
	if expected == "" {
		return false
	}
	got := r.Header.Get(AdminHeader)
	return got != "" && got == expected
}

// RequireAdmin enforces admin token for write endpoints (writes 401 on failure).
func RequireAdmin(w http.ResponseWriter, r *http.Request) bool {
	expected := os.Getenv("ADMIN_TOKEN")
	if expected == "" {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"ok":false,"error":"admin token not configured"}`))
		return false
	}
	got := r.Header.Get(AdminHeader)
	if got == "" || got != expected {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"ok":false,"error":"unauthorized"}`))
		return false
	}
	return true
}
