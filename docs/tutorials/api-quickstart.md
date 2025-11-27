# API Quickstart

**Time: 3 minutes** | **Difficulty: Beginner**

Use the AIM REST API directly with curl. Perfect for automation, CI/CD, and non-Python integrations.

## Prerequisites

- AIM running at http://localhost:8080 (backend API)
- curl installed
- An API key (we'll get one in Step 1)

---

## Step 1: Get Your API Key (30 seconds)

1. Login to AIM dashboard at http://localhost:3000
2. Go to **Settings → API Keys**
3. Click **Generate New Key**
4. Copy the key (starts with `aim_`)

```bash
# Save your API key as an environment variable
export AIM_API_KEY="aim_your_key_here"
```

---

## Step 2: Register an Agent (30 seconds)

```bash
curl -X POST "http://localhost:8080/api/v1/agents/register" \
  -H "Authorization: Bearer $AIM_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "api-demo-agent",
    "agent_type": "AI Agent",
    "description": "Agent registered via API",
    "capabilities": ["read_data", "send_notifications"]
  }'
```

**Response:**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "api-demo-agent",
  "status": "pending",
  "trust_score": 0.5,
  "public_key": "ed25519:...",
  "created_at": "2025-01-15T10:30:00Z"
}
```

Save the agent ID:
```bash
export AGENT_ID="550e8400-e29b-41d4-a716-446655440000"
```

---

## Step 3: Log an Action (30 seconds)

```bash
curl -X POST "http://localhost:8080/api/v1/agents/$AGENT_ID/actions" \
  -H "Authorization: Bearer $AIM_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "action_type": "api_call",
    "resource": "weather_api",
    "risk_level": "low",
    "metadata": {
      "endpoint": "/weather",
      "city": "San Francisco"
    }
  }'
```

**Response:**
```json
{
  "id": "action_123",
  "approved": true,
  "trust_score_at_time": 0.5,
  "logged_at": "2025-01-15T10:31:00Z"
}
```

---

## Step 4: Check Trust Score (15 seconds)

```bash
curl -X GET "http://localhost:8080/api/v1/agents/$AGENT_ID/trust-score" \
  -H "Authorization: Bearer $AIM_API_KEY"
```

**Response:**
```json
{
  "agent_id": "550e8400-e29b-41d4-a716-446655440000",
  "trust_score": 0.52,
  "factors": {
    "verification_status": 1.0,
    "action_success_rate": 1.0,
    "security_alerts": 1.0,
    "age_history": 0.5,
    "compliance_score": 1.0,
    "uptime": 1.0,
    "drift_detection": 1.0,
    "user_feedback": 1.0
  },
  "trend": "improving"
}
```

---

## Step 5: Register an MCP Server (30 seconds)

```bash
curl -X POST "http://localhost:8080/api/v1/mcp-servers" \
  -H "Authorization: Bearer $AIM_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "filesystem-server",
    "display_name": "Filesystem MCP Server",
    "server_url": "stdio://filesystem-server",
    "capabilities": ["read_file", "write_file", "list_directory"],
    "description": "Local filesystem access"
  }'
```

Save the MCP server ID:
```bash
export MCP_ID="mcp_server_id_from_response"
```

---

## Step 6: Link Agent to MCP Server (15 seconds)

```bash
curl -X POST "http://localhost:8080/api/v1/agents/$AGENT_ID/mcp-servers/$MCP_ID" \
  -H "Authorization: Bearer $AIM_API_KEY"
```

Now AIM will monitor this connection and alert on drift!

---

## Common API Endpoints

| Action | Method | Endpoint |
|--------|--------|----------|
| Register agent | POST | `/api/v1/agents/register` |
| Get agent | GET | `/api/v1/agents/:id` |
| List agents | GET | `/api/v1/agents` |
| Log action | POST | `/api/v1/agents/:id/actions` |
| Get trust score | GET | `/api/v1/agents/:id/trust-score` |
| Register MCP | POST | `/api/v1/mcp-servers` |
| Link agent to MCP | POST | `/api/v1/agents/:id/mcp-servers/:mcp_id` |
| Get security alerts | GET | `/api/v1/security/alerts` |
| Acknowledge alert | POST | `/api/v1/security/alerts/:id/acknowledge` |
| Get audit logs | GET | `/api/v1/admin/audit-logs` |

---

## Complete Script Example

Save this as `aim-demo.sh`:

```bash
#!/bin/bash
set -e

API_URL="http://localhost:8080"
API_KEY="${AIM_API_KEY:-your_api_key_here}"

echo "=== AIM API Demo ==="

# 1. Register agent
echo "Registering agent..."
AGENT_RESPONSE=$(curl -s -X POST "$API_URL/api/v1/agents/register" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "demo-agent-'$(date +%s)'",
    "agent_type": "AI Agent",
    "description": "Demo agent"
  }')
AGENT_ID=$(echo $AGENT_RESPONSE | jq -r '.id')
echo "Agent ID: $AGENT_ID"

# 2. Log some actions
echo "Logging actions..."
for i in 1 2 3; do
  curl -s -X POST "$API_URL/api/v1/agents/$AGENT_ID/actions" \
    -H "Authorization: Bearer $API_KEY" \
    -H "Content-Type: application/json" \
    -d "{
      \"action_type\": \"api_call\",
      \"resource\": \"test_api\",
      \"risk_level\": \"low\",
      \"metadata\": {\"iteration\": $i}
    }" > /dev/null
  echo "  Action $i logged"
done

# 3. Get trust score
echo "Getting trust score..."
curl -s "$API_URL/api/v1/agents/$AGENT_ID/trust-score" \
  -H "Authorization: Bearer $API_KEY" | jq '.trust_score'

# 4. Get activity
echo "Getting activity..."
curl -s "$API_URL/api/v1/agents/$AGENT_ID/activity" \
  -H "Authorization: Bearer $API_KEY" | jq '.total_actions'

echo "=== Demo Complete ==="
```

Run it:
```bash
chmod +x aim-demo.sh
./aim-demo.sh
```

---

## Error Handling

Common errors and solutions:

| Error | Cause | Solution |
|-------|-------|----------|
| 401 Unauthorized | Invalid API key | Check `$AIM_API_KEY` is set correctly |
| 404 Not Found | Agent/MCP doesn't exist | Verify the ID is correct |
| 400 Bad Request | Invalid JSON | Check your request body format |
| 409 Conflict | Agent name exists | Use a unique agent name |

---

## What's Next?

- **[Dashboard Walkthrough](./dashboard-walkthrough.md)** - Navigate the AIM UI
- **[Full API Reference](../API.md)** - All 160 endpoints documented
- **[Python SDK](./sdk-quickstart.md)** - Easier integration for Python apps

---

<div align="center">

[← SDK Quickstart](./sdk-quickstart.md) | [Back to Tutorials](./README.md) | [Dashboard Walkthrough →](./dashboard-walkthrough.md)

</div>
