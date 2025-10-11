#!/bin/bash
# Create Python SDK Test Agent via backend API
# This script creates a test agent for Python SDK validation

set -e

AIM_URL="http://localhost:8080"

echo "=========================================="
echo "Creating Python SDK Test Agent"
echo "=========================================="
echo

# Generate Ed25519 keypair using Python
KEYPAIR=$(python3 << 'PYTHON'
from nacl.signing import SigningKey
import base64
import json

signing_key = SigningKey.generate()
private_key_bytes = bytes(signing_key) + bytes(signing_key.verify_key)
public_key_bytes = bytes(signing_key.verify_key)

private_key_b64 = base64.b64encode(private_key_bytes).decode('utf-8')
public_key_b64 = base64.b64encode(public_key_bytes).decode('utf-8')

print(json.dumps({
    "private_key": private_key_b64,
    "public_key": public_key_b64
}))
PYTHON
)

PRIVATE_KEY=$(echo $KEYPAIR | python3 -c "import sys, json; print(json.load(sys.stdin)['private_key'])")
PUBLIC_KEY=$(echo $KEYPAIR | python3 -c "import sys, json; print(json.load(sys.stdin)['public_key'])")

echo "✅ Generated Ed25519 keypair"
echo

# Get auth token from browser session (via cookies or localStorage)
# For now, we'll create the agent using the public registration endpoint
echo "📦 Creating agent via backend API..."

# Create agent
RESPONSE=$(curl -s -X POST "$AIM_URL/api/v1/agents" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $(cat ~/.aim/credentials.json | jq -r '.refresh_token' 2>/dev/null || echo '')" \
  -d "{
    \"name\": \"python-sdk-test-agent\",
    \"display_name\": \"Python SDK Test Agent\",
    \"description\": \"Test agent for Python SDK validation and capability detection\",
    \"agent_type\": \"ai_agent\",
    \"version\": \"1.0.0\",
    \"public_key\": \"$PUBLIC_KEY\"
  }" 2>&1)

echo "$RESPONSE"

# Extract agent ID
AGENT_ID=$(echo "$RESPONSE" | jq -r '.id // .agent_id // empty' 2>/dev/null)

if [ -z "$AGENT_ID" ]; then
  echo "❌ Failed to create agent"
  echo "Response: $RESPONSE"
  exit 1
fi

echo
echo "✅ Python SDK Test Agent created!"
echo "   Agent ID: $AGENT_ID"
echo "   Name: python-sdk-test-agent"
echo

# Generate API key using direct database insert
echo "🔑 Generating API key..."

# Use backend API to create API key
API_KEY_RESPONSE=$(curl -s -X POST "$AIM_URL/api/v1/api-keys" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $(cat ~/.aim/credentials.json | jq -r '.refresh_token' 2>/dev/null || echo '')" \
  -d "{
    \"name\": \"Python SDK Test Key\",
    \"agent_id\": \"$AGENT_ID\"
  }" 2>&1)

API_KEY=$(echo "$API_KEY_RESPONSE" | jq -r '.key // empty' 2>/dev/null)

if [ -z "$API_KEY" ]; then
  echo "⚠️  Could not generate API key via API"
  echo "Response: $API_KEY_RESPONSE"
  echo
  echo "Please generate API key manually via dashboard:"
  echo "  $AIM_URL/dashboard/agents/$AGENT_ID"
else
  echo "✅ API key generated!"
  echo "   Key: $API_KEY"
fi

echo

# Save credentials for testing
cat > python_sdk_test_credentials.json << EOF
{
  "agent_id": "$AGENT_ID",
  "public_key": "$PUBLIC_KEY",
  "private_key": "$PRIVATE_KEY",
  "api_key": "${API_KEY:-GENERATE_VIA_DASHBOARD}",
  "aim_url": "$AIM_URL",
  "name": "python-sdk-test-agent"
}
EOF

chmod 600 python_sdk_test_credentials.json

echo "✅ Credentials saved to: python_sdk_test_credentials.json"
echo
echo "🎯 Next steps:"
echo "   1. View agent: $AIM_URL/dashboard/agents/$AGENT_ID"
if [ -z "$API_KEY" ]; then
  echo "   2. Generate API key via dashboard"
  echo "   3. Update python_sdk_test_credentials.json with API key"
  echo "   4. Run Python SDK test"
else
  echo "   2. Run Python SDK test with these credentials"
fi
echo
