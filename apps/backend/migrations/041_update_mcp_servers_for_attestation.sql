-- Migration: Update mcp_servers table for Agent Attestation model
-- Created: 2025-10-23
-- Purpose: Add attestation-related columns to mcp_servers table

-- Add new columns for attestation-based verification
ALTER TABLE mcp_servers
    ADD COLUMN IF NOT EXISTS verification_method VARCHAR(50) DEFAULT 'agent_attestation',
    ADD COLUMN IF NOT EXISTS attestation_count INT DEFAULT 0,
    ADD COLUMN IF NOT EXISTS confidence_score DECIMAL(5,2) DEFAULT 0.00,
    ADD COLUMN IF NOT EXISTS last_attested_at TIMESTAMPTZ;

-- Add constraint for confidence_score (0-100) - using DO block for idempotency
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'mcp_servers_confidence_score_check'
    ) THEN
        ALTER TABLE mcp_servers
            ADD CONSTRAINT mcp_servers_confidence_score_check
            CHECK (confidence_score >= 0 AND confidence_score <= 100);
    END IF;
END $$;

-- Remove NOT NULL constraint from public_key (not needed for attestation model)
-- MCP servers don't need Ed25519 keys - verified agents attest instead!
DO $$
BEGIN
    -- Only drop NOT NULL if column exists and is NOT NULL
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'mcp_servers'
          AND column_name = 'public_key'
          AND is_nullable = 'NO'
    ) THEN
        ALTER TABLE mcp_servers ALTER COLUMN public_key DROP NOT NULL;
    END IF;
END $$;

-- Create index for confidence score queries
CREATE INDEX IF NOT EXISTS idx_mcp_servers_confidence_score ON mcp_servers(confidence_score DESC);
CREATE INDEX IF NOT EXISTS idx_mcp_servers_verification_method ON mcp_servers(verification_method);
CREATE INDEX IF NOT EXISTS idx_mcp_servers_last_attested_at ON mcp_servers(last_attested_at DESC);

-- Add comments
COMMENT ON COLUMN mcp_servers.verification_method IS 'Method used to verify MCP: agent_attestation, api_key, or manual';
COMMENT ON COLUMN mcp_servers.attestation_count IS 'Number of verified agent attestations for this MCP';
COMMENT ON COLUMN mcp_servers.confidence_score IS 'Calculated confidence score (0-100) based on agent attestations';
COMMENT ON COLUMN mcp_servers.last_attested_at IS 'Timestamp of most recent attestation from any agent';
