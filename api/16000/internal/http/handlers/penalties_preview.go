package handlers

import (
	"net/http"
	"strings"
	"time"
)

func PenaltiesPreview(deps Stage8Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !deps.StorageReady() {
			errJSON(w, http.StatusServiceUnavailable, "storage not ready")
			return
		}
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		if !requireAdminQuick(w, r) {
			return
		}

		kprID := strings.TrimSpace(r.URL.Query().Get("kpr_id"))
		if kprID == "" {
			errJSON(w, http.StatusBadRequest, "kpr_id is required")
			return
		}
		asOfStr := strings.TrimSpace(r.URL.Query().Get("as_of"))
		asOf, err := parseAsOf(asOfStr)
		if err != nil {
			errJSON(w, http.StatusBadRequest, "invalid as_of (use YYYY-MM-DD)")
			return
		}
		asOf = asOf.UTC()

		// Load KPR and plan
		kprs := deps.GetItems("kpr_applications.json")
		kAny, ok := kprs[kprID]
		if !ok {
			errJSON(w, http.StatusBadRequest, "kpr not found")
			return
		}
		kpr, ok := kAny.(map[string]any)
		if !ok {
			errJSON(w, http.StatusInternalServerError, "invalid kpr data")
			return
		}

		plans := deps.GetItems("installment_plans.json")
		_, plan := findPlanMapByKPR(plans, kprID)
		if plan == nil {
			errJSON(w, http.StatusBadRequest, "installment plan not found")
			return
		}

		sched := normalizeSchedule(plan["schedule"])
		lines := make([]PenaltyLine, 0, 8)
		total := 0.0

		for _, it := range sched {
			no := intFromAny(it["no"])
			amt := floatFromAny(it["amount"])
			paid := floatFromAny(it["paid_amount"])
			status := str(it["status"])

			// only unpaid/partial are candidates
			if status == "paid" || approxEqual(paid, amt) {
				continue
			}

			dueStr := str(it["due_date"])
			if dueStr == "" {
				continue
			}
			due, err := time.Parse("2006-01-02", dueStr)
			if err != nil {
				continue
			}
			due = due.UTC()

			if !asOf.After(due) {
				continue
			}

			mo := monthsOverdue(due, asOf)
			do := daysOverdue(due, asOf)
			pen := penaltyForInstallment(amt, mo)
			if pen <= 0 {
				continue
			}

			lines = append(lines, PenaltyLine{
				InstallmentNo: no,
				DueDate:       dueStr,
				Amount:        amt,
				PaidAmount:    paid,
				Status:        status,
				DaysOverdue:   do,
				MonthsOverdue: mo,
				PenaltyDue:    pen,
			})
			total += pen
		}

		out := map[string]any{
			"kpr_id":        str(kpr["id"]),
			"as_of":         asOf.Format("2006-01-02"),
			"bucket":        monthBucket(asOf),
			"total_penalty": total,
			"lines":         lines,
		}
		okData(w, out)
	}
}
