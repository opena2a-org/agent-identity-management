# Agent-MCP "talks_to" Relationship Architecture

## 🎯 Vision: Stripe for AI Agent Security
**Make AIM so frictionless that developers don't even think about security - it just works.**

Like Stripe revolutionized payments by abstracting complexity, AIM will revolutionize AI agent identity management by making security **invisible and automatic**.

---

## 🧠 Current State Analysis

### What Exists
1. **Database**: `agents.talks_to` JSONB column storing array of MCP server names/IDs
2. **Migration**: `020_add_talks_to_column.up.sql` with GIN index for performance
3. **Domain Model**: `Agent.TalksTo []string` field
4. **Partial Implementation**: Some handlers check talks_to, but no CRUD endpoints

### What's Missing
1. ❌ **No API endpoints** for managing relationships (add/remove MCP servers from agent's talks_to)
2. ❌ **No UI** for manual mapping
3. ❌ **No auto-detection** mechanism
4. ❌ **No SDK wrapper** for frictionless integration
5. ❌ **No CLI commands** for relationship management
6. ❌ **No visualization** of agent-MCP relationships

---

## 🏗️ Architecture Design

### Option 1: Simple JSONB Array (Current - MVP)
**Pros**:
- ✅ Already implemented
- ✅ Fast queries with GIN index
- ✅ Simple to understand
- ✅ No join queries needed

**Cons**:
- ❌ No referential integrity
- ❌ Hard to query "which agents talk to this MCP?"
- ❌ No metadata (when added, who added, etc.)

**Recommendation**: **Keep for MVP**, extend with proper relationship table later for Enterprise features.

### Option 2: Proper Relationship Table (Future - Enterprise)
```sql
CREATE TABLE agent_mcp_relationships (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    mcp_server_id UUID NOT NULL REFERENCES mcp_servers(id) ON DELETE CASCADE,
    detected_method VARCHAR(50) NOT NULL, -- 'manual', 'auto_sdk', 'auto_config', 'cli'
    confidence_score DECIMAL(5,2) DEFAULT 100.0, -- 0-100 confidence of detection
    added_by UUID REFERENCES users(id), -- Who added this relationship
    added_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_verified_at TIMESTAMPTZ, -- Last time agent actually used this MCP
    usage_count INTEGER DEFAULT 0, -- How many times agent called this MCP
    metadata JSONB DEFAULT '{}'::JSONB, -- Additional context
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(agent_id, mcp_server_id)
);

CREATE INDEX idx_agent_mcp_agent ON agent_mcp_relationships(agent_id);
CREATE INDEX idx_agent_mcp_server ON agent_mcp_relationships(mcp_server_id);
CREATE INDEX idx_agent_mcp_active ON agent_mcp_relationships(is_active) WHERE is_active = true;
```

---

## 🚀 Implementation Phases

### Phase 1: Backend API Endpoints (This PR)
**Goal**: Enable manual mapping via API

#### New Endpoints
```go
// Agent → MCP relationship management
PUT    /api/v1/agents/:id/mcp-servers              // Add MCP servers to agent's talks_to
DELETE /api/v1/agents/:id/mcp-servers/:mcp_id      // Remove MCP server from agent's talks_to
GET    /api/v1/agents/:id/mcp-servers              // List MCP servers this agent talks to (with details)

// MCP → Agent relationship queries
GET    /api/v1/mcp-servers/:id/agents              // List agents that talk to this MCP (already exists)

// Bulk operations
POST   /api/v1/agents/:id/mcp-servers/bulk         // Add multiple MCP servers at once
DELETE /api/v1/agents/:id/mcp-servers/bulk         // Remove multiple MCP servers at once

// Auto-detection (future)
POST   /api/v1/agents/:id/mcp-servers/detect       // Trigger auto-detection for this agent
POST   /api/v1/agents/detect-all                   // Bulk detect for all agents (admin only)
```

#### Request/Response Formats
```json
// PUT /api/v1/agents/:id/mcp-servers
{
  "mcp_server_ids": ["uuid1", "uuid2"],
  "detected_method": "manual",
  "metadata": {
    "added_via": "web_ui",
    "notes": "Production MCP servers"
  }
}

// Response
{
  "message": "Successfully added 2 MCP servers",
  "talks_to": ["mcp-name-1", "mcp-name-2"],
  "added_servers": [
    {
      "id": "uuid1",
      "name": "mcp-name-1",
      "url": "https://mcp1.example.com"
    }
  ]
}

// GET /api/v1/agents/:id/mcp-servers
{
  "agent_id": "agent-uuid",
  "agent_name": "my-agent",
  "mcp_servers": [
    {
      "id": "uuid1",
      "name": "mcp-name-1",
      "description": "Production MCP",
      "url": "https://mcp1.example.com",
      "status": "verified",
      "trust_score": 85.5,
      "added_at": "2025-10-09T12:00:00Z",
      "detected_method": "manual"
    }
  ],
  "total": 2
}
```

### Phase 2: Auto-Detection Mechanism (This PR)
**Goal**: Automatically detect MCP servers from Claude Desktop config

#### Detection Strategy
```typescript
// 1. Parse Claude Desktop config
const configPath = '~/Library/Application Support/Claude/claude_desktop_config.json'
const config = JSON.parse(fs.readFileSync(configPath, 'utf8'))

// 2. Extract MCP servers
const mcpServers = config.mcpServers || {}
// Example:
// {
//   "filesystem": { "command": "npx", "args": ["-y", "@modelcontextprotocol/server-filesystem"] },
//   "github": { "command": "npx", "args": ["-y", "@modelcontextprotocol/server-github"] }
// }

// 3. Auto-register MCP servers if not exist
for (const [name, config] of Object.entries(mcpServers)) {
  await aim.registerMCPServer({
    name: name,
    command: config.command,
    args: config.args,
    detected_method: 'auto_config'
  })
}

// 4. Auto-map to current agent
await aim.addMCPServersToAgent(agentId, detectedServerIds, {
  detected_method: 'auto_config',
  confidence: 95.0
})
```

#### Backend Endpoint
```go
// POST /api/v1/agents/:id/mcp-servers/detect
type DetectMCPServersRequest struct {
    ConfigPath      *string            `json:"config_path,omitempty"`      // Override config path
    ConfigContent   *string            `json:"config_content,omitempty"`   // Provide config directly
    AutoRegister    bool               `json:"auto_register"`              // Auto-register new MCPs
    DryRun          bool               `json:"dry_run"`                    // Preview without changes
}

type DetectMCPServersResponse struct {
    DetectedServers []DetectedMCPServer `json:"detected_servers"`
    NewServers      []DetectedMCPServer `json:"new_servers"`      // Not yet registered
    ExistingServers []DetectedMCPServer `json:"existing_servers"` // Already registered
    AddedToAgent    []string            `json:"added_to_agent"`   // Server IDs added to agent
    Message         string              `json:"message"`
}

type DetectedMCPServer struct {
    Name        string `json:"name"`
    Command     string `json:"command"`
    Args        []string `json:"args"`
    IsNew       bool   `json:"is_new"`       // Not yet in AIM
    ServerID    *string `json:"server_id"`    // If already registered
    Confidence  float64 `json:"confidence"`   // Detection confidence
}
```

### Phase 3: Frontend UI Components (This PR)
**Goal**: Beautiful, intuitive mapping interface

#### Component 1: MCP Server Selector (Agent Details)
```tsx
// apps/web/components/agents/mcp-server-selector.tsx
<MCPServerSelector
  agentId={agent.id}
  currentServers={agent.talks_to || []}
  onUpdate={handleUpdate}
/>

// Features:
// - Multi-select dropdown with search
// - Shows MCP server status (verified, pending, etc.)
// - Live filtering by name/URL
// - Bulk add/remove
// - Auto-complete suggestions
```

#### Component 2: Relationship Visualization
```tsx
// apps/web/components/agents/agent-mcp-graph.tsx
<AgentMCPGraph
  agentId={agent.id}
  mcpServers={mcpServers}
/>

// Features:
// - Visual graph showing agent → MCP connections
// - Color-coded by trust score
// - Click to navigate to MCP details
// - Shows detection method (manual, auto, CLI)
```

#### Component 3: Auto-Detection UI
```tsx
// apps/web/components/agents/auto-detect-button.tsx
<AutoDetectButton
  agentId={agent.id}
  onDetect={handleDetect}
/>

// Features:
// - One-click auto-detection
// - Shows preview of detected MCPs
// - Confirms before applying changes
// - Progress indicator
```

### Phase 4: SDK Wrapper (Future)
**Goal**: Zero-friction SDK integration

#### SDK Design
```typescript
// aim-sdk/src/index.ts
import { AIMClient } from '@aim/sdk'

const aim = new AIMClient({
  apiKey: process.env.AIM_API_KEY,
  autoDetect: true,  // ✨ Magic happens here
  autoRegister: true // Auto-register detected MCPs
})

// ✅ Wrap MCP client initialization
const mcp = await aim.wrapMCPClient('filesystem', {
  command: 'npx',
  args: ['-y', '@modelcontextprotocol/server-filesystem']
})

// Behind the scenes:
// 1. Register 'filesystem' MCP server if not exists
// 2. Add to current agent's talks_to list
// 3. Log connection event
// 4. Monitor usage and trust score
// 5. Report to AIM backend

// ✅ Auto-detect from config
await aim.detectAndRegisterMCPs({
  configPath: '~/Library/Application Support/Claude/claude_desktop_config.json'
})
```

#### SDK Architecture
```typescript
// aim-sdk/src/mcp-wrapper.ts
class MCPWrapper {
  private aim: AIMClient
  private mcpServers: Map<string, MCPClient>

  async connect(name: string, config: MCPConfig): Promise<MCPClient> {
    // 1. Register MCP server with AIM
    const server = await this.aim.registerMCPServer({
      name,
      command: config.command,
      args: config.args,
      detected_method: 'auto_sdk'
    })

    // 2. Add to agent's talks_to list
    await this.aim.addMCPServerToAgent(this.aim.agentId, server.id)

    // 3. Create wrapped MCP client
    const client = new MCPClient(config)

    // 4. Wrap all methods with telemetry
    return new Proxy(client, {
      get: (target, prop) => {
        return async (...args: any[]) => {
          const start = Date.now()
          try {
            const result = await target[prop](...args)

            // Log successful call
            await this.aim.logMCPCall({
              mcpServerId: server.id,
              method: prop,
              durationMs: Date.now() - start,
              success: true
            })

            return result
          } catch (error) {
            // Log failed call
            await this.aim.logMCPCall({
              mcpServerId: server.id,
              method: prop,
              durationMs: Date.now() - start,
              success: false,
              error: error.message
            })
            throw error
          }
        }
      }
    })
  }
}
```

---

## 🎨 User Experience Flows

### Flow 1: Manual Mapping (Web UI)
```
1. User navigates to Agent Details page
2. Clicks "Manage MCP Servers" button
3. Modal opens with:
   - Left: List of available MCP servers (searchable)
   - Right: Currently mapped MCP servers
4. User selects MCP servers from left list
5. Clicks "Add Selected" button
6. Backend updates agent.talks_to array
7. UI shows success toast and updates visualization
8. Audit log entry created
```

### Flow 2: Auto-Detection (Web UI)
```
1. User navigates to Agent Details page
2. Clicks "Auto-Detect MCP Servers" button
3. Backend reads Claude Desktop config (or user uploads it)
4. Shows preview modal:
   - "Found 5 MCP servers"
   - List of detected servers (new vs existing)
   - Checkboxes to select which to add
5. User reviews and clicks "Add Selected"
6. Backend auto-registers new MCPs (if any)
7. Backend updates agent.talks_to array
8. UI shows success with details
```

### Flow 3: SDK Auto-Detection (Code)
```typescript
// Developer's code
import { AIMClient } from '@aim/sdk'

const aim = new AIMClient({
  apiKey: process.env.AIM_API_KEY,
  agentId: 'my-agent-uuid',
  autoDetect: true  // ✨ One flag, everything happens
})

// Just use MCP normally, AIM handles everything
const mcp = await aim.mcp('filesystem')
const files = await mcp.listFiles('/home/user')

// Behind the scenes:
// - 'filesystem' MCP registered with AIM ✅
// - Added to agent's talks_to list ✅
// - Usage logged for trust scoring ✅
// - Security monitoring active ✅
```

---

## 🔒 Security Considerations

### Access Control
1. **Only agent owner** can modify talks_to list
2. **Admin/Manager** can view all relationships
3. **Audit logging** for all relationship changes
4. **Rate limiting** on auto-detection endpoints

### Validation
1. **Verify MCP server exists** before adding to talks_to
2. **Check organization ownership** (agent and MCP must be in same org)
3. **Prevent duplicates** in talks_to array
4. **Validate MCP server status** (can't add revoked/suspended servers)

### Trust & Monitoring
1. **Alert on drift**: Agent calls MCP not in talks_to list
2. **Trust score impact**: Unauthorized MCP calls decrease trust score
3. **Automatic removal**: Revoked MCPs automatically removed from all agents
4. **Usage tracking**: Monitor how often each agent uses each MCP

---

## 📊 Success Metrics

### MVP Success (Phase 1-3)
- [ ] API endpoints for manual mapping (5 endpoints)
- [ ] Auto-detection endpoint working
- [ ] UI components functional (selector, visualization, auto-detect)
- [ ] End-to-end test passing
- [ ] Documentation complete

### Developer Experience Success
- [ ] Manual mapping takes < 30 seconds
- [ ] Auto-detection takes < 5 seconds
- [ ] Zero errors in typical workflow
- [ ] Clear error messages for edge cases

### Enterprise Success (Future)
- [ ] SDK adoption > 1000 developers
- [ ] 95% of relationships auto-detected (not manual)
- [ ] < 1% false positive rate
- [ ] Trust score correlation with actual security incidents

---

## 🚦 Implementation Priority

### Priority 1 (This Week)
1. ✅ Backend API endpoints (PUT, DELETE, GET for relationships)
2. ✅ Frontend MCP Server Selector component
3. ✅ Auto-detection endpoint (config parsing)
4. ✅ End-to-end test

### Priority 2 (Next Week)
1. Relationship visualization component
2. Bulk operations UI
3. Audit logging integration
4. Security alerts for drift

### Priority 3 (Future)
1. SDK wrapper implementation
2. CLI commands for relationship management
3. Advanced visualization (graph view)
4. Migration to proper relationship table (Enterprise)

---

## 🧪 Testing Strategy

### Unit Tests
```go
// Test agent service
func TestAgentService_AddMCPServers(t *testing.T)
func TestAgentService_RemoveMCPServers(t *testing.T)
func TestAgentService_GetMCPServers(t *testing.T)
func TestAgentService_DetectMCPServers(t *testing.T)
```

### Integration Tests
```go
// Test API endpoints
func TestMCPRelationshipEndpoints(t *testing.T) {
    // 1. Create agent
    // 2. Create MCP servers
    // 3. Add MCPs to agent
    // 4. Verify talks_to updated
    // 5. Remove MCPs from agent
    // 6. Verify talks_to updated
}

func TestAutoDetection(t *testing.T) {
    // 1. Mock Claude config file
    // 2. Call detect endpoint
    // 3. Verify MCPs auto-registered
    // 4. Verify agent.talks_to updated
}
```

### E2E Tests
```typescript
// Test UI workflow
describe('Agent-MCP Mapping', () => {
  it('should manually map MCP servers to agent', async () => {
    // 1. Navigate to agent details
    // 2. Click "Manage MCP Servers"
    // 3. Select MCP servers
    // 4. Click "Add Selected"
    // 5. Verify UI updates
    // 6. Verify API called correctly
  })

  it('should auto-detect MCP servers', async () => {
    // 1. Click "Auto-Detect"
    // 2. Upload config file
    // 3. Verify preview shown
    // 4. Click "Add Selected"
    // 5. Verify relationships created
  })
})
```

---

## 📝 Next Steps

1. **Review this architecture** with the team
2. **Create GitHub issues** for each task
3. **Start with Phase 1** (Backend API endpoints)
4. **Build Phase 2** (Auto-detection)
5. **Build Phase 3** (Frontend UI)
6. **Document SDK design** for future implementation

---

**Last Updated**: October 9, 2025
**Status**: Ready for Implementation
**Target Completion**: Phase 1-3 by October 16, 2025
