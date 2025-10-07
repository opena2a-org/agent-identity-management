-- Add organization auto-approval setting
ALTER TABLE organizations
ADD COLUMN IF NOT EXISTS auto_approve_sso BOOLEAN DEFAULT TRUE;

COMMENT ON COLUMN organizations.auto_approve_sso IS
'When TRUE, SSO users are automatically approved. When FALSE, admins must manually approve new SSO users.';

-- Create user status enum
DO $$ BEGIN
    CREATE TYPE user_status AS ENUM ('pending', 'active', 'suspended', 'deactivated');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

-- Add user status and approval tracking
ALTER TABLE users
ADD COLUMN IF NOT EXISTS status user_status DEFAULT 'active',
ADD COLUMN IF NOT EXISTS approved_by UUID REFERENCES users(id),
ADD COLUMN IF NOT EXISTS approved_at TIMESTAMPTZ;

-- Set all existing users to active status
UPDATE users SET status = 'active' WHERE status IS NULL;

-- Add index for efficient pending user queries
CREATE INDEX IF NOT EXISTS idx_users_org_status ON users(organization_id, status);

-- Add comments
COMMENT ON COLUMN users.status IS 'User account status: pending (awaiting approval), active (can use system), suspended (temporarily blocked), deactivated (permanently disabled)';
COMMENT ON COLUMN users.approved_by IS 'UUID of admin who approved this user (NULL for auto-approved or first user)';
COMMENT ON COLUMN users.approved_at IS 'Timestamp when user was approved';
