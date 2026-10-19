-- Migration 134: users.password_changed_at, the instant the user's password last changed.
--
-- AuthService.ChangePassword (the authenticated change and the change at first sign-in)
-- and RegistrationService.ResetPassword (a reset with the mailed token) write it. Every
-- request a login or SDK token authenticates reads it with force_password_change
-- (UserRepository.GetSessionState), and a token whose sign-in time is before the first
-- whole second after it is refused, so a password change ends every session signed in
-- before it. UserRepository.Update only moves it forward: a write that carries NULL or an
-- older instant leaves the recorded one in place.
--
-- NULL until the user's password first changes after this migration: no row is backfilled,
-- and a token of an account whose column is NULL is not refused by it.
--
-- Idempotent: ADD COLUMN IF NOT EXISTS. The column has no default.

ALTER TABLE users ADD COLUMN IF NOT EXISTS password_changed_at TIMESTAMPTZ;

COMMENT ON COLUMN users.password_changed_at IS
    'Instant the password last changed (NULL until it first changes); a token whose sign-in time is before the first whole second after it is refused';
