package handlers

import (
	"encoding/json"
	"net/http"
	"sort"
)

type Envelope struct {
	OK   bool        `json:"ok"`
	Data any         `json:"data"`
	Err  *ErrorShape `json:"error,omitempty"`
}

type ErrorShape struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Stage7Deps provides access to in-memory JSON items for Stage 7 read-only APIs.
//
// Implement this interface by wiring to your existing in-memory storage instance
// that Stage 6 already loads at startup.
type Stage7Deps interface {
	StorageReady() bool
	// GetItems returns the "items" object-of-objects for a given core JSON filename (e.g. "sites.json").
	GetItems(filename string) map[string]any
}

func WriteJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)
	_ = enc.Encode(payload)
}

func MethodNotAllowed(w http.ResponseWriter) {
	WriteJSON(w, http.StatusMethodNotAllowed, Envelope{
		OK:   false,
		Data: []any{},
		Err:  &ErrorShape{Code: "method_not_allowed", Message: "method not allowed"},
	})
}

func ServiceUnavailable(w http.ResponseWriter) {
	WriteJSON(w, http.StatusServiceUnavailable, Envelope{
		OK:   false,
		Data: []any{},
		Err:  &ErrorShape{Code: "storage_not_ready", Message: "storage is not ready"},
	})
}

func BadRequest(w http.ResponseWriter, code, message string) {
	WriteJSON(w, http.StatusBadRequest, Envelope{
		OK:   false,
		Data: []any{},
		Err:  &ErrorShape{Code: code, Message: message},
	})
}

// ItemsToSlice converts an "items" map (object-of-objects) into a stable, deterministic slice.
// It adds the key as field "id" on each returned object.
func ItemsToSlice(items map[string]any) []map[string]any {
	if items == nil {
		return []map[string]any{}
	}

	keys := make([]string, 0, len(items))
	for k := range items {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	out := make([]map[string]any, 0, len(keys))
	for _, id := range keys {
		raw, ok := items[id]
		if !ok || raw == nil {
			continue
		}
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}

		clone := make(map[string]any, len(m)+1)
		for k, v := range m {
			clone[k] = v
		}
		clone["id"] = id
		out = append(out, clone)
	}

	return out
}

func FilterByStringField(items []map[string]any, field, value string) []map[string]any {
	if value == "" {
		return []map[string]any{}
	}
	out := make([]map[string]any, 0)
	for _, it := range items {
		v, ok := it[field]
		if !ok {
			continue
		}
		s, ok := v.(string)
		if !ok {
			continue
		}
		if s == value {
			out = append(out, it)
		}
	}
	return out
}
