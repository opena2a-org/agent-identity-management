-- Add password support to registration flow
-- Allow users to register with email/password instead of just OAuth

-- Add password field to user_registration_requests
ALTER TABLE user_registration_requests 
ADD COLUMN password_hash VARCHAR(255);

-- Make OAuth fields nullable for manual registrations
ALTER TABLE user_registration_requests 
ALTER COLUMN oauth_provider DROP NOT NULL,
ALTER COLUMN oauth_user_id DROP NOT NULL;

-- Drop the existing unique constraint
ALTER TABLE user_registration_requests 
DROP CONSTRAINT user_registration_requests_oauth_provider_oauth_user_id_key;

-- Add new unique constraint that handles both OAuth and email registrations
CREATE UNIQUE INDEX idx_registration_oauth_unique 
ON user_registration_requests(oauth_provider, oauth_user_id) 
WHERE oauth_provider IS NOT NULL AND oauth_user_id IS NOT NULL;

-- Add unique constraint for email-based registrations (prevent duplicate pending requests)
CREATE UNIQUE INDEX idx_registration_email_pending_unique 
ON user_registration_requests(email) 
WHERE status = 'pending' AND oauth_provider IS NULL;

-- Add index for password-based registrations
CREATE INDEX idx_registration_password_exists 
ON user_registration_requests(email) 
WHERE password_hash IS NOT NULL;

-- Update table comments
COMMENT ON COLUMN user_registration_requests.password_hash IS 'Hashed password for email/password registrations (NULL for OAuth registrations)';
COMMENT ON COLUMN user_registration_requests.oauth_provider IS 'OAuth provider (NULL for email/password registrations)';
COMMENT ON COLUMN user_registration_requests.oauth_user_id IS 'OAuth user ID (NULL for email/password registrations)';
