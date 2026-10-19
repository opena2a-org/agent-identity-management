-- Migration 136: an agent or MCP server keeps no copy of the name and email of a
-- user it no longer names.
--
-- agents and mcp_servers copy the name and email of the user who created the row
-- into created_by_name and created_by_email (migrations 048 and 049), beside
-- created_by, and have updated_by, updated_by_name and updated_by_email for the user
-- who last changed the row. Migration 135 sets created_by to NULL when that user is
-- deleted, and the copies stayed: the agent and MCP server responses went on
-- returning the deleted user's name and email as createdByName and createdByEmail.
-- updated_by had no foreign key, so a value in it went on naming a user who no longer
-- existed.
--
-- The copies now go with the reference they copy. A BEFORE UPDATE trigger on each
-- table writes NULL to created_by_name and created_by_email when created_by is NULL,
-- and to updated_by_name and updated_by_email when updated_by is NULL. Deleting a
-- user sets created_by to NULL through ON DELETE SET NULL, an UPDATE of the row, so
-- the trigger runs in the same statement as the delete. updated_by gets the same
-- foreign key, after an id in it that no user has is set to NULL.
--
-- The rows that already lost their user, under migration 135, are cleared here once.
--
-- Re-running this migration replaces the function, the two triggers and the two
-- foreign keys, and finds no row left to clear.

CREATE OR REPLACE FUNCTION user_copies_follow_reference()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.created_by IS NULL THEN
        NEW.created_by_name := NULL;
        NEW.created_by_email := NULL;
    END IF;
    IF NEW.updated_by IS NULL THEN
        NEW.updated_by_name := NULL;
        NEW.updated_by_email := NULL;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_user_copies_follow_reference ON agents;

CREATE TRIGGER trg_user_copies_follow_reference
    BEFORE UPDATE ON agents
    FOR EACH ROW
    EXECUTE FUNCTION user_copies_follow_reference();

DROP TRIGGER IF EXISTS trg_user_copies_follow_reference ON mcp_servers;

CREATE TRIGGER trg_user_copies_follow_reference
    BEFORE UPDATE ON mcp_servers
    FOR EACH ROW
    EXECUTE FUNCTION user_copies_follow_reference();

-- Once: copies left without their reference, and an updated_by that names no user.

UPDATE agents
SET created_by_name = NULL,
    created_by_email = NULL
WHERE created_by IS NULL
  AND (created_by_name IS NOT NULL OR created_by_email IS NOT NULL);

UPDATE agents a
SET updated_by = NULL,
    updated_by_name = NULL,
    updated_by_email = NULL
WHERE (a.updated_by IS NULL AND (a.updated_by_name IS NOT NULL OR a.updated_by_email IS NOT NULL))
   OR (a.updated_by IS NOT NULL AND NOT EXISTS (SELECT 1 FROM users u WHERE u.id = a.updated_by));

UPDATE mcp_servers
SET created_by_name = NULL,
    created_by_email = NULL
WHERE created_by IS NULL
  AND (created_by_name IS NOT NULL OR created_by_email IS NOT NULL);

UPDATE mcp_servers m
SET updated_by = NULL,
    updated_by_name = NULL,
    updated_by_email = NULL
WHERE (m.updated_by IS NULL AND (m.updated_by_name IS NOT NULL OR m.updated_by_email IS NOT NULL))
   OR (m.updated_by IS NOT NULL AND NOT EXISTS (SELECT 1 FROM users u WHERE u.id = m.updated_by));

-- updated_by: set to NULL when its user is deleted, as created_by is.

ALTER TABLE agents DROP CONSTRAINT IF EXISTS agents_updated_by_fkey;

ALTER TABLE agents
    ADD CONSTRAINT agents_updated_by_fkey
        FOREIGN KEY (updated_by) REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE mcp_servers DROP CONSTRAINT IF EXISTS mcp_servers_updated_by_fkey;

ALTER TABLE mcp_servers
    ADD CONSTRAINT mcp_servers_updated_by_fkey
        FOREIGN KEY (updated_by) REFERENCES users(id) ON DELETE SET NULL;
