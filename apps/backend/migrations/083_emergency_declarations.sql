-- Migration 083: Break-Glass Emergency Access (PAM Tier 4)
-- Defined, logged, time-limited emergency access for incident response.

CREATE TABLE IF NOT EXISTS emergency_declarations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    incident_id TEXT NOT NULL,
    declared_by TEXT NOT NULL,
    declared_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    extended_until TIMESTAMPTZ,
    extended_by TEXT,
    affected_agents UUID[] NOT NULL DEFAULT '{}',
    granted_caps TEXT[] NOT NULL DEFAULT '{}',
    justification TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'ACTIVE'
        CHECK (status IN ('ACTIVE', 'EXPIRED', 'REVOKED')),
    review_completed BOOLEAN NOT NULL DEFAULT FALSE,
    review_completed_at TIMESTAMPTZ,
    review_completed_by TEXT,
    transparency_log_idx BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_emergency_active ON emergency_declarations (status)
    WHERE status = 'ACTIVE';
CREATE INDEX IF NOT EXISTS idx_emergency_review ON emergency_declarations (review_completed)
    WHERE review_completed = FALSE AND status = 'EXPIRED';
