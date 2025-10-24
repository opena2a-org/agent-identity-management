-- Migration: Create Capability Requests table
-- Created: 2025-10-22
-- Description: Capability requests for agents needing additional permissions after registration

CREATE TABLE IF NOT EXISTS capability_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    capability_type VARCHAR(100) NOT NULL,
    reason TEXT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    requested_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    reviewed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    requested_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reviewed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT capability_requests_status_check CHECK (status IN ('pending', 'approved', 'rejected'))
);

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_capability_requests_agent_id ON capability_requests(agent_id);
CREATE INDEX IF NOT EXISTS idx_capability_requests_status ON capability_requests(status);
CREATE INDEX IF NOT EXISTS idx_capability_requests_requested_by ON capability_requests(requested_by);
CREATE INDEX IF NOT EXISTS idx_capability_requests_reviewed_by ON capability_requests(reviewed_by);
CREATE INDEX IF NOT EXISTS idx_capability_requests_requested_at ON capability_requests(requested_at DESC);

-- Add comments for documentation
COMMENT ON TABLE capability_requests IS 'Capability requests submitted by agents needing additional permissions';
COMMENT ON COLUMN capability_requests.capability_type IS 'Type of capability being requested (e.g., file_access, network_access, api_access)';
COMMENT ON COLUMN capability_requests.reason IS 'Business justification for the capability request (minimum 10 characters)';
COMMENT ON COLUMN capability_requests.status IS 'Approval status: pending, approved, rejected';
COMMENT ON COLUMN capability_requests.requested_by IS 'User who submitted the capability request';
COMMENT ON COLUMN capability_requests.reviewed_by IS 'Admin/reviewer who approved or rejected the request';
