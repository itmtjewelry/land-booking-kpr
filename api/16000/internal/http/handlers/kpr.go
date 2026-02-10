package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/itmtjewelry/land-booking-kpr/internal/auth"
	"github.com/itmtjewelry/land-booking-kpr/internal/storage"
)

type kprCustomer struct {
	Name    string `json:"name"`
	Phone   string `json:"phone"`
	Email   string `json:"email"`
	NIK     string `json:"nik"`
	Address string `json:"address"`
}

type kprPrice struct {
	LandPrice    float64 `json:"land_price"`
	DpAmount     float64 `json:"dp_amount"`
	DpPaid       float64 `json:"dp_paid"`
	LoanAmount   float64 `json:"loan_amount"`
	TenorMonths  int     `json:"tenor_months"`
	InterestRate float64 `json:"interest_rate"`
	AdminFee     float64 `json:"admin_fee"`
	OtherFee     float64 `json:"other_fee"`
	Total        float64 `json:"total"`
}

type kprCreatePayload struct {
	BookingID string `json:"booking_id"`
	Notes     string `json:"notes"`
}

type kprUpdatePayload struct {
	Notes    string       `json:"notes"`
	Customer *kprCustomer `json:"customer"`
	Price    *kprPrice    `json:"price"`
}

func KPRCollection(deps Stage8Deps, w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		kprGetByBooking(deps, w, r)
	case http.MethodPost:
		kprCreate(deps, w, r)
	default:
		methodNotAllowed(w)
	}
}

func KPRByID(deps Stage8Deps, id string, w http.ResponseWriter, r *http.Request) {
	id = strings.TrimSpace(id)
	if id == "" || strings.Contains(id, "/") {
		errJSON(w, http.StatusBadRequest, "invalid id")
		return
	}

	switch r.Method {
	case http.MethodPut:
		kprUpdate(deps, id, w, r)
	default:
		methodNotAllowed(w)
	}
}

func KPRSubmit(deps Stage8Deps, id string, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	kprTransition(deps, id, w, r, "draft", "submitted")
}

func KPRApprove(deps Stage8Deps, id string, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	// submitted -> approved, but validate required fields for flat plan
	kprTransitionWithValidate(deps, id, w, r, "submitted", "approved", validateKPRForApprove)
}

func KPRReject(deps Stage8Deps, id string, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	kprTransition(deps, id, w, r, "submitted", "rejected")
}

func KPRCancel(deps Stage8Deps, id string, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	// allowed: draft/submitted -> cancelled
	if !auth.RequireAdmin(w, r) {
		return
	}
	filename := "kpr_applications.json"
	mu := deps.LockForFile(filename)
	mu.Lock()
	defer mu.Unlock()

	jf := mustLoadJSONFile(deps, filename)

	raw, ok := jf.Items[id]
	if !ok {
		errJSON(w, http.StatusBadRequest, "kpr not found")
		return
	}

	var cur map[string]any
	if err := json.Unmarshal(raw, &cur); err != nil {
		errJSON(w, http.StatusInternalServerError, "invalid stored kpr")
		return
	}

	st := str(cur["status"])
	if st != "draft" && st != "submitted" {
		errJSON(w, http.StatusConflict, "cancel allowed only for draft/submitted")
		return
	}

	cur["status"] = "cancelled"
	cur["updated_at"] = time.Now().UTC().Format(time.RFC3339)
	jf.Items[id] = mustJSON(cur)

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

// ===== internals =====

func kprGetByBooking(deps Stage8Deps, w http.ResponseWriter, r *http.Request) {
	if !deps.StorageReady() {
		errJSON(w, http.StatusServiceUnavailable, "storage not ready")
		return
	}

	bookingID := strings.TrimSpace(r.URL.Query().Get("booking_id"))
	if bookingID == "" {
		errJSON(w, http.StatusBadRequest, "booking_id is required")
		return
	}

	admin := auth.IsAdmin(r)
	items := deps.GetItems("kpr_applications.json")

	for _, v := range items {
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		if str(m["booking_id"]) != bookingID {
			continue
		}

		out := map[string]any{
			"id":         m["id"],
			"booking_id": m["booking_id"],
			"site_id":    m["site_id"],
			"subsite_id": m["subsite_id"],
			"zone_id":    m["zone_id"],
			"status":     m["status"],
			"notes":      m["notes"],
			"created_at": m["created_at"],
			"updated_at": m["updated_at"],
		}

		// Guest-safe: hide NIK/address, show basic only
		if cust, ok := m["customer"].(map[string]any); ok {
			outCust := map[string]any{
				"name":  cust["name"],
				"phone": cust["phone"],
				"email": cust["email"],
			}
			if admin {
				outCust["nik"] = cust["nik"]
				outCust["address"] = cust["address"]
			}
			out["customer"] = outCust
		}

		if admin {
			out["price"] = m["price"]
		}
		okData(w, out)
		return
	}

	errJSON(w, http.StatusNotFound, "kpr not found")
}

func kprCreate(deps Stage8Deps, w http.ResponseWriter, r *http.Request) {
	if !deps.StorageReady() {
		errJSON(w, http.StatusServiceUnavailable, "storage not ready")
		return
	}
	if !auth.RequireAdmin(w, r) {
		return
	}

	var p kprCreatePayload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid json")
		return
	}
	p.BookingID = strings.TrimSpace(p.BookingID)
	p.Notes = strings.TrimSpace(p.Notes)
	if p.BookingID == "" {
		errJSON(w, http.StatusBadRequest, "booking_id is required")
		return
	}

	// booking must exist & be confirmed
	bookings := deps.GetItems("bookings.json")
	bAny, ok := bookings[p.BookingID]
	if !ok {
		errJSON(w, http.StatusBadRequest, "booking not found")
		return
	}
	bMap, ok := bAny.(map[string]any)
	if !ok {
		errJSON(w, http.StatusInternalServerError, "invalid stored booking")
		return
	}
	if str(bMap["status"]) != "confirmed" {
		errJSON(w, http.StatusConflict, "booking not confirmed")
		return
	}

	filename := "kpr_applications.json"
	mu := deps.LockForFile(filename)
	mu.Lock()
	defer mu.Unlock()

	jf := mustLoadJSONFile(deps, filename)

	// only 1 KPR per booking
	for _, raw := range jf.Items {
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			continue
		}
		if str(m["booking_id"]) == p.BookingID {
			errJSON(w, http.StatusConflict, "kpr already exists for booking")
			return
		}
	}

	id := genID("kpr")
	now := time.Now().UTC().Format(time.RFC3339)

	obj := map[string]any{
		"id":         id,
		"booking_id": p.BookingID,
		"site_id":    bMap["site_id"],
		"subsite_id": bMap["subsite_id"],
		"zone_id":    bMap["zone_id"],
		"customer": map[string]any{
			"name":    "",
			"phone":   "",
			"email":   "",
			"nik":     "",
			"address": "",
		},
		"price": map[string]any{
			"land_price":    0,
			"dp_amount":     0,
			"dp_paid":       0,
			"loan_amount":   0,
			"tenor_months":  0,
			"interest_rate": 0,
			"admin_fee":     0,
			"other_fee":     0,
			"total":         0,
		},
		"status":     "draft",
		"notes":      p.Notes,
		"created_at": now,
		"updated_at": now,
	}

	jf.Items[id] = mustJSON(obj)

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

func kprUpdate(deps Stage8Deps, id string, w http.ResponseWriter, r *http.Request) {
	if !deps.StorageReady() {
		errJSON(w, http.StatusServiceUnavailable, "storage not ready")
		return
	}
	if !auth.RequireAdmin(w, r) {
		return
	}

	filename := "kpr_applications.json"
	mu := deps.LockForFile(filename)
	mu.Lock()
	defer mu.Unlock()

	jf := mustLoadJSONFile(deps, filename)

	raw, ok := jf.Items[id]
	if !ok {
		errJSON(w, http.StatusBadRequest, "kpr not found")
		return
	}

	var cur map[string]any
	if err := json.Unmarshal(raw, &cur); err != nil {
		errJSON(w, http.StatusInternalServerError, "invalid stored kpr")
		return
	}

	st := str(cur["status"])
	if st != "draft" && st != "submitted" {
		errJSON(w, http.StatusConflict, "updates allowed only for draft/submitted")
		return
	}

	var p kprUpdatePayload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid json")
		return
	}

	if strings.TrimSpace(p.Notes) != "" {
		cur["notes"] = strings.TrimSpace(p.Notes)
	}

	if p.Customer != nil {
		cm, _ := cur["customer"].(map[string]any)
		if cm == nil {
			cm = map[string]any{}
		}
		if strings.TrimSpace(p.Customer.Name) != "" {
			cm["name"] = strings.TrimSpace(p.Customer.Name)
		}
		if strings.TrimSpace(p.Customer.Phone) != "" {
			cm["phone"] = strings.TrimSpace(p.Customer.Phone)
		}
		if strings.TrimSpace(p.Customer.Email) != "" {
			cm["email"] = strings.TrimSpace(p.Customer.Email)
		}
		if strings.TrimSpace(p.Customer.NIK) != "" {
			cm["nik"] = strings.TrimSpace(p.Customer.NIK)
		}
		if strings.TrimSpace(p.Customer.Address) != "" {
			cm["address"] = strings.TrimSpace(p.Customer.Address)
		}
		cur["customer"] = cm
	}

	if p.Price != nil {
		pm, _ := cur["price"].(map[string]any)
		if pm == nil {
			pm = map[string]any{}
		}
		pm["land_price"] = p.Price.LandPrice
		pm["dp_amount"] = p.Price.DpAmount
		// dp_paid is updated later by payments stage; keep current if exists
		if _, ok := pm["dp_paid"]; !ok {
			pm["dp_paid"] = 0
		}
		pm["loan_amount"] = p.Price.LoanAmount
		pm["tenor_months"] = p.Price.TenorMonths
		pm["interest_rate"] = p.Price.InterestRate
		pm["admin_fee"] = p.Price.AdminFee
		pm["other_fee"] = p.Price.OtherFee
		pm["total"] = p.Price.Total
		cur["price"] = pm
	}

	cur["updated_at"] = time.Now().UTC().Format(time.RFC3339)
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

func kprTransition(deps Stage8Deps, id string, w http.ResponseWriter, r *http.Request, from, to string) {
	kprTransitionWithValidate(deps, id, w, r, from, to, nil)
}

func kprTransitionWithValidate(deps Stage8Deps, id string, w http.ResponseWriter, r *http.Request, from, to string, validator func(map[string]any) error) {
	if !deps.StorageReady() {
		errJSON(w, http.StatusServiceUnavailable, "storage not ready")
		return
	}
	if !auth.RequireAdmin(w, r) {
		return
	}

	filename := "kpr_applications.json"
	mu := deps.LockForFile(filename)
	mu.Lock()
	defer mu.Unlock()

	jf := mustLoadJSONFile(deps, filename)

	raw, ok := jf.Items[id]
	if !ok {
		errJSON(w, http.StatusBadRequest, "kpr not found")
		return
	}

	var cur map[string]any
	if err := json.Unmarshal(raw, &cur); err != nil {
		errJSON(w, http.StatusInternalServerError, "invalid stored kpr")
		return
	}

	st := str(cur["status"])
	if st != from {
		errJSON(w, http.StatusConflict, "invalid status transition")
		return
	}

	if validator != nil {
		if err := validator(cur); err != nil {
			errJSON(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	cur["status"] = to
	cur["updated_at"] = time.Now().UTC().Format(time.RFC3339)
	if to == "approved" {
		cur["approved_at"] = cur["updated_at"]
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
	okData(w, map[string]any{"id": id, "status": to})
}

func validateKPRForApprove(cur map[string]any) error {
	// require customer basic + price.loan_amount + tenor_months > 0
	cm, _ := cur["customer"].(map[string]any)
	if strings.TrimSpace(str(cm["name"])) == "" {
		return errBad("customer.name is required")
	}
	pm, _ := cur["price"].(map[string]any)
	loan, _ := pm["loan_amount"].(float64)
	tenorF, ok := pm["tenor_months"].(float64) // json numbers decode to float64 in map
	if ok {
		// stored from update might be float64
	}
	tenorI := 0
	if ok {
		tenorI = int(tenorF)
	} else if t, ok2 := pm["tenor_months"].(int); ok2 {
		tenorI = t
	}
	if loan <= 0 {
		return errBad("price.loan_amount must be > 0")
	}
	if tenorI <= 0 {
		return errBad("price.tenor_months must be > 0")
	}
	return nil
}

func mustLoadJSONFile(deps Stage8Deps, filename string) storage.JSONFile {
	loaded := deps.Loaded()
	jf, ok := loaded[filename]
	if !ok || jf.Items == nil {
		return storage.JSONFile{
			Meta:  map[string]any{"version": 1, "updated_at": nil},
			Items: map[string]json.RawMessage{},
		}
	}
	if jf.Meta == nil {
		jf.Meta = map[string]any{"version": 1, "updated_at": nil}
	}
	if jf.Items == nil {
		jf.Items = map[string]json.RawMessage{}
	}
	return jf
}
