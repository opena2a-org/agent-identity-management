-- Migration: Add intelligent behavioral baseline tracking for agents
-- This enables per-agent anomaly detection by learning each agent's normal behavior patterns

-- Table to store learned behavioral baselines per agent
CREATE TABLE IF NOT EXISTS agent_behavior_baselines (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    -- Baseline learning period
    baseline_start_at TIMESTAMPTZ NOT NULL,
    baseline_end_at TIMESTAMPTZ NOT NULL,
    samples_count INTEGER NOT NULL DEFAULT 0,

    -- Velocity metrics (calls per hour)
    avg_calls_per_hour DOUBLE PRECISION NOT NULL DEFAULT 0,
    stddev_calls_per_hour DOUBLE PRECISION NOT NULL DEFAULT 0,
    max_calls_per_hour INTEGER NOT NULL DEFAULT 0,

    -- Capability usage patterns (JSON: {"capability": usage_count})
    capability_usage JSONB NOT NULL DEFAULT '{}',

    -- Resource access patterns (JSON: {"resource_type": access_count})
    resource_patterns JSONB NOT NULL DEFAULT '{}',

    -- Common action sequences (JSON: [{"sequence": ["action1", "action2"], "count": N}])
    action_sequences JSONB NOT NULL DEFAULT '[]',

    -- Risk-weighted activity score
    avg_risk_score DOUBLE PRECISION NOT NULL DEFAULT 0,
    stddev_risk_score DOUBLE PRECISION NOT NULL DEFAULT 0,

    -- Metadata
    is_mature BOOLEAN NOT NULL DEFAULT FALSE,  -- True when baseline has enough data
    maturity_threshold INTEGER NOT NULL DEFAULT 50,  -- Min samples for mature baseline
    last_updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Ensure one baseline per agent
    UNIQUE(agent_id)
);

-- Indexes for efficient lookups
CREATE INDEX IF NOT EXISTS idx_agent_behavior_baselines_agent_id ON agent_behavior_baselines(agent_id);
CREATE INDEX IF NOT EXISTS idx_agent_behavior_baselines_org_id ON agent_behavior_baselines(organization_id);
CREATE INDEX IF NOT EXISTS idx_agent_behavior_baselines_is_mature ON agent_behavior_baselines(is_mature);

-- Table to store detected behavioral anomalies (for audit trail)
CREATE TABLE IF NOT EXISTS behavioral_anomalies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    -- Anomaly classification
    anomaly_type VARCHAR(50) NOT NULL,  -- velocity_spike, new_capability, new_resource, pattern_break, cross_agent
    severity VARCHAR(20) NOT NULL,      -- low, medium, high, critical

    -- Anomaly details
    description TEXT NOT NULL,

    -- Statistical details
    expected_value DOUBLE PRECISION,     -- What was expected based on baseline
    actual_value DOUBLE PRECISION,       -- What was observed
    deviation_sigma DOUBLE PRECISION,    -- How many std devs from normal

    -- Context
    trigger_action VARCHAR(100),         -- The action that triggered detection
    trigger_resource VARCHAR(255),       -- The resource involved
    trigger_capability VARCHAR(100),     -- The capability involved

    -- Metadata
    metadata JSONB NOT NULL DEFAULT '{}',
    detected_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Link to alert if one was created
    alert_id UUID REFERENCES alerts(id) ON DELETE SET NULL
);

-- Indexes for anomaly lookups
CREATE INDEX IF NOT EXISTS idx_behavioral_anomalies_agent_id ON behavioral_anomalies(agent_id);
CREATE INDEX IF NOT EXISTS idx_behavioral_anomalies_org_id ON behavioral_anomalies(organization_id);
CREATE INDEX IF NOT EXISTS idx_behavioral_anomalies_type ON behavioral_anomalies(anomaly_type);
CREATE INDEX IF NOT EXISTS idx_behavioral_anomalies_severity ON behavioral_anomalies(severity);
CREATE INDEX IF NOT EXISTS idx_behavioral_anomalies_detected_at ON behavioral_anomalies(detected_at);

-- Comments for documentation
COMMENT ON TABLE agent_behavior_baselines IS 'Stores learned behavioral patterns for each agent to enable intelligent anomaly detection';
COMMENT ON COLUMN agent_behavior_baselines.is_mature IS 'True when baseline has accumulated enough samples to be reliable for anomaly detection';
COMMENT ON COLUMN agent_behavior_baselines.avg_calls_per_hour IS 'Average API calls per hour during baseline period';
COMMENT ON COLUMN agent_behavior_baselines.stddev_calls_per_hour IS 'Standard deviation of calls per hour - used for statistical anomaly detection';
COMMENT ON COLUMN agent_behavior_baselines.capability_usage IS 'JSON map of capability to usage count, showing normal capability usage patterns';
COMMENT ON COLUMN agent_behavior_baselines.action_sequences IS 'Common action sequences observed, used to detect unusual action patterns';

COMMENT ON TABLE behavioral_anomalies IS 'Audit trail of detected behavioral anomalies for security analysis';
COMMENT ON COLUMN behavioral_anomalies.deviation_sigma IS 'Number of standard deviations from expected behavior (e.g., 3.5 means 3.5 sigma event)';
