-- Migration: Create operational metrics tables for trust scoring
-- Purpose: Store operational data for 8-factor trust algorithm

-- Agent Health Checks (for Uptime & Availability factor)
CREATE TABLE IF NOT EXISTS agent_health_checks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    check_time TIMESTAMP NOT NULL DEFAULT NOW(),
    is_successful BOOLEAN NOT NULL,
    response_time_ms INTEGER,
    error_message TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_agent_health_checks_agent_id ON agent_health_checks(agent_id);
CREATE INDEX IF NOT EXISTS idx_agent_health_checks_check_time ON agent_health_checks(check_time DESC);
CREATE INDEX IF NOT EXISTS idx_agent_health_checks_is_successful ON agent_health_checks(is_successful);

-- Agent Actions (for Action Success Rate and Verification Status factors)
CREATE TABLE IF NOT EXISTS agent_actions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    action_type VARCHAR(100) NOT NULL,
    action_name VARCHAR(255) NOT NULL,
    is_successful BOOLEAN NOT NULL,
    is_verified BOOLEAN NOT NULL DEFAULT FALSE, -- Ed25519 signature verification
    error_message TEXT,
    execution_time_ms INTEGER,
    metadata JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_agent_actions_agent_id ON agent_actions(agent_id);
CREATE INDEX IF NOT EXISTS idx_agent_actions_created_at ON agent_actions(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_agent_actions_is_successful ON agent_actions(is_successful);
CREATE INDEX IF NOT EXISTS idx_agent_actions_is_verified ON agent_actions(is_verified);
CREATE INDEX IF NOT EXISTS idx_agent_actions_action_type ON agent_actions(action_type);

-- Agent Behavioral Baselines (for Drift Detection factor)
CREATE TABLE IF NOT EXISTS agent_behavioral_baselines (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    metric_name VARCHAR(100) NOT NULL,
    baseline_value DECIMAL(10, 2) NOT NULL,
    current_value DECIMAL(10, 2) NOT NULL,
    deviation_percentage DECIMAL(5, 2) NOT NULL,
    is_anomaly BOOLEAN NOT NULL DEFAULT FALSE,
    baseline_period_start TIMESTAMP NOT NULL,
    baseline_period_end TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_agent_behavioral_baselines_agent_id ON agent_behavioral_baselines(agent_id);
CREATE INDEX IF NOT EXISTS idx_agent_behavioral_baselines_metric_name ON agent_behavioral_baselines(metric_name);
CREATE INDEX IF NOT EXISTS idx_agent_behavioral_baselines_is_anomaly ON agent_behavioral_baselines(is_anomaly);
CREATE INDEX IF NOT EXISTS idx_agent_behavioral_baselines_created_at ON agent_behavioral_baselines(created_at DESC);

-- Add updated_at trigger
CREATE TRIGGER update_agent_behavioral_baselines_updated_at BEFORE UPDATE ON agent_behavioral_baselines
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Agent User Feedback (for User Feedback factor)
CREATE TABLE IF NOT EXISTS agent_user_feedback (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    rating INTEGER NOT NULL CHECK (rating >= 1 AND rating <= 5),
    feedback_type VARCHAR(50) NOT NULL, -- 'thumbs_up', 'thumbs_down', 'rating', 'comment'
    comment TEXT,
    context JSONB DEFAULT '{}'::jsonb, -- What action/feature was rated
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_agent_user_feedback_agent_id ON agent_user_feedback(agent_id);
CREATE INDEX IF NOT EXISTS idx_agent_user_feedback_user_id ON agent_user_feedback(user_id);
CREATE INDEX IF NOT EXISTS idx_agent_user_feedback_rating ON agent_user_feedback(rating);
CREATE INDEX IF NOT EXISTS idx_agent_user_feedback_feedback_type ON agent_user_feedback(feedback_type);
CREATE INDEX IF NOT EXISTS idx_agent_user_feedback_created_at ON agent_user_feedback(created_at DESC);

-- Compliance Events (for Compliance Score factor)
CREATE TABLE IF NOT EXISTS agent_compliance_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    compliance_type VARCHAR(100) NOT NULL, -- 'soc2', 'hipaa', 'gdpr'
    requirement VARCHAR(255) NOT NULL,
    is_compliant BOOLEAN NOT NULL,
    violation_details TEXT,
    auto_remediated BOOLEAN NOT NULL DEFAULT FALSE,
    remediation_details TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_agent_compliance_events_agent_id ON agent_compliance_events(agent_id);
CREATE INDEX IF NOT EXISTS idx_agent_compliance_events_compliance_type ON agent_compliance_events(compliance_type);
CREATE INDEX IF NOT EXISTS idx_agent_compliance_events_is_compliant ON agent_compliance_events(is_compliant);
CREATE INDEX IF NOT EXISTS idx_agent_compliance_events_created_at ON agent_compliance_events(created_at DESC);
