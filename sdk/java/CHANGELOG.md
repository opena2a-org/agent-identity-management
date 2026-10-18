# Changelog

All notable changes to the AIM Java SDK will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Security

- **Breaking change for code that calls `requireSignedRequests` or `maxTimestampAge`.** `A2ASecuritySettings`
  carries the member names the server reads and answers with: `requireRequestSigning`
  (`isRequireRequestSigning`, `setRequireRequestSigning`, `Builder.requireRequestSigning`) in place of
  `requireSignedRequests`, and `nonceValiditySeconds`, in seconds, in place of `maxTimestampAge`. It adds
  `minAttestationCount`, `maxRequestsPerMinute`, `maxRequestsPerHour`, `createdBy` and `updatedBy`. The server
  reads neither old name, and it replaces every setting with the ones a `PUT /api/v1/a2a/security/settings`
  carries. Run against the server's handler, a body with `requireSignedRequests: true` and no
  `requireRequestSigning` was answered `200` and stored request signing off, and stored `nonceValiditySeconds`,
  `minAttestationCount`, `maxRequestsPerMinute` and `maxRequestsPerHour` as 0 where the body left them out. To
  change some settings, read them with `getSecuritySettings()` and pass the whole object, changed, to
  `updateSecuritySettings()`. The request is pinned in
  `sdk/typescript/tests/fixtures/sdk_requests/a2a_update_security_settings.json`, which the backend's handler
  tests post to that route and the TypeScript SDK's tests hold its client to (#652).
- `updateSecuritySettings` sends the settings members and none of `id`, `organizationId`, `createdBy`,
  `updatedBy`, `createdAt` and `updatedAt`, which are read from the server's answer, and it no longer sends
  `strictMode` and `monitorMode`. Settings read with `getSecuritySettings()`, changed and passed to
  `updateSecuritySettings()` were answered `400 {"error":"Invalid request body"}`, measured with the server's
  answer for an organization with no stored settings: the update sent `createdAt` and `updatedAt` as numbers,
  which the server does not read as a time. That update now sends every setting the server answered, with the
  change (#652).

### Added

- `CredentialManager.withSdkCredentialsLock` runs an action while it holds a lock on the SDK credentials
  file: `~/.aim/sdk_credentials.json.lock`, beside `~/.aim/sdk_credentials.json`, or the same name beside
  another credentials file passed to the overload that also takes a timeout. The lock keeps processes of this
  SDK apart, and threads of one process. On Linux it does not keep a process of the Java SDK apart from a
  process of the Python SDK: the Python SDK locks the same file with `flock`, the Java SDK with a record lock
  (`FileChannel.tryLock`), and Linux holds the two separately. Measured with the calls each SDK makes, on
  Linux 7.0.12 with Java 21.0.12 and Python 3.12.15: each took its lock while the other held its own, in both
  orders. On macOS 26.6.2, with Java 25.0.2 and Python 3.14.3, each was refused its lock while the other held
  its own, in both orders (#722). It waits up to `CredentialManager.SDK_CREDENTIALS_LOCK_TIMEOUT_MILLIS`
  (60 seconds) for another holder and, when the lock is not acquired in that time, throws `CredentialException`
  without running the action. A thread that holds the lock takes it again without waiting. On a file system
  with POSIX permissions the lock file is created readable and writable by the owner only.
  `CredentialManager.readSdkCredentialsFile` reads the credentials file alone, without environment variables,
  migration or discovery, so a process holding the lock reads the refresh token another process saved.
  `CredentialManager.withSdkCredentialsLock(Path, LockedAction)` takes the lock on another credentials file
  with the same 60-second wait.
- `CredentialManager.SHARED_ACCESS_TOKEN` and `CredentialManager.SHARED_ACCESS_TOKEN_REFRESH_TOKEN_SHA256`: the
  credentials keys of the access token a refresh obtained and the SHA-256 of the refresh token it was issued
  with. They are read from and saved to the file as `"sharedAccessToken": {"token": ..., "refreshTokenSha256":
  ...}`, as the Python SDK writes them.
- `AgentType` constants for the fourteen agent types the server accepts at registration that it did not list:
  `GEMINI`, `LLAMA`, `MISTRAL`, `COHERE`, `LLAMAINDEX`, `LANGGRAPH`, `HAYSTACK`, `SEMANTIC_KERNEL`, `COPILOT`,
  `ASSISTANT`, `CHATBOT`, `AUTOGPT`, `BABYAGI` and `AI_AGENT` (`ai_agent`). `AgentType.fromString` returns
  the constant for each of their values, where it returned `UNKNOWN` (#716).

### Changed

- `CredentialManager.saveSdkCredentials` holds that lock while it writes, waiting up to 60 seconds for
  another holder, and throws `CredentialException` without writing when the lock is not acquired. It writes the
  credentials to a temporary file in the same directory and moves it over the credentials file, so a process
  reading the file reads the old credentials or the new ones. On a file system with POSIX permissions the
  temporary file is created readable and writable by the owner only. On Windows, when another process has the
  credentials file open and the move is refused, the file is written in place instead.
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
  `LocalAtxVerifier.verify` throws nothing for a credential whose `trustScore` is a string: it rejects it as
  `MALFORMED`, with that message in its reason. A JSON number reads as before. AIM servers send these fields as JSON numbers; a proxy, mock or other server
  that sends them as strings needs to send numbers.
- `MCPIntegration.attestServer` (both overloads) now throws `AIMException` before sending anything. It sent
  the Base64 of the attestation in the `signature` member, where the server verifies an Ed25519 signature
  made with the agent's private key, and `MCPIntegration` cannot reach that key.
- The project URL in `pom.xml` and the first link in the README go to the project page,
  https://opena2a.org/agent-identity-management. They pointed at the GitHub repository and at the license
  file. The repository stays listed under `<scm>`.
- `requestCapability(capabilityType, reason, metadata)` no longer sends `metadata`, and logs a warning when it
  is given one. The capability request route binds `capabilityType` and `reason` only, so the metadata was not
  stored; put the context an approver needs in the reason (#716).
- Reconnecting to an existing agent (`AIMClient.secure` for an agent name AIM already holds) no longer sends
  `tagIds` in its `PUT /api/v1/agents/{id}`: the update route does not read them. Tags are applied when AIM
  creates the agent, and the javadoc of `Builder.tags` and `secure` says so (#716).
- After the server refuses a refresh, `AIMClient` presents the refused token to `POST /api/v1/auth/refresh` only.
  When it held an unexpired access token, it sent the refused token to `POST /api/v1/auth/sdk/recover` with that
  access token as the bearer, after printing "Token was revoked - attempting automatic recovery...". The access
  tokens `AIMClient` holds are minted by refreshing a downloaded SDK's token: they carry the download's token
  family and no sign-in time, and the recovery route answers such a bearer `401` before it reads the old token.
  That request and its line are removed, with the client's `attemptTokenRecovery` method.
- The `SDK TOKEN EXPIRED` banner printed after a refusal lists the server's maximum age for a download: "The SDK
  download reached the server's maximum age (SDK_TOKEN_FAMILY_MAX_AGE, 365 days by default). Refreshing does not
  extend it; a fresh download starts a new one." The server answers that refresh `401`
  `{"error":"Invalid or expired refresh token"}`. The banner's dashboard steps name "Developers, then SDK & docs"
  for the download and "Developers, then Credentials" for SDK tokens, the dashboard's tabs, where it named
  "Settings -> SDK Downloads" and "Settings -> SDK Tokens".
- Test: `AIMClientSdkRecoverRouteTest` refuses the second refresh with `Invalid or expired refresh token` and with
  `Token has been revoked or is invalid`, and for each checks that the client refreshes twice, sends nothing to the
  recovery route, prints the banner once with the maximum-age cause and the two dashboard steps, and prints no
  "Settings" and no "recovery". It reads from the backend source that the recovery handler refuses an access token
  without a sign-in before it reads the old token, that `jwt.go` gives an SDK token's refreshed access token no
  sign-in time, the `8760h` and `365 * 24 * time.Hour` defaults and the maximum-age refusal body, and from
  `apps/web/lib/hub-tabs.ts` that the dashboard's Developers tabs include SDK & docs and Credentials. On the
  previous client both refusal cases fail: each sent `POST /api/v1/auth/sdk/recover`.

### Deprecated

- `AIMClient.createHybridRequestHeaders` and `PQCOperations.createHybridRequestHeaders`. AIM's request
  verification does not accept these headers: it verifies a signature over the method, path, timestamp and
  body joined by newlines, and these headers sign `timestamp:METHOD:path:sha256(body)`. Their javadoc said
  they were compatible with the AIM backend; it now says they are not accepted.
- `AIMClient.logCapabilityResult`, `logSuccess` and `logFailure` send nothing and log a deprecation warning,
  as the Python SDK's `log_capability_result` does. They posted to
  `POST /api/v1/sdk-api/verifications/{id}/result`, a route the server marks withdrawn and refuses for every
  caller, so the post recorded nothing. `performAction` no longer calls them: it sends the verification request
  and nothing after the action runs, where it posted a result after the action returned and after it threw (#716).
- `AIMClient.Builder.clientId` and `clientSecret`. See the client ID and secret entry under Fixed (#716).

### Fixed

- Two tests that `mvn install` runs failed on a machine whose stored SDK credential (`~/.aim/sdk_credentials.json`)
  held an empty `sdkTokenId` and named an AIM server that was not running. Run with such a credential (`aimUrl`
  `http://127.0.0.1:18471`, nothing listening there), `AIMClientIntegrationTest.loadCredentials` threw
  `StringIndexOutOfBoundsException` (`Range [0, 8) out of bounds for length 0`) taking the token ID's first 8
  characters, and `RealWorldIntegrationTest.testAIMClientLive` called that server and failed (`Failed to connect to
  /127.0.0.1:18471`): `RealWorldIntegrationTest` took the server as available whenever a credential was found. A
  token ID or client ID of 8 characters or fewer is now printed whole, and the two live tests of
  `RealWorldIntegrationTest` run only when the credential holds a refresh token, or a client ID and a client secret,
  none of them blank, and its server answers `GET /health` with `200` within two seconds; otherwise they are
  skipped. With that credential the two classes ran 18 tests with 2 errors before this change, and run 18 with no
  error and 8 skipped after it (#792).
- Test: `src/test/java/org/opena2a/aim/LiveAimTest.java` (#792).
- `listAgents` documents the four agent statuses the server defines, `pending`, `verified`, `suspended` and
  `revoked`, as the values of its `status` filter. It listed `denied`, which no agent holds (#716).
- A client built with a client ID and secret and no refresh token throws `AuthenticationException` when it
  signs in, before sending anything, with a message that names the refresh token as the sign-in to use. The
  server's token route (`POST /api/v1/oauth/token`) accepts only the jwt-bearer grant. The client's sign-in
  built a `client_credentials` grant for `/oauth/token`, and `getAccessToken()` on such a client threw
  `StackOverflowError`. `AIMClient.secure` now refuses SDK credentials that hold a client ID and secret and no
  refresh token with the same exception, before it builds a client. The README and the Java SDK guide configure
  a refresh token (#716).
- `listMcpServers` reads the servers from the `mcpServers` member that
  `GET /api/v1/sdk-api/agents/{id}/mcp-servers` answers with, and returns at most `limit` of them. It read a
  `servers` member, which that answer does not carry, and returned an empty list. It no longer sends
  `?limit=`, which the route does not read. An answer without an `mcpServers` array throws `AIMException`, and
  a negative `limit` throws `IllegalArgumentException` before any request is sent, as
  `MCPIntegration.listServers` does (#716).
- `getSupportedAlgorithms` reads `GET /api/v1/crypto/algorithms` and returns the classical, post-quantum and
  hybrid entries of its answer in one list, in that order. It requested `/api/v1/pqc/algorithms`, a path the
  server does not register, and read the answer as a list, where the server answers an object holding
  `classical`, `postQuantum` and `hybrid` lists (#716).
- `deleteAgent` returns `{"success": true}` when the server answers the deletion with 204 and no body, as it
  does on success. It threw `AIMException` (`No content to map due to end-of-input`) on that answer (#716).
- `waitForApproval` puts the `denialReason` the server answers into the `ActionDeniedException` of a denied
  verification. It read a `message` member, which the answer does not carry, so the exception said
  `Action denied by administrator`. A verification the server answers as `expired` ends the wait with
  `ActionDeniedException`; it was treated as pending and polled until the timeout, which ended with
  `VerificationException`. `verifyCapability` with JIT access waits through the same code (#716).
- Automatic MCP server registration no longer sends `verificationMethod`, which `CreateMCPServerRequest` does
  not bind, as the Python SDK's automatic registration already does (#716).

- `AgentType.UNKNOWN` is sent as `ai_agent`, the value of `AgentType.AI_AGENT`, so registering an agent with it
  and filtering `listAgents` by it use an agent type the server defines. It was sent as `unknown`, which is not
  one of the agent types the server accepts at registration. `AgentType.fromString` still returns `UNKNOWN`
  for a value no other constant has, `unknown` included (#716).
- Processes sharing `~/.aim/sdk_credentials.json` refresh it once. The server accepts each refresh token once,
  and an `AIMClient` created from SDK credentials (`AIMClient.secure`, or `Builder.credentials`) presented the
  token it read at startup, so when two processes refreshed together the second presented a token the first
  had already rotated and was refused. The refresh now runs under the credentials lock and starts by re-reading
  the file. When another process already refreshed, the client uses the access token that process saved with
  the stored refresh token and sends no request; otherwise it presents the refresh token the file holds now.
  Every refresh saves the access token with the refresh token, rotated or not. When the lock is not acquired
  within 60 seconds the refresh is not sent and `AuthenticationException` is thrown. A refresh token the server
  refused with 401 is not presented again by the same client. A client built with only `Builder.refreshToken`
  does not read or write the file, as before.
- `registerMcp` also sends `metadata`, the fourth member the route binds, and the `AIMException` of a 401 or
  403 says that the route admits a signed-in user with the member role or higher and keeps the status code
  (`getStatusCode()`). It returned 500 there (#623).
- `MCPIntegration.registerServer`, `listServers`, `recordToolUsage` and `verifyAction` send the AIM client's
  access token in the `Authorization` header. They sent `Bearer ` with an empty token. Without an access token
  to send they now throw `AIMException` before any request is sent; `verifyAction` returns false, as it does
  when a request fails.
- `MCPIntegration.recordToolUsage` sends `mcpServerId`, `toolName`, `mcpUrl`, `mcpName` and `connectionType`, the
  members `POST /api/v1/sdk-api/agents/{id}/mcp-connections` binds. It sent `mcp_server_id`, `tool_name`,
  `mcp_url`, `mcp_name` and `connection_type`, which the route does not bind, and the route's handler answered
  each such body 400: `mcp_server_id is required` when the agent ID was a UUID, `Invalid agent ID` otherwise.
- `MCPIntegration.listServers` reads the servers from the `mcpServers` member that
  `GET /api/v1/sdk-api/agents/{id}/mcp-servers` answers with. It read a `servers` member, which that answer does
  not carry, and returned an empty list. An answer without an `mcpServers` array now throws `AIMException`.
- The route reads no query parameter, and `listServers` sends none. `listServers(client)` returns every server in
  the answer; `listServers(client, limit, offset)` skips `offset` of them and returns at most `limit`, and throws
  `IllegalArgumentException` for a negative `limit` or `offset`.
- `MCPIntegration.verifyAction` posts the tool name as `capability` to
  `POST /api/v1/mcp-servers/{id}/verify-capability` and returns true only when the answer's `allowed` is the JSON
  value `true`. It posted to `/api/v1/mcp-servers/{id}/verify-action`, which the server does not register, and
  read a `verified` member. The server ID goes into the request path, so a `serverId` that is not a UUID in
  8-4-4-4-12 hex form, or an empty `toolName`, now throws `IllegalArgumentException` before any request is sent.
- `MCPIntegration.attestServer`'s refusal names `AIMClient.attestMcp`, which signs the attestation with the private
  key the client holds.
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
