# E2E Detection System Test Guide

**Phase 4 Complete** - SDK Integration & Auto-Detection
**Date**: October 9, 2025
**Status**: Ready for E2E Testing

## Overview

This guide walks through end-to-end testing of the complete MCP detection system:
- Backend API endpoints
- Frontend Detection UI
- Python SDK auto-detection
- Database integration

## Prerequisites

### 1. Backend Server Running
```bash
cd apps/backend
go run cmd/server/main.go
```

**Expected**: Server starts on `http://localhost:8080`

### 2. Frontend Server Running
```bash
cd apps/web
npm run dev
```

**Expected**: Next.js starts on `http://localhost:3000`

### 3. Database Running
PostgreSQL should be running with migrations applied.

## Test Scenarios

### Scenario 1: View Detection Status (No Data)

**Objective**: Verify detection UI displays correctly when agent has no detections

**Steps:**
1. Navigate to dashboard: `http://localhost:3000/dashboard`
2. Click on any agent
3. Click "Detection" tab
4. Verify display shows:
   - ✅ SDK Integration Status card
   - ✅ "Not Installed" badge
   - ✅ Detected MCP Servers card
   - ✅ Empty state message: "No MCP servers detected yet"

**Chrome DevTools MCP Commands:**
```javascript
// Navigate to agent details page
await mcp__chrome-devtools__navigate_page({
  url: "http://localhost:3000/dashboard/agents/<agent-id>"
})

// Take snapshot
await mcp__chrome-devtools__take_snapshot()

// Click Detection tab
await mcp__chrome-devtools__click({ uid: "<detection-tab-uid>" })

// Take screenshot
await mcp__chrome-devtools__take_screenshot({ fullPage: true })
```

**Expected Result:**
- No errors in console
- UI renders cleanly
- Empty state displays properly

---

### Scenario 2: Report Detection via Backend API

**Objective**: Test backend detection reporting endpoint

**Steps:**
1. Get authentication token (JWT)
2. Get agent ID
3. Send POST request to `/api/v1/agents/:id/detection/report`

**cURL Command:**
```bash
# Get auth token first (login)
TOKEN="your-jwt-token"
AGENT_ID="your-agent-id"

# Report detections
curl -X POST http://localhost:8080/api/v1/agents/$AGENT_ID/detection/report \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "detections": [
      {
        "mcpServer": "@modelcontextprotocol/server-filesystem",
        "detectionMethod": "sdk_import",
        "confidence": 95.0,
        "details": {
          "packageName": "@modelcontextprotocol/server-filesystem",
          "version": "0.1.0"
        },
        "sdkVersion": "aim-sdk-python@1.0.0",
        "timestamp": "2025-10-09T12:00:00Z"
      },
      {
        "mcpServer": "@modelcontextprotocol/server-github",
        "detectionMethod": "claude_config",
        "confidence": 100.0,
        "details": {
          "configPath": "~/.claude/claude_desktop_config.json"
        },
        "sdkVersion": "aim-sdk-python@1.0.0",
        "timestamp": "2025-10-09T12:00:00Z"
      }
    ]
  }'
```

**Expected Response:**
```json
{
  "success": true,
  "detectionsProcessed": 2,
  "newMCPs": [
    "@modelcontextprotocol/server-filesystem",
    "@modelcontextprotocol/server-github"
  ],
  "existingMCPs": [],
  "message": "Detections processed successfully"
}
```

---

### Scenario 3: View Detection Status (With Data)

**Objective**: Verify detection UI displays detected MCPs correctly

**Steps:**
1. After reporting detections (Scenario 2)
2. Refresh agent details page
3. Click "Detection" tab
4. Verify display shows:
   - ✅ SDK Integration Status: "Installed" badge
   - ✅ SDK Version displayed
   - ✅ Last reported timestamp
   - ✅ Detected MCP Servers table with 2 rows
   - ✅ Confidence badges with correct colors
   - ✅ Detection method badges
   - ✅ First/Last seen timestamps

**Chrome DevTools MCP Commands:**
```javascript
// Refresh page
await mcp__chrome-devtools__navigate_page({
  url: "http://localhost:3000/dashboard/agents/<agent-id>"
})

// Click Detection tab
await mcp__chrome-devtools__click({ uid: "<detection-tab-uid>" })

// Take screenshot
await mcp__chrome-devtools__take_screenshot({
  fullPage: true,
  filePath: "/tmp/detection-with-data.png"
})

// Verify table exists
await mcp__chrome-devtools__take_snapshot()
// Look for table with MCP server names
```

**Expected Result:**
- SDK status shows "Installed"
- Table shows 2 detected MCPs
- Confidence scores displayed correctly (95% and 100%)
- Detection methods shown (sdk_import, claude_config)

---

### Scenario 4: Python SDK Auto-Detection

**Objective**: Test Python SDK auto-detection capabilities

**Steps:**
1. Create test Python script
2. Run auto-detection
3. Report to backend
4. Verify UI updates

**Test Script:**
```python
# test_detection_e2e.py
from aim_sdk import AIMClient, auto_detect_mcps
import os

# Setup (get these from dashboard)
AGENT_ID = os.getenv("AGENT_ID")
PUBLIC_KEY = os.getenv("PUBLIC_KEY")
PRIVATE_KEY = os.getenv("PRIVATE_KEY")
AIM_URL = "http://localhost:8080"

# Create client
client = AIMClient(
    agent_id=AGENT_ID,
    public_key=PUBLIC_KEY,
    private_key=PRIVATE_KEY,
    aim_url=AIM_URL
)

# Auto-detect MCPs
print("🔍 Running auto-detection...")
detections = auto_detect_mcps()
print(f"Found {len(detections)} MCP servers")

for detection in detections:
    print(f"  - {detection['mcpServer']} ({detection['detectionMethod']}, {detection['confidence']}%)")

# Report to AIM
if detections:
    print("\n📤 Reporting to AIM...")
    result = client.report_detections(detections)
    print(f"✅ Success: {result['success']}")
    print(f"   Processed: {result['detectionsProcessed']}")
    print(f"   New MCPs: {result['newMCPs']}")
    print(f"   Existing MCPs: {result['existingMCPs']}")
else:
    print("\n⚠️  No detections found (this is normal if no MCPs configured)")

print("\n✅ E2E test complete! Check the dashboard to verify UI updated.")
```

**Run Test:**
```bash
cd sdks/python
python test_detection_e2e.py
```

**Expected Output:**
```
🔍 Running auto-detection...
Found 2 MCP servers
  - @modelcontextprotocol/server-filesystem (claude_config, 100.0%)
  - mcp (sdk_import, 90.0%)

📤 Reporting to AIM...
✅ Success: True
   Processed: 2
   New MCPs: []
   Existing MCPs: ['@modelcontextprotocol/server-filesystem', 'mcp']

✅ E2E test complete! Check the dashboard to verify UI updated.
```

---

### Scenario 5: Multiple Detection Methods (Confidence Boosting)

**Objective**: Verify confidence score increases when same MCP detected by multiple methods

**Steps:**
1. Report detection from `claude_config` (100% confidence)
2. Report detection from `sdk_import` (90% confidence)
3. Verify confidence increased by 10% (capped at 99%)

**Test Data:**
```bash
# First detection
curl -X POST http://localhost:8080/api/v1/agents/$AGENT_ID/detection/report \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "detections": [{
      "mcpServer": "test-mcp-server",
      "detectionMethod": "claude_config",
      "confidence": 100.0
    }]
  }'

# Second detection (different method, same MCP)
curl -X POST http://localhost:8080/api/v1/agents/$AGENT_ID/detection/report \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "detections": [{
      "mcpServer": "test-mcp-server",
      "detectionMethod": "sdk_import",
      "confidence": 90.0
    }]
  }'

# Get status
curl -X GET http://localhost:8080/api/v1/agents/$AGENT_ID/detection/status \
  -H "Authorization: Bearer $TOKEN"
```

**Expected Result:**
- First detection: confidence = 100%
- After second detection: confidence capped at 99% (multiple methods boost +10%)
- Detection UI shows both methods: `claude_config, sdk_import`

---

### Scenario 6: Check Network Requests

**Objective**: Verify frontend makes correct API calls

**Chrome DevTools MCP Commands:**
```javascript
// Navigate to page
await mcp__chrome-devtools__navigate_page({
  url: "http://localhost:3000/dashboard/agents/<agent-id>"
})

// Click Detection tab
await mcp__chrome-devtools__click({ uid: "<detection-tab-uid>" })

// Check network requests
const requests = await mcp__chrome-devtools__list_network_requests({
  resourceTypes: ["fetch", "xhr"]
})

// Should see request to: GET /api/v1/agents/:id/detection/status
```

**Expected Requests:**
1. `GET /api/v1/agents/:id/detection/status`
2. Response status: `200 OK`
3. Response body includes: `sdkInstalled`, `detectedMCPs`, etc.

---

## Success Criteria

### Backend API
- ✅ POST `/api/v1/agents/:id/detection/report` returns 200
- ✅ Detections stored in `agent_mcp_detections` table
- ✅ Agent's `talks_to` array updated
- ✅ SDK heartbeat timestamp updated
- ✅ GET `/api/v1/agents/:id/detection/status` returns correct data
- ✅ Confidence boosting works (multiple methods)

### Frontend UI
- ✅ Detection tab renders without errors
- ✅ Empty state displays correctly
- ✅ SDK status card shows installation info
- ✅ Detected MCPs table displays data
- ✅ Confidence badges color-coded correctly
- ✅ Detection method badges display icons
- ✅ Timestamps formatted as relative time ("2 hours ago")

### Python SDK
- ✅ `auto_detect_mcps()` finds MCPs
- ✅ `client.report_detections()` sends to backend
- ✅ Claude config parsing works
- ✅ Import scanning works
- ✅ No exceptions thrown

### Integration
- ✅ SDK → Backend → Database → Frontend flow works
- ✅ Real-time updates reflected in UI
- ✅ No console errors
- ✅ No network errors

## Known Issues / Notes

1. **Authentication Required**: All API endpoints require valid JWT token
2. **Agent Must Exist**: Agent must be registered before reporting detections
3. **Claude Config**: Auto-detection only works if `~/.claude/claude_desktop_config.json` exists
4. **Import Scanning**: Only detects MCP packages currently installed/imported

## Troubleshooting

### Issue: 401 Unauthorized
**Solution**: Get fresh JWT token from login endpoint

### Issue: 404 Agent Not Found
**Solution**: Verify agent exists and agent ID is correct

### Issue: Empty Detections
**Solution**: This is normal if no Claude Desktop config and no MCP packages installed

### Issue: UI Not Updating
**Solution**: Hard refresh browser (Cmd+Shift+R) to clear cache

## Next Steps After Testing

1. ✅ Document any bugs found
2. ✅ Fix critical issues
3. ✅ Create video demo of working system
4. ✅ Write comprehensive user documentation
5. ✅ Consider Node.js/TypeScript SDK
6. ✅ Plan Phase 5: MCP Capability Auto-Detection

---

**Test Execution Date**: _____________
**Tester**: _____________
**Results**: ☐ Pass  ☐ Fail  ☐ Pass with issues

**Issues Found:**
_____________________________________________
_____________________________________________
_____________________________________________
