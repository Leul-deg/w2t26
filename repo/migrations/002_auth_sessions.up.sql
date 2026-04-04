-- Migration 002: Authentication and session tables
SET search_path = lms, public;

-- ── Roles ────────────────────────────────────────────────────────────────────
CREATE TABLE roles (
    id          UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    name        VARCHAR(100) NOT NULL UNIQUE,   -- 'administrator', 'operations_staff', 'content_moderator'
    description TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ── Permissions ───────────────────────────────────────────────────────────────
-- Named capability tokens used by RBAC middleware.
-- Format: <resource>:<action>  e.g.  readers:write, exports:create
CREATE TABLE permissions (
    id          UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    name        VARCHAR(100) NOT NULL UNIQUE,
    description TEXT,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- ── Role → Permission mapping ─────────────────────────────────────────────────
CREATE TABLE role_permissions (
    role_id       UUID NOT NULL REFERENCES roles(id)       ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

-- ── Users ─────────────────────────────────────────────────────────────────────
CREATE TABLE users (
    id              UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    username        VARCHAR(100) NOT NULL UNIQUE,
    email           VARCHAR(255) NOT NULL UNIQUE,
    -- bcrypt hash (cost ≥ 12) produced by golang.org/x/crypto/bcrypt
    password_hash   VARCHAR(255) NOT NULL,
    is_active       BOOLEAN      NOT NULL DEFAULT TRUE,
    -- Lockout tracking: reset on successful login
    failed_attempts SMALLINT     NOT NULL DEFAULT 0 CHECK (failed_attempts >= 0),
    locked_until    TIMESTAMPTZ,                      -- NULL means not locked
    last_login_at   TIMESTAMPTZ,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_username ON users(username);
CREATE INDEX idx_users_locked_until ON users(locked_until) WHERE locked_until IS NOT NULL;

-- ── User → Role mapping ───────────────────────────────────────────────────────
CREATE TABLE user_roles (
    user_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id     UUID        NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    assigned_by UUID        REFERENCES users(id) ON DELETE SET NULL,
    PRIMARY KEY (user_id, role_id)
);

-- ── Server-side sessions ──────────────────────────────────────────────────────
-- The session token itself is never stored; only a SHA-256 hex digest is kept.
-- The application hashes the cookie value before lookup.
CREATE TABLE sessions (
    id               UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    token_hash       VARCHAR(64)  NOT NULL UNIQUE,   -- hex(SHA-256(raw_token))
    user_id          UUID         NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workstation_id   VARCHAR(255),                    -- hostname or IP of originating workstation
    ip_address       INET,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    last_active_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    expires_at       TIMESTAMPTZ  NOT NULL,
    is_valid         BOOLEAN      NOT NULL DEFAULT TRUE
);

CREATE INDEX idx_sessions_token_hash  ON sessions(token_hash)  WHERE is_valid = TRUE;
CREATE INDEX idx_sessions_user_id     ON sessions(user_id);
CREATE INDEX idx_sessions_expires_at  ON sessions(expires_at)  WHERE is_valid = TRUE;

-- ── CAPTCHA challenges ────────────────────────────────────────────────────────
-- Issued after 3 consecutive failed login attempts from the same IP/username.
-- Deleted (or marked used) after a single validation attempt.
CREATE TABLE captcha_challenges (
    id            UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    challenge_key VARCHAR(64) NOT NULL UNIQUE,  -- opaque token sent to client
    answer_hash   VARCHAR(64) NOT NULL,          -- hex(SHA-256(correct_answer))
    username      VARCHAR(100),                  -- the username being challenged (may be NULL for IP-based)
    ip_address    INET,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at    TIMESTAMPTZ NOT NULL,
    used          BOOLEAN     NOT NULL DEFAULT FALSE
);

CREATE INDEX idx_captcha_key        ON captcha_challenges(challenge_key) WHERE used = FALSE;
CREATE INDEX idx_captcha_expires_at ON captcha_challenges(expires_at)    WHERE used = FALSE;
