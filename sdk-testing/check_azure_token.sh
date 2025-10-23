#!/bin/bash
PGHOST="aim-prod-db-1760993976.postgres.database.azure.com"
PGPORT="5432"
PGUSER="aimadmin"
PGPASSWORD="AIM-NewProdDB-2025!@#"
PGDATABASE="identity"
PGSSLMODE="require"

export PGHOST PGPORT PGUSER PGPASSWORD PGDATABASE PGSSLMODE

echo "Checking for token: 32d359de-42df-4583-ba30-0230c3ae1140"
psql -c "SELECT token_id, created_at, expires_at, revoked_at, revoke_reason FROM sdk_tokens WHERE token_id = '32d359de-42df-4583-ba30-0230c3ae1140';"
