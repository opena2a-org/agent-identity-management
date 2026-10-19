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

#### POST /api/v1/auth/delete-account

Ask for the deletion of the signed-in user's own account. The request carries the session's access token in the `Authorization` header and the user's password in the body. The password is checked as at sign-in, so a wrong password is recorded as a failed attempt and counts toward the account lockout.

An accepted request deactivates the account at once: sign-in and token refresh refuse it, and a route that requires a signed-in user answers an access token of the account, the one that made the request among them, with `401` and `{"error": "Token has been revoked: the account is deactivated"}`. The account deletion purge, which the server runs at start and every hour, deletes the account at its first run 29 days or more after the request. It deletes the user as `DELETE /api/v1/admin/users/{id}` does, so the same rows go with the user and the same rows are kept: among them, the user's API keys and the user's own audit log rows are deleted, and the agents and MCP servers the user created are kept without a creator. The request does not revoke the account's API keys before then. An admin who activates the account before the purge deletes it (`POST /api/v1/admin/users/{id}/activate`) withdraws the request.

When the purge deletes the last user of an organization, it deletes the organization too, in the same transaction, and with it the rows whose reference to the organization has `ON DELETE CASCADE`: among them the organization's agents, MCP servers, API keys, webhooks and audit log rows. Rows of `a2a_call_chain` and `a2a_security_violations` are kept, with their reference to the organization set to `NULL`. An organization that has a signed audit record chain is kept, as the references of `record_chains` and `record_debts` refuse its delete (`ON DELETE RESTRICT`): the account is deleted all the same, and the server logs the refusal.

**Request:**
```json
{
  "password": "<the user's password>"
}
```

**Response** (202):
```json
{
  "status": "deactivated",
  "deletionRequestedAt": "2026-10-19T12:30:00Z",
  "deletionDueAt": "2026-11-17T12:30:00Z"
}
```

`deletionDueAt` is 29 days after `deletionRequestedAt`, the time from which the purge deletes the account.

A request without a password answers `400`. A wrong password, a locked account and an account without a password answer `401` with `{"error": "Invalid password", "code": "invalidPassword"}`. The route's other `401` answers, which refuse the access token, carry no `code`: a client asks for the password again after the first and refreshes or ends the session after the others. The organization's only active admin is answered `409` with `"code": "lastActiveAdmin"` while the organization has another user who is not deactivated: another user is made an admin first, with `PUT /api/v1/admin/users/{id}/role`. An accepted request is recorded in the audit log as `account_deletion_requested`, a row of the user's own that the deletion removes. The deletion is recorded as `account_deleted`, a row that names no user, whose resource is the deleted user's id and whose `metadata.deletionRequestedAt` is the time of the request. No such row is written when the purge deletes the organization with the account, as the organization's audit log goes with it; the server logs a line that names the user and the organization.

---

#### Sign-in through the organization's OpenID Connect provider

An administrator stores the OpenID Connect provider the users of the organization sign in through: the provider's issuer, and the client id and client secret the provider registered for this deployment. An organization has at most one. A user then signs in at the provider with the authorization code flow and PKCE (`S256`), and the server answers a login pair for the user of that organization whose email the provider asserts. A configuration can also provision users: the first sign-in of an email that no user of the organization has then creates that user.

`PUT /api/v1/admin/oidc-sign-in` (admin role) stores the provider of the caller's organization, in place of any earlier one.

**Request:**
```json
{
  "issuer": "https://idp.example.com",
  "clientId": "<the client id the provider registered>",
  "clientSecret": "<the client secret the provider issued>",
  "provisioning": {
    "enabled": true,
    "role": "viewer",
    "emailDomains": ["example.com"]
  }
}
```

`provisioning` is optional. A body without it, or with `enabled` `false`, stores a configuration that provisions no user, and the role and domains of a provisioning that is not enabled are not stored. With `enabled` `true`, `role` is `viewer`, `member` or `manager`, and `viewer` when it is left out; `admin` is refused, so a provider's assertion never makes a user an administrator. `emailDomains` holds 1 to 20 domain names, such as `example.com`, stored in lower case without a final dot; a wildcard, an address, a URL and a name of one label are refused. A role or domains outside these rules answer `400`.

The issuer must be an `https` URL with no query or fragment. Before it stores anything, the server reads the provider's configuration document, `<issuer>/.well-known/openid-configuration`, and answers `400` when the document cannot be read, names another issuer, or names an authorization, token or key set endpoint that is not an `https` URL. Every request to the provider is sent with the server's egress client, which refuses internal addresses such as loopback. The client secret is stored encrypted with the key vault's master key (`KEYVAULT_MASTER_KEY`) and bound to the organization. No answer and no audit row carries it.

**Response** (200):
```json
{
  "configured": true,
  "issuer": "https://idp.example.com",
  "clientId": "<the client id>",
  "clientSecretSet": true,
  "provisioning": {
    "enabled": true,
    "role": "viewer",
    "emailDomains": ["example.com"]
  },
  "redirectUrl": "https://aim.example.com/auth/oidc/callback",
  "updatedBy": "550e8400-e29b-41d4-a716-446655440000",
  "updatedAt": "2026-10-19T08:00:00Z"
}
```

`provisioning` is `{"enabled": false}` for a configuration that provisions no user. `GET /api/v1/admin/oidc-sign-in` answers the same, or `{"configured": false, "redirectUrl": "..."}` when the organization has no provider. `DELETE /api/v1/admin/oidc-sign-in` removes the provider and answers `204`; a sign-in started before the removal is then refused. A stored provider and a removal are recorded in the audit log as `update` and `delete` of the resource type `oidc_sign_in_config`; the metadata of the `update` row names the issuer, the client id and the provisioning.

`redirectUrl` is the `/auth/oidc/callback` page under `FRONTEND_URL`, the redirect URI every authorization request names; register it with the provider.

In the dashboard, the organization's users sign in from its sign-in link, `<FRONTEND_URL>/auth/login?organization=<organization id>`, where the organization id is the `id` that `GET /api/v1/organizations/current` answers. When `organization` is a UUID, the sign-in page shows a "Sign in with your identity provider" button above the email and password form. The button calls the login route below, keeps the `flow` in the browser tab's session storage, and sends the browser to `authorizationUrl` when it is an `https` URL. The `/auth/oidc/callback` page removes that flow from storage as it reads it and posts it to the callback route with the `code` and `state` of the provider's redirect. A tab that holds no flow posts nothing, so the `code` and `state` of a callback address alone sign no one in. After a completed sign-in the page opens the dashboard, or the page the sign-in link's `returnUrl` names, or the password change page for an account that must change its password; after a refused one it says why and links back to the organization's sign-in link.

`POST /api/v1/auth/oidc/{organizationId}/login` starts a sign-in. It takes no body and no credentials.

**Response** (200):
```json
{
  "authorizationUrl": "https://idp.example.com/authorize?client_id=...&code_challenge=...&code_challenge_method=S256&nonce=...&redirect_uri=...&response_type=code&scope=openid+email+profile&state=...",
  "flow": "<organization id>.<sealed flow>",
  "expiresAt": "2026-10-19T08:10:00Z"
}
```

The browser goes to `authorizationUrl`. The client keeps `flow`, which holds the sign-in's state, nonce and PKCE code verifier encrypted with the key vault's master key and bound to the organization; the server keeps nothing between the two routes. An organization that has no provider, or is not active, answers `404`, and a provider whose configuration document cannot be read answers `502`.

`POST /api/v1/auth/oidc/callback` completes the sign-in, within 10 minutes of its start, with the `code` and `state` the provider redirected with.

**Request:**
```json
{
  "code": "<the code of the redirect>",
  "state": "<the state of the redirect>",
  "flow": "<the flow of the login answer>"
}
```

**Response** (200): the answer of `POST /api/v1/auth/login/local`, `accessToken`, `refreshToken` and `user`. The sign-in is recorded in the audit log as a `login` with `method` `oidc`.

The server redeems the code at the provider's token endpoint with the code verifier, authenticating with `client_secret_basic`, or with `client_secret_post` when the provider's configuration document lists it and not `client_secret_basic`. It accepts the ID token the provider answers only when the token is signed by a key of the provider's key set with RS256, RS384, RS512, PS256, PS384, PS512, ES256, ES384 or ES512; names the configured issuer exactly; has the client id among its audiences, and as its `azp` when it names one or has more than one audience; carries an `exp` that has not passed and an `iat` that is not ahead of the server's clock, each with 30 seconds of leeway, and an `nbf`, when it has one, that has passed; carries a subject; and carries the sign-in's nonce. The user is the user of the organization whose email equals the token's `email`, compared without regard to case, when the token's `email_verified` is `true`. Only the users of the organization the sign-in started for are compared: a provider stored for one organization never signs in a user of another. A configuration that provisions no user creates none.

When no user of the organization has the email and the configuration provisions users, the sign-in creates a user of that organization and signs it in. The user has the email in lower case; the token's `name`, or the email when the token has none, cut to 255 characters; the configuration's role; the status `active`; the provider `oidc`; and no password. The email must be an address at one of `emailDomains` exactly, so an address at a subdomain of a listed domain is not provisioned, and no user of another organization, deleted or not, may have it. The new user is recorded in the audit log as a `create` of the resource type `user` with `method` `oidc_provisioning`, before the `login`. The next sign-in with the email signs the same user in.

A refused sign-in answers `401` with `{"error": "Sign-in through the identity provider was refused", "code": "<reason>"}`:

| `code` | The sign-in was refused because |
|--------|---------------------------------|
| `flow_invalid` | the flow is not one this server sealed for the organization it names |
| `flow_expired` | the sign-in started more than 10 minutes ago |
| `state_mismatch` | `state` is not the state the sign-in started with |
| `provider_changed` | the organization's provider was removed, or its issuer or client id changed, since the start |
| `organization_inactive` | the organization cannot be read or is not active |
| `code_refused` | the provider's token endpoint refused the code |
| `id_token_invalid` | the ID token failed one of the checks above |
| `email_not_verified` | the ID token carries no email, or its `email_verified` is not `true` |
| `no_user` | no user of the organization has the email, and the configuration provisions none |
| `domain_not_provisioned` | the configuration provisions users, and the email is not an address at one of its `emailDomains` |
| `email_in_other_organization` | the configuration provisions users, and a user of another organization has the email |
| `ambiguous_user` | more than one user of the organization has the email |
| `account_inactive` | the user is suspended, deactivated or deleted |

A request without `code`, `state` or `flow` answers `400`, and a provider that cannot be reached answers `502`. Both routes count toward the credential routes' rate limit.

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
  "agentType": "ai_agent",
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
  "agentType": "ai_agent",
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
- `limit` (optional): Page size. When omitted, all agents are returned. A `limit` that is not a positive integer (for example `0`, `-5` or `abc`) is answered `400 Bad Request` with `{"error": "limit must be a positive integer"}`.
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
  "agentType": "ai_agent",
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
- **Credential routes**: 10 requests per minute. These are sign-in, with a password or through the organization's OpenID Connect provider, sign-out, token refresh, re-authentication, account deletion requests, `GET /api/v1/auth/me`, password change and reset, registration and access requests, the OAuth token and device-code routes, SDK token recovery, and bootstrap-token mint and exchange.

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
is signed with the secret the webhook had when the delivery was queued. The rotation is written to the audit log as an `update` of the
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
| `trust_score.changed` | An administrator sets an agent's trust score through `PUT /api/v1/agents/:id/trust-score`, or a capability violation lowers it, and the stored score differs from the one before; or a score that differs from the agent's at two decimal places is stored by a trust score calculation, such as `POST /api/v1/agents/:id/trust-score/recalculate` or `POST /api/v1/trust-score/calculate/:id`, by the recalculation after an agent is updated, verified, suspended, reactivated or revoked, after MCP servers are added to or removed from it, or after a capability is granted or revoked, by the drift penalty for a verification event that reports an MCP server the agent is not registered to talk to, or by the trust re-evaluation job. A registration that stores a score sends the event once, with `previousScore` 0 and the stored score. The score `POST /api/v1/detection/agents/:id/capabilities/report` stores sends no event. |
| `trust_score.low` | After an administrator sets an agent's trust score through `PUT /api/v1/agents/:id/trust-score`, or a capability violation lowers it, the trust score policy finds that a verified or pending agent's score fell and ends below 70%. `severity` is `critical` below 50%, where the policy also suspends the agent, and `warning` otherwise. |
| `alert.created` | An alert is raised for a capability check denied because the agent has no granted capabilities. Other alerts send no event. |
| `alert.acknowledged` | An alert is acknowledged through `POST /api/v1/admin/alerts/:id/acknowledge`. Acknowledging every alert at once, and resolving an alert, send no event. |
| `api_key.created` | An API key is created, including the key created with an agent. |
| `api_key.revoked` | An active API key is disabled through `PATCH /api/v1/api-keys/:id/disable`. |

`security.capability_drift` is sent by the capability drift monitor, which runs every 15 minutes by default
(`DRIFT_MONITOR_INTERVAL`). A run reads each organization's MCP capability drift over the last day, as
`GET /api/v1/supply-chain/drift-alerts` reads it, and sends each drift (server, capability and drift type)
once while the server runs; a drift whose send failed is sent on the next run.

A subscription accepts these names, and no event is sent for them: `api_key.expired`, `security.violation`,
`compliance.violation`, `security.data_exfiltration`. A create or update
naming any other event is refused with `400 {"error":"unsupported event: <name>"}`.

The data of an `agent.*` event is the agent after the change: `agentId`, `name`, `displayName`,
`agentType`, `status` and `trustScore`. A `trust_score.*` event carries `agentId`, `agentName`,
`previousScore` and `trustScore`, and `trust_score.low` adds `severity`. An `api_key.*` event carries
`apiKeyId`, `agentId`, `name`, `prefix` and, for a key that expires, `expiresAt`; it never carries the
key or its hash. A `security.capability_drift` event carries `serverName`, `capabilityName`, `driftType`
(`added`, `removed` or `stale`), `severity` and `detectedAt`, the time of the run that sent it.

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

### Verifying a delivery

Each attempt is signed when it is sent, a retry and a `webhook.test` delivery included:

| Header | Value |
|--------|-------|
| `X-Webhook-Timestamp` | The time the attempt was sent, in Unix seconds. Each retry carries a new one. |
| `X-Webhook-Signature` | `v1=` and the hex HMAC-SHA256, under the webhook's secret, of the string `<X-Webhook-Timestamp>.<raw body>`: the timestamp, a `.`, and the request body exactly as sent. |
| `X-Webhook-ID` | The webhook's `id`. The signature does not cover it. |

`X-Webhook-Event` names the event. Every attempt of one event sends the same body, whose `id`
([Webhook Payload](#webhook-payload)) identifies the event; the body's `timestamp` is when the event was
raised, not when the attempt was sent. `X-Webhook-Signature` holds comma-separated `<scheme>=<hex>`
entries: the server sends one, `v1`, and a later scheme would be added as another entry beside it.

A receiver accepts a delivery only when all three checks pass:

1. **Signature.** Recompute `v1=` and the hex HMAC-SHA256 of `<X-Webhook-Timestamp>.<raw body>` with
   the webhook's secret, and compare it with each entry of `X-Webhook-Signature` in constant time
   (`hmac.compare_digest` in Python, `crypto/subtle.ConstantTimeCompare` in Go). Use the body bytes as
   received, before any JSON parsing. No match means the body or the timestamp was changed, or the secret
   is not this webhook's: refuse the delivery.
2. **Timestamp.** Refuse a timestamp more than 5 minutes before or after the receiver's clock.
3. **Event `id`.** Remember the payload `id` of each event processed, at least until its delivery's
   `X-Webhook-Timestamp` is more than 5 minutes behind the receiver's clock, after which check 2 refuses a
   replay of it. Refuse to process a second delivery of an `id` already processed. A retry of an event the
   receiver did not process, such as one it answered with a `500`, carries the same `id` and a new
   timestamp, and is accepted.

An answer in 2xx ends a delivery, so a duplicate the receiver answers with a 2xx is not retried. At the
defaults in [Delivery and retries](#delivery-and-retries) the attempts of one event span up to 9 minutes
(waits of 60, 120 and 240 seconds, and up to 30 seconds for each of four attempts), longer when an attempt
waits for a free slot; a receiver that keeps each `id` longer than that also processes an event once when
its 2xx answer did not reach the server. One event sent to two webhooks carries the same `id` to both: a
receiver that serves several webhooks looks up the secret by `X-Webhook-ID` and remembers `X-Webhook-ID`
and `id` together. A `webhook.test` delivery has no `id` and no event to process; checks 1 and 2 apply
to it.

Checks 1 and 2 in Python, using only `hmac` and `hashlib`:

```python
import hashlib
import hmac

TOLERANCE_SECONDS = 300


def verify_delivery(secret, timestamp, signature, body, now):
    """Return True when a delivery is signed with secret and was sent within
    5 minutes of now.

    timestamp and signature are the X-Webhook-Timestamp and X-Webhook-Signature
    header values, body is the raw request body as bytes, and now is the
    receiver's clock in Unix seconds, for example time.time().
    """
    signed = timestamp.encode() + b"." + body
    expected = "v1=" + hmac.new(secret.encode(), signed, hashlib.sha256).hexdigest()
    entries = [entry.strip() for entry in signature.split(",")]
    if not any(hmac.compare_digest(expected.encode(), entry.encode()) for entry in entries):
        return False
    return abs(now - int(timestamp)) <= TOLERANCE_SECONDS
```

## Alert channels

An alert channel sends an organization's alerts to Slack, PagerDuty, a Microsoft Teams workflow or a
list of email addresses, each in that service's published format. Every alert the server stores is sent, whichever part of the server raised it, to each active
channel of its organization whose severity floor admits the alert's severity. A repeat that is counted
on an open alert is not sent again.

| Route | Who |
|-------|-----|
| `POST /api/v1/alert-channels` | Admin, manager or member |
| `GET /api/v1/alert-channels` | Any user of the organization |
| `GET /api/v1/alert-channels/:id` | Any user of the organization |
| `PUT /api/v1/alert-channels/:id` | Admin, manager or member |
| `DELETE /api/v1/alert-channels/:id` | Admin, manager or member |
| `POST /api/v1/alert-channels/:id/test` | Admin, manager or member |

A viewer is answered `403` on the routes that change a channel or send a message. Every route that
names a channel looks it up in the caller's organization: a channel of another organization is answered
`404 {"error":"not found"}`, the same as one that does not exist.

### POST /api/v1/alert-channels

Create a channel.

**Request (Slack):**
```json
{
  "name": "Security on-call",
  "kind": "slack",
  "config": {"webhookUrl": "https://hooks.slack.com/services/<the path Slack gives the webhook>"},
  "minSeverity": "warning"
}
```

**Request (PagerDuty):**
```json
{
  "name": "PagerDuty security service",
  "kind": "pagerduty",
  "config": {"routingKey": "<the 32-character integration key>"},
  "minSeverity": "high"
}
```

**Request (Microsoft Teams):**
```json
{
  "name": "Security channel in Teams",
  "kind": "teams",
  "config": {"webhookUrl": "<the HTTP URL the workflow's trigger gives>"},
  "minSeverity": "warning"
}
```

**Request (email):**
```json
{
  "name": "Security team mailbox",
  "kind": "email",
  "config": {"recipients": ["ops@example.com", "sec@example.com"]},
  "minSeverity": "high"
}
```

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | 1 to 100 characters. A name another channel of the organization has is answered `409`. |
| `kind` | Yes | `slack`, `pagerduty`, `teams` or `email`. |
| `config` | Yes | For `slack`, `{"webhookUrl": ...}`: a Slack incoming webhook URL, `https://hooks.slack.com/services/` followed by the webhook's path. For `pagerduty`, `{"routingKey": ...}`: the 32-character integration key of a PagerDuty Events API v2 integration. For `teams`, `{"webhookUrl": ...}`: the URL of a Teams workflow whose trigger is "When a Teams webhook request is received", see [Teams message](#teams-message). For `email`, `{"recipients": [...]}`: 1 to 20 email addresses, see [Email message](#email-message). |
| `minSeverity` | Yes | The severity floor: `info`, `warning`, `high` or `critical`. |
| `isActive` | No | `false` creates the channel inactive. Defaults to `true`. |

**Response (201 Created):** the channel.
```json
{
  "id": "3f1c2b7a-5d4e-4c3b-9a8f-1e2d3c4b5a69",
  "name": "Security on-call",
  "kind": "slack",
  "minSeverity": "warning",
  "isActive": true,
  "config": {"webhookUrl": "****Q7fz"},
  "lastDeliveryStatus": null,
  "lastDeliveryCode": null,
  "lastDeliveryAt": null,
  "createdAt": "2026-10-16T00:00:00Z",
  "updatedAt": "2026-10-16T00:00:00Z"
}
```

Each `config` value of a `slack`, `pagerduty` or `teams` channel is answered as `****` followed by its
last 4 characters, on every route. The webhook URL or routing key is the credential that posts to the
destination: the server stores it as given in its database and sends it only to that destination; no
answer, log line or audit log entry carries it. An `email` channel's recipients are not credentials and
are answered as stored, for example `"config": {"recipients": ["ops@example.com", "sec@example.com"]}`.

A create or update the server refuses is answered `400`:

| Answer | When |
|--------|------|
| `{"error":"unknown field: color"}` | The body has a key the route does not take. A `config` key the kind does not take is named `config.<key>`, for example `unknown field: config.channel`. |
| `{"error":"unsupported kind: sms"}` | `kind` is not `slack`, `pagerduty`, `teams` or `email`. |
| `{"error":"minSeverity must be one of info, warning, high, critical"}` | `minSeverity` is missing or is another value. |
| `{"error":"config.webhookUrl must be a Slack incoming webhook URL: https://hooks.slack.com/services/ followed by the webhook's path"}` | The URL has another form, or the check a webhook URL goes through refuses it. The answer never repeats the URL. |
| `{"error":"config.routingKey must be the 32-character integration key"}` | The routing key is not 32 characters. |
| `{"error":"config.webhookUrl must be the https URL of a Teams workflow's \"When a Teams webhook request is received\" trigger"}` | A `teams` URL that is not `https`, has no host, carries a user name or password, or that the check a webhook URL goes through refuses. Any host is taken, because a workflow's URL depends on its region and environment. The answer never repeats the URL. |
| `{"error":"config.recipients must be a list of 1 to 20 email addresses"}` | `recipients` is not an array of strings, is empty, or holds more than 20. |
| `{"error":"config.recipients[0] must be a bare email address such as ops@example.com, with no display name"}` | An entry, with surrounding spaces removed, is not a bare address: it has a display name, angle brackets, a comma, a CR or LF, or does not parse. The index names the entry. |
| `{"error":"email is not configured on this server"}` | The server started without an email service (its log says `AIM will continue without email notifications`), and the request creates an `email` channel, changes a channel's `kind` to `email`, or gives an `email` channel a `config`. Renaming, a floor change, deactivating and deleting an `email` channel need no email service. |
| `{"error":"name must be at most 100 characters"}` | `name` is longer than 100 characters; an empty one is answered `name is required`. |

### GET /api/v1/alert-channels

The organization's channels: `{"alertChannels": [<channel>, ...], "total": <count>}`. In each channel,
`lastDeliveryStatus` is `delivered` or `failed` after its latest delivery, `lastDeliveryCode` is the HTTP
status of that delivery's final attempt (`null` when no answer came, and always `null` for an `email`
channel), and `lastDeliveryAt` is when it ended. All three are `null` until the first delivery.

### GET /api/v1/alert-channels/:id

One channel.

### PUT /api/v1/alert-channels/:id

Update a channel. The body takes the create fields, all optional, refused as on create. When `config`
is left out, the stored destination stays. A `config` replaces the stored one whole and is checked as on
create; changing `kind` needs one. The answer is the channel.

### DELETE /api/v1/alert-channels/:id

Delete a channel. The answer is `204 No Content`.

A create, update and delete each write one audit log entry: action `create`, `update` or `delete`,
resource type `alert_channel`, and metadata `kind` and `minSeverity` only.

### POST /api/v1/alert-channels/:id/test

Send one test message through the channel at once, with no retry, and record it as the channel's last
delivery. The request has no body.

- Slack is sent `{"text": "AIM test message for alert channel <name>"}`, the name escaped as in
  [Slack message](#slack-message).
- PagerDuty is sent a `trigger` event with `payload.severity` `info`, `payload.summary`
  `AIM test event for alert channel <name>` and `dedup_key` `aim-channel-test-<channel id>`. **A test
  message to PagerDuty opens an alert on the PagerDuty service** the routing key belongs to, which
  notifies as that service is set up to; resolve it in PagerDuty.
- Teams is sent the [Teams message](#teams-message) with one `TextBlock`,
  `AIM test message for alert channel <name>`.
- Each email recipient is sent one plain-text message with the subject
  `AIM test message for alert channel <name>` (CR and LF removed, cut to 200 characters).

When the destination accepts the message, the answer is `200`; for an `email` channel, when every
recipient was sent it, with `statusCode` `null`:
```json
{"delivered": true, "statusCode": 200}
```

Otherwise it is `502`, with the status the destination answered (`null` when no answer came) and the
class of the failure, never the destination: `status <code>`, `timeout`, `refused address` (an address
the server does not send to) or `unreachable`. For an `email` channel the class is `email_unavailable`
when the server has no email service, and `email_failed` when the email service did not send the
message to every recipient.
```json
{"delivered": false, "statusCode": 404, "error": "status 404"}
```

### Severity floor

The severities rank `info` < `warning` < `high` < `critical`. A channel is sent each alert whose severity
ranks at or above its `minSeverity`: a `warning` channel is sent `warning`, `high` and `critical` alerts;
a `critical` channel only `critical` ones. An alert stored with any other severity is sent to no channel.

### Slack message

A `POST` to the channel's webhook URL with `Content-Type: application/json` and a body that carries
`text` and nothing else (no `blocks`). The text has these lines:

1. `AIM alert, severity <severity>: <title>`
2. the alert's description
3. `Agent: <agent name>`, only when the alert names an agent
4. `<FRONTEND_URL>/dashboard/admin/alerts`, the dashboard's alerts page

Slack reads `&`, `<` and `>` as control characters, so every string taken from the alert is sent with
them written as `&amp;`, `&lt;` and `&gt;`: an alert titled `<!here> & co` is shown as that text and does
not notify the channel.

```json
{"text": "AIM alert, severity high: Capability Violation Detected: billing-agent\nAgent 'billing-agent' attempted unauthorized capability 'db:write' ...\nAgent: billing-agent\nhttps://aim.example.com/dashboard/admin/alerts"}
```

### PagerDuty event

A `POST` to `https://events.pagerduty.com/v2/enqueue`, the PagerDuty Events API v2, with
`Content-Type: application/json`. The address is fixed; a channel names only its routing key.

```json
{
  "routing_key": "<the channel's routing key>",
  "event_action": "trigger",
  "dedup_key": "9b2e4c1a-7f3d-4e6b-8a5c-2d1f0e9b8a7c",
  "payload": {
    "summary": "Capability Violation Detected: billing-agent",
    "source": "billing-agent",
    "severity": "error",
    "timestamp": "2026-10-16T22:40:00Z",
    "class": "security_breach",
    "custom_details": {
      "alertId": "9b2e4c1a-7f3d-4e6b-8a5c-2d1f0e9b8a7c",
      "organizationId": "7d9f2b4a-1c3e-4f5a-8b6c-9d0e1f2a3b4c",
      "alertType": "security_breach",
      "severity": "high",
      "description": "Agent 'billing-agent' attempted unauthorized capability 'db:write' ...",
      "resourceType": "agent",
      "resourceId": "550e8400-e29b-41d4-a716-446655440000"
    }
  }
}
```

| Field | Value |
|-------|-------|
| `dedup_key` | The alert's id. |
| `payload.summary` | The alert's title, cut to 1,024 characters. |
| `payload.source` | The alert's agent name; for an alert that names no agent, the host of `FRONTEND_URL`. |
| `payload.severity` | AIM `critical` is `critical`, `high` is `error`, `warning` is `warning`, `info` is `info`. |
| `payload.timestamp` | When the alert was raised, RFC 3339. |
| `payload.class` | The alert type. |
| `payload.custom_details` | The alert's id, organization, type, AIM severity, description, resource type and resource id. |

### Teams message

A `POST` to the channel's webhook URL, the URL of a Teams workflow whose trigger is "When a Teams
webhook request is received", with `Content-Type: application/json` and **no `Authorization`
header**: with the trigger's "Anyone" option, a request that carries one fails. The body is a message
with one Adaptive Card:

```json
{
  "type": "message",
  "attachments": [
    {
      "contentType": "application/vnd.microsoft.card.adaptive",
      "contentUrl": null,
      "content": {
        "$schema": "http://adaptivecards.io/schemas/adaptive-card.json",
        "type": "AdaptiveCard",
        "version": "1.2",
        "body": [
          {"type": "TextBlock", "text": "AIM alert, severity high: Capability Violation Detected: billing-agent"},
          {"type": "TextBlock", "text": "Agent 'billing-agent' attempted unauthorized capability 'db:write' ..."},
          {"type": "TextBlock", "text": "Agent: billing-agent"},
          {"type": "TextBlock", "text": "https://aim.example.com/dashboard/admin/alerts"}
        ]
      }
    }
  ]
}
```

`body` holds one `TextBlock` for each line of the [Slack message](#slack-message) text, with the
alert's strings as written: Slack's `&amp;`, `&lt;` and `&gt;` escaping is not applied. Any `2xx`
answer is delivered, and the retries are those of every HTTP delivery (see below).

Workflows whose trigger accepts only requests from a Microsoft Entra tenant (the tenant-authenticated
options of the trigger, which need a Microsoft Entra token on the request) are not supported: the server
sends no token, and such a workflow refuses its requests.

### Email message

The server's email service (`EMAIL_PROVIDER`) sends each recipient its own plain-text message, so a
recipient does not see the others:

- Subject: `AIM alert (<severity>): <title>`, with CR and LF removed and cut to 200 characters, so an
  alert's title cannot add a header line.
- Body:

```text
AIM alert, severity high: Capability Violation Detected: billing-agent

Agent 'billing-agent' attempted unauthorized capability 'db:write' ...

Agent: billing-agent
Alert ID: 9b2e4c1a-7f3d-4e6b-8a5c-2d1f0e9b8a7c
Alert type: security_breach

https://aim.example.com/dashboard/admin/alerts
```

The description and `Agent:` lines are there only when the alert has them. Each recipient's message
is sent separately: a recipient the email service fails to send to is tried again up to 3 times, after
waits of 1, 4 and 16 seconds, whatever the failure, and does not stop the others. The delivery is
`delivered` when every recipient was sent the message and `failed` otherwise, with
`lastDeliveryCode` `null` either way. A log line names the channel and how many recipients failed,
with any address in the email service's error replaced by `[address]`. When the server has no email
service, an existing `email` channel's deliveries are `failed` with the class `email_unavailable`.
With `EMAIL_PROVIDER=console` the messages are written to the server's output, not sent.

No body carries an AIM credential, the channel's config beyond its own destination, or another
channel's data.

### Delivery, retries and the queue bound

Each stored alert is put on a queue that holds 1,024 alerts; storing the alert never waits for it. Four
workers take alerts off the queue. A worker sends an alert to all of its channels at the same time and
takes the next alert when each of those deliveries has ended.

An attempt waits up to 10 seconds for the destination. A `2xx` answer is delivered. A `429`, a `5xx` or
an attempt that got no answer is retried up to 3 times, after waits of 1, 4 and 16 seconds, so a
delivery makes at most 4 attempts. Any other answer, and an address the server does not send to, is
not retried. The final outcome is recorded as the channel's last delivery. Requests follow no redirect
and go to the destination directly, whatever `HTTP_PROXY` or `HTTPS_PROXY` say; the address of every
connection is checked when it is opened, as for webhooks. These rules are those of the Slack,
PagerDuty and Teams deliveries. An email delivery goes through the server's email service, which
opens its own connections; the 10-second bound does not apply to it, and its retries are those of
[Email message](#email-message).

When 1,024 alerts are already waiting, a new alert is sent to no channel: the server logs
`alert <alert id>: alert channel queue full`, and the alert itself is stored and shown as usual. Alerts
still in the queue when the server stops are not sent.

---

**📖 For more examples, see [Postman Collection](../postman/AIM.postman_collection.json)**
