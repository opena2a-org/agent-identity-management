-- Migration: Add auto_approve_sso column to organizations table
-- Issue: organizations table missing auto_approve_sso column
-- Solution: Add column with default TRUE, then update existing organization
-- Date: 2025-10-22
-- Updated: 2026-05-20 — lookup default org by domain (was: hardcoded sentinel
--          UUID a0000000-0000-0000-0000-000000000001 — see B2 hardening).

-- Add auto_approve_sso column if it doesn't exist
ALTER TABLE organizations
ADD COLUMN IF NOT EXISTS auto_approve_sso BOOLEAN NOT NULL DEFAULT TRUE;

-- Update default organization to ensure auto_approve_sso is TRUE.
-- Domain is UNIQUE on the organizations table so this is a stable lookup that
-- works whether the row was seeded via the legacy sentinel-UUID path (pre-B2)
-- or the random-UUID path (post-B2 / migration 013 v2).
UPDATE organizations
SET auto_approve_sso = TRUE
WHERE domain = 'admin.opena2a.org';

-- Add comment for clarity
COMMENT ON COLUMN organizations.auto_approve_sso IS 'Auto-approve SSO users for easier onboarding (default: TRUE)';
