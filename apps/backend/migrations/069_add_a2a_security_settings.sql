-- A2A Security Settings
-- Migration 068: Enforcement modes for A2A policy violations (monitor/strict)

-- ============================================================================
-- Add enforcement mode to existing policies
-- ============================================================================
ALTER TABLE a2a_policies ADD COLUMN IF NOT EXISTS enforcement_mode VARCHAR(20) DEFAULT 'monitor';
-- Values: 'monitor' (log violations), 'strict' (block violations)

-- ============================================================================
-- A2A Security Settings per organization
-- Global A2A security configuration for an organization
-- ============================================================================
CREATE TABLE IF NOT EXISTS a2a_security_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    -- Global enforcement mode for this organization
    enforcement_mode VARCHAR(20) NOT NULL DEFAULT 'monitor',
    -- 'monitor' - Log violations but allow requests
    -- 'strict'  - Block requests that violate policy

    -- Trust requirements
    require_verified_skills BOOLEAN DEFAULT FALSE,  -- Only allow verified skills
    min_trust_score FLOAT DEFAULT 0.0,              -- Minimum trust score for A2A
    min_attestation_count INT DEFAULT 0,            -- Minimum attestations for skills

    -- Request signing
    require_request_signing BOOLEAN DEFAULT TRUE,   -- Require Ed25519 signatures
    require_nonce_validation BOOLEAN DEFAULT TRUE,  -- Require nonce for anti-replay
    nonce_validity_seconds INT DEFAULT 300,         -- Nonce validity window

    -- Agent restrictions
    allowed_agent_ids JSONB DEFAULT '[]'::jsonb,    -- Empty = all agents allowed
    blocked_agent_ids JSONB DEFAULT '[]'::jsonb,    -- Blocked agents (takes priority)

    -- Rate limiting
    max_requests_per_minute INT DEFAULT 60,
    max_requests_per_hour INT DEFAULT 1000,

    -- Audit
    created_by UUID REFERENCES users(id),
    updated_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),

    CONSTRAINT unique_org_security_settings UNIQUE (organization_id)
);

CREATE INDEX IF NOT EXISTS idx_a2a_security_settings_org ON a2a_security_settings(organization_id);

-- ============================================================================
-- A2A Security Violations Log
-- Audit trail for policy violations (for monitor mode and strict mode rejections)
-- ============================================================================
CREATE TABLE IF NOT EXISTS a2a_security_violations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL,

    -- Request details
    requesting_agent_id UUID REFERENCES agents(id) ON DELETE SET NULL,
    target_agent_id UUID REFERENCES agents(id) ON DELETE SET NULL,
    skill_id VARCHAR(100),
    task_id UUID REFERENCES a2a_tasks(id) ON DELETE SET NULL,

    -- Violation details
    violation_type VARCHAR(50) NOT NULL,
    -- Types: 'LOW_TRUST', 'UNVERIFIED_SKILL', 'UNSIGNED_REQUEST', 'NONCE_INVALID',
    --        'AGENT_BLOCKED', 'RATE_LIMITED', 'POLICY_DENIED', 'REVOKED_AGENT'

    violation_details JSONB,  -- Additional context about the violation
    policy_id UUID REFERENCES a2a_policies(id) ON DELETE SET NULL,  -- Policy that triggered (if any)

    -- Action taken
    action_taken VARCHAR(20) NOT NULL,
    -- 'LOGGED' (monitor mode), 'BLOCKED' (strict mode)

    -- Request context
    request_signature TEXT,
    request_nonce VARCHAR(64),
    request_timestamp TIMESTAMPTZ,

    -- Metadata
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_a2a_violations_org ON a2a_security_violations(organization_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_a2a_violations_type ON a2a_security_violations(violation_type, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_a2a_violations_agent ON a2a_security_violations(requesting_agent_id, created_at DESC);

-- Trigger for updated_at
DROP TRIGGER IF EXISTS trigger_a2a_security_settings_updated_at ON a2a_security_settings;
CREATE TRIGGER trigger_a2a_security_settings_updated_at
    BEFORE UPDATE ON a2a_security_settings
    FOR EACH ROW EXECUTE FUNCTION update_a2a_updated_at();

-- ============================================================================
-- Comments
-- ============================================================================
COMMENT ON TABLE a2a_security_settings IS 'Organization-level A2A security configuration (enforcement mode, trust requirements)';
COMMENT ON TABLE a2a_security_violations IS 'Audit log of A2A policy violations for compliance and monitoring';
COMMENT ON COLUMN a2a_security_settings.enforcement_mode IS 'monitor = log violations, strict = block violations';
COMMENT ON COLUMN a2a_policies.enforcement_mode IS 'Per-policy enforcement mode override';
