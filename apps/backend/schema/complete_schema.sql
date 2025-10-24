-- AIM Complete Database Schema
-- This is the complete, production-ready schema for fresh deployments
-- No migrations needed - this is the final state
-- Generated: 2025-10-20

-- Organizations table
CREATE TABLE IF NOT EXISTS organizations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    domain VARCHAR(255) UNIQUE NOT NULL,
    plan_type VARCHAR(50) NOT NULL DEFAULT 'free',
    max_agents INTEGER NOT NULL DEFAULT 100,
    max_users INTEGER NOT NULL DEFAULT 10,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    auto_approve_sso BOOLEAN NOT NULL DEFAULT TRUE,
    settings JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_organizations_domain ON organizations(domain);
CREATE INDEX IF NOT EXISTS idx_organizations_is_active ON organizations(is_active);

-- Users table (complete with all authentication columns)
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    email VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    avatar_url TEXT,
    role VARCHAR(50) NOT NULL DEFAULT 'member',
    provider VARCHAR(50) NOT NULL,
    provider_id VARCHAR(255) NOT NULL,
    last_login_at TIMESTAMPTZ,

    -- Password authentication (from migration 006)
    password_hash TEXT,
    email_verified BOOLEAN DEFAULT FALSE,
    force_password_change BOOLEAN DEFAULT FALSE,

    -- User status and approvals (from migration 002)
    status VARCHAR(50) NOT NULL DEFAULT 'active',
    deleted_at TIMESTAMPTZ,
    approved_by UUID,
    approved_at TIMESTAMPTZ,

    -- Password reset (from migration 002)
    password_reset_token VARCHAR(255),
    password_reset_expires_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(organization_id, email),
    UNIQUE(provider, provider_id)
);

CREATE INDEX IF NOT EXISTS idx_users_organization_id ON users(organization_id);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_provider ON users(provider, provider_id);
CREATE INDEX IF NOT EXISTS idx_users_email_hash ON users(email, password_hash);
CREATE INDEX IF NOT EXISTS idx_users_email_verified ON users(email_verified);
CREATE INDEX IF NOT EXISTS idx_users_status ON users(status);
CREATE INDEX IF NOT EXISTS idx_users_password_reset_token ON users(password_reset_token);

COMMENT ON COLUMN users.password_hash IS 'Bcrypt hashed password for local authentication (NULL for OAuth-only users)';
COMMENT ON COLUMN users.email_verified IS 'Whether user email has been verified';
COMMENT ON COLUMN users.force_password_change IS 'Whether user must change password on next login';
COMMENT ON COLUMN users.status IS 'User account status: active, pending_approval, deactivated';
COMMENT ON COLUMN users.password_reset_token IS 'Token for password reset workflow (hashed)';
COMMENT ON COLUMN users.password_reset_expires_at IS 'Expiration time for password reset token';

-- System configuration table (from migration 007)
CREATE TABLE IF NOT EXISTS system_config (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key VARCHAR(255) NOT NULL UNIQUE,
    value TEXT NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_system_config_key ON system_config(key);

COMMENT ON TABLE system_config IS 'System-wide configuration settings';

-- Agents table (with all metadata columns from migration 003)
CREATE TABLE IF NOT EXISTS agents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    display_name VARCHAR(255) NOT NULL,
    description TEXT,
    agent_type VARCHAR(50) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    version VARCHAR(50),
    public_key TEXT,
    certificate_url TEXT,
    repository_url TEXT,
    documentation_url TEXT,
    trust_score DECIMAL(5,2) DEFAULT 0.00,
    verified_at TIMESTAMPTZ,

    -- Additional metadata (from migration 003)
    capabilities JSONB DEFAULT '[]'::jsonb,
    metadata JSONB DEFAULT '{}'::jsonb,
    tags TEXT[] DEFAULT ARRAY[]::TEXT[],

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by UUID NOT NULL REFERENCES users(id),
    UNIQUE(organization_id, name)
);

CREATE INDEX IF NOT EXISTS idx_agents_organization_id ON agents(organization_id);
CREATE INDEX IF NOT EXISTS idx_agents_agent_type ON agents(agent_type);
CREATE INDEX IF NOT EXISTS idx_agents_status ON agents(status);
CREATE INDEX IF NOT EXISTS idx_agents_trust_score ON agents(trust_score);
CREATE INDEX IF NOT EXISTS idx_agents_created_by ON agents(created_by);
CREATE INDEX IF NOT EXISTS idx_agents_tags ON agents USING GIN(tags);

-- API Keys table
CREATE TABLE IF NOT EXISTS api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    key_hash VARCHAR(64) NOT NULL UNIQUE,
    prefix VARCHAR(8) NOT NULL,
    last_used_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by UUID NOT NULL REFERENCES users(id)
);

CREATE INDEX IF NOT EXISTS idx_api_keys_organization_id ON api_keys(organization_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_agent_id ON api_keys(agent_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_key_hash ON api_keys(key_hash);
CREATE INDEX IF NOT EXISTS idx_api_keys_prefix ON api_keys(prefix);
CREATE INDEX IF NOT EXISTS idx_api_keys_is_active ON api_keys(is_active);

-- SDK Tokens table (from migration 008)
CREATE TABLE IF NOT EXISTS sdk_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    token_id VARCHAR(255) NOT NULL UNIQUE,
    device_name TEXT,
    device_fingerprint TEXT,
    ip_address VARCHAR(45),
    user_agent TEXT,
    last_used_at TIMESTAMPTZ,
    last_ip_address VARCHAR(45),
    usage_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    revoke_reason TEXT,
    metadata JSONB
);

CREATE INDEX IF NOT EXISTS idx_sdk_tokens_user_id ON sdk_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_sdk_tokens_organization_id ON sdk_tokens(organization_id);
CREATE INDEX IF NOT EXISTS idx_sdk_tokens_token_hash ON sdk_tokens(token_hash);
CREATE INDEX IF NOT EXISTS idx_sdk_tokens_token_id ON sdk_tokens(token_id);
CREATE INDEX IF NOT EXISTS idx_sdk_tokens_expires_at ON sdk_tokens(expires_at) WHERE revoked_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_sdk_tokens_revoked_at ON sdk_tokens(revoked_at) WHERE revoked_at IS NOT NULL;

COMMENT ON TABLE sdk_tokens IS 'SDK refresh tokens for secure token management and revocation';
COMMENT ON COLUMN sdk_tokens.token_hash IS 'SHA-256 hash of the refresh token';
COMMENT ON COLUMN sdk_tokens.token_id IS 'JWT token ID (jti claim) for lookup';
COMMENT ON COLUMN sdk_tokens.usage_count IS 'Number of times token has been used';

-- Security Policies table (from migration 009)
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

CREATE INDEX IF NOT EXISTS idx_security_policies_organization_id ON security_policies(organization_id);
CREATE INDEX IF NOT EXISTS idx_security_policies_policy_type ON security_policies(policy_type);
CREATE INDEX IF NOT EXISTS idx_security_policies_is_enabled ON security_policies(is_enabled);
CREATE INDEX IF NOT EXISTS idx_security_policies_priority ON security_policies(priority DESC);
CREATE INDEX IF NOT EXISTS idx_security_policies_created_by ON security_policies(created_by);
CREATE UNIQUE INDEX IF NOT EXISTS idx_security_policies_org_name ON security_policies(organization_id, name);

COMMENT ON TABLE security_policies IS 'Configurable security policies for threat detection and prevention';
COMMENT ON COLUMN security_policies.policy_type IS 'Type of policy: capability_violation, trust_score_low, unusual_activity, etc.';
COMMENT ON COLUMN security_policies.enforcement_action IS 'Action to take: alert_only, block_and_alert, allow';
COMMENT ON COLUMN security_policies.severity_threshold IS 'Minimum severity to trigger: info, low, medium, high, critical';
COMMENT ON COLUMN security_policies.priority IS 'Evaluation priority (higher first)';

-- Trust Scores table
CREATE TABLE IF NOT EXISTS trust_scores (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    score DECIMAL(5,2) NOT NULL,
    verification_status DECIMAL(5,2) NOT NULL,
    certificate_validity DECIMAL(5,2) NOT NULL,
    repository_quality DECIMAL(5,2) NOT NULL,
    documentation_score DECIMAL(5,2) NOT NULL,
    community_trust DECIMAL(5,2) NOT NULL,
    security_audit DECIMAL(5,2) NOT NULL,
    update_frequency DECIMAL(5,2) NOT NULL,
    age_score DECIMAL(5,2) NOT NULL,
    confidence DECIMAL(5,2) NOT NULL,
    last_calculated TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_trust_scores_agent_id ON trust_scores(agent_id);
CREATE INDEX IF NOT EXISTS idx_trust_scores_last_calculated ON trust_scores(last_calculated);
CREATE INDEX IF NOT EXISTS idx_trust_scores_created_at ON trust_scores(created_at DESC);

-- Audit Logs table
CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    action VARCHAR(100) NOT NULL,
    resource_type VARCHAR(50) NOT NULL,
    resource_id UUID NOT NULL,
    ip_address VARCHAR(45),
    user_agent TEXT,
    metadata JSONB,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_audit_logs_organization_id ON audit_logs(organization_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs(action);
CREATE INDEX IF NOT EXISTS idx_audit_logs_resource ON audit_logs(resource_type, resource_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_timestamp ON audit_logs(timestamp DESC);

-- Alerts table
CREATE TABLE IF NOT EXISTS alerts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    alert_type VARCHAR(100) NOT NULL,
    severity VARCHAR(50) NOT NULL,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    resource_type VARCHAR(50),
    resource_id UUID,
    is_acknowledged BOOLEAN NOT NULL DEFAULT FALSE,
    acknowledged_by UUID REFERENCES users(id),
    acknowledged_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_alerts_organization_id ON alerts(organization_id);
CREATE INDEX IF NOT EXISTS idx_alerts_alert_type ON alerts(alert_type);
CREATE INDEX IF NOT EXISTS idx_alerts_severity ON alerts(severity);
CREATE INDEX IF NOT EXISTS idx_alerts_is_acknowledged ON alerts(is_acknowledged);
CREATE INDEX IF NOT EXISTS idx_alerts_created_at ON alerts(created_at DESC);

-- MCP Servers table (from migration 004)
CREATE TABLE IF NOT EXISTS mcp_servers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    server_url TEXT NOT NULL,
    public_key TEXT NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by UUID NOT NULL REFERENCES users(id),
    UNIQUE(organization_id, name)
);

CREATE INDEX IF NOT EXISTS idx_mcp_servers_organization_id ON mcp_servers(organization_id);
CREATE INDEX IF NOT EXISTS idx_mcp_servers_status ON mcp_servers(status);
CREATE INDEX IF NOT EXISTS idx_mcp_servers_created_by ON mcp_servers(created_by);

-- Verification Events table (from migration 005)
CREATE TABLE IF NOT EXISTS verification_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    event_type VARCHAR(50) NOT NULL,
    status VARCHAR(50) NOT NULL,
    details JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_verification_events_agent_id ON verification_events(agent_id);
CREATE INDEX IF NOT EXISTS idx_verification_events_event_type ON verification_events(event_type);
CREATE INDEX IF NOT EXISTS idx_verification_events_created_at ON verification_events(created_at DESC);

-- Schema migrations tracking table
CREATE TABLE IF NOT EXISTS schema_migrations (
    version VARCHAR(255) PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Updated_at trigger function
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Add updated_at triggers
CREATE TRIGGER update_organizations_updated_at BEFORE UPDATE ON organizations
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_agents_updated_at BEFORE UPDATE ON agents
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_system_config_updated_at BEFORE UPDATE ON system_config
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_security_policies_updated_at BEFORE UPDATE ON security_policies
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_mcp_servers_updated_at BEFORE UPDATE ON mcp_servers
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Mark schema as applied
INSERT INTO schema_migrations (version) VALUES ('complete_schema_v1.0')
ON CONFLICT (version) DO NOTHING;
