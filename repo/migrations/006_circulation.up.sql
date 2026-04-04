-- Migration 006: Circulation events (checkout, return, hold, transit, renewal)
SET search_path = lms, public;

CREATE TABLE circulation_events (
    id             UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    copy_id        UUID         NOT NULL REFERENCES copies(id),
    reader_id      UUID         NOT NULL REFERENCES readers(id),
    branch_id      UUID         NOT NULL REFERENCES branches(id),
    -- event_type governs what fields are relevant:
    --   checkout       → due_date required
    --   return         → returned_at required
    --   renewal        → due_date (new due date) required
    --   hold_placed    → no additional fields
    --   hold_cancelled → no additional fields
    --   transit_out    → destination_branch_id recommended
    --   transit_in     → no additional fields
    event_type           VARCHAR(50)  NOT NULL
                         CHECK (event_type IN (
                             'checkout', 'return', 'renewal',
                             'hold_placed', 'hold_cancelled',
                             'transit_out', 'transit_in'
                         )),
    due_date             DATE,
    returned_at          TIMESTAMPTZ,
    destination_branch_id UUID       REFERENCES branches(id),
    performed_by         UUID        REFERENCES users(id) ON DELETE SET NULL,
    workstation_id       VARCHAR(255),
    notes                TEXT,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_circ_copy      ON circulation_events(copy_id, created_at DESC);
CREATE INDEX idx_circ_reader    ON circulation_events(reader_id, created_at DESC);
CREATE INDEX idx_circ_branch    ON circulation_events(branch_id, created_at DESC);
CREATE INDEX idx_circ_type      ON circulation_events(event_type);
CREATE INDEX idx_circ_due_date  ON circulation_events(due_date) WHERE due_date IS NOT NULL;
