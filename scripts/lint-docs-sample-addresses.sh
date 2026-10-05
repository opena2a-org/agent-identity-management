#!/usr/bin/env bash
# lint-docs-sample-addresses.sh — reject an email address in the Markdown pages
# under docs/ unless its domain is a placeholder or the project's own.
#
# Sample output in docs/ is copied from a real terminal, and a real terminal
# prints the account of whoever ran the command: an Azure CLI sample carried
# the signed-in user's own address into a published page that way. Sample
# accounts belong on a reserved domain (RFC 2606: example.com, example.net,
# example.org, and the .example, .test, .invalid and .localhost names), so a
# page never shows a real person's address.
#
# Allowed:
#   ALLOWED_DOMAINS — the domain itself and any subdomain of it
#   ALLOWED_TLDS    — any domain under one of these reserved top-level names
#
# Everything else is rejected with its file, line and address.

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

# example.* are reserved for documentation; opena2a.org is the project's own
# published contact domain; company.com and yourdomain.com are the generic
# placeholders the deployment guides already use.
ALLOWED_DOMAINS=(
    example.com
    example.net
    example.org
    opena2a.org
    company.com
    yourdomain.com
)

ALLOWED_TLDS=(
    example
    test
    invalid
    localhost
)

if [[ ! -d docs ]]; then
    echo "lint-docs-sample-addresses: no docs/ directory found in $repo_root" >&2
    exit 1
fi

allowed_domain() {
    local domain="$1" d
    for d in "${ALLOWED_DOMAINS[@]}"; do
        if [[ "$domain" == "$d" || "$domain" == *".$d" ]]; then
            return 0
        fi
    done
    for d in "${ALLOWED_TLDS[@]}"; do
        if [[ "${domain##*.}" == "$d" ]]; then
            return 0
        fi
    done
    return 1
}

address_pattern='[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}'

checked=0
rejected=()
while IFS= read -r hit; do
    # hit is <file>:<line>:<address>
    address="${hit##*:}"
    domain="$(printf '%s' "${address#*@}" | tr '[:upper:]' '[:lower:]')"
    checked=$((checked + 1))
    if ! allowed_domain "$domain"; then
        rejected+=("$hit")
    fi
done < <(grep -rIno -E --include='*.md' "$address_pattern" docs || true)

if [[ ${#rejected[@]} -ne 0 ]]; then
    echo "lint-docs-sample-addresses: ${#rejected[@]} address(es) in docs/ outside the placeholder domains:" >&2
    printf '  %s\n' "${rejected[@]}" >&2
    cat >&2 <<EOF

Fix by replacing each address with one on a reserved domain:
  before:  # Logged in as: jane.doe@<a real domain>
  after:   # Logged in as: admin@example.com

Reason: sample output is copied from a real session and carries the address of
whoever ran it. A published page should never show a real person's address.
EOF
    exit 1
fi

echo "lint-docs-sample-addresses: ok ($checked addresses in docs/ checked)"
