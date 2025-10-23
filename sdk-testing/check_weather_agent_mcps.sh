#!/bin/bash
PGHOST="aim-prod-db-1760993976.postgres.database.azure.com"
PGPORT="5432"
PGUSER="aimadmin"
PGPASSWORD="AIM-NewProdDB-2025!@#"
PGDATABASE="identity"
PGSSLMODE="require"

export PGHOST PGPORT PGUSER PGPASSWORD PGDATABASE PGSSLMODE

AGENT_ID="fd924f2f-898f-436d-9ac9-9db353dd8787"

echo "Checking MCP servers for weather-agent-demo..."
psql -c "SELECT id, agent_id, server_name, server_url, status, created_at FROM mcp_servers WHERE agent_id = '$AGENT_ID';"

echo ""
echo "Checking all MCP servers in database..."
psql -c "SELECT COUNT(*) as total_mcps FROM mcp_servers;"
