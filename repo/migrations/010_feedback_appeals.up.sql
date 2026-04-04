-- Migration 010: Feedback, tags, appeals, and arbitration decisions
SET search_path = lms, public;

-- ── Feedback tags (controlled vocabulary) ────────────────────────────────────
CREATE TABLE feedback_tags (
    id        UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    name      VARCHAR(100) NOT NULL UNIQUE,
    is_active BOOLEAN      NOT NULL DEFAULT TRUE
);

INSERT INTO feedback_tags (name) VALUES
    ('helpful'),
    ('outdated'),
    ('inappropriate'),
    ('excellent_resource'),
    ('needs_update'),
    ('inaccurate'),
    ('well_organised'),
    ('difficult_to_follow');

-- ── Feedback ──────────────────────────────────────────────────────────────────
-- Readers may submit star ratings and tagged comments against holdings or programs.
CREATE TABLE feedback (
    id            UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    branch_id     UUID        NOT NULL REFERENCES branches(id),
    reader_id     UUID        NOT NULL REFERENCES readers(id),
    -- target_type + target_id is a polymorphic reference
    target_type   VARCHAR(50) NOT NULL CHECK (target_type IN ('holding', 'program')),
    target_id     UUID        NOT NULL,
    rating        SMALLINT    CHECK (rating BETWEEN 1 AND 5),
    comment       TEXT,
    status        VARCHAR(50) NOT NULL DEFAULT 'pending'
                  CHECK (status IN ('pending', 'approved', 'rejected', 'flagged')),
    moderated_by  UUID        REFERENCES users(id) ON DELETE SET NULL,
    moderated_at  TIMESTAMPTZ,
    submitted_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_feedback_reader      ON feedback(reader_id);
CREATE INDEX idx_feedback_target      ON feedback(target_type, target_id);
CREATE INDEX idx_feedback_branch      ON feedback(branch_id, status);

-- ── Feedback → Tag mapping ────────────────────────────────────────────────────
CREATE TABLE feedback_tag_mappings (
    feedback_id UUID NOT NULL REFERENCES feedback(id)      ON DELETE CASCADE,
    tag_id      UUID NOT NULL REFERENCES feedback_tags(id) ON DELETE CASCADE,
    PRIMARY KEY (feedback_id, tag_id)
);

-- ── Appeals ───────────────────────────────────────────────────────────────────
-- Readers submit appeals against decisions: enrollment denials, suspensions, etc.
CREATE TABLE appeals (
    id           UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    branch_id    UUID         NOT NULL REFERENCES branches(id),
    reader_id    UUID         NOT NULL REFERENCES readers(id),
    appeal_type  VARCHAR(100) NOT NULL
                 CHECK (appeal_type IN (
                     'enrollment_denial', 'account_suspension',
                     'feedback_rejection', 'blacklist_removal', 'other'
                 )),
    -- Polymorphic target (the enrollment, reader record, or feedback being appealed)
    target_type  VARCHAR(50),
    target_id    UUID,
    reason       TEXT         NOT NULL,
    status       VARCHAR(50)  NOT NULL DEFAULT 'submitted'
                 CHECK (status IN ('submitted', 'under_review', 'resolved', 'dismissed')),
    submitted_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_appeals_branch  ON appeals(branch_id, status);
CREATE INDEX idx_appeals_reader  ON appeals(reader_id);

-- ── Appeal arbitration decisions ──────────────────────────────────────────────
-- Audit-grade record of who decided what and what changed as a result.
CREATE TABLE appeal_arbitrations (
    id               UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    appeal_id        UUID        NOT NULL REFERENCES appeals(id) ON DELETE CASCADE,
    arbitrator_id    UUID        NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    decision         VARCHAR(50) NOT NULL CHECK (decision IN ('upheld', 'dismissed', 'partial')),
    decision_notes   TEXT        NOT NULL,
    before_state     JSONB,   -- snapshot of the target record before the decision
    after_state      JSONB,   -- snapshot of the target record after the decision
    decided_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_arbitrations_appeal ON appeal_arbitrations(appeal_id);
