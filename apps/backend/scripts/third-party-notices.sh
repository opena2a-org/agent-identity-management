#!/bin/sh
# third-party-notices.sh - the third-party license notices the backend image
# carries, generated from this module's dependency graph when the image is
# built (infrastructure/docker/Dockerfile.backend, builder stage).
#
# Every library linked into aim-server, aim-migrate and aim-bootstrap is
# redistributed in the image, so the image owes each one's notices: the
# license text of every MIT, BSD and Apache-2.0 library, and for the MPL-2.0
# libraries (the HashiCorp Vault client and its dependencies, admitted in
# go-licenses-allowlist.tsv) a statement of how to obtain the Source Code Form.
# This script writes, under <out-dir>:
#
#   licenses/<library>/  what `go-licenses save` writes for the library: its
#                        license and notice files and, for a reciprocal license
#                        such as MPL-2.0, the library's complete source
#   manifest.csv         one row per library, `library,licenseURL,licenseName`,
#                        from `go-licenses report`; the URL points at the
#                        license file in the library's upstream repository at
#                        the version that was built in
#   README               what the two are
#
# and then verifies what it wrote before the build may continue:
#
#   * every manifest row names a library, a license and a URL that is not
#     Unknown;
#   * every manifest row has a directory under licenses/ holding a file;
#   * every MPL-2.0 library's directory holds its Go source;
#   * every row of go-licenses-allowlist.tsv is in the manifest exactly once,
#     with the license it was ruled for.
#
# Output is written to <out-dir>.partial and renamed to <out-dir> only after
# every check passes; a failed run removes it, so it never leaves a notices
# tree behind. Nothing here tolerates a failure: any non-zero step exits
# non-zero and fails the image build.
#
# Inputs:
#   GO_LICENSES_VERSION  required, the go-licenses version, e.g. v2.0.1
#   GO_LICENSES_SUM      required, the h1: sum of that module version
#   GOBIN                optional, where the checker is installed
#
# The pin is the one .github/workflows/security.yml gives
# scripts/go-licenses-check.sh, so the image's notices and the license gate
# are produced by the same checker build.
#
# Usage:  sh scripts/third-party-notices.sh <out-dir>
# Exit:   0 written and verified, 1 a step or a check failed, 2 usage.

set -eu

checker_module="github.com/google/go-licenses/v2"

fail() {
  printf 'third-party notices: %s\n' "$1" >&2
  exit 1
}

usage() {
  printf 'third-party notices: %s\n' "$1" >&2
  printf 'usage: sh scripts/third-party-notices.sh <out-dir>\n' >&2
  exit 2
}

[ "$#" -eq 1 ] || usage "expected exactly one argument, the output directory"
out="${1%/}"
[ -n "$out" ] || usage "the output directory must not be empty or /"
case "$out" in
  /*) ;;
  *) out="$PWD/$out" ;;
esac
staging="$out.partial"

version="${GO_LICENSES_VERSION:-}"
sum="${GO_LICENSES_SUM:-}"
if [ -z "$version" ] || [ -z "$sum" ]; then
  usage "GO_LICENSES_VERSION and GO_LICENSES_SUM must both be set"
fi

[ ! -e "$out" ] || usage "$out already exists; notices are written to a new directory"
[ ! -e "$staging" ] || usage "$staging already exists; remove it and run again"

module_root="$(cd "$(dirname "$0")/.." && pwd)"
allowlist="$module_root/go-licenses-allowlist.tsv"
[ -f "$allowlist" ] || fail "$allowlist is missing"

# ---- The checker, at the pin, verified on the binary that was built --------

GOBIN="${GOBIN:-${TMPDIR:-/tmp}/third-party-notices-bin}"
export GOBIN
mkdir -p "$GOBIN"
checker="$GOBIN/go-licenses"

go install "$checker_module@$version" || fail "go install $checker_module@$version failed"
[ -x "$checker" ] || fail "go install succeeded but $checker is missing or not executable"

build_info="$(go version -m "$checker")" || fail "go version -m $checker failed"
if ! printf '%s\n' "$build_info" | awk -F'\t' -v mod="$checker_module" -v ver="$version" -v want="$sum" '
  {
    i = 1
    while (i <= NF && $i == "") i++
    if ($i == "mod" && $(i + 1) == mod && $(i + 2) == ver && $(i + 3) == want) found = 1
  }
  END { exit(found ? 0 : 1) }
'; then
  fail "$checker is not $checker_module $version $sum"
fi

# ---- Generate ----------------------------------------------------------------

# Removed on every exit; after the final rename it no longer exists.
trap 'rm -rf "$staging"' EXIT
mkdir -p "$staging"
cd "$module_root"

if ! "$checker" report ./... > "$staging/report.csv" 2> "$staging/report.err"; then
  tail -n 20 "$staging/report.err" >&2
  fail "go-licenses report failed"
fi

if ! "$checker" save ./... "--save_path=$staging/licenses" 2> "$staging/save.err"; then
  tail -n 20 "$staging/save.err" >&2
  fail "go-licenses save failed"
fi

# go-licenses v2.0.1 writes no header; drop one if a later version does.
{
  printf 'library,licenseURL,licenseName\n'
  awk 'NR == 1 && $0 == "name,licenseURL,licenseName" { next } NF > 0 { print }' "$staging/report.csv"
} > "$staging/manifest.csv"
rm -f "$staging/report.csv" "$staging/report.err" "$staging/save.err"

# ---- Verify ------------------------------------------------------------------

libraries=0
mpl=0
pairs="$staging/pairs.tsv"
: > "$pairs"

while IFS= read -r row; do
  [ "$row" != "library,licenseURL,licenseName" ] || continue
  name="${row%%,*}"
  license="${row##*,}"
  rest="${row#*,}"
  url="${rest%,*}"
  case "$name" in
    '' | *[!A-Za-z0-9._/~-]* | *..*) fail "manifest row is not a library path: $row" ;;
  esac
  [ "$rest" != "$row" ] && [ "$url" != "$rest" ] || fail "manifest row for $name is not library,licenseURL,licenseName: $row"
  [ -n "$license" ] && [ "$license" != "Unknown" ] || fail "$name has no identified license"
  [ -n "$url" ] && [ "$url" != "Unknown" ] || fail "$name has no license URL, so the manifest cannot say where its source is"

  dir="$staging/licenses/$name"
  [ -d "$dir" ] || fail "$name is in the manifest but go-licenses save wrote no licenses/$name"
  [ -n "$(find "$dir" -maxdepth 1 -type f | head -n 1)" ] || fail "licenses/$name holds no license file"

  if [ "$license" = "MPL-2.0" ]; then
    [ -n "$(find "$dir" -maxdepth 1 -type f -name '*.go' | head -n 1)" ] ||
      fail "$name is MPL-2.0 but licenses/$name holds none of its source"
    mpl=$((mpl + 1))
  fi

  printf '%s\t%s\n' "$name" "$license" >> "$pairs"
  libraries=$((libraries + 1))
done < "$staging/manifest.csv"

[ "$libraries" -gt 0 ] || fail "go-licenses report named no libraries"

allowlisted=""
allowlisted_count=0
allowlist_rows="$staging/allowlist.rows"
awk '/^[ \t]*#/ { next } /^[ \t]*$/ { next } { print }' "$allowlist" > "$allowlist_rows"
tab="$(printf '\t')"
while IFS="$tab" read -r library ruled_license _rest; do
  case "$library" in
    '' | *[!A-Za-z0-9._/~-]*) fail "go-licenses-allowlist.tsv has a row whose library is not a module path: $library" ;;
  esac
  [ -n "$ruled_license" ] || fail "go-licenses-allowlist.tsv names no license for $library"
  hits="$(awk -F'\t' -v lib="$library" '$1 == lib' "$pairs" | wc -l | tr -d ' ')"
  [ "$hits" -eq 1 ] || fail "allowlisted $library appears $hits times in the manifest, not once"
  got="$(awk -F'\t' -v lib="$library" '$1 == lib { print $2 }' "$pairs")"
  [ "$got" = "$ruled_license" ] || fail "allowlisted $library is $got in the manifest, not the ruled $ruled_license"
  allowlisted="${allowlisted}allowlisted: $library $got
"
  allowlisted_count=$((allowlisted_count + 1))
done < "$allowlist_rows"
rm -f "$pairs" "$allowlist_rows"

cat > "$staging/README" <<'EOF'
Third-party license notices for the AIM server image

The aim-server, aim-migrate and aim-bootstrap binaries in /app are built from
the Go module github.com/opena2a-org/agent-identity-management/apps/backend
and the libraries it links. This directory was generated from that module's
dependency graph when the image was built.

manifest.csv  One row per library: library,licenseURL,licenseName. The
              licenseURL is the library's license file in its upstream
              repository at the version built into the binaries; the
              library's source can be obtained from that repository at
              that version.

licenses/     One directory per library, at its module path, holding its
              license and notice files. For a library under a reciprocal
              license such as MPL-2.0, the directory also holds the
              library's complete source as built into the binaries.
EOF

mv "$staging" "$out"

printf 'third-party notices: %s libraries, %s MPL-2.0 with source, %s allowlisted, written to %s\n' \
  "$libraries" "$mpl" "$allowlisted_count" "$out"
printf '%s' "$allowlisted"
