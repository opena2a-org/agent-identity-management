#!/usr/bin/env bash
# lint-docs-repo-links.sh — reject a link to the `opena2a` GitHub account in the
# repository's Markdown pages.
#
# The project lives under the `opena2a-org` organization. There is no `opena2a`
# account on GitHub, so a clone URL, issue link or module path under
# github.com/opena2a/ does not resolve: the clone fails and the link is a 404.
# The guides carried such links for the repository under its current name and
# under an older one (`identity`).
#
# Checked: every *.md file in the repository except CHANGELOG.md files, which
# record what earlier releases said. Both the HTTPS form (github.com/opena2a/...)
# and the SSH form (git@github.com:opena2a/...) are rejected, in any letter case.
#
# Everything rejected is printed with its file, line and link.

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

link_pattern='github\.com[/:]opena2a/[^][:space:]()<>"`'"'"']*'

hits=()
while IFS= read -r hit; do
    hits+=("${hit#./}")
done < <(grep -rIino -E \
    --include='*.md' --exclude='CHANGELOG.md' \
    --exclude-dir=node_modules --exclude-dir=.git \
    "$link_pattern" . || true)

if [[ ${#hits[@]} -ne 0 ]]; then
    echo "lint-docs-repo-links: ${#hits[@]} link(s) to the opena2a GitHub account, which does not exist:" >&2
    printf '  %s\n' "${hits[@]}" >&2
    cat >&2 <<EOF

Fix by pointing each link at the opena2a-org organization:
  before:  git clone https://github.com/opena2a/agent-identity-management.git
  after:   git clone https://github.com/opena2a-org/agent-identity-management.git

Reason: there is no opena2a account on GitHub. A clone from it fails and an
issue link to it is a 404.
EOF
    exit 1
fi

echo "lint-docs-repo-links: ok (no links to the opena2a GitHub account in Markdown)"
