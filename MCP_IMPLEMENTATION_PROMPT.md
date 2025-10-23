# MCP Enhancement Implementation Prompt

**For New Claude Session - Copy and paste this entire prompt:**

---

I need you to implement the MCP (Model Context Protocol) enhancements for the Agent Identity Management system according to the detailed design document located at:

**Design Doc**: `docs/plans/2025-10-23-mcp-enhancement-design.md`

## Context

This is an enterprise-grade Go + Next.js application for managing AI agent identities. The current implementation has auto-detection of MCPs (stored in `agent_mcp_detections`) and user registration of MCPs (stored in `mcp_servers`), but these two systems are disconnected.

## Your Mission

Implement the complete MCP enhancement design following the **7-phase implementation order** specified in the design doc:

### Phase 1: Database & Backend Foundation
- Create migration `039_create_agent_mcp_connections_table.sql`
- Add domain models for `AgentMCPConnection`
- Implement repository methods

### Phase 2: Backend API - MCP Promotion
- Implement `PromoteDetectionToMCPServer` service method
- Add `POST /api/v1/mcp-servers/promote/:detection_id` endpoint
- Update `GET /api/v1/mcp-servers?include_detected=true`

### Phase 3: Backend API - Connections
- Implement `GetConnectedAgents` service method
- Add `GET /api/v1/mcp-servers/:id/agents` endpoint
- Implement `GetMCPServersForAgent` service method
- Add `GET /api/v1/agents/:id/mcp-servers` endpoint

### Phase 4: Backend API - Verification & Introspection
- Implement `VerifyMCPWithEd25519` service method (cryptographic verification)
- Add `POST /api/v1/mcp-servers/:id/verify` endpoint
- Implement `IntrospectMCPCapabilities` service method
- Add `GET /api/v1/mcp-servers/:id/capabilities/introspect` endpoint

### Phase 5: Frontend - MCP List Page
- Update API client with new endpoints
- Create `<PromoteMCPModal />` component
- Add "Auto-Detected MCPs" section to list page
- Wire promotion flow

### Phase 6: Frontend - MCP Detail Page
- Create `<ConnectedAgentsTable />` component
- Add "Connected Agents" tab
- Create `<MCPIntrospectionButton />` component
- Enhance "Capabilities" tab with real data
- Create `<Ed25519VerificationModal />` component
- Wire "Verify" button to Ed25519 verification

### Phase 7: Testing & Polish
- Test all flows with Chrome DevTools MCP
- Verify no console errors
- Ensure proper error handling and loading states

## Critical Requirements

1. **Follow the design document exactly** - All API signatures, database schema, and UI flows are specified
2. **Use existing patterns** - The codebase already has Ed25519 verification for agents; mirror that for MCPs
3. **Maintain naming consistency** - Follow the naming conventions in `CLAUDE.md` (snake_case DB, camelCase JSON/TS)
4. **Test with Chrome DevTools MCP** - Use `mcp__chrome-devtools__*` tools to verify frontend works
5. **Commit incrementally** - Commit after each phase with clear messages

## Key Files to Reference

**Backend:**
- `apps/backend/internal/application/mcp_service.go` - Add new methods here
- `apps/backend/internal/interfaces/http/handlers/mcp_handler.go` - Add new endpoints
- `apps/backend/internal/infrastructure/crypto/ed25519_service.go` - Use for verification
- `apps/backend/internal/domain/mcp.go` - Add new domain models

**Frontend:**
- `apps/web/app/dashboard/mcp/page.tsx` - MCP list page
- `apps/web/app/dashboard/mcp/[id]/page.tsx` - MCP detail page
- `apps/web/lib/api.ts` - API client
- `apps/web/components/modals/` - Create new modals here

**Database:**
- `apps/backend/migrations/` - Create migration 039

## Success Criteria

- ✅ Auto-detected MCPs visible on MCP list page
- ✅ Users can promote detections to registered MCPs with Ed25519 key generation
- ✅ "Connected Agents" tab shows all agents using the MCP
- ✅ "Capabilities" tab displays real capabilities from introspection
- ✅ "Verify" button performs Ed25519 cryptographic verification
- ✅ All backend tests pass (`go test ./...`)
- ✅ No console errors in browser (test with Chrome DevTools MCP)
- ✅ All changes committed to git with descriptive messages

## Implementation Style

- **Use Test-Driven Development** - Write tests before implementation where applicable
- **Use brainstorming skill** if you encounter architectural questions
- **Use systematic-debugging skill** if you hit errors
- **Use Chrome DevTools MCP** for all frontend testing
- **Follow CLAUDE.md guidelines** - Check naming conventions before creating any new field

## Start Here

1. Read the design document: `docs/plans/2025-10-23-mcp-enhancement-design.md`
2. Review existing MCP implementation to understand patterns
3. Create Phase 1 migration and test it
4. Proceed through phases sequentially
5. Test each phase before moving to next

**Ready to begin?** Start with Phase 1: Database & Backend Foundation.

---

**End of Implementation Prompt**
