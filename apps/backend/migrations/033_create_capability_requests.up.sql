-- Create capability_requests table for admin approval workflow
-- This table tracks requests for ADDITIONAL capabilities after initial agent registration

CREATE TABLE IF NOT EXISTS capability_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    capability_type VARCHAR(255) NOT NULL,
    reason TEXT NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending', -- pending, approved, rejected
    requested_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reviewed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    requested_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reviewed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Ensure unique requests per agent/capability
    UNIQUE(agent_id, capability_type)
);

-- Indexes for performance
CREATE INDEX idx_capability_requests_agent_id ON capability_requests(agent_id);
CREATE INDEX idx_capability_requests_status ON capability_requests(status);
CREATE INDEX idx_capability_requests_requested_by ON capability_requests(requested_by);
CREATE INDEX idx_capability_requests_reviewed_by ON capability_requests(reviewed_by);

-- Trigger to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_capability_requests_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_update_capability_requests_updated_at
    BEFORE UPDATE ON capability_requests
    FOR EACH ROW
    EXECUTE FUNCTION update_capability_requests_updated_at();

-- Comments for documentation
COMMENT ON TABLE capability_requests IS 'Admin approval workflow for agent capability expansion requests';
COMMENT ON COLUMN capability_requests.status IS 'Request status: pending, approved, rejected';
COMMENT ON COLUMN capability_requests.reason IS 'User-provided justification for capability request';
