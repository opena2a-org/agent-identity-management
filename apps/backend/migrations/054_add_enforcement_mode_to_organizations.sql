-- Migration: Add enforcement_mode to organizations table
-- This allows admins to control whether verification failures block execution (strict) or just log (monitoring)

-- Add enforcement_mode column with default 'monitoring' (safe default for existing orgs)
ALTER TABLE organizations ADD COLUMN IF NOT EXISTS enforcement_mode VARCHAR(20) NOT NULL DEFAULT 'monitoring';

-- Add constraint to ensure valid values
ALTER TABLE organizations DROP CONSTRAINT IF EXISTS organizations_enforcement_mode_check;
ALTER TABLE organizations ADD CONSTRAINT organizations_enforcement_mode_check
    CHECK (enforcement_mode IN ('strict', 'monitoring'));

-- Add comment explaining the column
COMMENT ON COLUMN organizations.enforcement_mode IS 'Controls SDK behavior on verification failure: strict=block execution, monitoring=log and allow';

-- Update existing organizations to use monitoring mode (safe default)
UPDATE organizations SET enforcement_mode = 'monitoring' WHERE enforcement_mode IS NULL;
