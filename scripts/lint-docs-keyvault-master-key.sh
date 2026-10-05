#!/usr/bin/env bash
# lint-docs-keyvault-master-key.sh — reject a Markdown page under docs/ that
# sets ENVIRONMENT to anything other than development in a section that never
# names KEYVAULT_MASTER_KEY.
#
# Agent private keys are encrypted under KEYVAULT_MASTER_KEY. Only
# ENVIRONMENT=development may run without it, on a key generated at each start
# that is lost when the process exits. A deployment guide that shows a
# production, staging or testing configuration without the key leaves the
# operator with a server that refuses to start, or one whose encrypted data
# cannot be read after a restart.
#
# A section runs from one Markdown heading to the next. Lines inside a fenced
# code block never start a section, so a `# comment` in a shell sample does not
# split one. Every ENVIRONMENT line in a section without the key is rejected
# with its file, line and value. An ENVIRONMENT prefix on a `go test` command
# configures the test process, not a server, and is not checked.

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

if [[ ! -d docs ]]; then
    echo "lint-docs-keyvault-master-key: no docs/ directory found in $repo_root" >&2
    exit 1
fi

checked=0
rejected=()
while IFS= read -r -d '' page; do
    checked=$((checked + 1))
    while IFS= read -r hit; do
        rejected+=("$hit")
    done < <(awk -v page="$page" '
        function flush(    i) {
            if (!has_key) {
                for (i = 1; i <= n; i++) print page ":" hits[i]
            }
            n = 0
            has_key = 0
        }
        /^[ \t]*(```|~~~)/ { in_fence = !in_fence }
        !in_fence && /^#{1,6}[ \t]/ { flush() }
        /KEYVAULT_MASTER_KEY/ { has_key = 1 }
        {
            line = $0
            while (match(line, /(^|[^A-Za-z0-9_])ENVIRONMENT[ \t]*[=:][ \t]*["'"'"'`]?[A-Za-z]+/)) {
                value = substr(line, RSTART, RLENGTH)
                sub(/^.*ENVIRONMENT[ \t]*[=:][ \t]*["'"'"'`]?/, "", value)
                line = substr(line, RSTART + RLENGTH)
                # A prefix on `go test` configures the test process, not a server.
                if (line ~ /^[ \t]+go[ \t]+test([ \t]|$)/) continue
                if (value != "development") hits[++n] = NR ":ENVIRONMENT=" value
            }
        }
        END { flush() }
    ' "$page")
done < <(find docs -type f -name '*.md' -print0 | sort -z)

if [[ ${#rejected[@]} -ne 0 ]]; then
    echo "lint-docs-keyvault-master-key: ${#rejected[@]} non-development ENVIRONMENT setting(s) in docs/ in a section that never names KEYVAULT_MASTER_KEY:" >&2
    printf '  %s\n' "${rejected[@]}" >&2
    cat >&2 <<EOF

Fix by naming the key in the same section, next to the ENVIRONMENT setting:
  ENVIRONMENT=production
  KEYVAULT_MASTER_KEY=<output of: openssl rand -base64 32>

Reason: only ENVIRONMENT=development runs without KEYVAULT_MASTER_KEY. The
remedy for a missing key is the key, never ENVIRONMENT=development.
EOF
    exit 1
fi

echo "lint-docs-keyvault-master-key: ok ($checked pages in docs/ checked)"
