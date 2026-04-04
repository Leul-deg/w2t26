SET search_path = lms, public;

-- Restore privileges before dropping so the table can be dropped cleanly.
GRANT UPDATE, DELETE ON audit_events TO lms_user;
DROP TRIGGER IF EXISTS trg_forbid_audit_update ON audit_events;
DROP TRIGGER IF EXISTS trg_forbid_audit_delete ON audit_events;
DROP FUNCTION IF EXISTS forbid_audit_mutation();
DROP TABLE IF EXISTS audit_events;
