-- Migration 008: Programs, enrollment windows, prerequisites, rules, and enrollment records
SET search_path = lms, public;

-- ── Programs ──────────────────────────────────────────────────────────────────
-- venue_type maps to the 'room_type' reporting alias.
-- enrollment_channel maps to the 'channel' reporting alias.
CREATE TABLE programs (
    id                    UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    branch_id             UUID         NOT NULL REFERENCES branches(id),
    title                 VARCHAR(500) NOT NULL,
    description           TEXT,
    category              VARCHAR(100),
    -- venue_type: reporting alias = room_type / occupancy_rate denominator
    venue_type            VARCHAR(100),   -- e.g. 'reading_room', 'program_hall', 'study_room'
    venue_name            VARCHAR(255),
    capacity              SMALLINT     NOT NULL CHECK (capacity > 0),
    enrollment_opens_at   TIMESTAMPTZ,
    enrollment_closes_at  TIMESTAMPTZ,
    starts_at             TIMESTAMPTZ  NOT NULL,
    ends_at               TIMESTAMPTZ  NOT NULL,
    status                VARCHAR(50)  NOT NULL DEFAULT 'draft'
                          CHECK (status IN ('draft', 'published', 'cancelled', 'completed')),
    -- enrollment_channel: reporting alias = channel
    enrollment_channel    VARCHAR(50)  NOT NULL DEFAULT 'any'
                          CHECK (enrollment_channel IN ('any', 'staff_only', 'self_service')),
    created_by            UUID         REFERENCES users(id) ON DELETE SET NULL,
    created_at            TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_program_dates     CHECK (ends_at > starts_at),
    CONSTRAINT chk_enrollment_window CHECK (
        enrollment_closes_at IS NULL
        OR enrollment_opens_at IS NULL
        OR enrollment_closes_at > enrollment_opens_at
    )
);

CREATE INDEX idx_programs_branch  ON programs(branch_id, status);
CREATE INDEX idx_programs_dates   ON programs(starts_at, ends_at);
CREATE INDEX idx_programs_status  ON programs(status);

-- ── Program prerequisites ──────────────────────────────────────────────────────
-- A reader must have a 'completed' enrollment in required_program_id to be eligible.
CREATE TABLE program_prerequisites (
    id                  UUID  PRIMARY KEY DEFAULT uuid_generate_v4(),
    program_id          UUID  NOT NULL REFERENCES programs(id) ON DELETE CASCADE,
    required_program_id UUID  NOT NULL REFERENCES programs(id) ON DELETE CASCADE,
    description         TEXT,   -- human-readable override label
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (program_id, required_program_id),
    CONSTRAINT chk_prereq_self CHECK (program_id <> required_program_id)
);

-- ── Whitelist / blacklist enrollment rules ────────────────────────────────────
-- Whitelist: ONLY readers matching this rule may enroll.
-- Blacklist: readers matching this rule are DENIED enrollment.
-- If both exist for a program, blacklist takes precedence.
CREATE TABLE enrollment_rules (
    id           UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    program_id   UUID        NOT NULL REFERENCES programs(id) ON DELETE CASCADE,
    rule_type    VARCHAR(20) NOT NULL CHECK (rule_type IN ('whitelist', 'blacklist')),
    -- match_field: the reader attribute to test; match_value: the expected value
    match_field  VARCHAR(100) NOT NULL,   -- e.g. 'status_code', 'branch_id', 'reader_number'
    match_value  VARCHAR(255) NOT NULL,
    reason       TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_enroll_rules_program ON enrollment_rules(program_id, rule_type);

-- ── Enrollments ───────────────────────────────────────────────────────────────
-- One enrollment per reader per program (enforced by UNIQUE constraint).
-- Concurrency-safe add/drop requires a SELECT FOR UPDATE on the program row
-- to check capacity before inserting.
CREATE TABLE enrollments (
    id                 UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    program_id         UUID        NOT NULL REFERENCES programs(id),
    reader_id          UUID        NOT NULL REFERENCES readers(id),
    branch_id          UUID        NOT NULL REFERENCES branches(id),
    status             VARCHAR(50) NOT NULL DEFAULT 'pending'
                       CHECK (status IN ('pending', 'confirmed', 'waitlisted',
                                         'cancelled', 'completed', 'no_show')),
    -- enrollment_channel: reporting alias = channel
    enrollment_channel VARCHAR(50),
    waitlist_position  SMALLINT    CHECK (waitlist_position > 0),
    enrolled_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    enrolled_by        UUID        REFERENCES users(id) ON DELETE SET NULL,
    UNIQUE (program_id, reader_id)   -- one enrollment per reader per program
);

CREATE INDEX idx_enrollments_program ON enrollments(program_id, status);
CREATE INDEX idx_enrollments_reader  ON enrollments(reader_id);
CREATE INDEX idx_enrollments_branch  ON enrollments(branch_id);

-- ── Enrollment history (add/drop audit trail) ─────────────────────────────────
-- Every status change on an enrollment is recorded here.
-- This is distinct from the central audit_events table: it provides a
-- structured, queryable history per enrollment for the enrollment UI.
CREATE TABLE enrollment_history (
    id               UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    enrollment_id    UUID        NOT NULL REFERENCES enrollments(id) ON DELETE CASCADE,
    previous_status  VARCHAR(50) NOT NULL,
    new_status       VARCHAR(50) NOT NULL,
    changed_by       UUID        REFERENCES users(id) ON DELETE SET NULL,
    reason           TEXT,
    changed_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    workstation_id   VARCHAR(255)
);

CREATE INDEX idx_enroll_history ON enrollment_history(enrollment_id, changed_at DESC);
