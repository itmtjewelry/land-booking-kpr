package handlers

import (
	"math"
	"strconv"
	"strings"
	"time"
)

const penaltyFlatPerMonth = 50000.0 // Rp 50,000
const penaltyCapPct = 0.10          // 10%

type PenaltyLine struct {
	InstallmentNo int            `json:"installment_no"`
	DueDate       string         `json:"due_date"`
	Amount        float64        `json:"amount"`
	PaidAmount    float64        `json:"paid_amount"`
	Status        string         `json:"status"`
	DaysOverdue   int            `json:"days_overdue"`
	MonthsOverdue int            `json:"months_overdue"`
	PenaltyDue    float64        `json:"penalty_due"`
	Meta          map[string]any `json:"meta,omitempty"`
}

func parseAsOf(q string) (time.Time, error) {
	if strings.TrimSpace(q) == "" {
		return time.Now().UTC(), nil
	}
	// YYYY-MM-DD
	return time.Parse("2006-01-02", q)
}

func monthsOverdue(due time.Time, asOf time.Time) int {
	if !asOf.After(due) {
		return 0
	}
	// Count month boundary crossings: if due is 2026-03-05 and asOf is 2026-03-06 => 1 month overdue bucket
	y1, m1, _ := due.Date()
	y2, m2, _ := asOf.Date()
	months := (y2-y1)*12 + int(m2-m1) + 1
	if months < 0 {
		return 0
	}
	return months
}

func daysOverdue(due time.Time, asOf time.Time) int {
	if !asOf.After(due) {
		return 0
	}
	return int(asOf.Sub(due).Hours() / 24)
}

func penaltyForInstallment(installmentAmount float64, months int) float64 {
	if months <= 0 {
		return 0
	}
	raw := float64(months) * penaltyFlatPerMonth
	cap := math.Round(installmentAmount * penaltyCapPct) // round to nearest rupiah
	if cap < 0 {
		cap = 0
	}
	if raw > cap {
		return cap
	}
	return raw
}

func monthBucket(asOf time.Time) string {
	// bucket for duplicate prevention: YYYY-MM
	y, m, _ := asOf.Date()
	return strconv.Itoa(y) + "-" + left2(int(m))
}

func left2(n int) string {
	s := strconv.Itoa(n)
	if len(s) == 1 {
		return "0" + s
	}
	return s
}
