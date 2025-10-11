-- Migration rollback: Remove auto-granted capabilities
-- Date: 2025-10-11
-- Purpose: Rollback auto-grant migration

-- Remove only auto-granted capabilities (those created by migration)
-- We identify them by:
-- 1. granted_by = agent's created_by (auto-granted by creator)
-- 2. created_at close to migration time
-- 3. Empty capability_scope (migration sets {})

DELETE FROM agent_capabilities
WHERE capability_scope = '{}'::jsonb
  AND granted_by IN (
      SELECT created_by FROM agents WHERE id = agent_capabilities.agent_id
  );

-- Remove comments
COMMENT ON COLUMN agents.capabilities IS NULL;
COMMENT ON TABLE agent_capabilities IS NULL;
