# 📡 AIM API Documentation

Complete REST API reference for AIM (Agent Identity Management).

---

## 📋 Table of Contents

1. [Base URL](#base-url)
2. [Authentication](#authentication)
3. [API Endpoints](#api-endpoints)
   - [Authentication](#authentication-endpoints)
   - [Agents](#agents-endpoints)
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
  "trustScore": 50.0,
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
- `page` (optional): Page number (default: 1)
- `limit` (optional): Items per page (default: 20, max: 100)
- `status` (optional): Filter by status (`active`, `pending_verification`, `revoked`)
- `type` (optional): Filter by type (`ai_agent`, `mcp_server`)

**Response:**
```json
{
  "agents": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "name": "my-agent",
      "displayName": "My Awesome Agent",
      "type": "ai_agent",
      "status": "active",
      "trustScore": 75.5,
      "lastVerifiedAt": "2025-10-08T00:00:00Z",
      "createdAt": "2025-10-07T00:00:00Z"
    }
  ],
  "pagination": {
    "page": 1,
    "limit": 20,
    "total": 45,
    "totalPages": 3
  }
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
  "trustScore": 75.5,
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
  "trustScore": 75.5,
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
  "trustScore": 50.0,
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
      "trustScore": 85.0,
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

**Response:**
```json
{
  "agentId": "550e8400-e29b-41d4-a716-446655440000",
  "trustScore": 75.5,
  "factors": {
    "verificationStatus": 25.0,
    "uptime": 12.5,
    "actionSuccessRate": 14.0,
    "securityAlerts": 10.5,
    "complianceScore": 8.0,
    "ageAndHistory": 3.5,
    "driftDetection": 1.5,
    "userFeedback": 0.5
  },
  "lastCalculated": "2025-10-08T00:00:00Z"
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
      "trustScore": 75.5,
      "timestamp": "2025-10-08T00:00:00Z"
    },
    {
      "trustScore": 74.2,
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
    "trustScore": 75.5,
    "verifiedAt": "2025-10-08T00:00:00Z"
  }
}
```

---

**📖 For more examples, see [Postman Collection](../postman/AIM.postman_collection.json)**
