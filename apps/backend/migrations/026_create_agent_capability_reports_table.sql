-- Migration: Create Agent Capability Reports table
-- Created: 2025-10-22
-- Description: Sprint 3 - Advanced Analytics - Capability Detection and Reporting

-- Create agent_capability_reports table
CREATE TABLE IF NOT EXISTS agent_capability_reports (
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

-- Create indexes for better query performance
CREATE INDEX IF NOT EXISTS idx_agent_capability_reports_agent_id ON agent_capability_reports(agent_id);
CREATE INDEX IF NOT EXISTS idx_agent_capability_reports_detected_at ON agent_capability_reports(detected_at);
CREATE INDEX IF NOT EXISTS idx_agent_capability_reports_risk_level ON agent_capability_reports(risk_level);

-- Add comments for documentation
COMMENT ON TABLE agent_capability_reports IS 'Capability detection reports for agents (Sprint 3 - Advanced Analytics)';
COMMENT ON COLUMN agent_capability_reports.environment IS 'JSON object containing environment information (OS, language, runtime, etc.)';
COMMENT ON COLUMN agent_capability_reports.ai_models IS 'JSON array of AI models used by the agent';
COMMENT ON COLUMN agent_capability_reports.capabilities IS 'JSON array of detected capabilities';
COMMENT ON COLUMN agent_capability_reports.risk_assessment IS 'JSON object containing detailed risk assessment';
COMMENT ON COLUMN agent_capability_reports.risk_level IS 'Overall risk level: low, medium, high, or critical';
COMMENT ON COLUMN agent_capability_reports.overall_risk_score IS 'Overall risk score (0-100)';
COMMENT ON COLUMN agent_capability_reports.trust_score_impact IS 'Impact on trust score (positive or negative)';
