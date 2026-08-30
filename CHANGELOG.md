# Changelog

All notable changes to the AIM platform are documented here. The format is based
on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and from this release
forward the platform follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

> Scope: this changelog tracks the **platform** (backend + dashboard), tagged
> `platform-v<version>`. The SDKs are versioned and released independently
> (`aim-sdk` on PyPI, `@opena2a/aim-sdk` on npm, `org.opena2a:aim-sdk` on Maven
> Central) under their own `sdk-*-v<version>` tags.

## [Unreleased]

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
  would clear the agent's key material — and it has no caller. See #359.
- The tenant-scoping lint fails when an allowlist key names a method that does not
  exist. Fourteen of thirty-two entries resolved to nothing, presenting as reviewed
  exemptions while covering nothing. All fourteen were removed and the lint still
  passes, which is the evidence that none of the real handlers needed exempting.
  Remaining `needs review` entries are tracked in #358.

### Fixed

- **Every SDK isolation attestation was 404ing.** All three shipped clients POST
  `/api/v1/sdk-api/agents/{id}/isolation-attestation` (`AIMClient.ts:729`,
  `client.py:2427`, `AIMClient.java:1698`); the backend registered only
  `/agents/:id/isolation`, so no agent's self-reported isolation posture ever
  reached `isolation_attestations` and trust factor 9 sat at its `0.3` baseline
  for every agent in every deployment. Per the [CHIEF-CA] 2026-08-29 ruling the
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
