# Business Logic Questions Log

## Domain Ambiguities

Reporting terminology
Question: The original prompt includes hospitality-style reporting terms (occupancy rate, RevPAR, revenue mix, room type, channel). Are these required as literal user-facing report labels, are they only compatibility aliases, and should they remain visible to staff or only to managers?
My Understanding: Current project direction preserves them through aliasing onto LMS-native metrics, but confirmation is needed on their user-facing requirements.
Solution: Confirm business requirements with the owner regarding these reporting labels and their visibility scopes.

Branch scope semantics
Question: Administrators are effectively cross-branch today, while non-admins are branch-scoped. Should some managers see multiple assigned branches rather than one effective branch, and should report exports support branch roll-up for non-admin users with multi-branch assignments?
My Understanding: Multi-branch assignments for non-admins might require cross-branch report exports and multi-branch views for managers.
Solution: Clarify multi-branch reporting roll-up and manager view semantics with the product team.

Reader privacy reveal policy
Question: The product requires masked fields plus step-up reveal. Should reveal access be logged every time or only on successful reveal, and should there be a short-lived reveal session window after step-up, or one reveal per action?
My Understanding: Audit and security requirements may dictate either per-action logs or session-based access.
Solution: Clarify logging requirements and session window behavior for privacy reveals.

Stocktake anomaly handling
Question: The prompt mentions anomalous copies and follow-up holds. Should anomaly follow-up create a formal hold record, an issue flag, or both, and should stocktake findings feed directly into moderation/appeals analytics?
My Understanding: There needs to be a connection between anomalies and subsequent issue tracking or moderation analytics.
Solution: Define the technical workflow for anomaly follow-up actions and integration with analytics.

Enrollment rule depth
Question: The system already supports eligibility and atomic enrollment flows. Should waitlists become mandatory, should prerequisite satisfaction consider only completed programs or also approved equivalents, and should blacklist rules be temporary, permanent, or rule-driven by date?
My Understanding: Current features are basic and may need comprehensive rules for prerequisites and blacklists.
Solution: Define requirements for waitlist enforcement, equivalence logic for prerequisites, and time-based rules for blacklists.

## UX Questions

Reports UI
Question: Should report filters be customized per report definition rather than using a generic filter panel, and should cached aggregates and live results appear side by side by default, or should the UI default to one mode?
My Understanding: The UI might need distinct filter panels and clarity on live/cached presentation.
Solution: Establish UI guidelines for report filters and data display modes.

Excel support
Question: CSV export is compatible with Excel workflows. If native `.xlsx` output becomes required, should the app produce native spreadsheet files, or is CSV sufficient for compliance and staff workflows?
My Understanding: CSV may suffice, but native Excel could be desired for ease of use.
Solution: Confirm if CSV output fully satisfies compliance and workflow requirements or if native `.xlsx` generation is needed.

## Operational Questions

Scheduler behavior
Question: The reporting scheduler runs nightly in-process. Is local midnight the correct aggregation time, and should there be an admin UI to force nightly jobs outside the reports recalculate endpoint?
My Understanding: Background reporting typically needs timezone clarity and potential manual trigger capabilities in the UI.
Solution: Confirm the timezone rules for aggregation and decide if an admin UI button is required for manual job execution.

Audit retention
Question: Audit logging is first-class, but retention rules are not specified. How long should audit, export, and moderation records be retained, and should old audit rows ever be archived offline?
My Understanding: Storage costs and compliance require data lifecycle policies.
Solution: Determine quantitative retention periods and archiving strategies for audit and moderation logs.

## Documentation Questions

Reviewer-facing project docs
Question: The project now has multiple implementation reports and generated session files. Should they be part of the official deliverable or excluded from final handoff artifacts?
My Understanding: Generated artifacts might clutter the handoff if not officially required by reviewers.
Solution: Identify which documents constitute the final deliverable and exclude the rest.
