-- Migration 014 DOWN: Remove seed data only.
-- Schema tables are not dropped; only seeded rows are deleted.
SET search_path = lms, public;

DELETE FROM report_definitions WHERE name IN ('program_utilization', 'enrollment_mix');
DELETE FROM user_branch_assignments WHERE user_id = 'aaaaaaaa-0000-0000-0000-000000000001';
DELETE FROM branches WHERE id IN (
    'bbbbbbbb-0000-0000-0000-000000000001',
    'bbbbbbbb-0000-0000-0000-000000000002'
);
DELETE FROM user_roles  WHERE user_id = 'aaaaaaaa-0000-0000-0000-000000000001';
DELETE FROM users       WHERE id      = 'aaaaaaaa-0000-0000-0000-000000000001';
DELETE FROM role_permissions;
DELETE FROM permissions;
DELETE FROM roles WHERE id IN (
    '11111111-0000-0000-0000-000000000001',
    '11111111-0000-0000-0000-000000000002',
    '11111111-0000-0000-0000-000000000003'
);
