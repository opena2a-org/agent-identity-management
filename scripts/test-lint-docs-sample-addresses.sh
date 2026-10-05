#!/usr/bin/env bash
# test-lint-docs-sample-addresses.sh -- lint-docs-sample-addresses.sh must
# reject an address in docs/ that is not on a placeholder domain, and the
# repository's own docs must pass it.
#
# A deployment reference showed the signed-in Azure CLI account, a real
# person's address, as sample output. Each rejected address is paired with an
# accepted one of the same shape, because "the lint fails" alone is also
# satisfied by a lint that rejects everything.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LINT="$SCRIPT_DIR/lint-docs-sample-addresses.sh"

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

# run_lint <address>: lint a one-page docs/ tree whose sample output shows the
# address, in a copy of the repository layout the lint expects.
run_lint() {
  local address="$1"
  rm -rf "$WORK/repo"
  mkdir -p "$WORK/repo/scripts" "$WORK/repo/docs/guides"
  cp "$LINT" "$WORK/repo/scripts/"
  # shellcheck disable=SC2016 # the backticks are a literal Markdown code fence
  printf '# Sample\n\n```bash\n# Logged in as: %s\n```\n' "$address" > "$WORK/repo/docs/guides/page.md"
  bash "$WORK/repo/scripts/lint-docs-sample-addresses.sh" > "$WORK/out" 2>&1
}

expect_rejected() {
  local address="$1"
  if run_lint "$address"; then
    fail "accepted: $address"
  elif grep -qF "docs/guides/page.md:4:$address" "$WORK/out"; then
    pass "rejected and named with file and line: $address"
  else
    fail "rejected without naming file, line and address: $address"
    sed 's/^/        /' "$WORK/out"
  fi
}

expect_accepted() {
  local address="$1"
  if run_lint "$address"; then
    pass "accepted: $address"
  else
    fail "rejected: $address"
    sed 's/^/        /' "$WORK/out"
  fi
}

echo "a personal address in sample output is rejected; a reserved one is not"
expect_rejected 'jane.doe@not-a-placeholder.net'
expect_accepted 'admin@example.com'

echo "a subdomain of an allowed domain is allowed; a name that merely ends in one is not"
expect_accepted 'ops@mail.example.com'
expect_rejected 'ops@notexample.com'

echo "reserved top-level names are allowed"
expect_accepted 'dev@aim.test'
expect_rejected 'dev@aim.dev'

echo "domain case does not matter"
expect_accepted 'Admin@EXAMPLE.COM'

echo "the repository's own docs pass"
if bash "$LINT" > "$WORK/out" 2>&1; then
  pass "$(cat "$WORK/out")"
else
  fail "the repository's docs fail the lint"
  sed 's/^/        /' "$WORK/out"
fi

if [[ $FAILURES -ne 0 ]]; then
  echo "test-lint-docs-sample-addresses: $FAILURES failure(s)"
  exit 1
fi
echo "test-lint-docs-sample-addresses: all checks passed"
