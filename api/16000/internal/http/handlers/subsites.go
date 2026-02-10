package handlers

import "net/http"

// SubsitesHandler serves GET /api/v1/subsites?site_id=...
func SubsitesHandler(deps Stage7Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			MethodNotAllowed(w)
			return
		}
		if !deps.StorageReady() {
			ServiceUnavailable(w)
			return
		}

		siteID := r.URL.Query().Get("site_id")
		if siteID == "" {
			BadRequest(w, "missing_site_id", "missing required query param: site_id")
			return
		}

		items := deps.GetItems("subsites.json")
		all := ItemsToSlice(items)
		data := FilterByStringField(all, "site_id", siteID)

		WriteJSON(w, http.StatusOK, Envelope{OK: true, Data: data})
	}
}
