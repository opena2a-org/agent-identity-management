#!/usr/bin/env bash
# lint-tenant-scoping.sh — invokes the tenantscope-lint Go binary against the
# HTTP handlers package. Wraps the binary so the CI workflow has a stable
# entry point and so developers can run it locally without remembering the
# `go run` path.
#
# Usage:
#   ./scripts/lint-tenant-scoping.sh
#
# Exit code: 0 on pass, 1 on any new violation, 2 on tool error.

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root/apps/backend"

go run ./cmd/tenantscope-lint ./internal/interfaces/http/handlers
