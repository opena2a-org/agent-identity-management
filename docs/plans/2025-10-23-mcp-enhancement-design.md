# MCP Enhancement Design (Agent Attestation Model)
**Date**: 2025-10-23 (Updated)
**Author**: Claude (Architectural Design)
**Status**: Ready for Implementation
**Version**: 2.0 - Agent Attestation Approach

## Executive Summary

This design enhances the MCP (Model Context Protocol) implementation to:
1. Display auto-detected MCPs alongside user-registered MCPs
2. Establish bidirectional Agent ↔ MCP relationships
3. **Implement Agent-Attested MCP Verification** (cryptographically secure, zero developer effort)
4. Show real MCP capabilities via introspection
5. Display connected agents for each MCP

## Breakthrough: Agent Attestation (Not MCP Signing)

### The Problem with Traditional Approach
**Reality**: Developers don't control MCPs. Most MCPs are third-party (Anthropic, GitHub, OpenAI) or framework-provided (filesystem, sqlite). Asking developers to modify MCP servers is a non-starter.

### The Solution: Reverse the Trust Chain
**Instead of**: AIM → MCP Server (sign this) ❌ *We don't control the MCP!*

**Use**: AIM → Verified Agent → MCP (agent attests it works) ✅ *We control the agent!*

### Why This is Genius
- ✅ **Zero developer effort** - SDK handles everything automatically
- ✅ **Works for ALL MCPs** - Third-party, local utilities, custom, everything
- ✅ **Cryptographically secure** - Agent signs attestations with Ed25519
- ✅ **Continuous verification** - SDK re-attests periodically
- ✅ **Rich data** - Capabilities confirmed, latency measured, health checked

## Current State Analysis

### What's Already Built ✅
- Database schema for MCP servers (`mcp_servers` table)
- Auto-detection tracking (`agent_mcp_detections` table)
- Ed25519 crypto service for **agent** verification
- Automatic key generation for agents
- Capability detection service (`MCPCapabilityService`)
- Agent repository integration
- SDK with Ed25519 signing capability

### What's Missing ❌
- No connection between `agent_mcp_detections` and `mcp_servers` tables
- MCP list page only shows user-registered MCPs
- No MCP verification mechanism (button does nothing)
- Capabilities tab uses placeholder data
- Connected Agents tab not implemented
- No bidirectional MCP ↔ Agent relationship

## Architecture: Hybrid Promotion + Agent Attestation

### Core Principles

1. **Keep detections separate until user promotes them**
   - User consent and control
   - Clear audit trail
   - Flexibility to ignore false positives

2. **Verified agents attest to MCP identity**
   - Only Ed25519-verified agents can attest
   - Attestations are cryptographically signed
   - Multiple agents attesting = higher confidence

### Data Flow

```
SDK Agent Runtime (Verified via Ed25519)
    ↓ (detects MCP usage)
agent_mcp_detections table (auto-detected, unverified)
    ↓ (user clicks "Promote")
mcp_servers table (registered, awaiting attestation)
    ↓ (SDK automatically attests)
mcp_attestations table (signed attestations from verified agents)
    ↓ (creates link)
agent_mcp_connections table (bidirectional relationship)
```

### Agent Attestation Flow

```
1. Agent uses MCP (developer's normal code)
   ↓
2. SDK detects MCP usage automatically
   ↓
3. SDK tests MCP connection
   ↓
4. SDK lists MCP capabilities
   ↓
5. SDK creates attestation payload
   ↓
6. SDK signs attestation with agent's Ed25519 private key
   ↓
7. SDK submits to AIM backend
   ↓
8. Backend verifies signature using agent's public key
   ↓
9. Backend stores attestation
   ↓
10. MCP marked as "Attested" (confidence score calculated)
```

## Database Schema Changes

### New Table 1: `agent_mcp_connections`

```sql
CREATE TABLE IF NOT EXISTS agent_mcp_connections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    mcp_server_id UUID NOT NULL REFERENCES mcp_servers(id) ON DELETE CASCADE,
    detection_id UUID REFERENCES agent_mcp_detections(id) ON DELETE SET NULL,
    connection_type VARCHAR(50) NOT NULL CHECK (
        connection_type IN ('auto_detected', 'user_registered', 'attested')
    ),
    first_connected_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_attested_at TIMESTAMPTZ,
    attestation_count INT DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(agent_id, mcp_server_id)
);

CREATE INDEX idx_agent_mcp_connections_agent ON agent_mcp_connections(agent_id);
CREATE INDEX idx_agent_mcp_connections_mcp ON agent_mcp_connections(mcp_server_id);
CREATE INDEX idx_agent_mcp_connections_detection ON agent_mcp_connections(detection_id);
CREATE INDEX idx_agent_mcp_connections_type ON agent_mcp_connections(connection_type);
```

### New Table 2: `mcp_attestations` (THE KEY INNOVATION)

```sql
CREATE TABLE IF NOT EXISTS mcp_attestations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    mcp_server_id UUID NOT NULL REFERENCES mcp_servers(id) ON DELETE CASCADE,
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,

    -- Attestation payload (what the agent verified)
    attestation_data JSONB NOT NULL,
    -- {
    --   "agent_id": "uuid",
    --   "mcp_url": "https://api.anthropic.com/mcp",
    --   "mcp_name": "Anthropic MCP",
    --   "capabilities_found": ["prompt", "completion", "tool_use"],
    --   "connection_successful": true,
    --   "health_check_passed": true,
    --   "connection_latency_ms": 45,
    --   "timestamp": "2025-10-23T18:00:00Z",
    --   "sdk_version": "1.0.0"
    -- }

    -- Ed25519 signature of attestation_data (signed by agent's private key)
    signature TEXT NOT NULL,

    -- Verification status
    signature_verified BOOLEAN DEFAULT FALSE,
    verified_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ NOT NULL,
    is_valid BOOLEAN DEFAULT TRUE,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- One attestation per agent per MCP per time period
    UNIQUE(mcp_server_id, agent_id, verified_at)
);

CREATE INDEX idx_mcp_attestations_mcp ON mcp_attestations(mcp_server_id);
CREATE INDEX idx_mcp_attestations_agent ON mcp_attestations(agent_id);
CREATE INDEX idx_mcp_attestations_valid ON mcp_attestations(is_valid, verified_at);
CREATE INDEX idx_mcp_attestations_expires ON mcp_attestations(expires_at);
```

### Updated Table: `mcp_servers`

```sql
-- Add new columns for attestation-based verification
ALTER TABLE mcp_servers ADD COLUMN verification_method VARCHAR(50) DEFAULT 'agent_attestation';
ALTER TABLE mcp_servers ADD COLUMN attestation_count INT DEFAULT 0;
ALTER TABLE mcp_servers ADD COLUMN confidence_score DECIMAL(5,2) DEFAULT 0.00;
ALTER TABLE mcp_servers ADD COLUMN last_attested_at TIMESTAMPTZ;

-- Remove Ed25519 key requirement (not needed for attestation model)
ALTER TABLE mcp_servers ALTER COLUMN public_key DROP NOT NULL;

COMMENT ON COLUMN mcp_servers.verification_method IS 'agent_attestation, api_key, or manual';
COMMENT ON COLUMN mcp_servers.attestation_count IS 'Number of verified agent attestations';
COMMENT ON COLUMN mcp_servers.confidence_score IS 'Calculated from attestations (0-100)';
```

### Migration Files
1. `apps/backend/migrations/039_create_agent_mcp_connections_table.sql`
2. `apps/backend/migrations/040_create_mcp_attestations_table.sql`
3. `apps/backend/migrations/041_update_mcp_servers_for_attestation.sql`

## Backend API Changes

### New Endpoints

#### 1. Unified MCP List (Unchanged)
```
GET /api/v1/mcp-servers?include_detected=true
```

**Response:**
```json
{
  "registered": [
    {
      "id": "uuid",
      "name": "Anthropic MCP",
      "url": "https://api.anthropic.com/mcp",
      "status": "verified",
      "verification_method": "agent_attestation",
      "confidence_score": 95.5,
      "attestation_count": 3,
      "last_attested_at": "2025-10-23T17:30:00Z",
      "attested_by": ["prod-agent-1", "test-agent-2"],
      "connected_agents_count": 3,
      "capabilities_count": 12
    }
  ],
  "detected": [
    {
      "id": "detection_uuid",
      "mcp_server_name": "GitHub MCP",
      "agent_id": "agent_uuid",
      "agent_name": "test-agent-1",
      "confidence_score": 95.0,
      "detection_method": "runtime_analysis",
      "first_detected_at": "2025-10-23T10:00:00Z",
      "can_promote": true
    }
  ],
  "total": 5
}
```

#### 2. Promote Detection to Registered MCP (Unchanged)
```
POST /api/v1/mcp-servers/promote/:detection_id
```

#### 3. Submit MCP Attestation (NEW - Called by SDK)
```
POST /api/v1/mcp-servers/:id/attest
```

**Request:**
```json
{
  "attestation": {
    "agent_id": "uuid",
    "mcp_url": "https://api.anthropic.com/mcp",
    "mcp_name": "Anthropic MCP",
    "capabilities_found": ["prompt", "completion", "tool_use"],
    "connection_successful": true,
    "health_check_passed": true,
    "connection_latency_ms": 45,
    "timestamp": "2025-10-23T18:00:00Z",
    "sdk_version": "1.0.0"
  },
  "signature": "ed25519-signature-of-attestation-json"
}
```

**Response:**
```json
{
  "success": true,
  "attestation_id": "uuid",
  "mcp_confidence_score": 85.5,
  "attestation_count": 3,
  "message": "MCP attestation verified and recorded"
}
```

**Backend Logic:**
```go
func (s *MCPService) VerifyAndRecordAttestation(
    ctx context.Context,
    mcpServerID uuid.UUID,
    attestation AttestationPayload,
    signature string,
) (*MCPAttestation, error) {
    // 1. Fetch agent (MUST be Ed25519 verified)
    agent, err := s.agentRepo.GetByID(attestation.AgentID)
    if err != nil || !agent.IsVerified {
        return nil, fmt.Errorf("only verified agents can attest MCPs")
    }

    // 2. Verify signature using agent's public key
    attestationJSON := attestation.ToCanonicalJSON() // Sort keys for consistency
    valid := s.cryptoService.VerifyEd25519(
        agent.PublicKey,
        attestationJSON,
        signature,
    )
    if !valid {
        return nil, fmt.Errorf("invalid attestation signature")
    }

    // 3. Check attestation is recent (< 5 minutes old)
    if time.Since(attestation.Timestamp) > 5*time.Minute {
        return nil, fmt.Errorf("attestation expired")
    }

    // 4. Store attestation
    attestationRecord := &domain.MCPAttestation{
        MCPServerID:      mcpServerID,
        AgentID:          attestation.AgentID,
        AttestationData:  attestation,
        Signature:        signature,
        SignatureVerified: true,
        VerifiedAt:       time.Now(),
        ExpiresAt:        time.Now().Add(30 * 24 * time.Hour), // 30 days
        IsValid:          true,
    }

    if err := s.mcpRepo.CreateAttestation(ctx, attestationRecord); err != nil {
        return nil, err
    }

    // 5. Update MCP server stats
    if err := s.updateMCPConfidenceScore(ctx, mcpServerID); err != nil {
        return nil, err
    }

    // 6. Update agent_mcp_connections
    if err := s.updateAgentMCPConnection(ctx, attestation.AgentID, mcpServerID); err != nil {
        return nil, err
    }

    return attestationRecord, nil
}

// Calculate confidence score from attestations
func (s *MCPService) updateMCPConfidenceScore(
    ctx context.Context,
    mcpServerID uuid.UUID,
) error {
    attestations, err := s.mcpRepo.GetValidAttestations(ctx, mcpServerID)
    if err != nil {
        return err
    }

    // Confidence calculation factors:
    // - Number of unique agents attesting (20 points each, max 5 agents = 100)
    // - Average trust score of attesting agents (0-50 points)
    // - Recency of attestations (0-30 points)

    uniqueAgents := len(attestations)
    agentPoints := float64(uniqueAgents) * 20.0
    if agentPoints > 100.0 {
        agentPoints = 100.0
    }

    // Get trust scores of attesting agents
    var totalTrust float64
    for _, att := range attestations {
        agent, _ := s.agentRepo.GetByID(att.AgentID)
        totalTrust += agent.TrustScore
    }
    avgTrust := totalTrust / float64(len(attestations))
    trustPoints := (avgTrust / 100.0) * 50.0 // Scale to 0-50

    // Recency factor (% of attestations in last 7 days)
    recentCount := 0
    for _, att := range attestations {
        if time.Since(att.VerifiedAt) < 7*24*time.Hour {
            recentCount++
        }
    }
    recencyFactor := float64(recentCount) / float64(len(attestations))
    recencyPoints := recencyFactor * 30.0

    confidenceScore := (agentPoints + trustPoints + recencyPoints) / 1.8
    if confidenceScore > 100.0 {
        confidenceScore = 100.0
    }

    return s.mcpRepo.UpdateConfidenceScore(ctx, mcpServerID, confidenceScore)
}
```

#### 4. Get MCP Attestations
```
GET /api/v1/mcp-servers/:id/attestations
```

**Response:**
```json
{
  "attestations": [
    {
      "id": "uuid",
      "agent_id": "uuid",
      "agent_name": "prod-agent-1",
      "agent_trust_score": 95.5,
      "verified_at": "2025-10-23T17:30:00Z",
      "expires_at": "2025-11-22T17:30:00Z",
      "capabilities_confirmed": ["prompt", "completion", "tool_use"],
      "connection_latency_ms": 45,
      "health_check_passed": true,
      "is_valid": true
    }
  ],
  "total": 3,
  "confidence_score": 95.5,
  "last_attested_at": "2025-10-23T17:30:00Z"
}
```

#### 5. Get Connected Agents (Unchanged)
```
GET /api/v1/mcp-servers/:id/agents
```

#### 6. Get Agent MCP Connections (Unchanged)
```
GET /api/v1/agents/:id/mcp-servers
```

#### 7. Introspect MCP Capabilities (Unchanged)
```
GET /api/v1/mcp-servers/:id/capabilities/introspect
```

## SDK Integration (THE MAGIC)

### Automatic Attestation in Python SDK

**File**: `sdks/python/aim_sdk/client.py`

```python
import json
import asyncio
from typing import Dict, List, Optional
from datetime import datetime, timedelta
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
from mcp import Client as MCPClient

class AIMClient:
    def __init__(self, agent_id: str, private_key: str, api_url: str = "http://localhost:8080"):
        self.agent_id = agent_id
        self.private_key = Ed25519PrivateKey.from_private_bytes(bytes.fromhex(private_key))
        self.api_url = api_url

        # Track detected MCPs
        self._detected_mcps: Dict[str, MCPInfo] = {}

        # Start background attestation worker
        asyncio.create_task(self._background_attestor())

    def register_mcp_usage(self, mcp_url: str, mcp_name: str):
        """
        Called automatically when developer uses an MCP.
        Developer doesn't need to call this explicitly.
        """
        if mcp_url not in self._detected_mcps:
            self._detected_mcps[mcp_url] = MCPInfo(url=mcp_url, name=mcp_name)
            # Immediately attest on first detection
            asyncio.create_task(self._attest_mcp(mcp_url, mcp_name))

    async def _attest_mcp(self, mcp_url: str, mcp_name: str):
        """
        Automatically verify and attest an MCP.
        Runs in background - zero developer effort!
        """
        try:
            # Step 1: Test MCP connection
            mcp_client = MCPClient(mcp_url)
            await mcp_client.connect()

            # Step 2: List available capabilities
            capabilities = await mcp_client.list_capabilities()
            capability_names = [c.name for c in capabilities.tools]

            # Step 3: Health check (if MCP supports it)
            health_check_passed = True
            latency = 0
            try:
                start = datetime.utcnow()
                health = await mcp_client.call_tool("health_check", {})
                latency = (datetime.utcnow() - start).total_seconds() * 1000
                health_check_passed = health.success
            except:
                health_check_passed = False  # MCP doesn't support health check

            # Step 4: Create attestation payload
            attestation = {
                "agent_id": self.agent_id,
                "mcp_url": mcp_url,
                "mcp_name": mcp_name,
                "capabilities_found": capability_names,
                "connection_successful": True,
                "health_check_passed": health_check_passed,
                "connection_latency_ms": latency,
                "timestamp": datetime.utcnow().isoformat(),
                "sdk_version": "1.0.0"
            }

            # Step 5: Sign attestation with agent's Ed25519 private key
            attestation_json = json.dumps(attestation, sort_keys=True).encode('utf-8')
            signature = self.private_key.sign(attestation_json)
            signature_hex = signature.hex()

            # Step 6: Submit to AIM backend
            response = await self._submit_attestation(mcp_url, attestation, signature_hex)

            if response.get('success'):
                print(f"✅ MCP '{mcp_name}' attested successfully (confidence: {response.get('mcp_confidence_score')})")

        except Exception as e:
            # Report failed verification
            await self._report_mcp_failure(mcp_url, str(e))

    async def _submit_attestation(self, mcp_url: str, attestation: dict, signature: str):
        """Submit attestation to AIM backend"""
        # Find or create MCP server entry
        mcp = await self._get_or_create_mcp(mcp_url, attestation['mcp_name'])

        response = await self._http_post(
            f"{self.api_url}/api/v1/mcp-servers/{mcp['id']}/attest",
            json={
                "attestation": attestation,
                "signature": signature
            }
        )
        return response

    async def _background_attestor(self):
        """
        Background worker that re-attests MCPs periodically.
        Keeps attestations fresh (every 24 hours).
        """
        while True:
            await asyncio.sleep(24 * 60 * 60)  # 24 hours

            for mcp_url, mcp_info in self._detected_mcps.items():
                # Re-attest if last attestation > 24 hours ago
                if datetime.utcnow() - mcp_info.last_attested > timedelta(hours=24):
                    await self._attest_mcp(mcp_url, mcp_info.name)
```

### Developer Experience (ZERO EXTRA CODE)

```python
# File: developer_agent.py
from aim_sdk import AIMClient
from mcp import Client as MCPClient

# 1. Initialize AIM (already doing this for agent verification)
aim = AIMClient(
    agent_id="uuid",
    private_key="ed25519_private_key"
)

# 2. Use MCP normally (NO CHANGES NEEDED!)
anthropic_mcp = MCPClient("https://api.anthropic.com/mcp")
result = anthropic_mcp.call_tool("prompt", {"text": "Hello world"})

# ✨ SDK AUTOMATICALLY:
# - Detected MCP usage
# - Tested connection
# - Listed capabilities
# - Created attestation
# - Signed with agent's Ed25519 key
# - Submitted to AIM backend
# - MCP now marked as "Attested by prod-agent-1"

# Developer writes ZERO extra code!
# MCP automatically verified in background!
```

### Optional Manual Verification

```python
# If developer wants to verify immediately (optional)
aim.attest_mcp("https://api.anthropic.com/mcp")
```

## Frontend Changes

### MCP List Page (`apps/web/app/dashboard/mcp/page.tsx`)

**New Stats Cards:**
```typescript
const stats = [
  {
    name: "Total MCPs",
    value: totalMCPs,
    icon: Server
  },
  {
    name: "Attested MCPs",
    value: attestedCount,
    icon: Shield,
    change: "+3 this week",
    changeType: "positive"
  },
  {
    name: "Avg Confidence",
    value: `${avgConfidence}%`,
    icon: CheckCircle2
  },
  {
    name: "Auto-Detected",
    value: detectedCount,
    icon: Eye
  }
];
```

**Attestation Badge:**
```typescript
interface AttestationBadge {
  status: "attested" | "unattested" | "low_confidence";
  confidence_score: number;
  attestation_count: number;
  attested_by: string[];  // Agent names
}

function AttestationBadge({ mcp }: { mcp: MCPServer }) {
  if (mcp.attestation_count === 0) {
    return <Badge variant="secondary">Unattested</Badge>;
  }

  const confidence = mcp.confidence_score;
  const variant = confidence >= 80 ? "success" : confidence >= 50 ? "warning" : "secondary";

  return (
    <Badge variant={variant} className="flex items-center gap-1">
      <Shield className="h-3 w-3" />
      {confidence}% confidence
      <span className="text-xs">({mcp.attestation_count} agents)</span>
    </Badge>
  );
}
```

### MCP Detail Page - Verification Tab (REDESIGNED)

**Old Design** (Ed25519 MCP server signing):
```typescript
// ❌ This doesn't work - we don't control MCPs
<Button onClick={verifyMCP}>
  Verify MCP Server (Sign Challenge)
</Button>
```

**New Design** (Agent Attestation):
```typescript
interface VerificationTab {
  verification_method: "agent_attestation";
  attestations: Array<{
    agent_id: string;
    agent_name: string;
    agent_trust_score: number;
    verified_at: string;
    expires_at: string;
    capabilities_confirmed: string[];
    connection_latency_ms: number;
    health_check_passed: boolean;
  }>;
  confidence_score: number;
  last_attested_at: string;
}

function VerificationTab({ mcpId }: { mcpId: string }) {
  const { attestations, loading } = useAttestations(mcpId);

  return (
    <div className="space-y-6">
      {/* Confidence Score Card */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Shield className="h-5 w-5" />
            Verification Status
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex items-center justify-between">
            <div>
              <div className="text-3xl font-bold">
                {confidenceScore}%
              </div>
              <div className="text-sm text-muted-foreground">
                Confidence Score
              </div>
            </div>
            <ConfidenceGauge value={confidenceScore} />
          </div>

          <div className="mt-4 text-sm text-muted-foreground">
            Attested by {attestations.length} verified agent{attestations.length !== 1 ? 's' : ''}
          </div>
        </CardContent>
      </Card>

      {/* Attestations List */}
      <Card>
        <CardHeader>
          <CardTitle>Agent Attestations</CardTitle>
          <CardDescription>
            Verified agents that have successfully connected to this MCP
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Agent</TableHead>
                <TableHead>Trust Score</TableHead>
                <TableHead>Attested</TableHead>
                <TableHead>Expires</TableHead>
                <TableHead>Capabilities</TableHead>
                <TableHead>Status</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {attestations.map((att) => (
                <TableRow key={att.agent_id}>
                  <TableCell>
                    <Link href={`/dashboard/agents/${att.agent_id}`}>
                      {att.agent_name}
                    </Link>
                  </TableCell>
                  <TableCell>
                    <Badge variant="secondary">
                      {att.agent_trust_score.toFixed(1)}
                    </Badge>
                  </TableCell>
                  <TableCell>
                    {formatDateTime(att.verified_at)}
                  </TableCell>
                  <TableCell>
                    {formatDateTime(att.expires_at)}
                  </TableCell>
                  <TableCell>
                    <div className="flex gap-1 flex-wrap">
                      {att.capabilities_confirmed.slice(0, 3).map(cap => (
                        <Badge key={cap} variant="outline" className="text-xs">
                          {cap}
                        </Badge>
                      ))}
                      {att.capabilities_confirmed.length > 3 && (
                        <Badge variant="outline" className="text-xs">
                          +{att.capabilities_confirmed.length - 3} more
                        </Badge>
                      )}
                    </div>
                  </TableCell>
                  <TableCell>
                    {att.health_check_passed ? (
                      <Badge variant="success">Healthy</Badge>
                    ) : (
                      <Badge variant="secondary">No Health Check</Badge>
                    )}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      {/* How Verification Works Info */}
      <Alert>
        <Info className="h-4 w-4" />
        <AlertTitle>How Agent Attestation Works</AlertTitle>
        <AlertDescription>
          When verified agents connect to this MCP, the SDK automatically:
          <ul className="list-disc list-inside mt-2 space-y-1">
            <li>Tests MCP connection and lists capabilities</li>
            <li>Creates a cryptographically signed attestation</li>
            <li>Submits attestation to AIM backend</li>
            <li>Confidence score increases with more attestations</li>
          </ul>
          This happens automatically with zero developer effort!
        </AlertDescription>
      </Alert>
    </div>
  );
}
```

## User Flows

### Flow 1: Promote Auto-Detected MCP (Unchanged)

Same as before - user promotes detection to registered MCP.

### Flow 2: Automatic MCP Attestation (NEW - Zero Effort)

```
1. Developer writes agent code:
   mcp = MCPClient("https://api.anthropic.com/mcp")
   result = mcp.call_tool("prompt", {"text": "Hello"})

2. SDK detects MCP usage automatically
   - Intercepts MCPClient creation
   - Registers MCP with AIM SDK

3. SDK attests MCP in background (async):
   - Tests connection to MCP
   - Lists capabilities
   - Performs health check
   - Creates attestation payload
   - Signs with agent's Ed25519 private key
   - Submits to AIM backend

4. AIM Backend verifies attestation:
   - Fetches agent's public key
   - Verifies Ed25519 signature
   - Checks attestation is recent
   - Stores attestation record
   - Updates confidence score

5. UI updates automatically:
   - MCP confidence score: 0% → 75%
   - Attestation count: 0 → 1
   - "Attested by prod-agent-1" badge appears
   - Last attested: "Just now"

6. More agents use the same MCP:
   - test-agent-2 attests → confidence: 85%
   - api-agent-3 attests → confidence: 95%
   - 3 verified agents attesting = high confidence

Developer writes ZERO extra code!
```

### Flow 3: View MCP Attestations

```
1. User opens MCP detail page
2. Clicks "Verification" tab
3. Sees confidence score: 95%
4. Table shows 3 attestations:
   - prod-agent-1 (trust: 95.5) - 2 hours ago
   - test-agent-2 (trust: 88.0) - 1 day ago
   - api-agent-3 (trust: 92.3) - 3 days ago
5. Each attestation shows:
   - Agent name (clickable → agent detail)
   - Agent trust score
   - When attested
   - When expires (30 days)
   - Capabilities confirmed
   - Health check status
6. User clicks agent name → navigates to agent detail page
```

### Flow 4: View Connected Agents (Unchanged)

Same as before - displays agents connected to MCP.

### Flow 5: Capability Introspection (Unchanged)

Same as before - queries MCP for real capabilities.

## Implementation Order

### Phase 1: Database & Backend Foundation
1. Create migration `039_create_agent_mcp_connections_table.sql`
2. Create migration `040_create_mcp_attestations_table.sql`
3. Create migration `041_update_mcp_servers_for_attestation.sql`
4. Add domain models for `AgentMCPConnection` and `MCPAttestation`
5. Implement repository methods

### Phase 2: Backend API - MCP Promotion (Unchanged)
Same as before

### Phase 3: Backend API - Attestation Handling
1. Implement `VerifyAndRecordAttestation` service method
2. Add `POST /api/v1/mcp-servers/:id/attest` endpoint
3. Implement `updateMCPConfidenceScore` calculation
4. Add `GET /api/v1/mcp-servers/:id/attestations` endpoint

### Phase 4: Backend API - Connections (Unchanged)
Same as before

### Phase 5: Backend API - Introspection (Unchanged)
Same as before

### Phase 6: SDK - Automatic Attestation
1. Add MCP detection in SDK
2. Implement `_attest_mcp` method
3. Add Ed25519 signing for attestations
4. Implement background attestor
5. Test SDK attestation flow

### Phase 7: Frontend - MCP List Page (Updated)
1. Update API client with attestation endpoints
2. Add attestation badge component
3. Update stats to show attested count
4. Add confidence score display

### Phase 8: Frontend - MCP Detail Page (Redesigned)
1. Create attestations table component
2. Redesign Verification tab for attestations
3. Add confidence gauge component
4. Add "How It Works" info section

### Phase 9: Testing & Polish
1. Test attestation flow end-to-end
2. Test with Chrome DevTools MCP
3. Verify confidence score calculation
4. Test attestation expiry (30 days)

## Testing Strategy

### Backend Integration Tests

```go
func TestMCPAttestationFlow(t *testing.T) {
    // 1. Create verified agent
    // 2. Create MCP server
    // 3. Submit attestation
    // 4. Verify signature validation
    // 5. Verify confidence score calculation
    // 6. Verify attestation storage
}

func TestConfidenceScoreCalculation(t *testing.T) {
    // Test confidence formula with different scenarios
}

func TestAttestationExpiry(t *testing.T) {
    // Test attestations expire after 30 days
}

func TestInvalidAttestations(t *testing.T) {
    // Test invalid signatures rejected
    // Test old timestamps rejected
    // Test unverified agents rejected
}
```

### SDK Tests

```python
async def test_automatic_mcp_attestation():
    # 1. Create AIM client with verified agent
    # 2. Use MCP
    # 3. Verify attestation submitted automatically
    # 4. Verify signature valid
```

### Frontend Testing

Use Chrome DevTools MCP to verify:
1. Attestations display correctly
2. Confidence score updates
3. Agent links work
4. Expiry dates show correctly

## Security Considerations

1. **Signature Verification**: All attestations verified using Ed25519 before storage
2. **Agent Verification Required**: Only Ed25519-verified agents can attest
3. **Timestamp Validation**: Attestations must be recent (< 5 minutes old)
4. **Attestation Expiry**: Attestations expire after 30 days
5. **Confidence Decay**: Confidence score decreases if no recent attestations
6. **Unique Attestations**: One attestation per agent per MCP per time period

## Performance Considerations

1. **Async Attestation**: SDK attests in background, doesn't block developer code
2. **Batch Processing**: Backend can process multiple attestations efficiently
3. **Confidence Caching**: Confidence scores cached, recalculated on new attestations
4. **Index Optimization**: Indexes on all query paths
5. **Attestation Pruning**: Expired attestations auto-deleted (background job)

## Success Metrics

1. ✅ Auto-detected MCPs visible in UI
2. ✅ Zero developer effort for MCP verification
3. ✅ Attestations automatically submitted by SDK
4. ✅ Confidence score calculated from multiple agents
5. ✅ Verification tab shows all attestations
6. ✅ Agent links navigate to agent detail page
7. ✅ All tests pass (backend + SDK + frontend)
8. ✅ No console errors in Chrome DevTools

## Architectural Advantages

### Why Agent Attestation Wins

1. **Developer Experience**: Zero effort vs "modify MCP server"
2. **Universal**: Works for ALL MCPs (third-party, local, custom)
3. **Secure**: Cryptographic signatures from verified agents
4. **Continuous**: Re-attestation every 24 hours keeps data fresh
5. **Rich**: Captures capabilities, latency, health checks
6. **Social Proof**: Multiple agents attesting = higher confidence

### Comparison to Alternatives

| Approach | Developer Effort | Works for 3rd Party MCPs | Cryptographically Secure | Continuous Verification |
|----------|-----------------|-------------------------|------------------------|------------------------|
| **MCP Server Signing** | HIGH (modify MCP server) | ❌ No | ✅ Yes | ❌ No |
| **API Key Validation** | MEDIUM (provide API key) | ⚠️ Some | ❌ No | ❌ No |
| **Agent Attestation** | **ZERO** | ✅ Yes | ✅ Yes | ✅ Yes |

### Future Enhancements

1. **Attestation Analytics**: Track MCP uptime, latency trends
2. **Capability Drift Detection**: Alert when MCP capabilities change
3. **Trust Network**: Visualize agent-MCP relationship graph
4. **Attestation Marketplace**: Share trusted MCPs across organizations
5. **Smart Re-attestation**: Attest more frequently if MCP frequently changes

---

**End of Design Document (Agent Attestation Model)**
