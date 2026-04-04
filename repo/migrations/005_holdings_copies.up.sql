-- Migration 005: Holdings (title-level) and copies (copy-level items)
-- Copy-level inventory is NOT collapsed into title-level records.
-- Each physical item is a distinct row in the copies table with its own barcode.
SET search_path = lms, public;

-- ── Holdings (title-level bibliographic records) ───────────────────────────────
CREATE TABLE holdings (
    id               UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    branch_id        UUID         NOT NULL REFERENCES branches(id),
    title            VARCHAR(500) NOT NULL,
    author           VARCHAR(500),
    isbn             VARCHAR(20),
    publisher        VARCHAR(255),
    publication_year SMALLINT     CHECK (publication_year BETWEEN 1000 AND 2100),
    category         VARCHAR(100),
    subcategory      VARCHAR(100),
    language         VARCHAR(10)  NOT NULL DEFAULT 'en',
    description      TEXT,
    cover_image_path TEXT,   -- filesystem path; no external CDN (offline-first)
    is_active        BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    created_by       UUID         REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX idx_holdings_branch    ON holdings(branch_id);
CREATE INDEX idx_holdings_isbn      ON holdings(isbn)      WHERE isbn IS NOT NULL;
CREATE INDEX idx_holdings_title     ON holdings(branch_id, title);
CREATE INDEX idx_holdings_category  ON holdings(branch_id, category) WHERE category IS NOT NULL;

-- ── Copy status lookup ────────────────────────────────────────────────────────
CREATE TABLE copy_statuses (
    code        VARCHAR(50) PRIMARY KEY,
    description TEXT        NOT NULL,
    is_borrowable BOOLEAN   NOT NULL DEFAULT FALSE  -- can a reader borrow in this status?
);

INSERT INTO copy_statuses (code, description, is_borrowable) VALUES
    ('available',  'Copy is on shelf and available to borrow',             TRUE),
    ('checked_out','Copy is currently on loan to a reader',                FALSE),
    ('on_hold',    'Copy is reserved for a reader hold',                   FALSE),
    ('lost',       'Copy reported lost',                                   FALSE),
    ('damaged',    'Copy is damaged and under assessment',                 FALSE),
    ('withdrawn',  'Copy withdrawn from circulation permanently',          FALSE),
    ('in_transit', 'Copy is being transferred between branches',           FALSE),
    ('processing', 'New copy being catalogued and processed',              FALSE);

-- ── Copies (physical item-level records) ──────────────────────────────────────
-- Each row is one physical item with a unique barcode.
-- branch_id is the current physical location, which may differ from the
-- holding's home branch during an inter-branch transit.
CREATE TABLE copies (
    id             UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    holding_id     UUID        NOT NULL REFERENCES holdings(id),
    branch_id      UUID        NOT NULL REFERENCES branches(id),    -- physical location
    barcode        VARCHAR(100) NOT NULL UNIQUE,   -- unique across all branches
    status_code    VARCHAR(50) NOT NULL REFERENCES copy_statuses(code) DEFAULT 'available',
    condition      VARCHAR(50) NOT NULL DEFAULT 'good'
                   CHECK (condition IN ('new', 'good', 'fair', 'poor', 'damaged')),
    shelf_location VARCHAR(100),
    acquired_at    DATE,
    withdrawn_at   DATE,
    price_paid     NUMERIC(10,2),
    notes          TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_copies_holding  ON copies(holding_id);
CREATE INDEX idx_copies_branch   ON copies(branch_id);
CREATE INDEX idx_copies_status   ON copies(branch_id, status_code);
-- barcode uniqueness is enforced by the UNIQUE constraint; no extra index needed.
