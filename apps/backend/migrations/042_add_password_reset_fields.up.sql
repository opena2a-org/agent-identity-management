-- Add password reset fields to users table if they don't exist
-- Note: These fields may already exist from domain model definition

-- Add password_reset_token column (nullable)
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name='users' AND column_name='password_reset_token') THEN
        ALTER TABLE users ADD COLUMN password_reset_token TEXT;
    END IF;
END $$;

-- Add password_reset_expires_at column (nullable)
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.columns
                   WHERE table_name='users' AND column_name='password_reset_expires_at') THEN
        ALTER TABLE users ADD COLUMN password_reset_expires_at TIMESTAMPTZ;
    END IF;
END $$;

-- Create index on password_reset_token for faster lookups
CREATE INDEX IF NOT EXISTS idx_users_password_reset_token ON users(password_reset_token)
WHERE password_reset_token IS NOT NULL;

-- Create index on password_reset_expires_at for cleanup queries
CREATE INDEX IF NOT EXISTS idx_users_password_reset_expires_at ON users(password_reset_expires_at)
WHERE password_reset_expires_at IS NOT NULL;
