# MCP Enhancement Design
**Date**: 2025-10-23
**Author**: Claude (Architectural Design)
**Status**: Ready for Implementation

## Executive Summary

This design enhances the MCP (Model Context Protocol) implementation to:
1. Display auto-detected MCPs alongside user-registered MCPs
2. Establish bidirectional Agent ↔ MCP relationships
3. Implement Ed25519 cryptographic verification for MCPs
4. Show real MCP capabilities via introspection
5. Display connected agents for each MCP

## Current State Analysis

### What's Already Built ✅
- Database schema supports Ed25519 keys (`mcp_servers.public_key`)
- Auto-detection tracking (`agent_mcp_detections` table)
- Ed25519 crypto service in backend (`internal/infrastructure/crypto`)
- Automatic key generation for MCPs
- Challenge-based verification framework
- Capability detection service (`MCPCapabilityService`)
- Agent repository integration

### What's Missing ❌
- No connection between `agent_mcp_detections` and `mcp_servers` tables
- MCP list page only shows user-registered MCPs
- "Verify" button not wired to Ed25519 verification
- Capabilities tab uses placeholder data
- Connected Agents tab not implemented
- No bidirectional MCP ↔ Agent relationship

## Architecture: Hybrid Promotion Model

### Core Principle
**Keep detections separate until user promotes them.** This provides:
- User consent and control
- Clear audit trail
- Single source of truth after promotion
- Flexibility to ignore false positives

### Data Flow
```
SDK Agent Runtime
    ↓ (detects MCP usage)
agent_mcp_detections table (auto-detected, unverified)
    ↓ (user clicks "Promote")
mcp_servers table (registered, can be verified)
    ↓ (creates link)
agent_mcp_connections table (bidirectional relationship)
```

## Database Schema Changes

### New Table: `agent_mcp_connections`

```sql
CREATE TABLE IF NOT EXISTS agent_mcp_connections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    mcp_server_id UUID NOT NULL REFERENCES mcp_servers(id) ON DELETE CASCADE,
    detection_id UUID REFERENCES agent_mcp_detections(id) ON DELETE SET NULL,
    connection_type VARCHAR(50) NOT NULL CHECK (
        connection_type IN ('auto_detected', 'user_registered', 'verified')
    ),
    first_connected_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_verified_at TIMESTAMPTZ,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(agent_id, mcp_server_id)
);

CREATE INDEX idx_agent_mcp_connections_agent ON agent_mcp_connections(agent_id);
CREATE INDEX idx_agent_mcp_connections_mcp ON agent_mcp_connections(mcp_server_id);
CREATE INDEX idx_agent_mcp_connections_detection ON agent_mcp_connections(detection_id);
```

**Design Decisions:**
- **Bidirectional**: Supports both "agents using MCP X" and "MCPs used by agent Y" queries
- **Connection Type Tracking**: Distinguishes auto-detected vs user-registered vs verified
- **Soft Delete**: `is_active` flag allows deactivation without data loss
- **Audit Trail**: Tracks first connection and last verification separately
- **Nullable Detection ID**: Detection may be deleted, but connection persists

### Migration File
Create: `apps/backend/migrations/039_create_agent_mcp_connections_table.sql`

## Backend API Changes

### New Endpoints

#### 1. Unified MCP List
```
GET /api/v1/mcp-servers?include_detected=true
```

**Response:**
```json
{
  "registered": [
    {
      "id": "uuid",
      "name": "OpenAI MCP Server",
      "url": "https://mcp.openai.com",
      "status": "verified",
      "is_verified": true,
      "trust_score": 85.5,
      "connected_agents_count": 3,
      "capabilities_count": 12
    }
  ],
  "detected": [
    {
      "id": "detection_uuid",
      "mcp_server_name": "Anthropic MCP",
      "agent_id": "agent_uuid",
      "agent_name": "test-agent-1",
      "confidence_score": 95.0,
      "detection_method": "runtime_analysis",
      "first_detected_at": "2025-10-23T10:00:00Z",
      "last_seen_at": "2025-10-23T17:30:00Z",
      "can_promote": true
    }
  ],
  "total": 5
}
```

#### 2. Promote Detection to Registered MCP
```
POST /api/v1/mcp-servers/promote/:detection_id
```

**Request:**
```json
{
  "name": "Anthropic MCP Server",
  "description": "Official Anthropic Model Context Protocol server",
  "generate_keys": true
}
```

**Response:**
```json
{
  "mcp_server": {
    "id": "uuid",
    "name": "Anthropic MCP Server",
    "public_key": "ed25519:AAAA...",
    "status": "pending",
    "is_verified": false
  },
  "connection": {
    "id": "uuid",
    "agent_id": "uuid",
    "mcp_server_id": "uuid",
    "connection_type": "auto_detected",
    "first_connected_at": "2025-10-23T17:35:00Z"
  }
}
```

#### 3. Get Connected Agents
```
GET /api/v1/mcp-servers/:id/agents
```

**Response:**
```json
{
  "agents": [
    {
      "id": "uuid",
      "name": "prod-agent-1",
      "type": "ai_agent",
      "connection_type": "verified",
      "first_connected_at": "2025-10-20T10:00:00Z",
      "last_verified_at": "2025-10-23T15:00:00Z",
      "is_active": true
    }
  ],
  "total": 3,
  "verified_count": 2,
  "auto_detected_count": 1
}
```

#### 4. Get Agent MCP Connections
```
GET /api/v1/agents/:id/mcp-servers
```

**Response:**
```json
{
  "mcps": [
    {
      "id": "uuid",
      "name": "OpenAI MCP",
      "url": "https://mcp.openai.com",
      "connection_type": "verified",
      "first_connected_at": "2025-10-20T10:00:00Z",
      "last_verified_at": "2025-10-23T15:00:00Z",
      "is_active": true
    }
  ],
  "total": 2
}
```

#### 5. Ed25519 Cryptographic Verification
```
POST /api/v1/mcp-servers/:id/verify
```

**Request:**
```json
{
  "challenge": "server-generated-challenge-string",
  "signature": "ed25519-signature-of-challenge"
}
```

**Response:**
```json
{
  "verified": true,
  "verification_event_id": "uuid",
  "trust_score_change": 15.0,
  "new_trust_score": 85.5
}
```

#### 6. Introspect MCP Capabilities
```
GET /api/v1/mcp-servers/:id/capabilities/introspect
```

**Response:**
```json
{
  "capabilities": [
    {
      "id": "uuid",
      "name": "create_completion",
      "capability_type": "tool",
      "description": "Create text completion",
      "capability_schema": { "input": {...}, "output": {...} },
      "is_active": true
    }
  ],
  "introspected_at": "2025-10-23T17:40:00Z",
  "source": "introspection",
  "total": 12
}
```

### Backend Service Methods

**File**: `apps/backend/internal/application/mcp_service.go`

```go
// PromoteDetectionToMCPServer converts auto-detected MCP to registered MCP
func (s *MCPService) PromoteDetectionToMCPServer(
    ctx context.Context,
    detectionID uuid.UUID,
    req PromoteMCPRequest,
    orgID, userID uuid.UUID,
) (*domain.MCPServer, *domain.AgentMCPConnection, error) {
    // 1. Fetch detection from agent_mcp_detections
    // 2. Verify detection belongs to user's organization
    // 3. Check if MCP already exists (by URL/name matching)
    // 4. Create mcp_servers entry
    // 5. Auto-generate Ed25519 keys if requested
    // 6. Create agent_mcp_connections link
    // 7. Optionally soft-delete or mark detection as promoted
    // 8. Return both MCP server and connection
}

// GetConnectedAgents returns all agents connected to this MCP
func (s *MCPService) GetConnectedAgents(
    ctx context.Context,
    mcpServerID uuid.UUID,
    orgID uuid.UUID,
) ([]domain.AgentSummary, error) {
    // 1. Verify MCP belongs to organization
    // 2. Query agent_mcp_connections WHERE mcp_server_id = ?
    // 3. JOIN with agents table to get agent details
    // 4. Return agent summaries with connection metadata
}

// VerifyMCPWithEd25519 performs cryptographic verification
func (s *MCPService) VerifyMCPWithEd25519(
    ctx context.Context,
    mcpServerID uuid.UUID,
    challenge, signature string,
    orgID uuid.UUID,
) (*domain.VerificationEvent, error) {
    // 1. Fetch MCP server and verify ownership
    // 2. Get public key from mcp_servers.public_key
    // 3. Verify signature using Ed25519Service
    // 4. If valid:
    //    - Update mcp_servers.is_verified = TRUE
    //    - Update mcp_servers.last_verified_at = NOW()
    //    - Create verification_events entry
    //    - Update trust score
    //    - Update agent_mcp_connections.last_verified_at
    // 5. Return verification event
}

// IntrospectMCPCapabilities queries MCP server for real capabilities
func (s *MCPService) IntrospectMCPCapabilities(
    ctx context.Context,
    mcpServerID uuid.UUID,
) ([]domain.MCPCapability, error) {
    // 1. Fetch MCP server
    // 2. Call MCP protocol introspection endpoint
    // 3. Parse tools/resources/prompts response
    // 4. Store/update in mcp_server_capabilities table
    // 5. Mark source as "introspection"
    // 6. Return capabilities list
}

// GetMCPServersForAgent returns all MCPs connected to agent
func (s *MCPService) GetMCPServersForAgent(
    ctx context.Context,
    agentID uuid.UUID,
    orgID uuid.UUID,
) ([]domain.MCPServerWithConnection, error) {
    // 1. Verify agent belongs to organization
    // 2. Query agent_mcp_connections WHERE agent_id = ?
    // 3. JOIN with mcp_servers table
    // 4. Return MCP servers with connection metadata
}
```

## Frontend Changes

### MCP List Page (`apps/web/app/dashboard/mcp/page.tsx`)

**New TypeScript Interfaces:**

```typescript
interface UnifiedMCPView {
  registered: MCPServer[];
  detected: DetectedMCP[];
  total: number;
}

interface DetectedMCP {
  id: string;                    // detection_id
  mcp_server_name: string;
  agent_id: string;
  agent_name: string;
  confidence_score: number;
  detection_method: string;
  first_detected_at: string;
  last_seen_at: string;
  can_promote: boolean;
}
```

**UI Changes:**

1. **Add "Auto-Detected MCPs" Section** (below Registered MCPs)
   - Table showing unregistered MCPs detected by SDK
   - Columns: Name, Detected By (agent), Confidence, Method, First/Last Seen, Actions
   - "Promote" button for each row

2. **Add Toggle Filter**
   - "Show All" | "Registered Only" | "Detected Only"

3. **Update Stats Cards**
   - Total MCPs (registered + detected)
   - Verified MCPs
   - Auto-Detected MCPs (pending promotion)
   - Average Trust Score

### MCP Detail Page (`apps/web/app/dashboard/mcp/[id]/page.tsx`)

**Tab 1: Overview** (keep existing)

**Tab 2: Capabilities** (ENHANCED)

```typescript
interface CapabilitiesTab {
  capabilities: MCPCapability[];
  source: "introspection" | "manual" | "detected";
  last_introspected_at?: string;
  actions: {
    introspect: () => Promise<void>;
    refresh: () => Promise<void>;
  };
}
```

**UI Changes:**
- Replace placeholder data with API call to `GET /api/v1/mcp-servers/:id/capabilities`
- Add "Introspect Server" button to query MCP for real capabilities
- Show capability type badges (tool/resource/prompt)
- Display capability schema (JSON viewer)
- Show last introspection timestamp

**Tab 3: Connected Agents** (NEW)

```typescript
interface ConnectedAgentsTab {
  agents: Array<{
    id: string;
    name: string;
    type: string;
    connection_type: "auto_detected" | "user_registered" | "verified";
    first_connected_at: string;
    last_verified_at?: string;
    is_active: boolean;
  }>;
  stats: {
    total: number;
    verified: number;
    auto_detected: number;
  };
}
```

**UI Components:**
- Table showing all connected agents
- Columns: Agent Name, Type, Connection Type, First Connected, Last Verified, Status
- Filter by connection type
- Click agent name → navigate to agent detail page

**Tab 4: Verification** (ENHANCED)

**Current State:**
- Shows public key (if exists)
- Shows verification status
- Has "Verify" button (not functional)

**Enhanced State:**
- Wire "Verify" button to Ed25519 verification endpoint
- Show verification flow:
  1. Generate challenge
  2. MCP server signs challenge
  3. Backend verifies signature
  4. Update UI on success/failure
- Display verification history (from `verification_events`)
- Show trust score impact

### New Components

1. **`<PromoteMCPModal />`**
   - Location: `apps/web/components/modals/promote-mcp-modal.tsx`
   - Props: `detectionId`, `mcpServerName`, `agentName`, `onSuccess`, `onClose`
   - Fields: Name (pre-filled), Description, "Generate Ed25519 Keys" checkbox
   - Actions: Cancel, Promote

2. **`<Ed25519VerificationModal />`**
   - Location: `apps/web/components/modals/ed25519-verification-modal.tsx`
   - Props: `mcpServerId`, `publicKey`, `onSuccess`, `onClose`
   - Shows verification progress/status
   - Displays signature verification result

3. **`<MCPIntrospectionButton />`**
   - Location: `apps/web/components/mcp/introspection-button.tsx`
   - Props: `mcpServerId`, `onIntrospect`
   - Triggers capability introspection
   - Shows loading state during query

4. **`<ConnectedAgentsTable />`**
   - Location: `apps/web/components/mcp/connected-agents-table.tsx`
   - Props: `mcpServerId`
   - Fetches and displays connected agents
   - Supports filtering by connection type

## User Flows

### Flow 1: Promote Auto-Detected MCP

```
1. User navigates to /dashboard/mcp
2. Page displays two sections:
   - Registered MCPs (existing)
   - Auto-Detected MCPs (new)
3. User sees "Anthropic MCP" detected by "test-agent-1" with 95% confidence
4. User clicks "Promote" button
5. <PromoteMCPModal /> appears with:
   - Name: "Anthropic MCP" (pre-filled, editable)
   - Description: (empty, optional)
   - ☑ Generate Ed25519 Keys automatically
6. User clicks "Promote & Generate Keys"
7. Backend:
   - Creates mcp_servers entry
   - Generates Ed25519 key pair
   - Creates agent_mcp_connections link
   - Marks detection as promoted
8. UI updates:
   - MCP moves to "Registered MCPs" section
   - Shows status "pending" (not verified yet)
   - Success toast: "MCP promoted successfully"
```

### Flow 2: Ed25519 Cryptographic Verification

```
1. User opens MCP detail page for promoted MCP
2. Clicks "Verification" tab
3. Current state shows:
   - Public Key: ed25519:AAAA... (generated during promotion)
   - Status: Unverified
   - "Verify MCP Server" button
4. User clicks "Verify MCP Server"
5. <Ed25519VerificationModal /> appears
6. Backend:
   - Generates random challenge string
   - Returns challenge to frontend
7. Frontend displays: "Waiting for MCP server signature..."
8. MCP server (via MCP protocol):
   - Receives challenge
   - Signs with private key
   - Returns signature
9. Frontend sends signature to backend
10. Backend:
    - Verifies signature using public key
    - Updates mcp_servers.is_verified = TRUE
    - Updates mcp_servers.last_verified_at = NOW()
    - Creates verification_events entry
    - Updates trust score (+15 points)
    - Updates agent_mcp_connections.last_verified_at
11. UI updates:
    - Status badge: "Unverified" → "Verified"
    - Trust score: 70.0 → 85.0
    - Last Verified: "Just now"
    - Success toast: "MCP verified successfully"
```

### Flow 3: View Connected Agents

```
1. User opens MCP detail page
2. Clicks "Connected Agents" tab
3. Frontend calls GET /api/v1/mcp-servers/:id/agents
4. Backend:
   - Queries agent_mcp_connections WHERE mcp_server_id = ?
   - JOINs with agents table
   - Returns agent summaries with connection metadata
5. UI displays <ConnectedAgentsTable />:
   - Columns: Agent Name, Type, Connection Type, First Connected, Last Verified, Status
   - Filter dropdown: All | Auto-Detected | Verified
6. User clicks agent name
7. Navigates to /dashboard/agents/:id
```

### Flow 4: Capability Introspection

```
1. User opens MCP detail page
2. Clicks "Capabilities" tab
3. Current state shows:
   - Empty capabilities list (or placeholder data)
   - "Introspect Server" button
4. User clicks "Introspect Server"
5. Frontend calls GET /api/v1/mcp-servers/:id/capabilities/introspect
6. Backend:
   - Calls MCP protocol introspection endpoint
   - Parses response for tools/resources/prompts
   - Stores in mcp_server_capabilities table
   - Marks source as "introspection"
7. UI updates with real capabilities:
   - List of tools (e.g., "create_completion", "generate_image")
   - List of resources (e.g., "documentation", "examples")
   - List of prompts (e.g., "code_review", "summarize")
   - Each with description and schema
   - Last Introspected: "Just now"
8. User can expand capability to view JSON schema
```

## Implementation Order

### Phase 1: Database & Backend Foundation
1. Create migration `039_create_agent_mcp_connections_table.sql`
2. Run migration on local database
3. Add domain models for `AgentMCPConnection`
4. Implement repository methods for connections

### Phase 2: Backend API - MCP Promotion
1. Implement `PromoteDetectionToMCPServer` service method
2. Add `POST /api/v1/mcp-servers/promote/:detection_id` endpoint
3. Update `GET /api/v1/mcp-servers` to include `?include_detected=true`
4. Test promotion flow with integration tests

### Phase 3: Backend API - Connections
1. Implement `GetConnectedAgents` service method
2. Add `GET /api/v1/mcp-servers/:id/agents` endpoint
3. Implement `GetMCPServersForAgent` service method
4. Add `GET /api/v1/agents/:id/mcp-servers` endpoint

### Phase 4: Backend API - Verification & Introspection
1. Implement `VerifyMCPWithEd25519` service method
2. Add `POST /api/v1/mcp-servers/:id/verify` endpoint
3. Implement `IntrospectMCPCapabilities` service method
4. Add `GET /api/v1/mcp-servers/:id/capabilities/introspect` endpoint

### Phase 5: Frontend - MCP List Page
1. Update API client with new endpoints
2. Create `<PromoteMCPModal />` component
3. Add "Auto-Detected MCPs" section to list page
4. Wire promotion flow

### Phase 6: Frontend - MCP Detail Page
1. Create `<ConnectedAgentsTable />` component
2. Add "Connected Agents" tab
3. Create `<MCPIntrospectionButton />` component
4. Enhance "Capabilities" tab with real data
5. Create `<Ed25519VerificationModal />` component
6. Wire "Verify" button to Ed25519 verification

### Phase 7: Testing & Polish
1. Test all flows with Chrome DevTools MCP
2. Add loading states and error handling
3. Update documentation
4. Create user guide for MCP management

## Testing Strategy

### Backend Integration Tests

**File**: `apps/backend/tests/integration/mcp_connections_test.go`

```go
func TestMCPPromotionFlow(t *testing.T)
func TestGetConnectedAgents(t *testing.T)
func TestGetAgentMCPServers(t *testing.T)
func TestEd25519MCPVerification(t *testing.T)
func TestMCPCapabilityIntrospection(t *testing.T)
```

### Frontend Testing

**Chrome DevTools MCP Testing:**

1. Navigate to `/dashboard/mcp`
2. Verify auto-detected MCPs display
3. Test promotion flow
4. Verify Connected Agents tab
5. Test capability introspection
6. Test Ed25519 verification

## Security Considerations

1. **Ed25519 Key Storage**: Private keys stored in KeyVault (encrypted)
2. **Challenge Expiry**: Verification challenges expire after 5 minutes
3. **Organization Isolation**: All queries filtered by `organization_id`
4. **Signature Verification**: Uses constant-time comparison to prevent timing attacks
5. **HTTPS Required**: MCP introspection requires HTTPS endpoints
6. **Rate Limiting**: Verification and introspection endpoints rate-limited

## Performance Considerations

1. **Query Optimization**: Indexes on `agent_mcp_connections` for both directions
2. **Caching**: Cache MCP capabilities for 1 hour after introspection
3. **Pagination**: Connected agents list supports pagination
4. **Async Introspection**: Capability introspection runs async, returns job ID

## Success Metrics

1. ✅ Auto-detected MCPs visible in UI
2. ✅ Users can promote detections to registered MCPs
3. ✅ Ed25519 verification working end-to-end
4. ✅ Capabilities tab shows real data from introspection
5. ✅ Connected Agents tab displays all agent connections
6. ✅ Bidirectional queries work (agents → MCPs and MCPs → agents)
7. ✅ All tests pass (backend + frontend)
8. ✅ No console errors in Chrome DevTools

## Architectural Enhancements

### Why This Design is Superior

1. **User Control**: Users decide which auto-detected MCPs to trust
2. **Audit Trail**: Complete history of detection → promotion → verification
3. **Flexibility**: Can ignore false positives without cluttering MCP list
4. **Scalability**: Efficient bidirectional queries with proper indexing
5. **Security**: Ed25519 cryptographic verification ensures MCP authenticity
6. **Transparency**: Real-time capability introspection shows actual MCP capabilities

### Alternative Approaches Considered

**Auto-Upgrade Model**: Automatically create `mcp_servers` entries for detections
- **Rejected**: Removes user control, creates MCPs without consent

**Dual-Mode Model**: Keep detections completely separate from registered MCPs
- **Rejected**: No path to promotion, duplicates data, confusing UX

### Future Enhancements

1. **MCP Health Monitoring**: Periodic health checks for registered MCPs
2. **Capability Diff Detection**: Alert when MCP capabilities change
3. **Trust Score Decay**: Lower trust score for unverified MCPs over time
4. **Batch Promotion**: Promote multiple detections at once
5. **MCP Marketplace**: Discover and register popular MCPs from catalog

---

**End of Design Document**
