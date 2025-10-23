#!/bin/bash
PGHOST="aim-prod-db-1760993976.postgres.database.azure.com"
PGPORT="5432"
PGUSER="aimadmin"
PGPASSWORD="AIM-NewProdDB-2025!@#"
PGDATABASE="identity"
PGSSLMODE="require"

export PGHOST PGPORT PGUSER PGPASSWORD PGDATABASE PGSSLMODE

echo "Searching for token hash: c7368e34a0dbd13eff95e88a6657d51f0024e036f767c2e299508d23f789e7c9"
psql -c "SELECT token_hash, token_id, created_at, revoked_at FROM sdk_tokens WHERE token_hash = 'c7368e34a0dbd13eff95e88a6657d51f0024e036f767c2e299508d23f789e7c9';"
