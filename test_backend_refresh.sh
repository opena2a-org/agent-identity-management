#!/bin/bash

REFRESH_TOKEN=$(cat /Users/decimai/.aim/credentials.json | jq -r '.refresh_token')

echo "Testing token refresh with backend..."
echo "Token (first 30 chars): ${REFRESH_TOKEN:0:30}..."

curl -X POST \
  https://aim-prod-backend.graypebble-c7e67ab8.canadacentral.azurecontainerapps.io/api/v1/auth/refresh \
  -H "Content-Type: application/json" \
  -d "{\"refresh_token\": \"$REFRESH_TOKEN\"}" \
  -v 2>&1 | grep -A 20 "< HTTP"
