-- Drop RLS policies
DROP POLICY IF EXISTS webhook_deliveries_org_isolation ON webhook_deliveries;
DROP POLICY IF EXISTS webhooks_org_isolation ON webhooks;
DROP POLICY IF EXISTS security_scans_org_isolation ON security_scans;
DROP POLICY IF EXISTS security_incidents_org_isolation ON security_incidents;
DROP POLICY IF EXISTS security_anomalies_org_isolation ON security_anomalies;
DROP POLICY IF EXISTS security_threats_org_isolation ON security_threats;
DROP POLICY IF EXISTS mcp_servers_org_isolation ON mcp_servers;

-- Drop tables in reverse order
DROP TABLE IF EXISTS webhook_deliveries;
DROP TABLE IF EXISTS webhooks;
DROP TABLE IF EXISTS security_scans;
DROP TABLE IF EXISTS security_incidents;
DROP TABLE IF EXISTS security_anomalies;
DROP TABLE IF EXISTS security_threats;
DROP TABLE IF EXISTS mcp_server_keys;
DROP TABLE IF EXISTS mcp_servers;
