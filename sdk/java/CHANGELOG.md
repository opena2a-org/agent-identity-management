# Changelog

All notable changes to the AIM Java SDK will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- `registerPQCKey(..., true)` (both overloads) and `setHybridMode(true)` now throw `ConfigurationException`
  before sending anything. They sent `"enableHybrid": true` and `"enable": true`, which mark the agent as
  requiring both an Ed25519 and an ML-DSA signature, and no AIM SDK signs requests in that form yet. Register
  the key with `registerPQCKey(pqcPublicKey, algorithm, false)`; `setHybridMode(false)` still turns hybrid
  mode off.
- A JSON string is now refused where the SDK reads a trust or confidence number, whatever its length and
  including a numeric one such as `"0.75"`, which was converted to a number before. This covers
  `A2ATrustScore` (`score`, `a2aTrustScore`, `peerTrustAverage`, `confidence`, `factors`), `A2APeerTrust`
  (`peerTrustScore`, `successRate`), `A2ASecurityCheckResult.requesterTrustScore`,
  `A2ASecuritySettings.minTrustScore` and `A2AConsensusResult.confidenceScore`, and every other `Double`,
  `double`, `Float`, `float`, `BigDecimal` or `Number` member of a class the SDK maps from JSON, its local
  cache files included. The MCP server trust scores that `MCPIntegration` reads from the
  JSON tree, rather than mapping them, are unchanged. The method throws its usual exception (`A2AException` from `A2AClient`) with a `MismatchedInputException` as
  its cause, whose message names the field and the string's length and never the string:
  `Field "/score": a JSON string of 4 characters was refused where a number is bound; a JSON number is expected`.
  A JSON number reads as before. AIM servers send these fields as JSON numbers; a proxy, mock or other server
  that sends them as strings needs to send numbers.
- `MCPIntegration.attestServer` (both overloads) now throws `AIMException` before sending anything. It sent
  the Base64 of the attestation in the `signature` member, where the server verifies an Ed25519 signature
  made with the agent's private key, and `MCPIntegration` cannot reach that key.
- The project URL in `pom.xml` and the first link in the README go to the project page,
  https://opena2a.org/agent-identity-management. They pointed at the GitHub repository and at the license
  file. The repository stays listed under `<scm>`.

### Deprecated

- `AIMClient.createHybridRequestHeaders` and `PQCOperations.createHybridRequestHeaders`. AIM's request
  verification does not accept these headers: it verifies a signature over the method, path, timestamp and
  body joined by newlines, and these headers sign `timestamp:METHOD:path:sha256(body)`. Their javadoc said
  they were compatible with the AIM backend; it now says they are not accepted.

### Fixed

- `MCPIntegration.registerServer`, `listServers`, `recordToolUsage` and `verifyAction` send the AIM client's
  access token in the `Authorization` header. They sent `Bearer ` with an empty token. Without an access token
  to send they now throw `AIMException` before any request is sent; `verifyAction` returns false, as it does on
  any error.
- `AIMClient.listAgents` URL-encodes its query parameters. A `status` holding `&`, `#`, `=` or `+` was
  appended as written, so it could add or override query parameters or cut the URL short; the server now
  receives it as the value of the one `status` parameter.
- After the server refuses a refresh, `AIMClient` sends the old refresh token to `/api/v1/auth/sdk/recover`
  under `oldRefreshToken`, the key the server reads. It sent `old_refresh_token`, a key the server does not
  read, so no recovery request could succeed. `AIMClientSdkRecoverRouteTest` reads the key from the backend's
  recovery handler and fails when the client sends another (#603).
- The recovery request carries the access token `AIMClient` holds in its `Authorization` header. The route is
  mounted behind the auth middleware and mints only for the owner of the revoked refresh token, so a request
  without a bearer was answered 401 before its body was read, and recovery could not succeed with the key
  corrected either. A client that holds no access token, because no earlier refresh succeeded, sends no
  recovery request. `AIMClientSdkRecoverRouteTest` reads the route's registration from the backend and answers
  a recovery request without the held bearer with 401 (#625).
- A client idle past its access token's expiry sends no recovery request after the server refuses a refresh,
  and logs that the access token it holds has expired and when. `ensureValidToken` refreshes 60 seconds before
  the token's `exp`, so a client whose next call came later than `exp` sent the expired token as the recovery
  bearer, which the auth middleware answers with 401. The call now fails as it does for a client that holds no
  access token (#630).
- `registerMcp` adds the given MCP server to the agent's list. It sends `PUT /api/v1/agents/{id}/mcp-servers`
  with `mcpServerIds`, `detectedMethod` and `confidence`, the members the server reads, and returns what AIM
  answers: `message`, `talksTo`, `added_servers` and `total_count`. It posted `mcp_server_ids` and
  `detected_method` to `POST /api/v1/sdk-api/agents/{id}/mcp-servers`, the route that creates an MCP server and
  reads neither key, so the given server was never added to the agent's list. AIM adds to an agent's list only
  for a signed-in user with the member role or higher; a refusal raises `AIMException` with the status. The
  method does not create an MCP server. `AIMClientRegisterMcpRouteTest` reads the members and the route's
  member role gate from the backend source (#625).
