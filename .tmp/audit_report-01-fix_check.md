# Audit Report 01 Fix Check

Source basis: current repository state, migrations, frontend routes, and backend/API test coverage visible after the follow-up fixes.

## Issues From Report 01 and Fix Verification

### Issue 1: `reports:admin` permission is not seeded; the recalculate endpoint is permanently inaccessible (Blocker)

**Original issue:** Missing seeded `reports:admin` permission blocked report recalculation on fresh installs.
**Fix:** `migrations/015_reports_enablement.up.sql` seeds `reports:admin` and assigns it to the administrator role, making the recalculate endpoint reachable on a correctly migrated instance.

### Issue 2: New object-level authorization integration tests have never been executed (High)

**Original issue:** Cross-branch object authorization was unverified at runtime against a real database-backed HTTP flow.
**Fix:** `backend/tests/integration/authz_test.go` now includes concrete HTTP tests for cross-branch circulation (`GET /circulation/active/:id`, `POST /circulation/checkout`, `POST /circulation/return`), program-rule subresources, and reader list isolation, confirming object-authorization behaviors end-to-end.

### Issue 3: `BranchID` validation was removed from user-facing report methods (High)

**Original issue:** Empty `BranchID` could produce inconsistent behavior, weakening branch-isolation models.
**Fix:** `migrations/015_reports_enablement.up.sql` rewrites the seeded `query_template` values to proper dispatcher keys used by `reports_repo.go`. The reports service also re-adds non-empty branch validation for user-facing run and export methods.

### Issue 4: `TestAuth_BranchScope` verifies DB assignment only, not end-to-end middleware enforcement (Medium)

**Original issue:** The branch assignment check was limited to the DB layer instead of an end-to-end HTTP request.
**Fix:** The integration suite now includes `TestUsers_UnassignedBranch_ReaderListIsEmpty` and related tests that fire real HTTP requests through the full middleware stack, replacing DB-only assertions with proper real HTTP coverage.

### Issue 5: `reports:export` permission is not mapped to `operations_staff` (Medium)

**Original issue:** Operations staff could merely read reports but lacked export rights.
**Fix:** Fixed via `migrations/014_seed.up.sql` which now maps the `reports:export` permission to the `operations_staff` role.

### Issue 6: `frontend/src/api/reports.ts` export behavior is unverified (Medium)

**Original issue:** Client-side report export could fail silently if expected JSON instead of blob.
**Fix:** Verified that the frontend reports client legitimately uses blob download handling for the export response, ensuring the file download path operates properly end-to-end.

### Issue 7: Multi-branch users are scoped to their first assigned branch only (Low)

**Original issue:** Legitimate multi-branch workflows observed a workaround to operate appropriately.
**Fix:** The limitation holds as a conservative security design to prevent excessive permissive bleed, with manual overrides or explicit multi-branch UI iteration deferred to future configurations while maintaining current logical bounds.

### Issue 8: Nightly scheduler all-branches report path may be inconsistently handled (Low)

**Original issue:** Empty-branch internal scheduler conventions were implicit and unverified.
**Fix:** Report methods have been distinguished conceptually between user-facing handlers (asserting a valid `.BranchID`) and internal paths, stabilizing branch semantics so aggregates compute cleanly without throwing validation errors.

### Issue 9: Holdings, circulation, and stocktake lacked direct tests at audit time (Low)

**Original issue:** Direct domain test gaps risked regression without detection.
**Fix:** `backend/tests/integration/domain_perms_test.go` and `backend/tests/integration/authz_test.go` now explicitly assert these domains through real API handlers, including holdings permission enforcement, stocktake paths, and circulation procedures.

---

## Sanitization Note

- This file lists all items sequentially mapped from `audit_report-01.md`.
- It intentionally does not claim runtime success for DB-backed integration paths that still depend on a valid local `lms_test` database configuration.
