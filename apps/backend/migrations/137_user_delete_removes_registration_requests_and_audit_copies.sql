-- Migration 137: deleting a user deletes the registration requests filed with the
-- user's email, and the copies of the user's email and name in the audit rows about
-- the user.
--
-- A registration request holds the email and the first and last name of the person
-- who filed it, the password hash and signup answers when the registration gave
-- them, and no reference to the user its approval creates: the approval copies the
-- request's email into the new user, and the request stays, with status 'approved'.
-- Deleting the user left the request behind. Registration files a request with no
-- organization (organization_id NULL), so deleting the organization left it too.
--
-- Three audit rows an admin writes about a user copied the user's email into their
-- metadata as user_email: a role change (resource_type 'user_role', resource_id the
-- user), a registration approval (resource_type 'registration_approval', resource_id
-- the request, with user_name), and the admin's permanent delete (resource_type
-- 'user', resource_id the user, with user_name), which the server now writes without
-- either. These rows name the admin as their user, so the cascade that deletes the
-- user's own audit rows does not reach them.
--
-- An AFTER DELETE trigger on users now, for each deleted user:
--
-- - deletes the registration requests whose email is the user's, compared without
--   case, that hold the user's organization; and those that hold no organization
--   when no remaining user has that email. A request that holds another
--   organization is that organization's, and is kept.
-- - removes user_email and user_name from the metadata of the audit rows about the
--   user: the 'user' and 'user_role' rows of the user's organization whose
--   resource_id is the user, and the 'registration_approval' rows of the user's
--   organization whose user_email is the user's email. The rows are kept, as the
--   organization's record of what an admin did, and name the user by id only.
--
-- The trigger runs AFTER the delete, once the statement has deleted its rows, so an
-- organization delete, which deletes its users in one statement, checks for a
-- remaining user with every one of them already gone.
--
-- Once: the approved requests whose email no user has, compared without case, are
-- deleted. The 'user' and 'user_role' rows whose resource_id names no user lose
-- user_email and user_name, which takes them out of the permanent delete rows
-- written before this change; and so do the 'registration_approval' rows whose
-- user_email no user of the row's organization has.
--
-- Re-running this migration replaces the function and the trigger, and finds no row
-- left to delete or clear.

CREATE INDEX IF NOT EXISTS idx_user_registration_requests_lower_email
    ON user_registration_requests (lower(email));

CREATE OR REPLACE FUNCTION user_delete_removes_requests_and_audit_copies()
RETURNS TRIGGER AS $$
BEGIN
    DELETE FROM user_registration_requests r
    WHERE lower(r.email) = lower(OLD.email)
      AND (r.organization_id = OLD.organization_id
           OR (r.organization_id IS NULL
               AND NOT EXISTS (SELECT 1 FROM users u WHERE lower(u.email) = lower(OLD.email))));

    UPDATE audit_logs a
    SET metadata = a.metadata - 'user_email' - 'user_name'
    WHERE a.resource_type IN ('user', 'user_role')
      AND a.resource_id = OLD.id
      AND a.organization_id = OLD.organization_id
      AND jsonb_typeof(a.metadata) = 'object'
      AND (a.metadata ? 'user_email' OR a.metadata ? 'user_name');

    UPDATE audit_logs a
    SET metadata = a.metadata - 'user_email' - 'user_name'
    WHERE a.resource_type = 'registration_approval'
      AND a.organization_id = OLD.organization_id
      AND jsonb_typeof(a.metadata) = 'object'
      AND a.metadata ->> 'user_email' = OLD.email;

    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_user_delete_removes_requests_and_audit_copies ON users;

CREATE TRIGGER trg_user_delete_removes_requests_and_audit_copies
    AFTER DELETE ON users
    FOR EACH ROW
    EXECUTE FUNCTION user_delete_removes_requests_and_audit_copies();

-- Once: requests and audit copies left by users deleted before this migration.

DELETE FROM user_registration_requests r
WHERE r.status = 'approved'
  AND NOT EXISTS (SELECT 1 FROM users u WHERE lower(u.email) = lower(r.email));

UPDATE audit_logs a
SET metadata = a.metadata - 'user_email' - 'user_name'
WHERE a.resource_type IN ('user', 'user_role')
  AND jsonb_typeof(a.metadata) = 'object'
  AND (a.metadata ? 'user_email' OR a.metadata ? 'user_name')
  AND NOT EXISTS (SELECT 1 FROM users u WHERE u.id = a.resource_id);

UPDATE audit_logs a
SET metadata = a.metadata - 'user_email' - 'user_name'
WHERE a.resource_type = 'registration_approval'
  AND jsonb_typeof(a.metadata) = 'object'
  AND a.metadata ? 'user_email'
  AND NOT EXISTS (
      SELECT 1 FROM users u
      WHERE u.organization_id = a.organization_id
        AND u.email = a.metadata ->> 'user_email'
  );
