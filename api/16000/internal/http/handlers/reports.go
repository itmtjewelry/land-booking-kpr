package handlers

import (
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

func ReportKPRStatement(deps Stage8Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !deps.StorageReady() {
			errJSON(w, http.StatusServiceUnavailable, "storage not ready")
			return
		}
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}

		kprID := strings.TrimSpace(r.URL.Query().Get("kpr_id"))
		if kprID == "" {
			errJSON(w, http.StatusBadRequest, "kpr_id is required")
			return
		}

		isAdmin := adminTokenOK(r)

		// Load required files (in-memory)
		kprs := deps.GetItems("kpr_applications.json")
		kprAny, ok := kprs[kprID]
		if !ok {
			errJSON(w, http.StatusBadRequest, "kpr not found")
			return
		}
		kpr, ok := kprAny.(map[string]any)
		if !ok {
			errJSON(w, http.StatusInternalServerError, "invalid kpr data")
			return
		}

		bookingID := str(kpr["booking_id"])
		siteID := str(kpr["site_id"])
		subsiteID := str(kpr["subsite_id"])
		zoneID := str(kpr["zone_id"])

		// Booking
		bookings := deps.GetItems("bookings.json")
		var booking map[string]any
		if bookingID != "" {
			if bAny, ok := bookings[bookingID]; ok {
				if b, ok := bAny.(map[string]any); ok {
					booking = b
				}
			}
		}

		// Site/Subsite/Zone
		site := getItemMap(deps.GetItems("sites.json"), siteID)
		subsite := getItemMap(deps.GetItems("subsites.json"), subsiteID)
		zone := getItemMap(deps.GetItems("zones.json"), zoneID)

		// Plan (find by kpr_id)
		plans := deps.GetItems("installment_plans.json")
		planID, plan := findPlanMapByKPR(plans, kprID)
		if plan == nil {
			errJSON(w, http.StatusBadRequest, "installment plan not found for kpr")
			return
		}
		_ = planID

		// Payments
		pays := deps.GetItems("payments.json")
		payList := filterPaymentsForKPR(pays, kprID)

		// Customer (guest-safe)
		customer := map[string]any{}
		if cAny, ok := kpr["customer"].(map[string]any); ok {
			customer["name"] = str(cAny["name"])
			customer["phone"] = str(cAny["phone"])
			customer["email"] = str(cAny["email"])
			if isAdmin {
				// only admin sees sensitive fields
				if v := str(cAny["nik"]); v != "" {
					customer["nik"] = v
				}
				if v := str(cAny["address"]); v != "" {
					customer["address"] = v
				}
			}
		}

		// Price summary
		price := map[string]any{}
		if pAny, ok := kpr["price"].(map[string]any); ok {
			landPrice := floatFromAny(pAny["land_price"])
			dpAmount := floatFromAny(pAny["dp_amount"])
			dpPaid := floatFromAny(pAny["dp_paid"])
			loanAmount := floatFromAny(pAny["loan_amount"])
			tenor := intFromAny(pAny["tenor_months"])

			monthly := floatFromAny(plan["monthly_amount"])
			if monthly <= 0 && tenor > 0 {
				monthly = loanAmount / float64(tenor)
			}

			price["land_price"] = landPrice
			price["dp_amount"] = dpAmount
			price["dp_paid"] = dpPaid
			price["loan_amount"] = loanAmount
			price["tenor_months"] = tenor
			price["monthly_amount"] = monthly
		}

		// Schedule + progress
		schedule := normalizeSchedule(plan["schedule"])

		// Stage 11: penalties/late fees (computed)
		asOfStr := strings.TrimSpace(r.URL.Query().Get("as_of"))
		asOf, _ := parseAsOf(asOfStr)
		asOf = asOf.UTC()
		lateFeesDue := 0.0
		overdue := make([]map[string]any, 0, 8)
		for _, it := range schedule {
			no := intFromAny(it["no"])
			amt := floatFromAny(it["amount"])
			paid := floatFromAny(it["paid_amount"])
			st := str(it["status"])
			if st == "paid" || approxEqual(paid, amt) {
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
			lateFeesDue += pen
			overdue = append(overdue, map[string]any{
				"no":             no,
				"due_date":       dueStr,
				"amount":         amt,
				"paid_amount":    paid,
				"status":         st,
				"days_overdue":   do,
				"months_overdue": mo,
				"penalty_due":    pen,
			})
		}
		instTotal := len(schedule)
		instPaidCount := 0
		principalPaid := 0.0
		for _, it := range schedule {
			amt := floatFromAny(it["amount"])
			paid := floatFromAny(it["paid_amount"])
			principalPaid += paid
			if approxEqual(paid, amt) {
				instPaidCount++
			}
		}

		dpAmount := floatFromAny(price["dp_amount"])
		dpPaid := floatFromAny(price["dp_paid"])
		dpRemaining := dpAmount - dpPaid
		if dpRemaining < 0 {
			dpRemaining = 0
		}

		loanAmount := floatFromAny(price["loan_amount"])
		principalRemaining := loanAmount - principalPaid
		if principalRemaining < 0 {
			principalRemaining = 0
		}

		overall := str(kpr["status"])
		if overall == "" {
			overall = "active"
		}

		progress := map[string]any{
			"dp_remaining":            dpRemaining,
			"installments_paid_count": instPaidCount,
			"installments_total":      instTotal,
			"principal_paid":          principalPaid,
			"principal_remaining":     principalRemaining,
			"overall_status":          overall,
		}

		out := map[string]any{
			"kpr_id":               str(kpr["id"]),
			"booking_id":           bookingID,
			"customer":             customer,
			"site":                 pickIDName(site),
			"subsite":              pickIDName(subsite),
			"zone":                 pickIDName(zone),
			"price":                price,
			"progress":             progress,
			"late_fees_due":        lateFeesDue,
			"overdue_installments": overdue,
			"schedule":             schedule,
			"payments":             guestSafePayments(payList, isAdmin),
			"generated_at":         time.Now().UTC().Format(time.RFC3339),
		}

		// Optional: include booking summary if present
		if booking != nil {
			out["booking"] = map[string]any{
				"id":         str(booking["id"]),
				"status":     str(booking["status"]),
				"start_date": str(booking["start_date"]),
				"end_date":   str(booking["end_date"]),
				"created_at": str(booking["created_at"]),
				"updated_at": str(booking["updated_at"]),
			}
		}

		okData(w, out)
	}
}

func ReportZoneSummary(deps Stage8Deps) http.HandlerFunc {
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

		zoneID := strings.TrimSpace(r.URL.Query().Get("zone_id"))
		if zoneID == "" {
			errJSON(w, http.StatusBadRequest, "zone_id is required")
			return
		}

		bookings := deps.GetItems("bookings.json")
		kprs := deps.GetItems("kpr_applications.json")
		plans := deps.GetItems("installment_plans.json")

		bookingCount := 0
		confirmedCount := 0

		dpCollected := 0.0
		principalPaid := 0.0
		principalRemaining := 0.0

		for _, bAny := range bookings {
			b, ok := bAny.(map[string]any)
			if !ok {
				continue
			}
			if str(b["zone_id"]) != zoneID {
				continue
			}
			bookingCount++
			if str(b["status"]) == "confirmed" {
				confirmedCount++
			}

			// find KPR for this booking
			bookingID := str(b["id"])
			for _, kAny := range kprs {
				k, ok := kAny.(map[string]any)
				if !ok {
					continue
				}
				if str(k["booking_id"]) != bookingID {
					continue
				}
				pm, _ := k["price"].(map[string]any)
				dpCollected += floatFromAny(pm["dp_paid"])
				loan := floatFromAny(pm["loan_amount"])

				_, plan := findPlanMapByKPR(plans, str(k["id"]))
				if plan != nil {
					sched := normalizeSchedule(plan["schedule"])
					paid := 0.0
					for _, it := range sched {
						paid += floatFromAny(it["paid_amount"])
					}
					principalPaid += paid
					rem := loan - paid
					if rem < 0 {
						rem = 0
					}
					principalRemaining += rem
				} else {
					// no plan, assume remaining = loan
					principalRemaining += loan
				}
			}
		}

		out := map[string]any{
			"zone_id": zoneID,
			"counts": map[string]any{
				"bookings_total":     bookingCount,
				"bookings_confirmed": confirmedCount,
			},
			"money": map[string]any{
				"dp_collected":          dpCollected,
				"principal_paid":        principalPaid,
				"principal_outstanding": principalRemaining,
			},
			"generated_at": time.Now().UTC().Format(time.RFC3339),
		}

		okData(w, out)
	}
}

func ReportPortfolio(deps Stage8Deps) http.HandlerFunc {
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

		bookings := deps.GetItems("bookings.json")
		kprs := deps.GetItems("kpr_applications.json")
		plans := deps.GetItems("installment_plans.json")

		bookingByStatus := map[string]int{}
		for _, bAny := range bookings {
			b, ok := bAny.(map[string]any)
			if !ok {
				continue
			}
			bookingByStatus[str(b["status"])]++
		}

		kprByStatus := map[string]int{}
		dpCollected := 0.0
		principalPaid := 0.0
		principalRemaining := 0.0

		for _, kAny := range kprs {
			k, ok := kAny.(map[string]any)
			if !ok {
				continue
			}
			kprByStatus[str(k["status"])]++

			pm, _ := k["price"].(map[string]any)
			dpCollected += floatFromAny(pm["dp_paid"])
			loan := floatFromAny(pm["loan_amount"])

			_, plan := findPlanMapByKPR(plans, str(k["id"]))
			if plan != nil {
				sched := normalizeSchedule(plan["schedule"])
				paid := 0.0
				for _, it := range sched {
					paid += floatFromAny(it["paid_amount"])
				}
				principalPaid += paid
				rem := loan - paid
				if rem < 0 {
					rem = 0
				}
				principalRemaining += rem
			} else {
				principalRemaining += loan
			}
		}

		out := map[string]any{
			"counts": map[string]any{
				"bookings_by_status": bookingByStatus,
				"kpr_by_status":      kprByStatus,
			},
			"money": map[string]any{
				"dp_collected":          dpCollected,
				"principal_paid":        principalPaid,
				"principal_outstanding": principalRemaining,
			},
			"generated_at": time.Now().UTC().Format(time.RFC3339),
		}

		okData(w, out)
	}
}

/* ---------------- helpers ---------------- */

func adminTokenOK(r *http.Request) bool {
	token := strings.TrimSpace(r.Header.Get("X-Admin-Token"))
	if token == "" {
		return false
	}
	secret := strings.TrimSpace(os.Getenv("ADMIN_TOKEN"))
	if secret == "" {
		return false
	}
	return token == secret
}

func requireAdminQuick(w http.ResponseWriter, r *http.Request) bool {
	if !adminTokenOK(r) {
		errJSON(w, http.StatusUnauthorized, "admin token required")
		return false
	}
	return true
}

func getItemMap(items map[string]any, id string) map[string]any {
	if id == "" {
		return nil
	}
	if v, ok := items[id]; ok {
		if m, ok := v.(map[string]any); ok {
			return m
		}
	}
	return nil
}

func pickIDName(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{"id": "", "name": ""}
	}
	return map[string]any{
		"id":   str(m["id"]),
		"name": str(m["name"]),
	}
}

func findPlanMapByKPR(plans map[string]any, kprID string) (string, map[string]any) {
	for id, v := range plans {
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		if str(m["kpr_id"]) == kprID {
			return id, m
		}
	}
	return "", nil
}

func normalizeSchedule(v any) []map[string]any {
	raw, ok := v.([]any)
	if !ok || len(raw) == 0 {
		return []map[string]any{}
	}
	out := make([]map[string]any, 0, len(raw))
	for _, it := range raw {
		m, ok := it.(map[string]any)
		if !ok {
			continue
		}
		// ensure missing fields default
		if _, ok := m["paid_amount"]; !ok {
			m["paid_amount"] = 0
		}
		if _, ok := m["status"]; !ok {
			amt := floatFromAny(m["amount"])
			paid := floatFromAny(m["paid_amount"])
			if approxEqual(amt, paid) && amt > 0 {
				m["status"] = "paid"
			} else if paid > 0 {
				m["status"] = "partial"
			} else {
				m["status"] = "unpaid"
			}
		}
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool {
		return intFromAny(out[i]["no"]) < intFromAny(out[j]["no"])
	})
	return out
}

func filterPaymentsForKPR(items map[string]any, kprID string) []map[string]any {
	out := make([]map[string]any, 0, 32)
	for _, v := range items {
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		if str(m["kpr_id"]) == kprID {
			out = append(out, m)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		ai := str(out[i]["created_at"])
		aj := str(out[j]["created_at"])
		if ai == aj {
			return str(out[i]["id"]) < str(out[j]["id"])
		}
		return ai < aj
	})
	return out
}

func guestSafePayments(in []map[string]any, isAdmin bool) []map[string]any {
	out := make([]map[string]any, 0, len(in))
	for _, p := range in {
		x := map[string]any{
			"id":             str(p["id"]),
			"type":           str(p["type"]),
			"installment_no": intFromAny(p["installment_no"]),
			"amount":         floatFromAny(p["amount"]),
			"paid_at":        str(p["paid_at"]),
			"method":         str(p["method"]),
			"notes":          str(p["notes"]),
		}
		if isAdmin {
			if v := str(p["reference"]); v != "" {
				x["reference"] = v
			}
		}
		out = append(out, x)
	}
	return out
}
