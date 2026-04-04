-- Migration 013: Report definitions and aggregation cache
--
-- Hospitality-term aliases are preserved in metric_aliases JSONB:
--   occupancy_rate → slot_utilization_rate
--   revpar         → resource_yield_per_available_slot
--   revenue_mix    → enrollment_mix_by_category
--   room_type      → venue_type
--   channel        → enrollment_channel
-- Both the original key and the alias are queryable via metric_aliases.
SET search_path = lms, public;

CREATE TABLE report_definitions (
    id               UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    name             VARCHAR(255) NOT NULL UNIQUE,
    description      TEXT,
    -- Parameterised query template; parameters are substituted at generation time.
    query_template   TEXT         NOT NULL,
    -- metric_aliases: maps canonical metric names to display aliases.
    -- Also preserves hospitality-origin terms as recognised aliases.
    -- Example:
    --   { "slot_utilization_rate": "occupancy_rate",
    --     "resource_yield_per_available_slot": "revpar",
    --     "enrollment_mix_by_category": "revenue_mix",
    --     "venue_type": "room_type",
    --     "enrollment_channel": "channel" }
    metric_aliases   JSONB        NOT NULL DEFAULT '{}',
    default_filters  JSONB,
    is_active        BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- ── Report aggregation cache ──────────────────────────────────────────────────
-- Stores pre-computed results keyed by (definition, branch, period).
-- NULL branch_id = all-branches aggregate.
CREATE TABLE report_aggregates (
    id                    UUID  PRIMARY KEY DEFAULT uuid_generate_v4(),
    report_definition_id  UUID  NOT NULL REFERENCES report_definitions(id) ON DELETE CASCADE,
    branch_id             UUID  REFERENCES branches(id) ON DELETE CASCADE,
    period_start          DATE  NOT NULL,
    period_end            DATE  NOT NULL,
    aggregate_data        JSONB NOT NULL,
    generated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (report_definition_id, branch_id, period_start, period_end),
    CONSTRAINT chk_report_period CHECK (period_end >= period_start)
);

CREATE INDEX idx_report_agg_def    ON report_aggregates(report_definition_id, branch_id);
CREATE INDEX idx_report_agg_period ON report_aggregates(period_start, period_end);
