-- Migration 004: Reader profiles and status lookup
SET search_path = lms, public;

-- ── Reader status lookup ──────────────────────────────────────────────────────
CREATE TABLE reader_statuses (
    code        VARCHAR(50) PRIMARY KEY,
    description TEXT        NOT NULL,
    allows_borrowing BOOLEAN NOT NULL DEFAULT TRUE,
    allows_enrollment BOOLEAN NOT NULL DEFAULT TRUE,
    is_system   BOOLEAN     NOT NULL DEFAULT TRUE   -- system statuses cannot be deleted
);

INSERT INTO reader_statuses (code, description, allows_borrowing, allows_enrollment) VALUES
    ('active',               'Reader account is in good standing',                     TRUE,  TRUE),
    ('frozen',               'Account temporarily suspended; no new borrowing/enrollment', FALSE, FALSE),
    ('blacklisted',          'Account permanently barred from library services',        FALSE, FALSE),
    ('pending_verification', 'Registration submitted; awaiting staff verification',     FALSE, FALSE);

-- ── Readers ───────────────────────────────────────────────────────────────────
-- Sensitive fields are encrypted at the application layer (AES-256-GCM) before
-- being stored. The columns hold base64-encoded ciphertext. Plaintext values
-- are never written to the database directly.
--
-- Sensitive fields (masked by default in API responses):
--   national_id_enc, contact_email_enc, contact_phone_enc, date_of_birth_enc
--
-- Non-sensitive fields are stored in plaintext.
CREATE TABLE readers (
    id                    UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    branch_id             UUID         NOT NULL REFERENCES branches(id),
    reader_number         VARCHAR(50)  NOT NULL UNIQUE,  -- public-facing card/membership number
    status_code           VARCHAR(50)  NOT NULL REFERENCES reader_statuses(code) DEFAULT 'active',
    first_name            VARCHAR(255) NOT NULL,
    last_name             VARCHAR(255) NOT NULL,
    preferred_name        VARCHAR(255),
    -- Encrypted at rest; NULL means not collected
    national_id_enc       TEXT,           -- AES-256-GCM ciphertext
    contact_email_enc     TEXT,
    contact_phone_enc     TEXT,
    date_of_birth_enc     TEXT,
    -- Non-sensitive
    notes                 TEXT,
    registered_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    created_at            TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    created_by            UUID         REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX idx_readers_branch  ON readers(branch_id);
CREATE INDEX idx_readers_status  ON readers(branch_id, status_code);
CREATE INDEX idx_readers_name    ON readers(branch_id, last_name, first_name);

-- reader_number uniqueness is already enforced by the UNIQUE constraint above.
-- The index makes lookups fast without a separate explicit index.
