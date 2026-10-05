-- Migration 113: a deleted webhook no longer holds on to its name.
--
-- Migration 020 made webhook names unique per organization with a plain
-- UNIQUE (organization_id, name) constraint. Migration 064 then made deletion
-- a soft delete (deleted_at), and every read path filters deleted rows out.
-- The constraint still counted them, so once a webhook was deleted its name
-- could never be used again in that organization: creating a webhook with it,
-- or renaming another webhook to it, failed with a duplicate-key error.
--
-- This replaces the constraint with a unique index over live rows only. Two
-- webhooks that are not deleted still cannot share a name in one organization.
-- The index keeps the constraint's name, so a unique violation reports the
-- same name as before.
--
-- Safe on existing data: the old constraint already forbade any duplicate,
-- so the narrower index cannot find one. Idempotent: the constraint is dropped
-- only if present and the index is created only if absent.

ALTER TABLE webhooks DROP CONSTRAINT IF EXISTS webhooks_name_unique_per_org;

CREATE UNIQUE INDEX IF NOT EXISTS webhooks_name_unique_per_org
    ON webhooks (organization_id, name)
    WHERE deleted_at IS NULL;

COMMENT ON INDEX webhooks_name_unique_per_org IS
    'Webhook names are unique per organization among webhooks that are not deleted';
