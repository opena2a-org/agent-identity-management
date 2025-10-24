-- Migration: Reset admin password to known value for testing
-- Created: 2025-10-23
-- Purpose: Reset admin@opena2a.org password to test JWT role generation

-- Reset admin password to: AIM2025!Secure
-- This bcrypt hash was verified to match the password
UPDATE users
SET password_hash = '$2a$10$yybTFh5z/GHzwIHl/bNotOCVU3L9IxS/A0ufCwLiPbhFp4/DiYtsu',
    force_password_change = TRUE,
    updated_at = NOW()
WHERE email = 'admin@opena2a.org';

-- Add comment explaining this migration
COMMENT ON TABLE users IS 'Admin password reset to default (migration 034)';
