#!/bin/bash

# Analytics Implementation Test Script
# Tests real-time analytics tracking and reporting

set -e

BASE_URL="${BASE_URL:-http://localhost:8080}"

echo "🧪 Testing Analytics Implementation"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Step 1: Login
echo "1️⃣  Logging in..."
TOKEN=$(curl -s -X POST "$BASE_URL/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"email": "admin@opena2a.org", "password": "Admin2025!Secure"}' \
  | jq -r '.token')

if [ -z "$TOKEN" ] || [ "$TOKEN" = "null" ]; then
  echo "❌ Login failed!"
  exit 1
fi

echo "✅ Login successful (token: ${TOKEN:0:30}...)"
echo ""

# Step 2: Make API calls to generate analytics data
echo "2️⃣  Generating analytics data (20 API calls)..."
for i in {1..20}; do
  curl -s -H "Authorization: Bearer $TOKEN" "$BASE_URL/api/v1/agents" > /dev/null
  printf "."
done
echo ""
echo "✅ API calls completed"
echo ""

# Wait for aggregation
echo "⏳ Waiting 2 seconds for data aggregation..."
sleep 2
echo ""

# Step 3: Test analytics endpoints
echo "3️⃣  Testing analytics endpoints..."
echo ""

# Test 1: Usage Statistics
echo "📊 Test 1: Usage Statistics (day)"
USAGE=$(curl -s -H "Authorization: Bearer $TOKEN" \
  "$BASE_URL/api/v1/analytics/usage?period=day")
echo "$USAGE" | jq '.'

API_CALLS=$(echo "$USAGE" | jq -r '.api_calls')
DATA_VOLUME=$(echo "$USAGE" | jq -r '.data_volume')

if [ "$API_CALLS" -ge 20 ]; then
  echo "✅ API calls tracked: $API_CALLS (expected >= 20)"
else
  echo "❌ API calls NOT tracked correctly: $API_CALLS (expected >= 20)"
fi

if (( $(echo "$DATA_VOLUME > 0" | bc -l) )); then
  echo "✅ Data volume tracked: ${DATA_VOLUME} MB"
else
  echo "❌ Data volume NOT tracked: $DATA_VOLUME"
fi
echo ""

# Test 2: Trust Score Trends
echo "📈 Test 2: Trust Score Trends (4 weeks)"
TRENDS=$(curl -s -H "Authorization: Bearer $TOKEN" \
  "$BASE_URL/api/v1/analytics/trust-score-trends?weeks=4")
echo "$TRENDS" | jq '.'
echo ""

# Test 3: Verification Activity
echo "📅 Test 3: Verification Activity (6 months)"
ACTIVITY=$(curl -s -H "Authorization: Bearer $TOKEN" \
  "$BASE_URL/api/v1/analytics/verification-activity?months=6")
echo "$ACTIVITY" | jq '.'
echo ""

# Test 4: Agent Activity
echo "👤 Test 4: Agent Activity"
AGENTS=$(curl -s -H "Authorization: Bearer $TOKEN" \
  "$BASE_URL/api/v1/analytics/agents/activity?limit=5")
echo "$AGENTS" | jq '.'

AGENT_API_CALLS=$(echo "$AGENTS" | jq -r '.activities[0].api_calls // 0')
if [ "$AGENT_API_CALLS" -ge 20 ]; then
  echo "✅ Agent activity tracked: $AGENT_API_CALLS API calls"
else
  echo "⚠️  Agent activity: $AGENT_API_CALLS API calls (may be 0 if no authenticated agent calls)"
fi
echo ""

# Step 4: Check database tables directly
echo "4️⃣  Checking database tables..."
echo ""

echo "🗄️  Checking api_calls table..."
API_CALLS_COUNT=$(psql "postgresql://postgres:postgres@localhost:5432/identity?sslmode=disable" \
  -t -c "SELECT COUNT(*) FROM api_calls;")
echo "   Records in api_calls: $API_CALLS_COUNT"

if [ "$API_CALLS_COUNT" -ge 20 ]; then
  echo "✅ API calls table has data"
else
  echo "❌ API calls table is empty or has < 20 records"
fi
echo ""

echo "🗄️  Checking agent_activity_metrics table..."
METRICS_COUNT=$(psql "postgresql://postgres:postgres@localhost:5432/identity?sslmode=disable" \
  -t -c "SELECT COUNT(*) FROM agent_activity_metrics;")
echo "   Records in agent_activity_metrics: $METRICS_COUNT"

if [ "$METRICS_COUNT" -gt 0 ]; then
  echo "✅ Agent activity metrics aggregated"
else
  echo "⚠️  No agent activity metrics (expected if API calls not from agents)"
fi
echo ""

echo "🗄️  Sample api_calls data:"
psql "postgresql://postgres:postgres@localhost:5432/identity?sslmode=disable" \
  -c "SELECT method, endpoint, status_code, duration_ms, request_size_bytes + response_size_bytes as total_bytes, called_at FROM api_calls ORDER BY called_at DESC LIMIT 5;"
echo ""

# Step 5: Summary
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "📋 TEST SUMMARY"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

if [ "$API_CALLS" -ge 20 ] && (( $(echo "$DATA_VOLUME > 0" | bc -l) )) && [ "$API_CALLS_COUNT" -ge 20 ]; then
  echo "✅ ALL ANALYTICS TESTS PASSED!"
  echo ""
  echo "Results:"
  echo "  • API calls tracked: $API_CALLS (endpoint), $API_CALLS_COUNT (database)"
  echo "  • Data volume tracked: ${DATA_VOLUME} MB"
  echo "  • Agent metrics: $METRICS_COUNT hourly aggregates"
  echo ""
  echo "🎉 Analytics implementation working correctly!"
else
  echo "⚠️  SOME TESTS FAILED"
  echo ""
  echo "Results:"
  echo "  • API calls: $API_CALLS (endpoint), $API_CALLS_COUNT (database)"
  echo "  • Data volume: ${DATA_VOLUME} MB"
  echo "  • Agent metrics: $METRICS_COUNT"
  echo ""
  echo "Check logs for details"
fi
echo ""
