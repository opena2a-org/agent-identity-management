#!/bin/bash

# Migration Verification Script
# Ensures all critical migrations are present before deployment

set -e

MIGRATIONS_DIR="apps/backend/migrations"
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "========================================="
echo "Migration Verification"
echo "========================================="
echo ""

# Critical migrations that must exist
CRITICAL_MIGRATIONS=(
    "001_initial_schema.sql"
    "028_add_factors_column_to_trust_scores.sql"
    "029_set_default_values_for_trust_score_factors.sql"
    "030_fix_agents_trust_score_scale.sql"
)

MISSING=0

echo "Checking critical migrations..."
echo ""

for migration in "${CRITICAL_MIGRATIONS[@]}"; do
    if [ -f "$MIGRATIONS_DIR/$migration" ]; then
        echo -e "${GREEN}✓${NC} $migration"
    else
        echo -e "${RED}✗${NC} $migration (MISSING)"
        MISSING=$((MISSING + 1))
    fi
done

echo ""

# Count total migrations
TOTAL_MIGRATIONS=$(find "$MIGRATIONS_DIR" -name "*.sql" | wc -l | tr -d ' ')
echo "Total migrations found: $TOTAL_MIGRATIONS"

echo ""

if [ $MISSING -gt 0 ]; then
    echo -e "${RED}=========================================${NC}"
    echo -e "${RED}❌ Migration verification FAILED${NC}"
    echo -e "${RED}=========================================${NC}"
    echo ""
    echo "Missing $MISSING critical migration(s)"
    echo ""
    echo "CRITICAL: The following migrations are required for the"
    echo "capability reports endpoint to work correctly:"
    echo ""
    echo "  - Migration 028: Adds factors JSONB column"
    echo "  - Migration 029: Allows NULL for 8-factor columns"
    echo "  - Migration 030: Fixes trust_score scale"
    echo ""
    echo "Without these migrations, the capability reports endpoint"
    echo "will fail with database schema errors."
    echo ""
    exit 1
else
    echo -e "${GREEN}=========================================${NC}"
    echo -e "${GREEN}✅ All critical migrations present${NC}"
    echo -e "${GREEN}=========================================${NC}"
    echo ""
    echo "Deployment can proceed safely."
    echo ""
    exit 0
fi
