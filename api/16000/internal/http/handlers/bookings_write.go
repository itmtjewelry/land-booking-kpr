package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/itmtjewelry/land-booking-kpr/internal/auth"
	"github.com/itmtjewelry/land-booking-kpr/internal/storage"
)

type bookingPayload struct {
	ID            string  `json:"id"`
	SiteID        string  `json:"site_id"`
	SubsiteID     string  `json:"subsite_id"`
	ZoneID        string  `json:"zone_id"`
	CustomerName  string  `json:"customer_name"`
	CustomerPhone string  `json:"customer_phone"`
	CustomerEmail string  `json:"customer_email"`
	Status        string  `json:"status"`
	StartDate     string  `json:"start_date"`
	EndDate       string  `json:"end_date"`
	Price         float64 `json:"price"`
	Notes         string  `json:"notes"`
}

func BookingsWriteCollection(deps Stage8Deps, w http.ResponseWriter, r *http.Request) {
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

	var p bookingPayload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid json")
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)

	p.SiteID = strings.TrimSpace(p.SiteID)
	p.SubsiteID = strings.TrimSpace(p.SubsiteID)
	p.ZoneID = strings.TrimSpace(p.ZoneID)
	p.CustomerName = strings.TrimSpace(p.CustomerName)
	p.CustomerPhone = strings.TrimSpace(p.CustomerPhone)
	p.CustomerEmail = strings.TrimSpace(p.CustomerEmail)
	p.Status = strings.TrimSpace(p.Status)
	p.StartDate = strings.TrimSpace(p.StartDate)
	p.EndDate = strings.TrimSpace(p.EndDate)
	p.Notes = strings.TrimSpace(p.Notes)

	if p.SiteID == "" || p.SubsiteID == "" || p.ZoneID == "" {
		errJSON(w, http.StatusBadRequest, "site_id, subsite_id, zone_id are required")
		return
	}
	if p.CustomerName == "" {
		errJSON(w, http.StatusBadRequest, "customer_name is required")
		return
	}
	if p.StartDate == "" || p.EndDate == "" {
		errJSON(w, http.StatusBadRequest, "start_date and end_date are required")
		return
	}

	sT, eT, err := parseRange(p.StartDate, p.EndDate)
	if err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}

	if p.Status == "" {
		p.Status = "pending"
	}
	if p.Status != "pending" && p.Status != "confirmed" && p.Status != "cancelled" {
		errJSON(w, http.StatusBadRequest, "invalid status")
		return
	}

	// Validate chain: zone exists and matches subsite + site
	if err := validateZoneChain(deps, p.SiteID, p.SubsiteID, p.ZoneID); err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}

	if strings.TrimSpace(p.ID) == "" {
		p.ID = genID("booking")
	}

	filename := "bookings.json"
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

	// Strict overlap check for pending + confirmed
	if conflict := hasBookingOverlap(deps, "", p.ZoneID, sT, eT); conflict {
		errJSON(w, http.StatusConflict, "date range overlaps existing booking")
		return
	}

	obj := map[string]any{
		"id":             p.ID,
		"site_id":        p.SiteID,
		"subsite_id":     p.SubsiteID,
		"zone_id":        p.ZoneID,
		"customer_name":  p.CustomerName,
		"customer_phone": p.CustomerPhone,
		"customer_email": p.CustomerEmail,
		"status":         p.Status,
		"start_date":     p.StartDate,
		"end_date":       p.EndDate,
		"price":          p.Price,
		"notes":          p.Notes,
		"created_at":     now,
		"updated_at":     now,
	}

	jf.Items[p.ID] = mustJSON(obj)

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

func BookingsWriteByID(deps Stage8Deps, id string, w http.ResponseWriter, r *http.Request) {
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

	switch r.Method {
	case http.MethodPut:
		bookingsPut(deps, id, w, r)
	case http.MethodPost:
		// reserved (cancel route handled separately)
		methodNotAllowed(w)
	default:
		methodNotAllowed(w)
	}
}

func BookingCancelByID(deps Stage8Deps, id string, w http.ResponseWriter, r *http.Request) {
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

	filename := "bookings.json"
	mu := deps.LockForFile(filename)
	mu.Lock()
	defer mu.Unlock()

	loaded := deps.Loaded()
	jf, ok := loaded[filename]
	if !ok || jf.Items == nil {
		errJSON(w, http.StatusBadRequest, "booking not found")
		return
	}
	raw, exists := jf.Items[id]
	if !exists {
		errJSON(w, http.StatusBadRequest, "booking not found")
		return
	}

	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		errJSON(w, http.StatusInternalServerError, "invalid stored booking")
		return
	}

	status := str(m["status"])
	if status == "cancelled" {
		okData(w, map[string]any{"id": id, "status": "cancelled"})
		return
	}

	m["status"] = "cancelled"
	m["updated_at"] = time.Now().UTC().Format(time.RFC3339)

	jf.Items[id] = mustJSON(m)

	if err := storage.WriteJSONFileAtomic(deps.StorageDir(), filename, jf); err != nil {
		errJSON(w, http.StatusInternalServerError, "write failed")
		return
	}
	if err := deps.ReloadCore(); err != nil {
		errJSON(w, http.StatusInternalServerError, "reload failed")
		return
	}

	okData(w, map[string]any{"id": id, "status": "cancelled"})
}

func bookingsPut(deps Stage8Deps, id string, w http.ResponseWriter, r *http.Request) {
	filename := "bookings.json"
	mu := deps.LockForFile(filename)
	mu.Lock()
	defer mu.Unlock()

	loaded := deps.Loaded()
	jf, ok := loaded[filename]
	if !ok || jf.Items == nil {
		errJSON(w, http.StatusBadRequest, "booking not found")
		return
	}
	raw, exists := jf.Items[id]
	if !exists {
		errJSON(w, http.StatusBadRequest, "booking not found")
		return
	}

	var cur map[string]any
	if err := json.Unmarshal(raw, &cur); err != nil {
		errJSON(w, http.StatusInternalServerError, "invalid stored booking")
		return
	}

	curStatus := str(cur["status"])
	if curStatus == "cancelled" {
		errJSON(w, http.StatusConflict, "cannot update cancelled booking")
		return
	}

	var p bookingPayload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid json")
		return
	}

	// Keep IDs stable; allow updates to fields
	siteID := strings.TrimSpace(p.SiteID)
	subsiteID := strings.TrimSpace(p.SubsiteID)
	zoneID := strings.TrimSpace(p.ZoneID)

	if siteID == "" {
		siteID = str(cur["site_id"])
	}
	if subsiteID == "" {
		subsiteID = str(cur["subsite_id"])
	}
	if zoneID == "" {
		zoneID = str(cur["zone_id"])
	}

	startS := strings.TrimSpace(p.StartDate)
	endS := strings.TrimSpace(p.EndDate)
	if startS == "" {
		startS = str(cur["start_date"])
	}
	if endS == "" {
		endS = str(cur["end_date"])
	}

	sT, eT, err := parseRange(startS, endS)
	if err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}

	// Validate chain again
	if err := validateZoneChain(deps, siteID, subsiteID, zoneID); err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}

	newStatus := strings.TrimSpace(p.Status)
	if newStatus == "" {
		newStatus = curStatus
	}
	if newStatus != "pending" && newStatus != "confirmed" && newStatus != "cancelled" {
		errJSON(w, http.StatusBadRequest, "invalid status")
		return
	}
	// status flow A: pending -> confirmed -> cancelled
	if !validStatusTransition(curStatus, newStatus) {
		errJSON(w, http.StatusConflict, "invalid status transition")
		return
	}

	// Overlap check if still active (pending/confirmed)
	if newStatus != "cancelled" {
		if conflict := hasBookingOverlap(deps, id, zoneID, sT, eT); conflict {
			errJSON(w, http.StatusConflict, "date range overlaps existing booking")
			return
		}
	}

	// Apply updates
	cur["site_id"] = siteID
	cur["subsite_id"] = subsiteID
	cur["zone_id"] = zoneID
	cur["start_date"] = startS
	cur["end_date"] = endS
	cur["status"] = newStatus
	cur["updated_at"] = time.Now().UTC().Format(time.RFC3339)

	if strings.TrimSpace(p.CustomerName) != "" {
		cur["customer_name"] = strings.TrimSpace(p.CustomerName)
	}
	if p.CustomerPhone != "" {
		cur["customer_phone"] = strings.TrimSpace(p.CustomerPhone)
	}
	if p.CustomerEmail != "" {
		cur["customer_email"] = strings.TrimSpace(p.CustomerEmail)
	}
	if p.Notes != "" {
		cur["notes"] = strings.TrimSpace(p.Notes)
	}
	if p.Price != 0 {
		cur["price"] = p.Price
	}

	jf.Items[id] = mustJSON(cur)

	if err := storage.WriteJSONFileAtomic(deps.StorageDir(), filename, jf); err != nil {
		errJSON(w, http.StatusInternalServerError, "write failed")
		return
	}
	if err := deps.ReloadCore(); err != nil {
		errJSON(w, http.StatusInternalServerError, "reload failed")
		return
	}

	okData(w, map[string]any{"id": id})
}

// ===== validation helpers =====

func parseRange(startS, endS string) (time.Time, time.Time, error) {
	sT, err := time.Parse("2006-01-02", startS)
	if err != nil {
		return time.Time{}, time.Time{}, errBad("invalid start_date (YYYY-MM-DD)")
	}
	eT, err := time.Parse("2006-01-02", endS)
	if err != nil {
		return time.Time{}, time.Time{}, errBad("invalid end_date (YYYY-MM-DD)")
	}
	if eT.Before(sT) {
		return time.Time{}, time.Time{}, errBad("end_date must be >= start_date")
	}
	return sT, eT, nil
}

type errBad string

func (e errBad) Error() string { return string(e) }

func validStatusTransition(cur, next string) bool {
	if cur == next {
		return true
	}
	switch cur {
	case "pending":
		return next == "confirmed" || next == "cancelled"
	case "confirmed":
		return next == "cancelled"
	case "cancelled":
		return false
	default:
		return false
	}
}

func validateZoneChain(deps Stage8Deps, siteID, subsiteID, zoneID string) error {
	// zone exists?
	zones := deps.GetItems("zones.json")
	zAny, ok := zones[zoneID]
	if !ok {
		return errBad("zone_id not found")
	}
	zMap, ok := zAny.(map[string]any)
	if !ok {
		return errBad("invalid zone record")
	}
	if str(zMap["subsite_id"]) != subsiteID {
		return errBad("zone_id does not belong to subsite_id")
	}

	// subsite exists?
	subs := deps.GetItems("subsites.json")
	sAny, ok := subs[subsiteID]
	if !ok {
		return errBad("subsite_id not found")
	}
	sMap, ok := sAny.(map[string]any)
	if !ok {
		return errBad("invalid subsite record")
	}
	if str(sMap["site_id"]) != siteID {
		return errBad("subsite_id does not belong to site_id")
	}

	// site exists?
	sites := deps.GetItems("sites.json")
	if _, ok := sites[siteID]; !ok {
		return errBad("site_id not found")
	}
	return nil
}

func hasBookingOverlap(deps Stage8Deps, ignoreID, zoneID string, sT, eT time.Time) bool {
	items := deps.GetItems("bookings.json")
	for _, v := range items {
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		if str(m["id"]) == ignoreID {
			continue
		}
		if str(m["zone_id"]) != zoneID {
			continue
		}
		status := str(m["status"])
		if status == "cancelled" {
			continue
		}
		os := str(m["start_date"])
		oe := str(m["end_date"])
		oST, err1 := time.Parse("2006-01-02", os)
		oET, err2 := time.Parse("2006-01-02", oe)
		if err1 != nil || err2 != nil {
			continue
		}
		if rangesOverlap(sT, eT, oST, oET) {
			return true
		}
	}
	return false
}
