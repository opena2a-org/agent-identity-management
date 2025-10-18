#!/bin/bash
# Test token refresh endpoint with enterprise security

REFRESH_TOKEN=$(cat /Users/decimai/.aim/credentials.json | jq -r '.refresh_token')

echo "Testing token refresh endpoint..."
echo "Refresh token: ${REFRESH_TOKEN:0:50}..."
echo ""

curl -X POST http://localhost:8080/api/v1/auth/refresh \
  -H "Content-Type: application/json" \
  -d "{\"refresh_token\": \"$REFRESH_TOKEN\"}" \
  2>/dev/null | jq .
