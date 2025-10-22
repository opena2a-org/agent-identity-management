-- ============================================================================
-- AIM (Agent Identity Management) - Consolidated Database Schema V1
-- ============================================================================
-- This is the complete schema for fresh deployments.
-- For existing databases, use incremental migrations (001-030).
-- 
-- Created: October 22, 2025
-- Consolidates: Migrations 001-030
-- ============================================================================

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ============================================================================
-- ORGANIZATIONS
-- ============================================================================
CREATE TABLE organizations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    domain VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Default organization (OpenA2A Admin)
INSERT INTO organizations (id, name, domain, created_at, updated_at)
VALUES (
    'a0000000-0000-0000-0000-000000000001'::uuid,
    'OpenA2A Admin',
    'opena2a.org',
    CURRENT_TIMESTAMP,
    CURRENT_TIMESTAMP
) ON CONFLICT (id) DO NOTHING;

-- ============================================================================
-- USERS
-- ============================================================================
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    role VARCHAR(50) NOT NULL DEFAULT 'user' CHECK (role IN ('admin', 'user', 'viewer')),
    is_approved BOOLEAN NOT NULL DEFAULT false,
    force_password_change BOOLEAN NOT NULL DEFAULT false,
    reset_token VARCHAR(255),
    reset_token_expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX idx_users_organization_id ON users(organization_id);
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_reset_token ON users(reset_token);

-- Default admin user (admin@opena2a.org / AIM2025!Secure)
INSERT INTO users (id, organization_id, email, password_hash, first_name, last_name, role, is_approved, force_password_change, created_at, updated_at)
VALUES (
    'a0000000-0000-0000-0000-000000000002'::uuid,
    'a0000000-0000-0000-0000-000000000001'::uuid,
    'admin@opena2a.org',
    '$2a$10$rX8F3qZ5yH2nW7vL9kM4eO8J6tQ1sR9uP3mN5kL7jH9fE2dC1aB0z',
    'System',
    'Administrator',
    'admin',
    true,
    true,
    CURRENT_TIMESTAMP,
    CURRENT_TIMESTAMP
) ON CONFLICT (id) DO NOTHING;

-- ============================================================================
-- USER REGISTRATION REQUESTS
-- ============================================================================
CREATE TABLE user_registration_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    email VARCHAR(255) NOT NULL,
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    employee_id VARCHAR(100),
    department VARCHAR(100),
    role VARCHAR(100),
    manager_email VARCHAR(255),
    location VARCHAR(100),
    status VARCHAR(50) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected')),
    reviewed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at TIMESTAMPTZ,
    rejection_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_user_registration_requests_organization_id ON user_registration_requests(organization_id);
CREATE INDEX idx_user_registration_requests_status ON user_registration_requests(status);
CREATE INDEX idx_user_registration_requests_email ON user_registration_requests(email);

-- ============================================================================
-- AGENTS
-- ============================================================================
CREATE TABLE agents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL,
    public_key TEXT NOT NULL,
    verification_status VARCHAR(50) NOT NULL DEFAULT 'pending',
    trust_score DECIMAL(5,2) NOT NULL DEFAULT 75.00 CHECK (trust_score >= 0 AND trust_score <= 100),
    metadata JSONB DEFAULT '{}'::jsonb,
    is_active BOOLEAN NOT NULL DEFAULT true,
    last_verified_at TIMESTAMPTZ,
    last_seen_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX idx_agents_organization_id ON agents(organization_id);
CREATE INDEX idx_agents_verification_status ON agents(verification_status);
CREATE INDEX idx_agents_type ON agents(type);
CREATE INDEX idx_agents_created_by ON agents(created_by);

-- ============================================================================
-- TRUST SCORES (Historical Tracking)
-- ============================================================================
CREATE TABLE trust_scores (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    score DECIMAL(5,2) NOT NULL CHECK (score >= 0 AND score <= 100),
    confidence DECIMAL(3,2) NOT NULL DEFAULT 0.75 CHECK (confidence >= 0 AND confidence <= 1),
    factors JSONB NOT NULL DEFAULT '{}'::jsonb,
    verification_status DECIMAL(5,4) DEFAULT 0.0,
    uptime DECIMAL(5,4) DEFAULT 0.0,
    success_rate DECIMAL(5,4) DEFAULT 0.0,
    security_alerts DECIMAL(5,4) DEFAULT 0.0,
    compliance DECIMAL(5,4) DEFAULT 0.0,
    age DECIMAL(5,4) DEFAULT 0.0,
    drift_detection DECIMAL(5,4) DEFAULT 0.0,
    user_feedback DECIMAL(5,4) DEFAULT 0.0,
    last_calculated TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_trust_scores_agent_id ON trust_scores(agent_id);
CREATE INDEX idx_trust_scores_last_calculated ON trust_scores(last_calculated);
CREATE INDEX idx_trust_scores_factors ON trust_scores USING gin(factors);

-- ============================================================================
-- MCP SERVERS
-- ============================================================================
CREATE TABLE mcp_servers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    url VARCHAR(255) NOT NULL,
    public_key TEXT,
    verification_status VARCHAR(50) NOT NULL DEFAULT 'pending',
    is_active BOOLEAN NOT NULL DEFAULT true,
    metadata JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX idx_mcp_servers_organization_id ON mcp_servers(organization_id);
CREATE INDEX idx_mcp_servers_verification_status ON mcp_servers(verification_status);

-- ============================================================================
-- VERIFICATION EVENTS
-- ============================================================================
CREATE TABLE verification_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    event_type VARCHAR(50) NOT NULL,
    status VARCHAR(50) NOT NULL,
    details JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_verification_events_agent_id ON verification_events(agent_id);
CREATE INDEX idx_verification_events_created_at ON verification_events(created_at);

-- ============================================================================
-- SDK TOKENS
-- ============================================================================
CREATE TABLE sdk_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    agent_id UUID REFERENCES agents(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    expires_at TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX idx_sdk_tokens_organization_id ON sdk_tokens(organization_id);
CREATE INDEX idx_sdk_tokens_agent_id ON sdk_tokens(agent_id);
CREATE INDEX idx_sdk_tokens_token_hash ON sdk_tokens(token_hash);

-- ============================================================================
-- API KEYS
-- ============================================================================
CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    key_hash VARCHAR(255) NOT NULL UNIQUE,
    key_prefix VARCHAR(64) NOT NULL,
    name VARCHAR(255) NOT NULL,
    expires_at TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX idx_api_keys_organization_id ON api_keys(organization_id);
CREATE INDEX idx_api_keys_key_hash ON api_keys(key_hash);
CREATE INDEX idx_api_keys_key_prefix ON api_keys(key_prefix);

-- ============================================================================
-- SECURITY POLICIES
-- ============================================================================
CREATE TABLE security_policies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    policy_type VARCHAR(50) NOT NULL,
    config JSONB NOT NULL DEFAULT '{}'::jsonb,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX idx_security_policies_organization_id ON security_policies(organization_id);
CREATE INDEX idx_security_policies_policy_type ON security_policies(policy_type);

-- ============================================================================
-- SECURITY ALERTS
-- ============================================================================
CREATE TABLE security_alerts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    agent_id UUID REFERENCES agents(id) ON DELETE CASCADE,
    alert_type VARCHAR(50) NOT NULL,
    severity VARCHAR(20) NOT NULL CHECK (severity IN ('low', 'medium', 'high', 'critical')),
    title VARCHAR(255) NOT NULL,
    description TEXT NOT NULL,
    metadata JSONB DEFAULT '{}'::jsonb,
    status VARCHAR(50) NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'investigating', 'resolved', 'false_positive')),
    acknowledged_by UUID REFERENCES users(id) ON DELETE SET NULL,
    acknowledged_at TIMESTAMPTZ,
    resolved_by UUID REFERENCES users(id) ON DELETE SET NULL,
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_security_alerts_organization_id ON security_alerts(organization_id);
CREATE INDEX idx_security_alerts_agent_id ON security_alerts(agent_id);
CREATE INDEX idx_security_alerts_severity ON security_alerts(severity);
CREATE INDEX idx_security_alerts_status ON security_alerts(status);
CREATE INDEX idx_security_alerts_created_at ON security_alerts(created_at);

-- ============================================================================
-- SECURITY ANOMALIES
-- ============================================================================
CREATE TABLE security_anomalies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    agent_id UUID REFERENCES agents(id) ON DELETE CASCADE,
    anomaly_type VARCHAR(100) NOT NULL,
    severity VARCHAR(20) NOT NULL CHECK (severity IN ('low', 'medium', 'high', 'critical')),
    description TEXT NOT NULL,
    metadata JSONB DEFAULT '{}'::jsonb,
    detected_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    status VARCHAR(50) NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'investigating', 'resolved', 'false_positive')),
    resolved_at TIMESTAMPTZ,
    resolved_by UUID REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX idx_security_anomalies_organization_id ON security_anomalies(organization_id);
CREATE INDEX idx_security_anomalies_agent_id ON security_anomalies(agent_id);
CREATE INDEX idx_security_anomalies_severity ON security_anomalies(severity);
CREATE INDEX idx_security_anomalies_status ON security_anomalies(status);
CREATE INDEX idx_security_anomalies_detected_at ON security_anomalies(detected_at);

-- ============================================================================
-- AUDIT LOGS
-- ============================================================================
CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    agent_id UUID REFERENCES agents(id) ON DELETE SET NULL,
    action VARCHAR(100) NOT NULL,
    resource_type VARCHAR(50) NOT NULL,
    resource_id UUID,
    details JSONB DEFAULT '{}'::jsonb,
    ip_address VARCHAR(45),
    user_agent TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_audit_logs_organization_id ON audit_logs(organization_id);
CREATE INDEX idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX idx_audit_logs_agent_id ON audit_logs(agent_id);
CREATE INDEX idx_audit_logs_action ON audit_logs(action);
CREATE INDEX idx_audit_logs_resource_type ON audit_logs(resource_type);
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at);

-- ============================================================================
-- TRUST SCORE AUDIT LOGS
-- ============================================================================
CREATE TABLE trust_score_audit (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    old_score DECIMAL(5,2) NOT NULL,
    new_score DECIMAL(5,2) NOT NULL,
    reason VARCHAR(255),
    metadata JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_trust_score_audit_agent_id ON trust_score_audit(agent_id);
CREATE INDEX idx_trust_score_audit_created_at ON trust_score_audit(created_at);

-- Trust score change trigger
CREATE OR REPLACE FUNCTION log_trust_score_change()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.trust_score <> OLD.trust_score THEN
        INSERT INTO trust_score_audit (agent_id, old_score, new_score, reason, created_at)
        VALUES (NEW.id, OLD.trust_score, NEW.trust_score, 'Automatic update', CURRENT_TIMESTAMP);
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_log_trust_score
AFTER UPDATE ON agents
FOR EACH ROW
WHEN (NEW.trust_score IS DISTINCT FROM OLD.trust_score)
EXECUTE FUNCTION log_trust_score_change();

-- ============================================================================
-- SYSTEM CONFIG
-- ============================================================================
CREATE TABLE system_config (
    key VARCHAR(255) PRIMARY KEY,
    value TEXT NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- ============================================================================
-- AGENT CAPABILITIES
-- ============================================================================
CREATE TABLE agent_capabilities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    capability_name VARCHAR(255) NOT NULL,
    capability_type VARCHAR(100) NOT NULL,
    risk_level VARCHAR(20) NOT NULL CHECK (risk_level IN ('low', 'medium', 'high', 'critical')),
    is_allowed BOOLEAN NOT NULL DEFAULT true,
    metadata JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_agent_capabilities_agent_id ON agent_capabilities(agent_id);
CREATE INDEX idx_agent_capabilities_capability_type ON agent_capabilities(capability_type);
CREATE INDEX idx_agent_capabilities_risk_level ON agent_capabilities(risk_level);

-- ============================================================================
-- CAPABILITY VIOLATIONS
-- ============================================================================
CREATE TABLE capability_violations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    capability_id UUID REFERENCES agent_capabilities(id) ON DELETE SET NULL,
    violation_type VARCHAR(100) NOT NULL,
    severity VARCHAR(20) NOT NULL CHECK (severity IN ('low', 'medium', 'high', 'critical')),
    description TEXT NOT NULL,
    metadata JSONB DEFAULT '{}'::jsonb,
    detected_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    resolved_at TIMESTAMPTZ,
    resolved_by UUID REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX idx_capability_violations_organization_id ON capability_violations(organization_id);
CREATE INDEX idx_capability_violations_agent_id ON capability_violations(agent_id);
CREATE INDEX idx_capability_violations_capability_id ON capability_violations(capability_id);
CREATE INDEX idx_capability_violations_severity ON capability_violations(severity);
CREATE INDEX idx_capability_violations_detected_at ON capability_violations(detected_at);

-- ============================================================================
-- AGENT CAPABILITY REPORTS
-- ============================================================================
CREATE TABLE agent_capability_reports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    detected_at TIMESTAMPTZ NOT NULL,
    environment JSONB NOT NULL DEFAULT '{}'::jsonb,
    ai_models JSONB NOT NULL DEFAULT '[]'::jsonb,
    capabilities JSONB NOT NULL DEFAULT '[]'::jsonb,
    risk_assessment JSONB NOT NULL DEFAULT '{}'::jsonb,
    risk_level VARCHAR(20) NOT NULL CHECK (risk_level IN ('low', 'medium', 'high', 'critical')),
    overall_risk_score DECIMAL(5,2) NOT NULL CHECK (overall_risk_score >= 0 AND overall_risk_score <= 100),
    trust_score_impact DECIMAL(5,2) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_agent_capability_reports_agent_id ON agent_capability_reports(agent_id);
CREATE INDEX idx_agent_capability_reports_detected_at ON agent_capability_reports(detected_at);
CREATE INDEX idx_agent_capability_reports_risk_level ON agent_capability_reports(risk_level);

-- ============================================================================
-- OPERATIONAL METRICS
-- ============================================================================
CREATE TABLE operational_metrics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    agent_id UUID REFERENCES agents(id) ON DELETE CASCADE,
    metric_type VARCHAR(100) NOT NULL,
    metric_value DECIMAL(12,2) NOT NULL,
    unit VARCHAR(50),
    metadata JSONB DEFAULT '{}'::jsonb,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_operational_metrics_organization_id ON operational_metrics(organization_id);
CREATE INDEX idx_operational_metrics_agent_id ON operational_metrics(agent_id);
CREATE INDEX idx_operational_metrics_metric_type ON operational_metrics(metric_type);
CREATE INDEX idx_operational_metrics_recorded_at ON operational_metrics(recorded_at);

-- ============================================================================
-- WEBHOOKS
-- ============================================================================
CREATE TABLE webhooks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    url VARCHAR(500) NOT NULL,
    events TEXT[] NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    secret VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX idx_webhooks_organization_id ON webhooks(organization_id);
CREATE INDEX idx_webhooks_is_active ON webhooks(is_active);

-- ============================================================================
-- WEBHOOK DELIVERIES
-- ============================================================================
CREATE TABLE webhook_deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    webhook_id UUID NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
    event_type VARCHAR(100) NOT NULL,
    payload JSONB NOT NULL,
    response_status_code INT,
    response_body TEXT,
    delivered_at TIMESTAMPTZ,
    attempts INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_webhook_deliveries_webhook_id ON webhook_deliveries(webhook_id);
CREATE INDEX idx_webhook_deliveries_event_type ON webhook_deliveries(event_type);
CREATE INDEX idx_webhook_deliveries_created_at ON webhook_deliveries(created_at);

-- ============================================================================
-- TAGS
-- ============================================================================
CREATE TABLE tags (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    key VARCHAR(100) NOT NULL,
    value VARCHAR(255) NOT NULL,
    category VARCHAR(50),
    description TEXT,
    color VARCHAR(20),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    UNIQUE(organization_id, key, value)
);

CREATE INDEX idx_tags_organization_id ON tags(organization_id);
CREATE INDEX idx_tags_key ON tags(key);
CREATE INDEX idx_tags_category ON tags(category);

-- ============================================================================
-- AGENT TAGS
-- ============================================================================
CREATE TABLE agent_tags (
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    tag_id UUID NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (agent_id, tag_id)
);

CREATE INDEX idx_agent_tags_agent_id ON agent_tags(agent_id);
CREATE INDEX idx_agent_tags_tag_id ON agent_tags(tag_id);

-- ============================================================================
-- MCP SERVER TAGS
-- ============================================================================
CREATE TABLE mcp_server_tags (
    mcp_server_id UUID NOT NULL REFERENCES mcp_servers(id) ON DELETE CASCADE,
    tag_id UUID NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (mcp_server_id, tag_id)
);

CREATE INDEX idx_mcp_server_tags_mcp_server_id ON mcp_server_tags(mcp_server_id);
CREATE INDEX idx_mcp_server_tags_tag_id ON mcp_server_tags(tag_id);

-- ============================================================================
-- CAPABILITY REQUESTS
-- ============================================================================
CREATE TABLE capability_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    capability_name VARCHAR(255) NOT NULL,
    capability_type VARCHAR(100) NOT NULL,
    risk_level VARCHAR(20) NOT NULL CHECK (risk_level IN ('low', 'medium', 'high', 'critical')),
    justification TEXT,
    status VARCHAR(50) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected')),
    reviewed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at TIMESTAMPTZ,
    rejection_reason TEXT,
    metadata JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_capability_requests_organization_id ON capability_requests(organization_id);
CREATE INDEX idx_capability_requests_agent_id ON capability_requests(agent_id);
CREATE INDEX idx_capability_requests_status ON capability_requests(status);
CREATE INDEX idx_capability_requests_risk_level ON capability_requests(risk_level);

-- ============================================================================
-- SCHEMA VERSION TRACKING
-- ============================================================================
CREATE TABLE schema_migrations (
    version VARCHAR(255) PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Mark V1 as applied
INSERT INTO schema_migrations (version) VALUES ('V1__consolidated_schema');

-- ============================================================================
-- END OF CONSOLIDATED SCHEMA V1
-- ============================================================================
