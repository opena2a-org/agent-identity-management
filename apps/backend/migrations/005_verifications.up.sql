-- Create verifications table
CREATE TABLE IF NOT EXISTS verifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    agent_name VARCHAR(255) NOT NULL,
    action VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending', -- approved, denied, pending
    duration_ms INTEGER NOT NULL DEFAULT 0,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Create indexes for efficient queries
CREATE INDEX IF NOT EXISTS idx_verifications_organization_id ON verifications(organization_id);
CREATE INDEX IF NOT EXISTS idx_verifications_agent_id ON verifications(agent_id);
CREATE INDEX IF NOT EXISTS idx_verifications_status ON verifications(status);
CREATE INDEX IF NOT EXISTS idx_verifications_created_at ON verifications(created_at DESC);

-- Add comments
COMMENT ON TABLE verifications IS 'Agent verification requests and approval workflows';
COMMENT ON COLUMN verifications.status IS 'Verification status: approved, denied, pending';
COMMENT ON COLUMN verifications.action IS 'Action being verified (e.g., File Access Request, API Key Generation)';
COMMENT ON COLUMN verifications.duration_ms IS 'Time taken to complete verification in milliseconds';
COMMENT ON COLUMN verifications.metadata IS 'Additional metadata about the verification request';
