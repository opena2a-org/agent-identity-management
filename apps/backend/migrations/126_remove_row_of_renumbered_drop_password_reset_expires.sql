-- Migration 126: remove the schema_migrations row of the migration renumbered to 125.
--
-- 125_drop_unread_password_reset_expires.sql was first released as
-- 112_drop_unread_password_reset_expires.sql, beside
-- 112_trust_score_low_threshold_key_and_scale.sql. Both runners (cmd/server at start and
-- cmd/migrate) key a migration by its full file name, so a database migrated before the
-- rename applies 125 once more, where both of its statements are IF EXISTS and change
-- nothing, and keeps a row for a file this repository no longer ships.
--
-- This deletes that one row by its exact name. The row of
-- 112_trust_score_low_threshold_key_and_scale.sql, and every other row, is untouched. On a
-- database that never recorded the old name it deletes nothing.

DELETE FROM schema_migrations WHERE version = '112_drop_unread_password_reset_expires.sql';
