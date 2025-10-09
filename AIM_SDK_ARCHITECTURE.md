# AIM SDK Architecture - Comprehensive Design

## Core Principle

**AIM is NOT a scanning tool** - it's an SDK that agents integrate. The SDK runs inside the agent process, auto-detects MCPs using introspection/hooks, and reports to the AIM backend via API.

---

## Architecture Overview

```
┌──────────────────────────────────────────────────┐
│  User's Agent (JavaScript/Python/Go)             │
│                                                   │
│  ┌────────────────────────────────────────────┐ │
│  │  AIM SDK (embedded library)                │ │
│  │                                             │ │
│  │  ┌─────────────────────────────────────┐  │ │
│  │  │  Auto-Detection Module              │  │ │
│  │  │  - Import/require hooks             │  │ │
│  │  │  - MCP client interception          │  │ │
│  │  │  - Stack inspection                 │  │ │
│  │  │  - Module introspection             │  │ │
│  │  └─────────────────────────────────────┘  │ │
│  │                                             │ │
│  │  ┌─────────────────────────────────────┐  │ │
│  │  │  Runtime Monitor (optional)         │  │ │
│  │  │  - Tool call tracking               │  │ │
│  │  │  - Latency measurement              │  │ │
│  │  │  - Error tracking                   │  │ │
│  │  └─────────────────────────────────────┘  │ │
│  │                                             │ │
│  │  ┌─────────────────────────────────────┐  │ │
│  │  │  Reporting Module                   │  │ │
│  │  │  - Async event queue                │  │ │
│  │  │  - Batch reporting (30s/10 events)  │  │ │
│  │  │  - Retry logic                      │  │ │
│  │  │  - Offline cache                    │  │ │
│  │  └─────────────────────────────────────┘  │ │
│  │                                             │ │
│  └────────────────────────────────────────────┘ │
│                      ↓                           │
│              HTTP POST (async)                   │
└──────────────────────────────────────────────────┘
                       ↓
┌──────────────────────────────────────────────────┐
│  AIM Backend (Go/Fiber)                          │
│                                                   │
│  POST /api/v1/agents/:id/detection/report        │
│  POST /api/v1/agents/:id/detection/runtime       │
│  GET  /api/v1/agents/:id/detection/status        │
│                                                   │
│  - Receives detection data from SDKs             │
│  - Stores in PostgreSQL                          │
│  - Caches in Redis                               │
│  - Displays in dashboard                         │
└──────────────────────────────────────────────────┘
                       ↓
┌──────────────────────────────────────────────────┐
│  AIM Dashboard (Next.js)                         │
│                                                   │
│  - Shows detected MCPs per agent                 │
│  - Displays confidence scores                    │
│  - Shows detection method (import/runtime/etc)   │
│  - Real-time updates via WebSocket               │
└──────────────────────────────────────────────────┘
```

---

## SDK Components

### 1. Core SDK (Shared Logic)

**Purpose**: Common functionality across all language SDKs

**Responsibilities**:
- Configuration management (API key, endpoint, agent ID)
- HTTP client for AIM API calls
- Authentication (bearer token)
- Rate limiting (max 10 requests/min per agent)
- Error handling and retry logic
- Logging (configurable levels)

**Configuration**:
```typescript
interface AIMConfig {
  apiUrl: string;              // AIM backend URL
  apiKey: string;              // Agent's API key
  agentId: string;             // Agent UUID
  autoDetect?: boolean;        // Enable auto-detection (default: true)
  runtimeMonitor?: boolean;    // Enable runtime monitoring (default: false)
  reportInterval?: number;     // Batch interval in seconds (default: 30)
  offline?: boolean;           // Cache locally if AIM unreachable (default: true)
  logLevel?: 'debug' | 'info' | 'warn' | 'error';
}
```

### 2. Auto-Detection Module (Language-Specific)

**Purpose**: Detect MCPs from within agent process using introspection

#### JavaScript/TypeScript Implementation

**Detection Methods**:

1. **Import Hook** (ES Modules)
```typescript
// Hook into dynamic imports
const originalImport = global.import;
global.import = function(specifier) {
  if (specifier.includes('@modelcontextprotocol')) {
    aimSDK.reportDetection({
      mcpServer: extractServerName(specifier),
      detectionMethod: 'import',
      confidence: 95
    });
  }
  return originalImport.apply(this, arguments);
};
```

2. **Require Hook** (CommonJS)
```typescript
const Module = require('module');
const originalRequire = Module.prototype.require;

Module.prototype.require = function(id) {
  if (id.includes('@modelcontextprotocol')) {
    aimSDK.reportDetection({
      mcpServer: extractServerName(id),
      detectionMethod: 'require',
      confidence: 95
    });
  }
  return originalRequire.apply(this, arguments);
};
```

3. **MCP Client Interception**
```typescript
// Wrap StdioClientTransport constructor
const OriginalTransport = StdioClientTransport;
StdioClientTransport = class extends OriginalTransport {
  constructor(config) {
    super(config);

    // Extract MCP server from command args
    const serverName = extractServerFromCommand(config.command, config.args);

    aimSDK.reportDetection({
      mcpServer: serverName,
      detectionMethod: 'runtime_connection',
      confidence: 100,
      transport: 'stdio',
      command: config.command,
      args: config.args
    });
  }
};
```

#### Python Implementation

**Detection Methods**:

1. **Import Hook** (sys.meta_path)
```python
import sys
from importlib.abc import MetaPathFinder, Loader

class AIMImportFinder(MetaPathFinder):
    def find_module(self, fullname, path=None):
        if 'mcp' in fullname:
            aim_sdk.report_detection({
                'mcp_server': extract_server_name(fullname),
                'detection_method': 'import',
                'confidence': 95
            })
        return None

# Install the finder
sys.meta_path.insert(0, AIMImportFinder())
```

2. **MCP Client Interception**
```python
import mcp.client as mcp_client

# Wrap Client.__init__
OriginalClient = mcp_client.Client

class WrappedClient(OriginalClient):
    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)

        aim_sdk.report_detection({
            'mcp_server': extract_server_from_args(args, kwargs),
            'detection_method': 'runtime_connection',
            'confidence': 100
        })

mcp_client.Client = WrappedClient
```

3. **Stack Inspection**
```python
import inspect

def detect_mcp_usage():
    """Scan call stack for MCP-related modules"""
    for frame_info in inspect.stack():
        module = inspect.getmodule(frame_info.frame)
        if module and 'mcp' in module.__name__:
            aim_sdk.report_detection({
                'mcp_server': extract_server_from_module(module),
                'detection_method': 'stack_inspection',
                'confidence': 85
            })
```

#### Go Implementation

**Detection Methods**:

1. **Runtime Reflection**
```go
import (
    "reflect"
    "runtime"
)

func detectMCPImports() {
    // Analyze imported packages at runtime
    pcs := make([]uintptr, 100)
    n := runtime.Callers(0, pcs)

    for i := 0; i < n; i++ {
        fn := runtime.FuncForPC(pcs[i])
        if fn != nil && strings.Contains(fn.Name(), "mcp") {
            aimSDK.ReportDetection(Detection{
                MCPServer:       extractServerName(fn.Name()),
                DetectionMethod: "runtime_reflection",
                Confidence:      85,
            })
        }
    }
}
```

2. **Interface Wrapping**
```go
// Wrap MCP client interface
type WrappedMCPClient struct {
    *mcp.Client
}

func NewWrappedMCPClient(config mcp.Config) *WrappedMCPClient {
    client := mcp.NewClient(config)

    aimSDK.ReportDetection(Detection{
        MCPServer:       extractServerFromConfig(config),
        DetectionMethod: "runtime_connection",
        Confidence:      100,
        Transport:       config.Transport,
    })

    return &WrappedMCPClient{Client: client}
}
```

### 3. Runtime Monitor Module (Optional, Opt-In)

**Purpose**: Track MCP tool calls, measure performance, detect anomalies

**Features**:
- Tool call counting (which tools are used most)
- Latency measurement (p50, p95, p99)
- Error tracking (failed calls, error types)
- Usage patterns (peak hours, frequency)

**Implementation** (JavaScript example):
```typescript
// Intercept MCP tool calls
const originalCallTool = client.callTool;
client.callTool = async function(toolName, args) {
  const startTime = Date.now();

  try {
    const result = await originalCallTool.call(this, toolName, args);
    const latency = Date.now() - startTime;

    aimSDK.reportRuntime({
      mcpServer: this.serverName,
      toolName,
      success: true,
      latency,
      timestamp: new Date().toISOString()
    });

    return result;
  } catch (error) {
    const latency = Date.now() - startTime;

    aimSDK.reportRuntime({
      mcpServer: this.serverName,
      toolName,
      success: false,
      latency,
      error: error.message,
      timestamp: new Date().toISOString()
    });

    throw error;
  }
};
```

**Performance**:
- Zero blocking (all async)
- Batched reporting (hourly aggregates)
- Memory efficient (<5MB buffer)
- CPU overhead <0.1%

### 4. Reporting Module (Shared)

**Purpose**: Batch and send detection events to AIM backend

**Features**:
- Async event queue (non-blocking)
- Batch reporting (30s interval or 10 events, whichever first)
- Retry logic (exponential backoff)
- Offline mode (cache locally if AIM unreachable)
- Deduplication (don't report same MCP multiple times)

**Event Queue**:
```typescript
class ReportingQueue {
  private queue: DetectionEvent[] = [];
  private interval: number = 30000; // 30s
  private maxBatchSize: number = 10;

  add(event: DetectionEvent) {
    // Deduplicate
    const exists = this.queue.find(e =>
      e.mcpServer === event.mcpServer &&
      e.detectionMethod === event.detectionMethod
    );

    if (!exists) {
      this.queue.push(event);
    }

    // Flush if batch size reached
    if (this.queue.length >= this.maxBatchSize) {
      this.flush();
    }
  }

  async flush() {
    if (this.queue.length === 0) return;

    const batch = this.queue.splice(0, this.maxBatchSize);

    try {
      await this.sendToAIM(batch);
    } catch (error) {
      // Cache locally for retry
      await this.cacheOffline(batch);
    }
  }

  startBatchInterval() {
    setInterval(() => this.flush(), this.interval);
  }
}
```

**Retry Logic**:
```typescript
async sendToAIM(batch: DetectionEvent[], retries = 3) {
  for (let i = 0; i < retries; i++) {
    try {
      await fetch(`${config.apiUrl}/api/v1/agents/${config.agentId}/detection/report`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${config.apiKey}`,
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({ detections: batch })
      });
      return; // Success
    } catch (error) {
      if (i === retries - 1) throw error;
      await sleep(Math.pow(2, i) * 1000); // Exponential backoff
    }
  }
}
```

**Offline Cache**:
```typescript
async cacheOffline(batch: DetectionEvent[]) {
  // Store in local file (Node.js) or localStorage (browser)
  const cache = await this.loadCache();
  cache.push(...batch);
  await this.saveCache(cache);

  // Try to sync later
  setTimeout(() => this.syncOfflineCache(), 60000); // Retry in 1min
}
```

---

## Backend API Endpoints

### 1. Report Detection

**Endpoint**: `POST /api/v1/agents/:id/detection/report`

**Purpose**: SDK reports detected MCPs

**Request Body**:
```json
{
  "detections": [
    {
      "mcpServer": "filesystem",
      "detectionMethod": "import",
      "confidence": 95,
      "details": {
        "filePath": "src/index.ts",
        "lineNumber": 12,
        "importStatement": "import { FilesystemClient } from '@modelcontextprotocol/filesystem'"
      },
      "sdkVersion": "1.0.0",
      "timestamp": "2025-10-09T12:00:00Z"
    },
    {
      "mcpServer": "filesystem",
      "detectionMethod": "runtime_connection",
      "confidence": 100,
      "details": {
        "transport": "stdio",
        "command": "npx",
        "args": ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
      },
      "sdkVersion": "1.0.0",
      "timestamp": "2025-10-09T12:00:01Z"
    }
  ]
}
```

**Response**:
```json
{
  "success": true,
  "detectionsProcessed": 2,
  "newMCPs": ["filesystem"],
  "existingMCPs": [],
  "message": "Detections processed successfully"
}
```

**Backend Processing**:
1. Validate agent ID and API key
2. Deduplicate detections (same MCP, same method)
3. Store in `agent_mcp_detections` table
4. Update agent's `talks_to` array if new MCP
5. Increment trust score if high confidence (>90%)
6. Broadcast update via WebSocket (real-time dashboard)

### 2. Report Runtime Stats

**Endpoint**: `POST /api/v1/agents/:id/detection/runtime`

**Purpose**: SDK reports runtime MCP usage statistics

**Request Body**:
```json
{
  "mcpServer": "filesystem",
  "stats": {
    "period": "hourly",
    "startTime": "2025-10-09T12:00:00Z",
    "endTime": "2025-10-09T13:00:00Z",
    "toolCalls": {
      "read_file": {
        "count": 142,
        "successCount": 140,
        "errorCount": 2,
        "latency": {
          "p50": 23,
          "p95": 45,
          "p99": 78
        }
      },
      "write_file": {
        "count": 38,
        "successCount": 38,
        "errorCount": 0,
        "latency": {
          "p50": 31,
          "p95": 52,
          "p99": 89
        }
      }
    }
  },
  "sdkVersion": "1.0.0"
}
```

**Response**:
```json
{
  "success": true,
  "message": "Runtime stats recorded"
}
```

**Backend Processing**:
1. Store in TimescaleDB (time-series data)
2. Update MCP server trust score based on reliability
3. Flag anomalies (sudden latency spikes, high error rates)
4. Trigger alerts if thresholds exceeded

### 3. Get Detection Status

**Endpoint**: `GET /api/v1/agents/:id/detection/status`

**Purpose**: Check current detection status for an agent

**Response**:
```json
{
  "agentId": "123e4567-e89b-12d3-a456-426614174000",
  "sdkVersion": "1.0.0",
  "sdkInstalled": true,
  "autoDetectEnabled": true,
  "runtimeMonitorEnabled": false,
  "detectedMCPs": [
    {
      "name": "filesystem",
      "confidenceScore": 97.5,
      "detectedBy": ["import", "runtime_connection"],
      "firstDetected": "2025-10-09T12:00:00Z",
      "lastSeen": "2025-10-09T14:30:00Z",
      "toolCallCount": 180
    }
  ],
  "lastReportedAt": "2025-10-09T14:30:00Z"
}
```

---

## Database Schema

### New Tables

#### 1. `agent_mcp_detections`
```sql
CREATE TABLE agent_mcp_detections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    mcp_server_name VARCHAR(255) NOT NULL,
    detection_method VARCHAR(50) NOT NULL, -- 'import', 'require', 'runtime_connection', 'stack_inspection'
    confidence_score DECIMAL(5,2) NOT NULL,
    details JSONB, -- Method-specific details
    sdk_version VARCHAR(50) NOT NULL,
    first_detected_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(agent_id, mcp_server_name, detection_method),
    INDEX idx_detections_lookup (agent_id, mcp_server_name)
);
```

#### 2. `agent_mcp_runtime_stats`
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

-- Convert to TimescaleDB hypertable for efficient time-series queries
SELECT create_hypertable('agent_mcp_runtime_stats', 'period_start');
```

#### 3. `sdk_installations`
```sql
CREATE TABLE sdk_installations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    sdk_language VARCHAR(50) NOT NULL, -- 'javascript', 'python', 'go'
    sdk_version VARCHAR(50) NOT NULL,
    installed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_heartbeat_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    auto_detect_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    runtime_monitor_enabled BOOLEAN NOT NULL DEFAULT FALSE,

    UNIQUE(agent_id),
    INDEX idx_sdk_heartbeat (agent_id, last_heartbeat_at)
);
```

---

## SDK Packages

### 1. JavaScript/TypeScript SDK

**Package Name**: `@aim/sdk`
**Registry**: npm

**Installation**:
```bash
npm install @aim/sdk
```

**Usage**:
```typescript
import { AIMClient } from '@aim/sdk';

const aim = new AIMClient({
  apiUrl: 'https://aim.company.com',
  apiKey: process.env.AIM_API_KEY,
  agentId: 'my-agent-id',
  autoDetect: true,
  runtimeMonitor: false
});

// SDK automatically detects MCPs and reports
// Agent continues with normal MCP usage
import { Client } from '@modelcontextprotocol/sdk/client/index.js';
// ... rest of agent code
```

**Package Structure**:
```
@aim/sdk/
├── src/
│   ├── index.ts              # Main entry point
│   ├── client.ts             # AIMClient class
│   ├── detection/
│   │   ├── import-hook.ts    # ES module import hook
│   │   ├── require-hook.ts   # CommonJS require hook
│   │   └── client-interceptor.ts # MCP client wrapping
│   ├── monitoring/
│   │   └── runtime-monitor.ts # Tool call tracking
│   ├── reporting/
│   │   ├── queue.ts          # Event queue
│   │   ├── batch.ts          # Batch sender
│   │   └── offline-cache.ts  # Offline storage
│   └── utils/
│       ├── config.ts         # Configuration management
│       ├── logger.ts         # Logging
│       └── extractor.ts      # MCP server name extraction
├── package.json
├── tsconfig.json
└── README.md
```

**Size Target**: <500KB minified

### 2. Python SDK

**Package Name**: `aim-sdk`
**Registry**: PyPI

**Installation**:
```bash
pip install aim-sdk
```

**Usage**:
```python
from aim_sdk import AIMClient

aim = AIMClient(
    api_url="https://aim.company.com",
    api_key=os.getenv("AIM_API_KEY"),
    agent_id="my-agent-id",
    auto_detect=True,
    runtime_monitor=False
)

# SDK automatically detects MCPs and reports
# Agent continues with normal MCP usage
from mcp.client import Client
# ... rest of agent code
```

**Package Structure**:
```
aim-sdk/
├── aim_sdk/
│   ├── __init__.py
│   ├── client.py             # AIMClient class
│   ├── detection/
│   │   ├── __init__.py
│   │   ├── import_hook.py    # sys.meta_path hook
│   │   ├── client_interceptor.py # MCP client wrapping
│   │   └── stack_inspector.py # Stack inspection
│   ├── monitoring/
│   │   ├── __init__.py
│   │   └── runtime_monitor.py # Tool call tracking
│   ├── reporting/
│   │   ├── __init__.py
│   │   ├── queue.py          # Event queue
│   │   ├── batch.py          # Batch sender
│   │   └── offline_cache.py  # Offline storage
│   └── utils/
│       ├── __init__.py
│       ├── config.py         # Configuration management
│       ├── logger.py         # Logging
│       └── extractor.py      # MCP server name extraction
├── setup.py
├── pyproject.toml
└── README.md
```

**Size Target**: <300KB

### 3. Go SDK

**Package Name**: `github.com/opena2a/aim-sdk-go`
**Registry**: Go Modules

**Installation**:
```bash
go get github.com/opena2a/aim-sdk-go
```

**Usage**:
```go
import "github.com/opena2a/aim-sdk-go"

func main() {
    aim := aimsdk.NewClient(aimsdk.Config{
        APIURL:         "https://aim.company.com",
        APIKey:         os.Getenv("AIM_API_KEY"),
        AgentID:        "my-agent-id",
        AutoDetect:     true,
        RuntimeMonitor: false,
    })
    defer aim.Close()

    // SDK automatically detects MCPs and reports
    // Agent continues with normal MCP usage
}
```

**Package Structure**:
```
aim-sdk-go/
├── client.go              # AIMClient struct
├── config.go              # Configuration
├── detection/
│   ├── runtime_reflection.go # Runtime reflection
│   └── interface_wrapper.go  # Interface wrapping
├── monitoring/
│   └── runtime_monitor.go    # Tool call tracking
├── reporting/
│   ├── queue.go              # Event queue
│   ├── batch.go              # Batch sender
│   └── offline_cache.go      # Offline storage
├── utils/
│   ├── logger.go             # Logging
│   └── extractor.go          # MCP server name extraction
├── go.mod
├── go.sum
└── README.md
```

**Size Target**: <2MB compiled

---

## Performance Targets

### SDK Performance

**Startup Overhead**:
- JavaScript: <50ms
- Python: <100ms
- Go: <10ms

**Memory Footprint**:
- JavaScript: <10MB
- Python: <15MB
- Go: <5MB

**CPU Overhead**:
- All languages: <0.1% (imperceptible)

**Reporting Latency**:
- Detection event → Queue: <1ms
- Queue → API: 30s (batched)
- API → Dashboard: <100ms

**Network Efficiency**:
- Batch size: 10 events or 30s
- Compression: gzip
- API calls reduced by 95%

### Backend Performance

**API Endpoints**:
- `/detection/report`: <50ms p95
- `/detection/runtime`: <50ms p95
- `/detection/status`: <100ms p95

**Database**:
- Insert detection: <10ms
- Query agent detections: <50ms
- Aggregate runtime stats: <200ms

**Real-Time Updates**:
- WebSocket broadcast: <100ms
- Dashboard refresh: <500ms

---

## User Experience

### Agent Developer Journey

#### Step 1: Install SDK
```bash
npm install @aim/sdk
# or
pip install aim-sdk
# or
go get github.com/opena2a/aim-sdk-go
```

#### Step 2: Add 2-3 Lines of Code
```typescript
import { AIMClient } from '@aim/sdk';

const aim = new AIMClient({
  apiKey: process.env.AIM_API_KEY,
  agentId: 'my-agent-id'
});

// That's it! Continue with normal agent code
```

#### Step 3: Run Agent
```bash
npm start
# SDK automatically detects MCPs in background
```

#### Step 4: View Results in Dashboard
- Navigate to AIM dashboard
- See agent's detected MCPs
- View confidence scores
- Enable runtime monitoring if desired

**Zero Configuration** - SDK works out of the box with sensible defaults

---

## Security & Privacy

### Data Collection

**What SDK Collects**:
- MCP server names (e.g., "filesystem", "sqlite")
- Detection method (import, runtime connection)
- Confidence scores
- SDK version
- Timestamp

**What SDK DOES NOT Collect**:
- Agent source code
- MCP tool call arguments (unless runtime monitoring explicitly enabled)
- User data or PII
- Environment variables
- File contents

### Data Transmission

- HTTPS only (TLS 1.3)
- Bearer token authentication
- Rate limited (10 req/min per agent)
- No sensitive data in logs

### Open Source

- SDK is fully open source (Apache 2.0 license)
- Users can audit code
- Self-hosting supported
- No telemetry without consent

---

## Product Strategy

### Open Source (Community Edition)

**Included**:
- ✅ SDK with auto-detection (all languages)
- ✅ Basic runtime monitoring
- ✅ Up to 100 agents
- ✅ Community support
- ✅ Self-hosting

**Limitations**:
- 100 agents per organization
- 50 MCPs per agent
- Community support only
- No advanced analytics

### Premium (Enterprise Edition)

**Additional Features**:
- 🚀 Unlimited agents
- 🚀 Advanced runtime analytics
- 🚀 Security scanning via SDK
- 🚀 Compliance reporting (HIPAA, SOC 2)
- 🚀 Priority support (SLA)
- 🚀 Custom integrations
- 🚀 Dedicated account manager

**Pricing**:
- Pro: $99/month (up to 500 agents)
- Enterprise: Custom (unlimited, advanced features)

---

## How SDK Enables Future Products

### 1. SCAN (Static Security Analysis)

**Open Source**:
- SDK detects hardcoded API keys in loaded modules (memory inspection)
- Basic vulnerability detection

**Premium**:
- Deep code analysis (AST parsing in SDK)
- Advanced threat detection
- Automated remediation suggestions

### 2. DEPS (Supply Chain Security)

**Open Source**:
- SDK reports dependency tree at runtime
- Basic SBOM generation

**Premium**:
- SLSA/SALSA attestation verification
- Continuous dependency monitoring
- Vulnerability alerts

### 3. RTMN (Runtime Protection)

**Open Source**:
- Basic tool call monitoring
- Simple anomaly detection

**Premium**:
- Prompt injection detection
- Jailbreak prevention
- Real-time blocking
- ML-based threat detection

---

## Implementation Roadmap

### Phase 1: JavaScript SDK (Week 1-2)
- Core SDK architecture
- Import/require hooks
- MCP client interception
- Reporting module
- Unit tests

### Phase 2: Backend API (Week 2-3)
- Detection report endpoint
- Runtime stats endpoint
- Database schema
- WebSocket broadcasting
- Integration tests

### Phase 3: Dashboard UI (Week 3)
- Detection status display
- Confidence scores
- Real-time updates
- Performance metrics

### Phase 4: Python SDK (Week 4)
- Port JavaScript SDK to Python
- Import hooks (sys.meta_path)
- MCP client interception
- Unit tests

### Phase 5: Go SDK (Week 5)
- Port JavaScript SDK to Go
- Runtime reflection
- Interface wrapping
- Unit tests

### Phase 6: Documentation (Week 6)
- User guides
- API documentation
- SDK integration guides
- Video tutorials

### Phase 7: Testing & Launch (Week 7)
- End-to-end testing
- Performance testing
- Security audit
- Beta launch

---

## Success Metrics

### Technical Metrics
- ✅ SDK startup overhead <50ms
- ✅ Memory footprint <10MB
- ✅ CPU overhead <0.1%
- ✅ Detection accuracy >95%
- ✅ API latency <50ms p95

### Business Metrics
- ✅ 80% of agents use SDK
- ✅ 50% reduction in manual MCP registration
- ✅ 90% user satisfaction score
- ✅ Zero production incidents

### User Experience
- ✅ <5 minute integration time
- ✅ Zero configuration required
- ✅ No performance complaints
- ✅ High developer satisfaction

---

## Conclusion

The AIM SDK provides a **zero-friction** way for agents to integrate with AIM:

1. **2-line integration** - Minimal code changes
2. **Zero configuration** - Works out of the box
3. **Invisible performance** - <0.1% overhead
4. **Privacy-first** - No sensitive data collected
5. **Open source** - Fully auditable

This SDK-first approach enables:
- Automatic MCP discovery (no manual work)
- Real-time visibility (dashboard updates instantly)
- Foundation for premium products (security, compliance, analytics)
- Scalable architecture (works from 1 to 10,000 agents)

**The goal**: Make AI agent identity management as simple as `npm install @aim/sdk`.
