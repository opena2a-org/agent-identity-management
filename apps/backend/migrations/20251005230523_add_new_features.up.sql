-- MCP Servers Table
CREATE TABLE IF NOT EXISTS mcp_servers (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    url VARCHAR(500) NOT NULL,
    version VARCHAR(50),
    public_key TEXT,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    is_verified BOOLEAN DEFAULT FALSE,
    last_verified_at TIMESTAMPTZ,
    verification_url VARCHAR(500),
    capabilities TEXT[],
    trust_score DECIMAL(5,2) DEFAULT 0.0,
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT unique_mcp_server_url UNIQUE(organization_id, url)
);

CREATE INDEX idx_mcp_servers_organization ON mcp_servers(organization_id);
CREATE INDEX idx_mcp_servers_status ON mcp_servers(status);
CREATE INDEX idx_mcp_servers_verified ON mcp_servers(is_verified);

-- MCP Server Public Keys Table
CREATE TABLE IF NOT EXISTS mcp_server_keys (
    id UUID PRIMARY KEY,
    server_id UUID NOT NULL REFERENCES mcp_servers(id) ON DELETE CASCADE,
    public_key TEXT NOT NULL,
    key_type VARCHAR(50) NOT NULL, -- 'rsa', 'ed25519', etc.
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_mcp_server_keys_server ON mcp_server_keys(server_id);

-- Security Threats Table
CREATE TABLE IF NOT EXISTS security_threats (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    threat_type VARCHAR(100) NOT NULL,
    severity VARCHAR(50) NOT NULL,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    source VARCHAR(255), -- IP address, agent ID, etc.
    target_type VARCHAR(50), -- 'agent', 'user', 'api_key'
    target_id UUID,
    is_blocked BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    resolved_at TIMESTAMPTZ
);

CREATE INDEX idx_security_threats_organization ON security_threats(organization_id);
CREATE INDEX idx_security_threats_type ON security_threats(threat_type);
CREATE INDEX idx_security_threats_severity ON security_threats(severity);
CREATE INDEX idx_security_threats_blocked ON security_threats(is_blocked);

-- Security Anomalies Table
CREATE TABLE IF NOT EXISTS security_anomalies (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    anomaly_type VARCHAR(100) NOT NULL,
    severity VARCHAR(50) NOT NULL,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    resource_type VARCHAR(50),
    resource_id UUID,
    confidence DECIMAL(5,2) DEFAULT 0.0, -- 0-100
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_security_anomalies_organization ON security_anomalies(organization_id);
CREATE INDEX idx_security_anomalies_type ON security_anomalies(anomaly_type);
CREATE INDEX idx_security_anomalies_severity ON security_anomalies(severity);

-- Security Incidents Table
CREATE TABLE IF NOT EXISTS security_incidents (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    incident_type VARCHAR(100) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'open',
    severity VARCHAR(50) NOT NULL,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    affected_resources TEXT[],
    assigned_to UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    resolved_at TIMESTAMPTZ,
    resolved_by UUID REFERENCES users(id),
    resolution_notes TEXT
);

CREATE INDEX idx_security_incidents_organization ON security_incidents(organization_id);
CREATE INDEX idx_security_incidents_status ON security_incidents(status);
CREATE INDEX idx_security_incidents_severity ON security_incidents(severity);

-- Security Scans Table
CREATE TABLE IF NOT EXISTS security_scans (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    scan_type VARCHAR(100) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'running',
    threats_found INT DEFAULT 0,
    anomalies_found INT DEFAULT 0,
    vulnerabilities_found INT DEFAULT 0,
    security_score DECIMAL(5,2) DEFAULT 0.0,
    started_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMPTZ
);

CREATE INDEX idx_security_scans_organization ON security_scans(organization_id);
CREATE INDEX idx_security_scans_status ON security_scans(status);

-- Webhooks Table
CREATE TABLE IF NOT EXISTS webhooks (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    url VARCHAR(500) NOT NULL,
    events TEXT[] NOT NULL,
    secret VARCHAR(255) NOT NULL,
    is_active BOOLEAN DEFAULT TRUE,
    last_triggered TIMESTAMPTZ,
    failure_count INT DEFAULT 0,
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_webhooks_organization ON webhooks(organization_id);
CREATE INDEX idx_webhooks_active ON webhooks(is_active);

-- Webhook Deliveries Table
CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id UUID PRIMARY KEY,
    webhook_id UUID NOT NULL REFERENCES webhooks(id) ON DELETE CASCADE,
    event VARCHAR(100) NOT NULL,
    payload TEXT NOT NULL,
    status_code INT,
    response_body TEXT,
    success BOOLEAN DEFAULT FALSE,
    attempt_count INT DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_webhook_deliveries_webhook ON webhook_deliveries(webhook_id);
CREATE INDEX idx_webhook_deliveries_success ON webhook_deliveries(success);
CREATE INDEX idx_webhook_deliveries_created ON webhook_deliveries(created_at);

-- Add RLS policies for new tables

-- MCP Servers RLS
ALTER TABLE mcp_servers ENABLE ROW LEVEL SECURITY;

CREATE POLICY mcp_servers_org_isolation ON mcp_servers
    FOR ALL
    USING (organization_id = current_setting('app.current_org_id')::UUID);

-- Security Threats RLS
ALTER TABLE security_threats ENABLE ROW LEVEL SECURITY;

CREATE POLICY security_threats_org_isolation ON security_threats
    FOR ALL
    USING (organization_id = current_setting('app.current_org_id')::UUID);

-- Security Anomalies RLS
ALTER TABLE security_anomalies ENABLE ROW LEVEL SECURITY;

CREATE POLICY security_anomalies_org_isolation ON security_anomalies
    FOR ALL
    USING (organization_id = current_setting('app.current_org_id')::UUID);

-- Security Incidents RLS
ALTER TABLE security_incidents ENABLE ROW LEVEL SECURITY;

CREATE POLICY security_incidents_org_isolation ON security_incidents
    FOR ALL
    USING (organization_id = current_setting('app.current_org_id')::UUID);

-- Security Scans RLS
ALTER TABLE security_scans ENABLE ROW LEVEL SECURITY;

CREATE POLICY security_scans_org_isolation ON security_scans
    FOR ALL
    USING (organization_id = current_setting('app.current_org_id')::UUID);

-- Webhooks RLS
ALTER TABLE webhooks ENABLE ROW LEVEL SECURITY;

CREATE POLICY webhooks_org_isolation ON webhooks
    FOR ALL
    USING (organization_id = current_setting('app.current_org_id')::UUID);

-- Webhook Deliveries RLS (through webhook)
ALTER TABLE webhook_deliveries ENABLE ROW LEVEL SECURITY;

CREATE POLICY webhook_deliveries_org_isolation ON webhook_deliveries
    FOR ALL
    USING (
        EXISTS (
            SELECT 1 FROM webhooks
            WHERE webhooks.id = webhook_deliveries.webhook_id
            AND webhooks.organization_id = current_setting('app.current_org_id')::UUID
        )
    );
