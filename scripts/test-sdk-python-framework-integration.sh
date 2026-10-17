#!/usr/bin/env bash
# test-sdk-python-framework-integration.sh — run the Python SDK's LangChain,
# CrewAI and MCP adapter tests against a real backend.
#
# Usage:
#   DATABASE_URL='postgres://<user>[:<password>]@<host>[:<port>]/<db>?sslmode=disable' \
#   PYTHON=<interpreter> scripts/test-sdk-python-framework-integration.sh [SDK_DIR]
#
#   SDK_DIR  the sdk/python directory pytest runs in (default: this
#            repository's sdk/python). The backend is always built from this
#            repository's apps/backend.
#   PYTHON   an interpreter with the SDK installed with its dev, mcp,
#            langchain and crewai extras (default: python3).
#
# DATABASE_URL must name an empty database. `aim-bootstrap --default` leaves
# an admin that already exists untouched, and the tests sign in with the
# password this run generates.
#
# The run:
#   1. refuses to start when a stored AIM sign-in could reach the tests: the
#      SDK reads one from a .aim entry in its working directory or any
#      directory above it, so a .aim entry in SDK_DIR or above stops the run;
#   2. generates JWT_SECRET, KEYVAULT_MASTER_KEY and the admin password, and
#      prints none of them;
#   3. builds cmd/server, cmd/migrate and cmd/bootstrap, runs the migrations,
#      seeds the admin and starts the server on a free port, then waits for
#      /health/ready;
#   4. runs `python -m pytest -m integration` on the three adapter test files
#      with AIM_URL set, a fresh HOME, no inherited environment, and
#      HTTP(S)_PROXY pointing at a closed port, so only the loopback backend
#      is reachable;
#   5. stops the server whatever the outcome, and exits non-zero when a step
#      fails or pytest reports a failure, an error or a skip, or runs no test.
#
# Logs of a failed run stay in the directory the last line names.

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
backend_dir="$repo_root/apps/backend"
sdk_dir="${1:-$repo_root/sdk/python}"
python_bin="${PYTHON:-python3}"
admin_email="admin@opena2a.org"
test_files=(
  tests/test_langchain_integration.py
  tests/test_crewai_integration.py
  tests/test_mcp_integration.py
)

fail() {
  echo "test-sdk-python-framework-integration: $*" >&2
  exit 1
}

# --- 1. Preconditions ------------------------------------------------------

[ -d "$sdk_dir" ] || fail "SDK_DIR $sdk_dir is not a directory"
sdk_dir="$(cd "$sdk_dir" && pwd)"
[ -f "$sdk_dir/setup.py" ] || fail "SDK_DIR $sdk_dir holds no setup.py"
for f in "${test_files[@]}"; do
  [ -f "$sdk_dir/$f" ] || fail "SDK_DIR $sdk_dir holds no $f"
done

dir="$sdk_dir"
while :; do
  if [ -e "$dir/.aim" ]; then
    fail "refusing to run: $dir holds a .aim entry, and the SDK under test reads a stored sign-in from a .aim entry in its working directory or any directory above it.
  Verify: ls -d '$dir/.aim'
  Fix: run the tests from a clone outside it: d=\"\$(mktemp -d)\" && git clone -q '$repo_root' \"\$d/aim\" && '$repo_root/scripts/test-sdk-python-framework-integration.sh' \"\$d/aim/sdk/python\""
  fi
  [ "$dir" = "/" ] && break
  dir="$(dirname "$dir")"
done

[ -n "${DATABASE_URL:-}" ] || fail "DATABASE_URL is not set"
url_pattern='^postgres(ql)?://([^:@/]+)(:([^@]*))?@([^:/?]+)(:([0-9]+))?/([^?]+)(\?(.*))?$'
[[ "$DATABASE_URL" =~ $url_pattern ]] || fail "DATABASE_URL is not of the form postgres://<user>[:<password>]@<host>[:<port>]/<db>[?<options>]"
pg_user="${BASH_REMATCH[2]}"
pg_password="${BASH_REMATCH[4]}"
pg_host="${BASH_REMATCH[5]}"
pg_port="${BASH_REMATCH[7]:-5432}"
pg_db="${BASH_REMATCH[8]}"
pg_options="${BASH_REMATCH[10]}"
pg_sslmode="disable"
if [[ "&$pg_options" =~ \&sslmode=([^&]+) ]]; then
  pg_sslmode="${BASH_REMATCH[1]}"
fi

for tool in go openssl curl "$python_bin"; do
  command -v "$tool" >/dev/null 2>&1 || fail "$tool is required but not found in PATH"
done
python_bin="$(command -v "$python_bin")"

# --- 2. Ephemeral secrets (never printed) ----------------------------------

# JWT_SECRET: at least 32 characters. KEYVAULT_MASTER_KEY: base64 of exactly
# 32 bytes. The admin password must hold upper, lower, digit and special.
jwt_secret="$(openssl rand -hex 32)"
keyvault_master_key="$(openssl rand -base64 32)"
admin_password="$(openssl rand -base64 18 | tr -d '/+=' | head -c 24)Aa1!"

work="$(mktemp -d "${TMPDIR:-/tmp}/aim-sdk-python-framework.XXXXXX")"
logs="$work/logs"
mkdir -p "$logs" "$work/bin" "$work/home"
server_pid=""
status=1

cleanup() {
  if [ -n "$server_pid" ] && kill -0 "$server_pid" 2>/dev/null; then
    kill "$server_pid" 2>/dev/null || true
    wait "$server_pid" 2>/dev/null || true
  fi
  rm -rf "${work:?}/bin" "${work:?}/home"
  if [ "$status" -eq 0 ]; then
    rm -rf "${work:?}"
  else
    echo "test-sdk-python-framework-integration: logs kept in $logs" >&2
  fi
}
trap cleanup EXIT

# The backend processes get exactly these variables, nothing inherited.
# ENVIRONMENT=test raises both rate limits tenfold, as in the CI end-to-end
# job; outside the CI workflows only a test such as this one sets it.
backend_env=(
  env -i
  PATH="$PATH"
  HOME="$work/home"
  ENVIRONMENT=test
  DATABASE_URL="$DATABASE_URL"
  POSTGRES_HOST="$pg_host"
  POSTGRES_PORT="$pg_port"
  POSTGRES_USER="$pg_user"
  POSTGRES_PASSWORD="$pg_password"
  POSTGRES_DB="$pg_db"
  POSTGRES_SSL_MODE="$pg_sslmode"
  JWT_SECRET="$jwt_secret"
  KEYVAULT_MASTER_KEY="$keyvault_master_key"
  DEFAULT_ADMIN_PASSWORD="$admin_password"
)

# --- 3. Backend -------------------------------------------------------------

echo "Building the backend"
(cd "$backend_dir" && go build -o "$work/bin/aim-server" ./cmd/server \
  && go build -o "$work/bin/aim-migrate" ./cmd/migrate \
  && go build -o "$work/bin/aim-bootstrap" ./cmd/bootstrap) \
  || fail "the backend did not build"

# cmd/migrate reads the migrations directory relative to apps/backend.
echo "Running migrations"
(cd "$backend_dir" && "${backend_env[@]}" "$work/bin/aim-migrate" >"$logs/migrate.log" 2>&1) \
  || fail "migrations failed (see $logs/migrate.log)"

echo "Seeding the admin"
(cd "$backend_dir" && "${backend_env[@]}" "$work/bin/aim-bootstrap" --default --admin-email "$admin_email" \
  >"$logs/bootstrap.log" 2>&1) \
  || fail "aim-bootstrap --default failed (see $logs/bootstrap.log)"

port=""
for _ in $(seq 1 50); do
  candidate=$((20000 + RANDOM % 10000))
  if ! (exec 3<>"/dev/tcp/127.0.0.1/$candidate") 2>/dev/null; then
    port="$candidate"
    break
  fi
done
[ -n "$port" ] || fail "found no free port in 20000-29999"
aim_url="http://127.0.0.1:$port"

echo "Starting the backend on port $port"
cd "$backend_dir"
"${backend_env[@]}" APP_PORT="$port" "$work/bin/aim-server" >"$logs/server.log" 2>&1 &
server_pid=$!
cd "$repo_root"

ready=""
for _ in $(seq 1 90); do
  kill -0 "$server_pid" 2>/dev/null || fail "the backend exited before it was ready (see $logs/server.log)"
  if curl -fsS "$aim_url/health/ready" >/dev/null 2>&1; then
    ready=1
    break
  fi
  sleep 1
done
[ -n "$ready" ] || fail "the backend was not ready within 90s (see $logs/server.log)"
echo "Backend ready at $aim_url"

# --- 4. Tests ---------------------------------------------------------------

junit="$logs/pytest-junit.xml"
set +e
(
  cd "$sdk_dir" && env -i \
    PATH="$PATH" \
    LANG=C.UTF-8 \
    HOME="$work/home" \
    XDG_CONFIG_HOME="$work/home/.config" \
    TMPDIR="$work" \
    HTTPS_PROXY=http://127.0.0.1:9 HTTP_PROXY=http://127.0.0.1:9 \
    https_proxy=http://127.0.0.1:9 http_proxy=http://127.0.0.1:9 \
    NO_PROXY=127.0.0.1,localhost no_proxy=127.0.0.1,localhost \
    CREWAI_DISABLE_TELEMETRY=true CREWAI_DISABLE_TRACKING=true \
    CREWAI_TRACING_ENABLED=false CREWAI_DISABLE_VERSION_CHECK=true \
    OTEL_SDK_DISABLED=true LANGCHAIN_TRACING_V2=false NO_COLOR=1 \
    AIM_URL="$aim_url" \
    AIM_TEST_ADMIN_EMAIL="$admin_email" \
    AIM_TEST_ADMIN_PASSWORD="$admin_password" \
    "$python_bin" -m pytest -m integration -p no:cacheprovider -rs --tb=short \
      --junitxml="$junit" "${test_files[@]}"
)
pytest_status=$?
set -e

# --- 5. Verdict -------------------------------------------------------------

[ -f "$junit" ] || fail "pytest wrote no report (exit $pytest_status)"
read -r tests failures errors skipped < <(env -i PATH="$PATH" "$python_bin" -I -c '
import sys
import xml.etree.ElementTree as ET
root = ET.parse(sys.argv[1]).getroot()
suites = [root] if root.tag == "testsuite" else list(root.iter("testsuite"))
print(*(sum(int(s.get(k, 0)) for s in suites) for k in ("tests", "failures", "errors", "skipped")))
' "$junit")
echo "pytest: tests=$tests failures=$failures errors=$errors skipped=$skipped (exit $pytest_status)"
[ "$pytest_status" -eq 0 ] || fail "pytest exited $pytest_status"
[ "$tests" -gt 0 ] || fail "no test ran"
[ "$failures" -eq 0 ] && [ "$errors" -eq 0 ] || fail "$failures failure(s) and $errors error(s)"
[ "$skipped" -eq 0 ] || fail "$skipped test(s) skipped; every adapter test must run"
status=0
echo "All adapter tests ran and passed"
