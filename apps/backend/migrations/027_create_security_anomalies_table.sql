-- Migration: Create Security Anomalies table
-- Created: 2025-10-22
-- Description: Sprint 5 - Security Dashboard - Anomaly Detection

-- Create security_anomalies table
CREATE TABLE IF NOT EXISTS security_anomalies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    anomaly_type VARCHAR(100) NOT NULL,
    severity VARCHAR(20) NOT NULL CHECK (severity IN ('low', 'medium', 'high', 'critical')),
    title VARCHAR(255) NOT NULL,
    description TEXT,
    resource_type VARCHAR(100),
    resource_id UUID,
    confidence DECIMAL(5,2) NOT NULL CHECK (confidence >= 0 AND confidence <= 100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    resolved_at TIMESTAMPTZ,
    resolved_by UUID REFERENCES users(id) ON DELETE SET NULL
);

-- Create indexes for better query performance
CREATE INDEX IF NOT EXISTS idx_security_anomalies_organization ON security_anomalies(organization_id);
CREATE INDEX IF NOT EXISTS idx_security_anomalies_severity ON security_anomalies(severity);
CREATE INDEX IF NOT EXISTS idx_security_anomalies_created_at ON security_anomalies(created_at);
CREATE INDEX IF NOT EXISTS idx_security_anomalies_anomaly_type ON security_anomalies(anomaly_type);
CREATE INDEX IF NOT EXISTS idx_security_anomalies_resolved_at ON security_anomalies(resolved_at);

-- Add comments for documentation
COMMENT ON TABLE security_anomalies IS 'Security anomalies detected by the system (Sprint 5 - Security Dashboard)';
COMMENT ON COLUMN security_anomalies.anomaly_type IS 'Type of anomaly (e.g., unusual_access_pattern, privilege_escalation, data_exfiltration)';
COMMENT ON COLUMN security_anomalies.severity IS 'Severity level: low, medium, high, or critical';
COMMENT ON COLUMN security_anomalies.confidence IS 'Confidence score of the anomaly detection (0-100)';
COMMENT ON COLUMN security_anomalies.resource_type IS 'Type of resource affected (e.g., agent, user, api_key)';
COMMENT ON COLUMN security_anomalies.resource_id IS 'ID of the affected resource';
