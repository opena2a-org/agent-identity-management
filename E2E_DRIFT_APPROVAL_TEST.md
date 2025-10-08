
# E2E Test Script for Drift Approval

## Prerequisites
- Backend running on http://localhost:8080
- Frontend running on http://localhost:3000
- Logged in as admin user

## Test Steps

### 1. Create Test Agent
```bash
# Via UI: Dashboard > Agents > Register Agent
# Name: drift-test-agent
# Type: AI Agent
# MCP Servers (talks_to): filesystem-mcp, github-mcp
```

### 2. Get Agent ID
```bash
# Note the agent ID from the UI or API response
AGENT_ID="<your-agent-id>"
ORG_ID="<your-org-id>"
```

### 3. Create Verification Event with Drift
```python
import requests

response = requests.post(
    "http://localhost:8080/api/v1/verification-events",
    headers={"Authorization": "Bearer <your-token>"},
    json={
        "agent_id": AGENT_ID,
        "organization_id": ORG_ID,
        "protocol": "mcp",
        "verification_type": "identity",
        "status": "success",
        "confidence": 0.95,
        "current_mcp_servers": ["filesystem-mcp", "github-mcp", "external-api-mcp"],  # Drift!
        "current_capabilities": []
    }
)
print(response.json())
```

### 4. Check Alerts Page
- Navigate to http://localhost:3000/dashboard/admin/alerts
- Should see HIGH severity "Configuration Drift Detected" alert
- Alert should show "external-api-mcp" as unauthorized
- Should see "Approve Drift" button (blue, primary button)

### 5. Test Approve Drift
- Click "Approve Drift" button
- Confirm dialog should show: "external-api-mcp"
- Click OK
- Alert should be acknowledged
- Success message displayed

### 6. Verify Agent Updated
- Go to Agents page
- Click on drift-test-agent
- Check "Talks To" section
- Should now include: filesystem-mcp, github-mcp, external-api-mcp

### 7. Verify No More Drift
- Create another verification event with same servers
- Should NOT create new drift alert (servers now registered)

## Expected Results
✅ Drift alert created with external-api-mcp
✅ Approve button visible for configuration_drift alerts
✅ Confirmation dialog shows correct servers
✅ Agent registration updated with new MCP server
✅ Alert acknowledged after approval
✅ No more drift alerts for approved servers
