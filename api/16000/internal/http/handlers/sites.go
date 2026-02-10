package handlers

import "net/http"

// SitesHandler serves GET /api/v1/sites
func SitesHandler(deps Stage7Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			MethodNotAllowed(w)
			return
		}
		if !deps.StorageReady() {
			ServiceUnavailable(w)
			return
		}

		items := deps.GetItems("sites.json")
		data := ItemsToSlice(items)

		WriteJSON(w, http.StatusOK, Envelope{OK: true, Data: data})
	}
}
