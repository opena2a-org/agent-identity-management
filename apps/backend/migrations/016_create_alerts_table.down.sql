-- Drop alerts table
DROP TRIGGER IF EXISTS update_alerts_updated_at ON alerts;
DROP INDEX IF EXISTS idx_alerts_organization_id;
DROP INDEX IF EXISTS idx_alerts_agent_id;
DROP INDEX IF EXISTS idx_alerts_status;
DROP INDEX IF EXISTS idx_alerts_severity;
DROP INDEX IF EXISTS idx_alerts_alert_type;
DROP INDEX IF EXISTS idx_alerts_created_at;
DROP INDEX IF EXISTS idx_alerts_org_status;
DROP TABLE IF EXISTS alerts;
