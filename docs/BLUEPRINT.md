# Land Booking + KPR — Project Blueprint (LOCKED SPEC)

> Canonical architecture & rules for: land booking system with interactive polygon zones, JSON-first storage, role-based permissions, and ops playbook.
> This document is the single source of truth. Changes require explicit approval.

---

## 0) Core Principles

- Backend is the source of truth for permissions & state transitions.
- Storage is JSON-first for speed of development, DB-ready later.
- Reads are in-memory; writes are atomic + locked.
- Support features (tickets/chat) are non-core modules.
- Mobile-first for browsing and booking, but polygon drawing/editing is tablet/desktop only.
- Domain behavior is dynamic via JSON mapping; DNS just points to the public IP.

---

## 1) Infrastructure & Wiring (Step 1 — LOCKED)

### Server
- OS: Linux Lite
- Always use `sudo` for server operations
- Ownership default: `admin_remote:admin_remote`

### Runtime paths
- Backend runtime: `/var/api/16000`
- Frontend runtime: `/var/www/15000`

### Ports
- Frontend (Laravel): `15000`
- Core API (Go): `16000`
- Support service (Node, internal): `16001`

### Public assets
- All public/user files are stored in:
  - `/mnt/ss-10-235/IT/public-assets`
- Laravel `public/` must link to public-assets (symlink).

### Nginx
- Nginx config location:
  - `/etc/nginx/sites-available/multi-ports`

---

## 2) Data Model (JSON Storage) (Step 2 — LOCKED)

### Core JSON (required for app to run)
Location: `storage-json/`
- `users.json`
- `sites.json`
- `subsites.json`
- `zones.json`
- `bookings.json`
- `domains.json`

### Non-core JSON (optional support module)
Location: `storage-json/support/`
- `tickets.json` (ticket-based chat)

### Dependency rule
- Core must run even if support JSON is empty or missing.
- Support can reference core IDs (site_id, zone_id, booking_id) but core does not depend on support.

---

## 3) Roles, Auth-State & Permissions (LOCKED)

### Roles
- `ADMIN`
- `SALES_MANAGER`
- `CUSTOMER_SALES`
- `GUEST`

### Auth-state rule (critical)
- If no valid session/token → treat user as `GUEST` regardless of account role.
- Roles apply only when authenticated.

### Permissions summary
- Public browsing: allowed for all (including GUEST).
- ADMIN-only:
  - Create/edit/archive: sites, subsites, zones
  - Upload/edit subsite layout
  - Edit zone polygon geometry
  - Edit domains mapping
- SALES_MANAGER:
  - Approve/reject booking
  - Mark SOLD (only from BOOKED)
  - Set NOT_AVAILABLE (only from AVAILABLE; see status rules)
- CUSTOMER_SALES:
  - Request booking
  - Cancel own booking
- Personal document upload:
  - Authenticated users (own profile) + ADMIN

---

## 4) Domain Wiring via JSON (Step 3.5 — LOCKED)

### DNS
- Domain (e.g., `www.matahariland.co.id`) points to public IP (static).
- DNS is not dynamic; application behavior is dynamic.

### Domain mapping storage
- `storage-json/domains.json` maps `domain` → `site_id` and optional theme.

### API
- `GET /api/v1/domains/resolve?host=<domain>`
- Backend returns `site_id`, theme, and status.

### Nginx
- Generic routing only (no business logic).
- Preserve Host header to backend/frontend.

---

## 5) Frontend App Flow (Step 4 — LOCKED)

### Public Space (no login required)
- Interactive:
  - site list + details + images
  - subsite layout image
  - polygon zones (clickable)
  - KPR calculator
- Navbar top-right: Login / Signup
- If user tries to Book:
  - show Login/Signup prompt

### Login → Dashboard
- After login redirect to Dashboard
- Dashboard shows:
  - property cart entries
  - booking status list (pagination)
- If empty:
  - CTA “+ Add Property” (go to browsing)

### Mobile rule
- Mobile: view-only for polygons
- Tablet/Desktop: can draw/edit polygons (admin only)

### Helpdesk/Chat UX
- Ticket-based chat UI (messages thread).
- Used for follow-up and customer service.

---

## 6) Polygon Zones (Core UI Requirement) (LOCKED)

### Coordinate mode
- `NORMALIZED` coordinates (0..1), relative to layout image size.

### Subsite layout fields (in subsites.json)
- `layout.image_url`
- `layout.image_width`
- `layout.image_height`
- `layout.coordinate_mode = NORMALIZED`
- `layout.version`

### Zone polygon fields (in zones.json)
- `geometry.type = POLYGON`
- `geometry.points = [{x,y}, ...]`
- optional `ui.label_position`, `ui.z_index`, etc.

### Editable text sizing
- `ui.text.font_size` exists at:
  - subsite level
  - zone level

### Status colors
- `ui.status_colors` per zone, adjustable:
  - `AVAILABLE`, `BOOKED`, `SOLD`, `NOT_AVAILABLE`
  - each has `fill`, `stroke`, `opacity`

---

## 7) Zone Status State Machine (LOCKED)

### Zone statuses
- `AVAILABLE`
- `BOOKED`
- `SOLD`
- `NOT_AVAILABLE`

### Allowed transitions
- `AVAILABLE → BOOKED` (on booking approval)
- `BOOKED → SOLD` (Sales Manager/Admin; only from BOOKED)
- `BOOKED → AVAILABLE` (approved booking cancelled)
- `AVAILABLE → NOT_AVAILABLE` (Sales Manager/Admin; requires no approved booking)
- `NOT_AVAILABLE → AVAILABLE` (Sales Manager/Admin)

### Forbidden transitions
- `AVAILABLE → SOLD` (not allowed)
- `NOT_AVAILABLE → BOOKED` (not allowed)
- `NOT_AVAILABLE → SOLD` (not allowed)

### NOT_AVAILABLE metadata (recommended fields)
- `unavailable_reason`
- `unavailable_since`
- `unavailable_by_user_id`

---

## 8) Booking Workflow (LOCKED)

### Booking statuses
- `REQUESTED`
- `APPROVED`
- `REJECTED`
- `CANCELLED`

### Policy: multiple requests allowed
- Multiple users can REQUEST booking for the same zone while it is AVAILABLE.
- Zone remains AVAILABLE until an approval occurs.

### Roles
- Request booking: CUSTOMER_SALES / SALES_MANAGER / ADMIN
- Approve / Reject: SALES_MANAGER / ADMIN
- Cancel:
  - requester or ADMIN

### Booking → Zone coupling (atomic)
- REQUESTED: zone stays AVAILABLE
- APPROVED: zone becomes BOOKED, set `booked_by_user_id`
- CANCELLED (if approved): zone returns AVAILABLE, clear `booked_by_user_id`
- SOLD: set only from BOOKED by SALES_MANAGER/ADMIN

### Profile completion rule (enforced)
Booking request requires:
- profile `full_name`
- profile `phone`
- at least one personal document (photo/id/sim)
If incomplete: return `PROFILE_INCOMPLETE`.

### Ticket/chat push
After REQUESTED booking, user is encouraged/required to create a ticket for follow-up.

---

## 9) Support Module: Tickets + Chat (LOCKED)

### Storage
- `storage-json/support/tickets.json`

### Chat mode
- Ticket-based chat: each ticket includes `messages[]`.
- UI renders ticket thread like chat.

### Ticket linkage (optional)
- `booking_id`, `zone_id`, `site_id`

---

## 10) Logging (LOCKED)

### CSV log rotation
- Log file name format:
  - `logs/app-DDMMYYYY.csv`
  - Example: `logs/app-12012026.csv`
- Header written once per file.

### Columns (locked)
- `timestamp,level,service,action,user_id,entity_type,entity_id,message`

### Non-blocking rule
- Logging failures do not block core operations (best-effort).

---

## 11) Backend Data Flow & JSON Safety (LOCKED)

### Read
- Load JSON into memory at startup.
- Serve reads from memory.

### Write (atomic)
- Acquire file lock
- Validate permissions + payload
- Apply changes to in-memory clone
- Update audit fields
- Write `*.tmp`, fsync, atomic rename
- Release lock
- Update in-memory indexes
- Append CSV log

### Booking + zone atomicity
- Lock `bookings.json` and `zones.json` during approve/cancel/sold transitions.

---

## 12) Service Boundaries (Step 6 — LOCKED)

### Go (Core, authoritative) — :16000
- JSON read/write + locking
- domain resolve
- auth + permissions
- booking engine + state machine
- CSV logging
- health endpoint

### Node (Support/Integrations, non-authoritative) — :16001 (internal only)
- uploads to public-assets
- email notifications via domain email hosting SMTP
- tickets.json (support module)
- internal endpoints for Go to call

### Hard rule
- Node does NOT write core JSON.

---

## 13) API Error Model (Step 7 — LOCKED)

### Success envelope
```json
{ "ok": true, "data": {} }
