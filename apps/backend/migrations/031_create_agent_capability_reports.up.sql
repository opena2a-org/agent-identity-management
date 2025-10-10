-- Migration 031: Create agent_capability_reports table
-- Purpose: Store capability detection reports from SDKs
-- Date: October 10, 2025

-- Create agent_capability_reports table
CREATE TABLE IF NOT EXISTS agent_capability_reports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    detected_at TIMESTAMPTZ NOT NULL,

    -- Environment details (JSONB for flexible schema)
    environment JSONB NOT NULL DEFAULT '{}'::jsonb,

    -- AI model usage (JSONB array)
    ai_models JSONB NOT NULL DEFAULT '[]'::jsonb,

    -- Detected capabilities (JSONB object with nested capabilities)
    capabilities JSONB NOT NULL DEFAULT '{}'::jsonb,

    -- Risk assessment (JSONB object with risk score, alerts, etc.)
    risk_assessment JSONB NOT NULL DEFAULT '{}'::jsonb,

    -- Denormalized fields for quick filtering and indexing
    risk_level VARCHAR(20) NOT NULL, -- 'LOW', 'MEDIUM', 'HIGH', 'CRITICAL'
    overall_risk_score INT NOT NULL DEFAULT 0, -- 0-100
    trust_score_impact INT NOT NULL DEFAULT 0, -- -50 to 0

    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Create indexes for quick lookups
CREATE INDEX idx_agent_capability_reports_agent_id ON agent_capability_reports(agent_id);
CREATE INDEX idx_agent_capability_reports_risk_level ON agent_capability_reports(risk_level);
CREATE INDEX idx_agent_capability_reports_detected_at ON agent_capability_reports(detected_at DESC);
CREATE INDEX idx_agent_capability_reports_overall_risk_score ON agent_capability_reports(overall_risk_score DESC);

-- Create composite index for common queries
CREATE INDEX idx_agent_capability_reports_agent_risk ON agent_capability_reports(agent_id, risk_level, detected_at DESC);

-- Add comments
COMMENT ON TABLE agent_capability_reports IS 'Stores capability detection reports from SDKs with risk assessment';
COMMENT ON COLUMN agent_capability_reports.risk_level IS 'Risk level: LOW, MEDIUM, HIGH, CRITICAL';
COMMENT ON COLUMN agent_capability_reports.overall_risk_score IS 'Overall risk score from 0 (lowest) to 100 (highest)';
COMMENT ON COLUMN agent_capability_reports.trust_score_impact IS 'Impact on trust score (-50 to 0)';
