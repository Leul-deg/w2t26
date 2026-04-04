SET search_path = lms, public;

-- Restore privileges before dropping so the table can be dropped cleanly.
GRANT UPDATE, DELETE ON audit_events TO lms_user;
DROP TABLE IF EXISTS audit_events;
