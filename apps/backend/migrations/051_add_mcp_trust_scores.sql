-- Migration: Add MCP Trust Scores table
-- Implements 8-factor weighted trust scoring algorithm for MCP servers

-- Main MCP trust scores table
CREATE TABLE IF NOT EXISTS mcp_trust_scores (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    mcp_server_id UUID NOT NULL REFERENCES mcp_servers(id) ON DELETE CASCADE,
    score DECIMAL(5,4) NOT NULL CHECK (score >= 0 AND score <= 1),
    factors JSONB NOT NULL,
    confidence DECIMAL(5,4) NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
    last_calculated TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for efficient queries
CREATE INDEX IF NOT EXISTS idx_mcp_trust_scores_server_id ON mcp_trust_scores(mcp_server_id);
CREATE INDEX IF NOT EXISTS idx_mcp_trust_scores_score ON mcp_trust_scores(score DESC);
CREATE INDEX IF NOT EXISTS idx_mcp_trust_scores_created_at ON mcp_trust_scores(created_at DESC);

-- MCP trust score history for audit trail
CREATE TABLE IF NOT EXISTS mcp_trust_score_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    mcp_server_id UUID NOT NULL REFERENCES mcp_servers(id) ON DELETE CASCADE,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    trust_score DECIMAL(5,4) NOT NULL CHECK (trust_score >= 0 AND trust_score <= 1),
    previous_score DECIMAL(5,4) CHECK (previous_score >= 0 AND previous_score <= 1),
    change_reason TEXT NOT NULL DEFAULT 'automated_recalculation',
    changed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for history queries
CREATE INDEX IF NOT EXISTS idx_mcp_trust_score_history_server ON mcp_trust_score_history(mcp_server_id);
CREATE INDEX IF NOT EXISTS idx_mcp_trust_score_history_org ON mcp_trust_score_history(organization_id);
CREATE INDEX IF NOT EXISTS idx_mcp_trust_score_history_recorded ON mcp_trust_score_history(recorded_at DESC);

-- Add comments explaining the tables
COMMENT ON TABLE mcp_trust_scores IS 'MCP server trust scores calculated using 8-factor weighted algorithm';
COMMENT ON COLUMN mcp_trust_scores.score IS 'Overall trust score (0-1), weighted average of all factors';
COMMENT ON COLUMN mcp_trust_scores.factors IS 'JSON object containing individual factor scores';
COMMENT ON COLUMN mcp_trust_scores.confidence IS 'Confidence level in the score (0-1) based on available data';

COMMENT ON TABLE mcp_trust_score_history IS 'Audit trail for MCP trust score changes';
COMMENT ON COLUMN mcp_trust_score_history.change_reason IS 'Why the score changed (e.g., attestation_added, security_alert, manual_override)';
COMMENT ON COLUMN mcp_trust_score_history.changed_by IS 'User who triggered the change (NULL for automated changes)';

-- Function to automatically record trust score changes
CREATE OR REPLACE FUNCTION record_mcp_trust_score_change()
RETURNS TRIGGER AS $$
BEGIN
    -- Only record if score actually changed
    IF OLD.trust_score IS DISTINCT FROM NEW.trust_score THEN
        INSERT INTO mcp_trust_score_history (
            mcp_server_id,
            organization_id,
            trust_score,
            previous_score,
            change_reason,
            recorded_at
        ) VALUES (
            NEW.id,
            NEW.organization_id,
            NEW.trust_score,
            OLD.trust_score,
            'automated_recalculation',
            NOW()
        );
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to record trust score changes automatically
DROP TRIGGER IF EXISTS mcp_trust_score_change_trigger ON mcp_servers;
CREATE TRIGGER mcp_trust_score_change_trigger
    AFTER UPDATE OF trust_score ON mcp_servers
    FOR EACH ROW
    EXECUTE FUNCTION record_mcp_trust_score_change();
