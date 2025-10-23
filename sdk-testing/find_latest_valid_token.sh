#!/bin/bash
PGHOST="aim-prod-db-1760993976.postgres.database.azure.com"
PGPORT="5432"
PGUSER="aimadmin"
PGPASSWORD="AIM-NewProdDB-2025!@#"
PGDATABASE="identity"
PGSSLMODE="require"

export PGHOST PGPORT PGUSER PGPASSWORD PGDATABASE PGSSLMODE

echo "Finding all SDK tokens for admin user (sorted by creation time)..."
psql -c "SELECT token_id, created_at, revoked_at, 
        CASE 
          WHEN revoked_at IS NULL THEN 'VALID'
          ELSE 'REVOKED'
        END as status,
        metadata 
        FROM sdk_tokens 
        WHERE user_id = 'a0000000-0000-0000-0000-000000000002' 
        ORDER BY created_at DESC 
        LIMIT 10;"
