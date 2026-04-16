# Library Operations & Enrollment Management Suite (LMS)

Offline-first library management system with role-based access control, reader
profile management, holdings inventory, program enrollment, content moderation,
bulk import/export, configurable analytics, and compliance-oriented audit logging.

---

## Quick Start (Docker — recommended)

**Requirements:** Docker with the Compose plugin (`docker compose version` ≥ v2).
No Go, Node.js, or PostgreSQL installation required.

```bash
# 1 — Start all services (postgres + backend + frontend)
docker compose up -d

# 2 — Verify backend is ready
curl http://localhost:$(docker compose port backend 8080 | cut -d: -f2)/api/v1/health
# {"status":"ok","version":"0.1.0"}

# 3 — Open the frontend
#   http://localhost:<port shown by: docker compose port frontend 3000>
```

Docker Compose handles: PostgreSQL setup, encryption key generation, schema
migrations, and dependency installation. All services start in the correct order.

> **Port note:** ports are assigned dynamically to avoid conflicts with local
> services. Run `docker compose port backend 8080` and
> `docker compose port frontend 3000` to see the actual host ports.

### Run the full test suite (Docker)

```bash
./run_tests.sh
```

`run_tests.sh` resets the test database, runs backend unit tests, frontend
type-check + tests, and all API integration tests — entirely inside containers.

---

## Quick Start (local toolchain)

Requires Go 1.24, Node.js 18+, and a running PostgreSQL instance.

```bash
# 1 — Postgres setup (run once as superuser)
psql -U postgres -c "CREATE USER lms_user WITH PASSWORD 'changeme';"
psql -U postgres -c "CREATE DATABASE lms OWNER lms_user;"
psql -U postgres -c "CREATE DATABASE lms_test OWNER lms_user;"

# 2 — Backend config
cd backend && cp .env.example .env   # edit DATABASE_URL and CRYPTO_KEY_FILE

# 3 — Key file
mkdir -p /etc/lms && openssl rand -out /etc/lms/lms.key 32 && chmod 600 /etc/lms/lms.key

# 4 — Run migrations + start backend
go run ./cmd/migrate up && go run ./cmd/server

# 5 — Frontend (separate terminal)
cd frontend && npm install && npm run dev

# Login at http://localhost:3000  →  admin / Admin1234!
```

---

## Prerequisites

| Requirement | Minimum version |
|---|---|
| Go | 1.24 |
| Node.js | 18 |
| npm | 9 |
| PostgreSQL | 14 |
| `openssl` | any recent version |

> PostgreSQL must be running before the backend starts. There is no embedded or
> in-memory fallback.

---

## One-Time Setup

### 1. PostgreSQL database and user

```sql
-- Run as a PostgreSQL superuser (e.g. sudo -u postgres psql)
CREATE USER lms_user WITH PASSWORD 'changeme';
CREATE DATABASE lms       OWNER lms_user;
CREATE DATABASE lms_test  OWNER lms_user;
ALTER ROLE lms_user SET search_path = lms, public;
```

### 2. Encryption key file

```bash
mkdir -p /etc/lms
openssl rand -out /etc/lms/lms.key 32
chmod 600 /etc/lms/lms.key
```

Store this file outside the repository. Do **not** commit it.
Losing this key means losing access to encrypted reader fields.

### 3. Backend configuration

```bash
cd backend
cp .env.example .env
```

Edit `backend/.env`:

| Variable | Required | Default | Notes |
|---|---|---|---|
| `DATABASE_URL` | Yes | — | Full PostgreSQL DSN |
| `CRYPTO_KEY_FILE` | Yes | — | Path to the 32-byte key file |
| `SERVER_PORT` | No | `8080` | HTTP listen port |
| `SESSION_INACTIVITY_SECONDS` | No | `1800` | 30-minute default |
| `MIGRATIONS_PATH` | No | `../../migrations` | Relative or absolute path |

### 4. Backend dependencies

```bash
cd backend
go mod download
```

### 5. Frontend configuration and dependencies

```bash
cd frontend
cp .env.example .env   # default works for local dev (Vite proxies /api to :8080)
npm install
```

---

## Database Setup and Migrations

Run from the `backend/` directory:

```bash
# Apply all 17 migrations (schema + seed data)
go run ./cmd/migrate up

# Check current version
go run ./cmd/migrate version

# Roll back one migration
go run ./cmd/migrate down 1
```

Migration 014 (`014_seed.up.sql`) seeds:
- Roles: `administrator`, `operations_staff`, `content_moderator`
- Default admin account: `admin` / `Admin1234!`
- Two seed branches: `MAIN` and `EAST`
- Permissions mapped to roles

Migration 015 (`015_reports_enablement.up.sql`) adds:
- `reports:admin` permission (grants administrator the ability to trigger on-demand recalculation)
- Six report definitions with dispatch keys and metric aliases (replaces the two raw-SQL placeholders from 014)

Migration 017 (`017_feedback_appeals_submit.up.sql`) adds:
- `feedback:submit` and `appeals:submit` permissions
- Role mappings so staff submissions are permission-gated at the API layer

---

## Running the Application

### Backend

```bash
cd backend
go run ./cmd/server
# Starts on http://localhost:8080
```

Health checks:

```bash
curl http://localhost:8080/api/v1/health
# {"status":"ok","version":"0.1.0"}

curl http://localhost:8080/api/v1/ready
# 200 = DB reachable, 503 = DB not reachable
```

Login verification:

```bash
curl -s -c /tmp/lms.txt -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"Admin1234!"}' | jq .

curl -s -b /tmp/lms.txt http://localhost:8080/api/v1/auth/me | jq .
```

### Frontend

```bash
cd frontend
npm run dev
# Available at http://localhost:3000
```

Sign in with `admin` / `Admin1234!`.

---

## Seed Demo Credentials

All three accounts are seeded automatically by migrations and are ready to use
immediately after `docker compose up` or a local `go run ./cmd/migrate up`.

| Username | Password | Role | Branch | Notes |
|---|---|---|---|---|
| `admin` | `Admin1234!` | Administrator | MAIN + EAST | All permissions across all branches |
| `ops1` | `Staff1234!` | operations_staff | MAIN | Reader/holdings/circulation/import/export |
| `mod1` | `Moderate1234!` | content_moderator | MAIN | Content review, feedback, appeals |

**Change these passwords (or drop migration 018) in any non-development environment.**

---

## Role Descriptions

- `administrator`: all permissions across all domains.
- `operations_staff`: `readers:read`, `readers:write`, `readers:reveal_sensitive`, `holdings:read`, `holdings:write`, `copies:read`, `copies:write`, `circulation:read`, `circulation:write`, `stocktake:read`, `stocktake:write`, `programs:read`, `programs:write`, `enrollments:read`, `enrollments:write`, `feedback:read`, `feedback:submit`, `appeals:read`, `appeals:submit`, `content:read`, `imports:create`, `imports:preview`, `imports:commit`, `exports:create`, `reports:read`, `reports:export`.
- `content_moderator`: `content:read`, `content:submit`, `content:moderate`, `content:publish`, `feedback:read`, `feedback:submit`, `feedback:moderate`, `appeals:read`, `appeals:submit`, `appeals:decide`, `readers:read`, `reports:read`.

Full permission mappings are defined in `migrations/014_seed.up.sql` and `migrations/015_reports_enablement.up.sql`.

---

## Test Commands

### All tests — one command (Docker, recommended)

```bash
./run_tests.sh
```

Resets the `lms_test` database, then runs in sequence inside Docker containers:
1. Backend unit tests (`go test ./internal/...`)
2. Frontend lint + type-check (`npm run lint`)
3. Frontend component tests (`npm run test`)
4. Backend API integration tests (`go test ./API_TESTS/... -v`)

No local Go, Node.js, or PostgreSQL installation required.

### Unit tests (no database required)

```bash
cd backend
go test ./internal/...
```

Covers: config, all domain services (appeals, content, enrollment, exports, feedback, imports, moderation, reports), and handler permission enforcement.

### API integration tests (require `lms_test` database)

```bash
cd backend
DATABASE_TEST_URL=postgres://lms_user:changeme@localhost:5432/lms_test?sslmode=disable \
  go test ./API_TESTS/... -v
```

All API tests use `httptest.NewRequest` routed through a real Echo app backed by
a live PostgreSQL connection — no mocking at any layer. Tests cover 103 endpoints
across auth, RBAC, branch-scope isolation, full CRUD lifecycles, and conflict scenarios.

**Covered paths:**
- Successful login → 200 + session cookie
- Wrong password → 422 (generic message, no user enumeration)
- CAPTCHA escalation after 3 failures → 428
- Account lockout after 5 failures → 423 (15-minute lockout)
- Unauthenticated protected endpoint → 401
- RBAC forbidden → 403
- Branch scope assignment verified at the DB level
- **Cross-branch reader access → 404** (not 403, to prevent enumeration)
- **User without `readers:read` → 403**
- Masked sensitive fields → `••••••` for all sensitive fields
- Failed `POST /api/v1/auth/stepup` with wrong password → 401
- `POST /api/v1/readers/:id/reveal` without a recent successful step-up → 403
- Schema invariants: uniqueness constraints, seed data, audit append-only

### All tests

```bash
cd backend
DATABASE_TEST_URL=postgres://lms_user:changeme@localhost:5432/lms_test?sslmode=disable \
  go test ./... -v 2>&1 | grep -E "^(ok|FAIL|---)"
```

### Frontend tests

```bash
cd frontend
npm run test        # run once
npm run test:watch  # watch mode
```

**Covered paths:**
- Session restoration (authenticated/unauthenticated from `/auth/me`)
- Login form: invalid credentials, CAPTCHA handling, lockout message, success redirect
- Protected route: redirect to `/login` when unauthenticated
- Protected route: redirect to `/unauthorized` when role missing
- Protected route: renders children when authorized
- Role-based navigation rendering

### TypeScript compile check

```bash
cd frontend
npx tsc --noEmit
```

---

## Security-Sensitive Modules (Reviewer Locations)

| Control | File |
|---|---|
| Password hashing (bcrypt cost 12) | `backend/internal/domain/users/service.go` |
| Session token hashing (SHA-256, httpOnly cookie) | `backend/internal/domain/users/service.go`, `backend/internal/store/postgres/session_repo.go` |
| CAPTCHA after 3 failures | `backend/internal/domain/users/service.go: issueCaptcha()` |
| Account lockout after 5 failures | `backend/internal/domain/users/service.go: Login()` |
| RBAC middleware | `backend/internal/middleware/rbac.go` |
| Branch scope middleware | `backend/internal/middleware/branch_scope.go` |
| Sensitive field masking | `backend/internal/domain/readers/handler.go: maskedResponse()` |
| Step-up re-auth for reveal | `backend/internal/domain/readers/handler.go: RevealSensitive()` |
| AES-256 key infrastructure | `backend/internal/crypto/` |
| Audit logger (append-only) | `backend/internal/audit/logger.go` |
| Export audit (pre-creation before file) | `backend/internal/domain/exports/service.go`, `backend/internal/domain/reports/service.go` |
| Permission check pattern | `user.HasPermission()` called at the top of every handler before service |

---

## Implementation Status

### Implemented and verified (integration tests pass)

| Area | Notes |
|---|---|
| Auth: login, logout, session, CAPTCHA, lockout | Full integration tests |
| Branch scope enforcement | DB-level and middleware |
| RBAC permission enforcement | Middleware + handler-level guards |
| Reader CRUD with masked/reveal pattern | Step-up auth required for reveal |
| Holdings and copy-level inventory | Full service + handler |
| Stocktake sessions | Full service + handler |
| Bulk import with validation, preview, rollback | CSV and XLSX uploads/templates supported |
| Audited export generation | CSV/XLSX export job created before file generation |
| Program management | Full service + handler; UI now supports status, prerequisites, and rules |
| Enrollment with concurrency safety | SELECT FOR UPDATE; service tests |
| Governed content publishing workflow | State machine: draft→review→approved/rejected→published→archived |
| Moderation queue (assign + decide) | Handler + service tests |
| Feedback with tag resolution | Service tests |
| Appeals and arbitration | State machine: submitted→under_review→resolved/dismissed |
| Configurable analytics reports | 6 live query types; nightly pre-aggregation |
| Report CSV export with audit log | Uses export_jobs table |
| Audit log for all moderation/admin decisions | append-only; event types in model/audit.go |
| Frontend: login, session, navigation shell | |
| Frontend: all domain pages (readers through reports) | Wired in App.tsx |

### Implemented but not runtime-verified (static / unit tests only)

| Area | Gap |
|---|---|
| AES-256 field-level encryption at rest | Key infrastructure ready; encrypt/decrypt path exists; no test with a real encrypted value in the DB |
| Holdings/copies domain | Handler + service + frontend written; no integration tests |
| Circulation domain | Integration tests cover cross-branch checkout/return/active-checkout visibility and admin list behavior; full browser flows are not verified |
| Stocktake domain | Handler + service + frontend written; no integration tests |
| Programs/enrollment frontend | TypeScript compiles; not browser-tested |
| Content/moderation/feedback/appeals frontend | TypeScript compiles; not browser-tested |
| Reports frontend | TypeScript compiles; not browser-tested |
| Nightly aggregation scheduler | Goroutine wires correctly; not tested under load or clock |

### Deferred / incomplete

| Area | Status |
|---|---|
| User-management browser workflows | `/api/v1/users/*` is implemented and routed; UI/API flows have not been browser-tested end-to-end |
| `POST /api/v1/readers/:id/reveal` with real decryption | Returns decrypted data only when `CRYPTO_KEY_FILE` points to the key used during encryption; existing test data has no encrypted values |

---

## Project Layout

```
.
├── README.md
├── migrations/                        # SQL migrations 001–017 (applied in order)
│   ├── 001_init.up.sql                # lms schema, uuid extension
│   ├── 002_auth_sessions.up.sql
│   ├── 003_branches.up.sql
│   ├── 004_readers.up.sql
│   ├── 005_holdings_copies.up.sql
│   ├── 006_circulation.up.sql
│   ├── 007_stocktake.up.sql
│   ├── 008_programs_enrollment.up.sql # venue_type=room_type, enrollment_channel=channel
│   ├── 009_content_moderation.up.sql
│   ├── 010_feedback_appeals.up.sql
│   ├── 011_imports_exports.up.sql
│   ├── 012_audit.up.sql
│   ├── 013_reports.up.sql             # report_definitions, report_aggregates tables
│   ├── 014_seed.up.sql                # roles, permissions, admin user, branches
│   ├── 015_reports_enablement.up.sql  # reports:admin permission + 6 report definitions
│   ├── 016_session_stepup.up.sql      # stepup_at column for server-side step-up enforcement
│   └── 017_feedback_appeals_submit.up.sql # feedback:submit and appeals:submit permissions
│
├── backend/
│   ├── cmd/
│   │   ├── server/main.go             # wires all services and routes
│   │   └── migrate/main.go            # standalone migration CLI
│   └── internal/
│       ├── apierr/                    # Echo HTTP error handler
│       ├── apperr/                    # Typed errors: NotFound, Forbidden, Conflict, Validation
│       ├── audit/                     # Append-only audit logger
│       ├── config/                    # Config loading + validation
│       ├── crypto/                    # AES-256 key loading + encrypt/decrypt
│       ├── ctxutil/                   # Context helpers (GetUser, GetBranchID, etc.)
│       ├── db/                        # pgx pool wrapper
│       ├── domain/
│       │   ├── appeals/               # Appeals + arbitration service, handler, tests
│       │   ├── audit/                 # Audit query handler
│       │   ├── circulation/           # Checkout, return, history (SELECT FOR UPDATE)
│       │   ├── content/               # Governed content lifecycle, tests
│       │   ├── copies/                # Copy-level inventory
│       │   ├── enrollment/            # Enrollment with concurrency safety, tests
│       │   ├── exports/               # Audited CSV export, handler tests
│       │   ├── feedback/              # Feedback + tags, tests
│       │   ├── holdings/              # Holdings + copy management
│       │   ├── imports/               # Bulk import with rollback, tests
│       │   ├── moderation/            # Moderation queue, tests
│       │   ├── programs/              # Program management
│       │   ├── readers/               # Reader CRUD + masking + reveal
│       │   ├── reports/               # Configurable analytics + export, tests
│       │   ├── stocktake/             # Stocktake sessions
│       │   └── users/                 # Auth (login/logout/CAPTCHA/lockout) + user management
│       ├── health/                    # /health, /ready
│       ├── middleware/                # session, RBAC, branch scope, request ID
│       ├── model/                     # Domain types + audit constants
│       ├── scheduler/                 # Nightly pre-aggregation goroutine
│       └── store/postgres/            # All repository implementations
│
└── frontend/
    └── src/
        ├── api/                       # fetch clients: readers, content, feedback, etc.
        ├── auth/AuthContext.tsx        # Session state, login/logout
        ├── components/                # AppShell, ProtectedRoute
        └── pages/
            ├── readers/               # Reader list, detail, form
            ├── programs/              # Program list, detail, form
            ├── content/               # Content list, form (lifecycle actions)
            ├── moderation/            # Moderation queue, content review
            ├── feedback/              # Feedback list with inline moderation
            ├── appeals/               # Appeals list, detail, arbitration
            └── reports/               # Reports page
```

---

## KPI / Hospitality Alias Mapping

The prompt specified hospitality-origin metrics. These have been mapped to LMS-native equivalents. Both the original term and the canonical name are supported in the `metric_aliases` JSONB column of `report_definitions`.

| Hospitality term | LMS canonical name | Data source |
|---|---|---|
| `occupancy_rate` | `slot_utilization_rate` | `copies.status_code` (checked_out / total) |
| `revpar` | `resource_yield_per_available_slot` | `enrollments / programs.capacity` |
| `revenue_mix` | `enrollment_mix_by_category` | `enrollments` grouped by `programs.category` |
| `room_type` | `venue_type` | `programs.venue_type` column |
| `channel` | `enrollment_channel` | `programs.enrollment_channel` + `enrollments.enrollment_channel` |

Report definitions are seeded via migration 013 and are queryable at `GET /api/v1/reports/definitions`.

---

## Known Limitations

1. **User management is admin-only via the UI.** `/api/v1/users/*` is fully implemented; creating staff accounts and assigning roles/branches is available to users with `users:write` or `users:admin` permissions.
2. **Circulation SELECT FOR UPDATE scope.** Checkout/return uses row-level locking on the copy row to prevent concurrent double-checkout. The lock is copy-scoped (not reader-scoped), so one reader checking out two different copies concurrently is safe.
3. **Field-level encryption not exercised with live data.** The AES-256 encrypt/decrypt code is present and tested in `crypto/` package; however, no seed reader has encrypted fields, so `RevealSensitive` will decrypt blank/unencrypted bytes in development.
4. **Nightly scheduler fires once per day at midnight.** On first startup the next run is always tomorrow's midnight. There is no backfill for missed days; use `POST /api/v1/reports/recalculate` to compute historical ranges.
5. **Report definitions are populated by migrations 014 and 015.** Migration 013 creates the `report_definitions` schema only. Migration 014 inserts two placeholder definitions; migration 015 normalizes them to dispatch-key `query_template` values and adds four more (six total). Custom definitions can be added via direct DB insert; there is no admin UI.
6. **No HTTPS configuration.** The server listens on plain HTTP; TLS termination is expected at a reverse proxy.
7. **No rate limiting beyond lockout.** Login lockout is implemented; general API rate limiting is not.
8. **Multi-branch users are scoped to their first assigned branch.** `branch_scope` middleware calls `GetBranches(userID)` and takes `branches[0]`. A user assigned to both MAIN and EAST always operates against MAIN. This is conservative (not a security gap) but means multi-branch staff workflows require separate user accounts. A `?branch_id=` override is not yet implemented.

---

## Verification Boundaries

### Runtime-verified (integration tests against a real PostgreSQL database)

- Login success, failure, CAPTCHA escalation, lockout, session expiry
- Authorization: 401 for no session, 403 for wrong role, 404 for cross-branch access
- Object-level auth: reader from branch B returns 404 to a user scoped to branch A
- Masked fields: sensitive fields never return plaintext values
- Failed `POST /api/v1/auth/stepup` returns 401 for wrong password
- `POST /api/v1/readers/:id/reveal` returns 403 without a recent successful step-up
- Schema constraints: uniqueness, FK integrity, audit append-only
- Enrollment concurrency: SELECT FOR UPDATE prevents double-booking
- Import rollback: committed status blocked when errors present

### Static / unit-tested (stub repos, no real DB)

- All domain service state machines (content, appeals, enrollment, moderation)
- Permission checks in every handler (nil service pattern)
- Report filtering and aggregate recalculation logic
- Export audit record creation and finalisation
- Import validation rules (missing fields, duplicate detection, invalid enums)

### Not verified / manual testing required

- Frontend browser behavior (TypeScript compiles; not E2E tested)
- AES-256 encryption round-trip with real reader data
- Nightly scheduler timing (goroutine is wired; not clock-tested)
- Holdings and stocktake service behavior under load
- Multi-branch concurrent enrollment (service tests are single-process stubs)

---

## Troubleshooting

**Backend fails: "database ping failed"**
- `pg_isready -h localhost -p 5432` — confirm Postgres is running
- `psql -U lms_user -d lms -c '\conninfo'` — confirm credentials

**Backend fails: "configuration errors"**
- Both required env vars (`DATABASE_URL`, `CRYPTO_KEY_FILE`) must be set.
- `CRYPTO_KEY_FILE` must be a readable path. The file does not need to contain a valid key for basic operation (auth + reader CRUD without encryption).

**Frontend: "Could not reach the server"**
- Confirm backend is on port 8080. Vite proxies `/api/*` to `http://localhost:8080`.

**`go mod download` or `npm install` fails**
- Both require internet access. Run once while online; subsequent builds work offline.

**Integration tests skip without output**
- Set `DATABASE_TEST_URL`. Tests call `t.Skip()` when it is absent.
