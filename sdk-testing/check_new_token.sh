#!/bin/bash
PGHOST="aim-prod-db-1760993976.postgres.database.azure.com"
PGPORT="5432"
PGUSER="aimadmin"
PGPASSWORD="AIM-NewProdDB-2025!@#"
PGDATABASE="identity"
PGSSLMODE="require"

export PGHOST PGPORT PGUSER PGPASSWORD PGDATABASE PGSSLMODE

echo "Checking for NEW token: a1182816-08f6-4e44-a49f-8c9eeb6fd1ae"
psql -c "SELECT token_id, created_at, revoked_at, revoke_reason FROM sdk_tokens WHERE token_id = 'a1182816-08f6-4e44-a49f-8c9eeb6fd1ae';"
