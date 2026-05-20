#!/usr/bin/env bash
#
# Enforces the slice-initialization invariant for AIM handlers + repositories.
# A `var xs []T` declaration produces a nil slice that marshals to JSON `null`,
# which crashes the React dashboard when it tries to iterate the field with
# `.forEach` / `.map` / `.filter`. The lint flags every offender so the fix
# (`xs := make([]T, 0)`) can land before merge.
#
# Run from the repo root, or from any subdirectory.
set -euo pipefail

cd "$(dirname "$0")/.."

cd apps/backend

# Build to a per-run tempdir to avoid CWE-377 symlink attacks on a predictable
# /tmp/<name> path. mktemp -d creates the dir with mode 700 and a random suffix.
LINT_TMP=$(mktemp -d -t jsonarray-lint.XXXXXX)
trap 'rm -rf "$LINT_TMP"' EXIT

go build -o "$LINT_TMP/jsonarray-lint" ./cmd/jsonarray-lint/

"$LINT_TMP/jsonarray-lint" \
  internal/interfaces/http/handlers \
  internal/infrastructure/repository
