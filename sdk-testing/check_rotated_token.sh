#!/bin/bash
PGHOST="aim-prod-db-1760993976.postgres.database.azure.com"
PGPORT="5432"
PGUSER="aimadmin"
PGPASSWORD="AIM-NewProdDB-2025!@#"
PGDATABASE="identity"
PGSSLMODE="require"

export PGHOST PGPORT PGUSER PGPASSWORD PGDATABASE PGSSLMODE

echo "Checking for ROTATED token: 702f3493-edcc-4243-843d-7050158c2ed8"
psql -c "SELECT token_id, created_at, revoked_at, metadata FROM sdk_tokens WHERE token_id = '702f3493-edcc-4243-843d-7050158c2ed8';"
