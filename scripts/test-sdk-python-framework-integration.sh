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
# Logs of a failed run stay in the directory the last line names. Steps 1 to 3
# and the stop in step 5 are scripts/lib/aim-backend.sh, which the Java
# helpers' script shares.

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
sdk_dir="${1:-$repo_root/sdk/python}"
python_bin="${PYTHON:-python3}"
test_files=(
  tests/test_langchain_integration.py
  tests/test_crewai_integration.py
  tests/test_mcp_integration.py
)

aim_backend_name="test-sdk-python-framework-integration"
# shellcheck source=lib/aim-backend.sh
. "$repo_root/scripts/lib/aim-backend.sh"

# --- 1. Preconditions ------------------------------------------------------

[ -d "$sdk_dir" ] || fail "SDK_DIR $sdk_dir is not a directory"
sdk_dir="$(cd "$sdk_dir" && pwd)"
[ -f "$sdk_dir/setup.py" ] || fail "SDK_DIR $sdk_dir holds no setup.py"
for f in "${test_files[@]}"; do
  [ -f "$sdk_dir/$f" ] || fail "SDK_DIR $sdk_dir holds no $f"
done

aim_backend_refuse_stored_sign_in "$sdk_dir" sdk/python
aim_backend_parse_database_url

for tool in go openssl curl "$python_bin"; do
  command -v "$tool" >/dev/null 2>&1 || fail "$tool is required but not found in PATH"
done
python_bin="$(command -v "$python_bin")"

# --- 2. Ephemeral secrets and 3. Backend ------------------------------------

# ENVIRONMENT=test raises both rate limits tenfold, as in the CI end-to-end
# job; outside the CI workflows only a test such as this one sets it.
aim_backend_start aim-sdk-python-framework ENVIRONMENT=test

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
