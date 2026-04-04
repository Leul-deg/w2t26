# API Specification

## Overview

The backend exposes a REST API under `/api/v1`. Authentication is session-based and all privileged routes are intended for internal staff use only.

## Authentication

### `POST /api/v1/auth/login`

Authenticates a local staff user.

Request body:

```json
{
  "username": "admin",
  "password": "Admin1234!",
  "captcha_key": "optional-after-threshold",
  "captcha_answer": "optional-after-threshold"
}
```

Behavior:

- sets an `HttpOnly` session cookie on success
- may return CAPTCHA challenge after repeated failures
- may return lockout error after 5 failed attempts

### `GET /api/v1/auth/me`

Returns the authenticated user, roles, and permissions.

### `POST /api/v1/auth/logout`

Invalidates the current session.

### `POST /api/v1/auth/stepup`

Re-authenticates the current user for sensitive-field reveal.

## Readers

### `GET /api/v1/readers`

List readers with branch-scoped visibility.

Query examples:

- `search`
- `status`
- `page`
- `per_page`

### `POST /api/v1/readers`

Create a reader profile.

### `GET /api/v1/readers/:id`

Get reader detail with masked sensitive fields by default.

### `PATCH /api/v1/readers/:id`

Update reader profile.

### `PATCH /api/v1/readers/:id/status`

Change card status.

### `GET /api/v1/readers/:id/history`

Get borrowing history.

### `GET /api/v1/readers/:id/holdings`

Get current holdings.

### `POST /api/v1/readers/:id/reveal`

Reveal sensitive fields after step-up verification and permission check.

### `GET /api/v1/readers/statuses`

List supported reader statuses.

## Holdings and Copies

### `GET /api/v1/holdings`

List holdings with filters.

### `POST /api/v1/holdings`

Create a title-level holding record.

### `GET /api/v1/holdings/:id`

Get holding detail.

### `PATCH /api/v1/holdings/:id`

Update holding bibliographic fields.

### `DELETE /api/v1/holdings/:id`

Soft-deactivate holding.

### `GET /api/v1/holdings/:id/copies`

List copies for a holding.

### `POST /api/v1/holdings/:id/copies`

Add a copy to a holding.

### `GET /api/v1/copies/statuses`

List copy statuses.

### `GET /api/v1/copies/lookup?barcode=...`

Lookup a copy by barcode.

### `GET /api/v1/copies/:id`

Get copy detail.

### `PATCH /api/v1/copies/:id`

Update copy metadata such as shelf location and condition.

### `PATCH /api/v1/copies/:id/status`

Update copy circulation status.

## Stocktake

### `GET /api/v1/stocktake`

List stocktake sessions.

### `POST /api/v1/stocktake`

Create a stocktake session.

### `GET /api/v1/stocktake/:id`

Get stocktake session detail.

### `PATCH /api/v1/stocktake/:id/status`

Close or cancel a stocktake session.

### `GET /api/v1/stocktake/:id/findings`

List stocktake findings.

### `POST /api/v1/stocktake/:id/scan`

Record a barcode scan.

### `GET /api/v1/stocktake/:id/variances`

Return missing, unexpected, and misplaced copy variances.

## Programs and Enrollment

### `GET /api/v1/programs`

List programs.

### `POST /api/v1/programs`

Create a program.

### `GET /api/v1/programs/:id`

Get program detail.

### `PATCH /api/v1/programs/:id`

Update program.

### `POST /api/v1/programs/:id/enroll`

Enroll a reader.

### `POST /api/v1/programs/:id/drop`

Drop a reader.

### `GET /api/v1/enrollments`

List enrollments.

## Imports and Exports

### Imports

- `POST /api/v1/imports`
- `GET /api/v1/imports`
- `GET /api/v1/imports/:id`
- `POST /api/v1/imports/:id/preview`
- `POST /api/v1/imports/:id/commit`
- `POST /api/v1/imports/:id/rollback`

Import behavior:

- staged validation
- preview before commit
- full rollback on row-level error

### Exports

- `GET /api/v1/exports`
- `POST /api/v1/exports/readers`
- `POST /api/v1/exports/holdings`

Exports are logged in `export_jobs`.

## Governed Content

### `GET /api/v1/content`

List governed content items.

### `POST /api/v1/content`

Create draft content.

### `GET /api/v1/content/:id`

Get content detail.

### `PATCH /api/v1/content/:id`

Update draft content.

### `POST /api/v1/content/:id/submit`

Submit content for moderation queue.

### `POST /api/v1/content/:id/retract`

Retract submitted content back to draft.

### `POST /api/v1/content/:id/publish`

Publish approved content.

### `POST /api/v1/content/:id/archive`

Archive published content.

## Moderation

### `GET /api/v1/moderation/queue`

List moderation queue items.

### `GET /api/v1/moderation/items/:id`

Get queue item and linked content detail.

### `POST /api/v1/moderation/items/:id/assign`

Assign moderation item to current moderator.

### `POST /api/v1/moderation/items/:id/decide`

Approve or reject moderated content.

## Feedback

### `GET /api/v1/feedback`

List feedback items.

### `POST /api/v1/feedback`

Submit reader feedback with tags.

### `GET /api/v1/feedback/tags`

List available feedback tags.

### `GET /api/v1/feedback/:id`

Get feedback detail.

### `POST /api/v1/feedback/:id/moderate`

Moderate a feedback item.

## Appeals

### `GET /api/v1/appeals`

List appeals.

### `POST /api/v1/appeals`

Submit an appeal.

### `GET /api/v1/appeals/:id`

Get appeal detail and latest arbitration record.

### `POST /api/v1/appeals/:id/review`

Move appeal into `under_review`.

### `POST /api/v1/appeals/:id/arbitrate`

Record final arbitration decision.

## Reports

### `GET /api/v1/reports/definitions`

List active report definitions.

### `GET /api/v1/reports/run`

Run a live report query.

Required query params:

- `definition_id`
- `from`
- `to`

Optional extra query params are treated as report filters.

### `GET /api/v1/reports/aggregates`

Return cached aggregates for a report/date range.

### `POST /api/v1/reports/recalculate`

Recalculate cached aggregates on demand.

Request body:

```json
{
  "definition_id": "optional-single-definition",
  "from": "2026-03-01",
  "to": "2026-03-31"
}
```

### `GET /api/v1/reports/export`

Export a report result as CSV.

Behavior:

- creates an export audit record
- returns CSV response
- supports Excel-compatible workflows via CSV output

## Error Conventions

Expected status families include:

- `401` unauthenticated
- `403` forbidden
- `404` not found
- `409` conflict
- `422` validation error
- `423` account locked
- `428` CAPTCHA required
- `501` only for truly unimplemented modules still left in scaffold state

## Scope and Security Rules

- all privileged routes require an authenticated session
- role/permission checks are enforced in handlers and/or services
- non-admin users are branch-scoped
- sensitive reader fields are masked at the API layer
- report and export visibility is constrained by permissions and branch scope
