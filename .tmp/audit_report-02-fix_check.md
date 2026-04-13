# Audit Report 02 Fix Check

Source basis: current repository state, migrations, frontend routes, and backend/API test coverage visible after the follow-up fixes addressing the second audit cycle.

## Issues From Report 02 That Were Clearly Fixed Later

### Issue 1 — Reporting pipeline inconsistencies and missing seeds (Blocker)

**Original issue:** Seeded `query_template` values contained raw SQL snippets that the backend dispatcher could not process, and the `reports:admin` permission was missing. This rendered the reporting module non-functional on fresh installs.

**Fix:** `migrations/015_reports_enablement.up.sql` seeds the `reports:admin` permission and rewrites all report definitions to use the specific dispatcher keys (`utilization`, `enrollment_mix`, `resource_yield`, `circulation`, `reader_activity`, `feedback_summary`) expected by the `RunLiveQuery` logic in the repository layer.

---

### Issue 2 — Branch-scope default allowed unrestricted access (High)

**Original issue:** If a non-admin user had no branch assignment, the middleware defaulted to an empty string for the branch scope, which many repository queries interpreted as "unrestricted" or "all branches," violating the principle of least privilege.

**Fix:** `backend/internal/middleware/branch_scope.go` now assigns a sentinel nil-branch UUID (`00000000-0000-0000-0000-000000000000`) for unassigned users. All branch-filtered queries against this sentinel return empty results, ensuring a "fail-closed" security posture.

---

### Issue 3 — Object-level authorization gaps in Circulation and Holdings (High)

**Original issue:** Users could potentially interact with resources (like copies or program rules) belonging to other branches because handlers weren't verifying the parent object's branch ownership before processing requests.

**Fix:** Handlers for Circulation, Program Rules, and Holdings now perform branch-scoped lookups of the parent resource. Specifically, `AddCopy` in the holdings service now verifies the parent holding's branch before allowing a new copy to be created, and circulation actions now resolve `copy_id` through a branch-restricted filter.

---

### Issue 4 — Frontend placeholder modules (Medium)

**Original issue:** Critical business routes like `/holdings`, `/stocktake`, and `/circulation` were mapped to a generic `PlaceholderPage` component, meaning the frontend was not yet a functional delivery for those modules.

**Fix:** `frontend/src/App.tsx` has been updated to route all primary domain paths to concrete, implemented page components: `HoldingsListPage`, `StocktakePage`, `CirculationPage`, `ReportsPage`, and `EnrollmentsPage`.

---

### Issue 5 — Audit event metadata inconsistency (Medium)

**Original issue:** Enrollment and Export audit logs were missing the `BranchID` and `WorkstationID` context that was standard across the rest of the system, creating gaps in the forensic audit trail.

**Fix:** The enrollment and export services were updated to capture and pass `req.BranchID` and `req.WorkstationID` to the audit logger. All high-value mutations in these domains now produce consistent, context-rich audit events.

---

### Issue 6 — Navigation mismatch and tracking errors (Low)

**Original issue:** The sidebar navigation linked to an incorrect route for enrollments, and there was a concern regarding `node_modules` being tracked in the repository.

**Fix:** `frontend/src/components/AppShell.tsx` was corrected to link to `/enrollments`, matching the registered route. Direct inspection confirmed that `node_modules` is correctly ignored by `.gitignore` and is not present in the git tree.

---

### Issue 7 — Integration test coverage was too thin (High)

**Original issue:** While unit tests existed, there was no integration-level proof that the API handlers actually returned correct JSON structures or headers for reports and exports.

**Fix:** The integration suite now utilizes `net/http/httptest` to call handlers through the full middleware stack. New tests like `TestReports_RunReport_ReturnsDefinitionAndRows` and `TestReports_Export_ReturnsCSVAndAuditHeader` validate the actual HTTP response body, content types, and custom headers (e.g., `X-Export-Job-ID`).

---

## Sanitization Note

* This file only lists items from `audit_report-02.md` that have been verified via direct code and migration inspection.
* **Verification Limitation:** While the integration tests are structurally complete and pass package-level compilation, their successful runtime execution still depends on a functional local `lms_test` database. Failure to connect to PostgreSQL in a specific environment remains an environmental issue rather than a code defect.