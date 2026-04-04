-- Migration 012: Compliance-oriented audit event log (append-only)
--
-- This table is NEVER updated or deleted by the application. All audit writes
-- go through internal/audit/logger.go. Direct database access to this table
-- by application code outside of the audit package is prohibited.
SET search_path = lms, public;

CREATE TABLE audit_events (
    id            UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    event_type    VARCHAR(100) NOT NULL,
    -- Actor: the authenticated staff user who caused the event.
    -- actor_username is denormalised in case the user record is later deleted.
    actor_user_id UUID         REFERENCES users(id) ON DELETE SET NULL,
    actor_username VARCHAR(100),
    workstation_id VARCHAR(255),
    ip_address     INET,
    branch_id      UUID         REFERENCES branches(id) ON DELETE SET NULL,
    -- Resource: the entity that was affected (if any).
    resource_type  VARCHAR(100),
    resource_id    UUID,
    -- Before/after snapshots for admin changes, moderation decisions, etc.
    -- Sensitive field values must NOT appear in these snapshots.
    before_value   JSONB,
    after_value    JSONB,
    -- Additional structured context (e.g. export filters, lockout reason).
    metadata       JSONB,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Append-only enforcement note:
-- The application role (lms_user) should have INSERT but NOT UPDATE/DELETE on this table.
-- Enforced via:
REVOKE UPDATE, DELETE ON audit_events FROM lms_user;

-- Owners can bypass REVOKE, so enforce append-only semantics with a trigger too.
CREATE OR REPLACE FUNCTION forbid_audit_mutation()
RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'audit_events is append-only';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_forbid_audit_update
BEFORE UPDATE ON audit_events
FOR EACH ROW
EXECUTE FUNCTION forbid_audit_mutation();

CREATE TRIGGER trg_forbid_audit_delete
BEFORE DELETE ON audit_events
FOR EACH ROW
EXECUTE FUNCTION forbid_audit_mutation();

-- Indexes support the most common compliance queries.
CREATE INDEX idx_audit_event_type   ON audit_events(event_type);
CREATE INDEX idx_audit_actor        ON audit_events(actor_user_id);
CREATE INDEX idx_audit_branch       ON audit_events(branch_id, created_at DESC);
CREATE INDEX idx_audit_resource     ON audit_events(resource_type, resource_id);
CREATE INDEX idx_audit_created_at   ON audit_events(created_at DESC);
