-- Migration: Add Post-Quantum Cryptography (PQC) support
-- This migration adds fields needed for ML-DSA (FIPS 204) and hybrid Ed25519+ML-DSA signatures

-- Add PQC fields to agents table
ALTER TABLE agents ADD COLUMN IF NOT EXISTS pqc_public_key TEXT;
ALTER TABLE agents ADD COLUMN IF NOT EXISTS pqc_key_algorithm VARCHAR(50);
ALTER TABLE agents ADD COLUMN IF NOT EXISTS hybrid_mode_enabled BOOLEAN DEFAULT FALSE;
ALTER TABLE agents ADD COLUMN IF NOT EXISTS pqc_key_created_at TIMESTAMP;
ALTER TABLE agents ADD COLUMN IF NOT EXISTS pqc_key_expires_at TIMESTAMP;
ALTER TABLE agents ADD COLUMN IF NOT EXISTS previous_pqc_public_key TEXT;

-- Add default PQC algorithm preference to organizations
ALTER TABLE organizations ADD COLUMN IF NOT EXISTS default_pqc_algorithm VARCHAR(50) DEFAULT 'ML-DSA-65';
ALTER TABLE organizations ADD COLUMN IF NOT EXISTS require_pqc BOOLEAN DEFAULT FALSE;

-- Index for querying agents by PQC status
CREATE INDEX IF NOT EXISTS idx_agents_hybrid_mode ON agents(hybrid_mode_enabled) WHERE hybrid_mode_enabled = TRUE;
CREATE INDEX IF NOT EXISTS idx_agents_pqc_algorithm ON agents(pqc_key_algorithm) WHERE pqc_key_algorithm IS NOT NULL;

-- Add comment for documentation
COMMENT ON COLUMN agents.pqc_public_key IS 'ML-DSA public key (base64 encoded). Used for post-quantum signatures.';
COMMENT ON COLUMN agents.pqc_key_algorithm IS 'ML-DSA variant: ML-DSA-44, ML-DSA-65, or ML-DSA-87';
COMMENT ON COLUMN agents.hybrid_mode_enabled IS 'When true, requires BOTH Ed25519 and ML-DSA signatures for authentication';
COMMENT ON COLUMN agents.pqc_key_created_at IS 'When the PQC key pair was generated';
COMMENT ON COLUMN agents.pqc_key_expires_at IS 'Optional expiration time for PQC key';
COMMENT ON COLUMN agents.previous_pqc_public_key IS 'Previous PQC public key during rotation grace period';
COMMENT ON COLUMN organizations.default_pqc_algorithm IS 'Default ML-DSA variant for new agents: ML-DSA-44, ML-DSA-65, ML-DSA-87';
COMMENT ON COLUMN organizations.require_pqc IS 'When true, new agents must register with PQC keys';
