# Changelog

All notable changes to the AIM platform are documented here. The format is based
on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and from this release
forward the platform follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

> Scope: this changelog tracks the **platform** (backend + dashboard), tagged
> `platform-v<version>`. The SDKs are versioned and released independently
> under their own `sdk-*-v<version>` tags (`aim-sdk` on PyPI, `@opena2a/aim-sdk`
> on npm; the Java SDK is built from source in `sdk/java`).

## [Unreleased]

### Fixed — a negative offset on the agent alert, agent violation, admin alert and A2A violation routes is read as 0 (#844)

- `GET /api/v1/agents/:id/alerts`, `GET /api/v1/agents/:id/violations`, `GET /api/v1/admin/alerts` and
  `GET /api/v1/a2a/security/violations` read a negative `offset` as 0. `offset=-1` and `offset=-100` reach the page
  query (`LIMIT $2 OFFSET $3`) as offset 0 and are answered 200 with the first page; the first three routes echo
  `"offset": 0`. `offset=0` and `offset=2` reach the query as sent.
- Before, each of the four handlers sent the negative offset to that query unchanged. PostgreSQL refuses a negative
  `OFFSET` (`ERROR: OFFSET must not be negative`, measured on PostgreSQL 14.21 with a bound offset of -1 and of
  -100), and with the page query refused, the request was answered 500: `Failed to fetch alerts` on both
  alert routes, `An internal error occurred. Please try again later.` on the agent violation route and
  `Failed to get security violations` on the A2A route.
- Tests: `apps/backend/internal/interfaces/http/handlers/list_routes_negative_offset_test.go`.

### Fixed — a negative offset on the MCP server audit log route is read as 0 (#841)

- `GET /api/v1/mcp-servers/:id/audit-logs` reads a negative `offset` as 0. `offset=-1` and `offset=-100` are answered
  200 with the first page of the server's timeline, the same body `offset=0` gets; before, both panicked with
  `runtime error: slice bounds out of range` where the handler pages the timeline, and the request was answered 500.
  Measured from a member's session, behind the recovery middleware the server mounts, with zero events on the
  server's timeline and with three (an audit record, an attestation and a detected capability).
- Tests: `apps/backend/internal/interfaces/http/handlers/mcp_audit_logs_offset_test.go`.

### Fixed — `GET /api/v1/agents/:id/sdk` names the server with the port the request reached it on (#792)

- The Python package's `aim_sdk/config.py` (`AIM_URL`) and `README.md` (`AIM Server`) name the host of the request
  the package was downloaded with together with its port: a request to `127.0.0.1:18080` names
  `http://127.0.0.1:18080`. Before this change both files left the port out for each of the four hosts with a port
  the tests send (`127.0.0.1:18080`, `localhost:8080`, `[::1]:8080` and `aim.example.com:8443`), and a request to
  `127.0.0.1:18080` named `http://127.0.0.1`. The scheme's own port (80 for http, 443 for https) and a port that is
  not a number from 1 to 65535 are left out: a plain-http request to `aim.example.com:80` names
  `https://aim.example.com`.
- The comments on the compliance, drift detection and user feedback factors in
  `apps/backend/internal/application/trust_calculator.go` say that the calculation leaves a factor with no data out
  of the weighted mean and counts its 0.5 placeholder in the 9-factor sum that caps the composite. They said the
  factor was excluded from the composite and the 0.5 a display placeholder only. The calculation is unchanged.
- Tests: `TestAgentSDKDownload_NamesThePortTheRequestReached` and `TestGetAIMBaseURL_LeavesOutAPortThatIsNotANumber`
  in `apps/backend/internal/interfaces/http/handlers/agent_sdk_download_test.go`, `TestGetAIMBaseURL_HTTP` and
  `TestGetAIMBaseURL_DifferentHosts` in `apps/backend/internal/interfaces/http/handlers/agent_handler_test.go`, and
  `apps/backend/internal/application/trust_score_excluded_factors_docs_test.go`, which now also reads the
  calculator, `apps/backend/internal/domain/trust_score.go` and
  `apps/backend/internal/interfaces/http/handlers/trust_score_handler.go`.

### Added — `GET /api/v1/agents/:id/feedback` returns the counts and the average of an agent's user ratings (#666)

- The route answers the number of ratings users have given the agent (`total`), how many are positive (4 or 5),
  neutral (3) and negative (1 or 2) (`positiveCount`, `neutralCount`, `negativeCount`), and their average
  (`averageRating`), which is `null` when the agent has no ratings. The user feedback trust factor is calculated from
  `total`, `positiveCount` and `negativeCount`. The answer carries no rating's comment and not the user who left it.
- The server registers the route bound to its agent: a request made with an agent's own credential (a request it
  signs with its Ed25519 key, its OAuth service token or its API key) may name only that agent's ID. Another agent's
  ID, or a value that is not an ID, is answered `403` with `reasonCode` `agent_path_mismatch` before the named agent
  or any feedback is read.
- A user's session reads any agent of the user's organization. An agent of another organization is answered `403`, an
  ID that names no agent `404` and a value that is not an ID `400`.
- Tests: `apps/backend/internal/interfaces/http/handlers/agent_user_feedback_read_test.go` and
  `apps/backend/internal/application/trust_calculator_user_feedback_stats_test.go`.
  `TestAgentParameterRoutesUnderAgentGroupsAreDispositioned` requires the route to be registered through
  `bindAgentRoutes`.

### Fixed — `node scripts/lint-public-surface.mjs` exits 0 on the tracked tree (#838)

- On the commit this builds on, the lint exited 1 with one `local-home-path` finding at
  `apps/web/components/agents/detect-mcp-servers-dialog.test.tsx:109`: the mocked error of the test "keeps waiting
  when a read fails, and shows nothing from the failure" named a home directory path.
- That mocked error now names `/etc/mcp/servers.json`, and the test also checks that the dialog does not show that
  path.
- Tests: the lint reports 0 findings; `node --test scripts/test-lint-public-surface.mjs` reports 13 pass, 0 fail,
  where the commit this builds on reported 12 pass, 1 fail (the test that runs the lint on this repository's tracked
  tree). The six tests in `detect-mcp-servers-dialog.test.tsx` pass.

### Fixed — a negative offset on the agent audit log route is read as 0 (#834)

- `GET /api/v1/agents/:id/audit-logs` reads a negative `offset` as 0 and answers 200 with the first page. Before,
  `offset=-1` and `offset=-100` were answered 500 with a `runtime error: slice bounds out of range` body, measured
  from a member's session, from the agent's Ed25519-signed request and from its service token, with zero and with
  three records of the agent.
- `AuditService.GetAuditLogs` reads a negative offset as 0 before it queries by entity, by user or by organization.
  Before, a query by entity type and ID with offset -1 or -50 panicked, measured with zero, one and five records, and
  the user and organization queries were handed the negative offset.
- Tests: `apps/backend/internal/interfaces/http/handlers/agent_audit_logs_offset_test.go`,
  `TestAuditService_GetAuditLogs_ByEntityTypeAndID_NegativeOffsetReadsAsZero` and
  `TestAuditService_GetAuditLogs_NegativeOffsetReachesTheRepositoryAsZero` in
  `apps/backend/internal/application/audit_service_test.go`.

### Fixed — secrets resolve refuses a namespace or operation that holds a vertical bar, a carriage return or a line feed (#832)

- `POST /api/v1/secrets/resolve` verifies the agent's signature over `namespace|operation|nonce`, the fields joined
  with `|`, so one signed text stood for more than one set of field values: `git|hub|read|<nonce>` reads as namespace
  `git|hub` with operation `read`, and as namespace `git` with operation `hub|read`. Run against the handler before
  this change, a correctly signed request whose namespace or operation held `|`, a carriage return or a line feed
  (each of the three, in each field) passed the signature check and reached the nonce check.
- Such a request is now refused with `403` and
  `{"error":"namespace and operation must not contain a vertical bar (|), a carriage return or a line feed"}` before
  its signature is checked, so its nonce is not spent and no credential is read. The secrets audit entry records the
  refusal and names the field. The TypeScript and Python SDKs already refuse these three characters in either field
  before they sign.
- Tests: `TestResolve_FieldHoldingASeparatorIsRefusedBeforeTheNonceIsSpent` and
  `TestResolve_OneSignedTextIsNotAnsweredAsTwoSetsOfFields` in
  `apps/backend/internal/application/secrets_resolve_field_separator_test.go`, and
  `TestSecretsHandler_Resolve_FieldHoldingASeparatorIs403` in
  `apps/backend/internal/interfaces/http/handlers/secrets_resolve_handler_test.go`.

### Fixed — `POST /api/v1/secrets/namespaces` refuses a namespace name that the Python and TypeScript SDKs refuse to resolve (#830)

- A resolve request signs `namespace|operation|nonce`. `client.secrets.resolve()` in the Python SDK and
  `SecretsClient.resolve` in the TypeScript SDK refuse, before signing, a namespace that contains a line feed, a
  carriage return or a vertical bar (`|`), or that begins with the seven characters `DSSEv1 `. The route now answers
  `400` for such a name, and its `error` names the rule: `namespace must not contain a vertical bar (|): resolve signs
  namespace|operation|nonce` for `team|prod`, the same line naming a line feed or a carriage return for a name that
  holds one, and `namespace must not begin with "DSSEv1 ", the first bytes of a DSSE statement an agent key signs`.
  The agent is not looked up, and no namespace and no audit entry is written for the request.
- Before this change, the route answered `201` and stored the namespace for each of `team|prod`, `team`, a line feed
  and `prod`, `team`, a carriage return and `prod`, and
  `DSSEv1 46 application/vnd.opena2a.action-request.v1+json 2 {}`, names that neither SDK's `resolve` signs.
- A name that holds `DSSEv1` without the space, in lower case, or after its first character is stored as before.
- Tests: `apps/backend/internal/interfaces/http/handlers/secrets_namespace_name_test.go`, one refusal for each of the
  four, and five names that are stored.

### Fixed — `node scripts/lint-token-literals.mjs` exits 0 on the tracked tree (#829)

- On the commit this builds on, the lint exited 1 with six `credential-assignment` findings, each a test value in a
  credential-named field that the rule counts as generated: one in
  `apps/backend/cmd/bootstrap/aim_backend_script_test.go`, four in
  `apps/backend/internal/application/webhook_test_delivery_record_test.go` and one in
  `apps/backend/internal/interfaces/http/middleware/auth_session_age_test.go`.
- The webhooks in the webhook test carry the secret `not-a-real-secret`, the session age test signs its tokens with
  `test-only-jwt-secret-not-a-real-value` (37 characters; `NewJWTService` needs at least 32), and the case in which
  the server refuses the seeded admin's password sets the server's password to the seeded one with `not-` in front.
  `scripts/token-literal-allowlist.txt` is unchanged.
- Tests: the lint reports 0 findings, and the test of `scripts/test-lint-token-literals.mjs` that runs it on this
  repository's tracked tree, which failed on the commit this builds on, passes. The twelve tests in the three changed
  test files pass under `go test -race`.

### Fixed — the API reference says that a verification, a reactivation and an agent delete write an audit log entry (#826)

- `docs/API.md` said that this version of the server writes no audit record for a verification, a reactivation or a
  delete, and did not say that each of them writes an audit log entry, as the dashboard's API reference says for an
  addition of MCP servers. Measured with each route's handler over the audit service and the audit log repository on
  a sqlmock connection, with an agent service whose change succeeds, `POST /api/v1/agents/{id}/verify`,
  `POST /api/v1/agents/{id}/reactivate` and `DELETE /api/v1/agents/{id}` each write one `audit_logs` row for the
  agent. The reference now says that the server writes no audit record for them, only an audit log entry. The
  sentence of the dashboard's API reference for an approval of a capability request is unchanged.
- The entry for #740, and the earlier entries of this section that introduced the refusals of a verification, a
  reactivation and a delete, say the same.
- Tests: `apps/backend/internal/interfaces/http/handlers/agent_act_audit_log_entry_test.go` measures the `audit_logs`
  row of each of the three routes, and that a delete the agent service refuses writes none.
  `apps/backend/cmd/server/record_refusal_reference_test.go` now checks `docs/API.md` for the two new sentences; on
  the reference before this change it failed on both.

### Added — routes under `/api/v1/pam` for the capability tier registry, approvals of SUPER_PRIVILEGED capabilities, break-glass declarations and certification campaigns of an organization (#657)

- On the base commit no non-test Go file called `NewPAMService`, and a request to `/api/v1/pam/tiers` or any of the
  routes below was answered `404`. The server now builds the PAM service and mounts thirteen routes behind the
  dashboard session check: any signed-in user of an organization reads them; a member, manager or admin requests an
  approval; an admin decides one, declares, extends and revokes an emergency and creates a campaign; a manager or admin
  records a certification decision.
- An approval is requested for a capability the tier registry holds as SUPER_PRIVILEGED, for an agent of the caller's
  organization, and waits five minutes for a decision. The user who requested it cannot decide it (`403`); a decided
  or expired one is answered `409`. A break-glass declaration names agents of the organization and lasts at most 60
  minutes; an admin other than the declarer extends it once, by 15 minutes from its expiry, and an admin can revoke it
  while it is active. A certification decision is recorded while its campaign is open, for the agent and tier the
  campaign covers. An approval, declaration, campaign or agent of another organization is answered `404`. The
  verification and authorization routes do not read these records.
- Migration `141_pam_records_belong_to_an_organization.sql` adds `organization_id` to `emergency_declarations` and
  `certification_campaigns` (`ON DELETE CASCADE`) and `requested_by` to `pending_approvals` (`ON DELETE SET NULL`).
  The upgrade from 1.0.0 now applies 43 migrations; `docs/releases/platform-v1.1.0.md` carries its timing again.
- Tests: `apps/backend/internal/application/pam_service_test.go` (sqlmock),
  `apps/backend/internal/interfaces/http/handlers/pam_handler_test.go`, and
  `apps/backend/cmd/server/pam_routes_test.go`, which sends each route a request with no session and with a viewer,
  member, manager and admin session through the route table `mountAPIRoutes` builds; on the base commit all 65 of
  those requests were answered `404`.

### Fixed — the server refuses to start with a signing key the card attestation conformance vector publishes (#617)

`docs/specs/card-attestation-v2-vector.json` publishes two Ed25519 seeds so that anyone can reproduce its signatures, and so anyone can sign with them. The server now stops at startup when `AIM_SIGNING_KEY_CARD_ATTESTATION` or `AIM_SIGNING_KEY_ATC_ISSUER` holds either seed, or when `AIM_SIGNING_KEY_CARD_ATTESTATION_RETIRED` or `AIM_SIGNING_KEY_ATC_ISSUER_RETIRED` lists the public key of either seed, with an error that names the variable and not its value. A seed of your own loads as before.

### Added — `aim-server check-agent-keys` names the agents whose stored private key does not open (#617)

The startup conversion of stored agent private keys logs the rows it cannot read only as a count. `aim-server check-agent-keys` reads every stored agent private key with the `POSTGRES_*` and `KEYVAULT_MASTER_KEY` variables, writes nothing, and prints one line for each agent whose key does not open for its own row, `agent=<id> organization=<id> key=v1` or `key=unreadable`, then logs the counts and whether the next server start converts the `v1` keys or the agent's credentials have to be rotated. It refuses to run on a master key generated for the run. See [`docs/DEPLOYMENT.md`](docs/DEPLOYMENT.md).

### Added — shared request fixtures for three Python SDK calls (#617)

The bodies `register_mcp`, `report_execution_status` and `_register_single_mcp` send are in `sdk/python/tests/fixtures/sdk_requests/`. A Python test checks that the SDK sends exactly each body, and a Go test decodes each into the request type its handler binds with unknown members refused, and posts the `register_mcp` and `report_execution_status` bodies to their handlers.

### Fixed — the `/sdk-api`, `/detection`, `/agents` and `/a2a` routes admit a request an agent signed with ML-DSA or with both keys (#825)

- `AuthMiddleware` passes on a request the agent signature check authenticated with ML-DSA (`authenticated_via`
  `mldsa`) or with Ed25519 and ML-DSA together (`hybrid`), as it already did for one signed with Ed25519. Before, a
  request signed with `X-Algorithm` `ML-DSA-65` or `Ed25519+ML-DSA-65` was answered 401
  `{"error":"No authentication token provided"}` on `POST /api/v1/sdk-api/agents/:id/capabilities`,
  `POST /api/v1/detection/agents/:id/report`, `GET /api/v1/agents/:id` and `POST /api/v1/a2a/agents/:id/sign`, the
  routes measured, including from an agent in hybrid mode, which the agent signature check refuses when it signs with
  Ed25519 alone.
- The detection report handlers take a report the agent signed with ML-DSA or with both keys as they take one it
  signed with Ed25519. They serve `POST /api/v1/detection/agents/:id/report`,
  `POST /api/v1/detection/agents/:id/capabilities/report` and `POST /api/v1/sdk-api/agents/:id/detection/report`.
- Tests: `apps/backend/cmd/server/agent_signed_route_groups_auth_test.go`,
  `TestAuthMiddleware_SkipsIfAgentSignedWithAnyAlgorithm` in `apps/backend/internal/interfaces/http/middleware/auth_test.go`.

### Fixed — the session middleware records a revoked session's refused request at a credential route (#738)

- When the session middleware refuses an access token because its session (token family) is revoked, and the
  request was made at a credential route, the middleware records the refusal as the route's own check records the
  refusals it makes: one `credential_mint_refused` audit row and one `SECURITY credential_mint_refused` log line, with
  the route's label, the cause `sessionRevoked`, the session, the token's id, the user and the organization (the
  row's metadata holds `route`, `cause`, `familyId` and `jti`). The answer is unchanged: `401`
  `{"error":"Token has been revoked"}`.
- The credential routes are the 14 routes whose handler records `credential_mint_refused`, listed in
  `middleware.CredentialRoutes`: `GET /api/v1/sdk/download`, `POST /api/v1/oauth/device/approve`,
  `POST /api/v1/auth/sdk/recover`, `POST /api/v1/api-keys`, `GET /api/v1/agents/:id/sdk`,
  `GET /api/v1/agents/:id/credentials`, `POST /api/v1/agents/:id/rotate-credentials`, `PUT /api/v1/agents/:id/keys`,
  `POST /api/v1/secrets/namespaces/:id/rotate`, `POST /api/v1/webhooks`, `PUT /api/v1/webhooks/:id`,
  `POST /api/v1/webhooks/:id/rotate-secret`, `POST /api/v1/alert-channels` and `PUT /api/v1/alert-channels/:id`. The
  middleware and the handlers record each route under one label, taken from the same constant.
- The refusal is recorded only when an uncached read of the revocation store confirms the session's revocation, as
  the routes' own check requires. A token revoked by its own id while its session lives, a token that carries no
  session, and a token whose session the store cannot read (with the revoker failing closed) are refused as before
  and record nothing.
- Before this change, at each of the 14 routes a sign-in's access token whose session was revoked was answered `401`
  and no audit row was written (the new middleware tests, run against the previous middleware). In the backend's
  handler test, a download with the access token of an SDK token family, made right after a dashboard revoke of one
  of the family's tokens, was answered `401` and wrote no audit row and no `SECURITY` line.
- Tests: `apps/backend/internal/interfaces/http/middleware/auth_credential_route_refusal_test.go`;
  `apps/backend/cmd/server/credential_route_refusal_census_test.go`, which sends a revoked session's access token to
  each GET, POST, PUT, PATCH and DELETE route the server mounts under `/api/v1` and fails unless exactly the 14
  routes record the refusal, each under its own label;
  `apps/backend/internal/interfaces/http/handlers/credential_route_labels_census_test.go`, which fails unless each
  label in the table is named by exactly one handler's check and no check names a label outside it; and the dashboard
  revoke tests in `apps/backend/internal/interfaces/http/handlers/sdk_token_handler_family_test.go`, which now expect
  the row.

### Changed — an agent reads its own audit log with a request it signs or with its service token (#666)

- `GET /api/v1/agents/:id/audit-logs` answers a request an agent signs with its Ed25519 key, and one it sends with its
  OAuth service token, when `:id` is that agent's own ID. Neither credential carries a user ID, which the route
  required: it answered both `401` `{"error":"Unauthorized"}` for the agent's own ID, for another agent of its
  organization, for an agent of another organization and for an ID that names no agent.
- Such a request gets each record without `ipAddress`, `userAgent` and `metadata`, as every caller but an admin does,
  and its read is recorded in the audit log with the agent as the actor.
- Such a request that names another agent ID is answered `403` with `reasonCode` `agent_path_mismatch`, the answer of
  a route bound to its agent, before the named agent or any audit record is read.
- A user's session and an API key are answered as before.
- Tests: `apps/backend/internal/interfaces/http/handlers/agent_audit_logs_agent_caller_test.go`.

### Changed — `POST /api/v1/sdk-api/verifications` decides with the FGA engine's five steps, and an agent with no FGA policy is decided under a default policy (#656)

- The SDK verification route (`POST /api/v1/sdk-api/verifications`, and `POST /api/v1/verifications`, which shares its
  handler) decides with `FGAEngine.Authorize`. `AgentService.VerifyCapability` is step 1: the capabilities granted to
  the agent, the organization's enforcement mode and its security policies, with the violation record and the alert it
  writes. The engine then runs the attribute, context and chain checks of the agent's `fga_policies` row for the
  capability and dispatches the intent check for HIGH and MEDIUM risk. Before this change the route decided with
  `VerifyCapability` alone and emitted an `fga.authorize` span with one `fga.capability_check` child after the
  decision, without reading the agent's FGA policy.
- A call's `context` may name what it touches under `attributes` (a list of field names), `action` and `dataClass`,
  the members `POST /api/v1/agents/:id/authorize` takes in its body; the attribute check reads them. A denial by the
  attribute, context or chain check is enforced as the organization's mode enforces the route's other denials: in
  strict mode the call is denied with the check's reason (HTTP 403, `status` `denied`); in monitoring mode it runs,
  and the audit entry and the verification event record the denial under `fgaDeniedBy` and `fgaDeniedReason`. Both
  records carry `fgaOutcome` and `fgaStepsTriggered` for every decision. A policy row that cannot be read denies the
  call with `Verification error: failed to load FGA policy: ...` before step 1 runs.
- On both routes, an agent with no `fga_policies` row for the capability is decided under a default policy: no
  attribute, action or object restriction, no context rules and no chain rules, and the risk level detected from the
  capability, or the `riskLevel` an SDK verification names, with `critical` and `high` as HIGH. Steps 2, 3 and 4 run
  and allow, step 5 is dispatched for HIGH and MEDIUM and skipped for LOW, and the tool call and the attestation are
  recorded, as under a stored policy. Before, such an agent got step 1 only and no record. The `fga.authorize` span
  carries `fga.policy`, `stored` or `default`, and a step 1 denial sets `fga.denied_reason` and the error status on
  its child span.
- The READMEs, `docs/server-features.md`, `docs/DEPLOYMENT.md` and `apps/backend/docs/OBSERVABILITY.md` say so.
- Tests: `apps/backend/internal/interfaces/http/handlers/verification_fga_engine_test.go`,
  `apps/backend/internal/application/fga_engine_default_policy_test.go`,
  `sdk/python/tests/test_perform_action_authorization_claims.py`.

### Fixed — `GET /api/v1/agents/:id/sdk` answers for the language in `?language=` and carries the Python SDK's version (#792)

- The route reads the SDK language from `?language=`, the parameter the dashboard's agent download sends, and from
  `?lang=` when `?language=` is not given. Before this change it read `?lang=` alone: `?language=java`,
  `?language=rust`, `?language=nodejs` and `?language=go` each answered 200 with the Python package, and
  `?language=python&lang=java` answered 400. Now `?language=java` and `?language=rust` answer 400
  `Invalid language. Supported: python, nodejs, go`, and `?language=nodejs` and `?language=go` answer 501, as
  `?lang=nodejs` and `?lang=go` do.
- The Python package carries the version in `sdk/python/VERSION`, the version `GET /api/v1/sdk/download` names its
  Python archive after: in the archive's name (`aim-sdk-python-v<version>.zip`), `aim_sdk/__init__.py`, the
  `User-Agent` in `aim_sdk/client.py`, `aim_sdk/config.py`, `setup.py` and `README.md`. Each of them said `1.6.0`
  before. The server reads the file where it reads the SDKs `GET /api/v1/sdk/download` serves: under `SDK_BASE_DIR`
  when it is set, and otherwise under `./sdk`, where the server image copies the SDKs, or `../../sdk` when `./sdk`
  has no `python` folder. When the server cannot read the file, or the file holds anything other than a digit
  followed by up to 63 letters, digits, dots, plus signs and hyphens, the route answers 500
  `Failed to generate SDK` and sends no package.
- Tests: `apps/backend/internal/interfaces/http/handlers/agent_sdk_download_test.go`.

### Changed — the quick-start publishes the API and the dashboard on `127.0.0.1`, and the guide says `docker-compose.yml` publishes them on every interface (#792)

- `docker-compose.quickstart.yml` publishes the backend's host port (`AIM_BACKEND_PORT`, default 8080) and the
  dashboard's (`AIM_FRONTEND_PORT`, default 3000) on `127.0.0.1`, as it publishes PostgreSQL's and Redis's. Their
  `ports` entries named no host address before this change, and Docker publishes such a port on every interface of
  the host. To open the dashboard from another machine, change `127.0.0.1` to `0.0.0.0` in the two entries of the
  install's `docker-compose.quickstart.yml`, as the Docker section of `docs/DEPLOYMENT.md` now says.
- `docker-compose.yml` still publishes the two ports on every interface, and every other port it publishes on
  `127.0.0.1`. Comments at its `backend` and `frontend` ports and the Enable HTTPS section of `docs/DEPLOYMENT.md`
  say so, and say to write `127.0.0.1:` before the host port when the reverse proxy runs on the same host.
- Tests: `sdk/typescript/tests/quickstart-ports-and-project-from-environment.test.ts`.

### Fixed — the deployment guide names the withdrawn image tags the registries still serve, and the trust score texts say how the factors in `excludedFactors` count (#792)

- `docs/DEPLOYMENT.md` called the `aim-server` and `aim-dashboard` tags `1.5`, `1.5.0`, `1.23` and `1.23.0`, and the
  `aim-server` tags `1.22` and `1.22.0`, withdrawn, and put a `manifest unknown` pull down to the tag's removal from
  the registry. On 2026-10-21 Docker Hub answered `MANIFEST_UNKNOWN` for `1.22`, `1.22.0`, `1.23` and `1.23.0` and
  still served `1.5` and `1.5.0` of both images, and the GitHub Container Registry still served each of the withdrawn
  tags. The paragraph now says so and says not to run an image pulled by one of them.
- `excludedFactors` lists the factors with no data. The trust score's weighted mean leaves them out, and the score is
  that mean capped at the 9-factor sum in which each listed factor counts at its placeholder (0.5, or 0.3 for
  Execution Isolation), so when the cap applies the listed factors count in the score. `docs/sdk/trust-scoring.md`,
  `docs/API.md`, the server's comments and the Python SDK's `get_trust_score()` docstring said the listed factors
  were excluded from the composite or from the score; they now say how the factors count, and the trust scoring
  guide gives an example: an agent verified today whose verifications succeeded, with no compliance snapshot, no user
  feedback and no isolation attestation, has a mean of 0.881 over its seven measured factors and a published score of
  0.835. The calculation is unchanged.
- Tests: `apps/backend/internal/application/trust_score_excluded_factors_docs_test.go`.

### Added — a check that the root and `apps/web` manifests, the newest `CHANGELOG.md` release section and a `platform-v` tag name one platform version (#688)

- The root `package.json` holds the platform version. `node scripts/check-platform-version.mjs` exits 1, with one
  `refused:` line for each disagreement, when `apps/web/package.json` or the newest release section of `CHANGELOG.md`
  (the first `## [<version>]` heading other than `## [Unreleased]`, outside fenced code blocks) names another version,
  and exits 0 when they agree. Given a tag, it also exits 1 when the tag is not `platform-vX.Y.Z`, or when its X.Y.Z,
  as written, is not the root's version. On this tree, where both manifests read 1.0.0 and the newest release section
  is `[1.0.0]`, it exits 0 for `platform-v1.0.0` and 1 for `platform-v1.1.0`. It takes the tag from its argument and
  reads no environment variable. It exits 2 when a file cannot be read or parsed, a manifest carries no version, or
  `CHANGELOG.md` has no release section.
- The TypeScript SDK's test suite runs the check and its tests, `scripts/test-check-platform-version.mjs`, from
  `sdk/typescript/tests/platform-version.test.ts`. CI runs that suite on every pull request to `main` and every push
  to it, so a commit that opens a release section in `CHANGELOG.md` without setting both manifests to its version
  fails it.
- `CONTRIBUTING.md` names the root `package.json` as the platform's source of truth for its version, and its platform
  release steps run the check on the commit to be tagged, before the tag is pushed.

### Fixed — the API reference states the record refusal of a verification, a reactivation, an agent delete and a capability request approval only for a server that writes their audit record (#740)

- `docs/API.md` said that a verification, a reactivation and an agent delete, and the dashboard's API reference said
  that a capability request approval, are made only together with their audit record, and that the route answers
  `503` with `recordUnavailable` when the record cannot be written. The services make those changes together with an
  audit record only when a transition recorder is set on them, and no non-test Go file under `apps/backend` builds
  one with `transition.NewRecorder` or sets one with `SetTransitionRecorder`: this server makes the four changes
  without an audit record, writes only an audit log entry for a verification, a reactivation and a delete, and does
  not answer that `503`.
- The reference now states each refusal for a server that writes an audit record for the change, and says that this
  version of the server writes none and does not answer the refusal, as it said for an addition of MCP servers. The
  comments of `respondAgentStatusActError` and `ApproveCapabilityRequest`, and the three earlier entries of this
  section that introduced these refusals, state the same condition.
- Tests: `apps/backend/cmd/server/record_refusal_reference_test.go` holds the reference to the server's wiring. While
  no non-test Go file under `apps/backend` builds or sets a transition recorder, it fails when `docs/API.md` or
  `apps/web/lib/api-documentation.ts` lacks the statement that this server does not answer a record refusal, for the
  four changes and for an addition of MCP servers; once one is built or set, it fails while either still says so. It
  fails as well on any of the four sentences that promised a refusal without its condition. On the reference before
  this change it failed six of its eight checks.

### Fixed — a dashboard path whose percent-escape does not decode is answered with the not-found page, and the readability sweep answers the onboarding metrics (#683)

- `GET /dashboard/agents/%E0%A4%A`, `/dashboard/agents/%`, `/dashboard/agents/%ff` and `/dashboard/mcp/%E0%A4%A` were
  answered `500 Internal Server Error` as `text/plain`, with no `Content-Security-Policy` header. Next.js 16.3.6 throws
  a `DecodeError` when it decodes the `[id]` of such a path while it resolves the route, and looks for a Pages Router
  `/_error` page to answer it with, which the dashboard's build does not have. `next.config.ts` now returns a rule from
  `rewrites()` as `beforeFiles` (`apps/web/lib/undecodable-path.ts`) that sends a path under `/dashboard/` holding a
  `%` not followed by two hex digits, or escapes that are not the UTF-8 encoding of a character, to a path no page
  serves. Built with `next build` and served by the standalone server, those four paths, `/dashboard/mcp/%` and
  `/dashboard/mcp/%ff` are answered `404` with the not-found page and the `Content-Security-Policy` header, as
  `/dashboard/no-such-page` is, while `/dashboard/agents/abc` and `/dashboard/agents/%C3%A9` are answered `200` and
  `/api/v1/agents/%E0%A4%A` is relayed to the backend.
- The readability sweep (`apps/web/scripts/ux-sweep.mjs`) answers `GET /api/v1/platform-admin/onboarding-metrics` with
  the baseline the server computes when it has no organization to measure. The sweep answered that request with `{}`,
  the onboarding panel on `/dashboard/platform/onboarding` threw reading `medianSeconds`, and the sweep of that route
  recorded two console errors in each of its four variants and failed its gate. It now records none there and passes,
  with the empty and with the populated fixture.
- Tests: `apps/web/lib/undecodable-path.test.ts` checks the rule's pattern against `decodeURIComponent` on 123,008
  strings, every escape of one and two bytes among them. `apps/web/next-config.test.ts` builds the rewrites with the
  server's route code and checks that a page path whose dynamic segment does not decode is rewritten to a path no page
  serves, and that a page path that decodes is not. `apps/web/tests/ux-sweep.test.ts` checks the sweep's answer field by
  field against the server's answer with no organization to measure.

### Fixed — `GET /api/v1/agents/{id}/activity` lists an audit record whose address or user agent is NULL (#606)

- `audit_logs.ip_address` and `audit_logs.user_agent` are nullable columns, and the audit log repository read them
  into plain strings. Measured with the route's handler, the audit service and the audit repository over a sqlmock
  connection, the route answered `500` `{"error":"Failed to fetch agent activity"}` to an admin and to a viewer when
  one of the agent's rows held NULL in `ip_address`, in `user_agent`, or in both.
- `GetByID`, `GetByOrganization`, `GetByUser`, `GetByAgent`, `GetByResource`, `Search`, `GetRecentActionsByAgent` and
  `GetAgentActionsByIPAddress` now read both columns through `sql.NullString` and return a NULL as an empty string.
  The route answers `200`, and a record whose column is NULL carries no `ipAddress` or `userAgent` member, as a
  record whose column is empty carries none.
- Tests: `apps/backend/internal/interfaces/http/handlers/agent_activity_null_request_detail_test.go` calls the route's
  handler for each of the three NULL cases, and for a row holding empty strings, as an admin and as a viewer.
  `apps/backend/internal/infrastructure/repository/audit_log_null_request_detail_test.go` gives each of the eight
  reads a row with NULL in one or both columns. On the commit this builds on, the six NULL requests were answered
  `500`, the two empty-string requests `200`, and the eight reads returned an error ending in `converting NULL to
  string is unsupported` in each of the 22 cases.

### Fixed — the MCP attestation challenge and attest routes answer an unauthenticated request in AuthMiddleware

- `GET /api/v1/mcp-servers/:id/challenge` and `POST /api/v1/mcp-servers/:id/attest` answer a request that the agent
  signature check did not authenticate in `AuthMiddleware`. A request without credentials, or with `X-Agent-ID` and
  no `X-Timestamp`, is answered 401 `{"error":"No authentication token provided"}`, and a bearer value that is not a
  token 401 `{"error":"Invalid or expired token"}`. Before, these requests reached the handler, which answered 401
  only because no organization was set; a signed-in user's session got the same 401, and now reaches the handler with
  its organization. A request that the agent signature check authenticated, signed with Ed25519, ML-DSA or both keys,
  reaches the handler without passing through `AuthMiddleware`.
- Tests: `apps/backend/cmd/server/mcp_attestation_routes_auth_test.go`.

### Added — the card attestation conformance vector exercises the alg, issuer, purpose and expiry checks

- `docs/specs/card-attestation-v2-vector.json` publishes an ATC-issuer key (seed `inputs.atcIssuerSeed`) beside the
  card-attestation key and holds twelve cases. Three are each decided by one check and carry a signature valid over
  their own payload: a card attestation signed by the ATC-issuer key (`unknown-key`), `alg` `ES256`
  (`unsupported-format`) and issuer `aim-server.example` (`malformed`). Two carry `now`: one second before
  `expiresAt` (`verified`) and at `expiresAt` (`expired`).
- `verify-card-attestation.mjs` compares a vector case that carries `now` with that time, through the same
  `isExpired` its two-file mode uses. Step 7 of `card-attestation-v2.md` names the result `expired`.
- `TestCardAttestationConformanceVectorIsWhatTheServerSigns` compares the vector's whole JWK Set with the one the
  server publishes for the two seeds, and `TestCardAttestationConformanceVectorCheckCasesCarryAValidSignature` checks
  the signatures of the three cases decided by one check.

### Changed — the response census follows an error's text into a variable of another name

- `TestResponsesCarryNoErrorText` also fails on a response body that names a local variable assigned an error value
  or its text (`detail := err.Error()`, `reason = fmt.Sprintf("...: %v", lookupErr)`, `cause := err`), followed
  through further assignments in the same file. A variable assigned the result of another function that receives
  the error, such as `resolveRefusalLine(err)`, is not followed. With a probe file in the handlers package holding a
  500 body built from `message := "lookup failed: " + detail` after `detail := err.Error()`, the census on the base
  commit reported the probe's other body only, and now reports both. The handlers hold no such body: 2216 response
  bodies, none built from an error value.

### Changed — tests cover the ten domain functions that had none

- The `internal/domain` function average that the CI coverage step computes rose from 90.6% to 98.9% over 126
  functions (`go test -race -covermode=atomic ./internal/domain/...`, averaged as the step does; its minimum is 90%).

### Changed — the direct API guide and the API quickstart register an agent with camelCase body keys

- The `POST /api/v1/agents` examples in `docs/api/direct-api-usage.md` send `displayName`, `agentType` and `publicKey`
  in place of `display_name`, `agent_type` and `public_key`, and its request body and Go request type send
  `repositoryUrl` and `documentationUrl` in place of `repository_url` and `documentation_url`. The examples in
  `docs/tutorials/api-quickstart.md` send `displayName` and `agentType`. These are the member names of the create
  request, and the spelling `docs/API.md` uses.
- The API quickstart's `POST /api/v1/mcp-servers` example sends the server's address as `url`, the member the route's
  request binds, in place of `server_url`, and leaves out `display_name`, which that request has no member for.
- A test reads the body keys of the three pages' agent registration examples against the member names of the create
  request, and the curl bodies the guide posts to the agent route and the quickstart posts to the agent and MCP server
  routes against the member names of each route's request type.

### Added — agents read credentials from a CyberArk vault through the Central Credential Provider, turned on by `CYBERARK_CCP_URL` (#657)

- On the base commit no non-test Go file imported `internal/integrations/cyberark`. With `CYBERARK_CCP_URL`,
  `CYBERARK_CCP_APP_ID` and `CYBERARK_CCP_SAFE` set, the server now registers a `cyberark_ccp` secrets backend and logs
  `CyberArk CCP backend registered at <url> for safe "<safe>"`. `CYBERARK_CCP_CERT_PATH` and `CYBERARK_CCP_KEY_PATH`
  set a PEM client certificate and its key, and `CYBERARK_CCP_CA_CERT_PATH` a PEM bundle that verifies the CCP server.
  With no application ID or safe, a URL that is not `https`, or an application ID or safe holding `+`, `&`, `%` or `;`,
  the server logs `WARNING: CyberArk CCP backend configured but failed to initialize: <reason>` and starts without the
  backend. The environment table records the setting and startup line of each variable for unset, empty, whitespace,
  valid and invalid values.
- A secrets namespace created with `"backendType": "cyberark_ccp"` is held by the account named
  `aim-secrets-<namespace id>` in that safe. `POST /api/v1/secrets/resolve` sends CCP one
  `GET /AIMWebService/api/Accounts` naming `AppID`, `Safe` and, as `Object`, that account, and answers with the
  account's content encrypted to the agent's key. On such a namespace, `POST /api/v1/secrets/namespaces/:id/credentials`
  and `POST /api/v1/secrets/namespaces/:id/rotate` answer `409` with `the namespace's backend is read-only: store and
  rotate its credential in the backend`, send nothing to CCP and write no audit entry; deleting the namespace sends
  nothing to CCP. A resolve request is answered `403 Secret resolution refused` when the server runs without the
  backend (deny reason `backend not available: cyberark_ccp`) and when CCP answers 404 or 500 (deny reason
  `credential retrieval failed`).
- `cyberark.CCPRequest.Validate` returns the error `RetrieveCredential` returns for the same request before it sends
  anything; the backend calls it at startup with its application ID and safe.
- Sending session events to Privileged Session Manager stays under evaluation, and the PAM service is not wired into
  the server yet.
- Tests: `apps/backend/internal/infrastructure/secrets/cyberark_ccp_server_test.go`,
  `apps/backend/internal/interfaces/http/handlers/secrets_cyberark_ccp_test.go` and
  `apps/backend/internal/integrations/cyberark/connector_test.go` (`TestCCPRequestValidate`) against the CCP stub in
  `ccptest`, which serves TLS and requires a client certificate signed by its own CA;
  `TestSecretsService_CyberArkCCPRoundTrip` in `apps/backend/internal/application/secrets_external_backends_integration_test.go`
  (integration tag) with the namespace, agent, audit and nonce repositories on PostgreSQL 16.

### Added: conformance vectors for `agent-request-v2` statements

- `docs/specs/agent-request-v2-vectors.json` holds 33 requests signed as `agent-request-v2` statements, each with the
  answer `AgentRequestV2Middleware` gives it: the HTTP status, the `reasonCode` of a refusal, and whether the request
  leaves its nonce stored. Each request is written as sent (method, target, body, statement, `X-Agent-Request-Signature`
  lines and other headers), so another implementation can check its signer or verifier against the same bytes. The
  file's `description` states the format the vectors follow.
- 8 requests are accepted: Ed25519 alone on a GET with a query and on a POST with a body, ML-DSA-65 alone, Ed25519 and
  ML-DSA-65 from an agent in hybrid mode on two header lines and on one comma-separated line, timestamps 30 seconds
  behind and 30 seconds ahead of the server clock, and a statement with line breaks, indentation and its members in
  another order.
- 25 are refused:
  - `timestampOutsideWindow`: timestamps 31 seconds behind and 31 seconds ahead of the server clock.
  - `nonceReused`: the first accepted request sent again with its nonce already stored.
  - `invalidRequest`: nonces of 15 and of 17 bytes, an `agent-request-v1` signed text sent as the statement, a member
    the format does not define, and three signatures. `nonceRequired`: a statement with no nonce.
  - `audienceMismatch`: an audience of another origin, and one that differs from the server's only in letter case.
  - `requestMismatch`: a body without `body_sha256`, `body_sha256` without a body, the digest of other bytes, a query
    in another order than the one sent, and a method other than the one sent.
  - `conflictingAuthentication`: `X-Agent-ID`, `X-Timestamp` and `X-Signature` beside the statement, and
    `Authorization` beside it.
  - `hybridSignatureRequired`: an agent in hybrid mode signing with Ed25519 alone. `singleSignatureRequired`: an agent
    not in hybrid mode sending two signatures.
  - `agentKeyNotRecognized`: an ML-DSA-65 signature for an agent with no ML-DSA-65 key, and an `agent_id` of no
    registered agent. `signatureInvalid`: a signature under the `action-request-v1` payload type, and one over the
    statement without its encoding.
  - `agentStatusDenied`: a suspended agent. This is the one refusal that leaves its nonce stored, because the agent's
    status is read after admission.
- The keys of the four test agents are derived from labels written in the file (each seed is the SHA-256 of its
  label), and the ML-DSA-65 signatures are deterministic, so the file can be made again byte for byte.
- `TestAgentRequestV2VectorsAreCurrent` holds the file to what the test builds; from `apps/backend`,
  `go test ./internal/interfaces/http/middleware -run TestAgentRequestV2VectorsAreCurrent -update` rewrites it.
  `TestAgentRequestV2Vectors` sends each request through `AgentRequestV2Middleware` and checks the status and
  `reasonCode`, that a request answered 200 reaches the route handler and a refused one does not, the nonce rows
  stored, and that the admission store is called for the accepted requests and the window, nonce and status refusals
  and for no other. With the expected answer of the 31-seconds-behind vector changed to 200 in the file, both tests
  failed.
- No route mounts `AgentRequestV2Middleware` yet, so no route answers these requests.
- Tests: `apps/backend/internal/interfaces/http/middleware/agent_request_v2_vectors_test.go`.

### Added: `AIM_PUBLIC_ORIGIN` holds the audience an `agent-request-v2` statement must name, and the server logs it at startup

- `AIM_PUBLIC_ORIGIN` is the deployment's public origin, the audience `AgentRequestV2Middleware` requires an
  `agent-request-v2` statement to name, compared byte for byte. `config.Load` reads it into `ServerConfig.PublicOrigin`
  and keeps it as written: it does not trim it, lowercase it or check its form, so the environment table holds
  `"   "` and `"https://AIM.example.com/"` as they were set. Unset and empty both leave it empty.
- No route mounts `AgentRequestV2Middleware` yet, so the setting changes no response.
- The server logs one startup line for it,
  `Agent requests signed as agent-request-v2: no route accepts them yet (AIM_PUBLIC_ORIGIN, the audience they must name, is not set)`,
  which ends `is "https://aim.example.com")` in place of `is not set)` when it is set to `https://aim.example.com`. The
  value is quoted as Go quotes a string, so its case, its spaces and its path show, and a line feed or a carriage
  return in it is escaped.
- A test reads the backend's Go files outside the tests and holds the line to them: the line says no route accepts
  these requests while no file uses `AgentRequestV2Middleware`, and the test asserts it stops saying so once one does.
  As a control the same reading must find `PQCAgentMiddleware`; it finds its 6 uses, the six mounts in
  `cmd/server/main.go`. On planted source it reports each use of the middleware at its line, and neither its
  declaration nor a comment naming it.
- The environment table gains five `AIM_PUBLIC_ORIGIN` rows, for unset, empty, whitespace, `https://aim.example.com`
  and `https://AIM.example.com/`, each with the setting and the line; no other row changes.
- Tests: `apps/backend/cmd/server/agent_request_audience_test.go` and the environment table tests of
  `apps/backend/internal/config` and `apps/backend/cmd/server`.

### Added: `/metrics` counts the `agent-request-v2` requests the server refuses, by reason and calling SDK

- `aim_agent_request_v2_refusals_total` counts each request carrying an `agent-request-v2` statement that
  `AgentRequestV2Middleware` refuses, by `reason`, the kind of refusal, and by `sdk`, the calling SDK derived from the
  `User-Agent` header as for `aim_s1_refusals_total`. The middleware counts a refusal in the one place it answers a
  statement it refuses, before it writes the response, and moves no `aim_s1_refusals_total` series.
- `reason` takes 21 values: `audience_not_configured`, `conflicting_authentication`, `statement_malformed`,
  `nonce_missing`, `signature_header_malformed`, `audience_mismatch`, `method_mismatch`, `target_mismatch`,
  `body_mismatch`, `agent_lookup_failed`, `no_registered_key`, `registered_key_malformed`,
  `signature_invalid_ed25519`, `signature_invalid_mldsa`, `hybrid_required`, `single_signature_required`,
  `skew_past`, `skew_future`, `nonce_reused`, `freshness_check_unavailable` and `agent_status_denied`. A value
  outside this set is counted as `unclassified`, which the middleware does not pass.
- The responses are unchanged. Where one `reasonCode` answers several causes (`invalidRequest`, `requestMismatch`,
  `agentKeyNotRecognized`, `signatureInvalid`, `timestampOutsideWindow`), `reason` tells them apart for operators.
  `apps/backend/docs/OBSERVABILITY.md` lists each reason with the response it counts.
- No route mounts `AgentRequestV2Middleware`, so no request is counted here yet.
- Tests: `apps/backend/internal/infrastructure/metrics/agent_request_v2_refusals_test.go` and
  `apps/backend/internal/interfaces/http/middleware/agent_request_v2_refusal_count_test.go`, which sends 37 refused
  requests, at least one for each of the 21 reasons, and reads both refusal families from `/metrics` before and after
  each, and checks the middleware's source: its handler answers a statement only by passing the request on or
  through the counting refusal, and every refusal names a reason of the set.

### Added: `/metrics` counts the requests each agent signing scheme authenticates

- `aim_agent_requests_authenticated_total` counts the requests an agent's signature authenticated, by `type`, the
  scheme the request was signed in, and by `sdk`, the calling SDK, derived from the `User-Agent` header as for
  `aim_s1_refusals_total` (`python`, `typescript`, `java` or `other`).
- `type` is `agent-request-v1`, counted by `PQCAgentMiddleware` whichever algorithm verified the request and by
  `Ed25519AgentMiddleware`; `agent-request-v2`, counted by `AgentRequestV2Middleware`; `verification-read`, the
  `X-AIM-Agent-ID`, `X-AIM-Timestamp` and `X-AIM-Signature` headers of `GET /api/v1/sdk-api/verifications/{id}`;
  `verification-body`, a `POST /api/v1/verifications` or `POST /api/v1/sdk-api/verifications` body without
  `signedBytes`; or `action-request-v1`, a body of those two routes with `signedBytes`. A value outside this set is
  counted as `unclassified`.
- A request is counted where its authenticator accepts it, after the agent's status check. A request the
  authenticator refuses is not counted, nor is a request `PQCAgentMiddleware` or `Ed25519AgentMiddleware` passes to
  the next handler because it carries `Authorization`, or one `AgentRequestV2Middleware` passes on because it carries
  no `X-Agent-Request`.
- The series of the five schemes for the four `sdk` values are served from start at 0, so a scrape lists a scheme
  that has authenticated no request. No route mounts `AgentRequestV2Middleware`, so its series stay at 0.
- Tests, each scheme with accepted and refused requests:
  `apps/backend/internal/infrastructure/metrics/agent_requests_authenticated_test.go`,
  `apps/backend/internal/interfaces/http/middleware/agent_request_authenticated_count_test.go` and
  `apps/backend/internal/interfaces/http/handlers/verification_authenticated_count_test.go`.

### Added — an authenticator for requests signed as `agent-request-v2` statements, which no route uses yet

- `AgentRequestV2Middleware` authenticates a request that carries `X-Agent-Request`, the unpadded base64url of a
  JSON payload with exactly the members `method`, `target`, `audience`, `agent_id`, `timestamp` (RFC 3339 in UTC
  with `Z`) and `nonce` (16 bytes, unpadded base64url), and `body_sha256` (the unpadded base64url SHA-256 of the
  body as received) exactly when the request has a body. `X-Agent-Request-Signature` carries each signature as an
  algorithm (`Ed25519`, `ML-DSA-44`, `ML-DSA-65` or `ML-DSA-87`), a colon and the unpadded base64url signature over
  the DSSE v1 pre-authentication encoding of the payload type `application/vnd.opena2a.agent-request.v2+json` and
  the payload as received.
- The payload must name the request it arrives on: its method, its path and query as sent, the body, and an
  `audience` equal byte for byte to the public origin the middleware is given. The signatures are verified against
  the agent's registered keys; the request presents no key. An agent in hybrid mode needs one Ed25519 and one
  ML-DSA signature, as two header lines or one comma-separated line, and any other agent exactly one signature.
- After the signatures verify and before the agent's status is read, the middleware passes the agent, the stored
  agent's organization, the nonce and the timestamp to one `Admit` call of the admitter it is given, an interface
  `application.ActionRequestNonceService` implements. The middleware writes nothing itself, so a request it
  refuses before that call stores no nonce. A request from a suspended or revoked agent whose signatures verify
  spends its nonce.
- Refusals carry a `reasonCode`: 400 `invalidRequest`, `nonceRequired`, or `conflictingAuthentication` when the
  request also carries `Authorization`, `X-API-Key` or a header the other agent signature checks read; 401
  `audienceMismatch`, `requestMismatch`, `agentKeyNotRecognized`, `signatureInvalid`, `hybridSignatureRequired`,
  `singleSignatureRequired`, `timestampOutsideWindow`, `nonceReused` or `agentStatusDenied`; 503
  `audienceNotConfigured` when the middleware is given no public origin, and `freshnessCheckUnavailable` when the
  admission cannot run. A refused request does not reach the next handler. A request without `X-Agent-Request`
  goes to the next handler untouched.
- No route mounts the middleware, and no setting supplies its public origin.
- Tests: `apps/backend/internal/interfaces/http/middleware/agent_request_v2_test.go`,
  `apps/backend/internal/domain/agent_request_test.go`, and the agent-status read census in
  `apps/backend/internal/agentauth/architecture_test.go`.

### Fixed: the Quick start signs the release of the Python SDK on PyPI in to a self-hosted AIM with the sign-in from the dashboard's SDK download (#789)

- aim-sdk 2.0.3, the release on PyPI, cannot complete the Quick start's `aim-sdk login --url http://localhost:8080`:
  it opens `<url>/auth/login` and exchanges a code at `/api/v1/auth/token`, and the server registers neither. Run
  without a terminal against a server built from this tree, it printed nothing for 20 seconds while the server
  answered its `GET /auth/login` with `404`. Its `secure()` signs in with the sign-in in
  `~/.aim/sdk_credentials.json`, the first file it reads one from, as this repository's SDK does.
- `docs/quick-start.md` now titles the section Sign in from your dashboard (it was Install the SDK from your
  dashboard). After Download Python SDK it unzips the newest archive, runs
  `mkdir -p ~/.aim && cp aim-sdk-python/.aim/sdk_credentials.json ~/.aim/sdk_credentials.json` and
  `chmod 600 ~/.aim/sdk_credentials.json`, and keeps the release on PyPI. Installing the archive's SDK with
  `pip install -e ".[rich]"` is the alternative, in a fence of its own. The README's Quick start links the section
  as the self-hosted sign-in. With aim-sdk 2.0.3 from PyPI and those commands run as written, against a server
  built from this tree on a database migrated and seeded with `aim-bootstrap --default`, the Quick start ran to its
  end: `secure("my-first-agent", capabilities=["db:read"])` registered the agent pending at 59%, the `db:read` call
  returned the record once an administrator had verified the agent, and in strict mode the `db:write` call was
  refused with the reason the README quotes.
- The server is unchanged: it already serves every request that release sends for the Quick start. The release on
  PyPI prints a sign-in code at `aim-sdk login` once this repository's SDK is published.
- Tests: `apps/backend/cmd/server/readme_quick_start_pypi_release_integration_test.go` builds the server as `main`
  does over `TEST_DATABASE_URL` and downloads the Python SDK as a signed-in administrator. It takes the archive
  member the docs copy, then sends the requests aim-sdk 2.0.3 sends for the Quick start: the refresh, the existence
  check, the registration with a key pair of its own, the Ed25519-signed capability registrations and the
  verifications, approved for `db:read` and refused for `db:write` in strict mode.
  `sdk/python/tests/test_readme_self_hosted_sign_in.py` holds the section's first fence to the copy and the
  `chmod`, with no install, and checks that the SDK reads `~/.aim/sdk_credentials.json` before the archive's copy.
  On the previous docs both failed, finding no section titled Sign in from your dashboard; the section it replaces
  copies no sign-in.

### Fixed — the account deletion purge removes the OpenID Connect sign-in configuration of an organization it deletes

- An organization's OpenID Connect sign-in configuration, sealed client secret included, is a `system_config` row
  keyed `oidc_sign_in:<organization id>`, and `system_config` has no reference to organizations, so no cascade
  removes it. When the purge deletes an organization it leaves with no user, it now deletes that row in the same
  transaction. The configuration of an organization the purge keeps, because a user is left or because a reference
  refused the delete, stays.
- Tests: `apps/backend/internal/infrastructure/repository/user_deletion_oidc_sign_in_integration_test.go` stores a
  configuration for three organizations and runs the purge against PostgreSQL. On the commit this builds on, the
  configuration of the organization whose one user was purged was still there after the organization was deleted.

### Changed: `trust_score.changed` is sent for the scores an agent's registration, update, verification, suspension, reactivation, revocation and MCP server changes store

- The agent service stores a trust score when it registers an agent, and a recalculated one after
  `PUT /api/v1/agents/:id`, `POST /api/v1/agents/:id/verify`, `POST /api/v1/agents/:id/suspend`,
  `POST /api/v1/agents/:id/reactivate`, `POST /api/v1/agents/:id/revoke`, `PUT /api/v1/agents/:id/mcp-servers` and
  `DELETE /api/v1/agents/:id/mcp-servers/:mcp_id`; `POST /api/v1/agents/:id/mcp-servers/detect` adds the servers it
  maps through the same path as `PUT /api/v1/agents/:id/mcp-servers`. Before this change none of these sent
  `trust_score.changed`.
- Each of these recalculations now sends `trust_score.changed` when the score was stored, by the agent's row or by
  its trust score history row, whose insert sets `agents.trust_score`, and differs from the agent's score before at
  the two decimal places `agents.trust_score` keeps. The event carries both scores at that precision. The trust
  score policy does not run after them, so they send no `trust_score.low`.
- A registration sends `trust_score.changed` once, after its last write of the score, with `previousScore` 0 and the
  last score it stored: for an agent verified at registration, the score recalculated after the verification. It is
  sent whatever the score, as the 0.5 a new agent is given before its first score is not a previous score. A
  registration that stores no score sends no event.
- `docs/API.md` and the webhook dialog's description of the event say when it is sent; the score
  `POST /api/v1/detection/agents/:id/capabilities/report` stores still sends no event. Tests:
  `apps/backend/internal/application/agent_trust_score_senders_test.go`. Its registration test failed 5 of its 7
  cases and its recalculation test all 7 when run against the agent service before this change, each with no event
  dispatched. The registration and lifecycle tests in `webhook_events_test.go` now expect the event, and
  `agent_trust_score_registration_integration_test.go` covers a registration through the transition recorder when
  `TEST_DATABASE_URL` is set.

### Changed: `trust_score.changed` is sent for the scores a trust score calculation, a capability grant or revocation, and a drift penalty store

- The trust calculator, the capability service and the drift detection service store trust scores of their own and
  had no webhook service to send through. On the commit before this change, `POST /api/v1/agents/:id/trust-score/recalculate`,
  `POST /api/v1/trust-score/calculate/:id`, the recalculation after a capability is granted or revoked, and the drift
  penalty for a verification event that reports an MCP server the agent is not registered to talk to stored a new
  score and sent no event.
- The server now gives the three its webhook service. Each sends `trust_score.changed` when it has stored a score
  that differs from the agent's at the two decimal places `agents.trust_score` keeps, and the event carries both
  scores at that precision. A calculation counts as stored once both of its writes, the trust score history row and
  the agent's score, succeeded, and a drift penalty once the agent's score was written. After a grant or a revocation
  the score counts as stored when the agent's row or its history row was written, as the history insert sets
  `agents.trust_score`. The trust score policy does not run after these changes, so none of them sends
  `trust_score.low`.
- `docs/API.md` and the events offered in the dashboard's webhook dialog say when the two events are sent. Tests:
  `apps/backend/internal/application/trust_score_senders_test.go` fails to build on the commit before this change,
  where the three services have no webhook service, and `TestInitServices_SetsTheWebhookServiceOnTheEventServices`
  in `apps/backend/cmd/server/webhook_wiring_test.go` fails there with three of the six services given the webhook
  service.

### Fixed: HTTP request metrics count a request a route served under that route's path, so the values a caller puts in a path add no series

- `PrometheusMiddleware` labelled a request a route handler served with the path the caller sent, after replacing
  each segment of digits and each 36-character segment holding four hyphens with `:id`. A value of another shape in a
  path parameter or a wildcard became a new `path` value on `aim_http_requests_total` and
  `aim_http_request_duration_seconds`, and so did a request that a middleware on the route's own registration line
  refused, such as the rate limiter of `GET /api/v1/did/*`, a route that takes no credentials. In
  `prometheus_middleware_test.go`, 100 requests with distinct values on four such routes added 100 series to the
  registry before this change; with it they add none.
- The `path` label is now the route's path as the server registers it, group prefix included, for example
  `/api/v1/agents/:id/trust-score` or `/api/v1/did/*`. A request no route handler served keeps the label `unmatched`:
  an unknown path, a method the path does not accept, or a request a group middleware refused.
- A query or alert on a served request's own path, such as a `path` holding a DID under `/api/v1/did/`, receives no
  new samples and needs the route's path. A `:id` the replacement wrote stays only where the route names its
  parameter `:id`: requests to `POST /api/v1/agents/:id/log-capability/:audit_id` with two UUIDs were counted under
  `/api/v1/agents/:id/log-capability/:id` and are now counted under the route's path.
- `docs/DEPLOYMENT.md` describes the labels under Prometheus metric names. Tests: in `prometheus_middleware_test.go`,
  one test fails unless a request to a path-parameter, wildcard, two-parameter group or rate-limited route is counted
  under that route's path and under no other series, and another fails unless 100 requests with distinct path values
  add no series and none of the values appears in the body `/metrics` serves; both fail on the middleware before this
  change. A third sends 600 mixed-method requests, 400 of them concurrently, and fails unless the 600 are counted
  under the route with valid method labels; it passes before this change as well.

### Added: The server runs a capability drift monitor and a trust re-evaluation job

- The capability drift monitor runs every 15 minutes by default (`DRIFT_MONITOR_INTERVAL`, a Go duration, at
  least one minute). A run reads the MCP capability drift of every organization with a registered MCP server over
  the last day, the read `GET /api/v1/supply-chain/drift-alerts` makes, which records an alert for a drift that
  has no unacknowledged one, and sends `security.capability_drift` to the organization's webhooks once for each
  drift (server, capability and drift type) while the server runs. A drift whose send failed is sent on the next
  run. On the commit before this change, no code sent that event.
- The trust re-evaluation job runs every hour by default (`TRUST_REEVALUATION_INTERVAL`, at least five minutes).
  It recalculates the score of every agent that is not revoked and, for a score that moved by 0.001 or more since
  the last one on record, writes the trust score history row and the agent's score, as
  `POST /api/v1/agents/:id/trust-score/recalculate` does, then sends `trust_score.changed` when the agent's
  score changed at the two decimal places it is stored at. The job does not run the trust score policy.
- A value of either variable that does not parse, or is not positive, uses the default; a value under the minimum
  runs at the minimum. Both log a `WARNING:` line at startup naming the variable and the interval in effect.
  Stopping the server ends a run before its next organization or agent.
- `docs/API.md`, `docs/guides/A2A.md`, `docs/server-features.md` and `README.md` say when the two jobs run and what
  they send. Tests: `apps/backend/cmd/server/drift_monitor_test.go`,
  `apps/backend/cmd/server/trust_reevaluation_test.go`, the two variables' rows in
  `apps/backend/internal/config/testdata/environment_table.json`, and the `NotifyTrustScoreChanged` tests in
  `apps/backend/internal/application/webhook_events_test.go`.

### Added: an organization's users can sign in through the organization's OpenID Connect provider

- An administrator stores the organization's provider with `PUT /api/v1/admin/oidc-sign-in` (issuer, client id and
  client secret), reads it with `GET` and removes it with `DELETE` on the same path. Before it stores anything, the
  server reads the issuer's configuration document, and it refuses an issuer that is not an `https` URL, or whose
  document cannot be read or names another issuer. The configuration is a row of the `system_config` table keyed by
  the organization's id; this adds no migration. The client secret is stored encrypted with the key vault's master
  key and bound to the organization, and no answer or audit row carries it.
- `POST /api/v1/auth/oidc/{organizationId}/login` answers the provider's authorization URL (authorization code flow,
  PKCE `S256`, a state and a nonce) and a flow value that holds the state, the nonce and the code verifier encrypted
  and bound to the organization. `POST /api/v1/auth/oidc/callback` takes the code, the state and the flow, redeems the
  code, checks the ID token's signature with the provider's key set and its issuer, audience, `azp`, `exp`, `iat`,
  `nbf` and nonce, and answers a login pair, as `POST /api/v1/auth/login/local` does, for the user of that
  organization whose email the token asserts as verified. Only the users of that organization are compared, and without
  provisioning (the next entry) the sign-in creates no user. A refused sign-in answers `401` with a `code` that names
  the check; `docs/API.md` lists them. Both routes are behind the credential routes' rate limiter.
- The provider redirects the browser to the `/auth/oidc/callback` page under `FRONTEND_URL`, which the dashboard
  serves (see the entry on the dashboard's sign-in below).
- Tests: `apps/backend/internal/interfaces/http/handlers/oidc_sign_in_handler_test.go` drives the login and callback
  routes against a stub provider listening on 127.0.0.1 (`apps/backend/internal/infrastructure/oauth/oidctest`) and
  ends with a pair for a user of the configured organization. Through the same routes it refuses an ID token with a
  wrong issuer, audience or nonce, a bad signature or an `exp` in the past, and shows that the provider of one
  organization, asserting the email of a user of another, signs in no one and changes no user.
  `apps/backend/internal/application/oidc_sign_in_service_test.go` holds the flow's binding to its organization and
  its 10-minute lifetime, and `apps/backend/internal/infrastructure/oauth/oidc_provider_test.go` each ID token check.

### Added: an organization's OpenID Connect sign-in can create the user at the first sign-in

- `PUT /api/v1/admin/oidc-sign-in` takes an optional `provisioning` member: `enabled`, a `role` (`viewer`, `member` or
  `manager`, and `viewer` when it is left out; `admin` is refused) and 1 to 20 `emailDomains`. `GET` answers it, and
  the audit row of a stored configuration names it. A body without it stores a configuration that provisions no user.
  Provisioning is kept in the configuration's `system_config` row; this adds no migration.
- With provisioning enabled, a sign-in whose verified email no user of the organization has creates a user of that
  organization and answers a login pair for it. The user has the email in lower case, the ID token's name, the
  configured role, the status `active`, the provider `oidc` and no password. The email must be an address at one of
  the configured domains exactly, and no user of another organization, deleted or not, may have it; otherwise the
  callback answers `401` with the code `domain_not_provisioned` or `email_in_other_organization` and creates nothing.
  The new user is recorded in the audit log as a `create` of the resource type `user`, before the login.
- Tests: `apps/backend/internal/application/oidc_sign_in_provisioning_test.go` provisions a user of the configured
  organization and signs the same user in at the next sign-in, refuses addresses at other domains and at subdomains,
  never provisions the email of another organization's user, and ends as the user a concurrent first sign-in
  created. `apps/backend/internal/interfaces/http/handlers/oidc_sign_in_handler_provisioning_test.go` drives the
  administration, login and callback routes against the stub provider on 127.0.0.1.

### Added: the dashboard signs a user in through the organization's OpenID Connect provider

- `/auth/login?organization=<organization id>` is an organization's sign-in link. When `organization` is a UUID, the
  sign-in page shows a "Sign in with your identity provider" button above the email and password form. The button
  starts the sign-in with `POST /api/v1/auth/oidc/{organizationId}/login`, keeps the answered flow in the browser
  tab's session storage, and sends the browser to the provider's authorization URL when that URL is `https`. A page
  the browser restores from its back-forward cache on the way back from the provider has the button enabled.
- The new `/auth/oidc/callback` page, the redirect URI the server names, removes the flow from the tab's storage as it
  reads it and posts it with the redirect's `code` and `state` to `POST /api/v1/auth/oidc/callback`. It stores the
  answered pair as a new session and opens the dashboard, the sign-in link's `returnUrl`, or the password change page
  for an account that must change its password. A tab that holds no flow posts nothing, so the code and state of a
  callback address alone sign no one in. A refused sign-in shows a line for the server's refusal code; a provider
  redirect that carries an `error` shows a fixed line for it and never the redirect's `error_description`. Both link
  back to the organization's sign-in link. The page takes the code and state out of the address bar.
- The dashboard's API client treats a `401` from a route under `/api/v1/auth/oidc/` as that route's refusal, as it
  treats one from `/api/v1/public/`: it does not refresh the session, show the session-expired toast or reload the
  page.
- Tests: `apps/web/app/auth/login/page.oidc-sign-in.test.tsx` starts a sign-in from the link, refuses an answered
  address that is not `https`, and enables the button on a restored page. `apps/web/app/auth/oidc/callback/page.test.tsx`
  completes a sign-in once (also when the development build runs the effect twice), posts nothing from a tab that
  started none, and shows a refusal and a provider's error. `apps/web/lib/api.oidc-sign-in.test.ts` holds the two
  requests and the `401` refusal, and `apps/web/lib/oidc-sign-in.test.ts` the stored flow and the lines shown.

### Added: an organization can require its users to sign in through its OpenID Connect provider

- `PUT /api/v1/admin/oidc-sign-in` takes an optional `requireProviderSignIn`, `false` when it is left out. `GET`
  answers it, and the audit row of a stored configuration names it. It is kept in the configuration's `system_config`
  row; this adds no migration, and a configuration stored without it writes the document the earlier changes wrote.
- With it set, a user of the organization who is not an administrator is answered `403`, with the code
  `provider_sign_in_required` and the organization's id and no tokens, for their own password at
  `POST /api/v1/auth/login/local`, `POST /api/v1/public/login` and `POST /api/v1/auth/reauth`. The requirement is read
  after the password check, so a wrong password is answered `401` as before, and the `403` is not recorded as a failed
  attempt. A password change by such a user, at `POST /api/v1/auth/change-password` or at the required change's
  `POST /api/v1/public/change-password`, stands and answers no tokens. The same user signs in through the provider.
- Administrators keep password sign-in, so the organization can still change or remove a provider that cannot be
  reached; removing the provider removes the requirement with it. When the requirement cannot be read, those three
  routes answer a user who is not an administrator `500`, and that user's password change answers no tokens.
- Tests: `apps/backend/internal/interfaces/http/handlers/oidc_sign_in_required_test.go` stores the requirement through
  the administration route, refuses a member's own password at both sign-in routes and at re-authentication, signs
  the member in through the stub provider on 127.0.0.1, signs in an administrator of the organization and a member of
  another, and holds the two password changes and the unreadable requirement.
  `apps/backend/internal/application/provider_sign_in_requirement_test.go` holds the order of the checks and what a
  refusal records, and `apps/backend/internal/infrastructure/repository/oidc_sign_in_config_repository_requirement_test.go`
  the stored document.

### Added: an administrator sets up the organization's OpenID Connect sign-in in the dashboard, and the sign-in page offers the provider to a user the requirement refuses

- The dashboard's identity provider page, `/dashboard/admin/identity-provider` (the Identity provider tab of
  Organization, for administrators), reads the organization's provider with `GET /api/v1/admin/oidc-sign-in`, stores it
  with `PUT` and removes it with `DELETE` after a confirmation. It shows the redirect URI to register with the provider
  and, once a provider is stored, the organization's sign-in link, each with a copy button. Its form takes the issuer,
  the client ID and the client secret, whether a sign-in creates an account (with the role and the email domains), and
  whether the organization requires sign-in through the provider. Before it sends anything it checks that the issuer
  is an `https` URL, that the client ID and a client secret it cannot keep are given, and that provisioning lists 1 to
  20 domains. A `400` from the route is shown in the route's words; another failure is described from its status.
- `PUT /api/v1/admin/oidc-sign-in` keeps the stored client secret when the body leaves `clientSecret` out, or sends it
  empty or as spaces, and names the stored provider's issuer and client id, so the page changes the provisioning or the
  requirement without the secret. A body that names another issuer or client id, or comes from an organization with no
  stored provider, needs the secret and is answered `400` without it. The audit row of a stored configuration names
  `clientSecretKept`.
- When a password sign-in on the sign-in page is answered `403` with the code `provider_sign_in_required`, the page says
  that the organization requires sign-in through its identity provider and shows the "Sign in with your identity
  provider" button for the organization the refusal names, also on a page opened without a sign-in link, and in place
  of the organization a sign-in link names. The dashboard's API client keeps a refusal's `organizationId` on the error
  it throws.
- Tests: `apps/web/app/dashboard/admin/identity-provider/page.test.tsx` stores a provider, changes a setting with the
  client secret left empty, asks for the secret when the client ID or the issuer changes, checks the fields, shows a
  refusal, removes the provider after a confirmation and copies both values.
  `apps/web/app/auth/login/page.provider-sign-in-required.test.tsx` offers the refused account's provider and starts its
  sign-in. `apps/web/lib/identity-provider-settings.test.ts` holds the form's rules and body, and
  `apps/web/lib/api.oidc-sign-in-settings.test.ts` the three requests and the refusal's organization.
  `apps/backend/internal/application/oidc_sign_in_client_secret_test.go` and
  `apps/backend/internal/interfaces/http/handlers/oidc_sign_in_handler_client_secret_test.go` keep the secret for the
  same issuer and client id, sign in with it through the stub provider on 127.0.0.1, and refuse a missing secret for
  another issuer or client id. On the commit this builds on, the two backend tests that keep the secret fail, the route
  answering `400`, and each of the four web test files fails.

### Fixed: a user whose organization requires sign-in through its OpenID Connect provider confirms the deletion of their account with their password

- `POST /api/v1/auth/delete-account` checks the password through the sign-in's own checks, which refuse a user who is
  not an administrator of an organization with `requireProviderSignIn` after the password matched. The route answered
  that refusal as a wrong password: on the commit this builds on, a member of such an organization who sent their
  correct password was answered `401` with `{"error": "Invalid password", "code": "invalidPassword"}` and no deletion
  was requested, and the same `401` answered when the requirement could not be read.
- The request of such a user now stands: the password matched before the requirement was read, and the route issues no
  tokens, as a password change by such a user does. When the requirement cannot be read, the route answers `500` and
  records no request. A wrong password is answered `401` and recorded as before, and the accepted request is not a
  failed attempt. `docs/API.md` names both cases.
- Tests: `apps/backend/internal/interfaces/http/handlers/account_deletion_handler_test.go` confirms the deletion with
  the password of a member of a requiring organization after refusing the wrong one, and holds the unreadable
  requirement. On the commit this builds on, both tests fail, the route answering `401` with the code `invalidPassword`
  in place of `202` and `500`.

### Fixed — the deployment guide names the failed sign-in reasons `provider_sign_in_required` and `sign_in_refused` (#812)

- `POST /api/v1/public/login` and `POST /api/v1/auth/login/local` answer a user who is not an administrator `403` for
  their own password when the user's organization requires sign-in through its identity provider, and `500` when that
  requirement cannot be read, and the SIEM receives each as `aim.auth.signin_failed` with the reason
  `provider_sign_in_required` or `sign_in_refused`. `docs/DEPLOYMENT.md` said the event is sent for the sign-ins the two
  routes answer `401`, and its reason table named neither reason.
- The guide's event table now names the `403` and the `500`, its reason table gives both reasons for both routes and
  the status each reason is answered with, and its table of records outside the SIEM names both reasons among those of
  `POST /api/v1/auth/login/local` that AIM does not store.
- Tests: `apps/backend/internal/interfaces/http/handlers/sign_in_failure_siem_docs_test.go` sends eight sign-ins to
  both routes and holds the guide's three tables to the answers, the reasons reported and the failed attempts
  recorded. On the commit this builds on, it fails: the event table names `401` where the routes answered `401`, `403`
  and `500`, and the reason table has no answer column. `apps/backend/cmd/server/siem_provider_sign_in_refusals_test.go`
  sends the `403` and the `500` of both routes through the SIEM forwarder to a collector stub and reads the two reasons
  from the events it receives.

### Fixed: `GET /api/v1/verification-events` with `agent_id` answers `500` when the read of the agent's events fails, and reads an `offset` that is not an integer as `0`

- With `agent_id` set, `ListVerificationEvents` parsed the agent id into an `err` declared inside that branch, and
  the read of the agent's events assigned that inner `err`. The check after the branch read the handler's own `err`,
  which held the result of parsing `offset`. A failed read of the agent's events was answered `200` with
  `"events": null` and `"total": 0`, and a request with `offset=abc` was answered `500` with
  `Failed to retrieve verification events` although the read succeeded.
- The branch now only chooses which list is read, the organization's or the agent's, and that list is read once after
  the branch, into the `err` that is checked. With `agent_id` set, a failed read is answered `500` with
  `Failed to retrieve verification events`, and an `offset` of `abc` or `1.5` is read as `0` and answered `200`: the
  answers the list without `agent_id` gives to the same requests.
- Test: `apps/backend/internal/interfaces/http/handlers/verification_event_list_filter_test.go` sends a request
  whose read fails and requests with `offset=abc` and `offset=1.5`, each with and without `agent_id`. On the previous
  handler its three `agent_id` cases failed and its three cases without `agent_id` passed.

### Changed: the FGA engine holds no Registry ASC address until `FGA_REGISTRY_ASC_URL` sets one

- `NewFGAEngine` set the engine's Registry ASC address to `https://api.oa2a.org`, the OpenA2A Registry, and the server
  replaced it only when `FGA_REGISTRY_ASC_URL` was set. A new engine now holds none, and `initServices` sets one from
  `FGA_REGISTRY_ASC_URL` when it is set, as before. The quickstart sets no `FGA_` variable.
- No request changes: no code outside the tests reads the address. The engine reads the ASC risk summary from the
  `agent_security_contexts` table.
- Tests: `apps/backend/internal/application/fga_engine_registry_asc_url_test.go`. A new engine holds an empty Registry
  ASC address; the control sets one with `SetRegistryASCURL` and reads it back.

### Changed: the deployment guide says the quickstart server runs no Registry bridge and names the variables that turn it on, and a test holds the bridge to `REGISTRY_BRIDGE_ENABLED=true`

- The Docker section of `docs/DEPLOYMENT.md`'s "Install from this repository", which the self-hosted install section
  of `README.md` links, says that the quickstart passes no `REGISTRY_*` variable to `aim-server`, so the server runs no
  Registry bridge, the job that sends attestation data to the OpenA2A Registry, and that adding
  `REGISTRY_BRIDGE_ENABLED=true` and a `REGISTRY_BRIDGE_SECRET` to the `backend` service's `environment` and running
  `docker compose -f docker-compose.quickstart.yml up -d` turns it on. Set in the shell or in the install's `.env`, the
  two variables do not reach the server: `docker-compose.quickstart.yml` and `scripts/quickstart.sh` name no
  `REGISTRY_` variable, and the compose file's `backend` service reads no `env_file`. `README.md` does not change; it
  is at the word target its short-form test holds.
- No default changes. `config.Load` reads `REGISTRY_BRIDGE_ENABLED` as on only when it is exactly `true`, as the
  `REGISTRY_BRIDGE_ENABLED` rows of `internal/config/testdata/environment_table.json` record.
- `cmd/server` builds the bridge in `newRegistryBridgeService`, which returns nil unless `REGISTRY_BRIDGE_ENABLED` is
  `true`. For a nil bridge `startRegistryBridgeJob` starts no goroutine and logs nothing, and `main` passes it
  `services.RegistryBridge` without a nil check of its own. The job's start line,
  `Registry bridge enabled (runs every 6 hours)`, is logged before `startRegistryBridgeJob` returns, and its stop line,
  `Registry bridge stopped`, by the stop function it returns; the job's goroutine logged both before.
- Tests: `apps/backend/cmd/server/registry_bridge_opt_in_test.go`. With every `REGISTRY_` and `ATC_` variable unset,
  `config.Load` and `newRegistryBridgeService` build no bridge and `startRegistryBridgeJob` logs no line; the negative
  control sets `REGISTRY_BRIDGE_ENABLED=true`, and the bridge is built and the job logs its start and stop lines. A
  third test reads the non-test Go files of `cmd/server`: `NewRegistryBridgeService` is called only in
  `newRegistryBridgeService`, `initServices` sets `Services.RegistryBridge` from it with `cfg.Registry`, and `main`
  passes `services.RegistryBridge` to `startRegistryBridgeJob`.

### Fixed: `aim-bootstrap --default` adds the default security policies a fresh database did not receive

- Migrations `015_add_default_security_policies.sql` and `052_add_default_mcp_policies.sql` insert the OpenA2A Admin
  organization's default policies only when the user `admin@opena2a.org` already exists. Migration
  `013_create_default_admin_user.sql` no longer creates that user; `aim-bootstrap --default` does, after the migrations
  have run, so on a fresh database neither migration inserted a row. Measured on PostgreSQL 18.3: after the 133
  migrations and `aim-bootstrap --default`, `security_policies` held no row.
- A `--default` run that creates the admin now adds those twelve policies to the admin's organization, created by the
  admin, in the transaction that creates the admin: the six from migration 015 enabled, and the six MCP policies from
  migration 052 disabled, with the values migrations 105, 114 and 130 give them. A run that finds the admin in place
  adds none, so a policy an administrator deleted is not added back, and a bootstrap without `--default` adds none.
  Measured on PostgreSQL 18.3: after the 133 migrations, the first `aim-bootstrap --default` left twelve rows,
  identical to the rows the migrations leave on a database where the admin existed before migration 015 ran, and a
  second run left the same twelve.
- Tests: `apps/backend/cmd/bootstrap/default_policies_integration_test.go` runs the bootstrap twice against PostgreSQL
  and checks the policies it leaves against the names migrations 015 and 052 insert; run against the bootstrap before
  this change, it found no policy. It also applies the migrations for an organization whose admin exists and compares
  every column the bootstrap writes with theirs. `run_bootstrap_test.go` holds the statement order, the organization and
  admin each policy carries, and the rollback when a policy is refused, and
  `security_policy_seed_threshold_test.go` reads the bootstrap's two trust-score policies with the other places that
  seed one.

### Fixed: The server's `ADMIN_PASSWORD` seed adds the default security policies, as `aim-bootstrap --default` does

- `scripts/quickstart.sh`, which writes `ADMIN_PASSWORD` to the `.env` it creates, and the fleet governance guide create
  the first administrator with the server's `ADMIN_PASSWORD` seed at startup and do not run `aim-bootstrap`. That seed
  inserted the administrator only, so the OpenA2A Admin organization it seeds into received no security policy.
  Measured on PostgreSQL 18.3 with the server built before this change: on an empty database, after the startup
  migrations and the seed, `security_policies` held no row, and a second start left none.
- The start that creates the administrator now adds the twelve policies `aim-bootstrap --default` adds, created by that
  administrator, in the transaction that inserts it, and logs
  `Added 12 default security policies to the admin.opena2a.org organization`. A policy the organization already has
  under the same name is left as it is, and a start that finds an administrator adds none. When the database refuses a policy, the seed returns an error and rolls
  back the transaction that inserted the administrator. Both seeds read the policies from one table,
  `apps/backend/internal/infrastructure/seed/default_policies.go`, moved there from `apps/backend/cmd/bootstrap/`.
  Measured on PostgreSQL 18.3 with the server built from this change: on an empty database, the first start left twelve
  rows, identical in name, description, type, enforcement action, severity, rules, scope, enabled state and priority to
  the rows `aim-bootstrap --default` leaves, and a second start found the administrator and left twelve.
- Tests: `apps/backend/cmd/server/admin_seed_policies_integration_test.go` creates an empty database, applies the
  migrations with the server's startup function, runs the seed twice and checks the policies against the names
  migrations 015 and 052 insert, their organization, their creator and their enabled state; run against the server
  before this change, it found no policy. The sqlmock tests in `admin_seed_test.go` hold the statement order, the
  organization and administrator each policy INSERT carries, the log lines, and the rollback when a policy is refused.
  `security_policy_seed_threshold_test.go` reads the trust-score policies from the table's new path.

### Fixed: a request that ends in a returned error is recorded in `api_calls` with the status the client received

- `AnalyticsTracking` read the response before the app's error handler had run, so the record it writes to
  `api_calls` for a request that ended in a returned error carried status `200`, a response size of `0` and no error
  message: a `404` for a path no route serves, a `403` a handler returned as a `*fiber.Error`, and the `500` the error
  handler answers for a returned error that is not a `*fiber.Error`. The middleware now runs the app's error handler
  itself and returns nil, as `HTTPServerSpans` and `PrometheusMiddleware` do, and records the status and response size
  of the answer that handler writes. For a status of `400` or above, the error message is that answer's body when the
  body is shorter than 1,000 bytes, as it already was for an error a handler writes itself.
- `analytics_tracking_test.go` sends those three requests and one a handler answers `200` through an app with its own
  JSON error handler, and fails when a record's status, response size or error message differs from the answer the
  client received. On the previous middleware the first three were each recorded with status `200`.

### Added: `OAuthUser` carries the email-verified claim of Google's and Okta's user info, and a missing claim reads as not verified

- `parseUserInfo` in `apps/backend/internal/infrastructure/auth/oauth.go`, which `OAuthService.GetUserInfo` calls, read
  Google's `id`, `email`, `name` and `picture` and Okta's `sub`, `email` and `name`, and dropped the provider's claim
  that the email is verified. The `OAuthUser` it returned had `ID`, `Email`, `Name`, `AvatarURL` and `Provider`.
- `OAuthUser.EmailVerified` is set from Google's `verified_email` and from Okta's `email_verified`. A claim that is
  missing, or is not a JSON boolean (the test sends the string `"true"`), reads as `false`. The Microsoft case reads no
  claim and leaves the field `false`. A caller that links or registers an account by the email `GetUserInfo` returns
  can refuse when the field is `false`; nothing in the backend calls `GetUserInfo` outside its tests.
- Test: `apps/backend/internal/infrastructure/auth/oauth_email_verified_test.go` checks the field for Google payloads
  whose `verified_email` is `true`, `false`, the string `"true"` and absent, for Okta payloads whose `email_verified`
  is `true`, `false` and absent, and for a Microsoft Graph profile, and has `GetUserInfo` read Google JSON answers with
  `verified_email` `true`, `false` and absent from a loopback test server. On the previous `oauth.go` the test file did
  not compile, as `OAuthUser` had no `EmailVerified`.

### Changed: the account deletion purge deletes an organization it leaves with no user

- When the account deletion purge deletes the last user of an organization, it deletes the organization too, in the
  same transaction, with the rows whose reference to the organization has `ON DELETE CASCADE`: among them the
  organization's agents, MCP servers, API keys, webhooks and audit log rows. Before this change the purge deleted the
  user and kept the organization, with its agents, MCP servers and other rows and no user. `a2a_call_chain` and
  `a2a_security_violations` keep their rows, with their reference to the organization set to `NULL`.
- Migration `140_organization_references_allow_organization_delete.sql` gives `ON DELETE CASCADE` to the three
  references to `organizations(id)` that had no `ON DELETE` rule: `a2a_consent_records.organization_id`,
  `community_intelligence_reports.organization_id` and `device_codes.organization_id`. Deleting an organization that
  held an A2A consent record or a community intelligence report was refused with a foreign-key violation.
- An organization that has a signed audit record chain is kept, as `record_chains.organization_id` and
  `record_debts.organization_id` keep `ON DELETE RESTRICT`: the purge deletes the account all the same and logs the
  refusal. Another failure of the organization's delete rolls back the purge's batch, whose accounts the next run
  deletes.
- The purge locks the organization's row before it reads whether a user is left: a user that another transaction is
  adding to the organization when the purge reaches it is counted once that transaction commits, and the organization
  and that user are kept.
- No `account_deleted` audit row is written for an account whose organization the purge deleted with it, as the
  organization's audit log goes with the organization; the server logs a line that names the user and the
  organization.
- The purge chooses and locks the due accounts of a batch once, in a `MATERIALIZED` common table expression. Its
  `DELETE FROM users ... WHERE id IN (SELECT ... LIMIT ... FOR UPDATE SKIP LOCKED)` was planned as a nested loop that
  ran the subquery again for each row of `users`, and with the oldest due account locked by another transaction it
  deleted two accounts under a limit of one.
- The dashboard's `/auth/delete-account` page says that the organization is deleted too, with its agents and MCP
  servers, when no other user is left in it then, and `docs/API.md` describes the organization's delete.
- Tests: against PostgreSQL, `apps/backend/internal/infrastructure/repository/user_deletion_organization_integration_test.go`,
  five of whose six tests fail with the purge code and the migrations of this change's parent; and
  `apps/backend/internal/application/account_deletion_service_test.go` and
  `apps/web/app/auth/delete-account/page.test.tsx`.

### Added: a signed-in user can ask for the deletion of their own account, which is deactivated at once and deleted 29 days later

- `POST /api/v1/auth/delete-account` takes the signed-in user's password, checked as at sign-in: a wrong password is
  recorded as a failed attempt and counts toward the lockout, and a wrong password, a locked account and an account
  without a password are refused with `401`. An accepted request answers `202` with the account's status, the time of
  the request (`deletionRequestedAt`) and the time from which the account is deleted (`deletionDueAt`, 29 days later).
- The request deactivates the account and records its time in `users.deletion_requested_at` (migration
  `139_user_deletion_requests.sql`). The account deletion purge, which the server runs at start and every hour,
  deletes each account whose request is 29 days old or older, as `DELETE /api/v1/admin/users/:id` deletes a user: the
  user's API keys and the user's own audit rows go with it, and the agents and MCP servers the user created are kept
  without a creator, unless the purge deletes the organization with them, as the entry above describes. The request
  does not revoke the API keys before then. An admin who activates the account
  withdraws the request: a trigger the migration adds clears it when the account's status leaves `deactivated`.
- The organization's only active admin is refused with `409` and the code `lastActiveAdmin` while the organization
  has another user who is not deactivated. The requests of one organization are taken one at a time, so two admins
  who ask at once do not both get through.
- Deactivating an account ends its sessions. The auth middleware reads whether the account is deactivated from the
  user record it reads on every request, and answers the account's access tokens with `401`, after an admin's
  deactivation as after a deletion request; they were admitted until they expired (`JWT_ACCESS_TTL`, `2h` by default).
- The request is recorded as an `account_deletion_requested` audit row, which the deletion removes with the user's
  other rows, and the deletion as an `account_deleted` row that names no user and whose resource is the deleted
  user's id, when the organization is kept.
- The route shares the strict rate limit of the other `/api/v1/auth` routes (10 requests a minute), and `docs/API.md`
  documents it.
- The refused password's `401` carries the code `invalidPassword`. The route's other `401` answers, which refuse the
  access token, carry no code.
- The dashboard offers the request: the account menu's Delete account entry opens `/auth/delete-account`, which says
  what the request does, takes the password and, once the request is accepted, shows the time from which the account
  is deleted and clears the session the browser stores. The page sends the request through
  `api.requestAccountDeletion`, which throws a refused password to the form and keeps the session. `request()` in
  `apps/web/lib/api.ts` takes any `401` for a refused session: used for this route, it refreshed the session, sent
  the password a second time and signed the user out. After a `401` without the code, the session is refreshed and the
  request sent once more, and a session still refused is signed out by a request that carries no password. The only
  active admin's `409` links to the Users page, where another user is made an admin.
- Tests: `apps/backend/internal/interfaces/http/handlers/account_deletion_handler_test.go`,
  `apps/backend/internal/application/account_deletion_service_test.go`,
  `apps/backend/cmd/server/account_deletion_purge_job_test.go`, the deactivated account cases of
  `apps/backend/internal/interfaces/http/middleware/auth_password_change_test.go` and
  `TestStrictRateLimiterCoversExactlyTheCredentialRoutes`; against PostgreSQL,
  `apps/backend/internal/infrastructure/repository/user_deletion_request_integration_test.go` and
  `apps/backend/internal/interfaces/http/middleware/auth_deactivated_account_integration_test.go`, which fails on the
  code before this change: the deactivated account's access token was admitted. In the dashboard,
  `apps/web/lib/api.account-deletion.test.ts`, `apps/web/lib/account-deletion.test.ts`,
  `apps/web/app/auth/delete-account/page.test.tsx` and `apps/web/components/dashboard-header.test.tsx`.

### Fixed: deleting an admin removes the admin's email from the admin access reviews kept as compliance evidence

- `POST /api/v1/compliance/evidence/collect` for the `admin_access_review` check stores an access review whose data
  holds, under `adminUsers`, an entry for each user of the organization whose role is admin: the user's id, email, last
  sign-in and status. The review is the organization's, and deleting one of those users kept it with the user's email
  in the user's entry: `ComplianceService.GetEvidenceForCheck`, which `GET /api/v1/compliance/evidence/check/:checkName`
  calls, read the review back with that email.
- Migration `138_user_delete_removes_email_from_access_review_evidence.sql` adds an `AFTER DELETE` trigger to `users`.
  For each deleted user it removes `email` from the entries whose id is the user's in the `admin_access_review`
  evidence of the user's organization. The review and the entry are kept, the entry with its id, last sign-in and
  status. The migration also removes, once, `email` from the entries whose id names no user.
- Test: `apps/backend/internal/application/compliance_access_review_user_delete_integration_test.go`, against
  PostgreSQL. With every migration applied, one test collects two access reviews through
  `ComplianceService.CollectEvidence`, the first by the admin it then deletes through `UserRepository.Delete`, and reads
  them back through `GetEvidenceForCheck`: both keep an entry for each of the two admins, the deleted admin's entry
  keeps its id, last sign-in and status and has no email, and the other admin's entry is kept as collected. Against a
  database migrated without migration 138 it fails, with the deleted admin's email in both reviews. Another test
  deletes an organization that holds access reviews. In
  `apps/backend/internal/infrastructure/repository/user_delete_access_review_integration_test.go`, a test runs the
  migration file twice in a schema that stands in for a database migrated through 137.

### Fixed: deleting a user deletes the user's registration requests and the copies of the user's email and name in audit rows

- Approving a registration request copies its email into the user it creates and keeps the request, which holds the
  email and the first and last name it was filed with, and the password hash and signup answers when the registration
  gave them. Deleting the user kept the request, and as registration files a request without an organization,
  deleting the organization kept it too. An admin's role change writes the
  user's email into its audit row as `user_email`, a registration approval writes `user_email` and `user_name`, and
  the admin's permanent delete wrote both into the row it adds after the delete. These rows name the admin as their
  user, so deleting the user kept them with the user's email.
- Migration `137_user_delete_removes_registration_requests_and_audit_copies.sql` adds an `AFTER DELETE` trigger to
  `users`. For each deleted user it deletes the registration requests filed with the user's email, compared without
  case, that hold the user's organization, and those that hold none when no remaining user has that email; a request
  that holds another organization is kept. It removes `user_email` and `user_name` from the metadata of the `user` and
  `user_role` audit rows of the user's organization whose resource is the user, and of the organization's
  `registration_approval` rows whose `user_email` is the user's email; the rows are kept.
- `DELETE /api/v1/admin/users/:id` writes its audit row with `action` and `warning` in its metadata, and names the
  deleted user by the row's resource id only.
- The migration also deletes, once, the approved requests whose email no user has, and removes `user_email` and
  `user_name` from the `user` and `user_role` rows whose resource is a user who no longer exists, among them the
  permanent delete rows written before this change, and from the `registration_approval` rows whose `user_email` no
  user of the row's organization has.
- Test: `apps/backend/internal/infrastructure/repository/user_delete_requests_audit_integration_test.go`, against
  PostgreSQL. With every migration applied, one test deletes a user through `UserRepository.Delete` and checks that
  the requests filed with the user's email that hold the user's organization or none are deleted, with the email in
  either case, that a request with that email filed with another organization and another person's request are kept,
  and that the user's role change and registration approval rows keep their other metadata and lose the copies,
  while another user's rows and another organization's row are kept as written. One shows that a request that holds
  no organization is kept while a user of another organization has its email, and deleted with that user; one deletes
  an organization and checks that the requests of its users are deleted. Against a database migrated without
  migration 137 the three fail, each on a request or a copy that is kept. The fourth runs the migration file twice in a
  schema that stands in for a database migrated through 136. In
  `apps/backend/internal/interfaces/http/handlers/admin_handler_integration_test.go`, a test reads the audit row that
  `PermanentlyDeleteUser`, the handler of `DELETE /api/v1/admin/users/:id`, writes, and fails with the handler as it
  was, whose row held the user's email and name.

### Fixed: deleting a user clears the copies of the user's name and email that agents and MCP servers hold

- An agent and an MCP server store the name and email of the user who created them, returned as `createdByName` and
  `createdByEmail`, and have `updatedBy`, `updatedByName` and `updatedByEmail` for the user who last changed them,
  which no code in this repository writes. Deleting the user set `created_by` to `NULL` and kept the copies, so the
  agent and the MCP server went on returning the deleted user's name and email. `updated_by` had no foreign key, so
  a value written there kept the deleted user's id.
- Migration `136_user_delete_clears_name_and_email_copies.sql` adds a `BEFORE UPDATE` trigger to `agents` and
  `mcp_servers`: it writes `NULL` to `created_by_name` and `created_by_email` when `created_by` is `NULL`, and to
  `updated_by_name` and `updated_by_email` when `updated_by` is `NULL`. Deleting a user sets `created_by` to `NULL`
  through its foreign key, and that update runs the trigger. `agents.updated_by` and `mcp_servers.updated_by` now
  reference `users(id)` with `ON DELETE SET NULL`; before the foreign key is added, an `updated_by` that names no user
  is set to `NULL` with its copies.
- The migration also clears the copies on the rows that lost their user before it ran. An agent it clears gets the
  time of the migration as its `updated_at`.
- Test: `apps/backend/internal/infrastructure/repository/user_delete_copies_integration_test.go`, against PostgreSQL.
  One test, with every migration applied, deletes a user through `UserRepository.Delete` and reads back through
  `AgentRepository.GetByID` and `MCPServerRepository.GetByID` the agent and the MCP server the user created and last
  changed: no creator, no updater, an empty name and email, and `NULL` stored in each copy. An agent another user
  created and the deleted user last changed keeps its creator's copies, and an MCP server another user created and
  changed keeps all of its copies. Against a database migrated without migration 136 the test fails, with the deleted
  user's name and email returned. The other test runs the migration file twice in a schema that stands in for a
  database migrated through 135, and checks that a row whose creator was deleted and a row whose `updated_by` names no
  user lose those copies, that a row whose users exist keeps them, and that deleting that user then clears them.

### Fixed: a user who created an agent, an API key, an MCP server or another row the organization keeps can be deleted

- The admin's `DELETE /api/v1/admin/users/:id` deletes the user with `DELETE FROM users`, which failed with a
  foreign-key violation for a user named in any of eighteen references to `users(id)`: sixteen had no `ON DELETE` rule,
  and `webhooks.created_by` and `capability_requests.requested_by` had `ON DELETE RESTRICT`. The user who created an
  agent, an API key or an MCP server was one such user. Migration `135_user_references_allow_user_delete.sql` gives
  each of the eighteen a rule.
- The user's API keys and CLI sign-in device codes are deleted with the user, as the user's SDK tokens and bootstrap
  tokens already were. The API key middleware gives a request the key's creator as its user, and polling an approved
  device code mints a token pair for the user who approved it. An SDK install that authenticates with an API key the
  deleted user created needs a new key.
- The other rows are kept, with the reference to the user set to `NULL`: agents, MCP servers, webhooks, compliance
  evidence, capability requests and capability grants, A2A policies, security settings, delegations and revocations,
  alerts, sign-in lockouts, registration requests, and the users the deleted user approved. Deleting a user also
  deleted the security policies and the tags the user had created, and with a tag its links to agents and MCP
  servers; those are now kept the same way. `agents.created_by`, `mcp_servers.created_by`, `webhooks.created_by`,
  `compliance_evidence.collected_by`, `capability_requests.requested_by`, `security_policies.created_by` and
  `tags.created_by` accept `NULL`.
- An agent whose creator was deleted is read with a zero `CreatedBy`, and the handlers write that id as the user of the
  agent's audit rows, of the MCP servers it registers, and of its capability requests with their automatic approval
  and grant. The repositories now store `NULL` for the zero id, which failed the foreign key. A capability request
  whose requester was deleted is still read and listed, with an empty `requestedByEmail`, and can still be decided.
- The other thirteen references to `users(id)` keep their rule; among them, the user's audit log rows are deleted
  with the user. The copies of the user's name and email that agents and MCP servers hold are cleared with the user,
  as the entry above describes.
- Test: `apps/backend/internal/infrastructure/repository/user_delete_integration_test.go`, against PostgreSQL with
  every migration applied. One test reads the catalog: every reference to `users(id)` has `ON DELETE CASCADE` or
  `ON DELETE SET NULL`, and each of the twenty references the migration changed has the rule it chose. One deletes a user named in all twenty
  through `UserRepository.Delete`, and checks that the user's API key and device code are gone, that every other row
  is kept with the reference `NULL`, and that another user's API key and device code are kept. One deletes the creator
  of an agent and writes the agent's audit row, MCP server, and capability requests with their approval and grant
  through the repositories. Against a database migrated without migration 135 the three tests fail, the delete with
  `violates foreign key constraint "agents_created_by_fkey"`; with the repositories as they were, the third fails on
  the first such write.

### Changed: each agent private key the server stores is encrypted under a data key of its own, which is stored encrypted under `KEYVAULT_MASTER_KEY`

- `KeyVault.EncryptPrivateKey` wrote format v2: the private key encrypted with AES-256-GCM under the master key itself.
  It now writes format v3. For each private key it generates a 32-byte data key, encrypts the private key under the
  data key with AES-256-GCM, and stores the data key only in encrypted form, under the current master key with
  AES-256-GCM and a nonce of its own. The additional data of the encrypted data key binds the purpose label, the
  header (version byte 3 and the storage key id) and the agent row; the additional data of the encrypted private key
  binds the purpose label, the version byte and the agent row.
- Keys stored in format v2 still decrypt. For a v3 key under an older storage key id, the startup sweep decrypts the
  data key, checks that the private key decrypts under it, and encrypts the data key under the current master key,
  leaving the encrypted private key as it is; a v2 key under an older id is encrypted again in format v3. The one-time
  conversion of v1 keys writes format v3.
- A build from before this change refuses format v3 as `unsupported private key format`. `docs/DEPLOYMENT.md`
  describes the data key under Rotating `KEYVAULT_MASTER_KEY`, and the upgrade under Upgrading across the agent
  private key data key change: stop every instance of an earlier build before the first instance of this one starts.
- Test: `apps/backend/internal/crypto/keyvault_data_key_test.go` reads stored values by the documented layout. Three
  values the vault encrypts, two for different agent rows and one again for the first row, each carry a different
  data key, which decrypts under the master key and does not appear in the stored bytes; each private key decrypts
  under its data key and not under the master key, and a value in format v2, written by the v2 writer the vault held
  before, still decrypts. A v3 value built from the layout decrypts, and is refused with the data key or the private
  key encrypted for another purpose or agent row, the data key encrypted under another master key, or a header
  naming another storage key id. `RewrapPrivateKey`, which the sweep calls, moves only the data key and refuses a
  value whose encrypted private key was altered; the v1 conversion writes v3. Run against the vault before this
  change, each of the four tests fails.

### Fixed — `docs/API.md`'s `POST /api/v1/agents` request sends `agentType` and registers an agent, and its agent answers show `agentType` (#795)

- The `POST /api/v1/agents` section of `docs/API.md` gave the agent's type as `"type": "ai_agent"` in its request
  example and in its response example, and the response example of `GET /api/v1/agents/{id}` did the same. The
  request the handler binds, `CreateAgentRequest`, has no `type` member. Posted to the server's handler and agent
  service, the documented request was answered `400` with the error `agentType is required`.
- The three examples now give the type as `"agentType": "ai_agent"`. The documented request is answered `201`; the
  agent stored carries the `name`, `displayName`, `description` and `agentType` it sends, and the service grants the
  three `capabilities` it lists. The answer to that request, and the answer of `GET /api/v1/agents/{id}` for the
  agent it registers, carry the type as `agentType` and carry no `type` member.
- Test: `apps/backend/internal/interfaces/http/handlers/agent_create_required_members_test.go` reads the two sections
  of `docs/API.md`, checks the members the request example sends against the members `CreateAgentRequest` binds,
  posts the example to the server's handler and agent service, checks that every member the documented answer shows
  is one the handler's answer carries, and reads the stored agent back through the `GET /api/v1/agents/{id}` handler.
  On the previous `docs/API.md` each of its four checks failed.

### Security — a password change ends every session signed in before it, and the API enforces a required password change

- `AuthService.ChangePassword`, which `POST /api/v1/auth/change-password` and `POST /api/v1/public/change-password`
  call, and `RegistrationService.ResetPassword`, which `POST /api/v1/public/reset-password` calls, record the instant
  of the change in `users.password_changed_at`. Migration `134_users_password_changed_at.sql` adds the column, NULL
  until the user's password first changes. `UserRepository.Update` only moves it forward: a later write of a copy of
  the row that does not carry it, or carries an older instant, leaves it in place, and
  `apps/backend/internal/infrastructure/repository/user_repository_password_changed_at_integration_test.go` holds
  that against PostgreSQL.
- For each request the auth middleware authenticates with an access token, it reads that column and
  `force_password_change` from the user's row through `UserRepository.GetSessionState`, which `cmd/server` attaches to
  the JWT service. A token whose sign-in time is before the first whole second after the change is answered 401 by the
  auth middleware and by `POST /api/v1/auth/refresh`. The sign-in time is the token's `auth_time`, or its `iat` when it
  carries none, as an SDK-download token and an access token minted from one do. Token times are whole seconds, so a
  token minted in the second of the change is refused. `POST /api/v1/auth/refresh` also refuses an SDK-download token
  whose `sdk_tokens` row dates its family's download before the first whole second after the change, whatever the
  token's own `iat`.
- The session that made the change is issued a new pair: the answer of `POST /api/v1/auth/change-password` carries
  `accessToken` and `refreshToken` beside `message`, and `POST /api/v1/public/change-password` answers with a pair
  minted the same way. The new pair's `auth_time` is not before the first whole second after the change, so the change
  does not refuse it.
- `DeviceAuthService.PollToken` refuses a device code approved before the first whole second after the user's last
  password change with `ErrAccessDenied`.
- While `force_password_change` is set, the auth middleware admits the account's session only to
  `GET /api/v1/auth/me` and `POST /api/v1/auth/change-password`, compared as the router matches paths: without regard
  to case, and with or without one trailing slash. Every other route it guards answers 403 with
  `"code": "passwordChangeRequired"`, and `POST /api/v1/auth/refresh` answers the same. `POST /api/v1/auth/logout`
  reads no session through the middleware and is unchanged.
- `OptionalAuthMiddleware`, on the `/api/v1/public` routes, leaves a request unauthenticated when its token was signed
  in before the last password change, when the account must change its password, or when the user's row cannot be
  read.
- A route behind the auth middleware answers 503 when the user's row cannot be read, and 401 when no user has the
  token's user ID.
- An SDK install whose user changes or resets the password needs a new download: its SDK-download token was minted
  before the change.
- Once the change or reset is written, every `sdk_tokens` row of the user that is not yet revoked is marked revoked,
  with the reason `password_changed`, through the SDK token repository `cmd/server` attaches to `AuthService` and
  `RegistrationService`. `GET /api/v1/users/me/sdk-tokens` then lists those tokens only with `include_revoked=true`,
  showing that reason. A change or reset that is refused or not written revokes no row. When the rows cannot be
  written, the failure is logged and the change stands; the change refuses the tokens of those rows either way.
  `POST /api/v1/auth/sdk/recover` takes a row revoked for a password change to end its token family, as it takes a row
  revoked by a logout: it answers 401 for each token of the family and mints no credential.
- Tests: `apps/backend/internal/interfaces/http/handlers/password_change_sessions_test.go`,
  `apps/backend/internal/interfaces/http/handlers/password_change_sdk_tokens_test.go`,
  `apps/backend/internal/interfaces/http/middleware/auth_password_change_test.go`,
  `apps/backend/internal/infrastructure/auth/password_change_test.go`,
  `apps/backend/internal/infrastructure/repository/user_repository_password_change_test.go`,
  `apps/backend/internal/application/password_change_sessions_test.go`,
  `apps/backend/internal/application/password_change_sdk_tokens_test.go`,
  `apps/backend/cmd/server/password_change_sessions_wiring_test.go`.

### Changed: the SDK integration scripts complete the seeded admin's required password change before the tests sign in

- `scripts/lib/aim-backend.sh`, which `scripts/test-sdk-python-framework-integration.sh` and
  `scripts/test-sdk-java-helpers-integration.sh` source, seeds the admin with `aim-bootstrap --default`, whose account
  must change its default password, and the tests sign in as that admin: the Java helpers' test then calls
  `GET /api/v1/sdk/download?sdk=java`, and the adapter tests call `POST /api/v1/agents`. While the account's
  `force_password_change` is set, the API admits its session only to `GET /api/v1/auth/me` and
  `POST /api/v1/auth/change-password`, and answers those two routes, as every other, 403 `passwordChangeRequired`.
- Once the server is ready, the script now signs in with the seeded password, changes the password through
  `POST /api/v1/auth/change-password` to a second password it generates, which clears the requirement, waits for the
  next whole second, as a token minted in the second of a change is refused, and checks that a sign-in with the new
  password is admitted and requires no change. The tests sign in with that password, which `admin_password` holds
  after `aim_backend_start`. The request bodies and the bearer token go through files under the run's directory,
  mode 0700, removed when the step ends and by the script's EXIT trap; no password or token is on a command line or
  printed. A sign-in or a change the server refuses stops the run with the route and the status.
- Test: `apps/backend/cmd/bootstrap/aim_backend_script_test.go` runs the step against a server that answers the two
  routes as the backend does, and checks that the change carries the sign-in's token and the seeded password, that
  the password the tests get is the one the change set, that the sign-in after the change is made in a later whole
  second and requires no change, that no password or token is printed, and that a refused sign-in, a refused change
  and an account that still requires a change each stop the run.

### Fixed — the SDK install guides name the dashboard's SDK page, its download buttons and the archive the server sends (#799)

- `docs/sdk/python.md`, `docs/sdk/java.md`, `docs/DEMO_SCRIPT_ECHOLEAK.md` and `docs/tutorials/README.md` sent the reader
  to Settings → SDK Download, and `docs/security/token-rotation.md` and `docs/troubleshooting/README.md` to Dashboard →
  SDK Download. The dashboard's navigation has no Settings entry, and no page or tab of the dashboard is named SDK
  Download. Those two guides and `docs/troubleshooting/authentication.md` said to click "Download SDK", the label of a
  button the Credentials page shows when its SDK token list is empty, which opens the SDK page. Five of the guides ran
  `unzip` on `aim-sdk-python.zip`; the server names the archive `aim-sdk-<type>-v<version>.zip`, from this tree
  `aim-sdk-python-v2.0.3.zip` and `aim-sdk-java-v1.0.0.zip`.
- Each guide now names Developers → SDK & docs (`/dashboard/sdk`), the button "Download Python SDK" or "Download Java
  SDK", and the archive as the server names it. `docs/sdk/python.md` and `docs/sdk/java.md` name the folder the archive
  unpacks into, `aim-sdk-python` or `aim-sdk-java`. The demo script and the tutorials index run
  `unzip ~/Downloads/aim-sdk-python-v*.zip`, as `docs/quick-start.md` does.
- The four steps that copy credentials out of a downloaded SDK, in `docs/security/token-rotation.md`,
  `docs/troubleshooting/README.md` (two) and `docs/troubleshooting/authentication.md`, change to `~/Downloads` and
  extract the newest match, `unzip "$(ls -t aim-sdk-python-v*.zip | head -n 1)"`, before
  `cp -r aim-sdk-python/.aim ~/.aim/`; the one under Missing Credentials File had no extract command. With an earlier
  `aim-sdk-python-v2.0.3.zip` beside a newer `aim-sdk-python-v2.0.3 (1).zip`, a glob such as
  `unzip ~/Downloads/aim-sdk-python-v*.zip` hands `unzip` both names, and it exits 11 and extracts nothing; the
  newest-match command extracts the newer archive.
- Tests: `apps/backend/internal/interfaces/http/handlers/sdk_install_guides_docs_test.go` reads the Developers entry
  from `apps/web/lib/navigation.ts`, its SDK & docs tab from `apps/web/lib/hub-tabs.ts` and the download buttons'
  labels from `apps/web/app/dashboard/sdk/page.tsx`, downloads the Python and Java SDKs through the download handler
  to read the archive name it sends and the folder its entries sit in, and checks the seven guides against them. On
  the previous guides all seven failed.

### Fixed — three more guides name the dashboard's SDK page, button and archive, the credential steps replace `~/.aim/sdk_credentials.json`, and the install steps unzip one archive (#805)

- `docs/tutorials/sdk-quickstart.md` sent the reader to Settings → SDK Download and ran
  `unzip ~/Downloads/aim-sdk-python.zip` twice, `docs/tutorials/dashboard-walkthrough.md` listed an SDK Download row
  under Settings and gave Settings → SDK Download in its quick actions, and `sdk/java/README.md` said to go to
  Settings → SDK Download. Each now names Developers → SDK & docs (`/dashboard/sdk`), the button "Download Python
  SDK" or "Download Java SDK", and the archive as the server names it, `aim-sdk-python-v<version>.zip` or
  `aim-sdk-java-v<version>.zip`. The walkthrough moves the SDK out of its Settings table into a step of its own,
  Step 6, and Settings becomes Step 7. The Java README gives the commands that extract the newest
  `aim-sdk-java-v*.zip` in `~/Downloads` and copy its `.aim/sdk_credentials.json` to `~/.aim/sdk_credentials.json`.
- The credential steps in `docs/security/token-rotation.md`, `docs/troubleshooting/README.md` (two) and
  `docs/troubleshooting/authentication.md` ran `cp -r aim-sdk-python/.aim ~/.aim/`. Run with a
  `~/.aim/sdk_credentials.json` already in place, that command left the file as it was and wrote the archive's copy
  to `~/.aim/.aim/sdk_credentials.json`; `~/.aim/sdk_credentials.json` is the first file the Python SDK's
  `load_sdk_credentials` and the Java SDK's `CredentialManager` read credentials from. The steps now run
  `mkdir -p ~/.aim && cp aim-sdk-python/.aim/sdk_credentials.json ~/.aim/sdk_credentials.json` and
  `chmod 600 ~/.aim/sdk_credentials.json`, which replace that file, or create it when `~/.aim` does not exist. The
  step in `docs/troubleshooting/README.md` that ran `chmod 600 ~/.aim/credentials.json` restricts the copied file.
- `docs/quick-start.md`, `docs/DEMO_SCRIPT_ECHOLEAK.md` and `docs/tutorials/README.md` ran
  `unzip ~/Downloads/aim-sdk-python-v*.zip`. With `aim-sdk-python-v2.0.3.zip` and a newer
  `aim-sdk-python-v2.0.3 (1).zip` in `~/Downloads`, that command printed `caution: filename not matched`, exited 11
  and created no `aim-sdk-python` folder, in bash and in zsh. These guides and `docs/tutorials/sdk-quickstart.md` now
  run `unzip "$(ls -t ~/Downloads/aim-sdk-python-v*.zip | head -n 1)"`, which in the same case extracts the newer
  archive into the folder the command is run from.
- Tests: `apps/backend/internal/interfaces/http/handlers/sdk_install_guides_docs_test.go` checks the three guides as
  it checks the seven before them, and checks that each `unzip` of an SDK archive in the ten guides is the
  newest-match command and that each copy out of an archive's `.aim` folder is the `mkdir -p ~/.aim && cp` of
  `sdk_credentials.json`. On the previous guides eight of the ten failed.
  `sdk/python/tests/test_readme_self_hosted_sign_in.py` holds `docs/quick-start.md` to the newest-match command; it
  failed on the previous guide.

### Fixed — the SDK tabs link pages that continue the hosted flow, the TypeScript tab says where its API key comes from, and the Python example declares the README's capability

- The Python and TypeScript SDK tabs (`apps/web/lib/sdk-tabs.ts`, shown on the dashboard and on the get-started page
  of aim.opena2a.org) linked `https://opena2a.org/docs/tutorials/sdk-quickstart`. That address redirects to a
  self-hosted tutorial that starts from AIM running at `http://localhost:3000` and a Python SDK downloaded from the
  dashboard, with no `aim-sdk login` and no TypeScript content. The TypeScript example reads its key from
  `AIM_API_KEY`, and the tab did not say where the key comes from. The Python example called
  `secure("my-first-agent")`, where the README registers the agent with `capabilities=["db:read"]`.
- The Python tab links the AIM quick start (`https://opena2a.org/docs/aim/quick-start`), whose cloud section installs
  the SDK and signs in with `aim-sdk login` against aim.opena2a.org, and its example is the README's
  `secure("my-first-agent", capabilities=["db:read"])`.
- The TypeScript tab links the TypeScript SDK's reference (`sdk/typescript` in this repository). Its new note says
  that the client sends an API key, read from `AIM_API_KEY`, in the `X-API-Key` header; that the key is created under
  Developers, then Credentials (`/dashboard/credentials`), for an agent that already exists, because every API key
  belongs to one agent and an agent registered with the key joins the key's organization; and that the key alone is
  not an agent identity, because `registerAgent()` generates the new agent's own Ed25519 key pair.
- The Java tab's note no longer calls the SDK page's download "the Java SDK". It names the page (Developers, then SDK
  & docs, `/dashboard/sdk`) and states what the credential step needs from the archive: it holds
  `.aim/sdk_credentials.json`, which the SDK copies to `~/.aim/` the first time it runs from the unzipped folder.
- Tests: `apps/web/lib/sdk-tabs.test.ts` checks the Python tab's link and capability, the TypeScript tab's link and the
  key source and header its note names, that no tab links the self-hosted tutorial, and that the Java note does not
  call the download the Java SDK. The four cases failed on the previous table.

### Fixed — a revocation list refresh whose answer ends after the read's deadline is refused as `timeout`, however the answer's body ends (#784)

- A server can end an answer cleanly once it sees the client cancel the request, and the client can read that end
  before it sees its own cancellation. The list client then checked the body it had read as a document. On the
  previous code, against a transport that ends a `200` answer's body with no error once the request's context is
  done, a refresh was refused as `parse` for an empty discovery document and for a list body cut after
  `{"entryCount":0,`: at the refresh's deadline, at the HTTP client's timeout, and when the refresh was canceled.
- The body of a `200` answer that ends once the refresh's context is done, or once the HTTP client's timeout has
  passed since the request began, is now refused without being checked as a document: as `timeout` when a deadline
  passed, and as `fetch` when the refresh was canceled. A body that ends before then is checked as before, and an
  answer with another status than `200` is read as before.
- Tests: `apps/backend/internal/infrastructure/atc/crl_client_test.go`, through a transport that answers without a
  connection.

### Fixed — the quick start's administrator password meets the backend's password rule, and the script stops when the backend refuses it (#788)

- `scripts/quickstart.sh` generated `ADMIN_PASSWORD` with `openssl rand -base64 18`: 24 base64 characters, in which `+`
  and `/` are the only characters the backend's password rule counts as special. The backend refuses to seed the first
  administrator from a password without an upper-case letter, a lower-case letter, a digit and a special character: it
  logs `Failed to seed administrator from ADMIN_PASSWORD:` with the reason, creates no account, and starts anyway. Of
  2,000 outputs of that `openssl` command, 943 held neither `+` nor `/`. Run 200 times against docker and curl stubs
  whose backend log held that refusal, the previous script exited 0 each time and printed the `ADMIN_PASSWORD` of its
  `.env` as the `Login:` line.
- The script generates the password as `scripts/gen-dev-secrets.sh` does: up to 28 letters and digits from
  `openssl rand -base64 24`, then `Aa1!`. `scripts/test-quickstart.sh` generates its password the same way.
- After the backend answers `GET /health`, the script reads its log (`docker compose logs backend`). When the last seed
  line there (`Seeded administrator`, `ADMIN_PASSWORD not applied` or `Failed to seed administrator from
  ADMIN_PASSWORD`) is a refusal, the script prints the backend's reason and the next step, prints no credentials, and
  exits 1, on a first run and on a run over an existing install. For a refused password the step is to change
  `ADMIN_PASSWORD` in the install's `.env` and run the script again; for another reason it is to correct the cause,
  restart the backend and run the script again. With no seed line in the log, the script prints the credentials as
  before.
- Tests: `apps/backend/cmd/server/quickstart_admin_seed_test.go` runs the script against docker, curl and sleep stubs.
  It checks the `ADMIN_PASSWORD` the script writes against the backend's password rule, once from an `openssl` stub
  whose output is all lower-case letters and 24 times with the real `openssl`, and seeds an administrator from the
  first run's `.env` through `seedAdmin`. It hands the script the lines `seedAdmin` logs, and the line `main` logs for
  the error `seedAdmin` returns, for a refused password, a missing admin organization, a seeded administrator and one
  that already exists, and checks the exit status, the reason and step printed, and that no credentials follow a
  refusal. On the previous script the lower-case case failed, 11 of the 24 real `openssl` passwords failed the rule,
  and the three refusal cases exited 0.

### Fixed — `POST /api/v1/agents` answers 400 naming the members a request leaves out, and the dashboard's API reference example registers an agent (#790)

- A request to `POST /api/v1/agents` without `name`, `displayName` or `agentType`, or with an `agentType` the server
  does not register, was answered `500` with `An internal error occurred. Please try again later.`, which names
  nothing to change. Measured on the previous code with six bodies: the API reference's example, a body without
  `displayName`, one without `agentType`, one without `name`, `{}`, and one with `"agentType": "automation_bot"`.
- Such a request is now answered `400` with a line that names every required member it left out, under the names
  the API reference documents: `displayName is required`, `displayName and agentType are required`,
  `name, displayName and agentType are required`. An `agentType` the server does not register is answered `400` with
  `agentType must be one of: ` and the 21 types it registers, in alphabetical order, without the value sent. A
  refused request does not reach the agents table. The snake_case members `display_name` and `agent_type` are still
  read.
- The agent service returns these refusals as a declared error, `AgentRequestError`, and the registration handlers
  match it with `errors.As` and answer the line it states.
- The dashboard's API reference (`apps/web/lib/api-documentation.ts`) listed `name` and `type` as the required
  members of `POST /api/v1/agents`, with an optional `tags`, and its example sent `name`, `type`, `description` and
  `tags`. The request the handler binds has no `type` or `tags` member, and the example was answered `500`. The entry
  now marks `name`, `displayName` and `agentType` as required and no longer lists `type` or `tags`. Its example sends
  `name`, `displayName`, `agentType` and `description`, and registers an agent.
- Tests: `apps/backend/internal/interfaces/http/handlers/agent_create_required_members_test.go` posts the six bodies
  to the server's handler and agent service and checks each `400` line, and posts the API reference's example and
  checks that it is answered `201` and that the agent stored carries the members it sends. On the previous code each
  of the six bodies was answered `500`, and on the previous reference entry the example was refused.
  `apps/web/lib/api-documentation.test.ts` checks the entry's required members and the members its example sends.

### Fixed — the deployment guide's From source section creates the first administrator and says how to sign in (#791)

- The From source section of `docs/DEPLOYMENT.md` had the reader write `.env` with `scripts/gen-dev-secrets.sh`, start
  `postgres`, `redis`, `backend` and `frontend` and check `GET /health`, then went on to the Python SDK: it ran no step
  that creates an account and named no email and no password, so a reader who built the stack from source had nothing
  to sign in with. The section now runs `docker compose run --rm bootstrap` after the health check and says to sign in
  at `http://localhost:3000` as `admin@opena2a.org` with the `DEFAULT_ADMIN_PASSWORD` value in `.env`: the script
  writes that variable, the `bootstrap` service passes it to that one run and prints no password it was given, and when
  the variable is unset bootstrap generates a password and prints it once. The section also says that bootstrap marks
  the account for a password change, which the dashboard asks for at the first sign-in, and that the `bootstrap`
  profile keeps the service out of `docker compose up`.
- The command the report quotes, `docker compose up -d aim-postgres aim-redis aim-backend aim-frontend`, is the one the
  README of `platform-v1.0.0` carries. Compose 5.5.1 answers it with `no such service: aim-postgres`; the section on
  `main` already read `docker compose up -d postgres redis backend frontend`, and the README links to the section
  instead of carrying it. `apps/backend/cmd/server/compose_docs_names_test.go` checks the service names in the
  documented `docker compose` commands against the compose files.
- Test: `apps/backend/cmd/server/from_source_sign_in_docs_test.go` reads the service whose entrypoint runs
  `aim-bootstrap` from `docker-compose.yml`, the variables `scripts/gen-dev-secrets.sh` writes and the administrator
  email from `apps/backend/cmd/bootstrap/main.go`, and fails when the section does not run that service after a
  `docker compose up` that starts the services it depends on, omits the email or the dashboard URL, or names a password
  variable the script does not write.

### Fixed — the README's Quick start says the PyPI release does not yet print a sign-in code, and links a self-hosted install from the dashboard (#789)

- The Quick start installs the SDK with `pip install "aim-sdk[rich]"` and says `aim-sdk login` prints a code you
  approve on the dashboard's `/device` page, which is what the SDK in this repository does. The release on PyPI,
  aim-sdk 2.0.3, has no device sign-in: with it, `aim-sdk login --url http://localhost:8080` prints `Opening browser for
  authentication...` and an `http://localhost:8080/auth/login?response_type=code&...` address, then waits, and the API
  registers no `/auth/login` route. The sentence now says the PyPI release does not yet, and links the self-hosted
  install and #789.
- `docs/quick-start.md` gives that install under Install the SDK from your dashboard: Developers, then SDK & docs
  (`/dashboard/sdk`), Download Python SDK, then, from a folder outside the repository clone,
  `unzip ~/Downloads/aim-sdk-python-v*.zip`, `cd aim-sdk-python` and `pip install -e ".[rich]"`. The archive holds the
  SDK from the `sdk/python` tree the server image was built with and the user's sign-in in
  `aim-sdk-python/.aim/sdk_credentials.json`, which the SDK reads from any directory once installed editable. The steps
  it replaces named Settings → SDK Download, a "Download SDK" button, `unzip aim-sdk-python.zip` and
  `pip install -r requirements.txt`.
- A Python SDK test (`sdk/python/tests/test_readme_self_hosted_sign_in.py`) reads the README sentence that names
  `/device`, the dashboard page's path and button label from `apps/web/app/dashboard/sdk/page.tsx`, and the archive's
  name, folder and sign-in path from the server's download handler, and compares them with the README and the install
  steps.

### Fixed — `GET /api/v1/agents` answers `400` to a `limit` that is not a positive integer (#792)

- An authenticated `GET /api/v1/agents` with `limit=0`, `limit=-5`, `limit=abc`, `limit=1.5` or
  `limit=99999999999999999999` is answered `400 Bad Request` with `{"error": "limit must be a positive integer"}`,
  and no statement reaches the database. Run against the server's handler, agent service and agent repository over a sqlmock connection, each of
  these requests was answered `200` by the previous handler, with every agent of the organization from a statement
  with no `LIMIT` clause, and no `limit` in the answer: the handler read such a limit as `0`, which the repository
  reads as no paging. A request without `limit` still lists every agent, and an `offset` that is negative or not a
  number still reads as `0`.
- The Python SDK's `list_agents` and the Java SDK's `listAgents` send the limit they are given, capped at 100, so a
  limit of 0 or less from either is now answered `400` where it listed every agent.
- `docs/API.md` and `docs/guides/API_REFERENCE.md` give the `400` answer under the `limit` parameter.
- Tests: `apps/backend/internal/interfaces/http/handlers/agent_handler_list_filter_test.go` sends each of these
  limits, and a limit of 0 with an offset and with a status filter, through the server's handler, service and
  repository over sqlmock; on the previous handler all 7 cases fail. `TestAgentHandler_ListAgents_InvalidPagingParams`
  holds an offset of `-5` or `abc` at `0`, now beside a limit of 20.

### Fixed — the flight-search example's command loop returns when its standard input ends (#792)

- `examples/flight-search-agent/flight_agent.py` caught the `EOFError` that `input()` raises once standard input
  has ended as any other error: it printed `Error: EOF when reading a line` and asked for the next command. Run with
  standard input closed (`< /dev/null`), the previous loop printed that line 100 times, asking for a command before
  each, until the run was stopped. The loop now prints its goodbye line and returns at the first `EOFError`, with no
  error line.
- Test: `sdk/python/tests/test_flight_search_example_verification.py` gives the loop an `input()` that raises
  `EOFError` and fails if the loop asks again; on the previous example it fails.

### Fixed: the register-agent dialog's Copy key button stays inside the API key card at a 375px viewport (#780)

- After an agent is registered, the register-agent dialog shows a card titled "Your API key (for manual integration)"
  with the key field, the button that reveals the key and the Copy key button in one row. The key field kept its
  intrinsic width in that row, so at a phone width the row ran past the card. The field now shrinks to the width the
  row leaves it, and the buttons stay inside the card. The key shown and the text the button copies are unchanged.
- Measured in headless Chromium on macOS with the dashboard's stylesheet, with the key field masked as a password: on
  the previous code at 375x812 the field was 175 CSS px wide, the card's content edge was at x=317, and the Copy key
  button was drawn at x=289..376, 48 px tall, or at x=289..387.58 reading Copied. With this change the same
  measurement at 375x812 finds the field 91.86 px wide and the button at x=205.86..317, 34 px tall, reading Copy key,
  and the field 104.42 px wide and the button at x=218.42..317 reading Copied. At 1280x800 the measurement is the same
  before and after the change, with the button's right edge at the card's content edge, x=982.
- Test: `apps/web/components/modals/register-agent-modal.api-key-card.test.tsx`; its layout measurement runs where a
  Chromium build for the installed Playwright is present.

### Fixed — the API endpoint summary names the four dependencies `GET /health/ready` reports (#781)

- `docs/guides/API_ENDPOINT_SUMMARY.md` described `GET /health/ready` as "Readiness check (database + redis)". The
  body's `dependencies` object also reports `siem` and `revocationState`. The row now names all four: `database`, which
  is required and decides the status (200 when its ping succeeds within 2 s, 503 otherwise), and the optional `redis`,
  `siem` and `revocationState`, each of which sets `"degraded": true` without changing the status when it is
  `unavailable`.
- The section's implementation pointer named `apps/backend/cmd/server/main.go:112-144`, lines that register none of its
  three routes. It now names `main.go`, which registers them, `apps/backend/cmd/server/health_ready.go`, which builds
  the readiness body, and the body's contract, `apps/backend/contracts/health-ready.schema.json`.
- A backend test (`apps/backend/cmd/server/health_docs_paths_test.go`) reads the dependencies from the readiness
  handler's body, with no optional dependency set up and with every one set up, and fails when the row does not name
  one of them, or does not write "`<name>` (required" for exactly the ones the body reports as required.

### Added: the ATC verification specification gives a next step for each ATC error code, and shows the server's `503` answer with `Retry-After`

- The error table of section 4.2 of `docs/specs/atc-verification-v1.md` gave each code a status and a meaning, and no
  step to take. It has a Next step column, filled in each of its 10 rows. For `atc_revocation_unknown` the step is to
  send the request again after the `Retry-After` seconds, with the same ATC while it is valid, since the ATC was not
  judged; for `atc_revoked` it is not to send the ATC again.
- Section 4.2 shows the server's `503` answer to an ATC whose revocation state cannot be read: its status line, its
  `Content-Type`, `Retry-After: 5`, `X-ATC-Supported` and `X-ATC-Version` headers, and its body.
- Tests: `apps/backend/cmd/server/atc_spec_test.go` compares the status of each code in the error table with the
  status the server's ATC middleware answers for it, checks that the table's codes are the codes the ATC verifier
  returns and that each row gives a next step, and compares the `503` answer section 4.2 shows with the middleware's
  answer, through the server's verifier, to an ATC whose agent's status cannot be read. On the previous specification
  both tests failed. `apps/backend/internal/infrastructure/atc/crl_client_test.go` gains a test and a case for two
  behaviours of the revocation list client no test held: beside a verified signature by a listed key, a signature by
  a key the discovery document does not list, whose value does not verify, leaves the list held; and a discovery
  document longer than its 65,536 byte bound is refused as shape and the list is not requested.

### Fixed: a Registry credential revocation request waiting for a retry is sent after the server starts again

- The request asking the Registry to revoke a revoked agent's credentials, `POST /internal/atc/<agent id>/revoke`,
  was held in the server's memory between its attempts, as was the one issuance sends again for an agent revoked
  while the Registry was issuing. A server that stopped or restarted ended every request waiting for a retry, and
  the Registry was not asked again.
- Migration `133_registry_atc_revocations.sql` creates `registry_atc_revocations`. With `REGISTRY_ATC_TOKEN` set, the
  request is stored there once the revocation is stored and before it is sent, and a sender the server starts with
  its background jobs sends it: at once, then on the same retry schedule, 5 seconds, 30 seconds, 2 minutes,
  10 minutes and 30 minutes in turn, with the time of the next attempt kept in the row on the database clock. The
  sender looks for due requests every 5 seconds and whenever its server stores a request. A row is deleted when the
  Registry answers `2xx` or when the attempts end.
- A request still waiting when the server stops stays stored, and the next server to start sends it once it is due.
  A request in flight when the server stops is ended, and is due again 60 seconds after it was started. Every replica
  reads the same rows: a server takes a due request with `FOR UPDATE SKIP LOCKED` and holds it for 60 seconds while
  it sends it, so no other server takes it during that time.
- When issuance stores the request again, the stored request is replaced, and the answer to a request already in
  flight neither deletes nor reschedules it, so the later request is still sent.
- A request that cannot be stored is sent from memory, as before, and the failure is logged. With
  `REGISTRY_ATC_TOKEN` unset nothing is stored, as no request is sent. A request that follows a failed one is logged
  when the Registry revokes the credentials: `agent <id>: the Registry revoked the agent's credentials (attempt <n>)`.
- Tests: `apps/backend/internal/application/registry_atc_revocations_test.go` (a request waiting for a retry, and one
  in flight, when the server stops is sent by the next server on the same store; the stored retry schedule; the end
  of the attempts; a request stored again while one is in flight; a request the store refuses is sent from memory;
  `RevokeAgent` and issuance store their request),
  `apps/backend/internal/infrastructure/repository/registry_atc_revocation_repository_test.go` (the statements, their
  parameters and the database clock), `apps/backend/internal/infrastructure/registry/atc_client_test.go`
  (`Configured`), and, against PostgreSQL with `-tags=integration`,
  `apps/backend/internal/infrastructure/repository/registry_atc_revocation_integration_test.go` (the hold, the
  retry, a request stored again, concurrent claims taking each request once, a request claimed through a new
  database handle) and `apps/backend/internal/application/registry_atc_revocations_integration_test.go` (a real
  agent revoked while a local Registry answers 503, the first server stopped, and the request sent by the next
  server to a local Registry that answers 200).

### Fixed: no Agent Trust Credential is issued for a revoked agent

- The issuance service behind `POST /api/v1/agents/:id/atc` did not read the agent's status, so issuance for a
  revoked agent went ahead as for any other agent: its trust score was computed and the Registry was asked to sign a
  credential for it.
- A revoked agent is now refused before its trust score is computed or the Registry is asked. The route answers `409`
  with `reasonCode` and `code` `agentRevoked` and the error
  `This agent is revoked, and no Agent Trust Credential is issued for a revoked agent: register a new agent to replace it`.
- The agent is read again once the Registry has answered. An agent revoked while the Registry was issuing gets the
  same `409` and no credential, and AIM sends the Registry `POST /internal/atc/<agent id>/revoke` again, on the retry
  schedule the revoke route's request follows, so the Registry receives a revocation request after the issuance. It
  is sent when the issuance's answer did not arrive as well, as the Registry may have signed the credential. When the
  agent cannot be read again, no credential is handed on and no revocation is sent.
- The server hands the issuance service the Registry client it already hands the agent service.
- Tests: `apps/backend/internal/application/atc_issuance_revoked_agent_test.go` (a revoked agent gets no credential,
  no trust score and no Registry request; an agent revoked during issuance gets no credential and a second
  revocation request, also when the issuance's answer did not arrive; an agent not revoked gets its credential and
  no revocation; a failed second read hands nothing on; and, through the Registry client the server wires, a local
  Registry receives the revocation after the issuance) and
  `apps/backend/internal/interfaces/http/handlers/atc_issuance_handler_test.go` (the `409` body for an agent revoked
  before the request and for one revoked while the Registry issues).

### Added: `/metrics` carries `aim_atc_revocation_list_age_seconds` and `aim_atc_revocation_list_entries`, the age and the entries of the revocation list the server holds with `REGISTRY_URL` set

- With `REGISTRY_URL` set, `/metrics` carries two gauges of the list the refresher holds.
  `aim_atc_revocation_list_age_seconds` is the seconds since that list was last read: by the refresh that held it, or
  by a later refresh answered `304` to its ETag. `aim_atc_revocation_list_entries` is the number of entries in its
  `revocations` array, where an `agentDid` the list names twice counts twice. Neither gauge has a sample while no list
  is held, and without `REGISTRY_URL` `/metrics` carries no `aim_atc_revocation_` series. A scrape reads the list held
  and the server's clock, and sends no request to the authority.
- Against a TLS stub of an authority listening on 127.0.0.1, the age read 0 after the refresh that held a list, 90
  when the clock had moved 90 s with no refresh, 0 after a refresh answered `304` to the list's ETag, and 301 after a
  refresh, 301 s after that read, that the stub answered `503`; the list held was then no longer answered from. The
  entries read 3 for a list of three entries that names one `agentDid` twice, and 1 once a list of one entry was held.
- On the previous code, with a list held, the metrics listener's exposition carried no `aim_atc_revocation_` series.
- The two gauges pass promlint, the checks `promtool check metrics` runs.
- Tests: `apps/backend/internal/infrastructure/atc/crl_metrics_test.go`, against TLS stubs of an authority listening
  on 127.0.0.1, and `apps/backend/cmd/server/revocation_list_metrics_test.go`, which scrapes the metrics listener's
  app while the refresher of a TLS stub of an authority listening on 127.0.0.1 holds a list.

### Changed: the revocation list refresher reads the discovery document with `If-None-Match`, and reads the list in full when the document's keys change

- With `REGISTRY_URL` set, a request for the discovery document carries `If-None-Match` with the ETag of the last
  document the refresher used, when that document was read at the same address, and a `304` answer reuses the list
  address and the keys read from that document. The ETag of a document read at `/.well-known/opena2a` while
  `/.well-known/atp` answers 404 is sent to `/.well-known/opena2a` only. A `304` to a request that carried no
  `If-None-Match` is refused as `status`, for the discovery document as for the list. On the previous code a request
  for the discovery document carried no `If-None-Match`: against a TLS stub that sends an ETag with its document, the
  six requests of four refreshes carried none.
- A request for the list carries the ETag of the list held only while the discovery document lists the keys that list
  was checked under. When the document withdraws the key that signed the list held, lists its `keyId` with another
  public key, or lists another key beside it, the list is read in full and checked under the keys listed now: in the
  third case the list passes and is held again. On the previous code, after the document withdrew that key or listed
  its `keyId` with another public key, the list request carried the ETag of the list held, the stub answered `304`, and
  the list held still answered 9 minutes after its first read. Now such a list is refused as `signature`, the list held
  answers until 5 minutes after its last read, and a list signed by the key the document lists now is held at the next
  read.
- Tests: `apps/backend/internal/infrastructure/atc/crl_client_test.go`, against TLS stubs listening on 127.0.0.1.

### Added: `GET /health/ready` carries `dependencies.revocationState`, the state of the revocation list the server reads with `REGISTRY_URL` set

- `GET /health/ready` carries `dependencies.revocationState`, with `"required": false` and no `latencyMs`. It is
  `notConfigured` when `REGISTRY_URL` is unset. With `REGISTRY_URL` set it is `ok` while the list held is fit to answer
  from: last read at most 5 minutes ago, and at most 60 s past its `nextUpdate`. Otherwise it is `unavailable`,
  with one word in `reason`, and the response has `"degraded": true`. The entry does not change the HTTP status, which
  the database alone decides: 200 when it answers, 503 when it does not. The entry reads the refresher's record of its
  reads and sends no request.
- The words: `noList`, no list is held, and no read has ended or the last one ended with no answer before its deadline
  (the connection, TLS, or a redirect to an address that is not `https`); `status`, the discovery document or the list
  was answered with an HTTP status other than 200, or with 304 to a request without `If-None-Match`; `timeout`, a read
  had not ended by its deadline; `parse`, a document is not an I-JSON text (not JSON, not UTF-8, an object that names a
  member twice, a number beyond the binary64 range, or data after the value); `shape`, a document is longer than its
  bound or breaks the form of the list contract, a `nextUpdate` that is not after `thisUpdate` included; `signature`,
  the list's signatures do not verify under the keys the discovery document lists; `rollback`, the list's `thisUpdate`
  is before that of the list held; `expired`, the list's `thisUpdate` is more than 60 s ahead of the server's clock or
  its `nextUpdate` passed more than 60 s ago, or a list is held that is no longer fit to answer from and the last read
  held it, read it again or ended with no answer; and `refresherStalled`, no read has ended for more than 8 minutes,
  counted from the refresher's start until its first read ends.
- The refresher's log line names a refused read with the same words, and with `fetch` for a read that ended with no
  answer before its deadline:
  `Revocation list: refresh failed (status): the list at <address> answered 503; no list fit to answer from is held; next refresh in <wait>`.
  On the previous code the line named `fetch` for an answer other than 200 too, `shape` for a body that is not an
  I-JSON text, and `time` for a rollback, for a list outside the clock skew bound and for a `nextUpdate` that is not
  after `thisUpdate`: the outcomes the tests of the list client and the refresher asserted.
- The server starts the refresher before it registers `GET /health/ready`, and hands the route the refresher's state.
  On the previous code the route carried no entry for the list: with a list held, it answered 200 with the
  dependencies `database`, `redis` and `siem`.
- Tests: `apps/backend/cmd/server/health_ready_revocation_test.go`, against the route built with fixed states and with
  the refresher of a TLS stub of an authority listening on 127.0.0.1, and
  `apps/backend/internal/infrastructure/atc/crl_refresher_test.go` and
  `apps/backend/internal/infrastructure/atc/crl_client_test.go`, against TLS stubs listening on 127.0.0.1.

### Added: with `REGISTRY_URL` set, the server reads the revocation list an authority's discovery document names, and `ATC_CRL_ENDPOINT` has no default

- `REGISTRY_URL`, unset by default, is the base URL of an authority's discovery document. When it is set, the server
  starts a refresher that reads the document at `/.well-known/atp`, or at `/.well-known/opena2a` when the first answers
  404, then the revocation list at the document's `endpoints.revocations`, and holds the list when it passes the
  checks of the list client: an Ed25519 signature by a key the document lists, its `entryCount` and its times. The
  first read starts with the server. The next starts 4 minutes after a read that holds a list, or 60 s before the
  `nextUpdate` of the list held when that is sooner. After a read that fails or whose list is refused, and when the
  next read would start in less than 1 s, it starts after a wait drawn at random from 1 s to 30 s. No request the
  server answers fetches the list or decides on it: the ATC verifier reads revocation from the agent store, as
  before.
- `ATC_CRL_ENDPOINT` has no default. Its default was `https://registry.opena2a.org/api/v1/crl/latest`, which is not a
  route of the Registry route table pinned in `apps/backend/testdata/registry-routes.txt`; that table carries
  `GET /api/v1/trust/revocations`, `GET /.well-known/atp` and `GET /.well-known/opena2a`. With `REGISTRY_URL` set, the
  list is read from a set `ATC_CRL_ENDPOINT` instead of the address the discovery document names, and its signature
  is still checked under the document's keys. Without `REGISTRY_URL`, `ATC_CRL_ENDPOINT` is not read.
- The server logs one startup line for the list. With `REGISTRY_URL` unset it is
  `Revocation list: not read (REGISTRY_URL is not set)`, or
  `Revocation list: not read (REGISTRY_URL is not set; ATC_CRL_ENDPOINT is read only with it)` when
  `ATC_CRL_ENDPOINT` is set. With `REGISTRY_URL` set it is
  `Revocation list: read in the background from the authority at https://registry.example.com`, or, with
  `ATC_CRL_ENDPOINT` set too,
  `Revocation list: read in the background from https://mirror.example.com/revocations, with the keys of the authority at https://registry.example.com`.
  The line names the scheme and host of `REGISTRY_URL` and the scheme, host and path of `ATC_CRL_ENDPOINT`, and no
  user name and no query. When `REGISTRY_URL` is set and it or a set `ATC_CRL_ENDPOINT` is not an `https` URL with a
  host, starting the list returns `REGISTRY_URL must be an https URL with a host`, or the same words for
  `ATC_CRL_ENDPOINT`, and the server passes the error to `log.Fatal` after the words `Revocation list: `.
- The refresher logs a line after its first read and after each read that ends in another state than the one before
  it: `Revocation list: holds a list, thisUpdate <time>, nextUpdate <time>; next refresh in <wait>`, or
  `Revocation list: refresh failed (<outcome>): <cause>; <held>; next refresh in <wait>`, where the outcome is
  `status`, `timeout`, `fetch`, `parse`, `shape`, `signature`, `rollback` or `expired`, and `<held>` is
  `the list held is still answered from` or `no list fit to answer from is held`.
- `internal/config` reads `REGISTRY_URL`. Its environment table gains five `REGISTRY_URL` rows, and its
  `ATC_CRL_ENDPOINT` rows record an empty setting when the variable is unset or empty, and the startup line above.
- Tests: `apps/backend/internal/infrastructure/atc/crl_refresher_test.go` and
  `apps/backend/cmd/server/revocation_list_test.go`, against TLS stubs of an authority listening on 127.0.0.1, and
  the environment table tests of `apps/backend/internal/config` and `apps/backend/cmd/server`.

### Security: an ATC whose agent is suspended, revoked or not stored is refused, and an ATC whose agent's status cannot be read is answered 503 with `Retry-After`

- The ATC verifier checks revocation, step 8 of `docs/specs/atc-verification-v1.md`, against the status of the agent
  the ATC names, read from the server's store once per verification, a token the verifier has cached included, under
  a 2 second deadline. It verifies the `Authorization: ATC` header on the `/api/v1/agents` routes and
  `POST /api/v1/secrets/resolve`, and an `atcId` in that route's body. An ATC is revoked when no agent has its
  `agentId`, or when the agent's status is not `verified` or `pending`: the statuses the signature, API-key and
  service-token authentication refuses. A revoked header ATC is answered `401` and `atc_revoked`; a revoked body
  `atcId` is answered the route's `403` refusal, and its secrets audit entry names `atc_revoked`.
- On the previous code the verifier checked revocation against a list fetched from `ATC_CRL_ENDPOINT`, and counted a
  failed first fetch as not revoked. With that endpoint set to an address where nothing listens, the list client's
  `IsRevoked` returned `false` with no error, and the verifier the server builds accepted an ATC without reading
  the state of the agent it names.
- When the status read fails or has not answered by its deadline, the answer is `503` with `Retry-After: 5` and
  `{"error":"atc_revocation_unknown","message":"the revocation state of this ATC could not be read"}`, for the header
  and for the body `atcId`. For the body `atcId`, the secrets audit entry is a denial whose reason names
  `atc_revocation_unknown`. The verifier writes the cause to the server log.
- The server builds no revocation list client from `ATC_CRL_ENDPOINT`, so the variable has no effect, and the start
  check that stopped the server on a value that is not an `https` URL is removed with it.
- Tests: `apps/backend/internal/infrastructure/atc/revocation_test.go`,
  `apps/backend/internal/interfaces/http/middleware/atc_auth_revocation_test.go`,
  `apps/backend/internal/interfaces/http/handlers/secrets_resolve_atc_revocation_test.go`,
  `apps/backend/internal/application/secrets_resolve_atc_revocation_test.go`,
  `apps/backend/internal/infrastructure/repository/agent_status_read_test.go`.

### Fixed — the MCP server page lays out within a 375px phone viewport, and its Integration tab opens on a tap

- On the MCP server page (`apps/web/app/dashboard/mcp/[id]/page.tsx`), the header row holding the server's icon,
  name, URL and the Edit and Delete actions did not wrap, and its two groups did not shrink below the name's and the
  URL's unbroken width, so the row laid the page out wider than the phone. Measured in Chromium at a 375x812 viewport
  under mobile emulation, with every API call answered by the readability sweep's route mocks
  (`apps/web/scripts/ux-sweep.mjs`), the previous code laid the page out at 521 CSS px (`window.innerWidth`) with the
  URL `https://mcp.example.com` and at 560 with `https://mcp-server.example.com/mcp/v1/sse`; shown zoomed out, the
  Integration tab's right edge sat at 852 CSS px inside the scrolling tab list, and in one of two runs a tap at the
  tab did not select it until a mousedown event was dispatched to it. The row now wraps, its groups shrink, a long
  name breaks within the word where it overflows and the URL breaks anywhere, so the page keeps the device width.
- On the Details tab, the server ID, name, URL and description break inside their grid column; on the Attestations
  tab, the empty state's two cards take one column below the `md` breakpoint instead of a column as wide as their
  unwrapped code lines.
- With the change, measured as above with the URLs `https://mcp.example.com` and `https://mcp-server.example.com/mcp/v1/sse`,
  and with the name `production_mcp_gateway_server_us_east_1` and the URL
  `https://mcp-gateway.example.com/api/v1/mcp/servers/production/sse` on the Details, Capabilities, Connected agents,
  Attestations and Integration tabs, `window.innerWidth` reads 375 and the document's horizontal overflow 0, a tap
  selects the Integration tab, and the cURL example scrolls inside its card: its box ends at 338 CSS px and its
  content measures 925 CSS px against a 301 CSS px box.
- Test: `apps/web/tests/e2e/mobile-mcp-detail.spec.ts` opens the page at 375x812 under mobile emulation as a signed-in
  admin over route mocks, with the URL `https://mcp-server.example.com/mcp/v1/sse`, and checks that `window.innerWidth`
  and the document's scroll width are 375 as loaded and on the Integration tab, that the tab list ends inside the
  viewport, that a tap selects the Integration tab and shows its panel, and that the cURL example ends inside the
  viewport and scrolls inside its card. On the previous code, the same emulation and URL measured `window.innerWidth`
  at 560 as loaded.

### Fixed — the register-agent dialog's Copy button sits above the verification command instead of over its first line (#776)

- After an agent is registered with a capability, the manual integration step of the register-agent dialog shows a
  cURL command that verifies the capability. The command's Copy button was positioned in the top right corner of the
  command's block, over its first line. It now has a row of its own at the top of the block, and the command starts
  below that row. The command shown and the text the button copies are unchanged.
- Measured in headless Chromium on macOS with the dashboard's stylesheet, for an agent ID of 36 characters and the API
  at `http://localhost:8080`, with the button reading Copy: on the previous code the button was drawn over the last
  characters of the command's first line, `555555` from the last group of the agent ID at 1280x800, and `/localho` in
  the API address at 375x812. With this change the same measurement finds no character of the command under the
  button at either size, whether it reads Copy or Copied.
- Test: `apps/web/components/modals/register-agent-modal.copy-button.test.tsx`; its layout measurement runs where a
  Chromium build for the installed Playwright is present.

### Fixed: the MCP server setup guide and the register-agent dialog's verification command name the API the dashboard signs in to

- The setup guide on the Integration tab of an MCP server's page wrote its cURL commands for an address of its own:
  port 8080 on the dashboard's host name when that name is `localhost` or `127.0.0.1`, and the dashboard's origin
  otherwise. After an agent is registered with a capability, the register-agent dialog's manual integration step
  showed, and copied, a cURL command for the dashboard's origin with the first `-frontend` replaced by `-backend` and
  the first `:3000` by `:8080`. Evaluated from the previous code, on a dashboard opened at `http://localhost:3001`, the
  second stack's port in `docs/DEPLOYMENT.md`, the guide named `http://localhost:8080` and the dialog named
  `http://localhost:3001`; on one opened at `https://aim.example.com`, both named `https://aim.example.com`. The SDK
  setup guide component, which no page renders, wrote the `api_url` of its example with the guide's rule.
- The guide, the dialog and the SDK setup guide take the address from `apiBaseUrl()` in
  `apps/web/lib/api-base-url.ts`: the address the API client resolved, which is the URL the page hands the browser
  when `NEXT_PUBLIC_API_URL` is set in the dashboard's environment.
- Tests: `mcp-setup-guide.api-url.test.tsx`, `sdk-setup-guide.api-url.test.tsx` and
  `register-agent-modal.api-url.test.tsx` open each on `http://localhost:3001` with the page handing the browser
  `http://localhost:8081`, and on `https://aim.example.com` with `https://api.example.com`. The dialog test registers
  an agent with a capability and checks the command shown and the text the copy button copies. On the previous code,
  the 6 tests fail.
- Measured in Chromium on an `aim-dashboard` image built from this tree with
  `--build-arg NEXT_PUBLIC_API_URL=http://localhost:8080`, and two stacks started from `docker-compose.quickstart.yml`
  with the published `aim-server` image (revision `3ab8285e`) and every port bound to `127.0.0.1`, the second with
  `AIM_BACKEND_PORT=8081` and `AIM_FRONTEND_PORT=3001`. Accounts were registered through each dashboard's register
  page and approved through `AIM_PLATFORM_ADMINS`; sign-in was typed and clicked with keyboard and mouse input. On
  the second stack's dashboard, registration and sign-in went to `http://localhost:8081` (201, then 200), the MCP
  server guide named `http://localhost:8081`, and the dialog showed and copied a command for
  `http://localhost:8081`. On the first stack's dashboard, sign-in went to `http://localhost:8080` (200), and the
  guide and the dialog named `http://localhost:8080`. The second stack was then started again with
  `FRONTEND_API_URL=http://api.aim.example.test:18888`, and `FRONTEND_URL` and `ALLOWED_ORIGINS` set to
  `http://aim.example.test:18888`, behind a reverse proxy on `127.0.0.1:18888` that passes each host name to the
  stack's loopback port; the browser resolved both names to `127.0.0.1` and treated both origins as secure, as TLS
  would. Opened at `http://aim.example.test:18888`, the dashboard signed in at `http://api.aim.example.test:18888`
  (200), and the guide and the dialog named that address. In each run on that image, every request to an `/api/v1/`
  path went to that stack's API address, and the browser logged no CORS or Content-Security-Policy refusal.

### Fixed: revoking an agent asks the Registry to revoke the trust credentials it issued for the agent

- `POST /api/v1/agents/:id/revoke` revoked the agent in OpenA2A AIM (Agent Identity Management) only. The Registry
  signs the Agent Trust Credentials `POST /api/v1/agents/:id/atc` issues, and no request asked it to revoke the ones
  it had issued for the revoked agent.
- Once the revocation is stored, AIM now sends `POST /internal/atc/<agent id>/revoke` to the Registry at
  `REGISTRY_BRIDGE_URL` (default `https://api.oa2a.org`), with the `REGISTRY_ATC_TOKEN` bearer issuance uses and the
  body `{"reason":"revoked"}`. `revoked` is the reason every entry of AIM's own revocation list,
  `GET /api/v1/revocations`, carries. The request runs on its own goroutine: the revoke route answers without waiting
  for the Registry, and a Registry that refuses or cannot be reached leaves the agent revoked in AIM.
- A failed connection, a timeout, a `408`, a `429` or a `5xx` answer is followed by another request after 5 seconds,
  30 seconds, 2 minutes, 10 minutes and 30 minutes in turn, six requests at most; any other answer that is not `2xx`
  ends the attempts. Each failed request is logged with the agent ID. The request is stored before it is sent, and a
  request still waiting when the server stops is sent by the next server to start. With `REGISTRY_ATC_TOKEN` unset no
  request is sent, as no credential is issued without it.
- Tests: `apps/backend/internal/infrastructure/registry/atc_client_test.go` (the request's method, path, bearer and
  body, an agent ID holding a slash sent as one path segment, no request without a token or an agent ID, and which
  answers are retried) and `apps/backend/internal/application/agent_registry_revocation_test.go` (a revocation reaches
  a local Registry through the client the server wires, `RevokeAgent` returns before the Registry answers and leaves
  the agent revoked when the Registry fails, nothing is sent for a revocation that is refused or not stored, and the
  retry and stop rules).

### Changed: the `aim-dashboard` image takes no `NEXT_PUBLIC_API_URL` build argument, and its browser code holds no `http://localhost:8080`

- `infrastructure/docker/Dockerfile.frontend` declared `ARG NEXT_PUBLIC_API_URL` and set `ENV NEXT_PUBLIC_API_URL`
  from it before `npm run build`, so `next build` read the build argument the image workflow passes
  (`NEXT_PUBLIC_API_URL=http://localhost:8080`) and wrote it into the browser's code. Built from the previous commit
  with that argument, the image held `http://localhost:8080` in 2 files under `.next/static`, and a container started
  from it with no `NEXT_PUBLIC_API_URL` and opened on `127.0.0.1` sent sign-in to `http://localhost:8080`.
- The file declares no such argument and sets no such variable. A `--build-arg NEXT_PUBLIC_API_URL` reaches no
  instruction of the file, and the build prints no warning for it (measured with Docker 29.7.2); the dashboard takes
  the URL from `NEXT_PUBLIC_API_URL` in the container's environment, which both compose files set. A dashboard started
  with none calls the `-backend` host of a `-frontend` host name, or port 8080 on the host name it was opened at.
  `next build` in the image build reads no `NEXT_PUBLIC_API_URL`, so the check in `apps/web/lib/local-api-origin.ts`
  fails the image build when its browser code holds `http://localhost:8080`, with the image workflow's build argument
  as without it.
- `docs/guides/DEPLOYMENT_CHECKLIST.md` says the image takes no build argument for the URL.
- Tests: `sdk/typescript/tests/dockerfile-frontend-takes-no-api-url.test.ts` fails when a line outside comments in the
  file the `build-frontend` job of `docker-publish.yml` builds, the file `docker-compose.yml` builds, or the twin under
  `apps/backend` names `NEXT_PUBLIC_API_URL`. It runs the same check over the earlier text and over planted forms: an
  `ARG` alone or with a default, an `ENV` in either stage or on a continuation line, a `RUN` that assigns the
  variable, and a heredoc that writes it to `.env.production`. On the previous file, 2 of its 6 tests fail, naming
  lines 11 and 12.
- Measured on images built from this tree and from the previous commit, each with
  `--build-arg NEXT_PUBLIC_API_URL=http://localhost:8080`: this tree's image holds no `http://localhost:8080` under
  `.next/static`. In Chromium, signing in with keyboard and mouse input on a container started from it with no
  `NEXT_PUBLIC_API_URL` and opened on `127.0.0.1` sent the request to `http://127.0.0.1:8080`, under
  `connect-src 'self' 127.0.0.1:8080`; started with `NEXT_PUBLIC_API_URL=http://localhost:8081`, the page set
  `window.__RUNTIME_CONFIG__` to `{"apiUrl":"http://localhost:8081"}` and sent the request there; started with no
  `NEXT_PUBLIC_API_URL` and opened on `aim-frontend.example.test`, it sent the request to
  `http://aim-backend.example.test`, under a `connect-src` that lists `aim-backend.example.test` and no
  `http://localhost:8080`. No page's `connect-src` refused a request. A copy of the file with
  `ENV NEXT_PUBLIC_SUPPORT_EMAIL=http://localhost:8080` in the build stage failed to build with and without the build
  argument, naming the chunk that holds the address.

### Fixed: `docker-compose.yml` takes the dashboard's API URL from `FRONTEND_API_URL`, and the deployment guide says to set it behind a reverse proxy

- `docker-compose.yml` set `NEXT_PUBLIC_API_URL=http://localhost:${AIM_BACKEND_PORT:-8080}` on the dashboard's container,
  with no variable to change it: rendered by `docker compose config` with `FRONTEND_API_URL=https://api.example.test`
  and `AIM_BACKEND_PORT=8081`, it handed the dashboard `http://localhost:8081`. It now reads `FRONTEND_API_URL` as
  `docker-compose.quickstart.yml` does, and hands the dashboard `https://api.example.test` in that case and
  `http://localhost:8081` without it.
- The "Enable HTTPS (Let's Encrypt)" section of `docs/DEPLOYMENT.md` named no variable for the API's public origin. It
  now says to set `FRONTEND_API_URL` to the API's public origin, and `FRONTEND_URL` and `ALLOWED_ORIGINS` to the
  dashboard's, asks for one certificate covering both host names, and describes the `-frontend` host name rule as the
  fallback it is: it applies to a dashboard container with no `NEXT_PUBLIC_API_URL` in its environment, and both
  compose files set one whether or not `FRONTEND_API_URL` is set. With a dashboard image built from this tree, a stack
  started from either compose file and opened on a `-frontend` host name calls the URL the compose file hands it, not
  the `-backend` host; such a deployment sets `FRONTEND_API_URL` to the `-backend` origin.
- The Docker section of `docs/DEPLOYMENT.md` says the same for a dashboard opened from another machine or through a
  reverse proxy, names the URL its second stack's dashboard is handed (`http://localhost:8081`), and says that the
  `aim-dashboard` image `latest` pointed to on 2026-10-17 (revision `3ab8285e`) ignores `FRONTEND_API_URL`. It gives
  a `curl` command that prints the policy a stack's sign-in page carries.
- The `.env` reference in `docs/DEPLOYMENT.md` named `CORS_ALLOWED_ORIGINS`, which the backend does not read; it now
  names `ALLOWED_ORIGINS`, which `apps/backend/internal/config/config.go` reads. `.env.example` lists `FRONTEND_URL`,
  `ALLOWED_ORIGINS` and `FRONTEND_API_URL`, commented out, as `.env.quickstart` does. The comment on the dashboard's
  `NEXT_PUBLIC_API_URL` entry in both compose files names `FRONTEND_API_URL`.
- Tests: `sdk/typescript/tests/quickstart-ports-and-project-from-environment.test.ts` renders `docker-compose.yml`
  with `FRONTEND_API_URL`, `FRONTEND_URL` and `ALLOWED_ORIGINS` set, and with the three values the HTTPS section gives,
  and checks the dashboard's and the backend's environment; it checks that the Docker section names the three
  variables and the URL its second stack is handed, that `.env.example` and `.env.quickstart` name the variables their
  compose file reads, and that the guide's CORS variable is `ALLOWED_ORIGINS`. On the previous files, these 5 tests
  fail and the file's other 18 pass.
- Measured on an `aim-dashboard` image built from this tree with `--build-arg NEXT_PUBLIC_API_URL=http://localhost:8080`,
  as the image workflow builds it. Started with `NEXT_PUBLIC_API_URL=http://localhost:8081`, the guide's `curl`
  command, pointed at the container's port, printed `connect-src 'self' http://localhost:8081` (the page writes the
  quotes as `&#x27;`), and the page set `window.__RUNTIME_CONFIG__` to `{"apiUrl":"http://localhost:8081"}`; against
  `aim-dashboard:latest` started the same way, the command printed nothing and the page named no `localhost:8081`.
  Started with `NEXT_PUBLIC_API_URL=http://localhost:8080` and requested with `Host: aim-frontend.example.com`, the
  page handed the browser `http://localhost:8080`, and its `connect-src` listed no `aim-backend.example.com`; started
  with no `NEXT_PUBLIC_API_URL`, the same request carried no runtime configuration and a `connect-src` that lists
  `aim-backend.example.com`.

### Fixed: a dashboard built with no `NEXT_PUBLIC_API_URL` holds no `http://localhost:8080` in its browser code, and `next build` fails when such code does

- `next.config.ts` set `NEXT_PUBLIC_API_URL` to `http://localhost:8080` when the build read none, and `next build`
  writes that value into the browser's code. Built from the previous commit with no `NEXT_PUBLIC_API_URL`, the
  dashboard held the address in 2 files under `.next/static`; the published `aim-dashboard:latest` image (revision
  `3ab8285e`, built with `NEXT_PUBLIC_API_URL=http://localhost:8080`) holds it 23 times in 10 files. With no runtime
  configuration and a hostname that derives no `-backend` host, `getApiUrl` in `apps/web/lib/api.ts` returned that
  address, on a dashboard opened at `dashboard.example.com` as well.
- `next.config.ts` no longer lists `NEXT_PUBLIC_API_URL` under `env`: `next build` writes the value it reads, and
  nothing when it reads none. When no URL is named (no runtime configuration, no build value, no `-frontend`
  hostname), `getApiUrl` calls port 8080, the API's default port, on the host name the dashboard was opened at, and
  the dashboard's own origin when that name is an IPv6 address. The page's `connect-src` lists that host and port in
  place of `http://localhost:8080` (`defaultApiHost` in `apps/web/lib/backend-host.ts`, read by
  `apps/web/lib/runtime-config.ts`), and lists no build origin when the build read none.
- After it compiles, `next build` fails when a file under `.next/static` holds `http://localhost:8080` and the build's
  `NEXT_PUBLIC_API_URL` does not (`apps/web/lib/local-api-origin.ts`, run from `compiler.runAfterProductionCompile`
  in `next.config.ts`). The frontend job in `.github/workflows/ci.yml` runs `npm run build` with no
  `NEXT_PUBLIC_API_URL`. A build given `NEXT_PUBLIC_API_URL=http://localhost:8080`, as the image workflow's build
  argument gives it, passes and holds the address in 2 files.
- `docs/guides/DEPLOYMENT_CHECKLIST.md` describes the URL a dashboard built with no URL calls.
- Tests: `apps/web/lib/local-api-origin.test.ts` checks the file walk and when the check fails. `apps/web/next-config.test.ts`
  checks that the configuration lists no `NEXT_PUBLIC_API_URL` when the build reads none and that its
  `runAfterProductionCompile` fails over a build directory holding the address. `apps/web/lib/api.backend-host.test.ts`
  checks the URL the client calls with no URL named on four host names, and the page's `connect-src` for each.
  `apps/web/lib/runtime-config.test.ts` and `apps/web/app/layout.test.tsx` check the page's `connect-src` for a build
  that read no URL. On the previous code, 6 of these tests fail and `local-api-origin.test.ts` does not load.
- Measured with `next build` on this tree: with no `NEXT_PUBLIC_API_URL`, and with an empty one, the build passes and
  no file under `.next/static` holds `http://localhost:8080`; with `NEXT_PUBLIC_SUPPORT_EMAIL=http://localhost:8080`
  and no `NEXT_PUBLIC_API_URL`, the build fails and names the chunk that holds it. Served by `next start` with no
  `NEXT_PUBLIC_API_URL`, beside a stand-in API on `127.0.0.1:8080`: in Chromium, signing in with keyboard and mouse
  input on the dashboard opened at `http://127.0.0.1:13900` sent the request to `http://127.0.0.1:8080`
  (`Host: 127.0.0.1:8080`) under `connect-src 'self' 127.0.0.1:8080`, and opened at `http://localhost:13900` to
  `http://localhost:8080`, with no `connect-src` violation on either page.

### Fixed: the dashboard reads `NEXT_PUBLIC_API_URL` from its environment when it renders a page, and the page's `connect-src` lists that API

- The dashboard read `NEXT_PUBLIC_API_URL` only when it was built: `next build` writes the value into the browser's
  code, and `next.config.ts` built the `connect-src` of the `Content-Security-Policy` header from it. Served with
  `NEXT_PUBLIC_API_URL=http://localhost:8081` in its environment, a dashboard built with `http://localhost:8080` sent
  its sign-in page with no runtime configuration and with `connect-src 'self' http://localhost:8080`; with neither a
  runtime configuration nor a `-frontend` hostname, the API client takes the build's URL (`getApiUrl` in
  `apps/web/lib/api.ts`). The published `aim-dashboard` image is built with
  `NEXT_PUBLIC_API_URL=http://localhost:8080`, and `FRONTEND_API_URL` in `docker-compose.quickstart.yml`, which sets
  the variable on the container, did not reach the browser.
- The root layout (`apps/web/app/layout.tsx`) reads `NEXT_PUBLIC_API_URL` from the server's environment each time it
  renders a page (`apps/web/lib/runtime-config.ts`). When the variable is set, the page sets
  `window.__RUNTIME_CONFIG__.apiUrl`, which `getApiUrl` takes before its other sources. When it is not set, the page
  sets nothing and `getApiUrl` resolves the URL from its other sources: the `-backend` host of a `-frontend`
  hostname, else the URL the build read.
- Each page carries a `<meta http-equiv="Content-Security-Policy">` element whose `connect-src` lists the dashboard
  origin and the API origins the page may call: the server's `NEXT_PUBLIC_API_URL` when it is set; otherwise the
  build's URL, and the `-backend` host where the request's `Host` derives one. The header keeps every other
  directive, and its `connect-src` is `'self' http: https:`, which the page's element narrows. The header rule for
  `-frontend` hosts is removed, and the header builders are renamed: `contentSecurityPolicy` to
  `headerContentSecurityPolicy`, `dashboardSecurityHeaders` to `dashboardHeaderRules`. The previous names, whose header
  listed the API origin, are not kept. `apps/web/lib/api.ts` changes in one comment.
- The root layout reads the request's headers, so Next.js renders each page when it is requested: `next build` lists
  every page as dynamic, `/icon.png` excepted, and `/auth/login`, `/dashboard`, `/device` and `/dashboard/agents/abc`
  are served with `Cache-Control: private, no-cache, no-store, max-age=0, must-revalidate`.
- `docs/guides/DEPLOYMENT_CHECKLIST.md` and `docs/guides/QUICK_DEPLOYMENT_REFERENCE.md` describe the variable as read
  from the dashboard's environment and the `connect-src` as carried by the page.
- Tests: `apps/web/app/layout.test.tsx` renders the root layout with `NEXT_PUBLIC_API_URL=http://localhost:8081` and
  checks the page's runtime configuration and `connect-src`, and renders it with no URL on `aim-frontend.example.com`;
  both cases fail on the previous layout, which wrote neither. `apps/web/lib/runtime-config.test.ts` checks the page's
  values for a URL in the server's environment and for none, the `-frontend` host table the header test held, and
  that the inline script holds no `<`. `apps/web/next-config.test.ts` checks the header on every page under `app/`
  and on five `Host` values, `-frontend` ones included.
- Measured with one build made with `NEXT_PUBLIC_API_URL=http://localhost:8080` and served by its standalone server
  twice, with `NEXT_PUBLIC_API_URL=http://127.0.0.1:18081` and `http://127.0.0.1:18082`, each beside a stand-in API
  on that address: in Chromium, signing in with keyboard and mouse input on each dashboard sent the request to its
  own API (`Host: 127.0.0.1:18081` and `Host: 127.0.0.1:18082`), and a request from each page to the other's API was
  refused by the page's `connect-src`. Served with no `NEXT_PUBLIC_API_URL` and opened on
  `aim-frontend.example.test`, the page sent sign-in to `http://aim-backend.example.test`, which its `connect-src`
  lists.

### Fixed: on a `-frontend` hostname, the API documentation page and the MCP server dialog call the API the dashboard signs in to

- The API documentation page (`/dashboard/developers`) built the URL its request runner calls, and the URL in its cURL,
  JavaScript and Python samples, from the `NEXT_PUBLIC_API_URL` the dashboard was built with. The MCP server detail
  dialog fetched a server's attestations and audit log from the same value. The API client resolves its address in
  another order (`getApiUrl` in `apps/web/lib/api.ts`), where that value comes after the `-backend` host it derives from
  a `-frontend` hostname. On such a hostname, sign-in went to the `-backend` host while these requests went to the
  build's value; the published `aim-dashboard` image is built with `NEXT_PUBLIC_API_URL=http://localhost:8080`.
- `apiBaseUrl()` in `apps/web/lib/api-base-url.ts` returns the address the API client resolved, through the client's
  `baseURL` getter, which the register-agent and agent-detail dialogs already read. The page, the dialog and
  `downloadSDK` in `apps/web/lib/agent-sdk.ts` build their URLs from it. `apps/web/lib/api.ts` is unchanged.
- Tests: `apps/web/app/dashboard/developers/page.api-url.test.tsx` and
  `apps/web/components/modals/mcp-detail-modal.api-url.test.tsx` open the page and the dialog on
  `aim-frontend.example.com` with `NEXT_PUBLIC_API_URL=http://localhost:8080`, and check that the runner's request, the
  three samples and the dialog's two requests name `https://aim-backend.example.com`. Both tests fail on the previous
  code, where the runner's request and the dialog's two requests named `http://localhost:8080`.
  `apps/web/lib/agent-sdk.test.ts` checks the SDK download URL the same way. `apps/web/api-url-readers.test.ts` parses the non-test source files under `apps/web`, outside
  `node_modules` and dot-directories, and fails when a file other than `lib/api.ts` and `next.config.ts` reads
  `process.env.NEXT_PUBLIC_API_URL`. `apps/web/lib/api.backend-host.test.ts` checks that `apiBaseUrl()` returns the
  origin the client sends its requests to.

### Fixed — the SDK integration scripts pass aim-bootstrap and the server the user, password and database name of `DATABASE_URL` percent-decoded (#761)

- `scripts/lib/aim-backend.sh`, which `scripts/test-sdk-python-framework-integration.sh` and
  `scripts/test-sdk-java-helpers-integration.sh` source, exported the user, the password and the database name of
  `DATABASE_URL` as `POSTGRES_USER`, `POSTGRES_PASSWORD` and `POSTGRES_DB` with their `%XX` escapes. `cmd/migrate`
  opens `DATABASE_URL`, whose user, password and database name lib/pq decodes; aim-bootstrap and the server read the
  `POSTGRES_*` values without decoding them. Run against PostgreSQL 18 with an at-sign written `%40` in the password,
  the migrations applied, and then `aim-bootstrap --default`, and the server started alone with the same variables,
  failed with `password authentication failed for user "aim"`. The script now decodes the three values before it
  exports them, and the same run seeds the admin and passes the server's `/health/ready` check.
- A `%` in one of the three that does not begin a `%XX`, which lib/pq refuses as well, and `%00`, which no environment
  variable can carry, stop the run with a message that names the part of the URL and quotes none of it.
- Tests: `apps/backend/cmd/bootstrap/aim_backend_script_test.go`.

### Fixed — the dashboard reports a bodiless 200 to an alert channel test as a server error, and the webhook dialogs fit a 375px screen (#762)

- `testAlertChannel` in `apps/web/lib/api.ts` throws the server error line for a `200` whose body has no boolean
  `delivered`, such as a proxy's empty page. On the previous code it returned `null` for a `200` with an empty body.
- The create and detail webhook dialogs set their one grid column to `minmax(0,1fr)`, so the column keeps the
  dialog's width instead of growing to the payload example's longest line, and the detail dialog's URL breaks
  inside its box. Measured in headless Chromium at 375x812 with the dashboard's stylesheet, the create dialog's
  `scrollWidth` was 548px and the detail dialog's 456px against a 373px `clientWidth` on the previous code; both
  now fit.
- Tests: `apps/web/lib/api.alert-channels.test.ts`, `apps/web/components/webhook/webhook-dialog-width.test.tsx`
  (runs where a Chromium build for the installed Playwright is present).

### Fixed — the tests in `apps/backend/tests/integration` that need a running backend are skipped unless `BACKEND_URL` names one (#764)

- `WaitForBackend` and `ensureAIMBackendRunning`, the probes those tests call before their first request, skip the
  test before sending a request when neither `BACKEND_URL` nor the deprecated `TEST_BASE_URL` is set, and the skip
  message names `BACKEND_URL`. Before this change they sent `GET /health` to the address the suite falls back to,
  `http://localhost:8080`, and let the test go on when the answer named the AIM service. Against a local server that
  answered that way and refused the admin password, a test then failed at the admin sign-in with
  `admin login failed: request failed with status 401`.
- With `BACKEND_URL` set, the probes send `GET /health` to that address, let the test go on when the answer names the
  AIM service, and skip it when nothing listens there, when another service answers or when the answer is not JSON.
  With `BACKEND_URL` unset and `TEST_BASE_URL` set, they let the test go on against the `TEST_BASE_URL` address when
  the answer names the AIM service. Both probes call one function, `skipUnlessAIMBackend` in `test_helper.go`.
- `tests/README.md` gives `BACKEND_URL` no default and says when the tests are skipped, and
  `docs/guides/QUICK_START.md` sets `BACKEND_URL=http://localhost:8080` in the command that runs the suite.
- Tests: `apps/backend/tests/integration/backend_probe_test.go` runs each probe against a local server that answers
  `GET /health` with the AIM service name and refuses the admin sign-in with 401. With neither variable set and that
  server on the address the suite falls back to, the test is skipped and the server receives no request; with either
  variable naming the server, the test goes on after one `GET /health`; with `BACKEND_URL` naming a closed address, a
  server for another service or a server whose answer is not JSON, the test is skipped. With neither variable set,
  `go test -race ./tests/integration/` from `apps/backend` passed 19 top-level tests, skipped 266 and failed none, and
  each of the 266 skips gives `BACKEND_URL is unset` as its reason.

### Fixed — the backend's `.env.example` and the deployment guide name `APP_PORT` and `HOST`, the variables the server reads

- `apps/backend/.env.example` set `PORT=8080`, and the `.env` reference in `docs/DEPLOYMENT.md` set
  `SERVER_HOST=0.0.0.0` and `SERVER_PORT=8080`. The server reads none of the three, so a port or host set there had no
  effect. Both now set `APP_PORT=8080`, and the deployment guide shows `HOST`, commented out, with the address the API
  listens on when it is unset.

### Fixed — `GET /api/v1/secrets/audit` refuses a `since`, `limit` or `offset` it cannot read (#765)

- The route answers `400` and `{"error": "since must be an RFC 3339 time such as 2026-10-17T09:30:00Z"}` for a
  `since` that is not an RFC 3339 time, and `{"error": "limit must be an integer of 0 or more"}` or
  `{"error": "offset must be an integer of 0 or more"}` for a `limit` or `offset` that is not an integer or is below
  0, before it reads the agent or the audit log. A `since`, `limit` or `offset` that is absent or empty reads as no
  time filter, 50 and 0, as before.
- Run against the handler on the previous code, `since=not-a-time`, `since=2026-10-16`, `since=2026-10-16T10:00:00`
  and a `since` whose `+02:00` offset was sent unencoded, which the server decodes as a space, were each answered
  `200` with the agent's entries and no time filter; `limit=abc`, `limit=1.5` and `offset=abc` were answered `200`
  with a limit or offset of 0; `limit=-1` and `offset=-1` were answered `200` with the negative value passed to the
  audit log repository.
- Tests: `apps/backend/internal/interfaces/http/handlers/secrets_handler_audit_query_test.go` sends each refused
  form through the handler and checks the status, the error line and that the audit log was not read, and sends
  readable filters, a `since` with an encoded `+02:00` offset and one with fractional seconds among them, and checks
  the values the repository receives.

### Fixed — audit rows keep the user agent and client address of the request that wrote them (#751)

- `AuditService.LogAction` and `AuditService.LogAgentAction` copy the client address and user agent into the row.
  The handlers pass `c.Get("User-Agent")`, which is a view of the request buffer, since the fiber app does not set
  `Immutable`, and the server reuses that buffer for later requests. With the request's `User-Agent` written over in
  place after the route answered, the login row of `POST /public/login`, the logout row and the refresh route's
  `refresh_token_reuse` and `refresh_session_revoked` rows read the value written over it on the previous code.
- `siem.EventFromAuditLog` copies the action and resource type into the event as well, as it copies the client
  address and user agent, so an entry a route builds from request values does not change the queued event.
- Tests: `apps/backend/internal/interfaces/http/handlers/audit_user_agent_test.go`,
  `apps/backend/internal/application/audit_service_test.go`, `apps/backend/internal/integrations/siem/audit_test.go`.

### Changed — CI runs the compiled-executable lint, and the contributing guide names every script the TypeScript SDK's tests run (#751)

- `scripts/lint-no-executable-blobs.py` and its tests, `scripts/test-lint-no-executable-blobs.sh`, run in the
  TypeScript SDK's test suite, from `sdk/typescript/tests/repo-executable-blobs.test.ts`, which CI runs on every pull
  request to `main`. Before this change no workflow, package script or test ran either script. The suite runs both
  without the variables git reads to find a repository (`git rev-parse --local-env-vars`): run with the `GIT_DIR` a
  hook started from a linked worktree exports, the lint's tests fail and write `core.bare = true` into the config the
  main checkout shares.
- `CONTRIBUTING.md` said the TypeScript SDK's `npm test` runs three of the repository's documentation lints while it
  runs five scripts; it now names all five, `scripts/check-docs-prose.mjs` and `scripts/test-check-docs-prose.mjs`
  with `node` among them, and the executable lint and its tests. `sdk/typescript/tests/repo-doc-lints.test.ts`
  checks that the guide names every script its cases run and states their number.

### Security: the API server listens on 127.0.0.1 outside a container unless `HOST` is set (#757)

- The API listener binds `HOST` and `APP_PORT`, joined with `net.JoinHostPort`. With `HOST` unset or empty, the host is
  `127.0.0.1` outside a container and `0.0.0.0` in one. The server counts as in a container when `/.dockerenv` or
  `/run/.containerenv` exists or `KUBERNETES_SERVICE_HOST` is set and not empty. An IPv6 literal such as `HOST=::1`
  listens on IPv6. The image built from `infrastructure/docker/Dockerfile.backend` sets `HOST=0.0.0.0`.
- `config.Load` reads `HOST` and `KUBERNETES_SERVICE_HOST` with the server settings, by its one rule: an empty value
  reads as unset, and a value is used as written. The startup summary keeps its port line and ends with
  `API listener: <host>:<port> (HOST sets the address; unset, it is 0.0.0.0 in a container and 127.0.0.1 outside one)`,
  and the metrics wiring's lines name the same address.
- Outside a container, set `HOST=0.0.0.0` to accept connections from other hosts. `apps/backend/.env.example` and
  `docs/guides/INSTALLATION.md` describe the setting.
- `apps/backend/deployments/otel-demo/smoke-backend.sh`, `apps/backend/deployments/smoke/rest-list-endpoints-smoke.sh`
  and `sdk/typescript/scripts/release-smoke.sh` start their backend with `HOST=127.0.0.1`, and `release-smoke.sh`
  calls it at `http://127.0.0.1:<port>`.
- Tests: `apps/backend/cmd/server/api_listener_test.go` starts the API listener with `HOST` unset and no container
  detected and checks it bound `127.0.0.1` and answers `GET /health` there, starts it with `HOST=::1` and checks it bound
  `::1`, checks the address for each `HOST` and container case without listening, and checks the summary's last line.
  `apps/backend/internal/config/listen_host_test.go` covers `ListenHost`, `RunningInContainer` and `Load`.
  `internal/config/testdata/environment_table.json` has five rows each for `HOST` and `KUBERNETES_SERVICE_HOST`
  (unset, empty, whitespace only, a valid and an invalid value), 390 rows for 72 variables in all, and the table test
  runs each row with no container marker file, so the `HOST` default it checks is the one outside a container.

### Fixed — the CyberArk CCP client sends a `Query` that names several properties, and a value that holds a space (#758)

- `RetrieveCredential` sends a value that holds a space in `AppID`, `Safe`, `Folder`, `Object`, `UserName`, `Address`,
  `Database`, `PolicyID` or `Reason`, and a `Query` that names several properties separated by `;`, such as
  `Safe=AIM-Database;Object=db`, with or without a space in a property value. Each is query-escaped: `=` is sent as
  `%3D`, `;` as `%3B` and a space as `%20`. Before this change `RetrieveCredential` returned an error for each of
  these that said the parameter held a character CCP does not accept in a URL value.
- It still refuses, before sending, a `+`, `&` or `%` in any of those values or in `Query`, and a `;` in any of them
  but `Query`. A `;` in a `Query` is refused unless every `;`-separated part is a `name=value` property with a
  non-empty name, so a leading, trailing or doubled `;`, or a part without `=`, is refused. The error names the
  parameter and not its value.
- Tests: `apps/backend/internal/integrations/cyberark/connector_test.go` (`TestCCPConnectorRoundTrip`: `a value
  holding a space is sent with the space written as %20`, `a Query that names several properties is sent
  query-escaped` and `values CCP does not accept are refused before sending`). The CCP stub in `ccptest` accepts a
  `;` that separates two properties of a `Query`; it looks an account up by the `AppID`, `Safe` and `Object`
  parameters and does not evaluate a `Query`.

### Fixed: a SIEM event copies the client address and user agent out of the request it reports

- The `ipAddress` and `userAgent` of an `aim.audit.*` or `aim.auth.signin_failed` event are copied when the event is
  built, inside the request, and the sign-in failure report a sign-in route hands its observer carries copies as
  well. A handler receives both as views of the server's request buffer (`c.Get`, and `c.IP` behind a trusted
  proxy), which the server rewrites when it reads the next request, and the forwarder serializes the event after the
  request has ended. `go test -race` reports the forwarder's read of such a view against the next request's header
  parse as a data race; it did so in `TestSIEMAuthDecisionsRoundTrip` (`/api/v1/auth/login/local`) in the
  integration job, and not in every run. The client address of a login or logout audit row was copied already; its
  user agent was not. No event field, severity or answer changes.
- Tests: `apps/backend/internal/integrations/siem/audit_test.go`
  (`TestEventFromAuditLog_CopiesTheAddressAndUserAgentOutOfTheRequestBuffer`),
  `apps/backend/internal/integrations/siem/decisions_test.go`
  (`TestEventFromSignInFailure_CopiesTheAddressAndUserAgentOutOfTheRequestBuffer`) and
  `apps/backend/internal/interfaces/http/handlers/sign_in_failure_report_test.go`
  (`TestReportSignInFailure_KeepsTheRequestsOwnAddressAndUserAgent`): each builds the event or the report from
  views of a request buffer, rewrites the buffer as the next request's parse does, and reads the first request's
  values back. `TestSIEMAuthDecisionsRoundTrip` is unchanged.

### Changed — `cmd/server` reads the environment only through `internal/config`, and a recorded table pins each variable

- `internal/config` reads all 70 variables that `cmd/server` read, or that it read already, each into a typed field of
  `config.Config`: the 32 it read already and the 38 that `cmd/server` read on its own (the secrets
  backends, the registry bridge, the ATC issuer and CRL endpoint, FGA, the email startup line, the first
  administrator, `ALLOWED_ORIGINS`, `AUTH_REVOCATION_FAIL_OPEN`, `METRICS_LISTEN_ADDR`,
  `POSTGRES_APP_USER` and `POSTGRES_APP_PASSWORD`, and the `SIEM_*` and `AIM_SIGNING_KEY_*` variables,
  which the SIEM settings and the signing key ring now read through `Config`). `cmd/server` reads the
  `Config` value. Variables other packages read (token lifetimes, rate limits, email delivery,
  telemetry, the key vault) are read where they were, and fourteen of the 70 are also read by other
  packages with their own rules: `ENVIRONMENT` (the detection service, the rate limit and security header
  middleware, and the key vault; the detection service treats unset as not development, where
  `config.Load` reads unset as `development`), `JWT_SECRET` (the JWT package), `FRONTEND_URL` (the
  registration service and the AIP handler), `REGISTRY_BRIDGE_URL` (the registry client, which trims
  it), `KEYVAULT_MASTER_KEY` (the key vault), `EMAIL_PROVIDER` and `EMAIL_FROM_ADDRESS` (the email
  package, through its own wrapper over `os.Getenv`), and the seven OAuth client variables
  `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `MICROSOFT_CLIENT_ID`, `MICROSOFT_CLIENT_SECRET`,
  `OKTA_CLIENT_ID`, `OKTA_CLIENT_SECRET` and `OKTA_DOMAIN` (`NewOAuthService` in the auth package, which
  only its package's tests call).
- One rule, stated on `config.Load` and applied to every value it reads: an empty value reads as unset and gives the default, and a value
  is used as written, never trimmed. `REDIS_HOST` set and empty still turns Redis off; a number or a
  duration that does not parse still gives the default; and a flag is still on only for its word:
  `true` for `REDIS_USE_TLS` and `REGISTRY_BRIDGE_ENABLED`, `true` in any case for
  `AUTH_REVOCATION_FAIL_OPEN`, and `on` for `POSTGRES_SCOPE_CENSUS`.
- No default, accepted value or startup line changes. `internal/config/testdata/environment_table.json`
  holds 380 rows: for each variable, unset, empty, whitespace only, a valid and an invalid value, and
  a quote and a backslash for each password and token, with the setting and the startup line the
  previous code resolved and logged. `internal/config` checks every setting against it and
  `cmd/server` every line.
- `GET /api/v1/status` reports the `ENVIRONMENT` the server read at startup. It read the variable on
  each request before; for `ENVIRONMENT` unset, empty, whitespace only, `production` and `Production`,
  the request-time value and the startup value were the same.
- A test fails when a non-test file in `cmd/server` uses `os.Getenv`, `os.LookupEnv`, `os.Environ`,
  `os.ExpandEnv`, `syscall.Getenv` or `syscall.Environ`, whether called or passed as a value.

### Changed — the CyberArk CCP client encodes and checks its query as CyberArk publishes, authenticates with a client certificate and verifies the server; the PSM session sender is removed

- `internal/integrations/cyberark` is a client for the Central Credential Provider's
  `GET /AIMWebService/api/Accounts`. No server path calls it yet.
- `NewConnector` returns an error. It requires an `https` `CCPURL` with no user information, query or fragment,
  takes `CertPath` and `KeyPath` together or not at all, loads the client key pair and an optional `CACertPath` PEM
  bundle that verifies the CCP server (empty means the system roots), and builds a TLS client with TLS 1.2 as its
  minimum version. `Timeout` defaults to 30 seconds. An error names the setting at fault and never a file's content.
- `CCPRequest` carries `AppID`, `Safe`, `Folder`, `Object`, `UserName`, `Address`, `Database`, `PolicyID`, `Reason`,
  `Query`, `QueryFormat`, `ConnectionTimeout` (sent as `Connection%20Timeout`) and `FailRequestOnPasswordChange`
  (sent as `true` only when set), each under CyberArk's name and query-escaped with a space written as `%20`, so the
  URL holds no raw space and no `+`. Before anything is sent, `RetrieveCredential` refuses a request with no `AppID`,
  with `AppID` alone, with a `QueryFormat` other than `Exact` or `Regexp`, with a value holding `+`, `&` or `%`, or
  with a `;` other than one that separates two `name=value` properties of a `Query`. The error names the parameter
  and not its value.
- `CCPResponse` holds `Content`, `UserName`, `Address`, `Database` and `PasswordChangeInProcess`, read as a JSON
  boolean or as the string `true` or `false` in any letter case; `ChangeNumber` is removed. An account whose
  password is being changed is returned with `PasswordChangeInProcess` true. A 400, 403, 404 or 500 answer unwraps to
  `ErrCCPBadRequest`, `ErrCCPForbidden`, `ErrCCPNotFound` or `ErrCCPServerError`; every non-200 answer is a
  `*CCPStatusError` carrying the status and at most the first 256 bytes of the answer. A redirect is returned as its
  status and not followed. No error carries a 200 body.
- `StreamSessionEvent`, `PSMSessionEvent` and `Config.PSMURL` are removed.
- Tests: `apps/backend/internal/integrations/cyberark/connector_test.go`, against a CCP stub in
  `apps/backend/internal/integrations/cyberark/ccptest` that serves TLS and requires a client certificate signed by
  its own CA.

### Fixed — a credential on the HashiCorp Vault secrets backend resolves to the bytes that were stored, and the Vault and AWS Secrets Manager backends are tested against a server

- `HashiCorpVaultBackend.Retrieve` decodes the blob of the KV v2 entry from the base64 string Vault returns. `Store`
  and `Rotate` are unchanged: the Vault client sends the blob as a base64 string and Vault keeps it as one, so an
  entry written before this change is read back as its stored bytes too. Measured against `vault server -dev`
  (Vault 1.21.1) on the previous code, for the binary blobs and the text credential the tests store: `Retrieve`
  returned the base64 text of the stored bytes, and an agent that resolved a credential through
  `SecretsService.Resolve` decrypted it to that base64 text instead of the stored value.
- The Vault backend is tested against a Vault dev server the test starts on a loopback port: store, retrieve,
  rotate (the version goes from 1 to 2) and delete through a token client, and JWT login through
  `NewHashiCorpVaultBackend` with a key pair generated by the test, where a JWT signed by another key is refused at
  login. These tests skip when no `vault` binary is on PATH.
- `apps/backend/internal/infrastructure/secrets/awssmtest` is an in-memory server of the Secrets Manager HTTP API
  (AWS JSON 1.1; `CreateSecret`, `GetSecretValue`, `PutSecretValue`, `DeleteSecret`) for tests. The AWS backend is
  tested through the AWS SDK's Secrets Manager client pointed at it, with no AWS account and no AWS endpoint.
- `apps/backend/internal/infrastructure/secrets/vaultkvtest` is an in-memory server of the KV secrets engine
  version 2 HTTP API (`GET`, `PUT` and `POST` on `secret/data/<path>`, `GET` and `DELETE` on
  `secret/metadata/<path>`, and the `X-Vault-Token` check) for tests. The Vault backend is tested through the Vault
  client pointed at it as well, and a test that runs where a `vault` binary is on PATH sends the same requests to
  the stub and to `vault server -dev` (Vault 1.21.1) and requires the same answers.
- Two integration tests resolve a credential stored on each backend as the owning agent, with the namespace,
  agent, audit and nonce repositories on PostgreSQL, open it with the agent's key, and delete the namespace. The
  Vault one runs against `vaultkvtest` and the AWS one against `awssmtest`, so neither needs a `vault` binary or a
  vendor account.
- Tests: `apps/backend/internal/infrastructure/secrets/hashicorp_vault_server_test.go`,
  `apps/backend/internal/infrastructure/secrets/hashicorp_vault_stub_server_test.go`,
  `apps/backend/internal/infrastructure/secrets/vaultkvtest/vaultkvtest_test.go`,
  `apps/backend/internal/infrastructure/secrets/aws_secrets_server_test.go`,
  `apps/backend/internal/infrastructure/secrets/awssmtest/server_test.go`,
  `apps/backend/internal/application/secrets_external_backends_integration_test.go`.

### Fixed — the 1Password secrets backend reads back the credential bytes it stored, and the Azure Key Vault and GCP Secret Manager backends report a deleted secret as not found

- `POST /api/v1/secrets/namespaces/:id/credentials` and `POST /api/v1/secrets/namespaces/:id/rotate` decode
  `encryptedBlob` from base64, so a credential blob can hold any byte, and a 1Password item field value is a JSON
  string. The 1Password backend now writes the blob base64-encoded in the item's `blob` field, with a `blobEncoding`
  field set to `base64`, and decodes it on read. Run through the 1Password Connect client library against a stub
  Connect server on the previous code, a 10-byte blob holding four bytes that are not valid UTF-8 was read back as
  18 bytes, each of those four replaced with U+FFFD, and the same held for the blob a rotation wrote. An item with no
  `blobEncoding` field is read as before, as the bytes of its text; an item whose `blobEncoding` is not `base64` is
  refused.
- A read of a secret that the vault answers as missing returns `no credential found in azure for namespace <id>`
  (Azure Key Vault, HTTP 404) or `no credential found in gcp for namespace <id>` (GCP Secret Manager, gRPC
  `NOT_FOUND`), the form the 1Password backend returns for a namespace with no item. Run through each vendor's client
  library against a stub on the previous code, a read after a delete returned
  `secrets backend unavailable: azure retrieve failed: ...` and `secrets backend unavailable: gcp retrieve failed: ...`,
  the error the domain keeps for a backend that cannot be reached. Every other error of these reads is unchanged.
- Tests: `azure_key_vault_server_test.go`, `gcp_secret_server_test.go` and `onepassword_server_test.go` in
  `apps/backend/internal/infrastructure/secrets/` store, read, rotate and delete a credential through `azsecrets`,
  `secretmanager/apiv1` and `connect-sdk-go` against the test-only stubs `azkvtest` (HTTPS, with the 401 bearer
  challenge), `gcpsmtest` (the generated `SecretManagerServiceServer`) and `onepasswordtest`, each written from its
  vendor's published API reference. They check that each stub refuses a request without credentials, that the Azure
  client refuses a vault whose certificate it does not trust, and that a wrong 1Password bearer is refused. No test
  reaches a vendor service; a run against vendor accounts is not part of this change.

### Changed — the README is shorter, and the passages it no longer carries are in `docs/`

- What a verification request does to the trust score is now only in `docs/server-features.md` (Trust-gated
  capabilities), which already held the README's three sentences word for word; the README links that section, and
  `TestRootReadme_StatesWhenTheTrustScoreIsRecalculated` and `test_root_readme_short_form.py` read the paragraph
  there. The full list of 11 jobs is in `docs/USE-CASES.md`; the README names nine and links it. The Ed25519 keypair
  and YAML capability policies of local-only mode moved from the deployment table to the sentence under it.

### Fixed — the deployment guide says `ENVIRONMENT=test` also runs without `KEYVAULT_MASTER_KEY` (#753)

- `docs/DEPLOYMENT.md` says `KEYVAULT_MASTER_KEY` is required in every environment except `ENVIRONMENT=development`
  and `ENVIRONMENT=test`, in its environment example and in the paragraph on the key. Before this change both places
  named development alone. With either value, a server started without the key generates a new one at each start,
  and stored agent private keys cannot be decrypted after a restart. The paragraph names the values that refuse to
  start without the key (unset, empty, `production`, `staging`, `Development` and `testing`), and says
  `ENVIRONMENT=test` also raises both rate limits tenfold and is not for a deployment.
- Tests: `TestNewKeyVaultFromEnv_NoMasterKeyNeedsExplicitDevelopment` checks that `ENVIRONMENT=testing` refuses to
  start without the key.

### Added — the `fga.authorize` spans carry `gen_ai.agent.id`, the AIP identifier of the agent AIM resolved

- The `fga.authorize` span AIM emits for an authorization decision and the one it emits for an SDK verification
  carry the OpenTelemetry key `gen_ai.agent.id`. Its value is `did:aip:aim_` followed by an agent's UUID: on the
  authorization path, of the agent row AIM loaded for the decision; on the SDK verification path, of the agent whose
  signature AIM verified. The value is never taken from a request field. When AIM resolves no agent row, the span
  carries no `gen_ai.agent.id`, and the decision is the same.
- Unchanged: the legacy `agent.id` span attribute, every metric and its attributes, and span parenting.
- The otel-demo's `fga.authorize` span carries `gen_ai.agent.id`, and `apps/backend/deployments/otel-demo/README.md`
  says where the value comes from, how an agent's own spans join AIM's on that value, and that the value changes form
  when AIM adopts the AIP 1.1 `did:web` agent form.
- Tests: `apps/backend/internal/telemetry/genai_attrs_test.go` checks that the key is set when the agent identifier
  is present and absent when it is empty; `apps/backend/internal/application/fga_engine_genai_attrs_test.go` runs an
  authorization decision for a resolved agent, for a missing agent row and for a nil row, and an SDK verification span
  for a verified agent and for none, with request agent IDs that differ from the expected identifier.

### Fixed — the Java SDK's helper classes run against a real backend, and its SIEM claim says what a test against Splunk HEC's published format measured

- `scripts/test-sdk-java-helpers-integration.sh` brings up the backend as the Python adapters' script does (both
  source `scripts/lib/aim-backend.sh`, and both pass it `ENVIRONMENT=test`) and runs `mvn -B test -Dtest='org.opena2a.aim.integration.*IntegrationTest'` in
  `sdk/java` with `AIM_URL` set. `HelpersAgainstBackendIntegrationTest` registers one agent per helper with
  `AIMClient.secure`, using the credentials of `GET /api/v1/sdk/download?sdk=java`, and reads the agent's verification
  events through `GET /api/v1/verification-events/agent/{id}`. The run fails on a failure, an error or a skip. Without
  `AIM_URL` the class is skipped, so a plain `mvn test` runs as before. The script refuses to start when the SDK
  directory or a directory above it holds a `.aim` entry, or the SDK directory holds `sdk_credentials.json` or
  `credentials.json`, and the test JVM's `user.home` is a fresh directory.
- Measured against the backend: `AIMToolExecutor.execute`, `AIMFunctionCallingAdvisor.execute` (a function that ran
  and one refused for want of an approval) and `AIMChatModelListener`'s `onRequest`, `onResponse` and `wrap` record no
  verification. The `AIMClient` each helper holds records one when the test calls its `verifyCapability`. The helpers
  are not changed.
- `SecurityLoggerHecFormatTest` logs one event through each of SecurityLogger's 15 public `log*` overloads to a file.
  Each call adds one line, each line is one JSON object, and a receiver written from Splunk's pages "Format events for
  HTTP Event Collector" and "Troubleshoot HTTP Event Collector" accepted each line, sent unchanged, as an event's
  `event` field. A line names its time in `timestamp`, an ISO-8601 UTC string; HEC's `time` takes epoch seconds.
- `docs/sdk/java.md`, the `SecurityEvent` javadoc and the demo agent's SecurityLogger text said the output is
  compatible with Splunk, ELK, Datadog and other platforms. They now say what the test measured, and that ELK and
  Datadog were not tested.
- Tests: `HelpersAgainstBackendIntegrationTest`, run by the script, and `SecurityLoggerHecFormatTest`.

### Fixed — the Python SDK's LangChain, CrewAI and MCP adapters run against a real backend, which records a verification for each framework call

- `scripts/test-sdk-python-framework-integration.sh` builds the backend, runs its migrations on the database
  `DATABASE_URL` names, seeds the admin with `aim-bootstrap --default`, starts the server on a free port with
  `ENVIRONMENT=test`, as the CI end-to-end job does, and runs `tests/test_langchain_integration.py`, `tests/test_crewai_integration.py` and `tests/test_mcp_integration.py` in
  `sdk/python` with `pytest -m integration`. Each test asserts, through
  `GET /api/v1/verification-events/agent/{id}`, that the backend recorded one verification event, with the agent's id
  and the verified action, for each framework call the test makes. The run fails on a failure, an error or a skip.
  No test calls a model provider: tools and callbacks run through LangChain's tool runtime, and crews run on a CrewAI
  LLM that returns a fixed answer. The script refuses to start when the SDK directory or a directory above it holds a
  `.aim` entry, gives the tests a fresh `HOME` and no inherited environment, and prints none of the secrets it
  generates.
- `sdk/python/setup.py` has two new extras: `langchain` (`langchain-core==1.6.12`, `pydantic==2.12.5`) and `crewai`
  (`crewai==1.15.28`).
- Run against the backend with the previous adapter code, each of these calls left no verification event; each now
  leaves one:
  - `AIMCallbackHandler.on_tool_end`, for a tool LangChain ran with the handler in its callbacks. LangChain passes
    `run_id` as a `uuid.UUID`, the request body could not be encoded (`Object of type UUID is not JSON serializable`),
    and nothing was sent. The handler sends `run_id` as a string, in `on_tool_error` as well.
  - A verification with an empty resource: `AIMCrewWrapper.kickoff` and `kickoff_async` with `inputs={}`,
    `AIMTaskCallback.on_task_complete` and `on_task_error`, `@aim_verified_task` on a function called with no
    argument, and `@aim_verify` on a tool LangChain called with keyword arguments. The backend answered `401`
    (`Signature verification failed`): it signs an empty resource as `null`, and these adapters signed `""`. They now
    send no resource in that case.
  - `verify_mcp_action` and `MCPActionWrapper.execute_tool`. `POST /api/v1/mcp-servers/{id}/verify-capability`
    answered `200`; the route reads a `capability` member the adapter did not send. `verify_mcp_action` now verifies
    through the agent's verification route, `POST /api/v1/sdk-api/verifications`, with `mcp_server_id` and
    `risk_level` in the context, and returns the dictionary `AIMClient.verify_capability` returns. It does not look the
    server up.
- `list_mcp_servers` reads the `mcpServers` member that `GET /api/v1/sdk-api/agents/{id}/mcp-servers` answers. Run
  with the previous code, it read `servers` and returned an empty list for an agent with one registered MCP server.
- Tests: the three test files above, run by the script.

### Changed: the flight-search example's `get_fresh_sdk.py` prints the dashboard's sign-in and download steps instead of refreshing a stored credential

- `examples/flight-search-agent/get_fresh_sdk.py` refreshed the refresh token in `~/.aim/credentials.json`, wrote
  the new refresh token back to that file, and posted to `/api/v1/sdk/download/python` with the access token the
  refresh returned. The backend registers no route at that path; the SDK download is `GET /api/v1/sdk/download`,
  which refuses an access token minted by refreshing a downloaded SDK's credential (see the
  `SDK_TOKEN_FAMILY_MAX_AGE` entry below). The script now sends no request and leaves `~/.aim/credentials.json`
  as it was: it prints the steps to sign in at the dashboard's `/auth/login`, download the Python SDK from
  `/dashboard/sdk` and extract it into `./fresh-sdk/`. `docs/security/token-rotation.md` named a
  `get_fresh_sdk.sh` the repository does not have; it now names these steps and the script.
- Tests: `sdk/typescript/tests/flight-search-demo-helpers-stack-from-environment.test.ts` runs the script with a
  stored credential against a stand-in that answers a POST `401`, and against one that answers it `200` with a new
  token pair. Both runs send no request and name the dashboard's sign-in and SDK pages, and after the `200` run the
  credential file is unchanged. On the previous script the `200` run posted the refresh and then
  `/api/v1/sdk/download/python`.

### Added: the server prints one line at start that names its KeyVault master key

- Directly after `KeyVault initialized for automatic key generation`, the server prints one line through the standard
  log package: `KeyVault: master key source=<environment|ephemeral> storageKeyId=<n> masterKeyIdentity=<32 lowercase hex>`.
  `source` is `environment` when `KEYVAULT_MASTER_KEY` supplied the key, and `ephemeral` when the server generated one
  because `ENVIRONMENT` is `development` or `test`. `storageKeyId` is the storage key id new ciphertexts are written
  under (`KEYVAULT_MASTER_KEY_ID`, 1 when unset). `masterKeyIdentity` is the leftmost 16 bytes of HMAC-SHA256 keyed
  with the decoded master key over the label `opena2a-aim/keyvault-master-key/v1/key-id`, in lowercase hex. Before
  this change, a start with `KEYVAULT_MASTER_KEY` set printed no line about the master key: the KeyVault constructor
  prints only on the development branch.
- The identity names the key without revealing it: one key gives one identity across starts, and two keys give two
  identities. It is a MAC output, not a run of the key, and no production Go file under `apps/backend` uses it for
  anything but this line. A refused key prints no line, since the server stops at the KeyVault error before it.
- `NewKeyVaultFromEnv` prints no new line; its two development warning lines are unchanged.
- Tests: `apps/backend/internal/crypto/keyvault_test.go` (the source and storage key id, the constructor's output on
  a supplied key, the development branch and three refused keys, a known answer, and the identity's behaviour),
  `apps/backend/internal/crypto/keyvault_leak_test.go` (no run of the key in the line or the identity), and
  `apps/backend/internal/crypto/keyvault_identity_surface_test.go`, which reads the backend's production Go files and
  checks that one function computes the identity and one print, this line, prints it through the standard log
  package.

### Added: a dashboard test fails when a navigation link names a path no page serves

- `apps/web/lib/navigation-routes.test.ts` reads each sidebar entry's `href` and `hrefByRole` values in
  `apps/web/lib/navigation.ts` and each hub tab's `href` in `apps/web/lib/hub-tabs.ts`. It fails unless a page file
  under `apps/web/app` serves that path, or a `lib/redirects.ts` row sends that path to one a page file serves. A
  route group folder adds no path segment, and a dynamic page such as `app/dashboard/mcp/[id]/page.tsx` does not
  count for a static link. The failure names the link, its href and the page file it expected, for example
  `organization tab Audit log links to /dashboard/admin/audit-logs, and no page serves it and lib/redirects.ts has no row for it: expected app/dashboard/admin/audit-logs/page.tsx`.
- The Registrations tab, shown only on the self-hosted build, is checked like the other tabs.
- If the hosted build stops shipping the page behind a self-hosted-only tab, that tab's href is added to
  `HOSTED_EXEMPT_HREFS` in the same test file. The list is empty. A row leaves out only a hub tab marked `ossOnly`
  with that href, and only where `lib/deployment.ts` sets `HOSTED_DEPLOYMENT` to true; a sidebar entry or a tab
  without the flag is still checked, even with that href. The test fails on a row that names no `ossOnly` tab, for example
  `hosted exemption /dashboard/admin/audit-logs names no ossOnly hub tab: only a tab the hosted build hides may be exempt`.
- Measured on a copy of `apps/web` with `app/dashboard/admin/audit-logs/page.tsx` and its test removed: the existing
  navigation tests (`lib/navigation-edge.test.ts`, `lib/persona.test.ts`, `app/dashboard/layout.drawer.test.tsx`,
  `app/dashboard/layout.role.test.tsx`) passed, 47 of 47, and the new test failed with the line above. With the page
  restored, it passed.

### Security: the startup refusal for a missing `KEYVAULT_MASTER_KEY` names only the key

- When `KEYVAULT_MASTER_KEY` is unset or empty and `ENVIRONMENT` is not exactly `development` or `test`, the API
  refuses to start. Before this change the refusal also offered `ENVIRONMENT=development`, which starts the API on a
  key generated at every start, as a way out. It now names one remedy, the key: `SECURITY ERROR: KEYVAULT_MASTER_KEY
  is required when ENVIRONMENT="<value read>"; set KEYVAULT_MASTER_KEY to the base64 encoding of 32 random bytes
  (generate one with: openssl rand -base64 32) and keep it across restarts`.
- Tests: `apps/backend/internal/crypto/keyvault_test.go` checks, for an unset `ENVIRONMENT` and for `""`, `prod`,
  `production`, `staging` and `Development`, that the refusal contains `KEYVAULT_MASTER_KEY is required` and
  `openssl rand -base64 32` and contains neither `ENVIRONMENT=development` nor `ENVIRONMENT=test`.

### Fixed — `aim-bootstrap` prints its step lines the way the bootstrap guide shows them (#747)

- `aim-bootstrap` prints each step as its number, a space and the step (`1. Checking organization...`), and each
  step's result indented three spaces under it (`   Organization created`), as the sample run in
  `docs/guides/BOOTSTRAP_GUIDE.md` shows. Before this change, for a run against a database with no organization and
  no admin and for a `--default` re-run that finds both, the step lines read `1.Checking organization...`, and the
  result lines other than `Creating organization '<name>'...` began at the first column.
- Tests: `apps/backend/cmd/bootstrap/run_bootstrap_test.go` compares the step lines of a run against a database with
  no organization and no admin with the guide's sample run, and checks the indentation of a `--default` re-run that
  finds the organization and the admin.

### Security — `POST /api/v1/a2a/verify` reports a request from an agent in hybrid mode signed with Ed25519 alone invalid (#748)

- An A2A verification request carries one Ed25519 signature. Once that signature verifies, `POST /api/v1/a2a/verify`
  answers a request from an agent whose hybrid mode is on with `401` and
  `{"valid":false,"error":"Agent is in hybrid mode and this route verifies an Ed25519 signature alone","reasonCode":"hybridSignatureRequired"}`,
  the words and `reasonCode` of the token endpoint's and the SDK verification routes' refusal. Run against
  `A2AService.VerifyA2ARequest` on the previous code, a request from an agent in hybrid mode with status `verified`
  signed with its Ed25519 key alone was reported valid, with the agent's active grants as its capabilities.
- As on those routes, the flag is read after the signature verifies and before the agent's status: a signature that
  does not verify is answered `signature verification failed` with no `reasonCode`, and a suspended agent in hybrid
  mode is told it needs both signatures. The refusal records no nonce and reads no grants. `docs/guides/PQC.md` names
  the route.
- Tests: `apps/backend/internal/application/a2a_verify_hybrid_test.go`,
  `apps/backend/internal/interfaces/http/handlers/a2a_verify_hybrid_test.go`.

### Security — registering an agent or an MCP server applies only tags of the caller's organization (#749)

- `POST /api/v1/agents` and `POST /api/v1/mcp-servers` apply a `tagIds` entry that is a tag id only when that tag
  belongs to the caller's organization. An id of another organization's tag, or an id that names no tag, is skipped
  and the other entries are still applied, so the new agent or server is returned without the other organization's
  tag key, value and description.
- A `tagIds` entry on `POST /api/v1/agents` that is not a tag id is still read as a tag key of the caller's
  organization, found or created there.
- Tests: `apps/backend/internal/application/registration_tags_org_test.go`.

### Fixed — the integration docs mark the CyberArk connector and the Microsoft Sentinel adapter as planned, and Splunk forwarding as self-hosted only

- `README.md`, `docs/server-features.md` and `docs/DEPLOYMENT.md` name Splunk audit log forwarding, which runs in
  self-hosted deployments only, and mark the CyberArk connector and a Sentinel adapter for the Logs Ingestion API as
  planned. Sending session events to CyberArk Privileged Session Manager is under evaluation.
- Microsoft ended support for the Azure Monitor HTTP Data Collector API on 14 September 2026; the docs and the
  Sentinel adapter's comment now say that, where they said the API was retired.
- The webhook request example in `docs/API.md` leaves out `secret`, which `CreateWebhookRequest` does not read, and
  states that the signing secret is not returned and that an unknown event name is accepted and never sent.
- Tests: `apps/backend/cmd/server/integration_claims_docs_test.go`.

### Added: `KEYVAULT_MASTER_KEY` can be rotated with a ring of master keys and a startup sweep

- The backend reads a ring of master keys keyed by the storage key id in the header of each stored agent private key.
  `KEYVAULT_MASTER_KEY` is the current key and `KEYVAULT_MASTER_KEY_ID` its id (1 when unset, the id of every v2 key
  stored before this change); each `KEYVAULT_MASTER_KEY_<id>` adds another key under that id. The backend decrypts under any id
  in the ring, encrypts only under the current id, and refuses a key stored under an id outside the ring. Before this
  change the backend held one master key and refused every storage key id but 1, so a stored agent private key did
  not decrypt once `KEYVAULT_MASTER_KEY` was changed.
- At each start with more than one key in the ring, a sweep re-encrypts each stored agent private key under another
  id in the ring to the current id, one row at a time with a compare-and-swap, so a stopped sweep continues at the
  next start. It logs counts only, including, by id, how many stored keys are still under each other id; an older key
  can leave the ring once that count reads 0. The procedure is in `docs/DEPLOYMENT.md`, "Rotating
  `KEYVAULT_MASTER_KEY`".
- Server signing keys that are derived rather than provisioned come from the current key, so a rotation moves them.
- Tests: `apps/backend/internal/crypto/keyvault_ring_test.go`,
  `apps/backend/internal/application/agent_private_key_rewrap_test.go`, and
  `apps/backend/internal/application/agent_private_key_rewrap_integration_test.go`, which rotates twice on a scratch
  database and decrypts every row before and after each sweep, and part way through the first, a state the test sets
  up by moving two rows before that sweep runs.

### Changed — every HTTP request emits an OpenTelemetry server span, the OTLP exporters choose TLS as the OTLP specification defines, and the gauge `aim_mcp_servers_total` is renamed `aim_mcp_servers`

- Each request the backend serves, other than `/metrics`, starts a SERVER span named `{method} {route}` from the route
  Fiber matched, with `http.request.method`, `url.path`, `url.scheme`, `http.route`, `http.response.status_code`,
  `server.address`, `server.port`, `network.protocol.version`, `client.address` and `user_agent.original`. A 5xx
  answer sets the span's status to Error and `error.type`; a 4xx leaves the status unset. A request answered before
  its route is reached has no `http.route` and is named by its method. No query string, no header other than
  `User-Agent` and no body is recorded. A span a handler starts on the request context, `fga.authorize` included, is
  its child. An incoming `traceparent` is not continued.
- The connection follows `OTEL_EXPORTER_OTLP_ENDPOINT`, or the per-signal `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT`,
  `_METRICS_ENDPOINT` or `_LOGS_ENDPOINT` where one is set: `https://` is TLS and `http://` plaintext whatever
  `OTEL_EXPORTER_OTLP_INSECURE` says; `host:port` is plaintext with `OTEL_EXPORTER_OTLP_INSECURE=true`, TLS with
  `false`, and with neither TLS unless the host is `localhost`, in 127.0.0.0/8 or `::1`. Unset stays `localhost:4317`
  in plaintext, and the empty string still turns telemetry off. The server no longer forces plaintext.
- `OTEL_EXPORTER_OTLP_CERTIFICATE` (the CA that verifies the collector), `OTEL_EXPORTER_OTLP_CLIENT_CERTIFICATE` and
  `OTEL_EXPORTER_OTLP_CLIENT_KEY` are read, with their per-signal forms. A file that cannot be read stops telemetry
  with an error naming the variable, and the server starts without telemetry.
- An endpoint in URL form now exports. Measured against the previous code with `https://127.0.0.1:<port>` and
  `http://127.0.0.1:<port>`: no export arrived, and the exporter reported
  `invalid target address https://127.0.0.1:<port>`, because the URL was passed to it as a gRPC target.
- Upgrade: a `host:port` endpoint whose host is not loopback, with `OTEL_EXPORTER_OTLP_INSECURE` unset, now connects
  with TLS. For a collector that serves plaintext on such a host, set `http://host:port` or
  `OTEL_EXPORTER_OTLP_INSECURE=true`.
- A `host:port` endpoint with an IP address host, such as `127.0.0.1:4317`, still makes the OpenTelemetry trace and
  metric exporters log `parse url ... first path segment in URL cannot contain colon` once each at start: they parse
  the variable themselves whatever the backend passes them. Export works. The URL form logs no such line.
- `aim_mcp_servers_total`, a gauge with a counter's suffix, is served as `aim_mcp_servers`, with the same help text
  and values. A query, alert or dashboard on the old name matches nothing until it uses the new one. A test lints the
  served metrics with the Prometheus linter, which flagged only that name before the change.
- `docs/DEPLOYMENT.md` § Observability gives the endpoint forms, the TLS default, the three certificate variables,
  the request span and its attributes, and the renamed gauge. `apps/backend/docs/OBSERVABILITY.md` no longer says the
  backend dials with TLS disabled.

### Changed — a message the `smtp` email provider sends carries a `Message-ID`, STARTTLS and `AUTH PLAIN` are tested against a server, and the email providers are documented

- Every message the `smtp` provider sends carries one `Message-ID` header (RFC 5322 §3.6.4): `<`, 16 random bytes in
  hex, `@`, the domain of `EMAIL_FROM_ADDRESS` (the SMTP host when the address has no domain) and `>`, fresh for each
  message. The other headers are unchanged, `X-SMTPAPI` included.
- Against a loopback SMTP server that requires STARTTLS and `AUTH PLAIN`, the `smtp` provider's startup check and
  `SendEmail` send only `EHLO` and `STARTTLS` in clear text, authenticate after the TLS handshake and deliver to the
  envelope recipient. A wrong password fails with the server's `535` and no `MAIL` is sent; a server that offers no
  STARTTLS is refused before any `AUTH` or `MAIL`. Implicit TLS on port 465 is not supported.
- `azure` stays the default when `EMAIL_PROVIDER` is unset, so a deployment that relies on it keeps working. A
  provider that fails to start turns email off and the server keeps running.
- `docs/DEPLOYMENT.md` gains § Email: the three providers (`azure`, `smtp`, `console`) and the default, each
  provider's variables, the three of them that are read but not applied (`EMAIL_RATE_LIMIT_PER_MINUTE`,
  `SMTP_MAX_CONNECTIONS`, `AZURE_EMAIL_POLLING_ENABLED`), STARTTLS on `SMTP_PORT` and the refusal without it, what
  the startup check covers for each provider, and where `GET /api/v1/status` reports email (`services.email`).
- Tests: `apps/backend/internal/infrastructure/email/smtp_starttls_test.go`
  (`TestSMTPEmailService_STARTTLSAuthPlainRoundTrip`, `TestSMTPEmailService_MessageIDWithoutFromDomainUsesTheSMTPHost`)
  and additions to `apps/backend/internal/infrastructure/email/smtptest/server_test.go` for the loopback SMTP server's
  new options `RequireSTARTTLS`, `RequireAuthPlain` and `NoSTARTTLS`, written from RFC 3207, RFC 4954 and RFC 4616,
  with a certificate generated at test time for 127.0.0.1 and a record of each command and whether it arrived over TLS.

### Added — security events reach Microsoft Sentinel through the Azure Monitor Logs Ingestion API

- Setting `SIEM_SENTINEL_ENDPOINT`, `SIEM_SENTINEL_DCR_IMMUTABLE_ID`, `SIEM_SENTINEL_STREAM_NAME`,
  `SIEM_SENTINEL_TENANT_ID`, `SIEM_SENTINEL_CLIENT_ID` and `SIEM_SENTINEL_CLIENT_SECRET` configures a `sentinel`
  adapter; `SIEM_SENTINEL_AUTHORITY_HOST` is optional and defaults to `https://login.microsoftonline.com`. With Splunk
  HEC also configured, each event goes to both. When some but not all of the six are set, startup stops with an error
  that names each missing variable and quotes no value. The endpoint and the authority host must use `https`, or
  `http` to a loopback host.
- For a batch, the adapter asks Microsoft Entra ID for a token with the client credentials grant and scope
  `https://monitor.azure.com/.default`, keeps it until 60 seconds before its `expires_in` runs out, and posts the batch
  as one JSON array to `{endpoint}/dataCollectionRules/{dcrImmutableId}/streams/{streamName}?api-version=2023-01-01`.
  Any 2xx answer is delivery. A token request or upload that cannot reach its host, or is answered 408, 429 or 5xx, is
  retried as for Splunk. Any other answer is not, such as a refused token (`invalid_client`), a 2xx token answer
  without a bearer `access_token` or a positive `expires_in` (#767), or an upload answered 401 or 403, and after a 401 or 403 the next batch asks for a new token. A redirect is not followed. No error or log line carries the client secret or a token: an answer that echoes either is redacted.
- Each record has the columns `TimeGenerated` (the event's time, RFC 3339 in UTC), `EventType`, `Severity`,
  `AgentId`, `Capability`, `Outcome`, `Source` (`opena2a-aim`) and `Details` (an object).
- The readiness entry `dependencies.siem` and the counters `aim_siem_events_sent_total{adapter="sentinel"}` and
  `aim_siem_events_failed_total{adapter="sentinel"}` report Sentinel deliveries as they report Splunk's.
- The previous `SentinelAdapter`, which no configuration loaded and which posted to the HTTP Data Collector API, is
  removed.
- `internal/config` reads the seven variables into `config.SIEMConfig`, through which the server's SIEM settings read
  every `SIEM_*` variable. `internal/config/testdata/environment_table.json` has 37 rows for them, each variable
  unset, empty, whitespace only, with a valid and with an invalid value, and `SIEM_SENTINEL_CLIENT_SECRET` also with
  a quote and a backslash; `internal/config` checks each row's setting and `cmd/server` its startup line.
- `docs/DEPLOYMENT.md` § SIEM Forwarding gives the seven variables, the record's columns for the rule's stream
  declaration, how the Entra application is set up, how a refused token shows, and a Log Analytics query to check
  arrival, in place of the sentence that said Sentinel was not supported. `docs/server-features.md` names forwarding
  to Splunk and Microsoft Sentinel, which runs in self-hosted deployments only, where it said a Sentinel adapter for
  the Logs Ingestion API was planned. `apps/backend/cmd/server/integration_claims_docs_test.go` no longer counts
  Sentinel among the integrations whose documentation lines must state a status such as planned.
- Tests: `apps/backend/cmd/server/siem_sentinel_integration_test.go` (`TestSIEMSentinelRoundTrip`),
  `apps/backend/internal/integrations/siem/sentinel_test.go`, `apps/backend/internal/integrations/siem/env_test.go`,
  `apps/backend/internal/integrations/siem/adapter_test.go` and the loopback token and ingestion stubs in
  `apps/backend/internal/integrations/siem/sentineltest`.

### Added — the dashboard adds, edits, tests and deletes alert channels

- A Developers tab, Alert channels (`/dashboard/alert-channels`), shown to admins, managers and members, the roles
  `MemberMiddleware` lets create, change, delete and test a channel. It lists the organization's channels with
  name, kind, severity floor ("alerts at or above <severity>"), active state, the destination as the server answers
  it (Slack, PagerDuty and Teams values as `****` and their last 4 characters, email recipients as stored), and the
  last delivery's status, code and time, or "never".
- Add channel and Edit open one form: name, kind (Slack, PagerDuty, Microsoft Teams, Email), the kind's field (the
  Slack incoming webhook URL, the 32-character PagerDuty integration key, the Teams workflow URL, or up to 20 email
  recipients, one per line) with the format the server takes, and the severity floor. A `400` or `409` answer is
  shown beside the field its message names. On edit the field is empty and shows the current value as the server
  answers it; left empty, the `PUT` carries no `config` and the stored destination stays. `kind` is sent only when
  it changes.
- Test calls `POST /api/v1/alert-channels/:id/test` and shows "Delivered (<code>)", or the failure class the `502`
  answer names followed by "check the URL or key, then test again" (for an email channel, the recipients and the
  server's email service). A PagerDuty test asks first, since it opens an alert on the PagerDuty service. Delete asks
  first. `api.testAlertChannel` returns the `502` body as the outcome instead of the generic server error line.

### Added — alert channels also deliver to a Microsoft Teams workflow and to email addresses

- `POST` and `PUT /api/v1/alert-channels` take two more kinds. `teams` takes `{"webhookUrl": ...}`, the `https` URL of
  a Teams workflow whose trigger is "When a Teams webhook request is received", on any host; a URL that is not
  `https`, has no host, carries a user name or password, or that the webhook URL check refuses is answered `400`
  naming `config.webhookUrl`, and answers carry it as `****` and its last 4 characters. `email` takes
  `{"recipients": [...]}`, 1 to 20 bare addresses that `net/mail` parses to themselves, so an entry with a display
  name, a comma, a CR or an LF is answered `400` naming `config.recipients`; answers carry the recipients unmasked.
- A Teams delivery is a `POST` with `Content-Type: application/json` and no `Authorization` header, of
  `{"type":"message","attachments":[{"contentType":"application/vnd.microsoft.card.adaptive","contentUrl":null,"content":<card>}]}`,
  where the card is an `AdaptiveCard` of version `1.2` whose `body` holds one `TextBlock` per line of the Slack text,
  without Slack's escaping. Any `2xx` is delivered, and the HTTP retry rule applies. Workflows whose trigger needs a
  Microsoft Entra token are not supported.
- An email delivery sends each recipient one plain-text message through the server's email service, with the subject
  `AIM alert (<severity>): <title>` (CR and LF removed, cut to 200 characters) and a body holding the title,
  description, agent, alert id, alert type and the dashboard's alerts page. A recipient the service fails to send to
  is tried up to 3 more times after 1, 4 and 16 seconds and does not stop the others; the delivery is `delivered` only
  when every recipient was sent the message, with `lastDeliveryCode` `null`. The test message's subject is
  `AIM test message for alert channel <name>`. When the server started without an email service, creating an email
  channel, changing a channel to `email` or giving one a `config` is answered
  `400 {"error":"email is not configured on this server"}`, and an existing email channel's deliveries and test
  message fail with the class `email_unavailable` (the test route answers `502`).
- Migration `132_alert_channels_teams_email.sql` widens `alert_channels_kind_check` to `slack`, `pagerduty`, `teams`
  and `email`. An email channel's `config` column holds `{"recipients": [...]}`.
- `docs/API.md` § Alert channels gains both kinds and their config and refusals, the Teams body and the unsupported
  Entra-authenticated triggers, the email subject, body and retry rule, and that email channels need the server's
  email service. `apps/backend/cmd/server/integration_claims_docs_test.go` no longer counts Microsoft Teams among the
  integrations whose documentation lines must state a status such as planned.
- Tests: `apps/backend/cmd/server/alert_channels_teams_email_integration_test.go`
  (`TestAlertChannelsTeamsEmailRoundTrip`, against a loopback Teams receiver built from the trigger's published format
  and the server's SMTP email service sending to a loopback SMTP server),
  `apps/backend/internal/application/alert_channel_teams_email_test.go`,
  `apps/backend/internal/infrastructure/email/smtptest/server_test.go` (the loopback SMTP server, written from
  RFC 5321, in the new test package `smtptest`), and additions to
  `apps/backend/internal/domain/alert_channel_test.go`,
  `apps/backend/internal/infrastructure/repository/alert_channel_repository_test.go` and
  `apps/backend/internal/interfaces/http/handlers/alert_channel_handler_test.go`.

### Added — an organization's alerts reach Slack and PagerDuty in each service's format, with a severity floor per channel

- New routes `POST`, `GET /api/v1/alert-channels` and `GET`, `PUT`, `DELETE /api/v1/alert-channels/:id` manage an
  organization's alert channels, and `POST /api/v1/alert-channels/:id/test` sends one test message through a channel.
  A channel has a `name`, a `kind` (`slack` or `pagerduty`), a `config` (`{"webhookUrl": ...}`, a
  `https://hooks.slack.com/services/` URL, or `{"routingKey": ...}`, a 32-character integration key), a `minSeverity`
  floor and `isActive`. Reading needs any user of the organization; creating, changing, deleting and testing need
  member or above. A channel of another organization is answered `404`. A key the route does not take, an unknown
  `kind` or `minSeverity`, a webhook URL of another form and a routing key that is not 32 characters are answered `400`
  naming the field. Answers carry each `config` value as `****` and its last 4 characters; no answer, log line or audit
  log entry carries the full value. Create, update and delete each write one audit log entry on `alert_channel` with
  `kind` and `minSeverity` as its metadata.
- Every alert the alert repository inserts, whichever service raised it, is put on a queue of 1,024 alerts through the
  repository's `OnCreate` observer, and four workers send it to each active channel of its organization whose floor it
  reaches (`info` < `warning` < `high` < `critical`). Slack is sent `{"text": ...}` with the severity and title, the
  description, the agent and the dashboard's alerts page, with `&`, `<` and `>` in the alert's strings escaped.
  PagerDuty is sent an Events API v2 `trigger` to `https://events.pagerduty.com/v2/enqueue` with the alert's id as
  `dedup_key` and the severity mapped `critical`, `error`, `warning`, `info`. A `429`, a `5xx` or a network error is
  retried up to 3 times after 1, 4 and 16 seconds; other answers are not retried. Each delivery's outcome is recorded as
  the channel's `lastDeliveryStatus`, `lastDeliveryCode` and `lastDeliveryAt`. An alert that arrives when the queue is
  full is sent to no channel and logged with its id; the alert row is unaffected.
- Migration `131_alert_channels.sql` creates `alert_channels`. Its `config` column holds the webhook URL or routing key
  as given.
- `docs/API.md` gains § Alert channels: the routes and fields, the masking, the severity floor, both bodies with the
  severity mapping and escaping, the retry policy and the queue bound, and that a test message to PagerDuty opens an
  alert on the PagerDuty service. `apps/backend/cmd/server/integration_claims_docs_test.go` no longer counts PagerDuty
  among the integrations whose documentation lines must state a status such as planned.
- Tests: `apps/backend/cmd/server/alert_channels_integration_test.go` (`TestAlertChannelsRoundTrip`, against loopback
  Slack and PagerDuty receivers built from each service's published format),
  `apps/backend/internal/domain/alert_channel_test.go`,
  `apps/backend/internal/application/alert_channel_service_test.go`,
  `apps/backend/internal/application/alert_channel_delivery_client_integration_build_test.go`,
  `apps/backend/internal/infrastructure/repository/alert_channel_repository_test.go` and
  `apps/backend/internal/interfaces/http/handlers/alert_channel_handler_test.go`.

### Security — a webhook signature covers the time the attempt was sent, so a receiver can refuse a replayed delivery

- Each webhook delivery attempt, every retry and every `webhook.test` delivery included, is signed when it is sent. It
  carries `X-Webhook-Timestamp: <Unix seconds>` and `X-Webhook-Signature: v1=<hex>`, the HMAC-SHA256 under the
  webhook's secret of `<timestamp>.<raw body>`. The `v1=` prefix names the scheme, so a later one can be sent beside
  it. `X-Webhook-ID`, `X-Webhook-Event` and the body are unchanged; every attempt of one event sends the same body and
  payload `id`.
- The bare-hex form, the HMAC of the body alone, is no longer sent, and no compatibility mode keeps it: before the
  change in this release that returns a webhook's secret on create, no API answer carried the secret
  (`Webhook.Secret` is excluded from JSON), so a receiver had no secret to check that form with.
- Measured through the server's route table against a migrated database on the commit before this change: an
  `agent.created` delivery carried no `X-Webhook-Timestamp`, and its `X-Webhook-Signature` was 64 hex characters with
  no scheme prefix.
- `docs/API.md` § Webhooks gains "Verifying a delivery": the three headers, the signed string, the receiver's checks
  (recompute and compare in constant time; refuse a timestamp more than 5 minutes from the receiver's clock; remember
  each processed event `id` at least that long and refuse to process it twice, while accepting the retry of an event
  it did not process), and a Python example using only `hmac` and `hashlib`.
- Tests: `apps/backend/cmd/server/webhook_replay_guard_integration_test.go` (`TestWebhookReplayGuardRoundTrip`,
  integration tag) checks deliveries with a receiver written from that section, and runs the section's Python example,
  kept in `apps/backend/cmd/server/testdata/webhook_signature_example.py`, against the captured deliveries;
  `webhook_signature_example_docs_test.go` holds the example and the fixture equal;
  `apps/backend/internal/application/webhook_signature_test.go` checks the signed string against a value computed
  with Python's `hmac`, and the headers of an attempt and of a test delivery.

### Changed — a webhook's signing secret reaches its owner once, can be rotated, and a subscription to an unknown event or with an unknown field is refused

- `POST /api/v1/webhooks` takes an optional `secret` of 32 to 255 characters and stores it as given; a shorter one is
  answered `400 {"error":"secret must be at least 32 characters"}` and a longer one
  `400 {"error":"secret must be at most 255 characters"}`. Without `secret`, the server generates a 64-character hex
  secret. The `201` answer carries the webhook and its `secret` once, with `Cache-Control: no-store`;
  `GET /api/v1/webhooks`, `GET /api/v1/webhooks/:id` and `PUT /api/v1/webhooks/:id` never return it.
- `POST /api/v1/webhooks/:id/rotate-secret`, behind the member check and the organization checks of the update route,
  stores a new generated secret and answers `200 {"id": ..., "secret": ...}` once. There is no overlap period:
  deliveries queued after it are signed with the new secret only, and a delivery queued before it, retries included,
  is signed with the secret it was queued with. It writes an `update` audit row for the `webhook` with `rotatedSecret: true` and
  without the secret, and refuses a revoked session as create and update do.
- Create and update refuse a body key they do not take with `400 {"error":"unknown field: <key>"}`
  (`unknown fields: <a>, <b>` for several; an update does not take `secret`), and an event outside the server's list
  with `400 {"error":"unsupported event: <name>"}`. The five names `docs/API.md` lists as accepted and never sent stay
  accepted. A create with `isActive: false` stores the webhook inactive.
- Measured through the server's route table against a migrated database on the commit before this change: a create
  answered `201` without `secret`, both with no `secret` and with a 40-character one; a create answered `201` for a
  31-character `secret`, for `events` containing `auth.failed`, and for a body with a `headers` key; an update with a
  `headers` key answered `200`; and `POST /api/v1/webhooks/:id/rotate-secret` answered `404`.
- Dashboard: the create dialog offers the eleven events the server sends, with the descriptions `docs/API.md` gives
  them, checks a chosen secret's length, and after a create shows the returned secret once with a copy control and a
  line saying it will not be shown again. The details dialog no longer shows a secret field or a delivery history tab,
  neither of which the webhook read carries, and offers "Rotate secret", which after a confirmation shows the new
  secret once. `apps/web/lib/api.ts` adds `rotateWebhookSecret`, reads `lastTriggered`, the name the server sends, in
  place of `lastTriggeredAt`, drops `secret` and `deliveries` from the read types, and requires `name`, `url` and
  `events` in an update, which sets all three from the body.
- `docs/API.md` § Webhooks documents the create fields, the one-time `secret`, the update body, the rotate route and
  the `400` answers. It no longer says that the signing secret is not returned, that a `secret` in the request is
  ignored, or that an event name outside the table is accepted and never sent;
  `apps/backend/cmd/server/integration_claims_docs_test.go` holds the new statements.
- Tests: `apps/backend/cmd/server/webhook_secret_integration_test.go` (`TestWebhookSecretRoundTrip`, integration
  tag), `apps/backend/internal/application/webhook_secret_test.go`,
  `apps/backend/internal/application/webhook_delivery_client_integration_build_test.go`,
  `apps/backend/internal/interfaces/http/handlers/webhook_secret_handler_test.go`, the rotate row of
  `credential_routes_family_test.go`, and in `apps/web` `components/webhook/webhook-create-modal.test.tsx`,
  `components/webhook/webhook-detail-modal.test.tsx` and `lib/api.webhooks.test.ts`.

### Added — a refused or unreachable SIEM shows in `/health/ready` and `/metrics`, and the loss bound is stated

- `GET /health/ready` carries `dependencies.siem` with `required: false`. It is `notConfigured` when no SIEM is
  configured. It is `unavailable`, with the reason `dependency check failed`, while the most recent batch delivery to
  an adapter was not accepted after its attempts, or an event was dropped after the last accepted delivery; the
  response then has `degraded: true` and is still answered 200, because the SIEM is optional. It is `ok` otherwise,
  including before the first delivery, and the next accepted delivery returns it to `ok`. The entry reads the
  forwarder's record of its own deliveries and drops, sends no request to the SIEM, and carries no `latencyMs`. The
  readiness contract is unchanged: `dependencies` already took any named entry of this shape.
- `/metrics` carries `aim_siem_events_sent_total` and `aim_siem_events_failed_total`, each with an `adapter` label,
  and `aim_siem_events_dropped_total`, counted in events, on the registry the other `aim_` series are served from.
  No `aim_siem_` series is registered when no SIEM is configured.
- The forwarder records when its most recent delivery ended and whether it was accepted, and when it last dropped an
  event (`Forwarder.Health`), counts each adapter on its own (`Forwarder.AdapterStats`), and delivers what is queued
  on request (`Forwarder.Flush`).
- `docs/DEPLOYMENT.md` § SIEM Forwarding states the loss bound: a batch refused on its third attempt, about 3 seconds
  after the first, is not sent again, an event that finds 1,000 already queued is not queued, nothing is written to
  disk to send later, and HEC indexer acknowledgment is not requested. It names, for each kind of forwarded event,
  where its record survives outside the SIEM, and the three places a loss shows: the readiness entry, the three
  counters and the `ERROR SIEM delivery failed` log line.
- Tests: `apps/backend/cmd/server/siem_health_integration_test.go` (`TestSIEMHealthRoundTrip`),
  `apps/backend/cmd/server/health_ready_siem_test.go`, `apps/backend/internal/integrations/siem/forwarder_test.go`
  and `apps/backend/internal/integrations/siem/metrics_test.go`.

### Added — alerts and secrets management requests reach the SIEM

- With SIEM forwarding configured, each alert the backend writes is forwarded as an `aim.alert.created` event, whichever
  service raised it: the alert repository gives each alert it inserts to the observers added with its new `OnCreate`,
  and a repeat counted on an alert still open is not forwarded again. The event's `severity` is `info`, `medium`,
  `high` or `critical` for the alert severities `info`, `warning`, `high` and `critical`; `agentId` names the agent
  when the alert's resource is an agent or its metadata names one; `details` carry `alertId`, `organizationId`,
  `alertType`, and `resourceType` and `resourceId` when the alert has them. The alert's title, description, source
  address and metadata are not forwarded.
- Each secrets management request that succeeds now writes one audit log entry in the caller's organization, under
  the caller's user id, with the namespace's id as its resource: `POST /api/v1/secrets/namespaces` writes `create`
  on `secret_namespace`, `POST /api/v1/secrets/namespaces/:id/credentials` writes `create` on `secret_credential`,
  `POST /api/v1/secrets/namespaces/:id/rotate` writes `update` on `secret_credential`, and
  `DELETE /api/v1/secrets/namespaces/:id` writes `delete` on `secret_namespace`. The entry's metadata is the
  namespace's `backendType` only, never the credential, its length or its encryption parameters. A refused or failed
  request writes no entry, and a rotation refused for a revoked session still writes only its
  `credential_mint_refused` entry. The audit log forwarding sends these entries to the SIEM as `aim.audit.create`,
  `aim.audit.update` and `aim.audit.delete`.
- Measured at the previous commit through the route table against a migrated database: two `db:write` verifications
  refused for an agent holding `db:read` in a strict-mode organization wrote three alert rows and no
  `aim.alert.created` event reached the collector; a namespace create, a credential store, a rotation and a namespace
  delete were each answered with success and wrote no `audit_logs` row; a verification refused because the agent was
  suspended was already forwarded as `aim.audit.verification_refused`, and is unchanged.
- `docs/DEPLOYMENT.md` § SIEM Forwarding adds alerts, with their event type, severity mapping and fields, and the
  secrets management entries to the forwarded events, and its audit entry example gives a refused verification's
  `resourceType` as the `agent_action` the backend writes.
- Tests: `apps/backend/cmd/server/siem_alerts_secrets_integration_test.go` (`TestSIEMAlertsAndSecretsRoundTrip`),
  `apps/backend/cmd/server/siem_alerts_test.go`, `apps/backend/internal/integrations/siem/alert_test.go`,
  `apps/backend/internal/infrastructure/repository/alert_on_create_test.go` and
  `apps/backend/internal/interfaces/http/handlers/secrets_handler_audit_test.go`.

### Added — failed sign-ins and FGA denials reach the SIEM next to the audit log entries

- With SIEM forwarding configured (`SIEM_SPLUNK_HEC_URL` and `SIEM_SPLUNK_HEC_TOKEN`), each password sign-in that
  `POST /api/v1/public/login` or `POST /api/v1/auth/login/local` answers `401` is forwarded as an
  `aim.auth.signin_failed` event at severity `low`. Its `details` carry `method` (`password`), `reason` (the check
  that refused the sign-in, such as `password_mismatch` or `unknown_email`), `route`, `ipAddress`, `userAgent`, and
  `userId` and `organizationId` only when the submitted email matched a user. The event never carries the submitted
  email, the password or a token, and every answer of both routes is unchanged: an unknown email is still answered
  as a wrong password is.
- Each decision of `POST /api/v1/agents/:id/authorize` that does not allow the request is forwarded as an
  `aim.fga.denied` event at severity `medium`, with `agentId`, `capability`, `outcome`, and `details` carrying the
  agent's `organizationId`, `deniedBy` and `deniedReason`. An allowed decision is not forwarded. The event is queued
  after the decision is final, so the collector never delays or changes the answer.
- Measured at the previous commit through the route table against a migrated database: a wrong password and an
  unknown email on either route, and a `DENY` from the authorize route, sent nothing to the collector; only the
  successful sign-in's `aim.audit.login` entry arrived.
- `docs/DEPLOYMENT.md` § SIEM Forwarding lists the three kinds of forwarded event with their event types, severities
  and fields, and the reasons each sign-in route gives.
- Tests: `apps/backend/cmd/server/siem_auth_decisions_integration_test.go` (`TestSIEMAuthDecisionsRoundTrip`, both
  routes, against a collector that enforces the published HEC event format and answers),
  `apps/backend/cmd/server/siem_security_decisions_test.go`,
  `apps/backend/internal/integrations/siem/decisions_test.go`,
  `apps/backend/internal/integrations/siem/hectest/receiver_test.go`,
  `apps/backend/internal/application/fga_engine_decision_observer_test.go`,
  `apps/backend/internal/application/auth_service_sign_in_refusal_test.go` and
  `apps/backend/internal/interfaces/http/handlers/sign_in_failure_report_test.go`.

### Security — a logout with any token of an SDK token family ends the family, with or without a revocation store

- `POST /api/v1/auth/logout` with a downloaded SDK's refresh token in the body ends that token's family, whichever
  of the family's tokens is presented: the presented token's `sdk_tokens` row is retired, and every still-active row
  of the family is revoked with the reason `logout`. The family's live token is then refused at
  `POST /api/v1/auth/refresh` with `401`, and `POST /api/v1/auth/sdk/recover` refuses every token of the family.
  The rows decide it, so no revocation store is needed; where one is configured, its family key ends the family as
  well. Another token family of the same account stays live.
- The answer's `revoked.refreshToken` is `true` when the presented row is retired and the family's rows are revoked,
  and, where a revocation store is configured, its writes succeeded too. When one of those writes fails it is
  `false`.
- Tests: `apps/backend/internal/interfaces/http/handlers/auth_logout_sdk_family_test.go`.

### Fixed — the dashboard's API reference lists the fields `POST /api/v1/api-keys` reads and returns (#742)

- The API reference on `/dashboard/developers` lists `agentId` and `name` as the required request fields of
  `POST /api/v1/api-keys`, and its example body carries both. It listed `name`, `expiresAt` and `scopes`, and its
  example body, which had no `agentId`, was answered `400` `agent_id and name are required`.
- The reference says the key expires 90 days after it is created and leaves out `expiresAt` and `scopes`: a key
  created with an `expiresAt` one day ahead in the body still expires 90 days after it is created, and the handler's
  request struct has no `scopes` field.
- The documented response fields are `id`, `apiKey`, `name`, `agentId`, `expiresAt` and `createdAt`, the fields the
  handler returns. The reference listed `apiKey`, `keyId` and `createdAt`.
- `apps/backend/internal/interfaces/http/handlers/api_key_docs_test.go` posts the reference's example body to
  `CreateAPIKey` as a signed-in caller, and fails when the documented request fields are not `agentId` and `name`,
  when the body is refused, when the documented response fields differ from the returned ones, or when the key does
  not expire 90 days after it is created.

### Fixed — reading a user's or an organization's SDK token rows gives the read's error when it fails after the first row (#744)

- The `sdk_tokens` repository's `GetByUserID` and `GetByOrganizationID` check the result set's error after the last
  row and return it. Measured on the previous commit with a two-row result whose second row's read fails, with and
  without revoked rows: each method returned the first row and no error; it now returns the read's error and no rows.
- SDK credential recovery (`POST /api/v1/auth/sdk/recover`) reads the token family through `GetByUserID`, so such a
  read now reaches the route's answer for a family that cannot be read: for a presented token a rotation retired,
  `500` `{"error":"Failed to read the token family"}` and no credential minted.
- Tests: `apps/backend/internal/infrastructure/repository/sdk_token_repository_list_test.go`; the route's answer to a
  failed family read is `TestRecoverRevokedToken_AFamilyThatCannotBeReadMintsNothing`.

### Security — creating an API key requires a live sign-in session

- `POST /api/v1/api-keys` creates a key only for an access token that records a sign-in (one carrying `auth_time`),
  as `GET /api/v1/sdk/download`, `POST /api/v1/auth/sdk/recover` and `POST /api/v1/oauth/device/approve` do. An
  access token minted by refreshing an SDK-download token is answered `401` `Token has been revoked or is invalid`
  before the request body is read; no key is created, and the refusal is recorded once as a
  `credential_mint_refused` audit row and one `SECURITY` line with `route` `api_key_create` and `cause` `noSignIn`.
  An access token whose session was revoked is refused with `cause` `sessionRevoked`. A sign-in's access token, and
  the ones its refreshes mint, create the key with `201`. An access token that carries no session is not checked.

### Security — SDK credential recovery refuses a token family that has ended

- `POST /api/v1/auth/sdk/recover` refuses an SDK-download token whose token family has ended: when any `sdk_tokens`
  row of the family carries the revoke reason `logout`, `token_family_revoked` or `family_max_age`, the route answers
  `401` `{"error":"Token has been revoked or is invalid"}` and writes no row. A logout with any token of the family
  ends recovery of every token of that family, and so do a reuse and a refresh past
  `SDK_TOKEN_FAMILY_MAX_AGE`. The family's rows are the presented token's row, the rows its rotations grew from and
  the rows that grew from it, read from the owner's `sdk_tokens` rows, so the decision needs no revocation store;
  where a store is configured, its family key ends the family as well.
- A token retired by a rotation while its family is live is recovered. An ended family of the same account does not
  end another one.
- When the family's rows cannot be read the route answers `500` `{"error":"Failed to read the token family"}` and
  mints nothing.
- Tests: `apps/backend/internal/interfaces/http/handlers/sdk_token_recovery_family_end_test.go` and
  `apps/backend/internal/application/sdk_token_service_family_rows_test.go`.

### Changed — the backend's print and log lines say what happened in words, with no pictographic mark (#695)

- 384 lines in 34 non-test Go files under `apps/backend` named `fmt.` or `log.` and carried a pictographic
  mark, two of them commented out; the Redis summary for a disabled Redis and the three token revocation
  disabled lines that `cmd/server/main.go` returns as strings carried one too. The marks are gone and the
  words stay: `Database migrations completed successfully`, `Redis: disabled (running without caching)`,
  `Token revocation disabled (Redis unavailable): ...`.
- `aim-bootstrap` numbers its steps `1.` to `4.`, and `docs/guides/BOOTSTRAP_GUIDE.md` quotes those lines
  without the marks. `aim-server migrate` with `POSTGRES_HOST` and `POSTGRES_DB` set and `POSTGRES_USER`
  unset writes `Database configuration: required environment variable(s) not set: POSTGRES_USER` and exits 1.
- The marks counted are the ones `sdk/python/tests/test_no_emoji_in_console_output.py` keeps out of the
  Python SDK's console output. `TestBackendPrintAndLogLinesCarryNoPictographs` reads every non-test Go file
  under `apps/backend` and fails on a line that names `fmt.` or `log.` and carries one.

### Changed — the TypeScript SDK's test titles open with what they check, and the contributing guide says what its tests need (#695)

- 157 test titles in 24 files under `sdk/typescript` opened with a tracking identifier such as `AIM-11.AC4`
  or `CR-002:`, or with an ARP pattern identifier such as `PI-001:`. The tracking identifiers are gone, and a
  pattern identifier follows the behaviour: `detects path traversal (MCP-001)`.
  `tests/test-title-names.test.ts` parses the SDK's tests and fails on a `describe`, `it` or `test` title
  that opens with a capital letter, any capitals or digits, a hyphen and a digit.
- `CONTRIBUTING.md` gives the TypeScript SDK's test commands and says that its `npm test` runs three of the
  repository's documentation lints with `python3` and `bash`, needs `git` and a git checkout, and fails the
  link lint's case in a tree without `.git`, such as a source archive, or with an untracked file that links
  to the `opena2a` GitHub account. `tests/repo-doc-lints.test.ts` checks that the guide names the three
  scripts and quotes the link lint's own message.

### Changed — `GET /api/v1/detection/agents/:id/capabilities/latest` answers 204 for an agent that has sent no capability report

- For an agent of the caller's organization that has no capability report yet, the route answers 204
  with no body. Before this change it answered 404 `{"error":"no capability reports found for this
  agent"}`, measured with the handler and the detection service over a sqlmock connection. The agent
  page's Connections tab reads this route when it renders, and in Chromium each 404 answer printed
  `Failed to load resource: the server responded with a status of 404 (Not Found)` in the console; with
  the 204 the tab shows the same No Capabilities Detected panel and the console has no error.
- An agent the caller's organization does not hold is still answered 404 `{"error":"agent not found"}`,
  and an agent with a report 200 with its latest report. The dashboard's API reference lists the 204 and
  the report's members (`detectedAt`, `environment`, `aiModels`, `capabilities`, `riskAssessment`).

### Fixed — the agent page's Connections and Security tabs and the admin security policies page fit a 375px screen

- Measured in Chromium at 375x812 with the API mocked (the routes of `apps/web/scripts/ux-sweep.mjs`,
  with two violations and three policies): before this change the Connections tab of an agent without a
  capability report ran 38px past the screen's right edge, from the
  `@client.perform_action(capability="file:read")` code line, which has no space to wrap at; the Security
  tab ran 12px past it, from the Refresh and Export CSV buttons beside the title; and
  `/dashboard/admin/security-policies` ran 83px past it, from each policy card's 180px Enforcement Mode
  column beside its text, the Monitoring switch beside the global enforcement description and, on the MCP
  tab, the Create MCP Policy button beside the title. Each now measures 0px: the code lines break inside
  their box, the violations header, the page header and the global enforcement row wrap, and below the
  `sm` breakpoint a policy card stacks its enforcement controls under its text. At 1280x800 the same
  views measure 0px and a policy card keeps the controls beside its text.

### Security — the token endpoint and the SDK verification routes refuse an agent in hybrid mode that signs with Ed25519 alone (#740)

- `POST /api/v1/oauth/token`, `POST /api/v1/sdk-api/verifications` and `GET /api/v1/sdk-api/verifications/:id`
  verify the agent's Ed25519 signature in the handler, outside `PQCAgentMiddleware` and `Ed25519AgentMiddleware`.
  Once that signature verifies, each refuses an agent whose hybrid mode is on with `401`: the two verification
  routes with `{"error":"Agent is in hybrid mode and this route verifies an Ed25519 signature alone","reasonCode":"hybridSignatureRequired"}`,
  the words and `reasonCode` of `Ed25519AgentMiddleware`, and the token endpoint with
  `{"error":"invalid_client","error_description":"Agent is in hybrid mode and this route verifies an Ed25519 signature alone","reasonCode":"hybridSignatureRequired"}`.
  Run against the handlers on the previous code, an agent in hybrid mode with status `verified` that signed with its
  Ed25519 key alone was answered `200` with an access token by the token endpoint and `200` with the verification by
  `GET /api/v1/sdk-api/verifications/:id`, and its `POST /api/v1/sdk-api/verifications` request reached the
  capability check.
- As in the middlewares, the flag is read after the signature verifies and before the agent's status: a signature
  that does not verify gets the route's signature refusal, and a suspended agent in hybrid mode is told it needs both
  signatures. A `POST /api/v1/sdk-api/verifications` body that names `signedBytes` was already refused for an agent
  in hybrid mode. `docs/guides/PQC.md` names the three routes.
- Tests: `apps/backend/internal/interfaces/http/handlers/hybrid_mode_handler_signature_test.go`.

### Security — A2A policy evaluation reads no trust score of another organization's agent (#740)

- `POST /api/v1/a2a/policies/evaluate` reads the A2A trust scores of the body's `clientAgentId` and `remoteAgentId`
  only for agents of the caller's organization, which it looks up with one query that names the organization.
  Another organization's agent is answered as an agent with no score, as an id with no agent is: a policy that
  applies compares a score of 0, and a `DENY` reason states `0.00` for it. Run against the handler on the previous
  code, with an active policy of the caller's organization that applies to every agent and two agents of another
  organization whose stored A2A trust scores were 0.93 and 0.87, the answer was `ALLOW`, which told the caller both
  were at or above the threshold.
- A failed lookup of the agents is answered `500`, and no trust score is read.
- Tests: `apps/backend/internal/interfaces/http/handlers/a2a_policy_evaluation_organization_test.go`.

### Security — the downloaded Python SDK writes the agent's name and the server's URL as escaped Python strings (#740)

- `GET /api/v1/agents/:id/sdk?lang=python` writes `AGENT_ID`, `PUBLIC_KEY`, `PRIVATE_KEY`, `AIM_URL`, `AGENT_NAME`
  and `SDK_VERSION` into `aim_sdk/config.py` as escaped Python string literals, and the agent's name into
  `example.py`'s docstring escaped the same way: a backslash, a double quote, a line break and the other characters
  Go's `unicode.IsPrint` does not count as printable are written as escapes. A value with nothing to escape is
  written as before. Run on the
  previous code, an agent name made of `demo`, a double quote, a line break, `print("injected")`, a line break and
  `#` put `print("injected")` into `config.py` on a line of its own.
- Parsed with Python, the package generated for a name that holds a double quote, three double quotes, two `print`
  calls on lines of their own and a trailing backslash has a `config.py` of its docstring and six assignments, whose
  `AGENT_NAME` equals the name, and an `example.py` whose docstring holds the name.
- Tests: `apps/backend/internal/sdkgen/python_generator_escaping_test.go`.

### Security — a capability verification is denied when the organization's enforcement mode cannot be read (#740)

- `AgentService.VerifyCapability` reads the enforcement mode of the agent's organization to decide whether a denial
  is enforced (strict) or only logged (monitoring). When that read fails or finds no organization, the request is
  denied with the reason `Action denied: the enforcement mode of the agent's organization could not be read`, the
  read's error is returned, and no violation, alert or trust score change is recorded. This holds on each of the four
  paths that read the mode: an agent that is not verified, an agent with no granted capability, a capability the
  agent was not granted, and a granted capability whose definition names a minimum trust score.
- Run on the previous code, an agent with trust score 0.40 holding `file:delete`, whose definition requires 0.50, was
  allowed with `Action allowed by MONITORING mode: agent trust score 40% is below the 50% required for capability
  'file:delete'`, both when the read failed and when it found no organization.
- Tests: `apps/backend/internal/application/agent_service_enforcement_mode_unreadable_test.go`.

### Security — four registration log lines name ids instead of the registrant's address (#740)

- The server log names the registration request, user or organization by id where these lines wrote the
  registrant's address: the approval of an address on `AIM_PLATFORM_ADMINS`
  (`Auto-approving platform admin for registration request <id>`), the account and organization it creates
  (`Auto-created admin user <user id> for registration request <id> in organization <organization id>`), and an
  approval whose account cannot be created, whose lines name the registration request, write neither the address
  nor the provider ID (the address, for an email/password registration) and redact addresses in the database's
  error. The refusal of a registration no administrator could approve names no registrant:
  `registration refused: 0 active administrators, <n> valid AIM_PLATFORM_ADMINS entries, registrant listed=<true|false>`.
- The error that approval returns names the registration request instead of the address. Run on the previous code,
  the output of each of these cases and that error carried the address.
- Tests: `apps/backend/internal/application/registration_mail_log_privacy_test.go`.

### Fixed — of two concurrent decisions on one capability request, the later one answers 409 (#740)

- When the server has no transition recorder, `POST /api/v1/admin/capability-requests/:id/approve` and
  `POST /api/v1/admin/capability-requests/:id/reject` write the new status with one statement that also requires
  the request to be pending, so two decisions that both read the request as pending cannot both be written. The
  later write changes nothing, and its decision answers `409` with `{"error": "capability request is not pending"}`
  and grants nothing. A request with no row when the status is written is answered `404` with
  `{"error": "capability request not found"}`. Before this change the statement's only condition was the request's
  id (`WHERE id = $5`).
- The undo of an approval whose grant fails writes `pending` only while the request is `approved`.
- Tests: `apps/backend/internal/infrastructure/repository/capability_request_status_condition_test.go`,
  `apps/backend/internal/interfaces/http/handlers/capability_request_concurrent_decision_test.go`.

### Changed — a downloaded SDK's credential ends `SDK_TOKEN_FAMILY_MAX_AGE` after its download, and a credential-minting route needs a sign-in's access token

- Each refresh of an SDK-download token issues a new 90-day token. The chain now ends: `SDK_TOKEN_FAMILY_MAX_AGE`
  (a Go duration, `8760h` by default, 365 days) bounds a downloaded SDK's credential from the download, however
  often it is refreshed. The value is read once at boot beside `JWT_SESSION_MAX_AGE` and the value in force is
  logged; a value that is not a positive duration logs a warning and the default applies, and no value switches
  the bound off. A refresh past the bound is answered `401` `{"error":"Invalid or expired refresh token"}`, the
  expired token's answer, before the account is looked up; the presented `sdk_tokens` row is revoked with reason
  `family_max_age`, nothing is minted, and the refusal is recorded once as a `refresh_family_max_age` audit row
  and a `SECURITY` log line, identifiers and the family's birth only. The holder downloads a new SDK with a sign-in.
- The chain's birth travels with it: the row a rotation writes carries the presented row's `familyCreatedAt` in its
  metadata, or the presented row's own `created_at` when it carries none, so the birth survives a purge of the
  download's row and a rotation never makes a chain younger. A chain that exists when this change is deployed is
  read from its current row, so it ends between `SDK_TOKEN_FAMILY_MAX_AGE` minus 90 days and
  `SDK_TOKEN_FAMILY_MAX_AGE` after the deploy. The row `POST /api/v1/auth/sdk/recover` writes carries its own
  `created_at` as its birth. Sign-ins and their `JWT_SESSION_MAX_AGE`, and each SDK-download token's own 90-day
  lifetime, are unchanged. The variable is listed in `apps/backend/.env.example` and `docs/DEPLOYMENT.md`.
- `GET /api/v1/sdk/download`, `POST /api/v1/auth/sdk/recover` and `POST /api/v1/oauth/device/approve` mint only for
  an access token that records a sign-in: one carrying `auth_time`, as a sign-in's access token and the ones its
  refreshes mint do. An access token minted by refreshing an SDK-download token carries the download's session and
  no `auth_time`; it is refused at the three routes with their existing `401` `Token has been revoked or is
  invalid` and recorded once as a `credential_mint_refused` audit row whose `cause` is `noSignIn`. A revoked
  session's refusal, at these three routes and at the API key, agent key, secret rotation and webhook routes that
  check the session, now records `cause` `sessionRevoked` on its row and its `SECURITY` line. A downloaded SDK's
  credential therefore cannot download, recover or approve a new credential; that takes a sign-in. Creating an API
  key takes one too (see the entry above). The agent key, secret rotation and webhook routes do not check for a
  sign-in. An access token minted before this release carries no `auth_time` and is refused at the three routes
  until its sign-in is refreshed; an access token lives `JWT_ACCESS_TTL` (`2h` by default).

### Added — `POST /api/v1/auth/reauth` re-authenticates a signed-in user with their password

- `POST /api/v1/auth/reauth` takes the signed-in user's password and the refresh token of the session the bearer
  access token belongs to. The password is checked as at sign-in: a wrong password is recorded as a failed attempt
  and counts toward the lockout, and a locked account, an account without a password and a suspended, deactivated
  or deleted account are refused with `401`. A refresh token of another session, of another user, or of another
  kind (an access token, an SDK-download token) is refused with `401` before the password is read, and so is an
  access token minted from an SDK-download token used as the bearer.
- The presented refresh token is retired as at `POST /api/v1/auth/refresh`: a token already retired by a rotation,
  or one that loses the retirement to a concurrent refresh, is a reuse and ends the session. The successor pair stays
  in the session and both tokens carry an `auth_time` of the time of the re-authentication; the answer has the
  refresh route's shape, and without a revocation store the presented refresh token comes back unchanged
  (`rotated: false`) with an access token that carries the new `auth_time`. The session's maximum age
  (`JWT_SESSION_MAX_AGE`) counts from the re-authentication; a session already past it is not re-authenticated, since
  its access token is refused with `401` as on every route. A recovered SDK credential's `sdk_tokens` row follows
  the rotation. The re-authentication is recorded as a `login` audit row with `method` `reauthentication`.
- The route shares the strict rate limit of the other `/api/v1/auth` routes (10 requests a minute).
- `docs/API.md` documents the route and lists it among the credential routes of the rate limit.
- Tests: `apps/backend/internal/interfaces/http/handlers/auth_reauth_handler_test.go`,
  `apps/backend/internal/infrastructure/auth/jwt_auth_time_test.go` and
  `TestStrictRateLimiterCoversExactlyTheCredentialRoutes`.

### Fixed — the backend integration suite passes against a fresh backend with the production rate limits (#688)

- Measured with the suite and a backend built from the previous commit, on a fresh database with
  `ENVIRONMENT=test`: 277 top-level tests ran, 258 passed and 19 failed, none on a 429. The causes, each fixed in the suite:
  - 15 tests that send `POST /api/v1/agents` signed in as a user whose registration the admin approved, which the
    server gives the viewer role; the request answered 403 `Member access required (viewers cannot perform this action)`.
    Those tests now sign in through `CreateTestManager`, which sets the manager role between the approval and the
    sign-in: with the member role, 13 of them still failed, on 403 `Manager or admin access required` from the agent
    delete, suspend and verify routes they go on to call.
  - `TestMCPServer_GetByName` put a server name with spaces in the query string unescaped, and the server answered 400.
    The name is query-escaped.
  - `TestMCPServer_CrossOrganizationAccessDenied` registered the first address of a new domain and expected it to be
    approved into an organization of its own; the server held the registration, and the login answered
    `Account not yet approved by administrator` with no access token. The server approves such a registration only
    for an address on `AIM_PLATFORM_ADMINS`, so the test registers `TEST_CROSS_ORG_EMAIL`, checks that the user is in
    an organization other than the admin's, and is skipped when the variable is unset.
  - `TestOAuthTokenEndpointExists` and `TestOAuthTokenResponseFormat` signed an assertion with no `exp` and the audience
    `aim`, and the token endpoint answered 401 `Assertion is missing a valid exp claim`. The assertion carries `iat`,
    an `exp` two minutes ahead and the backend address as its audience.
- With those fixed and the backend on `ENVIRONMENT=development` (general limit 100 a minute, strict 10), 8 tests that
  send `/api/v1/mcp-servers` requests with no token were answered 429 where they expected 401. That group's limiter runs
  before the JWT middleware and counts every request from one address, signed in or not; the suite's rate-limit gate
  now holds a request to the group once its window is spent, as it does for the public, auth and OAuth groups.
- After the change, on a fresh database with `ENVIRONMENT=development` and `TEST_CROSS_ORG_EMAIL` on the backend's
  `AIM_PLATFORM_ADMINS`: 282 top-level tests ran, 282 passed, the backend's request log holds no 429, and the run took
  435 s. Against the same database with `ENVIRONMENT=test`, 282 passed in 23 s.
- `tests/README.md` names `TEST_CROSS_ORG_EMAIL`, and the backend HTTP API row of `docs/testing/release-smoke.md` gives the
  run command with it and the `AIM_PLATFORM_ADMINS` setting it needs, in place of the note that the row was broken.
- Tests that need no backend: `tests/integration/create_test_user_test.go`, `tests/integration/oauth_assertion_test.go`,
  and the MCP server group cases in `tests/integration/strict_rate_limit_gate_test.go`.

### Added — the server delivers agent, trust score, alert and API key events to subscribed webhooks, within limits on retries and pending deliveries (#672)

- The server sets its webhook service on the alert, agent and API key services, so the events
  `docs/API.md` lists under Webhook Events are sent to the active webhooks subscribed to them.
  `TestInitServices_SetsTheWebhookServiceOnTheEventServices` holds the wiring.
- An attempt waits at most 30 seconds for its endpoint and a delivery makes at most 5 retries, whatever
  the webhook's row stores, and the first retry delay is at most 30 minutes. A webhook created through
  the API stores none of these, so its deliveries wait 30 seconds per attempt and retry a failed attempt
  3 times, after waits of 60, 120 and 240 seconds. Each wait before a retry lasts the delay the retrying
  record names in `nextRetryAt`, and an abandoned delivery's record names none.
- At most 1,000 event deliveries are pending at once (sending or waiting to retry), and at most 100 for
  one webhook. At most 5 event delivery attempts are in flight, and at most 2 to one webhook; the wait
  before a retry holds no attempt slot. A delivery beyond a pending limit is not sent: it is recorded as
  `abandoned` with an error that names the limit, and the server log says so.
- On shutdown, after the API listener stops, the server stops the webhook service and waits up to 2
  seconds: a delivery waiting for a retry or an attempt slot is recorded as `abandoned`, and an event
  raised after that is not sent. A delivery record the database refuses is written to the server log.
- `docs/API.md` describes the delivery, retries and limits under Delivery and retries, and
  `docs/guides/API_REFERENCE.md` and the A2A guide no longer say that delivery is off. Tests:
  `apps/backend/internal/application/webhook_delivery_bounds_test.go`,
  `apps/backend/cmd/server/webhook_wiring_test.go`.
- Not sent yet: `api_key.expired`, `security.violation`, `compliance.violation`,
  `security.capability_drift` and `security.data_exfiltration`; a trust score change other than an
  administrator's update and a capability violation, including
  `POST /api/v1/agents/:id/trust-score/recalculate`; and an alert raised outside the alert service.

### Security — `PQCAgentMiddleware` admits an agent in hybrid mode only by a request signed with both Ed25519 and ML-DSA, and `Ed25519AgentMiddleware` refuses it

- `PQCAgentMiddleware` refuses a request from an agent whose hybrid mode is on unless its `X-Algorithm`
  is `Ed25519+ML-DSA-44`, `Ed25519+ML-DSA-65` or `Ed25519+ML-DSA-87` and both signatures verify. The
  refusal is a 401 with `{"error":"Agent is in hybrid mode: sign the request with both Ed25519 and ML-DSA
  (X-Algorithm Ed25519+ML-DSA-44, Ed25519+ML-DSA-65 or Ed25519+ML-DSA-87)","reasonCode":"hybridSignatureRequired"}`,
  counted under `aim_s1_refusals_total{reason="hybrid_required"}`. `Ed25519AgentMiddleware`, which
  verifies an Ed25519 signature alone, refuses an agent in hybrid mode with the same `reasonCode` and the
  message `Agent is in hybrid mode and this route verifies an Ed25519 signature alone`. Before this
  change both middlewares answered 200 to a hybrid-mode agent's request signed with its Ed25519 key alone.
- The hybrid-mode flag is read after the signature verifies and before the agent's status: a signature
  that does not verify gets the signature refusal, and a suspended hybrid-mode agent signing with one
  key is told it needs both signatures, not its status.
- `PQCAgentMiddleware` passes a request that carries an `Authorization` header to the next middleware
  without reading its signature, so turning hybrid mode off through `POST /api/v1/agents/:id/hybrid-mode`
  with a user's access token is not affected.

### Security — refusing an agent key that is not recognised costs the signature verification an invalid signature costs

- The agent-signature key check (`agentauth.Keys`) verifies the request's signature against a decoy key
  before it refuses an unknown agent, an agent with no registered key for the algorithm, a registered key
  that does not decode, or a presented key that is not the registered one. The decoy keys are generated
  once per process and their private keys are discarded. The refusal and its body are unchanged; it now
  costs the verification a known agent's signature with one byte flipped costs: one Ed25519
  verification for an Ed25519 or hybrid request, one ML-DSA verification at the request's level for an
  ML-DSA request.
- Before this change those refusals returned before any signature was verified. In the package's test
  harness on one development machine, an unknown agent's ML-DSA-87 refusal took a median of 41ns against
  143µs for a known agent's signature with one byte flipped; after it, 139µs against 141µs.

### Fixed — the `s1_refusals` line written on shutdown states how long its last period lasted

- On shutdown the server writes the `s1_refusals` line of the period still open. Its `period_s` is now
  the seconds from that period's first refusal to the shutdown, rounded up and at most 64; a period that
  runs its full length still reads `period_s=64`. Before this change the shutdown line read `period_s=64`
  for periods ended 0, 0.3, 5, 12 and 63 seconds after their first refusal.

### Fixed — the one-time MCP trust rescoring runs again at the next start when a server failed to score

- The first start after the upgrade scores every MCP server and then records the
  `mcp_trust_rescored_after_scale_migration` key in `system_config`. The key is now recorded only by a
  run in which every server scored. When any server fails, the server log reads `MCP trust rescoring did
  not complete, will retry at next start: <n> server(s) scored, <m> left unchanged after a scoring
  failure`, and the next start scores every server again, so a server that failed once gets another
  attempt instead of keeping its rescaled placeholder until its next verification. Before this change the
  key was recorded once every server had been attempted, the failed ones included.

### Fixed — the sign-in, password change, password reset and registration pending pages render no text below 14px

- The field labels, field errors, notes and lists on `/auth/login`, `/auth/change-password`,
  `/auth/reset-password` and `/auth/registration-pending` render at 14px (`text-sm`) at a 375px
  viewport. Before this change they used `text-xs` (12px). Each page's `page.test.tsx` asserts that no
  text renders below 14px: the login and change-password forms before and after a submit with empty
  fields, the reset-password form, success and expired-link states, and the registration pending page
  with and without a request ID.
- The two Show password controls on `/auth/reset-password` are 44x44 CSS px (`h-11 w-11`) with the
  icon centred, the size the login and change-password pages use. Before this change the button set no
  size of its own and drew only its 16px icon.

### Security — the demo render keeps its run secrets in a directory `mktemp` creates

- `docs/demo/render.sh` writes the secrets it generates for a run into a directory `mktemp -d` creates
  under `$TMPDIR`, and deletes that directory when the run ends. Before this change it used
  `$TMPDIR/aimdemo-<UTC second>`, a name another account can create first: given a directory of that
  name that already existed, it wrote its `compose.env`, which holds the secrets, into that directory
  and deleted the directory at the end of the run.

### Security — A2A policy evaluation reads the policies of the caller's organization (#652)

- `POST /api/v1/a2a/policies/evaluate` evaluates the active A2A policies of the organization in the
  caller's credentials. It passed the body's `organizationId` to the evaluation as given. Run against the
  handler with the policy store stubbed, a body naming another organization was answered `200` with that
  organization's decision and the names of its active policies in `policiesEvaluated`, and a body naming
  no organization read the policies of the zero UUID.
- A body whose `organizationId` names another organization is answered
  `403 {"error":"organizationId must be your own organization; leave it out to evaluate your organization's policies"}`
  before any policy is read. A body that leaves `organizationId` out or names the caller's own
  organization is evaluated for the caller's organization, and a caller whose credentials carry no
  organization is answered `401`.
- Tests: `apps/backend/internal/interfaces/http/handlers/a2a_policy_evaluation_organization_test.go`.

### Security — a logged-out SDK-download token is not recovered, with or without a revocation store (#612)

- `POST /api/v1/auth/sdk/recover` refuses an SDK-download token whose `sdk_tokens` row was revoked by
  `POST /api/v1/auth/logout` (revoke reason `logout`), with the refresh route's 401 answer
  `Token has been revoked or is invalid`, as it refuses a token whose chain a reuse ended. Where a revocation
  store is configured, the logout's family key already made it refuse that token; where none is configured, the
  route answered 200 with a new token pair and stored a new `sdk_tokens` row for it. Both outcomes were measured
  with the handler tests on a downloaded token that its owner logged out and then presented for recovery.
- Test: `TestRecoverRevokedToken_RefusesALoggedOutSDKToken`, with and without a revocation store.

### Fixed — MCP server creation requires a name and a URL, and an agent's MCP server list holds each server once (#623)

- `POST /api/v1/mcp-servers` and `POST /api/v1/sdk-api/agents/{id}/mcp-servers` answer 400 `name and url are
  required` (or the one that is missing) for a body without a name or a URL. They stored a server with an empty
  name and URL, and answered the next such body 409. That body was what the generated Python SDK sent to add a
  server to an agent's list.
- `PUT /api/v1/agents/{id}/mcp-servers` compares the given identifiers with the agent's list by MCP server: a server
  of the organization already on the list by its ID is not added again by its name, or the other way round.
  It compared them as strings, so one server added both ways left two entries, and `total_count` said 2 while
  `GET /api/v1/agents/{id}/mcp-servers` listed one server. An identifier that names no server of the organization
  is compared as given.
- `PUT /api/v1/agents/{id}/mcp-servers` answers an empty list with `mcpServerIds is required and must not be
  empty`, naming the member it reads. It named `mcp_server_ids`.
- The Python SDK generated for an agent's download sends `register_mcp` to `PUT /api/v1/agents/{id}/mcp-servers`
  with `mcpServerIds`, `detectedMethod`, `confidence` and `metadata`. It posted `mcp_server_ids` and
  `detected_method` to the route that creates an MCP server. AIM adds to an agent's list only for a signed-in user
  with the member role or higher and the generated SDK holds the agent's signing keys only, so the call raises an
  `AuthenticationError` that says so and points to the agent's page in the dashboard.
- The deleted-agent notice of the agents list wraps an agent name that has no space or hyphen. At 375 px a
  41-character name made the page scroll sideways and put the Dismiss button out of view, in the list view and in
  the error view.

### Fixed — an SDK verification checks the minimum trust score of the capability's definition (#658)

- `AgentService.VerifyCapability`, which decides `POST /api/v1/sdk-api/verifications` (where `@perform_action` and
  `performAction()` post each call), `POST /api/v1/verifications` and `POST /api/v1/agents/:id/verify-capability`,
  compares the agent's trust score with the `minTrustScore` of the requested capability's definition once it finds
  the capability among the agent's grants. The definition is the organization's own, else the core one; a wildcard
  grant such as `system:*` is checked against the definition of the action requested, such as `system:admin`. In
  strict enforcement mode an agent below the minimum is denied, with a reason such as
  `Action denied: agent trust score 40% is below the 50% required for capability 'file:delete'`, and the SDK route
  answers 403. In monitoring mode the request goes on through the security policy checks and is allowed; the
  `reason` `verify-capability` answers with says the agent is below the minimum. A capability with no definition, or
  not in `namespace:action` form, has no minimum. A failure to read the definition is returned as an error: the SDK
  route answers 403 and `verify-capability` answers 500. The check reads the trust score the agent has; it does not
  recalculate it.
- The same check is in `CapabilityService.VerifyAction`, whose handler no route mounts; both give the same reason.
- `docs/server-features.md`, `docs/use-cases/enforce-capabilities.md` and `docs/sdk/trust-scoring.md` say where the
  minimum is checked and that monitoring mode allows the action. `enforce-capabilities.md` said the minimums could be
  changed through the REST API; no route changes a capability definition, and it says so. Its table gives
  `file:delete` and `network:listen` as high-risk examples; it gave `db:delete`, which has no core definition, and
  `data:export`, which is medium-risk.
- Test: `agent_service_capability_trust_threshold_test.go`.

### Fixed — the guides say a call for a capability the agent was not granted is refused only in strict mode (#681)

- `docs/quick-start.md`, `docs/use-cases/enforce-capabilities.md` and `docs/DEMO_SCRIPT_ECHOLEAK.md` say that a new
  organization starts in monitoring mode, where the decorator registers the capability, AIM grants it and the call
  runs, and that AIM refuses such a call only in strict mode. Each names where an administrator turns strict mode on:
  Global Enforcement Mode on `/dashboard/admin/security-policies`, or `PUT /api/v1/admin/enforcement-settings` with
  `{"enforcementMode": "strict"}`. The quick-start said the decorated function runs only if verification passes, the
  use case said actions outside the capabilities list are denied automatically, and the demo script said AIM prevents
  EchoLeak-style attacks, each with no mode named.
- The pages also say that strict mode refuses every action of an agent that is not verified and does not verify an
  agent when it registers. The use case has `my-agent` verified before its example runs, and the demo script gains a
  setup step that sets strict mode and a step that verifies the agent before its legitimate request.
- `ROADMAP.md` keeps the Python SDK line's statement that an action AIM denies raises an error and does not run, and
  adds that AIM denies a capability the agent was not granted only in strict mode. It said "in every enforcement mode"
  and named neither mode.
- `aim-sdk login` keeps its closing text on the two modes and adds where an administrator turns strict mode on:
  Global Enforcement Mode on `/dashboard/admin/security-policies`.
- `apps/backend/internal/application/enforcement_mode_docs_test.go` runs the capability check in monitoring and strict
  mode, reads the migration and organization repository default, the dashboard's security policies page and the
  enforcement settings routes, and fails when one of these pages says the call is refused without naming strict mode
  or does not name where strict mode is set. `sdk/python/tests/test_cli_device_login.py` checks the `aim-sdk login`
  closing text.

### Fixed — approving or rejecting a capability request that is no longer pending answers 409 (#738)

- `POST /api/v1/admin/capability-requests/:id/approve` and `POST /api/v1/admin/capability-requests/:id/reject`
  answer `409` with `{"error": "capability request is not pending"}` for a request that is approved,
  auto-approved or rejected when the decision reads it, as after another administrator decided it first, and
  write nothing. Before this change both routes answered such a request, in each of the three statuses, with `500` and
  `An internal error occurred. Please try again later.`: no check covered a request that is not pending, and the
  not-found check compared the error's text with exactly `capability request not found`, which the service
  wraps as `capability request not found: <cause>`.
- A request the route's ownership check found and that has no row when the decision reads it again is answered
  `404` with `{"error": "capability request not found"}`. Before this change both routes answered it with `500`.
  A read that fails for another reason is still answered `500`.

### Fixed — saving an MCP server's edits keeps a consensus withdrawal made since the server was read (#738)

- `MCPServerRepository.Update`, through which `MCPService.UpdateMCPServer`, `VerifyMCPServer` and
  `CreateMCPServer` (for a URL already registered) save a server, writes `name`, `description`, `url`, `version`,
  `public_key`, `verification_url`, `capabilities` and `updated_at`. Before this change it also wrote `status`,
  `is_verified` and `last_verified_at` from the server value its caller had read before the edit.
- After a server is created, its verification is written by `VerifyServer`, `UpdateMCPVerificationStatus` and
  `WithdrawConsensusVerification`, each in one statement. A save from a value read before a consensus withdrawal
  now leaves the server pending. Before this change that save wrote the server back to `verified` with
  `is_verified` true and `consensus_verified_at` NULL, and both `WithdrawConsensusVerification` and the startup
  recheck's `ListConsensusVerifiedMCPServerIDs` read only servers whose `consensus_verified_at` is set.

### Fixed — MCP security policies match host names in any case, report unreadable rules, and refuse a floor off the trust scale (#593)

- An `allowedDomains` or `blockedDomains` entry matches a server's host without regard to case or a
  trailing dot. A `blockedDomains` entry blocks each of these pairs of server URL and pattern, which
  it did not block before: `https://x.EVIL.com/sse` and `*.evil.com`, `https://x.evil.com./sse` and
  `*.evil.com`, `https://evil.com./sse` and `evil.com`, `https://evil.com/sse` and `*.EVIL.com`.
- An `mcp_*` policy whose rules hold a value of a JSON type its rule format does not take, such as
  the string `"50"` for `minTrustScore`, is reported as violated, with the violated rule
  `Policy rules could not be decoded` and a reason that names the key. Before, such a policy
  reported no violation: an allowlist with `{"allowedDomains": ["only.example.org"],
  "minTrustScore": "50"}` did not report a server at `https://mcp.example.com/sse`.
- `POST /api/v1/admin/security-policies` and `PUT /api/v1/admin/security-policies/:id` answer 400
  for `mcp_*` rules that do not match the policy type's rule format, and for an `mcp_allowlist`
  `minTrustScore` outside 0 to 1, such as `70`; the error names the key. Before, create answered
  201 and update 200 and stored the rules as sent, for a floor of `70`, `-0.1` or `"50"` and for a
  string where a list, number or boolean belongs. The rules of the other policy types are not
  checked.
- Migration 130 adds `logging` and `completions` to the `allowedCapabilities` of the seeded
  "Trusted MCP Server Domains" allowlist where the list is still the seeded
  `["tools", "resources", "prompts"]`, so a server on an allowed domain that declares either is not
  reported as "Capability not in allowlist". The policy stays disabled.
- The API reference's example for `POST /api/v1/admin/security-policies` sends every field the
  handler binds, including `policyType`, `enforcementAction`, `severityThreshold`, `appliesTo` and
  `priority`, under the name `isEnabled` where it said `enabled`, and rules for a `trust_score_low`
  policy under `trust_threshold`, the key that type reads. It no longer sets `rules.action`, which
  no evaluator reads.
- From a source archive (`git archive`), `apps/web/tracked-build-cache.test.ts` and the tracked-file
  scan in `sdk/python/tests/test_enforcement_deadline.py` are skipped with the reason. Before, they
  failed with `not a git repository` and `CalledProcessError`.

### Added — the server grants delegations between agents and verifies a delegation chain, applying each delegation's trust attenuation factor and floor (#679)

- `POST /api/v1/a2a/delegations` grants a delegation from one of the organization's agents to another, for a
  manager or admin. A field left out takes the `a2a_delegation` column default: `trustAttenuationFactor` 0.8,
  `minDelegatedTrustScore` 0.3, `maxChainDepth` 3, `canDelegateFurther` false. An agent of another
  organization is answered 404, and a second active delegation for the same two agents 409.
  `GET /api/v1/a2a/delegations` and `GET /api/v1/a2a/delegations/{id}` read the organization's delegations,
  and `POST /api/v1/a2a/delegations/{id}/revoke`, for a manager or admin, marks one inactive. Each grant and
  revoke is recorded in the audit log; a record that cannot be written is logged by the server and does not
  undo the change.
- `POST /api/v1/a2a/delegations/verify` takes the IDs of a chain of the organization's delegations, root
  first, and answers `valid` with each delegation's effective trust. The trust starts at the root
  delegator's trust score and every delegation multiplies it by its own `trustAttenuationFactor`, rounded to
  six decimal places. A delegation whose effective trust is below its floor fails; the floor is the highest
  `minDelegatedTrustScore` of that delegation and the ones before it. With a root delegator at 0.9 and three
  delegations at the defaults the figures are 0.72, 0.576 and 0.4608; with a root delegator at 0.5 the third
  is 0.256 and the chain is not valid. A delegation also fails when it is revoked or outside its validity
  window, does not continue the chain, follows a delegation without `canDelegateFurther`, exceeds a
  `maxChainDepth`, names an agent whose status is not verified or pending, or does not cover the requested
  `skillId`.
- Before this change the only backend Go code, tests aside, that named either field was the `A2ADelegation`
  struct that declares them, and no backend Go code outside tests registered a `/delegations` route.
  The [A2A guide](docs/guides/A2A.md#delegation-trust-attenuation) describes the routes and the checks, and
  [Trust Scoring](docs/sdk/trust-scoring.md#delegation-trust-attenuation), which said the server applies
  neither value, says where it applies both.

### Fixed — login and logout audit rows record the client address a trusted proxy reports

- The login audit row (written by the local login, public login, SDK token recovery and device
  code routes) and the logout audit row record the same client address the refresh route's
  reuse and refusal rows record: the connecting peer, or, when `TRUSTED_PROXIES` names the peer,
  the address the proxy reports in `X-Real-IP` or `X-Forwarded-For`. A reported value that does
  not parse as an address is not recorded; the row names the peer instead. Without
  `TRUSTED_PROXIES` the headers are ignored and the row names the peer. Before this change, a
  `POST /public/login` and a `POST /api/v1/auth/logout` sent through a trusted proxy with
  `X-Real-IP` set recorded the proxy's address in their rows.
- The refresh route's `sign-in past the maximum session age refused` log line writes the same
  client address in its `ip=` field.
- The client address these rows and the refresh route's reuse and refusal rows record is copied
  out of the request, not a view of the request buffer, which the server reuses for later
  requests. The event SIEM forwarding queues from such a row is serialized after the request has
  ended, and it names the address of the request that wrote the row.

### Fixed — an A2A request that fails is no longer answered 2xx, and every failure response the A2A handler builds comes from one mapping

- Each failure response the A2A handler builds names one failure kind, and `a2a_failures.go` holds the status and
  reason every kind is answered with; the handler itself builds no other failure response. Refusals that come from
  shared helpers the handler calls keep the answers they had: an agent that is unknown or belongs to another
  organization is answered 404 `{"error":"not found"}` by the tenant-scope helper; a request with no organization
  in its context is answered 401 by the application's error handler; an agent credential that names another agent's
  ID on `POST /api/v1/a2a/cards` is answered 403 `agent_path_mismatch` by the agent path binding, and 500 when the
  authenticated agent cannot be read; and a request with an organization but no user in its context on
  `POST /api/v1/a2a/consent/:id/revoke` is answered 401 by the application's error handler.
- `a2a_failures_test.go` injects each failure an A2A route can meet on each of the 48 routes the A2A handler
  serves, 257 cases in all: malformed JSON, a path or query value that is not UTF-8 text or holds a NUL
  character, no organization in context, a policy evaluation that names another organization, an unknown,
  foreign, suspended or unreadable agent, a failed repository statement, a nonce that is empty, used before
  or cannot be read or recorded, and a signature that does not verify.
  Eleven answers changed, each measured with that test before and after. Every other case answers the status
  and body it answered before, byte for byte.
- `GET /api/v1/a2a/agents/:id/trust-score` and `GET /api/v1/a2a/trust/:id`, when the stored score cannot be
  read or is zero and computing it fails, answer 500 with the fixed server error line and log the cause. Before, both answered 200
  `{"agentId":"<agent id>","message":"Could not compute trust score","score":0}`.
- `GET /api/v1/a2a/tasks` answers 400 `Invalid agent ID` for an `agentId` that is not a UUID, measured with
  `billing`, a byte that is not UTF-8 and a NUL character. Before, it answered 200 and ignored the `agentId`.
- `GET /api/v1/a2a/consent/check` answers 400 `scope must be UTF-8 text without a NUL character` for such a
  `scope`. Before, a `scope` that is not UTF-8 was answered 200 with `hasConsent: false`, and a `scope` holding
  a NUL character was answered 500, because the database refuses it inside the consent check's JSON.
- `POST /api/v1/a2a/consent/:id/revoke` answers 400 `Invalid request body` for a body that is present and is
  not JSON, and revokes nothing. Before, it answered 200 `Consent revoked successfully` with `revoked: true`. A
  request with no body, or only white space, is still revoked with the default reason.
- `GET /api/v1/a2a/consensus/:agentId/:skillId` and `GET /api/v1/a2a/agents/:id/skills/:skillId/consensus`
  answer 400 `skillId must be UTF-8 text without a NUL character` for a `skillId` that is not UTF-8. Before,
  both answered 200 with a consensus result of no attestations.
- `DELETE /api/v1/a2a/skills/:id` answers 400 `id must be UTF-8 text without a NUL character` for an `id` that
  is not UTF-8. Before, it answered 204.
- `a2a_failures_census_test.go` fails when the A2A handler builds a 4xx or 5xx response, or calls
  `respondServerError`, `respondResourceNotFound` or `fiber.NewError`, outside the mapping.

### Changed — `PUT /api/v1/a2a/cards/:id` replaces the agent card document

- The route takes the registration body, `cardUrl` or `cardData`, and runs the registration
  `POST /api/v1/a2a/agents/:id/card` runs: it fetches the card from `cardUrl` or takes `cardData`,
  recomputes `cardHash`, issues a new attestation, stores the card in place of the previous one and
  answers the re-registered card with 200. The audit row's actor is the authenticated user, or the
  calling agent when the caller has no user.
- The route does not edit individual fields. A body with neither `cardUrl` nor `cardData` is answered
  400 with `cardUrl or cardData is required: this route replaces the agent card document and does not
  edit fields`, and the field-level members it carried (`name`, `description`, `version`, `endpoint`,
  `skills`) are named in the message and in `unreadMembers`. Another organization's agent is answered
  404 whatever the body carries.
- The 400 is the failure kind `a2aCardDocumentRequired` of the A2A failure mapping. `a2a_failures_test.go`
  sends this route a card document, and injects a body with neither member, one with field-level members
  only, and a card URL the server refuses, which brings it to 260 cases.

### Fixed — a capability request stores the metadata the agent sends

- `POST /api/v1/sdk-api/agents/:id/capability-requests` reads `metadata`, a JSON object of context
  for the reviewing admin, stores it with the request as sent and returns it with the request.
  Metadata that is not a JSON object is answered 400 `invalid request body`.

### Changed — the dashboard writes "MCP servers" in sentence case

- The admin users page's link to the MCP page, the agent connection graph's server count and the
  supply-chain HTML report's summary card and section heading read `MCP servers`. Before, they read
  `MCP Servers`; the navigation and the MCP page title already read `MCP servers`.
- The agent's MCP server list says `No MCP servers connected` when it is empty and titles its bulk removal
  dialog `Remove multiple MCP servers`. The server picker's button reads `Add MCP servers` and its dialog
  `Add MCP servers to agent`.
- The end-to-end test for a fresh agent's connections tab expects `No MCP servers detected`, the text the tab
  renders. Before, it expected `No MCP Servers Detected`, which matched the tab only because a text match ignores case.

### Security — a verification event logged for an unknown or unreadable agent answers the same error as a foreign agent

- `LogVerificationEvent` answers `agent not found` when the agent lookup fails, whether the agent
  does not exist or the read itself failed, the same error it answers for an agent of another
  organization and the same rule `CreateVerificationEvent` applies. The lookup's cause is written
  to the server log as `Verification event refused: agent <agent id> could not be read: <cause>`
  and is not part of the returned error.

### Fixed — the dialog Close button is a 44x44 px target (#731)

- The Close button `DialogContent` draws in a dialog's top-right corner
  (`apps/web/components/ui/dialog.tsx`) is 44x44 CSS px (`h-11 w-11`), with no variant that
  sizes it smaller, and centres its 16px X icon. Before this change the button set no size of its
  own and drew only the icon; #731 reports it measured 15x15 on the Detect MCP servers dialog at
  375px and 1280px wide. The button sits 2px in from the corner, so the icon is 16px in from the
  top and right, where the old `right-4 top-4` placed the button.
- `apps/web/components/ui/dialog.test.tsx` fails when the button's classes allow a height or width
  under 44px or do not centre the icon.

### Fixed — the Detect MCP servers dialog says its command can replace the agent's key pair

- Above its commands, the dashboard's Detect MCP servers dialog states that when the SDK on the
  agent's machine has no saved credentials for the agent, the command creates a new key pair and
  replaces the agent's public key with it before it registers the MCP servers. In that case
  `secure('<name>', auto_detect_mcp=True)` finds the agent by name and sends
  `PUT /api/v1/agents/{id}/keys` with the new public key.

### Security — a reused SDK-download token is recorded as the same client or a different one

- A refresh token from an SDK download is retired by its `sdk_tokens` row, not by a denylist
  entry, so a reuse of one carried no mark to compare with: its `refresh_token_reuse` audit row
  and `SECURITY` log line recorded `clientMatch` as `unknown` in each case measured against the
  refresh route (the client that rotated presenting the retired token again, with and without a
  revocation store; a presentation from another address; one with another user agent; the loser
  of two concurrent presentations). The reuse is now compared with the row the retiring rotation
  created, which stores the address and user agent of the client that rotated, under the rule
  the login family uses: `sameClient` when the address class (IPv4 exactly, IPv6 by its /64) and
  the user agent match, `differentClient` otherwise, `unknown` when that row cannot be read or an
  address does not parse. The token is refused and its family ends as before, whatever the
  classification; the login family's comparison is unchanged.
- The row a rotation creates stores the client address the route resolved through
  `TRUSTED_PROXIES` (`X-Real-IP`, then `X-Forwarded-For`), which is what the comparison reads,
  where it stored the connecting address; without `TRUSTED_PROXIES` the two are the same address.
  A forwarded value that does not parse as an address is not stored; the connecting address is.
- Tests: `apps/backend/internal/interfaces/http/handlers/auth_refresh_sdk_client_match_test.go`
  pins the three outcomes on the row path and what the rotation row stores;
  `TestCompareClients` in `apps/backend/internal/infrastructure/auth/client_mark_test.go` pins
  the comparison rule.

### Fixed — the token rotation guide says how long a stolen token keeps working

- `docs/security/token-rotation.md` said that token rotation "prevents attackers from using stolen tokens", that
  without it stolen tokens work "forever" and that with it they "expire immediately" after use. It now says that a
  stolen refresh token is refused once rotation has retired it, and how long a stolen copy keeps working where
  refreshes rotate. When the agent refreshes first, the copy is retired: presenting it is refused and ends the token
  family, the agent's new token included. When the thief refreshes first, the thief can keep refreshing until the
  agent's next refresh presents the retired token, which ends the family. While the agent is silent, the thief's
  tokens keep working at most this long: a sign-in from `aim-sdk login` until `JWT_SESSION_MAX_AGE` after the sign-in
  (8 hours by default), however often it is refreshed; a token from an SDK download, which has no maximum age, as
  long as it is refreshed at least every 90 days; an access token `JWT_ACCESS_TTL` from its issue (2 hours by
  default), since a refresh does not retire it. On a server without Redis a stolen copy of a sign-in's refresh token
  keeps working as long as the agent's own, until the sign-in ends.
- Its "Security Benefits" list says that a refresh that rotates retires the refresh token it was given and that a
  retired refresh token presented again is refused and ends its token family, where it said "Every token use triggers
  revocation of previous token", "No manual revocation needed" and "Immediate protection against stolen credentials".
- Test: `TestTokenRotationGuideStatesHowLongAStolenTokenWorks` in
  `apps/backend/internal/interfaces/http/handlers/token_rotation_guide_stolen_token_test.go` checks each of these
  statements against the refresh route and the auth middleware at the server's default lifetimes, and fails when the
  guide says "expire immediately".

### Changed — the dev stack takes its host ports and container names from the environment

- `docker-compose.yml` reads `AIM_PROJECT` (`aim`, the prefix of all eleven container names) and the
  quick-start's `AIM_BACKEND_PORT` (`8080`), `AIM_FRONTEND_PORT` (`3000`), `AIM_POSTGRES_PORT`
  (`5432`) and `AIM_REDIS_PORT` (`6379`) with the same defaults, plus one host port per other
  service: `AIM_ELASTICSEARCH_PORT` (`9200`), `AIM_ELASTICSEARCH_TRANSPORT_PORT` (`9300`),
  `AIM_MINIO_PORT` (`9000`), `AIM_MINIO_CONSOLE_PORT` (`9001`), `AIM_NATS_PORT` (`4222`),
  `AIM_NATS_MONITOR_PORT` (`8222`), `AIM_PROMETHEUS_PORT` (`9090`), `AIM_GRAFANA_PORT` (`3003`) and
  `AIM_LOKI_PORT` (`3100`). Rendered with none of them set, the file yields the same ports and names
  as before. The defaults of `FRONTEND_URL`, `ALLOWED_ORIGINS`, the dashboard's `NEXT_PUBLIC_API_URL`
  and Grafana's `GF_SERVER_ROOT_URL` follow the host ports. Every service other than the backend and
  the dashboard stays bound to `127.0.0.1`.
- The "From source" section of `docs/DEPLOYMENT.md` and `.env.example` list every variable with its
  default.

### Fixed — an MCP server loses consensus verification when its attesting agents stop agreeing

- A server that multi-agent consensus verified returns to `pending` when its attestations stop
  agreeing: the latest attestation of an attesting agent reports a failed connection, or the
  attesting agents report different tool sets. The server is rechecked each time one of its
  attestations is recorded or revoked, and once at every backend start. When an attestation is
  recorded after that and the attestations agree again, consensus verifies it again under the same
  agent, owner and confidence thresholds.
- A server verified through SDK registration or through `POST /api/v1/mcp-servers/:id/verify`
  keeps its status when attestations disagree, because that verification does not rest on the
  attestations. Running `POST /api/v1/mcp-servers/:id/verify` on a server that consensus verified
  moves it into this group.
- A server with no valid agent attestation left keeps its status: expired or revoked attestations
  are not a disagreement.
- Migration 129 adds `mcp_servers.consensus_verified_at`, set when consensus verifies a server. It
  marks a server verified before this change as verified by consensus when no agent registered it,
  an agent attestation was recorded at or before its last verification, and it has no successful
  challenge-response verification event. The recheck at the first start after the migration
  withdraws the verification of every marked server whose attestations disagree.

### Security — sign-in failure and lock alert log lines name no email address

- The server log lines a password sign-in writes name no account email address. The account-locked
  and auth-failure-pattern alert lines name the organization ID. The warnings for a failed lockout
  check, failure record, failure count or lockout insert print the store's error with each address
  in it replaced by `[address]`, and the warning for failures not cleared after a sign-in names the
  user ID. The alert records keep the address in their title, description and metadata.

### Security — the SDK download leaves out the SDK's tests and tooling and states no schedule

- `GET /api/v1/sdk/download` builds its zip from the SDK's tree without the SDK's own tests and
  development tooling: for Python, `tests/`, `scripts/`, `tools/`, `conftest.py`, `pytest.ini`,
  `requirements-dev.txt` and `test_sdk_comprehensive.py`; for Java, `src/test/`. A `tests`
  directory, a `conftest.py` and a `test_*.py` module are left out at any depth. The SDK's
  sources, packaging, README and `examples/` still ship, beside `QUICKSTART.md` and the embedded
  `.aim/sdk_credentials.json`.
- The Python `QUICKSTART.md` points at the `examples/` directory for more examples.
- A request for an SDK the endpoint does not serve is answered
  `400 {"error":"Invalid SDK type '<sdk>'. Available SDKs: 'python', 'java'."}`, and the
  endpoint's API description names the Python and Java SDKs it serves and gives no release date
  for any other.

### Security — a refused A2A card URL is a 400 with a fixed reason, and MCP verify and detect keep the cause in the server log

- `POST /api/v1/a2a/agents/:id/card` and `POST /api/v1/a2a/cards` answer a card URL the server
  refuses to fetch with `400 {"error":"cardUrl must be a public http or https URL"}`: a URL that
  does not parse, is not http or https, has no host or names a blocked host, a host the resolver
  reports does not exist, and an address the egress address policy refuses, whether that address
  comes from the check when the URL is accepted or from the lookup when the client connects (a
  host whose DNS answer changes to loopback between the two is refused, and no request reaches
  it). The reason, which names what the URL resolved to, is written to the server log with the
  agent ID and is not in the response. A host lookup that fails for another reason, such as a
  resolver that answers SERVFAIL, and a fetch, parse or storage failure, are answered with the
  fixed 5xx line. `ValidateExternalURL` keeps its "URL hostname could not be resolved" text and
  now keeps the lookup error reachable through `errors.As`.
- `POST /api/v1/mcp-servers/:id/verify`: the audit entry of a verification that failed records the
  server name and the `unreachable` status and no error text, and the failure is written to the
  server log with the server ID. A failure to generate the verification challenge is logged the
  same way. `POST /api/v1/mcp-servers/:id/detect-capabilities` logs a failure to read the detected
  capabilities with the server ID. Neither route's response carries the service's error.

### Added — an opt-in request-pool role and a row security census, both off by default

- `POSTGRES_POOL_ROLE` names a database role the server's request pool runs as. When it is set,
  every connection the pool opens runs `SET ROLE` to that role once, before the pool hands the
  connection out. If the database refuses the `SET ROLE`, that connection is closed and the error
  returned, so no connection serves a statement as the login itself; at startup the pool's ping
  fails and the server stops with `Failed to connect to database:` and the refusal. The role needs
  no login and no password: the server still logs in as `POSTGRES_USER`. Unset or empty, the
  default, the pool sets no role.
- The schema migrations, the first-administrator seed from `ADMIN_PASSWORD` and the one-time
  conversion of stored agent private keys to the v2 format run on a second handle that logs in the
  same way and never sets `POSTGRES_POOL_ROLE`; the server closes it before it listens.
  `aim-server migrate` runs on that handle too.
- Every migration transaction, in the server and in the standalone `cmd/migrate` tool, first runs
  `SELECT set_config('app.rls_service_scope', 'on', true)`. Where row-level security policies read
  that setting, a migration reads and writes every tenant's rows; where none does, it changes
  nothing.
- `POSTGRES_SCOPE_CENSUS=on` makes the request pool count the statements whose text names a
  tenant-scoped table (`organizations`, `users`, `agents`, `api_keys`, `mcp_servers`,
  `audit_logs`, `tags`, `agent_capabilities`, `mcp_attestations`, `a2a_peer_trust`, `a2a_skills`)
  and, of those, the ones that run outside a transaction that declared a tenant
  (`app.current_organization_id`) or the service scope (`app.rls_service_scope`). Every five
  minutes and at shutdown the server prints `row security census: statements=<n> undeclared=<n>`,
  then one `row security census: undeclared site=<file>:<line> count=<n>` line per code location
  that ran an undeclared statement. The lines carry counts and code locations only, never
  statement text or argument values. Unset, the default, or any other value leaves the census off.
- The tenant-scope helpers these settings serve, `WithTenant` and `WithServiceScope`, are added
  in `apps/backend/internal/infrastructure/database`. No request path calls them in this release.

### Security — webhook test deliveries send the event-delivery headers, are recorded as deliveries and log their failures, and a refused webhook URL is a 400 with a fixed reason

- `POST /api/v1/webhooks/{id}/test` sends the headers an event delivery sends: `Content-Type`,
  `X-Webhook-Signature`, `X-Webhook-Event`, `X-Webhook-ID` and `User-Agent: AIM-Webhook/1.0`. Its
  delivery record carries the webhook's organization, a `success` or `failed` status (with
  `HTTP <code>` as the error of a non-2xx answer), the duration, the attempt and completion times,
  and at most 1 KB of the endpoint's answer. A test delivery that gets no answer (the connection is
  refused, or the egress client refuses the address) is recorded as a failed delivery with the
  connection error. A record that cannot be stored leaves the test's result unchanged, and the
  failure to store it is written to the server log.
- A failed test is reported as `Webhook test failed: the endpoint responded with status <code>` or
  `Webhook test failed: the endpoint could not be reached`. The delivery error is not in the
  answer; it is written to the server log and kept in the delivery record.
- `POST /api/v1/webhooks` and `PUT /api/v1/webhooks/{id}` answer a URL the server refuses (one that
  does not parse, is not http or https, has no host, names a blocked host or a host the resolver
  reports does not exist, or resolves to an address the egress policy refuses) with
  `400 {"error":"url must be a public http or https URL"}`, and write the reason to the server log.
  A host lookup that fails for another reason, such as a resolver that answers SERVFAIL, says
  nothing about the URL and is answered with the fixed 5xx line, as any other service error is.
  `ValidateExternalURL` keeps its `URL hostname could not be resolved` text and now keeps the
  resolver's error for `errors.As`.

### Fixed — stopping the action-request nonce purge job waits for the job to exit

- Stopping the action-request nonce purge job cancels a purge in flight and returns once the job's
  goroutine has exited, so its stop line can no longer reach a later test's captured log and the
  server's shutdown no longer leaves the job running. A purge the stop cancelled writes no line.

### Security — an access token ends with the sign-in it belongs to

- The access token issued with a login pair, and the one each refresh of that login issues, now
  carries the sign-in's time (the registered `auth_time` claim, the same value as its refresh
  token's). A request on a user-authenticated route whose access token's sign-in is older than
  `JWT_SESSION_MAX_AGE` (8 hours by default) is answered with the existing
  `401 {"error":"Invalid or expired token"}`, by comparing that claim with the clock and with no
  read of the revocation store, so the dashboard asks the user to sign in again. Until now the
  middleware read no sign-in time, so a login access token that was within its own expiry and not
  revoked was accepted whatever the age of its sign-in; the last one a refresh issued before the
  limit therefore stayed usable for up to `JWT_ACCESS_TTL` (2 hours by default) past it.
- Access tokens without `auth_time` are not checked and live at most `JWT_ACCESS_TTL` from their
  issue: those issued before this release, those exchanged from an SDK-download token (which has
  no maximum session age), and standalone ones. Routes with optional authentication treat a
  request carrying an ended sign-in as anonymous. Tokens stay opaque to clients and no client
  changes.

### Changed — the A2A multi-agent demo's Java program follows `AIM_BACKEND_PORT` and `AIM_FRONTEND_PORT`

- `examples/a2a-multi-agent-demo/java` `A2AMultiAgentDemo.java` used `localhost:8080` when `AIM_URL`
  was not set, and printed its dashboard links on the backend URL, which serves no dashboard. Against
  a stack started on other host ports it reached nothing, or, beside a default stack on the same
  host, that stack. It now reads `AIM_BACKEND_PORT` (`8080`) and `AIM_FRONTEND_PORT` (`3000`) from
  the environment, then the repository's `.env` (three levels above `java/`, the directory it runs
  from), then the defaults, through the new `StackEnv.java`; `AIM_URL` still wins for the backend,
  and the dashboard links use the dashboard port. The example's README lists the variables for both
  demos.

### Changed — the A2A multi-agent demo's Python scripts follow `AIM_BACKEND_PORT` and `AIM_FRONTEND_PORT`

- `examples/a2a-multi-agent-demo/python/run_demo.py` used `localhost:8080` when neither `--aim-url`
  nor `AIM_URL` was given, and printed its dashboard links on the backend URL, which serves no
  dashboard. `research_agent.py` and `analysis_agent.py` fell back to `localhost:8080` when given an
  API token and no URL. Against a stack started on other host ports they reached nothing, or, beside
  a default stack on the same host, that stack. They now read `AIM_BACKEND_PORT` (`8080`) and
  `AIM_FRONTEND_PORT` (`3000`) from the environment, then the repository's `.env`, then the
  defaults, through the new `stack_env.py`; `--aim-url` and then `AIM_URL` still win for the
  backend, and the dashboard links use the dashboard port. The example's README lists the variables
  with their defaults.

### Changed — the LangChain CRUD and MCP server demos follow `AIM_BACKEND_PORT` and `AIM_FRONTEND_PORT`

- `examples/langchain-crud-agent/run-langchain-crud.sh` checked the backend's health on
  `localhost:8080` whatever `AIM_API_URL` said, and printed dashboard links on `localhost:3000`.
  `langchain_crud_agent.py` defaulted to `localhost:8080` and built its dashboard link by replacing
  `8080` with `3000` in the backend URL, so with `AIM_API_URL=http://localhost:18999` it linked
  `localhost:18999`. Against a stack started on other host ports they reached nothing, or, beside a
  default stack on the same host, that stack. Both now read `AIM_BACKEND_PORT` (`8080`) and
  `AIM_FRONTEND_PORT` (`3000`) from the environment, then the repository's `.env`, then the defaults
  (the agent through the new `stack_env.py`); an `AIM_API_URL` that is set still wins for the
  backend. When the backend does not answer, the launcher prints the health check it tried.
- `examples/mcp-server-demo/mcp-server.py` prints the dashboard page to register it on, with the
  port from `AIM_FRONTEND_PORT` read the same way. Both example READMEs name the variables with
  their defaults.

### Changed — the flight-search demo's helper scripts follow `AIM_BACKEND_PORT`, `AIM_FRONTEND_PORT` and `AIM_POSTGRES_PORT`

- `examples/flight-search-agent/get_fresh_sdk.py` and `verify_qa_complete.py` called the backend on
  `localhost:8080` and printed dashboard links on `localhost:3000`, `demo_search.py` and
  `test_flight_agent.py` printed the dashboard on `localhost:3000`, and `check_sdk_token.sh` ran
  `psql` on the default port 5432. Against a stack started on other host ports they reached nothing,
  or, beside a default stack on the same host, that stack. They now read `AIM_BACKEND_PORT`
  (`8080`), `AIM_FRONTEND_PORT` (`3000`) and `AIM_POSTGRES_PORT` (`5432`) from the environment, then
  the repository's `.env`, then the defaults, through `stack_env.py` for the Python scripts and
  `stack-env.sh` for the shell script. The example's README lists the new variables.
- When a deny does not land, `flight_agent.py` told the operator to run
  `curl localhost:8080/healthz`, a path the backend does not serve, whatever URL the agent used. It
  now prints `curl -sf <the agent's backend URL>/health`.

### Changed — the flight-search demo's stage scripts reach the stack through `AIM_PROJECT` and `AIM_BACKEND_PORT`

- `examples/flight-search-agent/reset-demo.sh` and `prep-stage.sh` ran every query in the fixed
  container `aim-postgres` and called the backend on `localhost:8080`. Against a stack started with
  another `AIM_PROJECT` or `AIM_BACKEND_PORT` they failed with `No such container`, or, beside a
  default stack on the same host, reset and seeded that stack instead. They now use the container
  `$AIM_PROJECT-postgres` and the backend at `http://localhost:$AIM_BACKEND_PORT`, each read from the
  environment, then the repository's `.env`, then the defaults (`aim`, `8080`). The strict-mode
  command `prep-stage.sh` prints names the same container, and the example's README lists both
  variables with their defaults.
- When the backend does not answer the admin login, `prep-stage.sh` prints the URL it tried and the
  health check to run, where it used to stop on a Python traceback.

### Fixed — the audit log list names the agent as the actor of a record that carries an agent id

- The actor cell of the dashboard's audit log list (`auditActor` in `apps/web/lib/audit-records.ts`) names the agent
  whenever a record carries an agent id, also when the record carries the agent owner's user id beside it, as the
  record of a verification of an agent's action does. This matches the `actorType` member that
  `GET /api/v1/admin/audit-logs` sends with each record. A record with only a user id names that user, and a record
  with neither says "No user or agent recorded".
- A test cell in `apps/web/lib/audit-records.test.ts` covers a record that carries both an agent id and a user id.

### Changed

- An addition of MCP servers to an agent refused because its audit record could not be written answers `503` with
  `recordUnavailable` and `Retry-After`, and the Add MCP servers dialog says nothing changed and when to try again.
- An addition widens what the agent may reach, an expansion. When the agent service has a transition recorder, an
  addition is made only together with its audit record; when the record path cannot be used the service refuses it
  whole and the agent's talks_to list is unchanged. The server does not set a transition recorder, so an addition
  writes no audit record, only its audit log entry, and this refusal does not occur.
  `PUT /api/v1/agents/:id/mcp-servers` now answers that refusal as the agent delete does: `503`, `Retry-After: 300`
  and `{"error": "<fixed line>", "reasonCode": "recordUnavailable", "code": "recordUnavailable"}`. The cause stays in
  the server log. Any other failure still answers as before.
- Measured with the handler of the previous commit, with the service returning the recorder's refusal: the route
  answered `500` with `{"error":"An internal error occurred. Please try again later."}`, no `reasonCode` and no
  `Retry-After`, and the dialog showed "An internal error occurred. Please try again later." in its red error box.
- The dialog now states this refusal once, inside the dialog, in the same neutral "Nothing was changed" alert as the
  delete dialog, with when to try again taken from `Retry-After`. Its text is the dashboard's own, never the server's
  line. The selection is kept, so the next attempt is one click, and that attempt or closing the dialog clears the
  alert. While the addition runs the Add button is `aria-disabled` rather than `disabled`, so focus stays in the
  dialog, and the dialog does not close until the server answers. Any other failure is shown as before.
- The dashboard's API reference states the `503`.
- Tests: `apps/backend/internal/interfaces/http/handlers/agent_talks_to_record_refusal_test.go`, the reason-code census
  in `refusal_reason_code_test.go`, and `apps/web/components/agents/mcp-server-selector.test.tsx`.

### Changed

- A reactivation or a verification of an agent refused because its audit record could not be written answers `503`
  with `recordUnavailable` and `Retry-After`, and the agent page says nothing changed and when to try again.
- Both let the agent act again, an expansion. When the agent service has a transition recorder, each is made only
  together with its audit record; when the record path cannot be used the service refuses the act whole and the
  agent's status is unchanged. The server does not set a transition recorder, so a reactivation or a verification
  writes no audit record, only its audit log entry, and this refusal does not occur.
  `POST /api/v1/agents/:id/reactivate` and `POST /api/v1/agents/:id/verify` now answer that refusal as the agent
  delete does: `503`, `Retry-After: 300` and
  `{"error": "<fixed line>", "reasonCode": "recordUnavailable", "code": "recordUnavailable"}`. The cause stays in the
  server log. A suspension narrows what the agent can do and is never refused by the record path. Any other failure
  of the three routes still answers as before.
- Measured with the handlers of the previous commit, with the service returning the recorder's refusal: both routes
  answered `500` with `{"error":"An internal error occurred. Please try again later."}`, no `reasonCode` and no
  `Retry-After`, and the agent page showed its red failure alert, "AIM could not reactivate this agent."
- The agent page now states this refusal once, beside the Verify and Reactivate buttons, in the same neutral
  "Nothing was changed" alert as the delete dialog, with when to try again taken from `Retry-After`. Its text is the
  dashboard's own, never the server's line. The next lifecycle action clears it, and it is never shown together with
  the red failure alert.
- `docs/API.md` states the `503`.
- Tests: `apps/backend/internal/interfaces/http/handlers/agent_lifecycle_record_refusal_test.go`, the reason-code
  census in `refusal_reason_code_test.go`, and `apps/web/app/dashboard/agents/[id]/page.lifecycle-outcomes.test.tsx`.

### Changed

- An approval of a capability request refused because its audit record could not be written answers `503` with
  `recordUnavailable` and `Retry-After`, and the approve dialog says nothing changed and when to try again.
- The approval and its grant are an expansion. When the capability request service has a transition recorder, they
  are made only together with their audit record; when the record path cannot be used the service refuses the
  approval whole and nothing is approved or granted. The server does not set a transition recorder, so an approval
  is made without an audit record and this refusal does not occur.
  `POST /api/v1/admin/capability-requests/:id/approve` now answers that refusal as the agent delete does: `503`,
  `Retry-After: 300` and `{"error": "<fixed line>", "reasonCode": "recordUnavailable", "code": "recordUnavailable"}`.
  The cause stays in the server log. Any other failure still answers as before.
- Measured with the handler of the previous commit, with the read of the request's record failing on a lock timeout:
  the route answered `500` with `{"error":"An internal error occurred. Please try again later."}`, no `reasonCode`
  and no `Retry-After`, and the dashboard closed the dialog and showed a destructive "Action Failed" alert on the
  page that quoted the server's line.
- The approve dialog now stays open while the decision runs and states this refusal once, inside the dialog, in the
  same neutral "Nothing was changed" alert as the delete dialog, with when to try again taken from `Retry-After`.
  Focus stays in the dialog. The next attempt, or closing the dialog, clears it. Any other failure still closes the
  dialog and is shown on the page. A rejection narrows nothing and is never refused by the record path.
- Tests: `apps/backend/internal/interfaces/http/handlers/capability_request_record_refusal_test.go`, the reason-code
  census in `refusal_reason_code_test.go`, and `apps/web/app/dashboard/admin/capability-requests/page.test.tsx`.

### Changed — a delete refused because its audit record could not be written answers 503 with `recordUnavailable` and `Retry-After`, and the delete dialog says nothing changed and when to try again

- When the agent service has a transition recorder, an agent's deletion is made only together with its audit record;
  when the record cannot be written the service refuses the delete whole and nothing is removed. The server does not
  set a transition recorder, so a delete writes no audit record, only its audit log entry, and this refusal does not
  occur.
  `DELETE /api/v1/agents/:id` now answers that refusal with `503`, `Retry-After: 300` (the seconds a failed record
  write keeps the server's record path unavailable) and
  `{"error": "<fixed line>", "reasonCode": "recordUnavailable", "code": "recordUnavailable"}`. The cause stays in the
  server log. Any other delete failure still answers `500` with the route's own line and no `Retry-After`.
- Measured with the handler of the previous commit, for the recorder's refusal and for the same refusal wrapped once
  more: both were answered `500` with `{"error":"The agent was not deleted and nothing was removed. Try again, and if
  it fails again, contact your administrator."}`, no `reasonCode` and no `Retry-After`.
- The API sends `Retry-After` in `Access-Control-Expose-Headers`, so the dashboard on its own origin can read it, and
  the dashboard's API client keeps it on the refusal as `retryAfterSeconds`.
- The delete dialog states this refusal once, inside the dialog, in an alert of the default variant (the person did
  nothing wrong) titled "Nothing was changed", with when to try again taken from `Retry-After`. Its text is the
  dashboard's own, never the server's line. The next attempt clears it.
- `docs/API.md` states the `503`.
- Tests: `apps/backend/internal/interfaces/http/handlers/record_unavailable_refusal_test.go`, the reason-code census in
  `refusal_reason_code_test.go`, `TestCORSMiddleware_ExposesHeaders`, and `apps/web/lib/api.retry-after.test.ts`,
  `apps/web/lib/record-path-refusal.test.ts` and `apps/web/components/agents/delete-agent-dialog.test.tsx`.

### Security — webhook, MCP server and agent card requests refuse the Dummy IPv6 Prefix and the unassigned part of 2001::/23

- The address policy behind `ValidateExternalURL` and the connection check of `NewEgressClient` also refuses
  100:0:0:1::/64 (Dummy IPv6 Prefix, RFC 9780) with the class `dummy prefix`, and 2001::/23 (IETF Protocol
  Assignments) with the class `IETF protocol assignments`. Teredo, benchmarking, ORCHID and ORCHIDv2 addresses inside
  2001::/23 are refused with their own class, as before.
- The assignments inside 2001::/23 that the IANA IPv6 Special-Purpose Address Registry marks globally reachable stay
  admitted: 2001:1::1, 2001:1::2 and 2001:1::3 (anycast), 2001:3::/32 (AMT), 2001:4:112::/48 (AS112-v6) and
  2001:30::/28 (Drone Remote ID Protocol Entity Tags).
- Measured with the test below on the commit this change is based on, `DeniedAddressClass` admitted the ten
  addresses of the test in these two ranges, and `ValidateExternalURL` admitted the same ten as IP-literal URLs, among
  them `https://[100:0:0:1::1]:443/`, `https://[2001:5::1]:443/` and `https://[2001:100::1]:443/`.
- Tests: `apps/backend/internal/infrastructure/utils/url_validator_special_purpose_test.go` passes 14 addresses that
  must be refused, each with its class, and 9 that must be admitted through `DeniedAddressClass` and, as IP-literal
  URLs, through `ValidateExternalURL`.

### Security — the welcome and approval mail lines and three approval lines log ids instead of the user's address

- On a registration by an address that is not in `AIM_PLATFORM_ADMINS` and its approval, seven log lines wrote the
  user's address. They name the registration request id or the user id instead: the welcome mail sent and not sent
  lines, the first-user admin line, the approval-with-password line, the account created line, and the approval mail
  sent and not sent lines.
- When the welcome or approval mail is not sent, the provider's error goes through the address redaction the password
  reset mail's failure line uses; in the test, the recipient in an SMTP `550` reply is logged as `[address]`.
- This change does not touch the platform admin auto-approval lines, the account creation failure lines or the line
  logged when a registration is refused because no administrator can approve it.
- Tests: `apps/backend/internal/application/registration_mail_log_privacy_test.go` registers and approves a user,
  once with both mails sent and once with both failing with a provider error that echoes the recipient, captures
  standard output, standard error and the standard logger, and fails when the address appears or when the request
  id or user id is missing.

### Security — webhook, MCP server and agent card requests refuse eight more special-purpose address ranges

- The address policy that the URL check and the connection check share also refuses 192.88.99.0/24 (6to4 relay
  anycast), ::/96 (IPv4-compatible), ::ffff:0:0:0/96 (IPv4-translated), 2001:2::/48 (benchmarking), 2001:10::/28
  (ORCHID), 2001:20::/28 (ORCHIDv2), 3fff::/20 (documentation) and 5f00::/16 (segment routing). A refusal names the
  class, for example `destination address is not allowed (6to4 relay anycast)`.
- The IPv4-compatible and IPv4-translated forms are refused whatever IPv4 address they carry. IPv4-mapped, NAT64 and
  6to4 addresses are judged on the IPv4 address they carry.
- A URL whose host lookup returns an answer that is not a plain IP address, such as the zoned literal
  `http://[fe80::1%25lo0]/`, is refused when it is checked, with the class `unparseable address`, the class the
  connection check gives such an address.
- A test passes 43 addresses the policy refuses and 7 public addresses through the URL check as IP literals, and
  fails when one of the first is admitted or one of the second is refused.

### Security — every message the SMTP mail service sends asks the relay not to track it

- The SMTP mail service, which `EMAIL_PROVIDER=smtp` selects, writes one `X-SMTPAPI` header on every message it
  sends, the password reset mail included. Its value is a fixed JSON constant, `trackingOffSMTPAPIHeader` in
  `apps/backend/internal/infrastructure/email/smtp_email_service.go`, that asks a SendGrid relay to turn off click
  tracking for HTML and plain-text links (`clicktrack`: `enable` 0, `enable_text` false), open tracking
  (`opentrack`: `enable` 0) and Google Analytics link parameters (`ganalytics`: `enable` 0). The header is written
  whatever the SMTP host, template or environment; relays other than SendGrid ignore it.
- Tests: `apps/backend/internal/infrastructure/email/smtp_tracking_test.go` builds the welcome, user-approved,
  password reset and verification reminder mails through the service's template renderer, and a raw HTML and a raw
  plain-text body, and fails unless each message carries exactly one `X-SMTPAPI` header whose value parses as JSON,
  turns all three filters off and keeps the header line under SendGrid's 1,000-character limit. It also fails when
  a rendered template contains `<img` or `url(`, either of which can act as an open-tracking image.

### Changed — the key vault says whether the server holds the agent's private key, and its Algorithm row shows only a stored algorithm

- `GET /api/v1/agents/{id}/key-vault` returns `serverHoldsPrivateKey`: `true` when the server stores an
  encrypted private key for the agent, `false` when it stores none. The response carries this fact only; the
  key itself, encrypted or not, is never in it.
- On the agent page's Security tab, the Algorithm row of the agent public key shows the algorithm the server
  stored for the key, and `Not recorded` when none is stored. Before, the row read `Ed25519` when no algorithm
  was stored.
- A test fails when the key vault response leaves out `serverHoldsPrivateKey` or carries `encryptedPrivateKey`,
  and when the Algorithm row shows `Ed25519` for a key with no stored algorithm.

### Added — `GET /health/ready` can report the commit a hosted build was synced from

- The readiness body may carry an optional `syncedFrom` key: the agent-identity-management commit, as a full
  lowercase 40-hex sha, that the latest merged sync on the deploying branch was pulled from. Protected paths
  may lag that commit; the sync pull request's "Not carried" report is the record of what a sync did not
  carry. A build sets it with `-X main.buildSyncedFrom=<40-hex>`; a build that sets nothing, or a value that
  is not a full lowercase sha, leaves the key out of the body. The image this repository publishes does not
  set it.
- `apps/backend/contracts/health-ready.schema.json` declares `syncedFrom` as an optional string with the
  pattern `^[0-9a-f]{40}$`.

### Changed — the agent page detects MCP servers from a More menu, with the command to run where the agent runs

- The agent page's header has a `More` menu after `Edit`. Its one entry, `Detect MCP servers`, opens a
  dialog with the commands to run on the machine where the agent runs: `aim-sdk login` (with `--url` and the
  dashboard's address), then `python -c "from aim_sdk import secure; secure('<agent name>',
  auto_detect_mcp=True)"` with the agent's name filled in. The SDK reads the MCP servers from that machine's
  Claude Desktop configuration and registers them to the agent. The dialog asks for no file path.
- While the dialog is open it reads the agent every 5 seconds. When the agent gains MCP servers, the dialog
  names them in place of `Waiting for the command to report.`, and closing it reloads the page. A failed read
  leaves the dialog waiting and shows no text from the response.
- The dashboard's API client no longer has `detectAndMapMCPServers`, and the dashboard no longer carries the
  `AutoDetectButton` and `DetectionStatus` components. The server's `POST /api/v1/agents/:id/mcp-servers/detect`
  route is unchanged.

### Removed — the agent Security tab's "Future Use Cases" and "Key Management Best Practices" blocks

- On an agent's Security tab, the section below the violations now starts with the "Agent Public Key" card and ends
  with the "Post-Quantum Companion Key" card. The purple "Future Use Cases" block above them and the "Key Management
  Best Practices" block below them are removed, and no text replaces them.
- The first block listed request signing as a future use; the Python, TypeScript and Java SDKs already sign requests
  and send the signature in the `X-AIM-Signature` header. The second said the agent's private key exists only on the
  agent and is never transmitted or stored elsewhere. The dashboard registers an agent without a public key, and for
  such an agent the server generates the key pair and stores the private key encrypted
  (`apps/backend/internal/application/agent_service.go`).
- `apps/web/components/agent/key-vault-tab.test.tsx` renders the tab with a mocked key vault and fails when either
  heading appears or when the "Agent Public Key" card no longer copies its key. It fails on the previous commit.

### Security — a revoked session's access token is refused on the routes behind the session middleware

- After a session is revoked (a reused refresh token, logout, or a dashboard revoke of an SDK token), the session
  middleware answers its access token with the 401 a token revoked by its own id gets (`Token has been revoked`):
  at once on the instance that recorded the revocation, and within 30 seconds on another instance. A route behind
  the optional session middleware serves such a request as anonymous. Before this change the middleware read the
  token's own id only: in the backend's middleware test, a sign-in's access token whose session had been revoked
  was answered 200 on `GET /api/v1/agents`, and so was the access token the SDK refresh had minted for a revoked
  SDK download. An access token minted before sessions carried a family (`sid`) is checked by its own id only.
- On a cache miss the middleware reads the family key (`revoked:fam:<family>`) after the token's own key, so a
  token with a family costs one more revocation store read per miss and none while its answer is cached (30
  seconds). A store error on the family read is answered as one on the token's own key: the token is refused, or
  served when `AUTH_REVOCATION_FAIL_OPEN=true`.
- A family's revocation key now lasts at least `JWT_ACCESS_TTL` plus one minute; the refresh lifetime and the
  token's own lifetime still set it when longer. With `JWT_REFRESH_TTL=1h` and `JWT_ACCESS_TTL=5h` the key was
  written for 61 minutes, so it could lapse while the family's access tokens were still valid.
- The three credential-minting routes keep their uncached family read and their `credential_mint_refused` record.
  A request the session middleware refuses does not reach them and is not recorded: in the backend's handler test,
  a download made right after a dashboard revoke on the same instance is refused by the middleware, and no
  `credential_mint_refused` record is written.

### Fixed — revoking an SDK token in the dashboard ends its token family, so the family's access token is refused at once

- `POST /api/v1/users/me/sdk-tokens/{id}/revoke` and `POST /api/v1/users/me/sdk-tokens/revoke-all` now write the
  family key (`revoked:fam:<family>`) for each token family among the rows they revoke, as the refresh route does
  after a reuse and logout does for a sign-in. Before this change the two routes revoked the rows only: in the
  backend's handler test, a download made with the access token the SDK refresh had minted for the family was
  answered 200 right after the dashboard revoke, and no family key was written. Now that download is answered
  401, and so is a refresh with the family's live refresh token.
- A row's family is read from its rotation lineage: each rotated row names the row it replaced in its
  `parent_token` metadata, and the download row at the top holds the family's id as its token ID. The key lasts for
  the longest of `JWT_REFRESH_TTL`, `JWT_ACCESS_TTL` and the row's own lifetime, plus one minute, the rule the
  refresh route's family write uses. A row whose lineage cannot be followed to its download row, or a key the revocation store refuses,
  is logged with identifiers only; the rows are revoked and the route answers as before. With no revocation store
  configured the routes revoke the rows and answer 200 as before.
- `SDKTokenHandler` now takes the JWT service as a second constructor argument.

### Changed — the dashboard sets its type from one six-step ramp, 12px for captions and 14px upward for reading text

- `apps/web/tailwind.config.js` replaces Tailwind's font-size scale with six steps, each a size over a line
  height: `text-xs` 12/16, `text-sm` 14/20, `text-base` 16/22, `text-xl` 20/26, `text-2xl` 24/32 and
  `text-3xl` 32/40. The 10.5px `text-2xs` step is removed, and Tailwind's `text-lg`, `text-4xl` and larger
  steps are not generated: the built stylesheet carries those six size rules and no rule for `text-lg`,
  `text-2xs` or `text-4xl`.
- In the dashboard source every `text-2xs` is now `text-xs`, every `text-lg` is `text-xl`, every `text-4xl`
  is `text-3xl`, and every arbitrary pixel size outside the MCP network graph is a step of the ramp: 9px and
  10px became 12px; 13px, 13.5px and 14px became 14px; 15px, 16px and 17px became 16px; 20px and 22px became
  20px; 26px and 28px became 24px; 30px and 32px became 32px. The `text-overline` and `glass-segment-item`
  classes in `globals.css` are 12px; before they were 10.5px and 11.5px.
- A test fails on a `text-2xs`, `text-lg`, `text-4xl` or larger, or arbitrary-length `text-[...]` class in
  `app/`, `components/`, `lib/` or `hooks/`; it proves itself on a seeded file first, holds the readability
  and target-size helpers to the config's six steps, and lists the network graph as the one file still
  carrying such classes until the graph has a sizing rule of its own. That list can only shrink: the test
  also fails when a listed file carries none. On the commit before this one the test listed 174 sites.
- The target-size check skips a link or button that is `hidden` at 375px, where it has no target.

### Changed — three dashboard sentences that held an em dash are punctuated with a full stop, a colon or commas

- The overview's live check-in panel, while nothing has checked in, reads `Run the install commands below.
  The moment your agent calls in, it appears here with its identity and trust score, and this page fills in
  on its own.`
- The agent page's drift score card describes itself as `Behavioral drift signal: runtime vs registered
  baseline (0% clean, 100% maximum drift)`.
- The trust score breakdown's execution isolation factor says the self-reported posture `is capped at 0.65,
  the score of an ordinary hardened container, no matter how strong a posture is claimed`.
- Every word stays; only the punctuation changed. The dash the drift score card prints in place of a value it
  has no number for is unchanged. `apps/web/tests/sentences-carry-no-em-dash.test.ts` reads the three
  components and fails on an em dash in any other line.
- `apps/web/tests/e2e/empty-state-sentences.spec.ts` checks the three sentences as Chromium renders them, at
  375 and 1280 px wide: the overview with the agent list and count answered as empty, and a newly registered
  agent's trust section with the execution isolation tooltip open. Each locator matches the whole sentence,
  so the sentence with an em dash in it does not match.

### Changed — the README and the use-case guides say `@opena2a/aim-core` runs locally, not against a server

- The README's SDK table gives `@opena2a/aim-core` the mode `Local`. Before, it read `Local or server`. The
  package keeps identities, policies and the audit log on the machine it runs on; its one server path, the
  reporter its `serverUrl` option starts, posts to `/v1/audit/events`, and the server registers no route
  under `/v1/`.
- The fleet governance guide's SDK approach no longer shows a TypeScript example that passes `serverUrl` to
  `AIMCore` and says the identities are registered on the server, the events sent to it and the trust score
  computed there. It says `@opena2a/aim-core` runs in local-only mode and names `@opena2a/aim-sdk` as the
  TypeScript client for an AIM Server, linking the TypeScript SDK README. The sentence on which parameters
  route identities and events through the server names the Python and Java parameters only.
- The audit guide's TypeScript example says its events stay on the machine, separate from the server's audit
  log. Before, it said they are also sent to the central database when `serverUrl` is set.
- The embedding guide's `AIMCore` constructor table no longer lists `serverUrl` as an AIM server URL for fleet
  reporting.
- A test fails when one of these three guides names `serverUrl` or one of these server claims again, or when
  the SDK table gives `@opena2a/aim-core` another mode.

### Changed — the README and the server features guide describe only the integrations the platform has

- The README's deployment section says that local-only mode keeps its own event format in
  `~/.opena2a/aim-core/audit.jsonl`, separate from the server's audit log. Before, it said all three modes
  share one audit-event schema and that local agents push history to a server through
  `AIMCore.enableReporting()`; the server registers no route that call could reach.
- The security-alerts row of the server trust-score table in `docs/server-features.md` names its source as
  `Observed`: the factor is scored from the agent's unacknowledged alerts. Before, the row read
  `NanoMind-modulated`; the calculator does read a threat-model-evaluation score when one is stored, but
  nothing in this repository writes one, so the modulation never applies.
- The sentence after that table, "Both can run for the same agent when local-to-server reporting is
  enabled.", is removed.
- A test reads the README and `docs/server-features.md` and fails when either names one of these
  claims again, when the deployment section loses the local-only sentence, or when the security-alerts row
  names another source.

### Fixed — the Java SDK download's QUICKSTART.md names the version the download is named after

- `GET /api/v1/sdk/download?sdk=java` names the zip `aim-sdk-java-v<version>.zip` after `sdk/java/VERSION`. The
  `QUICKSTART.md` inside it now tells the reader to add `org.opena2a:aim-sdk` at that same version. The version is
  read on every download, so a server built from a tree with a new `sdk/java/VERSION` needs no other change.
- Two backend tests, `TestJavaSDKVersionFileStatesThePomVersion` and `TestJavaSDKVersionStatementsAgreeWithThePom`,
  fail when `sdk/java/VERSION`, the Java SDK's own version strings (its User-Agent, the `sdkVersion` it sends, its
  SBOM component) or a document that names `org.opena2a:aim-sdk` states a version other than the one
  `sdk/java/pom.xml` declares, and list each file and line that does. The backend CI job runs them for every change
  under `apps/backend/` or `sdk/`.

### Fixed — the pending user list and the approval of a registration request give lastLoginAt in UTC (#722)

- `GET /api/v1/admin/users/pending` answers with each user as the service read it, and
  `POST /api/v1/admin/registration-requests/:id/approve` with the user the approval created. Both pass
  `lastLoginAt` through the helper the public sign-in routes, `/api/v1/auth/me` and `/api/v1/admin/users` use,
  so the value is RFC 3339 with a `Z` suffix whatever location it carried. A user who has never signed in keeps
  `"lastLoginAt": null`.
- Measured with the two handlers of the previous commit, each handed by its service a user whose `lastLoginAt`
  carried a UTC-6 location: both answered `"lastLoginAt": "2026-10-11T06:25:44.184309-06:00"`.
- The comment on the helper (`apps/backend/internal/interfaces/http/handlers/utc_time.go`) said every response
  that names a `lastLoginAt` passes through it. It now says which do: every handler of the package that answers
  with the `lastLoginAt` of a user a service handed it.
- Tests: `TestLastLoginAt_PendingUserListAnswersInUTC` and `TestLastLoginAt_ApprovedRegistrationAnswersInUTC` in
  `apps/backend/internal/interfaces/http/handlers/last_login_at_utc_test.go`. The change-password row of that
  file builds its new password from the suite's test password: `scripts/lint-token-literals.mjs` reported the
  line, which the previous commit carried unchanged, and now reports 0 findings.

### Fixed — the deployment guide says a quick-start run over an existing install leaves its `.env` as it is (#722)

- The Docker section of `docs/DEPLOYMENT.md` said the script records the values it ran with in the install's `.env`. A first
  run does. A run over an existing install starts the stack with the variables set for that run and leaves that
  `.env` as it is, so a later `docker compose` in that directory reads the values `.env` holds; the section now
  says so, and says to change the value in that `.env` to keep it. The `quickstart.sh` row of
  `docs/testing/release-smoke.md` says the same. `scripts/quickstart.sh` is unchanged.
- Measured against docker and curl stubs, in `scripts/test-quickstart-orphan-volume.sh`: over an install whose
  `.env` held `AIM_BACKEND_PORT=18081`, a run with `AIM_BACKEND_PORT=18082` started Compose with `18082`, waited
  for `localhost:18082`, and left `.env` byte for byte as it was. With Compose 5.5.1, `docker-compose config` on
  the quick-start file beside a `.env` holding `AIM_BACKEND_PORT=18081` rendered the backend's host port as
  `18081` with no variable set, and as `18082` with `AIM_BACKEND_PORT=18082` in the environment.

### Fixed — the two quick-start rows of the release smoke contract are rows of its table (#722)

- In `docs/testing/release-smoke.md` a blank line stood between the component smoke matrix and its rows for
  `scripts/test-quickstart.sh` and `scripts/test-quickstart-orphan-volume.sh`. A blank line ends a Markdown
  table, so the line is removed and the two rows belong to the table.
- A test checks that no pipe row of the file follows a blank line unless a delimiter row follows it.

### Changed — the TypeScript SDK release smoke names its PostgreSQL image by digest (#722)

- `sdk/typescript/scripts/release-smoke.sh` started its throwaway PostgreSQL from `timescale/timescaledb:latest-pg16`,
  a tag that moves. It now names the image by the digest that tag had on 2026-10-13, a multi-platform index with
  `linux/amd64` and `linux/arm64`, so every run starts the same image; a comment beside it gives the command that
  reads the tag's current digest. The hermetic smoke passed with it: `db:read` allowed, `db:write` denied in
  strict mode.
- The two compose files and the two backend smokes still name the tag.
- Test: `sdk/typescript/tests/release-smoke-image-digest.test.ts`.

### Fixed — `GET /health/ready` and `GET /api/v1/status` report the bare semver the image tag carries

- A server built from the platform release tag reports its version as the image tag reads: a build stamped with
  `platform-v1.1.0` answers `"version": "1.1.0"` on both routes, so `curl -s localhost:8080/health/ready | jq -r .version`
  equals the image tag. A build stamped with a commit sha or with the Dockerfile's `dev` default reports that value
  unchanged; an unstamped build reports `null` on both routes. The rule lives in the server, not in the publish
  workflow, so a local `docker build --build-arg VERSION=platform-v1.1.0` reports the same `1.1.0`.
- Measured with the readiness handler of the previous commit, stamped with `platform-v1.1.0`: `GET /health/ready`
  answered `"version": "platform-v1.1.0"`, the tag name rather than the image tag. The `GET /api/v1/status` handler
  of the previous commit carried the literal `"version": "1.0.0"` in its source (`git grep` of `cmd/server/main.go`),
  whatever was built; it now reports the stamped build version through the same rule as the readiness route.
- Tests: `apps/backend/cmd/server/reported_version_test.go`.

### Changed — the Docker quick-start takes its host ports and container names from the environment

- `docker-compose.quickstart.yml` reads the host ports from `AIM_BACKEND_PORT` (default `8080`),
  `AIM_FRONTEND_PORT` (`3000`), `AIM_POSTGRES_PORT` (`5432`) and `AIM_REDIS_PORT` (`6379`), and the
  prefix of the four container names from `AIM_PROJECT` (`aim`, giving `aim-postgres`, `aim-redis`,
  `aim-backend` and `aim-frontend`). Rendered with none of them set, the file yields the same ports
  and names as before. The defaults of `FRONTEND_URL`, `ALLOWED_ORIGINS` and the dashboard's
  `NEXT_PUBLIC_API_URL` follow the dashboard and backend host ports; an explicit value still wins.
  PostgreSQL and Redis stay bound to `127.0.0.1`.
- `scripts/quickstart.sh` waits for the backend and prints the dashboard and API URLs at the
  configured ports, and a first run records `AIM_PROJECT` and the four ports in the install's `.env`, so a later
  `docker compose` in that directory keeps them. On an existing install the environment wins over
  `.env` for that run, as it does for Compose itself, and the script leaves that `.env` as it is. A `COMPOSE_PROJECT_NAME` given to a first run is recorded
  too. Two quick-starts run side by side on one host when each has its own directory (`AIM_DIR`),
  `AIM_PROJECT` and ports; the Docker section of `docs/DEPLOYMENT.md` shows the command.
- `scripts/test-quickstart.sh` works in a directory from `mktemp -d` under its own Compose project
  and container names, takes the host ports from the same variables, and prints the teardown
  command for the stack it started. It no longer uses a fixed directory under `/tmp`, so it never
  runs `down -v` on a directory another run created.
- `docs/testing/release-smoke.md` carries rows for `scripts/test-quickstart.sh` and
  `scripts/test-quickstart-orphan-volume.sh`, which now also checks the ports and the `.env`
  fallback against docker and curl stubs. A test renders the compose file with and without the
  variables and checks that the Docker section of `docs/DEPLOYMENT.md` and the smoke contract name
  each variable with its default.

### Fixed — sign-in responses give lastLoginAt in UTC, as /api/v1/auth/me does

- `POST /api/v1/public/login` and `POST /api/v1/public/change-password` answer with the sign-in instant in UTC,
  as `2026-10-11T12:25:44.426584Z`. Before, the instant was clocked with the server's local offset and
  serialised with it: with the server in a UTC-6 zone, a sign-in with a password that is accepted answered
  `"lastLoginAt": "2026-10-11T06:25:44.426584-06:00"` (measured in the handler tests and in the release test of
  the `platform-v1.1.0` candidate). In that release test, which read the value back from PostgreSQL in a UTC
  session, `GET /api/v1/auth/me` for the same session named the same instant as `2026-10-11T12:25:44.426584Z`.
- `GET /api/v1/auth/me`, `GET /api/v1/admin/users` and a `POST /api/v1/public/login` that answers "You must
  change your password before continuing" name the stored `lastLoginAt` in UTC whatever time zone the value was
  read with. Before, they answered with the value as read: in the handler tests, a stored value in a UTC-6
  location was answered as `2026-10-11T06:25:44.184309-06:00`.
- A new handler test signs in with a stored `lastLoginAt` in a non-UTC location at each of the three public
  routes and checks the response and `/api/v1/auth/me` both end in `Z` and name the same instant; a second one
  lists the organization's users and checks the same form.

### Fixed — the backend integration suite signs in within the strict rate limit of the public routes

- `apps/backend/tests/integration` signs in as admin once per run and reuses the token. Before, each test that
  needed the admin signed in on its own: one run made 99 `POST /api/v1/public/login` requests and 126 requests to
  `/api/v1/public/` in 20 seconds (counted in the backend's request log, against a backend built from this tree
  with the strict limit at 100 a minute), 26 of them answered 429, and 25 tests failed on that answer. A run now
  makes 34 login requests and 62 requests to `/api/v1/public/`.
- Every request the package sends to a route under `/api/v1/public/`, `/api/v1/auth/` or `/api/v1/oauth/`, the
  three groups the server limits with `StrictRateLimitMiddleware`, passes through a gate. The gate reads the
  limiter's `X-RateLimit-Remaining` and `X-RateLimit-Reset` headers from each response and holds the next request
  to that group until the window resets, instead of letting the server answer 429. A refusal from a window spent
  before the run began (a dashboard login from the same address, for example) is retried once after the
  `Retry-After` the server names. The time a request is held does not count against the caller's client timeout.
  Against the production limit of ten a minute a run takes about six minutes; against a backend started with
  `ENVIRONMENT=test` (one hundred a minute) it does not wait and takes under twenty seconds.
- `TestLoginEndpointRateLimiting` reads the limiter's headers from one login response instead of sending twenty
  login attempts, which spent the public group's window for every test that ran after it.
- Seven tests prove the gate against a local fixed-window limiter without a backend: eight requests at three a
  window all answered 200, a refusal from a spent window followed by one retry that carries the request body, a
  hold longer than the client's timeout, each group tracked on its own, other routes passed through unchanged.
- Two tests that read a field of a login or token response with an unchecked type assertion now fail with the
  response body instead of ending the whole run with a panic; both responses were hidden behind the 429 before.

### Fixed — `POST /api/v1/agents/:id/pqc-key` answers the hybrid mode it stored (#720)

- The `hybridModeEnabled` of the 200 answer is the request's `enableHybrid`, the value the handler stores with the key,
  in place of the agent's hybrid mode as loaded before the store.
- Measured on the handler of the previous commit, with the agent as loaded holding the opposite hybrid mode: a
  manager's or admin's request with `"enableHybrid": true` was answered `"hybridModeEnabled": false` and stored hybrid
  mode on, and a manager's or member's request with `"enableHybrid": false` was answered `"hybridModeEnabled": true`
  and stored hybrid mode off.
- Test: `TestRegisterPQCKeyAnswersTheHybridModeItStored` in
  `apps/backend/internal/interfaces/http/handlers/pqc_register_hybrid_answer_test.go`.

### Fixed — four more handlers answer 400 for a query value that is not UTF-8 text or holds a NUL character (#720)

- `GET /api/v1/a2a/skills/search` (`q`), `GET /api/v1/mcp-servers/by-name` (`name`), `GET /api/v1/admin/alerts`
  (`severity`, then `status`) and `GET /api/v1/admin/verifications/pending` (`status`, `risk`, `search`, then
  `search_field`) answer `400` with `{"error": "<parameter> must be UTF-8 text without a NUL character"}`, naming the
  first refused parameter in that order, and read nothing. `GET /api/v1/agents` and `GET /api/v1/tags/search` share
  the check. A value that is UTF-8 text, such as `%e2%82%ac`, is read as before, and the verification list still
  compares its four values in lower case.
- Tests: `apps/backend/internal/interfaces/http/handlers/free_text_query_values_test.go`.

### Changed — every Go file of the backend's HTTP handler package is gofmt-formatted (#720)

- `TestHandlerFilesAreGofmtFormatted` in `apps/backend/internal/interfaces/http/handlers/gofmt_test.go` reads every
  `.go` file of the package, tests included, in place of the three it named, and fails when that list does not hold
  its own file. The 25 files `gofmt -l` listed there are formatted, which changes their spacing, alignment and blank
  lines only.

### Changed — the dashboard's token refresh comment says the lease narrows two tabs' overlap and does not rule it out (#720)

- The comment on `withRefreshLock` in `apps/web/lib/api.ts` said it runs one refresh at a time across every tab. Under
  the `localStorage` lease, two tabs whose claims take longer than the lease's wait to reach each other can both
  refresh, as the entry below states; the comment now says so.

### Security — a revoked session's access token cannot create an API key, take or replace an agent's key, rotate a secret, or create or change a webhook

- `POST /api/v1/api-keys`, `POST /api/v1/agents/:id/rotate-credentials`, `GET /api/v1/agents/:id/credentials`,
  `GET /api/v1/agents/:id/sdk`, `PUT /api/v1/agents/:id/keys`, `POST /api/v1/secrets/namespaces/:id/rotate`,
  `POST /api/v1/webhooks` and `PUT /api/v1/webhooks/:id` now read the acting session's revocation from the revocation
  store (no cache) and refuse an access token whose session was revoked with the 401
  `{"error":"Token has been revoked or is invalid"}`; a refused request reaches none of the route's service or
  repository calls. These routes hand out an API key or an agent's private key, register an agent public key of the
  caller's choosing, rotate a namespace's stored secret, or set the address a webhook delivers the organization's
  events to.
- A confirmed refusal is recorded in the organization's audit log as `credential_mint_refused`, with the route's label
  in its metadata (`api_key_create`, `agent_rotate_credentials`, `agent_credentials`, `agent_sdk_download`,
  `agent_keys_update`, `secret_rotate`, `webhook_create` or `webhook_update`), and as one `SECURITY` log line,
  identifiers only. As on `GET /api/v1/sdk/download`, `POST /api/v1/oauth/device/approve` and
  `POST /api/v1/auth/sdk/recover`, when the revocation store does not answer the request is refused, or served with
  `AUTH_REVOCATION_FAIL_OPEN=true`, and no refusal is recorded; an access token that carries no `sid` is not checked.
- Measured with the handlers of the previous commit, sending each route an otherwise valid request with an access
  token whose session was revoked in the revocation store: `POST /api/v1/api-keys` and `POST /api/v1/webhooks` were
  answered 201 and the other six routes 200, the answers a live session got; each route created, returned or changed
  what it acts on, and no refusal was recorded. A live session is answered as before.
- This change does not add the check to other routes, among them `POST /api/v1/secrets/namespaces/:id/credentials`
  and `POST` and `PUT /api/v1/agents/:id/pqc-key`.
- Tests: `apps/backend/internal/interfaces/http/handlers/credential_routes_family_test.go`.

### Changed — turning on hybrid mode takes the manager or admin role

- `POST /api/v1/agents/:id/pqc-key` with `"enableHybrid": true` and `POST /api/v1/agents/:id/hybrid-mode` with
  `"enable": true` answer a member 403, `{"error":"Manager or admin access required to turn on hybrid mode"}`, and
  store nothing. The register route adds a `hint`: `Send enableHybrid false to register the key without hybrid mode`.
  Hybrid mode marks the agent as requiring both an Ed25519 and an ML-DSA signature on each request it signs, so
  turning it on changes what the agent must present to authenticate.
- Measured on the handlers of the previous commit, behind the member gate both routes are mounted behind, with a
  well-formed ML-DSA-65 key on the register route and an agent holding both keys on the hybrid mode route: a member's
  request to turn hybrid mode on was answered 200 on each route and stored hybrid mode on.
- Measured the same way on both commits and unchanged: a manager or admin turns hybrid mode on through either route,
  and a member registers a key with `"enableHybrid": false` and turns hybrid mode off with `"enable": false`.

### Fixed — two dashboard tabs share one token refresh in a browser without Web Locks

- A browser has no Web Locks API on a dashboard served over plain HTTP from a host other than `localhost`. There the
  dashboard's tabs now take the refresh turn through a lease in `localStorage`, under the key `token_refresh_lease`. A
  tab writes its claim, waits 50 ms for a claim another tab wrote at the same moment to arrive, and refreshes only if
  its own claim is still the stored one. The other tab waits, then reads the stored session again and uses the tokens
  the first tab obtained instead of posting its own. The same turn is taken when the browser's lock manager refuses the
  request. A browser with Web Locks keeps the Web Lock.
- The lease lasts 10 seconds and its holder renews it every 2.5 seconds while its refresh is under way, so a tab closed
  in the middle of a refresh holds the others back for at most 10 seconds.
- `localStorage` has no compare-and-set, so this narrows the collision and does not rule it out: two tabs whose claims
  take longer than those 50 ms to reach each other can still both refresh.
- Measured with the dashboard of the previous commit in headless Chromium 147 with `navigator.locks` removed, against
  the backend with a revocation store (Redis) and PostgreSQL 18.3, three runs: two tabs of one browser context whose
  access token the API refused together each posted the stored refresh token to `POST /api/v1/auth/refresh`. The API
  answered 200 to one and 401 to the other and revoked the sign-in's token family. Measured against that API directly:
  after one refresh token was presented twice, the refresh token the first presentation returned was answered 401
  `Token has been revoked or is invalid`. With this change the same two tabs sent one refresh in each of three runs,
  stayed on the page, and held the pair it returned.
- Tests: `apps/web/lib/storage-lease.test.ts`, the cases under "token refresh where the browser has no Web Locks" in
  `apps/web/lib/api.refresh-single-flight.test.ts`, and `apps/web/tests/e2e/token-refresh-tabs.spec.ts`, which opens two
  tabs against the real backend, with Web Locks and without, and runs with the Playwright suite. A backend without a
  revocation store hands the presented refresh token back unchanged; the browser test asserts one refresh either way.

### Fixed — `GET /api/v1/tags/search` answers 400 for a `q` or `category` that is not UTF-8 text or holds a NUL character (#718)

- The handler answers `400` with `{"error": "<parameter> must be UTF-8 text without a NUL character"}`, naming `q` or
  `category` (`q` when both are refused), and sends no statement to the database. PostgreSQL refuses such a value as a text
  parameter. A `q` that is valid UTF-8, such as `q=%e2%82%ac`, is still searched for. `GET /api/v1/agents` and the tag
  search share the check.
- Tests: `TestSearchTags_ValueThatIsNotUTF8TextIsABadRequest` and `TestSearchTags_TextValuesReachTheSearchStatement`
  in `apps/backend/internal/interfaces/http/handlers/tag_handler_search_text_test.go`.

### Fixed — the password file entries say the driver reads a file only its owner may read or write and nobody may execute (#718)

- The #716 and #709 entries below and the comment on the server's connection string in
  `apps/backend/cmd/server/main.go` give `chmod 0600` as an example of such a mode, in place of saying that the file
  needs mode `0600`: the #716 entry records `aim-server migrate` exiting 0 with the file at modes `0400` and `0610`.

### Changed — `apps/backend/internal/interfaces/http/handlers/agent_handler.go` is gofmt-formatted (#718)

- `TestHandlerFilesAreGofmtFormatted` in `apps/backend/internal/interfaces/http/handlers/gofmt_test.go` fails when
  that file, `tag_handler.go` or `query_text.go` is not.

### Fixed — `GET /api/v1/agents` answers 400 for a status or agent type that is not UTF-8 text or holds a NUL character (#716)

- The handler answers `400` with `{"error": "<parameter> must be UTF-8 text without a NUL character"}`, naming
  `status`, `agentType` or `agent_type`, and sends no statement to the database. PostgreSQL refuses such a value as a
  text parameter.
- Measured with the handler of the previous commit, wired to its agent service and repository over PostgreSQL 18.3:
  `status=%00`, `status=a%00b`, `status=%ff`, `status=%c3%28`, `status=%ff&limit=5`, `agentType=%ff` and
  `agent_type=%00` were each answered 500 `{"error":"Failed to fetch agents"}`. A value that is valid UTF-8, such as
  `agentType=%e2%82%ac`, is still a filter, and was answered 200 with no agents before and after.
- `docs/API.md` states the 400.
- Tests: `TestListAgents_FilterValueThatIsNotUTF8TextIsABadRequest` and
  `TestListAgents_FilterValueOutsideASCIIIsAPredicate` in
  `apps/backend/internal/interfaces/http/handlers/agent_handler_list_filter_test.go`.

### Fixed — `aim-breakglass` names `PGCLIENTENCODING` or `PGDATESTYLE` when the driver refuses it, as `aim-bootstrap` does (#716)

- The two commands share one check, `DriverEnvironmentRefusal` in `apps/backend/internal/config`. When the driver
  refuses the connection string only for one of the two variables, `aim-breakglass` exits 2 with `aim-breakglass: the
  driver refuses a setting it reads from the environment, not from the database connection string:` followed by
  `PGCLIENTENCODING must be unset or UTF8`, by `PGDATESTYLE must be unset or "ISO, MDY"`, or by both. Any other
  refused string still ends the run with `the database connection string is not one the driver accepts`.
- Measured with a binary built from the previous commit, with `DATABASE_URL` and again with the `POSTGRES_*`
  variables naming a PostgreSQL 18.3 server: `PGCLIENTENCODING=LATIN1`, `PGDATESTYLE=German` and the two together
  each ended `aim-breakglass chain status --organization <id>` with exit code 2 and
  `aim-breakglass: the database connection string is not one the driver accepts`.
- Test: `TestADriverVariableRefusedFromTheEnvironmentIsNamed` in `apps/backend/cmd/breakglass/main_test.go`.

### Fixed — the bootstrap guide says which modes of the password file the driver reads (#716)

- `docs/guides/BOOTSTRAP_GUIDE.md`, the #709 entry below and the comment on the server's connection string say that
  the driver (lib/pq 1.10.9) skips the password file, as if it were absent, when the group or others can read or
  write it, or when its owner or others can execute it, so it reads a file that only its owner may read or write and
  nobody may execute (`chmod 0600`, for example).
- Measured with `aim-server migrate` built from the previous commit against PostgreSQL 18.3 with `scram-sha-256`
  authentication, with `POSTGRES_PASSWORD`, `PGPASSWORD` and `PGPASSFILE` unset and the role's line in `.pgpass` in
  the home directory: at modes `0600`, `0400` and `0610` the command exited 0; at `0644`, `0640`, `0660`, `0604`,
  `0601` and `0700` it exited 1 with `pq: password authentication failed for user "aim"`. `aim-bootstrap --default
  --yes` exited 1 with that error at mode `0644` and 0 at mode `0600`.
- Tests: `TestEmptyPostgresPassword_ServerAndBootstrapSendThePasswordOfPgpassInTheHomeDirectory`, with `PGPASSFILE`
  unset and with it set and empty, and `TestEmptyPostgresPassword_PgpassModeDecidesWhetherTheDriverReadsIt`, in
  `apps/backend/cmd/server/database_conn_string_test.go`.

### Fixed — with `POSTGRES_PASSWORD` empty, the server takes the database password from `PGPASSWORD` or the password file, as `aim-bootstrap` does (#709)

- The server leaves an empty `POSTGRES_PASSWORD` out of its connection string, as the URL `aim-bootstrap` builds leaves
  it out. The driver (lib/pq 1.10.9) then takes the password from `PGPASSWORD` when that is set, otherwise from the
  line that matches the login in the password file `PGPASSFILE` names, or in `.pgpass` in the home directory when
  `PGPASSFILE` is unset or empty, and sends an empty password when none of these gives one. The driver skips a password
  file, as if it were absent, when the group or others can read or write it, or when its owner or others can execute
  it, so it reads a file that only its owner may read or write and nobody may execute (`chmod 0600 ~/.pgpass`, for
  example). `aim-server migrate` opens
  the database with the same string. A `POSTGRES_PASSWORD` that is set is still the password sent, whatever
  `PGPASSWORD` and the password file hold.
- The server wrote an empty password as `password=''`. With that setting the driver sent an empty password in each of
  three cases a recording server measured on the previous connection string: `PGPASSWORD` set, a password file line
  for the login, and both.
- Measured with binaries built from the previous commit against PostgreSQL 18.3 with `scram-sha-256` authentication,
  with `POSTGRES_HOST`, `POSTGRES_PORT`, `POSTGRES_USER` and `POSTGRES_DB` set and `POSTGRES_PASSWORD` unset: with
  `PGPASSWORD` holding the role's password, and again with `PGPASSFILE` naming a file that holds the role's line and
  no `PGPASSWORD`, `aim-server migrate` exited 1 with `pq: password authentication failed for user "aim"` and
  `aim-bootstrap --default --yes` exited 0. With the binaries of this change `aim-server migrate` exits 0 in both
  cases. With neither a `PGPASSWORD` nor a password file, both commands exit 1 with that error, before and after.
- `docs/guides/BOOTSTRAP_GUIDE.md` says where the password comes from when `POSTGRES_PASSWORD` is empty.
- Tests: `TestEmptyPostgresPassword_ServerAndBootstrapSendTheDriversPassword`,
  `TestSetPostgresPassword_ComesBeforeTheDriversPassword` and
  `TestDatabaseConnString_QuotesAndEscapesEveryStringSetting` in
  `apps/backend/cmd/server/database_conn_string_test.go`. The first opens the server's string and the URL of
  `aim-bootstrap` against a server that records the login, and holds the two to the same password.

### Fixed — `aim-bootstrap` names `PGCLIENTENCODING` or `PGDATESTYLE` when the driver refuses it, and no longer reports a refused connection string for it (#709)

- The driver reads both variables from the environment. It accepts `UTF8` for the first and `ISO, MDY` for the
  second, and refuses `LATIN1` and `German`. When the driver refuses a variable's value and the connection string
  does not set the same setting, `aim-bootstrap` ends
  with `the driver refuses a setting it reads from the environment, not from the database connection string:`
  followed by `PGCLIENTENCODING must be unset or UTF8`, by `PGDATESTYLE must be unset or "ISO, MDY"`, or by both. It
  names the variable and not its value.
- To tell the two causes apart it parses the string again with the accepted settings written before it. A string the
  driver still refuses, one that sets `client_encoding=LATIN1` itself or one the driver cannot parse, ends the run
  with `the database connection string is not one the driver accepts`, as before.
- Measured with the `aim-bootstrap` of the previous commit and `POSTGRES_*` variables the driver accepts:
  `PGCLIENTENCODING=LATIN1`, `PGDATESTYLE=German`, and the two together each ended `aim-bootstrap --default --yes`
  with exit code 1 and `the database connection string is not one the driver accepts`.
- Not changed: a variable the driver does not support stops both commands with a Go panic. Measured before and after
  this change with `PGSERVICE=x` and with `PGHOSTADDR=127.0.0.1`: `aim-bootstrap --default --yes` and
  `aim-server migrate` exit 2 with `panic: setting PGSERVICE not supported` and
  `panic: setting PGHOSTADDR not supported`.
- `docs/guides/BOOTSTRAP_GUIDE.md` quotes the new line.
- Tests: `TestOpenDatabase_NamesTheEnvironmentVariableTheDriverRefuses` and
  `TestMainNamesTheEnvironmentVariableTheDriverRefuses` in `apps/backend/cmd/bootstrap/main_test.go`.

### Fixed — `GET /api/v1/agents` applies the `status` and `agentType` filters (#709)

- The list keeps the agents whose status equals `status` and whose type equals `agentType`. The type is read from
  `agent_type`, the name the Python SDK sends, when `agentType` is absent or empty. A value no agent holds returns
  no agents, and an empty value does not filter. `total` counts the agents that pass the filters, the same on every
  page. Each filter is an equality predicate of the list query and of its count, with the value sent as a parameter.
- Measured on a server built from the previous commit, as the administrator of an organization with three agents
  (two `verified` and one `suspended`; two `langchain` and one `crewai`):
  `GET /api/v1/agents?limit=100&offset=0&status=no-such-status&agent_type=no-such-type`, `?status=suspended`,
  `?agentType=langchain` and `?agent_type=crewai&limit=10` were each answered 200 with all three agents and a
  `total` of 3. A server built from this change answers them 200 with 0, 1, 2 and 1 agents, and a `total` equal to
  that count.
- The Java SDK's `listAgents` sends `status` and `agentType`, and the Python SDK's `list_agents` sends `status` and
  `agent_type`.
- `docs/API.md` and `docs/guides/API_REFERENCE.md` list the two parameters and say what `total` counts.
- Tests: `apps/backend/internal/interfaces/http/handlers/agent_handler_list_filter_test.go`, which runs the handler,
  the agent service and the agent repository over a connection that answers only the statement each case expects,
  and `apps/backend/internal/domain/agent_list_filter_test.go`.

### Fixed — the compose `bootstrap` service hands a database password with a single quote or a backslash to PostgreSQL unchanged (#701)

- Without `--database-url`, `aim-bootstrap` reads the database the way the server does: from `POSTGRES_HOST`,
  `POSTGRES_PORT`, `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB` and `POSTGRES_SSL_MODE` when `POSTGRES_HOST` is
  set, as a `postgres://` URL with the user and the password percent-encoded, and from `DATABASE_URL` otherwise. That
  is the order `aim-breakglass` reads them in.
- A `POSTGRES_HOST` that begins with `/` names the directory of a Unix socket, for `aim-bootstrap` as for the server.
  A URL's host cannot hold a slash, so the URL then carries the directory and the port as its `host` and `port`
  parameters.
- A connection string the driver does not accept ends the run with
  `the database connection string is not one the driver accepts`, followed by the places the string is taken from.
  `aim-bootstrap` parses the string before it uses it and does not print the driver's parse error. Measured with
  lib/pq 1.10.9 on a URL whose host holds a space: that error is `parse "<the URL>": invalid URL escape "%20"`, with
  the password in the URL it quotes.
- The `bootstrap` service in `docker-compose.yml` sets the backend's `POSTGRES_*` variables. Its `DATABASE_URL`, which
  holds the password between single quotes, stays for the `aim-bootstrap` of `platform-v1.0.0`, which reads only
  `--database-url` and `DATABASE_URL`; this `aim-bootstrap` does not read it while `POSTGRES_HOST` is set.
- Measured with the connection string parser of lib/pq 1.10.9 on the service's previous `DATABASE_URL`: the password
  `it's` was refused with `missing "=" after "s'" in connection info string`, and `ends-with\` with
  `unterminated quoted string literal in connection string`.
- `apps/backend/deployments/smoke/rest-list-endpoints-smoke.sh` and `apps/backend/deployments/otel-demo/smoke-backend.sh`
  give the `aim-bootstrap` they build the same `POSTGRES_*` variables as the backend they start, so a `.env` in
  `apps/backend` does not redirect it.
- Tests: `TestCompose_BootstrapServicePasswordReachesTheDriverUnchanged` in `apps/backend/cmd/bootstrap/compose_test.go`
  (it fails on the previous `docker-compose.yml` for `it's`, `back\slash` and `ends-with\`),
  `TestDatabaseURL_FlagThenServerVariablesThenDatabaseURL`, `TestDatabaseURL_ErrorsNameVariablesNotValues`,
  `TestOpenDatabase_RefusedConnectionStringIsNotQuoted`, `TestMainDialsTheUnixSocketInAPostgresHostDirectory` and
  `TestMainReportsARefusedConnectionStringWithoutQuotingIt` in `apps/backend/cmd/bootstrap/main_test.go`, and
  `apps/backend/internal/config/database_url_test.go`.

### Removed — the unused `internal/infrastructure/database` package (#701)

- No package imported it, and its `Connect` wrote the password into a key=value connection string with no quoting.
  `TestOnlyDatabaseConnStringBuildsAKeyValueConnectionString` in
  `apps/backend/cmd/server/database_conn_string_builders_test.go` holds the backend to one builder of that string,
  `databaseConnString`, which quotes and escapes every value.

### Fixed — the token revocation startup line names a Redis that could not be reached (#701)

- With no cache service, the line gives the reason the cache line gives:
  `Token revocation disabled (no Redis configured)` only when `REDIS_HOST` is set to the empty string,
  `Token revocation disabled (Redis unavailable)` when the startup Redis ping failed, whether `REDIS_HOST` names a host
  or is unset, and `Token revocation disabled (cache initialization failed)` when the cache could not be built on the
  connection. Each goes on with `: refresh tokens are not rotated and logout cannot revoke`.
- Tests: `TestRevocationStartupLine_GivesTheCacheLinesReason` and
  `TestTokenRotationGuideQuotesTheRevocationStartupLines` in `apps/backend/cmd/server/redis_startup_test.go`.

### Fixed — the token rotation guide says which refreshes rotate, and the troubleshooting guide quotes refresh lines the backend writes (#701)

- `docs/security/token-rotation.md` says in its opening, in "How often do tokens rotate?" and in "Can I disable token
  rotation?" that a token from an SDK download rotates on every refresh and a sign-in from `aim-sdk login` only on a
  server with Redis. Its configuration section names the environment variables the backend reads (`REDIS_HOST`,
  `AUTH_REVOCATION_FAIL_OPEN` and `JWT_ACCESS_TTL`) and quotes the startup lines that say which applies, in place of a
  block from `apps/backend/config/security.yaml`, a file the repository does not have.
- `docs/troubleshooting/authentication.md` ("Monitor Token Refresh Attempts") points at the access log line of
  `POST /api/v1/auth/refresh` and the refresh route's `Refresh:` warnings, and pipes `docker logs` with `2>&1`, since
  the backend writes its warnings to standard error. It quoted `Token refresh successful` and
  `Token rotated: old_token → new_token`, which nothing in the backend or the SDKs writes.
- Tests: `TestTokenRotationGuideSaysWhichRefreshesRotate` and `TestTroubleshootingGuideQuotesRefreshLinesTheBackendWrites`
  in `apps/backend/internal/interfaces/http/handlers/security_guides_token_docs_test.go`.

### Fixed — the REST list-endpoints smoke runs no `docker exec` (#701)

- `apps/backend/deployments/smoke/rest-list-endpoints-smoke.sh` reaches its throwaway Postgres with `psql` and
  `pg_isready` on the published port, as `otel-demo/smoke-backend.sh` does, and stops before running any command when
  `SMOKE_PG_CONTAINER` holds a character outside `[A-Za-z0-9_.-]` or `SMOKE_PG_DB` one outside `[A-Za-z0-9_]`. Its
  pre-flight checks for `psql` and `pg_isready` too.
- On a copy of the tree without the `.hmaignore` rule that exempts the script from DOCKERINJ-001,
  `hackmyagent@0.33.2 secure . --ci --static-only` reported `Docker exec with variable interpolation` (HIGH) at line 94
  of the script and scored the tree 69/100 before this change, and reports no finding and 100/100 after it. With the
  rule, the same scan reported no finding and 100/100 before this change too.
- Test: `apps/backend/deployments/smoke/smoke_script_test.go`.

### Fixed — the phone navigation drawer is a modal dialog, and the tab bar's Secure link covers its raised icon

- The navigation drawer that the bottom tab bar's Menu button opens is now a modal dialog named "Navigation"
  (`role="dialog"`, `aria-modal="true"`). While it is open, the page and the tab bar behind its backdrop are `inert`,
  so their links and buttons leave the keyboard order and the accessibility tree. Focus moves into the drawer, Escape
  closes it, and closing returns focus to the control that opened it. The drawer closes when the window widens to the
  desktop layout, where it is not drawn, and a closed drawer is `inert`.
- The raised icon of the tab bar's "Secure" link had a `-mt-3.5` offset of its own, which drew it 14px above the
  link's box. The link now carries that offset instead, so its box covers the icon it draws.
- `components/sidebar.test.tsx` and `components/overview/security-lens.test.tsx` render the active navigation link
  with its icon and the security tiles' units, read the theme colour each is drawn in, and hold it to 4.5:1 in both
  themes on the surfaces `tests/text-contrast.test.ts` measures the tokens on (`tests/text-colour.ts` now holds those
  surfaces for both). `tests/text-contrast.test.ts` and `tests/page-standard.test.ts` passed with the active link in
  the brand ink or the units in the tertiary ink; the new tests fail on either, at 4.30:1 and 3.94:1 in the dark theme
  on those surfaces. `app/dashboard/layout.drawer.test.tsx` covers the modal drawer with the real sidebar and tab bar,
  and `components/mobile-tab-bar.test.tsx` fails when a part of a tab carries a negative margin, offset or translate
  class in any of its spellings: the `-` prefix (`-mt-3.5`), a bracketed negative value (`mt-[-14px]`,
  `translate-y-[-14px]`), or either of those behind a variant (`max-lg:-mt-3.5`) or the `!` modifier. It reads class
  names, not drawn boxes. `tests/e2e/mobile-tab-bar.spec.ts` measures the boxes in a browser at each phone and tablet
  viewport it covers, 375x812 among them: every part of each tab lies inside that tab's own link or button.

### Changed — a full static scan passes, with each exemption recorded and reasoned in `.hmaignore`

- `npx hackmyagent@0.33.2 secure . --ci --format json` reported nine high-severity findings: AST-CRED-003 in eight Go
  test files whose literals are made up for the test, and DOCKERINJ-001 in
  `apps/backend/deployments/smoke/rest-list-endpoints-smoke.sh`, whose `docker exec` passes each variable quoted as
  one argument with no `sh -c`. `.hmaignore` names each of the nine files with its reason as a
  `<path>:<CHECK-ID> # <reason>` rule, and the same scan now reports no findings (score 100). Each line exempts its
  file from the one check it names; every other check still reads the file.
- The header of `.hmaignore` also documents the `expires:YYYY-MM-DD` attribute a rule can end in. The rule applies
  through that UTC day; from the next day the scan lists the line under `hmaignore.errors`, stops applying it and
  reports its findings again.
- That scan's LOG-002 ("Sensitive Data in Logs") reads only the `.ts` and `.js` files directly in the scanned
  directory. The repository root has none, so the scan records LOG-002 as not applicable and reads no file of
  `apps/web`. `apps/web/tests/no-sensitive-log.test.ts` makes the same match, a call whose first argument begins with
  `password`, `apiKey`, `secret` or `token`, for `console.log`, `info`, `warn`, `error`, `debug` and `trace`, over every
  tracked TypeScript and JavaScript source file of the dashboard, and fails on any it finds.

### Fixed — a database password with a single quote or a backslash reaches PostgreSQL as configured (#697)

- The backend writes every string setting of its PostgreSQL connection string (host, user, password, database name and
  SSL mode) as a quoted value, with a backslash in front of each backslash and each single quote inside it. That is
  the escape lib/pq 1.10.9 reads. A doubled single quote ends a quoted value there.
- Measured before this change, with a test server that records the login it receives. For the passwords `it's`,
  `'quoted'`, `a''b` and `a\'b` the connection failed in the driver with `missing "=" after`, and for `ends-with\` with
  `unterminated quoted string literal in connection string`. `a'=b` reached the server as `a`, `back\slash` as
  `backslash` and `a\\b` as `a\b`. With the user `aim owner` and the database name `identity records` the connection
  failed in the driver with `missing "=" after`. The user `corp\aim` and the database name `corp\identity` reached the
  server without the backslash.
- Tests: `TestInitDatabase_PasswordReachesPostgresUnchanged`,
  `TestInitDatabase_UserAndDatabaseNameReachPostgresUnchanged` and
  `TestDatabaseConnString_QuotesAndEscapesEveryStringSetting` in
  `apps/backend/cmd/server/database_conn_string_test.go`.

### Fixed — the startup summary names the Redis address that could not be reached (#697)

- With no Redis connection, the Redis line of the backend's startup summary reads
  `Redis: disabled (running without caching)` only when `REDIS_HOST` is set to the empty string. When the startup
  Redis ping failed, whether `REDIS_HOST` names a host or is unset, the line reads
  `Redis: <host>:<port> (unavailable, running without caching)`, with the address that was tried.
- Tests: `TestRedisStartupSummary_EmptyRedisHostIsDisabled`, `TestRedisStartupSummary_FailedRedisPingIsUnavailable`
  and `TestRedisStartupSummary_ConnectionIsConnected` in `apps/backend/cmd/server/redis_startup_test.go`.

### Fixed — the token rotation guide quotes the line the Python SDK prints after a rotation, and says when it prints (#697)

- `docs/security/token-rotation.md` answers "How do I know if my token has been rotated?" with the line
  `sdk/python/aim_sdk/oauth.py` prints, `Token rotated successfully — old refresh token revoked (new id: ...)`. The SDK
  prints it only when the server answers a refresh with a new refresh token, and the guide says which server does: a
  sign-in from `aim-sdk login` is rotated only by a server with Redis, and a server without Redis, or one whose Redis
  refused the write, answers with the refresh token it was given and `"rotated": false`. A token from an SDK download
  is rotated in the database and does not need Redis.
- Test: `TestTokenRotationGuideQuotesTheRotationLineTheSDKPrints` in
  `apps/backend/internal/interfaces/http/handlers/security_guides_token_docs_test.go` reads the line from `oauth.py`,
  refreshes a login token against the refresh handler with a revocation store, without one and with one that refuses
  the write, and refreshes an SDK download token without one.

### Changed — the SDK route census reads the server's own route table and every module of the Python SDK (#697)

- `TestSDKLoginRoutesContract` in `apps/backend/cmd/server/sdk_login_routes_contract_test.go` takes its routes from the
  table the server mounts, so a route registered through `registerSDKAPIRoutes`, `registerBootstrapTokenRoutes` or
  `registerOnboardingTelemetryRoutes` counts like any other. It reads its paths from every Python file under
  `sdk/python/aim_sdk`: an f-string that opens with the server URL, and an endpoint literal that opens with `/api/`.
- The census found six paths in the SDK that this server does not register, and lists them by module so that a seventh
  fails it: `/api/v1/a2a/tasks/{id}`, `/api/v1/a2a/security/settings/{id}` and `/api/v1/a2a/security/violations/{id}`
  in `a2a.py`, where the server registers `/api/v1/a2a/tasks`, `/api/v1/a2a/security/settings` and
  `/api/v1/a2a/security/violations`; `/api/v1/agents/{id}/policies` in `auto_hooks.py`; the dashboard link
  `/dashboard/admin/verifications` in `client.py`; and the Registry path `/api/v1/telemetry/runtime` in
  `telemetry/relay.py`.
- The cells in `sdk/typescript/tests/docker-publish-release-only.test.ts`, `dockerfile-frontend-base-pins.test.ts`,
  `license-compliance-workflow.test.ts` and `no-cloud-push-workflow.test.ts` are named after their file
  (`DOCKER-PUBLISH-RELEASE-ONLY.AC<n>`, `DOCKERFILE-FRONTEND-BASE-PINS.AC<n>`, `LICENSE-COMPLIANCE-WORKFLOW.AC<n>` and
  `NO-CLOUD-PUSH-WORKFLOW.AC<n>`), in place of a tracker identifier that resolves to nothing in this repository.

### Fixed — the A2A, supply chain and agent detail pages are readable in both themes and fit a phone screen

- On a 375px screen the A2A page scrolled sideways by 25px and the supply chain page by 27px: the header's buttons
  stayed beside the title. Below 640px they now sit under the title and wrap.
- On the supply chain page the "Active connections" figure read 2.05:1 in the light theme and now uses the brand ink;
  the "Bill of Materials" tab label read 4.21:1 and now uses the neutral badge of the tab beside it; the eye button in
  each MCP server row was 16x16 with no accessible name and is now 32x32 and named for its server.
- On the agent detail page the trust score badges in the activity timeline sat on tinted event cards and read 3.58:1 in
  the dark theme and 4.21:1 in the light theme; the event cards are now outlined rather than filled. The trust score
  history table headers read 4.20:1 in the dark theme and now use the secondary ink. The "About drift score" button
  (16x16) and the info button beside each of the nine trust factors (14x14, with no accessible name) are now 24x24, and
  each factor's is named for its factor. The drift score card takes every colour from the theme tokens.
- The readability sweep's populated fixture (`apps/web/scripts/ux-sweep.mjs --fixture populated`) now fills the A2A,
  supply chain and agent trust score history charts; before, the A2A charts drew 0 marks, the supply chain charts 9 and
  the agent detail page no chart. Measured over these three
  pages, both themes and both viewports: the gate went from exit code 1 to 0 and targets under 24x24 from 48 to 0 over
  the 12 variants; all eight chart entries pass together. The run is recorded in
  `docs/ux/chart-contrast-2026-10-11.md`.
- `tests/page-standard.test.ts` holds these pages to the same source rules as the overview, security and compliance
  pages. The printable supply chain report's stylesheet moves to `apps/web/lib/abom-print-style.ts`.

### Fixed — every link and button on the chart screens is at least 24px tall

- On the overview, the security page and the compliance page, eleven links and buttons were under 24px tall (16px to
  21.6px where the browser measured them), under the 24x24 minimum target of WCAG 2.5.8: the "See all" link and the SDK language tabs in the developer view,
  "Open Security" and "Open compliance" in the security and executive views, the security page's "View all",
  "Review agent" and "Investigate" links, and the compliance page's "View all alerts" and "Reset page" buttons. Each
  now has a minimum height of 24px.
- The readability sweep (`apps/web/scripts/ux-sweep.mjs`) now fails its gate on any target under 24x24 (a link inside
  running text is exempt) and on a chart whose "view as table" control is under 44x44. Targets under 44x44 and text
  under 14px stay counted, not gated. Measured over the overview's three views, the security page and the compliance
  page, both themes and both viewports: targets under 24x24 went from 30 to 0 over the 20 variants, and the gate
  passes. The run is recorded in `docs/ux/chart-contrast-2026-10-11.md`.
- `tests/page-standard.test.ts` reads the height of every link and button on these screens from its classes and fails
  on one under 24px.

### Fixed — the active navigation link, the overview's agent initials and the security tiles' units are readable in both themes

- The active link in the navigation read at 4.38:1 in the dark theme in the phone drawer, and its colour held under
  4.5:1 over the light chrome under the page's cyan wash. It now has its own colour token, held to 4.5:1 over every
  surface the navigation sits on in both themes (4.93:1 dark, 5.21:1 light at its lowest).
- The agent initials in the overview's agent list were drawn on four hex gradients written into the page, and read as
  low as 4.14:1 in the light theme and 4.24:1 in the dark theme. The four tints are now theme tokens with the same
  gradients, and each initial holds 4.5:1 on both stops in both themes (4.87:1 at the lowest). The check-in panel's
  avatar uses the first tint.
- The unit beside each figure in the security view's policy tiles ("monitored", "verified", "incidents") read at 4.20:1
  in the dark theme; it is now the secondary text colour.
- Measured in a browser with the readability sweep over the overview's three views, the security page and the
  compliance page, both themes and both viewports: no text under 4.5:1 remains (21 texts were), and the sweep's gate
  passes. The run is recorded in `docs/ux/chart-contrast-2026-10-11.md`.
- `tests/text-contrast.test.ts` holds the active link, the four tints and the tile units to 4.5:1 from the theme
  tokens in both themes, and `tests/page-standard.test.ts` now also fails on a hex value or colour function on these
  screens.

### Fixed — the compliance page fits a phone screen and its text is readable in both themes

- On a 375px screen the compliance page scrolled sideways by 104px: each audit trail event is a growing flex box,
  and without `min-w-0` it stayed as wide as its whole description line, so a long URL was never cut with an
  ellipsis. The box can now shrink, and the page has no horizontal overflow at 375x812 or 1280x800.
- The "Trust:" label and the event status in the audit trail read at 2.43:1 in the light theme and 2.56:1 in the dark
  theme. Every card on the page is now a glass card and every colour comes from the theme tokens (ink colours for text;
  the success, warning, danger and brand tokens for status), as on the overview and the security page. Measured in a
  browser after the change, no text on the page was under 4.5:1 in either theme or viewport except the active sidebar
  link, part of the shared navigation, which then still read 4.38:1 in the dark theme at 375x812; the entry above on
  the active navigation link fixes it.
- In the audit trail and the security alerts list, high and medium severity now share the warning colour (they were
  orange and yellow); critical stays in the danger colour, and the badge names the severity in words.
- `tests/page-standard.test.ts` holds the overview's lenses, the security page and the compliance page to two rules:
  no Tailwind palette class, and no growing flex item around truncated text that cannot shrink.
- `docs/ux/chart-contrast-2026-10-11.md` gains the third run, before and after, over the compliance page.

### Fixed — the compliance page's charts are drawn on the glass card, so their axis text is readable in the dark theme

- The trust score trend and the risk distribution on the compliance page sat on a `bg-white dark:bg-gray-800` card,
  a surface the chart colours were never measured against: the trend's axis text read at 4.05:1 in the dark theme,
  under the 4.5:1 floor. Both cards are now glass cards, like the cards around every other chart, and their headings,
  legend and caption use the ink colours. Measured in a browser after the change, the page's chart text is 4.79:1
  dark and 5.48:1 light, and its lowest series mark 4.90:1 dark and 3.40:1 light. The page's other cards are
  unchanged.
- `tests/chart-surface.test.ts` reads where every chart in `app/` and `components/` is placed, following a component
  that draws a chart to each place it is rendered, and fails when the nearest painted element around a chart is
  anything but the glass card (`glass`, `bg-glass`, or the `Card` component) or when a chart has no card at all.
- `docs/ux/chart-contrast-2026-10-11.md` gains the second run, over the same five entries.

### Added — the readability sweep measures the charts it loads

- `apps/web/scripts/ux-sweep.mjs` measures every chart on a page in the browser: each painted shape whose fill or
  stroke is a series colour (`--chart-1` to `--chart-7`, `--chart-muted`) is held to 3:1 against the chart's
  background (WCAG 1.4.11), each text drawn inside the chart to 4.5:1 (3:1 large), and each chart must carry an
  `aria-label` and its "View as table" control. A failure of any of these fails the sweep's gate. Shapes painted in
  no `--chart-*` colour are counted and listed without failing it.
- The overview is loaded once per lens (developer, security, executive). `--only <route,route>` limits a run to the
  named routes and refuses a name that is no route. `--fixture populated` answers the endpoints the overview, security
  and compliance charts read with sample series, so those charts draw marks instead of their empty states; the
  default fixture is unchanged.
- `tests/ux-sweep.test.ts` covers the lens entries, `--only`, the populated fixture and the gate, and measures a
  chart page in Chromium where Playwright's Chromium is installed.
- `docs/ux/chart-contrast-2026-10-11.md` records the first run over the overview's three lenses, the security page
  and the compliance page in both themes: every series mark measured 3.40:1 or higher, and every chart carried its
  label and table control. The compliance page's trust score trend, then drawn on a `dark:bg-gray-800` card instead
  of a glass surface, had axis text at 4.05:1 in the dark theme; the entry above on the compliance page's charts
  fixes it.

### Changed — every chart offers its values as a table, and the light theme draws four series darker

- Every chart drawn by `apps/web/components/charts` has a "View as table" control above the plot that swaps the
  chart for a table of the same values: one row per slice, bar or point, one column per series, a donut's centre
  summary as a closing row, and the chart's title as the table's caption. A point that does not carry a series'
  value reads "not reported", never zero. The table reads without colour and is the keyboard path to the data;
  "View as chart" swaps back. `DonutChart` and `CategoryBarChart` take `columns` and `TrendChart` takes `xLabel` for
  the table's headings.
- The control is 44px tall and takes that height from the chart's box. The overview's verification activity
  (developer and executive views), the executive view's trust distribution, the compliance page's risk distribution
  and the security page's posture score are 44px taller so their plots keep their size; the other charts give the
  44px from the plot.
- In the light theme the green, amber, sky and orange series measured 2.04, 1.87, 1.97 and 2.58 to 1 against the
  glass card, under the 3:1 floor for graphical objects (WCAG 1.4.11). The light theme now draws them as `#248a3d`,
  `#b87400`, `#0891b2` and `#d0590a`, measured 4.05, 3.49, 3.39 and 3.78 to 1. The dark theme is unchanged. Donut
  slice labels are drawn in the axis text colour instead of the slice colour, so they are held to the text floor.
- `tests/chart-contrast.test.ts` reads the `--chart-*` tokens from `app/globals.css` in both themes and fails when a
  series colour falls under 3:1, or the axis or tooltip text under 4.5:1, against any surface a chart is drawn on:
  the glass card over each stop of the page gradient, bare or under either wash, and the solid glass fallback.

### Changed — the overview and security pages draw their charts through the chart layer

- The overview's verification activity (developer and executive views), the executive view's trust distribution
  and the security page's posture score were drawn by hand as SVG arcs and CSS bars. They now render through
  `apps/web/components/charts`, so they take their colours from the `--chart-*` tokens in both themes and carry the
  layer's tooltip and screen-reader label.
- Verification activity is one stacked bar per month, verified and pending drawn as separate series; before, a month
  was one bar of both together. The chart's label reads its window, `Verifications per month, last N months`, from
  the months the endpoint returned. The executive view shows a legend for the two series.
- The trust distribution's label reads every bucket and the average trust score. When the stats carry no average, the
  centre of the donut is left empty instead of showing a dash. "Not scored" is drawn in the muted series colour
  instead of the near-transparent track colour.
- The layer gains a score ring (`ScoreRing`), stacked bars and an axis-free mode in `TrendChart`, a centre summary in
  `DonutChart`, and a `--chart-track` token.
- `tests/chart-standard.test.ts` now also fails when a file under `app/` or `components/` outside `components/charts`
  draws a chart by hand: an SVG `<circle>` or `<ellipse>`, a dashed stroke, or an element marked `role="img"`.

### Fixed — the docs link lint reads every file, the SDK download carries no Go README, the documentation lints pass, and the migrate command prints words, not pictographs (#690)

- The setup README the server writes into a downloaded SDK has no Go variant. The repository carries no Go SDK, the
  download route serves only `python` and `java`, and the Go text imported `github.com/opena2a/aim-sdk-go/client`,
  a module under the opena2a account, where the project has no repository.
- `scripts/lint-docs-repo-links.sh` checks every text file git tracks and every untracked file git does not ignore,
  not only Markdown. A disabled Go test, the Python SDK's example and two of its tests named the opena2a account;
  they now name opena2a-org. Changelogs, the lint and its test are not checked, nor is the import prefix
  `scripts/sync-to-cloud.sh` writes into the Go files of the repository it syncs to, which has to equal that
  repository's module path.
- `scripts/check-public-wording.py` passes on the public documents. It reported 7 hits in 6 files: lines calling the
  audit trail immutable, tamper-proof, tamper-evident or append-only. They now say audit trail or audit log, the
  quick start says the decorator signs the verification request with the agent's Ed25519 key, and the documentation
  index drops an "Immutable logging" bullet.
- `docs/guides/QUICK_START.md` says under Integration Tests that a backend started with `ENVIRONMENT=test` and no
  `KEYVAULT_MASTER_KEY` runs on a key it generates at every start, so agent keys one run stores cannot be decrypted
  by the next, and to set the key when the test database is kept. `scripts/test-lint-docs-keyvault-master-key.sh`
  reported 1 failure on that section and now passes.
- The comment in `infrastructure/docker/Dockerfile.backend` runs `docker compose run --rm backend /app/aim-migrate`,
  naming the compose service rather than its container name `aim-backend`.
- `cmd/migrate` prints its status and error lines without pictographic marks, and the sample output in
  `docs/guides/BOOTSTRAP_GUIDE.md` matches.
- The A2A guide and `docs/sdk/trust-scoring.md` say that the TypeScript SDK's `verifyDelegationChain()` takes the
  trust attenuation factor and the trust floor from the verifier's options and not from a delegation, whose fields
  of those names its signature does not cover. The TypeScript SDK's changelog describes the change.
- Tests hold these. The generated setup README for every SDK type names no path under the opena2a account. The
  migrate command's source has no line with a character the Python SDK's console-output test bars, and its
  database error prints none. The test that checks documented docker commands against the compose files reads
  Dockerfiles as well as Markdown. A test in the TypeScript SDK's suite runs `check-public-wording.py` and the
  tests of both documentation lints, which include a run on the repository. The A2A guide test requires
  `verifyDelegationChain()` to read neither field from a delegation.

### Fixed — the startup cache line says Redis is disabled when `REDIS_HOST` is empty (#693)

- When `REDIS_HOST` is set to the empty string, the backend's startup line for the cache service is
  `Cache service skipped (Redis disabled)`. `Cache service skipped (Redis unavailable)` is logged when the startup
  Redis ping fails, whether `REDIS_HOST` names a host or is unset.
- Tests: `TestInitCache_EmptyRedisHostIsLoggedAsDisabled` and `TestInitCache_FailedRedisPingIsLoggedAsUnavailable` in
  `apps/backend/cmd/server/redis_startup_test.go`.

### Changed — the scratch-repository docs test passes under a global git excludes file (#693)

- The test in `apps/backend/cmd/server/provider_sign_in_docs_test.go` that builds a scratch repository stages its
  tracked fixtures with `git add -f`, so it passes when the developer's global git excludes file names one of them.
  `TestProviderSignInDocFilesWithAGlobalExcludesFile` builds the same repository under a global excludes file that
  names every tracked fixture.
- The cells in `scripts/test-pr-review-generated-files.mjs` for the generated-file placeholder are named
  `GENERATED-FILES.AC<n>`, in place of a tracker identifier that resolves to nothing in this repository.

### Fixed — an empty `REDIS_HOST` or `OTEL_EXPORTER_OTLP_ENDPOINT` turns that client off (#688)

- When `REDIS_HOST` is set to the empty string, the backend makes no Redis connection and logs
  `Redis disabled (REDIS_HOST is set and empty)`. When it is unset, the backend still tries `localhost` on `REDIS_PORT`.
- When `OTEL_EXPORTER_OTLP_ENDPOINT` is set to the empty string, the backend installs no trace, metric or log exporter,
  dials no collector, and logs `OpenTelemetry disabled`. When it is unset, the endpoint is still `localhost:4317`.
- Measured on one host with both variables set to the empty string and nothing listening on `localhost:6379` or
  `localhost:4317`: the backend built from the previous commit logged `Redis connection failed: dial tcp [::1]:6379`,
  took 5.08 s to exit after SIGTERM, and logged `OpenTelemetry shutdown error` for `dial tcp [::1]:4317`. This build
  exited in 0.08 s and logged neither.
- `apps/backend/deployments/otel-demo/smoke-backend.sh` and `apps/backend/deployments/smoke/rest-list-endpoints-smoke.sh`
  set `METRICS_LISTEN_ADDR` for the backend they start, to `127.0.0.1:$SMOKE_METRICS_PORT` (default 19464 and 19465),
  instead of leaving it on the default `127.0.0.1:9464`.
- Tests: `internal/telemetry/init_test.go`, `cmd/server/redis_startup_test.go`, `TestLoad_RedisHost` in
  `internal/config/config_test.go`.

### Changed — the backend integration tests read their address from `BACKEND_URL` (#688)

- Every test in `apps/backend/tests/integration/` takes the backend address from `BACKEND_URL`, default
  `http://localhost:8080`. `TEST_BASE_URL` is deprecated: it is read only when `BACKEND_URL` is unset, with a note on
  standard error. Before, the helpers built on `NewTestContext` read only `TEST_BASE_URL` and `getBaseURL` read only
  `BACKEND_URL`.
- `tests/README.md` names the variables, and `docs/testing/release-smoke.md` gives the suite's path as
  `apps/backend/tests/integration/` and its run command.
- The A2A progressive-load row of `docs/testing/release-smoke.md` starts the dashboard with
  `node .next/standalone/apps/web/server.js` after copying the static files beside it, as the frontend image does;
  `next start` printed that it does not work with the `output: 'standalone'` build.
- Tests: `tests/integration/base_url_test.go`.

### Changed — the backend test that reads the guides reads the tracked file list (#686)

- The test in `apps/backend/cmd/server/provider_sign_in_docs_test.go` that fails if a guide, README or `.env.example`
  gives a provider route, a provider variable, or sign-in with Google, Microsoft, Okta or SSO takes its file list from
  `git ls-files`, so an untracked or ignored file is not read whatever its name or directory. It names no directory or
  file to skip other than the changelogs, and it needs git on PATH and a git checkout. A second test builds a
  scratch repository and fails if an untracked, ignored or non-Markdown file is read.

### Fixed — the backend image's license manifest takes the URL for `gopkg.in/yaml.v3` from the repository (#687)

- The license-notices step of the backend image build (`apps/backend/scripts/third-party-notices.sh`) now writes
  the `manifest.csv` license URL of each library listed in `apps/backend/go-licenses-urls.tsv` from that file, with
  `{version}` replaced by the library's version in the module's build list (`go list -m`). It replaces whatever URL
  `go-licenses report` gave for that library, so the row no longer depends on go-licenses resolving the library's
  repository during the build. The file lists `gopkg.in/yaml.v3`, whose row now reads
  `https://github.com/go-yaml/yaml/blob/v3.0.1/LICENSE`.
- An image build of `main` on 2026-10-11 failed at this step with `third-party notices: gopkg.in/yaml.v3 has no
  license URL, so the manifest cannot say where its source is`.
- The build fails if `go-licenses-urls.tsv` is missing, if a row's URL is not `https://` with `{version}`, if a
  library is listed twice, if `go list -m` gives no version for a listed library or `go.mod` replaces it, or if a
  listed library is not in the manifest exactly once. A library not listed in the file still fails the build when
  the report gives it no license URL.

### Fixed — the A2A guide documents MCP attestation as a call the agent makes, and says where delegation trust attenuation is applied (#679)

- `docs/guides/A2A.md` said the SDK creates an attestation the first time an agent uses an MCP server, and showed
  one being created under `@agent.perform_action`. The body of `perform_action` makes no attestation call. The
  guide's manual example called `AIMClient.from_sdk_token()` and `client.create_attestation()`, and the Python SDK
  defines neither. The section now names the call, `attest_mcp()` (`AIMClient.attestMcp()` in the Java SDK), the
  challenge and attest routes it signs against, and the fields it signs. It names `use_mcp_tool()` as the method
  that attests for the agent, with the cases in which it does, and the manual route a member can use without
  an agent key.
- The guide said each delegation's `trustAttenuationFactor` (default 0.8) is multiplied in at each hop, that a
  chain falling below `minDelegatedTrustScore` (default 0.3) is rejected, and that with per-capability trust
  thresholds a deeply delegated agent cannot execute high-risk capabilities. The server stores both values for
  each delegation in `a2a_delegation`, and no server code reads them. Only the TypeScript SDK's
  `verifyDelegationChain()` attenuates, from fields named `trustAttenuation` and `minDelegatedTrust`, and its
  first delegation keeps the root trust: for the guide's chain of three delegations it reports 1.0, 0.8 and 0.64,
  where the guide's diagram gave 0.8, 0.64 and 0.51. The section now says that the server, the Python SDK and the
  Java SDK do not attenuate, gives the TypeScript fields and their defaults, and says that both fields are left
  out of a delegation's signed payload, so whoever passes a chain on can change them.
- Two tests hold these. Every route, SDK method, signed field, attestation trigger and default the attestation
  section names must be the one the server, the Python SDK, the Java SDK and the dashboard carry, and every method
  its Python examples call must be one the Python SDK defines. The attenuation section must say the server does not
  attenuate for as long as no backend code outside the tests reads a delegation's attenuation factor or trust
  floor, and its defaults and diagram must be the figures `verifyDelegationChain()` uses and reports.

### Fixed — the tutorials, the deployment reference, the backend README and the SDK test plan give no count the repository does not bear out (#676)

- `docs/tutorials/README.md` and `docs/tutorials/api-quickstart.md` linked `docs/API.md` as "All 160 endpoints".
  That file documents 29 endpoints. Both lines now say what it holds, REST endpoints with request and response
  examples, and give no total.
- `docs/guides/QUICK_DEPLOYMENT_REFERENCE.md` gave the database 16 tables, in two places. The 127 migrations create
  96 tables and the migration runner creates `schema_migrations`, 97 in all. The guide no longer gives a total.
- `apps/backend/README.md` carried a table of coverage percentages for ten packages. It gave `internal/domain`
  97.5%, and `go test -cover ./internal/domain/` prints `coverage: 92.1% of statements`. The table is removed: a
  coverage figure holds for the commit it was measured on, and the README now says so and points to
  `go test -cover`, which prints the current figure for each package.
- `docs/SDK_TEST_PLAN.md` named Java SDK 1.1.0. `sdk/java/pom.xml` says 1.0.0, and said 1.0.0 on the report's
  date; the plan now names 1.0.0. The plan gave the Python SDK a 100% pass rate in its results table and 94.7% in
  its recommendations; the recommendations no longer restate a rate. A note at the top says the report is dated
  December 23, 2025, that it records no run against a later SDK version, where the current versions are, and that
  the Python runner it lists, `sdk_test_runner.py`, is not in the repository.
- Four tests hold these. A markdown line that links to `docs/API.md` and counts its endpoints must give the number
  that file documents. A table total in the deployment reference must be the number of tables the migrations and
  the migration runner create. The backend README must give no coverage percentage for a package. The SDK test
  plan must name no SDK version later than the one in `sdk/python/VERSION` or `sdk/java/pom.xml`, and every pass
  rate it gives an SDK must be the one its results table gives.

### Fixed — the guides' docker commands name the services, containers and network the compose file defines (#673)

- A `docker compose` command takes a service name from `docker-compose.yml` (`backend`, `postgres`, `redis`,
  `frontend`); a `docker` command takes the container name the file sets (`aim-backend`, `aim-postgres`, ...).
  `docs/quick-start.md`, `docs/examples/weather-agent.md` and the flight-search and LangChain CRUD example READMEs
  gave `docker compose` the container names, and `docker compose restart aim-backend` answers
  `no such service: aim-backend`. They now give the service names.
- `docs/guides/INSTALLATION.md`, `docs/troubleshooting/README.md` and `docs/troubleshooting/authentication.md`
  named containers `identity-backend`, `identity-postgres`, `identity-redis` and `identity-frontend`, which no
  compose file creates, and gave `docker compose` the service `identity-backend`, which
  `docker compose logs identity-backend` answers with `no such service: identity-backend`. They now name the
  `aim-*` containers and the `backend` service. The troubleshooting guide inspects the network the stack creates,
  `agent-identity-management_aim-network`, where it named `agent-identity-management_default`; every service joins
  `aim-network`, so the stack creates no default network.
- The installation guide's sample `docker compose ps` output lists each of the eleven services
  `docker compose up -d` starts, with its container name beside its service name.
- The backend image has no Go toolchain: its runtime stage holds the `aim-server`, `aim-migrate`, `aim-bootstrap`
  and `aim-breakglass` binaries in `/app`. `docs/guides/BOOTSTRAP_GUIDE.md` ran `go run cmd/bootstrap/main.go` in
  the backend container; it now passes the same flags to the one-shot `bootstrap` service
  (`docker compose run --rm bootstrap --admin-email=... --yes`), which runs `/app/aim-bootstrap`.
  `infrastructure/README.md` ran `go run cmd/migrate/main.go status` there; it now reads the migration lines the
  server logs at startup. `docs/guides/DEPLOYMENT_CHECKLIST.md` ran `/app/migrate up`, a path the image does not
  have; it now runs `/app/aim-server migrate`.
- A test reads every docker and docker compose command in the repository's markdown, except this changelog, and
  fails on a service the addressed compose file does not define, a container no compose file creates, a stack
  network the stack does not create, or a command in the backend container that is `go` or an `/app/` path the
  image does not ship. A second test pins the installation guide's sample `docker compose ps` output to
  `docker-compose.yml`. Test: `compose_docs_names_test.go` (server documentation).

### Fixed — the setup, deployment and troubleshooting guides check the server's health at `/health` (#670)

- `docs/guides/SETUP_GUIDE.md`, `docs/guides/DEPLOYMENT_CHECKLIST.md` (six places),
  `docs/troubleshooting/README.md` and `examples/README.md` told readers to check `/api/v1/health`, a path the
  server does not mount. They now check `/health`, which the server mounts beside `/health/ready`, and which the
  dashboard passes to the server on a deployment's domain (`apps/web/next.config.ts`). The setup guide's sample
  response now has the fields `/health` returns: `service`, `status` and `time`.
- A test reads every markdown file in the repository and fails on a health URL for the server, on port 8080 or a
  deployment's domain, whose path `main.go` does not mount. It also checks the setup guide's sample response
  against the handler's fields. Test: `health_docs_paths_test.go` (server routes).

### Fixed — the security guides name the credential files, the SDK tokens page, the access token lifetime and the record a reused token writes (#678)

- `docs/guides/SECURITY.md` names `~/.aim/agents/<agent-name>.json` for agent credentials in its storage section, its
  `chmod` example and its developer checklist, where it named `~/.aim/credentials.json`. The checklist no longer asks
  for the cryptography and keyring packages or an `.encrypted` file: the SDK keeps credentials as plaintext JSON
  protected by file permissions. Its git examples name `.aim/sdk_credentials.json`, the file an SDK download carries.
- The guide links the SDK tokens list at `/dashboard/credentials#sdk-tokens` (Developers → Credentials → SDK tokens),
  where it named `/dashboard/settings/sdk-tokens` and Settings → SDK Tokens, and its incident steps use the list's
  **Revoke all** button, which asks for a reason.
- The guide's refresh example sends `refreshToken`, the field `POST /api/v1/auth/refresh` binds, and shows the
  response's `accessToken`, `refreshToken`, `tokenType`, `expiresIn` and `rotated` fields, with `expiresIn` 7200, the
  2-hour default of `JWT_ACCESS_TTL`. It showed snake_case fields and an `expires_in` of 86400. The token rotation
  guide's flow diagram uses the same field names.
- `docs/security/token-rotation.md` says access tokens last 2 hours by default, set by `JWT_ACCESS_TTL`, so a refresh
  token rotates about every 2 hours during active use, where it said 1 hour.
- Its theft scenario and monitoring section say that a rotated-out refresh token presented again revokes its whole
  token family, the agent's current token included, and writes a `refresh_token_reuse` audit record, and that each
  later refused token of the family writes `refresh_session_revoked`; both are listed under Organization → Audit log
  and neither sends a notification. They said a security alert was generated, and pointed to an Admin → Security →
  Token Activity page that the dashboard does not serve.
- `apps/backend/internal/interfaces/http/handlers/security_guides_token_docs_test.go` downloads an SDK, refreshes and
  reuses its token against the refresh route, reads the Python SDK's credential paths and the dashboard's Credentials
  page, and fails when either guide quotes another file, page, lifetime or response field, or when the token rotation
  guide mentions an alert.

### Fixed — the guides describe the 9-factor trust score and quote the scores the trust calculator gives (#675)

- `docs/quick-start.md`, `docs/examples/weather-agent.md`, `docs/sdk/python.md`, `docs/sdk/trust-scoring.md` and
  `docs/DEMO_SCRIPT_ECHOLEAK.md` list nine factors with the weights the trust calculator uses: verification status
  25%, uptime 15%, action success rate 15%, security alerts 15%, compliance 10%, age 5%, drift detection 3%, user
  feedback 2% and execution isolation 10%. Their factor lists and example breakdowns had eight factors, and where they
  gave weights, age was 10% and drift detection and user feedback 5% each.
- The EchoLeak demo script says a new pending agent starts at 59% (0.5925), where it said 68%, and shows how the
  figure is made up: a new agent has no compliance data and no user feedback, so those two factors are left out of the
  mean, and the published score is capped at the sum with each of them counted at 0.50. It gives 82% for a new
  verified agent with no verifications recorded, where it said 90%, and its attack walkthrough starts from 59%.
- The quick-start and the weather agent example show the breakdown of an agent verified that day whose verifications
  all succeeded, with no compliance snapshot, user feedback or isolation attestation: 0.835, with compliance and user
  feedback marked as excluded. The Python SDK guide and the trust scoring guide show a breakdown with all nine factors
  measured (0.93 and 0.94), and the trust scoring guide's report example weighs each of the nine factors.
- `docs/DOCUMENTATION_INDEX.md` and the Trust Scoring Guide links in the SDK guides say 9-factor.
- `apps/backend/internal/application/trust_score_docs_test.go` computes each score these pages quote with the trust
  calculator, in the state the page describes, and fails when a page quotes another figure, lists another weight or
  describes an 8-factor score.

### Added — the agent and API key services send agent, trust score and API key webhook events (#672)

- The agent service sends `agent.created` for an agent it registers, and `agent.verified` as well when the
  agent's verification at registration is stored. It sends `agent.updated`, `agent.verified`, `agent.suspended`
  and `agent.deleted` for an update, verification, suspension or deletion it stores. After an administrator's
  trust score update (`PUT /api/v1/agents/:id/trust-score`) or a capability violation, it sends
  `trust_score.changed` when the stored score differs from the one before, `trust_score.low` when the trust
  score policy finds a fall that ends below 70% (`severity` is `critical` below 50% and `warning` otherwise),
  and `agent.suspended` when the policy suspends the agent. A refused or failed registration, update,
  verification, suspension or deletion sends nothing, and `trust_score.changed` is sent only for a stored score.
- The API key service sends `api_key.created` for a key it creates and `api_key.revoked` for an active key it
  disables. Neither event carries the key or its hash.
- Each event is dispatched after the change is stored, on a goroutine of its own and under a background
  context, so it does not end with the request.
- This change set no webhook service on the alert, agent or API key service in the server. The entry that
  bounds webhook delivery (#672, above) sets it, and from then these events and the alert events reach
  their endpoints. `POST /api/v1/webhooks/:id/test` sends a test delivery.
- `docs/API.md` lists each event, when it is sent, the data it carries, the defined names for which no event is
  sent, and that delivery is not switched on. `docs/guides/API_REFERENCE.md` and the A2A guide point at that
  list. The names `agent.registered`, `agent.revoked`, `trust_score.updated`, `compliance.report_generated`,
  `api_key.expiring`, `mcp.connection.new` and `mcp.drift.detected`, which the backend does not define, are no
  longer in the docs. Tests: `apps/backend/internal/application/webhook_events_test.go`.

### Fixed — the guides clone from and link to `github.com/opena2a-org/agent-identity-management` (#669)

- Nineteen references in `docs/quick-start.md`, `docs/DEPLOYMENT.md`, `docs/DOCUMENTATION_INDEX.md`,
  `docs/guides/` (`INSTALLATION.md`, `CONTRIBUTING.md`, `BOOTSTRAP_GUIDE.md`, `API_REFERENCE.md`,
  `API_ENDPOINT_SUMMARY.md`), `infrastructure/DEPLOYMENT.md` and `infrastructure/deployment/README.md` named
  `github.com/opena2a/agent-identity-management`, `github.com/opena2a/identity` or `github.com/opena2a/aim-sdk-go`.
  The project has no repository under the `opena2a` account: `git ls-remote` on both repository URLs answers
  `Repository not found`, and the repository, issue and discussion pages answer 404. The clone, upstream remote,
  issue and discussion links now point at `github.com/opena2a-org/agent-identity-management`, and each clone step
  changes into `agent-identity-management`.
- The import path in the contributing guide's code-organization sample is the backend module's,
  `github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain`.
- `scripts/lint-docs-repo-links.sh` rejects a link to the `opena2a` account in any Markdown page other than a
  changelog, with its file and line. Tests: `scripts/test-lint-docs-repo-links.sh`.

### Fixed — The TypeScript examples in the PQC, A2A and direct API pages import names the SDK exports (#680)

- `docs/guides/PQC.md` imported `isPqcAvailable`, `signHybrid`, `verifyHybrid` and `Algorithm` from
  `@opena2a/aim-sdk`, and `docs/guides/A2A.md` and `docs/api/direct-api-usage.md` imported `secure`. In a build of the
  SDK, `require('@opena2a/aim-sdk')` gives `undefined` for all five names: the SDK exports `isPQCAvailable`,
  `hybridSign` and `hybridVerify`, exports `Algorithm` as a type only, and has no `secure`.
- The PQC example now calls `isPQCAvailable`, `generateMLDSAKeyPair`, `signMLDSA`, `verifyMLDSA`,
  `generateHybridKeyPair`, `getHybridPublicKey`, `hybridSign`, `hybridVerify` and `encodeHybridSignature` with the
  SDK's signatures: a variant string such as `'ML-DSA-65'`, `hybridSign(keypair, message)`, and
  `hybridVerify(publicKey, message, signature)` resolving to a boolean. The page says `Algorithm` is a type-only union
  of the algorithm strings. The A2A example registers the orchestrator with `AIMClient.registerAgent` and lists its
  peers in `talksTo`, the field the SDK sends, in place of `mcpServers`, which the registration options do not have.
  The direct API page lists the TypeScript SDK as available, with a `registerAgent` and `verifyAction` example, in
  place of a "coming in 2026" preview.
- An SDK test (`sdk/typescript/src/docs-snippets.test.ts`) type-checks every TypeScript block under `docs/` that
  imports `@opena2a/aim-sdk` against the SDK source, and fails on a name the entry point does not export, a type used
  as a value, or a call that does not match the exported signature.

### Fixed — SECURITY.md describes the CSRF, SDK credential, configuration and personal-data controls the code has (#677)

- `SECURITY.md` said CSRF was prevented with tokens, that SDK credentials were encrypted with Fernet and a system
  keyring key, that several required secrets fell back to weak defaults when unset, and that all personal data was
  encrypted at rest. The server issues and checks no CSRF token, `aim-sdk login` writes
  `~/.aim/sdk_credentials.json` as plain JSON with file mode 0600, the server refuses to start without `JWT_SECRET`,
  and the `users` table keeps `email` and `name` in plain `VARCHAR(255)` columns.
- The page now says that the server reads no credential from a cookie and authenticates a request only by a token,
  API key or signature sent in a request header; that the Python SDK stores its login tokens and agent keys
  unencrypted, as plain JSON with file mode 0600; which missing, short or development-value secrets stop the server
  from starting, and that `POSTGRES_SSL_MODE` still defaults to `disable`; and that the application does not encrypt
  personal data at rest, with encryption in transit coming from the HTTPS endpoint in front of the server and from
  `POSTGRES_SSL_MODE`.
- A backend test (`apps/backend/cmd/server/security_policy_controls_contract_test.go`) fails if the page names a
  token-based CSRF control, Fernet or keyring encryption, secrets that fall back to weak defaults, or encrypted
  personal data, and checks each replacement statement against the code: no backend Go file reads a cookie or
  installs a CSRF middleware, the SDK's credential writer stores JSON with mode 0600, the configuration validator and
  key vault refuse the secrets the page names, and no migration changes the `users` email or name column.

### Fixed — The deployment, installation and troubleshooting pages name only files the repository carries (#674)

- `docs/DEPLOYMENT.md`, `docs/guides/DEPLOYMENT_CHECKLIST.md`, `docs/guides/INSTALLATION.md`, `docs/quick-start.md`,
  `docs/troubleshooting/README.md`, `infrastructure/README.md`, `infrastructure/DEPLOYMENT.md` and
  `infrastructure/deployment/README.md` told readers to run `./deploy.sh`, `./scripts/deploy-azure-production.sh` and
  `./scripts/migrate-to-postgres.sh`, to start `docker-compose.prod.yml` and `docker-compose.monitoring.yml`, to mount
  `infrastructure/nginx/nginx.conf`, to apply `infrastructure/k8s/*.yaml`, to build from `apps/backend/Dockerfile` and
  `apps/web/Dockerfile`, and to `cd` into `infrastructure/azure`, `infrastructure/gcp`, `infrastructure/aws` and
  `identity`. The repository carries none of them.
- Each step now uses what the repository carries: `./scripts/gen-dev-secrets.sh > .env` and `docker compose` on
  `docker-compose.yml` (`up -d --build` to build from the checkout, `up -d postgres redis` or
  `up -d prometheus grafana loki promtail` to start a subset, `down -v` to reset), `docker compose run --rm bootstrap`
  for the first administrator, `./infrastructure/scripts/deploy-azure-production.sh`,
  `./infrastructure/deployment/{azure,gcp,aws}/deploy.sh` run from the repository root, and
  `infrastructure/docker/Dockerfile.backend` and `Dockerfile.frontend`. The Kubernetes sections say the repository
  ships no manifests and apply the reader's own; the HTTPS step says it ships no reverse-proxy configuration. Grafana's
  port reads 3003 and its dashboards `infrastructure/monitoring/grafana/dashboards/`, as `docker-compose.yml` and the
  tree have them.
- The troubleshooting page sets `ALLOWED_ORIGINS`, which the server reads, for a CORS error, in place of editing
  `apps/backend/internal/api/middleware/cors.go`, which does not exist; reads backend logs with
  `docker compose logs backend` in place of `apps/backend/logs/oauth.log`, which the server does not write; and no
  longer has a "database is locked" entry, which described an SQLite mode the server does not have.
- The quick deployment reference no longer lists `AIM_DEPLOYMENT_TEST_REPORT.md` and `DEPLOYMENT_SUCCESS_SUMMARY.md`,
  and the start-here page no longer lists Kubernetes manifests and issue templates; the repository carries none of
  them. Links to `docs/deployment/`, `docs/security/best-practices.md`, `docs/index.md`, `docs/api/README.md`,
  `docs/sdk/README.md`, `infrastructure/deployment/{aws,azure,gcp}/README.md` and `k8s/`, none of which exists, point
  to the page that covers the topic or are removed.
- A backend test (`apps/backend/cmd/server/deployment_docs_files_contract_test.go`) reads these pages and fails on a
  script, compose file, manifest, directory, link target or repository path they name that the repository does not
  carry.

### Fixed — The API reference pages, tutorials and guides name only routes the server registers (#671)

- `docs/API.md`, `docs/guides/API_REFERENCE.md`, `docs/guides/API_ENDPOINT_SUMMARY.md`, `docs/api/direct-api-usage.md`,
  the API quickstart and MCP registration tutorials, the fleet governance, trust scoring, A2A and capability scope
  pages, both troubleshooting pages, the SDK test plan and the Python SDK's LangChain guide named methods and paths
  that are not in the server's route table, among them `/api/v1/keys`, `/api/v1/trust-scores/{agentId}`,
  `/api/v1/audit-logs`, `/api/v1/alerts`, `/api/v1/compliance/reports`, `/auth/google`, `/api/v1/auth/login/:provider`,
  `/api/agents/register`, `/api/v1/audit`, `PATCH /api/v1/agents/{id}`, `/debug/pprof/heap` and `/swagger/`. Each now
  names the route that does the job (`/api/v1/api-keys`, `/api/v1/trust-score/agents/{id}`, `/api/v1/admin/audit-logs`,
  `/api/v1/admin/alerts`, `/api/v1/compliance/export`, `/api/v1/public/login`, `POST /api/v1/agents`,
  `POST /api/v1/agents/{id}/reactivate`) or says the server has none.
- The rate limits read as the limiter sets them: 100 requests a minute on most routes and 10 on the credential routes,
  with a 429 body of `{"error": "Rate limit exceeded. Please try again later."}`. An error is documented as a JSON body
  whose `error` member is a message, and the pagination section gives each list route's own response, such as
  `{"agents": [...], "total": n}` for `GET /api/v1/agents`, in place of a common envelope.
- The agent registration examples send the `display_name` and `agent_type` values the server requires, and linking an
  agent to MCP servers is `PUT /api/v1/agents/{id}/mcp-servers` with a member's access token. The endpoint summary no
  longer reports an endpoint total, completion percentages or its "SDK Support (Planned)" list, and the count in each
  section's heading is the number of rows in that section's table. The `POST /api/v1/api-keys` response example in
  `docs/API.md` shows `aim_live_...`, the prefix the server gives a key, in place of a 43-character value with an
  `aim_sk_` prefix, which the server does not issue.
- A backend test (`apps/backend/cmd/server/api_docs_routes_contract_test.go`) reads these pages and fails on a method
  and path the server does not register.

### Fixed — `migrate status` lists migrations without changing the database, and `migrate down`, `force` and `drop` are refused (#668)

- `cmd/migrate` read no arguments. On an empty PostgreSQL 18 database, `status`, `create add_column_to_agents`,
  `down 1`, `force 001`, `drop` and an unknown word (`frobnicate`) each applied all 127 migrations and exited 0. On a
  database with every migration applied, each left `schema_migrations` and the table count unchanged and exited 0.
  `create` wrote no file. The tool now reads its command before it connects to the database.
- `status` prints how many migrations are applied, the last one applied, each pending migration, and any
  `schema_migrations` row with no matching file. It only reads: on a database without `schema_migrations` it reports
  every migration pending and does not create the table.
- `create <name>` writes `migrations/<next number>_<name>.sql` with a comment header, and needs neither a database nor
  `DATABASE_URL`. The name is lowercase letters and digits joined by underscores.
- `down`, `force` and `drop` are refused with exit code 2 and the reason, without connecting: the migrations have no
  down scripts, applied migrations are recorded by file name rather than as one version number, and the tool does not
  delete data. Any other command, and an argument after `up` or `status`, is refused the same way. No arguments, and
  `up`, apply the pending migrations as before.
- The installation, contributing and bootstrap guides list only the commands the tool offers, and the bootstrap
  guide's expected output is the output the tool prints.
- Tests: `commands_test.go` (migrate command).

### Security — every dashboard page is served with a Content-Security-Policy

- The dashboard sends a `Content-Security-Policy` header with the pages, static files and not-found answers it
  serves. Not every response carries this policy. A path the dashboard relays to the backend (`/api/*`, `/health`,
  `/health/*`, `/.well-known/agent.json`, `/.well-known/aip`) is answered with the backend's `Content-Security-Policy`
  header when the backend sends one, and with none otherwise. A 308 redirect for a moved page listed in
  `apps/web/lib/redirects.ts`, or for a path with a trailing slash, carries none. The policy is built in
  `apps/web/lib/content-security-policy.ts` and set from `headers()` in `apps/web/next.config.ts`. Script loads only
  from the dashboard origin. `connect-src` limits a request made with `fetch`, `XMLHttpRequest`, WebSocket,
  `EventSource` or `sendBeacon` to the dashboard origin and the origin of `NEXT_PUBLIC_API_URL`. The policy does not
  close every route by which a script running on the page could send data out: `img-src` allows an image request to
  any `https:` origin. `object-src 'none'`, `base-uri 'self'`, `form-action 'self'` and `frame-ancestors 'none'` rule
  out plugins, a `<base>` that points to another origin, a form post to another origin, and framing by any site.
- On a `-frontend` hostname the client calls the backend at the same hostname with one `-frontend` replaced by
  `-backend`, and `connect-src` also lists that backend host: `aim-frontend.example.com` calls
  `aim-backend.example.com`. One rule, in `apps/web/lib/backend-host.ts`, decides both. It applies to a hostname made
  only of `a-z`, `0-9`, `.` and `-`, and the `-frontend` it replaces is the first one with 1 to 253 characters before
  it and at most 253 after it (253 is the length limit of a host name). Where the rule does not apply
  (`my_app-frontend.test`; `-frontend.example.com`, where nothing precedes the `-frontend`), the client calls the
  `NEXT_PUBLIC_API_URL` origin and `connect-src` lists no derived host.
- The header takes the derived backend host from the `Host` header the dashboard server receives, and does not read
  `X-Forwarded-Host` for it. A reverse proxy in front of a `-frontend` dashboard has to pass the public host name on
  in `Host` (nginx: `proxy_set_header Host $host;`). When the server receives another `Host`, `connect-src` does not
  list the backend the browser derives from the address bar unless that backend is also the `NEXT_PUBLIC_API_URL`
  origin.
- `script-src` and `style-src` keep `'unsafe-inline'`. Next.js writes its bootstrap data as inline scripts and the
  theme is set by one, and a header set in `next.config` cannot carry a per-request nonce for statically rendered
  pages. `img-src` allows `https:` so a registration request shows the profile picture its identity provider
  returned. Under `next dev` the policy adds `'unsafe-eval'`, which React's development build needs.
- For deployers: `NEXT_PUBLIC_API_URL` is read when the dashboard is built, for both the client bundle and this
  header. The browser refuses an API request to any origin other than the dashboard's own, the `NEXT_PUBLIC_API_URL`
  origin and the derived `-backend` host, so build with `NEXT_PUBLIC_API_URL` set to the origin the browser uses.
  Setting the variable on the running container does not change the header: the standalone server of a dashboard
  built with one value and started with another sends the built value. `docs/guides/DEPLOYMENT_CHECKLIST.md` has the
  build command and the proxy requirement.
  `frame-ancestors 'none'` stops every site, the dashboard's own origin included, from showing it inside a frame.

### Fixed — `POST /api/v1/a2a/verify` carries the shared rate limiter (#654)

- `POST /api/v1/a2a/verify` takes no credentials. It now carries the shared rate limiter on its own registration
  line, as `GET /api/v1/did/*` and `GET /api/v1/revocations` do: under the production limits, a client that has
  sent 100 such requests within a minute is answered 429 on the next. Another method on the path that reaches the
  routes is answered 405 as before.
- The test that requires `/api/v1/a2a/verify` to be mounted once now reads the routes from the router the server
  builds, as that router matches them, rather than the source text. A second mount on the path with a trailing
  slash, or through `Add`, is now counted.
- Tests: `a2a_verify_route_test.go` (server routes).

### Changed — the READMEs say what decides an SDK verification, and where the 5-step engine runs (#656)

- The README, `sdk/python/README.md` and `sdk/java/README.md` say that `@perform_action` and `performAction()` have
  the server check each call against the capabilities granted to the agent and the organization's security
  policies; they said each call ran through Fine-Grained Authorization (FGA). The SDKs post each call to
  `/api/v1/sdk-api/verifications`, which decides with those. The 5-step engine runs on
  `POST /api/v1/agents/:id/authorize`, which no SDK in this repository calls.
- The Fine-Grained Authorization section, now in `docs/server-features.md`, names that route and says it runs step 1
  only when the agent has no FGA policy for the capability. It says step 5 runs for HIGH- and MEDIUM-risk policies
  without holding the decision, never denies, and needs a NanoMind daemon (`FGA_DAEMON_URL`) that neither compose
  file starts. The observability section, now in `docs/DEPLOYMENT.md`, says a decision carries one child span per
  step it ran, where it said five.
- Routing SDK verifications through the 5-step engine is still open.
- Test: `sdk/python/tests/test_perform_action_authorization_claims.py`.

### Fixed — `GET /api/v1/status` reports no `oauth` feature, and the trust-score paragraph names both verification routes (#685)

- The `features` object in the `GET /api/v1/status` answer no longer has an `oauth` member. It was computed from
  whether `GOOGLE_CLIENT_ID` is set, and no route serves provider sign-in (#667), so it described nothing the server
  does. The other members are unchanged. A backend test fails if a source file of the server package reads a
  provider setting.
- The trust-score paragraph, now in `docs/server-features.md`, says that a verification request through either
  route, `POST /api/v1/verifications` or `POST /api/v1/sdk-api/verifications`, does not recalculate the score; the
  two routes share a handler, and the paragraph named only the first.
- `npx @opena2a/aim-sdk init` reads the bootstrap token from `AIM_BOOTSTRAP_TOKEN` only and refuses a flag that
  carries it; see the TypeScript SDK changelog.

### Removed — the guides no longer describe sign-in with Google, Microsoft or Okta, which no route serves (#667)

- Users sign in with an email and password: the dashboard posts them to `POST /api/v1/public/login`, and API clients
  can use `POST /api/v1/auth/login/local`. The config reads the `GOOGLE_*`, `MICROSOFT_*` and `OKTA_*` variables, but
  no mounted route signs a user in with a provider.
  `docs/guides/INSTALLATION.md`, `docs/guides/SETUP_GUIDE.md`, `docs/DEPLOYMENT.md` and
  `docs/guides/DEPLOYMENT_CHECKLIST.md` gave those variables and the steps to register an application with each
  provider; `docs/API.md`, `docs/guides/API_REFERENCE.md`, `docs/guides/API_ENDPOINT_SUMMARY.md` and
  `docs/guides/SETUP_GUIDE.md` listed `/auth/google`, `/api/v1/auth/login/:provider` and
  `/api/v1/auth/callback/:provider`; `docs/guides/QUICK_DEPLOYMENT_REFERENCE.md`, `docs/guides/START_HERE.md`,
  `docs/guides/BOOTSTRAP_GUIDE.md`, `docs/guides/SECURITY.md`, `docs/DOCUMENTATION_INDEX.md`, `docs/troubleshooting/`,
  `infrastructure/DEPLOYMENT.md`, `infrastructure/deployment/README.md` and `SECURITY.md` described sign-in with OAuth
  or SSO. Those sections are removed, and the API guides list `POST /api/v1/auth/login/local` in their place.
- `.env.example` no longer offers `GOOGLE_CLIENT_ID` for "Sign in with Google", and
  `infrastructure/scripts/setup-oauth.sh`, which created a Google OAuth client with `gcloud` and wrote it to
  `apps/backend/.env`, is removed with its entry in `infrastructure/README.md`.
- A backend test sends a signed-in request to each provider login and callback path through the routes the server
  mounts and fails if one is served, and fails if a guide, README or `.env.example` gives a provider route, a provider
  variable, or sign-in with Google, Microsoft, Okta or SSO.

### Fixed — the pages that install the Python SDK give `pip install aim-sdk`, the README's install command (#663)

- The Python SDK is published on PyPI as `aim-sdk` (`name="aim-sdk"` in `sdk/python/setup.py`). `docs/quick-start.md`
  and `docs/tutorials/sdk-quickstart.md` said "There is NO pip package", `docs/sdk/python.md` said "There is NO pip
  package for the AIM SDK" under a "NO pip install Available" heading, and `docs/examples/weather-agent.md`,
  `docs/examples/flight-tracker.md`, `docs/examples/database-agent.md`, `docs/integrations/crewai.md` and
  `docs/integrations/langchain.md` said "NO pip install available - must download from dashboard". Each page now gives
  `pip install aim-sdk`; `docs/quick-start.md`, `docs/sdk/python.md` and `docs/tutorials/sdk-quickstart.md` follow it
  with `aim-sdk login --url http://localhost:8080`, and describe the dashboard's pre-configured SDK download as an
  alternative.
- Those five example and integration pages linked to
  `quick-start.md#step-3-download-aim-sdk-and-install-dependencies-30-seconds`, an anchor no heading produced. Step 3
  of the quick start is now "Install the AIM SDK", and each link points at its anchor.
- A test in the Python SDK suite fails when a page under `docs/` says the SDK has no pip package, when one of these
  eight pages does not give `pip install aim-sdk`, or when a link from a page under `docs/` into `quick-start.md` names
  an anchor no heading produces.

### Fixed — the setup, installation and contributing guides state the Go and Node.js versions the platform needs (#660)

- `apps/backend/go.mod` declares `go 1.25.0`: Go 1.24.0 refuses to build the backend
  (`go.mod requires go >= 1.25.0`), and Go 1.25.0 builds every backend package and compiles its tests. The guides
  named Go 1.21+ (`apps/backend/README.md`, `docs/guides/CONTRIBUTING.md`, `docs/guides/INSTALLATION.md`), 1.22+
  (`README.md`), 1.23+ (`CONTRIBUTING.md`, `docs/DEPLOYMENT.md`, `docs/guides/QUICK_START.md`) and 1.23.1+
  (`docs/guides/SETUP_GUIDE.md`). Each now names Go 1.25+; the README's line is now in `docs/DEPLOYMENT.md`.
- The dashboard's locked dependencies declare the Node.js versions they support: Next.js 16 declares 20.9 or later,
  and the test and lint dependencies declare 20.19 or later on Node.js 20 and 22.13 or later on Node.js 22. The
  guides named Node.js 18+ (`CONTRIBUTING.md`, `docs/guides/CONTRIBUTING.md`, `docs/guides/INSTALLATION.md`,
  `docs/guides/QUICK_START.md`), 20+ (`README.md`, `docs/guides/SETUP_GUIDE.md`) and 22+ (`docs/DEPLOYMENT.md`).
  Each now names Node.js 20.19+ or 22.13+; the README's line is now in `docs/DEPLOYMENT.md`.
- `docs/guides/CONTRIBUTING.md` and `docs/guides/INSTALLATION.md` listed pnpm 8+ and gave pnpm commands; the
  repository installs with npm from `package-lock.json`. They now list npm 10+, which the root `package.json`
  requires, and give the npm form of each command.
- A test in the TypeScript SDK suite fails when one of these guides states a Go version below the `go` directive of
  `apps/backend/go.mod`, a Node.js version outside the `engines.node` range of the root `package.json` or of a
  package in the root or dashboard lockfile that is not limited to one platform, or a pnpm command.

### Added — the server forwards every audit log entry it writes to a Splunk HTTP Event Collector (#657)

- No server code imported the SIEM adapters in `internal/integrations/siem`. Setting `SIEM_SPLUNK_HEC_URL` and
  `SIEM_SPLUNK_HEC_TOKEN` now forwards each audit log entry, once its write succeeds, as a HEC event with sourcetype
  `opena2a:aim` and event type `aim.audit.<action>`. Entries are queued (up to 1,000) and sent in batches of up to 100
  at least every 5 seconds. A batch the collector cannot take now (no connection, 408, 429 or 5xx) is tried up to 3
  times; any other refusal is not retried. `SIEM_MIN_SEVERITY` leaves out entries below a severity. A full queue
  leaves the entry out of forwarding and logs a warning; the request that wrote it never waits for the collector.
  `SIEM_SPLUNK_INDEX` names an index; unset, the event names none and the token's default index applies.
- Startup stops with an error that names the variable when only one of the URL and the token is set, when the URL is
  `http` to a host that is not loopback, or when the severity is not `info`, `low`, `medium`, `high` or `critical`.
  At shutdown the queued entries are sent within 2 seconds. See `docs/DEPLOYMENT.md`, "SIEM Forwarding".
- The README no longer lists Microsoft Sentinel delivery: the Azure Monitor HTTP Data Collector API that the Sentinel
  adapter targets was retired on 14 September 2026, and the server does not load the adapter.
- Tests: `forwarder_test.go`, `splunk_test.go`, `audit_test.go` and `env_test.go` (SIEM adapters),
  `audit_log_on_create_test.go` (audit log repository) and `siem_forwarding_test.go` (server).

### Fixed — `POST /api/v1/a2a/verify` is mounted once, and another method on the path that reaches the routes is answered 405 (#652)

- `POST /api/v1/a2a/verify` was mounted twice: once without authentication, and again inside the authenticated
  `/a2a` group. The first answers every POST that reaches the routes, so the second was never reached; it is
  removed. A GET, PUT, PATCH or DELETE on the path that reaches the routes passed to the group: without credentials
  it was answered 401 `No authentication token provided`, and with a valid access token it was already answered
  405. It is now answered 405 with `Allow: POST` before the group's authentication runs. A POST is answered as
  before.
- The transition recorder's tests now refuse a change whose event id is the event id of the nil request and which
  names no request, and a registration given the event id of the request it names. They tell the refusal of a
  change given its own agent's opening event id, and of an event id that is not a lowercase hyphenated UUID, each by
  its own message.
- Tests: `a2a_verify_route_test.go` (server routes) and `transition_test.go` (transition recorder).

### Fixed — both 400 answers of the `POST /api/v1/a2a/verify` handler carry `valid: false` (#649)

- `POST /api/v1/a2a/verify` answered 400 with `error` only, both for a nonce it refuses and for a body it cannot read
  (`Invalid request body`), while its 401 carried `valid: false`. A client that branches on `valid` found it absent
  in the 400. Both 400 answers now carry `valid: false` beside `error`. The status codes and messages are unchanged.
  A 500 still carries `error` only: it says nothing about the request.
- Tests: `a2a_verify_server_error_test.go` (HTTP handlers) covers both 400 answers.

### Fixed — filing a pending capability request no longer reads every agent of the organization (#648)

- The transition recorder read the id of every agent in the organization for each change that carried an event id, to
  refuse one that was another agent's opening event id. Only the record of a pending capability request carries one,
  so each such filing in an organization of N agents read N rows. The recorder now takes a given event id only with
  the request it is derived from, and refuses any other before anything is read. A request's event id and an agent's
  opening event id are derived in separate namespaces, so the event id of a request is no agent's opening event id.
- A change given the opening event id of its own agent, or of another agent in the organization, is still refused
  before its statement runs. So is a change given an event id that is not the event id of the request it names, and
  a change that names a request and gives no event id. Only server code gives a change an event id; no API request
  can.
- Tests: `transition_test.go` covers a change given an event id, which reads its own agent before and after its
  statement and runs no other statement, and the event ids the recorder takes and refuses.
  `authorization_transition_opening_integration_test.go` still covers the opening event id of another agent.

### Fixed — an A2A request whose nonce cannot be stored is answered 400, and other follow-ups (#645)

- `POST /api/v1/a2a/verify` stored the request's nonce without checking it. A nonce longer than 64 characters failed
  the write and was answered 500, and an empty nonce was recorded and the request verified. A nonce that is empty,
  longer than 64 characters, not UTF-8, or that contains a NUL character is now answered 400 with
  `nonce must be 1 to 64 characters of UTF-8 text without a NUL character`, before anything is read or recorded.
- When the agent row could not be read, `POST /api/v1/a2a/verify` answered 401 with `agent not found`. It now answers
  500, as it does when the nonce table or the agent's grants cannot be read. An agent that does not exist is still
  answered 401 with `agent not found`.
- The transition recorder refused a change given its own agent's opening event id only. A change given the opening
  event id of another agent in the organization was recorded there, and that agent's next change was then refused
  with `does not open its history`. Such a change is now refused before its statement runs. Only server code gives a
  change an event id; no API request can.
- Tests: `a2a_verify_status_test.go` (application) covers the nonce check, the 64-character limit counted in
  characters, a failed agent read, and a nonce found used at the check, which is refused with
  `nonce already used (replay attack)` without reading the agent or writing the nonce; no test covered that refusal
  before. `a2a_verify_server_error_test.go` (HTTP handlers) covers the 400 and the 500. `transition_test.go` and
  `authorization_transition_opening_integration_test.go` cover the opening event id of another agent.

### Fixed — an A2A request whose nonce cannot be recorded does not verify as valid, and other follow-ups (#643)

- `POST /api/v1/a2a/verify` answered `valid: true` when the request's nonce could not be written to the nonce table:
  the write error was discarded. The nonce was then not stored, so the same signed request verified again each time
  it was presented while its timestamp was accepted. A request is now answered `valid: true` only once its nonce is
  recorded. When the write fails, the answer is 500, as it is when the nonce table cannot be read. When another
  request recorded the same nonce between the check and the write, the answer is 401 with
  `nonce already used (replay attack)`, as it is when the check finds the nonce.
- The entry for #635 states that the agent API leaves out a grant whose capability type is outside the credential's
  token grammar. It listed the honeytoken, duplicate and order differences only. No behaviour changes.
- Tests: `a2a_verify_status_test.go` (application) fails on the nonce behaviour described above.
  `agent_response_capabilities_test.go` (HTTP handlers) pins the grant the agent API leaves out. In
  `atc_issuance_handler_test.go` (HTTP handlers) the test that `POST /api/v1/agents/:id/atc` answers another
  organization's agent like a missing one runs against an issuance service that refuses and counts, so a request
  that got past the organization check would fail the test's assertions; it ran with no issuance service, where such
  a request stopped the test run with a nil-pointer panic.

### Fixed — the agent API lists an agent's capabilities as its credential does, and other follow-ups (#635)

- `GET /api/v1/agents/:id`, `GET /api/v1/agents`, `POST /api/v1/agents`, `PUT /api/v1/agents/:id` and the `agent`
  object of `POST /api/v1/agents/:id/suspend`, `/reactivate` and `/revoke` listed the capability type of every active
  grant row under `capabilities`, honeytoken grants and duplicates included, in the order the rows were read. They now
  list them as the trust credential, the agent card and the A2A verification result do: no honeytoken grant, each
  capability once, sorted ascending.
- A grant whose capability type is outside the credential's `namespace:operation` token grammar
  (`^[a-z0-9_-]+:[A-Za-z0-9_.*-]+$`), such as `write_database`, is left out of `capabilities` in the same responses,
  as it is left out of the credential. These responses listed it before. The grant stays active: the capability check
  behind `POST /api/v1/agents/:id/verify-capability` still matches it, and `GET /api/v1/agents/:id/capabilities`
  still lists it. Nothing in a response says that a grant was left out.
- When the grant table cannot be read, `GET /api/v1/agents/:id` answers 500 instead of 200 with `"capabilities": []`,
  which said the agent held no grant. The create, update, suspend, reactivate and revoke responses still report the
  change, which was already made, and carry the agent without a `capabilities` field. `GET /api/v1/agents` is
  unchanged and still lists `[]` for every agent when the page's grants cannot be read.
- `GET /api/v1/agents/:id` serves `createdByApiKeyId`, the API key the agent was created with, so the agent detail
  page's link to that key renders. The create, update, suspend, reactivate and revoke responses carry it too.
- `POST /api/v1/a2a/verify` answered `valid: true` for a correctly signed request from a suspended or revoked agent. A
  request signed by an agent whose status does not permit authentication (the same allow-list the auth middlewares
  apply: `verified` and `pending` pass) is now answered 401 with
  `Agent is not permitted to authenticate (status: suspended)`. The status is named only once the signature has
  verified, and the refusal spends no nonce.
- `POST /api/v1/a2a/verify` answers 500 when the nonce table or the agent's grants cannot be read. It answered 401 with
  `failed to check nonce` or `failed to load capability grants`, reporting a database failure as an authentication
  failure. The nonce is still not spent, so the same signed request can be retried.
- The config drift policy's capability-change and dangerous-capability checks read the agent's active grants. They read
  the list the SDK reported at registration, so a capability granted or revoked after registration was not seen; an
  agent registered with no capabilities and granted `admin:all` afterwards was reported as holding no dangerous
  capability. A honeytoken grant is not a capability to these checks. When the grants cannot be read, the evaluation
  fails and decides nothing, as it does when the policies cannot be read.
- Tests: `agent_response_capabilities_test.go` and `a2a_verify_server_error_test.go` (HTTP handlers),
  `a2a_verify_status_test.go` and `security_policy_config_drift_grants_test.go` (application) each fail on the
  behaviour described above.

### Added — the server reports at start whether audit record chains can start

- At start the server reads whether a foreign key can still remove `audit_logs` or
  `verification_events` rows by cascade, and whether any organization's audit record chain has
  started. It writes one `record_path_start` console line and serves two gauges on `/metrics`:
  `aim_record_path_pre_chain` and `aim_record_cascading_foreign_keys{constraint}`. Neither names an
  organization.
- On the schema as shipped no chain has started and five foreign keys remove audit rows by cascade,
  so the line reads `state=pre_chain` and names them, with
  `missing_migration=cascading_foreign_key_replacement`, `aim_record_path_pre_chain` is 1 and each of
  the five is a series. No chain starts in this state. Once a migration replaces them, the line reads
  `state=chained`.
- If a chain has started and such a foreign key exists, the line is a `SECURITY` line ending
  `finding=cascading_foreign_keys`: a cascade there would remove chained rows with no record.

### Changed — the compliance report and an MCP server's audit timeline name who acted by the audit record's own rule

- The audit section of `GET /api/v1/compliance/export` names each record's actor the way the audit
  record's `actorType` does. Each of `recentActions` and `topUsers` carries `actorType`, and its
  `userId` is the id of the party that type names, a user's or an agent's. The verification of an
  agent's action, which also stores the agent's owner, is counted for the agent, not the owner. A
  system act has no `userId`, rather than the all-zero id, and is not listed among the top actors.
  A recent action's `ipAddress` is absent when the record holds none.
- The CSV form of that export gains an `Actor Type` column at the end of its Recent Actions and Top
  Users tables. A system act's `User ID` cell is empty.
- PHI access evidence collected by `POST /api/v1/compliance/evidence/collect` carries `actorType`
  and the actor's `userId` for each access. It no longer fails with 500 for an organization holding
  a view or export record with no user id; such an access has no `userId`.
- An audit event on `GET /api/v1/mcp-servers/{id}/audit-logs` takes `actorType`, `actorId` and
  `actorName` from the same rule, so an agent's verified action reads as the agent's, not its
  owner's.

### Changed — audit records say who acted, and leave out an address or user agent that was never recorded

- Every audit record returned by `GET /api/v1/admin/audit-logs`, `GET /api/v1/admin/audit-logs/{id}`,
  `GET /api/v1/admin/audit-logs/export?format=json`, `GET /api/v1/agents/{id}/audit-logs` and
  `GET /api/v1/agents/{id}/activity` carries `actorType`: `agent` when the record names an agent,
  otherwise `user` when it names a user, otherwise `system`. The verification of an agent's action,
  which also stores the agent's owner as `userId`, reads `agent`. `operator_command` belongs to the
  same vocabulary; no record carries it yet.
- In those records `ipAddress` and `userAgent` are absent when the record holds none, rather than
  empty strings. On the two agent routes only an admin reads `ipAddress`, `userAgent` and the full
  `metadata`, as before; every other caller gets `actorType` and none of those three members.
- The CSV export gains an `ActorType` column after `Metadata`. It no longer fails with 500 for an
  organization holding a record of an agent's or the system's act; such a record has an empty
  `UserID` cell.

### Added — `aim-breakglass chain status`, the operator's read of an organization's audit record chain

- `aim-breakglass chain status --organization <id>` reports one organization's chain through the
  same chain-state read as `GET /api/v1/admin/audit-logs/chain/head`: `chainState`, `chainId`,
  `head`, `latestCheckpoint`, and `reason` when the chain cannot be extended. With `--json` it prints
  the route's body, member for member; without it, one line per member with what the state and the
  reason mean.
- It reads the server's database through the server's `POSTGRES_HOST`, `POSTGRES_PORT`,
  `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB` and `POSTGRES_SSL_MODE`, or through
  `DATABASE_URL` when `POSTGRES_HOST` is not set. A configuration error names the variable, never
  its value.
- An id that names no organization exits 1 with `no organization has id <id>`, rather than reading
  as a chain that has not started. Exit codes: 0 done, whatever the chain's state; 1 failed; 2 usage
  or configuration error.
- The backend image ships the command as `/app/aim-breakglass`, next to `aim-migrate` and
  `aim-bootstrap`.

### Added — admins can read the state of their organization's audit record chain

- `GET /api/v1/admin/audit-logs/chain/head` answers with `chainState` (`notStarted`, `extendable` or
  `notExtendable`), `chainId`, `head` (`seq` and `hash`) and `latestCheckpoint`, and with `reason`
  (`no_records`, `head_mismatch` or `record_modified`) when the chain cannot be extended. It reads the
  caller's organization only, through the same chain-state read the record writer runs before it
  extends a chain.
- An organization with no chain is answered 200 with `chainState` `notStarted` and `chainId`, `head`
  and `latestCheckpoint` null. No chain starts on the schema as shipped, whose foreign keys still
  remove audit rows by cascade, so every organization reads `notStarted` for now. `latestCheckpoint`
  is null until chain checkpoints are written.

### Fixed — repeated capability-check alerts are counted on one alert, and other follow-ups (#637)

- Repeated requests to `POST /api/v1/agents/:id/verify-capability` from one agent for the same capability and resource
  are counted on one open alert instead of creating an alert per request. This covers the "Unregistered Capability Used
  (Monitoring Mode)" alert, raised in monitoring mode for an agent with no granted capabilities, and the "Capability
  Violation Detected" alert, raised when an agent attempts a capability outside its grants. Three such requests in a
  monitoring-mode organization listed three alerts, each with an `occurrenceCount` of 1; they now list one alert with
  an `occurrenceCount` of 3. The trust score for the same requests is unchanged: the security-alerts factor deducts
  for each occurrence an unacknowledged alert counts, so one alert with an `occurrenceCount` of 3 deducts what three
  alerts deducted.
- A request that a strict-mode organization denies because the agent has no granted capabilities raises a "Capability
  Violation" alert. The check that raises it compared the denial reason with text the server never returns, so no
  alert was written. Repeats of the denial are counted on that alert, and it is kept apart from the "Capability
  Violation Detected" alert: once the agent is granted a capability, a request outside its grants raises that High
  alert, as it did before. The new alert does not change the trust score: the security-alerts factor already deducts
  for the denial through its capability-violation record, and it does not deduct for this alert.
- A repeat that arrives while an administrator acknowledges the alert it would be counted on is stored as a new alert.
  The repeat could be counted on the alert after the acknowledgment, and no new alert was created.
- `POST /api/v1/agents/:id/atc` answers 404 for another organization's agent, as it does for an ID that matches no
  agent and as `GET /api/v1/agents/:id` does. It answered 403, which told the caller that the agent exists.
- Migration 128 sets the comment on `isolation_attestations.verified` to the text migration 108 sets now. A database
  that applied 108 before its comment was edited kept the old comment.
- On a screen narrower than 640px the alerts page keeps each alert card inside the screen: the Acknowledge or Resolve
  button sits under the alert's text instead of beside it, and a long word in a title wraps inside the card. At 375px
  the button was drawn past the card's right edge and the page scrolled sideways.
- The alert panel and its backdrop start at the top of the screen. They started 24px down, so the top of the page
  stayed undimmed and the end of the panel fell below the screen.

### Fixed — an alert's repeat count is in the alerts API and dashboard, and concurrent repeats of one dedupe key share one alert (#633)

- Alerts returned by `GET /api/v1/admin/alerts`, `GET /api/v1/security/alerts` and
  `GET /api/v1/agents/:id/alerts` carry `occurrenceCount` (1 for an alert raised once) and `lastSeenAt` (`null`
  until a repeat arrives). The column existed, but no response included it, so a coalesced repeat was invisible. The
  dashboard's alert list shows "Seen N times" with the last time, and the alert panel shows the occurrence count and
  the last-seen time, for an alert raised more than once.
- On a narrow screen the alert list wraps the row that holds "Seen N times" inside the alert card. The row did not
  wrap, so at a phone width of 375px the repeat count was drawn outside the card and widened the page.
- Concurrent first occurrences of one dedupe key in one organization, with no open alert for that key, are stored as
  one alert that counts each of them. The lookup for an open alert and the insert ran as two separate statements, so
  each occurrence could find no alert and insert its own, and later repeats were counted on the newest of them only.
  They now run in one transaction under a lock on the organization and key.
- The migration that adds alert coalescing is renumbered from 126 to 127, so it no longer shares a number with another
  migration, and it removes the `schema_migrations` row of its old name on a database that applied it before the
  rename. A test fails when a new migration reuses a number.

### Changed — repeats of an alert that carries a dedupe key are counted on one open alert

- An alert created with a dedupe key is counted on the newest unacknowledged alert in its organization with the same
  key, when that alert was created less than 10 minutes earlier: that alert's `occurrence_count` goes up and
  `last_seen_at` records the latest repeat (migration 127 adds the `dedupe_key`, `occurrence_count` and `last_seen_at`
  columns). Acknowledging the alert ends this, and the next repeat creates a new alert. An alert without a dedupe key
  is stored as a new alert, as before. The alerts the capability check behind
  `POST /api/v1/agents/:id/verify-capability` raises for a capability the agent was not granted carry a dedupe key
  (see the entry for #637).

### Fixed — the Java SDK's token recovery carries the access token the route requires, and other follow-ups (#625)

- After the server refuses a refresh, the Java SDK's `AIMClient` sends its request to
  `POST /api/v1/auth/sdk/recover` with the access token it holds in the `Authorization` header. The route is mounted
  behind the auth middleware and mints only for the bearer who owns the revoked refresh token, so a request without
  one was answered 401 before its body was read, and the recovery could not succeed with the `oldRefreshToken` key
  corrected either. A client that holds no access token, because no earlier refresh succeeded, sends no recovery
  request. `AIMClientSdkRecoverRouteTest` reads the route's registration from the backend and answers a recovery
  request without the held bearer with 401.
- The Java SDK's `AIMClient.registerMcp` adds the given MCP server to the agent's list with
  `PUT /api/v1/agents/{id}/mcp-servers`, sending `mcpServerIds`, `detectedMethod` and `confidence`, the members
  `AddMCPServersRequest` declares. It posted `mcp_server_ids` and `detected_method` to
  `POST /api/v1/sdk-api/agents/{id}/mcp-servers`, the route that creates an MCP server and reads neither key, so the
  server was never added to the agent's list. `AIMClientRegisterMcpRouteTest` reads the request members and the
  route's member role gate from the backend source.
- Migration 126 deletes the `schema_migrations` row `112_drop_unread_password_reset_expires.sql`. A database migrated
  before that migration was renumbered to 125 kept the row after applying 125 again, as a record of a file the
  repository no longer ships. No other row is touched, and the row of
  `112_trust_score_low_threshold_key_and_scale.sql` stays. A test runs the server's migration step over a database
  holding the old row and fails while it remains.
- `docs/guides/QUICK_START.md` says to start the backend under test with `ENVIRONMENT=test` before running the
  integration suite. It set the variable on the `go test` process, which does not change the backend's limits, so a
  backend started with the `development` that `docker-compose.yml` sets by default answered the suite's repeated
  `/api/v1/public/login` and `/api/v1/public/register` calls with 429 after ten in a minute.
- `sdk/typescript/vitest.config.ts` sets `minWorkers: 1`, so `npx vitest run --maxWorkers=N <file>` runs without a
  matching `--minWorkers`. vitest 1.6 otherwise starts one worker per CPU but one, so on a machine with more than
  N + 1 CPUs it stopped the run before any test with `options.minThreads and options.maxThreads must not conflict`.
- The doc comment on the ATC issuance checks names `atx-spec` as the repository that holds
  `schemas/atx-credential-v1.1.schema.json`, and the logo IDAT entry below no longer cites a pattern that is not in
  this repository.

### Fixed — a handler reached without an organization answers 401 instead of panicking, and other follow-ups (#617)

- 59 handler lines in the A2A, admin, compliance, MCP, security, security policy, trust score and webhook handlers
  read the caller's organization with `c.Locals("organization_id").(uuid.UUID)`, which panics when the
  request carries no organization. They now read it through `RequireOrganizationID` and answer its 401. A test parses
  every handler source file and fails on a single-value assertion of `c.Locals("organization_id")`, with a planted
  control it must catch, and four of the handlers are called without an organization and must answer 401.
- The automatic suspension for a critical trust score reads the agent status transition table itself instead of
  relying on its caller's status check, and `MarkAsCompromised` leaves an agent that is already suspended or revoked
  alone in its UPDATE. Neither path reaches a revoked agent today; a new caller of either can no longer move a revoked
  agent to `suspended`, from where `POST /api/v1/agents/:id/reactivate` accepts it.
- `GET /api/v1/analytics/agents/activity` answered a failed activity query with `"note": "Activity data unavailable: "`
  followed by the database error's text in a 200 body. The error now goes to the server log and the note reads
  `Activity data unavailable: the activity query failed`.
- The expired password reset sweep matched rows whose `password_reset_token` was already NULL when a past
  `password_reset_expires_at` was left on them, counted them as cleared tokens and re-stamped their `updated_at`. It
  now matches only rows that hold a token.
- `aim-server migrate` with `POSTGRES_HOST`, `POSTGRES_USER` or `POSTGRES_DB` unset ended in a Go panic trace. It now
  exits 1 with one line that names each missing variable; the server reports the same error from `config.Load`.
- The refusal of a shared signing key named the active variables when the shared key came from a `_RETIRED` list. It
  now names the variable that contributed each copy, and a `_RETIRED` list that names one key twice is reported as
  such.
- `docker-compose.yml` passes `AIM_SIGNING_KEY_CARD_ATTESTATION_RETIRED` and `AIM_SIGNING_KEY_ATC_ISSUER_RETIRED` to
  the backend, so a rotation that lists the old key there publishes it in a compose deployment. A test fails when the
  backend service does not pass a variable the signing key loader reads.
- Stopping the A2A nonce cleanup job cancels a cleanup in flight and returns once the job's goroutine has exited, so
  its stop line can no longer reach a later test's captured log.
- The agents list took the deleted-agent notice out of `sessionStorage` on mount and then lost it when the list
  failed to load. The error view now shows the notice.
- `docs/DEPLOYMENT.md` documents `aim-server migrate`: CREATE on schema `public` is revoked from `PUBLIC` as well as
  from the application role, the role's password reaches the server log when `log_statement` is `ddl` or `all`, and
  every instance of an earlier release must be stopped before the first instance of a release with the v2 agent
  private key format starts.
- Tests added for refusals that had none: an audit record whose `sig_algs` is empty and a genesis at sequence 1, a
  served card attestation with another issuer, algorithm, `issuedAt` or `expiresAt`, an expired card attestation in
  the A2A verification result, and the observed verification event sources being exactly `service` and `system`. The
  key vault tests stop on a missing error instead of panicking on `err.Error()`.

### Fixed — the Java SDK's token recovery sends the key the server reads

- After the server refuses a refresh, the Java SDK's `AIMClient` sends the old refresh token to
  `POST /api/v1/auth/sdk/recover` under `oldRefreshToken`, the key the handler reads. It sent
  `old_refresh_token`, so no recovery request could succeed. `AIMClientSdkRecoverRouteTest` reads the key from the
  handler source and fails when the client sends another. The route also requires an access token for the refresh
  token's owner, and the request carried none until the #625 entry above.

### Changed — verification event failures are written to the server log with a timestamp

- A verification event refused because its agent could not be read, and a failed drift detection while recording
  one, are written through the standard logger like the rest of the server's log, so each line carries the time.
  They were printed to standard output without one.

### Changed — the FGA observability docs and demo dashboards describe Step 5 as it runs now

- `apps/backend/docs/OBSERVABILITY.md` no longer lists `DENY_INTENT` among the outcomes the engine produces or a
  `fga.intent_check_sync` span; it names `DENY_INTENT` as a historical value that `access_attestations` rows written
  before Step 5 stopped denying may still carry. The span tree, the TraceQL and PromQL examples and the
  `fga.intent_checks` notes describe the detached check for HIGH and MEDIUM risk. The demo Grafana dashboards no
  longer offer `DENY_INTENT` as an outcome, and the demo signal generator emits `fga.intent_check_async`.

### Changed — the migration that drops `users.password_reset_expires` is numbered 125

- `112_drop_unread_password_reset_expires.sql` shared its prefix with `112_trust_score_low_threshold_key_and_scale.sql`
  and is now `125_drop_unread_password_reset_expires.sql`. Both runners key a migration by its full file name, so a
  database that applied it as 112 applies it again as 125, where both of its statements are `IF EXISTS` and change
  nothing.

### Fixed — a dashboard build no longer leaves a clean checkout modified

- `apps/web/next-env.d.ts` is no longer tracked. Next.js rewrites it on every `next build`, `next dev` and
  `next typegen`, and 16.3.6 adds an import of `.next/types/root-params.d.ts` to it on build, so a clean checkout
  showed the file as modified after one build. `.gitignore` names it, as the Next.js TypeScript documentation
  advises, and `npm run type-check` in `apps/web` runs `next typegen` first, so the typecheck still has the file on
  a fresh checkout.

### Changed — an agent's trust credential carries its active capability grants, not the list reported at registration

- `POST /api/v1/agents/:id/atc` asked the Registry to sign the capability list the SDK reported when the agent was
  registered. That list was never updated: a capability revoked later stayed in every credential issued after the
  revocation, and a capability granted later was missing from all of them. The credential's `capabilities` are now the
  agent's active grants at `issuedAt`, read from the same table the capability verification path authorizes against:
  every grant not revoked and not a honeytoken, deduplicated and sorted, so two issuances of an unchanged grant set carry
  identical bytes under the signature. A grant whose type is outside the credential schema's `namespace:operation`
  grammar is left out and counted in the server log; it is never rewritten and never fails the issuance. An agent with
  no active grant is credentialed with `"capabilities": []`, never with the field absent or null.
- Contract: a credential's `capabilities` are the agent's active grants at `issuedAt` and are bounded by `expiresAt`; a
  holder's copy is at most that long out of date. Each call to the route issues a fresh credential, so a consumer that
  needs the current set re-fetches, and a consumer that needs the live answer calls AIM's capability verification, which
  reads the grant table directly.
- The agent card's AIM extension (`capabilities` under `aim`) and the A2A request verification result served the same
  registration-time list; both now serve the active grants, so they read the same list as the credential. When the grant
  table cannot be read, the card request and the verification fail; the registration-time list is never served in its
  place.
- The `agent` object in the responses of `POST /api/v1/agents/:id/suspend`, `POST /api/v1/agents/:id/reactivate` and
  `POST /api/v1/agents/:id/revoke` was the stored agent row, so its `capabilities` was the registration-time list. It is
  now the agent as `GET /api/v1/agents/:id` returns it, with `capabilities` read from the active grants. That object
  does not carry `lastCapabilityCheckAt`, `lastHeartbeat` or `keyRotationGraceUntil`, so those three fields are no
  longer in these three responses.
- The list the SDK reported at registration is still stored on the agent and is unchanged; it is a self-reported
  declaration, and no surface serves it under the name `capabilities` any longer.
- `apps/backend/internal/application/atc_capabilities_test.go` fails when a credential issued after a revocation still
  names the revoked capability or lacks one granted since, when two issuances of the same grant set differ, when a
  honeytoken or revoked row appears, when a grant outside the grammar is rewritten or fails the issuance instead of
  being counted, when an agent with no grant is sent a null or absent `capabilities`, or when a failed grant read lets
  the issuance proceed.
- `apps/backend/internal/interfaces/http/handlers/agent_act_response_capabilities_test.go` fails when the suspend,
  reactivate or revoke response's `agent.capabilities` is anything other than the agent's active grants.

### Changed — a suspended agent's refused signed action-request statement is recorded in the audit log

- A signed action-request statement (a body naming `signedBytes`) sent to `POST /api/v1/sdk-api/verifications` or
  `POST /api/v1/verifications` by an agent whose status does not permit it to act, such as a suspended or revoked
  agent, is refused with `401` and reason code `agentStatusDenied` in strict and in monitoring mode alike. The refusal
  wrote nothing. It now writes the same `audit_logs` row as the plain body form's refusal: action
  `verification_refused`, resource type `agent_action`, the agent's id, and metadata `refusal` (`agent_status`),
  `agentStatus`, `enforcementMode`, `capability` (the statement's `action_type`), `resource` and `httpStatus` (`401`).
  The row never carries the signature, the public key, the signed bytes, the nonce or the signed context.
- The row is written only after the signature verifies against the agent's registered key and the nonce is admitted,
  so a caller holding only an agent id cannot create one, and a replayed statement (refused as `nonceReused`) or one
  outside the time window writes nothing. The answer is unchanged, and a failed audit write still refuses with `401`.
- `apps/backend/internal/interfaces/http/handlers/verification_statement_refusal_audit_test.go` fails when the
  refusal writes no row or more than one on either route in either mode, when the row carries another metadata key, a
  key-shaped value or the signed context, when a statement that failed authentication, fell outside the window or was
  replayed writes a row, or when a failed audit write changes the answer.

### Changed — a suspended agent's refused verification request is recorded in the audit log

- `POST /api/v1/sdk-api/verifications` (and `POST /api/v1/verifications`, the same handler) refuses an agent whose
  status does not permit it to act, such as a suspended or revoked agent, with `403` and
  `Agent status is <status>, cannot perform actions`, in strict and in monitoring mode alike. The refusal wrote
  nothing, so an organisation that suspended an agent could not see that the agent kept trying. A request in the
  `agentId`/`signature`/`publicKey` body form that is refused this way now writes one `audit_logs` row with the action
  `verification_refused`, resource type `agent_action` and the agent's id, and metadata `refusal` (`agent_status`),
  `agentStatus`, `enforcementMode` (`strict`, `monitoring`, or `unknown` when the organisation does not load),
  `capability`, `resource` and `httpStatus` (`403`). The row never carries the signature, the presented public key,
  the signed message or the request context.
- The row is written only after the signature verifies against the agent's registered key, so a caller holding only
  an agent id cannot create one; a different key or a bad signature is still answered `401` and writes nothing. The
  answer is unchanged, and a failed audit write still refuses with `403`.
- `apps/backend/internal/interfaces/http/handlers/verification_refusal_audit_test.go` fails when the refusal writes no
  row or more than one in either mode, when the row carries another metadata key or a key-shaped value, when a request
  that failed authentication writes a row, or when a failed audit write changes the answer.

### Changed — Terraform saved plans and local state are kept out of the repository

- The repository-root `.gitignore` now ignores `tfplan`, `*.tfplan`, `*.tfstate` and `*.tfstate.*`. A saved
  plan or a local state file holds resource attributes in clear text, including generated passwords and keys,
  and a `terraform plan -out tfplan` run anywhere in the tree used to leave it ready for a normal `git add`.
  None is tracked today. A test in `apps/backend/cmd/server` fails if one is committed or if a rule is
  removed, and evaluates the rules with any personal global excludes file switched off.

### Fixed — an issued trust credential is returned only when it carries both signature suites and the values AIM asked for

- `POST /api/v1/agents/:id/atc` returned whatever credential the issuer signed. OpenA2A AIM (Agent Identity Management)
  hands on the issuer's signed bytes verbatim, so a credential signed with Ed25519 alone, or one stating another score
  or a higher trust level than AIM requested, reached the caller as AIM's credential. ATX core section 1.1 says the
  signature block carries at minimum one Ed25519 signature and one ML-DSA-65 signature.
- AIM now refuses the issuer's credential, and the route answers 502 as it does for any failed issuance, when it has
  no Ed25519 signature, no ML-DSA-65 signature, or a signature in any other suite (the ATX credential schema admits
  only Ed25519 and ML-DSA-65 as `signatures[].algorithm`, and they are the only suites ATX core section 14 registers);
  when it names another agent; when its `trustScore` differs from the 0-100 score AIM sent in the six-digit form the
  v1.1 signature covers; or when its `trustLevel` is higher than the level AIM requested. A lower level is accepted.
  Signature validity is still checked by the relying party's verifier, not here.
- Issuance through an issuer that signs with Ed25519 only, or reads `trustScore` on the 0-1 scale, now fails until the
  issuer signs with both suites and reads the 0-100 scale.
- `apps/backend/internal/application/atc_issuance_service_test.go` fails when the service returns a credential with
  either suite missing, an unregistered suite, another agent, a changed score or a raised level, and checks the hybrid
  and Ed25519-only atx-conformance credentials vendored under `sdk/java/src/test/resources/atx-fixtures/`.

### Fixed — an issued trust credential asks for no trust level above what one signing authority may assert

- `POST /api/v1/agents/:id/atc` asked the credential issuer for trust level 3 when the agent's behavioral score was
  0.75 or higher, and for level 4 at 0.90 or higher. One authority signs each credential OpenA2A AIM (Agent Identity
  Management) requests. ATX core section 12 says a conforming issuer must not assert level 3 or higher on a credential
  that does not carry signatures from at least two distinct authorities, nor level 4 without the root cosignature, so
  a credential issued from such a request broke that rule.
- AIM now asks for at most level 2: a score that maps to level 3 or 4 is requested as level 2. Levels 0 to 2 and the
  0-100 `trustScore` are unchanged, so the credential still carries the full behavioral score. A relying party or
  policy that requires level 3 or higher is not met by a credential AIM requests. A credential issued before this
  change keeps its level until it is reissued or expires.
- `apps/backend/internal/application/atc_issuance_service_test.go` fails when the issuance request asks for a level
  above 2.

### Fixed — the fleet governance guide's deployment step creates an account to sign in with

- Step 1 of `docs/use-cases/fleet-governance.md` set neither `ADMIN_EMAIL` nor `ADMIN_PASSWORD`. The server creates
  its first administrator only from `ADMIN_PASSWORD`, so the stack started with no user, and Step 5 sent the reader
  to a dashboard they could not sign in to.
- The step now writes `ADMIN_EMAIL` and a generated `ADMIN_PASSWORD` to `.env`, and the compose file passes both to
  the server. The password carries the upper- and lower-case letter, digit and special character the server
  requires. The step shows the log line that confirms the administrator was created, and Step 5 says to sign in
  with it and that the dashboard asks for a new password at first sign-in.
- A test evaluates the step's `.env` many times and checks that the server accepts each password, seeds the
  administrator from the step's environment, and logs the line the step shows.

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
  Stop every instance of the earlier release before the first new instance starts: once the migration is recorded it
  does not read the rows again, so a v1 key an earlier instance writes afterwards stays refused until rotated
  (`docs/DEPLOYMENT.md`, "Upgrading across the agent private key format change").
- Registration assigns the agent ID before it encrypts the generated key, and the agent repository keeps an ID the
  caller assigned instead of replacing it.
- Tests pin the additional data byte for byte and fail when a ciphertext copied from one row decrypts under another,
  when a ciphertext sealed for another purpose or a v1 ciphertext is accepted on the request path, when the migration
  re-binds a v2 ciphertext copied from another row, when a completed migration reads agent keys again, or when a
  planted key, its ciphertext or an agent ID appears in the migration log or a refusal error.

### Security — a check fails on token-shaped literals in tracked files

- `node scripts/lint-token-literals.mjs` reads every file `git ls-files` returns, binary files included, and fails on
  a three-segment base64url token (the JWT shape), a private key block, an AIM API key, an AWS, GitHub, Slack, Stripe,
  Google or `sk-` prefixed key, or a credential-named field assigned a high-entropy string. It prints the path, line,
  rule and a fingerprint of each finding, never the matched text.
- Each run also scans a synthetic file it plants in a temporary directory, carrying one sample per rule. If any sample
  is missed, the run reports INCONCLUSIVE and exits 2 instead of passing. A tracked path it cannot read, or an empty
  file list, also exits 2.
- Literals reviewed as non-credentials are listed, each with a reason, in `scripts/token-literal-allowlist.txt`. An
  entry covers one literal at one path, and an entry that no longer matches fails the run.
- The SDK token tracking middleware test builds its JWT at run time instead of storing one, and the Python scripts
  under `tests/scripts/` read the API key from `AIM_API_KEY` instead of carrying a key issued by a local backend.
- `scripts/test-lint-token-literals.mjs` (`node --test scripts/test-lint-token-literals.mjs`) fails when a tracked
  token is not reported or is printed, when a missed planted sample or an unreadable path does not make the run
  inconclusive, or when this repository's tracked tree has a finding.

### Fixed — The logo SVG's embedded PNG is split into one more IDAT chunk

- The PNG embedded in `apps/web/public/opena2a-logo.svg` carries one more IDAT chunk, split inside the one run of its
  base64 text that reads, ignoring case, as an internal directory name followed by `/`.
  `scripts/lint-public-surface.mjs` blanks data-URI payloads before matching, so it does not report the run, and its
  built-in `local-home-path` class does not match the unsplit file either. The concatenated IDAT bytes and every other
  chunk are byte-identical, so the decoded pixels are unchanged.

### Added — the security policies page shows the trust score a low-trust policy triggers below

- On `/dashboard/admin/security-policies`, the card of a `trust_score_low` policy carries a
  `Trust score below:` line with the threshold the evaluator applies to that policy, as the
  percentage the dashboard shows trust scores in. With the rules migration 015 seeds, 'Critical
  Trust Score Block' reads `50%` and 'Low Trust Score Alert' reads `70%`.
- When the rules carry no numeric `trust_threshold`, the line reads
  `30% (default: these rules set no numeric trust_threshold)`. The evaluator reads no other key,
  `threshold` included, and applies 0.3 in that case, so the line names the key to set.
- A `trust_threshold` above 1 is shown as stored, with what it does: `{"trust_threshold": 70}` reads
  `7000% (trust_threshold 70 is above the 0-1 scale: every score is below it)`. A value of 0
  reads `0% (no score is below it)`, and a negative value carries the same note.
- Measured on the page before this change, with two `trust_score_low` policies that differed in
  name and rules only: one with `{"trust_threshold": 0.5, "auto_disable": true, "notify": [...]}`,
  which the evaluator applies at 0.5, and one with
  `{"threshold": 50, "auto_disable": true, "notify": [...]}`, which it applies at 0.3. Both cards
  showed the same details row, `Applies to: all`, `Severity threshold: critical` and
  `Rules: 3 configured`, and nothing worked out from the values in the rules.
- The dashboard works the line out from the policy's `rules` with its own copy of the evaluator's
  key and default. A backend test reads that copy and fails when either differs from the
  evaluator's constants. No API answer and no stored policy changes.

### Security — the rate limits no longer loosen when ENVIRONMENT is development

- The general rate limit (100 requests per minute per user, or per client IP when
  unauthenticated) and the strict limit (10 per minute) now apply when `ENVIRONMENT=development`.
  The strict limit covers `/api/v1/public/*` (login, registration, password change and reset,
  access requests, agent registration), `/api/v1/auth/*`, `/api/v1/oauth/*` (token and the
  device-code flow) and bootstrap-token mint and exchange. Under `development` both used
  to be ten times higher (1000 and 100), and `docker-compose.yml` and
  `docker-compose.quickstart.yml` give the backend `development` unless the shell sets
  `ENVIRONMENT`, while `docs/demo/stack/compose.yml` always does.
- Only `ENVIRONMENT=test` still raises both limits tenfold. The CI end-to-end job sets it; never
  set it in a deployment. A local script that signs in more than ten times a minute against a
  `development` server now receives 429; run it with `ENVIRONMENT=test`.
- The server logs the limits in force in one line at start, in every environment, for example
  `Rate limits in force: general 100/min, strict 10/min, per user or per client IP.` It used to
  log only when the limits were raised.
- The compose files are unchanged, and nothing else that reads `development` changes (the key
  generated at start when `KEYVAULT_MASTER_KEY` is unset, the five-minute detection
  deduplication window).

### Security — a reused SDK-download token ends the chain that grew from it

- Presenting an SDK-download refresh token (the 90-day token embedded in a downloaded SDK) that
  was already rotated out now ends the chain that grew from it, as a reused login refresh token
  ends its sign-in (RFC 9700 section 4.14.2). Every token issued from that download by rotation is
  refused from then on: the family is revoked in the revocation store, and every `sdk_tokens` row
  of the download is revoked with reason `token_family_revoked`, so the chain also ends where no
  revocation store is configured. The event is recorded as a login reuse is
  (`refresh_token_reuse`, then `refresh_session_revoked` for each later member refused), with
  identifiers only. A rotated-out SDK token used to be refused on its own while the token that
  replaced it stayed valid.
- Two presentations of one SDK-download token in the same instant no longer both rotate. The
  `sdk_tokens` retirement matches only an active row. When a presentation's rotation does not
  take effect, its row is read again, and a row that another presentation retired by rotation in
  the meantime makes it a reuse: it is refused, receives no tokens, and any row it created is
  revoked with the family. It used to receive a second live token.
- `POST /api/v1/auth/sdk/recover` refuses a token whose chain a reuse ended, with the refresh
  route's 401 answer; the SDK is downloaded again instead. A token its owner logged out
  (`POST /api/v1/auth/logout`) is refused the same way, with or without a revocation store. A token
  its owner revoked from the dashboard (`POST /api/v1/users/me/sdk-tokens/{id}/revoke`) is recovered
  where no revocation store is configured; where one is, the revoke writes the family key and the
  route refuses the token.
- SDK-download tokens carry the registered `sid` claim naming their download (the downloaded
  token's own id, copied on every rotation), and the access tokens minted from them carry it too,
  so the SDK download, device approval and SDK recovery routes refuse an access token from an
  ended chain. A download's family never shares an id with the sign-in it was downloaded from.
  `POST /api/v1/auth/logout` with an SDK-download token revokes that download's family as it does
  a sign-in's. A family's `sdk_tokens` rows are found from the download's row (its `token_id` is
  the family's id) through the `parent_token` entry each rotation records; no row gains a field,
  and there is no migration. A token downloaded before this change is the root of its own family
  and carries it on its first rotation. Tokens are opaque to clients; no SDK change is needed.

### Changed — breaking: `/metrics` moves off the API port to its own listener, loopback by default

- What to check before upgrading: a Prometheus job that scrapes `/metrics` on port 8080 or through the
  dashboard without a token stops receiving data. To scrape from the same host, target
  `http://127.0.0.1:9464/metrics`. To scrape from another host, set `METRICS_AUTH_TOKEN`
  (`openssl rand -hex 32`) and give Prometheus the token with `authorization` and `credentials_file`.
- The backend serves `GET /metrics` on a dedicated listener at `METRICS_LISTEN_ADDR`, default
  `127.0.0.1:9464`, which serves nothing else. On a loopback address it needs no token. Any other
  address (`0.0.0.0:9464`, `:9464`, a hostname, `localhost` included) requires `METRICS_AUTH_TOKEN`;
  without it the backend refuses to start and names both settings.
- Without `METRICS_AUTH_TOKEN` the API port has no `/metrics` route and answers 404. With it, the API
  port and the dedicated listener both serve `/metrics` behind `Authorization: Bearer <token>`, and
  each request to the API port's `/metrics` is logged with its status and the connection's peer
  address. No setting restores an open `/metrics` on the API port.
- A `METRICS_AUTH_TOKEN` shorter than 32 characters is treated as unset and logged as an error. The
  token is compared by SHA-256 digest, so response timing reveals neither its content nor its length.
- If the dedicated listener cannot bind, the backend logs an error naming `METRICS_LISTEN_ADDR` and
  keeps serving the API without metrics; it never falls back to the API port. At startup the backend
  logs one line per metrics surface, with its address and whether it requires the token, and one
  line describing this change.
- The dashboard no longer proxies `/metrics` to the backend; the dashboard origin answers 404 there.
- `docker-compose.yml` sets `METRICS_LISTEN_ADDR=0.0.0.0:9464` inside the backend container without
  publishing the port, requires `METRICS_AUTH_TOKEN` in `.env` (`./scripts/gen-dev-secrets.sh` now
  emits one), and hands it to Prometheus as a compose secret. `infrastructure/monitoring/prometheus.yml`
  scrapes `backend:9464` with that token instead of `host.docker.internal:8080`.
- The `aim_trust_score{agent_id,agent_name}` and `aim_mcp_attestations_total{agent_id,mcp_id,status}`
  families are removed, so no metric carries an agent, MCP server, organization or user identifier.
  `aim_trust_score_distribution` remains. A test fails when a label name outside the reviewed set,
  or one that identifies an entity, is registered.

### Security — `/metrics` was readable without authentication on the API port and the dashboard origin

- Since 2025-11-21 (7d9e157d) the API port served `/metrics` to anyone who could reach it, and since
  2026-04-13 (2c6ba110, #101) the dashboard origin proxied it as well. Both are in `platform-v1.0.0`,
  which has no option to require a token. The response lists the API's endpoints with request counts,
  statuses and latencies, and a request path a caller sent could appear in it as a label value.
- Builds from 2026-08-04 (#349) accept `METRICS_AUTH_TOKEN` but leave `/metrics` open when it is
  unset. Operators who cannot upgrade yet should set `METRICS_AUTH_TOKEN` on such a build, or on
  `platform-v1.0.0` and earlier block `/metrics` at their proxy on both the API origin and the
  dashboard origin. The API did not log requests to `/metrics` on those builds, so its own logs
  cannot show whether a deployment's metrics were read.

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

### Fixed — the fleet governance guide's deployment step starts the server

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

### Removed — the second backend Dockerfile; the quickstart test builds the published one

- `apps/backend/infrastructure/docker/Dockerfile.backend` was a second backend Dockerfile, built only by
  `scripts/test-quickstart.sh`. Its runtime stage took `alpine:latest` rather than the published image's
  `alpine:3.21`, it built the server with cgo and stamped no version, so the quickstart test ran an image no release
  ships. It is removed, and `scripts/test-quickstart.sh` builds `infrastructure/docker/Dockerfile.backend`, the file
  the published `aim-server` image is built from. `scripts/check-go-toolchain.sh` checks the builder tag of that one
  Dockerfile.
- `sdk/typescript/tests/dockerfile-backend-single-source.test.ts` fails when a Dockerfile other than the one
  `docker-publish.yml` builds compiles `./cmd/server`, when the quickstart test builds another file, or when any
  Dockerfile in the tree takes a base with no digest and no tag or the tag `latest`.

### Fixed — the token endpoint reports the lifetime of the token it issues

- `POST /api/v1/oauth/token` answered `"expires_in": 3600` while the token it returned lived for the configured
  access-token lifetime, `JWT_ACCESS_TTL`, 2 hours by default. A client caching by `expires_in` dropped a valid token
  at half its life, and with a lifetime under an hour kept presenting a token the server had already expired.
  `expires_in` is now the lifetime stamped into the token.
- The TypeScript SDK requests the JWT-bearer grant this endpoint accepts, at the path it is served on; see
  `sdk/typescript/CHANGELOG.md`.

### Security — an agent credential acts only on its own agent ID on the SDK-API, detection and A2A agent and card write routes

- The `/api/v1/sdk-api/agents/:id/...` routes for heartbeat, capabilities, capability requests, MCP servers,
  connections and usage, and the detection report checked only that the agent in the path belonged to the caller's
  organization, so one agent's key could report for any other agent in it. The isolation attestation route and its
  `/isolation` alias compared the caller's agent ID with the path, but only after looking the agent up, so another
  agent's ID was answered differently from an ID that names no agent. When the caller authenticates as an agent, a
  path ID other than its own is now refused `403 {"reasonCode":"agent_path_mismatch"}` on each of these routes
  before any lookup, and another agent's ID gets the same response as an ID that names no agent. Requests made with
  a user token are unchanged.
- `GET /api/v1/sdk-api/agents/:identifier` accepts, from an agent caller, only its own ID or its own name, and
  resolves the name from the caller's own record. The lookup by name the SDKs make with a user token during setup is
  unchanged.
- The same check applies to the four `/api/v1/detection/agents/:id/` routes, to
  `POST /api/v1/a2a/agents/:id/card`, `.../card/refresh`, `.../sign` and `.../trust-score/compute`, and to the A2A
  card aliases `POST /api/v1/a2a/cards/:id/attestation`, which refreshes the same card attestation as
  `.../card/refresh`, and `PUT /api/v1/a2a/cards/:id`. Both aliases checked the organization only.
- `POST /api/v1/a2a/cards` takes `agentId` from the request body and checked neither the caller nor the organization,
  so any authenticated caller could register a card for any agent. An agent caller naming another agent there now
  gets the same `403 {"reasonCode":"agent_path_mismatch"}`, before any lookup, and every caller naming an agent
  outside its own organization gets `404 {"error":"not found"}`.
- The server refuses to start with an SDK-API `/agents/:` route that declares neither the check nor a reason it is
  exempt, and a test walks the backend source for every route under a group that authenticates agents whose path
  has a parameter directly after an `agents`, `cards` or `trust` segment or one named `agentId` or `agent_id`, and
  requires each to be bound, an exception with a reason, or a dated hold. The remaining `/api/v1/agents/:id/...`
  routes and the A2A read routes that name an agent (`/agents/:id/...`, `/attestations/:agentId/:skillId`,
  `/consensus/:agentId/:skillId`, `GET /cards/:id` and `GET /trust/:id`) are enumerated there and are unchanged in
  this release. Two A2A routes are admitted exceptions: `PUT /api/v1/a2a/trust/:id` answers 405 to every caller, and
  in `POST /api/v1/a2a/trust/:id/interaction` the ID names the peer the calling agent interacted with. An agent ID in
  a request body is outside the walk.

### Added — request records show how the caller authenticated and whether it used its own agent ID

- Each `api_calls` row now records `auth_method` (`ed25519`, `mldsa`, `hybrid`, `api_key`, `atc` or `service`; empty
  for a user session). On a route that checks its agent ID against the caller, the row also records
  `agent_path_route` (the matched route) and `agent_path_outcome` (`self`, `other`, `no_principal` or `fault`).
  Migration 117 adds the three columns.
- The `/api/v1/agents/:id/...` routes and the A2A read routes listed above that do not yet apply the check now
  record that comparison without changing any response. This counts, per route, the agent callers that name another
  agent's ID, before the check is applied there. `apps/backend/scripts/agent_path_binding_report.sql` reports the
  count per route over the last 14 days, with the auth method and user agent of each such caller.

### Security — agent-signature authentication reads an agent's status only after the signature verifies

The PQC and Ed25519 agent-signature middlewares, the OAuth jwt-bearer token endpoint, `POST /api/v1/sdk-api/verifications` and `GET /api/v1/sdk-api/verifications/:id` read the agent row before checking the request's signature and answered from it. A caller holding only an agent id could tell an unknown agent from one with no key or a different key, and could read whether the agent was suspended or revoked. They now read only the agent's registered keys until the signature verifies. An unknown agent, an agent with no registered key, a registered key that does not decode to its algorithm's key length and a presented key that is not the registered one all get one 401, `{"error":"the signing key is not the registered key of a known agent","reasonCode":"agentKeyNotRecognized"}`. On the token endpoint those causes, and a signature that does not verify, get `invalid_client` with the `error_description` `the assertion is not signed by the registered key of a known agent`; a stored key that does not decode was a 500 there and is now this 401. In every case a stored key that does not decode writes one server log line naming the agent, never the key. The middlewares still count the four causes apart in `aim_s1_refusals_total` (`agent_lookup_failed`, `no_registered_key`, `registered_key_malformed`, `public_key_mismatch`), which only an operator reads; an ML-DSA key of another level than the request names is counted as `no_registered_key`. The status refusal, `Agent is not permitted to authenticate (status: <status>)`, now comes only after the signature verifies; the middlewares and `GET /api/v1/sdk-api/verifications/:id` send it with `"reasonCode":"agentStatusDenied"`, and the middlewares send a signature that does not verify with `"reasonCode":"signatureInvalid"`. `POST /api/v1/sdk-api/verifications` still answers an unknown agent 404. `GET /api/v1/sdk-api/verifications/:id` now refuses a revoked or suspended agent, which could read its own verification events before. Hybrid `Ed25519+ML-DSA-*` requests now verify: the ML-DSA half was checked against the hybrid algorithm's name, which no key matches, so every hybrid request was refused.

### Fixed — onboarding events are capped per organization, and a token mint is not retried on every error

- `POST /api/v1/onboarding/events` stores the same event (and, for `tab_selected`, the same tab) at most 20
  times per organization in 24 hours. A report past the cap is answered `202` with `"recorded": false`, so a
  signed-in member posting in a loop no longer grows the onboarding baseline's event totals without bound.
- `POST /api/v1/onboarding/bootstrap-tokens` runs its transaction a second time only when a concurrent mint by
  the same user took the one-open-token slot. Any other database error, such as the database being unreachable,
  fails the mint after one attempt.
- The bootstrap token exchange answer carries `dashboardUrl`, the configured `FRONTEND_URL`, so a client links
  to the registered agent on the dashboard instead of deriving the dashboard address from the API address.
- The dashboard reports `onboarding_viewed` once per sign-in in a browser tab. Signing out or in clears the
  marker, so when another account signs in on the same tab its first-run view is recorded too.
- The API reference says server-side onboarding events are written off the request path, not after the
  response: the write can finish before or after the request is answered.

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
- The finding renders below the row's title and badge at the full width of the activity row. Inside the title
  column, beside the badge, it had about 120px at a phone width of 375px, and its words were broken mid-word. The
  page test fails when the finding is rendered inside the title row again.

### Removed — the unread `users.password_reset_expires` column and its index

- The `users` table carried two password reset expiry columns. The reset flow reads and writes
  `password_reset_expires_at`; `password_reset_expires`, added later with the partial index
  `idx_users_password_reset_expires`, was never read or written and held only `NULL`s. Migration 125 drops the column
  and the index. It is a no-op on a database that does not have them, and leaves `password_reset_expires_at`,
  `password_reset_token` and `idx_users_password_reset_token` unchanged.
- `apps/backend/internal/infrastructure/repository/users_password_reset_columns_integration_test.go` fails when a
  migrated database still has the unread column or its index, or is missing the column the reset flow reads.

### Fixed — the dashboard asks for the signed-in user once per page load

- The dashboard shell, the sidebar, the header and the deactivation check each requested `/api/v1/auth/me` when a
  page loaded, and that route shares the strict rate limit of the sign-in route. A long enough run of page loads
  from one address was refused with 429, which left the sidebar without its account section and refused the next
  sign-in from that address. The client now sends one profile request for the callers that ask while it is on the
  wire; the next caller after it settles, or a caller with another session token, sends its own.

### Fixed — the quickstart stops when an earlier install's database is still on the machine

- After the install directory was deleted, `scripts/quickstart.sh` started a first run over the earlier
  install's `<dir>_postgres_data` volume. It generated a new `POSTGRES_PASSWORD`, which an already-initialised
  database ignores, and the run ended in "Backend did not become healthy" with no cause. If only the compose
  file had been deleted, it also overwrote the `.env` that held the one password the database accepts. The
  first-run path now checks for the volume before writing anything; when it exists, the script exits 1 and
  prints two choices: put the earlier `.env` and compose file back (keeps the data), or the exact
  `docker rm -f` / `docker volume rm` commands to start over (deletes it). The script never removes a volume.
  The health-check timeout now names `docker volume ls` for a volume the check does not find.

### Fixed — a request-analytics row that fails to insert is counted, not dropped without a trace

`AnalyticsTracking` writes each `api_calls` row after the response, and an insert that failed was discarded with no log line and no counter, so a low or zero API-call count could not be told apart from rows that were lost. A failed insert is now counted by error class: the two-character SQLSTATE class read from the driver's error code, `timeout`, `pool` (no connection before the deadline) or `other`. The server writes one line per 64-second period that had a failure, and the pending line on shutdown, in the form `api_calls_insert_failed period_start=<UTC> period_end=<UTC> <class>=<count> ...`. The line carries only the period and the counts: never the error's text, which can repeat the rejected value, and no endpoint, address, user agent or identifier from the row. An insert now has a 30-second deadline, the wait for a connection included, so one that cannot finish is counted rather than left waiting.

### Fixed — an agent that took part in A2A tasks, messages or consent records can be deleted

- `DELETE /api/v1/agents/:id` (Delete agent in the dashboard) failed with a 500 for any agent
  named in an A2A task, an A2A message or an A2A consent record: those five references to the
  agent had no delete rule, so the database refused the delete. Migration 120 makes them cascade,
  as the other per-agent A2A tables already do. Deleting an agent now also deletes the A2A tasks
  it was the client or remote agent of (with their messages), the A2A messages it sent, and the
  A2A consent records that name it as grantor or recipient. An A2A attestation between two other
  agents that cited one of those tasks is kept, with its task link cleared.
- When a delete still fails, `DELETE /api/v1/agents/:id` no longer answers with the database's
  own error text (for the case above, a foreign-key message naming tables and constraints). Its
  `error` now reads "The agent was not deleted and nothing was removed. Try again, and if it fails
  again, contact your administrator." The cause is written to the server log with the agent and
  organization IDs. The status stays 500.
- Delete agent in the dashboard opens the same dialog from the agents list and from the agent
  page. It says what the delete removes (the agent and its records, including its API keys,
  stored secrets, capabilities, trust and verification history, and the A2A tasks, messages and
  consent records it took part in), what is kept (audit log entries, API call records, A2A
  security violations and the MCP servers it registered), and that it cannot be undone. A failed
  delete keeps the dialog open and states the reason and a next step inside it, where screen
  readers announce it. As for the agent page's other actions, the reason comes from the response's
  status (a role that may not delete, an agent that no longer exists, AIM out of reach), never from
  the server's text. Before, the agent page closed its dialog and stated the failure beside its
  actions, and the agents list showed nothing. After a delete, the agents list states that the
  agent was deleted.
- In a development build of the dashboard (`next dev`), the agents list no longer drops the
  "was deleted" notice for a delete made from the agent page. React runs mount effects twice
  there, and the second run cleared the notice the first run had read.

### Fixed — the SDK authentication guide no longer says quantum attacks cannot break Ed25519

- `docs/sdk/authentication.md` listed "No Known Vulnerabilities" among Ed25519's strengths and
  said that, unlike RSA, it withstands quantum attacks. Ed25519, like RSA, is not resistant to
  quantum attacks: a large enough quantum computer running Shor's algorithm could recover a
  private key from its public key. The guide now says so and links to the Post-Quantum
  Cryptography guide for ML-DSA signatures.

### Fixed — MCP server verification compares what the attesting agents report

- An MCP server was marked verified once at least three agents created by at least two users had
  attested it and its confidence score reached 60. The rule counted attestations and did not read
  them: three agents reporting three different tool lists verified a server, and so did three
  agents that each reported a failed connection.
- Verification now also requires the attestations to agree: the latest attestation of every
  attesting agent must report a successful connection, and all of them must report the same tool
  set. Tool names are compared as a set, so order, repeats and the `tools`, `resources` and
  `prompts` category entries do not count as differences. Manual attestations are not agent
  reports and are left out, as they already were from the agent and owner counts.
- `GET /api/v1/mcp-servers/:id/consensus-status` reports `attestationsAgree`, `agreeingAgents`,
  `connectionFailures` and `distinctToolSets`, lists failed connections and differing tool sets
  under `missingCriteria`, and returns `consensusReached: true` only when the attestations agree.
- Manifest drift recording from attestations still starts at the agent, owner and confidence
  thresholds, so an attestation that reports a different tool set is recorded as drift rather
  than switching drift recording off.
- Servers verified before this change, and servers whose attestations disagree after they are
  verified, are covered by the entry on withdrawn consensus verification above.

### Fixed — an agent heartbeat stores the heartbeat time and writes nothing else to the agent

- `POST /api/v1/sdk-api/agents/:id/heartbeat` read the agent and then saved its whole copy of the
  row to record the heartbeat. That save has no `last_heartbeat` column, so no heartbeat time was
  ever stored, and it wrote status and key material back from the earlier read: a suspension or
  key rotation saved between that read and that write was overwritten with the values the
  heartbeat had read.
- A heartbeat now writes `last_heartbeat` and `updated_at` only, and the response reports the
  stored heartbeat time and the agent's status after the write.

### Fixed — an authorized capability check stores its time and writes nothing else to the agent

- The capability check service, which no route serves, stored the time of an authorized check by
  setting it on the agent it had read and saving that whole copy of the row. That save has no
  `last_capability_check_at` column, so the time was never stored, and it wrote status and key
  material back from the earlier read: a suspension or key rotation saved between that read and that
  write was overwritten with the values the check had read.
- An authorized check now writes `last_capability_check_at` and `updated_at` only, with the
  database's time. The check's result is unchanged, and a failure to store the time still does not
  refuse the check.

### Fixed — the backend image carries the license notices of the Go libraries built into it

- The `aim-server` image held the binaries, the migrations and the SDK directory and no license
  material for the third-party Go libraries linked into `aim-server`, `aim-migrate` and
  `aim-bootstrap`.
- The image now ships `/app/third_party/`: `licenses/<module path>/` holds each library's license
  and notice files, and for the ten MPL-2.0 HashiCorp modules behind the Vault client, the
  library's source as built in. `manifest.csv` lists every library with its license and a link to
  that license in the upstream repository at the version built in. `README` describes the layout.
- The image build generates the directory with `go-licenses`, pinned to the same version and
  checksum as the license check, and fails if any library lacks a license file or a link, or an
  MPL-2.0 library lacks its source.

### Fixed — the Java SDK documentation no longer states dependency versions its pom contradicts

- `sdk/java/README.md`, which ships in the SDK download, listed Jackson 2.16 and BouncyCastle 1.79,
  and `docs/sdk/java.md` listed Jackson 2.16 and BouncyCastle 1.77. `sdk/java/pom.xml` pins Jackson
  2.18.11 and BouncyCastle 1.85.
- Both documents now name each library and its purpose, and point at `sdk/java/pom.xml` for the
  version. A test in the Java SDK fails when either document states a version the pom contradicts.

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

### Fixed — webhook, MCP server and agent card requests follow no redirect and connect only to the addresses registration admits

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

### Fixed — a webhook delivery names the event it carries in `X-Webhook-Event`

- A delivery, each of its retries and a replayed delivery sent `X-Webhook-Event` with the first event the webhook
  subscribes to, not the event being delivered: a webhook subscribed to `agent.created` and `agent.deleted` received
  `X-Webhook-Event: agent.created` with an `agent.deleted` payload. The header now names the event in the payload's
  `event` field. The test send already named its own event, `webhook.test`.

### Fixed — an agent's activity returns no network address, user agent or personal metadata to a non-admin

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

### Fixed — the per-agent and per-MCP-server audit logs return no network address, user agent or metadata to a non-admin

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

### Fixed — the token endpoint accepts an assertion addressed to the server's address and port when `AIM_BASE_URL` is unset

- `POST /api/v1/oauth/token` requires the assertion's `aud` to name the server. It accepted `AIM_BASE_URL` and also
  the origin the request arrived on, which it built from the host name in the `Host` header without its port. With
  `AIM_BASE_URL` unset, a server reached at `http://localhost:8080` therefore refused `aud` `http://localhost:8080`,
  which is the value the TypeScript SDK's `OAuthTokenManager` signs when given that base URL, and accepted
  `http://localhost` instead.
- With `AIM_BASE_URL` unset, the accepted origin is taken from the `Host` header the client sends, and now keeps its
  port. `http://localhost:8080` and `http://localhost:8080/api/v1` are accepted on that server, and an `aud` naming
  the same host on another port or scheme is refused. The origin of a server on the scheme's default port is
  accepted as before. Set `AIM_BASE_URL` to have `aud` judged against the configured address instead.
- With `AIM_BASE_URL` set, only `AIM_BASE_URL` and `AIM_BASE_URL` followed by `/api/v1` are accepted, and the `Host`
  header is not consulted. It used to be, so a client that named another host in `Host` could have an assertion
  addressed to that host accepted. A client that signs `aud` with another address of the server is now refused
  there; it has to sign `AIM_BASE_URL`.
- Tests send the request to an address with a port and to one without, and check which audiences each accepts. They
  also send a `Host` header that names another host, with a port and without one, with `AIM_BASE_URL` set and unset.

### Fixed — `POST /api/v1/public/agents/register` answers 401 without a user access token, and the API reference says it needs one

- The route registers an agent only for a signed-in user: it takes the user and organization from a user access
  token and reads no other credential. A request with no credential, with an API key in `X-AIM-API-Key`,
  `X-API-Key` or `Authorization: Bearer`, or with an agent's OAuth access token from `POST /api/v1/oauth/token` as
  the bearer, used to run through key generation and an insert the database refused, and came back as 400
  `invalid organization or user for this registration`. It is now answered 401 before the body is read, with a
  message that names the two paths that register an agent: a user access token as `Authorization: Bearer <token>`
  on this route, or an API key in `X-API-Key` at `POST /api/v1/agents`. A signed-in caller is served as before.
- The dashboard's API reference listed the route as public with no auth required, so the request it built carried
  no token. Its example body also used `type` where the route reads `agentType`, and left out `displayName`. The
  entry now requires a bearer token and documents the four required fields, and the route's API description says the
  same.
- A client that sent an API key to this route and matched on the 400 now sees 401; the key is accepted at
  `POST /api/v1/agents`.

### Fixed — the Java SDK signs requests in the form the platform verifies, and its approval poll is signed

- `AIMClient` signed an `X-Signature` GET over `METHOD\npath\ntimestamp\n`, and a POST with an empty body the same
  way. The platform verifies over those three lines with no trailing newline and adds the body as a fourth line only
  when the request has one. Both now sign the form the platform verifies. The signed GET is the read of an agent's
  MCP servers made when an MCP server registration returns 409. These requests also carry the client's bearer
  token, and the platform does not check `X-Signature` on a request with an `Authorization` header, so the bearer
  token is what authenticates them, before and after this change.
- `waitForApproval` polled `GET /api/v1/sdk-api/verifications/{id}` with a bearer token. That endpoint authenticates
  only the `X-AIM-Agent-ID`, `X-AIM-Timestamp` and `X-AIM-Signature` headers, so every poll was refused and a pending
  action timed out even after an administrator approved it. Each poll now sends those headers, signed over
  `GET\n/api/v1/sdk-api/verifications/<id>\n<agent id>\n<timestamp>` with both IDs in canonical lowercase. A client
  with no signing key, or with an agent or verification ID that is not a UUID, now fails at once with a
  `VerificationException` instead of polling until the timeout.
- Tests verify each signature the client sends against the message the platform rebuilds, written out in the test:
  a GET, a POST with a body, a POST with an empty body, and an approval poll made with upper-case IDs.

### Fixed — No tracked file carries a home directory path from the machine that built or wrote it

- The six backend executables whose removal is described under "Removed — the source tree no longer carries
  prebuilt server binaries" embedded the home directory paths of the machine that built them (its Go module cache
  and its checkout). The two instructions that ran the tracked `./server`, in `sdk/python/tests/README.md` and the
  LangChain CRUD example, now say `go run ./cmd/server`.
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
  advisories, each of which covers every 2.18.x release through 2.18.10: `jackson-databind`
  GHSA-wv8q-qhhj-9h54 (CVE-2026-91776) and GHSA-cxp5-3px4-pw24 (CVE-2026-91777), and `jackson-core`
  GHSA-7hhh-6rmp-j9qf (CVE-2026-89425) and GHSA-p6pp-m3f8-5c89 (CVE-2026-89407). (#560)
- GitHub published the two `jackson-databind` advisories on 2026-09-30 and the two `jackson-core` advisories on
  2026-10-01. The CVE records were published on 2026-09-22 (CVE-2026-89407) and 2026-09-23 (the other three).
  Dates are UTC.
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

- Six macOS x86_64 executables built on developer machines were tracked under `apps/backend`: `aim-server`,
  `bin/aim-backend`, `bin/server`, `cmd/server/server`, `main` and `server`, 13.1 to 25.0 MB each and 129.2 MB
  together. Nothing used them: the container image builds the server from source. They are removed (history is
  left as is), and ignore rules keep a local `go build` or `go build -o` to those paths out of `git status`.
- A test in `apps/backend/cmd/server` reads every blob of the commit under test, not the working tree, and fails if
  one is a compiled executable, object or library (Mach-O, ELF, PE, ar, WebAssembly or Java class) or has the
  executable bit without a `#!` line. A PE file must carry its `PE\0\0` header; a file that only starts with `MZ` is
  not flagged. No path is exempt. Without `git` on `PATH`, or outside a git checkout, the test fails with the reason
  instead of skipping. A second test plants one file of each kind in a scratch repository, commits them, deletes one
  from the working tree and checks that the census reports exactly those. The ignore check covers all six former
  binary paths, `cmd/server/server` included.
- `TestTheLocallyBuiltServerBinaryIsNeitherTrackedNorStageable`, also in `apps/backend/cmd/server`, fails if a
  compiled `server` is tracked in that directory again or no ignore rule covers it.

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
  trust score. Migration 114 divides every stored JSON number above 1 and up to 100 by 100,
  including the seeded floors of 50 and 30, which sat above every possible trust score, and sets a
  number above 100 to 1. A number from 0 to 1 is kept as a fraction, so a floor the old form saved
  as 1, meaning 1%, now reads as 100%: the migration's NOTICE lines name each `mcp_*` policy whose
  floor lies above 0 and at most 1, and each whose floor is not a JSON number, which it leaves
  unchanged. A JSON `null` floor reads as no floor and stays `null`. The admin security-policies
  page still shows the floor as a percentage and now saves it as a fraction.
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

### Added — `aim-server migrate` applies the migrations and can provision a role that is not a superuser

- `aim-server migrate` applies the pending migrations and exits. It reads only the `POSTGRES_*` variables. With
  `POSTGRES_APP_USER` and `POSTGRES_APP_PASSWORD` set it first provisions that role (login, not a superuser, no
  BYPASSRLS, no CREATE on schema `public`, read and write on the tables and sequences the migrations create), refuses
  a `POSTGRES_APP_USER` that names the role it connects as, and after the migrations logs the role's measured state:
  `Application role aim_app: superuser=false bypassrls=false ownedRelations=0`. CREATE on schema `public` is revoked
  from `PUBLIC` as well as from the role, because before PostgreSQL 15 every role inherits it through `PUBLIC`; on
  those servers every role without its own grant loses it, and the command logs the revoke. The role's password is
  sent as a literal in `CREATE ROLE`/`ALTER ROLE`, so a server with `log_statement` set to `ddl` or `all` writes it
  to its log; `docs/DEPLOYMENT.md` "Database Migrations and the Application Role" describes both. Existing tables are granted to the
  role only when it is created; a later run leaves table grants alone, so a privilege revoked from the role stays
  revoked.
- The server's startup migration step no longer runs `CREATE TABLE IF NOT EXISTS schema_migrations` when the table
  exists. That statement needs CREATE on the schema even then, and stopped a server connected as a role without it.
- `docker-compose.quickstart.yml` is unchanged and still connects the backend as `postgres`: it pulls the published
  `aim-server` image, and no published image has the `migrate` command yet.
- An integration test provisions a role against a migrated database and checks its attributes, that it owns no
  relation, reads and writes the tables, cannot create one, passes the startup migration step, and keeps a revoked
  privilege on a second run.

### Changed — a password sign-in's audit row no longer copies the account's email and role

- The `login` audit row written by `POST /api/v1/auth/login/local` carried the account's email and role in its
  metadata, beside the `user_id` column that already names the account. Its metadata now carries route facts only
  (`method` and `familyId`), as the other sign-in routes' rows do; the account is identified by the row's own columns.
  Rows written before this change are left as they are.
- `apps/backend/internal/interfaces/http/handlers/sign_in_family_audit_test.go` fails when any sign-in route's login
  row carries an `email`, `name` or `role` in its metadata.

### Changed — every sign-in records the token family it issued

- Each route that issues a sign-in token pair now writes one `login` audit row in the user's organization whose
  metadata carries `familyId`, the session id (`sid`) the issued tokens carry. A family named later on a refresh, a
  logout or a refusal row resolves to the user and organization that signed in.
- `POST /api/v1/auth/login/local` already wrote a `login` row; it now carries `familyId`. `POST /api/v1/public/login`
  (an account that must change its password first included, and the sign-in that follows
  `POST /api/v1/public/change-password`), `POST /api/v1/oauth/device/token` and `POST /api/v1/auth/sdk/recover` wrote
  no row and now write one.
- `metadata.method` names how the user signed in: `password`, `device_code` or `sdk_token_recovery`. A recovery's row
  also carries `recoveredFrom`, the id of the revoked SDK token. The rows carry identifiers only, never a token.

### Changed — trust scoring counts only verification outcomes the server observed

- Every verification event now records who stands behind its outcome, in a new `source` column (migration 124):
  `service` (the server ran the check), `system` (a server-internal process recorded it), `caller_reported` or
  `agent_reported`. The server sets it from the code path that writes the event; no request field sets it, and an
  event without a known source is not written.
- The verification statistics behind trust factors 1 to 3 (verification status, uptime, action success rate) and the
  score's confidence count only `service` and `system` events. The outcome an agent reports through
  `POST /api/v1/agents/:id/log-capability/:audit_id` is recorded as `agent_reported` and is absent from the total, the
  success and failure counts, the averages and the last verification time. A source value not on that list counts
  nothing.
- Events written before the column existed carry no source and keep counting as they did.
- **Upgrading:** for an agent whose only events in the last 30 days are action-result reports recorded after the
  upgrade, factors 1 to 3 use the agent-status baseline they use when no events are recorded. No client change is
  needed.

### Fixed — a revoked agent can no longer be reactivated, verified or suspended

- `POST /api/v1/agents/:id/reactivate` set any agent to `verified`, a revoked one included, and the agent's keys were
  not changed, so the key it held when it was revoked authenticated again. `POST /api/v1/agents/:id/verify` did the
  same, and the agent page offered Verify and Suspend on a revoked agent.
- Each lifecycle route now reads one transition table: verify moves `pending` to `verified`, suspend moves `pending`
  or `verified` to `suspended`, and reactivate moves `suspended` to `verified`. Any other status is refused with
  `409 {"error": "<reason>", "reasonCode": "agentStatusTransitionRefused"}` (with the same value in `code`) and nothing
  is written; the reason names the act that applies instead, for a revoked agent `register a new agent to replace it`.
- An agent already in the act's target status answers 200 and nothing is written. The 200 bodies of verify, suspend
  and reactivate carry `changed`, false exactly in that case.
- The revoke response's `message` is now `Agent revoked.` It said to run reactivate within 30 days to restore the
  agent, and that its data is retained for 30 days, which no code enforces. `retentionUntil` is unchanged.
- The agent page offers Verify only for a pending agent (a verified agent shows a Verified badge), Suspend only for a
  pending or verified agent, Reactivate only for a suspended agent, and none of them on a revoked agent.
- **Upgrading:** a client that reactivated or verified a revoked agent, verified a suspended agent, or reactivated a
  pending agent now receives the 409. Verify a pending agent, reactivate a suspended one, and register a new agent to
  replace a revoked one.

### Fixed — a 4xx response states its reason instead of carrying an error's text

- 61 handler response bodies below 500 (400, 401, 403, 404, 409, 413 and two 200 fallbacks) put the text of a Go
  error in the body. Most passed a library or service message through: a UUID or JSON decoding error, a wrapped repository
  error behind a 400 (`failed to update password: ...` on a password change, the repository error on an API key
  delete or a device-code approval), a password-policy error (`password validation failed: ...`) through the helper
  the registration and access-request handlers share, or every reason a secrets resolve was refused, agent and
  namespace names included. Each now answers a line written for the caller.
- A refusal the caller can fix names the request member and the rule it breaks: `newPassword must be at least 8
  characters long`, `id must be a UUID`, `signupProfile.role must be one of the listed values`,
  `sandbox is not a recognized sandbox type`, `encryptedBlob must decode to at most 1048576 bytes`. The services
  return these refusals as declared errors (for example `ErrCurrentPasswordIncorrect`, `ErrAPIKeyNotDisabled`,
  `ErrDeviceCodeNotPending`, `ErrCapabilityAlreadyGranted`), and the handlers match them with `errors.Is` or
  `errors.As` instead of comparing text.
- A failure behind one of these endpoints that is not the caller's to fix (a repository or hashing error during a
  password change, password reset, API key delete, device-code approval or capability registration) now answers 500
  with the fixed server line and goes to the server log.
- `POST /api/v1/secrets/resolve` still refuses with 403. A malformed, stale or reused nonce and an `agentPublicKey`
  that is not the registered key are named; every other refusal answers `Secret resolution refused`, and its reason
  stays in the secrets audit entry and the server log.
- The census test in the handlers package now covers every status: it fails on any response (`c.JSON`,
  `c.Status(s).JSON`, `Send`, `SendString`, `fiber.NewError`) and any `fiber.Map` or `ErrorResponse` literal whose
  body is built from an error value. A second test plants an error text in a server failure and checks that it is
  in the log line and not in the response.

### Fixed — a 5xx response carries a fixed line, never the server's error text

- 130 handler responses at a 5xx status put the text of a Go error in the body, as `error`, `details` or `message`,
  naming tables, constraints and query fragments. Each now answers
  `{"error":"An internal error occurred. Please try again later."}`, a line declared once in the handlers package,
  and writes the error to the server log with the status, method and path. A failed `DELETE /api/v1/agents/:id`
  keeps its own stated sentence. Agent registration and the MCP attestation endpoints still answer 400, 403, 404 and
  409 with the reason they did before.
- The dashboard shows the same line for every 5xx, whatever the body holds: an older server's error text or a proxy's
  HTML page. A 503 refusal that names its reason (`reasonCode`, such as `noAdministrators`) keeps its own line.
- A test in the handlers package reads every handler file and fails on a response at a 5xx status whose body is built
  from an error value. A status counts as 5xx when it is a 5xx constant, a local variable set to one, or a call to a
  package function that returns one.

### Fixed — secrets resolve encrypts the credential to the agent's registered key and answers a signed request once

- `POST /api/v1/secrets/resolve` encrypts the credential to the agent's registered Ed25519 key, however the agent
  authenticated (ATC or signed request). A request whose `agentPublicKey` is not that key is refused with 403 before
  the credential is read and before its nonce is spent.
- `nonce` must be `<RFC 3339 time>:<random part>`, the form both SDKs send, with a time no more than 30 seconds from
  the server clock in either direction. Any other nonce is refused with 403.
- Each signed request (namespace, operation and nonce) is answered once. It is admitted through the same single
  statement and table, `agent_request_nonces`, as signed action-request statements, and the same purge deletes it. A
  repeat is refused with 403. When the check cannot run, including before the first purge after start, the answer is
  503 and nothing is resolved.

### Added — action verification accepts a signed statement with a nonce and a 30-second window

- `POST /api/v1/sdk-api/verifications` and `POST /api/v1/verifications` accept a second body form: exactly
  `signedBytes`, `signature` and `publicKey`, each unpadded base64url. `signedBytes` is the DSSE v1
  pre-authentication encoding of the payload type `application/vnd.opena2a.action-request.v1+json` and a JSON
  payload with `action_type`, `agent_id`, `resource` (a string or `null`), `timestamp` (RFC 3339 in UTC with `Z`)
  and `nonce` (16 random bytes), and optionally `context`, `delegation_digests` and `risk_level`. The Ed25519
  signature is checked over the bytes as received, and the action is decided and recorded from the signed payload.
  Numbers in `context` are recorded with the digits they were signed with.
- A statement is accepted only when its `timestamp` is within 30 seconds of the database clock, and an agent's nonce
  is accepted once. The time check and the nonce insert are one database statement, shared by both routes and every
  replica: a statement accepted on one route is refused on the other. The window is a code constant.
- Refusals carry a `reasonCode`. The shape checks answer 400 `invalidRequest` or `nonceRequired`, or 422
  `limitExceeded` for `signedBytes` over 65,536 bytes or a payload nested deeper than 32 levels. An unknown agent and
  a key that is not the agent's registered key get the same 401 `agentKeyNotRecognized` body. The others are 401
  `signatureInvalid`, `hybridSignatureRequired`, `timestampOutsideWindow`, `nonceReused` and `agentStatusDenied`,
  and 503 `freshnessCheckUnavailable` when the database cannot run the check. A refused statement writes no
  verification event, audit entry or alert.
- A statement from a suspended or revoked agent whose key and signature check out still spends its nonce. To stop a
  key someone else holds from writing nonces, rotate or remove the key, or delete the agent.
- Accepted nonces are kept in a new table, `agent_request_nonces` (migration 123), until 31 seconds after their
  window ends. A purge deletes them once at start and then every 60 seconds; until a purge has completed, and
  whenever none has for two intervals, every statement is refused with 503 and nothing is stored.
- A request in the original form is handled as before. One that also carries a top-level `nonce` or
  `delegationDigests` is refused with 400: those members are checked only inside a signed statement.

### Fixed — expired A2A request nonces are deleted on a schedule

- An A2A request nonce, and the SHA-256 request hash stored with it, stayed in `a2a_request_nonces` after it expired
  until an admin called `POST /api/v1/a2a/maintenance/cleanup-nonces`. The server now deletes them every 60 seconds,
  logs the interval when the job starts, and logs only the count of rows each run deletes.
- A nonce is kept while a request carrying it could still pass the timestamp check: for the five-minute signature
  timestamp tolerance past its expiry, and for twice that past its first use.
- The admin route stays and runs the same cleanup on demand, with the same bound.

### Fixed — an expired password reset token is cleared from the account

- A password reset token that was never used stayed on the user's row after it expired, until the user requested
  another reset. A token stored without an expiry, which the reset lookup never accepts, stayed the same way.
- The server's five-minute cleanup job now clears both and logs only how many rows it cleared. A reset link that has
  not expired keeps working.
- The `users` update trigger still sets `updated_at` on each row the job clears.

### Changed — a third party can verify an agent card attestation from the served card and the JWK Set

- A served card's attestation could not be checked by anyone but the server. The signed `issuedAt` was a different
  instant from the stored one, timestamps were signed with sub-microsecond precision that PostgreSQL does not keep,
  the served `issuer` was empty, and `cardHash` was not served. A refreshed attestation also signed the hash of the
  card as stored in JSONB rather than the registered card's hash.
- Attestations are now signed in the `opena2a-aim/card-attestation/v2` format: the label, a newline, and the JSON
  payload (`cardHash`, `agentId`, `issuer`, `issuedAt`, `expiresAt`), with both timestamps in UTC and whole seconds.
  The stored and served timestamps are the signed ones, in UTC. `aim.attestation` in a served card adds `format` and
  `cardHash`, and `issuer` reads `aim-server`.
- `docs/specs/card-attestation-v2.md` is the verification procedure. `docs/specs/card-attestation-v2-vector.json` is
  a conformance vector with seven cases, and `docs/specs/verify-card-attestation.mjs` is a Node.js verifier with no
  dependencies that checks a served card or the vector. The procedure also covers key rotation with
  `AIM_SIGNING_KEY_CARD_ATTESTATION_RETIRED` and the effect of rotating `KEYVAULT_MASTER_KEY` on derived keys.
- Cards record the format as `attestationFormat` (migration 122). Attestations issued before the upgrade read no
  format, are served without `format`, cannot be verified by a third party, and are re-signed in v2 when refreshed.

### Changed — each server signing purpose has its own key, and the public keys are published

- The backend signs with two Ed25519 keys, one per purpose: `card-attestation` (A2A agent card attestations) and
  `atc-issuer` (the key the ATC verifier trusts for `ATC_ISSUER_URI`). A signature made for one purpose does not
  verify for another. Before this release one key, derived from `KEYVAULT_MASTER_KEY`, served both.
- Each key is provisioned in `AIM_SIGNING_KEY_CARD_ATTESTATION` or `AIM_SIGNING_KEY_ATC_ISSUER` (a base64 32-byte
  Ed25519 seed). When a variable is unset, that key is derived from `KEYVAULT_MASTER_KEY` with HKDF-SHA256, so
  upgrading needs no new configuration. **Provisioning both keys is recommended in production**: a provisioned key does not change when the master key is rotated, and holding the
  master key does not reveal it. See "Server Signing Keys" in `docs/DEPLOYMENT.md`.
- `<VARIABLE>_RETIRED` lists earlier public keys that still verify, so a key can be rotated without refusing what it
  already signed.
- Startup stops with an error naming the variable when a value is malformed or when two purposes are given the same
  key. Startup logs each key's purpose, key ID and source, never key material.
- New route `GET /.well-known/jwks.json` (no authentication) serves a JSON Web Key Set of the card-attestation and
  ATC-issuer public keys, each with `kid` (hex SHA-256 of the public key), `purpose` and `status`; an active key
  also carries `source`.
- Agent cards record the key that signed their attestation as `attestationKeyId` and `attestationAlg` (migration
  121), and the AIM extension of a served agent card carries them as `keyId` and `alg`. Attestations issued before
  the upgrade keep their signature with no key ID, expire within their validity window (24 hours by default), and are
  re-signed under the new key when refreshed.
- The previous shared key is not published. A deployment that pinned that public key elsewhere, for example as a
  trusted ATC issuer key, must pin the `atc-issuer` key from `/.well-known/jwks.json` instead.

### Changed — refusals that carry a machine-readable reason also send it as `reasonCode`

- `reasonCode` is the API's member for a refusal's machine-readable reason. The four refusals that already sent one
  in `code` now send the same value in `reasonCode` as well: `noAdministrators` (registration and access requests
  when no administrator can approve them), `verificationEventWriteNotAccepted` (`POST /api/v1/verification-events`),
  `executionOutcomeNotAccepted` (`POST /api/v1/sdk-api/verifications/:id/result`) and `enforcementModeUnavailable`
  (`POST /api/v1/sdk-api/verifications/:id/execution-status`). `code` is still sent with the same value, so existing
  clients are unaffected.
- The dashboard reads `reasonCode` first and falls back to `code`.

### Fixed — a verification event refused because its agent could not be read logs the cause

- When a verification event was refused because the agent read failed (for example, a database error), the server
  logged only "agent not found", so the cause was lost. The log line now names the agent ID and the read error. The
  error returned to the caller is unchanged, so an unknown agent and an agent of another organization still return
  the same error.
- Automatic verification-event logging now checks that the agent belongs to the organization the event is recorded
  under, as manual recording already did, instead of relying on each caller to check first. Current callers already
  pass the agent's own organization, so the events they record are unchanged.
- Removed an unused helper that registered the verification-event routes. The server never called it; the mounted
  routes, including the manager-or-admin requirement on `DELETE /api/v1/verification-events/:id`, are unchanged, and
  that route's description now states the manager-or-admin requirement instead of "admin only".

### Fixed — a deleted webhook's name can be used again

- Deleting a webhook only marks it deleted, but the per-organization name uniqueness still counted deleted
  webhooks. Once a webhook was deleted, creating a new webhook with the same name, or renaming another webhook to
  it, failed with a duplicate-key error. Migration 113 replaces the `webhooks_name_unique_per_org` constraint with a
  unique index of the same name over webhooks that are not deleted. Two live webhooks in one organization still
  cannot share a name.

### Fixed — the alert detail panel presents a user-account alert as an account, not as an unknown agent

- Alerts raised on a dashboard user account (`account_locked`, `auth_failure_pattern`) opened a detail panel built
  only for agents: "Agent name Unknown", the nil UUID as the agent ID, an empty verification history, and a
  "View agent" button to a page that does not exist. The panel also asked the agent endpoints about the user. An
  alert whose resource type is `user` now shows an **Account** section with the email and, when the alert names
  one, the user ID; it hides the agent and verification history sections, makes no agent requests, and links to
  the users page instead. Agent alerts are unchanged.
- The panel's context block states a refresh-token reuse's `clientMatch` in words: "The same client that last used
  this token", "A different client from the one that last used this token", or "Could not be determined", under the
  label "Presented by". An unrecognised value is shown as recorded.

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
- `GET /api/v1/mcp-servers/:id/attestations` is mounted on the agent-signed group, whose middleware passes a request
  that carries a bearer token through without authenticating it. The dashboard's request therefore reached the
  handler with no organization, which answered `500` and would now answer `401`. A request with a bearer token now
  continues to the same path on the JWT-authenticated group, which answers it with the MCP server's attestations; an
  agent-signed request is served as before.

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

### Fixed — an issued trust credential carries publisherDid, buildAttestation and a 0-100 trustScore

- `POST /api/v1/agents/:id/atc` sent the issuer the agent's `trustScore` on the 0-1 scale of AIM's 9-factor score, and
  sent no `publisherDid` and no `buildAttestation`. The ATX v1.1 specification puts `trustScore` on the wire as a
  0-100 number and makes both other fields mandatory, so a credential issued from that request failed the ATX
  credential schema in a conformance verifier, and an agent scored 62 out of 100 was described as 0.62.
- AIM now sends the score on the 0-100 scale (0.62 is sent as 62), `publisherDid` as
  `did:opena2a:publisher:aim_<organization id>`, and `buildAttestation` as the address of the agent's DID document on
  the deployment's public DID resolver, `<FRONTEND_URL>/api/v1/did/<agent DID>`. The trust level AIM requests is
  unchanged. Issuance refuses to run when no public origin is configured.
- This needs an issuer that reads `trustScore` on the 0-100 scale. An issuer that still reads it on the 0-1 scale
  refuses these requests.
- `apps/backend/internal/application/atc_issuance_service_test.go` fails when a field AIM puts in the issuance request
  breaks its rule in the ATX v1.1 credential schema, or when the score it sends is not on the 0-100 scale.

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
  over plain HTTP from a host other than `localhost`, the turn across tabs is the lease described
  in the entry "two dashboard tabs share one token refresh in a browser without Web Locks" above.
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
  The session middleware now refuses such a token on the other routes it guards as well, at once on
  the instance that recorded the revocation and within 30 s on another (entry "a revoked session's
  access token is refused on the routes behind the session middleware").

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
- SDK-download tokens keep their 90-day lifetime and rotation.
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
- Java SDK: a JSON string is refused wherever the SDK binds a `Double`, `double`, `Float`, `float`,
  `BigDecimal` or `Number` from JSON, at any length and including a numeric string such as `"0.75"`;
  a JSON number still binds. This covers the A2A trust and confidence fields and the ATX credential's
  `trustScore`. `A2AClient` throws `A2AException` and `LocalAtxVerifier.verify` rejects the credential
  as `MALFORMED`; either way the message names the field and the string's length, says a JSON number
  is expected, and never contains the string. The MCP server trust scores `MCPIntegration` reads from
  the JSON tree are unchanged. `FloatingPointCensusTest` lists every floating-point declaration and
  read in the SDK, and fails on a new one until it is listed with the test that drives it from JSON
  or the reason it binds no number from JSON.
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
  forward across a redeploy. No write path sets the column yet; once an
  independent verification source exists, TEE attestation and orchestrator/host
  metadata may set it, an HMA static scan may not, and the SDK never.

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
  stays registered as a deprecated alias to the same handler (a published path is not
  removed, because a client may still call it). No SDK changed.

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
