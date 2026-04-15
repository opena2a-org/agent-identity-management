-- Migration 085: Add execution isolation level trust factor
-- Adds isolation attestation table and execution_isolation column to trust_scores

-- Isolation attestation records from agents
CREATE TABLE IF NOT EXISTS isolation_attestations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    sandbox VARCHAR(50) NOT NULL DEFAULT 'none',
    network VARCHAR(50) NOT NULL DEFAULT 'none',
    filesystem VARCHAR(50) NOT NULL DEFAULT 'none',
    process VARCHAR(50) NOT NULL DEFAULT 'none',
    score DECIMAL(5,4) NOT NULL DEFAULT 0.0,
    reported_at TIMESTAMP NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_isolation_attestations_agent_id ON isolation_attestations(agent_id);
CREATE INDEX IF NOT EXISTS idx_isolation_attestations_reported_at ON isolation_attestations(agent_id, reported_at DESC);

-- Add execution_isolation factor column to trust_scores
ALTER TABLE trust_scores ADD COLUMN IF NOT EXISTS execution_isolation DECIMAL(5,4) DEFAULT 0.0;

-- NanoMind TME integration: store latest TME evaluation per agent
CREATE TABLE IF NOT EXISTS nanomind_tme_evaluations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    tme_score DECIMAL(5,4) NOT NULL,
    threat_classes JSONB DEFAULT '[]',
    model_version VARCHAR(50) NOT NULL,
    evaluated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_nanomind_tme_agent_id ON nanomind_tme_evaluations(agent_id);
CREATE INDEX IF NOT EXISTS idx_nanomind_tme_evaluated_at ON nanomind_tme_evaluations(agent_id, evaluated_at DESC);
