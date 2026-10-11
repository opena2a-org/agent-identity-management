#!/usr/bin/env bash
# lint-docs-repo-links.sh — reject a link to the `opena2a` GitHub account in the
# repository's files.
#
# The project lives under the `opena2a-org` organization. The `opena2a` account
# on GitHub is not the project's, and the project has no repository under it,
# so a clone URL, issue link or module path under github.com/opena2a/ does not
# resolve: the clone fails and the link is a 404.
# The guides carried such links for the repository under its current name and
# under an older one (`identity`), and source files carried them too: a Go
# import, the Go SDK README the server generated, and repository URLs in the
# Python SDK's example and tests.
#
# Checked: every text file git tracks, and every untracked file git does not
# ignore, so a new file is checked before it is added. Not checked:
#   - CHANGELOG.md files, which record what earlier releases said;
#   - this script and its test, which quote the links they reject;
#   - the import prefix scripts/sync-to-cloud.sh writes into the Go files of the
#     repository it syncs to (DST_IMPORT). It has to equal the module path that
#     repository's go.mod declares, which this repository does not hold, so it
#     is exempt as that one file and prefix; any other link in the script is
#     checked.
# Both the HTTPS form (github.com/opena2a/...) and the SSH form
# (git@github.com:opena2a/...) are rejected, in any letter case.
#
# Everything rejected is printed with its file, line and link.

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

if [[ "$(git rev-parse --show-toplevel 2>/dev/null)" != "$(pwd -P)" ]]; then
    echo "lint-docs-repo-links: $repo_root is not the top of a git work tree; the lint asks git for the files to check" >&2
    exit 2
fi

link_pattern='github\.com[/:]opena2a/[^][:space:]()<>"`'"'"']*'

# <file>:<link> pairs that name no repository on GitHub (see above).
exempt=(
    'scripts/sync-to-cloud.sh:github.com/opena2a/identity/backend/internal/'
)

status=0
matches="$(git grep --untracked -I -i -n -o -E "$link_pattern" -- . \
    ':(exclude,glob)**/CHANGELOG.md' \
    ':(exclude)scripts/lint-docs-repo-links.sh' \
    ':(exclude)scripts/test-lint-docs-repo-links.sh')" || status=$?
if [[ $status -gt 1 ]]; then
    echo "lint-docs-repo-links: git grep failed with exit status $status" >&2
    exit 2
fi

hits=()
while IFS= read -r hit; do
    [[ -n "$hit" ]] || continue
    file="${hit%%:*}"
    link="${hit#*:*:}"
    for pair in "${exempt[@]}"; do
        [[ "$file:$link" == "$pair" ]] && continue 2
    done
    hits+=("$hit")
done <<< "$matches"

if [[ ${#hits[@]} -ne 0 ]]; then
    echo "lint-docs-repo-links: ${#hits[@]} link(s) to the opena2a GitHub account, which is not the project's:" >&2
    printf '  %s\n' "${hits[@]}" >&2
    cat >&2 <<EOF

Fix by pointing each link at the opena2a-org organization:
  before:  git clone https://github.com/opena2a/agent-identity-management.git
  after:   git clone https://github.com/opena2a-org/agent-identity-management.git

Reason: the project has no repository under the opena2a account. A clone from
it fails and an issue link to it is a 404.
EOF
    exit 1
fi

echo "lint-docs-repo-links: ok (no links to the opena2a GitHub account in the repository's files)"
