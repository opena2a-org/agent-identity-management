-- Seed Default Super Admin Account
-- Creates default organization and super admin user
-- Default credentials: admin@opena2a.org / ChangeMe123!
-- MUST change password on first login

-- Create default organization
INSERT INTO organizations (
    id,
    name,
    display_name,
    description,
    created_at,
    updated_at
) VALUES (
    'a0000000-0000-0000-0000-000000000001',
    'opena2a',
    'OpenA2A',
    'Default organization for AIM platform',
    NOW(),
    NOW()
) ON CONFLICT (name) DO NOTHING;

-- Create default super admin user
-- Password: ChangeMe123! (bcrypt hashed)
-- Generated with: bcrypt.GenerateFromPassword([]byte("ChangeMe123!"), bcrypt.DefaultCost)
INSERT INTO users (
    id,
    organization_id,
    email,
    password_hash,
    first_name,
    last_name,
    role,
    is_active,
    is_approved,
    email_verified,
    must_change_password,
    created_at,
    updated_at
) VALUES (
    'b0000000-0000-0000-0000-000000000001',
    'a0000000-0000-0000-0000-000000000001',
    'admin@opena2a.org',
    crypt('ChangeMe123!', gen_salt('bf')),
    'Super',
    'Admin',
    'super_admin',
    TRUE,
    TRUE,
    TRUE,
    TRUE,  -- Force password change on first login
    NOW(),
    NOW()
) ON CONFLICT (email) DO NOTHING;

-- Record this migration
INSERT INTO schema_migrations (version, name) VALUES (2, '002_seed_default_admin') ON CONFLICT DO NOTHING;
