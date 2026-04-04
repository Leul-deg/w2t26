SET search_path = lms, public;

DROP TABLE IF EXISTS readers;
-- reader_statuses has seeded rows; truncate before drop if needed
DROP TABLE IF EXISTS reader_statuses;
