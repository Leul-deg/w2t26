# Audit Report 01 Fix Check

Source basis: current repository state, migrations, frontend routes, and backend/API test coverage visible after the follow-up fixes.

## Issues From Report 01 That Were Clearly Fixed Later

### Issue 1 — Missing `reports:admin` seed blocked report recalculation (Blocker)

**Original issue:** `reports:admin` was absent from `migrations/014_seed.up.sql`, so the `/api/v1/reports/recalculate` endpoint was permanently dead code on any fresh install. Administrators had no way to trigger on-demand aggregate recalculation because the permission could not be assigned through the normal role-permission seed path.

**Fix:** `migrations/015_reports_enablement.up.sql` seeds `reports:admin` and assigns it to the administrator role, making the recalculate endpoint reachable on a correctly migrated instance.

---

### Issue 2 — Report branch-scope and pipeline semantics were inconsistent (High)

**Original issue:** `BranchID` validation had been removed from user-facing report service methods. The seeded `query_template` values in `014_seed.up.sql` stored raw SQL snippets instead of the dispatcher keys the runtime switch statement expected, causing every seeded report to fail at runtime. Empty `BranchID` could also produce inconsistent behavior if middleware ever failed to populate scope.

**Fix:** `migrations/015_reports_enablement.up.sql` rewrites the seeded `query_template` values to the dispatcher keys actually used by `reports_repo.go` (`utilization`, `enrollment_mix`, `resource_yield`, `circulation`, `reader_activity`, `feedback_summary`). The reports service re-adds non-empty branch validation for user-facing run and export methods.

---

### Issue 3 — `reports:export` unavailable to operations staff (Medium)

**Original issue:** `reports:export` was not mapped to `operations_staff` in `014_seed.up.sql`. Operations staff could read report definitions but could not export them, making the reporting workflow incomplete for the most common operator role.

**Fix:** `migrations/014_seed.up.sql` maps `reports:export` to `operations_staff`. The frontend reports client uses blob download handling for the export response so the file download path is end-to-end consistent.

---

### Issue 4 — Branch-scope test verified only DB assignment, not HTTP enforcement (Medium)

**Original issue:** `TestAuth_BranchScope` checked that branch assignment rows existed in the database but never issued an HTTP request through the branch-scope middleware. The middleware enforcement path — specifically whether an unassigned non-admin user received restricted or unrestricted scope — was untested end-to-end.

**Fix:** The integration suite now includes `TestUsers_UnassignedBranch_ReaderListIsEmpty` and related cross-branch HTTP tests that fire real requests through the full middleware stack and assert on the actual response status and body, replacing the DB-only assertion with real HTTP coverage.

---

### Issue 5 — End-to-end branch-scope and object-authorization coverage was too thin (High)

**Original issue:** Cross-branch object authorization (e.g. an EAST-branch user attempting to read or act on a MAIN-branch copy) was asserted only in code; no HTTP integration test had been executed against a real database. The branch-scope isolation claim was unverified at runtime.

**Fix:** `backend/tests/integration/authz_test.go` now includes concrete HTTP tests for cross-branch circulation (`GET /circulation/active/:id`, `POST /circulation/checkout`, `POST /circulation/return`), program-rule subresources (`GET/POST/DELETE /programs/:id/rules`), reader list isolation for unassigned users, and user management cross-branch guards — all asserting on real HTTP responses with 404 / 200 expectations. A cross-branch copy-parent authorization test for `POST /holdings/:id/copies` was also added in `domain_perms_test.go`.

---

### Issue 6 — Holdings, circulation, and stocktake lacked direct tests (Low)

**Original issue:** At audit time the holdings, circulation, and stocktake domains had no direct handler or permission tests. Domain regressions in those areas could go undetected because coverage stopped at the auth and readers integration surface.

**Fix:** `backend/tests/integration/domain_perms_test.go` and `backend/tests/integration/authz_test.go` now exercise those domains through real API handlers. Holdings tests cover list/create permission enforcement and the cross-branch copy-parent path. Stocktake tests cover read/write permission enforcement. Circulation tests cover the cross-branch IDOR scenarios and the admin all-branches list path. All assertions run through real HTTP handlers rather than relying solely on static review.

---

## Sanitization Note

- This file only lists items from `audit_report-01.md` that are now directly supported by the repository state.
- It intentionally does not claim runtime success for DB-backed integration paths that still depend on a valid local `lms_test` database configuration.
