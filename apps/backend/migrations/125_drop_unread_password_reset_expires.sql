-- Migration 125: drop users.password_reset_expires and its index.
--
-- Migration 002 adds users.password_reset_expires_at, the expiry the password reset flow
-- reads and writes (UserRepository: the reset token lookup filters on it, Update writes it).
-- Migration 011 later added a second column, users.password_reset_expires, with a partial
-- index idx_users_password_reset_expires. No code has ever read or written that column, so
-- on every migrated database it holds only NULLs, and the index covers no rows while still
-- sitting on the users table.
--
-- The column holds no value any code consumes, so dropping it loses nothing the reset flow
-- depends on. The live column, the reset token and idx_users_password_reset_token are
-- untouched.
--
-- Idempotent: both statements are IF EXISTS, so this is a no-op on a database that never
-- carried the column. The index is dropped first so the intent is explicit; DROP COLUMN
-- would remove it as well.

DROP INDEX IF EXISTS idx_users_password_reset_expires;

ALTER TABLE users DROP COLUMN IF EXISTS password_reset_expires;
