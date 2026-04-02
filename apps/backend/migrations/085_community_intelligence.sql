-- Migration 085: Community Intelligence opt-in telemetry
-- Allows organizations to opt into sharing anonymized trust factor distributions
-- with the OpenA2A Registry for community benchmarking.

-- Add opt-in columns to organizations table
ALTER TABLE organizations ADD COLUMN IF NOT EXISTS community_intelligence_enabled BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE organizations ADD COLUMN IF NOT EXISTS community_intelligence_consented_at TIMESTAMPTZ;

-- Track anonymized reports sent to the Registry
CREATE TABLE IF NOT EXISTS community_intelligence_reports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    report_type VARCHAR(50) NOT NULL,  -- 'trust_factor_distribution'
    agent_count INTEGER NOT NULL,
    data_points JSONB NOT NULL,  -- anonymized aggregate data
    submitted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    registry_accepted BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ci_reports_org ON community_intelligence_reports(organization_id);
CREATE INDEX IF NOT EXISTS idx_ci_reports_submitted ON community_intelligence_reports(submitted_at);
