#!/usr/bin/env bash
# lint-docker-paths-filter.sh — the `docker` paths filter in security.yml must
# cover every path the Dockerfiles actually read.
#
# Why this exists. The filter was maintained BESIDE the Dockerfiles instead of
# derived FROM them, and drifted twice:
#
#   #389  ci.yml's `backend` filter listed apps/backend/** but not sdk/**, while
#         Dockerfile.backend does `COPY sdk ./sdk`. An sdk-only change skipped
#         the backend job entirely.
#   #398  security.yml's `docker` filter listed infrastructure/docker/** and two
#         lockfile pairs but neither sdk/** nor apps/backend/**, so Container
#         Scan skipped on changes to an image that contains both.
#
# Both were invisible in review because a skipped job satisfies a required
# status check: the PR reads green with the container never built. The class
# does not close by fixing the list a third time, only by making the list
# answerable from the Dockerfiles.
#
# What it checks: every `COPY <src>` in every Dockerfile resolves to a path
# covered by at least one `docker:` filter pattern. Build-stage-internal copies
# (`COPY --from=...`) are skipped: they move artifacts inside the build, not
# from the repo.
#
# Usage:  ./scripts/lint-docker-paths-filter.sh
# Exit:   0 pass, 1 an uncovered COPY source, 2 tool error.

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

workflow=".github/workflows/security.yml"
[ -r "$workflow" ] || { echo "lint-docker-paths-filter: cannot read $workflow" >&2; exit 2; }

dockerfiles=(infrastructure/docker/Dockerfile.*)
[ -e "${dockerfiles[0]}" ] || {
  echo "lint-docker-paths-filter: found no Dockerfiles to derive from" >&2
  exit 2
}

# The `docker:` block of the filter, as a list of bare patterns.
patterns="$(awk '
  /^[[:space:]]*docker:[[:space:]]*$/ { inblock=1; next }
  inblock && /^[[:space:]]*-[[:space:]]*.[^"]*.[[:space:]]*$/ {
    line=$0
    gsub(/^[[:space:]]*-[[:space:]]*/, "", line)
    gsub(/^['"'"'"]|['"'"'"][[:space:]]*$/, "", line)
    print line
    next
  }
  inblock && /^[[:space:]]*[a-zA-Z_-]+:[[:space:]]*$/ { inblock=0 }
' "$workflow")"

[ -n "$patterns" ] || {
  echo "lint-docker-paths-filter: parsed ZERO patterns from the docker: block." >&2
  echo "  The filter format changed and this check is now blind. Fix the parser," >&2
  echo "  do not delete the check: a scan that matches nothing reports the same" >&2
  echo "  clean result as a scan that read everything." >&2
  exit 2
}

covered() {
  local src="$1" pat prefix
  while IFS= read -r pat; do
    [ -n "$pat" ] || continue
    prefix="${pat%/\*\*}"
    [ "$src" = "$pat" ] && return 0
    case "$src" in "$prefix"/*) return 0 ;; esac
    case "$src" in "$prefix") return 0 ;; esac
  done <<< "$patterns"
  return 1
}

fail=0
checked=0
for df in "${dockerfiles[@]}"; do
  while IFS= read -r src; do
    [ -n "$src" ] || continue
    case "$src" in --from=*) continue ;; esac   # intra-build, not a repo path
    src="${src%/}"                              # `apps/backend/` -> `apps/backend`
    checked=$((checked + 1))
    if ! covered "$src"; then
      echo "FAIL  $df copies '$src', which no docker: filter pattern covers." >&2
      echo "      A change under $src alters the image and would skip Container Scan." >&2
      fail=1
    fi
  done < <(grep -oE '^COPY[[:space:]]+[^[:space:]]+' "$df" | awk '{print $2}')
done

[ "$checked" -gt 0 ] || {
  echo "lint-docker-paths-filter: parsed ZERO COPY lines. Parser is blind; fix it." >&2
  exit 2
}

if [ "$fail" -eq 0 ]; then
  echo "docker paths filter covers all $checked COPY source(s) across ${#dockerfiles[@]} Dockerfile(s)."
fi
exit "$fail"
