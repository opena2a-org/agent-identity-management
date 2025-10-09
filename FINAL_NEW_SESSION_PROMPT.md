# Comprehensive MCP Detection Implementation - New Session Prompt

**Copy this entire prompt into a new Claude Code session to implement Phase 4**

---

## Context

You are working on **AIM (Agent Identity Management)**, an open-source enterprise platform for managing AI agent and MCP (Model Context Protocol) server identities.

**Project Location**: `/Users/decimai/workspace/agent-identity-management/`

### Tech Stack
- **Backend**: Go (Fiber v3 framework)
- **Frontend**: Next.js 15 (App Router), TypeScript, Shadcn/ui
- **Database**: PostgreSQL 16
- **Cache**: Redis 7

---

## What's Already Built (Phases 1-3) ✅

### Backend API
- ✅ `POST /api/v1/agents/:id/mcp-servers` - Manual MCP registration
- ✅ `POST /api/v1/agents/:id/mcp-servers/detect` - Auto-detect from Claude Desktop config
- ✅ `GET /api/v1/agents/:id/mcp-servers` - Get agent's MCPs
- ✅ `DELETE /api/v1/agents/:id/mcp-servers/:mcp_id` - Remove single MCP
- ✅ `DELETE /api/v1/agents/:id/mcp-servers/bulk` - Remove multiple MCPs
- ✅ Authentication, authorization, audit logging

### Frontend UI
- ✅ `AutoDetectButton` - One-click Claude Desktop config detection
- ✅ `MCPServerSelector` - Manual multi-select interface
- ✅ `MCPServerList` - View and manage connections
- ✅ `AgentMCPGraph` - Visual relationship graph
- ✅ Agent details page with tabs (Connections, Graph, Details)

### Database
- ✅ `agents` table with `talks_to` JSONB array
- ✅ `mcp_servers` table
- ✅ `audit_logs` table

### Detection Methods (2/4 Complete)
1. ✅ **Manual Registration** - User adds MCPs via UI (100% confidence)
2. ✅ **Claude Desktop Config** - Auto-detect from config file (85% confidence)
3. 🔄 **SDK Integration** - NEW (to implement)
4. 🔄 **Direct API Calls** - NEW (to implement)

---

## Your Task: Implement Phase 4

Add 2 new detection methods that work **alongside** existing ones:

### Method 3: SDK Integration (95-100% confidence)
- Agents embed AIM SDK (`npm install @aim/sdk`)
- SDK auto-detects MCP imports/connections at runtime
- SDK reports to AIM API asynchronously (batched)
- Zero configuration, minimal code changes

### Method 4: Direct API Calls (90-100% confidence)
- Agents make HTTP POST to AIM API
- Manually report MCP usage
- No SDK required
- Full control over reporting

---

## Implementation Plan

**Read these documents first** (in order):

1. `/Users/decimai/workspace/agent-identity-management/COMPLETE_AIM_DETECTION_ARCHITECTURE.md`
   - Shows how all 4 methods work together
   - Complete architecture diagram
   - Database schema
   - API endpoints

2. `/Users/decimai/workspace/agent-identity-management/COMPREHENSIVE_DETECTION_IMPLEMENTATION_PLAN.md`
   - Detailed step-by-step implementation
   - Code examples for all components
   - Testing strategy

3. `/Users/decimai/workspace/agent-identity-management/TALKS_TO_COMPLETE_IMPLEMENTATION.md`
   - What we've already built (Phases 1-3)
   - Existing code locations
   - Current architecture

4. `/Users/decimai/workspace/agent-identity-management/AIM_SDK_ARCHITECTURE.md`
   - SDK design and implementation details
   - Language-specific approaches (JS, Python, Go)
   - Performance requirements

---

## Phase 4 Implementation Steps

### Step 1: Backend - Detection API (Start Here)

#### 1.1 Create Database Migrations
**File**: `apps/backend/migrations/029_create_detection_tables.up.sql`

Create 3 new tables:
- `agent_mcp_detections` - Cache detection results with confidence scores
- `sdk_installations` - Track SDK installations and status
- `agent_mcp_runtime_stats` - Optional runtime analytics (for premium)

**File**: `apps/backend/migrations/029_create_detection_tables.down.sql`

Drop tables for rollback.

#### 1.2 Create Domain Models
**File**: `apps/backend/internal/domain/detection.go`

Define types:
- `DetectionMethod` enum (manual, claude_config, sdk_import, sdk_runtime, direct_api)
- `AgentMCPDetection` struct
- `SDKInstallation` struct
- `RuntimeStats` struct
- Request/Response types

#### 1.3 Create Detection Service
**File**: `apps/backend/internal/application/detection_service.go`

Implement methods:
- `ReportDetections(ctx, agentID, orgID, req)` - Process detection events
- `ReportRuntimeStats(ctx, agentID, orgID, req)` - Store runtime stats
- `GetDetectionStatus(ctx, agentID, orgID)` - Get current status

**Key Logic**:
- Validate agent belongs to organization
- Store in `agent_mcp_detections` table
- Update `agents.talks_to` array if new MCP
- Deduplicate detections
- Boost confidence if multiple methods detect same MCP
- Audit log all operations

#### 1.4 Create HTTP Handlers
**File**: `apps/backend/internal/interfaces/http/handlers/detection_handler.go`

Implement handlers:
- `ReportDetection(c fiber.Ctx)` - POST /detection/report
- `ReportRuntime(c fiber.Ctx)` - POST /detection/runtime
- `GetStatus(c fiber.Ctx)` - GET /detection/status

**Validation**:
- Parse and validate agent ID from URL
- Get organization ID from auth context
- Validate request body
- Return appropriate HTTP status codes

#### 1.5 Register Routes
**File**: `apps/backend/cmd/server/main.go`

Add new routes:
```go
detectionHandler := handlers.NewDetectionHandler(detectionService)
apiV1.Post("/agents/:id/detection/report", authMiddleware, detectionHandler.ReportDetection)
apiV1.Post("/agents/:id/detection/runtime", authMiddleware, detectionHandler.ReportRuntime)
apiV1.Get("/agents/:id/detection/status", authMiddleware, detectionHandler.GetStatus)
```

#### 1.6 Test Backend
```bash
# Run migrations
psql -d aim -f apps/backend/migrations/029_create_detection_tables.up.sql

# Start server
cd apps/backend && go run cmd/server/main.go

# Test endpoint
curl -X POST http://localhost:8080/api/v1/agents/{agent-id}/detection/report \
  -H "Authorization: Bearer TOKEN" \
  -d '{
    "detections": [{
      "mcpServer": "filesystem",
      "detectionMethod": "direct_api",
      "confidence": 100,
      "timestamp": "2025-10-09T12:00:00Z"
    }]
  }'
```

---

### Step 2: JavaScript/TypeScript SDK

#### 2.1 Create SDK Project
```bash
mkdir aim-sdk-js
cd aim-sdk-js
npm init -y
npm install typescript @types/node
```

#### 2.2 Implement SDK Core
**File**: `aim-sdk-js/src/client.ts`

Implement `AIMClient` class:
- Configuration management (API key, agent ID, API URL)
- HTTP client for API calls
- Authentication (bearer token)
- Error handling

#### 2.3 Implement Detection Hooks
**File**: `aim-sdk-js/src/detection/import-hook.ts`

Hook into ES module imports:
```typescript
// Intercept dynamic imports to detect @modelcontextprotocol/*
```

**File**: `aim-sdk-js/src/detection/require-hook.ts`

Hook into CommonJS requires:
```typescript
// Wrap Module.prototype.require to detect MCP imports
```

**File**: `aim-sdk-js/src/detection/client-interceptor.ts`

Intercept MCP client creation:
```typescript
// Wrap StdioClientTransport constructor
```

#### 2.4 Implement Reporting
**File**: `aim-sdk-js/src/reporting/queue.ts`

Event queue with:
- Deduplication (don't report same MCP twice)
- Batching (30s or 10 events, whichever first)
- Async/non-blocking

**File**: `aim-sdk-js/src/reporting/batch.ts`

Batch sender with:
- HTTP POST to AIM API
- Retry logic (exponential backoff)
- Offline cache (localStorage/file)

#### 2.5 Test SDK
```typescript
// Create test agent
import { AIMClient } from './src/client'

const aim = new AIMClient({
  apiKey: 'test-key',
  agentId: 'test-agent'
})

// SDK should detect this import
import { Client } from '@modelcontextprotocol/sdk/client/index.js'
```

#### 2.6 Publish to npm
```bash
npm version 1.0.0
npm publish --access public
```

---

### Step 3: Python SDK

#### 3.1 Create SDK Project
```bash
mkdir aim-sdk-python
cd aim-sdk-python
touch setup.py pyproject.toml
```

#### 3.2 Implement SDK Core
**File**: `aim_sdk/client.py`

Port JavaScript SDK to Python:
- `AIMClient` class
- Configuration management
- HTTP client (requests library)

#### 3.3 Implement Detection Hooks
**File**: `aim_sdk/detection/import_hook.py`

```python
import sys
from importlib.abc import MetaPathFinder

class AIMImportFinder(MetaPathFinder):
    def find_module(self, fullname, path=None):
        if 'mcp' in fullname:
            aim_sdk.report_detection(...)
        return None

sys.meta_path.insert(0, AIMImportFinder())
```

#### 3.4 Test SDK
```python
from aim_sdk import AIMClient

aim = AIMClient(api_key='test-key', agent_id='test-agent')

# SDK should detect this import
from mcp.client import Client
```

#### 3.5 Publish to PyPI
```bash
python setup.py sdist bdist_wheel
twine upload dist/*
```

---

### Step 4: Go SDK

#### 4.1 Create SDK Module
```bash
mkdir aim-sdk-go
cd aim-sdk-go
go mod init github.com/opena2a/aim-sdk-go
```

#### 4.2 Implement SDK Core
**File**: `client.go`

```go
package aimsdk

type Client struct {
    apiKey  string
    agentID string
    apiURL  string
}

func NewClient(config Config) *Client {
    // Initialize client
}
```

#### 4.3 Implement Detection
**File**: `detection/runtime_reflection.go`

```go
// Use runtime.Callers to detect MCP imports
```

#### 4.4 Test SDK
```go
import "github.com/opena2a/aim-sdk-go"

func main() {
    aim := aimsdk.NewClient(aimsdk.Config{
        APIKey:  "test-key",
        AgentID: "test-agent",
    })
    defer aim.Close()
}
```

---

### Step 5: Frontend UI Updates

#### 5.1 Create Detection Status Component
**File**: `apps/web/components/agents/detection-status.tsx`

Display:
- SDK installation status (Yes/No, version)
- Detected MCPs with confidence scores
- Detection methods per MCP (badges)
- Last seen timestamp

#### 5.2 Create Method Badge Component
**File**: `apps/web/components/agents/detection-method-badge.tsx`

Show detection method with icon:
- sdk_import → Code icon
- sdk_runtime → Activity icon
- claude_config → Package icon
- manual → User icon

Color by confidence:
- Green: ≥95%
- Yellow: 85-94%
- Gray: <85%

#### 5.3 Update Agent Details Page
**File**: `apps/web/app/dashboard/agents/[id]/page.tsx`

Add new tab: "Detection Status"

Show:
- SDK status card
- Detection methods used
- Confidence scores
- Runtime analytics (if enabled)

#### 5.4 Update API Client
**File**: `apps/web/lib/api.ts`

Add methods:
```typescript
async getDetectionStatus(agentId: string) {
  return this.get(`/agents/${agentId}/detection/status`)
}
```

---

### Step 6: Integration Testing

#### 6.1 Backend Tests
```bash
cd apps/backend
go test ./internal/application/detection_service_test.go -v
go test ./internal/interfaces/http/handlers/detection_handler_test.go -v
```

#### 6.2 SDK Tests
```bash
# JavaScript
cd aim-sdk-js && npm test

# Python
cd aim-sdk-python && pytest tests/

# Go
cd aim-sdk-go && go test ./...
```

#### 6.3 End-to-End Test

1. Start AIM backend
2. Create test agent
3. Run agent with SDK
4. Verify detection in database
5. Check frontend displays status

#### 6.4 Chrome DevTools Testing
```typescript
// Navigate to agent details
mcp__chrome-devtools__navigate_page({
  url: "http://localhost:3000/dashboard/agents/{agent-id}"
})

// Check detection status tab
mcp__chrome-devtools__click({ uid: "detection-status-tab-uid" })

// Verify no console errors
mcp__chrome-devtools__list_console_messages()

// Take screenshot
mcp__chrome-devtools__take_screenshot()
```

---

### Step 7: Documentation

#### 7.1 User Guide
**File**: `docs/user-guide/detection-methods.md`

Explain:
- 4 detection methods
- When to use each
- SDK integration instructions
- Direct API examples
- Confidence scores

#### 7.2 SDK Documentation
**Files**:
- `aim-sdk-js/README.md`
- `aim-sdk-python/README.md`
- `aim-sdk-go/README.md`

Include:
- Installation instructions
- Quick start example
- Configuration options
- API reference

#### 7.3 API Documentation
**File**: `docs/api/detection-endpoints.md`

Document:
- POST /detection/report
- POST /detection/runtime
- GET /detection/status
- Request/response formats
- Error codes

---

## Important Guidelines

### Naming Conventions (CRITICAL)
**ALWAYS match naming across all layers:**

- **Database** (PostgreSQL): `snake_case`
  - `detection_method`, `confidence_score`, `first_detected_at`

- **Backend** (Go structs): `PascalCase`
  - `DetectionMethod`, `ConfidenceScore`, `FirstDetectedAt`

- **Backend** (JSON tags): `camelCase`
  - `json:"detectionMethod"`, `json:"confidenceScore"`

- **Frontend** (TypeScript): `camelCase`
  - `detectionMethod`, `confidenceScore`, `firstDetectedAt`

**Check CLAUDE.md for complete naming conventions!**

### Code Quality
- ✅ Every feature needs tests (unit + integration)
- ✅ Clear function/variable names
- ✅ Error handling and logging
- ✅ Security validation (input sanitization, auth checks)
- ✅ Performance profiling (<100ms API latency target)

### SDK Performance Requirements
- Startup overhead: <50ms (JavaScript), <100ms (Python), <10ms (Go)
- Memory footprint: <10MB (JavaScript), <15MB (Python), <5MB (Go)
- CPU overhead: <0.1% (imperceptible)
- Batch reporting: 30s or 10 events

### Testing
- Unit tests for all functions
- Integration tests for APIs
- E2E tests with Chrome DevTools MCP
- Performance benchmarks

---

## Success Criteria

Before marking Phase 4 complete, verify:

- [ ] 3 new database tables created and migrated
- [ ] 3 new API endpoints working (/report, /runtime, /status)
- [ ] 3 SDKs implemented and published (npm, PyPI, Go modules)
- [ ] Frontend displays detection status and method badges
- [ ] Multiple detection methods work together (confidence boosting)
- [ ] All tests passing (backend, SDKs, frontend)
- [ ] Chrome DevTools MCP shows no console errors
- [ ] Documentation complete (user guides, API docs, SDK READMEs)
- [ ] 100% backward compatibility (Phases 1-3 still work)
- [ ] Performance targets met (<100ms API, <0.1% CPU for SDKs)

---

## Key Files to Reference

### Existing Code (Phases 1-3)
```
apps/backend/
├── internal/
│   ├── application/
│   │   └── agent_service.go               # Existing agent logic
│   ├── interfaces/http/handlers/
│   │   └── agent_handler.go               # Existing HTTP handlers
│   └── domain/
│       ├── agent.go                       # Agent model
│       └── mcp_server.go                  # MCP server model
└── cmd/server/
    └── main.go                            # Route registration

apps/web/
├── lib/
│   └── api.ts                             # Frontend API client
├── components/agents/
│   ├── auto-detect-button.tsx             # Existing component
│   ├── mcp-server-selector.tsx            # Existing component
│   ├── mcp-server-list.tsx                # Existing component
│   └── agent-mcp-graph.tsx                # Existing component
└── app/dashboard/agents/[id]/
    └── page.tsx                           # Agent details page
```

### New Files to Create (Phase 4)
```
apps/backend/
├── migrations/
│   ├── 029_create_detection_tables.up.sql   # NEW
│   └── 029_create_detection_tables.down.sql # NEW
├── internal/
│   ├── domain/
│   │   └── detection.go                     # NEW
│   ├── application/
│   │   └── detection_service.go             # NEW
│   └── interfaces/http/handlers/
│       └── detection_handler.go             # NEW

apps/web/
└── components/agents/
    ├── detection-status.tsx                 # NEW
    └── detection-method-badge.tsx           # NEW

SDKs (separate repos):
├── aim-sdk-js/                              # NEW
├── aim-sdk-python/                          # NEW
└── aim-sdk-go/                              # NEW
```

---

## Common Pitfalls to Avoid

1. **Don't skip testing** - Test each component before moving on
2. **Match naming exactly** - Backend JSON tags MUST match frontend interfaces
3. **Validate all inputs** - SQL injection, path traversal, etc.
4. **Handle errors gracefully** - Don't fail entire batch if one detection fails
5. **Profile performance** - Ensure SDKs are truly <0.1% CPU overhead
6. **Test with Chrome DevTools MCP** - Catch frontend bugs immediately
7. **Update audit logs** - Log all detection events
8. **Maintain backward compatibility** - Don't break existing features

---

## Questions to Ask

Before implementing, clarify:

1. Should runtime monitoring (stats) be opt-in or default? **(Suggest: Opt-in)**
2. Should SDK auto-register agents on first heartbeat? **(Suggest: Yes)**
3. What should happen if SDK can't reach AIM API? **(Suggest: Cache locally, retry later)**
4. Should we enforce SDK version compatibility? **(Suggest: Yes, warn on old versions)**
5. Should confidence boosting be configurable? **(Suggest: No, use fixed formula)**

---

## Resources

### Documentation to Read First
1. `COMPLETE_AIM_DETECTION_ARCHITECTURE.md` - Complete architecture
2. `COMPREHENSIVE_DETECTION_IMPLEMENTATION_PLAN.md` - Detailed implementation
3. `AIM_SDK_ARCHITECTURE.md` - SDK design
4. `TALKS_TO_COMPLETE_IMPLEMENTATION.md` - Existing features
5. `CLAUDE.md` - Project conventions and guidelines

### External Libraries
- **Go**: Fiber v3, pgx, uuid
- **JavaScript**: esbuild (for AST parsing)
- **Python**: importlib, requests
- **Go SDK**: net/http, encoding/json

### MCP SDK References
- JavaScript: `@modelcontextprotocol/sdk`
- Python: `mcp`
- Docs: https://modelcontextprotocol.io/

---

## Ready to Start?

Begin with **Step 1: Backend - Detection API**. Create the database migrations first, then implement the detection service, handlers, and route registration.

Take your time, test thoroughly, and follow the "ghost mode" philosophy - detection should be invisible to users.

**Goal**: Make MCP detection so comprehensive and frictionless that AIM becomes the definitive source of truth for AI agent ecosystems.

Good luck! 🚀

---

**Last Updated**: October 9, 2025
**Project**: Agent Identity Management (AIM)
**Phase**: 4 (SDK Integration + Direct API)
