#!/bin/bash
REFRESH_TOKEN=$(cat /Users/decimai/workspace/aim-sdk-python/.aim/credentials.json | jq -r '.refresh_token')

echo "Testing token refresh..."
curl -X POST \
  https://aim-prod-backend.graypebble-c7e67ab8.canadacentral.azurecontainerapps.io/api/v1/auth/refresh \
  -H "Content-Type: application/json" \
  -d "{\"refresh_token\": \"$REFRESH_TOKEN\"}" \
  -s | jq .
