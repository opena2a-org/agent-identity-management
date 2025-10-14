-- Rollback password support from registration flow

-- Drop new indexes
DROP INDEX IF EXISTS idx_registration_password_exists;
DROP INDEX IF EXISTS idx_registration_email_pending_unique;
DROP INDEX IF EXISTS idx_registration_oauth_unique;

-- Restore the original unique constraint (this will fail if there are NULL values)
-- Note: In a real rollback, you'd need to clean up any NULL oauth data first
ALTER TABLE user_registration_requests 
ADD CONSTRAINT user_registration_requests_oauth_provider_oauth_user_id_key 
UNIQUE(oauth_provider, oauth_user_id);

-- Make OAuth fields NOT NULL again
ALTER TABLE user_registration_requests 
ALTER COLUMN oauth_provider SET NOT NULL,
ALTER COLUMN oauth_user_id SET NOT NULL;

-- Remove password field
ALTER TABLE user_registration_requests 
DROP COLUMN password_hash;
