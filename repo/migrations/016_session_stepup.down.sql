SET search_path = lms, public;

ALTER TABLE sessions DROP COLUMN IF EXISTS stepup_at;
