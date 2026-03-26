-- Migration 084: Privileged Access Certification (PAM Tier 5)
-- Periodic human review and recertification of agent capability templates.

CREATE TABLE IF NOT EXISTS certification_campaigns (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    scope_type TEXT NOT NULL CHECK (scope_type IN ('ALL', 'TEAM', 'TIER', 'AGENT')),
    scope_value TEXT,
    starts_at TIMESTAMPTZ NOT NULL,
    closes_at TIMESTAMPTZ NOT NULL,
    status TEXT NOT NULL DEFAULT 'PENDING'
        CHECK (status IN ('PENDING', 'ACTIVE', 'GRACE_PERIOD', 'CLOSED')),
    created_by TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS certification_decisions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id UUID NOT NULL REFERENCES certification_campaigns(id) ON DELETE CASCADE,
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    capability TEXT NOT NULL,
    decision TEXT CHECK (decision IN ('CERTIFIED', 'MODIFIED', 'REVOKED', 'SUSPENDED')),
    reviewer_id TEXT,
    reviewer_note TEXT,
    decided_at TIMESTAMPTZ,
    transparency_log_idx BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (campaign_id, agent_id, capability)
);

CREATE INDEX IF NOT EXISTS idx_cert_decisions_campaign ON certification_decisions (campaign_id);
CREATE INDEX IF NOT EXISTS idx_cert_decisions_undecided ON certification_decisions (decision)
    WHERE decision IS NULL;
