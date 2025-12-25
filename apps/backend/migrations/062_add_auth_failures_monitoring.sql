-- Migration: Add authentication failure monitoring tables
-- Description: Track failed authentication attempts and account lockouts for security

-- Auth failures table - records each failed authentication attempt
CREATE TABLE IF NOT EXISTS auth_failures (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    email VARCHAR(255) NOT NULL,
    failure_type VARCHAR(50) NOT NULL DEFAULT 'invalid_credentials',
    ip_address VARCHAR(45),
    user_agent TEXT,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes for efficient querying
CREATE INDEX IF NOT EXISTS idx_auth_failures_email ON auth_failures(email);
CREATE INDEX IF NOT EXISTS idx_auth_failures_email_created ON auth_failures(email, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_auth_failures_org_created ON auth_failures(organization_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_auth_failures_ip ON auth_failures(ip_address);

-- Auth lockouts table - tracks account lockouts
CREATE TABLE IF NOT EXISTS auth_lockouts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    email VARCHAR(255) NOT NULL,
    failed_attempts INT NOT NULL DEFAULT 0,
    locked_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    locked_until TIMESTAMP WITH TIME ZONE NOT NULL,
    unlocked_at TIMESTAMP WITH TIME ZONE,
    unlocked_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes for lockout queries
CREATE INDEX IF NOT EXISTS idx_auth_lockouts_email ON auth_lockouts(email);
CREATE INDEX IF NOT EXISTS idx_auth_lockouts_email_active ON auth_lockouts(email, locked_until)
    WHERE unlocked_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_auth_lockouts_org ON auth_lockouts(organization_id, created_at DESC);

-- Add trigger to auto-update updated_at
CREATE OR REPLACE FUNCTION update_auth_lockouts_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_auth_lockouts_updated_at ON auth_lockouts;
CREATE TRIGGER trigger_auth_lockouts_updated_at
    BEFORE UPDATE ON auth_lockouts
    FOR EACH ROW
    EXECUTE FUNCTION update_auth_lockouts_updated_at();

-- Add alert type for authentication failures
-- (Alert types are strings, no schema change needed)

COMMENT ON TABLE auth_failures IS 'Records failed authentication attempts for security monitoring';
COMMENT ON TABLE auth_lockouts IS 'Tracks account lockouts due to repeated failed authentication';
