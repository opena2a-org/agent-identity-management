#!/usr/bin/env bash
# go-licenses-check.sh — the Go dependency license verdict for the
# "Check Go licenses" step of .github/workflows/security.yml.
#
# THE VERDICT IS THIS SCRIPT'S EXIT STATUS, AND THAT IS THE CHECKER'S:
#
#   0  no disallowed license found   go-licenses exited 0 and every allowance
#                                    in apps/backend/go-licenses-allowlist.tsv
#                                    still holds
#   1  a license problem             go-licenses exited non-zero AND said so,
#                                    or an allowance no longer holds
#   2  the tool, not the licenses    it could not be built, could not be
#                                    verified, could not run, or failed before
#                                    it reached a verdict — or the allowlist
#                                    that feeds it is malformed
#
# Exit 1 is the only status that means "a license problem". Nothing about the
# tool can ever produce it, which is the whole point of the file. A non-zero
# checker exit never yields 0; a zero checker exit yields 0 only when every
# allowance still holds.
#
# Why this exists. The step used to be, inline in the workflow:
#
#     go install github.com/google/go-licenses@latest
#     go-licenses check ./... --disallowed_types=restricted,reciprocal 2>&1 \
#       | tee license-report.txt
#     if grep -iE 'GPL|AGPL|SSPL' license-report.txt | grep -iv 'LGPL'; then
#       echo "::error::Copyleft license detected in Go dependencies"; exit 1
#     fi
#
# which carried four defects. Each one is closed here:
#
#  1. The step's exit status was `tee`'s, not the checker's — the workflow sets
#     no pipefail — so the only surviving verdict was the text grep. Any
#     restricted or reciprocal license not spelled GPL, AGPL or SSPL passed,
#     and any line naming both LGPL and AGPL was thrown away by the
#     `grep -iv 'LGPL'`. No pipe appears below, and the checker's status is
#     returned unaltered.
#  2. An explicit --disallowed_types REPLACES the tool's default of
#     forbidden,unknown (check.go applies that default only when the flag is
#     empty). So `restricted,reciprocal` had silently un-gated class forbidden
#     — which is where AGPL lands — and class unknown, which is every license
#     the tool cannot identify. Making the checker's status the verdict with
#     that list would have shipped a gate WEAKER than the grep it replaced.
#     Hence all four classes in DISALLOWED_TYPES.
#  3. `@latest` on the v1 module path resolved to v1.6.0 (2023-01-18), because
#     v2 moved to the /v2 path, and left no record of what had run. The
#     version and its h1 sum are pinned in security.yml and re-asserted on the
#     INSTALLED binary here before the checker is trusted.
#  4. One transient module-proxy error read as a license failure. Every
#     failure is now classified exactly once, in a fixed order, and a tool
#     failure can never be reported as a finding.
#
# The allowances. apps/backend/go-licenses-allowlist.tsv admits one license on
# one library, by exact path, under a recorded ruling. Each row becomes one
# `--ignore=<library>`, so `check` stops reporting it — and `--ignore` is a
# blunt instrument: it silences a path PREFIX, permanently, with no record of
# what it silenced. So every row is then re-asserted against a second,
# UNIGNORED `go-licenses report ./...`:
#
#   * the library is still in the module graph      (else the row is dead)
#   * it still carries exactly the ruled license    (else the ruling is stale)
#   * no unlisted library sits under its path       (else the row silences
#                                                    something nobody ruled on)
#
# and the verdict line names every allowance that was actually exercised, so
# an allowance is visible where the verdict is read and never just in a file.
#
# Inputs — security.yml is the single home of the pin:
#   GO_LICENSES_VERSION      required, the pinned version, e.g. v2.0.1
#   GO_LICENSES_SUM          required, the h1: sum of that module version
#   GO_LICENSES_RETRY_DELAY  optional, seconds before the one retry (default 5)
#   GOBIN                    optional, where the checker is installed
#   GITHUB_STEP_SUMMARY      optional, the step summary file to append to
#
# Nothing here relaxes module verification: the checksum database is left at
# its default, so `go install` itself refuses a substituted module, and the sum
# is then re-asserted on the binary that was actually built.
#
# Usage:  ./scripts/go-licenses-check.sh
# Exit:   0 pass, 1 a license problem, 2 a tool or allowlist failure.

# Deliberately no `-e`: a non-zero status from the installer or the checker is
# this script's input, not a reason to abort before classifying it.
set -uo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

# The module the checker is built from. v2 lives under the /v2 path; the bare
# path resolves to the abandoned v1.6.0 line.
readonly GO_LICENSES_MODULE="github.com/google/go-licenses/v2"

# The Go module whose dependency licenses are the subject.
readonly CHECK_DIR="apps/backend"

# The recorded license allowances for that module, relative to CHECK_DIR.
readonly ALLOWLIST_NAME="go-licenses-allowlist.tsv"

# The class set, as a literal. See defect 2 above before narrowing it: this
# value REPLACES the tool's default, it does not extend it.
readonly DISALLOWED_TYPES="forbidden,unknown,restricted,reciprocal"

# A library path is a Go import path and nothing else. Anything outside this
# set in the allowlist is a typo, a comment that lost its `#`, or an attempt to
# smuggle a glob or a shell metacharacter into an `--ignore` argument.
readonly LIBRARY_RE='^[A-Za-z0-9._/~-]+$'

# A module substituted under us, or a sum that does not match go.sum. Never
# retried: a second fetch returns the same bad module, and retrying a security
# refusal until it passes is the one thing a retry must not do.
readonly CHECKSUM_RE='checksum mismatch|SECURITY ERROR'

# Transport, proxy and checksum-database failures — the class that says nothing
# whatever about the licenses. Retried once, then reported as a tool failure.
readonly NETWORK_RE='stream error|INTERNAL_ERROR|proxy\.golang\.org|sum\.golang\.org|storage\.googleapis\.com|dial tcp|i/o timeout|timed out|timeout|temporary failure|network is unreachable|connection reset|TLS handshake|unexpected EOF'

# The three forms go-licenses v2.0.1 reports a finding in, copied from its
# check.go. A finding is the ONLY thing that may exit 1 off the checker, and it
# is recognised before the network pattern is consulted — otherwise a finding
# against a library whose own name contains "timeout" would be misread as a
# proxy blip.
FINDING_PATTERNS=(
  $'^Did not find license for library \'.*\'\\.$'
  $'^Not allowed license \'.*\' found for library \'.*\'\\.$'
  $'^License \'.*\' of not allowed license type \'.*\' found for library \'.*\'\\.$'
)

summary_file="${GITHUB_STEP_SUMMARY:-/dev/null}"

# One verdict line and the output that classified it, to the step summary and
# to stdout. The caller sets the exit status.
emit() {
  local phrase="$1" excerpt="${2:-}"
  {
    printf 'License Compliance (Go): %s\n' "$phrase"
    if [ -n "$excerpt" ]; then printf '%s\n' "$excerpt"; fi
  } >> "$summary_file"
  printf 'License Compliance (Go): %s\n' "$phrase"
  if [ -n "$excerpt" ]; then printf '%s\n' "$excerpt"; fi
}

# matches <ere> <text>
matches() {
  grep -qE -- "$1" <<< "$2"
}

# lines_matching <ere> <text> — the matching lines, empty if none.
lines_matching() {
  local out
  out="$(grep -E -- "$1" <<< "$2")" || out=""
  printf '%s' "$out"
}

# finding_lines <text> — the finding lines; returns 1 when the output carries
# none, which is what separates "a license was rejected" from "the tool broke".
finding_lines() {
  local text="$1" pat hit all=""
  for pat in "${FINDING_PATTERNS[@]}"; do
    hit="$(lines_matching "$pat" "$text")"
    if [ -n "$hit" ]; then all+="$hit"$'\n'; fi
  done
  [ -n "$all" ] || return 1
  printf '%s' "${all%$'\n'}"
}

# tail_excerpt <text> — enough of an unrecognised failure to act on.
tail_excerpt() {
  tail -n 20 <<< "$1"
}

version="${GO_LICENSES_VERSION:-}"
sum="${GO_LICENSES_SUM:-}"
retry_delay="${GO_LICENSES_RETRY_DELAY:-5}"

if [ -z "$version" ] || [ -z "$sum" ]; then
  emit 'tool could not be built' \
    'GO_LICENSES_VERSION and GO_LICENSES_SUM must both be set; the pin lives in .github/workflows/security.yml.'
  exit 2
fi

# ── The allowlist, parsed before anything is installed or run ───────────────
#
# It comes first because it decides what the checker is ASKED, so a malformed
# row must never reach an `--ignore` argument — and because a policy file that
# cannot be read is a refusal, never a pass.

allowlist_path="$repo_root/$CHECK_DIR/$ALLOWLIST_NAME"

libraries=()
ruled_licenses=()
ruled_stamps=()

if [ ! -f "$allowlist_path" ]; then
  emit 'allowlist malformed' \
    "$CHECK_DIR/$ALLOWLIST_NAME is missing; it is policy input to this gate, and its absence is not a pass."
  exit 2
fi

malformed_rows="$(
  awk -F'\t' -v libre="$LIBRARY_RE" '
    /^[ \t]*#/ { next }
    /^[ \t]*$/ { next }
    {
      bad = ""
      if (NF != 4) {
        bad = "expected 4 TAB-separated fields, saw " NF
      } else {
        for (i = 1; i <= 4; i++) if ($i == "") bad = "field " i " is empty"
        if (bad == "" && $1 !~ libre) bad = "library does not match " libre
      }
      if (bad != "") printf "line %d: %s: %s\n", NR, bad, $0
    }
  ' "$allowlist_path"
)"

if [ -n "$malformed_rows" ]; then
  emit 'allowlist malformed' \
    "$CHECK_DIR/$ALLOWLIST_NAME: every row is <library> TAB <license> TAB <ruled> TAB <reason>, all four non-empty."$'\n'"$malformed_rows"
  exit 2
fi

while IFS= read -r row; do
  [ -n "$row" ] || continue
  IFS=$'\t' read -r row_library row_license row_ruled _row_reason <<< "$row"
  libraries+=("$row_library")
  ruled_licenses+=("$row_license")
  ruled_stamps+=("$row_ruled")
done < <(awk -F'\t' '/^[ \t]*#/ { next } /^[ \t]*$/ { next } { print }' "$allowlist_path")

# is_allowlisted <library> — true when the path is itself a row, not merely
# covered by one.
is_allowlisted() {
  local needle="$1" candidate
  for candidate in ${libraries[@]+"${libraries[@]}"}; do
    [ "$candidate" = "$needle" ] && return 0
  done
  return 1
}

# One `--ignore` per row, in file order, and nothing else ignored.
ignore_args=()
for row_library in ${libraries[@]+"${libraries[@]}"}; do
  ignore_args+=("--ignore=$row_library")
done

# The checker is executed from GOBIN by path, never resolved off PATH: the sum
# is asserted on one specific file and that file is the one that must run.
if [ -z "${GOBIN:-}" ]; then
  GOBIN="${TMPDIR:-/tmp}/go-licenses-check-bin"
fi
export GOBIN
if ! mkdir -p "$GOBIN"; then
  emit 'tool could not be built' "cannot create GOBIN directory $GOBIN"
  exit 2
fi
checker="$GOBIN/go-licenses"

# Somewhere to keep the report's stderr apart from its stdout: the report IS a
# CSV document, and a warning line merged into it would be parsed as a library.
scratch="${TMPDIR:-/tmp}/go-licenses-check.$$"
if ! mkdir -p "$scratch"; then
  emit 'checker could not run' "cannot create scratch directory $scratch"
  exit 2
fi
trap 'rm -rf "$scratch"' EXIT

# ── Install, classified: checksum refusal, then network-class, then give up ──

install_out=""
run_install() {
  install_out="$(go install "${GO_LICENSES_MODULE}@${version}" 2>&1)"
}

attempt=0
while :; do
  attempt=$((attempt + 1))
  if run_install; then break; fi

  if matches "$CHECKSUM_RE" "$install_out"; then
    emit 'tool checksum mismatch' "$(lines_matching "$CHECKSUM_RE" "$install_out")"
    exit 2
  fi

  if [ "$attempt" -eq 1 ] && matches "$NETWORK_RE" "$install_out"; then
    printf 'go-licenses-check: install hit a network-class failure; retrying once in %ss.\n' "$retry_delay"
    sleep "$retry_delay"
    continue
  fi

  emit 'tool could not be built' "$(tail_excerpt "$install_out")"
  exit 2
done

# ── The pin, asserted on the binary that was actually built ─────────────────

if [ ! -x "$checker" ]; then
  emit 'tool could not be built' "go install reported success but $checker is missing or not executable"
  exit 2
fi

version_out="$(go version -m "$checker" 2>&1)"

# `go version -m` prints build info as tab-separated fields; the `mod` line
# carries the main module's path, version and h1 sum. All four must match, or
# the binary is not the one the pin names and the checker does not run.
mod_line_present() {
  awk -F'\t' -v mod="$GO_LICENSES_MODULE" -v ver="$version" -v want="$sum" '
    {
      i = 1
      while (i <= NF && $i == "") i++
      if ($i == "mod" && $(i + 1) == mod && $(i + 2) == ver && $(i + 3) == want) found = 1
    }
    END { exit(found ? 0 : 1) }
  ' <<< "$version_out"
}

if ! mod_line_present; then
  observed="$(lines_matching $'(^|\t)mod\t' "$version_out")"
  if [ -z "$observed" ]; then observed='(go version -m reported no mod line)'; fi
  emit 'tool checksum mismatch' \
    "$observed"$'\n'"$(printf 'expected: mod\t%s\t%s\t%s' "$GO_LICENSES_MODULE" "$version" "$sum")"
  exit 2
fi

# ── Check, classified: finding, then network-class, then no verdict ─────────

if ! cd "$repo_root/$CHECK_DIR"; then
  emit 'checker could not run' "cannot enter $CHECK_DIR under $repo_root"
  exit 2
fi

check_out=""
run_check() {
  check_out="$(
    "$checker" check ./... "--disallowed_types=$DISALLOWED_TYPES" \
      ${ignore_args[@]+"${ignore_args[@]}"} 2>&1
  )"
}

attempt=0
while :; do
  attempt=$((attempt + 1))
  if run_check; then break; fi

  # A finding first, always: it is the verdict, and it is never retried. An
  # allowance can silence a finding by never producing one; it can never
  # absorb one that the checker still reports.
  if findings="$(finding_lines "$check_out")"; then
    emit 'disallowed license found' "$findings"
    exit 1
  fi

  if [ "$attempt" -eq 1 ] && matches "$NETWORK_RE" "$check_out"; then
    printf 'go-licenses-check: the checker hit a network-class failure with no finding; retrying once in %ss.\n' "$retry_delay"
    sleep "$retry_delay"
    continue
  fi

  if matches "$NETWORK_RE" "$check_out"; then
    emit 'checker could not run' "$(lines_matching "$NETWORK_RE" "$check_out")"
    exit 2
  fi

  # Non-zero, no finding, nothing recognisable. Reporting this as a license
  # failure is the original defect moved one layer down; it gets its own
  # phrase so it can be read and fixed.
  emit 'checker failed before a verdict' "$(tail_excerpt "$check_out")"
  exit 2
done

# ── The allowances, re-asserted against an UNIGNORED report ─────────────────
#
# `check` has just passed, which only means nothing OUTSIDE the ignored paths
# is disallowed. What was inside them is now read back from a report that
# ignores nothing, so each row is measured rather than trusted.

report_csv=""
report_out=""
run_report() {
  local errf="$scratch/report.err" rc
  : > "$errf"
  report_csv="$("$checker" report ./... 2>"$errf")"
  rc=$?
  report_out="$report_csv"$'\n'"$(cat "$errf")"
  return "$rc"
}

attempt=0
while :; do
  attempt=$((attempt + 1))
  if run_report; then break; fi

  if [ "$attempt" -eq 1 ] && matches "$NETWORK_RE" "$report_out"; then
    printf 'go-licenses-check: the report hit a network-class failure; retrying once in %ss.\n' "$retry_delay"
    sleep "$retry_delay"
    continue
  fi

  if matches "$NETWORK_RE" "$report_out"; then
    emit 'checker could not run' "$(lines_matching "$NETWORK_RE" "$report_out")"
    exit 2
  fi

  emit 'checker failed before a verdict' "$(tail_excerpt "$report_out")"
  exit 2
done

# `go-licenses report` at v2.0.1 writes CSV name,licenseURL,licenseName. The
# name is the first field and the license name the last, so a licenseURL that
# ever carries a comma cannot shift the two fields that matter.
report_names=()
report_licenses=()
while IFS= read -r csv_row; do
  [ -n "$csv_row" ] || continue
  csv_name="${csv_row%%,*}"
  csv_license="${csv_row##*,}"
  if [ "$csv_name" = "name" ] && [ "$csv_license" = "licenseName" ]; then continue; fi
  report_names+=("$csv_name")
  report_licenses+=("$csv_license")
done <<< "$report_csv"

exercised=""
row_index=0
while [ "$row_index" -lt "${#libraries[@]}" ]; do
  row_library="${libraries[$row_index]}"
  row_license="${ruled_licenses[$row_index]}"
  row_ruled="${ruled_stamps[$row_index]}"
  row_index=$((row_index + 1))

  hits=0
  reported_license=""
  seen=0
  while [ "$seen" -lt "${#report_names[@]}" ]; do
    if [ "${report_names[$seen]}" = "$row_library" ]; then
      hits=$((hits + 1))
      reported_license="${report_licenses[$seen]}"
    fi
    seen=$((seen + 1))
  done

  if [ "$hits" -eq 0 ]; then
    emit 'allowlist entry unused' \
      "$row_library is allowlisted but go-licenses report does not name it: the allowance covers nothing in the module graph, so the row is stale and must go."
    exit 1
  fi

  if [ "$hits" -gt 1 ]; then
    # The report contradicts itself, so no allowance can be measured against
    # it. That is the tool failing to reach a verdict, never a license verdict.
    emit 'checker failed before a verdict' \
      "go-licenses report named $row_library on $hits rows; it must name each library once."
    exit 2
  fi

  if [ "$reported_license" != "$row_license" ]; then
    emit 'allowlisted library changed license' \
      "$row_library is allowlisted for $row_license (ruled $row_ruled) but go-licenses report reads $reported_license: the ruling covered $row_license only."
    exit 1
  fi

  # `--ignore` silences a path PREFIX. Anything under this row's path that is
  # not itself a row was never ruled on, and would have been silenced without
  # anyone deciding to.
  seen=0
  while [ "$seen" -lt "${#report_names[@]}" ]; do
    covered="${report_names[$seen]}"
    seen=$((seen + 1))
    [ "$covered" != "$row_library" ] || continue
    [ "${covered#"$row_library"}" != "$covered" ] || continue
    if ! is_allowlisted "$covered"; then
      emit 'allowlist prefix covers an unlisted library' \
        "--ignore=$row_library also silences $covered, which is not itself an allowlist row: that library was never ruled on."
      exit 1
    fi
  done

  exercised+="allowlisted: $row_library $reported_license ($row_ruled)"$'\n'
done

emit "no disallowed license found; ${#libraries[@]} allowlisted" "${exercised%$'\n'}"
exit 0
