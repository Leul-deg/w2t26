# Audit Report 02 Fix Check

Source basis: fixes observed across `self-report-03.md`, `self-report-04.md`, and `self-report-05.md`

## Issues From Report 02 That Were Clearly Fixed Later

1. Circulation stopped being a stub and was connected in backend and frontend routing.
   - How it was fixed: later reports noted circulation handlers were registered in `main.go` and the frontend circulation page was no longer a placeholder route.

2. Imports/exports permission mismatch was fixed to the seeded taxonomy.
   - How it was fixed: later reports noted handlers were updated to use `imports:create/preview/commit` and `exports:create`.

3. Server-side step-up enforcement was added for sensitive reveal.
   - How it was fixed: later reports noted the reveal endpoint now validates a recent server-side step-up timestamp.

4. Circulation cross-branch object authorization was hardened.
   - How it was fixed: later reports noted branch ownership checks were added before using `copy_id`, plus branch predicates in repo lock queries.

5. Program enrollment-rule endpoints now parent-authorize by branch.
   - How it was fixed: later reports noted program rule handlers verify parent program accessibility before listing, adding, or removing rules.

## Sanitization Note

- Only fixes that map directly to issues listed in `audit_report-02.md` and are explicitly described as fixed in later `.tmp` reports are included here.
- Later improvements that belonged to different reports, or only partially overlapped with `audit_report-02.md`, were intentionally removed.
