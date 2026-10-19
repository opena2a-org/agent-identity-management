-- Migration 138: deleting a user removes the user's email from the admin access
-- reviews the organization keeps as compliance evidence.
--
-- Collecting evidence for the admin_access_review check stores, in the data of a
-- compliance_evidence row, an entry under adminUsers for each user of the
-- organization whose role is admin: the user's id, email, last sign-in and status.
-- The evidence is the organization's, and deleting one of those users kept it, with
-- collected_by set to NULL by migration 135 when that user had collected it, and with
-- the user's email in the user's entry.
--
-- An AFTER DELETE trigger on users now, for each deleted user, removes email from the
-- entries whose id is the user's in the admin_access_review evidence of the user's
-- organization. The entry is kept, with the id, last sign-in and status it was
-- collected with: as with the audit rows of migration 137, the record stays and names
-- the user by id only.
--
-- Once: email is removed from the entries whose id names no user, the entries of
-- users deleted before this migration.
--
-- Re-running this migration replaces the function and the trigger, and finds no entry
-- left to clear.

CREATE OR REPLACE FUNCTION user_delete_removes_access_review_emails()
RETURNS TRIGGER AS $$
BEGIN
    -- The containment test holds only for an adminUsers array with an entry whose id
    -- is the user's, so jsonb_array_elements reads an array; ->> 'id' is NULL for an
    -- element that is not an object, so - 'email' only reaches entries.
    UPDATE compliance_evidence e
    SET data = jsonb_set(e.data, '{adminUsers}', (
            SELECT jsonb_agg(CASE WHEN x.elem ->> 'id' = OLD.id::text
                                  THEN x.elem - 'email'
                                  ELSE x.elem
                             END ORDER BY x.ord)
            FROM jsonb_array_elements(e.data -> 'adminUsers') WITH ORDINALITY AS x(elem, ord)))
    WHERE e.organization_id = OLD.organization_id
      AND e.check_name = 'admin_access_review'
      AND e.data -> 'adminUsers' @> jsonb_build_array(jsonb_build_object('id', OLD.id::text));

    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_user_delete_removes_access_review_emails ON users;

CREATE TRIGGER trg_user_delete_removes_access_review_emails
    AFTER DELETE ON users
    FOR EACH ROW
    EXECUTE FUNCTION user_delete_removes_access_review_emails();

-- Once: the emails left in the entries of users deleted before this migration. An
-- entry is cleared when its id is a string that names no user and it holds an email;
-- a review is rewritten only when it has such an entry.

WITH cleared AS (
    SELECT e.id,
           jsonb_agg(CASE WHEN x.gone THEN x.elem - 'email' ELSE x.elem END ORDER BY x.ord) AS admin_users
    FROM compliance_evidence e
    CROSS JOIN LATERAL (
        SELECT a.elem,
               a.ord,
               jsonb_typeof(a.elem -> 'id') = 'string'
                   AND a.elem ? 'email'
                   AND a.elem ->> 'id' NOT IN (SELECT u.id::text FROM users u) AS gone
        FROM jsonb_array_elements(CASE WHEN jsonb_typeof(e.data -> 'adminUsers') = 'array'
                                       THEN e.data -> 'adminUsers'
                                  END) WITH ORDINALITY AS a(elem, ord)
    ) x
    WHERE e.check_name = 'admin_access_review'
    GROUP BY e.id
    HAVING bool_or(x.gone)
)
UPDATE compliance_evidence e
SET data = jsonb_set(e.data, '{adminUsers}', cleared.admin_users)
FROM cleared
WHERE e.id = cleared.id;
