#!/usr/bin/env bash
# sync-to-cloud.sh -- One-directional sync from agent-identity-management to aim-cloud
#
# Usage:
#   ./scripts/sync-to-cloud.sh <source-dir> <target-dir> [--dry-run]
#
# When run from the GitHub Action, source/target are checked-out paths.
# For local use:
#   ./scripts/sync-to-cloud.sh . ../aim-cloud
#   ./scripts/sync-to-cloud.sh . ../aim-cloud --dry-run

set -euo pipefail

# -- Arguments ----------------------------------------------------------------

SOURCE="${1:?Usage: sync-to-cloud.sh <source-dir> <target-dir> [--dry-run]}"
TARGET="${2:?Usage: sync-to-cloud.sh <source-dir> <target-dir> [--dry-run]}"
DRY_RUN=false

for arg in "${@:3}"; do
  case "$arg" in
    --dry-run) DRY_RUN=true ;;
    *) echo "Unknown flag: $arg"; exit 1 ;;
  esac
done

SOURCE="$(cd "$SOURCE" && pwd)"
TARGET="$(cd "$TARGET" && pwd)"

# -- Import paths --------------------------------------------------------------

SRC_IMPORT="github.com/opena2a-org/agent-identity-management/apps/backend/internal/"
DST_IMPORT="github.com/opena2a/identity/backend/internal/"

# -- Protected files manifest --------------------------------------------------
# Read from aim-cloud/.sync-protect if it exists, otherwise use built-in list.

PROTECT_FILE="$TARGET/.sync-protect"
PROTECTED_PATTERNS=()

load_protected_patterns() {
  if [[ -f "$PROTECT_FILE" ]]; then
    while IFS= read -r line; do
      # Skip comments and blank lines
      [[ "$line" =~ ^#.*$ || -z "$line" ]] && continue
      PROTECTED_PATTERNS+=("$line")
    done < "$PROTECT_FILE"
  else
    # Fallback built-in list
    PROTECTED_PATTERNS=(
      "apps/backend/cmd/server/main.go"
      "apps/backend/internal/infrastructure/stripe/"
      "apps/web/app/auth/login/page.tsx"
      "apps/web/app/auth/google/"
      "apps/web/app/auth/callback/"
      "apps/web/app/auth/error/"
      "apps/web/app/settings/"
      "apps/web/app/onboarding/"
      "apps/web/app/get-started/"
      "apps/web/app/terms/"
      "apps/web/app/privacy/"
      "apps/web/app/device/"
      "apps/web/lib/api.ts"
      "apps/web/hooks/use-plan-usage.ts"
      "apps/web/proxy.ts"
      "apps/web/components/dashboard/sidebar.tsx"
    )
  fi
}

load_protected_patterns

# -- Counters ------------------------------------------------------------------

ADDED=0
MODIFIED=0
SKIPPED=0
IMPORT_REWRITES=0

# -- Helpers -------------------------------------------------------------------

is_protected() {
  local relpath="$1"
  for pattern in "${PROTECTED_PATTERNS[@]}"; do
    # Exact file match
    if [[ "$relpath" == "$pattern" ]]; then
      return 0
    fi
    # Directory prefix match (pattern ends with /)
    if [[ "$pattern" == */ && "$relpath" == "$pattern"* ]]; then
      return 0
    fi
    # Stripe path check
    if [[ "$relpath" == *"/stripe/"* ]]; then
      return 0
    fi
  done
  return 1
}

log_info() {
  echo "[SYNC] $*"
}

log_skip() {
  echo "[SKIP] $*"
  SKIPPED=$((SKIPPED + 1))
}

log_dry() {
  echo "[DRY ] $*"
}

# -- Backend sync --------------------------------------------------------------

sync_backend() {
  log_info "--- Backend Go files ---"

  local src_backend="$SOURCE/apps/backend"
  local dst_backend="$TARGET/apps/backend"

  if [[ ! -d "$src_backend" ]]; then
    log_info "No backend directory in source, skipping"
    return
  fi

  # Find all Go files in source backend
  while IFS= read -r -d '' src_file; do
    local relpath="${src_file#"$SOURCE/"}"
    local dst_file="$TARGET/$relpath"

    if is_protected "$relpath"; then
      log_skip "$relpath (protected)"
      continue
    fi

    if $DRY_RUN; then
      if [[ -f "$dst_file" ]]; then
        log_dry "MODIFY $relpath"
      else
        log_dry "ADD    $relpath"
      fi
      continue
    fi

    # Ensure target directory exists
    mkdir -p "$(dirname "$dst_file")"

    if [[ -f "$dst_file" ]]; then
      # Copy and check if content changed
      local old_hash new_hash
      old_hash=$(shasum -a 256 "$dst_file" | cut -d' ' -f1)
      cp "$src_file" "$dst_file"
      new_hash=$(shasum -a 256 "$dst_file" | cut -d' ' -f1)
      if [[ "$old_hash" != "$new_hash" ]]; then
        log_info "MODIFY $relpath"
        MODIFIED=$((MODIFIED + 1))
      fi
    else
      cp "$src_file" "$dst_file"
      log_info "ADD    $relpath"
      ADDED=$((ADDED + 1))
    fi
  done < <(find "$src_backend" -name '*.go' -print0)

  # Also sync non-Go backend files that matter: schema, scripts, seed, tests
  for subdir in schema scripts seed tests; do
    local src_sub="$src_backend/$subdir"
    local dst_sub="$dst_backend/$subdir"
    if [[ -d "$src_sub" ]]; then
      if $DRY_RUN; then
        log_dry "SYNC   apps/backend/$subdir/"
      else
        mkdir -p "$dst_sub"
        rsync -a --delete "$src_sub/" "$dst_sub/"
        log_info "SYNCED apps/backend/$subdir/"
      fi
    fi
  done
}

# -- Import path rewriting ----------------------------------------------------

rewrite_imports() {
  log_info "--- Rewriting Go import paths ---"

  local dst_backend="$TARGET/apps/backend"

  if [[ ! -d "$dst_backend" ]]; then
    return
  fi

  if $DRY_RUN; then
    local count
    count=$(grep -rl "$SRC_IMPORT" "$dst_backend" --include='*.go' 2>/dev/null | wc -l | tr -d ' ' || true)
    log_dry "Would rewrite imports in $count files"
    return
  fi

  while IFS= read -r -d '' gofile; do
    if grep -q "$SRC_IMPORT" "$gofile" 2>/dev/null; then
      if [[ "$OSTYPE" == "darwin"* ]]; then
        sed -i '' "s|${SRC_IMPORT}|${DST_IMPORT}|g" "$gofile"
      else
        sed -i "s|${SRC_IMPORT}|${DST_IMPORT}|g" "$gofile"
      fi
      IMPORT_REWRITES=$((IMPORT_REWRITES + 1))
    fi
  done < <(find "$dst_backend" -name '*.go' -print0)

  log_info "Rewrote imports in $IMPORT_REWRITES files"
}

# -- Frontend sync -------------------------------------------------------------

sync_frontend() {
  log_info "--- Frontend files ---"

  local src_web="$SOURCE/apps/web"
  local dst_web="$TARGET/apps/web"

  if [[ ! -d "$src_web" ]]; then
    log_info "No web directory in source, skipping"
    return
  fi

  # Sync specific frontend directories that are shared
  local shared_dirs=(
    "app/dashboard"
    "app/api"
    "app/auth/register"
    "app/auth/change-password"
    "app/auth/forgot-password"
    "app/auth/reset-password"
    "components"
    "lib"
    "hooks"
    "tests"
  )

  for dir in "${shared_dirs[@]}"; do
    local src_dir="$src_web/$dir"
    local dst_dir="$dst_web/$dir"

    if [[ ! -d "$src_dir" ]]; then
      continue
    fi

    # Sync files one at a time to respect protected patterns
    while IFS= read -r -d '' src_file; do
      local relpath="${src_file#"$SOURCE/"}"
      local dst_file="$TARGET/$relpath"

      if is_protected "$relpath"; then
        log_skip "$relpath (protected)"
        continue
      fi

      if $DRY_RUN; then
        if [[ -f "$dst_file" ]]; then
          log_dry "MODIFY $relpath"
        else
          log_dry "ADD    $relpath"
        fi
        continue
      fi

      mkdir -p "$(dirname "$dst_file")"

      if [[ -f "$dst_file" ]]; then
        local old_hash new_hash
        old_hash=$(shasum -a 256 "$dst_file" | cut -d' ' -f1)
        cp "$src_file" "$dst_file"
        new_hash=$(shasum -a 256 "$dst_file" | cut -d' ' -f1)
        if [[ "$old_hash" != "$new_hash" ]]; then
          log_info "MODIFY $relpath"
          MODIFIED=$((MODIFIED + 1))
        fi
      else
        cp "$src_file" "$dst_file"
        log_info "ADD    $relpath"
        ADDED=$((ADDED + 1))
      fi
    done < <(find "$src_dir" -type f \
      ! -name '*.d.ts' \
      ! -path '*/node_modules/*' \
      ! -path '*/.next/*' \
      -print0)
  done

  # Sync root-level config files (package.json is NOT synced -- deps differ)
  local root_files=(
    "tailwind.config.js"
    "postcss.config.js"
    "tsconfig.json"
    "eslint.config.mjs"
    "vitest.config.ts"
    "middleware.ts"
  )

  for f in "${root_files[@]}"; do
    local src_file="$src_web/$f"
    local dst_file="$dst_web/$f"
    local relpath="apps/web/$f"

    if [[ ! -f "$src_file" ]]; then
      continue
    fi

    if is_protected "$relpath"; then
      log_skip "$relpath (protected)"
      continue
    fi

    if $DRY_RUN; then
      log_dry "SYNC   $relpath"
      continue
    fi

    if [[ -f "$dst_file" ]]; then
      local old_hash new_hash
      old_hash=$(shasum -a 256 "$dst_file" | cut -d' ' -f1)
      cp "$src_file" "$dst_file"
      new_hash=$(shasum -a 256 "$dst_file" | cut -d' ' -f1)
      if [[ "$old_hash" != "$new_hash" ]]; then
        log_info "MODIFY $relpath"
        MODIFIED=$((MODIFIED + 1))
      fi
    else
      cp "$src_file" "$dst_file"
      log_info "ADD    $relpath"
      ADDED=$((ADDED + 1))
    fi
  done
}

# -- Migrations sync (additive only) ------------------------------------------

sync_migrations() {
  log_info "--- Migrations (additive only) ---"

  local src_migrations="$SOURCE/apps/backend/migrations"
  local dst_migrations="$TARGET/apps/backend/migrations"

  if [[ ! -d "$src_migrations" ]]; then
    log_info "No migrations in source, skipping"
    return
  fi

  mkdir -p "$dst_migrations"

  for src_file in "$src_migrations"/*.sql; do
    [[ -f "$src_file" ]] || continue
    local fname
    fname=$(basename "$src_file")
    local dst_file="$dst_migrations/$fname"
    local relpath="apps/backend/migrations/$fname"

    # Respect .sync-protect: cloud pins some upstream migrations out of its tree
    # (renamed/renumbered duplicates, bring-up-only test data). The basename
    # guard below only skips identical filenames -- it misses a protected
    # migration that was renumbered in cloud, re-syncing a duplicate number.
    if is_protected "$relpath"; then
      log_skip "$relpath (protected)"
      continue
    fi

    if [[ -f "$dst_file" ]]; then
      # Already exists -- do not overwrite (migrations are immutable)
      continue
    fi

    if $DRY_RUN; then
      log_dry "ADD    apps/backend/migrations/$fname"
    else
      cp "$src_file" "$dst_file"
      log_info "ADD    apps/backend/migrations/$fname"
      ADDED=$((ADDED + 1))
    fi
  done
}

# -- SDK sync ------------------------------------------------------------------

sync_sdk() {
  log_info "--- SDK files ---"

  local src_sdk="$SOURCE/sdk"
  local dst_sdk="$TARGET/sdk"

  if [[ ! -d "$src_sdk" ]]; then
    log_info "No SDK directory in source, skipping"
    return
  fi

  if $DRY_RUN; then
    log_dry "SYNC   sdk/"
  else
    mkdir -p "$dst_sdk"
    rsync -a --delete \
      --exclude='node_modules' \
      --exclude='__pycache__' \
      --exclude='.venv' \
      "$src_sdk/" "$dst_sdk/"
    log_info "SYNCED sdk/"
  fi
}

# -- Dependency reconciliation -------------------------------------------------
# Public repo and aim-cloud keep independent go.mod files (`.sync-protect`-ed):
# public has AIM Secrets backends (cloud.google.com/secretmanager, Azure
# keyvault, HashiCorp vault, etc.); aim-cloud has Stripe. When the sync copies
# Go source that imports a dep public has added, aim-cloud's go.mod is missing
# it and `go build` reports `updates to go.mod needed`. Running `go mod tidy`
# in aim-cloud's tree pulls in the new transitive deps while keeping Stripe
# (because protected main.go + internal/infrastructure/stripe/ still reference
# it). Without this, every public dependency bump breaks the sync pipeline.

reconcile_go_deps() {
  if $DRY_RUN; then
    log_info "--- Dry run: skipping go mod tidy ---"
    return
  fi

  log_info "--- Reconciling aim-cloud go.mod with synced code ---"

  local dst_backend="$TARGET/apps/backend"

  if [[ ! -f "$dst_backend/go.mod" ]]; then
    log_info "No go.mod in target backend, skipping tidy"
    return
  fi

  if (cd "$dst_backend" && go mod tidy 2>&1); then
    log_info "go mod tidy: DONE"
  else
    echo ""
    echo "========================================="
    echo "go mod tidy FAILED -- aim-cloud cannot reconcile deps."
    echo "Likely cause: synced code imports a package public added but"
    echo "the module proxy cannot resolve from aim-cloud's module path."
    echo "========================================="
    exit 1
  fi
}

# -- Build verification --------------------------------------------------------

verify_build() {
  if $DRY_RUN; then
    log_info "--- Dry run: skipping build verification ---"
    return
  fi

  log_info "--- Verifying Go build ---"

  local dst_backend="$TARGET/apps/backend"

  if [[ ! -f "$dst_backend/go.mod" ]]; then
    log_info "No go.mod in target backend, skipping build check"
    return
  fi

  if (cd "$dst_backend" && go build ./... 2>&1); then
    log_info "Build verification: PASSED"
  else
    echo ""
    echo "========================================="
    echo "BUILD FAILED -- sync introduced errors."
    echo "Review the changes and fix before merging."
    echo "========================================="
    exit 1
  fi
}

# -- Main ----------------------------------------------------------------------

echo ""
echo "============================================"
if $DRY_RUN; then
  echo "  Sync Preview (DRY RUN)"
else
  echo "  Syncing agent-identity-management -> aim-cloud"
fi
echo "============================================"
echo "  Source: $SOURCE"
echo "  Target: $TARGET"
echo "  Protected patterns: ${#PROTECTED_PATTERNS[@]}"
echo "============================================"
echo ""

sync_backend
rewrite_imports
sync_frontend
sync_migrations
sync_sdk
reconcile_go_deps
verify_build

echo ""
echo "============================================"
echo "  Sync Summary"
echo "============================================"
echo "  Files added:     $ADDED"
echo "  Files modified:  $MODIFIED"
echo "  Files skipped:   $SKIPPED"
echo "  Import rewrites: $IMPORT_REWRITES"
if $DRY_RUN; then
  echo "  Mode: DRY RUN (no files changed)"
fi
echo "============================================"
echo ""
