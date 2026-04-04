# Open Questions

## Domain Ambiguities

### Reporting terminology

The original prompt includes hospitality-style reporting terms:

- occupancy rate
- RevPAR
- revenue mix
- room type
- channel

Current project direction preserves them through aliasing onto LMS-native metrics, but the business owner should confirm whether:

- these are required as literal user-facing report labels
- they are only compatibility aliases
- they should remain visible to staff or only to managers

### Branch scope semantics

Administrators are effectively cross-branch today, while non-admins are branch-scoped. Open questions:

- should some managers see multiple assigned branches rather than one effective branch
- should report exports support branch roll-up for non-admin users with multi-branch assignments

### Reader privacy reveal policy

The product requires masked fields plus step-up reveal. Clarifications still useful:

- should reveal access be logged every time or only on successful reveal
- should there be a short-lived reveal session window after step-up, or one reveal per action

### Stocktake anomaly handling

The prompt mentions anomalous copies and follow-up holds. Clarifications:

- should anomaly follow-up create a formal hold record, an issue flag, or both
- should stocktake findings feed directly into moderation/appeals analytics

### Enrollment rule depth

The system already supports eligibility and atomic enrollment flows, but open questions remain:

- should waitlists become mandatory
- should prerequisite satisfaction consider only completed programs or also approved equivalents
- should blacklist rules be temporary, permanent, or rule-driven by date

## UX Questions

### Reports UI

- Should report filters be customized per report definition rather than using a generic filter panel?
- Should cached aggregates and live results appear side by side by default, or should the UI default to one mode?

### Excel support

CSV export is compatible with Excel workflows, but if native `.xlsx` output becomes required:

- should the app produce native spreadsheet files
- or is CSV sufficient for compliance and staff workflows

## Operational Questions

### Scheduler behavior

The reporting scheduler runs nightly in-process. Confirm:

- is local midnight the correct aggregation time
- should there be an admin UI to force nightly jobs outside the reports recalculate endpoint

### Audit retention

Audit logging is first-class, but retention rules are not specified:

- how long should audit, export, and moderation records be retained
- should old audit rows ever be archived offline

## Documentation Questions

### Reviewer-facing project docs

The project now has multiple implementation reports and generated session files. Decide whether:

- they should be part of the official deliverable
- or excluded from final handoff artifacts
