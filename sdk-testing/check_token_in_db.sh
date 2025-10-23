#!/bin/bash
PGHOST="aim-prod-db-1760993976.postgres.database.azure.com"
PGPORT="5432"
PGUSER="aimadmin"
PGPASSWORD="AIM-NewProdDB-2025!@#"
PGDATABASE="identity"
PGSSLMODE="require"

export PGHOST PGPORT PGUSER PGPASSWORD PGDATABASE PGSSLMODE

echo "Checking for token: c98ec2c1-ebec-4c99-b732-e13b2431311a"
psql -c "SELECT token_id, created_at, revoked_at, revoke_reason FROM sdk_tokens WHERE token_id = 'c98ec2c1-ebec-4c99-b732-e13b2431311a';"
