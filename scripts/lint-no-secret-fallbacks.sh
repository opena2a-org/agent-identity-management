#!/usr/bin/env bash
# lint-no-secret-fallbacks.sh — reject any docker-compose env var of the form
# ${SECRET_NAME:-fallback}. The fallback pattern silently substitutes a known
# (often source-committed) value when the operator forgets to set the env var,
# and is the root cause of CWE-798 in this repo's history.
#
# Two-tier policy:
#
#   REQUIRED_SECRETS — must use ${VAR:?...} (fail-fast). No fallback of ANY kind
#   is allowed, including empty (`:-`). An empty fallback for a required secret
#   lets the container start with VAR="" and produces a confusing second-layer
#   failure inside the application code instead of at compose-time.
#
#   OPTIONAL_SECRETS — `${VAR:-}` (empty fallback) is allowed; non-empty
#   fallbacks remain forbidden. These are credentials for features that are
#   genuinely optional (e.g. SMTP auth — the platform works without email).
#
# Allowed shapes for OPTIONAL_SECRETS only:
#   ${VAR:-}      — empty fallback, feature disabled when unset
#
# Allowed shapes for both tiers:
#   ${VAR}        — fail at compose-time if unset (no default)
#   ${VAR:?msg}   — fail-fast with operator-friendly message
#
# Forbidden everywhere:
#   ${VAR:-non-empty}  — silent fallback to a default (CWE-798)
#
# The list of vars below is the closed set of secret-shaped names AIM exposes.
# Add new names when introducing new secrets; never remove names without
# explicit security review.

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

# REQUIRED_SECRETS: must use ${VAR:?...}. Empty fallback ${VAR:-} is NOT
# allowed for these — an empty value here means the container starts with the
# var blank and the app errors at a later layer with a more confusing message
# than the compose-time :? error.
REQUIRED_SECRETS=(
    POSTGRES_PASSWORD
    REDIS_PASSWORD
    JWT_SECRET
    KEYVAULT_MASTER_KEY
    MINIO_ROOT_PASSWORD
    GRAFANA_ADMIN_PASSWORD
)

# OPTIONAL_SECRETS: ${VAR:-} (empty fallback) is allowed because the feature
# itself is optional. Non-empty fallback is still forbidden.
OPTIONAL_SECRETS=(
    SMTP_PASSWORD
)

compose_files=(docker-compose*.yml)
if [[ ! -e "${compose_files[0]}" ]]; then
    echo "lint-no-secret-fallbacks: no docker-compose*.yml files found in $repo_root" >&2
    exit 1
fi

required_alt="$(IFS='|'; echo "${REQUIRED_SECRETS[*]}")"
optional_alt="$(IFS='|'; echo "${OPTIONAL_SECRETS[*]}")"

# Required-tier pattern: ${(REQUIRED):-anything} is forbidden (even empty).
required_pattern="\\\$\\{(${required_alt}):-"

# Optional-tier pattern: ${(OPTIONAL):-<non-empty>} is forbidden.
optional_pattern="\\\$\\{(${optional_alt}):-[^}]"

fail=0
for f in "${compose_files[@]}"; do
    if matches="$(grep -EHn "$required_pattern" "$f" 2>/dev/null)"; then
        echo "lint-no-secret-fallbacks: REQUIRED secret used \${VAR:-...} in $f (must be \${VAR:?msg})" >&2
        echo "$matches" >&2
        fail=1
    fi
    if matches="$(grep -EHn "$optional_pattern" "$f" 2>/dev/null)"; then
        echo "lint-no-secret-fallbacks: OPTIONAL secret used non-empty fallback in $f (only \${VAR:-} is allowed)" >&2
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

echo "lint-no-secret-fallbacks: ok (${#REQUIRED_SECRETS[@]} required + ${#OPTIONAL_SECRETS[@]} optional names checked across ${#compose_files[@]} compose files)"
