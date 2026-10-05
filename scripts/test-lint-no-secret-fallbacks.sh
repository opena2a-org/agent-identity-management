#!/usr/bin/env bash
# test-lint-no-secret-fallbacks.sh -- lint-no-secret-fallbacks.sh must reject a
# non-empty fallback for every secret-shaped name the compose files carry.
#
# ADMIN_PASSWORD reached both compose files without being on either list, so
# `${ADMIN_PASSWORD:-<a known password>}` passed the lint. Each planted
# fallback is paired with the allowed shape for the same name, because "the
# lint fails" alone is also satisfied by a lint that rejects everything.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LINT="$SCRIPT_DIR/lint-no-secret-fallbacks.sh"

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

FAILURES=0

fail() {
  echo "  FAIL: $*"
  FAILURES=$((FAILURES + 1))
}

pass() {
  echo "  ok:   $*"
}

# run_lint <compose line>: lint a one-service compose file carrying the line,
# in a copy of the repository layout the lint expects (scripts/ under the root).
run_lint() {
  local line="$1"
  rm -rf "$WORK/repo"
  mkdir -p "$WORK/repo/scripts"
  cp "$LINT" "$WORK/repo/scripts/"
  printf 'services:\n  backend:\n    environment:\n      - %s\n' "$line" > "$WORK/repo/docker-compose.yml"
  bash "$WORK/repo/scripts/lint-no-secret-fallbacks.sh" > "$WORK/out" 2>&1
}

expect_rejected() {
  local line="$1"
  if run_lint "$line"; then
    fail "accepted: $line"
  elif grep -qF "$line" "$WORK/out"; then
    pass "rejected and named: $line"
  else
    fail "rejected without naming the line: $line"
  fi
}

expect_accepted() {
  local line="$1"
  if run_lint "$line"; then
    pass "accepted: $line"
  else
    fail "rejected: $line"
    sed 's/^/        /' "$WORK/out"
  fi
}

echo "optional secrets: only an empty fallback is allowed"
expect_rejected 'ADMIN_PASSWORD=${ADMIN_PASSWORD:-AIM2025!Secure}'
expect_accepted 'ADMIN_PASSWORD=${ADMIN_PASSWORD:-}'
expect_rejected 'DEFAULT_ADMIN_PASSWORD=${DEFAULT_ADMIN_PASSWORD:-changeme}'
expect_accepted 'DEFAULT_ADMIN_PASSWORD=${DEFAULT_ADMIN_PASSWORD:-}'

echo "required secrets: no fallback of any kind"
expect_rejected 'JWT_SECRET=${JWT_SECRET:-}'
expect_accepted 'JWT_SECRET=${JWT_SECRET:?Set JWT_SECRET in .env}'

echo "a name that merely ends in a listed name is not that name"
expect_accepted 'GF_SECURITY_ADMIN_PASSWORD=${GRAFANA_ADMIN_PASSWORD:?Set GRAFANA_ADMIN_PASSWORD in .env}'

echo "the repository's own compose files pass"
if bash "$LINT" > "$WORK/out" 2>&1; then
  pass "$(cat "$WORK/out")"
else
  fail "the repository's compose files fail the lint"
  sed 's/^/        /' "$WORK/out"
fi

if [[ $FAILURES -ne 0 ]]; then
  echo "test-lint-no-secret-fallbacks: $FAILURES failure(s)"
  exit 1
fi
echo "test-lint-no-secret-fallbacks: all checks passed"
