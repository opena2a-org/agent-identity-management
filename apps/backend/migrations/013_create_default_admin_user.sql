-- Migration: Create default organization for fresh deployments
-- Created: 2025-10-21
-- Updated: 2026-05-20 — removed sentinel UUIDs (CWE-330) and the hardcoded
--          admin user INSERT (CWE-798). The admin user is now seeded by the
--          Go bootstrap binary at apps/backend/cmd/bootstrap with --default,
--          which generates a random UUID and a random password (printed once
--          to stdout). See HARDENING.md for the bootstrap flow.
--
-- Why the admin INSERT moved out of SQL: AIM Cloud runs on Azure Postgres,
-- which does not allow-list pgcrypto / uuid-ossp. SQL-side password hashing
-- via crypt()/gen_salt() is therefore unavailable in production. gen_random_uuid()
-- is built-in on Postgres 13+ so the org row keeps its random-UUID assignment
-- here; the password hashing lives in Go (auth.PasswordHasher, bcrypt cost=12).

-- Create default organization if it doesn't exist.
-- ON CONFLICT (domain) DO NOTHING makes this idempotent: re-runs preserve the
-- random UUID assigned on first execution.
INSERT INTO organizations (id, name, domain, plan_type, max_agents, max_users, is_active)
VALUES (
    gen_random_uuid(),
    'OpenA2A Admin',
    'admin.opena2a.org',
    'enterprise',
    10000,
    1000,
    TRUE
)
ON CONFLICT (domain) DO NOTHING;

-- Comments reflect the post-2026-05-20 seeding model: the org is seeded here,
-- the admin user is seeded by `aim-bootstrap --default` after migrations run.
COMMENT ON TABLE organizations IS 'Default OpenA2A Admin org (lookup by domain=admin.opena2a.org) seeded with gen_random_uuid()';
COMMENT ON TABLE users IS 'Default admin user seeded by apps/backend/cmd/bootstrap --default; force_password_change=TRUE on first login';
