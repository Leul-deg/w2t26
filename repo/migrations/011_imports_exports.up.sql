-- Migration 011: Bulk import and audited export records
SET search_path = lms, public;

-- ── Import jobs ───────────────────────────────────────────────────────────────
-- Lifecycle: uploaded → previewing → preview_ready → committed | rolled_back | failed
-- The application must roll back ALL import_rows in a single transaction if any
-- row-level error occurs (enforced in the import service, not here).
CREATE TABLE import_jobs (
    id              UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    branch_id       UUID         NOT NULL REFERENCES branches(id),
    import_type     VARCHAR(100) NOT NULL
                    CHECK (import_type IN ('readers', 'holdings', 'copies', 'programs', 'enrollments')),
    status          VARCHAR(50)  NOT NULL DEFAULT 'uploaded'
                    CHECK (status IN (
                        'uploaded', 'previewing', 'preview_ready',
                        'committed', 'rolled_back', 'failed'
                    )),
    file_name       VARCHAR(500) NOT NULL,
    file_path       TEXT,          -- local path to the uploaded file
    row_count       INTEGER,
    error_count     INTEGER        NOT NULL DEFAULT 0,
    error_summary   JSONB,         -- array of { row, field, message } for preview display
    uploaded_by     UUID           NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    uploaded_at     TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    committed_at    TIMESTAMPTZ,
    rolled_back_at  TIMESTAMPTZ,
    workstation_id  VARCHAR(255)
);

CREATE INDEX idx_import_jobs_branch ON import_jobs(branch_id, status);
CREATE INDEX idx_import_jobs_user   ON import_jobs(uploaded_by);

-- ── Import rows (staging) ─────────────────────────────────────────────────────
-- Every row parsed from the uploaded file is staged here before commit.
-- On commit, these rows are used to insert/update the target tables atomically.
-- On rollback or failure, the import_job status is updated; rows are retained
-- for 30 days for audit review, then cleaned up by a maintenance job.
CREATE TABLE import_rows (
    id             UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    job_id         UUID        NOT NULL REFERENCES import_jobs(id) ON DELETE CASCADE,
    row_number     INTEGER     NOT NULL,
    raw_data       JSONB       NOT NULL,   -- original parsed values
    parsed_data    JSONB,                  -- validated/normalised values
    status         VARCHAR(50) NOT NULL DEFAULT 'pending'
                   CHECK (status IN ('pending', 'valid', 'invalid', 'committed', 'rolled_back')),
    error_details  TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (job_id, row_number)
);

CREATE INDEX idx_import_rows_job ON import_rows(job_id, status);

-- ── Export jobs (audited) ─────────────────────────────────────────────────────
-- Every export is logged regardless of content. The row is created BEFORE the
-- file is generated; exported_at is the completion timestamp.
-- No file contents are stored in the database.
CREATE TABLE export_jobs (
    id              UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    branch_id       UUID         NOT NULL REFERENCES branches(id),
    export_type     VARCHAR(100) NOT NULL
                    CHECK (export_type IN (
                        'readers', 'holdings', 'copies', 'circulation',
                        'programs', 'enrollments', 'audit_events', 'report'
                    )),
    filters_applied JSONB,         -- the filter parameters used for this export
    row_count       INTEGER,       -- populated after generation
    file_name       VARCHAR(500),
    exported_by     UUID           NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    exported_at     TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    workstation_id  VARCHAR(255)
);

CREATE INDEX idx_export_jobs_branch ON export_jobs(branch_id, exported_at DESC);
CREATE INDEX idx_export_jobs_user   ON export_jobs(exported_by);
