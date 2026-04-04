-- Migration 014: Seed data for reviewer use
--
-- Seed data provides a working starting state for local development and review.
-- It does NOT represent production data. Passwords are documented below.
--
-- Seeded credentials:
--   Username: admin       Password: Admin1234!  Role: administrator
--
-- Seeded branches: MAIN (Main Branch), EAST (East Branch)
--
-- To reset seed data: run migration 014 down, then 014 up again.
SET search_path = lms, public;

-- ── Roles ─────────────────────────────────────────────────────────────────────
INSERT INTO roles (id, name, description) VALUES
    ('11111111-0000-0000-0000-000000000001', 'administrator',    'Full system access across all branches'),
    ('11111111-0000-0000-0000-000000000002', 'operations_staff', 'Reader, holdings, and circulation management within assigned branch'),
    ('11111111-0000-0000-0000-000000000003', 'content_moderator','Content review and moderation within assigned branch');

-- ── Permissions ───────────────────────────────────────────────────────────────
INSERT INTO permissions (id, name, description) VALUES
    -- Readers
    ('22220001-0000-0000-0000-000000000001', 'readers:read',             'View reader profiles'),
    ('22220001-0000-0000-0000-000000000002', 'readers:write',            'Create and update reader profiles'),
    ('22220001-0000-0000-0000-000000000003', 'readers:delete',           'Deactivate reader profiles'),
    ('22220001-0000-0000-0000-000000000004', 'readers:reveal_sensitive', 'Reveal encrypted sensitive fields after step-up'),
    -- Holdings
    ('22220002-0000-0000-0000-000000000001', 'holdings:read',            'View holdings catalogue'),
    ('22220002-0000-0000-0000-000000000002', 'holdings:write',           'Create and update holdings'),
    -- Copies
    ('22220003-0000-0000-0000-000000000001', 'copies:read',              'View copy-level inventory'),
    ('22220003-0000-0000-0000-000000000002', 'copies:write',             'Create and update copy records'),
    -- Circulation
    ('22220004-0000-0000-0000-000000000001', 'circulation:read',         'View circulation history'),
    ('22220004-0000-0000-0000-000000000002', 'circulation:write',        'Record circulation events'),
    -- Stocktake
    ('22220005-0000-0000-0000-000000000001', 'stocktake:read',           'View stocktake sessions'),
    ('22220005-0000-0000-0000-000000000002', 'stocktake:write',          'Create and manage stocktake sessions'),
    -- Programs
    ('22220006-0000-0000-0000-000000000001', 'programs:read',            'View programs'),
    ('22220006-0000-0000-0000-000000000002', 'programs:write',           'Create and manage programs'),
    -- Enrollments
    ('22220007-0000-0000-0000-000000000001', 'enrollments:read',         'View enrollments'),
    ('22220007-0000-0000-0000-000000000002', 'enrollments:write',        'Enroll and drop readers'),
    ('22220007-0000-0000-0000-000000000003', 'enrollments:admin',        'Override waitlists and force enrollment changes'),
    -- Feedback
    ('22220008-0000-0000-0000-000000000001', 'feedback:read',            'View submitted feedback'),
    ('22220008-0000-0000-0000-000000000002', 'feedback:moderate',        'Approve or reject feedback'),
    -- Appeals
    ('22220009-0000-0000-0000-000000000001', 'appeals:read',             'View submitted appeals'),
    ('22220009-0000-0000-0000-000000000002', 'appeals:decide',           'Issue arbitration decisions on appeals'),
    -- Content
    ('22220010-0000-0000-0000-000000000001', 'content:read',             'View published content'),
    ('22220010-0000-0000-0000-000000000002', 'content:submit',           'Submit content for review'),
    ('22220010-0000-0000-0000-000000000003', 'content:moderate',         'Review and decide on content moderation items'),
    ('22220010-0000-0000-0000-000000000004', 'content:publish',          'Publish approved content'),
    -- Imports
    ('22220011-0000-0000-0000-000000000001', 'imports:create',           'Upload import files'),
    ('22220011-0000-0000-0000-000000000002', 'imports:preview',          'View import preview'),
    ('22220011-0000-0000-0000-000000000003', 'imports:commit',           'Commit or roll back an import'),
    -- Exports
    ('22220012-0000-0000-0000-000000000001', 'exports:create',           'Generate and download exports'),
    -- Reports
    ('22220013-0000-0000-0000-000000000001', 'reports:read',             'View and generate reports'),
    ('22220013-0000-0000-0000-000000000002', 'reports:export',           'Export report data'),
    -- Users
    ('22220014-0000-0000-0000-000000000001', 'users:read',               'View user accounts'),
    ('22220014-0000-0000-0000-000000000002', 'users:write',              'Create and update user accounts'),
    ('22220014-0000-0000-0000-000000000003', 'users:admin',              'Manage roles and branch assignments'),
    -- Audit
    ('22220015-0000-0000-0000-000000000001', 'audit:read',               'View audit event log');

-- ── Role → Permission mapping ─────────────────────────────────────────────────
-- Administrator: all permissions
INSERT INTO role_permissions (role_id, permission_id)
SELECT '11111111-0000-0000-0000-000000000001', id FROM permissions;

-- Operations staff: reader/holdings/copies/circulation/stocktake/programs/enrollments/reports
INSERT INTO role_permissions (role_id, permission_id)
SELECT '11111111-0000-0000-0000-000000000002', id FROM permissions
WHERE name IN (
    'readers:read', 'readers:write', 'readers:reveal_sensitive',
    'holdings:read', 'holdings:write',
    'copies:read', 'copies:write',
    'circulation:read', 'circulation:write',
    'stocktake:read', 'stocktake:write',
    'programs:read', 'programs:write',
    'enrollments:read', 'enrollments:write',
    'feedback:read',
    'appeals:read',
    'content:read',
    'imports:create', 'imports:preview', 'imports:commit',
    'exports:create',
    'reports:read', 'reports:export'
);

-- Content moderator: content/feedback/appeals moderation
INSERT INTO role_permissions (role_id, permission_id)
SELECT '11111111-0000-0000-0000-000000000003', id FROM permissions
WHERE name IN (
    'content:read', 'content:submit', 'content:moderate', 'content:publish',
    'feedback:read', 'feedback:moderate',
    'appeals:read', 'appeals:decide',
    'readers:read',
    'reports:read'
);

-- ── Seed admin user ───────────────────────────────────────────────────────────
-- Password: Admin1234!  (bcrypt cost 12, generated by golang.org/x/crypto/bcrypt)
-- CHANGE THIS PASSWORD on first login in any non-development environment.
INSERT INTO users (id, username, email, password_hash) VALUES
    ('aaaaaaaa-0000-0000-0000-000000000001',
     'admin',
     'admin@lms.local',
     '$2a$12$o4LKznEUDa1xVT352VPBye52YqJw763uyqdqAEgRNDmfZ6FoqRU4K');

-- Assign administrator role to the seed admin user
INSERT INTO user_roles (user_id, role_id) VALUES
    ('aaaaaaaa-0000-0000-0000-000000000001', '11111111-0000-0000-0000-000000000001');

-- ── Seed branches ─────────────────────────────────────────────────────────────
INSERT INTO branches (id, code, name, address) VALUES
    ('bbbbbbbb-0000-0000-0000-000000000001', 'MAIN', 'Main Branch',  '1 Library Street, Central District'),
    ('bbbbbbbb-0000-0000-0000-000000000002', 'EAST', 'East Branch',  '42 Reading Avenue, East Quarter');

-- Assign admin to both branches
INSERT INTO user_branch_assignments (user_id, branch_id) VALUES
    ('aaaaaaaa-0000-0000-0000-000000000001', 'bbbbbbbb-0000-0000-0000-000000000001'),
    ('aaaaaaaa-0000-0000-0000-000000000001', 'bbbbbbbb-0000-0000-0000-000000000002');

-- ── Seed report definitions (with metric alias mappings) ──────────────────────
INSERT INTO report_definitions (name, description, query_template, metric_aliases) VALUES
    (
        'program_utilization',
        'Program slot utilization by venue type and enrollment channel',
        'SELECT p.venue_type, p.enrollment_channel, p.capacity,
                COUNT(e.id) FILTER (WHERE e.status = ''confirmed'') AS confirmed_count,
                ROUND(COUNT(e.id) FILTER (WHERE e.status = ''confirmed'')::numeric / NULLIF(p.capacity, 0) * 100, 2) AS slot_utilization_rate
         FROM lms.programs p
         LEFT JOIN lms.enrollments e ON e.program_id = p.id
         WHERE p.branch_id = :branch_id
           AND p.starts_at >= :period_start AND p.starts_at < :period_end
         GROUP BY p.venue_type, p.enrollment_channel, p.capacity',
        '{
            "slot_utilization_rate": "occupancy_rate",
            "venue_type": "room_type",
            "enrollment_channel": "channel"
        }'
    ),
    (
        'enrollment_mix',
        'Enrollment distribution by program category (revenue mix analog)',
        'SELECT p.category,
                COUNT(e.id) AS enrollment_count,
                ROUND(COUNT(e.id)::numeric / NULLIF(SUM(COUNT(e.id)) OVER (), 0) * 100, 2) AS enrollment_mix_by_category
         FROM lms.programs p
         JOIN lms.enrollments e ON e.program_id = p.id
         WHERE p.branch_id = :branch_id
           AND p.starts_at >= :period_start AND p.starts_at < :period_end
         GROUP BY p.category',
        '{"enrollment_mix_by_category": "revenue_mix"}'
    );
