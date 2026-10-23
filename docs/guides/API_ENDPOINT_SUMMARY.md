# AIM API Endpoint Summary

This page lists the server's main routes by area. It is not the whole route table: that is `apps/backend/cmd/server/main.go` and `apps/backend/cmd/server/sdk_api_routes.go`. A backend test (`apps/backend/cmd/server/api_docs_routes_contract_test.go`) fails when this page names a method and path the server does not register.

---

## 📊 Endpoint Categories

### 1. **Runtime Verification (Core Mission)** - 3 endpoints ⭐️

**Purpose**: Pre-execution authorization checks for AI agents and MCP servers

| Method | Endpoint | Description | Authentication |
|--------|----------|-------------|----------------|
| POST | `/api/v1/agents/:id/verify-capability` | Verify agent capability before execution | JWT Required |
| POST | `/api/v1/agents/:id/log-capability/:audit_id` | Log capability execution result | JWT Required |
| POST | `/api/v1/mcp-servers/:id/verify-capability` | Verify MCP capability before execution | JWT Required |

**Implementation Details**:
- **File**: `apps/backend/internal/interfaces/http/handlers/agent_handler.go:277-374`
- **File**: `apps/backend/internal/interfaces/http/handlers/mcp_handler.go`
- **Routes**: `apps/backend/cmd/server/main.go:420-422, 494`
- **ADR**: `architecture/adr/006-runtime-verification-capability-authorization.md`

**Request Flow**:
```
Agent → POST /agents/:id/verify-capability
       ↓
AIM checks: exists? verified? has capabilities?
       ↓
Response: {allowed: true/false, reason: "...", audit_id: "..."}
       ↓
If allowed → Agent executes capability
       ↓
Agent → POST /agents/:id/log-capability/:audit_id (result)
```

**Example Request**:
```json
POST /api/v1/agents/agent_123/verify-capability
{
  "capability": "file:read",
  "resource": "/data/reports/sales.csv",
  "metadata": {
    "file_size": "5MB",
    "user_id": "user_456"
  }
}
```

**Example Response**:
```json
{
  "allowed": true,
  "reason": "Capability matches registered capabilities",
  "audit_id": "audit_789"
}
```

---

### 2. **Authentication & Authorization** - 6 endpoints

| Method | Endpoint | Description | Authentication |
|--------|----------|-------------|----------------|
| POST | `/api/v1/public/register` | Request a user account (waits for an administrator's approval) | None |
| POST | `/api/v1/public/login` | Sign in with email and password | None |
| POST | `/api/v1/auth/login/local` | Email and password login; returns an access token and a refresh token | None |
| POST | `/api/v1/auth/refresh` | Exchange a refresh token (in the JSON body) for new tokens; the refresh token is rotated | None |
| POST | `/api/v1/auth/logout` | User logout; revokes the bearer and the refresh token in the JSON body, reports what was revoked | None |
| GET | `/api/v1/auth/me` | Get current user info | JWT Required |

**Implementation**: `apps/backend/internal/interfaces/http/handlers/auth_handler.go`

---

### 3. **Agent Management** - 8 endpoints

| Method | Endpoint | Description | Authentication | Authorization |
|--------|----------|-------------|----------------|---------------|
| GET | `/api/v1/agents/` | List all agents for organization | JWT Required | Any |
| POST | `/api/v1/agents/` | Create new agent | JWT Required | Member+ |
| GET | `/api/v1/agents/:id` | Get agent details | JWT Required | Any |
| PUT | `/api/v1/agents/:id` | Update agent | JWT Required | Member+ |
| DELETE | `/api/v1/agents/:id` | Delete agent | JWT Required | Manager+ |
| POST | `/api/v1/agents/:id/verify` | Admin verification of agent | JWT Required | Manager+ |
| POST | `/api/v1/agents/:id/verify-capability` | **Runtime verification** ⭐️ | JWT Required | Any |
| POST | `/api/v1/agents/:id/log-capability/:audit_id` | **Log action result** ⭐️ | JWT Required | Any |

**Implementation**: `apps/backend/internal/interfaces/http/handlers/agent_handler.go`

---

### 4. **API Key Management** - 4 endpoints

| Method | Endpoint | Description | Authentication | Authorization |
|--------|----------|-------------|----------------|---------------|
| GET | `/api/v1/api-keys/` | List API keys | JWT Required | Any |
| POST | `/api/v1/api-keys/` | Create API key | JWT Required | Member+ |
| PATCH | `/api/v1/api-keys/:id/disable` | Disable API key | JWT Required | Member+ |
| DELETE | `/api/v1/api-keys/:id` | Delete a disabled API key | JWT Required | Member+ |

**Implementation**: `apps/backend/internal/interfaces/http/handlers/apikey_handler.go`

---

### 5. **Trust Scoring** - 4 endpoints

| Method | Endpoint | Description | Authentication | Authorization |
|--------|----------|-------------|----------------|---------------|
| POST | `/api/v1/trust-score/calculate/:id` | Trigger trust score calculation | JWT Required | Manager+ |
| GET | `/api/v1/trust-score/agents/:id` | Get agent trust score | JWT Required | Any |
| GET | `/api/v1/trust-score/agents/:id/history` | Get trust score history | JWT Required | Any |
| GET | `/api/v1/trust-score/agents/:id/breakdown` | Get each factor's weight and contribution | JWT Required | Any |

**Implementation**: `apps/backend/internal/interfaces/http/handlers/trustscore_handler.go`

---

### 6. **Admin & User Management** - 7 endpoints

| Method | Endpoint | Description | Authentication | Authorization |
|--------|----------|-------------|----------------|---------------|
| GET | `/api/v1/admin/users` | List all users | JWT Required | Admin |
| PUT | `/api/v1/admin/users/:id/role` | Update user role | JWT Required | Admin |
| DELETE | `/api/v1/admin/users/:id` | Deactivate user | JWT Required | Admin |
| GET | `/api/v1/admin/audit-logs` | Get audit logs | JWT Required | Admin |
| GET | `/api/v1/admin/alerts` | Get system alerts | JWT Required | Admin |
| POST | `/api/v1/admin/alerts/:id/acknowledge` | Acknowledge alert | JWT Required | Admin |
| POST | `/api/v1/admin/alerts/:id/resolve` | Resolve alert | JWT Required | Admin |

**Implementation**: `apps/backend/internal/interfaces/http/handlers/admin_handler.go`

---

### 7. **Compliance & Reporting** - 10 endpoints

| Method | Endpoint | Description | Authentication | Authorization |
|--------|----------|-------------|----------------|---------------|
| GET | `/api/v1/compliance/status` | Get compliance status | JWT Required | Admin |
| GET | `/api/v1/compliance/metrics` | Get compliance metrics | JWT Required | Admin |
| GET | `/api/v1/compliance/access-review` | Get access review | JWT Required | Admin |
| POST | `/api/v1/compliance/check` | Run compliance check | JWT Required | Admin |
| GET | `/api/v1/compliance/export` | Download a compliance report (`format=csv` or `json`) | JWT Required | Admin |
| GET | `/api/v1/compliance/trending` | Get compliance score history | JWT Required | Admin |
| POST | `/api/v1/compliance/snapshot` | Record a point-in-time compliance score | JWT Required | Admin |
| GET | `/api/v1/compliance/evidence` | List collected evidence | JWT Required | Admin |
| POST | `/api/v1/compliance/evidence/collect` | Collect evidence for a check | JWT Required | Admin |
| GET | `/api/v1/compliance/evidence/check/:checkName` | Get evidence for one check | JWT Required | Admin |

The audit log export is `GET /api/v1/admin/audit-logs/export`. The compliance export supports one framework, `aim`.

**Implementation**: `apps/backend/internal/interfaces/http/handlers/compliance_handler.go`

---

### 8. **MCP Server Management** - 9 endpoints

| Method | Endpoint | Description | Authentication | Authorization |
|--------|----------|-------------|----------------|---------------|
| GET | `/api/v1/mcp-servers/` | List MCP servers | JWT Required | Any |
| POST | `/api/v1/mcp-servers/` | Register MCP server | JWT Required | Member+ |
| GET | `/api/v1/mcp-servers/:id` | Get MCP details | JWT Required | Any |
| PUT | `/api/v1/mcp-servers/:id` | Update MCP server | JWT Required | Member+ |
| DELETE | `/api/v1/mcp-servers/:id` | Delete MCP server | JWT Required | Manager+ |
| POST | `/api/v1/mcp-servers/:id/verify` | Cryptographic verification | JWT Required | Manager+ |
| POST | `/api/v1/mcp-servers/:id/keys` | Add public key | JWT Required | Member+ |
| GET | `/api/v1/mcp-servers/:id/verification-status` | Get verification status | JWT Required | Any |
| POST | `/api/v1/mcp-servers/:id/verify-capability` | **Runtime verification** ⭐️ | JWT Required | Any |

**Implementation**: `apps/backend/internal/interfaces/http/handlers/mcp_handler.go`

---

### 9. **Security Dashboard** - 6 endpoints

| Method | Endpoint | Description | Authentication | Authorization |
|--------|----------|-------------|----------------|---------------|
| GET | `/api/v1/security/dashboard` | Get the security dashboard | JWT Required | Manager+ |
| GET | `/api/v1/security/alerts` | List security alerts | JWT Required | Manager+ |
| GET | `/api/v1/security/threats` | Get detected threats | JWT Required | Manager+ |
| GET | `/api/v1/security/anomalies` | Get anomaly detections | JWT Required | Manager+ |
| GET | `/api/v1/security/metrics` | Get security metrics | JWT Required | Manager+ |
| GET | `/api/v1/security/violations` | Get capability violations | JWT Required | Manager+ |

**Implementation**: `apps/backend/internal/interfaces/http/handlers/security_handler.go`

---

### 10. **Analytics & Reporting** - 6 endpoints

| Method | Endpoint | Description | Authentication | Authorization |
|--------|----------|-------------|----------------|---------------|
| GET | `/api/v1/analytics/dashboard` | Get dashboard statistics | JWT Required | Any |
| GET | `/api/v1/analytics/usage` | Get usage statistics | JWT Required | Any |
| GET | `/api/v1/analytics/activity` | Get an activity summary | JWT Required | Any |
| GET | `/api/v1/analytics/trends` | Get trust score trends | JWT Required | Any |
| GET | `/api/v1/analytics/verification-activity` | Get verification activity | JWT Required | Any |
| GET | `/api/v1/analytics/agents/activity` | Get agent activity | JWT Required | Any |

**Implementation**: `apps/backend/internal/interfaces/http/handlers/analytics_handler.go`

---

### 11. **Webhooks** - 5 endpoints

| Method | Endpoint | Description | Authentication | Authorization |
|--------|----------|-------------|----------------|---------------|
| POST | `/api/v1/webhooks/` | Create webhook | JWT Required | Member+ |
| GET | `/api/v1/webhooks/` | List webhooks | JWT Required | Any |
| GET | `/api/v1/webhooks/:id` | Get webhook details | JWT Required | Any |
| DELETE | `/api/v1/webhooks/:id` | Delete webhook | JWT Required | Member+ |
| POST | `/api/v1/webhooks/:id/test` | Test webhook | JWT Required | Member+ |

**Implementation**: `apps/backend/internal/interfaces/http/handlers/webhook_handler.go`

---

### 12. **Health & Monitoring** - 3 endpoints

| Method | Endpoint | Description | Authentication |
|--------|----------|-------------|----------------|
| GET | `/health` | Basic health check | None |
| GET | `/health/ready` | Readiness check. The body carries `ready`, `service`, `commit`, `version`, `revision`, `checkedAt`, `degraded` and `dependencies`, and `syncedFrom` when the build was stamped with a full lowercase commit sha by `-X main.buildSyncedFrom`. `revision` is the name of the Azure Container Apps revision the server runs in, read from `CONTAINER_APP_REVISION` at startup, and `""` where that variable is unset or empty. The `dependencies` object reports `database` (required: 200 when its ping succeeds within 2 s, 503 otherwise) and `redis` and `revocationState` (optional: an `unavailable` one sets `degraded` to `true` without changing the status). SIEM delivery is read on `/metrics`, as `aim_siem_forwarding_failing` | None |
| GET | `/api/v1/admin/dashboard/stats` | Dashboard statistics | JWT Required (Admin) |

**Implementation**: `apps/backend/cmd/server/main.go` registers the three routes; `apps/backend/cmd/server/health_ready.go` builds the `/health/ready` body, whose contract is `apps/backend/contracts/health-ready.schema.json`

---

## 🔐 Authentication & Authorization

### Authentication Methods
1. **Email and Password**:
   - `POST /api/v1/auth/login/local`

2. **JWT Tokens**:
   - Issued when a user signs in with email and password (`POST /api/v1/public/login`)
   - Used for subsequent API calls
   - Include in header: `Authorization: Bearer <token>`

3. **API Keys**: created with `POST /api/v1/api-keys`, sent in `X-API-Key` or `Authorization: Bearer <key>`

The server registers no single sign-on (OAuth provider) login routes.

### Authorization Roles
- **Viewer**: Read-only access
- **Member**: Can create/modify their own resources
- **Manager**: Can manage agents, verify, delete
- **Admin**: Full access including user management, compliance

### Middleware Stack
- `AuthMiddleware`: Validates JWT token
- `RateLimitMiddleware`: Standard rate limiting
- `StrictRateLimitMiddleware`: Stricter limits for sensitive operations
- `MemberMiddleware`: Requires Member+ role
- `ManagerMiddleware`: Requires Manager+ role
- `AdminMiddleware`: Requires Admin role

---

## 🎯 Core Mission Architecture

### Runtime Verification Flow (ADR-006)

**The Problem AIM Solves**:
- Organizations need to trust AI agents and MCP servers
- Agents can drift from their authorized capabilities
- No visibility into what AI tools are doing
- Security teams blind to AI-related threats

**AIM's Solution**:
```
1. REGISTRATION
   Employee → Registers Agent → Defines Capabilities → AIM stores

2. RUNTIME VERIFICATION (Every Action)
   Agent → Requests Action → AIM verifies → Allow/Deny → Agent executes → Logs result

3. AUDIT TRAIL
   AIM logs all verifications → Security dashboard → Compliance reports
```

### Capability Schema

**Agent Capabilities**:
```go
type AgentCapabilities struct {
    // File operations
    CanReadFiles     bool
    CanWriteFiles    bool
    AllowedPaths     []string   // ["/data/reports/*"]
    ForbiddenPaths   []string   // ["/etc/*", "/root/*"]
    MaxFileSize      int64      // Bytes

    // Code execution
    CanExecuteCode   bool
    AllowedLanguages []string   // ["python", "javascript"]

    // Network access
    CanAccessNetwork bool
    AllowedDomains   []string   // ["api.company.com"]
    ForbiddenDomains []string   // ["*.external.com"]

    // Database access
    CanQueryDatabase    bool
    AllowedDatabases    []string   // ["analytics_db"]
    AllowedQueryTypes   []string   // ["SELECT"]
    ForbiddenTables     []string   // ["employees", "payroll"]
    MaxResultRows       int        // 1000

    // Rate limits
    MaxActionsPerMinute int        // 100
    MaxActionsPerHour   int        // 1000

    // Business hours
    AllowedHours        []string   // ["9:00-17:00"]
    AllowedDaysOfWeek   []string   // ["Mon", "Tue", "Wed", "Thu", "Fri"]
}
```

### Security Features
1. **Pre-execution Authorization**: Every action verified before execution
2. **Anomaly Detection**: Identify unusual behavior patterns
3. **Capability Drift Detection**: Alert when agents exceed scope
4. **Complete Audit Trail**: All actions logged for compliance
5. **Rate Limiting**: Prevent abuse and DoS attacks
6. **Multi-Tenancy**: Organization-level isolation with RLS

---

## 🚀 Performance Targets

| Metric | Target | Current Status |
|--------|--------|----------------|
| Verification Latency (p99) | <50ms | TBD (to be measured) |
| False Positives | <1% | TBD |
| Anomaly Detection Accuracy | >95% | TBD |
| Audit Trail Coverage | 100% | ✅ Implemented |
| API Availability | 99.9% | TBD |

---

## 📝 API Documentation

The server serves no OpenAPI or Swagger document. The API references are [docs/API.md](../API.md) and [API_REFERENCE.md](API_REFERENCE.md).

---

**Project**: Agent Identity Management (AIM)
**Repository**: github.com/opena2a-org/agent-identity-management
