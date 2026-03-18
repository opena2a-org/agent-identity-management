-- Device Authorization Grant (RFC 8628) support for CLI login
CREATE TABLE IF NOT EXISTS device_codes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    device_code VARCHAR(255) NOT NULL UNIQUE,
    user_code VARCHAR(16) NOT NULL UNIQUE,
    client_id VARCHAR(255) NOT NULL DEFAULT 'opena2a-cli',
    scope VARCHAR(512) DEFAULT '',
    verification_uri VARCHAR(512) NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    interval_seconds INT NOT NULL DEFAULT 5,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    user_id UUID REFERENCES users(id),
    organization_id UUID REFERENCES organizations(id),
    ip_address VARCHAR(45),
    user_agent TEXT,
    approved_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_device_codes_device_code ON device_codes(device_code);
CREATE INDEX idx_device_codes_user_code ON device_codes(user_code);
CREATE INDEX idx_device_codes_status ON device_codes(status);
CREATE INDEX idx_device_codes_expires_at ON device_codes(expires_at);
