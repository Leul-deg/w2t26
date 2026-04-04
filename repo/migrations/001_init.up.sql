-- Migration 001: Foundation
-- Enables required PostgreSQL extensions and creates the application schema
-- container. All application tables are created in subsequent migrations.

-- uuid_generate_v4() is used for primary keys throughout the application.
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- pgcrypto is used for gen_random_bytes() as an additional entropy source.
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- The lms schema groups all application tables, keeping the public schema clean.
-- All subsequent migrations create objects in the lms schema.
CREATE SCHEMA IF NOT EXISTS lms;

-- Set the search path so subsequent migrations and queries default to lms.
-- This is a session-level setting and must be applied in each connection or
-- set as the default for the database user:
--   ALTER ROLE lms_user SET search_path = lms, public;
