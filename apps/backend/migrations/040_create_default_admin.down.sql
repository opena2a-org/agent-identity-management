-- Rollback: Remove default admin user
-- This is safe to run - only removes the default admin, not other admins

DELETE FROM users
WHERE email = 'admin@localhost'
AND name = 'Default Administrator';
