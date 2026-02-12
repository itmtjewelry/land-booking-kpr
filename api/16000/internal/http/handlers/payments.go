package handlers

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/itmtjewelry/land-booking-kpr/internal/auth"
	"github.com/itmtjewelry/land-booking-kpr/internal/storage"
)

type paymentCreatePayload struct {
	KPRID         string  `json:"kpr_id"`
	BookingID     string  `json:"booking_id"` // optional (server can derive)
	InstallmentNo int     `json:"installment_no"`
	Amount        float64 `json:"amount"`
	Method        string  `json:"method"`
	Reference     string  `json:"reference"`
	Notes         string  `json:"notes"`
	PaidAt        string  `json:"paid_at"` // optional YYYY-MM-DD
}

func PaymentsCollection(deps Stage8Deps, w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		paymentsGet(deps, w, r)
	case http.MethodPost:
		paymentsCreate(deps, w, r)
	default:
		methodNotAllowed(w)
	}
}

func paymentsGet(deps Stage8Deps, w http.ResponseWriter, r *http.Request) {
	if !deps.StorageReady() {
		errJSON(w, http.StatusServiceUnavailable, "storage not ready")
		return
	}
	if !auth.RequireAdmin(w, r) {
		return
	}

	kprID := strings.TrimSpace(r.URL.Query().Get("kpr_id"))
	bookingID := strings.TrimSpace(r.URL.Query().Get("booking_id"))
	if kprID == "" && bookingID == "" {
		errJSON(w, http.StatusBadRequest, "kpr_id or booking_id is required")
		return
	}

	items := deps.GetItems("payments.json")
	out := make([]map[string]any, 0, 32)

	for _, v := range items {
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		if kprID != "" && str(m["kpr_id"]) != kprID {
			continue
		}
		if bookingID != "" && str(m["booking_id"]) != bookingID {
			continue
		}
		out = append(out, m)
	}

	sort.Slice(out, func(i, j int) bool {
		// stable by created_at, then id
		ai := str(out[i]["created_at"])
		aj := str(out[j]["created_at"])
		if ai == aj {
			return str(out[i]["id"]) < str(out[j]["id"])
		}
		return ai < aj
	})

	okData(w, out)
}

func paymentsCreate(deps Stage8Deps, w http.ResponseWriter, r *http.Request) {
	if !deps.StorageReady() {
		errJSON(w, http.StatusServiceUnavailable, "storage not ready")
		return
	}
	if !auth.RequireAdmin(w, r) {
		return
	}

	var p paymentCreatePayload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid json")
		return
	}

	p.KPRID = strings.TrimSpace(p.KPRID)
	p.BookingID = strings.TrimSpace(p.BookingID)
	p.Method = strings.TrimSpace(p.Method)
	p.Reference = strings.TrimSpace(p.Reference)
	p.Notes = strings.TrimSpace(p.Notes)
	p.PaidAt = strings.TrimSpace(p.PaidAt)

	if p.KPRID == "" {
		errJSON(w, http.StatusBadRequest, "kpr_id is required")
		return
	}
	if p.InstallmentNo < 0 {
		errJSON(w, http.StatusBadRequest, "installment_no must be >= 0")
		return
	}
	if p.Amount <= 0 {
		errJSON(w, http.StatusBadRequest, "amount must be > 0")
		return
	}
	if p.Method == "" {
		errJSON(w, http.StatusBadRequest, "method is required")
		return
	}

	paidAt := p.PaidAt
	if paidAt == "" {
		paidAt = time.Now().UTC().Format("2006-01-02")
	} else {
		if _, err := time.Parse("2006-01-02", paidAt); err != nil {
			errJSON(w, http.StatusBadRequest, "paid_at must be YYYY-MM-DD")
			return
		}
	}

	// Lock order to avoid deadlock
	lockKPR := deps.LockForFile("kpr_applications.json")
	lockPlan := deps.LockForFile("installment_plans.json")
	lockPay := deps.LockForFile("payments.json")

	lockKPR.Lock()
	defer lockKPR.Unlock()
	lockPlan.Lock()
	defer lockPlan.Unlock()
	lockPay.Lock()
	defer lockPay.Unlock()

	// Load fresh JSONFile snapshots from in-memory
	kprJF := mustLoadJSONFile(deps, "kpr_applications.json")
	planJF := mustLoadPlanFile(deps, "installment_plans.json")
	payJF := mustLoadPlanFile(deps, "payments.json") // same shape helper

	// Find KPR raw
	kprRaw, ok := kprJF.Items[p.KPRID]
	if !ok {
		errJSON(w, http.StatusBadRequest, "kpr not found")
		return
	}
	var kpr map[string]any
	if err := json.Unmarshal(kprRaw, &kpr); err != nil {
		errJSON(w, http.StatusInternalServerError, "invalid stored kpr")
		return
	}
	if str(kpr["status"]) != "approved" && str(kpr["status"]) != "completed" {
		errJSON(w, http.StatusConflict, "payments allowed only for approved/completed kpr")
		return
	}

	bookingID := str(kpr["booking_id"])
	if bookingID == "" {
		errJSON(w, http.StatusInternalServerError, "kpr missing booking_id")
		return
	}
	if p.BookingID != "" && p.BookingID != bookingID {
		errJSON(w, http.StatusBadRequest, "booking_id mismatch")
		return
	}

	// Find installment plan for this KPR
	planID, planObj, err := findPlanByKPR(planJF, p.KPRID)
	if err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}

	// Validate payment against remaining (reject overpay)
	if p.InstallmentNo == 0 {
		// DP
		pm, _ := kpr["price"].(map[string]any)
		dpAmount := floatFromAny(pm["dp_amount"])
		dpPaid := floatFromAny(pm["dp_paid"])
		remain := dpAmount - dpPaid
		if remain <= 0 {
			errJSON(w, http.StatusConflict, "dp already fully paid")
			return
		}
		if p.Amount > remain+0.0000001 {
			errJSON(w, http.StatusConflict, "overpayment: exceeds remaining dp")
			return
		}
		// apply
		pm["dp_paid"] = dpPaid + p.Amount
		kpr["price"] = pm
	} else {
		// Installment payment must match schedule
		sched, ok := planObj["schedule"].([]any)
		if !ok || len(sched) == 0 {
			errJSON(w, http.StatusInternalServerError, "invalid plan schedule")
			return
		}
		if p.InstallmentNo < 1 || p.InstallmentNo > len(sched) {
			errJSON(w, http.StatusBadRequest, "installment_no out of range")
			return
		}

		itAny := sched[p.InstallmentNo-1]
		it, ok := itAny.(map[string]any)
		if !ok {
			errJSON(w, http.StatusInternalServerError, "invalid schedule item")
			return
		}
		amt := floatFromAny(it["amount"])
		paid := floatFromAny(it["paid_amount"])
		remain := amt - paid
		if remain <= 0 {
			errJSON(w, http.StatusConflict, "installment already fully paid")
			return
		}
		if p.Amount > remain+0.0000001 {
			errJSON(w, http.StatusConflict, "overpayment: exceeds remaining installment")
			return
		}

		// apply schedule update (derived state)
		newPaid := paid + p.Amount
		it["paid_amount"] = newPaid
		if approxEqual(newPaid, amt) {
			it["status"] = "paid"
		} else {
			it["status"] = "partial"
		}
		sched[p.InstallmentNo-1] = it
		planObj["schedule"] = sched
		planObj["updated_at"] = time.Now().UTC().Format(time.RFC3339)
	}

	// Append payment record (append-only)
	paymentID := genID("payment")
	now := time.Now().UTC().Format(time.RFC3339)
	pType := "installment"
	if p.InstallmentNo == 0 {
		pType = "dp"
	}

	payObj := map[string]any{
		"id":             paymentID,
		"type":           pType,
		"kpr_id":         p.KPRID,
		"booking_id":     bookingID,
		"installment_no": p.InstallmentNo,
		"amount":         p.Amount,
		"paid_at":        paidAt,
		"method":         p.Method,
		"reference":      p.Reference,
		"notes":          p.Notes,
		"created_at":     now,
	}

	payJF.Items[paymentID] = mustJSON(payObj)

	// If installments all paid => KPR completed (derived)
	if p.InstallmentNo != 0 {
		if allInstallmentsPaid(planObj) {
			kpr["status"] = "completed"
			kpr["updated_at"] = time.Now().UTC().Format(time.RFC3339)
		}
	}

	// Persist all modified files atomically
	// 1) payments.json
	if err := storage.WriteJSONFileAtomic(deps.StorageDir(), "payments.json", payJF); err != nil {
		errJSON(w, http.StatusInternalServerError, "write payments failed")
		return
	}
	// 2) installment_plans.json (only changed for installment payments)
	planJF.Items[planID] = mustJSON(planObj)
	if err := storage.WriteJSONFileAtomic(deps.StorageDir(), "installment_plans.json", planJF); err != nil {
		errJSON(w, http.StatusInternalServerError, "write plan failed")
		return
	}
	// 3) kpr_applications.json (dp_paid or status completed)
	kpr["updated_at"] = time.Now().UTC().Format(time.RFC3339)
	kprJF.Items[p.KPRID] = mustJSON(kpr)
	if err := storage.WriteJSONFileAtomic(deps.StorageDir(), "kpr_applications.json", kprJF); err != nil {
		errJSON(w, http.StatusInternalServerError, "write kpr failed")
		return
	}

	if err := deps.ReloadCore(); err != nil {
		errJSON(w, http.StatusInternalServerError, "reload failed")
		return
	}

	okData(w, map[string]any{"id": paymentID})
}

func findPlanByKPR(planJF storage.JSONFile, kprID string) (string, map[string]any, error) {
	for id, raw := range planJF.Items {
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			continue
		}
		if str(m["kpr_id"]) == kprID {
			return id, m, nil
		}
	}
	return "", nil, errBad("installment plan not found for kpr")
}

func allInstallmentsPaid(plan map[string]any) bool {
	sched, ok := plan["schedule"].([]any)
	if !ok || len(sched) == 0 {
		return false
	}
	for _, itAny := range sched {
		it, ok := itAny.(map[string]any)
		if !ok {
			return false
		}
		amt := floatFromAny(it["amount"])
		paid := floatFromAny(it["paid_amount"])
		if !approxEqual(paid, amt) {
			return false
		}
	}
	return true
}

func approxEqual(a, b float64) bool {
	if a == b {
		return true
	}
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff < 0.000001
}

func floatFromAny(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(t), 64)
		return f
	default:
		return 0
	}
}

// Optional helper if you ever want sorted payment list (not required in JSON)
func sortPaymentsByCreatedAt(arr []map[string]any) {
	sort.Slice(arr, func(i, j int) bool {
		ai := str(arr[i]["created_at"])
		aj := str(arr[j]["created_at"])
		if ai == aj {
			return str(arr[i]["id"]) < str(arr[j]["id"])
		}
		return ai < aj
	})
}
