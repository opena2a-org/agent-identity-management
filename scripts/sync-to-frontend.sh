#!/usr/bin/env bash
# sync-to-frontend.sh -- Sync apps/web/ from agent-identity-management to aim-frontend
#
# Usage:
#   ./scripts/sync-to-frontend.sh <source-dir> <target-dir> [--dry-run]
#
# For local use:
#   ./scripts/sync-to-frontend.sh . ../aim-frontend
#   ./scripts/sync-to-frontend.sh . ../aim-frontend --dry-run

set -euo pipefail

# -- Arguments ----------------------------------------------------------------

SOURCE="${1:?Usage: sync-to-frontend.sh <source-dir> <target-dir> [--dry-run]}"
TARGET="${2:?Usage: sync-to-frontend.sh <source-dir> <target-dir> [--dry-run]}"
DRY_RUN=false

for arg in "${@:3}"; do
  case "$arg" in
    --dry-run) DRY_RUN=true ;;
    *) echo "Unknown flag: $arg"; exit 1 ;;
  esac
done

SOURCE="$(cd "$SOURCE" && pwd)"
TARGET="$(cd "$TARGET" && pwd)"

WEB_SRC="$SOURCE/apps/web"

if [[ ! -d "$WEB_SRC" ]]; then
  echo "ERROR: $WEB_SRC does not exist"
  exit 1
fi

# -- Protected files manifest --------------------------------------------------

PROTECT_FILE="$TARGET/.sync-protect"
PROTECTED_PATTERNS=()

if [[ -f "$PROTECT_FILE" ]]; then
  while IFS= read -r line; do
    [[ "$line" =~ ^#.*$ || -z "$line" ]] && continue
    PROTECTED_PATTERNS+=("$line")
  done < "$PROTECT_FILE"
fi

is_protected() {
  local rel_path="$1"
  for pattern in "${PROTECTED_PATTERNS[@]}"; do
    # Directory pattern (trailing slash) -- match anything under it
    if [[ "$pattern" == */ ]]; then
      if [[ "$rel_path" == "${pattern}"* || "$rel_path" == "${pattern%/}" ]]; then
        return 0
      fi
    else
      if [[ "$rel_path" == "$pattern" ]]; then
        return 0
      fi
    fi
  done
  return 1
}

# -- Sync logic ---------------------------------------------------------------

COPIED=0
SKIPPED=0
PROTECTED_COUNT=0

echo "=== sync-to-frontend.sh ==="
echo "Source: $WEB_SRC"
echo "Target: $TARGET"
echo "Protected patterns: ${#PROTECTED_PATTERNS[@]}"
echo "Dry run: $DRY_RUN"
echo ""

# Exclude patterns for find
EXCLUDE_PATTERNS=(-not -path '*/node_modules/*' -not -path '*/.next/*' -not -name '*.bak' -not -name '*.bkp' -not -name '*.final' -not -name '*.fmt' -not -name 'Dockerfile' -not -name 'staticwebapp.config.json' -not -name 'tsconfig.tsbuildinfo')

while IFS= read -r -d '' src_file; do
  # Get path relative to apps/web/
  rel_path="${src_file#"$WEB_SRC/"}"

  # Skip apps/ subdirectory (backend migrations that leaked into web dir)
  if [[ "$rel_path" == apps/* ]]; then
    continue
  fi

  # Check if protected
  if is_protected "$rel_path"; then
    echo "  PROTECTED: $rel_path"
    PROTECTED_COUNT=$((PROTECTED_COUNT + 1))
    continue
  fi

  dst_file="$TARGET/$rel_path"

  # Check if file differs
  if [[ -f "$dst_file" ]] && diff -q "$src_file" "$dst_file" > /dev/null 2>&1; then
    SKIPPED=$((SKIPPED + 1))
    continue
  fi

  if [[ "$DRY_RUN" == true ]]; then
    echo "  WOULD COPY: $rel_path"
  else
    mkdir -p "$(dirname "$dst_file")"
    cp "$src_file" "$dst_file"
    echo "  COPIED: $rel_path"
  fi
  COPIED=$((COPIED + 1))
done < <(find "$WEB_SRC" -type f "${EXCLUDE_PATTERNS[@]}" -print0 | sort -z)

echo ""
echo "=== Summary ==="
echo "Copied: $COPIED"
echo "Unchanged: $SKIPPED"
echo "Protected: $PROTECTED_COUNT"

if [[ "$DRY_RUN" == true ]]; then
  echo "(dry run -- no files were modified)"
fi
