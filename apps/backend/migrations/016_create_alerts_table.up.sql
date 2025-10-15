-- Create alerts table for security and operational alerts
CREATE TABLE IF NOT EXISTS alerts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    agent_id UUID REFERENCES agents(id) ON DELETE CASCADE,
    alert_type VARCHAR(50) NOT NULL, -- 'suspicious_activity', 'trust_drop', 'failed_verification', 'unusual_usage'
    severity VARCHAR(20) NOT NULL, -- 'low', 'medium', 'high', 'critical'
    title VARCHAR(255) NOT NULL,
    description TEXT NOT NULL,
    metadata JSONB DEFAULT '{}'::jsonb,
    status VARCHAR(20) NOT NULL DEFAULT 'active', -- 'active', 'acknowledged', 'dismissed', 'resolved'
    acknowledged_by UUID REFERENCES users(id) ON DELETE SET NULL,
    acknowledged_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Create indexes for common queries
CREATE INDEX idx_alerts_organization_id ON alerts(organization_id);
CREATE INDEX idx_alerts_agent_id ON alerts(agent_id);
CREATE INDEX idx_alerts_status ON alerts(status);
CREATE INDEX idx_alerts_severity ON alerts(severity);
CREATE INDEX idx_alerts_alert_type ON alerts(alert_type);
CREATE INDEX idx_alerts_created_at ON alerts(created_at DESC);
CREATE INDEX idx_alerts_org_status ON alerts(organization_id, status);

-- Create trigger to update updated_at
CREATE TRIGGER update_alerts_updated_at
    BEFORE UPDATE ON alerts
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Add comments for documentation
COMMENT ON TABLE alerts IS 'Security and operational alerts for agents and organizations';
COMMENT ON COLUMN alerts.alert_type IS 'Type of alert: suspicious_activity, trust_drop, failed_verification, unusual_usage';
COMMENT ON COLUMN alerts.severity IS 'Alert severity: low, medium, high, critical';
COMMENT ON COLUMN alerts.status IS 'Alert status: active, acknowledged, dismissed, resolved';
COMMENT ON COLUMN alerts.metadata IS 'Additional context about the alert (JSON)';
