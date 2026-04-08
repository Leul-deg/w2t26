# Audit Report 02 Fix Check

Source basis: fixes observed across `self-report-03.md`, `self-report-04.md`, and `self-report-05.md`

## Issues From Report 02 That Later Became Fixed

1. Circulation stopped being a stub and was connected in backend and frontend routing.
   - How it was fixed: later reports noted circulation handlers were registered in `main.go` and the frontend circulation page was no longer a placeholder route.

2. Imports/exports permission mismatch was fixed to the seeded taxonomy.
   - How it was fixed: later reports noted handlers were updated to use `imports:create/preview/commit` and `exports:create`.

3. Server-side step-up enforcement was added for sensitive reveal.
   - How it was fixed: later reports noted the reveal endpoint now validates a recent server-side step-up timestamp.

4. Reports migration and docs consistency improved.
   - How it was fixed: later reports noted migration documentation was updated and report dispatch alignment was improved.

5. Circulation cross-branch object authorization was hardened.
   - How it was fixed: later reports noted branch ownership checks were added before using `copy_id`, plus branch predicates in repo lock queries.

6. Program enrollment-rule endpoints now parent-authorize by branch.
   - How it was fixed: later reports noted program rule handlers verify parent program accessibility before listing, adding, or removing rules.

7. The unassigned-user sentinel path was no longer downgraded to global user scope.
   - How it was fixed: later reports noted the users handler preserves the sentinel branch ID instead of converting it to empty scope.

8. Admin empty-branch list behavior was normalized in major domains.
   - How it was fixed: later reports noted admin all-branch behavior was explicitly handled in circulation, programs, and reports.

9. Frontend reports branch targeting was implemented.
   - How it was fixed: later reports noted the frontend reports API and UI now pass `branch_id` for admin report actions.

10. Frontend admin program branch targeting was implemented.
   - How it was fixed: later reports noted program creation now accepts and sends `branch_id` for admin users.

11. Suspect integration-test assertions around unassigned and sentinel behavior were corrected.
   - How it was fixed: later reports noted the response-shape assertion and temp-role setup were corrected in the authz integration suite.
