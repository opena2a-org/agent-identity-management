#!/bin/bash
# ============================================
# AIM Migration Verification Script
# ============================================
# Purpose: Verify all required tables and columns exist
# Usage: ./verify_migrations.sh [DATABASE_URL]
#
# This script ensures fresh deployments have complete schema
# Run this after deploying to catch missing tables/columns
# ============================================

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Database connection (default to production)
DB_URL="${1:-postgresql://aimadmin:AIM-NewProdDB-2025!@#@aim-prod-db-1760993976.postgres.database.azure.com:5432/identity?sslmode=require}"

echo "🔍 Verifying AIM database schema..."
echo ""

# Required tables (from all migrations)
REQUIRED_TABLES=(
    "organizations"
    "users"
    "agents"
    "mcp_servers"
    "api_keys"
    "alerts"
    "audit_logs"
    "verification_events"
    "system_config"
    "sdk_tokens"
    "security_policies"
    "analytics_events"
    "analytics_aggregates"
    "user_registration_requests"
    "migrations"
)

# Check tables
echo "📋 Checking tables..."
MISSING_TABLES=()
for table in "${REQUIRED_TABLES[@]}"; do
    if psql "$DB_URL" -tAc "SELECT 1 FROM information_schema.tables WHERE table_name='$table'" | grep -q 1; then
        echo -e "${GREEN}✓${NC} Table: $table"
    else
        echo -e "${RED}✗${NC} Table: $table (MISSING)"
        MISSING_TABLES+=("$table")
    fi
done

echo ""

# Check columns for critical tables (without associative arrays)
echo "📝 Checking critical columns..."
MISSING_COLUMNS=()

# Users table
echo "  Table: users"
for column in id organization_id email name role status password_hash email_verified force_password_change password_reset_token password_reset_expires_at approved_by approved_at provider provider_id; do
    if psql "$DB_URL" -tAc "SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='$column'" | grep -q 1; then
        echo -e "    ${GREEN}✓${NC} $column"
    else
        echo -e "    ${RED}✗${NC} $column (MISSING)"
        MISSING_COLUMNS+=("users.$column")
    fi
done
echo ""

# Agents table
echo "  Table: agents"
for column in id organization_id name agent_type status trust_score last_verified_at verification_method public_key_fingerprint; do
    if psql "$DB_URL" -tAc "SELECT 1 FROM information_schema.columns WHERE table_name='agents' AND column_name='$column'" | grep -q 1; then
        echo -e "    ${GREEN}✓${NC} $column"
    else
        echo -e "    ${RED}✗${NC} $column (MISSING)"
        MISSING_COLUMNS+=("agents.$column")
    fi
done
echo ""

# Alerts table
echo "  Table: alerts"
for column in id organization_id alert_type severity title is_acknowledged acknowledged_by acknowledged_at; do
    if psql "$DB_URL" -tAc "SELECT 1 FROM information_schema.columns WHERE table_name='alerts' AND column_name='$column'" | grep -q 1; then
        echo -e "    ${GREEN}✓${NC} $column"
    else
        echo -e "    ${RED}✗${NC} $column (MISSING)"
        MISSING_COLUMNS+=("alerts.$column")
    fi
done
echo ""

# User registration requests table
echo "  Table: user_registration_requests"
for column in id email first_name last_name password_hash status organization_id; do
    if psql "$DB_URL" -tAc "SELECT 1 FROM information_schema.columns WHERE table_name='user_registration_requests' AND column_name='$column'" | grep -q 1; then
        echo -e "    ${GREEN}✓${NC} $column"
    else
        echo -e "    ${RED}✗${NC} $column (MISSING)"
        MISSING_COLUMNS+=("user_registration_requests.$column")
    fi
done
echo ""

# Summary
echo "============================================"
if [ ${#MISSING_TABLES[@]} -eq 0 ] && [ ${#MISSING_COLUMNS[@]} -eq 0 ]; then
    echo -e "${GREEN}✅ All schema checks passed!${NC}"
    echo ""
    echo "Total tables verified: ${#REQUIRED_TABLES[@]}"
    exit 0
else
    echo -e "${RED}❌ Schema verification FAILED${NC}"
    echo ""

    if [ ${#MISSING_TABLES[@]} -gt 0 ]; then
        echo "Missing tables (${#MISSING_TABLES[@]}):"
        for table in "${MISSING_TABLES[@]}"; do
            echo "  - $table"
        done
        echo ""
    fi

    if [ ${#MISSING_COLUMNS[@]} -gt 0 ]; then
        echo "Missing columns (${#MISSING_COLUMNS[@]}):"
        for column in "${MISSING_COLUMNS[@]}"; do
            echo "  - $column"
        done
        echo ""
    fi

    echo "🔧 Action Required:"
    echo "  1. Check migration files in apps/backend/migrations/"
    echo "  2. Ensure all migrations ran successfully"
    echo "  3. Run missing migrations manually if needed"
    echo ""
    exit 1
fi
