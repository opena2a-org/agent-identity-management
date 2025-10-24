# MCP Enhancement Implementation Prompt (Agent Attestation Model)

**For New Claude Session - Copy and paste this entire prompt:**

---

I need you to implement the MCP (Model Context Protocol) enhancements for the Agent Identity Management system using the **Agent Attestation approach** according to the detailed design document:

**Design Doc**: `docs/plans/2025-10-23-mcp-enhancement-design.md` (Version 2.0 - Agent Attestation Model)

## Revolutionary Approach: Agent Attestation

**Key Insight**: Developers don't control MCPs (most are third-party: Anthropic, GitHub, OpenAI). Instead of asking MCPs to verify themselves, **verified agents attest to MCP identity**.

### The Magic: Zero Developer Effort

```python
# Developer writes normal code (unchanged):
from aim_sdk import AIMClient
from mcp import Client as MCPClient

aim = AIMClient(agent_id="...", private_key="...")
mcp = MCPClient("https://api.anthropic.com/mcp")
result = mcp.call_tool("prompt", {"text": "Hello"})

# ✨ SDK AUTOMATICALLY (in background):
# 1. Detects MCP usage
# 2. Tests connection and lists capabilities
# 3. Creates attestation payload
# 4. Signs with agent's Ed25519 private key
# 5. Submits to AIM backend
# 6. MCP marked as "Attested by agent-name" with confidence score

# Developer writes ZERO extra code!
```

## Context

This is an enterprise-grade Go + Next.js application for AI agent identity management. The system already has:
- ✅ Agent Ed25519 verification working
- ✅ MCP auto-detection (`agent_mcp_detections` table)
- ✅ MCP registration (`mcp_servers` table)
- ❌ **Missing**: Connection between detections and registrations
- ❌ **Missing**: MCP verification mechanism

## Your Mission

Implement the Agent Attestation model following the **9-phase implementation order**:

### Phase 1: Database & Backend Foundation
**New Tables**:
1. `agent_mcp_connections` - Bidirectional agent ↔ MCP relationships
2. `mcp_attestations` - **THE KEY INNOVATION** - Cryptographically signed attestations from verified agents
3. Update `mcp_servers` - Add attestation-related columns

**Files to create**:
- `apps/backend/migrations/039_create_agent_mcp_connections_table.sql`
- `apps/backend/migrations/040_create_mcp_attestations_table.sql`
- `apps/backend/migrations/041_update_mcp_servers_for_attestation.sql`

### Phase 2: Backend API - MCP Promotion
- Implement `PromoteDetectionToMCPServer` service method
- Add `POST /api/v1/mcp-servers/promote/:detection_id` endpoint
- Update `GET /api/v1/mcp-servers?include_detected=true`

### Phase 3: Backend API - Attestation Handling (THE CORE INNOVATION)
**Implement Agent Attestation Verification**:

1. `POST /api/v1/mcp-servers/:id/attest` endpoint (called by SDK)
   - Verify agent is Ed25519-verified
   - Verify Ed25519 signature of attestation
   - Check attestation timestamp (< 5 min old)
   - Store attestation
   - Calculate confidence score

2. Confidence Score Formula:
   ```go
   // Factors:
   // - Number of agents attesting (20 points each, max 5 = 100)
   // - Average trust score of agents (0-50 points)
   // - Recency of attestations (0-30 points)

   agentPoints := min(len(agents) * 20.0, 100.0)
   trustPoints := (avgTrustScore / 100.0) * 50.0
   recencyPoints := (recentCount / totalCount) * 30.0
   confidence := (agentPoints + trustPoints + recencyPoints) / 1.8
   ```

3. `GET /api/v1/mcp-servers/:id/attestations` endpoint

### Phase 4: Backend API - Connections
- Implement `GetConnectedAgents` service method
- Add `GET /api/v1/mcp-servers/:id/agents` endpoint
- Implement `GetMCPServersForAgent` service method
- Add `GET /api/v1/agents/:id/mcp-servers` endpoint

### Phase 5: Backend API - Introspection
- Implement `IntrospectMCPCapabilities` service method
- Add `GET /api/v1/mcp-servers/:id/capabilities/introspect` endpoint

### Phase 6: SDK - Automatic Attestation (THE MAGIC)
**Enhance Python SDK** (`sdks/python/aim_sdk/client.py`):

1. Add `register_mcp_usage()` method (auto-called on MCP detection)
2. Implement `_attest_mcp()` method:
   - Test MCP connection
   - List capabilities
   - Health check (if supported)
   - Create attestation payload
   - Sign with agent's Ed25519 private key
   - Submit to backend

3. Implement background re-attestation (every 24 hours)

**Critical**: This must be **automatic and invisible** to developers!

### Phase 7: Frontend - MCP List Page
1. Update API client with attestation endpoints
2. Create `<AttestationBadge />` component
   - Shows confidence score
   - Shows attestation count
   - Color-coded by confidence (green ≥80%, yellow ≥50%, gray <50%)
3. Add stats cards:
   - Total MCPs
   - Attested MCPs
   - Avg Confidence
   - Auto-Detected

### Phase 8: Frontend - MCP Detail Page (REDESIGNED Verification Tab)
**Replace "Verify MCP" button with Attestations Display**:

1. Create `<AttestationsTable />` component showing:
   - Agent name (clickable → agent detail)
   - Agent trust score
   - When attested
   - When expires (30 days)
   - Capabilities confirmed
   - Health check status

2. Create `<ConfidenceGauge />` component (0-100% visual gauge)

3. Add "How It Works" info section explaining Agent Attestation

### Phase 9: Testing & Polish
1. Test attestation flow end-to-end with real SDK
2. Test with Chrome DevTools MCP
3. Verify confidence score calculation
4. Test signature verification rejects invalid attestations

## Critical Requirements

1. **Follow the design document exactly** - Agent Attestation approach, NOT MCP server signing
2. **Zero developer effort** - SDK handles attestation automatically
3. **Cryptographic security** - All attestations Ed25519 signed and verified
4. **Maintain naming consistency** - Follow `CLAUDE.md` conventions
5. **Test with Chrome DevTools MCP** - Verify frontend works perfectly
6. **Commit incrementally** - Each phase gets its own commit

## Key Files to Reference

**Design Document**:
- `docs/plans/2025-10-23-mcp-enhancement-design.md` - **READ THIS FIRST!**

**Backend**:
- `apps/backend/internal/application/mcp_service.go` - Add attestation methods
- `apps/backend/internal/interfaces/http/handlers/mcp_handler.go` - Add endpoints
- `apps/backend/internal/infrastructure/crypto/ed25519_service.go` - Use for signature verification
- `apps/backend/internal/domain/mcp.go` - Add domain models

**SDK**:
- `sdks/python/aim_sdk/client.py` - Add automatic attestation

**Frontend**:
- `apps/web/app/dashboard/mcp/page.tsx` - MCP list page
- `apps/web/app/dashboard/mcp/[id]/page.tsx` - MCP detail page
- `apps/web/lib/api.ts` - API client
- `apps/web/components/mcp/` - Create new MCP components

**Database**:
- `apps/backend/migrations/` - Create migrations 039, 040, 041

## Attestation Data Structure

**Attestation Payload** (signed by agent):
```json
{
  "agent_id": "uuid",
  "mcp_url": "https://api.anthropic.com/mcp",
  "mcp_name": "Anthropic MCP",
  "capabilities_found": ["prompt", "completion", "tool_use"],
  "connection_successful": true,
  "health_check_passed": true,
  "connection_latency_ms": 45,
  "timestamp": "2025-10-23T18:00:00Z",
  "sdk_version": "1.0.0"
}
```

**Backend Verification**:
1. Fetch agent's public key from database
2. Verify Ed25519 signature: `VerifyEd25519(publicKey, payload, signature)`
3. Check timestamp is recent (< 5 minutes old)
4. Store if valid

## Success Criteria

- ✅ Auto-detected MCPs visible on MCP list page
- ✅ Users can promote detections to registered MCPs
- ✅ SDK automatically attests MCPs with **zero developer code**
- ✅ Backend verifies Ed25519 signatures on attestations
- ✅ Confidence score calculated from multiple agents
- ✅ Verification tab shows attestations table (not "Verify" button)
- ✅ Agent names link to agent detail pages
- ✅ All backend tests pass (`go test ./...`)
- ✅ No console errors in browser (test with Chrome DevTools MCP)

## Implementation Style

- **Use Test-Driven Development** - Write tests before implementation
- **Use brainstorming skill** if you encounter design questions
- **Use systematic-debugging skill** if you hit errors
- **Use Chrome DevTools MCP** for all frontend testing
- **Follow CLAUDE.md guidelines** - Check naming conventions

## What Makes This Genius

**Traditional approach** (doesn't work):
```
AIM → MCP Server (please sign this challenge)
      ↑
      We don't control third-party MCPs!
```

**Agent Attestation** (works perfectly):
```
AIM → Verified Agent (attest this MCP) → MCP (test connection)
      ↑
      We control the agent via SDK!
```

**Benefits**:
- ✅ Works for ALL MCPs (Anthropic, GitHub, OpenAI, filesystem, sqlite, etc.)
- ✅ Zero developer effort (automatic in SDK)
- ✅ Cryptographically secure (Ed25519 signatures)
- ✅ Continuous verification (re-attest every 24 hours)
- ✅ Rich data (capabilities, latency, health checks)
- ✅ Social proof (multiple agents = higher confidence)

## Start Here

1. **Read the design document**: `docs/plans/2025-10-23-mcp-enhancement-design.md`
2. Review existing Ed25519 agent verification to mirror for attestations
3. Create Phase 1 migrations and test them
4. Proceed through phases sequentially
5. Test each phase before moving to next

**Ready to begin?** Start with Phase 1: Database & Backend Foundation.

---

**Key Difference from Traditional Verification**:
- ❌ **Don't** ask MCPs to sign challenges (we don't control them)
- ✅ **Do** have verified agents attest to MCPs (we control agents via SDK)

This is the breakthrough that makes MCP verification actually work in the real world! 🚀

---

**End of Implementation Prompt**
