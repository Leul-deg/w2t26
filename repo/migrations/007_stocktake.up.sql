-- Migration 007: Stocktake sessions and findings
SET search_path = lms, public;

-- ── Stocktake sessions ────────────────────────────────────────────────────────
-- A stocktake session is a named scan run for one branch.
-- Only one session per branch may be 'open' or 'in_progress' at a time
-- (enforced by the application layer; a partial index aids detection).
CREATE TABLE stocktake_sessions (
    id           UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    branch_id    UUID         NOT NULL REFERENCES branches(id),
    name         VARCHAR(255) NOT NULL,
    status       VARCHAR(50)  NOT NULL DEFAULT 'open'
                 CHECK (status IN ('open', 'in_progress', 'closed', 'cancelled')),
    started_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    closed_at    TIMESTAMPTZ,
    started_by   UUID         NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    closed_by    UUID         REFERENCES users(id) ON DELETE SET NULL,
    notes        TEXT,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_stocktake_branch ON stocktake_sessions(branch_id, status);
-- Prevent two active sessions for the same branch.
CREATE UNIQUE INDEX idx_stocktake_one_active
    ON stocktake_sessions(branch_id)
    WHERE status IN ('open', 'in_progress');

-- ── Stocktake findings ────────────────────────────────────────────────────────
-- One row per barcode scanned in a session.
-- copy_id is NULL if the barcode is not in the system (unexpected item).
CREATE TABLE stocktake_findings (
    id              UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id      UUID        NOT NULL REFERENCES stocktake_sessions(id) ON DELETE CASCADE,
    copy_id         UUID        REFERENCES copies(id) ON DELETE SET NULL,
    scanned_barcode VARCHAR(100) NOT NULL,
    finding_type    VARCHAR(50) NOT NULL
                    CHECK (finding_type IN ('found', 'missing', 'unexpected', 'damaged')),
    scanned_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    scanned_by      UUID        REFERENCES users(id) ON DELETE SET NULL,
    notes           TEXT,
    UNIQUE (session_id, scanned_barcode)   -- no duplicate scans in the same session
);

CREATE INDEX idx_findings_session ON stocktake_findings(session_id);
CREATE INDEX idx_findings_copy    ON stocktake_findings(copy_id) WHERE copy_id IS NOT NULL;
