# Library Operations & Enrollment Management Suite

## Overview

This project is an offline-first internal operations suite for a public or corporate library. It is designed for staff workstations and supports three major operational areas:

- daily library operations such as reader management, circulation, holdings, and stocktake
- scheduled program enrollment with transactional capacity and conflict handling
- governed content publishing, feedback moderation, appeals, reporting, and compliance auditing

The system is intended to run either on a single machine or over a local network without any third-party online dependency.

## Product Goals

- Give library staff a secure internal interface for daily operational work.
- Enforce role-based access control for `Administrator`, `Operations Staff`, and `Content Moderator`.
- Preserve privacy of reader data through masking, encryption, and step-up verification.
- Support offline compliance reviews through audit trails, local exports, and nightly report aggregation.
- Keep backend and frontend decoupled so the system remains maintainable and deployable in different local setups.

## Architecture

### Frontend

- React + TypeScript
- Desktop-oriented internal UI
- Role-aware navigation and protected routes
- Session restoration via backend `/auth/me`
- Page modules for readers, programs, imports/exports, content moderation, appeals, and reporting

### Backend

- Go with Echo-style REST API
- Clear domain-oriented service structure
- Middleware for sessions, RBAC, and branch scoping
- Domain packages for readers, holdings, stocktake, programs, enrollment, imports, exports, content, moderation, feedback, appeals, and reports

### Database

- PostgreSQL as the system of record
- Schema-driven domain boundaries with migrations
- Audit and export history stored in first-class tables
- Pre-aggregated reporting data cached in `report_aggregates`

## Core Domains

### Authentication and Security

- Local username/password authentication only
- Salted password hashing
- Server-side session management
- 30-minute inactivity timeout
- CAPTCHA after 3 failed login attempts
- 15-minute lockout after 5 failed login attempts
- Audit logging for login, failure, logout, lockout, and sensitive actions

### Readers

- Reader search and profile management
- Card statuses: `active`, `frozen`, `blacklisted`
- Current holdings and borrowing history
- Sensitive fields masked by default
- Step-up reveal flow for authorized roles

### Holdings and Stocktake

- Copy-level barcode operations
- Shelf/location/status updates
- Stocktake session lifecycle
- Variance detection and anomaly handling
- Audit coverage on operational changes

### Programs and Enrollment

- Program configuration with windows, prerequisites, and rules
- Capacity-aware atomic enrollment
- Duplicate and conflict prevention
- Enrollment history and auditability

### Governed Content and Moderation

- Draft -> review -> publish lifecycle
- Moderation queue and decisions
- Feedback tagging and moderation
- Appeals and arbitration workflow

### Reporting and Export

- Configurable reports with branch-scoped visibility
- Audited CSV exports
- Nightly pre-aggregation plus on-demand recalculation
- KPI alias handling for hospitality-style terms in the original prompt

## Roles and Access Model

### Administrator

- Full system access across branches
- Can manage operational modules, publishing, exports, and reporting
- Can trigger report recalculation

### Operations Staff

- Focused on readers, holdings, circulation, stocktake, programs, enrollment, and some reporting/export functions
- Scoped to assigned branch data

### Content Moderator

- Focused on content workflow, moderation queue, feedback moderation, appeals, and reporting visibility
- Scoped to assigned branch data unless elevated

## Data Protection and Compliance

- Sensitive fields are encrypted at rest using a locally managed key
- API responses apply masking rules based on permission
- Before/after audit trails are recorded for important admin and moderation changes
- Exports are logged before and after generation
- No cloud analytics, SaaS auth, or third-party compliance dependency is required

## KPI Ambiguity Handling

The original prompt includes hospitality-oriented terms that are not naturally library-native. This project preserves them through explicit aliasing rather than dropping them:

- `occupancy_rate` -> `slot_utilization_rate`
- `revpar` -> `resource_yield_per_available_slot`
- `revenue_mix` -> `enrollment_mix_by_category`
- `room_type` -> `venue_type`
- `channel` -> `enrollment_channel`

This keeps the business request intact while grounding implementation in the actual LMS data model.

## Deployment Model

- Fully offline operation
- Local PostgreSQL instance
- Backend and frontend run on the same machine or local network
- Nightly reporting scheduler runs in-process without cron or cloud infrastructure
