#!/bin/bash
PGHOST="aim-prod-db-1760993976.postgres.database.azure.com"
PGPORT="5432"
PGUSER="aimadmin"
PGPASSWORD="AIM-NewProdDB-2025!@#"
PGDATABASE="identity"
PGSSLMODE="require"

export PGHOST PGPORT PGUSER PGPASSWORD PGDATABASE PGSSLMODE

TOKEN_HASH="db6bb36fa35ea285cf25b5356a3b518c426369db73b4ea10846ae00665bc2919"

echo "Checking if token hash exists in database..."
echo "Hash: $TOKEN_HASH"
echo ""
psql -c "SELECT token_id, created_at, revoked_at, metadata FROM sdk_tokens WHERE token_hash = '$TOKEN_HASH';"
