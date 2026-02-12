package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/itmtjewelry/land-booking-kpr/internal/storage"
)

type penaltyChargeReq struct {
	KPRID         string `json:"kpr_id"`
	AsOf          string `json:"as_of"`
	InstallmentNo int    `json:"installment_no"`
	Notes         string `json:"notes"`
	Method        string `json:"method"`
	Reference     string `json:"reference"`
}

func PenaltiesCharge(deps Stage8Deps, w http.ResponseWriter, r *http.Request) {
	if !deps.StorageReady() {
		errJSON(w, http.StatusServiceUnavailable, "storage not ready")
		return
	}
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	if !requireAdminQuick(w, r) {
		return
	}

	var req penaltyChargeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid json")
		return
	}

	req.KPRID = strings.TrimSpace(req.KPRID)
	if req.KPRID == "" {
		errJSON(w, http.StatusBadRequest, "kpr_id is required")
		return
	}
	if req.InstallmentNo <= 0 {
		errJSON(w, http.StatusBadRequest, "installment_no must be >= 1")
		return
	}

	asOf, err := parseAsOf(strings.TrimSpace(req.AsOf))
	if err != nil {
		errJSON(w, http.StatusBadRequest, "invalid as_of (use YYYY-MM-DD)")
		return
	}
	asOf = asOf.UTC()
	bucket := monthBucket(asOf)

	// Load KPR
	kprs := deps.GetItems("kpr_applications.json")
	kAny, ok := kprs[req.KPRID]
	if !ok {
		errJSON(w, http.StatusBadRequest, "kpr not found")
		return
	}
	kpr, ok := kAny.(map[string]any)
	if !ok {
		errJSON(w, http.StatusInternalServerError, "invalid kpr data")
		return
	}

	// Load plan by KPR
	plans := deps.GetItems("installment_plans.json")
	_, plan := findPlanMapByKPR(plans, req.KPRID)
	if plan == nil {
		errJSON(w, http.StatusBadRequest, "installment plan not found")
		return
	}

	schedule := normalizeSchedule(plan["schedule"])

	var target map[string]any
	for _, it := range schedule {
		if intFromAny(it["no"]) == req.InstallmentNo {
			target = it
			break
		}
	}
	if target == nil {
		errJSON(w, http.StatusBadRequest, "installment not found")
		return
	}

	amount := floatFromAny(target["amount"])
	paid := floatFromAny(target["paid_amount"])
	status := str(target["status"])
	dueStr := str(target["due_date"])
	if dueStr == "" {
		errJSON(w, http.StatusBadRequest, "installment has no due_date")
		return
	}

	if status == "paid" || approxEqual(paid, amount) {
		errJSON(w, http.StatusConflict, "installment already paid")
		return
	}

	due, err := time.Parse("2006-01-02", dueStr)
	if err != nil {
		errJSON(w, http.StatusBadRequest, "invalid due_date")
		return
	}
	due = due.UTC()

	if !asOf.After(due) {
		errJSON(w, http.StatusConflict, "not overdue yet")
		return
	}

	mo := monthsOverdue(due, asOf)
	penalty := penaltyForInstallment(amount, mo)
	if penalty <= 0 {
		errJSON(w, http.StatusConflict, "penalty is zero")
		return
	}

	// Load payments via same helper used by payments.go (Items are json.RawMessage!)
	lockPay := deps.LockForFile("payments.json")
	lockPay.Lock()
	defer lockPay.Unlock()

	payJF := mustLoadPlanFile(deps, "payments.json")

	// Duplicate prevention: same kpr_id + installment_no + bucket
	for _, raw := range payJF.Items {
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			continue
		}
		if str(m["type"]) != "penalty" {
			continue
		}
		if str(m["kpr_id"]) != req.KPRID {
			continue
		}
		if intFromAny(m["installment_no"]) != req.InstallmentNo {
			continue
		}
		if str(m["bucket"]) == bucket {
			errJSON(w, http.StatusConflict, "penalty already charged for this month")
			return
		}
	}

	now := time.Now().UTC()
	id := "penalty_" + now.Format("20060102T150405.000000000Z")

	method := strings.TrimSpace(req.Method)
	if method == "" {
		method = "internal"
	}

	payment := map[string]any{
		"id":             id,
		"type":           "penalty",
		"kpr_id":         req.KPRID,
		"booking_id":     str(kpr["booking_id"]),
		"installment_no": req.InstallmentNo,
		"amount":         penalty,
		"bucket":         bucket,
		"method":         method,
		"notes":          strings.TrimSpace(req.Notes),
		"reference":      strings.TrimSpace(req.Reference),
		"paid_at":        now.Format("2006-01-02"),
		"created_at":     now.Format(time.RFC3339),
		"updated_at":     now.Format(time.RFC3339),
	}

	b, err := json.Marshal(payment)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, "marshal failed")
		return
	}

	payJF.Items[id] = json.RawMessage(b)
	if payJF.Meta == nil {
		payJF.Meta = map[string]any{}
	}
	payJF.Meta["updated_at"] = now.Format(time.RFC3339)

	if err := storage.WriteJSONFileAtomic(deps.StorageDir(), "payments.json", payJF); err != nil {
		errJSON(w, http.StatusInternalServerError, "write failed: "+err.Error())
		return
	}

	okData(w, map[string]any{
		"id":     id,
		"amount": penalty,
		"bucket": bucket,
	})
}
