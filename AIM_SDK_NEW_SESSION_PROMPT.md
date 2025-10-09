# Prompt for New Claude Code Session - AIM SDK Implementation

Copy and paste this entire prompt into a new Claude Code session:

---

## Context: AIM Platform (Existing Infrastructure)

You're working on **AIM (Agent Identity Management)**, an enterprise open-source platform for managing AI agent identities and MCP (Model Context Protocol) server usage.

**Repository**: `/Users/decimai/workspace/agent-identity-management/`

### Tech Stack
- **Backend**: Go 1.23+ (Fiber v3 framework)
- **Frontend**: Next.js 15 (App Router), TypeScript, Shadcn/ui, Tailwind CSS
- **Database**: PostgreSQL 16
- **Cache**: Redis 7

---

## What Already Exists (DO NOT REBUILD)

### ✅ Backend (`apps/backend/`)
- **Agents API**: Full CRUD operations, authentication, trust scoring
- **MCP Servers API**: Registration, management
- **Agent-MCP Relationships**: `talks_to` field tracking which MCPs each agent uses
- **Existing Endpoint**: `POST /api/v1/agents/:id/mcp-servers/detect` (uses Claude Desktop config only)
- **Database**: Complete schema with migrations
- **Clean Architecture**: Domain → Application → Infrastructure → Interfaces

**Key Existing Files**:
```
apps/backend/
├── cmd/server/main.go                        # Entry point, routes
├── internal/
│   ├── domain/
│   │   ├── agent.go                          # Agent entity
│   │   └── mcp_server.go                     # MCP server entity
│   ├── application/
│   │   ├── agent_service.go                  # Agent business logic
│   │   └── mcp_service.go                    # MCP business logic
│   ├── infrastructure/repository/
│   │   ├── agent_repository.go               # Agent DB operations
│   │   └── mcp_repository.go                 # MCP DB operations
│   └── interfaces/http/handlers/
│       ├── agent_handler.go                  # Agent HTTP endpoints
│       └── mcp_handler.go                    # MCP HTTP endpoints
└── migrations/                               # Database migrations
```

### ✅ Frontend (`apps/web/`)
- **Agent Management UI**: Registration, list view, details page
- **MCP Server Management UI**: Registration, assignment to agents
- **Agent Details Page**: Shows MCP connections with graph visualization
- **Auto-Detect Button**: Currently uses Claude Desktop config parsing
- **Components**: AutoDetectButton, MCPServerSelector, MCPServerList, AgentMCPGraph

**Key Existing Files**:
```
apps/web/
├── app/dashboard/
│   ├── agents/[id]/page.tsx                  # Agent details page
│   ├── agents/page.tsx                       # Agent list
│   └── mcp/page.tsx                          # MCP server list
├── components/agents/
│   ├── auto-detect-button.tsx                # Auto-detect modal
│   ├── mcp-server-selector.tsx               # Manual MCP selection
│   ├── mcp-server-list.tsx                   # MCP connection list
│   └── agent-mcp-graph.tsx                   # Relationship graph
└── lib/api.ts                                # API client functions
```

---

## What We're Building (SDK-Based Auto-Detection)

### Goal
Build **AIM SDKs** (JavaScript, Python, Go) that agents integrate to **auto-detect MCP usage** and report to the existing AIM platform.

### How It Works
```
Agent Code (anywhere)
    ↓
Integrates @aim/sdk (npm install @aim/sdk)
    ↓
SDK auto-detects MCPs via import hooks
    ↓
Reports to: POST /api/v1/agents/:id/mcp-detected (NEW endpoint)
    ↓
AIM Backend receives and stores detection data
    ↓
Dashboard UI shows SDK-detected MCPs with badges
```

### What We're NOT Doing
❌ Building separate scanning tools or CLI utilities
❌ Scanning user filesystems from AIM backend
❌ Runtime monitoring (moved to roadmap)
✅ Building SDKs that embed in agent code
✅ Enhancing existing AIM API to receive SDK reports
✅ Updating UI to show SDK-detected MCPs

---

## Implementation Tasks

### Read This First
```bash
cat /Users/decimai/workspace/agent-identity-management/AIM_SDK_IMPLEMENTATION_PLAN.md
cat /Users/decimai/workspace/agent-identity-management/CLAUDE.md  # Naming conventions
```

### Phase 1: Backend API Enhancement ⭐ START HERE

#### Task 1.1: Database Migration
**File**: `apps/backend/migrations/030_add_sdk_detection_support.up.sql` (new)

**Requirements**:
- Add `detection_method` column to `agent_mcp_servers` table
  - Values: 'manual', 'sdk_import', 'sdk_connection', 'config'
  - Default: 'manual'
- Add `confidence_score` column (DECIMAL 5,2, default 100.0)
- Add `detected_at` timestamp (nullable)
- Add `last_seen_at` timestamp (nullable)
- Create index on `detection_method`
- Create `sdk_detection_events` audit table with:
  - id, agent_id, sdk_version, detection_method, mcp_servers_detected (JSONB), agent_metadata (JSONB), created_at

**Validation**:
```bash
# Run migration
go run cmd/migrate/main.go up

# Verify tables
psql -U postgres -d aim -c "\d agent_mcp_servers"
psql -U postgres -d aim -c "\d sdk_detection_events"
```

**Down Migration**: `apps/backend/migrations/030_add_sdk_detection_support.down.sql`
- Drop columns and table in reverse order

---

#### Task 1.2: Update Domain Models
**File**: `apps/backend/internal/domain/agent_mcp_relationship.go` (enhance existing or create if missing)

**Requirements**:
- Add new fields to AgentMCPRelationship struct:
  ```go
  DetectionMethod  string     // "manual", "sdk_import", "sdk_connection", "config"
  ConfidenceScore  float64    // 0-100
  DetectedAt       *time.Time
  LastSeenAt       *time.Time
  ```

**File**: `apps/backend/internal/domain/sdk_detection.go` (new)

**Requirements**:
- Create SDKDetectionEvent struct matching database table
- Create SDKDetectedMCP struct for individual detections
- Create SDKDetectionRequest struct for API input

**Validation**:
```bash
# Ensure code compiles
cd apps/backend && go build ./...
```

---

#### Task 1.3: Create SDK Service
**File**: `apps/backend/internal/application/sdk_service.go` (new)

**Requirements**:
- Create `SDKService` struct with dependencies:
  - agentRepo
  - mcpRepo
  - db connection
- Implement `ProcessSDKDetectionReport()` method:
  1. Validate agent exists
  2. Validate API key matches agent
  3. For each detected MCP:
     - Check if MCP server exists (if not, warn but don't fail)
     - Update agent.talks_to array
     - Update detection metadata (method, confidence, timestamps)
  4. Store audit event in `sdk_detection_events` table
  5. Return success response

**Error Handling**:
- Invalid agent ID → 404
- Invalid API key → 401
- MCP server not found → Log warning, continue (don't block detection)
- Database error → 500

**Validation**:
```bash
# Write unit tests
cd apps/backend && go test ./internal/application/sdk_service_test.go
```

---

#### Task 1.4: Create API Handler
**File**: `apps/backend/internal/interfaces/http/handlers/agent_handler.go` (update existing)

**Requirements**:
- Add new method: `HandleSDKDetectionReport(c *fiber.Ctx) error`
- Route: `POST /api/v1/agents/:id/mcp-detected`
- Request body validation:
  ```json
  {
    "sdkVersion": "1.0.0",
    "detectedMCPs": [
      {
        "name": "filesystem",
        "detectionMethod": "sdk_import",
        "confidenceScore": 95.0,
        "details": {"source": "import_hook"}
      }
    ],
    "agentMetadata": {"runtime": "node", "nodeVersion": "v20.0.0"}
  }
  ```
- Response:
  ```json
  {
    "success": true,
    "mcpsDetected": 1
  }
  ```

**Authentication**:
- Require `Authorization: Bearer <api_key>` header
- Validate API key matches agent

**Validation**:
- Test with curl:
  ```bash
  curl -X POST http://localhost:8080/api/v1/agents/{agent_id}/mcp-detected \
    -H "Authorization: Bearer test-api-key" \
    -H "Content-Type: application/json" \
    -d '{
      "sdkVersion": "1.0.0",
      "detectedMCPs": [{"name": "filesystem", "detectionMethod": "sdk_import", "confidenceScore": 95.0}]
    }'
  ```

---

#### Task 1.5: Register Route
**File**: `apps/backend/cmd/server/main.go` (update existing)

**Requirements**:
- Add route to existing agent routes group:
  ```go
  agents.Post("/:id/mcp-detected", handlers.HandleSDKDetectionReport)
  ```

**Validation**:
```bash
# Start server
cd apps/backend && go run cmd/server/main.go

# Verify route exists
curl http://localhost:8080/api/v1/agents/test-id/mcp-detected
# Should return 401 (auth required) not 404
```

---

### Phase 2: JavaScript/TypeScript SDK

#### Task 2.1: Create SDK Package Structure
**Directory**: `packages/aim-sdk-js/` (new)

**Files to create**:
```
packages/aim-sdk-js/
├── package.json
├── tsconfig.json
├── .gitignore
├── README.md
├── src/
│   ├── index.ts              # Exports
│   ├── client.ts             # AIMClient class
│   ├── detectors/
│   │   ├── import-detector.ts
│   │   └── connection-detector.ts
│   ├── reporters/
│   │   └── api-reporter.ts
│   └── types.ts
└── tests/
    └── client.test.ts
```

**package.json Requirements**:
```json
{
  "name": "@aim/sdk",
  "version": "1.0.0",
  "description": "AIM SDK for automatic MCP detection",
  "main": "dist/index.js",
  "types": "dist/index.d.ts",
  "scripts": {
    "build": "tsc",
    "test": "jest"
  },
  "keywords": ["aim", "mcp", "agent"],
  "license": "MIT"
}
```

**Validation**:
```bash
cd packages/aim-sdk-js
npm install
npm run build
# Should create dist/ folder with compiled JS
```

---

#### Task 2.2: Implement AIMClient
**File**: `packages/aim-sdk-js/src/client.ts`

**Requirements**:
- Constructor accepts config: `{ apiUrl, apiKey, agentId, autoDetect, detectionMethods }`
- Initialize detectors if `autoDetect: true`
- Start periodic reporting (every 10 seconds, debounced)
- Provide manual `detect()` method

**See**: Implementation plan Phase 2.2 for full code example

**Validation**:
```bash
# Write tests
npm test
```

---

#### Task 2.3: Implement Import Detector
**File**: `packages/aim-sdk-js/src/detectors/import-detector.ts`

**Requirements**:
- Hook into Node.js `require()` using `Module.prototype.require`
- Detect `@modelcontextprotocol/*` packages
- Store detected MCP names in Set (avoid duplicates)
- Return detections with 95% confidence

**Edge Cases**:
- Handle ESM imports (harder to hook)
- Don't crash if MCP package not found
- Thread-safe (multiple requires simultaneously)

**Validation**:
- Create test agent: `test/fixtures/js-agent/`
- Install `@modelcontextprotocol/sdk`
- Verify SDK detects it

---

#### Task 2.4: Implement API Reporter
**File**: `packages/aim-sdk-js/src/reporters/api-reporter.ts`

**Requirements**:
- POST to `/api/v1/agents/:id/mcp-detected`
- Include `Authorization: Bearer <apiKey>` header
- Fail silently on network errors (don't break agent)
- Retry logic (max 3 retries with exponential backoff)

**Validation**:
```bash
# Test against local AIM backend
node test-reporter.js
# Should see detection in AIM dashboard
```

---

### Phase 3: Python SDK

#### Task 3.1: Create SDK Package Structure
**Directory**: `packages/aim-sdk-py/` (new)

**Files to create**:
```
packages/aim-sdk-py/
├── setup.py
├── pyproject.toml
├── README.md
├── aim_sdk/
│   ├── __init__.py
│   ├── client.py
│   ├── detectors/
│   │   ├── __init__.py
│   │   └── import_detector.py
│   └── reporters/
│       ├── __init__.py
│       └── api_reporter.py
└── tests/
    └── test_client.py
```

**setup.py Requirements**:
```python
from setuptools import setup, find_packages

setup(
    name="aim-sdk",
    version="1.0.0",
    packages=find_packages(),
    install_requires=["requests>=2.28.0"],
)
```

**Validation**:
```bash
cd packages/aim-sdk-py
pip install -e .
python -c "from aim_sdk import AIMClient; print('OK')"
```

---

#### Task 3.2: Implement AIMClient
**File**: `packages/aim-sdk-py/aim_sdk/client.py`

**Requirements**:
- Same API as JavaScript SDK
- Use `sys.meta_path` for import hooks
- Threading for periodic reporting

**See**: Implementation plan Phase 3.2 for full code

**Validation**:
```bash
pytest tests/
```

---

#### Task 3.3: Implement Import Hook Detector
**File**: `packages/aim-sdk-py/aim_sdk/detectors/import_detector.py`

**Requirements**:
- Implement `MetaPathFinder` interface
- Insert at beginning of `sys.meta_path`
- Detect `mcp` or `mcp-*` packages
- Don't interfere with normal imports

**Validation**:
- Create test agent: `test/fixtures/py-agent/`
- Install `mcp` package
- Verify SDK detects it

---

### Phase 4: Go SDK

#### Task 4.1: Create SDK Package Structure
**Directory**: `packages/aim-sdk-go/` (new)

**Files to create**:
```
packages/aim-sdk-go/
├── go.mod
├── go.sum
├── client.go
├── reporters/
│   └── api_reporter.go
├── examples/
│   └── main.go
└── README.md
```

**go.mod Requirements**:
```
module github.com/opena2a/aim-sdk-go

go 1.23

require (
    // dependencies
)
```

**Note**: Go doesn't support runtime import hooks easily. SDK provides manual reporting API.

**Validation**:
```bash
cd packages/aim-sdk-go
go build ./...
go test ./...
```

---

### Phase 5: Frontend UI Updates

#### Task 5.1: Create Detection Method Badge Component
**File**: `apps/web/components/agents/detection-method-badge.tsx` (new)

**Requirements**:
- Props: `{ method: 'sdk_import' | 'sdk_connection' | 'config' | 'manual', confidenceScore?: number }`
- Show icon + label + confidence score
- Color coding:
  - sdk_import: Blue (Code icon)
  - sdk_connection: Green (Plug icon)
  - config: Gray (FileCode icon)
  - manual: Purple (User icon)

**Icons**: Use `lucide-react` (already in project)

**Validation**: Use Chrome DevTools MCP
```typescript
// Navigate to agent details page
mcp__chrome-devtools__navigate_page({ url: "http://localhost:3000/dashboard/agents/test-id" })

// Take snapshot
mcp__chrome-devtools__take_snapshot()

// Verify badge renders
mcp__chrome-devtools__take_screenshot()
```

---

#### Task 5.2: Update MCP Server List Component
**File**: `apps/web/components/agents/mcp-server-list.tsx` (update existing)

**Requirements**:
- Update TypeScript interface to include:
  ```typescript
  interface MCPConnection {
    id: string;
    name: string;
    detectionMethod?: 'sdk_import' | 'sdk_connection' | 'config' | 'manual';
    confidenceScore?: number;
    detectedAt?: string;
    lastSeenAt?: string;
  }
  ```
- Add `<DetectionMethodBadge>` next to MCP name
- Show "Last Seen" timestamp if available

**Validation**: Chrome DevTools MCP
```typescript
// Fill test data and verify rendering
mcp__chrome-devtools__list_network_requests({ resourceTypes: ["fetch"] })
// Should see API call to get agent MCPs

mcp__chrome-devtools__list_console_messages()
// Should be no errors
```

---

#### Task 5.3: Create SDK Setup Guide Component
**File**: `apps/web/components/agents/sdk-setup-guide.tsx` (new)

**Requirements**:
- Tabbed interface (JavaScript, Python, Go)
- Show code snippet for each SDK
- Include agent's actual API key and ID (from props)
- Copy-to-clipboard button
- Link to full documentation

**Validation**: Chrome DevTools MCP
```typescript
// Click through tabs
mcp__chrome-devtools__click({ uid: "python-tab-uid" })
mcp__chrome-devtools__take_screenshot()

// Test copy button
mcp__chrome-devtools__click({ uid: "copy-button-uid" })
// Should copy to clipboard
```

---

#### Task 5.4: Update Agent Details Page
**File**: `apps/web/app/dashboard/agents/[id]/page.tsx` (update existing)

**Requirements**:
- Add new tab: "SDK Setup"
- Show `<SDKSetupGuide>` component
- Update existing "Connections" tab to show detection methods
- Update API types to match backend response

**Validation**: Chrome DevTools MCP
```typescript
// Navigate to page
mcp__chrome-devtools__navigate_page({ url: "http://localhost:3000/dashboard/agents/test-id" })

// Test all tabs
mcp__chrome-devtools__click({ uid: "sdk-setup-tab-uid" })
mcp__chrome-devtools__take_screenshot()

// Check for console errors
mcp__chrome-devtools__list_console_messages()
```

---

#### Task 5.5: Update API Client
**File**: `apps/web/lib/api.ts` (update existing)

**Requirements**:
- Update `getAgent()` response type to include detection metadata
- Update `getMCPServers()` response type
- Ensure camelCase consistency (backend JSON → frontend)

**CRITICAL**: Check naming consistency (see CLAUDE.md)
- Backend JSON uses camelCase
- Frontend TypeScript uses camelCase
- MUST MATCH EXACTLY

**Validation**:
```bash
# Start frontend dev server
cd apps/web && npm run dev

# Open in browser, check Network tab
# Verify API responses match TypeScript interfaces
```

---

### Phase 6: Integration Testing

#### Task 6.1: Create Test Agents
**Directory**: `test/fixtures/` (new)

Create 3 test agents:

**JavaScript Agent** (`test/fixtures/js-agent/`):
```bash
mkdir -p test/fixtures/js-agent
cd test/fixtures/js-agent
npm init -y
npm install @modelcontextprotocol/sdk
npm install @aim/sdk
```

Create `test/fixtures/js-agent/index.js`:
```javascript
const { AIMClient } = require('@aim/sdk');
const { Client } = require('@modelcontextprotocol/sdk');

const aim = new AIMClient({
  apiUrl: 'http://localhost:8080',
  apiKey: 'test-api-key',
  agentId: 'test-agent-id',
  autoDetect: true
});

console.log('Agent started. SDK should detect MCP imports.');
```

**Python Agent** (`test/fixtures/py-agent/`):
```bash
mkdir -p test/fixtures/py-agent
cd test/fixtures/py-agent
python -m venv venv
source venv/bin/activate
pip install mcp aim-sdk
```

Create `test/fixtures/py-agent/main.py`:
```python
from aim_sdk import AIMClient
import mcp

aim = AIMClient(
    api_url='http://localhost:8080',
    api_key='test-api-key',
    agent_id='test-agent-id',
    auto_detect=True
)

print("Agent started. SDK should detect MCP imports.")
```

**Validation**:
```bash
# Run each test agent
node test/fixtures/js-agent/index.js
python test/fixtures/py-agent/main.py

# Check AIM backend logs for detection reports
# Check database: SELECT * FROM sdk_detection_events;
# Check dashboard UI for detected MCPs
```

---

#### Task 6.2: End-to-End Test
**Scenario**: User registers agent → Installs SDK → SDK auto-detects → Dashboard shows connections

**Steps**:
1. Register new agent via UI
2. Copy API key from dashboard
3. Install SDK in test agent
4. Configure SDK with API key and agent ID
5. Run test agent
6. Verify SDK sends detection report (check backend logs)
7. Refresh dashboard
8. Verify MCPs appear in "Connections" tab with SDK badge

**Validation**: Chrome DevTools MCP
```typescript
// 1. Navigate to agent registration
mcp__chrome-devtools__navigate_page({ url: "http://localhost:3000/dashboard/agents/new" })

// 2. Fill form
mcp__chrome-devtools__fill_form({
  elements: [
    { uid: "name-input", value: "Test SDK Agent" },
    { uid: "type-select", value: "ai_agent" }
  ]
})

// 3. Submit
mcp__chrome-devtools__click({ uid: "submit-button" })

// 4. Navigate to agent details
// (Get agent ID from response)

// 5. Verify SDK setup guide shows
mcp__chrome-devtools__take_snapshot()
// Should see SDK Setup tab

// 6. Run test agent (manual step)

// 7. Refresh page
mcp__chrome-devtools__navigate_page({ url: "http://localhost:3000/dashboard/agents/{agent_id}" })

// 8. Verify MCPs detected
mcp__chrome-devtools__take_screenshot()
// Should see filesystem with "SDK Import" badge
```

---

### Phase 7: Documentation

#### Task 7.1: SDK Quick Start Guide
**File**: `docs/sdk/quick-start.md` (new)

**Requirements**:
- Installation instructions for all 3 SDKs
- Code examples
- Configuration options
- Troubleshooting section

---

#### Task 7.2: API Documentation
**File**: `docs/api/sdk-detection-endpoint.md` (new)

**Requirements**:
- Endpoint specification
- Request/response examples
- Error codes
- Authentication requirements

---

#### Task 7.3: Update Main README
**File**: `README.md` (update existing)

**Requirements**:
- Add "SDK-Based Detection" section
- Link to SDK documentation
- Update feature list

---

## Validation Checklist

Before marking complete, verify:

### Backend
- [ ] Database migration runs successfully
- [ ] New endpoint responds correctly (test with curl)
- [ ] API key authentication works
- [ ] Invalid requests return proper error codes
- [ ] Audit events stored in `sdk_detection_events` table
- [ ] Unit tests passing (`go test ./...`)

### SDKs
- [ ] JavaScript SDK builds without errors
- [ ] Python SDK installs via pip
- [ ] Go SDK compiles
- [ ] Import detection works for each SDK
- [ ] API reporting succeeds (check backend logs)
- [ ] SDKs fail gracefully on network errors

### Frontend
- [ ] Detection method badges render correctly
- [ ] Agent details page shows SDK-detected MCPs
- [ ] SDK setup guide displays code snippets
- [ ] No TypeScript errors (`npm run build`)
- [ ] **Chrome DevTools MCP testing shows no console errors**
- [ ] API types match backend responses (naming consistency)

### Integration
- [ ] End-to-end test passes (agent → SDK → API → UI)
- [ ] Dashboard updates after SDK reports
- [ ] Multiple detections merge correctly (no duplicates)
- [ ] Concurrent SDK reports don't cause race conditions

### Documentation
- [ ] Quick start guide complete
- [ ] API documentation accurate
- [ ] README updated
- [ ] Code examples tested

---

## Common Pitfalls to Avoid

### Naming Consistency (CRITICAL!)
**Problem**: Using different names for the same concept across backend/frontend/database

**Solution**: Always check CLAUDE.md for naming conventions
- Database: `snake_case`
- Backend JSON: `camelCase`
- Frontend: `camelCase`
- MUST MATCH EXACTLY

**Example**:
```go
// Backend (Go struct field)
DetectionMethod string

// Backend (JSON tag) - MUST match frontend
`json:"detectionMethod"`

// Frontend (TypeScript)
detectionMethod: string
```

### Icon Library Consistency
**Rule**: This project uses **lucide-react** exclusively
```typescript
// CORRECT ✅
import { Code, Plug, FileCode, User } from 'lucide-react';

// WRONG ❌
import { CodeIcon } from '@heroicons/react/24/outline';
```

### Chrome DevTools MCP Testing
**Mandatory**: Test ALL frontend changes with chrome-devtools MCP before marking complete

**Why**: Catches console errors, API mismatches, rendering issues

**How**:
```typescript
// 1. Navigate
mcp__chrome-devtools__navigate_page({ url: "..." })

// 2. Take snapshot (see element UIDs)
mcp__chrome-devtools__take_snapshot()

// 3. Interact
mcp__chrome-devtools__click({ uid: "..." })

// 4. Verify
mcp__chrome-devtools__list_console_messages()  // No errors
mcp__chrome-devtools__list_network_requests()  // API called successfully
```

### SDK Performance
**Rule**: SDKs must not impact agent performance
- Detection runs async
- Reporting is debounced (10s)
- Network errors fail silently
- No blocking operations

### Error Handling
**Rule**: Never break agent execution due to SDK errors
```javascript
// CORRECT ✅
try {
  await reporter.report(data);
} catch (error) {
  console.error('[AIM SDK] Failed to report:', error);
  // Continue - don't throw
}

// WRONG ❌
await reporter.report(data);  // If fails, agent crashes
```

---

## Getting Started

### Step 1: Read Context
```bash
cat /Users/decimai/workspace/agent-identity-management/AIM_SDK_IMPLEMENTATION_PLAN.md
cat /Users/decimai/workspace/agent-identity-management/CLAUDE.md
```

### Step 2: Start with Phase 1 (Backend)
Begin with database migration, then work through backend tasks sequentially.

### Step 3: Test Backend Before Moving to SDKs
Verify backend endpoint works with curl before building SDKs.

### Step 4: Build SDKs One at a Time
Start with JavaScript (most common), then Python, then Go.

### Step 5: Update UI
Enhance existing components to show SDK-detected MCPs.

### Step 6: Integration Testing
Create test agents and verify end-to-end flow.

### Step 7: Chrome DevTools MCP Validation
Test every UI change with chrome-devtools MCP to catch issues.

---

## Questions or Issues?

### Performance Issues
- Profile each SDK (<50ms initialization, <1% CPU)
- Check backend logs for slow queries
- Verify API response times (<100ms)

### Detection Not Working
- Check SDK logs for errors
- Verify API key is correct
- Check backend logs for incoming requests
- Verify agent.talks_to is updating

### UI Not Updating
- Check Network tab (is API called?)
- Check Console tab (any errors?)
- Verify TypeScript types match backend JSON
- Use chrome-devtools MCP to debug

### Build Errors
- Run `go build ./...` (backend)
- Run `npm run build` (frontend)
- Check for missing dependencies
- Verify Go/Node versions match requirements

---

## Success Criteria

When all tasks complete, you should have:

✅ Backend endpoint receiving SDK detection reports
✅ 3 SDKs (JavaScript, Python, Go) published and working
✅ UI showing SDK-detected MCPs with badges
✅ Integration tests passing
✅ Documentation complete
✅ Chrome DevTools MCP testing shows zero errors

---

## Ready to Build?

Start with **Phase 1, Task 1.1** (Database Migration) and work through sequentially.

Use Chrome DevTools MCP for all UI testing.

Take your time, test thoroughly, and follow existing code patterns.

Good luck! 🚀
