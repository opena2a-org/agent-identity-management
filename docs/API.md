# 📡 AIM API Documentation

Complete REST API reference for AIM (Agent Identity Management).

---

## 📋 Table of Contents

1. [Base URL](#base-url)
2. [Authentication](#authentication)
3. [API Endpoints](#api-endpoints)
   - [Authentication](#authentication-endpoints)
   - [Agents](#agents-endpoints)
   - [Onboarding](#onboarding-endpoints)
   - [MCP Servers](#mcp-servers-endpoints)
   - [API Keys](#api-keys-endpoints)
   - [Trust Scores](#trust-scores-endpoints)
   - [Audit Logs](#audit-logs-endpoints)
   - [Alerts](#alerts-endpoints)
   - [Compliance](#compliance-endpoints)
   - [Webhooks](#webhooks-endpoints)
   - [Admin](#admin-endpoints)
4. [Error Handling](#error-handling)
5. [Rate Limiting](#rate-limiting)
6. [Webhooks](#webhooks)

---

## Base URL

```
Development: http://localhost:8080
Production:  https://api.yourdomain.com
```

All API endpoints are prefixed with `/api/v1` unless otherwise noted.

---

## Authentication

AIM supports multiple authentication methods:

### JWT Bearer Token (Recommended)

```bash
curl -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  http://localhost:8080/api/v1/agents
```

### API Key (For Programmatic Access)

```bash
curl -H "X-API-Key: YOUR_API_KEY" \
  http://localhost:8080/api/v1/agents
```

### OAuth (SSO)

Supported providers:
- Google (`/auth/google`)
- Microsoft (`/auth/microsoft`)
- Okta (`/auth/okta`)

---

## API Endpoints

### Authentication Endpoints

#### POST /auth/register

Register a new user account.

**Request:**
```json
{
  "email": "user@example.com",
  "password": "SecurePass123!",
  "firstName": "John",
  "lastName": "Doe"
}
```

**Response:**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "email": "user@example.com",
  "firstName": "John",
  "lastName": "Doe",
  "createdAt": "2025-10-08T00:00:00Z"
}
```

---

#### POST /auth/login

Login with email and password.

**Request:**
```json
{
  "email": "user@example.com",
  "password": "SecurePass123!"
}
```

**Response:**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expiresAt": "2025-10-09T00:00:00Z",
  "user": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "email": "user@example.com",
    "firstName": "John",
    "lastName": "Doe"
  }
}
```

---

#### POST /auth/refresh

Refresh JWT token.

**Headers:**
```
Authorization: Bearer YOUR_JWT_TOKEN
```

**Response:**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expiresAt": "2025-10-09T00:00:00Z"
}
```

---

#### GET /auth/google

Initiate Google OAuth flow.

**Redirect:**
Redirects to Google OAuth consent screen.

---

#### GET /auth/google/callback

Handle Google OAuth callback.

**Query Parameters:**
- `code`: OAuth authorization code

**Response:**
Redirects to frontend with JWT token.

---

### Agents Endpoints

#### POST /api/v1/agents

Register a new AI agent.

**Headers:**
```
Authorization: Bearer YOUR_JWT_TOKEN
Content-Type: application/json
```

**Request:**
```json
{
  "name": "my-agent",
  "displayName": "My Awesome Agent",
  "description": "Production agent for user management",
  "type": "ai_agent",
  "publicKey": "base64-encoded-ed25519-public-key",
  "version": "1.0.0",
  "repositoryUrl": "https://github.com/myorg/my-agent",
  "documentationUrl": "https://docs.myorg.com",
  "capabilities": ["read_database", "modify_user", "send_email"]
}
```

**Response:**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "organizationId": "660e8400-e29b-41d4-a716-446655440000",
  "name": "my-agent",
  "displayName": "My Awesome Agent",
  "description": "Production agent for user management",
  "type": "ai_agent",
  "publicKey": "base64-encoded-ed25519-public-key",
  "version": "1.0.0",
  "status": "pending_verification",
  "trustScore": 0.50,
  "createdAt": "2025-10-08T00:00:00Z",
  "updatedAt": "2025-10-08T00:00:00Z"
}
```

---

#### GET /api/v1/agents

List all agents for authenticated user's organization.

**Headers:**
```
Authorization: Bearer YOUR_JWT_TOKEN
```

**Query Parameters:**
- `limit` (optional): Page size. When omitted, all agents are returned.
- `offset` (optional): Number of agents to skip (default: 0). Only applied when `limit` is set.

**Response:**

`total` is always the organization-wide agent count, so clients can keep paginating. The `limit` and `offset` fields are echoed back only when `limit` was provided.

```json
{
  "agents": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "name": "my-agent",
      "displayName": "My Awesome Agent",
      "agentType": "ai_agent",
      "status": "verified",
      "trustScore": 0.755,
      "capabilities": ["file:read"],
      "tags": [],
      "createdAt": "2025-10-07T00:00:00Z"
    }
  ],
  "total": 45,
  "limit": 20,
  "offset": 0
}
```

---

#### GET /api/v1/agents/{id}

Get agent details by ID.

**Headers:**
```
Authorization: Bearer YOUR_JWT_TOKEN
```

**Response:**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "organizationId": "660e8400-e29b-41d4-a716-446655440000",
  "name": "my-agent",
  "displayName": "My Awesome Agent",
  "description": "Production agent for user management",
  "type": "ai_agent",
  "publicKey": "base64-encoded-ed25519-public-key",
  "version": "1.0.0",
  "status": "active",
  "trustScore": 0.755,
  "capabilities": ["read_database", "modify_user"],
  "metadata": {
    "totalActions": 1234,
    "successRate": 98.5,
    "uptime": 99.9
  },
  "createdAt": "2025-10-07T00:00:00Z",
  "updatedAt": "2025-10-08T00:00:00Z",
  "lastVerifiedAt": "2025-10-08T00:00:00Z"
}
```

---

#### POST /api/v1/agents/{id}/verify

Verify agent using challenge-response authentication.

**Step 1: Request Challenge**

```bash
POST /api/v1/agents/{id}/verify/challenge
```

**Response:**
```json
{
  "challenge": "base64-encoded-random-challenge",
  "expiresAt": "2025-10-08T00:05:00Z"
}
```

**Step 2: Submit Signed Response**

```bash
POST /api/v1/agents/{id}/verify/response
```

**Request:**
```json
{
  "challenge": "base64-encoded-challenge",
  "signature": "base64-encoded-ed25519-signature"
}
```

**Response:**
```json
{
  "verified": true,
  "verificationId": "770e8400-e29b-41d4-a716-446655440000",
  "trustScore": 0.755,
  "verifiedAt": "2025-10-08T00:00:00Z"
}
```

---

#### PUT /api/v1/agents/{id}

Update agent details.

**Headers:**
```
Authorization: Bearer YOUR_JWT_TOKEN
Content-Type: application/json
```

**Request:**
```json
{
  "displayName": "Updated Agent Name",
  "description": "Updated description",
  "version": "1.1.0"
}
```

**Response:**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "displayName": "Updated Agent Name",
  "description": "Updated description",
  "version": "1.1.0",
  "updatedAt": "2025-10-08T00:00:00Z"
}
```

---

#### DELETE /api/v1/agents/{id}

Delete (revoke) an agent.

**Headers:**
```
Authorization: Bearer YOUR_JWT_TOKEN
```

**Response:**
```json
{
  "message": "Agent revoked successfully",
  "revokedAt": "2025-10-08T00:00:00Z"
}
```

---

### Onboarding Endpoints

A bootstrap token lets a signed-in user register a first agent from a terminal with one command. The token:

- registers exactly one agent, in the organization of the user who minted it, owned by that user (scope `agents:register`);
- expires 15 minutes after it is minted, and is refused once used or revoked;
- is stored only as a SHA-256 hash plus an 8-character display prefix, and is returned in plaintext once, by the mint call;
- is accepted only in the `X-AIM-Bootstrap-Token` header or the JSON body. A token sent in a query string is revoked and the request is refused, because URLs are recorded by proxies and access logs.

Minting a new token revokes the caller's previous unused one. Mint and exchange are rate limited (10 requests per minute; per user for mint, per client address for exchange).

#### POST /api/v1/onboarding/bootstrap-tokens

Mint a bootstrap token. Requires a member, manager or admin session.

**Headers:**
```
Authorization: Bearer YOUR_JWT_TOKEN
```

**Response (201, `Cache-Control: no-store`):**
```json
{
  "id": "7c1e2a4b-9d3f-4e5a-8b6c-0d1e2f3a4b5c",
  "token": "aim_ob_<43 characters>",
  "displayPrefix": "Xk3q9TfA",
  "scope": "agents:register",
  "expiresAt": "2026-10-08T12:15:00Z"
}
```

#### POST /api/v1/onboarding/bootstrap-tokens/revoke

Revoke the caller's unused bootstrap tokens. Requires a member, manager or admin session. Tokens of other users and other organizations are never affected.

**Response:**
```json
{ "revoked": 1 }
```

#### POST /api/v1/onboarding/bootstrap-tokens/exchange

Register one agent with a bootstrap token. The token is the only credential; no session is read. Every body field is optional. `name` defaults to `my-first-agent` and `agentType` to `custom`. Send `publicKey` (base64 Ed25519) to keep the private key on the client; without it the server generates a key pair and returns the private key once.

**Headers:**
```
X-AIM-Bootstrap-Token: aim_ob_<43 characters>
Content-Type: application/json
```

**Request:**
```json
{
  "name": "my-first-agent",
  "agentType": "custom",
  "publicKey": "base64-encoded-ed25519-public-key"
}
```

**Response (201, `Cache-Control: no-store`):**
```json
{
  "agentId": "550e8400-e29b-41d4-a716-446655440000",
  "organizationId": "660e8400-e29b-41d4-a716-446655440000",
  "name": "my-first-agent",
  "displayName": "my-first-agent",
  "status": "pending",
  "publicKey": "base64-encoded-ed25519-public-key",
  "aimUrl": "https://aim-api.example.com",
  "dashboardUrl": "https://aim.example.com"
}
```

`dashboardUrl` is the dashboard address the server is configured with (`FRONTEND_URL`), so a client can link to the agent at `<dashboardUrl>/dashboard/agents/<agentId>` without deriving the dashboard address from the API address. It is omitted when `FRONTEND_URL` is empty.

**Refusals:** `401` with `code` set to `bootstrap_token_invalid`, `bootstrap_token_expired`, `bootstrap_token_used` or `bootstrap_token_revoked`; `400` with `code` `bootstrap_token_in_url` for a token in the query string; `409` when the organization already has an agent with that name (the token stays usable, so retry with another `name`). To recover from any `401`, mint a new token from the onboarding screen.

#### Onboarding telemetry

AIM records onboarding steps per organization so operators can measure how long it takes a new organization to register its first agent. An event row holds the organization, the event name, the SDK tab for `tab_selected`, and a timestamp. It holds no user, email address, client address, user agent or free text.

| Event | Recorded by |
|---|---|
| `onboarding_viewed`, `tab_selected`, `onboarding_completed`, `onboarding_skipped` | the dashboard, through `POST /api/v1/onboarding/events` |
| `token_minted`, `token_exchanged` | the server, when a bootstrap token is minted or exchanged |
| `first_agent_registered` | the server, once per organization, stamped with the earliest agent's `created_at` |

Server-side events are written off the request path of the request that caused them, and a failed write never fails that request. Migration 116 backfills `first_agent_registered` for every organization that already has an agent.

#### POST /api/v1/onboarding/events

Record a dashboard onboarding event for the caller's organization. Any signed-in role may call it. The organization comes from the session; nothing in the body can choose it. `tab` is required for `tab_selected` (one of `python`, `typescript`, `java`, `go`, `cli`, `mcp`, `claude`, `cursor`) and refused on every other event. Server-side events are refused with `400`.

An organization can record the same event (and, for `tab_selected`, the same tab) at most 20 times in 24 hours, whichever of its members reports it. A report past that cap is accepted and not stored: the response is `202` with `"recorded": false`.

**Request:**
```json
{ "event": "tab_selected", "tab": "python" }
```

**Response (202):**
```json
{ "recorded": true }
```

#### GET /api/v1/platform-admin/onboarding-metrics

Time to first agent across every organization, and onboarding events in the last 30 days. Requires an organization admin session whose email is listed in `AIM_PLATFORM_ADMINS`; any other caller gets `403`.

`time_to_first_agent` is the earliest agent's `created_at` minus the organization's `created_at`. It is computed from those two columns on every call, so it covers organizations created before any event was recorded. `allTime` covers every organization; `recent` covers organizations created in the last `windowDays`. Percentiles use the nearest-rank method and are `null` when no organization in the set has an agent. `excludedNegative` counts organizations whose earliest agent predates the organization (seeded or migrated data); they count toward `organizationsWithAgent` but not toward the percentiles. An agent deleted since registration no longer counts; the earliest remaining agent stands in for it.

**Response (example shape):**
```json
{
  "generatedAt": "2026-10-08T12:00:00Z",
  "windowDays": 30,
  "allTime": {
    "organizations": 120,
    "organizationsWithAgent": 48,
    "conversionRate": 0.4,
    "medianSeconds": 312,
    "p75Seconds": 5400,
    "p90Seconds": 172800,
    "fastestSeconds": 41,
    "buckets": [
      { "label": "under 1 minute", "upperSeconds": 60, "count": 3 },
      { "label": "1 to 5 minutes", "upperSeconds": 300, "count": 20 },
      { "label": "5 to 60 minutes", "upperSeconds": 3600, "count": 10 },
      { "label": "1 to 24 hours", "upperSeconds": 86400, "count": 5 },
      { "label": "1 to 7 days", "upperSeconds": 604800, "count": 6 },
      { "label": "over 7 days", "upperSeconds": null, "count": 4 }
    ],
    "excludedNegative": 0
  },
  "recent": { "organizations": 14, "organizationsWithAgent": 6, "...": "same fields as allTime" },
  "events": [
    { "event": "onboarding_viewed", "organizations": 14, "total": 31 },
    { "event": "tab_selected", "organizations": 9, "total": 17 },
    { "event": "token_minted", "organizations": 0, "total": 0 },
    { "event": "token_exchanged", "organizations": 0, "total": 0 },
    { "event": "first_agent_registered", "organizations": 6, "total": 6 },
    { "event": "onboarding_completed", "organizations": 0, "total": 0 },
    { "event": "onboarding_skipped", "organizations": 0, "total": 0 }
  ]
}
```

`events` lists every event type in funnel order, with zero counts for types not seen in the window.

---

### MCP Servers Endpoints

#### POST /api/v1/mcp-servers

Register a Model Context Protocol (MCP) server.

**Headers:**
```
Authorization: Bearer YOUR_JWT_TOKEN
Content-Type: application/json
```

**Request:**
```json
{
  "name": "my-mcp-server",
  "displayName": "My MCP Server",
  "description": "Production MCP server for AI context",
  "endpoint": "https://mcp.example.com",
  "publicKey": "base64-encoded-ed25519-public-key",
  "capabilities": ["context_search", "data_retrieval"],
  "metadata": {
    "version": "1.0.0",
    "protocol": "MCP/1.0"
  }
}
```

**Response:**
```json
{
  "id": "880e8400-e29b-41d4-a716-446655440000",
  "name": "my-mcp-server",
  "displayName": "My MCP Server",
  "endpoint": "https://mcp.example.com",
  "status": "pending_verification",
  "trustScore": 0.50,
  "createdAt": "2025-10-08T00:00:00Z"
}
```

---

#### GET /api/v1/mcp-servers

List all MCP servers.

**Headers:**
```
Authorization: Bearer YOUR_JWT_TOKEN
```

**Response:**
```json
{
  "mcpServers": [
    {
      "id": "880e8400-e29b-41d4-a716-446655440000",
      "name": "my-mcp-server",
      "displayName": "My MCP Server",
      "endpoint": "https://mcp.example.com",
      "status": "active",
      "trustScore": 0.85,
      "lastVerifiedAt": "2025-10-08T00:00:00Z"
    }
  ]
}
```

---

### API Keys Endpoints

#### POST /api/v1/keys

Generate a new API key.

**Headers:**
```
Authorization: Bearer YOUR_JWT_TOKEN
Content-Type: application/json
```

**Request:**
```json
{
  "name": "Production API Key",
  "expiresAt": "2026-10-08T00:00:00Z",
  "permissions": ["agents:read", "agents:write"]
}
```

**Response:**
```json
{
  "id": "990e8400-e29b-41d4-a716-446655440000",
  "name": "Production API Key",
  "key": "aim_sk_1234567890abcdefghijklmnopqrstuvwxyz",
  "keyPrefix": "aim_sk_1234",
  "expiresAt": "2026-10-08T00:00:00Z",
  "createdAt": "2025-10-08T00:00:00Z"
}
```

**⚠️ Note:** The full API key is only shown once during creation!

---

#### GET /api/v1/keys

List all API keys.

**Headers:**
```
Authorization: Bearer YOUR_JWT_TOKEN
```

**Response:**
```json
{
  "apiKeys": [
    {
      "id": "990e8400-e29b-41d4-a716-446655440000",
      "name": "Production API Key",
      "keyPrefix": "aim_sk_1234",
      "status": "active",
      "lastUsedAt": "2025-10-08T00:00:00Z",
      "expiresAt": "2026-10-08T00:00:00Z",
      "createdAt": "2025-10-07T00:00:00Z"
    }
  ]
}
```

---

#### DELETE /api/v1/keys/{id}

Revoke an API key.

**Headers:**
```
Authorization: Bearer YOUR_JWT_TOKEN
```

**Response:**
```json
{
  "message": "API key revoked successfully",
  "revokedAt": "2025-10-08T00:00:00Z"
}
```

---

### Trust Scores Endpoints

#### GET /api/v1/trust-scores/{agentId}

Get current trust score for an agent.

**Headers:**
```
Authorization: Bearer YOUR_JWT_TOKEN
```

**Response** (factor values are 0-1 scores, not weighted contributions; factors
with no data are excluded from the composite per AIP §6.1 — their placeholder
values here are not measurements):
```json
{
  "agentId": "550e8400-e29b-41d4-a716-446655440000",
  "agentName": "my-agent",
  "score": 0.86,
  "factors": {
    "verificationStatus": 1.0,
    "uptime": 0.98,
    "successRate": 0.95,
    "securityAlerts": 1.0,
    "compliance": 0.8,
    "age": 0.75,
    "driftDetection": 1.0,
    "userFeedback": 0.75,
    "executionIsolation": 0.52
  },
  "calculatedAt": "2026-07-02T00:00:00Z"
}
```

---

#### GET /api/v1/trust-scores/{agentId}/history

Get trust score history.

**Headers:**
```
Authorization: Bearer YOUR_JWT_TOKEN
```

**Query Parameters:**
- `startDate` (optional): ISO 8601 date
- `endDate` (optional): ISO 8601 date
- `interval` (optional): `hour`, `day`, `week`, `month`

**Response:**
```json
{
  "agentId": "550e8400-e29b-41d4-a716-446655440000",
  "history": [
    {
      "trustScore": 0.755,
      "timestamp": "2025-10-08T00:00:00Z"
    },
    {
      "trustScore": 0.742,
      "timestamp": "2025-10-07T00:00:00Z"
    }
  ]
}
```

---

### Audit Logs Endpoints

#### GET /api/v1/audit-logs

List audit logs.

**Headers:**
```
Authorization: Bearer YOUR_JWT_TOKEN
```

**Query Parameters:**
- `page` (optional): Page number
- `limit` (optional): Items per page
- `agentId` (optional): Filter by agent ID
- `actionType` (optional): Filter by action type
- `startDate` (optional): Filter from date
- `endDate` (optional): Filter to date

**Response:**
```json
{
  "logs": [
    {
      "id": "aa0e8400-e29b-41d4-a716-446655440000",
      "agentId": "550e8400-e29b-41d4-a716-446655440000",
      "agentName": "my-agent",
      "actionType": "read_database",
      "resource": "users_table",
      "status": "success",
      "metadata": {
        "recordsRead": 100,
        "duration": "150ms"
      },
      "timestamp": "2025-10-08T00:00:00Z"
    }
  ],
  "pagination": {
    "page": 1,
    "limit": 50,
    "total": 1234,
    "totalPages": 25
  }
}
```

---

#### POST /api/v1/audit-logs/export

Export audit logs to CSV/JSON.

**Headers:**
```
Authorization: Bearer YOUR_JWT_TOKEN
Content-Type: application/json
```

**Request:**
```json
{
  "format": "csv",
  "filters": {
    "startDate": "2025-10-01T00:00:00Z",
    "endDate": "2025-10-08T00:00:00Z",
    "agentId": "550e8400-e29b-41d4-a716-446655440000"
  }
}
```

**Response:**
```json
{
  "downloadUrl": "https://aim.example.com/exports/audit-logs-2025-10-08.csv",
  "expiresAt": "2025-10-09T00:00:00Z"
}
```

---

### Alerts Endpoints

#### GET /api/v1/alerts

List security alerts.

**Headers:**
```
Authorization: Bearer YOUR_JWT_TOKEN
```

**Query Parameters:**
- `severity` (optional): `low`, `medium`, `high`, `critical`
- `status` (optional): `active`, `acknowledged`, `resolved`
- `agentId` (optional): Filter by agent

**Response:**
```json
{
  "alerts": [
    {
      "id": "bb0e8400-e29b-41d4-a716-446655440000",
      "severity": "high",
      "type": "certificate_expiry",
      "title": "Certificate expiring soon",
      "description": "Agent certificate expires in 7 days",
      "agentId": "550e8400-e29b-41d4-a716-446655440000",
      "status": "active",
      "createdAt": "2025-10-08T00:00:00Z"
    }
  ]
}
```

---

#### POST /api/v1/alerts/{id}/acknowledge

Acknowledge an alert.

**Headers:**
```
Authorization: Bearer YOUR_JWT_TOKEN
```

**Response:**
```json
{
  "id": "bb0e8400-e29b-41d4-a716-446655440000",
  "status": "acknowledged",
  "acknowledgedBy": "user@example.com",
  "acknowledgedAt": "2025-10-08T00:00:00Z"
}
```

---

### Compliance Endpoints

#### GET /api/v1/compliance/reports

Get compliance reports.

**Headers:**
```
Authorization: Bearer YOUR_JWT_TOKEN
```

**Query Parameters:**
- `type` (optional): `soc2`, `hipaa`, `gdpr`
- `period` (optional): `month`, `quarter`, `year`

**Response:**
```json
{
  "reports": [
    {
      "id": "cc0e8400-e29b-41d4-a716-446655440000",
      "type": "soc2",
      "period": "2025-Q4",
      "status": "passed",
      "score": 95.5,
      "findings": [
        {
          "control": "AC-2.1",
          "status": "compliant",
          "evidence": "All agents verified within 24 hours"
        }
      ],
      "generatedAt": "2025-10-08T00:00:00Z"
    }
  ]
}
```

---

## Error Handling

All errors follow a consistent format:

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Invalid agent name",
    "details": {
      "field": "name",
      "constraint": "must be alphanumeric and 3-50 characters"
    }
  }
}
```

### Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `VALIDATION_ERROR` | 400 | Request validation failed |
| `UNAUTHORIZED` | 401 | Authentication required |
| `FORBIDDEN` | 403 | Insufficient permissions |
| `NOT_FOUND` | 404 | Resource not found |
| `CONFLICT` | 409 | Resource already exists |
| `RATE_LIMIT_EXCEEDED` | 429 | Too many requests |
| `INTERNAL_ERROR` | 500 | Server error |

---

## Rate Limiting

Default rate limits:

- **Authenticated requests**: 100 requests/minute
- **Unauthenticated requests**: 10 requests/minute

Rate limit headers:

```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1696780800
```

When rate limited:

```json
{
  "error": {
    "code": "RATE_LIMIT_EXCEEDED",
    "message": "Too many requests",
    "retryAfter": 60
  }
}
```

---

## Webhooks

Subscribe to events via webhooks:

### POST /api/v1/webhooks

Create a webhook.

**Request:**
```json
{
  "url": "https://yourapp.com/webhooks/aim",
  "events": ["agent.verified", "alert.created"],
  "secret": "your-webhook-secret"
}
```

### Webhook Events

- `agent.registered`
- `agent.verified`
- `agent.revoked`
- `alert.created`
- `alert.acknowledged`
- `trust_score.updated`
- `compliance.report_generated`

### Webhook Payload

```json
{
  "event": "agent.verified",
  "timestamp": "2025-10-08T00:00:00Z",
  "data": {
    "agentId": "550e8400-e29b-41d4-a716-446655440000",
    "trustScore": 0.755,
    "verifiedAt": "2025-10-08T00:00:00Z"
  }
}
```

---

**📖 For more examples, see [Postman Collection](../postman/AIM.postman_collection.json)**
