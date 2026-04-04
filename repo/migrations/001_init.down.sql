-- Migration 001 DOWN: Remove foundation objects.
-- WARNING: Dropping the lms schema removes ALL application tables.
-- Only run this on a development database. It is irreversible without a backup.

DROP SCHEMA IF EXISTS lms CASCADE;

-- Extensions are shared across the database and are intentionally not dropped
-- here, as other schemas may depend on them. To remove them manually:
--   DROP EXTENSION IF EXISTS "pgcrypto";
--   DROP EXTENSION IF EXISTS "uuid-ossp";
