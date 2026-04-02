-- Remediation tracking: catch -> fix -> rescan -> improvement measured
CREATE TABLE IF NOT EXISTS remediation_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    finding_id VARCHAR(100) NOT NULL,        -- e.g., "PI-001", "CRED-002"
    source VARCHAR(50) NOT NULL,             -- "agentpwn" or "hma_scan"
    source_event_id VARCHAR(255),            -- agentpwn callback ID or HMA scan ID
    agent_identifier VARCHAR(255),           -- anonymized: user-agent hash or package name
    initial_score INTEGER,                   -- 0-100 score at time of finding
    initial_severity VARCHAR(20),            -- critical/high/medium/low
    remediation_applied BOOLEAN DEFAULT FALSE,
    remediation_scan_id VARCHAR(255),        -- HMA scan ID that showed improvement
    rescan_score INTEGER,                    -- 0-100 score after remediation
    improvement_delta INTEGER,               -- rescan_score - initial_score
    remediation_duration_hours REAL,         -- time from finding to fix
    status VARCHAR(30) NOT NULL DEFAULT 'open', -- open, in_progress, remediated, wont_fix
    metadata JSONB,                          -- extra context
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_remediation_finding ON remediation_records(finding_id);
CREATE INDEX idx_remediation_source ON remediation_records(source);
CREATE INDEX idx_remediation_status ON remediation_records(status);
CREATE INDEX idx_remediation_created ON remediation_records(created_at);
