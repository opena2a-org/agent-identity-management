-- Migration: Make agent_id nullable in mcp_attestations for manual attestations
-- Created: 2025-11-09
-- Purpose: Allow manual attestations from users who don't have an agent

-- Drop the unique constraint that includes agent_id
ALTER TABLE mcp_attestations DROP CONSTRAINT IF EXISTS unique_attestation_per_verification;

-- Drop the foreign key constraint
ALTER TABLE mcp_attestations DROP CONSTRAINT IF EXISTS mcp_attestations_agent_id_fkey;

-- Make agent_id nullable
ALTER TABLE mcp_attestations ALTER COLUMN agent_id DROP NOT NULL;

-- Re-add the foreign key constraint with NULL allowed
ALTER TABLE mcp_attestations
    ADD CONSTRAINT mcp_attestations_agent_id_fkey
    FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE;

-- Re-add the unique constraint (now allowing NULL agent_id for manual attestations)
-- For manual attestations, agent_id will be NULL and the constraint will only check mcp_server_id and verified_at
ALTER TABLE mcp_attestations
    ADD CONSTRAINT unique_attestation_per_verification
    UNIQUE NULLS NOT DISTINCT (mcp_server_id, agent_id, verified_at);

-- Add comment
COMMENT ON COLUMN mcp_attestations.agent_id IS 'Agent that created this attestation (NULL for manual attestations by users)';
