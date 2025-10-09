# Comprehensive MCP Detection - Implementation Plan

## Overview

Build upon the **existing AIM platform** (Phases 1-3) to add **SDK integration** and **Direct API** detection methods, creating a complete MCP detection system.

---

## What We've Already Built (Phases 1-3)

### ✅ Backend API
- `POST /api/v1/agents/:id/mcp-servers` - Add MCPs manually
- `POST /api/v1/agents/:id/mcp-servers/detect` - Auto-detect from Claude Desktop config
- `GET /api/v1/agents/:id/mcp-servers` - Get agent's MCPs
- `DELETE /api/v1/agents/:id/mcp-servers/:mcp_id` - Remove single MCP
- `DELETE /api/v1/agents/:id/mcp-servers/bulk` - Remove multiple MCPs

### ✅ Frontend UI
- `AutoDetectButton` - One-click Claude Desktop config detection
- `MCPServerSelector` - Manual multi-select
- `MCPServerList` - View and manage connections
- `AgentMCPGraph` - Visual relationship graph

### ✅ Database
- `agents.talks_to` - JSONB array of MCP server names
- `mcp_servers` - MCP registry
- `audit_logs` - All operations tracked

---

## What We're Building (Phase 4)

Add 2 new detection methods that work **alongside** existing ones:

1. **SDK Integration** - Agents embed AIM SDK, auto-detects MCPs at runtime
2. **Direct API Calls** - Agents make HTTP POST to report MCPs manually

---

## Complete Detection Methods (4 Total)

| # | Method | Status | Confidence | User Effort | Best For |
|---|--------|--------|------------|-------------|----------|
| 1 | **Manual Registration** | ✅ Built | 100% | High | Testing, small teams |
| 2 | **Claude Desktop Config** | ✅ Built | 85% | Low | Existing Claude users |
| 3 | **SDK Integration** | 🔄 New | 95-100% | Minimal | New agents, full visibility |
| 4 | **Direct API Calls** | 🔄 New | 90-100% | Medium | Custom agents, existing infra |

---

## Architecture

### Current Architecture (Built)
```
Frontend (Next.js) ↔ Backend (Go/Fiber) ↔ PostgreSQL
      ↑                     ↑
      └─────────┬───────────┘
                │
      ┌─────────┴─────────┐
      │  Detection Sources │
      ├───────────────────┤
      │ 1. Manual UI      │ ✅ Built
      │ 2. Claude Config  │ ✅ Built
      └───────────────────┘
```

### Target Architecture (Phase 4)
```
Frontend (Next.js) ↔ Backend (Go/Fiber) ↔ PostgreSQL
      ↑                     ↑                   ↑
      └─────────┬───────────┼───────────────────┘
                │           │
      ┌─────────┴───────────┴────────┐
      │     Detection Sources        │
      ├──────────────────────────────┤
      │ 1. Manual UI           ✅    │
      │ 2. Claude Config       ✅    │
      │ 3. SDK Integration     🔄    │ ← New
      │ 4. Direct API          🔄    │ ← New
      └──────────────────────────────┘
```

---

## Phase 4 Implementation Plan

### Part 1: Backend - Detection API Endpoints

#### 1.1 Database Migrations

**File**: `apps/backend/migrations/029_create_detection_tables.up.sql`

```sql
-- Detection cache table
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

-- SDK installation tracking
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

-- Runtime statistics (optional, for premium)
CREATE TABLE agent_mcp_runtime_stats (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    mcp_server_name VARCHAR(255) NOT NULL,
    tool_name VARCHAR(255) NOT NULL,
    period_start TIMESTAMPTZ NOT NULL,
    period_end TIMESTAMPTZ NOT NULL,
    call_count INTEGER NOT NULL DEFAULT 0,
    success_count INTEGER NOT NULL DEFAULT 0,
    error_count INTEGER NOT NULL DEFAULT 0,
    latency_p50_ms INTEGER,
    latency_p95_ms INTEGER,
    latency_p99_ms INTEGER,

    INDEX idx_runtime_stats_time (agent_id, mcp_server_name, period_start)
);
```

**File**: `apps/backend/migrations/029_create_detection_tables.down.sql`

```sql
DROP TABLE IF EXISTS agent_mcp_runtime_stats;
DROP TABLE IF EXISTS sdk_installations;
DROP TABLE IF EXISTS agent_mcp_detections;
```

#### 1.2 Domain Models

**File**: `apps/backend/internal/domain/detection.go`

```go
package domain

import (
    "time"
    "github.com/google/uuid"
)

type DetectionMethod string

const (
    DetectionMethodManual         DetectionMethod = "manual"
    DetectionMethodClaudeConfig   DetectionMethod = "claude_config"
    DetectionMethodSDKImport      DetectionMethod = "sdk_import"
    DetectionMethodSDKRuntime     DetectionMethod = "sdk_runtime"
    DetectionMethodDirectAPI      DetectionMethod = "direct_api"
)

type AgentMCPDetection struct {
    ID               uuid.UUID       `json:"id"`
    AgentID          uuid.UUID       `json:"agentId"`
    MCPServerName    string          `json:"mcpServerName"`
    DetectionMethod  DetectionMethod `json:"detectionMethod"`
    ConfidenceScore  float64         `json:"confidenceScore"`
    Details          map[string]interface{} `json:"details,omitempty"`
    SDKVersion       string          `json:"sdkVersion,omitempty"`
    FirstDetectedAt  time.Time       `json:"firstDetectedAt"`
    LastSeenAt       time.Time       `json:"lastSeenAt"`
}

type SDKInstallation struct {
    ID                    uuid.UUID `json:"id"`
    AgentID               uuid.UUID `json:"agentId"`
    SDKLanguage           string    `json:"sdkLanguage"`
    SDKVersion            string    `json:"sdkVersion"`
    InstalledAt           time.Time `json:"installedAt"`
    LastHeartbeatAt       time.Time `json:"lastHeartbeatAt"`
    AutoDetectEnabled     bool      `json:"autoDetectEnabled"`
    RuntimeMonitorEnabled bool      `json:"runtimeMonitorEnabled"`
}

type RuntimeStats struct {
    ID            uuid.UUID `json:"id"`
    AgentID       uuid.UUID `json:"agentId"`
    MCPServerName string    `json:"mcpServerName"`
    ToolName      string    `json:"toolName"`
    PeriodStart   time.Time `json:"periodStart"`
    PeriodEnd     time.Time `json:"periodEnd"`
    CallCount     int       `json:"callCount"`
    SuccessCount  int       `json:"successCount"`
    ErrorCount    int       `json:"errorCount"`
    LatencyP50MS  int       `json:"latencyP50MS,omitempty"`
    LatencyP95MS  int       `json:"latencyP95MS,omitempty"`
    LatencyP99MS  int       `json:"latencyP99MS,omitempty"`
}

// Request/Response types
type DetectionReportRequest struct {
    Detections []DetectionEvent `json:"detections"`
}

type DetectionEvent struct {
    MCPServer       string                 `json:"mcpServer"`
    DetectionMethod DetectionMethod        `json:"detectionMethod"`
    Confidence      float64                `json:"confidence"`
    Details         map[string]interface{} `json:"details,omitempty"`
    SDKVersion      string                 `json:"sdkVersion,omitempty"`
    Timestamp       time.Time              `json:"timestamp"`
}

type DetectionReportResponse struct {
    Success             bool     `json:"success"`
    DetectionsProcessed int      `json:"detectionsProcessed"`
    NewMCPs             []string `json:"newMCPs"`
    ExistingMCPs        []string `json:"existingMCPs"`
    Message             string   `json:"message"`
}

type RuntimeStatsRequest struct {
    MCPServer  string                 `json:"mcpServer"`
    Stats      RuntimeStatsData       `json:"stats"`
    SDKVersion string                 `json:"sdkVersion"`
}

type RuntimeStatsData struct {
    Period    string                    `json:"period"`
    StartTime time.Time                 `json:"startTime"`
    EndTime   time.Time                 `json:"endTime"`
    ToolCalls map[string]ToolCallStats  `json:"toolCalls"`
}

type ToolCallStats struct {
    Count        int         `json:"count"`
    SuccessCount int         `json:"successCount"`
    ErrorCount   int         `json:"errorCount"`
    Latency      LatencyData `json:"latency"`
}

type LatencyData struct {
    P50 int `json:"p50"`
    P95 int `json:"p95"`
    P99 int `json:"p99"`
}

type DetectionStatusResponse struct {
    AgentID              uuid.UUID                `json:"agentId"`
    SDKVersion           string                   `json:"sdkVersion,omitempty"`
    SDKInstalled         bool                     `json:"sdkInstalled"`
    AutoDetectEnabled    bool                     `json:"autoDetectEnabled"`
    RuntimeMonitorEnabled bool                    `json:"runtimeMonitorEnabled"`
    DetectedMCPs         []DetectedMCPSummary     `json:"detectedMCPs"`
    LastReportedAt       *time.Time               `json:"lastReportedAt,omitempty"`
}

type DetectedMCPSummary struct {
    Name            string            `json:"name"`
    ConfidenceScore float64           `json:"confidenceScore"`
    DetectedBy      []DetectionMethod `json:"detectedBy"`
    FirstDetected   time.Time         `json:"firstDetected"`
    LastSeen        time.Time         `json:"lastSeen"`
    ToolCallCount   int               `json:"toolCallCount,omitempty"`
}
```

#### 1.3 Detection Service

**File**: `apps/backend/internal/application/detection_service.go`

```go
package application

import (
    "context"
    "encoding/json"
    "time"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgxpool"
    "yourproject/internal/domain"
)

type DetectionService struct {
    db *pgxpool.Pool
}

func NewDetectionService(db *pgxpool.Pool) *DetectionService {
    return &DetectionService{db: db}
}

// ReportDetections processes detection events from SDK or Direct API
func (s *DetectionService) ReportDetections(
    ctx context.Context,
    agentID uuid.UUID,
    orgID uuid.UUID,
    req *domain.DetectionReportRequest,
) (*domain.DetectionReportResponse, error) {
    // Validate agent belongs to organization
    var exists bool
    err := s.db.QueryRow(ctx,
        "SELECT EXISTS(SELECT 1 FROM agents WHERE id = $1 AND organization_id = $2)",
        agentID, orgID,
    ).Scan(&exists)
    if err != nil || !exists {
        return nil, fmt.Errorf("agent not found or unauthorized")
    }

    newMCPs := []string{}
    existingMCPs := []string{}
    processed := 0

    // Process each detection
    for _, detection := range req.Detections {
        // Store in agent_mcp_detections table
        detailsJSON, _ := json.Marshal(detection.Details)

        _, err := s.db.Exec(ctx, `
            INSERT INTO agent_mcp_detections (
                agent_id, mcp_server_name, detection_method,
                confidence_score, details, sdk_version,
                first_detected_at, last_seen_at
            ) VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
            ON CONFLICT (agent_id, mcp_server_name, detection_method)
            DO UPDATE SET
                last_seen_at = NOW(),
                confidence_score = EXCLUDED.confidence_score,
                details = EXCLUDED.details
        `, agentID, detection.MCPServer, detection.DetectionMethod,
            detection.Confidence, detailsJSON, detection.SDKVersion)

        if err != nil {
            continue // Log error but don't fail entire batch
        }

        // Check if MCP is already in agent's talks_to
        var talksTo []string
        var talksToJSON []byte
        err = s.db.QueryRow(ctx,
            "SELECT talks_to FROM agents WHERE id = $1", agentID,
        ).Scan(&talksToJSON)

        if err == nil {
            json.Unmarshal(talksToJSON, &talksTo)
        }

        // Add to talks_to if not present
        found := false
        for _, mcp := range talksTo {
            if mcp == detection.MCPServer {
                found = true
                existingMCPs = append(existingMCPs, detection.MCPServer)
                break
            }
        }

        if !found {
            talksTo = append(talksTo, detection.MCPServer)
            updatedJSON, _ := json.Marshal(talksTo)

            _, err = s.db.Exec(ctx,
                "UPDATE agents SET talks_to = $1, updated_at = NOW() WHERE id = $2",
                updatedJSON, agentID)

            if err == nil {
                newMCPs = append(newMCPs, detection.MCPServer)
            }
        }

        processed++
    }

    return &domain.DetectionReportResponse{
        Success:             true,
        DetectionsProcessed: processed,
        NewMCPs:             newMCPs,
        ExistingMCPs:        existingMCPs,
        Message:             fmt.Sprintf("Processed %d detections successfully", processed),
    }, nil
}

// ReportRuntimeStats stores runtime statistics from SDK
func (s *DetectionService) ReportRuntimeStats(
    ctx context.Context,
    agentID uuid.UUID,
    orgID uuid.UUID,
    req *domain.RuntimeStatsRequest,
) error {
    // Validate agent
    var exists bool
    err := s.db.QueryRow(ctx,
        "SELECT EXISTS(SELECT 1 FROM agents WHERE id = $1 AND organization_id = $2)",
        agentID, orgID,
    ).Scan(&exists)
    if err != nil || !exists {
        return fmt.Errorf("agent not found or unauthorized")
    }

    // Insert stats for each tool
    for toolName, stats := range req.Stats.ToolCalls {
        _, err := s.db.Exec(ctx, `
            INSERT INTO agent_mcp_runtime_stats (
                agent_id, mcp_server_name, tool_name,
                period_start, period_end,
                call_count, success_count, error_count,
                latency_p50_ms, latency_p95_ms, latency_p99_ms
            ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
        `, agentID, req.MCPServer, toolName,
            req.Stats.StartTime, req.Stats.EndTime,
            stats.Count, stats.SuccessCount, stats.ErrorCount,
            stats.Latency.P50, stats.Latency.P95, stats.Latency.P99)

        if err != nil {
            return err
        }
    }

    return nil
}

// GetDetectionStatus returns detection status for an agent
func (s *DetectionService) GetDetectionStatus(
    ctx context.Context,
    agentID uuid.UUID,
    orgID uuid.UUID,
) (*domain.DetectionStatusResponse, error) {
    // Validate agent
    var exists bool
    err := s.db.QueryRow(ctx,
        "SELECT EXISTS(SELECT 1 FROM agents WHERE id = $1 AND organization_id = $2)",
        agentID, orgID,
    ).Scan(&exists)
    if err != nil || !exists {
        return nil, fmt.Errorf("agent not found or unauthorized")
    }

    response := &domain.DetectionStatusResponse{
        AgentID:      agentID,
        SDKInstalled: false,
        DetectedMCPs: []domain.DetectedMCPSummary{},
    }

    // Check SDK installation
    var sdk domain.SDKInstallation
    err = s.db.QueryRow(ctx, `
        SELECT sdk_version, auto_detect_enabled, runtime_monitor_enabled, last_heartbeat_at
        FROM sdk_installations WHERE agent_id = $1
    `, agentID).Scan(&sdk.SDKVersion, &sdk.AutoDetectEnabled,
        &sdk.RuntimeMonitorEnabled, &sdk.LastHeartbeatAt)

    if err == nil {
        response.SDKInstalled = true
        response.SDKVersion = sdk.SDKVersion
        response.AutoDetectEnabled = sdk.AutoDetectEnabled
        response.RuntimeMonitorEnabled = sdk.RuntimeMonitorEnabled
        response.LastReportedAt = &sdk.LastHeartbeatAt
    }

    // Get detected MCPs with aggregated confidence
    rows, err := s.db.Query(ctx, `
        SELECT
            mcp_server_name,
            ARRAY_AGG(DISTINCT detection_method) as methods,
            AVG(confidence_score) as avg_confidence,
            MIN(first_detected_at) as first_detected,
            MAX(last_seen_at) as last_seen
        FROM agent_mcp_detections
        WHERE agent_id = $1
        GROUP BY mcp_server_name
    `, agentID)

    if err != nil {
        return response, nil // Return partial response
    }
    defer rows.Close()

    for rows.Next() {
        var mcp domain.DetectedMCPSummary
        var methods []string

        err := rows.Scan(&mcp.Name, &methods, &mcp.ConfidenceScore,
            &mcp.FirstDetected, &mcp.LastSeen)
        if err != nil {
            continue
        }

        // Convert methods to DetectionMethod type
        for _, m := range methods {
            mcp.DetectedBy = append(mcp.DetectedBy, domain.DetectionMethod(m))
        }

        // Boost confidence if multiple methods
        if len(mcp.DetectedBy) >= 2 {
            mcp.ConfidenceScore = min(99.0, mcp.ConfidenceScore + 10)
        }
        if len(mcp.DetectedBy) >= 3 {
            mcp.ConfidenceScore = min(99.0, mcp.ConfidenceScore + 20)
        }

        response.DetectedMCPs = append(response.DetectedMCPs, mcp)
    }

    return response, nil
}

func min(a, b float64) float64 {
    if a < b {
        return a
    }
    return b
}
```

#### 1.4 HTTP Handlers

**File**: `apps/backend/internal/interfaces/http/handlers/detection_handler.go`

```go
package handlers

import (
    "github.com/gofiber/fiber/v3"
    "github.com/google/uuid"
    "yourproject/internal/application"
    "yourproject/internal/domain"
)

type DetectionHandler struct {
    detectionService *application.DetectionService
}

func NewDetectionHandler(ds *application.DetectionService) *DetectionHandler {
    return &DetectionHandler{detectionService: ds}
}

// POST /api/v1/agents/:id/detection/report
func (h *DetectionHandler) ReportDetection(c fiber.Ctx) error {
    // Get agent ID from URL
    agentID, err := uuid.Parse(c.Params("id"))
    if err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
            "error": "Invalid agent ID",
        })
    }

    // Get organization ID from auth context
    orgID, _ := c.Locals("organization_id").(uuid.UUID)

    // Parse request body
    var req domain.DetectionReportRequest
    if err := c.Bind().JSON(&req); err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
            "error": "Invalid request body",
        })
    }

    // Process detections
    response, err := h.detectionService.ReportDetections(
        c.Context(), agentID, orgID, &req)

    if err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "error": err.Error(),
        })
    }

    return c.Status(fiber.StatusOK).JSON(response)
}

// POST /api/v1/agents/:id/detection/runtime
func (h *DetectionHandler) ReportRuntime(c fiber.Ctx) error {
    agentID, err := uuid.Parse(c.Params("id"))
    if err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
            "error": "Invalid agent ID",
        })
    }

    orgID, _ := c.Locals("organization_id").(uuid.UUID)

    var req domain.RuntimeStatsRequest
    if err := c.Bind().JSON(&req); err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
            "error": "Invalid request body",
        })
    }

    err = h.detectionService.ReportRuntimeStats(c.Context(), agentID, orgID, &req)
    if err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "error": err.Error(),
        })
    }

    return c.Status(fiber.StatusOK).JSON(fiber.Map{
        "success": true,
        "message": "Runtime stats recorded",
    })
}

// GET /api/v1/agents/:id/detection/status
func (h *DetectionHandler) GetStatus(c fiber.Ctx) error {
    agentID, err := uuid.Parse(c.Params("id"))
    if err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
            "error": "Invalid agent ID",
        })
    }

    orgID, _ := c.Locals("organization_id").(uuid.UUID)

    status, err := h.detectionService.GetDetectionStatus(c.Context(), agentID, orgID)
    if err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "error": err.Error(),
        })
    }

    return c.Status(fiber.StatusOK).JSON(status)
}
```

#### 1.5 Route Registration

**File**: `apps/backend/cmd/server/main.go` (add to existing routes)

```go
// Detection endpoints (new)
detectionHandler := handlers.NewDetectionHandler(detectionService)
apiV1.Post("/agents/:id/detection/report", authMiddleware, detectionHandler.ReportDetection)
apiV1.Post("/agents/:id/detection/runtime", authMiddleware, detectionHandler.ReportRuntime)
apiV1.Get("/agents/:id/detection/status", authMiddleware, detectionHandler.GetStatus)
```

---

### Part 2: SDK Implementation

#### 2.1 JavaScript/TypeScript SDK

**Package**: `@aim/sdk` (npm)

**Repository Structure**:
```
aim-sdk-js/
├── src/
│   ├── index.ts                  # Main entry
│   ├── client.ts                 # AIMClient class
│   ├── detection/
│   │   ├── import-hook.ts        # ES module hook
│   │   ├── require-hook.ts       # CommonJS hook
│   │   └── client-interceptor.ts # MCP client wrapping
│   ├── reporting/
│   │   ├── queue.ts              # Event queue
│   │   └── batch.ts              # Batch sender
│   └── utils/
│       ├── config.ts             # Configuration
│       └── logger.ts             # Logging
├── package.json
├── tsconfig.json
├── README.md
└── examples/
    └── basic-usage.ts
```

**Example Usage**:
```typescript
import { AIMClient } from '@aim/sdk';

// Initialize SDK
const aim = new AIMClient({
  apiKey: process.env.AIM_API_KEY,
  agentId: 'my-agent-id'
});

// SDK automatically detects this import
import { Client } from '@modelcontextprotocol/sdk/client/index.js';

// Agent continues normally...
```

#### 2.2 Python SDK

**Package**: `aim-sdk` (PyPI)

**Repository Structure**:
```
aim-sdk-python/
├── aim_sdk/
│   ├── __init__.py
│   ├── client.py                 # AIMClient class
│   ├── detection/
│   │   ├── __init__.py
│   │   ├── import_hook.py        # sys.meta_path hook
│   │   └── client_interceptor.py # MCP client wrapping
│   ├── reporting/
│   │   ├── __init__.py
│   │   ├── queue.py              # Event queue
│   │   └── batch.py              # Batch sender
│   └── utils/
│       ├── __init__.py
│       ├── config.py             # Configuration
│       └── logger.py             # Logging
├── setup.py
├── pyproject.toml
├── README.md
└── examples/
    └── basic_usage.py
```

**Example Usage**:
```python
from aim_sdk import AIMClient

# Initialize SDK
aim = AIMClient(
    api_key=os.getenv("AIM_API_KEY"),
    agent_id="my-agent-id"
)

# SDK automatically detects this import
from mcp.client import Client

# Agent continues normally...
```

#### 2.3 Go SDK

**Package**: `github.com/opena2a/aim-sdk-go`

**Repository Structure**:
```
aim-sdk-go/
├── client.go                     # AIMClient struct
├── config.go                     # Configuration
├── detection/
│   └── runtime_reflection.go    # Runtime reflection
├── reporting/
│   ├── queue.go                  # Event queue
│   └── batch.go                  # Batch sender
├── utils/
│   └── logger.go                 # Logging
├── go.mod
├── go.sum
├── README.md
└── examples/
    └── basic_usage.go
```

**Example Usage**:
```go
import "github.com/opena2a/aim-sdk-go"

func main() {
    aim := aimsdk.NewClient(aimsdk.Config{
        APIKey:  os.Getenv("AIM_API_KEY"),
        AgentID: "my-agent-id",
    })
    defer aim.Close()

    // Agent continues normally...
}
```

---

### Part 3: Frontend UI Updates

#### 3.1 Detection Status Component

**File**: `apps/web/components/agents/detection-status.tsx`

```tsx
'use client'

import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Check, X } from 'lucide-react'

interface DetectionStatusProps {
  agentId: string
  status: {
    sdkInstalled: boolean
    sdkVersion?: string
    autoDetectEnabled: boolean
    detectedMCPs: Array<{
      name: string
      confidenceScore: number
      detectedBy: string[]
      lastSeen: string
    }>
  }
}

export function DetectionStatus({ agentId, status }: DetectionStatusProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Detection Status</CardTitle>
      </CardHeader>
      <CardContent>
        <div className="space-y-4">
          {/* SDK Status */}
          <div className="flex items-center gap-2">
            {status.sdkInstalled ? (
              <Check className="h-5 w-5 text-green-600" />
            ) : (
              <X className="h-5 w-5 text-red-600" />
            )}
            <span>
              SDK {status.sdkInstalled ? 'Installed' : 'Not Installed'}
              {status.sdkVersion && (
                <span className="text-muted-foreground ml-2">v{status.sdkVersion}</span>
              )}
            </span>
          </div>

          {/* Detected MCPs */}
          <div className="space-y-2">
            <h4 className="font-medium">Detected MCPs ({status.detectedMCPs.length})</h4>
            {status.detectedMCPs.map((mcp) => (
              <div key={mcp.name} className="flex items-center justify-between p-2 border rounded">
                <span>{mcp.name}</span>
                <div className="flex items-center gap-2">
                  <Badge variant="secondary">
                    {mcp.confidenceScore.toFixed(0)}% confidence
                  </Badge>
                  <div className="text-xs text-muted-foreground">
                    {mcp.detectedBy.join(', ')}
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>
      </CardContent>
    </Card>
  )
}
```

#### 3.2 Method Badge Component

**File**: `apps/web/components/agents/detection-method-badge.tsx`

```tsx
import { Badge } from '@/components/ui/badge'
import { Code, Package, Activity, User } from 'lucide-react'

interface DetectionMethodBadgeProps {
  method: string
  confidence: number
}

export function DetectionMethodBadge({ method, confidence }: DetectionMethodBadgeProps) {
  const getIcon = () => {
    switch (method) {
      case 'sdk_import': return <Code className="h-3 w-3" />
      case 'sdk_runtime': return <Activity className="h-3 w-3" />
      case 'claude_config': return <Package className="h-3 w-3" />
      case 'manual': return <User className="h-3 w-3" />
      default: return null
    }
  }

  const getColor = () => {
    if (confidence >= 95) return 'bg-green-500/10 text-green-600'
    if (confidence >= 85) return 'bg-yellow-500/10 text-yellow-600'
    return 'bg-gray-500/10 text-gray-600'
  }

  return (
    <Badge className={`flex items-center gap-1 ${getColor()}`}>
      {getIcon()}
      {method.replace('_', ' ')}
    </Badge>
  )
}
```

---

## Testing Strategy

### Backend Tests
```bash
go test ./internal/application/detection_service_test.go
go test ./internal/interfaces/http/handlers/detection_handler_test.go
```

### SDK Tests
```bash
# JavaScript
npm test

# Python
pytest tests/

# Go
go test ./...
```

### Integration Tests
1. Agent with SDK reports detections
2. Backend stores in database
3. Frontend displays updated status
4. Multiple detection methods boost confidence

---

## Documentation

### User Documentation
- SDK integration guides (JS, Python, Go)
- Direct API usage examples
- Detection method comparison
- Troubleshooting guide

### Developer Documentation
- API endpoint specifications
- Database schema reference
- SDK architecture
- Contributing guide

---

## Success Criteria

- ✅ 3 new API endpoints working
- ✅ 3 new database tables created
- ✅ 3 SDK packages published (npm, PyPI, Go modules)
- ✅ Frontend displays detection status
- ✅ Multiple detection methods work together
- ✅ Confidence boosting implemented
- ✅ 100% backward compatibility (existing features work)

---

## Timeline

**Week 1**: Backend API + Database
**Week 2**: JavaScript SDK
**Week 3**: Python & Go SDKs
**Week 4**: Frontend UI + Testing

---

**Last Updated**: October 9, 2025
**Next Step**: Implement backend detection API (Part 1)
