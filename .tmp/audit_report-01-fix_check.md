# Audit Report 01 Fix Check

Source basis: current repository state, migrations, frontend routes, and backend/API test coverage visible after the follow-up fixes.

## Issues From Report 01 That Were Clearly Fixed Later

1. `reports:admin` was missing from seed data.
   - How it was fixed: `migrations/015_reports_enablement.up.sql` now seeds `reports:admin` and assigns it to the administrator role, making `/api/v1/reports/recalculate` reachable on a fresh migrated instance.

2. Report branch-scope and pipeline semantics were inconsistent.
   - How it was fixed: the reports service again requires a non-empty branch for user-facing runs and exports, while `migrations/015_reports_enablement.up.sql` normalizes seeded `query_template` values to the dispatcher keys actually supported by the runtime.

3. `reports:export` was unavailable to operations staff.
   - How it was fixed: `migrations/014_seed.up.sql` maps `reports:export` to `operations_staff`, and the frontend reports client uses blob download handling for the export response.

4. Branch-scope default behavior was no longer permissive for unassigned non-admins.
   - How it was fixed: `backend/internal/middleware/branch_scope.go` now assigns the nil-branch sentinel UUID for unassigned non-admin users, which collapses to empty results instead of unrestricted access.

5. End-to-end branch-scope and object-authorization coverage was too thin.
   - How it was fixed: the integration suite now includes cross-branch HTTP tests for readers, circulation, program-rule subresources, unassigned-user sentinel behavior, and report endpoints, plus an additional cross-branch copy-parent authorization test for `POST /holdings/:id/copies`.

6. Domain-specific test gaps for holdings, circulation, and stocktake reduced confidence.
   - How it was fixed: `backend/tests/integration/domain_perms_test.go` and `backend/tests/integration/authz_test.go` now exercise those domains through real API handlers rather than relying only on static review.

## Sanitization Note

- This file only lists items from `audit_report-01.md` that are now directly supported by the repository state.
- It intentionally does not claim runtime success for DB-backed integration paths that still depend on a valid local `lms_test` database configuration.
