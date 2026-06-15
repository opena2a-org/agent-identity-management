#!/usr/bin/env bash
# Real-backend REST smoke for the dashboard list endpoints.
#
# Boots a throwaway Postgres + a fresh AIM backend binary (NO OTel stack —
# this smoke is about the HTTP/SQL contract, not telemetry), logs in as
# admin, and drives the list endpoints that enrich each row from related
# tables. It exists to catch the bug class that unit tests CANNOT: the
# bulk-prefetch SQL added to kill the per-row N+1 (GetMCPServerTagsByServerIDs,
# MCPServerCapabilityRepository.GetByServerIDs, AgentService.GetAgentsByIDs)
# only ever runs against a real schema. A wrong column/table name compiles,
# passes every mock-backed unit test, and — because the list handlers
# deliberately degrade to empty on query error rather than 500 — would NOT
# even surface as a non-200. So a 200 check is not enough: this smoke seeds a
# tag and a capability on a server and asserts they come BACK through the bulk
# query. A broken query shows up as a missing tag / zero toolCount, which is
# the externally-observable signal per docs/testing/release-smoke.md.
#
# Hermetic: own throwaway Postgres on an alt port, own backend on an alt
# port, torn down on exit. Does not touch a developer's running stack.
#
# What it does, in order:
#   1. Pre-flight: docker, curl, jq, go, openssl on PATH.
#   2. Throwaway Postgres (aim-listsmoke-postgres) on $SMOKE_POSTGRES_PORT.
#   3. Build + run the backend on $SMOKE_BACKEND_PORT. Wait on /health.
#   4. Seed admin via bootstrap; reset force_password_change; login.
#   5. MCP list contract: create tag -> create server with that tag ->
#      seed one 'tool' capability row -> GET /api/v1/mcp-servers and assert
#      the server comes back WITH the tag and toolCount==1 (proves both
#      bulk queries return correct, keyed data).
#   6. Pending-verifications contract: GET /api/v1/admin/verifications/pending
#      and assert 200 + the documented response shape (verifications array +
#      pagination object) — exercises the bulk agent-name prefetch path.
#   7. Teardown.
#
# Run from anywhere:   apps/backend/deployments/smoke/rest-list-endpoints-smoke.sh
#
# Exit codes:
#   0 = all list-endpoint contracts verified
#   1 = pre-flight failure (missing tool)
#   2 = docker run / build failed
#   3 = a service failed health check
#   4 = an HTTP step failed (login / create / list)
#   5 = a list response was missing the enriched signal (bulk query regression)
set -euo pipefail

cd "$(dirname "$0")"
SMOKE_DIR="$(pwd)"
BACKEND_DIR="$(cd ../.. && pwd)"

SMOKE_BACKEND_PORT="${SMOKE_BACKEND_PORT:-18081}"
SMOKE_POSTGRES_PORT="${SMOKE_POSTGRES_PORT:-55433}"
SMOKE_PG_CONTAINER="${SMOKE_PG_CONTAINER:-aim-listsmoke-postgres}"
SMOKE_PG_PASSWORD="${SMOKE_PG_PASSWORD:-listsmoke_postgres_password}"
SMOKE_PG_DB="${SMOKE_PG_DB:-identity}"

ADMIN_EMAIL="${ADMIN_EMAIL:-admin@opena2a.org}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-AIM2025!ListSmoke}"
JWT_SECRET="${JWT_SECRET:-$(openssl rand -hex 32)}"

API="http://localhost:${SMOKE_BACKEND_PORT}/api/v1"

echo "==> AIM REST list-endpoints smoke (hermetic)"
echo "    Backend  : localhost:${SMOKE_BACKEND_PORT}"
echo "    Postgres : localhost:${SMOKE_POSTGRES_PORT} (container ${SMOKE_PG_CONTAINER})"

echo
echo "==> [1/6] Pre-flight"
for tool in docker curl jq go openssl; do
    command -v "$tool" >/dev/null || { echo "FAIL: $tool not on PATH"; exit 1; }
done

BACKEND_PID=""
BACKEND_BIN="$SMOKE_DIR/.list-smoke-backend.bin"
BOOTSTRAP_BIN="$SMOKE_DIR/.list-smoke-bootstrap.bin"
BACKEND_LOG="$SMOKE_DIR/.list-smoke-backend.log"
cleanup() {
    if [ -n "${BACKEND_PID:-}" ] && kill -0 "$BACKEND_PID" 2>/dev/null; then
        kill "$BACKEND_PID" 2>/dev/null || true
        wait "$BACKEND_PID" 2>/dev/null || true
    fi
    rm -f "$BACKEND_BIN" "$BOOTSTRAP_BIN"
    if [ "${KEEP_STACKS:-0}" != "1" ]; then
        docker rm -f "$SMOKE_PG_CONTAINER" >/dev/null 2>&1 || true
    fi
}
trap cleanup EXIT

psql_smoke() {
    docker exec -e PGPASSWORD="$SMOKE_PG_PASSWORD" "$SMOKE_PG_CONTAINER" \
        psql -U postgres -d "$SMOKE_PG_DB" -v ON_ERROR_STOP=1 "$@"
}

echo
echo "==> [2/6] Throwaway Postgres"
docker rm -f "$SMOKE_PG_CONTAINER" >/dev/null 2>&1 || true
docker run -d --name "$SMOKE_PG_CONTAINER" \
    -e POSTGRES_PASSWORD="$SMOKE_PG_PASSWORD" \
    -e POSTGRES_DB="$SMOKE_PG_DB" \
    -p "127.0.0.1:${SMOKE_POSTGRES_PORT}:5432" \
    timescale/timescaledb:latest-pg16 >/dev/null || { echo "FAIL: docker run postgres"; exit 2; }
attempt=0
while [ $attempt -lt 30 ]; do
    if docker exec "$SMOKE_PG_CONTAINER" pg_isready -U postgres -d "$SMOKE_PG_DB" >/dev/null 2>&1; then
        echo "    postgres ready"; break
    fi
    attempt=$((attempt+1)); sleep 2
done
[ $attempt -lt 30 ] || { echo "FAIL: postgres not ready"; exit 3; }

echo
echo "==> [3/6] Build + run backend (no OTel)"
(
    cd "$BACKEND_DIR"
    go build -o "$BACKEND_BIN" ./cmd/server || exit 2
    go build -o "$BOOTSTRAP_BIN" ./cmd/bootstrap || exit 2
) || { echo "FAIL: backend build"; exit 2; }

(
    cd "$BACKEND_DIR"
    env \
        APP_PORT="$SMOKE_BACKEND_PORT" \
        ENVIRONMENT="development" \
        POSTGRES_HOST="localhost" \
        POSTGRES_PORT="$SMOKE_POSTGRES_PORT" \
        POSTGRES_USER="postgres" \
        POSTGRES_PASSWORD="$SMOKE_PG_PASSWORD" \
        POSTGRES_DB="$SMOKE_PG_DB" \
        POSTGRES_SSL_MODE="disable" \
        REDIS_HOST="" \
        JWT_SECRET="$JWT_SECRET" \
        ADMIN_EMAIL="$ADMIN_EMAIL" \
        ADMIN_PASSWORD="$ADMIN_PASSWORD" \
        OTEL_EXPORTER_OTLP_ENDPOINT="" \
        "$BACKEND_BIN" >"$BACKEND_LOG" 2>&1 &
    echo $! > "$SMOKE_DIR/.list-smoke-backend.pid"
)
BACKEND_PID="$(cat "$SMOKE_DIR/.list-smoke-backend.pid")"
rm -f "$SMOKE_DIR/.list-smoke-backend.pid"
echo "    backend pid=$BACKEND_PID, logs at $BACKEND_LOG"

attempt=0
while [ $attempt -lt 60 ]; do
    if curl -sf "http://localhost:${SMOKE_BACKEND_PORT}/health" 2>/dev/null \
        | jq -e '.service == "agent-identity-management"' >/dev/null 2>&1; then
        echo "    backend ready"; break
    fi
    if ! kill -0 "$BACKEND_PID" 2>/dev/null; then
        echo "FAIL: backend exited during boot"; tail -60 "$BACKEND_LOG"; exit 3
    fi
    attempt=$((attempt+1)); sleep 2
done
[ $attempt -lt 60 ] || { echo "FAIL: backend not ready after 120s"; tail -60 "$BACKEND_LOG"; exit 3; }

echo
echo "==> [4/6] Seed admin + login"
(
    cd "$BACKEND_DIR"
    env DATABASE_URL="postgres://postgres:${SMOKE_PG_PASSWORD}@localhost:${SMOKE_POSTGRES_PORT}/${SMOKE_PG_DB}?sslmode=disable" \
        "$BOOTSTRAP_BIN" --default --admin-email="$ADMIN_EMAIL" --admin-password="$ADMIN_PASSWORD" --yes \
        >>"$BACKEND_LOG" 2>&1
) || { echo "FAIL: bootstrap --default"; tail -40 "$BACKEND_LOG"; exit 4; }
psql_smoke -c "UPDATE users SET force_password_change = FALSE WHERE email = '${ADMIN_EMAIL}';" >/dev/null

TOKEN=$(curl -sf -X POST "${API}/public/login" -H "Content-Type: application/json" \
    -d "$(jq -n --arg e "$ADMIN_EMAIL" --arg p "$ADMIN_PASSWORD" '{email:$e, password:$p}')" \
    | jq -r '.accessToken // empty')
[ -n "$TOKEN" ] && [ "$TOKEN" != "null" ] || { echo "FAIL: login returned no accessToken"; exit 4; }
echo "    login OK"

auth() { curl -s -H "Authorization: Bearer ${TOKEN}" "$@"; }

echo
echo "==> [5/6] MCP list contract (bulk tags + capabilities)"
# Create a tag, then a server carrying it, so the bulk tag join has something
# to return. A broken GetMCPServerTagsByServerIDs would drop this tag.
TAG_ID=$(auth -X POST "${API}/tags" -H "Content-Type: application/json" \
    -d '{"key":"env","value":"smoke","category":"environment"}' | jq -r '.id // .tag.id // empty')
[ -n "$TAG_ID" ] && [ "$TAG_ID" != "null" ] || { echo "FAIL: tag create returned no id"; exit 4; }

SERVER_NAME="smoke-mcp-$(date +%s)"
SERVER_ID=$(auth -X POST "${API}/mcp-servers" -H "Content-Type: application/json" \
    -d "$(jq -n --arg n "$SERVER_NAME" --arg t "$TAG_ID" \
        '{name:$n, url:"https://smoke.example.com/mcp", description:"list-smoke", tagIds:[$t]}')" \
    | jq -r '.id // empty')
[ -n "$SERVER_ID" ] && [ "$SERVER_ID" != "null" ] || { echo "FAIL: mcp create returned no id"; exit 4; }
echo "    created tag=${TAG_ID} server=${SERVER_ID}"

# Seed one active 'tool' capability directly (no public create-capability
# route; production populates this via the attestation pipeline). A broken
# GetByServerIDs would make toolCount come back 0.
psql_smoke -c "INSERT INTO mcp_server_capabilities
    (id, mcp_server_id, name, capability_type, is_active, detected_at, created_at, updated_at)
    VALUES (gen_random_uuid(), '${SERVER_ID}', 'smoke-tool', 'tool', true, now(), now(), now());" >/dev/null
echo "    seeded one 'tool' capability"

LIST=$(auth "${API}/mcp-servers/")
ROW=$(echo "$LIST" | jq -c --arg id "$SERVER_ID" '.mcpServers[]? | select(.id == $id)')
[ -n "$ROW" ] || { echo "FAIL: created server not in list response"; echo "$LIST" | head -c 800; exit 5; }

TAG_BACK=$(echo "$ROW" | jq -r --arg t "$TAG_ID" '[.tags[]? | select(.id == $t)] | length')
TOOLS_BACK=$(echo "$ROW" | jq -r '.toolCount')
if [ "$TAG_BACK" != "1" ]; then
    echo "FAIL: bulk tag query regression — seeded tag absent from list row"
    echo "    row: $ROW"; exit 5
fi
if [ "$TOOLS_BACK" != "1" ]; then
    echo "FAIL: bulk capability query regression — toolCount=${TOOLS_BACK}, expected 1"
    echo "    row: $ROW"; exit 5
fi
echo "    list row carries the bulk-prefetched tag (1) and toolCount=${TOOLS_BACK}"

echo
echo "==> [6/6] Pending-verifications contract"
PENDING_OUT="$(mktemp -t list-smoke-pending.XXXXXX)"
PENDING_CODE=$(auth -o "$PENDING_OUT" -w '%{http_code}' "${API}/admin/verifications/pending")
[ "$PENDING_CODE" = "200" ] || { echo "FAIL: pending verifications returned HTTP ${PENDING_CODE}"; cat "$PENDING_OUT"; rm -f "$PENDING_OUT"; exit 4; }
if ! jq -e '(.verifications | type == "array") and (.pagination | type == "object")' "$PENDING_OUT" >/dev/null 2>&1; then
    echo "FAIL: pending verifications response missing verifications[]/pagination shape"
    head -c 800 "$PENDING_OUT"; rm -f "$PENDING_OUT"; exit 5
fi
rm -f "$PENDING_OUT"
echo "    pending verifications 200 + shape OK"

echo
echo "==> PASS: list-endpoint bulk-enrichment contracts verified against a real backend"
echo "    GET /mcp-servers              -> tag + toolCount returned via bulk query"
echo "    GET /admin/verifications/pending -> 200 + verifications[]/pagination"
