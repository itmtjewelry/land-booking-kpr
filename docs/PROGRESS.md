# **SPEC_LOCKED — READY FOR IMPLEMENTATION**Project Progress — Land Booking + KPR

> This file tracks the CURRENT STATE of the project.
> Always read this file before making changes.

---

## 🔖 Current State
Stage 8 checklist: APPROVED & LOCKED

Implementation: NOT STARTED

Awaiting: your explicit “Proceed with Stage 8 implementation.”

Implementation: NOT STARTED

Awaiting: YOUR EXPLICIT ACCEPTANCE

---

## ✅ Completed (Locked)
- ✅ Step 1–12: Architecture, UX, Security, Ops (LOCKED)
- ✅ Blueprint committed to GitHub
- ✅ GitHub push protection resolved
- ✅ Server mirror in `/var/www/15000`
- ✅ Progress tracking workflow established
- ✅ Implementation Stage 1: Go core skeleton (/api/v1/health) ready
- ✅ Implementation Stage 2: Backend wired via symlink (/var/api/16000 → /var/www/15000/api/16000)
- ✅ Go core skeleton committed and live from GitHub
- ✅ Implementation Stage 3: Go core running via systemd (auto-start, restart-on-failure)
- ✅ Stage 4: Nginx reverse proxy on port 15080 (/ → 15000, /api → 16000)
- ✅ Stage 5 — JSON storage initialization (design → skeleton)
- ✅ Stage 6 — DONE & LOCKED
- ✅ Stage 7 is now DONE & LOCKED
---

## ⏭️ Next Step
**Implementation Stage 1: Go Core Skeleton**
- Create Go service
- Health endpoint only (`/api/v1/health`)
- No business logic yet

---

## 🧱 Implementation Roadmap
- ⬜ Go core skeleton
- ⬜ Node support skeleton
- ⬜ Nginx routing config
- ⬜ JSON loader + lock manager
- ⬜ Domain resolver
- ⬜ Booking engine
- ⬜ Frontend integration
- ⬜ Pre-prod testing
- ⬜ Go-live

---

## 🕒 Last Updated
- Date: 2026-01-13
- By: admin_remote

## Stage 9 — Bookings + Availability (DONE ✅)

- Added guest-safe bookings read:
  - GET /api/v1/bookings?zone_id=...
  - Guest hides customer_phone/customer_email; admin header reveals
- Added availability endpoint:
  - GET /api/v1/availability?zone_id=...&from=YYYY-MM-DD&to=YYYY-MM-DD
- Added admin-only booking writes:
  - POST /api/v1/bookings
  - PUT /api/v1/bookings/{id}
  - POST /api/v1/bookings/{id}/cancel
- Enforced strict overlap rules for pending/confirmed bookings.
- Enforced chain validation: site → subsite → zone.

## Stage 10.3 — KPR Submit/Approve + Installments (Flat) (DONE ✅)

- Added KPR API:
  - POST /api/v1/kpr (admin, create draft from confirmed booking, 1 per booking)
  - GET /api/v1/kpr?booking_id=... (guest-safe; admin reveals full)
  - PUT /api/v1/kpr/{id} (admin; allowed in draft/submitted)
  - POST /api/v1/kpr/{id}/submit
  - POST /api/v1/kpr/{id}/approve (validates required fields)
  - POST /api/v1/kpr/{id}/reject
  - POST /api/v1/kpr/{id}/cancel
- Added Installment Plan API (flat formula):
  - POST /api/v1/installments/{kpr_id}/generate (admin; only when KPR approved)
  - GET /api/v1/installments?kpr_id=...
- Updated strict JSON loader to include:
  - kpr_applications.json, installment_plans.json, payments.json
- Added JSON templates for the new core files (no real data in Git).
