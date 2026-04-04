-- Migration 016: Add step-up timestamp to sessions.
--
-- stepup_at records the last time a user successfully re-authenticated via
-- POST /auth/stepup. The reveal-sensitive endpoint requires stepup_at to be
-- non-null and within the last 15 minutes before decrypting sensitive fields.
SET search_path = lms, public;

ALTER TABLE sessions ADD COLUMN stepup_at TIMESTAMPTZ;
