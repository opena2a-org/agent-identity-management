# MCP Feature Enhancement Implementation Plan

## Executive Summary

This plan outlines the implementation of comprehensive MCP (Model Context Protocol) server management features to position AIM as the definitive enterprise solution for MCP asset management. The platform's value proposition: **"We bring enterprise-grade visibility and governance to the MCP wild west."**

---

## Current State Assessment

### Already Implemented ✅

| Feature | Backend | Frontend | Status |
|---------|---------|----------|--------|
| MCP Registration | ✅ | ✅ | Complete |
| MCP Attestation (Ed25519) | ✅ | Partial | Backend complete, UI needs polish |
| MCP Confidence Score | ✅ | ✅ | Working |
| MCP Trust Score (basic) | ✅ | ✅ | Field exists, algorithm basic |
| MCP Capabilities | ✅ | ✅ | Type array stored, details via tab |
| Agent-MCP Connections | ✅ | ✅ | Table + API + basic UI |
| MCP Discovery (SDK) | ✅ | ❌ | SDK detects, no dashboard UI |
| Multi-Agent Consensus | ✅ | ❌ | Algorithm works, no progress UI |
| Audit Trail Fields | ✅ | Partial | Fields exist, no filtered view |

### Missing Features ⚠️

1. **Frontend Attestation UI** - Consensus progress bars, manual attestation form, revocation interface
2. **MCP Trust Scoring Algorithm** - 8-factor weighted algorithm like agents
3. **MCP-to-Agent Connection Graph** - Visual network diagram
4. **MCP Discovery Dashboard** - Show detected MCPs, registration workflow
5. **MCP Compliance Policies** - Allowlist/blocklist enforcement
6. **MCP Audit Trail View** - Filtered audit logs for MCP operations

---

## Implementation Plan

### Phase 1: Frontend Attestation UI (Priority: HIGH)

**Goal:** Complete the attestation user experience

#### 1.1 Consensus Progress Component
**File:** `apps/web/components/mcp/consensus-progress.tsx`

```typescript
interface ConsensusProgressProps {
  mcpServerId: string;
  currentAgents: number;
  requiredAgents: number;  // Default: 3
  currentOwners: number;
  requiredOwners: number;  // Default: 2
  confidenceScore: number;
  requiredScore: number;   // Default: 60
}
```

**Features:**
- Progress bars for each threshold (agents, owners, score)
- Color-coded status (red → yellow → green)
- "What's needed" checklist showing missing criteria
- Estimated time to verification based on attestation velocity

#### 1.2 Manual Attestation Form
**File:** `apps/web/components/mcp/manual-attestation-form.tsx`

**Features:**
- Connection test (ping MCP URL)
- Health check verification
- Capability confirmation checkboxes
- Notes field for context
- Submit button that calls `/api/v1/mcp-servers/{id}/manual-attest`

#### 1.3 Revocation Interface
**File:** `apps/web/components/mcp/attestation-revocation.tsx`

**Features:**
- List of attestations with revoke button (managers only)
- Reason selection (compromised, outdated, false positive)
- Confirmation dialog
- Audit log entry display

#### 1.4 Update MCP Detail Page
**File:** `apps/web/app/dashboard/mcp/[id]/page.tsx`

**Changes:**
- Add ConsensusProgress component to hero section
- Add ManualAttestationForm to Attestations tab
- Add revocation buttons to attestation list

---

### Phase 2: MCP Trust Scoring Algorithm (Priority: HIGH)

**Goal:** Implement 8-factor trust scoring for MCPs, mirroring agents

#### 2.1 Create MCP Trust Calculator
**File:** `apps/backend/internal/application/mcp_trust_calculator.go`

```go
// MCPTrustCalculator implements trust scoring for MCP servers
// 8-factor weighted algorithm:
//   - Attestation Consensus (25%): Multi-agent verification strength
//   - Connection Health (15%): Uptime and response times
//   - Capability Stability (15%): Schema changes, deprecations
//   - Security Posture (15%): TLS, authentication, known vulns
//   - Organization Compliance (10%): Meets policy requirements
//   - Age & History (10%): Operating duration without issues
//   - Usage Patterns (5%): Normal vs anomalous usage
//   - User Feedback (5%): Admin ratings and notes
```

#### 2.2 Trust Score Factors
| Factor | Weight | Source | Calculation |
|--------|--------|--------|-------------|
| Attestation Consensus | 25% | mcp_attestations | `valid_attestations / required_threshold` |
| Connection Health | 15% | Health checks | `successful_pings / total_pings` over 30 days |
| Capability Stability | 15% | mcp_server_capabilities | `unchanged_capabilities / total_capabilities` |
| Security Posture | 15% | MCP metadata | TLS enabled, auth required, no CVEs |
| Organization Compliance | 10% | Policy evaluation | Meets allowlist rules |
| Age & History | 10% | mcp_servers.created_at | Days since registration |
| Usage Patterns | 5% | agent_mcp_connections | Normal usage volume |
| User Feedback | 5% | Manual inputs | Admin ratings |

#### 2.3 Database Migration
**File:** `apps/backend/migrations/051_add_mcp_trust_scores.sql`

```sql
CREATE TABLE mcp_trust_scores (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    mcp_server_id UUID NOT NULL REFERENCES mcp_servers(id) ON DELETE CASCADE,
    score DECIMAL(5,4) NOT NULL,
    factors JSONB NOT NULL,
    confidence DECIMAL(5,4) NOT NULL,
    last_calculated TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_mcp_trust_scores_server_id ON mcp_trust_scores(mcp_server_id);
CREATE INDEX idx_mcp_trust_scores_score ON mcp_trust_scores(score DESC);
```

#### 2.4 Frontend Trust Score Display
**File:** `apps/web/components/mcp/trust-score-breakdown.tsx`

- Similar to agent trust score breakdown component
- 8-factor radar chart
- Score history graph
- Factor explanations with improvement suggestions

---

### Phase 3: MCP-to-Agent Connection Graph (Priority: MEDIUM)

**Goal:** Visual network diagram showing MCP-Agent relationships

#### 3.1 Graph Data API
**File:** `apps/backend/internal/interfaces/http/handlers/mcp_graph_handler.go`

```go
// GET /api/v1/mcp-servers/graph
// Returns network graph data for visualization
type MCPGraphResponse struct {
    Nodes []GraphNode `json:"nodes"`
    Edges []GraphEdge `json:"edges"`
}

type GraphNode struct {
    ID    string `json:"id"`
    Type  string `json:"type"`  // "mcp" or "agent"
    Label string `json:"label"`
    Size  int    `json:"size"`  // Based on connection count
    Color string `json:"color"` // Based on trust/confidence score
}

type GraphEdge struct {
    Source string `json:"source"`
    Target string `json:"target"`
    Weight int    `json:"weight"` // Attestation count
}
```

#### 3.2 Frontend Graph Component
**File:** `apps/web/components/mcp/connection-graph.tsx`

**Technology:** React Flow or D3.js force-directed graph

**Features:**
- Interactive pan/zoom
- Click node to view details
- Color-coded by trust level
- Edge thickness = connection strength
- Filter by: status, trust score, attestation count
- Export as image/SVG

#### 3.3 New Dashboard Tab
**File:** `apps/web/app/dashboard/mcp/graph/page.tsx`

- Full-page network visualization
- Sidebar with filters and legend
- Search to highlight specific nodes

---

### Phase 4: MCP Discovery Dashboard (Priority: MEDIUM)

**Goal:** Surface detected MCPs and streamline registration

#### 4.1 Discovery API Endpoint
**File:** `apps/backend/internal/interfaces/http/handlers/mcp_discovery_handler.go`

```go
// GET /api/v1/mcp-servers/discovered
// Returns MCPs detected by agents but not yet registered
type DiscoveredMCP struct {
    Name            string   `json:"name"`
    URL             string   `json:"url"`
    DetectedBy      []string `json:"detectedBy"`       // Agent IDs
    DetectionMethod string   `json:"detectionMethod"`  // claude_config, sdk_import, runtime
    DetectedAt      string   `json:"detectedAt"`
    IsRegistered    bool     `json:"isRegistered"`
    MatchingServer  string   `json:"matchingServerId"` // If registered
}
```

#### 4.2 Discovery Dashboard UI
**File:** `apps/web/app/dashboard/mcp/discovery/page.tsx`

**Features:**
- Table of detected MCPs (name, URL, detection source, agents)
- "Register" button opens pre-filled registration form
- "Ignore" button to dismiss false positives
- Filter by detection method
- Bulk registration support

#### 4.3 SDK Detection Reporting
- SDK already has `MCPDetector` class
- Add `report_detections()` method to send to backend
- Backend stores in `mcp_detections` table

---

### Phase 5: MCP Compliance Policies (Priority: HIGH)

**Goal:** Allowlist/blocklist enforcement for MCP servers

#### 5.1 New Policy Types
**File:** `apps/backend/internal/domain/security_policy.go`

```go
const (
    // Existing policy types...

    // NEW: MCP-specific policies
    PolicyTypeMCPAllowlist     PolicyType = "mcp_allowlist"      // Only allow listed MCPs
    PolicyTypeMCPBlocklist     PolicyType = "mcp_blocklist"      // Block specific MCPs
    PolicyTypeMCPCapabilities  PolicyType = "mcp_capabilities"   // Required/forbidden capabilities
    PolicyTypeMCPUnverified    PolicyType = "mcp_unverified"     // Action on unverified MCPs
)
```

#### 5.2 Policy Rules Schema
```go
// MCP Allowlist Rules
type MCPAllowlistRules struct {
    AllowedDomains    []string `json:"allowedDomains"`    // ["*.company.com", "github.com"]
    AllowedNames      []string `json:"allowedNames"`      // ["filesystem-mcp", "github-mcp"]
    AllowedCapabilities []string `json:"allowedCapabilities"` // ["tools", "resources"]
    RequireVerified   bool     `json:"requireVerified"`   // Must be verified
    MinTrustScore     float64  `json:"minTrustScore"`     // Minimum trust score
    MinAttestations   int      `json:"minAttestations"`   // Minimum attestation count
}

// MCP Blocklist Rules
type MCPBlocklistRules struct {
    BlockedDomains []string `json:"blockedDomains"` // ["malicious.com"]
    BlockedNames   []string `json:"blockedNames"`   // ["compromised-mcp"]
    BlockedURLs    []string `json:"blockedUrls"`    // Exact URL matches
}
```

#### 5.3 Policy Evaluation in Verification Flow
**File:** `apps/backend/internal/application/mcp_service.go`

```go
// EvaluateMCPPolicy checks if MCP server meets policy requirements
func (s *MCPService) EvaluateMCPPolicy(ctx context.Context, mcpServer *domain.MCPServer) (*domain.PolicyEvaluationResult, error) {
    policies, err := s.policyRepo.GetActiveByOrganization(mcpServer.OrganizationID)
    // ... evaluate each policy
}
```

#### 5.4 Frontend Policy Management
**File:** `apps/web/app/dashboard/admin/mcp-policies/page.tsx`

**Features:**
- Create/edit MCP policies
- Domain allowlist/blocklist management
- Capability requirements
- Trust score thresholds
- Policy priority ordering
- Test policy against existing MCPs

---

### Phase 6: MCP Audit Trail View (Priority: MEDIUM)

**Goal:** Filtered audit log view for MCP operations

#### 6.1 API Enhancement
**File:** `apps/backend/internal/interfaces/http/handlers/audit_handler.go`

```go
// GET /api/v1/audit-logs?resourceType=mcp_server
// GET /api/v1/mcp-servers/{id}/audit-logs
```

#### 6.2 Frontend Audit View
**File:** `apps/web/app/dashboard/mcp/[id]/audit/page.tsx`

**Features:**
- Timeline view of all MCP operations
- Filter by action type (create, update, verify, attest, revoke)
- Filter by actor (user, agent, system)
- Export to CSV
- Real-time updates via WebSocket

#### 6.3 Audit Actions to Track
- `mcp_server.created` - MCP registered
- `mcp_server.updated` - MCP details changed
- `mcp_server.verified` - Status changed to verified
- `mcp_server.suspended` - MCP suspended
- `mcp_server.deleted` - MCP removed
- `mcp_attestation.created` - New attestation submitted
- `mcp_attestation.revoked` - Attestation revoked
- `mcp_connection.created` - Agent connected to MCP
- `mcp_policy.evaluated` - Policy check performed

---

## Implementation Order

### Sprint 1 (Week 1-2): Core Trust & Attestation
1. ✅ Phase 2.1-2.2: MCP Trust Calculator (backend)
2. ✅ Phase 2.3: Database migration for trust scores
3. ✅ Phase 1.1: Consensus Progress Component
4. ✅ Phase 1.2: Manual Attestation Form
5. ✅ Phase 1.4: Update MCP Detail Page

### Sprint 2 (Week 2-3): Policies & Discovery
1. ✅ Phase 5.1-5.2: MCP Policy Types (backend)
2. ✅ Phase 5.3: Policy Evaluation Integration
3. ✅ Phase 5.4: Policy Management UI
4. ✅ Phase 4.1-4.2: Discovery Dashboard

### Sprint 3 (Week 3-4): Visualization & Audit
1. ✅ Phase 3.1: Graph Data API
2. ✅ Phase 3.2-3.3: Connection Graph UI
3. ✅ Phase 6.1-6.3: Audit Trail View
4. ✅ Phase 2.4: Trust Score Breakdown UI
5. ✅ Phase 1.3: Revocation Interface

---

## File Summary

### New Backend Files
- `internal/application/mcp_trust_calculator.go`
- `internal/interfaces/http/handlers/mcp_graph_handler.go`
- `internal/interfaces/http/handlers/mcp_discovery_handler.go`
- `migrations/051_add_mcp_trust_scores.sql`
- `migrations/052_add_mcp_detections.sql`

### New Frontend Files
- `components/mcp/consensus-progress.tsx`
- `components/mcp/manual-attestation-form.tsx`
- `components/mcp/attestation-revocation.tsx`
- `components/mcp/trust-score-breakdown.tsx`
- `components/mcp/connection-graph.tsx`
- `app/dashboard/mcp/discovery/page.tsx`
- `app/dashboard/mcp/graph/page.tsx`
- `app/dashboard/mcp/[id]/audit/page.tsx`
- `app/dashboard/admin/mcp-policies/page.tsx`

### Modified Files
- `app/dashboard/mcp/[id]/page.tsx` - Add consensus progress, attestation form
- `internal/domain/security_policy.go` - Add MCP policy types
- `internal/application/mcp_service.go` - Add policy evaluation
- `lib/api.ts` - Add new API methods

---

## Success Metrics

1. **Attestation Completion Rate**: % of MCPs reaching verified status
2. **Policy Coverage**: % of MCPs with applicable policies
3. **Discovery-to-Registration Time**: Average time from detection to registration
4. **Trust Score Distribution**: Health of MCP ecosystem
5. **Audit Trail Usage**: Frequency of audit log queries

---

## Risk Mitigation

| Risk | Mitigation |
|------|------------|
| Graph performance with many nodes | Limit to 100 nodes, paginate, use WebGL |
| Policy evaluation latency | Cache policy results, async evaluation |
| Discovery false positives | Allow "ignore" action, tune detection |
| Trust score gaming | Multi-factor algorithm, anomaly detection |

---

## Design Decisions (User Confirmed)

1. **Graph Library**: **React Flow** - Easier to implement, good built-in controls
2. **Policy Enforcement**: **Configurable per policy** - Admins choose alert-only vs block-and-alert
3. **Trust Score Recalculation**: **Event-driven** - Recalculate when attestations, connections, or status changes
