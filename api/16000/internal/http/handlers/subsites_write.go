package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/itmtjewelry/land-booking-kpr/internal/auth"
	"github.com/itmtjewelry/land-booking-kpr/internal/storage"
)

type subsitePayload struct {
	ID     string `json:"id"`
	SiteID string `json:"site_id"`
	Name   string `json:"name"`
}

func SubsitesWriteCollection(deps Stage8Deps, w http.ResponseWriter, r *http.Request) {
	if !deps.StorageReady() {
		errJSON(w, http.StatusServiceUnavailable, "storage not ready")
		return
	}
	if !auth.RequireAdmin(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	var p subsitePayload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid json")
		return
	}

	p.SiteID = strings.TrimSpace(p.SiteID)
	p.Name = strings.TrimSpace(p.Name)
	if p.SiteID == "" {
		errJSON(w, http.StatusBadRequest, "site_id is required")
		return
	}
	if p.Name == "" {
		errJSON(w, http.StatusBadRequest, "name is required")
		return
	}

	// Validate parent site exists
	siteItems := deps.GetItems("sites.json")
	if _, ok := siteItems[p.SiteID]; !ok {
		errJSON(w, http.StatusBadRequest, "site_id not found")
		return
	}

	if strings.TrimSpace(p.ID) == "" {
		p.ID = genID("subsite")
	}

	filename := "subsites.json"
	mu := deps.LockForFile(filename)
	mu.Lock()
	defer mu.Unlock()

	loaded := deps.Loaded()
	jf, ok := loaded[filename]
	if !ok {
		jf = storage.JSONFile{
			Meta:  map[string]any{"version": 1, "updated_at": nil},
			Items: map[string]json.RawMessage{},
		}
	}
	if jf.Meta == nil {
		jf.Meta = map[string]any{"version": 1, "updated_at": nil}
	}
	if jf.Items == nil {
		jf.Items = make(map[string]json.RawMessage)
	}

	if _, exists := jf.Items[p.ID]; exists {
		errJSON(w, http.StatusConflict, "id already exists")
		return
	}

	raw, _ := json.Marshal(map[string]any{
		"id":      p.ID,
		"site_id": p.SiteID,
		"name":    p.Name,
	})
	jf.Items[p.ID] = raw

	if err := storage.WriteJSONFileAtomic(deps.StorageDir(), filename, jf); err != nil {
		errJSON(w, http.StatusInternalServerError, "write failed")
		return
	}
	if err := deps.ReloadCore(); err != nil {
		errJSON(w, http.StatusInternalServerError, "reload failed")
		return
	}

	okData(w, map[string]any{"id": p.ID})
}

func SubsitesWriteByID(deps Stage8Deps, id string, w http.ResponseWriter, r *http.Request) {
	if !deps.StorageReady() {
		errJSON(w, http.StatusServiceUnavailable, "storage not ready")
		return
	}
	if !auth.RequireAdmin(w, r) {
		return
	}

	id = strings.TrimSpace(id)
	if id == "" {
		errJSON(w, http.StatusBadRequest, "invalid id")
		return
	}

	filename := "subsites.json"
	mu := deps.LockForFile(filename)
	mu.Lock()
	defer mu.Unlock()

	loaded := deps.Loaded()
	jf, ok := loaded[filename]
	if !ok {
		jf = storage.JSONFile{
			Meta:  map[string]any{"version": 1, "updated_at": nil},
			Items: map[string]json.RawMessage{},
		}
	}
	if jf.Meta == nil {
		jf.Meta = map[string]any{"version": 1, "updated_at": nil}
	}
	if jf.Items == nil {
		jf.Items = make(map[string]json.RawMessage)
	}

	switch r.Method {
	case http.MethodPut:
		if _, exists := jf.Items[id]; !exists {
			errJSON(w, http.StatusBadRequest, "id not found")
			return
		}

		var p subsitePayload
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			errJSON(w, http.StatusBadRequest, "invalid json")
			return
		}

		p.SiteID = strings.TrimSpace(p.SiteID)
		p.Name = strings.TrimSpace(p.Name)
		if p.SiteID == "" {
			errJSON(w, http.StatusBadRequest, "site_id is required")
			return
		}
		if p.Name == "" {
			errJSON(w, http.StatusBadRequest, "name is required")
			return
		}

		siteItems := deps.GetItems("sites.json")
		if _, ok := siteItems[p.SiteID]; !ok {
			errJSON(w, http.StatusBadRequest, "site_id not found")
			return
		}

		raw, _ := json.Marshal(map[string]any{
			"id":      id,
			"site_id": p.SiteID,
			"name":    p.Name,
		})
		jf.Items[id] = raw

		if err := storage.WriteJSONFileAtomic(deps.StorageDir(), filename, jf); err != nil {
			errJSON(w, http.StatusInternalServerError, "write failed")
			return
		}
		if err := deps.ReloadCore(); err != nil {
			errJSON(w, http.StatusInternalServerError, "reload failed")
			return
		}
		okData(w, map[string]any{"id": id})
		return

	case http.MethodDelete:
		zoneItems := deps.GetItems("zones.json")
		for _, v := range zoneItems {
			m, ok := v.(map[string]any)
			if !ok {
				continue
			}
			if sid, ok := m["subsite_id"].(string); ok && sid == id {
				errJSON(w, http.StatusConflict, "cannot delete subsite with zones")
				return
			}
		}

		if _, exists := jf.Items[id]; !exists {
			okData(w, map[string]any{"deleted": false})
			return
		}
		delete(jf.Items, id)

		if err := storage.WriteJSONFileAtomic(deps.StorageDir(), filename, jf); err != nil {
			errJSON(w, http.StatusInternalServerError, "write failed")
			return
		}
		if err := deps.ReloadCore(); err != nil {
			errJSON(w, http.StatusInternalServerError, "reload failed")
			return
		}
		okData(w, map[string]any{"deleted": true})
		return

	default:
		methodNotAllowed(w)
	}
}
