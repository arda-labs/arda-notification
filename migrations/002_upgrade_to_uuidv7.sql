-- Migration: 002_upgrade_to_uuidv7.sql
-- PostgreSQL 18: Use native uuidv7() for time-ordered UUID generation.
--
-- Benefits:
--   1. UUIDs are sorted by creation time → much better B-tree index locality
--   2. Reduces index fragmentation for high-insert tables
--   3. Still globally unique (random component)
--   4. Existing UUIDv4 rows remain valid (backward compatible)
-- Change default for new rows to use PG18's native uuidv7()
ALTER TABLE notifications
ALTER COLUMN id
SET DEFAULT uuidv7 ();