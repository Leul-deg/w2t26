-- Migration 018 rollback: remove demo users
SET search_path = lms, public;

DELETE FROM user_branch_assignments WHERE user_id IN (
    'aaaaaaaa-0000-0000-0000-000000000002',
    'aaaaaaaa-0000-0000-0000-000000000003'
);

DELETE FROM user_roles WHERE user_id IN (
    'aaaaaaaa-0000-0000-0000-000000000002',
    'aaaaaaaa-0000-0000-0000-000000000003'
);

-- Sessions must be removed before deactivating users.
DELETE FROM sessions WHERE user_id IN (
    'aaaaaaaa-0000-0000-0000-000000000002',
    'aaaaaaaa-0000-0000-0000-000000000003'
);

UPDATE users SET is_active = false WHERE id IN (
    'aaaaaaaa-0000-0000-0000-000000000002',
    'aaaaaaaa-0000-0000-0000-000000000003'
);
