#!/usr/bin/env bash
# lint-no-secret-fallbacks.sh — reject any docker-compose env var of the form
# ${SECRET_NAME:-fallback}. The fallback pattern silently substitutes a known
# (often source-committed) value when the operator forgets to set the env var,
# and is the root cause of CWE-798 in this repo's history.
#
# Allowed shapes (these PASS):
#   ${SECRET_NAME}                    — fail at compose-time if unset
#   ${SECRET_NAME:?Set SECRET in .env} — fail-fast with operator-friendly message
#   ${SECRET_NAME:-}                  — empty fallback (treats the secret as
#                                       optional, does not leak a credential)
#
# Forbidden shapes (these FAIL):
#   ${SECRET_NAME:-known-bad-value}   — silent fallback to a non-empty default
#
# The list of vars below is the closed set of secret-shaped names AIM exposes.
# Add new names when introducing new secrets; never remove names without
# explicit security review.

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

# Names treated as secret-shaped. Order matches docker-compose.yml usage order
# (POSTGRES_PASSWORD/REDIS_PASSWORD first since they appear in multiple jobs).
SECRET_VARS=(
    POSTGRES_PASSWORD
    REDIS_PASSWORD
    JWT_SECRET
    KEYVAULT_MASTER_KEY
    MINIO_ROOT_PASSWORD
    GRAFANA_ADMIN_PASSWORD
    SMTP_PASSWORD
)

compose_files=(docker-compose*.yml)
if [[ ! -e "${compose_files[0]}" ]]; then
    echo "lint-no-secret-fallbacks: no docker-compose*.yml files found in $repo_root" >&2
    exit 1
fi

# Build the alternation pattern once.
alt="$(IFS='|'; echo "${SECRET_VARS[*]}")"
# ${(SECRET_NAME):-<non-empty>} — only non-empty fallbacks leak credentials.
# ${VAR:-} is allowed: empty fallback is the "feature optional" pattern and
# does not introduce a hardcoded credential.
pattern="\\\$\\{(${alt}):-[^}]"

fail=0
for f in "${compose_files[@]}"; do
    # grep -E uses ERE; -n prints line numbers; capture both to differentiate
    # locations in the failure message.
    if matches="$(grep -EHn "$pattern" "$f" 2>/dev/null)"; then
        echo "lint-no-secret-fallbacks: forbidden \${VAR:-fallback} pattern in $f" >&2
        echo "$matches" >&2
        fail=1
    fi
done

if [[ $fail -ne 0 ]]; then
    cat >&2 <<EOF

Fix by replacing each match with the fail-fast operator:
  before:  \${VAR:-some-default}
  after:   \${VAR:?Set VAR in .env (run ./scripts/gen-dev-secrets.sh > .env to generate one)}

Reason: a fallback default for a secret-shaped env var is CWE-798 (Use of Hard-
coded Credentials). Operators who forget to override silently inherit the
default, and a default committed in source is by definition not a secret.
EOF
    exit 1
fi

echo "lint-no-secret-fallbacks: ok (${#SECRET_VARS[@]} names checked across ${#compose_files[@]} compose files)"
