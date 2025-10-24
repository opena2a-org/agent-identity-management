-- Migration: Create Analytics Tracking Tables
-- Description: Real-time analytics tracking for API calls, data volume, and agent activity
-- Created: 2025-10-20

-- ====================================================================================
-- API Call Tracking Table
-- ====================================================================================
CREATE TABLE IF NOT EXISTS api_calls (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    agent_id UUID REFERENCES agents(id) ON DELETE SET NULL,
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,

    -- Request Details
    method VARCHAR(10) NOT NULL,          -- GET, POST, PUT, DELETE
    endpoint VARCHAR(500) NOT NULL,       -- /api/v1/agents
    status_code INTEGER NOT NULL,         -- 200, 404, 500, etc.

    -- Performance Metrics
    duration_ms INTEGER NOT NULL,         -- Response time in milliseconds
    request_size_bytes INTEGER DEFAULT 0, -- Request body size
    response_size_bytes INTEGER DEFAULT 0, -- Response body size

    -- Metadata
    user_agent TEXT,
    ip_address INET,
    error_message TEXT,

    -- Timestamps
    called_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Indexes for performance
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for fast queries
CREATE INDEX idx_api_calls_org_time ON api_calls(organization_id, called_at DESC);
CREATE INDEX idx_api_calls_agent_time ON api_calls(agent_id, called_at DESC) WHERE agent_id IS NOT NULL;
CREATE INDEX idx_api_calls_endpoint_time ON api_calls(endpoint, called_at DESC);
CREATE INDEX idx_api_calls_status_time ON api_calls(status_code, called_at DESC);

-- Hypertable for time-series optimization (if using TimescaleDB)
-- SELECT create_hypertable('api_calls', 'called_at', if_not_exists => TRUE);

-- ====================================================================================
-- Agent Activity Metrics Table
-- ====================================================================================
CREATE TABLE IF NOT EXISTS agent_activity_metrics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    -- Activity Metrics (per hour)
    api_calls_count INTEGER DEFAULT 0,
    data_processed_bytes BIGINT DEFAULT 0,
    verifications_count INTEGER DEFAULT 0,
    errors_count INTEGER DEFAULT 0,

    -- Performance Metrics
    avg_response_time_ms INTEGER DEFAULT 0,
    p95_response_time_ms INTEGER DEFAULT 0,

    -- Period
    hour_timestamp TIMESTAMPTZ NOT NULL, -- Start of the hour (e.g., 2025-10-20 14:00:00)

    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Unique constraint: one record per agent per hour
    UNIQUE(agent_id, hour_timestamp)
);

-- Indexes for fast queries
CREATE INDEX idx_agent_activity_agent_time ON agent_activity_metrics(agent_id, hour_timestamp DESC);
CREATE INDEX idx_agent_activity_org_time ON agent_activity_metrics(organization_id, hour_timestamp DESC);

-- ====================================================================================
-- Organization Daily Metrics Table
-- ====================================================================================
CREATE TABLE IF NOT EXISTS organization_daily_metrics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    -- Daily Aggregate Metrics
    total_api_calls INTEGER DEFAULT 0,
    total_data_processed_bytes BIGINT DEFAULT 0,
    total_verifications INTEGER DEFAULT 0,
    successful_verifications INTEGER DEFAULT 0,
    failed_verifications INTEGER DEFAULT 0,

    -- Agent Metrics
    total_agents INTEGER DEFAULT 0,
    verified_agents INTEGER DEFAULT 0,
    pending_agents INTEGER DEFAULT 0,
    avg_trust_score DECIMAL(5,2) DEFAULT 0.00,

    -- Performance Metrics
    avg_response_time_ms INTEGER DEFAULT 0,
    p95_response_time_ms INTEGER DEFAULT 0,
    total_errors INTEGER DEFAULT 0,

    -- Period
    date DATE NOT NULL, -- 2025-10-20

    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Unique constraint: one record per organization per day
    UNIQUE(organization_id, date)
);

-- Indexes for fast queries
CREATE INDEX idx_org_daily_metrics_org_date ON organization_daily_metrics(organization_id, date DESC);
CREATE INDEX idx_org_daily_metrics_date ON organization_daily_metrics(date DESC);

-- ====================================================================================
-- Trust Score History Table
-- ====================================================================================
CREATE TABLE IF NOT EXISTS trust_score_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    -- Trust Score Snapshot
    trust_score DECIMAL(5,2) NOT NULL,
    previous_score DECIMAL(5,2),
    change_reason VARCHAR(100), -- "verification_success", "drift_detected", etc.

    -- Metadata
    metadata JSONB,

    -- Timestamps
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for historical analysis
CREATE INDEX idx_trust_score_history_agent_time ON trust_score_history(agent_id, recorded_at DESC);
CREATE INDEX idx_trust_score_history_org_time ON trust_score_history(organization_id, recorded_at DESC);

-- ====================================================================================
-- Functions for Automatic Metric Aggregation
-- ====================================================================================

-- Function to aggregate hourly agent metrics
CREATE OR REPLACE FUNCTION aggregate_agent_hourly_metrics()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO agent_activity_metrics (
        agent_id,
        organization_id,
        api_calls_count,
        data_processed_bytes,
        hour_timestamp
    )
    VALUES (
        NEW.agent_id,
        NEW.organization_id,
        1,
        NEW.request_size_bytes + NEW.response_size_bytes,
        date_trunc('hour', NEW.called_at)
    )
    ON CONFLICT (agent_id, hour_timestamp)
    DO UPDATE SET
        api_calls_count = agent_activity_metrics.api_calls_count + 1,
        data_processed_bytes = agent_activity_metrics.data_processed_bytes + EXCLUDED.data_processed_bytes,
        updated_at = NOW();

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to auto-aggregate when API call is logged
CREATE TRIGGER trigger_aggregate_agent_metrics
AFTER INSERT ON api_calls
FOR EACH ROW
WHEN (NEW.agent_id IS NOT NULL)
EXECUTE FUNCTION aggregate_agent_hourly_metrics();

-- Function to log trust score changes
CREATE OR REPLACE FUNCTION log_trust_score_change()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.trust_score IS DISTINCT FROM OLD.trust_score THEN
        INSERT INTO trust_score_history (
            agent_id,
            organization_id,
            trust_score,
            previous_score,
            change_reason
        )
        VALUES (
            NEW.id,
            NEW.organization_id,
            NEW.trust_score,
            OLD.trust_score,
            'automated_update'
        );
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to auto-log trust score changes
CREATE TRIGGER trigger_log_trust_score
AFTER UPDATE ON agents
FOR EACH ROW
WHEN (NEW.trust_score IS DISTINCT FROM OLD.trust_score)
EXECUTE FUNCTION log_trust_score_change();

-- ====================================================================================
-- Comments
-- ====================================================================================
COMMENT ON TABLE api_calls IS 'Tracks all API calls for analytics and monitoring';
COMMENT ON TABLE agent_activity_metrics IS 'Hourly aggregate metrics per agent for performance tracking';
COMMENT ON TABLE organization_daily_metrics IS 'Daily aggregate metrics per organization for dashboards';
COMMENT ON TABLE trust_score_history IS 'Historical trust score changes for trend analysis';

COMMENT ON COLUMN api_calls.duration_ms IS 'Response time in milliseconds';
COMMENT ON COLUMN api_calls.request_size_bytes IS 'Size of request body in bytes';
COMMENT ON COLUMN api_calls.response_size_bytes IS 'Size of response body in bytes';
