# Changelog

All notable changes to the AIM platform are documented here. The format is based
on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and from this release
forward the platform follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

> Scope: this changelog tracks the **platform** (backend + dashboard), tagged
> `platform-v<version>`. The SDKs are versioned and released independently
> under their own `sdk-*-v<version>` tags (`aim-sdk` on PyPI, `@opena2a/aim-sdk`
> on npm; the Java SDK is built from source in `sdk/java`).

## [Unreleased]

### Fixed — the dashboard asks for the signed-in user once per page load

- The dashboard shell, the sidebar, the header and the deactivation check each requested `/api/v1/auth/me` when a
  page loaded, and that route shares the strict rate limit of the sign-in route. A long enough run of page loads
  from one address was refused with 429, which left the sidebar without its account section and refused the next
  sign-in from that address. The client now sends one profile request for the callers that ask while it is on the
  wire; the next caller after it settles, or a caller with another session token, sends its own.

### Fixed — the Java SDK documentation no longer states dependency versions its pom contradicts

- `sdk/java/README.md`, which ships in the SDK download, listed Jackson 2.16 and BouncyCastle 1.79,
  and `docs/sdk/java.md` listed Jackson 2.16 and BouncyCastle 1.77. `sdk/java/pom.xml` pins Jackson
  2.18.11 and BouncyCastle 1.85.
- Both documents now name each library and its purpose, and point at `sdk/java/pom.xml` for the
  version. A test in the Java SDK fails when either document states a version the pom contradicts.

### Changed — the Java SDK's `reportCapabilities` follows the enforcement mode and raises when a registration fails

- What to check before upgrading: in strict mode a reported capability stays pending until an
  administrator approves it, and the result counts it under `pending`, not `granted`. A
  registration that AIM refuses or fails now throws `AIMException` with the HTTP status, and the
  capabilities after it are not sent. Failures used to be swallowed, and one whose message
  contained "500" was counted as granted.
- `reportCapabilities` sends one `POST /api/v1/sdk-api/agents/{id}/capabilities/register` per
  distinct capability, one at a time in the order each first appears, with the body
  `{"capabilityType": ...}`, and reads the outcome AIM answers with. It used to post to
  `/api/v1/sdk-api/agents/{id}/capabilities`.
- The result keeps `granted` and `total` and adds `pending` and `results`. `granted` counts
  capabilities granted now or already held, `pending` counts those awaiting approval, `total`
  counts distinct capabilities, and `results` has one entry per capability with its `status`
  (`granted`, `already_exists` or `pending`) and, for a new pending request, its `requestId`.
- `reportCapabilities(List)` is new. `reportCapabilities(List, Map)` is deprecated: its `scope` is
  ignored, and passing one logs a warning. A null or empty entry throws `ConfigurationException`
  before anything is sent.
- `registerCapability(String, String)` sends `capabilityType`. It sent `capability`, which the
  route does not read, so every call was refused with 400.
- `registerCapability(String, String, String)` reads `status` from a 409 answer, which can be
  `pending` as well as `already_exists`; it used to report every 409 as `already_exists`. A 404
  now throws; it used to be reported as `success: true` with status `not_tracked`.

### Fixed — a password sign-in records the sign-in time and writes nothing else to the user

- Both password sign-in routes (`POST /api/v1/auth/login/local` and `POST /api/v1/public/login`)
  read the user, checked the password, and then saved their whole copy of the user row to record
  `last_login_at`. A password change, password reset, deactivation or role change that was saved
  between that read and that write was overwritten with the values the sign-in had read. The write
  also cleared any pending password-reset token, because the sign-in's read never loads it, so a
  reset link requested before a successful sign-in stopped working.
- A sign-in now writes `last_login_at` and `updated_at` only. A pending reset token stays valid
  until it is used or expires.

### Changed — the API refuses to start without `KEYVAULT_MASTER_KEY` unless `ENVIRONMENT` is `development` or `test`

- With no `KEYVAULT_MASTER_KEY`, the API generated a new master key at every start unless
  `ENVIRONMENT` was exactly `production`. An unset `ENVIRONMENT`, or a value such as `prod` or
  `staging`, ran on that generated key. The key changes at each restart, so the server signing key
  changed with it and agent private keys stored under the previous key could no longer be
  decrypted.
- A generated key is now used only when `ENVIRONMENT` is exactly `development` or `test`. Any other
  value, including an unset one, stops startup with an error that names `KEYVAULT_MASTER_KEY` and
  the `ENVIRONMENT` value it read. The error never contains a key.
- To upgrade, set `KEYVAULT_MASTER_KEY` (`openssl rand -base64 32`) on every deployment that is not
  local development. The docker-compose files already require it.

### Fixed — the Java SDK's `useMcpTool` reaches the usage report route

### Fixed: webhook, MCP server and agent card requests follow no redirect and connect only to the addresses registration admits

- A webhook URL, an MCP server's capability and verification URLs and an agent card URL are checked when they are
  accepted. The request itself was sent by a client that, for webhooks and MCP servers, followed up to 10 redirects,
  and, for all of them, resolved the hostname again when it connected. An endpoint that redirected, or whose name
  later resolved to a loopback, private or cloud metadata address, therefore received a request from the server, and
  the webhook test send returned the status code and the connection error text to the caller.
- All of these requests now go through one client. It returns a 3xx as the response and never requests its
  `Location`: a webhook delivery records the 3xx as a failed delivery, so an endpoint that relies on a redirect (for
  example from http to https) needs to be registered with its final URL. It checks the address of every connection,
  retries and replays included, after the name is resolved and before it connects, with the same address policy as
  registration. It ignores `HTTP_PROXY` and `HTTPS_PROXY`.
- The address policy also refuses 0.0.0.0/8, 240.0.0.0/4, 255.255.255.255, the Azure platform address
  168.63.129.16, 2001::/32 (Teredo), 100::/64, 2001:db8::/32 and fec0::/10, and judges IPv4-mapped, NAT64
  (64:ff9b::/96, 64:ff9b:1::/48) and 6to4 (2002::/16) addresses on the IPv4 address they carry.
- A refused URL or connection is reported by the class of address, for example `destination address is not allowed
  (loopback)`, without the resolved address or the resolver's error.
- The webhook test send stores at most 1 KB of the endpoint's response, as deliveries already did.
- Tests check that a 307 or 302 from an endpoint is recorded and its target is never requested, that a stored URL whose
  name resolves to loopback is refused when the connection is made, that registration and the connection check agree
  on every address class, that the proxy environment is not used, that refusal text carries no address, and that
  each of these services builds its client only through the shared constructor. Each check has a control that shows
  it can fail.

### Fixed: an agent's activity returns no network address, user agent or personal metadata to a non-admin

- `GET /api/v1/agents/:id/activity` is open to every principal in the organization and returns the audit records an
  agent wrote: its verification requests, capability violations and honeytoken hits. It returned each record's
  `ipAddress`, `userAgent` and full `metadata`, including the context the agent sent with its call. Some of these
  records are also returned by `GET /api/v1/agents/:id/audit-logs`, which withholds those members from a non-admin, so
  a manager, member, viewer, API key or agent could read them through the activity route instead.
- For every caller except an admin, the route now returns each record without `ipAddress` and `userAgent` (absent,
  not empty), and its `metadata` holds only AIM's decision on the call: `actionType`, `resource`, `riskLevel`,
  `trustScore`, `autoApproved` and `denialReason`. The agent page builds its activity timeline and refused-call
  findings from these. A record with none of them has no `metadata` member. An admin's response is unchanged.
- Tests write an activity record with a canary address, user agent, email and metadata value, then check that a
  manager's, member's, viewer's and role-less caller's response contains none of them and keeps the decision members,
  and that an admin's response still contains them.

### Fixed: the per-agent and per-MCP-server audit logs return no network address, user agent or metadata to a non-admin

- `GET /api/v1/agents/:id/audit-logs` and `GET /api/v1/mcp-servers/:id/audit-logs` are open to every principal in
  the organization, while `/api/v1/admin/audit-logs`, which returns the same records, admits only admins. Both
  routes returned each record's `ipAddress` and `metadata`, and the per-agent route also its `userAgent`, so a
  manager, member, viewer, API key or agent could read the network address and client of every colleague who acted
  on the agent or server, and any email the action logged.
- For every caller except an admin, both routes now return each audit record without `ipAddress`, `userAgent` and
  `metadata`: the members are absent, not empty. `userName`, the action, the actor and the timestamp stay. An
  admin's response is unchanged. Attestation and capability events in the MCP server timeline keep their metadata,
  which holds no data about a person.
- Tests write a record with a canary address, user agent, email and metadata value, then check that a manager's,
  member's, viewer's and role-less caller's response to each route contains none of them, and that an admin's
  response and `GET /api/v1/admin/audit-logs/:id` still contain them.

### Fixed: the fleet governance guide's deployment step starts the server

- Step 1 of `docs/use-cases/fleet-governance.md` gave the server `DATABASE_URL`, which it does not read, so the
  server stopped at startup on the missing `POSTGRES_HOST`. Its sample `JWT_SECRET` was 29 characters, and the
  server refuses one under 32. The dashboard was given `API_URL`, which it does not read, and the `/health` output
  shown was not the one the server returns.
- The step now writes `JWT_SECRET`, `KEYVAULT_MASTER_KEY` and `POSTGRES_PASSWORD` to `.env` with `openssl rand`, and
  its compose file gives the server the `POSTGRES_*` settings it reads, a Redis service for token revocation, and a
  database healthcheck it waits on, since the server connects once at start. The dashboard gets
  `NEXT_PUBLIC_API_URL`. The fabricated `docker compose up` listing is removed, and the `/health` output is the
  server's.
- A test reads the step as docker compose would and loads the server's configuration from it. It checks that the
  database and Redis hosts name services in the file, that the database credentials match, that each image is one
  the release publishes, that the dashboard reads each variable it is given, and that the `/health` output has the
  handler's fields.

### Fixed: the token endpoint accepts an assertion addressed to the server's address and port when `AIM_BASE_URL` is unset

- `POST /api/v1/oauth/token` requires the assertion's `aud` to name the server: either `AIM_BASE_URL` or the origin
  the request arrived on. It built that origin from the host name without its port. With `AIM_BASE_URL` unset, a
  server reached at `http://localhost:8080` therefore refused `aud` `http://localhost:8080`, which is the value the
  TypeScript SDK's `OAuthTokenManager` signs when given that base URL, and accepted `http://localhost` instead.
- The origin now keeps the port from the `Host` header. `http://localhost:8080` and `http://localhost:8080/api/v1`
  are accepted on that server, and an `aud` naming the same host on another port or scheme is refused. An `aud`
  equal to `AIM_BASE_URL` is accepted as before, and so is the origin of a server on the scheme's default port.
- Tests send the request to an address with a port and to one without, and check which audiences each accepts.

### Fixed — the fleet governance guide describes the access token the server issues, and no longer points at a JWK Set

- `docs/use-cases/fleet-governance.md` told readers to request a token at `/api/v1/token` with `agentId` and `scope`
  in a JSON body, showed a response with `accessToken`, `tokenType`, `expiresIn` and `scope` whose token header read
  EdDSA, and told other services to verify the token against `/.well-known/jwks.json`. The server mounts the endpoint
  at `POST /api/v1/oauth/token`, accepts only the RFC 7523 JWT-bearer grant, returns `access_token`, `token_type` and
  `expires_in`, signs the token with HS256 under `JWT_SECRET`, and publishes no JWK Set. The documented request named
  a path the server does not mount, and no published key verified the token.
- The step now documents the grant: the assertion the agent signs with its Ed25519 key, the `sub`, `aud` and `exp`
  claims the server requires, the `AIM_BASE_URL` setting that `aud` must equal, and the response as the server
  returns it. A new section says how the token is verified: by AIM Server, on its `/api/v1/agents` routes, which
  check the signature, the expiry, revocation and the agent's status on each request. It says to keep `JWT_SECRET`
  on the server and not to hand the access token to another service as proof of the agent's identity.
- The step's three `OIDC_*` environment variables are removed: the server reads none of them. The guide and the use
  case index name the endpoint an OAuth 2.0 token endpoint.
- A test compares the step with a token the handler issues, so a change to the signing method, the path or the
  response fields fails until the guide is rewritten to match.

### Fixed — `POST /api/v1/public/agents/register` answers 401 without a user access token, and the API reference says it needs one

- The route registers an agent only for a signed-in user: it takes the user and organization from a user access
  token and reads no other credential. A request with no credential, or with an API key in `X-AIM-API-Key`,
  `X-API-Key` or `Authorization: Bearer`, used to run through key generation and an insert the database refused, and
  came back as 400 `invalid organization or user for this registration`. It is now answered 401 before the body is
  read, with a message that names the two paths that register an agent: a user access token as
  `Authorization: Bearer <token>` on this route, or an API key in `X-API-Key` at `POST /api/v1/agents`. A signed-in
  caller is served as before.
- The dashboard's API reference listed the route as public with no auth required, so the request it built carried
  no token. Its example body also used `type` where the route reads `agentType`, and left out `displayName`. The
  entry now requires a bearer token and documents the four required fields, and the route's API description says the
  same.
- A client that sent an API key to this route and matched on the 400 now sees 401; the key is accepted at
  `POST /api/v1/agents`.

### Fixed — the Java SDK signs requests in the form the platform verifies, and its approval poll is signed

- `AIMClient` signed an `X-Signature` GET over `METHOD\npath\ntimestamp\n`, and a POST with an empty body the same
  way. The platform verifies over those three lines with no trailing newline and adds the body as a fourth line only
  when the request has one, so it refused both with 401. The signed read of an agent's MCP servers, made when an MCP
  server registration returns 409, was one of them. Both now sign the form the platform verifies.
- `waitForApproval` polled `GET /api/v1/sdk-api/verifications/{id}` with a bearer token. That endpoint authenticates
  only the `X-AIM-Agent-ID`, `X-AIM-Timestamp` and `X-AIM-Signature` headers, so every poll was refused and a pending
  action timed out even after an administrator approved it. Each poll now sends those headers, signed over
  `GET\n/api/v1/sdk-api/verifications/<id>\n<agent id>\n<timestamp>` with both IDs in canonical lowercase. A client
  with no signing key, or with an agent or verification ID that is not a UUID, now fails at once with a
  `VerificationException` instead of polling until the timeout.
- Tests verify each signature the client sends against the message the platform rebuilds, written out in the test:
  a GET, a POST with a body, a POST with an empty body, and an approval poll made with upper-case IDs.

### Security — a stored agent private key decrypts only in the agent row it was written for

- Server-generated agent private keys were sealed with AES-256-GCM and no additional data, so a ciphertext copied
  into another row of `agents.encrypted_private_key` decrypted as that agent's key on the credential and A2A signing
  paths. Keys are now stored in format v2: a cleartext header (a version byte and a storage-key id), the nonce, and
  the ciphertext sealed with the additional data `"aim/agent-private-key" 0x00 <header> 0x00 <agent id>`, where the
  agent ID is the row's primary key in canonical lowercase. Each decrypt takes the ID from the row it loaded. A
  ciphertext copied to another row, sealed for another purpose, or carrying an altered header is refused, and the
  credential rotation helper keeps the binding.
- The request path refuses the earlier v1 format. At startup, before the server accepts requests, a one-time
  migration re-encrypts each v1 key to v2 bound to its own row with a compare-and-swap update and logs counts only
  (`v1Read`, `v2Written`, `alreadyV2`, `failures`), never a key or an agent ID. It is recorded in `schema_migrations`
  as `keyvault/agent-private-key-v2` once a run finishes with no failures; until then it runs again at the next
  startup. A row that cannot be read stays refused until that agent's credentials are rotated. An earlier release
  cannot read v2 keys, so rolling back after the migration leaves server-generated keys unreadable until rotated.
- Registration assigns the agent ID before it encrypts the generated key, and the agent repository keeps an ID the
  caller assigned instead of replacing it.
- Tests pin the additional data byte for byte and fail when a ciphertext copied from one row decrypts under another,
  when a ciphertext sealed for another purpose or a v1 ciphertext is accepted on the request path, when the migration
  re-binds a v2 ciphertext copied from another row, when a completed migration reads agent keys again, or when a
  planted key, its ciphertext or an agent ID appears in the migration log or a refusal error.

### Fixed — No tracked file carries a home directory path from the machine that built or wrote it

- Six backend executables were tracked in the repository: macOS builds from a developer machine, 13 to 25 MB each,
  that embedded that machine's home directory paths (its Go module cache and its checkout). Nothing used them; the
  backend image builds `cmd/server` from source. They are removed, and `.gitignore` names the local build outputs
  (`apps/backend/server`, `main`, `bin/` and `cmd/server/server`) beside the existing `aim-server` entry. The two
  instructions that ran the tracked `./server`, in `sdk/python/tests/README.md` and the LangChain CRUD example, now
  say `go run ./cmd/server`.
- The demo census (`docs/demo/lib/census.mjs`) assembles its local-path canary at run time, so the script holds no
  home directory path of its own. Its positive control still reports all nine classes.
- One negative assertion in an SDK doc test builds its pattern with `new RegExp` so its own source no longer matches
  a tree-wide scan for internal path references by chance.
- `scripts/lint-public-surface.mjs` reads every file `git ls-files` returns, binary files included, and fails on a
  line that carries a macOS home directory path. `git grep` did not report the paths inside the executables; this
  script did. Extra classes come from a file kept outside the repository, in the census's
  `class<TAB>regex<TAB>canary` format (`--forbidden <file>` or `$PUBLIC_SURFACE_FORBIDDEN_FILE`). Base64 that no
  reader reads, a data: URI payload or a lockfile integrity digest, is blanked before matching. Every class must
  catch its own canary first, otherwise the run reads INCONCLUSIVE (exit 2); findings print path, line and class,
  never the matched text.
- `scripts/test-lint-public-surface.mjs` (`node --test scripts/test-lint-public-surface.mjs`) holds 13 cells. One runs
  the lint over this repository and fails on any finding; before this change it reported 7 (the six executables and
  the census line).

### Fixed — The agent page reports verify, suspend, reactivate and delete outcomes beside the actions

- The agent page reported these four actions in browser alerts. An alert carries no link, ignores the theme, and on
  failure showed the server's error text, which for a failed delete was the database's raw foreign-key message. The
  outcome now renders inline under the action buttons: a success as a status message, a failure as one sentence of
  reason and one next step, announced to assistive technology. The reason comes from the response's stated `code` or
  its status (403: your role does not allow it; 404: the agent no longer exists, with a link back to the list; no
  response: AIM could not be reached), never from the server's text; anything else reads "AIM could not delete this
  agent." with a next step.
- `components/ui/action-outcome.tsx` and `lib/action-outcome.ts` are the shared inline outcome for dashboard actions.
- `tests/no-browser-alert.test.ts` counts the lines under `apps/web/app` that open a browser alert, the same lines as
  `git grep -n -E 'alert\(' -- apps/web/app ':!*.fmt' ':!*.final' ':!*.bkp' ':!*.bak'`, and fails when the count
  rises above 16 (22 before this change), with a planted call as its positive control.

### Fixed — The API reference names the password reset fields the endpoint reads

- The developer page documented `POST /api/v1/public/reset-password` with a `token` field and no confirmation
  field. The endpoint reads `resetToken`, `newPassword` and `confirmPassword`, all required, so a request written from
  the documentation was refused with `Reset token is required`. The schema and example body now use those three
  fields, and `newPassword` notes the 8-character minimum the endpoint enforces.
- `public_password_reset_docs_test.go` compares the documented schema fields, their `required` flags and the example
  body of the forgot-password and reset-password endpoints against the json tags of the request structs their
  handlers bind, and fails on any difference.

### Fixed — The EchoLeak demo script shows a placeholder for the admin password

- The login step in `docs/DEMO_SCRIPT_ECHOLEAK.md` also printed the fixed admin password that stacks seeded before
  `aim-bootstrap` were given, so a presenter copying the script carried that value into a new install or a recording.
  The step now shows `<admin-password>` and says the password is set per install: the `DEFAULT_ADMIN_PASSWORD` value
  in `.env` (`./scripts/gen-dev-secrets.sh` generates one), or, if that is unset, a random password that
  `aim-bootstrap --default` prints once on first deploy.
- `sdk/typescript/tests/demo-script-admin-password.test.ts` fails if the demo script contains the fixed value
  anywhere, or if its login step lacks the placeholder or either password source.

### Fixed — MCP server routes answer 404 for another organization's server, the same as for an unknown ID

- Eleven routes under `/api/v1/mcp-servers/:id` answered `404 {"error":"MCP server not found"}` for an ID that does
  not exist and `403 {"error":"Access denied"}` for a server in another organization, so a signed-in caller could
  tell which server IDs exist outside their own organization. They now load the server through the same ownership
  check as the other tenant-scoped handlers and answer `404 {"error":"not found"}` in both cases. The routes are
  `GET`, `PUT` and `DELETE /:id`, `POST /:id/verify`, `POST /:id/keys`, `POST /:id/detect-capabilities`, and `GET`
  on `/:id/verification-status`, `/:id/capabilities`, `/:id/agents`, `/:id/verification-events` and
  `/:id/audit-logs`. Requests for the caller's own servers are unchanged.
- `mcp_handler_tenant_scope_test.go` sends an unknown ID and another organization's ID to each route and fails
  unless both answer 404 with byte-equal bodies; it also checks that the audit-logs route reads no logs for a server
  the caller does not own, and that the caller's own server still returns its audit timeline.

### Fixed — `aim_http_requests_total` records the status the client received, and unknown paths share one series

- `PrometheusMiddleware` read the response status before the app's error handler had run, so a request that ended in
  an error was recorded with status `200`: a 404 on an unknown path, a 401 from an auth group, a 403 or a 500 returned
  by a handler. The middleware now runs the app's error handler itself, as Fiber's logger middleware does, and records
  the status that handler sets. `aim_http_request_duration_seconds` carries the same corrected label.
- A request that reaches no route handler is recorded with the path label `unmatched` instead of its own path. This
  covers unknown paths and requests a group middleware refuses before routing reaches a handler, such as a request
  without credentials under `/api/v1/agents`. Before, every distinct path a caller sent added a new series, whether
  or not the caller could read `/metrics`. Requests a route handler serves keep their normalized path label.
- `prometheus_middleware_test.go` drives requests through a Fiber app and fails when the recorded status differs from
  the status the client received, or when distinct unknown or refused paths add series.

### Removed — backend scripts that approved registrations and signed admin tokens outside the service

- `apps/backend/scripts/approval/approve_registration.go`, `approve_registration.sql`, `quick_approve.sql` and
  `simple_approve.sql` approved a pending registration by writing `users` and `user_registration_requests` directly,
  and `apps/backend/scripts/jwt/generate_jwt.go` signed a 24-hour admin token with `JWT_SECRET`. None wrote an
  `audit_logs` row or refused a production database, and none was built into or copied into the backend image. They
  are removed. Approve a registration with `POST /api/v1/admin/registration-requests/:id/approve`, which records an
  audit entry, and create the first admin with `aim-bootstrap`.
- `sdk/typescript/tests/backend-scripts-no-out-of-band-approval.test.ts` fails when a file under
  `apps/backend/scripts` writes `user_registration_requests`, signs a JWT, or inserts into `users` without first
  refusing a database that holds a production-shaped organization, as the seed files do.

### Fixed — an empty `talks_to` list no longer allows every MCP server in the agent-signed action check

- `PublicMCPHandler.VerifyMCPAction` approved any MCP server in the agent's organization when the agent's `talks_to`
  list was empty, and refused only servers missing from a non-empty list. An empty list now names no servers, so the
  check returns 403 `unauthorized_mcp_access` for every server, which matches how
  `AgentService.GetAgentMCPServers` reads the same list. The handler is not mounted on any route in this release.
- `public_mcp_handler_talks_to_test.go` sends signed requests through the handler and fails when an agent with a nil
  or empty `talks_to` list is approved, when a server listed by name is refused, or when an unlisted server is
  approved.

### Fixed — an SDK refresh no longer hands out a refresh token that has no row

- `POST /api/v1/auth/refresh` with an SDK refresh token revoked the presented token's `sdk_tokens` row, then inserted
  a row for the new refresh token and ignored the insert's error. When the insert failed, the response still carried
  the new token with `rotated: true`, but the next refresh refused it (an SDK token without a row), and the presented
  token's row was already revoked, so the SDK had to be set up again. The revocation and the insert now run in one
  database transaction, and the new token is returned only when it commits. When it does not, neither write takes
  effect: the presented token comes back unchanged with a fresh access token and `rotated: false`, the failure is
  logged, and a later refresh rotates normally.
- A failure to record the presented token's usage (`last_used_at`, `usage_count`) is now logged; the refresh still
  proceeds.
- A recovered SDK token (login-issued, with a row) whose denylist write fails comes back unchanged with its row left
  live. Its row was revoked before, so the next refresh refused the token the response had just returned.
- `apps/backend/internal/interfaces/http/handlers/auth_refresh_rotation_test.go` fails when an SDK refresh whose row
  insert fails returns a new token or revokes the old row, when a failed usage record is not logged, and when a
  recovered token's row is revoked while the token is returned unchanged.
  `apps/backend/internal/infrastructure/repository/sdk_token_repository_rotate_test.go` fails when the revocation and
  the insert do not commit or roll back together.

### Fixed — an SDK refresh no longer leaves two live refresh tokens when the old one cannot be revoked

- `POST /api/v1/auth/refresh` with an SDK refresh token revokes the presented token's `sdk_tokens` row, then records a
  row for a new refresh token and returns it. When that revocation failed, the handler still created the new row and
  returned the new token, so the old and the new refresh token were both live. The presented token now comes back
  unchanged with a fresh access token and `rotated: false`, and no row is created; a later refresh rotates once the old
  row can be revoked. Login refresh tokens already behaved this way when their denylist write failed.
- `apps/backend/internal/interfaces/http/handlers/auth_refresh_rotation_test.go` fails when an SDK refresh whose row
  revocation fails returns a new refresh token or creates a row, and when that failure changes a login token's rotation.

### Security — the Java SDK and the Java A2A example use jackson 2.18.11

- `sdk/java/pom.xml` and `examples/a2a-multi-agent-demo/java/pom.xml` set `jackson.version` to 2.18.11; platform 1.0.0
  pinned 2.16.1 in both. The property sets `jackson-databind` and `jackson-datatype-jsr310`, and `jackson-core` and
  `jackson-annotations` resolve to the same version. 2.18.11 is the first 2.18.x release outside four high-severity
  advisories published on 2026-09-22, each of which covers every 2.18.x release through 2.18.10: `jackson-databind`
  GHSA-wv8q-qhhj-9h54 (CVE-2026-91776) and GHSA-cxp5-3px4-pw24 (CVE-2026-91777), and `jackson-core`
  GHSA-7hhh-6rmp-j9qf (CVE-2026-89425) and GHSA-p6pp-m3f8-5c89 (CVE-2026-89407). (#560)
- As of 2026-10-05 no published advisory for `jackson-databind`, `jackson-core`, `jackson-annotations` or
  `jackson-modules-java8` includes 2.18.11 in its vulnerable range. The Java SDK is built from source: rebuild it, and
  the example, to pick up the new version.

### Added — an admin or manager can turn off hybrid mode from the agent page

- The agent page's Key Vault tab showed "Hybrid mode enabled" with no way to turn it off; the act existed only as a
  call to `POST /api/v1/agents/:id/hybrid-mode`. The Post-Quantum Companion Key card now carries a "Turn off hybrid
  mode" control for an admin or manager of the agent's organization while hybrid mode is on. A member or viewer
  does not see it.
- The control asks for confirmation before it sends `{"enable": false}`. After the server accepts, the tab reads the
  key vault again and shows the stored value; a refusal shows the server's reason and leaves the badge as it was.
- The dashboard does not offer turning hybrid mode on.

### Security — the server refuses a `KEYVAULT_MASTER_KEY` of 32 zero bytes

- A master key that decodes to 32 zero bytes passed the length check and was used to encrypt agent private keys.
  The key vault now refuses it, so startup stops with `master key must not be all zero bytes` and the command that
  generates a key (`openssl rand -base64 32`). The error does not repeat the supplied value. Key rotation refuses the
  same value as its new key.
- **Upgrading:** a deployment whose `KEYVAULT_MASTER_KEY` is 32 zero bytes no longer starts. Set it to the output of
  `openssl rand -base64 32`; agent private keys stored under the old value cannot be decrypted under the new one.

### Removed — the source tree no longer carries prebuilt server binaries

- Five macOS x86_64 executables built on developer machines were tracked under `apps/backend`: `aim-server`,
  `bin/aim-backend`, `bin/server`, `main` and `server`, 108.5 MB together. Nothing used them: the container image
  builds the server from source. They are removed (history is left as is), and ignore rules keep a local
  `go build -o` to those paths out of `git status`.
- A test in `apps/backend/cmd/server` reads every blob of the commit under test, not the working tree, and fails if
  one is a compiled executable, object or library (Mach-O, ELF, PE, ar, WebAssembly or Java class) or has the
  executable bit without a `#!` line. A PE file must carry its `PE\0\0` header; a file that only starts with `MZ` is
  not flagged. No path is exempt. Without `git` on `PATH`, or outside a git checkout, the test fails with the reason
  instead of skipping. A second test plants one file of each kind in a scratch repository, commits them, deletes one
  from the working tree and checks that the census reports exactly those. The ignore check covers all six former
  binary paths, `cmd/server/server` included.

### Fixed — password reset tokens are stored as a SHA-256 digest

- `users.password_reset_token` now holds the hex SHA-256 digest of the token in the reset link, as its column
  comment already said, instead of the token itself. `POST /api/v1/public/reset-password` hashes the token it
  receives and looks the user up by that digest, so a value read from the table does not reset a password. The
  link, the request body and the 24-hour expiry are unchanged.
- No schema change: the digest fits the existing column and index. A reset link sent before the upgrade stops
  working, and the user requests a new one; the old value stays stored until a new reset request replaces it.

### Fixed — deployment and installation guides name `KEYVAULT_MASTER_KEY` as required outside development

- `docs/DEPLOYMENT.md`, `docs/guides/INSTALLATION.md`, `docs/guides/DEPLOYMENT_CHECKLIST.md`,
  `docs/guides/QUICK_DEPLOYMENT_REFERENCE.md` and `docs/guides/SETUP_GUIDE.md` showed production and staging
  configurations without `KEYVAULT_MASTER_KEY`. Each now names it as required in every environment except
  `ENVIRONMENT=development`, with `openssl rand -base64 32` to generate it once and the instruction to keep the same
  value across restarts. Agent private keys are encrypted under it; with `ENVIRONMENT=production` the server refuses to
  start without it, and in development a missing key is replaced by one generated at each start, whose data cannot be
  decrypted after a restart. The remedy given is the key, never `ENVIRONMENT=development`.
- `HARDENING.md` no longer says the configuration validator fails on a missing `KEYVAULT_MASTER_KEY`. The validator
  rejects known development values of the key; the refusal to start without it comes from the key vault.
- `scripts/lint-docs-keyvault-master-key.sh` rejects a section of a page under `docs/` that sets `ENVIRONMENT` to
  anything other than `development` without naming `KEYVAULT_MASTER_KEY`, with its file, line and value.

### Added — the dashboard lists an organization's audit records

- `/dashboard/admin/audit-logs` lists the records `GET /api/v1/admin/audit-logs` returns, newest first, 50 per page.
  Each row shows the time in UTC, the actor, the action and the target (resource type and ID). The page is reached
  from the Organization tabs and is open to admins only, as the route is. No row shows a network address, a user
  agent or metadata.
- A user act names the user and an agent act names the agent. A record with neither says "No user or agent
  recorded" instead of attributing the act to the system.
- `GET /api/v1/admin/audit-logs` without a `user_id` filter, and its JSON export, now send `userName` and
  `agentName` with each record that has a user or an agent, as the per-agent and per-resource audit routes already
  do. The CSV export is unchanged.

### Fixed — sample output in the docs shows a placeholder account

- The Azure CLI sample in `docs/guides/QUICK_DEPLOYMENT_REFERENCE.md` showed the signed-in account as a real
  person's address. It now shows `admin@example.com`.
- `scripts/lint-docs-sample-addresses.sh` fails if a Markdown page under `docs/` carries an email address outside
  the reserved example domains, the project's own domain and the generic placeholders the guides already use, and
  names each one with its file and line. `scripts/test-lint-docs-sample-addresses.sh` tests it.

### Removed — a prebuilt server binary is no longer part of the source tree

- `apps/backend/cmd/server/server`, a 20.7 MB macOS x86_64 executable built on a developer machine, was tracked in
  the repository. Nothing used it: the container image builds the server from source. It is removed, and an ignore
  rule keeps the output of `go build` in that directory out of `git status`.
- A test in `apps/backend/cmd/server` fails if a compiled `server` is tracked there again or the ignore rule goes.

### Changed — the developer compose stack passes `DEFAULT_ADMIN_PASSWORD` only to a one-shot bootstrap run

- `docker-compose.yml` set `DEFAULT_ADMIN_PASSWORD` in the backend's environment. The backend never reads it; only
  `aim-bootstrap --default` does. The long-running container still kept the value for its whole life, where
  `docker inspect` and any process in the container could read it.
- A new `bootstrap` service runs `/app/aim-bootstrap --default` with `DEFAULT_ADMIN_PASSWORD` and the database
  connection, and exits: `docker compose run --rm bootstrap`. Its profile keeps it out of `docker compose up`. The
  backend no longer has the variable.
- The documented step this replaces, `docker compose run --rm aim-backend /app/aim-bootstrap --default`, named a
  service the file does not define, and the backend's environment has no `DATABASE_URL` for the bootstrap to connect
  with. `docs/quick-start.md` now writes `.env` with `./scripts/gen-dev-secrets.sh` before `docker compose up -d`,
  which does not start without it, and adds the bootstrap step.
- A test reads each root `docker-compose*.yml` and fails if any service other than a one-shot bootstrap run sets
  `DEFAULT_ADMIN_PASSWORD`.

### Fixed — an approved device code is exchanged for one token pair

- `POST /api/v1/oauth/device/token` minted a new token pair, each its own session, on every poll of an approved
  device code until the code expired 15 minutes after it was issued. The code never left `approved`.
- The code now moves to `consumed` in the same database transaction that mints its pair, so it is exchanged once.
  Of several polls racing on one approved code, one receives the pair. A later poll mints nothing and is answered
  `400` with `invalid_grant` (RFC 6749 Section 5.2, which RFC 8628 Section 3.5 inherits). If the pair fails to
  mint, the transaction rolls back and the code stays `approved`, so the client's next poll can still receive it.
- `aim-sdk login` stops polling when it receives its pair and is unaffected. The dashboard's device page already
  shows a code that is no longer pending as "no longer waiting for approval".

### Fixed — a wrong password at sign-in is reported once, and not as an expired session

- The dashboard treated every 401 as an expired session, including the sign-in route's answer to a wrong password.
  A failed sign-in showed a "Session expired" toast and a "Could not sign in: Unauthorized" toast, wrote
  "Unauthorized" under the password field, spent any stored refresh token, and reloaded the page 1.5 seconds later,
  which cleared the email address. The server's "Invalid email or password" was never shown.
- A 401 from a route under `/api/v1/public/` is now that route's own refusal: the client raises it with the server's
  message and leaves the session alone. A 401 from any other route still ends the session as before.
- The sign-in page shows a server-side failure once, in an alert at the top of the sign-in card, and moves focus to
  it. The alert stays until the next submit. No toast is shown and no server text is written under an input. The
  invalid-credentials refusal marks both inputs invalid because it names both. When the server sent no message (the
  request did not reach it, or it answered without one), the alert gives a neutral message that does not say the
  credentials were wrong. Client-side validation still shows its errors under the fields.

### Changed — `aim-bootstrap` prints the admin password only when it generated it

- `aim-bootstrap` ended every run by printing the admin email and password, including a password the operator
  supplied with `--admin-password` or `DEFAULT_ADMIN_PASSWORD`. That copied the password into any log capturing the
  command's output, which `DEFAULT_ADMIN_PASSWORD` exists to avoid. The credential block is now printed only when
  `--default` generated the password; with a supplied password the run names the admin account and says the
  password is not printed.
- Tests cover both cases: a supplied password, by flag or environment variable, is absent from the output, and a
  generated one is printed once with the capture notice.

### Security — the server refuses to start with the `JWT_SECRET` placeholder from `.env.example`

- The root `.env.example` sets `JWT_SECRET` to a placeholder long enough to pass the 32-character minimum, so a copy
  of the file used without editing started the server signing tokens with a value published in this repository. That
  placeholder is now on the list of known defaults: startup stops with an error naming `JWT_SECRET` and the command
  that generates a value (`openssl rand -hex 32`).
- **Upgrading:** a deployment whose `.env` still carries that placeholder no longer starts. Set `JWT_SECRET` to the
  output of `openssl rand -hex 32`; users signed in under the old value sign in again.
- `apps/backend/.env.example` no longer puts a comment on the same line as an empty value. godotenv, which the backend
  uses to load `.env` files, reads `JWT_SECRET=   # REQUIRED: ...` as the value `# REQUIRED: ...`, which also passed
  the length check. `POSTGRES_PASSWORD`, `SMTP_USERNAME` and `SMTP_PASSWORD` had the same layout and now carry their
  comment on the line above.
- A test loads each environment template (`.env.example`, `.env.quickstart`, `apps/backend/.env.example`) as written
  and expects startup to refuse its `JWT_SECRET`.

### Fixed — on tablet-width screens the end of a dashboard page is no longer hidden beneath the bottom tab bar

- From 640 to 1023 pixels wide the dashboard shows the bottom tab bar, but the page's bottom padding fell back to 24
  pixels there, so the last part of a page, scrolled to its end, stayed beneath the bar and its raised Secure button. The
  padding now is the bar's height token plus 1.75rem at every width the bar is shown: on a phone without a safe-area
  inset it is unchanged at 112 pixels, it grows with the inset, and from 1024 pixels up it stays 24 pixels.
- `apps/web/tests/e2e/mobile-tab-bar.spec.ts` checks, at 375x812, 375x667 and 768x1024 for a role that can register
  agents, that a page scrolled to its end ends above the raised Secure button, that nothing is painted over that
  button, and that the navigation drawer ends above it.

### Changed — the dashboard's Compliance and Credentials pages have new addresses

- Compliance is at `/dashboard/compliance`. It is still shown to admins only, and
  `/dashboard/admin/compliance` answers with a 308 redirect to it.
- API keys and SDK tokens are listed on one page, Developers → Credentials at
  `/dashboard/credentials`, in one section each (`#api-keys`, `#sdk-tokens`).
  `/dashboard/api-keys` and `/dashboard/sdk-tokens` answer with a 308 redirect to it, so
  bookmarks keep working.
- The links from an agent, an MCP server, an alert or a threat to the API key or SDK token
  that registered it open that section of the Credentials page and mark the key or token
  they name (`?highlight=<id>`). The former pages did not mark it.

### Changed — a newly registered agent's page shows its first-run steps, and the old success URL redirects

- `/dashboard/agents/<id>` shows a first-run panel until the agent makes its first successful
  authenticated call (`lastActive` is empty), unless the agent is suspended or revoked: the
  agent's identifier and public key with copy buttons, and the Python and Java steps that connect
  it. The server sets `lastActive` after every successful agent-authenticated request, so the
  panel goes away after the first one.
- The separate page at `/dashboard/agents/:id/success` is removed. That success URL answers with a
  permanent (308) redirect to `/dashboard/agents/:id`, so a bookmarked or printed link still
  reaches the agent.
- At phone widths the identifier and public-key rows truncate their value instead of widening the
  page, so both copy buttons stay on screen.
- The Java steps, on the first-run panel and in the dashboard's SDK quickstart, say where
  `AIMClient.secure()` reads its credentials (`AIM_REFRESH_TOKEN` or
  `~/.aim/sdk_credentials.json`) and that the Java download on the SDK page carries them; a clone
  of the repository carries neither (#583).

### Fixed — the agent page loads when a trust-score answer lacks its fields

- The trust-score card on `/dashboard/agents/<id>` shows its empty state when
  `GET /api/v1/trust-score/agents/<id>/breakdown` answers without `factors`, or the history
  answer without `history`. Before, either answer threw while the page rendered and replaced the
  whole page with "This page could not load." (#583).

### Fixed — MCP security policy rules mean what the admin page says

- `rules.minTrustScore` on the `mcp_*` security policies is on the [0,1] scale of an MCP server's
  trust score. Migration 114 divides every stored JSON number above 1 by 100, including the seeded
  floors of 50 and 30, which sat above every possible trust score. The admin security-policies page
  still shows the floor as a percentage and now saves it as a fraction.
- Each MCP policy card on the admin security-policies page has an Edit button. Nothing opened the
  Edit MCP Policy dialog before, so a stored MCP policy could not be changed from the page. Saving
  an edit keeps the policy's stored severity threshold and scope; the dialog no longer resets them
  to `medium` and `all`.
- A bare `*` in `allowedDomains` or `blockedDomains` matches every server. It matched no host
  before, so the seeded `["*"]` allowlists would have rejected every server.
- `allowedCapabilities` on an `mcp_allowlist` policy is checked for a server that the policy's
  `allowedDomains` or `allowedNames` admit: such a server violates the policy when it declares a
  capability outside the list. The field was ignored before. A policy that sets neither
  `allowedDomains` nor `allowedNames` enforces nothing, as before, so `allowedCapabilities` on
  its own has no effect.
- MCP policies are still not evaluated by any running code path, so none of this changes whether
  a connection is allowed or blocked today (#355).

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
  `"version": null`. The build now stamps `version` from the `VERSION` build argument and `commit` from the
  `COMMIT` build argument: `docker build --build-arg COMMIT=$(git rev-parse HEAD) ...`. A build that passes no
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
