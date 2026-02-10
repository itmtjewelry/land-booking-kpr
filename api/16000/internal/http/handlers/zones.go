package handlers

import "net/http"

// ZonesHandler serves GET /api/v1/zones?subsite_id=...
func ZonesHandler(deps Stage7Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			MethodNotAllowed(w)
			return
		}
		if !deps.StorageReady() {
			ServiceUnavailable(w)
			return
		}

		subsiteID := r.URL.Query().Get("subsite_id")
		if subsiteID == "" {
			BadRequest(w, "missing_subsite_id", "missing required query param: subsite_id")
			return
		}

		items := deps.GetItems("zones.json")
		all := ItemsToSlice(items)
		data := FilterByStringField(all, "subsite_id", subsiteID)

		WriteJSON(w, http.StatusOK, Envelope{OK: true, Data: data})
	}
}
