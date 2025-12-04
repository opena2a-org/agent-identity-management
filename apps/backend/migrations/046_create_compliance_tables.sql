-- Migration: 046_create_compliance_tables.sql
-- Description: Create tables for compliance evidence collection and score trending

-- Compliance Evidence table - stores evidence collected for compliance audits
CREATE TABLE IF NOT EXISTS compliance_evidence (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    framework VARCHAR(50) NOT NULL, -- soc2, hipaa
    check_name VARCHAR(255) NOT NULL, -- Which compliance check this evidence supports
    evidence_type VARCHAR(50) NOT NULL, -- audit_log, configuration, screenshot, document, attestation, system_check, access_review, security_scan
    title VARCHAR(500) NOT NULL,
    description TEXT,
    data JSONB DEFAULT '{}', -- Structured evidence data
    file_url TEXT, -- Optional file attachment URL
    collected_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    collected_by UUID NOT NULL REFERENCES users(id), -- User or system that collected
    is_automatic BOOLEAN NOT NULL DEFAULT false, -- Auto-collected vs manual
    valid_until TIMESTAMP WITH TIME ZONE, -- Evidence expiration (null = no expiration)
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Indexes for compliance_evidence
CREATE INDEX IF NOT EXISTS idx_compliance_evidence_org ON compliance_evidence(organization_id);
CREATE INDEX IF NOT EXISTS idx_compliance_evidence_framework ON compliance_evidence(organization_id, framework);
CREATE INDEX IF NOT EXISTS idx_compliance_evidence_check ON compliance_evidence(organization_id, check_name);
CREATE INDEX IF NOT EXISTS idx_compliance_evidence_collected ON compliance_evidence(collected_at DESC);
CREATE INDEX IF NOT EXISTS idx_compliance_evidence_valid ON compliance_evidence(valid_until) WHERE valid_until IS NOT NULL;

-- Compliance Snapshots table - stores point-in-time compliance scores for trending
CREATE TABLE IF NOT EXISTS compliance_snapshots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    framework VARCHAR(50) NOT NULL, -- soc2, hipaa
    score DECIMAL(5,2) NOT NULL, -- 0-100 score
    passed_checks INTEGER NOT NULL DEFAULT 0,
    failed_checks INTEGER NOT NULL DEFAULT 0,
    total_checks INTEGER NOT NULL DEFAULT 0,
    check_results JSONB DEFAULT '{}', -- Individual check results map
    snapshot_date TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Indexes for compliance_snapshots
CREATE INDEX IF NOT EXISTS idx_compliance_snapshots_org ON compliance_snapshots(organization_id);
CREATE INDEX IF NOT EXISTS idx_compliance_snapshots_framework ON compliance_snapshots(organization_id, framework);
CREATE INDEX IF NOT EXISTS idx_compliance_snapshots_date ON compliance_snapshots(snapshot_date DESC);
CREATE INDEX IF NOT EXISTS idx_compliance_snapshots_trending ON compliance_snapshots(organization_id, framework, snapshot_date);

-- Add comments for documentation
COMMENT ON TABLE compliance_evidence IS 'Stores evidence collected for compliance audits (SOC 2, HIPAA)';
COMMENT ON TABLE compliance_snapshots IS 'Stores point-in-time compliance scores for trending analysis';

COMMENT ON COLUMN compliance_evidence.framework IS 'Compliance framework: soc2, hipaa';
COMMENT ON COLUMN compliance_evidence.evidence_type IS 'Type: audit_log, configuration, screenshot, document, attestation, system_check, access_review, security_scan';
COMMENT ON COLUMN compliance_evidence.is_automatic IS 'True if evidence was auto-collected by system, false if manually uploaded';
COMMENT ON COLUMN compliance_evidence.valid_until IS 'Evidence expiration date, null means no expiration';

COMMENT ON COLUMN compliance_snapshots.score IS 'Compliance score from 0-100';
COMMENT ON COLUMN compliance_snapshots.check_results IS 'JSON map of check_name -> boolean (passed/failed)';
