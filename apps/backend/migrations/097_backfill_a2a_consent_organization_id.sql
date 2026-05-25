-- Migration 097: Backfill a2a_consent_records.organization_id for
-- rows written before PR #149's RecordConsent fix, then add a
-- NOT NULL constraint so future writes are guaranteed org-scoped.
--
-- Background (audit doc § 50):
--
-- PR #149 added `WHERE user_id = $1 AND organization_id = $2` to
-- A2AConsentRepository.ListByUser to close the cross-tenant
-- userId enumeration IDOR (audit #42). The companion fix to
-- RecordConsent now stamps caller orgID on every new write, so
-- future rows are always org-scoped. But existing NULL-org rows
-- from the pre-fix write path remain dark to the user's view
-- because the new WHERE clause filters them out.
--
-- This migration:
-- 1. Derives organization_id from the grantor agent (the agent that
--    granted the consent; its organization_id is the canonical owner
--    of the consent record per the cross-tenant model).
-- 2. Adds a NOT NULL constraint so the schema enforces what the
--    application now guarantees.
--
-- Closes #179.

-- Step 1: Backfill from grantor agent's organization.
UPDATE a2a_consent_records c
SET organization_id = a.organization_id,
    updated_at = NOW()
FROM agents a
WHERE c.grantor_agent_id = a.id
  AND c.organization_id IS NULL
  AND a.organization_id IS NOT NULL;

-- Step 2: Defensive cleanup. If any rows still have NULL
-- organization_id after backfill (e.g., grantor agent rows where
-- the agent itself has NULL organization_id, which should be
-- impossible per the agents schema but is checked defensively),
-- they are orphaned and the NOT NULL constraint below would
-- otherwise fail to add. Delete them — they are unrecoverable
-- consent records that no tenant can claim, and per audit doc
-- they have been invisible to all tenants since PR #149 anyway.
DELETE FROM a2a_consent_records
WHERE organization_id IS NULL;

-- Step 3: Enforce the invariant. RecordConsent now always sets
-- organization_id; this constraint guarantees the schema agrees.
ALTER TABLE a2a_consent_records
    ALTER COLUMN organization_id SET NOT NULL;

-- Step 4: The original partial index
-- `idx_a2a_consent_org ON a2a_consent_records(organization_id)
-- WHERE organization_id IS NOT NULL` was correct under the
-- nullable schema. Now that the column is NOT NULL, the WHERE
-- predicate is redundant. Replace with a full index so PostgreSQL
-- can use it for the org-scoped read path that ListByUser now
-- requires.
DROP INDEX IF EXISTS idx_a2a_consent_org;
CREATE INDEX IF NOT EXISTS idx_a2a_consent_org ON a2a_consent_records(organization_id);
