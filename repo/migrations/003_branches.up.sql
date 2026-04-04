-- Migration 003: Branches and user branch assignments
SET search_path = lms, public;

-- ── Branches ──────────────────────────────────────────────────────────────────
-- Every resource record is owned by one branch.
-- Administrators see all branches. Staff see only their assigned branch(es).
CREATE TABLE branches (
    id         UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    code       VARCHAR(20)  NOT NULL UNIQUE,   -- short identifier, e.g. 'MAIN', 'EAST'
    name       VARCHAR(255) NOT NULL,
    address    TEXT,
    phone      VARCHAR(50),
    is_active  BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- ── User → Branch assignment ──────────────────────────────────────────────────
-- A non-administrator user must have at least one branch assignment to access
-- branch-scoped data. Administrators bypass this check in the RBAC middleware.
CREATE TABLE user_branch_assignments (
    user_id     UUID        NOT NULL REFERENCES users(id)    ON DELETE CASCADE,
    branch_id   UUID        NOT NULL REFERENCES branches(id) ON DELETE CASCADE,
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    assigned_by UUID        REFERENCES users(id) ON DELETE SET NULL,
    PRIMARY KEY (user_id, branch_id)
);

CREATE INDEX idx_user_branch_branch ON user_branch_assignments(branch_id);
