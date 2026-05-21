#!/usr/bin/env bash
# lint-tenant-scoping.sh — invokes the tenantscope-lint Go binary in
# dual-scan mode: checks the HTTP handlers package for missing
# LoadOwned guards (class #1 path-id IDORs) AND the application
# services package for orgID-accepted-but-unused parameters (class #3
# IDORs). Wraps the binary so the CI workflow has a stable entry point
# and so developers can run it locally without remembering the `go run`
# path.
#
# Usage:
#   ./scripts/lint-tenant-scoping.sh
#
# Exit code: 0 on pass, 1 on any new violation in EITHER scan, 2 on
# tool error.

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root/apps/backend"

# No-arg invocation runs the dual scan (handlers + application).
# Both scans must pass; either failure exits non-zero.
go run ./cmd/tenantscope-lint
