-- Migration: Create default admin user for fresh deployments
-- Created: 2025-10-21
-- Purpose: Every fresh deployment MUST have a default admin user
--          Admin is forced to change password on first login

-- Create default organization if it doesn't exist
INSERT INTO organizations (id, name, domain, plan_type, max_agents, max_users, is_active)
VALUES (
    'a0000000-0000-0000-0000-000000000001'::uuid,
    'OpenA2A Admin',
    'admin.opena2a.org',
    'enterprise',
    10000,
    1000,
    TRUE
)
ON CONFLICT (domain) DO NOTHING;

-- Create default admin user if it doesn't exist
-- Password: AIM2025!Secure
-- Bcrypt hash generated with cost=10
INSERT INTO users (
    id,
    organization_id,
    email,
    name,
    role,
    provider,
    provider_id,
    password_hash,
    status,
    email_verified,
    force_password_change
)
VALUES (
    'a0000000-0000-0000-0000-000000000002'::uuid,
    'a0000000-0000-0000-0000-000000000001'::uuid,
    'admin@opena2a.org',
    'System Administrator',
    'admin',
    'local',
    'admin@opena2a.org',
    '$2a$10$yybTFh5z/GHzwIHl/bNotOCVU3L9IxS/A0ufCwLiPbhFp4/DiYtsu',  -- Password: AIM2025!Secure (MUST be changed on first login)
    'active',
    TRUE,
    TRUE  -- Admin MUST change password on first login
)
ON CONFLICT (organization_id, email) DO NOTHING;

-- Add comment explaining this migration
COMMENT ON TABLE organizations IS 'Default organization (id: a0000000-0000-0000-0000-000000000001) created for system admin';
COMMENT ON TABLE users IS 'Default admin user (admin@opena2a.org) created with force_password_change=TRUE';
