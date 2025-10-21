-- Migration: Create security_policies table
-- Created: 2025-10-20
-- Purpose: Add security policies table for configurable security rules

CREATE TABLE IF NOT EXISTS security_policies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    policy_type VARCHAR(50) NOT NULL,
    enforcement_action VARCHAR(50) NOT NULL,
    severity_threshold VARCHAR(50) NOT NULL,
    rules JSONB NOT NULL DEFAULT '{}'::jsonb,
    applies_to TEXT NOT NULL DEFAULT 'all',
    is_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    priority INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE
);

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_security_policies_organization_id ON security_policies(organization_id);
CREATE INDEX IF NOT EXISTS idx_security_policies_policy_type ON security_policies(policy_type);
CREATE INDEX IF NOT EXISTS idx_security_policies_is_enabled ON security_policies(is_enabled);
CREATE INDEX IF NOT EXISTS idx_security_policies_priority ON security_policies(priority DESC);
CREATE INDEX IF NOT EXISTS idx_security_policies_created_by ON security_policies(created_by);

-- Add unique constraint on name per organization
CREATE UNIQUE INDEX IF NOT EXISTS idx_security_policies_org_name ON security_policies(organization_id, name);

-- Add comments
COMMENT ON TABLE security_policies IS 'Configurable security policies for threat detection and prevention';
COMMENT ON COLUMN security_policies.policy_type IS 'Type of policy: capability_violation, trust_score_low, unusual_activity, etc.';
COMMENT ON COLUMN security_policies.enforcement_action IS 'Action to take: alert_only, block_and_alert, allow';
COMMENT ON COLUMN security_policies.severity_threshold IS 'Minimum severity to trigger: info, low, medium, high, critical';
COMMENT ON COLUMN security_policies.rules IS 'Policy rules configuration in JSON format';
COMMENT ON COLUMN security_policies.applies_to IS 'Scope: all, agent_id:xxx, agent_type:ai, etc.';
COMMENT ON COLUMN security_policies.priority IS 'Evaluation priority (higher first)';
