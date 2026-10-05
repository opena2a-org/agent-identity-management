#!/bin/sh
# check-go-toolchain.sh -- the backend's Go toolchain is declared once, in the
# `toolchain` directive of apps/backend/go.mod, and every build of the backend
# runs exactly that release.
#
# Why this exists. The version used to live in three places that moved
# independently: go.mod's `go` line, ten `go-version: "1.25"` literals across
# ci.yml, release.yml and security.yml, and the golang builder tag in the
# backend Dockerfiles. Bumping one left the others behind, so CI could test on
# one release while the published image was compiled by another.
#
# What it checks:
#   1. apps/backend/go.mod carries exactly one `toolchain goX.Y.Z` directive.
#   2. `go env GOVERSION`, run inside apps/backend, reports that release.
#      actions/setup-go (v5) installs the release named by the `go` line, not
#      the `toolchain` line; with the default GOTOOLCHAIN=auto the go command
#      then switches to the directive by itself. A mismatch here means
#      GOTOOLCHAIN pins another release, or the switch did not happen.
#   3. Every `FROM golang:<tag>` in the backend Dockerfile names that release
#      (golang:X.Y.Z-alpine). The official golang images set
#      GOTOOLCHAIN=local, so the builder tag alone decides which release
#      compiles the published binaries; go.mod cannot move it.
#   4. No workflow sets `go-version:`. Every setup-go step reads
#      `go-version-file: apps/backend/go.mod`, so the literal cannot come back.
#
# Usage:  sh scripts/check-go-toolchain.sh [repo-root]
# Exit:   0 pass, 1 a mismatch, 2 tool error.

set -eu

root=${1:-$(cd "$(dirname "$0")/.." && pwd)}
gomod="$root/apps/backend/go.mod"
dockerfiles="infrastructure/docker/Dockerfile.backend"
workflows="$root/.github/workflows"

fail=0
mismatch() { echo "check-go-toolchain: $*" >&2; fail=1; }
die() { echo "check-go-toolchain: $*" >&2; exit 2; }

[ -r "$gomod" ] || die "cannot read $gomod"
[ -d "$workflows" ] || die "cannot find $workflows"

# 1. The declaration.
count=$(awk '$1 == "toolchain" { n++ } END { print n + 0 }' "$gomod")
if [ "$count" -ne 1 ]; then
  echo "check-go-toolchain: apps/backend/go.mod carries $count toolchain directives; it must carry exactly one (toolchain go1.X.Y)" >&2
  exit 1
fi
want=$(awk '$1 == "toolchain" { print $2 }' "$gomod")
if ! printf '%s\n' "$want" | grep -Eq '^go[0-9]+\.[0-9]+\.[0-9]+$'; then
  echo "check-go-toolchain: apps/backend/go.mod says 'toolchain $want'; it must name a point release (toolchain go1.X.Y)" >&2
  exit 1
fi
version=${want#go}

# 2. The release the go command actually runs for this module.
have=$(cd "$root/apps/backend" && go env GOVERSION) || die "go env GOVERSION failed in apps/backend"
have=${have%% *}
if [ "$have" != "$want" ]; then
  mismatch "go env GOVERSION in apps/backend reports $have; the toolchain directive says $want." \
    "Unset GOTOOLCHAIN (or set it to auto) so the go command switches to the directive."
fi

# 3. The builder stages that compile the published binaries.
stages=0
for df in $dockerfiles; do
  [ -r "$root/$df" ] || die "cannot read $df"
  refs=$(awk 'toupper($1) == "FROM" {
      for (i = 2; i <= NF; i++) {
        if ($i ~ /^--/) continue
        if ($i ~ /^(docker\.io\/)?(library\/)?golang:/) print NR ":" $i
        break
      }
    }' "$root/$df")
  if [ -z "$refs" ]; then
    mismatch "$df has no golang builder stage; expected FROM golang:$version-alpine"
    continue
  fi
  for entry in $refs; do
    stages=$((stages + 1))
    lineno=${entry%%:*}
    tag=${entry#*golang:}
    tag=${tag%%@*}
    if [ "${tag%%-*}" != "$version" ]; then
      mismatch "$df:$lineno builds on golang:$tag; the toolchain directive says $want (expected golang:$version-alpine)"
    fi
  done
done

# 4. No workflow restates the version.
literals=$(grep -nE '^[[:space:]]*go-version:' "$workflows"/*.yml "$workflows"/*.yaml 2>/dev/null || true)
if [ -n "$literals" ]; then
  printf '%s\n' "$literals" | while IFS= read -r line; do
    line=${line#"$root"/}
    where=$(printf '%s\n' "$line" | cut -d: -f1-2)
    echo "check-go-toolchain: $where sets go-version:; use go-version-file: apps/backend/go.mod" >&2
  done
  fail=1
fi

if [ "$fail" -ne 0 ]; then
  exit 1
fi
echo "check-go-toolchain: $want everywhere (go.mod toolchain directive, go env GOVERSION, $stages Dockerfile builder stages, no go-version: in workflows)"
