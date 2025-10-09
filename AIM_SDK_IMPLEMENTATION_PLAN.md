# AIM SDK Implementation Plan - MCP Auto-Detection

## Executive Summary

**Goal**: Build AIM SDKs (JavaScript, Python, Go) that agents integrate to auto-detect MCP usage and report to the existing AIM platform.

**Key Principle**: Agents install `@aim/sdk` → SDK auto-detects MCPs → Reports to AIM API → Dashboard shows connections

**What We're NOT Doing**: Building separate scanning tools, CLI utilities, or filesystem analyzers. Detection happens **inside** the agent process via SDK.

---

## Current State (What We Already Have)

### ✅ Backend (Go/Fiber) - `apps/backend/`
- Agent management API (CRUD operations)
- MCP server management API
- Agent-MCP relationships (`talks_to` field)
- Trust scoring system
- Authentication & authorization (JWT)
- Audit logging
- Database schema (PostgreSQL)

### ✅ Frontend (Next.js) - `apps/web/`
- Agent registration and management UI
- MCP server registration UI
- Agent details page with MCP connections
- Auto-detect button (currently uses Claude Desktop config)
- MCP server selector (manual assignment)
- Agent-MCP graph visualization

### ✅ Existing API Endpoints
```
POST   /api/v1/agents                              # Create agent
GET    /api/v1/agents                              # List agents
GET    /api/v1/agents/:id                          # Get agent details
PUT    /api/v1/agents/:id                          # Update agent
DELETE /api/v1/agents/:id                          # Delete agent

POST   /api/v1/mcp-servers                         # Register MCP server
GET    /api/v1/mcp-servers                         # List MCP servers

POST   /api/v1/agents/:id/mcp-servers/detect      # Auto-detect (Claude Desktop config)
POST   /api/v1/agents/:id/mcp-servers              # Add MCP to agent
DELETE /api/v1/agents/:id/mcp-servers/:mcpId      # Remove MCP from agent
```

---

## What We're Building (SDK-Based Detection)

### New Components

**1. AIM SDK Packages** (New)
- `@aim/sdk` - JavaScript/TypeScript NPM package
- `aim-sdk` - Python PyPI package
- `github.com/opena2a/aim-sdk-go` - Go module

**2. SDK Detection Logic** (New)
- Import/require hook detection
- MCP client connection interception
- Automatic reporting to AIM API

**3. Enhanced Backend Endpoints** (Update Existing)
- Enhance existing auto-detect endpoint for SDK-based reporting
- Add SDK authentication and validation

**4. Updated UI** (Enhance Existing)
- Show SDK-detected MCPs with badges
- Display detection method (SDK vs Manual)
- Real-time updates when SDK reports

---

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│  User's Agent (runs anywhere)                               │
│                                                              │
│  ┌────────────────────────────────────────────────────────┐ │
│  │  Agent Code                                            │ │
│  │                                                          │ │
│  │  import { Client } from '@modelcontextprotocol/sdk'   │ │
│  │  import { AIMClient } from '@aim/sdk'  // ← NEW       │ │
│  │                                                          │ │
│  │  const aim = new AIMClient({                          │ │
│  │    apiKey: process.env.AIM_API_KEY,                   │ │
│  │    agentId: 'agent-uuid'                              │ │
│  │  })                                                     │ │
│  │                                                          │ │
│  │  // SDK auto-detects filesystem MCP                   │ │
│  │  const mcp = new Client(...)                          │ │
│  └────────────────────────────────────────────────────────┘ │
│            ↓                                                 │
│  ┌────────────────────────────────────────────────────────┐ │
│  │  AIM SDK (embedded library)                           │ │
│  │                                                          │ │
│  │  Detection Methods:                                    │ │
│  │  ✓ Import/Require Hook (95% confidence)              │ │
│  │  ✓ Connection Interception (100% confidence)         │ │
│  │                                                          │ │
│  │  Reports to: POST /api/v1/agents/:id/mcp-detected    │ │
│  └────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
                      ↓ HTTPS POST
┌─────────────────────────────────────────────────────────────┐
│  AIM Platform (existing backend)                            │
│                                                              │
│  Endpoints:                                                  │
│  POST /api/v1/agents/:id/mcp-detected  // ← ENHANCE       │
│    - Receives SDK detection reports                         │
│    - Validates API key                                      │
│    - Updates agent.talks_to                                 │
│    - Returns success/error                                  │
│                                                              │
│  Dashboard UI:                                              │
│  - Shows SDK-detected MCPs with badges                     │
│  - Displays confidence scores                               │
│  - Real-time updates                                        │
└─────────────────────────────────────────────────────────────┘
```

---

## Implementation Phases

### Phase 1: Backend API Enhancement (Enhance Existing)

**Goal**: Update existing auto-detect endpoint to receive SDK reports

#### 1.1 Database Schema Addition
**File**: `apps/backend/migrations/030_add_sdk_detection_support.up.sql`

```sql
-- Add detection metadata to agent_mcp_servers junction table
ALTER TABLE agent_mcp_servers ADD COLUMN IF NOT EXISTS detection_method VARCHAR(50) DEFAULT 'manual';
-- Values: 'manual', 'sdk_import', 'sdk_connection', 'config'

ALTER TABLE agent_mcp_servers ADD COLUMN IF NOT EXISTS confidence_score DECIMAL(5,2) DEFAULT 100.0;

ALTER TABLE agent_mcp_servers ADD COLUMN IF NOT EXISTS detected_at TIMESTAMPTZ;

ALTER TABLE agent_mcp_servers ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMPTZ;

-- Index for querying by detection method
CREATE INDEX IF NOT EXISTS idx_agent_mcp_detection_method
ON agent_mcp_servers(detection_method);

-- Table to store SDK detection events (audit trail)
CREATE TABLE IF NOT EXISTS sdk_detection_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    sdk_version VARCHAR(20) NOT NULL,
    detection_method VARCHAR(50) NOT NULL,
    mcp_servers_detected JSONB NOT NULL,
    agent_metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    INDEX idx_sdk_events_agent (agent_id, created_at)
);
```

#### 1.2 Update Domain Models
**File**: `apps/backend/internal/domain/agent_mcp_relationship.go` (enhance existing)

```go
type AgentMCPRelationship struct {
    AgentID          uuid.UUID
    MCPServerName    string
    DetectionMethod  string    // "manual", "sdk_import", "sdk_connection", "config"
    ConfidenceScore  float64   // 0-100
    DetectedAt       *time.Time
    LastSeenAt       *time.Time
    CreatedAt        time.Time
    UpdatedAt        time.Time
}

type SDKDetectionEvent struct {
    ID                 uuid.UUID
    AgentID            uuid.UUID
    SDKVersion         string
    DetectionMethod    string
    MCPServersDetected []SDKDetectedMCP
    AgentMetadata      map[string]interface{}
    CreatedAt          time.Time
}

type SDKDetectedMCP struct {
    Name            string
    DetectionMethod string
    ConfidenceScore float64
    Details         map[string]interface{}
}
```

#### 1.3 Create SDK Service
**File**: `apps/backend/internal/application/sdk_service.go` (new)

```go
package application

type SDKService struct {
    agentRepo repository.AgentRepository
    mcpRepo   repository.MCPServerRepository
    db        *sql.DB
}

// ProcessSDKDetectionReport handles SDK detection reports
func (s *SDKService) ProcessSDKDetectionReport(ctx context.Context, req SDKDetectionRequest) error {
    // 1. Validate agent exists and API key matches
    // 2. Validate MCP servers in detection list
    // 3. Update agent.talks_to with detected MCPs
    // 4. Store detection event for audit
    // 5. Return success
}

type SDKDetectionRequest struct {
    AgentID         uuid.UUID
    SDKVersion      string
    DetectedMCPs    []SDKDetectedMCP
    AgentMetadata   map[string]interface{}
}
```

#### 1.4 Update API Handler
**File**: `apps/backend/internal/interfaces/http/handlers/agent_handler.go` (enhance existing)

```go
// Add new endpoint
func (h *AgentHandler) HandleSDKDetectionReport(c *fiber.Ctx) error {
    // POST /api/v1/agents/:id/mcp-detected
    // Request body: { sdkVersion, detectedMCPs: [...] }

    agentID, err := uuid.Parse(c.Params("id"))
    if err != nil {
        return c.Status(400).JSON(fiber.Map{"error": "Invalid agent ID"})
    }

    var req SDKDetectionRequest
    if err := c.BodyParser(&req); err != nil {
        return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
    }

    req.AgentID = agentID

    if err := h.sdkService.ProcessSDKDetectionReport(c.Context(), req); err != nil {
        return c.Status(500).JSON(fiber.Map{"error": err.Error()})
    }

    return c.JSON(fiber.Map{
        "success": true,
        "mcpsDetected": len(req.DetectedMCPs),
    })
}
```

#### 1.5 Register Route
**File**: `apps/backend/cmd/server/main.go` (update existing)

```go
// Add new route to existing agent routes
agents.Post("/:id/mcp-detected", handlers.HandleSDKDetectionReport)
```

---

### Phase 2: JavaScript/TypeScript SDK

**Goal**: Build `@aim/sdk` NPM package that agents install

#### 2.1 SDK Project Structure
```
packages/aim-sdk-js/
├── package.json
├── tsconfig.json
├── src/
│   ├── index.ts              # Main entry point
│   ├── client.ts             # AIMClient class
│   ├── detectors/
│   │   ├── import-detector.ts    # Import hook detection
│   │   └── connection-detector.ts # MCP connection interception
│   ├── reporters/
│   │   └── api-reporter.ts   # Reports to AIM API
│   └── types.ts              # TypeScript types
├── tests/
│   └── client.test.ts
└── README.md
```

#### 2.2 Main SDK Client
**File**: `packages/aim-sdk-js/src/client.ts`

```typescript
import { ImportDetector } from './detectors/import-detector';
import { ConnectionDetector } from './detectors/connection-detector';
import { APIReporter } from './reporters/api-reporter';

export interface AIMClientConfig {
  apiUrl: string;
  apiKey: string;
  agentId: string;
  autoDetect?: boolean;
  detectionMethods?: ('import' | 'connection')[];
}

export class AIMClient {
  private config: AIMClientConfig;
  private reporter: APIReporter;
  private detectors: Array<ImportDetector | ConnectionDetector>;

  constructor(config: AIMClientConfig) {
    this.config = {
      autoDetect: true,
      detectionMethods: ['import', 'connection'],
      ...config,
    };

    this.reporter = new APIReporter(config.apiUrl, config.apiKey, config.agentId);
    this.detectors = [];

    if (this.config.autoDetect) {
      this.initializeDetectors();
    }
  }

  private initializeDetectors() {
    const methods = this.config.detectionMethods!;

    if (methods.includes('import')) {
      const importDetector = new ImportDetector();
      importDetector.start();
      this.detectors.push(importDetector);
    }

    if (methods.includes('connection')) {
      const connectionDetector = new ConnectionDetector();
      connectionDetector.start();
      this.detectors.push(connectionDetector);
    }

    // Report detected MCPs every 10 seconds (debounced)
    setInterval(() => this.reportDetections(), 10000);
  }

  private async reportDetections() {
    const allDetections = this.detectors.flatMap(d => d.getDetections());

    if (allDetections.length === 0) return;

    await this.reporter.report({
      sdkVersion: '1.0.0',
      detectedMCPs: allDetections,
      agentMetadata: {
        runtime: 'node',
        nodeVersion: process.version,
      },
    });
  }

  // Manual detection trigger
  async detect(): Promise<DetectedMCP[]> {
    const allDetections = this.detectors.flatMap(d => d.getDetections());
    return allDetections;
  }
}
```

#### 2.3 Import Hook Detector
**File**: `packages/aim-sdk-js/src/detectors/import-detector.ts`

```typescript
import Module from 'module';

export class ImportDetector {
  private detectedMCPs: Set<string> = new Set();
  private originalRequire: any;

  start() {
    this.hookRequire();
  }

  private hookRequire() {
    const originalRequire = Module.prototype.require;
    const self = this;

    Module.prototype.require = function (id: string) {
      // Detect @modelcontextprotocol/* packages
      if (id.startsWith('@modelcontextprotocol/')) {
        const mcpName = id.replace('@modelcontextprotocol/server-', '');
        self.detectedMCPs.add(mcpName);
      }

      return originalRequire.apply(this, arguments);
    };
  }

  getDetections(): DetectedMCP[] {
    return Array.from(this.detectedMCPs).map(name => ({
      name,
      detectionMethod: 'sdk_import',
      confidenceScore: 95.0,
      details: { source: 'import_hook' },
    }));
  }
}

interface DetectedMCP {
  name: string;
  detectionMethod: string;
  confidenceScore: number;
  details: Record<string, any>;
}
```

#### 2.4 Connection Interceptor
**File**: `packages/aim-sdk-js/src/detectors/connection-detector.ts`

```typescript
export class ConnectionDetector {
  private detectedMCPs: Set<string> = new Set();

  start() {
    // Hook into MCP Client initialization
    this.interceptMCPClient();
  }

  private interceptMCPClient() {
    // Use proxy to intercept MCP Client connections
    // This requires access to the MCP SDK classes
    // Implementation depends on how agent initializes MCP clients
  }

  getDetections(): DetectedMCP[] {
    return Array.from(this.detectedMCPs).map(name => ({
      name,
      detectionMethod: 'sdk_connection',
      confidenceScore: 100.0,
      details: { source: 'connection_intercept' },
    }));
  }
}
```

#### 2.5 API Reporter
**File**: `packages/aim-sdk-js/src/reporters/api-reporter.ts`

```typescript
export class APIReporter {
  private apiUrl: string;
  private apiKey: string;
  private agentId: string;

  constructor(apiUrl: string, apiKey: string, agentId: string) {
    this.apiUrl = apiUrl;
    this.apiKey = apiKey;
    this.agentId = agentId;
  }

  async report(data: SDKDetectionReport): Promise<void> {
    try {
      const response = await fetch(
        `${this.apiUrl}/api/v1/agents/${this.agentId}/mcp-detected`,
        {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${this.apiKey}`,
          },
          body: JSON.stringify(data),
        }
      );

      if (!response.ok) {
        console.error('[AIM SDK] Failed to report detections:', response.statusText);
      }
    } catch (error) {
      console.error('[AIM SDK] Failed to report detections:', error);
      // Fail silently - don't break agent execution
    }
  }
}

interface SDKDetectionReport {
  sdkVersion: string;
  detectedMCPs: DetectedMCP[];
  agentMetadata: Record<string, any>;
}
```

#### 2.6 Package Configuration
**File**: `packages/aim-sdk-js/package.json`

```json
{
  "name": "@aim/sdk",
  "version": "1.0.0",
  "description": "AIM SDK for automatic MCP detection in AI agents",
  "main": "dist/index.js",
  "types": "dist/index.d.ts",
  "scripts": {
    "build": "tsc",
    "test": "jest",
    "prepublishOnly": "npm run build"
  },
  "keywords": ["aim", "mcp", "agent", "detection"],
  "author": "OpenA2A",
  "license": "MIT",
  "devDependencies": {
    "@types/node": "^20.0.0",
    "typescript": "^5.0.0",
    "jest": "^29.0.0"
  }
}
```

---

### Phase 3: Python SDK

**Goal**: Build `aim-sdk` PyPI package

#### 3.1 SDK Project Structure
```
packages/aim-sdk-py/
├── setup.py
├── pyproject.toml
├── aim_sdk/
│   ├── __init__.py
│   ├── client.py
│   ├── detectors/
│   │   ├── __init__.py
│   │   ├── import_detector.py
│   │   └── connection_detector.py
│   └── reporters/
│       ├── __init__.py
│       └── api_reporter.py
├── tests/
│   └── test_client.py
└── README.md
```

#### 3.2 Main SDK Client
**File**: `packages/aim-sdk-py/aim_sdk/client.py`

```python
from typing import List, Optional
from .detectors.import_detector import ImportDetector
from .reporters.api_reporter import APIReporter

class AIMClient:
    def __init__(
        self,
        api_url: str,
        api_key: str,
        agent_id: str,
        auto_detect: bool = True,
        detection_methods: Optional[List[str]] = None
    ):
        self.api_url = api_url
        self.api_key = api_key
        self.agent_id = agent_id
        self.auto_detect = auto_detect
        self.detection_methods = detection_methods or ['import', 'connection']

        self.reporter = APIReporter(api_url, api_key, agent_id)
        self.detectors = []

        if self.auto_detect:
            self._initialize_detectors()

    def _initialize_detectors(self):
        if 'import' in self.detection_methods:
            import_detector = ImportDetector()
            import_detector.start()
            self.detectors.append(import_detector)

        # Schedule periodic reporting (every 10 seconds)
        import threading
        threading.Timer(10.0, self._report_detections).start()

    def _report_detections(self):
        all_detections = []
        for detector in self.detectors:
            all_detections.extend(detector.get_detections())

        if all_detections:
            self.reporter.report({
                'sdkVersion': '1.0.0',
                'detectedMCPs': all_detections,
                'agentMetadata': {
                    'runtime': 'python',
                    'pythonVersion': sys.version
                }
            })

    def detect(self) -> List[dict]:
        """Manual detection trigger"""
        all_detections = []
        for detector in self.detectors:
            all_detections.extend(detector.get_detections())
        return all_detections
```

#### 3.3 Import Hook Detector
**File**: `packages/aim-sdk-py/aim_sdk/detectors/import_detector.py`

```python
import sys
from importlib.abc import MetaPathFinder, Loader

class ImportDetector(MetaPathFinder):
    def __init__(self):
        self.detected_mcps = set()

    def start(self):
        # Insert custom import hook
        sys.meta_path.insert(0, self)

    def find_module(self, fullname, path=None):
        # Detect mcp or mcp-* packages
        if fullname.startswith('mcp') or 'mcp' in fullname:
            # Extract MCP server name
            mcp_name = fullname.replace('mcp_server_', '').replace('mcp.', '')
            self.detected_mcps.add(mcp_name)
        return None  # Let default import mechanism handle it

    def get_detections(self):
        return [
            {
                'name': name,
                'detectionMethod': 'sdk_import',
                'confidenceScore': 95.0,
                'details': {'source': 'import_hook'}
            }
            for name in self.detected_mcps
        ]
```

#### 3.4 API Reporter
**File**: `packages/aim-sdk-py/aim_sdk/reporters/api_reporter.py`

```python
import requests

class APIReporter:
    def __init__(self, api_url: str, api_key: str, agent_id: str):
        self.api_url = api_url
        self.api_key = api_key
        self.agent_id = agent_id

    def report(self, data: dict):
        try:
            response = requests.post(
                f"{self.api_url}/api/v1/agents/{self.agent_id}/mcp-detected",
                json=data,
                headers={
                    'Authorization': f'Bearer {self.api_key}',
                    'Content-Type': 'application/json'
                },
                timeout=5
            )

            if not response.ok:
                print(f"[AIM SDK] Failed to report detections: {response.text}")
        except Exception as e:
            print(f"[AIM SDK] Failed to report detections: {e}")
            # Fail silently - don't break agent execution
```

---

### Phase 4: Go SDK

**Goal**: Build `github.com/opena2a/aim-sdk-go` Go module

#### 4.1 SDK Project Structure
```
packages/aim-sdk-go/
├── go.mod
├── go.sum
├── client.go
├── detectors/
│   └── import_detector.go
├── reporters/
│   └── api_reporter.go
└── examples/
    └── main.go
```

#### 4.2 Main SDK Client
**File**: `packages/aim-sdk-go/client.go`

```go
package aimsdk

import (
    "context"
    "time"
)

type AIMClient struct {
    config    Config
    reporter  *APIReporter
    detectors []Detector
}

type Config struct {
    APIURL           string
    APIKey           string
    AgentID          string
    AutoDetect       bool
    DetectionMethods []string
}

func NewClient(config Config) *AIMClient {
    client := &AIMClient{
        config:    config,
        reporter:  NewAPIReporter(config.APIURL, config.APIKey, config.AgentID),
        detectors: []Detector{},
    }

    if config.AutoDetect {
        client.initializeDetectors()
    }

    return client
}

func (c *AIMClient) initializeDetectors() {
    // Go doesn't support runtime import hooks easily
    // Detection relies on build-time analysis or manual reporting
}

func (c *AIMClient) Detect() []DetectedMCP {
    var allDetections []DetectedMCP
    for _, detector := range c.detectors {
        allDetections = append(allDetections, detector.GetDetections()...)
    }
    return allDetections
}

// Manual reporting
func (c *AIMClient) ReportMCP(name string) error {
    return c.reporter.Report(context.Background(), SDKDetectionReport{
        SDKVersion: "1.0.0",
        DetectedMCPs: []DetectedMCP{
            {
                Name:            name,
                DetectionMethod: "manual",
                ConfidenceScore: 100.0,
            },
        },
    })
}
```

---

### Phase 5: UI Updates (Enhance Existing)

**Goal**: Update existing UI to show SDK-detected MCPs

#### 5.1 Detection Method Badge Component
**File**: `apps/web/components/agents/detection-method-badge.tsx` (new)

```typescript
import { Badge } from '@/components/ui/badge';
import { Code, Plug, FileCode, User } from 'lucide-react';

interface DetectionMethodBadgeProps {
  method: 'sdk_import' | 'sdk_connection' | 'config' | 'manual';
  confidenceScore?: number;
}

export function DetectionMethodBadge({ method, confidenceScore }: DetectionMethodBadgeProps) {
  const config = {
    sdk_import: {
      label: 'SDK Import',
      icon: Code,
      color: 'bg-blue-500/10 text-blue-600',
    },
    sdk_connection: {
      label: 'SDK Connection',
      icon: Plug,
      color: 'bg-green-500/10 text-green-600',
    },
    config: {
      label: 'Config File',
      icon: FileCode,
      color: 'bg-gray-500/10 text-gray-600',
    },
    manual: {
      label: 'Manual',
      icon: User,
      color: 'bg-purple-500/10 text-purple-600',
    },
  }[method];

  const Icon = config.icon;

  return (
    <Badge variant="outline" className={`${config.color} gap-1`}>
      <Icon className="h-3 w-3" />
      {config.label}
      {confidenceScore && (
        <span className="text-xs opacity-70">({confidenceScore.toFixed(0)}%)</span>
      )}
    </Badge>
  );
}
```

#### 5.2 Update MCP Server List
**File**: `apps/web/components/agents/mcp-server-list.tsx` (enhance existing)

Add new columns to show detection method and confidence:

```typescript
// Add to existing component
<div className="flex items-center gap-2">
  <span className="font-medium">{server.name}</span>
  <DetectionMethodBadge
    method={server.detectionMethod}
    confidenceScore={server.confidenceScore}
  />
</div>
```

#### 5.3 Update Agent Details Page
**File**: `apps/web/app/dashboard/agents/[id]/page.tsx` (enhance existing)

Update interface to include detection metadata:

```typescript
interface MCPServer {
  id: string;
  name: string;
  description: string;
  isActive: boolean;
  trustScore: number;
  detectionMethod?: 'sdk_import' | 'sdk_connection' | 'config' | 'manual'; // NEW
  confidenceScore?: number; // NEW
  detectedAt?: string; // NEW
  lastSeenAt?: string; // NEW
}
```

#### 5.4 SDK Setup Guide Component
**File**: `apps/web/components/agents/sdk-setup-guide.tsx` (new)

```typescript
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';

export function SDKSetupGuide({ agentId, apiKey }: { agentId: string; apiKey: string }) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Auto-Detect MCPs with AIM SDK</CardTitle>
      </CardHeader>
      <CardContent>
        <Tabs defaultValue="javascript">
          <TabsList>
            <TabsTrigger value="javascript">JavaScript</TabsTrigger>
            <TabsTrigger value="python">Python</TabsTrigger>
            <TabsTrigger value="go">Go</TabsTrigger>
          </TabsList>

          <TabsContent value="javascript">
            <pre className="bg-muted p-4 rounded-lg">
{`npm install @aim/sdk

import { AIMClient } from '@aim/sdk';

const aim = new AIMClient({
  apiUrl: '${window.location.origin}',
  apiKey: '${apiKey}',
  agentId: '${agentId}',
  autoDetect: true
});`}
            </pre>
          </TabsContent>

          <TabsContent value="python">
            <pre className="bg-muted p-4 rounded-lg">
{`pip install aim-sdk

from aim_sdk import AIMClient

aim = AIMClient(
    api_url='${window.location.origin}',
    api_key='${apiKey}',
    agent_id='${agentId}',
    auto_detect=True
)`}
            </pre>
          </TabsContent>

          <TabsContent value="go">
            <pre className="bg-muted p-4 rounded-lg">
{`go get github.com/opena2a/aim-sdk-go

import aimsdk "github.com/opena2a/aim-sdk-go"

aim := aimsdk.NewClient(aimsdk.Config{
    APIURL:     "${window.location.origin}",
    APIKey:     "${apiKey}",
    AgentID:    "${agentId}",
    AutoDetect: true,
})`}
            </pre>
          </TabsContent>
        </Tabs>
      </CardContent>
    </Card>
  );
}
```

---

## Testing Strategy

### Phase 6: SDK Testing

#### 6.1 Unit Tests (Each SDK)
- Test import/require hook detection
- Test API reporting logic
- Test error handling (network failures)
- Test configuration validation

#### 6.2 Integration Tests
**Create test agent fixtures**:

```
test/fixtures/
├── js-agent/
│   ├── package.json
│   ├── index.js
│   └── .env
├── py-agent/
│   ├── requirements.txt
│   ├── main.py
│   └── .env
└── go-agent/
    ├── go.mod
    ├── main.go
    └── .env
```

**Test flow**:
1. Install AIM SDK in test agent
2. Configure SDK with test API key
3. Run agent (it should auto-detect MCPs)
4. Verify AIM API received detection report
5. Verify UI shows detected MCPs

#### 6.3 Backend Tests
- Test SDK detection endpoint validation
- Test duplicate detection handling
- Test API key authentication
- Test concurrent SDK reports

---

## Documentation

### Phase 7: SDK Documentation

#### 7.1 User Documentation
**File**: `docs/sdk/getting-started.md` (new)

```markdown
# AIM SDK - Getting Started

## Installation

### JavaScript/TypeScript
\`\`\`bash
npm install @aim/sdk
\`\`\`

### Python
\`\`\`bash
pip install aim-sdk
\`\`\`

### Go
\`\`\`bash
go get github.com/opena2a/aim-sdk-go
\`\`\`

## Quick Start

### 1. Get Your API Key
1. Log into AIM dashboard
2. Navigate to Settings > API Keys
3. Generate new API key for your agent

### 2. Initialize SDK

**JavaScript:**
\`\`\`javascript
import { AIMClient } from '@aim/sdk';

const aim = new AIMClient({
  apiUrl: 'https://aim.yourcompany.com',
  apiKey: process.env.AIM_API_KEY,
  agentId: 'your-agent-id',
  autoDetect: true
});
\`\`\`

**Python:**
\`\`\`python
from aim_sdk import AIMClient

aim = AIMClient(
    api_url='https://aim.yourcompany.com',
    api_key=os.getenv('AIM_API_KEY'),
    agent_id='your-agent-id',
    auto_detect=True
)
\`\`\`

### 3. Run Your Agent
That's it! The SDK will auto-detect MCPs and report to AIM.

## Configuration Options

- `autoDetect`: Enable auto-detection (default: true)
- `detectionMethods`: Array of methods to use (default: ['import', 'connection'])
- `reportInterval`: How often to report (default: 10 seconds)

## Manual Detection

If you prefer manual control:

\`\`\`javascript
const aim = new AIMClient({ autoDetect: false, ... });

// Manually report MCP usage
aim.reportMCP('filesystem');
\`\`\`
```

#### 7.2 API Documentation
**File**: `docs/api/sdk-detection-endpoint.md` (new)

```markdown
# SDK Detection Endpoint

## POST /api/v1/agents/:id/mcp-detected

Reports MCP detections from AIM SDK.

**Authentication:** Bearer token (agent API key)

**Request Body:**
\`\`\`json
{
  "sdkVersion": "1.0.0",
  "detectedMCPs": [
    {
      "name": "filesystem",
      "detectionMethod": "sdk_import",
      "confidenceScore": 95.0,
      "details": { "source": "import_hook" }
    }
  ],
  "agentMetadata": {
    "runtime": "node",
    "nodeVersion": "v20.0.0"
  }
}
\`\`\`

**Response:**
\`\`\`json
{
  "success": true,
  "mcpsDetected": 1
}
\`\`\`

**Error Responses:**
- 400: Invalid request body
- 401: Invalid API key
- 404: Agent not found
- 500: Internal server error
```

---

## Roadmap (Future Phases)

### Phase 8: Runtime Monitoring (OMITTED FOR NOW)
**Status**: Moved to roadmap, not part of initial SDK release

**Why Omitted**:
- Runtime monitoring adds complexity
- Requires agent-side instrumentation
- Performance concerns
- Privacy implications

**Future Implementation**:
- Opt-in runtime monitoring
- Track MCP tool calls
- Monitor API usage patterns
- Detect anomalies

### Phase 9: Advanced Features
- SDK analytics dashboard (most popular MCPs, adoption rates)
- SDK health monitoring (report success rates, latency)
- Automatic MCP version detection
- Conflict detection (multiple agents using same MCP)
- SDK CLI tool for testing detection locally

### Phase 10: Ecosystem Integration
- VS Code extension (shows detected MCPs in editor)
- GitHub Action (detect MCPs in CI/CD)
- Docker image scanning (detect MCPs in containerized agents)
- Kubernetes operator (auto-register agents with AIM)

---

## Performance Targets

### SDK Performance
- **Initialization**: <50ms overhead
- **Detection**: <100ms per method
- **Reporting**: Async, non-blocking (10s debounce)
- **Memory**: <10MB RSS
- **CPU**: <1% overhead

### Backend Performance
- **Endpoint response time**: <100ms p95
- **Concurrent reports**: Handle 1000+ requests/sec
- **Database writes**: Batched inserts for efficiency

---

## Success Metrics

### Technical Metrics
- ✅ SDK installed in 80%+ of registered agents
- ✅ Auto-detection accuracy >90%
- ✅ Zero agent crashes caused by SDK
- ✅ <100ms API response time
- ✅ >99.9% SDK uptime

### Business Metrics
- ✅ 50%+ reduction in manual MCP registration
- ✅ 90%+ user satisfaction with SDK
- ✅ <5% support tickets related to SDK issues

---

## Security Considerations

### SDK Security
- No sensitive data sent to AIM (only MCP names)
- API keys stored securely (environment variables)
- HTTPS-only communication
- Rate limiting on API endpoints

### Privacy
- SDK reports only MCP names, not data
- No code scanning or filesystem access
- No tracking of user behavior
- Open source (users can audit)

---

## Deployment Checklist

### Before Release
- [ ] All SDK packages published (npm, PyPI, Go modules)
- [ ] Backend endpoint deployed and tested
- [ ] UI updates deployed
- [ ] Documentation complete (user guides, API docs)
- [ ] Integration tests passing
- [ ] Security audit completed
- [ ] Performance benchmarks meet targets
- [ ] Example agents created for each SDK

### Post-Release
- [ ] Monitor SDK adoption (dashboard)
- [ ] Collect user feedback
- [ ] Track detection accuracy
- [ ] Monitor API performance
- [ ] Plan Phase 8 (runtime monitoring)

---

## Conclusion

This plan delivers **SDK-based MCP detection** that:
1. ✅ Works through existing AIM infrastructure (no new tools)
2. ✅ Requires only SDK installation (`npm install @aim/sdk`)
3. ✅ Auto-detects MCPs with 90%+ accuracy
4. ✅ Zero performance impact on agents
5. ✅ Enhances existing AIM platform (not rebuilding)
6. ✅ Provides immediate value to users

The SDK is **embedded in agent code**, not a separate scanning tool. Detection happens **at runtime**, and results are reported to the **existing AIM API**.
