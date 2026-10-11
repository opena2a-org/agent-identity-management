#!/usr/bin/env bash
# test-lint-docs-repo-links.sh -- lint-docs-repo-links.sh must reject a link to
# the `opena2a` GitHub account in a Markdown page, and the repository's own
# Markdown must pass it.
#
# The install, deployment and contributing guides told readers to clone from
# github.com/opena2a/, an account that does not exist, and linked its issue
# tracker. Each rejected link is paired with an accepted one of the same shape,
# because "the lint fails" alone is also satisfied by a lint that rejects
# everything.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LINT="$SCRIPT_DIR/lint-docs-repo-links.sh"

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

# run_lint <page> <line>: lint a copy of the repository layout the lint expects
# in which <page> holds a heading, a blank line and then <line> (line 3).
run_lint() {
  local page="$1" line="$2"
  rm -rf "$WORK/repo"
  mkdir -p "$WORK/repo/scripts" "$WORK/repo/$(dirname "$page")"
  cp "$LINT" "$WORK/repo/scripts/"
  printf '# Page\n\n%s\n' "$line" > "$WORK/repo/$page"
  bash "$WORK/repo/scripts/lint-docs-repo-links.sh" > "$WORK/out" 2>&1
}

# expect_rejected <page> <line> <link>: the lint fails and prints a line that is
# exactly <page>, line 3 and <link>, so a link printed with the text around it
# does not count.
expect_rejected() {
  local page="$1" line="$2" link="$3"
  if run_lint "$page" "$line"; then
    fail "accepted: $page: $line"
  elif grep -qxF "  $page:3:$link" "$WORK/out"; then
    pass "rejected and named with file and line: $page: $line"
  else
    fail "rejected without naming file, line and link: $page: $line"
    sed 's/^/        /' "$WORK/out"
  fi
}

expect_accepted() {
  local page="$1" line="$2"
  if run_lint "$page" "$line"; then
    pass "accepted: $page: $line"
  else
    fail "rejected: $page: $line"
    sed 's/^/        /' "$WORK/out"
  fi
}

echo "a clone URL under the opena2a account is rejected; one under opena2a-org is not"
expect_rejected docs/guides/INSTALLATION.md \
  'git clone https://github.com/opena2a/agent-identity-management.git' \
  'github.com/opena2a/agent-identity-management.git'
expect_accepted docs/guides/INSTALLATION.md \
  'git clone https://github.com/opena2a-org/agent-identity-management.git'

echo "an issue link inside Markdown link syntax is rejected and named without the closing parenthesis"
expect_rejected docs/guides/CONTRIBUTING.md \
  '- **Bugs**: [GitHub Issues](https://github.com/opena2a/identity/issues)' \
  'github.com/opena2a/identity/issues'
expect_accepted docs/guides/CONTRIBUTING.md \
  '- **Bugs**: [GitHub Issues](https://github.com/opena2a-org/agent-identity-management/issues)'

echo "a Go module path under the opena2a account is rejected"
# shellcheck disable=SC2016 # the backticks are a literal Markdown code span
expect_rejected docs/guides/API_ENDPOINT_SUMMARY.md \
  '- **Go**: `github.com/opena2a/aim-sdk-go`' \
  'github.com/opena2a/aim-sdk-go'

echo "the SSH form and any letter case are rejected"
expect_rejected infrastructure/DEPLOYMENT.md \
  'git clone git@github.com:opena2a/agent-identity-management.git' \
  'github.com:opena2a/agent-identity-management.git'
expect_rejected infrastructure/DEPLOYMENT.md \
  'https://GitHub.com/OpenA2A/agent-identity-management/issues' \
  'GitHub.com/OpenA2A/agent-identity-management/issues'
expect_accepted infrastructure/DEPLOYMENT.md \
  'git clone git@github.com:opena2a-org/agent-identity-management.git'

echo "a page outside docs/ is checked; a changelog, which records earlier text, is not"
expect_rejected README.md \
  'https://github.com/opena2a/agent-identity-management/issues' \
  'github.com/opena2a/agent-identity-management/issues'
expect_accepted CHANGELOG.md \
  'https://github.com/opena2a/agent-identity-management/issues'

echo "the repository's own Markdown passes"
if bash "$LINT" > "$WORK/out" 2>&1; then
  pass "$(cat "$WORK/out")"
else
  fail "the repository's Markdown fails the lint"
  sed 's/^/        /' "$WORK/out"
fi

if [[ $FAILURES -ne 0 ]]; then
  echo "test-lint-docs-repo-links: $FAILURES failure(s)"
  exit 1
fi
echo "test-lint-docs-repo-links: all checks passed"
