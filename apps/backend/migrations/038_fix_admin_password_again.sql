-- Migration: Fix admin password (reapply with correct hash)
-- Created: 2025-10-23
-- Purpose: Update admin password to correctly hashed value

-- Update admin password to verified working hash
UPDATE users
SET password_hash = '$2a$10$yybTFh5z/GHzwIHl/bNotOCVU3L9IxS/A0ufCwLiPbhFp4/DiYtsu',
    force_password_change = FALSE,
    updated_at = NOW()
WHERE email = 'admin@opena2a.org';

-- Add comment
COMMENT ON TABLE users IS 'Admin password fixed with verified hash (migration 038)';
