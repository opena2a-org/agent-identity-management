#!/usr/bin/env bash
# test-lint-docs-keyvault-master-key.sh -- lint-docs-keyvault-master-key.sh must
# reject a docs/ section that sets a non-development ENVIRONMENT without naming
# KEYVAULT_MASTER_KEY, and the repository's own docs must pass it.
#
# The deployment, installation and setup guides showed production and staging
# configurations that never named the master key. Each rejected page is paired
# with an accepted one of the same shape, because "the lint fails" alone is
# also satisfied by a lint that rejects everything.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LINT="$SCRIPT_DIR/lint-docs-keyvault-master-key.sh"

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

# run_lint <page>: lint a one-page docs/ tree holding the page text, in a copy
# of the repository layout the lint expects.
run_lint() {
  local page="$1"
  rm -rf "$WORK/repo"
  mkdir -p "$WORK/repo/scripts" "$WORK/repo/docs/guides"
  cp "$LINT" "$WORK/repo/scripts/"
  printf '%s\n' "$page" > "$WORK/repo/docs/guides/page.md"
  bash "$WORK/repo/scripts/lint-docs-keyvault-master-key.sh" > "$WORK/out" 2>&1
}

# expect_rejected <description> <expected file:line:setting> <page>
expect_rejected() {
  local description="$1" expected="$2" page="$3"
  if run_lint "$page"; then
    fail "accepted: $description"
  elif grep -qF "docs/guides/page.md:$expected" "$WORK/out"; then
    pass "rejected and named with file and line: $description"
  else
    fail "rejected without naming docs/guides/page.md:$expected: $description"
    sed 's/^/        /' "$WORK/out"
  fi
}

# expect_accepted <description> <page>
expect_accepted() {
  local description="$1" page="$2"
  if run_lint "$page"; then
    pass "accepted: $description"
  else
    fail "rejected: $description"
    sed 's/^/        /' "$WORK/out"
  fi
}

FENCE='```'

echo "a production block must name the key"
expect_rejected "production block without the key" "4:ENVIRONMENT=production" \
"## Production

${FENCE}bash
ENVIRONMENT=production
LOG_LEVEL=warn
${FENCE}"
expect_accepted "production block with the key" \
"## Production

${FENCE}bash
ENVIRONMENT=production
KEYVAULT_MASTER_KEY=your-base64-encoded-32-byte-key
LOG_LEVEL=warn
${FENCE}"

echo "every environment except development needs the key"
expect_rejected "staging block without the key" "4:ENVIRONMENT=staging" \
"## Staging

${FENCE}bash
ENVIRONMENT=staging
${FENCE}"
expect_accepted "development block without the key" \
"## Development

${FENCE}bash
ENVIRONMENT=development
${FENCE}"
expect_rejected "a differently cased development is not development" "4:ENVIRONMENT=Development" \
"## Development

${FENCE}bash
ENVIRONMENT=Development
${FENCE}"

echo "the key must be named in the same section"
expect_rejected "key named only in another section" "4:ENVIRONMENT=production" \
"## Production

${FENCE}bash
ENVIRONMENT=production
${FENCE}

## Secrets

Set KEYVAULT_MASTER_KEY."
expect_accepted "key named in prose under the same heading" \
"## Production

${FENCE}bash
ENVIRONMENT=production
${FENCE}

Set KEYVAULT_MASTER_KEY as well."
expect_accepted "a shell comment inside a code block does not end the section" \
"## Reference

${FENCE}bash
ENVIRONMENT=production
# Security Settings
KEYVAULT_MASTER_KEY=your-base64-encoded-32-byte-key
${FENCE}"

echo "prose, YAML and command-prefix settings are checked"
# shellcheck disable=SC2016 # the backticks are a literal Markdown code span
expect_rejected "an inline code span in a numbered step" "3:ENVIRONMENT=production" \
'## Production

1. Set `ENVIRONMENT=production`'
expect_rejected "a quoted YAML value" "4:ENVIRONMENT=production" \
"## Kubernetes

${FENCE}yaml
ENVIRONMENT: \"production\"
${FENCE}"
expect_rejected "a prefix on a server start" "4:ENVIRONMENT=test" \
"## Run

${FENCE}bash
ENVIRONMENT=test ./server
${FENCE}"
expect_accepted "a prefix on go test configures the test process, not a server" \
"## Integration tests

${FENCE}bash
ENVIRONMENT=test go test ./tests/integration/... -v
${FENCE}"
expect_accepted "a variable that only ends in ENVIRONMENT is not checked" \
"## Other

${FENCE}bash
OTEL_ENVIRONMENT=production
${FENCE}"

echo "the repository's own docs pass"
if bash "$LINT" > "$WORK/out" 2>&1; then
  pass "$(cat "$WORK/out")"
else
  fail "the repository's docs fail the lint"
  sed 's/^/        /' "$WORK/out"
fi

if [[ $FAILURES -ne 0 ]]; then
  echo "test-lint-docs-keyvault-master-key: $FAILURES failure(s)"
  exit 1
fi
echo "test-lint-docs-keyvault-master-key: all checks passed"
