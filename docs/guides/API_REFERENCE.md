# API Reference

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
  "redirectUrl": "https://accounts.google.com/o/oauth2/v2/auth?..."
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
  "name": "John Doe",
  "role": "admin",
  "organizationId": "789e4567-e89b-12d3-a456-426614174000",
  "provider": "google",
  "createdAt": "2025-01-01T00:00:00Z"
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

### Public Login

```http
POST /api/v1/public/login
```

**Body:**
```json
{
  "email": "user@example.com",
  "password": "your-password"
}
```

**Response (approved user):**
```json
{
  "success": true,
  "message": "Login successful",
  "user": {
    "id": "123e4567-e89b-12d3-a456-426614174000",
    "email": "user@example.com",
    "name": "John Doe",
    "role": "admin",
    "organizationId": "789e4567-e89b-12d3-a456-426614174000"
  },
  "accessToken": "eyJhbGciOiJIUzI1NiIs...",
  "refreshToken": "eyJhbGciOiJIUzI1NiIs...",
  "isApproved": true
}
```

**Response (pending approval):**
```json
{
  "success": true,
  "message": "Your registration is pending admin approval",
  "isApproved": false
}
```

**Errors:**
- `400` - Missing or empty email/password
- `401` - Invalid credentials or deactivated account

**Example:**
```bash
curl -X POST \
  -H "Content-Type: application/json" \
  -d '{"email": "user@example.com", "password": "your-password"}' \
  http://localhost:8080/api/v1/public/login
```

**Note:** This endpoint does not require authentication. It is the primary login method for users created via the public registration flow or the admin dashboard.

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
      "organizationId": "789e4567-e89b-12d3-a456-426614174000",
      "name": "code-reviewer",
      "displayName": "Code Review Assistant",
      "description": "AI agent for code review and suggestions",
      "agentType": "ai_agent",
      "status": "verified",
      "version": "1.0.0",
      "trustScore": 0.85,
      "repositoryUrl": "https://github.com/org/agent",
      "documentationUrl": "https://docs.example.com",
      "publicKey": "-----BEGIN PUBLIC KEY-----\n...",
      "certificateUrl": "https://certs.example.com/agent.pem",
      "createdAt": "2025-01-01T00:00:00Z",
      "updatedAt": "2025-01-02T00:00:00Z",
      "verifiedAt": "2025-01-02T00:00:00Z"
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
  "displayName": "Code Review Assistant",
  "description": "AI agent for code review and suggestions",
  "agentType": "ai_agent",
  "version": "1.0.0",
  "repositoryUrl": "https://github.com/org/agent",
  "documentationUrl": "https://docs.example.com",
  "publicKey": "-----BEGIN PUBLIC KEY-----\nMIIBIjANB..."
}
```

**Field Requirements:**
- `name` (required) - Unique identifier (alphanumeric, hyphens, underscores)
- `displayName` (required) - Human-readable name
- `description` (required) - Detailed description
- `agentType` (required) - `ai_agent` or `mcp_server`
- `version` (optional) - Semantic version (e.g., "1.0.0")
- `repositoryUrl` (optional) - GitHub/GitLab repository
- `documentationUrl` (optional) - Documentation URL
- `publicKey` (optional) - PEM-formatted RSA public key
- `certificateUrl` (optional) - X.509 certificate URL

**Response:**
```json
{
  "id": "456e4567-e89b-12d3-a456-426614174000",
  "organizationId": "789e4567-e89b-12d3-a456-426614174000",
  "name": "code-reviewer",
  "status": "pending",
  "trustScore": 0.0,
  "createdAt": "2025-01-01T00:00:00Z"
}
```

**Example:**
```bash
curl -X POST \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "code-reviewer",
    "displayName": "Code Review Assistant",
    "description": "AI agent for code review",
    "agentType": "ai_agent"
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
  "trustScore": 0.75,
  "verifiedAt": "2025-01-02T00:00:00Z"
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
- `agentId` (optional) - Filter by agent
- `isActive` (optional) - Filter by active status (true/false)

**Response:**
```json
{
  "apiKeys": [
    {
      "id": "789e4567-e89b-12d3-a456-426614174000",
      "agentId": "456e4567-e89b-12d3-a456-426614174000",
      "name": "Production API Key",
      "prefix": "aim_live_",
      "isActive": true,
      "expiresAt": "2026-01-01T00:00:00Z",
      "lastUsedAt": "2025-01-05T12:30:00Z",
      "createdAt": "2025-01-01T00:00:00Z"
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
  "agentId": "456e4567-e89b-12d3-a456-426614174000",
  "name": "Production API Key",
  "expiresAt": "2026-01-01T00:00:00Z"
}
```

**Response:**
```json
{
  "id": "789e4567-e89b-12d3-a456-426614174000",
  "apiKey": "aim_live_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6",
  "agentId": "456e4567-e89b-12d3-a456-426614174000",
  "name": "Production API Key",
  "createdAt": "2025-01-01T00:00:00Z"
}
```

**Important:** Save the `apiKey` value immediately. It cannot be retrieved again.

**Example:**
```bash
curl -X POST \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "agentId": "456e4567-e89b-12d3-a456-426614174000",
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
  "agentId": "456e4567-e89b-12d3-a456-426614174000",
  "trustScore": 0.85,
  "factors": {
    "verificationStatus": 1.0,
    "certificateValidity": 0.9,
    "repositoryQuality": 0.8,
    "documentationScore": 0.7,
    "communityTrust": 0.8,
    "securityAudit": 0.9,
    "updateFrequency": 0.85,
    "ageScore": 0.6
  },
  "calculatedAt": "2025-01-02T00:00:00Z"
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
  "agentId": "456e4567-e89b-12d3-a456-426614174000",
  "previousScore": 0.80,
  "newScore": 0.85,
  "calculatedAt": "2025-01-05T00:00:00Z"
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
      "trustScore": 0.85,
      "createdAt": "2025-01-05T00:00:00Z"
    },
    {
      "trustScore": 0.80,
      "createdAt": "2025-01-02T00:00:00Z"
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
- `organizationId` (optional) - Filter by organization
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
      "name": "John Doe",
      "role": "admin",
      "provider": "google",
      "organizationId": "789e4567-e89b-12d3-a456-426614174000",
      "createdAt": "2025-01-01T00:00:00Z",
      "lastLoginAt": "2025-01-05T12:00:00Z"
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
  "updatedAt": "2025-01-05T00:00:00Z"
}
```

---

### Get Audit Logs

```http
GET /api/v1/admin/audit-logs
```

**Query Parameters:**
- `userId` (optional) - Filter by user
- `action` (optional) - Filter by action type
- `resourceType` (optional) - Filter by resource type
- `startDate` (optional) - ISO 8601 datetime
- `endDate` (optional) - ISO 8601 datetime
- `limit` (optional) - Number of results (default: 100)
- `offset` (optional) - Pagination offset

**Response:**
```json
{
  "logs": [
    {
      "id": "999e4567-e89b-12d3-a456-426614174000",
      "userId": "123e4567-e89b-12d3-a456-426614174000",
      "userEmail": "user@example.com",
      "action": "create",
      "resourceType": "agent",
      "resourceId": "456e4567-e89b-12d3-a456-426614174000",
      "ipAddress": "192.168.1.1",
      "userAgent": "Mozilla/5.0...",
      "metadata": {
        "agentName": "code-reviewer",
        "agentType": "ai_agent"
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
- `isAcknowledged` (optional) - true/false
- `limit` (optional)
- `offset` (optional)

**Response:**
```json
{
  "alerts": [
    {
      "id": "888e4567-e89b-12d3-a456-426614174000",
      "alertType": "apiKeyExpiring",
      "severity": "warning",
      "title": "API Key 'Production API Key' Expiring Soon",
      "description": "API key will expire in 5 days",
      "resourceType": "apiKey",
      "resourceId": "789e4567-e89b-12d3-a456-426614174000",
      "isAcknowledged": false,
      "createdAt": "2025-01-05T00:00:00Z"
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
  "isAcknowledged": true,
  "acknowledgedBy": "123e4567-e89b-12d3-a456-426614174000",
  "acknowledgedAt": "2025-01-05T13:00:00Z"
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
  "periodDays": 30
}
```

**Response:**
```json
{
  "organizationId": "789e4567-e89b-12d3-a456-426614174000",
  "generatedAt": "2025-01-05T00:00:00Z",
  "period": "Last 30 days",
  "summary": {
    "totalAgents": 25,
    "verifiedAgents": 20,
    "pendingAgents": 3,
    "averageTrustScore": 0.78,
    "activeApiKeys": 15,
    "totalAuditLogs": 5432,
    "unacknowledgedAlerts": 2
  },
  "agents": [
    {
      "id": "456e4567-e89b-12d3-a456-426614174000",
      "name": "Code Reviewer",
      "agentType": "ai_agent",
      "status": "verified",
      "trustScore": 0.85,
      "hasCertificate": true,
      "lastVerified": "2025-01-02"
    }
  ],
  "auditActivity": {
    "totalActions": 5432,
    "uniqueUsers": 12,
    "topActions": {
      "create": 234,
      "update": 156,
      "verify": 45
    },
    "recentActions24h": 87
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
- `periodDays` (optional) - Default: 30

**Response:** File download

**Example:**
```bash
curl -H "Authorization: Bearer <token>" \
  "http://localhost:8080/api/v1/compliance/export?format=json&periodDays=30" \
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
    "retryAfter": 30
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
  "hasMore": true
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
- **Documentation**: https://opena2a.org/docs
- **Discord**: https://discord.gg/opena2a
