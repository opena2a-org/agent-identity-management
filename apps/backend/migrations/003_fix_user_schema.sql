-- Migration: Fix users table schema to match backend domain model exactly
-- This corrects the schema mismatch between database and Go domain model

-- Step 1: Drop the incorrectly structured users table
DROP TABLE IF EXISTS users CASCADE;

-- Step 2: Recreate users table with correct structure matching domain.User
CREATE TABLE users (
    -- Core identity fields
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    email VARCHAR(255) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,  -- Single name field (not first_name/last_name)
    avatar_url VARCHAR(512),     -- Optional avatar URL

    -- Authorization and status
    role VARCHAR(50) NOT NULL DEFAULT 'member' CHECK (role IN ('admin', 'manager', 'member', 'viewer')),
    status VARCHAR(50) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'active', 'suspended', 'deactivated')),

    -- Authentication fields
    password_hash VARCHAR(255),  -- bcrypt hash (nullable for OAuth-only users)
    force_password_change BOOLEAN NOT NULL DEFAULT FALSE,
    password_reset_token VARCHAR(255),
    password_reset_expires_at TIMESTAMPTZ,

    -- Approval tracking
    approved_by UUID REFERENCES users(id) ON DELETE SET NULL,
    approved_at TIMESTAMPTZ,

    -- Activity tracking
    last_login_at TIMESTAMPTZ,

    -- Audit fields
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ  -- Soft delete timestamp
);

-- Step 3: Create indexes for performance
CREATE INDEX idx_users_organization_id ON users(organization_id);
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_status ON users(status);
CREATE INDEX idx_users_role ON users(role);
CREATE INDEX idx_users_approved_by ON users(approved_by);
CREATE INDEX idx_users_deleted_at ON users(deleted_at);

-- Step 4: Create trigger to auto-update updated_at timestamp
CREATE OR REPLACE FUNCTION update_users_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_update_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION update_users_updated_at();

-- Step 5: Reseed default super admin with correct schema
INSERT INTO users (
    id,
    organization_id,
    email,
    name,
    role,
    status,
    password_hash,
    force_password_change,
    created_at,
    updated_at
) VALUES (
    'b0000000-0000-0000-0000-000000000001',
    'a0000000-0000-0000-0000-000000000001',
    'admin@opena2a.org',
    'Super Admin',
    'admin',
    'active',
    '$2a$12$TaOE8dN8LkfP4GCcezcDGezv3yYhOpjAjnqtMIcRoO7U1qWpPLAqG',  -- bcrypt hash for 'UltraSupersecured1!'
    TRUE,  -- Must change password on first login
    NOW(),
    NOW()
) ON CONFLICT (email) DO UPDATE SET
    name = EXCLUDED.name,
    role = EXCLUDED.role,
    status = EXCLUDED.status,
    password_hash = EXCLUDED.password_hash,
    force_password_change = EXCLUDED.force_password_change;

-- Step 6: Add helpful comment
COMMENT ON TABLE users IS 'Platform users with enterprise-grade authentication and RBAC';
COMMENT ON COLUMN users.status IS 'User account status: pending (awaiting approval), active (can use system), suspended (temporarily blocked), deactivated (permanently disabled)';
COMMENT ON COLUMN users.role IS 'User permission level: admin (full access), manager (manage team), member (standard user), viewer (read-only)';
COMMENT ON COLUMN users.force_password_change IS 'True if user must change password on next login (enterprise security requirement)';
COMMENT ON COLUMN users.approved_by IS 'ID of admin user who approved this registration';
