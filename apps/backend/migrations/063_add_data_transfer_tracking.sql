-- Migration: Add data transfer tracking for exfiltration detection
-- Description: Track cumulative data transfers per agent for detecting large-scale data exfiltration

-- Data transfers table - tracks response sizes and cumulative transfers
CREATE TABLE IF NOT EXISTS data_transfers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    -- Transfer details
    action_type VARCHAR(255) NOT NULL,
    resource VARCHAR(1024),
    destination_ip VARCHAR(45),
    destination_domain VARCHAR(255),

    -- Size tracking (in bytes)
    request_size BIGINT DEFAULT 0,
    response_size BIGINT DEFAULT 0,

    -- Metadata
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes for efficient querying
CREATE INDEX IF NOT EXISTS idx_data_transfers_agent_id ON data_transfers(agent_id);
CREATE INDEX IF NOT EXISTS idx_data_transfers_agent_created ON data_transfers(agent_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_data_transfers_org_created ON data_transfers(organization_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_data_transfers_destination ON data_transfers(destination_domain, destination_ip);

-- Aggregated transfers table - hourly rollups for efficient threshold checking
CREATE TABLE IF NOT EXISTS data_transfer_aggregates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    -- Time bucket (hourly)
    bucket_start TIMESTAMP WITH TIME ZONE NOT NULL,
    bucket_end TIMESTAMP WITH TIME ZONE NOT NULL,

    -- Aggregated metrics
    total_request_bytes BIGINT DEFAULT 0,
    total_response_bytes BIGINT DEFAULT 0,
    transfer_count INT DEFAULT 0,
    unique_destinations INT DEFAULT 0,

    -- Top destinations (for forensics)
    top_destinations JSONB DEFAULT '[]',

    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    -- Ensure one aggregate per agent per hour
    UNIQUE (agent_id, bucket_start)
);

-- Indexes for aggregate queries
CREATE INDEX IF NOT EXISTS idx_data_transfer_agg_agent ON data_transfer_aggregates(agent_id, bucket_start DESC);
CREATE INDEX IF NOT EXISTS idx_data_transfer_agg_org ON data_transfer_aggregates(organization_id, bucket_start DESC);
CREATE INDEX IF NOT EXISTS idx_data_transfer_agg_bytes ON data_transfer_aggregates(total_response_bytes DESC);

-- Trigger to auto-update updated_at
CREATE OR REPLACE FUNCTION update_data_transfer_aggregates_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_data_transfer_aggregates_updated_at ON data_transfer_aggregates;
CREATE TRIGGER trigger_data_transfer_aggregates_updated_at
    BEFORE UPDATE ON data_transfer_aggregates
    FOR EACH ROW
    EXECUTE FUNCTION update_data_transfer_aggregates_updated_at();

-- Comments
COMMENT ON TABLE data_transfers IS 'Individual data transfer records for exfiltration detection';
COMMENT ON TABLE data_transfer_aggregates IS 'Hourly aggregated transfer metrics per agent for threshold checking';
COMMENT ON COLUMN data_transfers.response_size IS 'Response body size in bytes';
COMMENT ON COLUMN data_transfer_aggregates.top_destinations IS 'JSON array of top destination domains/IPs by transfer volume';
