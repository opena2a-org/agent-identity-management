#!/bin/bash
PGHOST="aim-prod-db-1760993976.postgres.database.azure.com"
PGPORT="5432"
PGUSER="aimadmin"
PGPASSWORD="AIM-NewProdDB-2025!@#"
PGDATABASE="identity"
PGSSLMODE="require"

export PGHOST PGPORT PGUSER PGPASSWORD PGDATABASE PGSSLMODE

echo "Searching for weather-agent-demo in database..."
psql -c "SELECT id, name, description, status, trust_score, created_at FROM agents WHERE name LIKE '%weather%' ORDER BY created_at DESC LIMIT 5;"
