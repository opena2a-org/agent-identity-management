# Complete AIM Detection Architecture

## Overview

This document describes **all the ways** AI agents can report MCP server usage to AIM, integrating both **what we've already built** (Phases 1-3) and **new detection methods** (SDK, Direct API).

---

## Current AIM Architecture (What We've Built)

### Backend (Go/Fiber)
- ✅ Agent management API
- ✅ MCP server management API
- ✅ Agent-MCP relationship API (talks_to)
- ✅ Auto-detection endpoint (Claude Desktop config)
- ✅ Authentication & authorization (JWT)
- ✅ Audit logging for all operations

### Frontend (Next.js)
- ✅ Dashboard
- ✅ Agent details page with tabs
- ✅ 4 UI components (AutoDetect, Selector, List, Graph)
- ✅ Real-time updates

### Database (PostgreSQL)
- ✅ `agents` table with `talks_to` JSONB array
- ✅ `mcp_servers` table
- ✅ `audit_logs` table

---

## Complete Detection Methods

AIM supports **6 detection methods** (2 built, 4 to add):

| # | Method | Status | Best For | Confidence | User Effort |
|---|--------|--------|----------|------------|-------------|
| 1 | **Manual Registration** | ✅ Built | Testing, small teams | 100% | High |
| 2 | **Claude Desktop Config** | ✅ Built | Claude Desktop users | 85% | Low |
| 3 | **SDK Integration** | 🔄 New | New agents, full visibility | 95-100% | Minimal |
| 4 | **Direct API Calls** | 🔄 New | Custom agents, existing infra | 90-100% | Medium |
| 5 | **CI/CD Integration** | 🚧 Future | Automated detection | 90% | Minimal |
| 6 | **Network Discovery** | 🚧 Future | Enterprise monitoring | 80% | Zero |

---

## Architecture Diagram

```
┌───────────────────────────────────────────────────────────────────────────────┐
│                               DETECTION SOURCES                                │
├───────────────────────────────────────────────────────────────────────────────┤
│                                                                                │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐    │
│  │  Manual UI   │  │  Claude      │  │  AIM SDK     │  │  Direct API  │    │
│  │  (Built ✅)   │  │  Desktop     │  │  (New 🔄)    │  │  (New 🔄)    │    │
│  │              │  │  (Built ✅)   │  │              │  │              │    │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘    │
│         │                 │                  │                  │             │
│         └─────────────────┼──────────────────┼──────────────────┘             │
│                           │                  │                                │
│                           ↓                  ↓                                │
│  ┌──────────────┐  ┌─────────────────────────────────────┐                  │
│  │  CI/CD       │  │  Network Discovery (Enterprise)     │                  │
│  │  (Future 🚧) │→ │  (Future 🚧)                        │                  │
│  └──────────────┘  └─────────────────────────────────────┘                  │
│                                                                                │
└───────────────────────────────────────────────────────────────────────────────┘
                                       ↓
┌───────────────────────────────────────────────────────────────────────────────┐
│                                  AIM BACKEND                                   │
├───────────────────────────────────────────────────────────────────────────────┤
│                                                                                │
│  API Endpoints (Go/Fiber)                                                     │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  ✅ Built:                                                           │    │
│  │  POST   /api/v1/agents/:id/mcp-servers                   (Manual)   │    │
│  │  POST   /api/v1/agents/:id/mcp-servers/detect            (Claude)   │    │
│  │  GET    /api/v1/agents/:id/mcp-servers                              │    │
│  │  DELETE /api/v1/agents/:id/mcp-servers/:mcp_id                      │    │
│  │  DELETE /api/v1/agents/:id/mcp-servers/bulk                         │    │
│  │                                                                       │    │
│  │  🔄 New:                                                             │    │
│  │  POST   /api/v1/agents/:id/detection/report              (SDK)      │    │
│  │  POST   /api/v1/agents/:id/detection/runtime             (SDK)      │    │
│  │  GET    /api/v1/agents/:id/detection/status              (SDK)      │    │
│  │  POST   /api/v1/detection/batch                          (CI/CD)    │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                                │
│  Service Layer                                                                │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  - AgentService (✅ built)                                           │    │
│  │  - MCPService (✅ built)                                             │    │
│  │  - DetectionService (🔄 new)                                         │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                                │
│  Database (PostgreSQL)                                                        │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  ✅ agents.talks_to (JSONB array)                                    │    │
│  │  ✅ mcp_servers (registry)                                           │    │
│  │  ✅ audit_logs                                                       │    │
│  │  🔄 agent_mcp_detections (new - detection cache)                    │    │
│  │  🔄 sdk_installations (new - SDK status)                            │    │
│  │  🔄 agent_mcp_runtime_stats (new - usage analytics)                 │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                                │
└───────────────────────────────────────────────────────────────────────────────┘
                                       ↓
┌───────────────────────────────────────────────────────────────────────────────┐
│                                  AIM FRONTEND                                  │
├───────────────────────────────────────────────────────────────────────────────┤
│                                                                                │
│  Dashboard (Next.js)                                                          │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │  ✅ Agent Details Page:                                              │    │
│  │     - AutoDetectButton (Claude Desktop config)                       │    │
│  │     - MCPServerSelector (Manual selection)                           │    │
│  │     - MCPServerList (View/manage)                                    │    │
│  │     - AgentMCPGraph (Visual relationships)                           │    │
│  │                                                                       │    │
│  │  🔄 Detection Status Page (New):                                     │    │
│  │     - SDK installation status                                        │    │
│  │     - Detection method badges                                        │    │
│  │     - Confidence scores                                              │    │
│  │     - Runtime analytics charts                                       │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                                                                │
└───────────────────────────────────────────────────────────────────────────────┘
```

---

## Method 1: Manual Registration (✅ Built)

### How It Works
1. User navigates to agent details page
2. Clicks "Add MCP Servers" button
3. Selects servers from multi-select dialog
4. Clicks "Add" to save

### Implementation
- **Frontend**: `MCPServerSelector` component
- **Backend**: `POST /api/v1/agents/:id/mcp-servers`
- **Database**: Updates `agents.talks_to` array

### When To Use
- Testing and development
- Small teams (<10 agents)
- One-off configurations
- No SDK integration yet

### Example
```bash
curl -X POST http://localhost:8080/api/v1/agents/123/mcp-servers \
  -H "Authorization: Bearer TOKEN" \
  -d '{"mcp_server_identifiers": ["filesystem", "github"]}'
```

---

## Method 2: Claude Desktop Config (✅ Built)

### How It Works
1. User clicks "Auto-Detect MCPs" button
2. System reads `claude_desktop_config.json`
3. Parses `mcpServers` section
4. Optionally auto-registers new servers
5. Maps all detected servers to agent

### Implementation
- **Frontend**: `AutoDetectButton` component
- **Backend**: `POST /api/v1/agents/:id/mcp-servers/detect`
- **Config**: `~/Library/Application Support/Claude/claude_desktop_config.json`

### When To Use
- Agents using Claude Desktop
- Quick setup for existing Claude users
- Migration from Claude Desktop to AIM

### Config Format
```json
{
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "/data"]
    },
    "github": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-github"],
      "env": { "GITHUB_TOKEN": "..." }
    }
  }
}
```

### Example
```bash
curl -X POST http://localhost:8080/api/v1/agents/123/mcp-servers/detect \
  -H "Authorization: Bearer TOKEN" \
  -d '{
    "config_path": "~/Library/Application Support/Claude/claude_desktop_config.json",
    "auto_register": true,
    "dry_run": false
  }'
```

---

## Method 3: SDK Integration (🔄 New)

### How It Works
1. Agent integrates AIM SDK (`npm install @aim/sdk`)
2. SDK auto-detects MCP imports/connections at runtime
3. SDK batches detection events (30s or 10 events)
4. SDK reports to AIM API asynchronously
5. AIM dashboard updates in real-time

### Implementation

#### Agent Code (JavaScript)
```typescript
import { AIMClient } from '@aim/sdk';

// Initialize SDK (2 lines)
const aim = new AIMClient({
  apiKey: process.env.AIM_API_KEY,
  agentId: 'my-agent-id'
});

// SDK automatically detects this import and reports it
import { Client } from '@modelcontextprotocol/sdk/client/index.js';

// Agent continues normally...
```

#### Backend Endpoints (New)
```
POST /api/v1/agents/:id/detection/report   - SDK reports detected MCPs
POST /api/v1/agents/:id/detection/runtime  - SDK reports runtime stats (optional)
GET  /api/v1/agents/:id/detection/status   - Check detection status
```

#### Detection Methods (SDK-Side)
1. **Import Hook**: Detects `import '@modelcontextprotocol/*'` (95% confidence)
2. **Runtime Connection**: Intercepts `MCPClient` creation (100% confidence)
3. **Stack Inspection**: Scans call stack for MCP usage (85% confidence)

### When To Use
- New agents being built
- Maximum visibility (runtime monitoring)
- Zero manual work
- Continuous detection

### Benefits
- **Zero configuration**: Works out of the box
- **Real-time updates**: Dashboard shows MCPs instantly
- **Runtime analytics**: Tool call counts, latency, errors
- **Invisible performance**: <0.1% CPU, <10MB memory

### SDK Packages
- **JavaScript/TypeScript**: `@aim/sdk` (npm)
- **Python**: `aim-sdk` (PyPI)
- **Go**: `github.com/opena2a/aim-sdk-go` (Go modules)

---

## Method 4: Direct API Calls (🔄 New)

### How It Works
1. Agent makes direct HTTP POST to AIM API
2. Manually reports MCP usage
3. No SDK required
4. Full control over what/when to report

### Implementation

#### Agent Code (Any Language)
```typescript
// Agent manually reports MCP usage
async function reportMCPUsage() {
  const response = await fetch(
    `https://aim.company.com/api/v1/agents/${agentId}/detection/report`,
    {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${apiKey}`,
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({
        detections: [
          {
            mcpServer: 'filesystem',
            detectionMethod: 'manual_report',
            confidence: 100,
            details: {
              transport: 'stdio',
              command: 'npx',
              args: ['-y', '@modelcontextprotocol/server-filesystem']
            },
            timestamp: new Date().toISOString()
          }
        ]
      })
    }
  );
}

// Call on agent startup
await reportMCPUsage();
```

#### Backend Endpoint
```
POST /api/v1/agents/:id/detection/report
```

#### Request Format
```json
{
  "detections": [
    {
      "mcpServer": "filesystem",
      "detectionMethod": "manual_report",
      "confidence": 100,
      "details": {
        "transport": "stdio",
        "command": "npx",
        "args": ["-y", "@modelcontextprotocol/server-filesystem", "/data"]
      },
      "timestamp": "2025-10-09T12:00:00Z"
    }
  ]
}
```

### When To Use
- Custom agent frameworks
- Existing infrastructure (can't add SDK)
- Specific reporting requirements
- Language not supported by SDK

### Benefits
- **No dependencies**: Direct HTTP calls
- **Full control**: Report exactly what you want
- **Flexible**: Works with any language/framework
- **Custom logic**: Implement your own detection

---

## Method 5: CI/CD Integration (🚧 Future)

### How It Works
1. Add AIM action to GitHub Actions / GitLab CI
2. On every commit, action scans code for MCP usage
3. Reports to AIM API automatically
4. Updates agent's MCPs in real-time

### Implementation (Conceptual)

#### GitHub Action
```yaml
name: AIM Detection
on: [push]
jobs:
  detect-mcps:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - uses: opena2a/aim-detect-action@v1
        with:
          aim-api-url: ${{ secrets.AIM_API_URL }}
          aim-api-key: ${{ secrets.AIM_API_KEY }}
          agent-id: ${{ secrets.AGENT_ID }}
```

#### Backend Endpoint
```
POST /api/v1/detection/batch
```

### When To Use
- Automated detection
- Pre-deployment validation
- Continuous compliance
- No runtime overhead

---

## Method 6: Network Discovery (🚧 Future - Enterprise)

### How It Works
1. AIM Discovery Service probes network
2. Detects agents via network connections
3. Inspects MCP stdio/SSE/WebSocket traffic
4. Automatically registers discovered agents/MCPs

### Implementation (Conceptual)
- Passive network monitoring
- Process inspection (with permissions)
- Traffic analysis
- No agent changes required

### When To Use
- Enterprise environments
- Zero-touch detection
- Shadow IT discovery
- Compliance auditing

---

## Database Schema Updates

### New Tables for SDK & Direct API

#### 1. `agent_mcp_detections` (Detection Cache)
```sql
CREATE TABLE agent_mcp_detections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    mcp_server_name VARCHAR(255) NOT NULL,
    detection_method VARCHAR(50) NOT NULL,
    confidence_score DECIMAL(5,2) NOT NULL,
    details JSONB,
    sdk_version VARCHAR(50),
    first_detected_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(agent_id, mcp_server_name, detection_method),
    INDEX idx_detections_lookup (agent_id, mcp_server_name)
);
```

**Purpose**: Cache detection results from all methods with confidence scores

**Fields**:
- `detection_method`: 'manual', 'claude_config', 'sdk_import', 'sdk_runtime', 'direct_api', 'ci_cd', 'network_discovery'
- `confidence_score`: 0-100 (how confident we are)
- `details`: Method-specific metadata (import path, command, etc.)

#### 2. `sdk_installations` (SDK Status)
```sql
CREATE TABLE sdk_installations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    sdk_language VARCHAR(50) NOT NULL,
    sdk_version VARCHAR(50) NOT NULL,
    installed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_heartbeat_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    auto_detect_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    runtime_monitor_enabled BOOLEAN NOT NULL DEFAULT FALSE,

    UNIQUE(agent_id),
    INDEX idx_sdk_heartbeat (agent_id, last_heartbeat_at)
);
```

**Purpose**: Track which agents have SDK installed and their status

#### 3. `agent_mcp_runtime_stats` (Usage Analytics)
```sql
CREATE TABLE agent_mcp_runtime_stats (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    mcp_server_name VARCHAR(255) NOT NULL,
    tool_name VARCHAR(255) NOT NULL,
    period_start TIMESTAMPTZ NOT NULL,
    period_end TIMESTAMPTZ NOT NULL,
    call_count INTEGER NOT NULL,
    success_count INTEGER NOT NULL,
    error_count INTEGER NOT NULL,
    latency_p50_ms INTEGER NOT NULL,
    latency_p95_ms INTEGER NOT NULL,
    latency_p99_ms INTEGER NOT NULL,

    INDEX idx_runtime_stats_time (agent_id, mcp_server_name, period_start)
);

SELECT create_hypertable('agent_mcp_runtime_stats', 'period_start');
```

**Purpose**: Store runtime usage statistics from SDK (optional, opt-in)

---

## Frontend UI Updates

### Current UI (✅ Built)
- Agent details page with 4 components
- AutoDetectButton (Claude Desktop)
- MCPServerSelector (Manual)
- MCPServerList (View/manage)
- AgentMCPGraph (Relationships)

### New UI (🔄 To Add)

#### 1. Detection Status Badge
**Location**: Agent cards on dashboard

**Display**:
```
Agent Name             [SDK ✓] [Manual ✓] [Confidence: 95%]
```

#### 2. Detection Methods Tab
**Location**: Agent details page, new tab

**Features**:
- Show all detection methods used
- Confidence score per method
- Last detected timestamp
- SDK installation status
- Runtime monitoring toggle

#### 3. Runtime Analytics Charts
**Location**: Agent details page, new tab

**Displays**:
- MCP tool call counts (bar chart)
- Latency trends (line chart)
- Error rates (pie chart)
- Peak usage hours (heatmap)

#### 4. Detection Method Badges
**Component**: `<DetectionMethodBadge />`

**Example**:
```tsx
<DetectionMethodBadge method="sdk_import" confidence={95} />
<DetectionMethodBadge method="claude_config" confidence={85} />
<DetectionMethodBadge method="manual" confidence={100} />
```

---

## API Endpoints Summary

### Existing (✅ Built)
```
GET    /api/v1/agents/:id/mcp-servers              - Get agent's MCPs
POST   /api/v1/agents/:id/mcp-servers              - Add MCPs (manual)
DELETE /api/v1/agents/:id/mcp-servers/:mcp_id      - Remove single MCP
DELETE /api/v1/agents/:id/mcp-servers/bulk         - Remove multiple MCPs
POST   /api/v1/agents/:id/mcp-servers/detect       - Auto-detect (Claude config)
```

### New (🔄 To Add)
```
POST   /api/v1/agents/:id/detection/report         - SDK/Direct API report
POST   /api/v1/agents/:id/detection/runtime        - Runtime stats (optional)
GET    /api/v1/agents/:id/detection/status         - Detection status
POST   /api/v1/detection/batch                     - Batch detection (CI/CD)
```

---

## Implementation Priority

### Phase 1: SDK Foundation (Week 1-2)
1. JavaScript/TypeScript SDK
   - Import/require hooks
   - MCP client interception
   - Reporting module
   - NPM package

2. Backend Detection API
   - `POST /detection/report` endpoint
   - `agent_mcp_detections` table
   - `sdk_installations` table

3. Basic UI Updates
   - Detection status display
   - Method badges

### Phase 2: Direct API + Python SDK (Week 3)
1. Direct API documentation
2. Python SDK
3. Runtime analytics backend

### Phase 3: Go SDK + UI (Week 4)
1. Go SDK
2. Runtime analytics UI
3. Detection dashboard

### Phase 4: CI/CD Integration (Future)
1. GitHub Action
2. GitLab CI component
3. Batch detection API

### Phase 5: Network Discovery (Future - Enterprise)
1. Discovery service
2. Network monitoring
3. Compliance reporting

---

## How Methods Work Together

### Example: Comprehensive Detection

An agent can use **multiple detection methods simultaneously**:

1. **Developer** manually adds "filesystem" via UI → 100% confidence
2. **SDK** detects "github" via import hook → 95% confidence
3. **SDK** confirms "github" via runtime connection → 100% confidence (boosted)
4. **Direct API** reports "sqlite" manually → 100% confidence
5. **Claude Desktop** config shows all three → 85% confidence (validation)

**Result**: Agent has 3 MCPs with high confidence, detected via 5 methods

### Confidence Boosting

When multiple methods detect the same MCP:
- Single method: Base confidence (85-100%)
- Two methods: Average + 10% bonus (up to 99%)
- Three+ methods: Average + 20% bonus (cap at 99%)
- Never 100% to indicate "multiple verifications, very confident"

### Dashboard Display
```
Agent: my-agent
MCPs Detected: 3

filesystem
├── Detected by: Manual, SDK (import), SDK (runtime), Claude Desktop
├── Confidence: 99% (4 methods)
└── Last seen: 2 minutes ago

github
├── Detected by: SDK (import), SDK (runtime)
├── Confidence: 98% (2 methods)
└── Last seen: 30 seconds ago

sqlite
├── Detected by: Direct API
├── Confidence: 100% (1 method)
└── Last seen: 5 minutes ago
```

---

## Security & Privacy

### Data Collection

**What AIM Collects**:
- MCP server names
- Detection method used
- Confidence scores
- Timestamps
- SDK version (if applicable)

**What AIM DOES NOT Collect**:
- Agent source code
- MCP tool call arguments (unless runtime monitoring explicitly enabled)
- User data or PII
- Environment variables
- File contents

### Authentication

All API endpoints require:
- Valid JWT token
- Agent must belong to user's organization
- Member permissions for write operations

### Audit Logging

Every detection is logged:
```json
{
  "action": "mcp_detected",
  "method": "sdk_import",
  "agent_id": "...",
  "mcp_server": "filesystem",
  "confidence": 95,
  "user_id": "...",
  "timestamp": "..."
}
```

---

## Success Metrics

### Technical
- ✅ 6 detection methods supported
- ✅ <100ms API latency (p95)
- ✅ >95% detection accuracy
- ✅ <0.1% CPU overhead (SDK)
- ✅ Real-time dashboard updates

### User Experience
- ✅ Multiple integration options (SDK, API, UI, Claude, CI/CD)
- ✅ Zero configuration required (SDK)
- ✅ Full control available (Direct API)
- ✅ Confidence scores shown
- ✅ Multi-method validation

### Business
- ✅ 80%+ of agents use SDK (target)
- ✅ 50%+ reduction in manual work
- ✅ 90%+ user satisfaction
- ✅ Foundation for premium products

---

## Conclusion

AIM now supports **comprehensive MCP detection** through:

1. ✅ **Manual Registration** - User control, built-in UI
2. ✅ **Claude Desktop Config** - Existing users, auto-detection
3. 🔄 **SDK Integration** - New agents, real-time, invisible
4. 🔄 **Direct API** - Custom agents, full flexibility
5. 🚧 **CI/CD Integration** - Automated, pre-deployment
6. 🚧 **Network Discovery** - Enterprise, zero-touch

**All methods** integrate seamlessly into the existing AIM platform, storing data in the same database, displaying in the same dashboard, and providing unified visibility.

**Goal**: Make MCP detection so comprehensive and frictionless that enterprises trust AIM as the definitive source of truth for their AI agent ecosystem.

---

**Last Updated**: October 9, 2025
**Status**: Methods 1-2 built ✅, Methods 3-4 in progress 🔄
