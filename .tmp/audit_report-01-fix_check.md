# Audit Report 01 Fix Check

Source basis: fixes observed across `self-report-03.md`, `self-report-04.md`, and `self-report-05.md`

## Issues From Report 01 That Later Became Fixed

1. Imports/exports permission taxonomy was corrected to match the seeded model.
   - How it was fixed: later reports noted handler checks were aligned to `imports:create/preview/commit` and `exports:create` instead of the earlier mismatched names.

2. Server-side step-up enforcement was added for sensitive reveal.
   - How it was fixed: later reports noted `/readers/:id/reveal` now checks a recent step-up window backed by session schema/repository support.

3. Circulation was wired into backend/frontend and no longer remained a stub.
   - How it was fixed: later reports noted backend circulation routes were registered and the frontend circulation page was connected in the route table.

4. Circulation cross-branch object-ID authorization was hardened.
   - How it was fixed: later reports noted `copy_id` resolution and active-checkout paths now validate branch ownership before use, with repo-level defense-in-depth predicates.

5. Program rule endpoints were branch-authorized by parent program.
   - How it was fixed: later reports noted `ListRules`, `AddRule`, and `RemoveRule` verify parent program accessibility before read or write.

6. Sentinel branch handling for user listing was fixed so it no longer downgraded to global scope.
   - How it was fixed: later reports noted the users handler preserved the sentinel branch ID instead of coercing it to empty/global scope.

7. Admin empty-branch behavior was normalized in circulation, programs, and reports.
   - How it was fixed: later reports noted empty branch context was deliberately handled for admin all-branch list and report flows.

8. Admin reports branch targeting was implemented end to end.
   - How it was fixed: later reports noted frontend reports requests, UI fields, and backend handlers all carried explicit `branch_id` for admin flows.

9. Admin program creation branch targeting was implemented.
   - How it was fixed: later reports noted the frontend program create path now captures and sends `branch_id` for admin users.

10. Previously suspect integration test assertions around sentinel/unassigned behavior were corrected.
   - How it was fixed: later reports noted the unassigned reader-list test now asserts `items`, and the unassigned user-list test uses a temp role with `users:read`.
