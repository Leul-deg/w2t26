-- Migration 018: Demo users for reviewer convenience
--
-- Seeds ready-to-use accounts for each role so reviewers can test role-based
-- behavior without writing SQL. All demo users are assigned to MAIN branch.
--
-- Seeded credentials:
--   Username: ops1         Password: Staff1234!    Role: operations_staff
--   Username: mod1         Password: Moderate1234! Role: content_moderator
--
-- CHANGE THESE PASSWORDS (or drop migration 018) in any non-development environment.
SET search_path = lms, public;

-- ── operations_staff demo user ────────────────────────────────────────────────
INSERT INTO users (id, username, email, password_hash) VALUES
    ('aaaaaaaa-0000-0000-0000-000000000002',
     'ops1',
     'ops1@lms.local',
     '$2a$12$ekwLDlYGYtSFRG5DUQaXZe8SRMMlQTgM4oqpXAOA4c/a57.COYEH2')
ON CONFLICT DO NOTHING;

INSERT INTO user_roles (user_id, role_id)
SELECT 'aaaaaaaa-0000-0000-0000-000000000002', id FROM roles WHERE name = 'operations_staff'
ON CONFLICT DO NOTHING;

INSERT INTO user_branch_assignments (user_id, branch_id) VALUES
    ('aaaaaaaa-0000-0000-0000-000000000002', 'bbbbbbbb-0000-0000-0000-000000000001')
ON CONFLICT DO NOTHING;

-- ── content_moderator demo user ───────────────────────────────────────────────
INSERT INTO users (id, username, email, password_hash) VALUES
    ('aaaaaaaa-0000-0000-0000-000000000003',
     'mod1',
     'mod1@lms.local',
     '$2a$12$2HLVOz4Gc4KPcx3EjUjnxu7xfsFwpfUiNihMq7lTyq2ks3Yw2E2e6')
ON CONFLICT DO NOTHING;

INSERT INTO user_roles (user_id, role_id)
SELECT 'aaaaaaaa-0000-0000-0000-000000000003', id FROM roles WHERE name = 'content_moderator'
ON CONFLICT DO NOTHING;

INSERT INTO user_branch_assignments (user_id, branch_id) VALUES
    ('aaaaaaaa-0000-0000-0000-000000000003', 'bbbbbbbb-0000-0000-0000-000000000001')
ON CONFLICT DO NOTHING;
