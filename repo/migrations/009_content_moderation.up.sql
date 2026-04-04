-- Migration 009: Governed content items and moderation queue
SET search_path = lms, public;

-- ── Governed content ──────────────────────────────────────────────────────────
-- Content that must pass a moderation workflow before becoming visible.
-- Lifecycle: draft → pending_review → (approved | rejected) → published → archived
CREATE TABLE governed_content (
    id             UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    branch_id      UUID         NOT NULL REFERENCES branches(id),
    title          VARCHAR(500) NOT NULL,
    content_type   VARCHAR(100) NOT NULL
                   CHECK (content_type IN ('announcement', 'document', 'digital_resource', 'policy')),
    body           TEXT,
    file_path      TEXT,        -- local filesystem path to uploaded file (offline-first)
    file_name      VARCHAR(500),
    status         VARCHAR(50)  NOT NULL DEFAULT 'draft'
                   CHECK (status IN ('draft', 'pending_review', 'approved', 'rejected', 'published', 'archived')),
    submitted_by   UUID         NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    submitted_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    published_at   TIMESTAMPTZ,
    archived_at    TIMESTAMPTZ,
    rejection_reason TEXT,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_content_branch  ON governed_content(branch_id, status);
CREATE INDEX idx_content_status  ON governed_content(status);
CREATE INDEX idx_content_type    ON governed_content(content_type, status);

-- ── Moderation items ──────────────────────────────────────────────────────────
-- Each piece of content that enters review generates one moderation item.
-- A content item may have at most one non-decided moderation item at a time.
CREATE TABLE moderation_items (
    id              UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    content_id      UUID        NOT NULL REFERENCES governed_content(id) ON DELETE CASCADE,
    assigned_to     UUID        REFERENCES users(id) ON DELETE SET NULL,
    status          VARCHAR(50) NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending', 'in_review', 'decided')),
    decision        VARCHAR(50) CHECK (decision IN ('approved', 'rejected')),
    decision_reason TEXT,
    decided_by      UUID        REFERENCES users(id) ON DELETE SET NULL,
    decided_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_mod_status      ON moderation_items(status);
CREATE INDEX idx_mod_assigned    ON moderation_items(assigned_to) WHERE status <> 'decided';
CREATE INDEX idx_mod_content     ON moderation_items(content_id);
