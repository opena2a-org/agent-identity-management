-- Create security_policies table for configurable security enforcement
CREATE TABLE IF NOT EXISTS security_policies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    policy_type VARCHAR(50) NOT NULL,
    enforcement_action VARCHAR(50) NOT NULL DEFAULT 'block_and_alert',
    severity_threshold VARCHAR(20) NOT NULL DEFAULT 'high',
    rules JSONB DEFAULT '{}',
    applies_to VARCHAR(255) DEFAULT 'all',
    is_enabled BOOLEAN DEFAULT true,
    priority INT DEFAULT 100,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    created_by UUID REFERENCES users(id) ON DELETE SET NULL
);

-- Indexes for performance
CREATE INDEX idx_security_policies_org ON security_policies(organization_id);
CREATE INDEX idx_security_policies_type ON security_policies(policy_type);
CREATE INDEX idx_security_policies_enabled ON security_policies(is_enabled) WHERE is_enabled = true;
CREATE INDEX idx_security_policies_priority ON security_policies(priority DESC);

-- Trigger to update updated_at
CREATE TRIGGER update_security_policies_updated_at
    BEFORE UPDATE ON security_policies
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Insert default security policies for demonstration
-- Organizations will get these default policies on creation
COMMENT ON TABLE security_policies IS 'Configurable security policies for enforcement actions (alert_only, block_and_alert, allow)';
