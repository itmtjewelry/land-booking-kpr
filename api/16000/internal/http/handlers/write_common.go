package handlers

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"
)

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func okData(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "data": data})
}

func errJSON(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"ok": false, "error": msg})
}

func methodNotAllowed(w http.ResponseWriter) {
	errJSON(w, http.StatusMethodNotAllowed, "method not allowed")
}

func genID(prefix string) string {
	now := time.Now().UTC().Format("20060102T150405.000000000Z")
	h := sha1.Sum([]byte(now))
	return prefix + "_" + now + "_" + hex.EncodeToString(h[:6])
}
