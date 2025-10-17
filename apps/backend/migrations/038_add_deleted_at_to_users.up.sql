-- Add deleted_at column for soft delete tracking
ALTER TABLE users ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

-- Create index for efficient queries on active (non-deleted) users
CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON users(deleted_at) WHERE deleted_at IS NULL;

-- Create composite index for organization + deleted_at queries
CREATE INDEX IF NOT EXISTS idx_users_org_deleted ON users(organization_id, deleted_at);

-- Add comment
COMMENT ON COLUMN users.deleted_at IS 'Timestamp when user was deactivated/soft-deleted. NULL means user is not deleted.';

-- Update existing deactivated users to have deleted_at timestamp
UPDATE users 
SET deleted_at = updated_at 
WHERE status = 'deactivated' AND deleted_at IS NULL;
