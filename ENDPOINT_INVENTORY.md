# AIM Backend API Endpoint Inventory
**Generated**: 2025-01-21
**Source**: `/Users/decimai/workspace/agent-identity-management/apps/backend/cmd/server/main.go`

## Total Endpoint Count: 116 endpoints

---

## 1. Health & Status (3 endpoints)
| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| GET | `/health` | None | Health check |
| GET | `/health/ready` | None | Readiness check |
| GET | `/api/v1/status` | None | API status |

---

## 2. SDK API (4 endpoints)
**Base**: `/api/v1/sdk-api`

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| GET | `/agents/:identifier` | SDK Token | Get agent by ID/name (SDK only) |
| POST | `/agents/:id/capabilities` | SDK Token | SDK capability reporting |
| POST | `/agents/:id/mcp-servers` | SDK Token | SDK MCP registration |
| POST | `/agents/:id/detection/report` | SDK Token | SDK MCP detection reporting |

---

## 3. Public API (9 endpoints)
**Base**: `/api/v1/public`

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| POST | `/agents/register` | None | **ONE-LINE** agent registration |
| POST | `/register` | None | **USER REGISTRATION** |
| GET | `/register/:requestId/status` | None | Check registration status |
| POST | `/login` | None | Public login |
| POST | `/change-password` | JWT | Forced password change |
| POST | `/forgot-password` | None | Password reset request |
| POST | `/reset-password` | None | Password reset with token |
| POST | `/request-access` | None | Request platform access |

---

## 4. Authentication (5 endpoints)
**Base**: `/api/v1/auth`

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| POST | `/login/local` | None | Local email/password login |
| POST | `/logout` | JWT | Logout |
| POST | `/refresh` | Refresh Token | Refresh access token |
| GET | `/me` | JWT | Get current user |
| POST | `/change-password` | JWT | Change password |

---

## 5. SDK Download (1 endpoint)
**Base**: `/api/v1/sdk`

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| GET | `/download` | JWT | Download Python SDK with embedded credentials |

---

## 6. SDK Tokens (4 endpoints)
**Base**: `/api/v1/sdk-tokens`

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| GET | `/` | JWT | List all SDK tokens |
| GET | `/count` | JWT | Get active token count |
| POST | `/:id/revoke` | JWT | Revoke specific token |
| POST | `/revoke-all` | JWT | Revoke all tokens |

---

## 7. Detection (3 endpoints)
**Base**: `/api/v1/detection`

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| POST | `/agents/:id/report` | JWT | Report detection |
| GET | `/agents/:id/status` | JWT | Get detection status |
| POST | `/agents/:id/capabilities/report` | JWT | Report capabilities |

---

## 8. Agents (27 endpoints)
**Base**: `/api/v1/agents`

### Core CRUD
| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| GET | `/` | JWT | List all agents |
| POST | `/` | JWT (Member+) | Create agent |
| GET | `/:id` | JWT | Get agent details |
| PUT | `/:id` | JWT (Member+) | Update agent |
| DELETE | `/:id` | JWT (Manager+) | Delete agent |

### Agent Lifecycle
| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| POST | `/:id/verify` | JWT (Manager+) | Verify agent |
| POST | `/:id/suspend` | JWT (Manager+) | Suspend agent |
| POST | `/:id/reactivate` | JWT (Manager+) | Reactivate agent |
| POST | `/:id/rotate-credentials` | JWT (Member+) | Rotate credentials |

### Agent Actions
| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| POST | `/:id/verify-action` | JWT | Verify agent action |
| POST | `/:id/log-action/:audit_id` | JWT | Log action result |

### Agent SDK
| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| GET | `/:id/sdk` | JWT | Download SDK for agent |
| GET | `/:id/credentials` | JWT | Get agent credentials |

### MCP Server Relationships
| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| GET | `/:id/mcp-servers` | JWT | Get MCP servers agent talks to |
| PUT | `/:id/mcp-servers` | JWT (Member+) | Add MCP servers (bulk) |
| DELETE | `/:id/mcp-servers/:mcp_id` | JWT (Member+) | Remove MCP server |
| POST | `/:id/mcp-servers/detect` | JWT (Member+) | Auto-detect MCPs from config |

### Trust Score
| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| GET | `/:id/trust-score` | JWT | Get current trust score |
| GET | `/:id/trust-score/history` | JWT | Get trust score history |
| PUT | `/:id/trust-score` | JWT (Admin) | Manually update score |
| POST | `/:id/trust-score/recalculate` | JWT (Manager+) | Recalculate score |

### Agent Security
| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| GET | `/:id/key-vault` | JWT | Get key vault info |
| GET | `/:id/audit-logs` | JWT | Get agent audit logs |

### Agent Tags
| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| GET | `/:id/tags` | JWT | Get agent tags |
| POST | `/:id/tags` | JWT (Member+) | Add tags to agent |
| DELETE | `/:id/tags/:tagId` | JWT (Member+) | Remove tag from agent |
| GET | `/:id/tags/suggestions` | JWT | Get tag suggestions |

### Agent Capabilities
| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| GET | `/:id/capabilities` | JWT | Get agent capabilities |
| POST | `/:id/capabilities` | JWT (Manager+) | Grant capability |
| DELETE | `/:id/capabilities/:capabilityId` | JWT (Manager+) | Revoke capability |

### Agent Violations
| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| GET | `/:id/violations` | JWT | Get violations by agent |

---

## 9. API Keys (4 endpoints)
**Base**: `/api/v1/api-keys`

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| GET | `/` | JWT | List API keys |
| POST | `/` | JWT (Member+) | Create API key |
| PATCH | `/:id/disable` | JWT (Member+) | Disable API key |
| DELETE | `/:id` | JWT (Member+) | Delete API key |

---

## 10. Trust Score (3 endpoints)
**Base**: `/api/v1/trust-score`

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| POST | `/calculate/:id` | JWT (Manager+) | Calculate trust score |
| GET | `/agents/:id` | JWT | Get trust score |
| GET | `/agents/:id/history` | JWT | Get trust score history |

---

## 11. Admin - User Management (12 endpoints)
**Base**: `/api/v1/admin`

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| GET | `/users` | JWT (Admin) | List all users |
| GET | `/users/pending` | JWT (Admin) | Get pending users |
| POST | `/users/:id/approve` | JWT (Admin) | Approve user |
| POST | `/users/:id/reject` | JWT (Admin) | Reject user |
| PUT | `/users/:id/role` | JWT (Admin) | Update user role |
| POST | `/users/:id/deactivate` | JWT (Admin) | Deactivate user (soft delete) |
| POST | `/users/:id/activate` | JWT (Admin) | Activate user |
| DELETE | `/users/:id` | JWT (Admin) | Permanently delete user |
| POST | `/registration-requests/:id/approve` | JWT (Admin) | Approve registration request |
| POST | `/registration-requests/:id/reject` | JWT (Admin) | Reject registration request |
| GET | `/organization/settings` | JWT (Admin) | Get org settings |
| PUT | `/organization/settings` | JWT (Admin) | Update org settings |

---

## 12. Admin - Audit & Monitoring (5 endpoints)

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| GET | `/admin/audit-logs` | JWT (Admin) | Get audit logs |
| GET | `/admin/alerts` | JWT (Admin) | Get alerts |
| GET | `/admin/alerts/unacknowledged/count` | JWT (Admin) | Get unacknowledged count |
| POST | `/admin/alerts/:id/acknowledge` | JWT (Admin) | Acknowledge alert |
| POST | `/admin/alerts/:id/resolve` | JWT (Admin) | Resolve alert |

---

## 13. Admin - Dashboard (1 endpoint)

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| GET | `/admin/dashboard/stats` | JWT (Admin) | Get dashboard stats |

---

## 14. Admin - Security Policies (6 endpoints)

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| GET | `/admin/security-policies` | JWT (Admin) | List policies |
| GET | `/admin/security-policies/:id` | JWT (Admin) | Get policy |
| POST | `/admin/security-policies` | JWT (Admin) | Create policy |
| PUT | `/admin/security-policies/:id` | JWT (Admin) | Update policy |
| DELETE | `/admin/security-policies/:id` | JWT (Admin) | Delete policy |
| PATCH | `/admin/security-policies/:id/toggle` | JWT (Admin) | Toggle policy |

---

## 15. Admin - Capability Requests (4 endpoints)

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| GET | `/admin/capability-requests` | JWT (Admin) | List capability requests |
| GET | `/admin/capability-requests/:id` | JWT (Admin) | Get capability request |
| POST | `/admin/capability-requests/:id/approve` | JWT (Admin) | Approve request |
| POST | `/admin/capability-requests/:id/reject` | JWT (Admin) | Reject request |

---

## 16. Compliance (7 endpoints)
**Base**: `/api/v1/compliance`

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| GET | `/status` | JWT (Admin) | Get compliance status |
| GET | `/metrics` | JWT (Admin) | Get compliance metrics |
| GET | `/audit-log/access-review` | JWT (Admin) | Get access review |
| GET | `/audit-log/data-retention` | JWT (Admin) | Get data retention |
| GET | `/access-review` | JWT (Admin) | Get access review (duplicate?) |
| POST | `/check` | JWT (Admin) | Run compliance check |
| GET | `/data-retention` | JWT (Admin) | Get data retention policies |

---

## 17. MCP Servers (15 endpoints)
**Base**: `/api/v1/mcp-servers`

### Core CRUD
| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| GET | `/` | JWT | List MCP servers |
| POST | `/` | JWT (Member+) | Create MCP server |
| GET | `/:id` | JWT | Get MCP server |
| PUT | `/:id` | JWT (Member+) | Update MCP server |
| DELETE | `/:id` | JWT (Manager+) | Delete MCP server |

### MCP Verification
| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| POST | `/:id/verify` | JWT (Manager+) | Verify MCP server |
| POST | `/:id/keys` | JWT (Member+) | Add public key |
| GET | `/:id/verification-status` | JWT | Get verification status |
| POST | `/:id/verify-action` | JWT | Verify MCP action |

### MCP Metadata
| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| GET | `/:id/capabilities` | JWT | Get detected capabilities |
| GET | `/:id/agents` | JWT | Get agents that talk to MCP |
| GET | `/:id/verification-events` | JWT | Get verification events |

### MCP Tags
| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| GET | `/:id/tags` | JWT | Get MCP tags |
| POST | `/:id/tags` | JWT (Member+) | Add tags to MCP |
| DELETE | `/:id/tags/:tagId` | JWT (Member+) | Remove tag from MCP |
| GET | `/:id/tags/suggestions` | JWT | Get tag suggestions |

---

## 18. Security (3 endpoints)
**Base**: `/api/v1/security`

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| GET | `/threats` | JWT (Manager+) | Get threats |
| GET | `/anomalies` | JWT (Manager+) | Get anomalies |
| GET | `/metrics` | JWT (Manager+) | Get security metrics |

---

## 19. Analytics (5 endpoints)
**Base**: `/api/v1/analytics`

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| GET | `/dashboard` | JWT | Get dashboard stats (viewer-accessible) |
| GET | `/usage` | JWT | Get usage statistics |
| GET | `/trends` | JWT | **Get trust score trends** |
| GET | `/verification-activity` | JWT | Get verification activity |
| GET | `/agents/activity` | JWT | Get agent activity |

---

## 20. Webhooks (4 endpoints)
**Base**: `/api/v1/webhooks`

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| POST | `/` | JWT (Member+) | Create webhook |
| GET | `/` | JWT | List webhooks |
| GET | `/:id` | JWT | Get webhook |
| DELETE | `/:id` | JWT (Member+) | Delete webhook |

---

## 21. Verifications (3 endpoints)
**Base**: `/api/v1/verifications`

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| POST | `/` | JWT | Create verification |
| GET | `/:id` | JWT | Get verification |
| POST | `/:id/result` | JWT | Submit verification result |

---

## 22. Verification Events (8 endpoints)
**Base**: `/api/v1/verification-events`

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| GET | `/` | JWT | List verification events |
| GET | `/recent` | JWT | Get recent events |
| GET | `/statistics` | JWT | Get statistics |
| GET | `/stats` | JWT | Get aggregated stats |
| GET | `/agent/:id` | JWT | Get events for agent |
| GET | `/mcp/:id` | JWT | Get events for MCP |
| GET | `/:id` | JWT | Get verification event |
| POST | `/` | JWT (Member+) | Create verification event |
| DELETE | `/:id` | JWT (Manager+) | Delete verification event |

---

## 23. Tags (5 endpoints)
**Base**: `/api/v1/tags`

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| GET | `/` | JWT | Get all tags |
| POST | `/` | JWT (Member+) | Create tag |
| GET | `/popular` | JWT | Get popular tags |
| GET | `/search` | JWT | Search tags |
| DELETE | `/:id` | JWT (Manager+) | Delete tag |

---

## 24. Capabilities (1 endpoint)
**Base**: `/api/v1/capabilities`

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| GET | `/` | JWT | List all capabilities |

---

## Summary by Category

| Category | Endpoint Count |
|----------|----------------|
| Health & Status | 3 |
| SDK API | 4 |
| Public API | 9 |
| Authentication | 5 |
| SDK Download | 1 |
| SDK Tokens | 4 |
| Detection | 3 |
| Agents | 27 |
| API Keys | 4 |
| Trust Score | 3 |
| Admin - Users | 12 |
| Admin - Audit/Monitoring | 5 |
| Admin - Dashboard | 1 |
| Admin - Security Policies | 6 |
| Admin - Capability Requests | 4 |
| Compliance | 7 |
| MCP Servers | 15 |
| Security | 3 |
| Analytics | 5 |
| Webhooks | 4 |
| Verifications | 3 |
| Verification Events | 8 |
| Tags | 5 |
| Capabilities | 1 |
| **TOTAL** | **116** |

---

## Authentication Levels

- **None**: Public endpoints (no auth required)
- **SDK Token**: SDK-only endpoints
- **JWT**: Requires authentication (any role)
- **JWT (Member+)**: Member, Manager, Admin, Super Admin
- **JWT (Manager+)**: Manager, Admin, Super Admin
- **JWT (Admin)**: Admin and Super Admin only
- **Refresh Token**: Special token type for refresh endpoint
