#!/bin/bash

# AIM Endpoint Testing Script
# Tests all reported issues systematically

TOKEN="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoiYTAwMDAwMDAtMDAwMC0wMDAwLTAwMDAtMDAwMDAwMDAwMDAyIiwib3JnYW5pemF0aW9uX2lkIjoiYTAwMDAwMDAtMDAwMC0wMDAwLTAwMDAtMDAwMDAwMDAwMDAxIiwiZW1haWwiOiJhZG1pbkBvcGVuYTJhLm9yZyIsInJvbGUiOiJhZG1pbiIsImlzcyI6ImFnZW50LWlkZW50aXR5LW1hbmFnZW1lbnQiLCJzdWIiOiJhMDAwMDAwMC0wMDAwLTAwMDAtMDAwMC0wMDAwMDAwMDAwMDIiLCJleHAiOjE3NjEyMzUyOTcsIm5iZiI6MTc2MTE0ODg5NywiaWF0IjoxNzYxMTQ4ODk3LCJqdGkiOiJmYTBmZjUxYS0xNjQ0LTRmZjQtOTE3Yi01OGNlZDAxMTc0M2EifQ.HE47a5wq-xl4tAKqk6uMTU91v_VxUit0txEf_uxZ_o4"
BASE_URL="http://localhost:8080/api/v1"

echo "=== AIM Endpoint Testing ==="
echo ""

# Create test agent
echo "Creating test agent..."
AGENT_RESPONSE=$(curl -s -X POST "$BASE_URL/agents" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Test Agent","type":"ai_agent","description":"Test agent","identifier":"test-agent-001"}')
AGENT_ID=$(echo $AGENT_RESPONSE | jq -r '.id')
echo "Created agent: $AGENT_ID"
echo ""

# Test Issue 1: Agent Tags
echo "=== Testing Issue 1: Agent Tags ==="
echo "GET /api/v1/agents/$AGENT_ID/tags"
curl -s -X GET "$BASE_URL/agents/$AGENT_ID/tags" \
  -H "Authorization: Bearer $TOKEN" | jq .
echo ""

# Test Issue 2: Create Tag on Agent
echo "POST /api/v1/agents/$AGENT_ID/tags"
curl -s -X POST "$BASE_URL/agents/$AGENT_ID/tags" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"key":"environment","value":"production","category":"deployment"}' | jq .
echo ""

# Test Issue 3: API Key Creation
echo "=== Testing Issue 3: API Key Creation ==="
curl -s -X POST "$BASE_URL/api-keys" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Test API Key","expiresAt":"2025-12-31T23:59:59Z"}' | jq .
echo ""

# Test Issue 4: Capability Reports
echo "=== Testing Issue 4: Capability Reports ==="
curl -s -X POST "$BASE_URL/detection/agents/$AGENT_ID/capabilities/report" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "detectedAt": "2025-10-22T10:00:00Z",
    "environment": {"os": "linux"},
    "aiModels": [{"name": "claude-3-5-sonnet"}],
    "capabilities": [{"name": "file_read"}],
    "riskAssessment": {
      "riskLevel": "low",
      "overallRiskScore": 20,
      "trustScoreImpact": 5
    }
  }' | jq .
echo ""

# Test Issue 5: Organization Settings
echo "=== Testing Issue 5: Organization Settings ==="
curl -s -X GET "$BASE_URL/admin/organization/settings" \
  -H "Authorization: Bearer $TOKEN" | jq .
echo ""

# Test Issue 6: Capability Requests
echo "=== Testing Issue 6: Capability Requests ==="
curl -s -X GET "$BASE_URL/admin/capability-requests" \
  -H "Authorization: Bearer $TOKEN" | jq .
echo ""

# Test Issue 7: MCP Server Creation
echo "=== Testing Issue 7: MCP Server Creation ==="
curl -s -X POST "$BASE_URL/mcp-servers" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Test MCP Server","url":"https://example.com/mcp","description":"Test server"}' | jq .
echo ""

# Test Issue 8: Security Anomalies
echo "=== Testing Issue 8: Security Anomalies ==="
curl -s -X GET "$BASE_URL/security/anomalies" \
  -H "Authorization: Bearer $TOKEN" | jq .
echo ""

# Test Issue 9: Webhooks
echo "=== Testing Issue 9: Webhooks ==="
curl -s -X POST "$BASE_URL/webhooks" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Test Webhook","url":"https://example.com/webhook","events":["agent.created"]}' | jq .
echo ""

# Test Issue 10: Tags
echo "=== Testing Issue 10: Tags ==="
curl -s -X GET "$BASE_URL/tags" \
  -H "Authorization: Bearer $TOKEN" | jq .
echo ""

echo "=== Testing Complete ==="
