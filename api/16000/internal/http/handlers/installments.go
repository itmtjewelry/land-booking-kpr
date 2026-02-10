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

func InstallmentsRead(deps Stage8Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		kprID := strings.TrimSpace(r.URL.Query().Get("kpr_id"))
		if kprID == "" {
			errJSON(w, http.StatusBadRequest, "kpr_id is required")
			return
		}

		items := deps.GetItems("installment_plans.json")
		for _, v := range items {
			m, ok := v.(map[string]any)
			if !ok {
				continue
			}
			if str(m["kpr_id"]) != kprID {
				continue
			}
			okData(w, m)
			return
		}
		errJSON(w, http.StatusNotFound, "installment plan not found")
	}
}

func InstallmentsGenerate(deps Stage8Deps, kprID string, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	if !deps.StorageReady() {
		errJSON(w, http.StatusServiceUnavailable, "storage not ready")
		return
	}
	if !auth.RequireAdmin(w, r) {
		return
	}

	kprID = strings.TrimSpace(kprID)
	if kprID == "" || strings.Contains(kprID, "/") {
		errJSON(w, http.StatusBadRequest, "invalid kpr id")
		return
	}

	// Ensure KPR exists and approved
	kprs := deps.GetItems("kpr_applications.json")
	var kpr map[string]any
	for _, v := range kprs {
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		if str(m["id"]) == kprID {
			kpr = m
			break
		}
	}
	if kpr == nil {
		errJSON(w, http.StatusBadRequest, "kpr not found")
		return
	}
	if str(kpr["status"]) != "approved" {
		errJSON(w, http.StatusConflict, "installments can be generated only when approved")
		return
	}

	pm, _ := kpr["price"].(map[string]any)
	loan, _ := pm["loan_amount"].(float64)

	tenor := intFromAny(pm["tenor_months"])
	if loan <= 0 || tenor <= 0 {
		errJSON(w, http.StatusBadRequest, "invalid loan_amount/tenor_months")
		return
	}

	monthly := loan / float64(tenor)

	approvedAt := str(kpr["approved_at"])
	apT, err := time.Parse(time.RFC3339, approvedAt)
	if err != nil {
		apT = time.Now().UTC()
	}
	// deterministic rule: first due date = day 5 of next month (UTC)
	y, m, _ := apT.Date()
	first := time.Date(y, m, 5, 0, 0, 0, 0, time.UTC).AddDate(0, 1, 0)

	type schedItem struct {
		No         int     `json:"no"`
		DueDate    string  `json:"due_date"`
		Amount     float64 `json:"amount"`
		PaidAmount float64 `json:"paid_amount"`
		Status     string  `json:"status"`
	}

	schedule := make([]schedItem, 0, tenor)
	for i := 0; i < tenor; i++ {
		d := first.AddDate(0, i, 0)
		schedule = append(schedule, schedItem{
			No:         i + 1,
			DueDate:    d.Format("2006-01-02"),
			Amount:     monthly,
			PaidAmount: 0,
			Status:     "unpaid",
		})
	}

	filename := "installment_plans.json"
	mu := deps.LockForFile(filename)
	mu.Lock()
	defer mu.Unlock()

	jf := mustLoadPlanFile(deps, filename)

	// prevent duplicate plan for same kpr
	for _, raw := range jf.Items {
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			continue
		}
		if str(m["kpr_id"]) == kprID {
			errJSON(w, http.StatusConflict, "installment plan already exists")
			return
		}
	}

	id := genID("plan")
	now := time.Now().UTC().Format(time.RFC3339)

	obj := map[string]any{
		"id":             id,
		"kpr_id":         kprID,
		"formula":        "flat",
		"loan_amount":    loan,
		"tenor_months":   tenor,
		"monthly_amount": monthly,
		"schedule":       schedule,
		"created_at":     now,
		"updated_at":     now,
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

func mustLoadPlanFile(deps Stage8Deps, filename string) storage.JSONFile {
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

func intFromAny(v any) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case string:
		i, _ := strconv.Atoi(strings.TrimSpace(t))
		return i
	default:
		return 0
	}
}

func sortScheduleIfNeeded(m map[string]any) {
	arr, ok := m["schedule"].([]any)
	if !ok {
		return
	}
	sort.Slice(arr, func(i, j int) bool {
		mi, _ := arr[i].(map[string]any)
		mj, _ := arr[j].(map[string]any)
		return str(mi["due_date"]) < str(mj["due_date"])
	})
	m["schedule"] = arr
}
