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

Every path below is written in full. The server registers its API under `/api/v1`; `/health`, `/health/ready` and the `/.well-known/` discovery documents sit at the root.

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

The server registers no single sign-on (OAuth provider) login routes. Users sign in with email and password, below.

---

## API Endpoints

### Authentication Endpoints

#### POST /api/v1/public/register

Request a user account. The request waits for an administrator's approval; it does not sign the user in. `password` needs at least 8 characters, and `signupProfile` (`role`, `primaryUseCase`, `referralSource`) is optional.

**Request:**
```json
{
  "email": "user@example.com",
  "password": "SecurePass123!",
  "firstName": "John",
  "lastName": "Doe"
}
```

**Response** (201): `success`, `message`, `requestId` and the stored `registrationRequest`. `GET /api/v1/public/register/{requestId}/status` reports where the request stands.

---

#### POST /api/v1/public/login

Sign in with email and password.

**Request:**
```json
{
  "email": "user@example.com",
  "password": "SecurePass123!"
}
```

**Response** (200):
```json
{
  "success": true,
  "message": "Login successful",
  "user": { "id": "550e8400-e29b-41d4-a716-446655440000", "email": "user@example.com" },
  "accessToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refreshToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "isApproved": true
}
```

An account an administrator has not approved yet answers with `isApproved: false` and no tokens. An account that must change its password answers with `requiresPasswordChange: true`.

`POST /api/v1/auth/login/local` takes the same request and answers with `accessToken`, `refreshToken` and `user` (`id`, `email`, `name`, `role`, `organizationId`, `forcePasswordChange`).

---

#### POST /api/v1/auth/refresh

Exchange a refresh token for a new access token. The refresh token travels in the body, not in an `Authorization` header.

**Request:**
```json
{
  "refreshToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

**Response:**
```json
{
  "accessToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refreshToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "tokenType": "Bearer",
  "expiresIn": 7200,
  "rotated": true
}
```

`expiresIn` is the access token's lifetime in seconds (`JWT_ACCESS_TTL`, 2 hours by default). `rotated` is `true` when the presented refresh token was retired and a new one issued; `false` means `refreshToken` is the token presented, unchanged.

A downloaded SDK's refresh token is refreshed the same way, into a new 90-day token each time, for at most `SDK_TOKEN_FAMILY_MAX_AGE` after the download (`8760h` by default). Past that the refresh is answered `401` `Invalid or expired refresh token` and the SDK is downloaded again with a sign-in. The access token such a refresh yields is a valid bearer, but it records no sign-in (it carries no `auth_time`), so `GET /api/v1/sdk/download`, `POST /api/v1/auth/sdk/recover`, `POST /api/v1/oauth/device/approve` and `POST /api/v1/api-keys` refuse it with `401`.

`POST /api/v1/auth/sdk/recover` refuses a downloaded SDK's token whose family has ended, with `401` `Token has been revoked or is invalid`: the family ends when any of its tokens is logged out, when one of its tokens is presented again after a rotation, or when one is refused past `SDK_TOKEN_FAMILY_MAX_AGE`, with or without a revocation store. A token retired by a rotation while its family is live is recovered.

`POST /api/v1/auth/logout` with a downloaded SDK's refresh token in the body (`{"refreshToken": "..."}`) ends that token's family, whichever of the family's tokens it is: the presented token's `sdk_tokens` row is retired and every still-active row of the family is revoked with the reason `logout`, with or without a revocation store. The family's live token is then refused at `POST /api/v1/auth/refresh` with `401`, and no token of the family is recovered. The answer's `revoked.refreshToken` is `true` when those rows are revoked and, where a revocation store is configured, its writes succeeded too. Another token family of the same account stays live.

---

#### POST /api/v1/auth/reauth

Re-authenticate the signed-in user. The request carries the session's access token in the `Authorization` header, and the user's password with the session's current refresh token in the body. The password is checked as at sign-in, so a wrong password is recorded as a failed attempt and counts toward the account lockout. The refresh token is retired as at `POST /api/v1/auth/refresh` and replaced by a pair in the same session whose `auth_time` claim is the time of the re-authentication. The session's maximum age (`JWT_SESSION_MAX_AGE`) then counts from it. A session already past its maximum age is not re-authenticated: its access token is refused with `401` as on every route, and the user signs in again.

**Request:**
```json
{
  "password": "<the user's password>",
  "refreshToken": "<the session's refresh token>"
}
```

**Response** (200): the shape of `POST /api/v1/auth/refresh`. Without a revocation store `rotated` is `false`, `refreshToken` is the token presented, and `accessToken` carries the new `auth_time`.

A wrong password, a locked account and an account without a password answer `401` with `{"error": "Invalid password"}`. A refresh token that is not a login refresh token of the bearer's session answers `401` with `{"error": "Invalid or expired refresh token"}`. A refresh token already retired by a rotation ends the session, as at refresh, and it and a member of a revoked session answer `401` with `{"error": "Token has been revoked or is invalid"}`. A suspended, deactivated or deleted account answers `401` with `{"error": "Account is not active"}`. A successful re-authentication is recorded in the audit log as a `login` with `method` `reauthentication`.

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
- `status` (optional): Keep the agents whose status is exactly this value (`pending`, `verified`, `suspended` or `revoked`).
- `agentType` (optional): Keep the agents whose type is exactly this value (for example `ai_agent` or `langchain`). The server reads `agent_type` when `agentType` is absent or empty.

A filter value that no agent holds returns no agents. An empty filter value does not filter.
A filter value that is not valid UTF-8 or holds a NUL character is answered `400 Bad Request` with an error that names
the parameter, for example `{"error": "status must be UTF-8 text without a NUL character"}`.

**Response:**

`total` is the count of the organization's agents that pass the filters, the same on every page, so clients can keep paginating; with no filter it is the organization-wide agent count. The `limit` and `offset` fields are echoed back only when `limit` was provided.

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

Mark an agent in your organization as verified. A manager or administrator signs this request; it takes no body.

**Response:**
```json
{
  "verified": true,
  "changed": true,
  "trustScore": 0.755,
  "verifiedAt": "2025-10-08T00:00:00Z"
}
```

`changed` reports whether the request changed the agent's status.

A verification is made only together with its audit record, and so is a reactivation (`POST /api/v1/agents/{id}/reactivate`). When that record cannot be written, the agent's status is unchanged and the route answers `503` with `Retry-After: 300` (seconds) and `{"error": "...", "reasonCode": "recordUnavailable", "code": "recordUnavailable"}`. Retry after that time.

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

Delete an agent. A manager or administrator signs this request. `POST /api/v1/agents/{id}/revoke` revokes an agent without deleting it.

**Headers:**
```
Authorization: Bearer YOUR_JWT_TOKEN
```

**Response:** 204 No Content.

A delete is made only together with its audit record. When that record cannot be written, nothing is deleted and the route answers `503` with `Retry-After: 300` (seconds) and `{"error": "...", "reasonCode": "recordUnavailable", "code": "recordUnavailable"}`. Retry after that time.

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

#### POST /api/v1/api-keys

Generate a new API key for one agent. `agentId` and `name` are required. The key expires 90 days after it is created.

Creating an API key requires a live sign-in session: the bearer is a sign-in's access token, or one its refreshes mint. An access token minted by refreshing a downloaded SDK's token, or one whose session was revoked, is answered `401` `Token has been revoked or is invalid` and no key is created.

**Headers:**
```
Authorization: Bearer YOUR_JWT_TOKEN
Content-Type: application/json
```

**Request:**
```json
{
  "agentId": "550e8400-e29b-41d4-a716-446655440000",
  "name": "Production API Key"
}
```

**Response** (201):
```json
{
  "id": "990e8400-e29b-41d4-a716-446655440000",
  "apiKey": "aim_live_...",
  "name": "Production API Key",
  "agentId": "550e8400-e29b-41d4-a716-446655440000",
  "expiresAt": "2026-01-06T00:00:00Z",
  "createdAt": "2025-10-08T00:00:00Z"
}
```

**Note:** `apiKey` is returned only in this response. Store it before you close it.

---

#### GET /api/v1/api-keys

List your organization's API keys. `agent_id` (optional) keeps only one agent's keys.

**Headers:**
```
Authorization: Bearer YOUR_JWT_TOKEN
```

**Response:** `{"apiKeys": [...], "total": <count>}`. Each listed key carries its `prefix`, `keyHash`, `isActive`, `lastUsedAt` and `expiresAt`, never the key itself.

---

#### PATCH /api/v1/api-keys/{id}/disable

Disable an API key. It stops authenticating at once.

**Response:** `{"message": "API key disabled successfully"}`

---

#### DELETE /api/v1/api-keys/{id}

Delete an API key. Disable it first: deleting a key that is still enabled answers 400.

**Headers:**
```
Authorization: Bearer YOUR_JWT_TOKEN
```

**Response:** 204 No Content.

---

### Trust Scores Endpoints

#### GET /api/v1/trust-score/agents/{id}

Get current trust score for an agent. `GET /api/v1/trust-score/agents/{id}/breakdown` returns each factor's weight and contribution.

**Headers:**
```
Authorization: Bearer YOUR_JWT_TOKEN
```

**Response** (factor values are 0-1 scores, not weighted contributions; factors
with no data are excluded from the composite per AIP §6.1, and their placeholder
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

#### GET /api/v1/trust-score/agents/{id}/history

Get trust score history, newest first.

**Headers:**
```
Authorization: Bearer YOUR_JWT_TOKEN
```

**Query Parameters:**
- `limit` (optional): number of entries (default: 30)

**Response:**
```json
{
  "agentId": "550e8400-e29b-41d4-a716-446655440000",
  "agentName": "my-agent",
  "history": [
    {
      "trustScore": 0.755,
      "previousScore": 0.742,
      "reason": "why the score changed",
      "timestamp": "2025-10-08T00:00:00Z"
    }
  ],
  "total": 1
}
```

---

### Audit Logs Endpoints

The organization-wide audit log routes need an administrator's session. `GET /api/v1/agents/{id}/audit-logs` and `GET /api/v1/mcp-servers/{id}/audit-logs` list one agent's or one MCP server's entries.

#### GET /api/v1/admin/audit-logs

List audit logs.

**Headers:**
```
Authorization: Bearer YOUR_JWT_TOKEN
```

**Query Parameters:**
- `limit` (optional): Items per page (default: 100)
- `offset` (optional): Number of entries to skip
- `action` (optional): Filter by action
- `entity_type` (optional): Filter by resource type (`agent`, `api_key`, `user`, ...)
- `entity_id` (optional): Filter by resource ID
- `user_id` (optional): Filter by the user who acted
- `start_date`, `end_date` (optional): RFC 3339 timestamps

**Response:**
```json
{
  "logs": [
    {
      "id": "aa0e8400-e29b-41d4-a716-446655440000",
      "organizationId": "660e8400-e29b-41d4-a716-446655440000",
      "userId": "770e8400-e29b-41d4-a716-446655440000",
      "action": "verify",
      "resourceType": "agent",
      "resourceId": "550e8400-e29b-41d4-a716-446655440000",
      "metadata": {
        "agentName": "my-agent",
        "trustScore": 0.755
      },
      "timestamp": "2025-10-08T00:00:00Z"
    }
  ],
  "total": 1234,
  "limit": 100,
  "offset": 0
}
```

---

#### GET /api/v1/admin/audit-logs/export

Download audit logs as a file. Up to 10,000 entries.

**Headers:**
```
Authorization: Bearer YOUR_JWT_TOKEN
```

**Query Parameters:**
- `format` (optional): `csv` (default) or `json`
- `start_date`, `end_date` (optional): RFC 3339 timestamps

**Response:** the file itself, `audit-logs.csv` (`text/csv`) or `audit-logs.json` (`application/json`), sent as an attachment.

---

#### GET /api/v1/admin/audit-logs/chain/head

Read the state of the caller's organization's audit record chain. Admins only. The route reads the caller's organization and no other, through the same chain-state read the record writer runs before it extends a chain. `aim-breakglass chain status --organization <id> --json` prints the same body.

**Headers:**
```
Authorization: Bearer YOUR_JWT_TOKEN
```

**Response members:**
- `chainState`: `notStarted`, `extendable` or `notExtendable`
- `chainId`: the chain's id, `null` when no chain has started
- `head`: the stored head, `seq` and `hash` (lowercase hex SHA-256 of the newest record), `null` when no chain has started. When the chain is not extendable, it is the stored head that cannot be linked to.
- `latestCheckpoint`: `null` until chain checkpoints are written
- `reason`: present only when `chainState` is `notExtendable`: `no_records`, `head_mismatch` or `record_modified`

**Response (chain not started):**
```json
{
  "chainState": "notStarted",
  "chainId": null,
  "head": null,
  "latestCheckpoint": null
}
```

**Response (chain cannot be extended):**
```json
{
  "chainState": "notExtendable",
  "chainId": "6f1c2a7e-3b9d-4e2f-8a10-5c4d3e2f1a0b",
  "head": {
    "seq": 41,
    "hash": "9f2c4e6a8b0d1f3e5a7c9b1d3f5e7a9c2b4d6f8e0a1c3e5b7d9f1a3c5e7b9d0f"
  },
  "latestCheckpoint": null,
  "reason": "head_mismatch"
}
```

A failed read answers 500 with `{"error": "Failed to read the audit record chain"}`.

---

### Alerts Endpoints

The organization-wide alert routes need an administrator's session. `GET /api/v1/agents/{id}/alerts` lists one agent's alerts.

#### GET /api/v1/admin/alerts

List security alerts.

**Headers:**
```
Authorization: Bearer YOUR_JWT_TOKEN
```

**Query Parameters:**
- `status` (optional): `acknowledged` or `unacknowledged`
- `limit` (optional): Items per page (default: 100)
- `offset` (optional): Number of alerts to skip

**Response:**
```json
{
  "alerts": [
    {
      "id": "bb0e8400-e29b-41d4-a716-446655440000",
      "organizationId": "660e8400-e29b-41d4-a716-446655440000",
      "alertType": "trust_score_drop",
      "severity": "high",
      "title": "Trust score dropped",
      "description": "Agent trust score fell below the threshold",
      "resourceType": "agent",
      "resourceId": "550e8400-e29b-41d4-a716-446655440000",
      "isAcknowledged": false,
      "acknowledgedBy": null,
      "acknowledgedAt": null,
      "createdAt": "2025-10-08T00:00:00Z"
    }
  ],
  "total": 1,
  "allCount": 1,
  "acknowledgedCount": 0,
  "unacknowledgedCount": 1,
  "criticalCount": 0
}
```

`severity` is one of `info`, `warning`, `high` or `critical`.

---

#### POST /api/v1/admin/alerts/{id}/acknowledge

Acknowledge an alert. `POST /api/v1/admin/alerts/{id}/resolve` resolves one, and `POST /api/v1/admin/alerts/bulk-acknowledge` acknowledges several.

**Headers:**
```
Authorization: Bearer YOUR_JWT_TOKEN
```

**Response:** 204 No Content. An alert outside your organization answers 404.

---

### Compliance Endpoints

The compliance routes need an administrator's session. `GET /api/v1/compliance/status` returns the organization's current compliance status, and `GET /api/v1/compliance/metrics`, `/trending` and `/evidence` return its metrics, score history and collected evidence.

#### GET /api/v1/compliance/export

Download a compliance report as a file.

**Headers:**
```
Authorization: Bearer YOUR_JWT_TOKEN
```

**Query Parameters:**
- `format` (optional): `csv` (default) or `json`
- `framework` (optional): `aim`, the only framework supported
- `start_date`, `end_date` (optional): RFC 3339 timestamps (default: the last 30 days)

**Response:** the file itself, `compliance-report.csv` or `compliance-report.json`, sent as an attachment.

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

## Rate Limiting

Requests are counted per signed-in user when the server knows the user before the limiter runs, and per client IP address otherwise, over a one-minute window:

- **Most routes**: 100 requests per minute.
- **Credential routes**: 10 requests per minute. These are sign-in, sign-out, token refresh, re-authentication, `GET /api/v1/auth/me`, password change and reset, registration and access requests, the OAuth token and device-code routes, SDK token recovery, and bootstrap-token mint and exchange.

Responses carry the limiter's headers. `X-RateLimit-Reset` is the number of seconds until the window resets:

```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 42
```

A refused request answers 429 with a `Retry-After` header (seconds) and this body:

```json
{
  "error": "Rate limit exceeded. Please try again later."
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
  "name": "Production receiver",
  "url": "https://yourapp.com/webhooks/aim",
  "events": ["agent.verified", "alert.created"],
  "secret": "<a secret of 32 to 255 characters>"
}
```

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | A name for the webhook. |
| `url` | Yes | The endpoint deliveries are sent to: a public `http` or `https` URL. |
| `events` | Yes | Event names from [Webhook Events](#webhook-events). |
| `isActive` | No | `false` creates the webhook inactive. Defaults to `true`. |
| `secret` | No | The secret deliveries are signed with, 32 to 255 characters, stored as given. When it is left out, the server generates a 64-character hex secret. |

**Response (201 Created):** the webhook, and its `secret` this once:
```json
{
  "id": "0c6f3c1e-8f0e-4a63-9a43-2f1d8b7e5a10",
  "organizationId": "7d9f2b4a-1c3e-4f5a-8b6c-9d0e1f2a3b4c",
  "name": "Production receiver",
  "url": "https://yourapp.com/webhooks/aim",
  "events": ["agent.verified", "alert.created"],
  "status": "",
  "timeoutSeconds": 0,
  "maxRetries": 0,
  "retryDelaySeconds": 0,
  "successCount": 0,
  "failureCount": 0,
  "isActive": true,
  "createdAt": "2026-10-16T00:00:00Z",
  "updatedAt": "2026-10-16T00:00:00Z",
  "createdBy": "550e8400-e29b-41d4-a716-446655440000",
  "secret": "<the secret from the request>"
}
```

The create answer is the only one that carries the secret: `GET /api/v1/webhooks`,
`GET /api/v1/webhooks/:id` and `PUT /api/v1/webhooks/:id` never return it. Keep it when the webhook is
created; a secret that is lost is replaced with `POST /api/v1/webhooks/:id/rotate-secret`.

A create or update the server refuses is answered `400`:

| Answer | When |
|--------|------|
| `{"error":"unknown field: headers"}` | The body has a key the route does not take (`unknown fields: a, b` for several). Keys are matched exactly, case included. |
| `{"error":"unsupported event: auth.failed"}` | An event name that is not one of those in [Webhook Events](#webhook-events). |
| `{"error":"secret must be at least 32 characters"}` | `secret` is shorter than 32 characters; one longer than 255 is answered `secret must be at most 255 characters`. |
| `{"error":"url must be a public http or https URL"}` | The URL does not parse, is not `http` or `https`, has no host, names a host that does not exist, or resolves to an address the server does not send to, such as a loopback or private one. |

### PUT /api/v1/webhooks/:id

Update a webhook. The body takes `name`, `url`, `events` and `isActive`, refused as on create. It does
not take `secret`: a body carrying one is answered `400 {"error":"unknown field: secret"}`.

### POST /api/v1/webhooks/:id/rotate-secret

Replace a webhook's secret with a new generated 64-character hex secret. The request has no body.

**Response (200 OK):** the new secret, this once:
```json
{
  "id": "0c6f3c1e-8f0e-4a63-9a43-2f1d8b7e5a10",
  "secret": "<64 hex characters>"
}
```

There is no overlap period: every delivery queued after the rotation is signed with the new secret only,
and the old secret signs nothing queued after it. A delivery queued before the rotation, retries included,
keeps the signature it was queued with. The rotation is written to the audit log as an `update` of the
`webhook`, with `rotatedSecret: true` in its metadata and without the secret. A webhook that does not
exist is answered `404`.

### Webhook Events

An active webhook subscribed to an event receives it when:

| Event | Sent when |
|-------|-----------|
| `agent.created` | An agent is registered. |
| `agent.verified` | An agent is verified through `POST /api/v1/agents/:id/verify`, or at registration. |
| `agent.updated` | An agent is updated through `PUT /api/v1/agents/:id`. |
| `agent.suspended` | An agent is suspended through `POST /api/v1/agents/:id/suspend`, or by the trust score policy (see `trust_score.low`). |
| `agent.deleted` | An agent is deleted through `DELETE /api/v1/agents/:id`. |
| `trust_score.changed` | An administrator sets an agent's trust score through `PUT /api/v1/agents/:id/trust-score`, or a capability violation lowers it, and the stored score differs from the one before. Other trust score changes, including `POST /api/v1/agents/:id/trust-score/recalculate`, send no event. |
| `trust_score.low` | After one of the changes `trust_score.changed` names, the trust score policy finds that a verified or pending agent's score fell and ends below 70%. `severity` is `critical` below 50%, where the policy also suspends the agent, and `warning` otherwise. |
| `alert.created` | An alert is raised for a capability check denied because the agent has no granted capabilities. Other alerts send no event. |
| `alert.acknowledged` | An alert is acknowledged through `POST /api/v1/admin/alerts/:id/acknowledge`. Acknowledging every alert at once, and resolving an alert, send no event. |
| `api_key.created` | An API key is created, including the key created with an agent. |
| `api_key.revoked` | An active API key is disabled through `PATCH /api/v1/api-keys/:id/disable`. |

A subscription accepts these names, and no event is sent for them: `api_key.expired`, `security.violation`,
`compliance.violation`, `security.capability_drift`, `security.data_exfiltration`. A create or update
naming any other event is refused with `400 {"error":"unsupported event: <name>"}`.

The data of an `agent.*` event is the agent after the change: `agentId`, `name`, `displayName`,
`agentType`, `status` and `trustScore`. A `trust_score.*` event carries `agentId`, `agentName`,
`previousScore` and `trustScore`, and `trust_score.low` adds `severity`. An `api_key.*` event carries
`apiKeyId`, `agentId`, `name`, `prefix` and, for a key that expires, `expiresAt`; it never carries the
key or its hash.

`POST /api/v1/webhooks/:id/test` sends a `webhook.test` delivery to the endpoint at once.

### Delivery and retries

Each event is sent to each active webhook subscribed to it as a `POST`, after the change that raised it
is stored. An attempt waits up to 30 seconds for the endpoint. An attempt that gets no answer, or an
answer outside 2xx, is retried up to 3 times, after waits of 60, 120 and 240 seconds. A webhook row
that stores its own retry count, first retry delay or wait uses it, capped at 5 retries, a 30-minute
delay and a 30-second wait.

The server holds at most this much event delivery at once, however many events are raised and however
slow or failing an endpoint is:

| Bound | Limit |
|-------|-------|
| Deliveries pending (sending or waiting to retry) | 1,000 |
| Deliveries pending for one webhook | 100 |
| Event delivery attempts in flight | 5 |
| Event delivery attempts in flight to one webhook | 2 |

A delivery beyond a pending limit is not sent: it is recorded as an `abandoned` delivery whose error
names the limit. When the server shuts down, a delivery waiting to retry is not retried and is recorded
as `abandoned`.

### Webhook Payload

```json
{
  "id": "0c6f3c1e-8f0e-4a63-9a43-2f1d8b7e5a10",
  "event": "agent.verified",
  "timestamp": "2026-10-11T00:00:00Z",
  "organizationId": "7d9f2b4a-1c3e-4f5a-8b6c-9d0e1f2a3b4c",
  "data": {
    "agentId": "550e8400-e29b-41d4-a716-446655440000",
    "name": "billing-agent",
    "displayName": "Billing Agent",
    "agentType": "ai_agent",
    "status": "verified",
    "trustScore": 0.755
  }
}
```

---

**📖 For more examples, see [Postman Collection](../postman/AIM.postman_collection.json)**
