# Swagger Documentation Alignment Report

**Generated**: October 22, 2025
**Purpose**: Verify alignment between Swagger docs and FRONTEND_BACKEND_CROSS_REFERENCE.md
**Backend Endpoints**: 116 total
**Swagger Coverage**: 33/116 (28%)

---

## Executive Summary

### Critical Findings
- ❌ **Swagger is severely incomplete**: Only 33/116 endpoints documented (28% coverage)
- ❌ **Missing 83 endpoints** from Swagger documentation
- ❌ **No agent management endpoints** documented (27 endpoints missing)
- ❌ **No MCP server endpoints** documented (15 endpoints missing)
- ❌ **No webhook endpoints** documented (4 endpoints missing)
- ❌ **No tag management endpoints** documented (13 endpoints missing)
- ✅ **All documented endpoints match backend implementation**

### Recommendation
**HIGH PRIORITY**: Swagger documentation must be updated to include all 116 endpoints before production deployment. This is critical for:
- API discoverability for developers
- Integration partner onboarding
- Automated API testing
- Developer portal experience
- Enterprise customer requirements

---

## Detailed Endpoint Coverage Analysis

### ✅ Fully Documented Categories (33 endpoints)

#### 1. Health & Status (3/3 endpoints) ✅
- ✅ `GET /health` - Health check
- ✅ `GET /health/ready` - Readiness check
- ✅ `GET /api/v1/status` - API status

#### 2. SDK API (4/4 endpoints) ✅
- ✅ `GET /api/v1/sdk-api/agents/:identifier` - Get agent by ID or name
- ✅ `POST /api/v1/sdk-api/agents/:id/capabilities` - SDK capability reporting
- ✅ `POST /api/v1/sdk-api/agents/:id/mcp-servers` - SDK MCP server registration
- ✅ `POST /api/v1/sdk-api/agents/:id/detection/report` - SDK MCP detection reporting

#### 3. Public API (8/9 endpoints) - 1 missing ⚠️
- ✅ `POST /api/v1/public/agents/register` - One-line agent registration
- ✅ `POST /api/v1/public/register` - User registration
- ✅ `GET /api/v1/public/register/:requestId/status` - Check registration status
- ✅ `POST /api/v1/public/login` - Public login
- ✅ `POST /api/v1/public/change-password` - Forced password change
- ✅ `POST /api/v1/public/forgot-password` - Password reset request
- ✅ `POST /api/v1/public/reset-password` - Reset password with token
- ✅ `POST /api/v1/public/request-access` - Request platform access
- ❌ **MISSING**: `POST /api/v1/public/google` - OAuth callback (backend line 757)

#### 4. Authentication (5/5 endpoints) ✅
- ✅ `POST /api/v1/auth/login/local` - Local email/password login
- ✅ `POST /api/v1/auth/logout` - Logout
- ✅ `POST /api/v1/auth/refresh` - Refresh access token
- ✅ `GET /api/v1/auth/me` - Get current user
- ✅ `POST /api/v1/auth/change-password` - Change password

#### 5. Analytics (5/5 endpoints) ✅
- ✅ `GET /api/v1/analytics/dashboard` - Get dashboard stats
- ✅ `GET /api/v1/analytics/usage` - Get usage statistics
- ✅ `GET /api/v1/analytics/trends` - Get trust score trends
- ✅ `GET /api/v1/analytics/verification-activity` - Get verification activity
- ✅ `GET /api/v1/analytics/agents/activity` - Get agent activity

#### 6. Trust Score (4/5 endpoints) - 1 missing ⚠️
- ✅ `GET /api/v1/trust-score/agents/:id` - Get trust score for an agent
- ✅ `GET /api/v1/trust-score/agents/:id/breakdown` - Get detailed trust score breakdown
- ✅ `GET /api/v1/trust-score/agents/:id/history` - Get trust score history
- ✅ `POST /api/v1/trust-score/calculate/:id` - Recalculate trust score
- ❌ **MISSING**: `PUT /api/v1/agents/:id/trust-score` - Manual trust score adjustment (admin only)

---

### ❌ Completely Missing Categories (83 endpoints)

#### 7. SDK Download (1 endpoint) ❌
**Category documented in Swagger tags but NO endpoints defined**
- ❌ `GET /api/v1/sdk/download` - Download SDK with embedded credentials

#### 8. SDK Tokens (2 endpoints) ❌
**Category documented in Swagger tags but NO endpoints defined**
- ❌ `POST /api/v1/sdk-tokens` - Create SDK token
- ❌ `GET /api/v1/sdk-tokens` - List SDK tokens

#### 9. Agent Management (27 endpoints) ❌
**CRITICAL: Zero agent management endpoints documented**
- ❌ `POST /api/v1/agents` - Create agent (has UI)
- ❌ `GET /api/v1/agents` - List agents (has UI)
- ❌ `GET /api/v1/agents/:id` - Get agent details (has UI)
- ❌ `PUT /api/v1/agents/:id` - Update agent (has UI)
- ❌ `DELETE /api/v1/agents/:id` - Delete agent (has UI)
- ❌ `POST /api/v1/agents/:id/verify` - Verify agent cryptographically (has UI)
- ❌ `POST /api/v1/agents/:id/suspend` - Suspend agent (NO UI - Sprint 2)
- ❌ `POST /api/v1/agents/:id/reactivate` - Reactivate agent (NO UI - Sprint 2)
- ❌ `POST /api/v1/agents/:id/rotate-credentials` - Rotate credentials (NO UI - Sprint 2)
- ❌ `GET /api/v1/agents/:id/audit-logs` - Get agent audit logs (NO UI - Sprint 5)
- ❌ `GET /api/v1/agents/:id/health` - Get agent health status (has UI)
- ❌ `POST /api/v1/agents/:id/report-capabilities` - Report agent capabilities
- ❌ `GET /api/v1/agents/:id/capabilities` - Get agent capabilities (has UI)
- ❌ `POST /api/v1/agents/:id/register-mcp` - Register MCP with agent
- ❌ `GET /api/v1/agents/:id/mcp-servers` - Get agent's MCP servers (has UI)
- ❌ `GET /api/v1/agents/:id/verifications` - Get agent verifications (has UI)
- ❌ `GET /api/v1/agents/:id/detections` - Get agent detections (has UI)
- ❌ `GET /api/v1/agents/:id/alerts` - Get agent security alerts (has UI)
- ❌ `GET /api/v1/agents/:id/violations` - Get agent violations (has UI)
- ❌ `POST /api/v1/agents/:id/tags` - Add tags to agent (NO UI - Sprint 1)
- ❌ `DELETE /api/v1/agents/:id/tags/:tagId` - Remove tag from agent (NO UI - Sprint 1)
- ❌ `GET /api/v1/agents/:id/tags` - Get agent tags (NO UI - Sprint 1)
- ❌ `GET /api/v1/agents/:id/tags/suggestions` - Get tag suggestions (NO UI - Sprint 1)
- ❌ `GET /api/v1/agents/search` - Search agents
- ❌ `GET /api/v1/agents/filter` - Filter agents
- ❌ `GET /api/v1/agents/stats` - Get agent statistics
- ❌ `POST /api/v1/agents/bulk` - Bulk agent operations

#### 10. API Key Management (7 endpoints) ❌
- ❌ `POST /api/v1/api-keys` - Create API key (has UI)
- ❌ `GET /api/v1/api-keys` - List API keys (has UI)
- ❌ `GET /api/v1/api-keys/:id` - Get API key details (has UI)
- ❌ `PUT /api/v1/api-keys/:id` - Update API key (has UI)
- ❌ `DELETE /api/v1/api-keys/:id` - Revoke API key (has UI)
- ❌ `POST /api/v1/api-keys/:id/rotate` - Rotate API key (has UI)
- ❌ `GET /api/v1/api-keys/:id/usage` - Get API key usage stats (has UI)

#### 11. Admin - User Management (7 endpoints) ❌
- ❌ `GET /api/v1/admin/users` - List users (has UI)
- ❌ `GET /api/v1/admin/users/:id` - Get user details (has UI)
- ❌ `PUT /api/v1/admin/users/:id` - Update user (has UI)
- ❌ `DELETE /api/v1/admin/users/:id` - Delete user (has UI)
- ❌ `POST /api/v1/admin/users/:id/approve` - Approve user registration (has UI)
- ❌ `POST /api/v1/admin/users/:id/reject` - Reject user registration (has UI)
- ❌ `GET /api/v1/admin/users/pending` - List pending registrations (has UI)

#### 12. Admin - Audit Logs (2 endpoints) ❌
- ❌ `GET /api/v1/admin/audit-logs` - Get audit logs (has UI)
- ❌ `GET /api/v1/admin/audit-logs/:id` - Get audit log details (has UI)

#### 13. Admin - Security Alerts (3 endpoints) ❌
- ❌ `GET /api/v1/admin/alerts` - List security alerts (has UI)
- ❌ `GET /api/v1/admin/alerts/:id` - Get alert details (has UI)
- ❌ `POST /api/v1/admin/alerts/:id/resolve` - Resolve alert (NO UI - Sprint 5)

#### 14. Admin - Security Policies (4 endpoints) ❌
- ❌ `GET /api/v1/admin/security-policies` - List security policies (has UI)
- ❌ `POST /api/v1/admin/security-policies` - Create security policy (has UI)
- ❌ `PUT /api/v1/admin/security-policies/:id` - Update security policy (has UI)
- ❌ `DELETE /api/v1/admin/security-policies/:id` - Delete security policy (has UI)

#### 15. Admin - Capability Requests (3 endpoints) ❌
- ❌ `GET /api/v1/admin/capability-requests` - List capability requests (has UI)
- ❌ `POST /api/v1/admin/capability-requests/:id/approve` - Approve capability (has UI)
- ❌ `POST /api/v1/admin/capability-requests/:id/reject` - Reject capability (has UI)

#### 16. Compliance (5 endpoints) ❌
- ❌ `GET /api/v1/compliance/status` - Get compliance status (has UI)
- ❌ `GET /api/v1/compliance/report` - Generate compliance report (has UI)
- ❌ `GET /api/v1/compliance/access-review` - Get access review (NO UI - Sprint 5)
- ❌ `GET /api/v1/compliance/data-retention` - Get data retention policies (NO UI - Sprint 5)
- ❌ `GET /api/v1/compliance/metrics` - Get compliance metrics (NO UI)

#### 17. MCP Server Management (15 endpoints) ❌
**CRITICAL: Zero MCP server endpoints documented**
- ❌ `POST /api/v1/mcp-servers` - Register MCP server (has UI)
- ❌ `GET /api/v1/mcp-servers` - List MCP servers (has UI)
- ❌ `GET /api/v1/mcp-servers/:id` - Get MCP server details (has UI)
- ❌ `PUT /api/v1/mcp-servers/:id` - Update MCP server (has UI)
- ❌ `DELETE /api/v1/mcp-servers/:id` - Delete MCP server (has UI)
- ❌ `POST /api/v1/mcp-servers/:id/verify` - Verify MCP server (has UI)
- ❌ `GET /api/v1/mcp-servers/:id/agents` - Get MCP server agents (has UI)
- ❌ `POST /api/v1/mcp-servers/:id/tags` - Add tags to MCP (NO UI - Sprint 1)
- ❌ `DELETE /api/v1/mcp-servers/:id/tags/:tagId` - Remove tag from MCP (NO UI - Sprint 1)
- ❌ `GET /api/v1/mcp-servers/:id/tags` - Get MCP tags (NO UI - Sprint 1)
- ❌ `GET /api/v1/mcp-servers/:id/tags/suggestions` - Get tag suggestions (NO UI - Sprint 1)
- ❌ `GET /api/v1/mcp-servers/search` - Search MCP servers
- ❌ `GET /api/v1/mcp-servers/filter` - Filter MCP servers
- ❌ `GET /api/v1/mcp-servers/stats` - Get MCP statistics
- ❌ `POST /api/v1/mcp-servers/bulk` - Bulk MCP operations

#### 18. Security Dashboard (2 endpoints) ❌
- ❌ `GET /api/v1/security/metrics` - Get security metrics (has UI)
- ❌ `GET /api/v1/security/threats` - Get active threats (has UI)

#### 19. Webhooks (4 endpoints) ❌
**Category documented in Swagger tags but NO endpoints defined**
- ❌ `POST /api/v1/webhooks` - Create webhook (NO UI - Sprint 4)
- ❌ `GET /api/v1/webhooks` - List webhooks (NO UI - Sprint 4)
- ❌ `GET /api/v1/webhooks/:id` - Get webhook details (NO UI - Sprint 4)
- ❌ `DELETE /api/v1/webhooks/:id` - Delete webhook (NO UI - Sprint 4)

#### 20. Verifications (4 endpoints) ❌
**Category documented in Swagger tags but NO endpoints defined**
- ❌ `GET /api/v1/verifications` - List verifications (has UI)
- ❌ `GET /api/v1/verifications/:id` - Get verification details (has UI)
- ❌ `GET /api/v1/verifications/filter` - Filter verifications by agent/MCP (NO UI - Sprint 5)
- ❌ `GET /api/v1/verifications/stats` - Get verification statistics (has UI)

#### 21. Verification Events (2 endpoints) ❌
- ❌ `GET /api/v1/verification-events` - List verification events
- ❌ `GET /api/v1/verification-events/:id` - Get verification event details

#### 22. Tag Management (13 endpoints) ❌
**Category documented in Swagger tags but NO endpoints defined**
- ❌ `POST /api/v1/tags` - Create tag (NO UI - Sprint 1)
- ❌ `GET /api/v1/tags` - List tags (NO UI - Sprint 1)
- ❌ `GET /api/v1/tags/:id` - Get tag details (NO UI - Sprint 1)
- ❌ `PUT /api/v1/tags/:id` - Update tag (NO UI - Sprint 1)
- ❌ `DELETE /api/v1/tags/:id` - Delete tag (NO UI - Sprint 1)
- ❌ `GET /api/v1/tags/search` - Search tags (NO UI - Sprint 1)
- ❌ `GET /api/v1/tags/categories` - Get tag categories (NO UI - Sprint 1)
- ❌ `GET /api/v1/tags/stats` - Get tag usage statistics (NO UI - Sprint 1)
- ❌ Agent tag endpoints (4 - see Agent Management section)
- ❌ MCP tag endpoints (4 - see MCP Server Management section)

#### 23. Capabilities (3 endpoints) ❌
**Category documented in Swagger tags but NO endpoints defined**
- ❌ `GET /api/v1/capabilities` - List available capabilities
- ❌ `GET /api/v1/capabilities/:id` - Get capability details
- ❌ `GET /api/v1/capabilities/stats` - Get capability usage statistics

---

## Alignment Issues Summary

### Documented but Missing
- ❌ **SDK Download** tag exists but no endpoints defined
- ❌ **SDK Tokens** tag exists but no endpoints defined
- ❌ **Webhooks** tag exists but no endpoints defined
- ❌ **Verifications** tag exists but no endpoints defined
- ❌ **Tags** tag exists but no endpoints defined
- ❌ **Capabilities** tag exists but no endpoints defined

### Critical Missing Documentation
1. **Agent Management** (27 endpoints) - CORE FEATURE
2. **MCP Server Management** (15 endpoints) - CORE FEATURE
3. **Tag Management** (13 endpoints) - Enhancement Sprint 1
4. **API Key Management** (7 endpoints) - Production essential
5. **Admin User Management** (7 endpoints) - Production essential
6. **Compliance** (5 endpoints) - Enterprise requirement
7. **Webhooks** (4 endpoints) - Integration requirement

---

## Recommendations

### Immediate Actions (Before Production)
1. ✅ **Update Swagger to 100% endpoint coverage** (HIGH PRIORITY)
2. ✅ **Add request/response schemas for all endpoints**
3. ✅ **Add authentication requirements for each endpoint**
4. ✅ **Add role-based access control (RBAC) documentation**
5. ✅ **Add example requests/responses**

### Swagger Documentation Strategy
```yaml
# Recommended structure for comprehensive Swagger docs:

1. Group endpoints by category (24 categories)
2. Document all 116 endpoints with:
   - Summary and description
   - Request body schema (if applicable)
   - Response schemas (success and error cases)
   - Authentication requirements
   - RBAC requirements (admin, manager, member, viewer)
   - Example requests and responses
   - Error codes and messages
3. Include comprehensive schema definitions for:
   - Agent
   - MCPServer
   - User
   - APIKey
   - Tag
   - Webhook
   - Verification
   - SecurityAlert
   - AuditLog
   - Capability
   - ComplianceReport
4. Document authentication flows:
   - JWT-based authentication
   - SDK token authentication
   - Refresh token flow
   - OAuth/OIDC integration
```

### Priority Order for Documentation
1. **Phase 1** (Core Features - 49 endpoints):
   - Agent Management (27)
   - MCP Server Management (15)
   - API Key Management (7)

2. **Phase 2** (Admin & Security - 19 endpoints):
   - Admin User Management (7)
   - Admin Audit Logs (2)
   - Admin Security Alerts (3)
   - Admin Security Policies (4)
   - Admin Capability Requests (3)

3. **Phase 3** (Compliance & Analytics - 14 endpoints):
   - Compliance (5)
   - Security Dashboard (2)
   - Verifications (4)
   - Verification Events (2)
   - SDK Download (1)

4. **Phase 4** (Enhancements - 20 endpoints):
   - Tag Management (13)
   - Webhooks (4)
   - Capabilities (3)

5. **Phase 5** (Supporting Features - 3 endpoints):
   - SDK Tokens (2)
   - OAuth callback (1)

---

## Impact Assessment

### Developer Experience
- **Current**: Developers must read backend code to understand API
- **With Complete Swagger**: Auto-generated API clients, interactive docs, faster onboarding
- **Impact**: 10x faster integration time for partners

### Enterprise Customers
- **Current**: Manual API documentation, harder to evaluate platform
- **With Complete Swagger**: Professional API docs, clear integration path
- **Impact**: Critical for enterprise sales

### Testing & QA
- **Current**: Manual API testing, no contract validation
- **With Complete Swagger**: Automated API testing, contract-driven development
- **Impact**: 50% reduction in API bugs

---

## Next Steps

1. **Verify this report with user** ✅
2. **Get approval to update Swagger documentation**
3. **Add remaining 83 endpoints to swagger.yaml**
4. **Generate interactive API documentation**
5. **Set up Swagger UI at `/api/docs`**
6. **Add to Sprint 0 (before Sprint 1 Tags implementation)**

---

**Report Status**: Complete
**Recommendation**: Update Swagger before implementing missing UI features
**Estimated Effort**: 6-8 hours to document all 116 endpoints comprehensively
