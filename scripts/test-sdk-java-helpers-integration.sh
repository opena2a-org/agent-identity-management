#!/usr/bin/env bash
# test-sdk-java-helpers-integration.sh — run the Java SDK's helper tests
# (org.opena2a.aim.integration.*IntegrationTest) against a real backend.
#
# Usage:
#   DATABASE_URL='postgres://<user>[:<password>]@<host>[:<port>]/<db>?sslmode=disable' \
#   [MAVEN_REPO_LOCAL=<maven local repository>] \
#   scripts/test-sdk-java-helpers-integration.sh [SDK_DIR]
#
#   SDK_DIR           the sdk/java directory Maven runs in (default: this
#                     repository's sdk/java). The backend is always built from
#                     this repository's apps/backend.
#   MAVEN_REPO_LOCAL  the Maven local repository (default: one under the run's
#                     fresh HOME, which the run deletes, so every dependency is
#                     downloaded again).
#
# DATABASE_URL must name an empty database. `aim-bootstrap --default` leaves
# an admin that already exists untouched, and the tests sign in with the
# password this run generates.
#
# The run:
#   1. refuses to start when a stored AIM sign-in could reach the tests: a
#      .aim entry in SDK_DIR or a directory above it, or an
#      sdk_credentials.json or credentials.json in SDK_DIR, which the Java
#      SDK reads from its working directory;
#   2. generates JWT_SECRET, KEYVAULT_MASTER_KEY and the admin password, and
#      prints none of them;
#   3. builds cmd/server, cmd/migrate and cmd/bootstrap, runs the migrations,
#      seeds the admin and starts the server on a free port, then waits for
#      /health/ready (steps 2 and 3 are scripts/lib/aim-backend.sh);
#   4. runs `mvn -B test -Dtest='org.opena2a.aim.integration.*IntegrationTest'`
#      with AIM_URL set, a fresh HOME, user.home set to that HOME, and no
#      inherited environment. The tests take their SDK credentials from the
#      backend's Java SDK download, so the SDK reaches only the loopback
#      backend;
#   5. stops the server whatever the outcome, and exits non-zero when a step
#      fails or Maven reports a failure, an error or a skip, or runs no test.
#
# Logs of a failed run stay in the directory the last line names.

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
sdk_dir="${1:-$repo_root/sdk/java}"
test_pattern='org.opena2a.aim.integration.*IntegrationTest'
test_file=src/test/java/org/opena2a/aim/integration/HelpersAgainstBackendIntegrationTest.java

aim_backend_name="test-sdk-java-helpers-integration"
# shellcheck source=lib/aim-backend.sh
. "$repo_root/scripts/lib/aim-backend.sh"

# --- 1. Preconditions ------------------------------------------------------

[ -d "$sdk_dir" ] || fail "SDK_DIR $sdk_dir is not a directory"
sdk_dir="$(cd "$sdk_dir" && pwd)"
[ -f "$sdk_dir/pom.xml" ] || fail "SDK_DIR $sdk_dir holds no pom.xml"
[ -f "$sdk_dir/$test_file" ] || fail "SDK_DIR $sdk_dir holds no $test_file"

aim_backend_refuse_stored_sign_in "$sdk_dir" sdk/java
for f in sdk_credentials.json credentials.json; do
  if [ -e "$sdk_dir/$f" ]; then
    fail "refusing to run: $sdk_dir holds $f, and the Java SDK reads a stored sign-in from its working directory.
  Verify: ls -l '$sdk_dir/$f'
  Fix: move $f out of $sdk_dir, or run the tests from a clone: d=\"\$(mktemp -d)\" && git clone -q '$repo_root' \"\$d/aim\" && '$repo_root/scripts/$aim_backend_name.sh' \"\$d/aim/sdk/java\""
  fi
done

aim_backend_parse_database_url

for tool in go openssl curl java mvn; do
  command -v "$tool" >/dev/null 2>&1 || fail "$tool is required but not found in PATH"
done

# --- 2. Ephemeral secrets and 3. Backend ------------------------------------

# ENVIRONMENT=test raises both rate limits tenfold, as in the CI end-to-end
# job; outside the CI workflows only a test such as this one sets it.
aim_backend_start aim-sdk-java-helpers ENVIRONMENT=test

# --- 4. Tests ---------------------------------------------------------------

maven_repo="${MAVEN_REPO_LOCAL:-$work/home/.m2/repository}"
mvn_log="$logs/mvn.log"
# user.home goes in twice: as a Maven property, which surefire copies into the
# test JVM (the pom's own argLine replaces a -DargLine), and in argLine for a
# pom that reads it.
mvn_env=(
  env -i
  PATH="$PATH"
  LANG=C.UTF-8
  HOME="$work/home"
  XDG_CONFIG_HOME="$work/home/.config"
  TMPDIR="$work"
  NO_COLOR=1
  OPENA2A_TELEMETRY=off
  AIM_URL="$aim_url"
  AIM_TEST_ADMIN_EMAIL="$admin_email"
  AIM_TEST_ADMIN_PASSWORD="$admin_password"
)
if [ -n "${JAVA_HOME:-}" ]; then
  mvn_env+=(JAVA_HOME="$JAVA_HOME")
fi

set +e
(
  cd "$sdk_dir" && "${mvn_env[@]}" mvn -B \
    -Dmaven.repo.local="$maven_repo" \
    -Duser.home="$work/home" \
    -DargLine="-Duser.home=$work/home" \
    test -Dtest="$test_pattern" >"$mvn_log" 2>&1
)
mvn_status=$?
set -e

# --- 5. Verdict -------------------------------------------------------------

summary="$(grep -E 'Tests run: [0-9]+, Failures: [0-9]+, Errors: [0-9]+, Skipped: [0-9]+$' "$mvn_log" | tail -n 1 || true)"
if [ -z "$summary" ]; then
  tail -n 40 "$mvn_log" >&2
  fail "Maven reported no test summary (exit $mvn_status, see $mvn_log)"
fi
read -r tests failures errors skipped < <(echo "$summary" \
  | sed -E 's/.*Tests run: ([0-9]+), Failures: ([0-9]+), Errors: ([0-9]+), Skipped: ([0-9]+)$/\1 \2 \3 \4/')
grep -E 'Tests run: .* -- in ' "$mvn_log" || true
echo "mvn: tests=$tests failures=$failures errors=$errors skipped=$skipped (exit $mvn_status)"
if [ "$mvn_status" -ne 0 ]; then
  tail -n 60 "$mvn_log" >&2
  fail "mvn exited $mvn_status (see $mvn_log)"
fi
[ "$tests" -gt 0 ] || fail "no test ran"
[ "$failures" -eq 0 ] && [ "$errors" -eq 0 ] || fail "$failures failure(s) and $errors error(s)"
[ "$skipped" -eq 0 ] || fail "$skipped test(s) skipped; every helper test must run"
status=0
echo "All helper tests ran and passed"
