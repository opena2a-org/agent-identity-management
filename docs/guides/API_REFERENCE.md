# 🔌 API Reference

Complete API reference for Agent Identity Management platform.

**Base URL**: `http://localhost:8080/api/v1` (development)

**Authentication**: Bearer token in `Authorization` header or API key in `X-API-Key` header

## Table of Contents

- [Authentication](#authentication)
- [Agents](#agents)
- [API Keys](#api-keys)
- [Trust Scores](#trust-scores)
- [Admin](#admin)
- [Compliance](#compliance)
- [Error Handling](#error-handling)
- [Rate Limits](#rate-limits)

---

## Authentication

### Initiate OAuth Login

```http
GET /api/v1/auth/login/:provider
```

**Parameters:**
- `provider` (path) - OAuth provider: `google`, `microsoft`, or `okta`

**Response:**
```json
{
  "redirect_url": "https://accounts.google.com/o/oauth2/v2/auth?..."
}
```

**Example:**
```bash
curl http://localhost:8080/api/v1/auth/login/google
```

---

### OAuth Callback

```http
GET /api/v1/auth/callback/:provider
```

**Parameters:**
- `provider` (path) - OAuth provider
- `code` (query) - Authorization code from provider
- `state` (query) - CSRF token

**Response:** Redirects to frontend with tokens

---

### Get Current User

```http
GET /api/v1/auth/me
```

**Headers:**
```
Authorization: Bearer <access_token>
```

**Response:**
```json
{
  "id": "123e4567-e89b-12d3-a456-426614174000",
  "email": "user@example.com",
  "display_name": "John Doe",
  "role": "admin",
  "organization_id": "789e4567-e89b-12d3-a456-426614174000",
  "organization_name": "Acme Corp",
  "created_at": "2025-01-01T00:00:00Z"
}
```

**Example:**
```bash
curl -H "Authorization: Bearer <token>" \
  http://localhost:8080/api/v1/auth/me
```

---

### Logout

```http
POST /api/v1/auth/logout
```

**Headers:**
```
Authorization: Bearer <access_token>
```

**Response:**
```json
{
  "message": "Logged out successfully"
}
```

---

## Agents

### List Agents

```http
GET /api/v1/agents
```

**Headers:**
```
Authorization: Bearer <access_token>
```

**Query Parameters:**
- `status` (optional) - Filter by status: `pending`, `verified`, `suspended`, `revoked`
- `type` (optional) - Filter by type: `ai_agent`, `mcp_server`
- `limit` (optional) - Number of results (default: 50, max: 100)
- `offset` (optional) - Pagination offset (default: 0)

**Response:**
```json
{
  "agents": [
    {
      "id": "456e4567-e89b-12d3-a456-426614174000",
      "organization_id": "789e4567-e89b-12d3-a456-426614174000",
      "name": "code-reviewer",
      "display_name": "Code Review Assistant",
      "description": "AI agent for code review and suggestions",
      "agent_type": "ai_agent",
      "status": "verified",
      "version": "1.0.0",
      "trust_score": 0.85,
      "repository_url": "https://github.com/org/agent",
      "documentation_url": "https://docs.example.com",
      "public_key": "-----BEGIN PUBLIC KEY-----\n...",
      "certificate_url": "https://certs.example.com/agent.pem",
      "created_at": "2025-01-01T00:00:00Z",
      "updated_at": "2025-01-02T00:00:00Z",
      "verified_at": "2025-01-02T00:00:00Z"
    }
  ],
  "total": 10,
  "limit": 50,
  "offset": 0
}
```

**Example:**
```bash
curl -H "Authorization: Bearer <token>" \
  "http://localhost:8080/api/v1/agents?status=verified&limit=10"
```

---

### Create Agent

```http
POST /api/v1/agents
```

**Headers:**
```
Authorization: Bearer <access_token>
Content-Type: application/json
```

**Body:**
```json
{
  "name": "code-reviewer",
  "display_name": "Code Review Assistant",
  "description": "AI agent for code review and suggestions",
  "agent_type": "ai_agent",
  "version": "1.0.0",
  "repository_url": "https://github.com/org/agent",
  "documentation_url": "https://docs.example.com",
  "public_key": "-----BEGIN PUBLIC KEY-----\nMIIBIjANB..."
}
```

**Field Requirements:**
- `name` (required) - Unique identifier (alphanumeric, hyphens, underscores)
- `display_name` (required) - Human-readable name
- `description` (required) - Detailed description
- `agent_type` (required) - `ai_agent` or `mcp_server`
- `version` (optional) - Semantic version (e.g., "1.0.0")
- `repository_url` (optional) - GitHub/GitLab repository
- `documentation_url` (optional) - Documentation URL
- `public_key` (optional) - PEM-formatted RSA public key
- `certificate_url` (optional) - X.509 certificate URL

**Response:**
```json
{
  "id": "456e4567-e89b-12d3-a456-426614174000",
  "organization_id": "789e4567-e89b-12d3-a456-426614174000",
  "name": "code-reviewer",
  "status": "pending",
  "trust_score": 0.0,
  "created_at": "2025-01-01T00:00:00Z"
}
```

**Example:**
```bash
curl -X POST \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "code-reviewer",
    "display_name": "Code Review Assistant",
    "description": "AI agent for code review",
    "agent_type": "ai_agent"
  }' \
  http://localhost:8080/api/v1/agents
```

---

### Get Agent

```http
GET /api/v1/agents/:id
```

**Parameters:**
- `id` (path) - Agent UUID

**Response:** Same as agent object in List Agents

---

### Update Agent

```http
PUT /api/v1/agents/:id
```

**Headers:**
```
Authorization: Bearer <access_token>
Content-Type: application/json
```

**Body:** Same fields as Create Agent (all optional)

**Response:** Updated agent object

---

### Delete Agent

```http
DELETE /api/v1/agents/:id
```

**Response:**
```json
{
  "message": "Agent deleted successfully"
}
```

---

### Verify Agent

```http
POST /api/v1/agents/:id/verify
```

**Response:**
```json
{
  "verified": true,
  "trust_score": 0.75,
  "verified_at": "2025-01-02T00:00:00Z"
}
```

**Note:** Only admins and managers can verify agents.

---

## API Keys

### List API Keys

```http
GET /api/v1/api-keys
```

**Query Parameters:**
- `agent_id` (optional) - Filter by agent
- `is_active` (optional) - Filter by active status (true/false)

**Response:**
```json
{
  "api_keys": [
    {
      "id": "789e4567-e89b-12d3-a456-426614174000",
      "agent_id": "456e4567-e89b-12d3-a456-426614174000",
      "name": "Production API Key",
      "prefix": "aim_live_",
      "last_4": "a1b2",
      "is_active": true,
      "expires_at": "2026-01-01T00:00:00Z",
      "last_used_at": "2025-01-05T12:30:00Z",
      "created_at": "2025-01-01T00:00:00Z"
    }
  ]
}
```

**Note:** Full API key is never returned after creation.

---

### Generate API Key

```http
POST /api/v1/api-keys
```

**Body:**
```json
{
  "agent_id": "456e4567-e89b-12d3-a456-426614174000",
  "name": "Production API Key",
  "expires_at": "2026-01-01T00:00:00Z"
}
```

**Response:**
```json
{
  "id": "789e4567-e89b-12d3-a456-426614174000",
  "api_key": "aim_live_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6",
  "agent_id": "456e4567-e89b-12d3-a456-426614174000",
  "name": "Production API Key",
  "created_at": "2025-01-01T00:00:00Z"
}
```

**⚠️ Important:** Save the `api_key` value immediately. It cannot be retrieved again.

**Example:**
```bash
curl -X POST \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "agent_id": "456e4567-e89b-12d3-a456-426614174000",
    "name": "Production API Key"
  }' \
  http://localhost:8080/api/v1/api-keys
```

---

### Revoke API Key

```http
DELETE /api/v1/api-keys/:id
```

**Response:**
```json
{
  "message": "API key revoked successfully"
}
```

---

## Trust Scores

### Get Trust Score

```http
GET /api/v1/trust-score/agents/:id
```

**Response:**
```json
{
  "agent_id": "456e4567-e89b-12d3-a456-426614174000",
  "trust_score": 0.85,
  "factors": {
    "verification_status": 1.0,
    "certificate_validity": 0.9,
    "repository_quality": 0.8,
    "documentation_score": 0.7,
    "community_trust": 0.8,
    "security_audit": 0.9,
    "update_frequency": 0.85,
    "age_score": 0.6
  },
  "calculated_at": "2025-01-02T00:00:00Z"
}
```

---

### Recalculate Trust Score

```http
POST /api/v1/trust-score/calculate/:id
```

**Response:**
```json
{
  "agent_id": "456e4567-e89b-12d3-a456-426614174000",
  "previous_score": 0.80,
  "new_score": 0.85,
  "calculated_at": "2025-01-05T00:00:00Z"
}
```

---

### Get Trust Score History

```http
GET /api/v1/trust-score/agents/:id/history
```

**Query Parameters:**
- `limit` (optional) - Number of records (default: 100)
- `offset` (optional) - Pagination offset

**Response:**
```json
{
  "history": [
    {
      "trust_score": 0.85,
      "created_at": "2025-01-05T00:00:00Z"
    },
    {
      "trust_score": 0.80,
      "created_at": "2025-01-02T00:00:00Z"
    }
  ]
}
```

---

## Admin

**Note:** All admin endpoints require `admin` or `manager` role.

### List Users

```http
GET /api/v1/admin/users
```

**Query Parameters:**
- `organization_id` (optional) - Filter by organization
- `role` (optional) - Filter by role
- `limit` (optional) - Number of results
- `offset` (optional) - Pagination offset

**Response:**
```json
{
  "users": [
    {
      "id": "123e4567-e89b-12d3-a456-426614174000",
      "email": "user@example.com",
      "display_name": "John Doe",
      "role": "admin",
      "provider": "google",
      "organization_id": "789e4567-e89b-12d3-a456-426614174000",
      "organization_name": "Acme Corp",
      "created_at": "2025-01-01T00:00:00Z",
      "last_login_at": "2025-01-05T12:00:00Z"
    }
  ]
}
```

---

### Update User Role

```http
PUT /api/v1/admin/users/:id/role
```

**Body:**
```json
{
  "role": "manager"
}
```

**Roles:**
- `admin` - Full platform access
- `manager` - Can verify agents, manage users
- `member` - Can create/manage agents
- `viewer` - Read-only access

**Response:**
```json
{
  "id": "123e4567-e89b-12d3-a456-426614174000",
  "role": "manager",
  "updated_at": "2025-01-05T00:00:00Z"
}
```

---

### Get Audit Logs

```http
GET /api/v1/admin/audit-logs
```

**Query Parameters:**
- `user_id` (optional) - Filter by user
- `action` (optional) - Filter by action type
- `resource_type` (optional) - Filter by resource type
- `start_date` (optional) - ISO 8601 datetime
- `end_date` (optional) - ISO 8601 datetime
- `limit` (optional) - Number of results (default: 100)
- `offset` (optional) - Pagination offset

**Response:**
```json
{
  "logs": [
    {
      "id": "999e4567-e89b-12d3-a456-426614174000",
      "user_id": "123e4567-e89b-12d3-a456-426614174000",
      "user_email": "user@example.com",
      "action": "create",
      "resource_type": "agent",
      "resource_id": "456e4567-e89b-12d3-a456-426614174000",
      "ip_address": "192.168.1.1",
      "user_agent": "Mozilla/5.0...",
      "metadata": {
        "agent_name": "code-reviewer",
        "agent_type": "ai_agent"
      },
      "timestamp": "2025-01-05T12:00:00Z"
    }
  ],
  "total": 1247
}
```

---

### Get Alerts

```http
GET /api/v1/admin/alerts
```

**Query Parameters:**
- `severity` (optional) - `info`, `warning`, `critical`
- `is_acknowledged` (optional) - true/false
- `limit` (optional)
- `offset` (optional)

**Response:**
```json
{
  "alerts": [
    {
      "id": "888e4567-e89b-12d3-a456-426614174000",
      "alert_type": "api_key_expiring",
      "severity": "warning",
      "title": "API Key 'Production API Key' Expiring Soon",
      "description": "API key will expire in 5 days",
      "resource_type": "api_key",
      "resource_id": "789e4567-e89b-12d3-a456-426614174000",
      "is_acknowledged": false,
      "created_at": "2025-01-05T00:00:00Z"
    }
  ]
}
```

---

### Acknowledge Alert

```http
POST /api/v1/admin/alerts/:id/acknowledge
```

**Response:**
```json
{
  "id": "888e4567-e89b-12d3-a456-426614174000",
  "is_acknowledged": true,
  "acknowledged_by": "123e4567-e89b-12d3-a456-426614174000",
  "acknowledged_at": "2025-01-05T13:00:00Z"
}
```

---

## Compliance

### Generate Compliance Report

```http
POST /api/v1/compliance/generate
```

**Body:**
```json
{
  "period_days": 30
}
```

**Response:**
```json
{
  "organization_id": "789e4567-e89b-12d3-a456-426614174000",
  "generated_at": "2025-01-05T00:00:00Z",
  "period": "Last 30 days",
  "summary": {
    "total_agents": 25,
    "verified_agents": 20,
    "pending_agents": 3,
    "average_trust_score": 0.78,
    "active_api_keys": 15,
    "total_audit_logs": 5432,
    "unacknowledged_alerts": 2
  },
  "agents": [
    {
      "id": "456e4567-e89b-12d3-a456-426614174000",
      "name": "Code Reviewer",
      "type": "ai_agent",
      "status": "verified",
      "trust_score": 0.85,
      "has_certificate": true,
      "last_verified": "2025-01-02"
    }
  ],
  "audit_activity": {
    "total_actions": 5432,
    "unique_users": 12,
    "top_actions": {
      "create": 234,
      "update": 156,
      "verify": 45
    },
    "recent_actions_24h": 87
  },
  "recommendations": [
    "Verify 3 pending agent(s) to improve security posture",
    "2 verified agent(s) lack certificate URLs. Add certificates to improve trust scores.",
    "Address 2 unacknowledged alert(s) to maintain security compliance"
  ]
}
```

---

### Export Compliance Report

```http
GET /api/v1/compliance/export
```

**Query Parameters:**
- `format` (required) - `json`, `csv`, or `pdf`
- `period_days` (optional) - Default: 30

**Response:** File download

**Example:**
```bash
curl -H "Authorization: Bearer <token>" \
  "http://localhost:8080/api/v1/compliance/export?format=json&period_days=30" \
  -o compliance_report.json
```

---

## Error Handling

All errors follow this format:

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Invalid input data",
    "details": {
      "field": "email",
      "reason": "Invalid email format"
    }
  }
}
```

**Common Error Codes:**

| Code | HTTP Status | Description |
|------|-------------|-------------|
| VALIDATION_ERROR | 400 | Invalid request data |
| UNAUTHORIZED | 401 | Missing or invalid authentication |
| FORBIDDEN | 403 | Insufficient permissions |
| NOT_FOUND | 404 | Resource not found |
| CONFLICT | 409 | Resource already exists |
| RATE_LIMIT_EXCEEDED | 429 | Too many requests |
| INTERNAL_ERROR | 500 | Server error |

---

## Rate Limits

**IP-based:**
- 60 requests per minute
- Applies to unauthenticated requests

**User-based:**
- 300 requests per minute
- Applies to authenticated requests

**API Key-based:**
- 1000 requests per hour
- Applies to API key authentication

**Response Headers:**
```
X-RateLimit-Limit: 60
X-RateLimit-Remaining: 45
X-RateLimit-Reset: 1640995200
```

**Rate Limit Exceeded:**
```json
{
  "error": {
    "code": "RATE_LIMIT_EXCEEDED",
    "message": "Too many requests, please try again later",
    "retry_after": 30
  }
}
```

---

## Pagination

All list endpoints support pagination:

**Query Parameters:**
- `limit` - Number of items per page (max 100)
- `offset` - Number of items to skip

**Response Format:**
```json
{
  "data": [...],
  "total": 250,
  "limit": 50,
  "offset": 0,
  "has_more": true
}
```

---

## Webhooks

Coming soon! Subscribe to events:
- `agent.created`
- `agent.verified`
- `agent.suspended`
- `api_key.created`
- `api_key.expiring`
- `alert.created`

---

## SDKs

Official SDKs coming soon:

- **JavaScript/TypeScript**
- **Python**
- **Go**
- **Ruby**

Community SDKs welcome!

---

## Support

- **API Issues**: https://github.com/opena2a/identity/issues
- **Documentation**: https://docs.opena2a.org
- **Discord**: https://discord.gg/opena2a
