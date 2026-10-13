#!/usr/bin/env bash
# Release smoke for the TypeScript SDK (@opena2a/aim-sdk): verifyAction
# against a real AIM server, from the tarball a user installs.
#
# Unit tests run the client's sources against mocked responses, so they
# cannot see a client that posts to a route the server never registered.
# The 1.3.1 tarball did exactly that: verifyAction posted to /api/v1/verify,
# the server answered 404, and every unit test passed. This smoke installs
# the packed client into a throwaway consumer project, creates an agent on a
# running backend, and calls verifyAction from the consumer. The signal is
# the server's own record: the call must return a decision (allowed, denied
# or pending), the SDK must report the event ID the server assigned, and
# GET /api/v1/verifications/<id> must read back the same decision.
#
# It also reads the packed dist: the CommonJS and the ESM build must each
# name /api/v1/sdk-api/verifications, and neither may name /api/v1/verify.
#
# The client under test, AIM_SDK_SPEC:
#   unset                       build sdk/typescript and pack it (before a release)
#   @opena2a/aim-sdk@latest     the published package (after a release)
#   ./aim-sdk-x.y.z.tgz         a tarball already packed
#   anything else `npm pack` accepts, including a built SDK directory
#
# The server, AIM_SMOKE_BASE_URL:
#   unset   hermetic: a throwaway Postgres container and a backend built from
#           this tree, on alternative ports, both removed on exit. A second
#           call then switches the organization to strict enforcement and
#           asks for a capability the agent does not hold: it must return
#           'denied', recorded by the server as denied.
#   set     an AIM stack that is already running, for example the quickstart
#           stack at http://localhost:8080. ADMIN_EMAIL and ADMIN_PASSWORD
#           must sign in to it, with the first sign-in password change done.
#           The smoke creates one agent there and deletes it on exit.
#
# Run:
#   sdk/typescript/scripts/release-smoke.sh
#   AIM_SDK_SPEC=@opena2a/aim-sdk@latest sdk/typescript/scripts/release-smoke.sh
#   AIM_SMOKE_BASE_URL=http://localhost:8080 AIM_SDK_SPEC=@opena2a/aim-sdk@latest \
#     sdk/typescript/scripts/release-smoke.sh     (ADMIN_EMAIL and ADMIN_PASSWORD exported)
#
# Exit codes (docs/testing/release-smoke.md):
#   0 = verifyAction returned a decision the server recorded, and the packed
#       client names the route the server serves
#   1 = pre-flight failure (a missing tool or setting)
#   2 = build, pack, install or docker run failed
#   3 = a service failed its health check
#   4 = a setup request failed (sign-in, agent create, credentials read)
#   5 = verifyAction returned no decision (for example a 404), or not the
#       one the step expects
#   6 = the server's record does not match the decision the SDK reported
#   7 = the packed client names the wrong verification route
set -euo pipefail

# Resolve a path-shaped spec before leaving the caller's directory.
SDK_SPEC="${AIM_SDK_SPEC:-}"
if [ -n "$SDK_SPEC" ] && [ -e "$SDK_SPEC" ]; then
    SDK_SPEC="$(cd "$(dirname "$SDK_SPEC")" && pwd)/$(basename "$SDK_SPEC")"
fi

cd "$(dirname "$0")"
SCRIPTS_DIR="$(pwd)"
SDK_DIR="$(cd .. && pwd)"
BACKEND_DIR="$(cd ../../../apps/backend && pwd)"

EXTERNAL_BASE_URL="${AIM_SMOKE_BASE_URL:-}"
SMOKE_BACKEND_PORT="${SMOKE_BACKEND_PORT:-18082}"
SMOKE_POSTGRES_PORT="${SMOKE_POSTGRES_PORT:-55434}"
SMOKE_PG_CONTAINER="${SMOKE_PG_CONTAINER:-aim-sdksmoke-postgres}"
SMOKE_PG_DB="${SMOKE_PG_DB:-identity}"
ADMIN_EMAIL="${ADMIN_EMAIL:-admin@opena2a.org}"

echo "==> TypeScript SDK release smoke: verifyAction against a real server"

echo
echo "==> [1/7] Pre-flight"
TOOLS="curl jq node npm tar"
if [ -z "$EXTERNAL_BASE_URL" ]; then
    TOOLS="$TOOLS docker go openssl"
fi
for tool in $TOOLS; do
    command -v "$tool" >/dev/null || { echo "FAIL: $tool not on PATH"; exit 1; }
done
if [ -n "$EXTERNAL_BASE_URL" ]; then
    [ -n "${ADMIN_PASSWORD:-}" ] || { echo "FAIL: AIM_SMOKE_BASE_URL is set but ADMIN_PASSWORD is not"; exit 1; }
    BASE_URL="${EXTERNAL_BASE_URL%/}"
else
    BASE_URL="http://localhost:${SMOKE_BACKEND_PORT}"
    ADMIN_PASSWORD="${ADMIN_PASSWORD:-Smoke-$(openssl rand -hex 12)-A1}"
    SMOKE_PG_PASSWORD="$(openssl rand -hex 16)"
fi
API="${BASE_URL}/api/v1"
echo "    server : ${BASE_URL}$([ -n "$EXTERNAL_BASE_URL" ] && echo ' (running stack)' || echo ' (hermetic)')"
echo "    client : ${SDK_SPEC:-sdk/typescript, built and packed}"

WORK="$(mktemp -d)"
BACKEND_PID=""
TOKEN=""
AGENT_ID=""
BACKEND_LOG="$WORK/backend.log"
# Secrets never go on a command line, where other processes can read them:
# request bodies go through standard input, the bearer token through a header
# file in the private work directory, and the agent's key through exported
# variables.
AUTH_HEADER="$WORK/auth.header"
auth() { curl -s -H @"$AUTH_HEADER" "$@"; }
cleanup() {
    if [ -n "$EXTERNAL_BASE_URL" ] && [ -n "$AGENT_ID" ] && [ -n "$TOKEN" ]; then
        auth -o /dev/null -X DELETE "${API}/agents/${AGENT_ID}" || true
    fi
    if [ -n "$BACKEND_PID" ] && kill -0 "$BACKEND_PID" 2>/dev/null; then
        kill "$BACKEND_PID" 2>/dev/null || true
        wait "$BACKEND_PID" 2>/dev/null || true
    fi
    if [ -z "$EXTERNAL_BASE_URL" ] && [ "${KEEP_STACKS:-0}" != "1" ]; then
        docker rm -f "$SMOKE_PG_CONTAINER" >/dev/null 2>&1 || true
    fi
    rm -rf "$WORK"
}
trap cleanup EXIT

echo
echo "==> [2/7] Pack the client and read its routes"
if [ -z "$SDK_SPEC" ]; then
    (cd "$SDK_DIR" && npm run build >"$WORK/build.log" 2>&1) \
        || { echo "FAIL: npm run build in sdk/typescript"; tail -30 "$WORK/build.log"; exit 2; }
    SDK_SPEC="$SDK_DIR"
fi
TARBALL_NAME="$(cd "$WORK" && npm pack "$SDK_SPEC" --json --pack-destination "$WORK" 2>"$WORK/pack.log" \
    | jq -r '.[0].filename // empty')" || true
[ -n "$TARBALL_NAME" ] && [ -f "$WORK/$TARBALL_NAME" ] \
    || { echo "FAIL: npm pack $SDK_SPEC"; tail -20 "$WORK/pack.log"; exit 2; }
TARBALL="$WORK/$TARBALL_NAME"
mkdir "$WORK/unpacked"
tar -xzf "$TARBALL" -C "$WORK/unpacked"
echo "    packed $TARBALL_NAME"

# Every file of each module format, not only dist/index.*: since the build
# splits shared code into chunks, the client lives in a chunk that the entry
# files import.
ROUTE_FAILURES=0
DIST="$WORK/unpacked/package/dist"
for format in js mjs; do
    served=$( (grep -rl --include="*.${format}" 'sdk-api/verifications' "$DIST" 2>/dev/null || true) | wc -l | tr -d ' ')
    unserved=$( (grep -rlE --include="*.${format}" "[\"'\`]/api/v1/verify[\"'\`]" "$DIST" 2>/dev/null || true) | wc -l | tr -d ' ')
    echo "    dist/**/*.${format}: ${served} file(s) name /api/v1/sdk-api/verifications, ${unserved} name /api/v1/verify"
    if [ "$served" -lt 1 ] || [ "$unserved" -ne 0 ]; then
        ROUTE_FAILURES=$((ROUTE_FAILURES+1))
    fi
done

echo
echo "==> [3/7] Install the tarball into a consumer project"
mkdir "$WORK/consumer" "$WORK/home"
printf '{"name":"aim-sdk-release-smoke","private":true,"type":"module"}\n' >"$WORK/consumer/package.json"
(cd "$WORK/consumer" && npm install --no-audit --no-fund "$TARBALL" >"$WORK/install.log" 2>&1) \
    || { echo "FAIL: npm install of the tarball"; tail -30 "$WORK/install.log"; exit 2; }
cp "$SCRIPTS_DIR/release-smoke-consumer.mjs" "$WORK/consumer/consumer.mjs"
echo "    installed $(jq -r '.name + "@" + .version' "$WORK/consumer/node_modules/@opena2a/aim-sdk/package.json")"

echo
if [ -n "$EXTERNAL_BASE_URL" ]; then
    echo "==> [4/7] Running stack: skipping the hermetic boot"
else
    echo "==> [4/7] Throwaway Postgres and a backend built from this tree"
    docker rm -f "$SMOKE_PG_CONTAINER" >/dev/null 2>&1 || true
    POSTGRES_PASSWORD="$SMOKE_PG_PASSWORD" docker run -d --name "$SMOKE_PG_CONTAINER" \
        -e POSTGRES_PASSWORD \
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

    (cd "$BACKEND_DIR" && go build -o "$WORK/backend.bin" ./cmd/server && go build -o "$WORK/bootstrap.bin" ./cmd/bootstrap) \
        >"$WORK/go-build.log" 2>&1 || { echo "FAIL: backend build"; tail -30 "$WORK/go-build.log"; exit 2; }

    (
        cd "$BACKEND_DIR"
        export APP_PORT="$SMOKE_BACKEND_PORT" ENVIRONMENT="development"
        export POSTGRES_HOST="localhost" POSTGRES_PORT="$SMOKE_POSTGRES_PORT" POSTGRES_USER="postgres"
        export POSTGRES_PASSWORD="$SMOKE_PG_PASSWORD" POSTGRES_DB="$SMOKE_PG_DB" POSTGRES_SSL_MODE="disable"
        export REDIS_HOST="" OTEL_EXPORTER_OTLP_ENDPOINT=""
        JWT_SECRET="$(openssl rand -hex 32)"
        KEYVAULT_MASTER_KEY="$(openssl rand -base64 32)"
        export JWT_SECRET KEYVAULT_MASTER_KEY
        export ADMIN_EMAIL="$ADMIN_EMAIL" ADMIN_PASSWORD="$ADMIN_PASSWORD"
        "$WORK/backend.bin" >"$BACKEND_LOG" 2>&1 &
        echo $! >"$WORK/backend.pid"
    )
    BACKEND_PID="$(cat "$WORK/backend.pid")"
    echo "    backend pid=$BACKEND_PID"
fi

echo
echo "==> [5/7] Wait for the backend"
attempt=0
while [ $attempt -lt 60 ]; do
    if curl -sf "${BASE_URL}/health" 2>/dev/null | jq -e '.service == "agent-identity-management"' >/dev/null 2>&1; then
        echo "    backend ready"; break
    fi
    if [ -n "$BACKEND_PID" ] && ! kill -0 "$BACKEND_PID" 2>/dev/null; then
        echo "FAIL: backend exited during boot"; tail -60 "$BACKEND_LOG"; exit 3
    fi
    attempt=$((attempt+1)); sleep 2
done
[ $attempt -lt 60 ] || { echo "FAIL: backend not ready after 120s"; [ -f "$BACKEND_LOG" ] && tail -60 "$BACKEND_LOG"; exit 3; }

echo
echo "==> [6/7] Sign in and create an agent"
if [ -z "$EXTERNAL_BASE_URL" ]; then
    (
        cd "$BACKEND_DIR"
        export DATABASE_URL="postgres://postgres:${SMOKE_PG_PASSWORD}@localhost:${SMOKE_POSTGRES_PORT}/${SMOKE_PG_DB}?sslmode=disable"
        export DEFAULT_ADMIN_PASSWORD="$ADMIN_PASSWORD"
        "$WORK/bootstrap.bin" --default --admin-email="$ADMIN_EMAIL" --yes >>"$BACKEND_LOG" 2>&1
    ) || { echo "FAIL: bootstrap --default"; tail -40 "$BACKEND_LOG"; exit 4; }
    # psql inside the container signs in over the local socket, so no password
    # crosses a command line.
    docker exec "$SMOKE_PG_CONTAINER" \
        psql -U postgres -d "$SMOKE_PG_DB" -v ON_ERROR_STOP=1 \
        -c "UPDATE users SET force_password_change = FALSE WHERE email = '${ADMIN_EMAIL}';" >/dev/null
fi

TOKEN=$(ADMIN_EMAIL="$ADMIN_EMAIL" ADMIN_PASSWORD="$ADMIN_PASSWORD" \
    jq -n '{email: env.ADMIN_EMAIL, password: env.ADMIN_PASSWORD}' \
    | curl -s -X POST "${API}/public/login" -H "Content-Type: application/json" --data-binary @- \
    | jq -r '.accessToken // empty')
[ -n "$TOKEN" ] || { echo "FAIL: sign-in as ${ADMIN_EMAIL} returned no accessToken (on a running stack, finish the first sign-in password change first)"; exit 4; }
printf 'Authorization: Bearer %s\n' "$TOKEN" >"$AUTH_HEADER"

AGENT_NAME="sdk-release-smoke-$(date +%s)"
AGENT=$(jq -n --arg n "$AGENT_NAME" \
        '{name:$n, displayName:$n, description:"TypeScript SDK release smoke", agentType:"ai_agent", capabilities:["db:read"]}' \
    | auth -X POST "${API}/agents" -H "Content-Type: application/json" --data-binary @-)
AGENT_ID=$(echo "$AGENT" | jq -r '.id // empty')
ORG_ID=$(echo "$AGENT" | jq -r '.organizationId // empty')
[ -n "$AGENT_ID" ] && [ -n "$ORG_ID" ] || { echo "FAIL: agent create returned no id or organizationId"; echo "$AGENT" | jq -c 'del(.privateKey)' 2>/dev/null | head -c 600; exit 4; }

# The private key stays in this process and the consumer's environment; it is
# never printed.
CREDENTIALS=$(auth "${API}/agents/${AGENT_ID}/credentials")
AGENT_PUBLIC_KEY=$(echo "$CREDENTIALS" | jq -r '.publicKey // empty')
AGENT_PRIVATE_KEY=$(echo "$CREDENTIALS" | jq -r '.privateKey // empty')
CREDENTIALS=""
[ -n "$AGENT_PUBLIC_KEY" ] && [ -n "$AGENT_PRIVATE_KEY" ] || { echo "FAIL: agent credentials read returned no key pair"; exit 4; }
echo "    agent ${AGENT_NAME} (${AGENT_ID}) holds db:read"

# verify_action <capability>: one verifyAction call from the consumer. Sets
# ANSWER to the consumer's JSON line and DECISION to its decision; exits 5
# when the call returned no decision.
verify_action() {
    set +e
    ANSWER=$(
        cd "$WORK/consumer" || exit 1
        # Only this agent's identity: no AIM_* setting of the caller's leaks in.
        for name in $(compgen -e | grep '^AIM_'); do unset "$name"; done
        export HOME="$WORK/home"
        export AIM_BASE_URL="$BASE_URL" AIM_AGENT_ID="$AGENT_ID" AIM_ORGANIZATION_ID="$ORG_ID"
        export AIM_PUBLIC_KEY="$AGENT_PUBLIC_KEY" AIM_PRIVATE_KEY="$AGENT_PRIVATE_KEY"
        export SMOKE_ACTION="$1"
        node consumer.mjs 2>"$WORK/consumer.err"
    )
    local code=$?
    set -e
    echo "    verifyAction('$1'): ${ANSWER:-<no output>}"
    if [ "$code" -ne 0 ] || [ "$(echo "$ANSWER" | jq -r '.outcome // empty' 2>/dev/null)" != "decision" ]; then
        tail -5 "$WORK/consumer.err" | sed 's/^/    /'
        echo "FAIL: verifyAction('$1') returned no decision"
        [ "$ROUTE_FAILURES" -eq 0 ] || echo "      and the packed client names the wrong verification route (see step 2)"
        exit 5
    fi
    DECISION=$(echo "$ANSWER" | jq -r '.decision')
}

# expect_record: the server's record of the event the SDK reported must read
# back the SDK's decision. Exits 6 when it does not.
expect_record() {
    local event recorded_as record status
    event=$(echo "$ANSWER" | jq -r '.eventId // empty')
    case "$DECISION" in
        allowed) recorded_as="approved" ;;
        denied) recorded_as="denied" ;;
        pending) recorded_as="pending" ;;
        *) echo "FAIL: unknown decision '$DECISION'"; exit 5 ;;
    esac
    [ -n "$event" ] || { echo "FAIL: verifyAction reported no event ID for its '$DECISION' decision"; exit 6; }
    record=$(auth "${API}/verifications/${event}")
    status=$(echo "$record" | jq -r '.status // empty' 2>/dev/null)
    echo "    server record ${event}: ${status:-<none>}"
    [ "$status" = "$recorded_as" ] \
        || { echo "FAIL: the SDK reported '$DECISION' but the server recorded '${status:-nothing}'"; echo "$record" | head -c 600; exit 6; }
}

echo
echo "==> [7/7] verifyAction from the consumer"
verify_action "db:read"
expect_record
SUMMARY="db:read ${DECISION}"

# A denial reaches the client as a 403 whose body carries the decision, a
# path a mocked response cannot prove. It needs strict enforcement, which is
# an organization-wide setting, so it runs only on the hermetic stack.
if [ -z "$EXTERNAL_BASE_URL" ]; then
    code=$(jq -n '{enforcementMode: "strict"}' \
        | auth -o /dev/null -w '%{http_code}' -X PUT "${API}/admin/enforcement-settings" \
            -H "Content-Type: application/json" --data-binary @-)
    [ "$code" = "200" ] || { echo "FAIL: setting strict enforcement answered HTTP $code"; exit 4; }
    echo "    enforcement mode: strict"
    verify_action "db:write"
    [ "$DECISION" = "denied" ] \
        || { echo "FAIL: in strict mode verifyAction('db:write') for an agent without it returned '$DECISION', not 'denied'"; exit 5; }
    expect_record
    SUMMARY="$SUMMARY, db:write ${DECISION} (strict)"
fi
AGENT_PRIVATE_KEY=""

if [ "$ROUTE_FAILURES" -ne 0 ]; then
    echo "FAIL: the packed client names the wrong verification route (see step 2)"
    exit 7
fi

echo
echo "==> PASS: $(echo "$ANSWER" | jq -r '"@opena2a/aim-sdk@" + .sdkVersion'): verifyAction returned a decision the server recorded: ${SUMMARY}"
