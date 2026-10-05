# Changelog

All notable changes to the AIM platform are documented here. The format is based
on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and from this release
forward the platform follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

> Scope: this changelog tracks the **platform** (backend + dashboard), tagged
> `platform-v<version>`. The SDKs are versioned and released independently
> under their own `sdk-*-v<version>` tags (`aim-sdk` on PyPI, `@opena2a/aim-sdk`
> on npm; the Java SDK is built from source in `sdk/java`).

## [Unreleased]

### Fixed — the Java SDK's `useMcpTool` reaches the usage report route

- `AIMClient.useMcpTool` posted to `/api/v1/sdk-api/agents/{id}/mcp-usage`, a path the API does not
  register, with a body no handler reads. Every call was answered 404. The method reports a failure
  as `success: false` in its result and a warning in the log rather than throwing, so tool usage
  recorded this way never arrived and nothing stopped.
- It now posts to `POST /api/v1/sdk-api/agents/{id}/mcp-usage-report` with the report that route
  reads: one use of the named tool on the named MCP server. `serverId` has to be the server's id as
  returned at registration; a report for any other value is answered 404.
- The `mcpUrl` and `mcpName` arguments are not part of a usage report and are not sent. The method
  signature is unchanged.

### Removed — the unread token lifetime defaults in the backend configuration

- `apps/backend/internal/config/config.go` read `JWT_ACCESS_TTL` and `JWT_REFRESH_TTL` with defaults of 24 hours and
  7 days into two fields nothing used. The server's token lifetimes come from `NewJWTService`, which reads
  `JWT_ACCESS_TTL` (2 hours by default), `JWT_REFRESH_TTL` (168 hours) and `JWT_SESSION_MAX_AGE` (8 hours). The unread
  fields and their defaults are removed, so each lifetime has one default. Token lifetimes are unchanged.
- `apps/backend/internal/config/config_test.go` fails when `config.go` reads any of the three lifetime variables.

### Fixed — the agent page shows why a call was refused and where to grant it

- The agent page's activity timeline listed a refused call as `Denied` with its resource, risk and trust score, but not
  the reason, although `GET /api/v1/agents/{id}/activity` returns it in `metadata.denialReason`, the same text the SDK
  prints in its `ActionDeniedError`. A refused call now shows what was refused, the server's reason verbatim, and the
  fix. A capability the agent was not granted points at Capability requests, where an administrator approves the
  agent's request, and at `POST /api/v1/agents/{id}/capabilities` with the capability. An unverified agent points at
  Verify agent, a compromised agent at its security review, and a named policy at Security policies. The links go to
  administrator-only pages and are shown to administrators only.
- `apps/web/app/dashboard/agents/[id]/page.refused-call.test.tsx` fails when the page renders a refused activity row
  without its reason or its grant path; `apps/web/lib/refused-call.test.ts` covers each kind of refusal.

### Removed — the unread `users.password_reset_expires` column and its index

- The `users` table carried two password reset expiry columns. The reset flow reads and writes
  `password_reset_expires_at`; `password_reset_expires`, added later with the partial index
  `idx_users_password_reset_expires`, was never read or written and held only `NULL`s. Migration 112 drops the column
  and the index. It is a no-op on a database that does not have them, and leaves `password_reset_expires_at`,
  `password_reset_token` and `idx_users_password_reset_token` unchanged.
- `apps/backend/internal/infrastructure/repository/users_password_reset_columns_integration_test.go` fails when a
  migrated database still has the unread column or its index, or is missing the column the reset flow reads.

### Removed — the second backend Dockerfile; the quickstart test builds the published one

- `apps/backend/infrastructure/docker/Dockerfile.backend` was a second backend Dockerfile, built only by
  `scripts/test-quickstart.sh`. Its runtime stage took the floating `alpine:latest`, it built the server with cgo and
  stamped no version, so the quickstart test ran an image no release ships, on base bytes that could change without a
  commit. It is removed, and `scripts/test-quickstart.sh` builds `infrastructure/docker/Dockerfile.backend`, the file
  the published `aim-server` image is built from.
- `sdk/typescript/tests/dockerfile-backend-single-source.test.ts` fails when a Dockerfile other than the one
  `docker-publish.yml` builds compiles `./cmd/server`, when the quickstart test builds another file, or when any
  Dockerfile in the tree takes a base with no digest and no tag or the tag `latest`.

### Removed — a JWT is no longer accepted as an agent's ATC

- The server checked agent ATCs, in an `Authorization: ATC` header or in the secrets resolve request body, with a
  verifier that fell back to a JWT format when a token was not a CBOR ATC. Nothing issued that JWT format: the server
  had a function to mint it, but no route or service called it. The fallback is removed, and the server accepts the
  CBOR ATC of the ATC verification spec only. A JWT presented as an ATC is refused; in the header, with `401` and
  `atc_malformed`.
- `apps/backend/internal/infrastructure/atc/server_verifier_test.go` and
  `apps/backend/internal/interfaces/http/middleware/atc_auth_test.go` fail when the verifier the server builds accepts
  a JWT-format token signed with the server key.

### Fixed — a logged-out SDK-download token no longer shows as active in the SDK token list

- `POST /api/v1/auth/logout` with an SDK-download refresh token in the body put the token on the denylist, so the
  refresh route refused it, but left its `sdk_tokens` row active: `GET /api/v1/users/me/sdk-tokens` and the dashboard
  kept listing the logged-out token as active. Logout now also marks that row revoked, with the reason `logout`. A row
  already revoked, by rotation or an earlier logout, keeps its reason. The answer's `revoked.refreshToken` is `true`
  only when the row is revoked too.
### Changed — the SDK token list returns token metadata with camelCase keys

- `GET /api/v1/users/me/sdk-tokens` returned each token's `metadata` exactly as stored. A token created by
  refresh-token rotation therefore carried `parent_token` and `rotated_from` next to `rotationCount`, while every
  other key in the response is camelCase. The list now returns every metadata key in camelCase, so those two are
  `parentToken` and `rotatedFrom`. Stored tokens are unchanged, and the dashboard does not read this field. A
  client that read the snake_case keys from this response reads the camelCase names instead.

### Fixed — a CLI login on a production stack is no longer throttled while it waits for approval

- `POST /api/v1/oauth/device/code` handed out a poll interval of 5 seconds, while every `/api/v1/oauth/device` route
  shares the strict rate limit of 10 requests per minute per address in production. A login sends the code request
  and 12 polls a minute, so a login not approved within about 45 seconds was answered
  `429 Rate limit exceeded. Please try again later.`, and so were the dashboard's verify and approve calls when the
  browser shared the CLI's address. The interval is now 10 seconds: the busiest minute of a login is the code
  request, six polls, and the verify and approve calls, which is nine requests. A CLI that follows the interval it
  is given now completes a login up to 10 seconds after approval instead of 5.

### Fixed — shutdown on SIGTERM is bounded and a failed shutdown no longer skips cleanup

- On SIGTERM or an interrupt, the server waited for every in-flight request with no time limit, so one stalled request
  held shutdown open until the orchestrator killed the process. When stopping the listener returned an error, the
  server exited at once with `Server forced to shutdown`, which skipped the drain of the fine-grained authorization
  engine's asynchronous intent checks and the deferred cleanup (background jobs, Redis, the database pool, the
  telemetry flush). Stopping the listener is now bounded at 10 seconds. A timeout or error is logged as
  `API server did not shut down cleanly within 10s: <error>`, and shutdown continues with the intent-check drain
  (bounded at 10 seconds) and the cleanup.

### Fixed — an authenticated request without an organization or user answers 401, not 500

- Agent, A2A, capability, capability request, lifecycle, MCP attestation, secrets, security policy, tag, admin and
  authorize routes read the caller's organization and user from the request context through shared helpers. When a
  caller passed authentication without one of them, the helper wrote a 401 and returned a plain error, and the
  server's error handler replaced that response with `500 Internal Server Error`. The helpers now return a 401 error
  that the error handler answers as `401` with `"message": "Organization ID not found in context"` (or
  `"User ID not found in context"`), in the same `{"error": true, "message": ..., "timestamp": ...}` shape as other
  API errors. A test now checks that every call site returns the helper's error unchanged.

### Changed — `/.well-known/aip` identifies the provider as `did:web:<provider-host>`

- The discovery document served `"providerDid": "did:aip:provider_opena2a"` on every deployment, a fixed value that
  was not the `did:web:<provider-host>` form the AIP specification (section 3.2) requires and that named the same
  provider for every self-hosted instance. `providerDid` is now `did:web:` followed by the host of `FRONTEND_URL`, the
  dashboard origin that also proxies `/.well-known/aip`: the hosted service serves `did:web:aim.opena2a.org`, and a
  default local install serves `did:web:localhost%3A3000` (a non-default port is percent-encoded, as `did:web`
  requires). The value comes from configuration, not from the request, so the dashboard and API hosts name the same
  provider and a `Host` header cannot change it. When `FRONTEND_URL` names no DNS host, the document omits
  `providerDid` and the backend logs why. Clients that compared `providerDid` to `did:aip:provider_opena2a` need to
  read the new value. Agent identifiers (`did:aip:aim_<uuid>`) and the DID resolver are unchanged; the provider's
  `/.well-known/did.json` document is not served yet.

### Fixed — `/health/ready` reports a configured Redis that failed at startup as unavailable

- When `REDIS_HOST` was set and the backend could not connect to Redis at startup, it continued without Redis and
  `GET /health/ready` reported `"redis": {"status": "notConfigured"}` with `degraded` false, which hid the outage. The
  readiness body now reads `notConfigured` only when `REDIS_HOST` is unset. A Redis named by `REDIS_HOST` whose
  startup connection failed reads `unavailable` with `degraded` true until the backend is restarted, as does one that
  stops answering after startup. Redis stays optional: the response is still 200 while the database is reachable.

### Changed — `ADMIN_PASSWORD` seeds the first administrator and no longer resets a changed password

- The backend wrote `ADMIN_PASSWORD` over the password of the administrator named by `ADMIN_EMAIL` on every start, so
  a password changed in the dashboard reverted at the next restart while `ADMIN_PASSWORD` stayed set (the quick
  start's `.env` sets it). `ADMIN_PASSWORD` is now a first-run seed. When the database has no administrator and no
  account with `ADMIN_EMAIL`, the backend creates that administrator in the `admin.opena2a.org` organization; the
  password must meet the platform's password rule and must be changed at first sign-in, as with
  `aim-bootstrap --default`. Once any administrator exists, the backend changes no account's password and logs that
  `ADMIN_PASSWORD` was not applied. Before this change the backend created no administrator from `ADMIN_PASSWORD`; it
  only overwrote an existing one's password. Setting `ADMIN_PASSWORD` no longer recovers a lost administrator
  password; use the password-reset flow (`POST /api/v1/public/forgot-password`) instead.
- `scripts/lint-no-secret-fallbacks.sh` checks `ADMIN_PASSWORD`: a compose file may leave it empty
  (`${ADMIN_PASSWORD:-}`) but may not fall back to a fixed password.

### Fixed — a trust-score policy is evaluated at the threshold it is seeded with

- The default `trust_score_low` policies were seeded with their threshold where the evaluator does not read it.
  'Critical Trust Score Block' carried `{"threshold": 50}` and 'Low Trust Score Alert' `{"threshold": 70}`; the
  evaluator reads `trust_threshold` on the trust score's 0-1 scale and, finding none, applied 0.3 to both, so editing
  the seeded value changed nothing. The bundled seed script and the policy backfill wrote `{"trust_threshold": 70.0}`,
  a percent compared to a 0-1 score, which every agent is below. The seeds now write `trust_threshold` as a 0-1 value,
  and migration 112 rewrites existing `trust_score_low` rows the same way: a numeric `threshold` moves to
  `trust_threshold`, and a value above 1 and at most 100 is divided by 100. After the migration, 'Critical Trust Score
  Block' blocks an evaluable agent below 0.50 (it blocked below 0.30) and 'Low Trust Score Alert' alerts below 0.70.
  The suspension applied when a recalculated score drops below 0.50 stays a platform rule, separate from policy rows.

### Fixed — an issued trust credential's content hash matches the ATX schema

- `POST /api/v1/agents/:id/atc` sent the Registry a `contentHash` of the form `sha256:<hex>`, and the Registry
  signs that field as given. The ATX v1.1 credential schema requires 64 lowercase hex characters with no prefix,
  so every credential issued this way failed schema validation in a conformance verifier. AIM now sends the bare
  hex digest. A credential issued before this change keeps its prefixed value until it is reissued or expires.

### Fixed — the backend image stamps the version and commit that `/health/ready` reports

- `GET /health/ready` reports `commit` and `version` from two values stamped into the server at build time. The image
  Dockerfile, `infrastructure/docker/Dockerfile.backend`, stamped a name the server does not declare and no commit, and
  the Go linker drops such a stamp without an error, so every image built from it reported `"commit": null` and
  `"version": null`. The build now stamps `version` from the `VERSION` build argument and `commit` from a new
  `GIT_COMMIT` build argument: `docker build --build-arg GIT_COMMIT=$(git rev-parse HEAD) ...`. A build that passes no
  40-character commit SHA still reports `"commit": null`.
- `sdk/typescript/tests/dockerfile-backend-build-stamp.test.ts` fails when the Dockerfile the image workflow builds
  stamps a name the server does not declare, or does not stamp both values from those two build arguments.

### Added — refused signed agent requests are counted and logged by reason

- A signed agent request (Ed25519, ML-DSA or hybrid) that the request-signature middleware refused got a 401, or a 400
  for an unsupported algorithm, and left nothing else behind. A server with a drifting clock, or an SDK release signing
  the wrong bytes, refused honest agents with no counter and no log line to show it. Each refusal now increments
  `aim_s1_refusals_total{reason, sdk}` on `/metrics` and is included in one `s1_refusals` log line per 64-second period
  in which any request was refused. The responses themselves are unchanged.
- `reason` is one of 14 values, one per kind of refusal. A timestamp outside the 30-second window is counted as
  `skew_past` or `skew_future`, so a fast server clock and a slow one read differently. `sdk` is `python`, `typescript`
  or `java` from the SDK's `User-Agent` prefix, and `other` for every other caller.
- A refusal happens before the caller is authenticated, so neither the labels nor the line carry an agent id, key,
  signature, signed bytes or the raw `User-Agent`.
- A test reads the two middleware source files and fails when a refusal is added that is not counted, and drives each
  of the 50 refusal branches once to check it moves exactly its own series. The reasons and how to read them are in
  `apps/backend/docs/OBSERVABILITY.md`.

### Changed — dashboard charts render through one chart layer

- The charts on the A2A, compliance, MCP supply-chain and security pages and the agent trust-score history now render
  through one set of components in `apps/web/components/charts`, which take every series, grid, axis and tooltip colour
  from `--chart-*` tokens declared in `globals.css` for the light and dark themes. The compliance page's trust-score
  trend and risk donut drop their fixed colours, so their grid, axes and tooltip now follow the dark theme.
- Each chart is announced to screen readers as one image whose label states what it measures and the values it draws,
  for example "Tasks by state: COMPLETED 4, FAILED 1". Count axes show whole numbers only.
- `tests/chart-standard.test.ts` fails the web suite when a file outside `components/charts` imports the charting
  library, or when chart code carries a hex, rgb/hsl or palette-class colour.

### Fixed — two dashboard tabs no longer sign each other out when the access token expires

- A login refresh token can be used once. When the access token expired with the dashboard open in
  two tabs, each tab posted the same stored refresh token to `POST /api/v1/auth/refresh`. The API
  accepted the first and refused the second as a reused token, which ends the sign-in, and the
  refused tab then cleared the session the first tab had just stored, so every tab went to the
  login page. Several requests refused together in one tab did the same.
- The dashboard now refreshes one at a time across all of its tabs (the Web Locks API). A tab that
  waited its turn reads the stored session again and uses the tokens another tab already obtained
  instead of posting its own. In a browser without Web Locks, which includes a dashboard served
  over plain HTTP from a host other than `localhost`, the refresh is one at a time within each tab
  only, so two tabs can still collide there.
- A refused or failed refresh clears the stored session only while it still holds the refresh token
  that was presented; a session another tab stored in the meantime is kept.

### Fixed — MCP servers registered before the upgrade get a calculated trust score

- In 1.0.0 an MCP server's `trustScore` was never calculated: it was set to 75.0 when the SDK registered or verified
  the server, and stayed 0.0 for a manually registered one. Migration 104 moves the field to the [0,1] scale by dividing
  by 100, which turns those values into `0.75` and `0.0` without calculating anything, so they read like calculated
  scores until each server is verified again.
- The first start after the upgrade now scores every MCP server with the 8-factor trust calculator, in the background,
  and records the `mcp_trust_rescored_after_scale_migration` key in `system_config` so later starts skip the pass. A
  server whose scoring fails keeps its stored value and is named in the server log. If the pass cannot finish, for
  example because the process stops part-way, the key is not written and the next start runs the pass again.

### Fixed — capability verifications are recorded as capability checks

- `POST /api/v1/sdk-api/verifications` recorded every verification event as `verification_type: identity`, because the
  type was guessed from the capability name (a substring match on `capability` or `permission`) and no real capability
  (`http:post`, `fs:read`) contains either word. A denied out-of-grant action therefore appeared on the activity timeline
  as "Verification: identity, Status: failed", which reads as a failed identity check. A request that names a
  capability is now recorded as `capability`; filters on `verification_type = 'capability'` see these events (#380).

### Security — MCP server and tag routes answer only for the caller's organization

- `POST /api/v1/mcp-servers/:id/verify-capability` and `GET /api/v1/mcp-servers/:id/connections`
  loaded the MCP server by id without checking the caller's organization. Both now answer
  `404 {"error":"not found"}` for a server outside it, the same as for a server that does not
  exist, and the capability check no longer records a verification event against another
  organization's server.
- `PUT /api/v1/tags/:id` answered a tag in another organization with a 500 naming the mismatch and
  a missing tag with a 404. Both now get the same 404.
- Two handlers that no route mounts, `MCPHandler.GetConnectedAgents` and
  `PublicMCPHandler.VerifyMCPAction`, check ownership the same way, so mounting either later does
  not expose another organization's servers.
- The four detection routes under `/api/v1/detection/agents/:id/` were reviewed and are unchanged:
  each service method checks `agents.id = $1 AND organization_id = $2` before reading or writing.
- The tenant-scoping lint no longer allowlists any handler as awaiting review, and a test fails if
  one is added back (#358).

### Security — a reused refresh token is recorded as the same client or a different one

- A refresh token presented again after it was rotated or logged out is still refused, and its
  sign-in still ends. Its `refresh_token_reuse` audit row and `SECURITY` log line now also say
  whether the presenter looks like the client that retired it: `clientMatch` is `sameClient` (for
  example two browser tabs or two SDK processes racing with one token), `differentClient`, or
  `unknown`.
- To compare, rotation and sign-out store a keyed, truncated hash of the token's id, the client's
  address (IPv4 exactly, IPv6 by its /64) and its user agent as the value of the token's denylist
  entry, for the token's remaining lifetime. The key is derived from `JWT_SECRET`; neither the
  address nor the user agent is stored in clear there. Signing out with a token that was already
  rotated keeps the mark of the client that rotated it, so a sign-out cannot make a later replay
  look like the same client.
- `unknown` means the comparison could not be made: an entry written before this version, a
  revocation store that cannot read values back, or a client address that does not parse.
- Both refusal records use the client address the rate limiter uses: the address a proxy named in
  `TRUSTED_PROXIES` reports, and otherwise the connecting address as before. A reported address
  that does not parse is not recorded; the connecting address is recorded instead.

### Security — the API no longer uses cookies for sign-in

- Sign-in no longer sets `access_token` or `refresh_token` cookies, and the API no longer accepts
  them in place of the `Authorization` header. This covers password sign-in, a sign-in that must
  change its password, and the local sign-in route. A sign-in form posted from another site
  therefore no longer leaves a session in the browser.
- Sign-out revokes the bearer and the refresh token sent in the JSON body (`refreshToken`); a
  refresh token held only in a cookie is not revoked. Clients that send `Authorization: Bearer` and
  put the refresh token in the request body are unaffected. Sign-out still clears the cookies that
  earlier versions set.
- Upgrade the dashboard with or before the API: every earlier dashboard opens pages only when the
  `access_token` cookie is present and sends visitors back to sign-in without it.

### Security — the API accepts a sign-in only from the Authorization header

- The API authenticates a request only from the `Authorization: Bearer` header on every route
  behind its session middleware; the `access_token` cookie no longer stands in for the header there.
  Dashboards already send the header and are unaffected; a client that sent only the cookie now gets
  401 and should send the header. The sign-in cookies themselves are removed in the entry above.

### Security — the SDK credential recovery route mints only for the account that is signed in

- `POST /api/v1/auth/sdk/recover` minted a fresh login pair for the owner of any revoked SDK token
  it was given, whoever was signed in: any account that could hold a session could turn another
  user's revoked SDK credential into a live session for that user. The route now mints only when
  the signed-in user and organisation are the token's owner; any other account is answered exactly
  as if the token did not exist. No shipped SDK reaches this route as written.

### Security — a revoked session cannot mint a new credential with its still-valid access token

- After a session is revoked (a reused refresh token, or logout), the browser's or command line's
  access token stays valid until it expires (`JWT_ACCESS_TTL`, 2 h by default). In that window it
  could download an SDK (a 90-day credential), approve a device sign-in for a command line, or
  recover an SDK credential. `GET /api/v1/sdk/download`, `POST /api/v1/oauth/device/approve` and
  `POST /api/v1/auth/sdk/recover` now read the session's revocation directly (no cache) before
  minting and refuse a revoked session with the same 401 the refresh route answers; a confirmed
  refusal is recorded in the organization's audit log as `credential_mint_refused` (identifiers
  only) and as a `SECURITY` line. Login access tokens carry their session's `sid` for this; a
  token minted before this change carries none and is not checked. No client change is needed.
  Other authenticated routes keep serving a revoked session's access token until it expires; that
  window is `JWT_ACCESS_TTL`, and closing it on every request is a separate, measured change.

### Security — two presentations of one refresh token in the same instant no longer both rotate

- Retiring a login refresh token on `POST /api/v1/auth/refresh` is now a set-if-absent write when
  the revocation store supports it (Redis does, with no configuration): two presentations of one
  token within a request's duration both used to read "not revoked" and both rotate, leaving two
  live chains in one session that only a later reuse would end. Now the presentation that loses
  the write is refused as a reuse (the session is revoked and the reuse recorded) and receives no
  tokens. A store without set-if-absent keeps the previous behaviour; a failed write still returns
  the presented token unchanged with `rotated: false`. The 401 answer, `rotated` and the SDK-download
  path are unchanged.

### Fixed — logouts are recorded in the audit log

- `POST /api/v1/auth/logout` wrote its audit row from a request value that the public auth route
  never set, so no logout was ever recorded. The row now comes from the presented token's own
  claims (the bearer access token, or the refresh token a command line sends): one `logout` row per
  logout with a valid token, carrying the user, organisation, client address, user agent, the
  token's id and its session; garbage tokens record nothing.

### Fixed — the dashboard keeps its sign-in in one place

- The dashboard kept two copies of a sign-in: a cookie that decided which pages opened, and a
  stored token that API requests used. The two could belong to different accounts. Pages now
  open or redirect from the same stored sign-in that API requests use, and the dashboard no
  longer sends cookies to the API.
- Signing out sends the current refresh token, so the whole session ends even after a refresh.
- A tab showing one account reloads when another tab signs in as a different account.
- The MCP server details view read the sign-in from a key nothing wrote, so attestations and
  recent audit entries never loaded. It now uses the signed-in session.
- The API explorer's Log out button now signs you out of the dashboard.

### Fixed — a new dashboard session never keeps the previous account's refresh token

- The dashboard's API client stored a refresh token only when the caller passed one, so a new
  session started with an access token alone kept the refresh token of whoever was signed in
  before, and the next silent refresh ran as that account. A new session without a refresh token
  now clears the stored one; a token refresh without one keeps the current token, as before.

### Removed — an unused password-reset handler that would have put the email address in the reset link

- An unused password-reset handler that was never routed and would have put the account's email
  address in the reset link. The live reset link carries only the token.

### Fixed — the reset-mail failure log records the account id, not the recipient's address

- The reset-mail failure log records the account id instead of the recipient's address, and the mail
  provider's error is redacted before it is written, since an SMTP reply often echoes the recipient.

### Security — a sign-in ends a fixed time after it began, however often it is refreshed

- A login refresh token now records when its sign-in happened (the registered `auth_time` claim),
  and every refresh copies it unchanged. `POST /api/v1/auth/refresh` refuses a sign-in older than
  `JWT_SESSION_MAX_AGE` (8 hours by default) with the existing
  `401 {"error":"Invalid or expired refresh token"}`, and the dashboard asks the user to sign in
  again. Before this, a refresh token that kept being refreshed never reached an end, so a stolen
  one could outlive the sign-in it came from.
- The limit holds with or without a revocation store. Refresh tokens issued before this release
  count from the time they were issued, so every existing sign-in ends within
  `JWT_SESSION_MAX_AGE` of the upgrade.
- SDK-download tokens keep their 90-day lifetime and rotation. Access tokens are unchanged: the
  last one issued before the limit stays valid until its own expiry (`JWT_ACCESS_TTL`, 2 hours by
  default).
- `JWT_SESSION_MAX_AGE` takes a duration such as `8h` or `24h`. A missing, unreadable or
  non-positive value uses 8 hours and logs a warning; the value in effect is logged at startup.
- `apps/backend/.env.example` and `docs/DEPLOYMENT.md` listed `JWT_EXPIRY` and `JWT_EXPIRATION`,
  which nothing reads; they now list `JWT_ACCESS_TTL`, `JWT_REFRESH_TTL` and
  `JWT_SESSION_MAX_AGE`.

### Security — a reused refresh token ends the sign-in, and logout ends the whole session

- Presenting a login refresh token that was already rotated out ends that sign-in: every
  refresh token issued from it by rotation is refused from then on (RFC 9700 section 4.14.2,
  refresh token reuse detection), and the event is recorded in the organization's audit log
  (`refresh_token_reuse` for the presentation that ended the session, `refresh_session_revoked`
  for each later member refused) and as a `SECURITY` line in the backend log, with identifiers
  only. A rotated-out token used to be refused on its own while the chain that grew from it
  stayed valid, so whoever rotated first kept a working session. A legitimate client that
  replays its own old token (two tabs, two processes on one credentials file) now signs in
  again; a refusal carries the same answer as before.
- Logging out ends the whole session. A browser's `refresh_token` cookie is set once at login,
  so after any refresh the dashboard's logout (including the idle and eight-hour automatic
  logouts) revoked a token that was already retired and left the refreshed token valid until it
  expired. `POST /api/v1/auth/logout` now ends the sign-in the presented refresh token belongs
  to, and reports `revoked.refreshToken: true` only when that write succeeded.
- Login refresh tokens carry the registered `sid` claim naming their sign-in (the login token's
  own id, copied on every rotation). Tokens are opaque to clients and no client changes; a token
  minted before this change is the root of its own session and joins on its first rotation.
  `POST /api/v1/auth/refresh` reads the revocation store directly (no in-process cache) so a
  replay is never served from a stale entry; a store that does not answer records nothing.

### Fixed — a rotated-out login refresh token is refused on its next use

- `POST /api/v1/auth/refresh` retires the presented login-issued refresh token (its id is written
  to the revocation denylist for its remaining lifetime) before answering with the new one, so a
  refresh token that has been rotated is refused with 401 on its next use; it used to stay usable
  until its own expiry although the code claimed otherwise. A refresh token is rotated only when
  the old one was actually retired: without a revocation store (no Redis), or when the store
  refuses the write, the answer carries a fresh access token and the presented refresh token
  unchanged, and the new `rotated` field reports it, so a login session on such a stack ends at
  `JWT_REFRESH_TTL`, or `JWT_SESSION_MAX_AGE` after sign-in if that is sooner. SDK-download tokens keep their row-based rotation. A refresh answered after a
  lost response now requires signing in again, which is the cost of single-use refresh tokens.

### Fixed — logout revokes a refresh token sent in the body and reports what it revoked

- `POST /api/v1/auth/logout` also reads the refresh token from a JSON body (`{"refreshToken": ...}`,
  the body wins over the `refresh_token` cookie), so a client that holds the pair outside a browser
  can revoke it, and answers a `revoked` report (`accessToken`, `refreshToken`) that is `true` only
  when the token's id was written to the denylist; a server without revocation configured reports
  `false` instead of a silent no-op. The route, the cookie channel and the message are unchanged.

### Fixed — an API-call analytics row keeps its own request's endpoint and user agent

`AnalyticsTracking` read the method, path, user agent and client address from the request context and wrote the `api_calls` row from a goroutine after the handler returned. Those strings are views onto buffers Fiber reuses for the next request, so a row could store another request's endpoint and user agent: under sequential load in a test, every row read as the last request's path. The middleware now copies each string it keeps. The rows are the record used to answer whether an unauthenticated route was ever called, so a wrong endpoint is a wrong answer (opena2a-org/aim-cloud#24).

### Changed — the A2A composite no longer scores an agent it has no data for, and its PUT refuses

The A2A trust score started every agent at 0.5 and credited 0.1 for a missing response
time and 0.1 for account age, so an agent with no task at all read 0.7 while an agent
with a perfect task record, no peers and no response figures read 0.6. An agent with no
completed or failed task is now UNSCORED: `a2aTrustScore` is absent and `scoreStatus`
reads `unscored` on `GET /api/v1/a2a/agents/:id/trust-score`, `GET /api/v1/a2a/trust/:id`
and the compute route; missing response data earns nothing. Routing by intent no longer
defaults a missing score to 0.5: an unscored agent never passes a positive threshold,
ranks last, and is listed with `trustScore: null` only when no threshold is asked for.
`PUT /api/v1/a2a/trust/:id`, which bound a body and discarded it, now answers 405 with the
reason: the score is measured, not asserted. No migration: the column was already nullable.

### Security

- `POST /api/v1/auth/refresh` and `POST /api/v1/auth/sdk/recover` minted access tokens
  from the refresh token's own claims instead of the user record: a reduced role
  persisted on SDK token chains for up to 90 days, deactivated and soft-deleted
  accounts could keep refreshing until their refresh token expired, and login-issued
  sessions lost their role on first refresh (fail-closed 403). Both endpoints now
  resolve role and email from the user record at every mint and return 401 for
  accounts that are not active or pending. (#432, GHSA-3hvp-fmvj-6gwx)
- `GET /api/v1/a2a/cards` returned every organization's agent cards to any
  authenticated caller: `cardUrl`, the full `cardData` agent-card document,
  `agentId` and `attestationSignature`. The only predicate was `is_valid = TRUE`,
  and the route carries no role middleware. Its pagination was weaker than the
  endpoints fixed alongside it — the limit was parsed with no bound at all, not
  even a positive check, so `?limit=-1` reached PostgreSQL as a negative `LIMIT`
  and surfaced as a 500, and a large limit was unbounded. The query is now scoped
  to the caller's organization by joining `agents` (`a2a_agent_cards` has no
  organization column of its own) and the limit is capped.
  **Cross-organization discovery is unaffected**: the public per-agent card is
  still `/.well-known/agent.json`, and cross-organization search still lives on
  the `/discovery/*` routes. This endpoint is an SDK-compatibility list whose
  sibling routes all operate on a single card the caller owns.

- `GET /api/v1/a2a/consents` and `GET /api/v1/a2a/trust-scores` returned every
  organization's rows to any authenticated caller. Neither query carried an
  `organization_id` predicate and neither route carried a role middleware, unlike
  its neighbours on the same group, so one tenant's token read every tenant's
  consent records — `userId`, `purpose`, `dataTypes`, both agent IDs, `userAgent`
  — and every tenant's agent IDs and behavioural metrics. `limit` was
  caller-controlled with no upper bound, so a single request could page the whole
  table. Both queries are now scoped to the caller's organization; the trust-score
  query reaches the boundary by joining `agents`, since `a2a_trust_scores` has no
  organization column of its own. Present since the A2A routes were introduced.
- The row COUNT on both endpoints is scoped as well, not just the rows. An
  unscoped total tells a caller how much data every other tenant holds without
  returning a single row, so scoping the rows alone would have left a cardinality
  disclosure behind.
- `GET /api/v1/a2a/consent/check` was a cross-tenant consent oracle. The query
  matched on `user_id`, `grantor_agent_id`, `recipient_agent_id` and `scope` and
  carried no organization predicate, so its answer described any row in the
  table whatever organization owned it. `user_id` is what made that reachable:
  an unvalidated string with no ownership relation to anything, filtered on
  directly, so an authenticated caller who guessed one learned whether a given
  user had granted a given consent in someone else's organization. The query is
  now scoped to the caller's organization.
- `POST /api/v1/a2a/consent` accepted a consent record naming an agent the
  caller does not own as the grantor. Both agent IDs arrive in the request body
  and the only constraint on them was a foreign key to `agents(id)`, which
  requires a real agent rather than one of yours, while `organization_id` was
  stamped from the caller's own org. The stored owner and the grantor's owner
  could therefore diverge on every such write, which is what made an
  organization predicate on the read side worth having in the first place. The
  grantor is now verified against the caller's organization, and a grantor that
  does not exist is refused with the same error as one that belongs to someone
  else, so the response does not confirm which.
  **Cross-organization consent is unchanged**: the recipient may still be
  another organization's agent, which is the entire purpose of the feature.
- `GET /api/v1/revocations` returned an empty revocation list to every caller,
  always. `GetRevocationList` asked the repository for `List(0, 0)`, which reaches
  PostgreSQL as `LIMIT 0` — zero rows, not "unbounded". The route is mounted and
  unauthenticated here, so a verifier polling it was told that nothing had ever
  been revoked, and a caching client treats that as a successful fetch and holds
  it as fresh. No 4xx, no 5xx, no log line: the control reported healthy while
  enforcing nothing.
- The revocation list no longer exposes `name`. It emitted every revoked agent's
  organization-internal name across all organizations on an unauthenticated
  endpoint. This was inert only because the list was always empty, so fixing the
  limit without removing the field would have shipped the disclosure in the same
  deploy. `revokedAt` is also gone: it was read from `agents.updated_at`, which
  any later write to the row rewrites, and there is no `revoked_at` column to
  source it from.
- Java SDK: `CrlCache` accepted a decoded `Crl(entries=null)` as a valid list and
  cached it as fresh, and `LocalAtxVerifier` read null entries as "nothing is
  revoked". A CRL feed whose JSON carries its list under a different key therefore
  produced a silent fail-open — every revoked credential verifying, with no error
  and a healthy-looking `status()`. Both now reject a malformed list; a genuinely
  empty one still verifies normally.
- A NULL in a nullable column failed the entire row rather than the field on most
  agent read paths (`description` on five of six, `rotation_count` and
  `trust_score` more widely). `GetByID` is the one that matters: the agent auth
  middlewares call it, and a failed read there is reported as "Agent not found",
  which is indistinguishable from a revocation denial. An agent created without a
  description could not authenticate and the operator was told it did not exist.
- Dependency bumps. `package-lock.json`: `next` 16.3.5 (CVE-2026-75604, GHSA-2xp9-vwfh-vxw4,
  CVE-2026-64641, CVE-2026-64642, CVE-2026-64645, CVE-2026-64649), `sharp` 0.35.4
  (GHSA-f88m-g3jw-g9cj, GHSA-rgj7-g3m4-5g8c), `postcss` 8.5.18 or later (CVE-2026-45623,
  CVE-2026-73646), `nanoid` 3.3.19 (CVE-2026-67213, CVE-2026-67214). `apps/backend/go.mod`:
  `golang.org/x/crypto` v0.55.0 (CVE-2026-56854), `google.golang.org/grpc` v1.83.2
  (CVE-2026-84304, CVE-2026-84445, GHSA-hrxh-6v49-42gf), `github.com/go-jose/go-jose/v4` v4.1.4
  (CVE-2026-34986), `golang.org/x/text` v0.41.0 (CVE-2026-56852). (#491)
- `apps/web/next.config.js` closes the self-hosted image optimizer (`images.unoptimized` plus a
  `localPatterns` entry matching no path), so `/_next/image` returns 404. No page uses
  `next/image`, so nothing rendered changes. GHSA-2xp9-vwfh-vxw4 needs a same-origin image
  source, which a default deployment does not have; if a proxy or CDN in front of this app
  serves user-supplied files on that origin, check your access logs for `/_next/image`.
- Java SDK: `jackson.version` moves from 2.18.8 to 2.18.11 in both Java manifests
  (`sdk/java/pom.xml` and `examples/a2a-multi-agent-demo/java/pom.xml`). 2.18.11 is the lowest
  2.18.x release outside the published ranges of four advisories: GHSA-wv8q-qhhj-9h54
  (CVE-2026-91776) and GHSA-cxp5-3px4-pw24 (CVE-2026-91777) in `jackson-databind`, and
  GHSA-7hhh-6rmp-j9qf (CVE-2026-89425) and GHSA-p6pp-m3f8-5c89 (CVE-2026-89407) in
  `jackson-core`. It is also outside the range of GHSA-q4xh-88c3-wmh7 (CVE-2026-68497), which
  contains 2.18.8. (#560)
- Earlier dependency bumps in this cycle. Java SDK (`sdk/java/pom.xml`): jackson 2.16.1 to 2.18.8
  (CVE-2026-54512, CVE-2026-54513) (#340); bouncycastle 1.79 to 1.84 (CVE-2026-5598) (#294), then
  to 1.85 (CVE-2026-8763, CVE-2026-13506) (#500). The Java demo under
  `examples/a2a-multi-agent-demo` takes the same jackson and bouncycastle lines as the SDK, and
  the Python examples install from exact locks (#511). `apps/backend/go.mod`:
  `golang.org/x/crypto` 0.52.0 and `golang.org/x/net` 0.55.0 (#340), then `golang.org/x/net`
  v0.56.0 (CVE-2026-46600, GHSA-gg3m-vvp2-p2c5) (#405).
- Both dashboard Dockerfiles pin their base images, and the runtime stage is rooted on alpine
  (#496). The runtime base is re-pinned to alpine 3.21.8, which carries openssl 3.3.7-r1
  (CVE-2026-45447 in 3.3.7-r0) (#509).
- Java SDK: `LocalAtxVerifier` accepted a credential whose declared ML-DSA-65 signature was
  forged as long as its Ed25519 signature was intact; the post-quantum entry was recorded as
  present and never checked. Every declared signature entry is now verified (#550).
- Java SDK: `LocalAtxVerifier` took the authorities allowed to sign a v1.1 credential from the
  credential's own signed `issuerChain` without requiring them to be trusted issuers, so a
  signer whose key was configured under a DID-URL key id could name itself in the chain and
  sign for a trusted issuer. The chain's contribution is now intersected with the trust
  anchors' trusted issuers (#462).
- Java SDK: `LocalAtxVerifier.verify(byte[])` and `verify(String)` strict-parse the credential
  and reject a duplicate object member at any depth, including case variants a lenient parser
  folds together, before any field is interpreted (#337).
- `POST /api/v1/verification-events` accepted a verification outcome reported by the caller. It
  now refuses every authenticated member with 403 and the code
  `verificationEventWriteNotAccepted`; the server's own verification paths still record events,
  and the read routes and DELETE are unchanged. The service behind it also refuses an agent of
  another organization before reading it, with the same error as an unknown agent
  (#556, #559).
- `POST /api/v1/sdk-api/verifications/:id/result` and `.../execution-status` were registered
  outside the SDK API group with a rate limiter as their only middleware, so both served
  unauthenticated writes, and `/result` set the verification's decision: an agent holding its
  pending verification id could approve its own action. Both routes now sit behind the SDK API
  group's agent authentication with an agent ownership check, and `/result` no longer writes
  the decision (#372).
- `POST /api/v1/oauth/token` had no agent-status check, so a revoked agent that still held its private
  key could keep minting service tokens. Status is now enforced at issuance and again when a
  service token is used. The endpoint also read only the `sub` claim of the assertion; it now
  requires `aud` and `exp` (RFC 7523 section 3) with a bounded lifetime, checked after the
  signature (#363).
- Secrets namespaces: creating a namespace took the agent id from the request body, and listing
  namespaces and reading the audit log took it from a query parameter, none with an owner
  check. All now scope through the agent's organization. The agent bulk-status lookup filters
  by organization in SQL, and `GET /api/v1/a2a/tasks`, which had no organization predicate,
  returns a task only when one of its two agents belongs to the caller's organization. The
  tenant-scoping lint now also watches query parameters, and API-call analytics record calls
  to unauthenticated routes (#363).
- A revoked or suspended agent that still held its key material authenticated successfully on
  the Ed25519 signature, post-quantum signature and API-key paths, because those middlewares
  never read the agent's status. All of them now check it against one allow-list: `verified`
  and `pending` pass, every other value is denied (#353).
- OAuth service tokens carried `role: "service"`, which the member gate admitted, so a token
  obtained by one agent reached every member-gated route, including
  `GET /api/v1/agents/:id/credentials`. Service tokens now carry their own issuer and no role,
  the session middleware refuses them, and the member gates are allow-lists. `PUT /api/v1/agents/:id`
  accepts a member or a service principal acting on its own agent (#347).
- `GET /api/v1/agents/:id/credentials` and `GET /api/v1/agents/:id/sdk`, which return an
  agent's private key material, were reachable with an organization-scoped API key. Both now
  require a signed-in member; viewers get 403 (#343).
- `POST /api/v1/agents/:id/mcp-servers/detect` read a file path from the request body and
  opened it without validation, which let a signed-in member read any file the server could.
  The path must now be one of the standard desktop-client config locations (#326).
- Session tokens carry a type. An SDK refresh token (90 days) or a login refresh token can no
  longer be presented as a bearer access token, and the refresh route refuses an access token
  (#305, #308). Tokens minted before the type claim existed are no longer accepted on either
  path (#309, #492).
- The public `did:aip` resolver answers a pending (registered, not yet verified) agent exactly
  as it answers an unknown DID, so the response no longer tells a caller which ids are
  registered, and `/api/v1/did/*` is rate limited. The rate limiter's client address is the
  rightmost `X-Forwarded-For` hop that is not in `TRUSTED_PROXIES`, instead of the leftmost
  entry, which the client controls (#490).
- The device grant's token endpoint minted a session for an account that was suspended,
  deactivated or deleted between approval and the command line's next poll. The poll now
  answers `access_denied`, as the refresh route does (#506).
- Dashboard: after sign-in the browser follows only a same-origin return path. A `returnUrl`
  containing a tab, line feed or carriage return, a double-encoded value, or dot segments
  resolving to `//host` passed the previous check and left the origin (#526).
- Dashboard: the landing page no longer adopts a `token=` query parameter as the session. No
  shipped flow issued such a link; in a signed-in browser it replaced the API session with
  whatever bearer was on the URL (#505).
- The FGA context check no longer fails open. An unparseable `context_rules` document used to
  be allowed with a warning, and a missing risk summary skipped the rules. The decision now
  follows `contextRules.onUnavailable` (`deny` or `allow`), and an absent key, an unknown value
  or an unparseable document means `deny`. The result reports it under `contextCheck`
  (#454).
- `GET /metrics` served the full Prometheus scrape without authentication. Setting
  `METRICS_AUTH_TOKEN` makes it require `Authorization: Bearer <token>`; with the variable
  unset the endpoint stays open, as before, and the server logs a warning at startup (#349).

### Added

- Server-side token revocation. Logout revokes the presented access and refresh tokens in a
  Redis-backed denylist for their remaining lifetime; the session middleware and
  `POST /api/v1/auth/refresh` refuse a revoked token. A store outage refuses by default
  (`AUTH_REVOCATION_FAIL_OPEN=true` prefers availability); without Redis, revocation is off
  and tokens end at their expiry. `CORS_ALLOW_VERCEL_PREVIEWS=true` opts in to preview
  origins (#307).
- Dashboard sessions end after 30 minutes without activity, with a two-minute warning, and
  eight hours after sign-in. `GET /api/v1/auth/me` returns `organizationName`, and the refresh
  route reports the real access-token lifetime (#305). A sign-in over a stale stored session
  starts a new eight-hour window instead of being signed out at once (#369).
- `aim-sdk login` completes through the OAuth 2.0 device grant (RFC 8628) the backend serves,
  and the dashboard gains the `/device` consent page where a signed-in user approves the code
  (#504).
- Agents accept an optional `declaredPurpose` (category, task scopes, capability
  justification, autonomy, data scopes, egress scopes), validated against a closed core
  vocabulary plus organization-namespaced values. Migration 100 (#289).
- Machine API keys can register agents: `POST /api/v1/agents` admits an organization-scoped
  API key or a signed-in member. Every other member-gated route still requires a member, and
  registering with an API key does not mint a further API key (#341).
- Honeytoken capabilities: an operator can mark a granted capability as a decoy. A
  verification that matches it raises a high-severity alert and an audit event and leaves the
  authorization decision unchanged. Migration 102 (#316).
- MCP manifest drift detection: a baseline of each MCP server's tool manifest, and a drift
  record with a severity when tools are added, removed or changed, wired into the capability
  and attestation flows (#274). The recompute runs in one transaction under a per-server lock
  and raises an operator alert on drift (#313).
- Agent Trust Credential issuance: AIM computes the agent's trust score and delegates signing
  and transparency-log recording to the Registry, authenticated with `REGISTRY_ATC_TOKEN`; it
  fails closed on a missing token or a refused request (#312).
- An SDK endpoint for an agent's self-reported isolation posture, scored server-side, and the
  isolation factor on the dashboard (#311). Its route and its scoring were corrected later in
  this cycle; see the Changed and Fixed entries below.
- `POST /api/v1/trust-score/agents/:id/feedback` records a 1 to 5 rating for an agent in the
  caller's organization, audit-logged, and feeds the user-feedback trust factor.
- FGA intent check: the counters `fga.intent_checks` and `fga.intent_skipped` (#322); the
  check distinguishes a classified answer, an abstention and an operational failure, and a
  non-2xx classifier response counts as `fail_open` rather than an abstention (#323). The
  `fga.authorize` span carries `gen_ai.agent.*` authorization attributes (#324).
- Registration accepts three optional profile questions (role, primary use case, referral
  source), and `GET /api/v1/admin/registration-requests`, which the admin registrations page
  already called, exists (#291).
- Agent type `demo`, used by the `aim-sdk demo` command: shown as a demo in every list and
  counted against the plan quota (#431).
- Java SDK: local ATX credential verification (`org.opena2a.aim.atx`), including the binding
  of a DID-URL key to its issuer (#300), and a `CrlCache` that refreshes the revocation list
  in the background so verification makes no network call (#318).

### Changed

- **Trust factor 9 (execution isolation) no longer takes a self-report at face
  value.** The factor is fed by an agent's own claim about its sandbox, network,
  filesystem and process isolation, and it carries 10% of the composite. An agent
  that typed `firecracker + airgap + readonly + full` scored 1.0 on it — roughly
  +0.07 of trust over the no-attestation baseline — for the cost of four strings,
  and the claim counted forever after being made once. Two gates now apply when
  the factor is read:
  - An **unverified** attestation scores `min(posture, 0.65)`. The ceiling is
    derived, not chosen: it is `ScoreIsolation(docker, namespace, readonly,
    seccomp)`, the commodity-container tier, so retuning the posture weights moves
    it with the tier it names. A maximal claim now earns exactly what an honest
    hardened container earns. It is a `min()`, so an agent that truthfully reports
    no isolation keeps its honest `0.0` rather than being lifted to the ceiling.
  - A **90-day expiry** on `reported_at`, uniform for verified and unverified
    rows. A stale attestation scores the `0.3` baseline and is *not* added to the
    excluded-factor set: staleness is a scoring decision, not missing data, and
    renormalizing the weight away would return exactly what the agent lost by
    letting the attestation rot. Re-attesting restores the score.

  Both gates are read-side. The stored row keeps the honest posture score — the
  table records what was claimed, and the scorer decides what the claim is worth.
  Migration 108 adds `verified` / `verified_by` / `verified_at` to
  `isolation_attestations`, and **nothing writes `verified = true`**: the ingest
  path hard-sets false, the `INSERT` writes the literal `FALSE`, and no endpoint
  can reach the column, so `0.65` is the effective maximum for every agent today.
  `verified` is bound to the row rather than the agent, so a re-attestation
  supersedes its predecessor and starts unverified — verification never carries
  forward across a redeploy. Independent verification itself is a follow-up
  (roadmap `aim-isolation-verification` Phase 2); when it lands, TEE attestation
  and orchestrator/host metadata may set the column, an HMA static scan may not,
  and the SDK never.

  Unchanged and still broken at the time of that change: both shipped SDKs POST
  `/api/v1/sdk-api/agents/<id>/isolation-attestation` while the backend registered
  `/agents/:id/isolation`, so SDK attestations 404'd and the table was near-empty.
  That route fix was tracked separately and deliberately not folded in here; it
  landed afterwards, in the Fixed entry below.
- `AgentRepository.List` now returns an error for a non-positive limit instead of
  reinterpreting it. Returning everything would have traded a silent-empty bug for
  a silent-unbounded one; an error is the only outcome that fails visibly. Callers
  pass an explicit page size. Adds `ListRevokedIDs`, which filters in SQL so the
  unauthenticated revocation endpoint no longer reads every agent row to emit a
  subset.
- `AgentService.EnforceKeyExpiry` returns `ErrKeyExpiryEnforcementUnavailable`
  rather than reporting success for work it cannot do. It is unimplementable as
  written — `List` does not select `key_expires_at`, and suspending via `Update`
  would clear the agent's key material — and it has no caller. See #359. A later change in
  this cycle replaced it with a working implementation; see the key-expiry entry below.
- The tenant-scoping lint fails when an allowlist key names a method that does not
  exist. Fourteen of thirty-two entries resolved to nothing, presenting as reviewed
  exemptions while covering nothing. All fourteen were removed and the lint still
  passes, which is the evidence that none of the real handlers needed exempting.
  Remaining `needs review` entries are tracked in #358.
- `AgentService.EnforceKeyExpiry` suspends agents whose key has expired and whose grace window,
  if any, has closed, in one statement that writes only the status and leaves key material,
  capability grants and the rotation count untouched. It returns the number suspended.
  Nothing calls it yet (#543).
- The agent trust score is composed from measured factors only. A factor whose data source is
  not wired, failed or empty is excluded and its weight redistributed, instead of contributing
  a neutral 0.5 at full weight; a fresh calculation lists them as `excludedFactors` (#330),
  and the stored score keeps `execution_isolation` and the excluded set (migration 103, #332).
  The compliance and user-feedback factors read real data, and the score breakdown reports
  nine factors whose weights sum to 1.0 instead of eight summing to 0.90.
- MCP server trust scores are calculated. Every server used to carry the literal 75.0 or 0.0
  on a column that mixed three scales, so the minimum-trust policy could not refuse an
  SDK-registered server. The score is now computed from the server's inputs on one 0 to 1
  scale; migration 104 rescales existing rows and adds a range constraint (#350).
- The admin security-policy page states that MCP policies are not enforced: nothing evaluates
  them. Migration 105 disables the six seeded MCP policies and leaves an operator's own
  policies as set; the blocking banner, the stat tiles and the confirmation dialog no longer
  count or describe MCP policies as enforcement (#356).
- `GET /health/ready` answers a fixed body (`ready`, `service`, `commit`, `version`,
  `checkedAt`, `degraded`, `dependencies`) with `Cache-Control: no-store` on 200 and 503. The
  database check is bounded at 2 seconds, an unavailable optional Redis reports
  `degraded: true`, and driver error text never reaches the body. `GET /health` is unchanged
  (#455).
- `GET /api/v1/agents` accepts `limit` and `offset` and loads capabilities and tags for the
  page in one query each instead of two queries per agent; without the parameters the response
  is unchanged (#295). The MCP server and pending-verification lists load the same way (#297),
  and the dashboard's A2A page loads progressively (#298).
- Dashboard: a new visual design with light and dark themes that follow the system preference
  until the user picks one (#419, #427); an overview with Developer, Security and Executive
  views (#420); the sign-in, registration, agent, SDK, developer, security, MCP and A2A pages
  moved onto it (#421, #422, #423, #424); navigation reduced to seven top-level entries with
  tabs inside each (#430); a live first-check-in listener and whole-number metrics (#428); an
  app icon (#429); opaque menus and mobile navigation drawer (#433). No route changed its
  path.

### Removed

- The community-intelligence push. A six-hourly job posted anonymized trust-factor
  distributions of opted-in organizations to a Registry route that was never registered, so
  every push failed silently. The job, its opt-in routes (enable, disable, status,
  benchmarks) and the admin push trigger are gone; the SDKs still accept the registration
  flag and ignore it (#498).

### Fixed

- A fresh agent's first calls while it was pending were recorded as failed verifications, so
  after verification its trust score was composed from refusals, the low-trust policy blocked
  the action it had been granted, and a recalculation suspended it. A score with no
  successful verification in the calculator's 30-day window is now not evaluated by the
  low-trust policy and does not suspend the agent (#502).
- On a self-hosted install with no `AIM_PLATFORM_ADMINS` allowlist and no active
  administrator, a sign-up or access request created a pending request nobody could approve.
  Both are now refused with 503, code `noAdministrators` and a message naming the variable. An
  allowlisted address is told its account is approved and is sent to sign-in (#425); the
  refusal log reports truthfully whether the address was listed (#426).
- MCP drift alerts lower the trust score of agents connected to the drifted server. The drift
  factor looked alerts up by agent id while MCP drift alerts are keyed by server id, so they
  never matched (#321).
- Updating an MCP server no longer writes its cached trust score. Patching an unrelated field
  could write back a stale score and insert a history row describing a recalculation that
  never ran (#354).
- Migration 111 gives the `fga_policies` `onUnavailable` check constraint one name on every
  deployment; a deployment that applied an earlier variant of migration 110 carried a
  different name (#489).

- **Every SDK isolation attestation was 404ing.** All three shipped clients POST
  `/api/v1/sdk-api/agents/{id}/isolation-attestation` (`AIMClient.ts:729`,
  `client.py:2427`, `AIMClient.java:1698`); the backend registered only
  `/agents/:id/isolation`, so no agent's self-reported isolation posture ever
  reached `isolation_attestations` and trust factor 9 sat at its `0.3` baseline
  for every agent in every deployment. Per the recorded architecture decision of 2026-08-29 the
  SDK/spec path is canonical: it is now registered on the existing
  `SubmitIsolationAttestation` handler, and `POST /api/v1/sdk-api/agents/:id/isolation`
  stays registered as a deprecated alias to the same handler (Binding Decision 6
  forbids removing a published path). No SDK changed.

  The suite could not see the outage because the integration tests registered the
  handler at a path they chose themselves, so they agreed with the server about a
  path no client used. SDK-API route registration now lives in one table
  (`apps/backend/cmd/server/sdk_api_routes.go`) that `main.go` and the tests both
  mount through `registerSDKAPIRoutes`: tests may hand-mount handlers, but they
  take paths from the table the server registers, and a parity test reads the
  paths back out of the SDK sources.

- **Timestamp columns no longer depend on the writer's time zone.** Every
  `TIMESTAMP` (without time zone) column is now `TIMESTAMPTZ` (migration 106):
  42 application columns, plus `schema_migrations.applied_at` on deployments
  whose database was bootstrapped by the server rather than the migrate command.
  A naked `TIMESTAMP` drops the UTC offset on write and is read back labelled
  UTC, so the stored instant shifted by the writing process's offset. The
  one case that changes an access decision is `api_keys.expires_at`: a key
  written east of UTC stayed valid past its stated expiry by exactly that offset
  (measured: +9h at `Asia/Tokyo`, +2h at `Europe/Berlin`). West of UTC keys
  expired early.

  The other converted columns are correctness and audit fixes, not access-control
  fixes, and it is worth being exact about which is which:
  `audit_logs.timestamp` recorded the time of an audited action in the writer's
  zone, which is an evidentiary problem; `agent_capabilities.revoked_at` is
  enforced only as `revoked_at IS NULL`, which is zone-independent; and
  `agents.pqc_key_expires_at` reaches no enforcement path at all — the expiry
  `EnforceKeyExpiry` reads is `key_expires_at`, which was already `TIMESTAMPTZ`.

  A second path needed no Go code at all: the DSN sets no `TimeZone`, so
  `DEFAULT NOW()` and migration 092's `verified_at` trigger were cast under
  whatever zone the PostgreSQL *session* had. Converting the column type is what
  closes both paths — the columns are now correct regardless of either zone.

  **Who was affected:** deployments running at a non-UTC offset. The published
  container image sets no `TZ` and has no `/etc/localtime`, so it runs at UTC and
  was not affected. Self-hosters running the backend natively, setting `TZ`, or
  on a PostgreSQL whose session zone is not UTC were.

  **Upgrade note.** The conversion does not rewrite table heaps, but it does
  rebuild the 20 indexes that touch the converted columns, and every affected
  table is locked until the migration commits. Measured at 2,000,000 rows:
  238 ms for an indexed column, versus 0.33 ms with no index. Budget accordingly
  on a large `audit_logs`.

  **Existing rows** are reinterpreted as UTC, which is exact for anything written
  at UTC and adopts the already-shifted instant otherwise. `agents` writes one
  `TIMESTAMPTZ` and one naked column from the same clock in a single INSERT, so
  operators can check their own history before upgrading — the query is in the
  migration header.

  Two guards keep the class closed: `cmd/timestamptz-lint` fails CI on a naked
  `TIMESTAMP` in any migration, and an integration test asserts the applied
  schema holds none.

- Agent registration now returns `400 Bad Request` (was `500 Internal Server
  Error`) when the request references an organization or user that no longer
  exists — a foreign-key violation surfaced when a stale API key or deleted org
  is used. A bad credential is a client error, not a backend fault, and the 500
  made it look like an outage to SDK and CI callers. The duplicate-name case
  (`409 Conflict`) is unchanged. Both conditions are now typed sentinel errors
  (`application.ErrAgentNameExists`, `application.ErrInvalidOrgOrUser`) mapped
  via `errors.Is` in the authenticated and public registration handlers,
  replacing fragile error-string comparison.

## [1.0.0] - 2026-06-01

First stable release of the AIM platform. The stage in
[STATUS.md](STATUS.md) is `stable`, and every gate criterion in
[HARDENING.md](HARDENING.md)'s "Roadmap to 1.0" was met (PRs #247, #248, #250).
Semver is honored from this release forward: breaking changes go through a
deprecation cycle and ship in a major bump; security reports are handled per
[SECURITY.md](SECURITY.md).

See [README.md](README.md) for the full feature set (agent identity and
attestation, Ed25519 key management, per-capability trust and execution modes,
MCP and A2A support, PKCE / OAuth Device Grant auth, and the Python/TypeScript/Java
SDKs).

### Fixed
- SDK download embedded an `http://` server URL behind the TLS-terminating
  ingress; the SDK's refresh-token POST was then 301'd and its body dropped,
  failing registration with 401. Embedded URLs are now coerced to `https` for any
  public host, and the credentials email is keyed as both `userEmail` (Python SDK)
  and `email` (Java SDK) (#263).
- Agent-creation no longer leaks raw PostgreSQL driver errors (e.g. foreign-key
  constraint violations) to the API response, and the duplicated
  "failed to create agent:" error prefix is removed (#264).
- Release CI `verify-publish` poll window widened to ~2 minutes so registry
  propagation no longer marks successful publishes as failed (#265).

### Notes
- This is the first release under the `platform-v*` tag convention. Earlier bare
  `v*` tags in this repository tracked the Python SDK's version line and a legacy
  ad-hoc tag, not the platform; they do not represent platform releases.

[1.0.0]: https://github.com/opena2a-org/agent-identity-management/releases/tag/platform-v1.0.0
