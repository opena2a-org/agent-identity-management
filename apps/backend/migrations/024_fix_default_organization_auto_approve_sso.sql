-- Migration: Add auto_approve_sso column to organizations table
-- Issue: organizations table missing auto_approve_sso column
-- Solution: Add column with default TRUE, then update existing organization
-- Date: 2025-10-22

-- Add auto_approve_sso column if it doesn't exist
ALTER TABLE organizations
ADD COLUMN IF NOT EXISTS auto_approve_sso BOOLEAN NOT NULL DEFAULT TRUE;

-- Update default organization to ensure auto_approve_sso is TRUE
UPDATE organizations
SET auto_approve_sso = TRUE
WHERE id = 'a0000000-0000-0000-0000-000000000001'::uuid;

-- Add comment for clarity
COMMENT ON COLUMN organizations.auto_approve_sso IS 'Auto-approve SSO users for easier onboarding (default: TRUE)';
