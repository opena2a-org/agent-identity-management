# 🚀 AIM SDK Implementation - Phases 1, 2, 4, 5 (Complete Context)

**Copy this entire prompt into a new Claude Code session to implement the remaining phases**

---

## 📋 Executive Summary

You are implementing the final 4 phases of **AIM (Agent Identity Management)**, an open-source enterprise platform that achieves the "Stripe Moment" for AI agent identity.

**Project Location**: `/Users/decimai/workspace/agent-identity-management/`

**Your Mission**: Build backend API, JavaScript SDK, Go SDK, and UI updates to enable SDK-based MCP detection across all platforms.

**Expected Outcome**: Agents can install `@aim/sdk` (JS) or `aim-sdk` (Python/Go) → SDK auto-detects MCP usage → Reports to AIM API → Dashboard shows connections.

---

## 🎯 What You're Building (High-Level)

```
┌─────────────────────────────────────────────────────────────┐
│  User's Agent (JavaScript/Python/Go)                        │
│                                                              │
│  import { AIMClient } from '@aim/sdk'  // JavaScript        │
│  from aim_sdk import AIMClient        # Python (DONE ✅)    │
│  import aimsdk "github.com/.../aim-sdk-go" // Go            │
│                                                              │
│  const aim = new AIMClient({                                │
│    apiKey: process.env.AIM_API_KEY,                         │
│    agentId: 'agent-uuid'                                    │
│  })                                                          │
│                                                              │
│  // SDK auto-detects when agent uses MCP servers           │
│  const mcp = new Client(...)  // ← SDK intercepts this     │
└─────────────────────────────────────────────────────────────┘
                      ↓ HTTPS POST
┌─────────────────────────────────────────────────────────────┐
│  AIM Platform (Backend API - YOU'RE BUILDING THIS)          │
│                                                              │
│  POST /api/v1/agents/:id/mcp-detected                      │
│    - Receives SDK detection reports                         │
│    - Updates agent.talks_to array                           │
│    - Stores detection metadata                              │
│    - Returns success/error                                  │
└─────────────────────────────────────────────────────────────┘
                      ↓
┌─────────────────────────────────────────────────────────────┐
│  AIM Dashboard (UI - YOU'RE UPDATING THIS)                  │
│                                                              │
│  Shows:                                                      │
│  • SDK-detected MCPs with badges                           │
│  • Detection method (sdk_import, sdk_connection)            │
│  • Confidence scores (90-100%)                              │
│  • Real-time updates                                        │
└─────────────────────────────────────────────────────────────┘
```

---

## ✅ What's Already Done (Phase 3 - Python SDK)

### Python SDK (100% Complete)
**Location**: `/Users/decimai/workspace/agent-identity-management/sdks/python/`

**Features**:
- ✅ Zero-config registration: `agent = register_agent("my-agent")`
- ✅ Auto-detect capabilities from imports (requests → make_api_calls)
- ✅ Auto-detect MCP servers from Claude Desktop config
- ✅ Ed25519 cryptographic verification
- ✅ OAuth token management
- ✅ 27 automated tests (100% passing)

**Key Files to Reference**:
```
sdks/python/aim_sdk/
├── client.py                    # Main AIMClient + register_agent()
├── capability_detection.py      # Auto-detect capabilities
├── detection.py                 # Auto-detect MCP servers
├── oauth.py                     # OAuth token management
└── exceptions.py                # Error handling

sdks/python/
├── test_e2e.py                  # End-to-end integration tests
├── example_auto_detection.py    # Working example (no backend)
└── README.md                    # Complete documentation
```

**Python SDK API Design** (Use as reference for JS/Go):
```python
from aim_sdk import AIMClient, register_agent

# Simple registration
agent = register_agent("my-agent", api_key="aim_abc123")

# Or with full control
client = AIMClient(
    agent_id="uuid",
    public_key="base64-key",
    private_key="base64-key",
    aim_url="http://localhost:8080"
)

# Capability auto-detection
from aim_sdk import auto_detect_capabilities
capabilities = auto_detect_capabilities()
# Returns: ["make_api_calls", "send_email", "read_files", ...]

# MCP auto-detection
from aim_sdk import auto_detect_mcps
mcps = auto_detect_mcps()
# Returns: [{"mcpServer": "filesystem", "confidence": 100, ...}]
```

---

## 🏗️ Current Backend State (What Already Exists)

### Existing Database Schema
**File**: `apps/backend/internal/domain/agent.go`

```go
type Agent struct {
    ID             uuid.UUID       `json:"id"`
    OrganizationID uuid.UUID       `json:"organizationId"`
    Name           string          `json:"name"`
    PublicKey      string          `json:"publicKey"`
    TalksTo        pq.StringArray  `json:"talksTo" gorm:"type:text[]"` // ← MCP servers
    TrustScore     float64         `json:"trustScore"`
    Status         string          `json:"status"`
    CreatedAt      time.Time       `json:"createdAt"`
    UpdatedAt      time.Time       `json:"updatedAt"`
}
```

### Existing API Endpoints
**File**: `apps/backend/cmd/server/main.go`

```go
// Already implemented:
agents.Post("/", handlers.CreateAgent)
agents.Get("/", handlers.ListAgents)
agents.Get("/:id", handlers.GetAgent)
agents.Put("/:id", handlers.UpdateAgent)
agents.Delete("/:id", handlers.DeleteAgent)

// MCP-related (already working):
agents.Post("/:id/mcp-servers/detect", handlers.AutoDetectMCPs)  // Claude config
agents.Post("/:id/mcp-servers", handlers.AddMCPToAgent)         // Manual add
agents.Delete("/:id/mcp-servers/:mcpId", handlers.RemoveMCP)    // Remove

// YOU NEED TO ADD:
agents.Post("/:id/mcp-detected", handlers.HandleSDKDetectionReport)  // ← NEW!
```

### Existing Frontend Components
**Location**: `apps/web/components/agents/`

```typescript
// Already built:
<AutoDetectButton />        // Detects from Claude Desktop config
<MCPServerSelector />       // Manual multi-select
<MCPServerList />           // Shows agent's MCPs
<AgentMCPGraph />          // Visual relationship graph

// YOU NEED TO ADD:
<DetectionMethodBadge />    // Shows sdk_import, sdk_connection, etc.
<SDKSetupGuide />          // Code snippets for SDK installation
```

---

## 🔨 Phase 1: Backend API Enhancement (START HERE)

**Estimated Time**: 2-3 hours

### Step 1.1: Database Migration

**Create**: `apps/backend/migrations/030_add_sdk_detection_support.up.sql`

```sql
-- Add detection metadata to existing agent_mcp_servers table
-- (This table links agents to MCP servers)
ALTER TABLE agent_mcp_servers ADD COLUMN IF NOT EXISTS detection_method VARCHAR(50) DEFAULT 'manual';
-- Possible values: 'manual', 'sdk_import', 'sdk_connection', 'config'

ALTER TABLE agent_mcp_servers ADD COLUMN IF NOT EXISTS confidence_score DECIMAL(5,2) DEFAULT 100.0;

ALTER TABLE agent_mcp_servers ADD COLUMN IF NOT EXISTS detected_at TIMESTAMPTZ;

ALTER TABLE agent_mcp_servers ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMPTZ;

-- Index for efficient queries
CREATE INDEX IF NOT EXISTS idx_agent_mcp_detection_method
ON agent_mcp_servers(detection_method);

-- Create audit table for SDK detection events
CREATE TABLE IF NOT EXISTS sdk_detection_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    sdk_version VARCHAR(20) NOT NULL,
    sdk_language VARCHAR(20) NOT NULL,  -- 'javascript', 'python', 'go'
    detection_method VARCHAR(50) NOT NULL,
    mcp_servers_detected JSONB NOT NULL,
    agent_metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sdk_events_agent ON sdk_detection_events(agent_id, created_at);
CREATE INDEX idx_sdk_events_org ON sdk_detection_events(organization_id, created_at);
```

**Create**: `apps/backend/migrations/030_add_sdk_detection_support.down.sql`

```sql
-- Rollback migration
DROP TABLE IF EXISTS sdk_detection_events;

DROP INDEX IF EXISTS idx_agent_mcp_detection_method;

ALTER TABLE agent_mcp_servers DROP COLUMN IF EXISTS detection_method;
ALTER TABLE agent_mcp_servers DROP COLUMN IF EXISTS confidence_score;
ALTER TABLE agent_mcp_servers DROP COLUMN IF EXISTS detected_at;
ALTER TABLE agent_mcp_servers DROP COLUMN IF EXISTS last_seen_at;
```

**Run Migration**:
```bash
cd apps/backend
psql -d aim -f migrations/030_add_sdk_detection_support.up.sql
```

---

### Step 1.2: Domain Models

**Create**: `apps/backend/internal/domain/sdk_detection.go`

```go
package domain

import (
    "time"
    "github.com/google/uuid"
)

// SDKDetectionRequest represents an SDK reporting detected MCPs
type SDKDetectionRequest struct {
    SDKVersion  string           `json:"sdkVersion"`
    SDKLanguage string           `json:"sdkLanguage"` // javascript, python, go
    DetectedMCPs []DetectedMCP   `json:"detectedMCPs"`
    AgentMetadata map[string]interface{} `json:"agentMetadata,omitempty"`
}

// DetectedMCP represents a single MCP detected by SDK
type DetectedMCP struct {
    Name            string                 `json:"name"`
    DetectionMethod string                 `json:"detectionMethod"` // sdk_import, sdk_connection
    ConfidenceScore float64                `json:"confidenceScore"` // 0-100
    Details         map[string]interface{} `json:"details,omitempty"`
}

// SDKDetectionEvent is stored in database for audit trail
type SDKDetectionEvent struct {
    ID                 uuid.UUID              `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
    AgentID            uuid.UUID              `json:"agentId" gorm:"type:uuid;not null"`
    OrganizationID     uuid.UUID              `json:"organizationId" gorm:"type:uuid;not null"`
    SDKVersion         string                 `json:"sdkVersion" gorm:"not null"`
    SDKLanguage        string                 `json:"sdkLanguage" gorm:"not null"`
    DetectionMethod    string                 `json:"detectionMethod" gorm:"not null"`
    MCPServersDetected JSONBMap               `json:"mcpServersDetected" gorm:"type:jsonb"`
    AgentMetadata      JSONBMap               `json:"agentMetadata" gorm:"type:jsonb"`
    CreatedAt          time.Time              `json:"createdAt" gorm:"autoCreateTime"`
}

// JSONBMap is a helper type for JSONB columns
type JSONBMap map[string]interface{}

// TableName specifies the table name
func (SDKDetectionEvent) TableName() string {
    return "sdk_detection_events"
}
```

**Update**: `apps/backend/internal/domain/agent_mcp_relationship.go` (if exists) or create new

```go
package domain

import (
    "time"
    "github.com/google/uuid"
)

// AgentMCPRelationship represents the many-to-many relationship
// between agents and MCP servers with detection metadata
type AgentMCPRelationship struct {
    AgentID         uuid.UUID  `json:"agentId" gorm:"type:uuid;primaryKey"`
    MCPServerName   string     `json:"mcpServerName" gorm:"primaryKey"`
    DetectionMethod string     `json:"detectionMethod" gorm:"default:manual"` // manual, sdk_import, sdk_connection, config
    ConfidenceScore float64    `json:"confidenceScore" gorm:"default:100.0"`
    DetectedAt      *time.Time `json:"detectedAt,omitempty"`
    LastSeenAt      *time.Time `json:"lastSeenAt,omitempty"`
    CreatedAt       time.Time  `json:"createdAt" gorm:"autoCreateTime"`
    UpdatedAt       time.Time  `json:"updatedAt" gorm:"autoUpdateTime"`
}

func (AgentMCPRelationship) TableName() string {
    return "agent_mcp_servers"
}
```

---

### Step 1.3: SDK Service

**Create**: `apps/backend/internal/application/sdk_service.go`

```go
package application

import (
    "context"
    "fmt"
    "time"

    "github.com/google/uuid"
    "gorm.io/gorm"

    "agent-identity-management/apps/backend/internal/domain"
    "agent-identity-management/apps/backend/internal/infrastructure/repository"
)

type SDKService struct {
    agentRepo repository.AgentRepository
    mcpRepo   repository.MCPServerRepository
    db        *gorm.DB
}

func NewSDKService(
    agentRepo repository.AgentRepository,
    mcpRepo repository.MCPServerRepository,
    db *gorm.DB,
) *SDKService {
    return &SDKService{
        agentRepo: agentRepo,
        mcpRepo:   mcpRepo,
        db:        db,
    }
}

// ProcessSDKDetectionReport handles SDK detection reports
func (s *SDKService) ProcessSDKDetectionReport(
    ctx context.Context,
    agentID uuid.UUID,
    organizationID uuid.UUID,
    req domain.SDKDetectionRequest,
) error {
    // 1. Validate agent exists and belongs to organization
    agent, err := s.agentRepo.GetByID(ctx, agentID)
    if err != nil {
        return fmt.Errorf("agent not found: %w", err)
    }

    if agent.OrganizationID != organizationID {
        return fmt.Errorf("agent does not belong to organization")
    }

    // 2. Start transaction
    return s.db.Transaction(func(tx *gorm.DB) error {
        now := time.Now()

        // 3. Process each detected MCP
        for _, detectedMCP := range req.DetectedMCPs {
            // Check if MCP server exists in system
            mcpServer, err := s.mcpRepo.GetByName(ctx, detectedMCP.Name)
            if err != nil {
                // MCP server doesn't exist - auto-register it
                mcpServer = &domain.MCPServer{
                    Name:        detectedMCP.Name,
                    Description: fmt.Sprintf("Auto-detected by SDK (%s)", req.SDKLanguage),
                    IsActive:    true,
                    TrustScore:  70.0, // Default trust score for SDK-detected MCPs
                }
                if err := s.mcpRepo.Create(ctx, mcpServer); err != nil {
                    return fmt.Errorf("failed to auto-register MCP %s: %w", detectedMCP.Name, err)
                }
            }

            // 4. Update agent_mcp_servers relationship
            var relationship domain.AgentMCPRelationship
            result := tx.Where("agent_id = ? AND mcp_server_name = ?", agentID, detectedMCP.Name).
                First(&relationship)

            if result.Error == gorm.ErrRecordNotFound {
                // New relationship - create it
                relationship = domain.AgentMCPRelationship{
                    AgentID:         agentID,
                    MCPServerName:   detectedMCP.Name,
                    DetectionMethod: detectedMCP.DetectionMethod,
                    ConfidenceScore: detectedMCP.ConfidenceScore,
                    DetectedAt:      &now,
                    LastSeenAt:      &now,
                }
                if err := tx.Create(&relationship).Error; err != nil {
                    return fmt.Errorf("failed to create relationship: %w", err)
                }
            } else if result.Error != nil {
                return result.Error
            } else {
                // Existing relationship - update it
                // Boost confidence if detected by multiple methods
                if relationship.DetectionMethod != detectedMCP.DetectionMethod {
                    relationship.ConfidenceScore = min(100.0, relationship.ConfidenceScore+5.0)
                }
                relationship.LastSeenAt = &now
                relationship.DetectionMethod = detectedMCP.DetectionMethod
                if err := tx.Save(&relationship).Error; err != nil {
                    return fmt.Errorf("failed to update relationship: %w", err)
                }
            }

            // 5. Update agent.talks_to array (for backward compatibility)
            if !contains(agent.TalksTo, detectedMCP.Name) {
                agent.TalksTo = append(agent.TalksTo, detectedMCP.Name)
            }
        }

        // 6. Save updated agent
        if err := tx.Save(&agent).Error; err != nil {
            return fmt.Errorf("failed to update agent: %w", err)
        }

        // 7. Store detection event for audit trail
        detectionEvent := domain.SDKDetectionEvent{
            AgentID:            agentID,
            OrganizationID:     organizationID,
            SDKVersion:         req.SDKVersion,
            SDKLanguage:        req.SDKLanguage,
            DetectionMethod:    req.DetectedMCPs[0].DetectionMethod, // Primary method
            MCPServersDetected: domain.JSONBMap{
                "mcps": req.DetectedMCPs,
            },
            AgentMetadata: req.AgentMetadata,
        }
        if err := tx.Create(&detectionEvent).Error; err != nil {
            return fmt.Errorf("failed to store detection event: %w", err)
        }

        return nil
    })
}

// Helper functions
func contains(slice []string, item string) bool {
    for _, s := range slice {
        if s == item {
            return true
        }
    }
    return false
}

func min(a, b float64) float64 {
    if a < b {
        return a
    }
    return b
}
```

---

### Step 1.4: HTTP Handler

**Create**: `apps/backend/internal/interfaces/http/handlers/sdk_handler.go`

```go
package handlers

import (
    "github.com/gofiber/fiber/v3"
    "github.com/google/uuid"

    "agent-identity-management/apps/backend/internal/application"
    "agent-identity-management/apps/backend/internal/domain"
)

type SDKHandler struct {
    sdkService *application.SDKService
}

func NewSDKHandler(sdkService *application.SDKService) *SDKHandler {
    return &SDKHandler{
        sdkService: sdkService,
    }
}

// HandleSDKDetectionReport handles POST /api/v1/agents/:id/mcp-detected
func (h *SDKHandler) HandleSDKDetectionReport(c fiber.Ctx) error {
    // 1. Parse agent ID from URL
    agentIDStr := c.Params("id")
    agentID, err := uuid.Parse(agentIDStr)
    if err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
            "error": "Invalid agent ID format",
        })
    }

    // 2. Get organization ID from auth context
    organizationID, ok := c.Locals("organizationId").(uuid.UUID)
    if !ok {
        return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
            "error": "Missing organization context",
        })
    }

    // 3. Parse request body
    var req domain.SDKDetectionRequest
    if err := c.Bind().JSON(&req); err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
            "error": "Invalid request body",
            "details": err.Error(),
        })
    }

    // 4. Validate request
    if len(req.DetectedMCPs) == 0 {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
            "error": "No MCPs detected in request",
        })
    }

    if req.SDKVersion == "" {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
            "error": "SDK version is required",
        })
    }

    if req.SDKLanguage == "" {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
            "error": "SDK language is required",
        })
    }

    // 5. Process detection report
    if err := h.sdkService.ProcessSDKDetectionReport(
        c.Context(),
        agentID,
        organizationID,
        req,
    ); err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "error": "Failed to process detection report",
            "details": err.Error(),
        })
    }

    // 6. Return success
    return c.Status(fiber.StatusOK).JSON(fiber.Map{
        "success": true,
        "mcpsDetected": len(req.DetectedMCPs),
        "message": fmt.Sprintf("Successfully processed %d MCP detections", len(req.DetectedMCPs)),
    })
}
```

---

### Step 1.5: Register Route

**Update**: `apps/backend/cmd/server/main.go`

```go
// Find the section where agent routes are registered
// Add this line:

// Initialize SDK handler
sdkService := application.NewSDKService(agentRepo, mcpRepo, db)
sdkHandler := handlers.NewSDKHandler(sdkService)

// Register SDK detection route (requires authentication)
agents.Post("/:id/mcp-detected", authMiddleware, sdkHandler.HandleSDKDetectionReport)
```

---

### Step 1.6: Test Backend

**Create**: `apps/backend/internal/application/sdk_service_test.go`

```go
package application

import (
    "context"
    "testing"
    "time"

    "github.com/google/uuid"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"

    "agent-identity-management/apps/backend/internal/domain"
)

// Test ProcessSDKDetectionReport with valid data
func TestSDKService_ProcessSDKDetectionReport_Success(t *testing.T) {
    // Setup
    agentID := uuid.New()
    orgID := uuid.New()

    mockAgentRepo := new(MockAgentRepository)
    mockMCPRepo := new(MockMCPServerRepository)
    mockDB := setupTestDB(t)

    service := NewSDKService(mockAgentRepo, mockMCPRepo, mockDB)

    // Mock agent exists
    mockAgentRepo.On("GetByID", mock.Anything, agentID).Return(&domain.Agent{
        ID:             agentID,
        OrganizationID: orgID,
        Name:           "test-agent",
        TalksTo:        []string{},
    }, nil)

    // Mock MCP server exists
    mockMCPRepo.On("GetByName", mock.Anything, "filesystem").Return(&domain.MCPServer{
        Name: "filesystem",
    }, nil)

    // Test
    req := domain.SDKDetectionRequest{
        SDKVersion:  "1.0.0",
        SDKLanguage: "javascript",
        DetectedMCPs: []domain.DetectedMCP{
            {
                Name:            "filesystem",
                DetectionMethod: "sdk_import",
                ConfidenceScore: 95.0,
            },
        },
    }

    err := service.ProcessSDKDetectionReport(context.Background(), agentID, orgID, req)

    // Assert
    assert.NoError(t, err)
    mockAgentRepo.AssertExpectations(t)
    mockMCPRepo.AssertExpectations(t)
}

// Add more tests for error cases, duplicate detection, confidence boosting, etc.
```

**Manual API Test**:
```bash
# Start backend
cd apps/backend
go run cmd/server/main.go

# Test endpoint (replace with actual agent ID and API key)
curl -X POST http://localhost:8080/api/v1/agents/{agent-id}/mcp-detected \
  -H "Authorization: Bearer {api-key}" \
  -H "Content-Type: application/json" \
  -d '{
    "sdkVersion": "1.0.0",
    "sdkLanguage": "javascript",
    "detectedMCPs": [
      {
        "name": "filesystem",
        "detectionMethod": "sdk_import",
        "confidenceScore": 95.0,
        "details": {"source": "import_hook"}
      }
    ],
    "agentMetadata": {
      "runtime": "node",
      "nodeVersion": "v20.0.0"
    }
  }'

# Expected response:
# {"success":true,"mcpsDetected":1,"message":"Successfully processed 1 MCP detections"}
```

---

## 🟨 Phase 2: JavaScript/TypeScript SDK

**Estimated Time**: 4-5 hours

### Step 2.1: Project Setup

```bash
mkdir -p packages/aim-sdk-js
cd packages/aim-sdk-js
npm init -y
npm install --save-dev typescript @types/node
npm install --save-dev jest @types/jest ts-jest
```

**Create**: `packages/aim-sdk-js/package.json`

```json
{
  "name": "@aim/sdk",
  "version": "1.0.0",
  "description": "AIM SDK for automatic MCP detection in AI agents (JavaScript/TypeScript)",
  "main": "dist/index.js",
  "types": "dist/index.d.ts",
  "scripts": {
    "build": "tsc",
    "test": "jest",
    "prepublishOnly": "npm run build"
  },
  "keywords": ["aim", "mcp", "agent", "detection", "identity"],
  "author": "OpenA2A",
  "license": "MIT",
  "devDependencies": {
    "@types/node": "^20.0.0",
    "typescript": "^5.0.0",
    "jest": "^29.0.0",
    "@types/jest": "^29.0.0",
    "ts-jest": "^29.0.0"
  },
  "files": [
    "dist/**/*"
  ]
}
```

**Create**: `packages/aim-sdk-js/tsconfig.json`

```json
{
  "compilerOptions": {
    "target": "ES2020",
    "module": "commonjs",
    "lib": ["ES2020"],
    "declaration": true,
    "outDir": "./dist",
    "rootDir": "./src",
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "forceConsistentCasingInFileNames": true
  },
  "include": ["src/**/*"],
  "exclude": ["node_modules", "dist", "**/*.test.ts"]
}
```

---

### Step 2.2: Core SDK Client

**Create**: `packages/aim-sdk-js/src/index.ts`

```typescript
export { AIMClient, AIMClientConfig } from './client';
export { DetectedMCP } from './types';
export { autoDetectCapabilities } from './detection/capability-detector';
export { autoDetectMCPs } from './detection/mcp-detector';
```

**Create**: `packages/aim-sdk-js/src/types.ts`

```typescript
export interface AIMClientConfig {
  apiUrl: string;
  apiKey: string;
  agentId: string;
  autoDetect?: boolean;
  detectionMethods?: ('import' | 'connection')[];
  reportInterval?: number; // milliseconds, default 10000 (10 seconds)
}

export interface DetectedMCP {
  name: string;
  detectionMethod: 'sdk_import' | 'sdk_connection';
  confidenceScore: number; // 0-100
  details?: Record<string, any>;
}

export interface SDKDetectionReport {
  sdkVersion: string;
  sdkLanguage: 'javascript';
  detectedMCPs: DetectedMCP[];
  agentMetadata?: Record<string, any>;
}

export interface DetectionMethod {
  start(): void;
  stop(): void;
  getDetections(): DetectedMCP[];
}
```

**Create**: `packages/aim-sdk-js/src/client.ts`

```typescript
import { AIMClientConfig, DetectedMCP, DetectionMethod } from './types';
import { ImportDetector } from './detection/import-detector';
import { ConnectionDetector } from './detection/connection-detector';
import { APIReporter } from './reporting/api-reporter';

export class AIMClient {
  private config: Required<AIMClientConfig>;
  private reporter: APIReporter;
  private detectors: DetectionMethod[] = [];
  private reportInterval?: NodeJS.Timeout;

  constructor(config: AIMClientConfig) {
    // Set defaults
    this.config = {
      autoDetect: true,
      detectionMethods: ['import', 'connection'],
      reportInterval: 10000, // 10 seconds
      ...config,
    };

    this.reporter = new APIReporter(
      this.config.apiUrl,
      this.config.apiKey,
      this.config.agentId
    );

    if (this.config.autoDetect) {
      this.initializeDetectors();
    }
  }

  private initializeDetectors() {
    const methods = this.config.detectionMethods;

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

    // Start periodic reporting
    this.reportInterval = setInterval(() => {
      this.reportDetections().catch(err => {
        console.error('[AIM SDK] Failed to report detections:', err);
      });
    }, this.config.reportInterval);
  }

  private async reportDetections(): Promise<void> {
    const allDetections = this.detectors.flatMap(d => d.getDetections());

    if (allDetections.length === 0) {
      return;
    }

    await this.reporter.report({
      sdkVersion: '1.0.0',
      sdkLanguage: 'javascript',
      detectedMCPs: allDetections,
      agentMetadata: {
        runtime: 'node',
        nodeVersion: process.version,
        platform: process.platform,
      },
    });
  }

  /**
   * Manually trigger detection (for testing or on-demand use)
   */
  async detect(): Promise<DetectedMCP[]> {
    const allDetections = this.detectors.flatMap(d => d.getDetections());
    return allDetections;
  }

  /**
   * Manually report a specific MCP usage
   */
  async reportMCP(name: string): Promise<void> {
    await this.reporter.report({
      sdkVersion: '1.0.0',
      sdkLanguage: 'javascript',
      detectedMCPs: [
        {
          name,
          detectionMethod: 'sdk_import',
          confidenceScore: 100.0,
        },
      ],
    });
  }

  /**
   * Clean up resources
   */
  destroy(): void {
    if (this.reportInterval) {
      clearInterval(this.reportInterval);
    }
    this.detectors.forEach(d => d.stop());
  }
}

export { AIMClientConfig };
```

---

### Step 2.3: Import Hook Detector

**Create**: `packages/aim-sdk-js/src/detection/import-detector.ts`

```typescript
import Module from 'module';
import { DetectionMethod, DetectedMCP } from '../types';

export class ImportDetector implements DetectionMethod {
  private detectedMCPs: Set<string> = new Set();
  private originalRequire?: typeof Module.prototype.require;

  start(): void {
    this.hookRequire();
  }

  stop(): void {
    if (this.originalRequire) {
      Module.prototype.require = this.originalRequire;
    }
  }

  private hookRequire(): void {
    const self = this;
    this.originalRequire = Module.prototype.require;

    // TypeScript typing workaround
    const moduleProto = Module.prototype as any;

    moduleProto.require = function (this: any, id: string) {
      // Detect MCP packages
      if (id.startsWith('@modelcontextprotocol/')) {
        // Extract MCP server name
        // Example: @modelcontextprotocol/server-filesystem → filesystem
        const mcpName = id
          .replace('@modelcontextprotocol/server-', '')
          .replace('@modelcontextprotocol/sdk', 'sdk-core');

        if (mcpName !== 'sdk-core') {
          self.detectedMCPs.add(mcpName);
        }
      }

      // Call original require
      return self.originalRequire!.apply(this, arguments as any);
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
```

---

### Step 2.4: Connection Detector (Advanced)

**Create**: `packages/aim-sdk-js/src/detection/connection-detector.ts`

```typescript
import { DetectionMethod, DetectedMCP } from '../types';

/**
 * ConnectionDetector intercepts MCP Client instantiations
 *
 * Note: This requires runtime patching of MCP SDK classes.
 * For production use, consider monitoring actual stdio/http connections.
 */
export class ConnectionDetector implements DetectionMethod {
  private detectedMCPs: Set<string> = new Set();
  private originalClientConstructor?: any;

  start(): void {
    // This is a placeholder implementation
    // In production, you would:
    // 1. Intercept MCP Client constructor
    // 2. Monitor StdioClientTransport connections
    // 3. Hook into WebSocket or HTTP connections

    // For now, we'll just detect based on import patterns
    // A full implementation would require MCP SDK integration
  }

  stop(): void {
    if (this.originalClientConstructor) {
      // Restore original constructor
    }
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

---

### Step 2.5: API Reporter

**Create**: `packages/aim-sdk-js/src/reporting/api-reporter.ts`

```typescript
import { SDKDetectionReport } from '../types';

export class APIReporter {
  private apiUrl: string;
  private apiKey: string;
  private agentId: string;
  private lastReport: Record<string, number> = {}; // MCP name -> timestamp

  constructor(apiUrl: string, apiKey: string, agentId: string) {
    this.apiUrl = apiUrl;
    this.apiKey = apiKey;
    this.agentId = agentId;
  }

  async report(data: SDKDetectionReport): Promise<void> {
    // Deduplicate: Only report if MCP not reported in last 60 seconds
    const now = Date.now();
    const newMCPs = data.detectedMCPs.filter(mcp => {
      const lastReported = this.lastReport[mcp.name];
      return !lastReported || now - lastReported > 60000; // 60 seconds
    });

    if (newMCPs.length === 0) {
      return;
    }

    try {
      const response = await fetch(
        `${this.apiUrl}/api/v1/agents/${this.agentId}/mcp-detected`,
        {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${this.apiKey}`,
          },
          body: JSON.stringify({
            ...data,
            detectedMCPs: newMCPs,
          }),
        }
      );

      if (!response.ok) {
        const errorText = await response.text();
        console.error('[AIM SDK] Failed to report detections:', response.status, errorText);
        return;
      }

      // Update last report timestamps
      newMCPs.forEach(mcp => {
        this.lastReport[mcp.name] = now;
      });

    } catch (error) {
      console.error('[AIM SDK] Failed to report detections:', error);
      // Fail silently - don't break agent execution
    }
  }
}
```

---

### Step 2.6: Capability Detection (Bonus)

**Create**: `packages/aim-sdk-js/src/detection/capability-detector.ts`

```typescript
/**
 * Auto-detect agent capabilities from imports
 * Similar to Python SDK capability detection
 */
export function autoDetectCapabilities(): string[] {
  const capabilities = new Set<string>();

  // Common package to capability mappings
  const packageMappings: Record<string, string> = {
    'axios': 'make_api_calls',
    'node-fetch': 'make_api_calls',
    'nodemailer': 'send_email',
    'pg': 'access_database',
    'mysql': 'access_database',
    'mongodb': 'access_database',
    'fs': 'read_files',
    'child_process': 'execute_code',
  };

  // Check require.cache for loaded modules
  const loadedModules = Object.keys(require.cache);

  loadedModules.forEach(modulePath => {
    Object.keys(packageMappings).forEach(packageName => {
      if (modulePath.includes(`/node_modules/${packageName}/`)) {
        capabilities.add(packageMappings[packageName]);
      }
    });
  });

  // Always include basic capabilities
  capabilities.add('read_files');
  capabilities.add('write_files');

  return Array.from(capabilities).sort();
}
```

**Create**: `packages/aim-sdk-js/src/detection/mcp-detector.ts`

```typescript
import * as fs from 'fs';
import * as path from 'path';
import * as os from 'os';

interface MCPDetection {
  mcpServer: string;
  detectionMethod: 'claude_config' | 'import';
  confidence: number;
  command?: string;
  args?: string[];
  env?: Record<string, string>;
}

/**
 * Auto-detect MCP servers from Claude Desktop config
 */
export function autoDetectMCPs(): MCPDetection[] {
  const detections: MCPDetection[] = [];

  // Try to read Claude Desktop config
  const configPath = path.join(
    os.homedir(),
    '.claude',
    'claude_desktop_config.json'
  );

  if (fs.existsSync(configPath)) {
    try {
      const configContent = fs.readFileSync(configPath, 'utf-8');
      const config = JSON.parse(configContent);

      if (config.mcpServers && typeof config.mcpServers === 'object') {
        Object.entries(config.mcpServers).forEach(([name, serverConfig]: [string, any]) => {
          detections.push({
            mcpServer: name,
            detectionMethod: 'claude_config',
            confidence: 100,
            command: serverConfig.command,
            args: serverConfig.args,
            env: serverConfig.env,
          });
        });
      }
    } catch (error) {
      console.error('[AIM SDK] Failed to parse Claude Desktop config:', error);
    }
  }

  return detections;
}
```

---

### Step 2.7: Tests

**Create**: `packages/aim-sdk-js/src/__tests__/client.test.ts`

```typescript
import { AIMClient } from '../client';

describe('AIMClient', () => {
  it('should initialize with config', () => {
    const client = new AIMClient({
      apiUrl: 'http://localhost:8080',
      apiKey: 'test-key',
      agentId: 'test-agent',
      autoDetect: false, // Disable for testing
    });

    expect(client).toBeDefined();
    client.destroy();
  });

  it('should detect MCPs manually', async () => {
    const client = new AIMClient({
      apiUrl: 'http://localhost:8080',
      apiKey: 'test-key',
      agentId: 'test-agent',
      autoDetect: true,
    });

    const detections = await client.detect();
    expect(Array.isArray(detections)).toBe(true);

    client.destroy();
  });

  // Add more tests...
});
```

---

### Step 2.8: Build and Publish

```bash
# Build
npm run build

# Test
npm test

# Publish to npm (when ready)
npm login
npm publish --access public
```

**Create**: `packages/aim-sdk-js/README.md`

```markdown
# @aim/sdk - JavaScript/TypeScript SDK

AIM SDK for automatic MCP detection in AI agents.

## Installation

```bash
npm install @aim/sdk
```

## Quick Start

```javascript
import { AIMClient } from '@aim/sdk';

const aim = new AIMClient({
  apiUrl: 'https://aim.yourcompany.com',
  apiKey: process.env.AIM_API_KEY,
  agentId: 'your-agent-id',
  autoDetect: true, // Enable auto-detection
});

// SDK will automatically detect and report MCP usage!
```

## Features

- ✅ Automatic MCP detection from imports
- ✅ Automatic reporting to AIM API
- ✅ Zero-config operation
- ✅ TypeScript support

## API

### `new AIMClient(config)`

Create a new AIM client.

### `client.detect()`

Manually trigger detection (returns array of detected MCPs).

### `client.reportMCP(name)`

Manually report a specific MCP usage.

### `client.destroy()`

Clean up resources (stop detectors, clear intervals).

## License

MIT
```

---

## 🟦 Phase 4: Go SDK

**Estimated Time**: 3-4 hours

### Step 4.1: Project Setup

```bash
mkdir -p packages/aim-sdk-go
cd packages/aim-sdk-go
go mod init github.com/opena2a/aim-sdk-go
```

**Create**: `packages/aim-sdk-go/go.mod`

```go
module github.com/opena2a/aim-sdk-go

go 1.21

require (
    // Add dependencies as needed
)
```

---

### Step 4.2: Core SDK Client

**Create**: `packages/aim-sdk-go/client.go`

```go
package aimsdk

import (
    "context"
    "time"
)

// Config holds SDK configuration
type Config struct {
    APIURL           string
    APIKey           string
    AgentID          string
    AutoDetect       bool
    DetectionMethods []string // "import", "runtime"
    ReportInterval   time.Duration
}

// Client is the main SDK client
type Client struct {
    config   Config
    reporter *APIReporter
    detectors []Detector
    stopChan chan struct{}
}

// Detector interface for detection methods
type Detector interface {
    Start() error
    Stop()
    GetDetections() []DetectedMCP
}

// NewClient creates a new AIM SDK client
func NewClient(config Config) *Client {
    // Set defaults
    if config.ReportInterval == 0 {
        config.ReportInterval = 10 * time.Second
    }
    if len(config.DetectionMethods) == 0 {
        config.DetectionMethods = []string{"runtime"}
    }

    client := &Client{
        config:    config,
        reporter:  NewAPIReporter(config.APIURL, config.APIKey, config.AgentID),
        detectors: []Detector{},
        stopChan:  make(chan struct{}),
    }

    if config.AutoDetect {
        client.initializeDetectors()
        go client.startPeriodicReporting()
    }

    return client
}

func (c *Client) initializeDetectors() {
    // Go doesn't support runtime import hooks easily
    // For now, rely on manual reporting
    // Future: Could analyze go.mod dependencies at build time
}

func (c *Client) startPeriodicReporting() {
    ticker := time.NewTicker(c.config.ReportInterval)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            c.reportDetections()
        case <-c.stopChan:
            return
        }
    }
}

func (c *Client) reportDetections() {
    var allDetections []DetectedMCP
    for _, detector := range c.detectors {
        allDetections = append(allDetections, detector.GetDetections()...)
    }

    if len(allDetections) == 0 {
        return
    }

    report := SDKDetectionReport{
        SDKVersion:  "1.0.0",
        SDKLanguage: "go",
        DetectedMCPs: allDetections,
        AgentMetadata: map[string]interface{}{
            "runtime": "go",
            "goVersion": runtime.Version(),
        },
    }

    if err := c.reporter.Report(context.Background(), report); err != nil {
        // Log error but don't fail
        fmt.Printf("[AIM SDK] Failed to report detections: %v\n", err)
    }
}

// Detect manually triggers detection
func (c *Client) Detect() []DetectedMCP {
    var allDetections []DetectedMCP
    for _, detector := range c.detectors {
        allDetections = append(allDetections, detector.GetDetections()...)
    }
    return allDetections
}

// ReportMCP manually reports a specific MCP usage
func (c *Client) ReportMCP(ctx context.Context, name string) error {
    report := SDKDetectionReport{
        SDKVersion:  "1.0.0",
        SDKLanguage: "go",
        DetectedMCPs: []DetectedMCP{
            {
                Name:            name,
                DetectionMethod: "manual",
                ConfidenceScore: 100.0,
            },
        },
    }

    return c.reporter.Report(ctx, report)
}

// Close cleans up resources
func (c *Client) Close() {
    close(c.stopChan)
    for _, detector := range c.detectors {
        detector.Stop()
    }
}
```

---

### Step 4.3: Types

**Create**: `packages/aim-sdk-go/types.go`

```go
package aimsdk

// DetectedMCP represents a detected MCP server
type DetectedMCP struct {
    Name            string                 `json:"name"`
    DetectionMethod string                 `json:"detectionMethod"`
    ConfidenceScore float64                `json:"confidenceScore"`
    Details         map[string]interface{} `json:"details,omitempty"`
}

// SDKDetectionReport is sent to AIM API
type SDKDetectionReport struct {
    SDKVersion    string                 `json:"sdkVersion"`
    SDKLanguage   string                 `json:"sdkLanguage"`
    DetectedMCPs  []DetectedMCP          `json:"detectedMCPs"`
    AgentMetadata map[string]interface{} `json:"agentMetadata,omitempty"`
}
```

---

### Step 4.4: API Reporter

**Create**: `packages/aim-sdk-go/reporter.go`

```go
package aimsdk

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

// APIReporter reports detections to AIM API
type APIReporter struct {
    apiURL    string
    apiKey    string
    agentID   string
    httpClient *http.Client
    lastReport map[string]time.Time // MCP name -> last reported time
}

// NewAPIReporter creates a new API reporter
func NewAPIReporter(apiURL, apiKey, agentID string) *APIReporter {
    return &APIReporter{
        apiURL:     apiURL,
        apiKey:     apiKey,
        agentID:    agentID,
        httpClient: &http.Client{Timeout: 10 * time.Second},
        lastReport: make(map[string]time.Time),
    }
}

// Report sends detection report to AIM API
func (r *APIReporter) Report(ctx context.Context, report SDKDetectionReport) error {
    // Deduplicate: Only report if not reported in last 60 seconds
    now := time.Now()
    var newMCPs []DetectedMCP

    for _, mcp := range report.DetectedMCPs {
        lastReported, exists := r.lastReport[mcp.Name]
        if !exists || now.Sub(lastReported) > 60*time.Second {
            newMCPs = append(newMCPs, mcp)
        }
    }

    if len(newMCPs) == 0 {
        return nil
    }

    report.DetectedMCPs = newMCPs

    // Marshal request body
    body, err := json.Marshal(report)
    if err != nil {
        return fmt.Errorf("failed to marshal request: %w", err)
    }

    // Create HTTP request
    url := fmt.Sprintf("%s/api/v1/agents/%s/mcp-detected", r.apiURL, r.agentID)
    req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
    if err != nil {
        return fmt.Errorf("failed to create request: %w", err)
    }

    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", r.apiKey))

    // Send request
    resp, err := r.httpClient.Do(req)
    if err != nil {
        return fmt.Errorf("failed to send request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
    }

    // Update last report timestamps
    for _, mcp := range newMCPs {
        r.lastReport[mcp.Name] = now
    }

    return nil
}
```

---

### Step 4.5: Example Usage

**Create**: `packages/aim-sdk-go/examples/main.go`

```go
package main

import (
    "context"
    "fmt"
    "time"

    aimsdk "github.com/opena2a/aim-sdk-go"
)

func main() {
    // Initialize AIM SDK
    client := aimsdk.NewClient(aimsdk.Config{
        APIURL:         "http://localhost:8080",
        APIKey:         "aim_test_key_12345",
        AgentID:        "your-agent-id",
        AutoDetect:     false, // Manual mode for Go
        ReportInterval: 10 * time.Second,
    })
    defer client.Close()

    fmt.Println("AIM SDK initialized")

    // Manually report MCP usage
    // In Go, detection is typically manual since we can't hook imports
    if err := client.ReportMCP(context.Background(), "filesystem"); err != nil {
        fmt.Printf("Failed to report MCP: %v\n", err)
    } else {
        fmt.Println("Successfully reported filesystem MCP usage")
    }

    // Keep running for demo
    time.Sleep(30 * time.Second)
}
```

---

### Step 4.6: Tests

**Create**: `packages/aim-sdk-go/client_test.go`

```go
package aimsdk

import (
    "testing"
    "time"
)

func TestNewClient(t *testing.T) {
    client := NewClient(Config{
        APIURL:     "http://localhost:8080",
        APIKey:     "test-key",
        AgentID:    "test-agent",
        AutoDetect: false,
    })

    if client == nil {
        t.Fatal("Expected client to be created")
    }

    client.Close()
}

func TestClient_ReportMCP(t *testing.T) {
    // This would require a mock HTTP server
    // For now, just test that it doesn't panic
    client := NewClient(Config{
        APIURL:     "http://localhost:8080",
        APIKey:     "test-key",
        AgentID:    "test-agent",
        AutoDetect: false,
    })
    defer client.Close()

    // This will fail to connect, but shouldn't panic
    ctx := context.Background()
    _ = client.ReportMCP(ctx, "test-mcp")
}
```

---

### Step 4.7: README

**Create**: `packages/aim-sdk-go/README.md`

```markdown
# aim-sdk-go

AIM SDK for Go agents.

## Installation

```bash
go get github.com/opena2a/aim-sdk-go
```

## Quick Start

```go
import aimsdk "github.com/opena2a/aim-sdk-go"

func main() {
    client := aimsdk.NewClient(aimsdk.Config{
        APIURL:  "https://aim.yourcompany.com",
        APIKey:  os.Getenv("AIM_API_KEY"),
        AgentID: "your-agent-id",
    })
    defer client.Close()

    // Manually report MCP usage
    client.ReportMCP(context.Background(), "filesystem")
}
```

## Features

- ✅ Manual MCP reporting
- ✅ Automatic periodic reporting
- ✅ Type-safe API

## Note

Unlike JavaScript and Python SDKs, Go SDK uses manual reporting due to Go's static nature.
Import detection would require build-time analysis.

## License

MIT
```

---

## 🎨 Phase 5: UI Updates

**Estimated Time**: 2-3 hours

### Step 5.1: Detection Method Badge

**Create**: `apps/web/components/agents/detection-method-badge.tsx`

```typescript
import { Badge } from '@/components/ui/badge';
import { Code, Plug, FileCode, User } from 'lucide-react';

interface DetectionMethodBadgeProps {
  method: 'sdk_import' | 'sdk_connection' | 'config' | 'manual';
  confidenceScore?: number;
}

const METHOD_CONFIG = {
  sdk_import: {
    label: 'SDK Import',
    icon: Code,
    color: 'bg-blue-500/10 text-blue-600 border-blue-200',
  },
  sdk_connection: {
    label: 'SDK Connection',
    icon: Plug,
    color: 'bg-green-500/10 text-green-600 border-green-200',
  },
  config: {
    label: 'Config File',
    icon: FileCode,
    color: 'bg-gray-500/10 text-gray-600 border-gray-200',
  },
  manual: {
    label: 'Manual',
    icon: User,
    color: 'bg-purple-500/10 text-purple-600 border-purple-200',
  },
};

export function DetectionMethodBadge({ method, confidenceScore }: DetectionMethodBadgeProps) {
  const config = METHOD_CONFIG[method];
  const Icon = config.icon;

  return (
    <Badge variant="outline" className={`${config.color} gap-1.5 font-normal`}>
      <Icon className="h-3 w-3" />
      <span>{config.label}</span>
      {confidenceScore !== undefined && (
        <span className="text-xs opacity-70 ml-0.5">
          ({confidenceScore.toFixed(0)}%)
        </span>
      )}
    </Badge>
  );
}
```

---

### Step 5.2: Update MCP Server List

**Update**: `apps/web/components/agents/mcp-server-list.tsx`

```typescript
// Add these imports
import { DetectionMethodBadge } from './detection-method-badge';

// Update the MCPServer interface (at the top of file)
interface MCPServer {
  id: string;
  name: string;
  description: string;
  isActive: boolean;
  trustScore: number;
  // NEW: Detection metadata
  detectionMethod?: 'sdk_import' | 'sdk_connection' | 'config' | 'manual';
  confidenceScore?: number;
  detectedAt?: string;
  lastSeenAt?: string;
}

// Update the table to show detection method
// Find the existing table row and add a new column:
<TableRow key={server.id}>
  <TableCell>
    <div className="flex items-center gap-2">
      <span className="font-medium">{server.name}</span>
      {server.detectionMethod && (
        <DetectionMethodBadge
          method={server.detectionMethod}
          confidenceScore={server.confidenceScore}
        />
      )}
    </div>
  </TableCell>
  <TableCell>{server.description}</TableCell>
  <TableCell>
    <Badge variant={server.isActive ? 'default' : 'secondary'}>
      {server.isActive ? 'Active' : 'Inactive'}
    </Badge>
  </TableCell>
  <TableCell>
    <Badge variant={getTrustScoreBadgeVariant(server.trustScore)}>
      {server.trustScore.toFixed(0)}%
    </Badge>
  </TableCell>
  {/* Add last seen column if detection method exists */}
  {server.detectionMethod && server.lastSeenAt && (
    <TableCell className="text-xs text-muted-foreground">
      Last seen: {new Date(server.lastSeenAt).toLocaleDateString()}
    </TableCell>
  )}
  <TableCell>
    <Button variant="ghost" size="sm" onClick={() => onRemove(server.id)}>
      <X className="h-4 w-4" />
    </Button>
  </TableCell>
</TableRow>
```

---

### Step 5.3: SDK Setup Guide Component

**Create**: `apps/web/components/agents/sdk-setup-guide.tsx`

```typescript
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Code2, Copy, CheckCircle2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { useState } from 'react';

interface SDKSetupGuideProps {
  agentId: string;
  apiKey: string;
}

export function SDKSetupGuide({ agentId, apiKey }: SDKSetupGuideProps) {
  const [copiedLang, setCopiedLang] = useState<string | null>(null);

  const copyToClipboard = (text: string, lang: string) => {
    navigator.clipboard.writeText(text);
    setCopiedLang(lang);
    setTimeout(() => setCopiedLang(null), 2000);
  };

  const apiUrl = typeof window !== 'undefined' ? window.location.origin : 'http://localhost:3000';

  const examples = {
    javascript: `npm install @aim/sdk

import { AIMClient } from '@aim/sdk';

const aim = new AIMClient({
  apiUrl: '${apiUrl}',
  apiKey: '${apiKey}',
  agentId: '${agentId}',
  autoDetect: true  // Enable auto-detection
});

// That's it! SDK will auto-detect MCP usage`,

    python: `pip install aim-sdk

from aim_sdk import register_agent

# ONE LINE - Zero configuration!
agent = register_agent(
    "${agentId.split('-')[0]}-agent",
    api_key="${apiKey}",
    aim_url="${apiUrl}"
)

# Auto-detects capabilities + MCPs automatically`,

    go: `go get github.com/opena2a/aim-sdk-go

import aimsdk "github.com/opena2a/aim-sdk-go"

func main() {
    client := aimsdk.NewClient(aimsdk.Config{
        APIURL:  "${apiUrl}",
        APIKey:  "${apiKey}",
        AgentID: "${agentId}",
    })
    defer client.Close()

    // Manually report MCP usage
    client.ReportMCP(ctx, "filesystem")
}`,
  };

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center gap-2">
          <Code2 className="h-5 w-5 text-primary" />
          <CardTitle>Auto-Detect MCPs with AIM SDK</CardTitle>
        </div>
        <CardDescription>
          Install the SDK in your agent to automatically detect and report MCP usage
        </CardDescription>
      </CardHeader>
      <CardContent>
        <Tabs defaultValue="javascript" className="w-full">
          <TabsList className="grid w-full grid-cols-3">
            <TabsTrigger value="javascript">JavaScript</TabsTrigger>
            <TabsTrigger value="python">Python</TabsTrigger>
            <TabsTrigger value="go">Go</TabsTrigger>
          </TabsList>

          {Object.entries(examples).map(([lang, code]) => (
            <TabsContent key={lang} value={lang} className="space-y-4">
              <div className="relative">
                <pre className="bg-muted p-4 rounded-lg text-sm overflow-x-auto">
                  <code>{code}</code>
                </pre>
                <Button
                  size="sm"
                  variant="ghost"
                  className="absolute top-2 right-2"
                  onClick={() => copyToClipboard(code, lang)}
                >
                  {copiedLang === lang ? (
                    <>
                      <CheckCircle2 className="h-4 w-4 mr-1 text-green-500" />
                      Copied!
                    </>
                  ) : (
                    <>
                      <Copy className="h-4 w-4 mr-1" />
                      Copy
                    </>
                  )}
                </Button>
              </div>

              <div className="text-sm text-muted-foreground space-y-1">
                <p className="font-medium">What happens automatically:</p>
                <ul className="list-disc list-inside space-y-1 ml-2">
                  <li>Detects MCP server usage from imports</li>
                  <li>Reports to AIM API every 10 seconds</li>
                  <li>Updates dashboard in real-time</li>
                  <li>Zero performance impact (&lt;0.1% CPU)</li>
                </ul>
              </div>
            </TabsContent>
          ))}
        </Tabs>

        <div className="mt-6 p-4 bg-blue-50 border border-blue-200 rounded-lg">
          <p className="text-sm text-blue-900">
            <strong>💡 Pro Tip:</strong> The SDK works automatically - just install it and run your agent.
            Check this dashboard to see detected MCPs appear in real-time!
          </p>
        </div>
      </CardContent>
    </Card>
  );
}
```

---

### Step 5.4: Update Agent Details Page

**Update**: `apps/web/app/dashboard/agents/[id]/page.tsx`

```typescript
// Add imports
import { DetectionMethodBadge } from '@/components/agents/detection-method-badge';
import { SDKSetupGuide } from '@/components/agents/sdk-setup-guide';

// Update the AgentDetailsPage component

// Add a new tab for SDK Setup
<Tabs defaultValue="details" className="space-y-6">
  <TabsList>
    <TabsTrigger value="details">Details</TabsTrigger>
    <TabsTrigger value="connections">Connections</TabsTrigger>
    <TabsTrigger value="graph">Graph</TabsTrigger>
    <TabsTrigger value="sdk">SDK Setup</TabsTrigger> {/* NEW */}
  </TabsList>

  {/* Existing tabs... */}

  {/* NEW: SDK Setup Tab */}
  <TabsContent value="sdk">
    <SDKSetupGuide
      agentId={agent.id}
      apiKey={apiKey} // You'll need to get this from your API
    />
  </TabsContent>
</Tabs>
```

---

### Step 5.5: Update API Client

**Update**: `apps/web/lib/api.ts`

```typescript
// Add this method to your API client class

/**
 * Get agent's MCP servers with detection metadata
 */
async getAgentMCPServers(agentId: string): Promise<MCPServer[]> {
  const response = await this.get(`/agents/${agentId}/mcp-servers`);
  return response.mcpServers || [];
}

// Update MCPServer interface to include detection metadata
interface MCPServer {
  id: string;
  name: string;
  description: string;
  isActive: boolean;
  trustScore: number;
  detectionMethod?: 'sdk_import' | 'sdk_connection' | 'config' | 'manual';
  confidenceScore?: number;
  detectedAt?: string;
  lastSeenAt?: string;
}
```

---

### Step 5.6: Test UI with Chrome DevTools MCP

```typescript
// Test the SDK Setup Guide component
// 1. Navigate to agent details page
mcp__chrome-devtools__navigate_page({
  url: "http://localhost:3000/dashboard/agents/{agent-id}"
})

// 2. Take snapshot
mcp__chrome-devtools__take_snapshot()

// 3. Click SDK Setup tab
mcp__chrome-devtools__click({ uid: "sdk-tab-uid" })

// 4. Verify no console errors
mcp__chrome-devtools__list_console_messages()

// 5. Take screenshot
mcp__chrome-devtools__take_screenshot()

// 6. Test copy button
mcp__chrome-devtools__click({ uid: "copy-button-uid" })

// 7. Verify "Copied!" message appears
mcp__chrome-devtools__take_screenshot()
```

---

## 🎯 Quality Standards (CRITICAL - READ THIS)

### Naming Consistency (MUST FOLLOW)

**Database (PostgreSQL)**: `snake_case`
```sql
detection_method
confidence_score
detected_at
last_seen_at
```

**Backend (Go structs)**: `PascalCase`
```go
DetectionMethod
ConfidenceScore
DetectedAt
LastSeenAt
```

**Backend (JSON tags)**: `camelCase`
```go
DetectionMethod string `json:"detectionMethod"`
ConfidenceScore float64 `json:"confidenceScore"`
DetectedAt *time.Time `json:"detectedAt"`
LastSeenAt *time.Time `json:"lastSeenAt"`
```

**Frontend (TypeScript)**: `camelCase`
```typescript
detectionMethod: string;
confidenceScore: number;
detectedAt: string;
lastSeenAt: string;
```

**CRITICAL**: JSON field names MUST match EXACTLY between backend and frontend!

---

### Testing Requirements

**Backend**:
- [ ] Unit tests for SDKService (success, errors, duplicates)
- [ ] Integration tests for API endpoint
- [ ] Test confidence boosting logic
- [ ] Test duplicate detection handling

**SDKs**:
- [ ] Unit tests for detection logic
- [ ] Integration tests with mock API
- [ ] Test auto-detection accuracy
- [ ] Test API reporting (success and failure)

**Frontend**:
- [ ] Component tests (DetectionMethodBadge, SDKSetupGuide)
- [ ] Chrome DevTools MCP testing (no console errors)
- [ ] Visual regression testing (screenshots)
- [ ] Copy-to-clipboard functionality

---

### Error Handling

**All code must**:
- ✅ Handle network failures gracefully
- ✅ Never break agent execution (SDKs fail silently)
- ✅ Provide clear, actionable error messages
- ✅ Log errors for debugging (but don't spam)

---

### Performance Targets

**Backend**:
- API response time: <100ms (p95)
- Handle 1000+ concurrent requests

**SDKs**:
- Initialization: <50ms (JS), <100ms (Python), <10ms (Go)
- Memory: <10MB (JS), <15MB (Python), <5MB (Go)
- CPU: <0.1% overhead (imperceptible)

---

## 📚 Reference Files (MUST READ)

### Python SDK (Completed - Use as Reference)

**Location**: `/Users/decimai/workspace/agent-identity-management/sdks/python/`

**Key files to study**:
```
aim_sdk/client.py           # Main client implementation
aim_sdk/detection.py        # MCP detection logic
aim_sdk/oauth.py            # OAuth token management
test_e2e.py                # End-to-end tests (EXCELLENT REFERENCE!)
README.md                  # Documentation
```

### Existing Backend Code

**Location**: `/Users/decimai/workspace/agent-identity-management/apps/backend/`

**Study these**:
```
internal/application/agent_service.go     # Service layer pattern
internal/interfaces/http/handlers/agent_handler.go  # Handler pattern
cmd/server/main.go                        # Route registration
migrations/                               # Database migration examples
```

### Frontend Components

**Location**: `/Users/decimai/workspace/agent-identity-management/apps/web/`

**Study these**:
```
components/agents/auto-detect-button.tsx  # Existing auto-detect
components/agents/mcp-server-list.tsx     # Table component
components/ui/badge.tsx                   # Badge component
lib/api.ts                                # API client
```

---

## 🚀 Getting Started Checklist

Before you begin, make sure:

1. ✅ Read this entire document carefully
2. ✅ Study the Python SDK implementation (`sdks/python/`)
3. ✅ Understand the existing backend structure
4. ✅ Review naming conventions (CRITICAL!)
5. ✅ Set up your development environment:
   ```bash
   # Backend
   cd apps/backend
   go mod download

   # Frontend
   cd apps/web
   npm install

   # Start backend
   cd apps/backend && go run cmd/server/main.go

   # Start frontend
   cd apps/web && npm run dev
   ```

---

## 🎯 Implementation Order (RECOMMENDED)

1. **Phase 1: Backend API** (2-3 hours)
   - Start here! This is the foundation for everything else.
   - Test thoroughly before moving on.

2. **Phase 2: JavaScript SDK** (4-5 hours)
   - Most commonly used language for AI agents.
   - Reference Python SDK heavily.

3. **Phase 5: UI Updates** (2-3 hours)
   - Visual feedback is important for testing.
   - Test with Chrome DevTools MCP.

4. **Phase 4: Go SDK** (3-4 hours)
   - Simplest SDK (manual reporting).
   - Can be done last.

---

## 🧪 End-to-End Testing Flow

After completing all phases, test the complete flow:

1. **Start Backend**
   ```bash
   cd apps/backend && go run cmd/server/main.go
   ```

2. **Start Frontend**
   ```bash
   cd apps/web && npm run dev
   ```

3. **Create Test Agent**
   - Register agent via UI
   - Get API key

4. **Test JavaScript SDK**
   ```bash
   cd packages/aim-sdk-js
   npm install
   # Create test script that imports @modelcontextprotocol/server-filesystem
   # Verify detection appears in dashboard
   ```

5. **Test Python SDK** (Already works!)
   ```bash
   cd sdks/python
   python test_e2e.py
   ```

6. **Test Go SDK**
   ```bash
   cd packages/aim-sdk-go
   go run examples/main.go
   # Verify manual reporting works
   ```

7. **Verify in Dashboard**
   - Navigate to agent details page
   - Check "Connections" tab shows MCPs
   - Verify detection method badges appear
   - Confirm confidence scores display correctly
   - Test "SDK Setup" tab renders correctly

---

## 📝 Success Criteria

Mark phases complete when:

### Phase 1: Backend API ✅
- [ ] Database migration runs successfully
- [ ] API endpoint accepts SDK reports
- [ ] agent.talks_to array updates correctly
- [ ] Detection metadata stored (method, confidence, timestamps)
- [ ] Duplicate detections handled (confidence boosting)
- [ ] Audit trail created (sdk_detection_events table)
- [ ] Unit tests pass (80%+ coverage)
- [ ] Manual API test successful (curl)

### Phase 2: JavaScript SDK ✅
- [ ] NPM package builds without errors
- [ ] Import hook detects @modelcontextprotocol/* packages
- [ ] API reporter sends requests correctly
- [ ] Deduplication works (60-second window)
- [ ] Auto-detection runs every 10 seconds
- [ ] Unit tests pass
- [ ] Integration test with real backend succeeds
- [ ] Published to NPM (or ready to publish)

### Phase 4: Go SDK ✅
- [ ] Go module builds without errors
- [ ] Manual reporting works
- [ ] API reporter sends requests correctly
- [ ] Client cleanup works (Close() method)
- [ ] Unit tests pass
- [ ] Example runs successfully
- [ ] Published to Go modules (or ready to publish)

### Phase 5: UI Updates ✅
- [ ] DetectionMethodBadge renders all methods correctly
- [ ] MCPServerList shows detection metadata
- [ ] SDKSetupGuide displays code for all 3 languages
- [ ] Copy-to-clipboard works
- [ ] "SDK Setup" tab appears on agent details page
- [ ] No console errors (verified with Chrome DevTools MCP)
- [ ] Visual regression tests pass (screenshots)
- [ ] Mobile responsive (tested on small screens)

---

## 🆘 Troubleshooting

### Backend Issues

**Problem**: Migration fails
**Solution**: Check PostgreSQL connection, verify syntax

**Problem**: API endpoint returns 500
**Solution**: Check logs, verify request body format matches domain.SDKDetectionRequest

**Problem**: agent.talks_to not updating
**Solution**: Check transaction logic, verify GORM save() is called

### JavaScript SDK Issues

**Problem**: Import hook not detecting
**Solution**: Verify Module.prototype.require is being patched before imports

**Problem**: API requests failing
**Solution**: Check CORS settings, verify API key format

**Problem**: TypeScript compilation errors
**Solution**: Check tsconfig.json, ensure all types are defined

### Go SDK Issues

**Problem**: Module not found
**Solution**: Run `go mod tidy`, check go.mod path

**Problem**: HTTP requests failing
**Solution**: Verify context is passed correctly, check timeout settings

### UI Issues

**Problem**: Component not rendering
**Solution**: Check import paths, verify component is exported

**Problem**: Badge colors not showing
**Solution**: Check Tailwind CSS classes are valid, run `npm run build`

**Problem**: Copy button not working
**Solution**: Verify navigator.clipboard API is available (HTTPS required)

---

## 💡 Best Practices (Learn from Python SDK)

### From Python SDK Success

1. **Graceful Degradation**
   - Python SDK falls back from OAuth to API key mode
   - Apply same pattern to JS/Go SDKs

2. **Comprehensive Testing**
   - Python SDK has 27 tests (100% passing)
   - Aim for similar coverage in JS/Go

3. **Clear Error Messages**
   - Python SDK provides actionable errors
   - Example: "aim_url is required when using API key mode"

4. **User-Friendly Examples**
   - Python SDK has 3 examples (no backend, full demo, classic)
   - Create similar examples for JS/Go

5. **Zero-Config Default**
   - Python SDK works with one line: `register_agent("my-agent")`
   - JS SDK should be equally simple

---

## 🎉 When You're Done

After completing all 4 phases:

1. **Update main README**
   - Add SDK installation instructions
   - Update architecture diagram
   - Add "Getting Started" section

2. **Create demo video** (optional but recommended)
   - Show SDK installation
   - Demonstrate auto-detection
   - Show dashboard updates in real-time

3. **Write blog post** (optional)
   - "The Stripe Moment for AI Agent Identity"
   - Technical deep-dive
   - Comparison with traditional approaches

4. **Publish packages**
   - NPM: `npm publish @aim/sdk`
   - PyPI: Already published
   - Go: Tag release on GitHub

5. **Celebrate!** 🎉
   - You've built a production-ready SDK system
   - Agents can now auto-detect MCP usage across 3 languages
   - You've achieved the "Stripe Moment" for AI agent identity!

---

**Last Updated**: October 9, 2025
**Project**: Agent Identity Management (AIM)
**Repository**: https://github.com/opena2a-org/agent-identity-management

---

**Good luck building! You've got this!** 🚀
