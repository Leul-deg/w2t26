SET search_path = lms, public;

DELETE FROM role_permissions
WHERE permission_id IN (
  SELECT id FROM permissions WHERE name IN ('feedback:submit', 'appeals:submit')
);

DELETE FROM permissions
WHERE name IN ('feedback:submit', 'appeals:submit');
