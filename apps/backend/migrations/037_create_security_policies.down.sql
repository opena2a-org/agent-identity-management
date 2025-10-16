-- Drop security_policies table
DROP TRIGGER IF EXISTS update_security_policies_updated_at ON security_policies;
DROP TABLE IF EXISTS security_policies CASCADE;
