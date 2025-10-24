#!/bin/bash

SDK_TOKEN_ID=$(cat /Users/decimai/.aim/credentials.json | jq -r '.sdk_token_id')

echo "Checking SDK token: $SDK_TOKEN_ID"
echo ""

PGPASSWORD=postgres psql -h localhost -U postgres -d identity << EOF
SELECT
    id,
    token_id,
    LEFT(token_hash, 30) as token_hash_prefix,
    revoked_at IS NULL as is_active,
    last_used_at,
    created_at,
    expires_at
FROM sdk_tokens
WHERE token_id = '$SDK_TOKEN_ID';
EOF
