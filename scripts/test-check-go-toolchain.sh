#!/usr/bin/env bash
# test-check-go-toolchain.sh -- check-go-toolchain.sh must fail on each kind of
# drift it exists to catch, and pass on a tree where nothing drifted.
#
# The passing case is the control: a check that exits 1 on every input would
# satisfy every red case below on its own. Each case builds a fresh fixture
# tree, so one mutation cannot leak into the next. `go` is a stub on PATH that
# answers `go env GOVERSION` with $STUB_GOVERSION, so the cases need neither a
# network nor a particular Go install.
#
# Usage:  ./scripts/test-check-go-toolchain.sh   (SH=dash to run the check under dash)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHECK="$SCRIPT_DIR/check-go-toolchain.sh"
SH="${SH:-sh}"

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

WANT=go1.26.8
DIGEST=sha256:0000000000000000000000000000000000000000000000000000000000000000

mkdir -p "$WORK/bin"
cat > "$WORK/bin/go" <<'EOF'
#!/bin/sh
if [ "$#" -eq 2 ] && [ "$1" = env ] && [ "$2" = GOVERSION ]; then
  echo "$STUB_GOVERSION"
  exit 0
fi
echo "stub go: unexpected arguments: $*" >&2
exit 2
EOF
chmod +x "$WORK/bin/go"

FAILURES=0
CASE=0

# fixture: writes a tree where every declaration agrees, and sets $d to it.
fixture() {
  CASE=$((CASE + 1))
  local dir="$WORK/case$CASE"
  mkdir -p "$dir/apps/backend/infrastructure/docker" "$dir/infrastructure/docker" "$dir/.github/workflows"
  printf 'module example.com/fixture\n\ngo 1.25.0\n\ntoolchain %s\n' "$WANT" > "$dir/apps/backend/go.mod"
  local df
  for df in infrastructure/docker/Dockerfile.backend apps/backend/infrastructure/docker/Dockerfile.backend; do
    printf 'FROM golang:%s-alpine@%s AS builder\nRUN go version\n\nFROM alpine:3.21@%s\n' \
      "${WANT#go}" "$DIGEST" "$DIGEST" > "$dir/$df"
  done
  cat > "$dir/.github/workflows/ci.yml" <<'EOF'
jobs:
  backend:
    steps:
      - uses: actions/setup-go@40f1582b2485089dde7abd97c1529aa768e1baff # v5
        with:
          go-version-file: apps/backend/go.mod
EOF
  d="$dir"
}

# expect <name> <exit code> <stderr substring> <fixture dir> [STUB_GOVERSION]
expect() {
  local name="$1" want_rc="$2" want_msg="$3" dir="$4" stub="${5:-$WANT}"
  local rc=0 out
  out="$(PATH="$WORK/bin:$PATH" STUB_GOVERSION="$stub" "$SH" "$CHECK" "$dir" 2>&1)" || rc=$?
  if [ "$rc" -ne "$want_rc" ]; then
    echo "FAIL  $name: exit $rc, expected $want_rc"
    printf '      %s\n' "$out"
    FAILURES=$((FAILURES + 1))
  elif [[ "$out" != *"$want_msg"* ]]; then
    echo "FAIL  $name: output does not mention '$want_msg'"
    printf '      %s\n' "$out"
    FAILURES=$((FAILURES + 1))
  else
    echo "ok    $name (exit $rc)"
    printf '      %s\n' "$out"
  fi
}

fixture
expect "every declaration agrees" 0 "$WANT everywhere" "$d"

fixture
expect "go env GOVERSION differs from the directive" 1 "go env GOVERSION in apps/backend reports go1.26.3" "$d" go1.26.3

fixture
sed -i.bak 's/golang:1\.26\.8-alpine/golang:1.25-alpine/' "$d/infrastructure/docker/Dockerfile.backend"
expect "published Dockerfile builder tag differs" 1 "infrastructure/docker/Dockerfile.backend:1 builds on golang:1.25-alpine" "$d"

fixture
sed -i.bak 's/golang:1\.26\.8-alpine/golang:1.26.7-alpine/' "$d/apps/backend/infrastructure/docker/Dockerfile.backend"
expect "apps/backend Dockerfile builder tag differs" 1 "apps/backend/infrastructure/docker/Dockerfile.backend:1 builds on golang:1.26.7-alpine" "$d"

fixture
sed -i.bak 's/^FROM golang:.*/FROM alpine:3.21/' "$d/infrastructure/docker/Dockerfile.backend"
expect "Dockerfile lost its golang builder stage" 1 "has no golang builder stage" "$d"

fixture
sed -i.bak 's|go-version-file: apps/backend/go.mod|go-version: "1.25"|' "$d/.github/workflows/ci.yml"
expect "a workflow restates go-version:" 1 ".github/workflows/ci.yml:6 sets go-version:" "$d"

fixture
sed -i.bak '/^toolchain /d' "$d/apps/backend/go.mod"
expect "go.mod has no toolchain directive" 1 "carries 0 toolchain directives" "$d"

fixture
sed -i.bak 's/^toolchain .*/toolchain go1.26/' "$d/apps/backend/go.mod"
expect "toolchain directive is not a point release" 1 "must name a point release" "$d"

fixture
rm "$d/apps/backend/infrastructure/docker/Dockerfile.backend"
expect "a backend Dockerfile is missing" 2 "cannot read apps/backend/infrastructure/docker/Dockerfile.backend" "$d"

echo
if [ "$FAILURES" -ne 0 ]; then
  echo "$FAILURES of $CASE cases failed"
  exit 1
fi
echo "all $CASE cases passed"
