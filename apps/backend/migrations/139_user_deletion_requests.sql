-- Migration 139: a user can ask for the deletion of their own account.
--
-- POST /api/v1/auth/delete-account deactivates the signed-in user's account and
-- records in users.deletion_requested_at when the deletion was asked for. The
-- account deletion purge, which the server runs at start and every hour, deletes
-- each account whose request is 29 days old or older and that is still
-- deactivated. It deletes with DELETE FROM users, as the admin's permanent delete
-- does, so the ON DELETE rules of migrations 135 and 136 and the AFTER DELETE
-- triggers of migrations 137 and 138 apply to it.
--
-- A request stands only while the account stays deactivated: a BEFORE INSERT OR
-- UPDATE trigger writes NULL to deletion_requested_at when status is anything but
-- 'deactivated'. An admin who activates the account (POST
-- /api/v1/admin/users/:id/activate sets status to 'active') withdraws the request,
-- and a later deactivation does not bring it back.
--
-- A partial index holds the rows with a request, the only rows the purge reads.
--
-- Re-running this migration keeps the column and the index, and replaces the
-- function and the trigger.

ALTER TABLE users ADD COLUMN IF NOT EXISTS deletion_requested_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_users_deletion_requested_at
    ON users (deletion_requested_at)
    WHERE deletion_requested_at IS NOT NULL;

CREATE OR REPLACE FUNCTION user_deletion_request_follows_status()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.deletion_requested_at IS NOT NULL AND NEW.status IS DISTINCT FROM 'deactivated' THEN
        NEW.deletion_requested_at := NULL;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_user_deletion_request_follows_status ON users;

CREATE TRIGGER trg_user_deletion_request_follows_status
    BEFORE INSERT OR UPDATE OF status, deletion_requested_at ON users
    FOR EACH ROW
    EXECUTE FUNCTION user_deletion_request_follows_status();
