-- Migration: Remove auto_approve_sso column (SSO is premium-only feature)
-- Issue: auto_approve_sso added but SSO removed from Community version
-- Solution: Drop column entirely as it's not used in Community edition
-- Date: 2025-10-22

-- Drop auto_approve_sso column if it exists
ALTER TABLE organizations
DROP COLUMN IF EXISTS auto_approve_sso;
