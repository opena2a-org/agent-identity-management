#!/bin/bash

# Test auto-verification workflow
# This script creates a new agent and verifies it gets auto-verified

set -e

echo "🧪 Testing Auto-Verification Workflow"
echo "======================================"
echo ""

# Get auth token (assuming user is logged in)
TOKEN=$(cat ~/.aim_token 2>/dev/null || echo "")

if [ -z "$TOKEN" ]; then
  echo "❌ No auth token found. Please log in first."
  exit 1
fi

API_URL="http://localhost:8080/api/v1"

echo "1️⃣ Creating new test agent with full metadata..."
AGENT_RESPONSE=$(curl -s -X POST "$API_URL/agents" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "test-agent-auto-verify-'$(date +%s)'",
    "display_name": "Auto-Verify Test Agent",
    "description": "Testing auto-verification workflow with full metadata",
    "agent_type": "ai_agent",
    "version": "1.0.0",
    "repository_url": "https://github.com/test/agent",
    "documentation_url": "https://docs.test.com",
    "capabilities": ["execute_code", "read_files", "write_files"]
  }')

echo "$AGENT_RESPONSE" | jq '.'

# Extract agent ID and status
AGENT_ID=$(echo "$AGENT_RESPONSE" | jq -r '.agent.id')
STATUS=$(echo "$AGENT_RESPONSE" | jq -r '.agent.status')
VERIFIED_AT=$(echo "$AGENT_RESPONSE" | jq -r '.agent.verified_at')
TRUST_SCORE=$(echo "$AGENT_RESPONSE" | jq -r '.agent.trust_score')

echo ""
echo "2️⃣ Checking auto-verification results..."
echo "   Agent ID: $AGENT_ID"
echo "   Status: $STATUS"
echo "   Verified At: $VERIFIED_AT"
echo "   Trust Score: $TRUST_SCORE"
echo ""

# Verify results
if [ "$STATUS" = "verified" ]; then
  echo "✅ SUCCESS: Agent was auto-verified!"
  echo "   Status is 'verified'"
else
  echo "❌ FAILED: Agent was NOT auto-verified"
  echo "   Expected status: 'verified'"
  echo "   Got status: '$STATUS'"
  exit 1
fi

if [ "$VERIFIED_AT" != "null" ] && [ -n "$VERIFIED_AT" ]; then
  echo "✅ SUCCESS: verified_at timestamp is set"
  echo "   Verified At: $VERIFIED_AT"
else
  echo "❌ FAILED: verified_at is null or empty"
  echo "   Expected: timestamp"
  echo "   Got: $VERIFIED_AT"
  exit 1
fi

# Trust score should be >= 0.3 for auto-verification
TRUST_THRESHOLD=0.3
if (( $(echo "$TRUST_SCORE >= $TRUST_THRESHOLD" | bc -l) )); then
  echo "✅ SUCCESS: Trust score meets threshold (>= 0.3)"
  echo "   Trust Score: $TRUST_SCORE"
else
  echo "⚠️  WARNING: Trust score below threshold"
  echo "   Expected: >= $TRUST_THRESHOLD"
  echo "   Got: $TRUST_SCORE"
fi

echo ""
echo "3️⃣ Fetching agent to double-check verification..."
AGENT_CHECK=$(curl -s -X GET "$API_URL/agents/$AGENT_ID" \
  -H "Authorization: Bearer $TOKEN")

echo "$AGENT_CHECK" | jq '.'

STATUS_CHECK=$(echo "$AGENT_CHECK" | jq -r '.status')
VERIFIED_AT_CHECK=$(echo "$AGENT_CHECK" | jq -r '.verified_at')

if [ "$STATUS_CHECK" = "verified" ] && [ "$VERIFIED_AT_CHECK" != "null" ]; then
  echo ""
  echo "🎉 AUTO-VERIFICATION TEST PASSED!"
  echo "   ✅ Agent created with status: verified"
  echo "   ✅ verified_at timestamp populated"
  echo "   ✅ Trust score calculated: $TRUST_SCORE"
  echo ""
  echo "Auto-verification is working correctly! 🚀"
else
  echo ""
  echo "❌ AUTO-VERIFICATION TEST FAILED"
  echo "   Status: $STATUS_CHECK"
  echo "   Verified At: $VERIFIED_AT_CHECK"
  exit 1
fi
