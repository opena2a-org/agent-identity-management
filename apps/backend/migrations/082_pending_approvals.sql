-- Migration 082: Human Approval Gates (PAM Tier 3)
-- SUPER_PRIVILEGED capabilities require human approval before grant issuance.

CREATE TABLE IF NOT EXISTS pending_approvals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    capability TEXT NOT NULL,
    resource_path TEXT NOT NULL DEFAULT '',
    justification TEXT NOT NULL,
    intent_class TEXT,
    intent_confidence FLOAT,
    asc_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    atc_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    approver_id UUID,
    approver_decision TEXT CHECK (approver_decision IN ('APPROVED', 'DENIED', 'TIMEOUT')),
    approver_note TEXT,
    ticket_id TEXT,                              -- ITSM ticket reference
    ticket_system TEXT,                          -- servicenow, pagerduty, jira
    requested_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    decided_at TIMESTAMPTZ,
    grant_id UUID,
    transparency_log_idx BIGINT
);

CREATE INDEX IF NOT EXISTS idx_pending_approvals_agent ON pending_approvals (agent_id);
CREATE INDEX IF NOT EXISTS idx_pending_approvals_pending ON pending_approvals (approver_decision)
    WHERE approver_decision IS NULL;
