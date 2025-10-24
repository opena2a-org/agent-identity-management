-- Create user_registration_requests table for email/password and OAuth registration workflow
CREATE TABLE IF NOT EXISTS user_registration_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL,
    first_name VARCHAR(255) NOT NULL,
    last_name VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255), -- Only for email/password registrations
    oauth_provider VARCHAR(50), -- Nullable for manual registrations
    oauth_user_id VARCHAR(255), -- Nullable for manual registrations
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    requested_at TIMESTAMP NOT NULL DEFAULT NOW(),
    reviewed_at TIMESTAMP,
    reviewed_by UUID REFERENCES users(id),
    rejection_reason TEXT,
    profile_picture_url TEXT,
    oauth_email_verified BOOLEAN NOT NULL DEFAULT FALSE,
    metadata JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Create indexes for common queries
CREATE INDEX IF NOT EXISTS idx_user_registration_requests_email ON user_registration_requests(email);
CREATE INDEX IF NOT EXISTS idx_user_registration_requests_status ON user_registration_requests(status);
CREATE INDEX IF NOT EXISTS idx_user_registration_requests_organization_id ON user_registration_requests(organization_id);
CREATE INDEX IF NOT EXISTS idx_user_registration_requests_requested_at ON user_registration_requests(requested_at DESC);

-- Add updated_at trigger
CREATE TRIGGER update_user_registration_requests_updated_at BEFORE UPDATE ON user_registration_requests
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
