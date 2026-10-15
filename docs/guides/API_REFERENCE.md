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

Users sign in with email and password through [Public Login](#public-login) below. The server registers no single sign-on (OAuth provider) login routes.

### Log In with Email and Password

```http
POST /api/v1/auth/login/local
```

**Request Body:**
```json
{
  "email": "user@example.com",
  "password": "<password>"
}
```

**Response:**
```json
{
  "accessToken": "eyJhbGc...",
  "refreshToken": "eyJhbGc...",
  "user": {
    "id": "123e4567-e89b-12d3-a456-426614174000",
    "email": "user@example.com",
    "name": "John Doe",
    "role": "admin",
    "organizationId": "789e4567-e89b-12d3-a456-426614174000",
    "forcePasswordChange": false
  }
}
```

A missing email or password is answered `400`; a wrong email or password is answered `401` with `{"error": "Invalid email or password"}`.

**Example:**
```bash
curl -X POST http://localhost:8080/api/v1/auth/login/local \
  -H "Content-Type: application/json" \
  -d '{"email": "user@example.com", "password": "<password>"}'
```

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
  "organizationName": "Example Org",
  "lastLoginAt": "2025-01-02T00:00:00Z",
  "createdAt": "2025-01-01T00:00:00Z",
  "status": "active"
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

**Body (optional):** the refresh token to revoke. The dashboard and the Python SDK (after `aim-sdk login`) both send it here. The API reads no cookie: a `refresh_token` cookie is not revoked, though the answer still clears the session cookies that earlier versions set.
```json
{
  "refreshToken": "<refresh_token>"
}
```

**Response:** `revoked` reports what was written to the server's denylist. A value is `true` only when that token was presented, validated, and its id was denylisted; it is `false` when the token was absent or invalid, or when revocation is not configured on the server (no Redis).
```json
{
  "message": "Logged out successfully",
  "revoked": { "accessToken": true, "refreshToken": true }
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
- `limit` (optional) - Page size. When omitted, every agent is returned.
- `offset` (optional) - Number of agents to skip (default: 0). Applied only when `limit` is set.
- `status` (optional) - Keep the agents whose status is exactly this value (`pending`, `verified`, `suspended` or `revoked`).
- `agentType` (optional) - Keep the agents whose type is exactly this value (for example `ai_agent` or `langchain`). The server reads `agent_type` when `agentType` is absent or empty.

A filter value that no agent holds returns no agents, and an empty filter value does not filter.

`total` is the count of the organization's agents that pass the filters, the same on every page; with no filter it is the organization's agent count. `limit` and `offset` are echoed back only when `limit` was sent.

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
  "http://localhost:8080/api/v1/agents?limit=50"
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
      "provider": "local",
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
      "createdAt": "2025-01-05T00:00:00Z",
      "occurrenceCount": 1,
      "lastSeenAt": null
    }
  ]
}
```

`occurrenceCount` is how many times the alert was raised, and `lastSeenAt` records the latest repeat; it is `null`
for an alert raised once. The server counts a repeat on an existing alert, while that alert is unacknowledged and
less than 10 minutes old, instead of creating a new one. The alerts `POST /api/v1/agents/:id/verify-capability` raises
for a capability the agent was not granted are counted this way: repeated requests from one agent for the same
capability and resource add to one alert's `occurrenceCount`. Every other alert is created once per event, with an
`occurrenceCount` of 1.

---

### Acknowledge Alert

```http
POST /api/v1/admin/alerts/:id/acknowledge
```

**Response:** 204 No Content. An alert outside your organization answers 404.

---

## Compliance

The compliance routes need an administrator's session. `GET /api/v1/compliance/status`, `/metrics`, `/trending` and `/evidence` return the organization's current status, metrics, score history and collected evidence; `POST /api/v1/compliance/check` runs the compliance checks.

### Export Compliance Report

```http
GET /api/v1/compliance/export
```

**Query Parameters:**
- `format` (optional) - `csv` (default) or `json`
- `framework` (optional) - `aim`, the only framework supported
- `start_date`, `end_date` (optional) - RFC 3339 timestamps (default: the last 30 days)

**Response:** the file itself, `compliance-report.csv` or `compliance-report.json`, sent as an attachment.

**Example:**
```bash
curl -H "Authorization: Bearer <token>" \
  "http://localhost:8080/api/v1/compliance/export?format=json" \
  -o compliance_report.json
```

---

## Error Handling

An error answers with an HTTP status and a JSON body whose `error` member is a message:

```json
{
  "error": "Invalid agent ID"
}
```

| HTTP Status | Meaning |
|-------------|---------|
| 400 | The request is malformed or fails validation |
| 401 | No credentials, or credentials the server does not accept |
| 403 | The credentials are valid but the role is not allowed |
| 404 | The resource does not exist in your organization |
| 409 | The resource already exists |
| 429 | Rate limit exceeded |
| 500 | Server error |

---

## Rate Limits

Requests are counted per signed-in user when the server knows the user before the limiter runs, and per client IP address otherwise, over a one-minute window:

- **Most routes:** 100 requests per minute.
- **Credential routes:** 10 requests per minute. These are sign-in, sign-out, token refresh, `GET /api/v1/auth/me`, password change and reset, registration and access requests, the OAuth token and device-code routes, SDK token recovery, and bootstrap-token mint and exchange.

**Response Headers:** `X-RateLimit-Reset` is the number of seconds until the window resets.
```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 42
```

**Rate Limit Exceeded:** 429, with a `Retry-After` header (seconds) and this body:
```json
{
  "error": "Rate limit exceeded. Please try again later."
}
```

---

## Pagination

There is no common list envelope. Each list answers with an array named for what it lists and a `total`, and the routes that page take `limit` and `offset`:

| Route | Response | Paging |
|-------|----------|--------|
| `GET /api/v1/agents` | `{"agents": [...], "total": 45}`, plus `limit` and `offset` when `limit` was sent; with a `status` or `agentType` filter, `total` counts the agents that pass it | `limit` (no default: every agent), `offset` |
| `GET /api/v1/admin/audit-logs` | `{"logs": [...], "total": 1234, "limit": 100, "offset": 0}` | `limit` (default 100), `offset` |
| `GET /api/v1/admin/alerts` | `{"alerts": [...], "total": 3, ...}` with per-state counts | `limit` (default 100), `offset` |
| `GET /api/v1/api-keys` | `{"apiKeys": [...], "total": 2}` | none |

---

## Webhooks

`/api/v1/webhooks` creates, lists, updates, deletes and tests webhook subscriptions.
`POST /api/v1/webhooks/:id/test` sends a test delivery.

The events a subscription can name, and when each is sent, are listed in
[Webhook Events](../API.md#webhook-events): `agent.created`, `agent.verified`, `agent.updated`,
`agent.suspended`, `agent.deleted`, `trust_score.changed`, `trust_score.low`, `alert.created`,
`alert.acknowledged`, `api_key.created` and `api_key.revoked`. Retries and the limits on pending
deliveries are in [Delivery and retries](../API.md#delivery-and-retries).

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

- **API Issues**: https://github.com/opena2a-org/agent-identity-management/issues
- **Documentation**: https://opena2a.org/docs
- **Discord**: https://discord.gg/opena2a
