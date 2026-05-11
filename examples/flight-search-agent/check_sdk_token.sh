#!/usr/bin/env bash
#
# check_sdk_token.sh — diagnostic helper for the flight-search-agent demo.
#
# Reads the SDK token ID from ~/.aim/credentials.json and queries the
# `sdk_tokens` table on the local AIM Postgres to show whether the token
# is active, when it was last used, and when it expires. Run this when
# the agent reports "SDK TOKEN EXPIRED" or registration fails.
#
# Requires: jq, psql, a local AIM Postgres running on localhost:5432
# with database `identity` and user `postgres` (the dev compose default).
#
# Configuration via env vars (override anything you've changed locally):
#   AIM_CREDENTIALS_FILE  default ${HOME}/.aim/credentials.json
#   AIM_PG_HOST           default localhost
#   AIM_PG_USER           default postgres
#   AIM_PG_PASSWORD       default postgres
#   AIM_PG_DB             default identity
set -euo pipefail

CREDENTIALS_FILE="${AIM_CREDENTIALS_FILE:-${HOME}/.aim/credentials.json}"
PG_HOST="${AIM_PG_HOST:-localhost}"
PG_USER="${AIM_PG_USER:-postgres}"
PG_PASSWORD="${AIM_PG_PASSWORD:-postgres}"
PG_DB="${AIM_PG_DB:-identity}"

if [ ! -f "$CREDENTIALS_FILE" ]; then
    echo "❌ Credentials file not found: $CREDENTIALS_FILE"
    echo "   Either:"
    echo "     - Run \`python3 flight_agent.py\` once to register and create the file, or"
    echo "     - Download a fresh SDK from the AIM dashboard (Settings → SDK Downloads), or"
    echo "     - Override the path: AIM_CREDENTIALS_FILE=/path/to/credentials.json $0"
    exit 1
fi

for tool in jq psql; do
    if ! command -v "$tool" >/dev/null; then
        echo "❌ Required tool not found on PATH: $tool"
        exit 1
    fi
done

SDK_TOKEN_ID=$(jq -r '.sdk_token_id // empty' "$CREDENTIALS_FILE")
if [ -z "$SDK_TOKEN_ID" ]; then
    echo "❌ No \`sdk_token_id\` field in $CREDENTIALS_FILE"
    echo "   The file may be from a pre-token-based SDK release."
    exit 1
fi

echo "Checking SDK token: $SDK_TOKEN_ID"
echo ""

# Pass the token id via psql's -v variable binding so it is properly
# quoted by psql, not by bash heredoc interpolation. The credentials file
# is written by the SDK itself so the risk is theoretical, but using
# -v keeps the query injection-safe regardless of how the file got there.
PGPASSWORD="$PG_PASSWORD" psql \
    -h "$PG_HOST" \
    -U "$PG_USER" \
    -d "$PG_DB" \
    -v sdk_token_id="$SDK_TOKEN_ID" <<'EOF'
SELECT
    id,
    token_id,
    LEFT(token_hash, 30) as token_hash_prefix,
    revoked_at IS NULL as is_active,
    last_used_at,
    created_at,
    expires_at
FROM sdk_tokens
WHERE token_id = :'sdk_token_id';
EOF
