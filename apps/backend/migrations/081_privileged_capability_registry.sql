-- Migration 081: Privileged Capability Registry (PAM Tier 2)
-- Classifies capabilities into STANDARD/PRIVILEGED/SUPER_PRIVILEGED tiers.

-- Add capability tier to FGA policies
ALTER TABLE fga_policies ADD COLUMN IF NOT EXISTS
    capability_tier TEXT NOT NULL DEFAULT 'STANDARD'
    CHECK (capability_tier IN ('STANDARD', 'PRIVILEGED', 'SUPER_PRIVILEGED'));

-- Org-level capability tier configuration
CREATE TABLE IF NOT EXISTS privileged_capability_registry (
    capability TEXT PRIMARY KEY,
    tier TEXT NOT NULL CHECK (tier IN ('STANDARD', 'PRIVILEGED', 'SUPER_PRIVILEGED')),
    tier_reason TEXT NOT NULL,
    max_ttl_seconds INT NOT NULL DEFAULT 14400,  -- 4h for STANDARD
    requires_mfa BOOLEAN NOT NULL DEFAULT FALSE,
    requires_ticket BOOLEAN NOT NULL DEFAULT FALSE,
    renewal_allowed BOOLEAN NOT NULL DEFAULT TRUE,
    created_by TEXT NOT NULL DEFAULT 'system',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed default tier classifications
INSERT INTO privileged_capability_registry (capability, tier, tier_reason, max_ttl_seconds, requires_mfa, renewal_allowed)
VALUES
    ('db.read', 'STANDARD', 'Non-PII read access', 14400, FALSE, TRUE),
    ('api.call', 'STANDARD', 'Public API calls', 14400, FALSE, TRUE),
    ('file.read', 'STANDARD', 'Non-sensitive file access', 14400, FALSE, TRUE),
    ('memory.write', 'STANDARD', 'Own namespace writes', 14400, FALSE, TRUE),
    ('db.read:pii', 'PRIVILEGED', 'PII field access', 1800, FALSE, TRUE),
    ('db.write', 'PRIVILEGED', 'Database write access', 1800, FALSE, TRUE),
    ('api.call:external', 'PRIVILEGED', 'External API calls', 1800, FALSE, TRUE),
    ('model.invoke', 'PRIVILEGED', 'LLM model invocation', 1800, FALSE, TRUE),
    ('infra.modify', 'SUPER_PRIVILEGED', 'Production infrastructure changes', 900, TRUE, FALSE),
    ('db.delete', 'SUPER_PRIVILEGED', 'Database delete operations', 900, TRUE, FALSE),
    ('db.write:bulk', 'SUPER_PRIVILEGED', 'Bulk write operations', 900, TRUE, FALSE),
    ('credential.rotate', 'SUPER_PRIVILEGED', 'Credential rotation', 900, TRUE, FALSE)
ON CONFLICT (capability) DO NOTHING;
