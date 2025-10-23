#!/bin/bash
PGHOST="aim-prod-db-1760993976.postgres.database.azure.com"
PGPORT="5432"
PGUSER="aimadmin"
PGPASSWORD="AIM-NewProdDB-2025!@#"
PGDATABASE="identity"
PGSSLMODE="require"

export PGHOST PGPORT PGUSER PGPASSWORD PGDATABASE PGSSLMODE

AGENT_ID="fd924f2f-898f-436d-9ac9-9db353dd8787"

echo "Checking MCP servers registered by weather-agent-demo..."
psql -c "SELECT id, name, url, status, is_verified, registered_by_agent, created_at 
         FROM mcp_servers 
         WHERE registered_by_agent = '$AGENT_ID';"

echo ""
echo "Checking for weather-related MCP servers..."
psql -c "SELECT id, name, url, status, registered_by_agent 
         FROM mcp_servers 
         WHERE name ILIKE '%weather%' OR url ILIKE '%weather%';"
