SET search_path = lms, public;

INSERT INTO permissions (id, name, description)
VALUES
  ('22220008-0000-0000-0000-000000000003', 'feedback:submit', 'Submit reader feedback'),
  ('22220009-0000-0000-0000-000000000003', 'appeals:submit', 'Submit reader appeals')
ON CONFLICT (id) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT role_id, permission_id
FROM (
  VALUES
    ('11111111-0000-0000-0000-000000000001', 'feedback:submit'),
    ('11111111-0000-0000-0000-000000000001', 'appeals:submit'),
    ('11111111-0000-0000-0000-000000000002', 'feedback:submit'),
    ('11111111-0000-0000-0000-000000000002', 'appeals:submit'),
    ('11111111-0000-0000-0000-000000000003', 'feedback:submit'),
    ('11111111-0000-0000-0000-000000000003', 'appeals:submit')
) AS grants(role_id, permission_name)
JOIN permissions p ON p.name = grants.permission_name
ON CONFLICT DO NOTHING;
