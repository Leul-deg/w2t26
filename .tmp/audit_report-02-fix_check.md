# Audit Report 02 Fix Check

Source basis: current repository state plus the backend/frontend changes present after the follow-up remediation work.

## Issues From Report 02 That Were Clearly Fixed Later

1. Circulation stopped being a stub and was connected in backend and frontend routing.
   - How it was fixed: `backend/cmd/server/main.go` registers the circulation handlers, and `frontend/src/App.tsx` now routes `/circulation` to a concrete `CirculationPage`.

2. Imports/exports permission mismatch was fixed to the seeded taxonomy.
   - How it was fixed: the code now uses the seeded permission names `imports:create`, `imports:preview`, `imports:commit`, and `exports:create`.

3. Server-side step-up enforcement was added for sensitive reveal.
   - How it was fixed: the reveal flow now requires a recent successful server-side step-up before encrypted reader fields can be disclosed.

4. Circulation cross-branch object authorization was hardened.
   - How it was fixed: circulation resolves `copy_id` through a branch-scoped copy lookup before checkout, return, or active-checkout access, and integration tests cover the 404 behavior for cross-branch requests.

5. Program enrollment-rule endpoints now parent-authorize by branch.
   - How it was fixed: the program-rule handlers load the parent program through the caller's branch scope before listing, adding, or removing rules.

6. Reporting pipeline inconsistencies were corrected.
   - How it was fixed: `migrations/015_reports_enablement.up.sql` seeds `reports:admin` and rewrites seeded report definitions so `query_template` stores the runtime dispatcher keys (`utilization`, `enrollment_mix`, `resource_yield`, `circulation`, `reader_activity`, `feedback_summary`) instead of incompatible raw SQL snippets.

7. Branch-scope default no longer degrades to unrestricted access for unassigned non-admin users.
   - How it was fixed: `backend/internal/middleware/branch_scope.go` now assigns the nil-branch sentinel UUID, which produces empty branch-filtered result sets rather than all-branch visibility.

8. General object-level authorization gaps were tightened further.
   - How it was fixed: in addition to the earlier circulation and program-rule hardening, `backend/internal/domain/holdings/service.go` now verifies the parent holding through branch scope before allowing `POST /holdings/:id/copies`, closing a remaining cross-branch parent-object path.

9. Frontend placeholder-module concerns were resolved for the flagged paths.
   - How it was fixed: the current frontend route table points `/holdings`, `/stocktake`, `/circulation`, `/reports`, and `/enrollments` at concrete pages rather than `PlaceholderPage`.

10. Audit-event consistency improved for the affected export and enrollment flows.
    - How it was fixed: audit logging now records branch/workstation context for enrollment status changes and export creation events, making those audit rows more consistent with the rest of the system.

11. Navigation mismatch for `/enrollments` was resolved.
    - How it was fixed: the sidebar nav item in `frontend/src/components/AppShell.tsx` now matches the concrete `/enrollments` route registered in `frontend/src/App.tsx`.

12. Vendored frontend dependencies are not tracked in version control.
    - How it was fixed: `frontend/node_modules/` is excluded by `.gitignore` and carries zero tracked files (`git ls-files frontend/node_modules` returns no results). The directory may be present on a developer's working copy after `npm install` but is never committed to the repository.

13. Missing API tests were addressed with direct handler-level integration coverage.
    - How it was fixed: the integration suite now includes concrete request/response checks for report run/export endpoints and the cross-branch holding-copy path, in addition to the previously added branch-scope and object-authorization HTTP tests.

## Sanitization Note

- This file only records items from `audit_report-02.md` that are now directly supported by the repository contents.
- DB-backed integration execution still depends on a valid local `lms_test` PostgreSQL configuration; where that environment is unavailable, runtime success remains a manual verification item rather than an asserted fact.
