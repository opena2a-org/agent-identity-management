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
  read, so no recovery request could succeed. The route also requires an access token for the refresh token's
  owner, and the recovery request carries none. `AIMClientSdkRecoverRouteTest` reads the key from the backend's
  recovery handler and fails when the client sends another (#603).
