-- Migration: Auto-grant declared capabilities for existing agents
-- Date: 2025-10-11
-- Purpose: Backward compatibility - convert agent.capabilities to agent_capabilities records
--
-- This migration implements the new capability architecture:
-- - agent.capabilities (JSONB array) = DECLARED capabilities (reference only)
-- - agent_capabilities (table) = GRANTED capabilities (enforcement source of truth)
--
-- For existing agents with declared capabilities but no granted capabilities,
-- we auto-grant their declared capabilities to maintain backward compatibility.

-- Auto-grant declared capabilities for existing agents
INSERT INTO agent_capabilities (
    id,
    agent_id,
    capability_type,
    capability_scope,
    granted_by,
    granted_at,
    revoked_at,
    created_at,
    updated_at
)
SELECT
    gen_random_uuid(),                              -- Generate new UUID
    a.id,                                           -- Agent ID
    jsonb_array_elements_text(a.capabilities),      -- Each capability from array
    '{}'::jsonb,                                     -- Empty scope (no restrictions)
    a.created_by,                                   -- Auto-grant by agent creator
    NOW(),                                          -- Granted now
    NULL,                                           -- Not revoked
    NOW(),                                          -- Created now
    NOW()                                           -- Updated now
FROM agents a
WHERE a.capabilities IS NOT NULL
  AND jsonb_array_length(a.capabilities) > 0        -- Has declared capabilities
  AND a.status = 'verified'                         -- Only verified agents
  AND NOT EXISTS (
      -- Skip agents that already have granted capabilities
      SELECT 1 FROM agent_capabilities ac
      WHERE ac.agent_id = a.id
      AND ac.revoked_at IS NULL
  );

-- Log migration results
DO $$
DECLARE
    granted_count INTEGER;
    agent_count INTEGER;
BEGIN
    -- Count how many capabilities were granted
    SELECT COUNT(*) INTO granted_count
    FROM agent_capabilities
    WHERE created_at >= NOW() - INTERVAL '1 minute';

    -- Count how many agents were affected
    SELECT COUNT(DISTINCT agent_id) INTO agent_count
    FROM agent_capabilities
    WHERE created_at >= NOW() - INTERVAL '1 minute';

    RAISE NOTICE '✅ Auto-granted % capabilities for % existing agents', granted_count, agent_count;
END $$;

-- Add helpful comment to capabilities column
COMMENT ON COLUMN agents.capabilities IS
'DECLARED capabilities (reference only). Enforcement uses agent_capabilities table. See CAPABILITY_ARCHITECTURE.md for details.';

COMMENT ON TABLE agent_capabilities IS
'GRANTED capabilities (enforcement source of truth). Only capabilities in this table are enforced by VerifyAction.';
