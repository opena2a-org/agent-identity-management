# Changelog

All notable changes to `@opena2a/aim-sdk` are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this package adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- `createAgentRequestHeaders` signs a request as an agent with `agent-request-v1`, the Ed25519 request signature AIM
  verifies, and returns the four headers to send with it: `X-Agent-ID`, `X-Timestamp`, `X-Signature` and
  `X-Public-Key`. The signed bytes are the method, the path and query string of the URL, `X-Timestamp`, and, when the
  body is not empty, the body exactly as sent; the URL's host is not signed. `X-Timestamp` is this host's clock in
  Unix seconds, and AIM accepts the request only within 30 seconds of its own clock, earlier or later. There is no
  nonce: anyone who obtains the request can send it to AIM again until AIM's clock is more than 30 seconds past
  `X-Timestamp`. The method must be GET, POST, PUT, PATCH or DELETE in upper case, and the URL absolute http or https
  with no username or password and no empty query (a `?` with nothing after it, which Node 20's fetch drops from the
  request and Node 24's sends). The private key must be standard base64 of 32 bytes (a seed) or 64 bytes (the seed,
  then its public key, as AIM issues it), and its public key must equal the configured one. Anything else throws
  `ConfigurationError`, and nothing is signed. This change does not alter how `AIMClient`, `SecretsClient` and
  `A2AClient` send requests.

- `npx @opena2a/aim-sdk init` registers a first agent with a bootstrap token from the onboarding screen. The token
  is read from `AIM_BOOTSTRAP_TOKEN` (or `--token`, with a note that a flag value is visible in the process list) and
  sent only in the `X-AIM-Bootstrap-Token` header to `POST /api/v1/onboarding/bootstrap-tokens/exchange`. The
  agent's Ed25519 key pair is generated locally and only the public key is sent. Credentials are written to
  `~/.aim/agents/<name>.json` (file 0600, directory 0700) in the layout the Python SDK reads, with the file name
  sanitized as the Python SDK does it (letters and digits of any script, `-` and `_` are kept, so `café` is
  `café.json` from either SDK), and the agent's dashboard URL is printed. The dashboard address is the one the
  server reports in the exchange answer (`dashboardUrl`); for a server that reports none it is derived from
  `--url`, mapping port 8080 to 3000 only on a loopback host and dropping only a trailing `/api` path segment.
  `--url` (default `AIM_URL`, then `https://aim.opena2a.org`) points it at a self-hosted server and `--name`
  (default `my-first-agent`) names the agent. The `--token` note about shell history is printed before the value
  is checked, so it also appears when the value is refused as not a bootstrap token. The token is never printed
  or saved, server text is scrubbed of token-shaped strings, redirects are not followed (so the token header cannot
  be forwarded to another host), and existing credentials for the name are refused before the single-use token is
  spent. The README describes the command under Quick Start.

### Tests

- The signed-field enforcement harness now proves that the verifier compares each signed instant to the clock, not
  only that the signature covers it. Its violations change a field after signing, so the signature check alone
  rejected every one of them: a verifier that ignored `createdAt` or `expiresAt` (the 1.0.2 `expiresAt` incident, and
  #417) still passed. Each signed instant (derived from the signed payload) must now also have a clock violation, a
  validly signed delegation that `verifyDelegation`, `checkDelegationTemporalValidity` and `verifyDelegationChain`
  must reject at the evaluation time: before `createdAt`, after `expiresAt`, and exactly at `expiresAt`. The
  `createdAt` lower bound itself was enforced in #467 (#417).
### Added

- `LocalVerifier.verifyCredential` and `LocalVerifier.authorize` accept the credential as raw JSON text or
  bytes as well as a parsed object. Raw input goes through the strict parse in `@opena2a/atx-verify` before
  any field is read, so a credential carrying a duplicate member at any depth, including two names that
  differ only in case, is rejected as `MALFORMED` with a reason naming the member. A parsed object cannot
  get that check because `JSON.parse` has already dropped one of the duplicates. The two strict-parse ATX
  conformance fixtures now replay through the SDK and are rejected; a new test replays every vendored
  fixture through the raw-text entry.

### Fixed

- `LocalVerifier` verifies every ML-DSA-65 signature a credential declares. `@opena2a/atx-verify` 0.3.0 checks
  Ed25519 only and recorded an ML-DSA-65 entry without checking it, so a credential whose ML-DSA-65 signature was
  forged verified as long as its Ed25519 signature was intact. Each declared ML-DSA-65 signature must now verify, over
  the same canonical payload, against an `ML-DSA-65` key in `publicKeys` that may sign for the credential's issuer;
  with no such key the credential rejects as `SIGNATURE_INVALID`. An issuer that signs with both suites therefore needs
  both of its keys in `publicKeys`. A signature whose algorithm is not `Ed25519` or `ML-DSA-65` now rejects instead
  of being passed over. A v1.1 `issuerChain` DID now lends its DID-URL-bound keys only when it is also in
  `trustedIssuers`, as the Java SDK's verifier does, so a signer can no longer name itself in the chain and sign for
  a trusted issuer. The SDK now returns the expected verdict on every vendored ATX conformance fixture, including
  `v1_1-hybrid-mldsa-tampered.json` and `v1_1-untrusted-chain-authority.json`, which it accepted; the conformance test
  now checks each fixture's verdict and rejection category, not only that raw and parsed input agree.

- `OAuthTokenManager` now obtains a token the AIM server issues. It posted to `/oauth/token`, which the server does
  not serve (404), with `grant_type=client_credentials`, which the token endpoint answers with 400
  `unsupported_grant_type`; its client assertion was padded standard base64 and its signature covered a
  request-signature string rather than the JWT, so the server could not verify it either. The SDK now posts to
  `/api/v1/oauth/token` with the RFC 7523 JWT-bearer grant
  (`grant_type=urn:ietf:params:oauth:grant-type:jwt-bearer`) and a compact JWS: unpadded base64url segments, the
  Ed25519 signature taken over `header.payload` with the agent's key, and `sub`, `aud` and a five-minute `exp`.
  The README's OAuth line describes this grant instead of a client credentials flow.
- `OAuthTokenManager` signs the assertion with the 32-byte seed of the configured private key, so the 64-byte form
  AIM issues and `npx @opena2a/aim-sdk init` writes (the seed, then its public key) obtains a token. Signing with
  those 64 bytes as given threw `Uint8Array expected` from the Ed25519 signer, and no request was sent; only a
  32-byte seed worked. The key is decoded as `createAgentRequestHeaders` decodes it: standard base64 of 32 or 64
  bytes, with the last 32 bytes of a 64-byte key equal to the seed's public key. Any other key throws
  `ConfigurationError` before a request is sent.
- `AIMClient` signs the requests it sends with agent credentials (`X-AIM-Signature`) with the 32-byte seed of the
  configured private key, so `verifyAction` completes with the 64-byte form AIM issues and
  `npx @opena2a/aim-sdk init` writes. With that form the client obtained a token and then threw `Uint8Array expected`
  from the Ed25519 signer, and the request was not sent. The key is decoded as `OAuthTokenManager` decodes it; any
  other key throws `ConfigurationError` before a request is sent.
- The Express middleware and the Fastify plugin accept the agent identity in their options. `createAIMMiddleware`
  and `aimPlugin` take `credentials` (an `AgentCredentials` object) or a ready `client`; the README examples, which
  passed only `{ baseUrl, apiKey }`, answered 401 on every verified route without contacting AIM. Passing both
  options is refused. When the environment holds three of the four `AIM_*` agent variables, the "No credentials
  available" error now names the missing one (for example `AIM_ORGANIZATION_ID`), which reaches the 401 body. A test
  drives both README examples with a real client against a fake AIM server and asserts the action is verified
  there (#449).

- `aim-arp telemetry log` and `telemetry status` no longer read a corrupt or unreadable audit log as an empty one.
  A log holding only unparseable lines printed "No telemetry has been sent yet" and "Sent: 0"; `log` now reports
  how many lines could not be read and how many records it shows, and `status` adds an `Unreadable:` line beside
  the counts. A log that exists but cannot be read (for example `EACCES`) is reported as unreadable, with `Sent:
  unknown`, and `log` exits 1. A JSON line without the audit record's fields is counted as unreadable instead of
  throwing a raw `TypeError` from the renderer. `readAuditRecords` keeps its return shape; the new `readAuditLog`
  in the signature module returns the records with the unreadable count and read error (#416).
- `A2AClient.getTaskHistory` and `A2AClient.listSkills` now reach routes the AIM server registers. Both requested
  paths the server does not serve (`GET /api/v1/a2a/tasks/{agentId}` and `GET /api/v1/a2a/skills`), so every call
  failed. `getTaskHistory` now lists through `GET /api/v1/a2a/tasks?agentId=...&limit=...` and returns the tasks
  the target agent took part in as client or remote agent, mapping the server's `externalTaskId`, `contextId` and
  `state` to the method's existing `taskId`, `taskType` and `status` fields. `listSkills` now returns the current
  agent's skills from `GET /api/v1/a2a/agents/{agentId}/skills`, the route `getSkills` uses. Both keep their
  return types.
- `aimErrorHandler` answers every SDK error as JSON. It handled only denials and
  authentication failures and passed every other SDK error to `next(error)`, so an
  AIM server answering 500 reached Express's default handler, which outside
  `NODE_ENV=production` rendered an HTML page with the stack trace and local file
  paths and printed the stack to stderr. Those errors now get the body the Fastify
  plugin gives them, `{ statusCode, code, error, message }`, keeping a 4xx or 5xx
  status from the SDK error and using 500 otherwise. Errors that did not come from the
  SDK still go to `next(error)`, and so does any error once the response has started.
  A new test drives the README middleware stack against a fake AIM server on loopback
  (#450).
- An unreachable AIM server with no cached token now rejects with `NetworkError`, naming
  the token endpoint and the errno, as every other request does. The token exchange
  wrapped only a timeout, so the raw `TypeError: fetch failed` from `fetch` reached the
  caller, and `aimErrorHandler` passed it to Express's default handler, which answered
  500 with an HTML page carrying the stack trace (#450).

### Changed

- `A2AClient.updateTrustScore` is deprecated: the server refuses the write with 405
  because the A2A trust score is measured from recorded interactions, not asserted.
  A routed agent's `trustScore` is `null` while its composite is unscored.
- `A2AClient.revokeAttestation` is deprecated. The AIM server registers no route that revokes an attestation (it
  serves only `POST /api/v1/a2a/attestations` and `GET /api/v1/a2a/attestations/{agentId}/{skillId}`), so the method
  posted to a path the server does not serve. It now rejects with an error that says so, without sending a request,
  and never resolves. It is kept so existing code still compiles.

### Security — ARP's L2 no longer sends anything to a vendor by default

ARP shipped `intelligence: { enabled: true, adapter: 'agent-proxy' }` as its
default, and `agent-proxy` chose a destination by reading whichever model key
happened to be exported: `ANTHROPIC_API_KEY`, else `OPENAI_API_KEY`, else a
local Ollama. On any machine with one of those set — a developer laptop, a CI
runner for an agent project — a default-configured ARP therefore sent
qualifying events to that vendor, authenticated with the operator's own key and
billed to their own account. Nothing in the README said so.

Measured against a real install of published 1.3.1, with the HTTP layer stubbed
so no socket is opened and with a synthetic key: one `critical` event under the
shipped default produced an outbound attempt to `api.anthropic.com/v1/messages`,
headers `x-api-key, anthropic-version, content-type`, and the body carried the
value planted in the event verbatim. A `critical` event bypasses batching, and
the network monitor emits `critical` and is on by default, so this is the
default path rather than an edge case.

Affected: **1.0.0 through 1.3.1**, every published version — verified by reading
`dist/arp/index.js` out of each tarball. `arp-guard@0.3.0` is **not** affected;
it predates the ARP module and contains none of this code.

**What changes**

- `intelligence.enabled` now defaults to off, and every gate that can start L2
  requires an explicit `true`. Absent means off. Previously the gates tested
  `!== false`, so an absent value meant ON — and because config is merged
  shallowly, an operator who set only `intelligence.budgetUsd` replaced the whole
  object and armed the vendor channel. **Narrowing the budget, a hardening
  action, used to switch L2 on.**
- `intelligence.adapter` has no default and must be named explicitly. There is no
  automatic provider selection any more.
- `createAdapter('agent-proxy')` and `autoDetectAdapter()` now throw. The
  `'agent-proxy'` union member is kept so existing builds still compile; it
  refuses at runtime rather than guessing a destination. **This is the breaking
  change**: code that set `adapter: 'agent-proxy'` explicitly must now choose
  `'ollama'`, `'anthropic'` or `'openai'`.
- When L2 is switched on but no adapter is named, ARP says so once on startup and
  runs without L2. L0 and L1 are unaffected and still gate.
- Even with a remote adapter configured deliberately, `event.data` no longer
  travels raw. It is rendered through an allowlist, and withheld values are
  replaced by a length-and-character-class descriptor. The pattern id and the fact
  of a match still travel, so assessment still works. This closes the sharpest
  case, where check `OL-001` ("API key in output") rendered the matched credential
  into the prompt and posted it to the vendor.

  **Scope of that redaction, stated exactly:** it covers `event.data`. It does
  **not** cover `event.description`, which the prompt still carries verbatim. Most
  descriptions are tool-authored text plus an identifier (a pattern id, a hostname,
  a file path, a tool name), but the process monitor composes descriptions that
  embed up to 100 characters of the child command line. So an operator who
  deliberately configures a remote adapter still sends that much command line to
  their chosen vendor. This is pinned by a test rather than left to be discovered
  (`l2-egress-default.test.ts`, T6) and is tracked for a follow-up; it does not
  affect the default configuration, under which L2 makes no outbound call at all.

**If you were relying on the old behaviour**, set both fields explicitly:

```typescript
intelligence: { enabled: true, adapter: 'ollama' }   // inference stays local
```

**Rotation.** Keys sent as `x-api-key` went to their own intended vendor and are
not an exposure. What may be exposed is material carried in event content:
operators who ran with the AI layer (`aiLayer.prompt.enabled`, opt-in) should
treat credentials their agent surfaced in model output as disclosed to the
configured vendor and rotate those.

### The L2 status line reports what is running, and withheld values are coarser

Two follow-ups to the L2 change above, both found by an independent review of it.

The CLI's `Intelligence:` line restated the L2 gate instead of asking it. The first
version tested `enabled !== false` and kept printing `3-Layer (L0+L1+L2)` after the
default changed; the replacement tested `enabled === true && adapter`, which is still
wrong for `adapter: 'agent-proxy'` — that passes both checks and then throws on
construction, so the line announced a layer that was not running. Neither version had
a test. The line now calls `describeL2Status()`, which settles it by attempting the
construction the coordinator attempts, and reports the reason when L2 is not running.

`<withheld: N chars, classes>` reported an exact length. For a small value that pair
is not a summary, it is the value: `true` rendered as `4 chars, lower` and `false` as
`5 chars, lower`, so a boolean was recoverable, and short enums from a known set the
same way. Lengths are now bucketed (`up to 8 chars`, `9-16`, `17-32`, `33-64`,
`65-128`, `over 128`), and values of 8 characters or fewer carry no class information
at all.

### The egress census asks what leaves the process, not what leaves for the registry

`telemetry-egress-census.test.ts` claimed to pin the whole channel set and did
not: it filtered on `api.oa2a.org`, so the L2 vendor channel was invisible
because it addresses `api.anthropic.com` — and invisible a second time because it
sends via `mod.request(...)` through an aliased module handle the matcher did not
recognise. Widening the host list alone would not have found it.

The census now enumerates every module that transmits off-process, matches
aliased sends, and carries named positive controls so a matcher that stops seeing
a known channel fails instead of reporting clean. Modules that transmit because
the caller asked them to — the AIM client, A2A, OAuth, secrets, CRL fetch, the
ARP proxy forwarder — are listed with the reason rather than filtered out, so
they stay in scope if one ever grows an observational side-channel.

### Fixed — aim-arp telemetry CLI polish (#412)

- `aim-arp telemetry` reports an unreadable OpenA2A home as unknown state (exit 1) instead of printing defaults. `status` used to show "OFF (not turned on)" with an opt-out marker hidden behind a permissions error, and `log` said nothing had been sent. The opt-out marker check now treats a marker it cannot stat as present, so consent that cannot be read counts as refused.
- A filesystem failure (EACCES, EROFS, ENOSPC, ...) names the path and the permissions check instead of printing Node's one-line error. When `opt-out` cannot write its marker it says the opt-out did not take effect and names `OPENA2A_TELEMETRY=off`.
- `<subcommand> --help` prints that subcommand's usage and arguments; `telemetry --no-purge opt-out` names the order that works; `telemetry disclosure` uses the same indent and rule as the other commands; the help says "telemetry disclosure" rather than "install-time disclosure" (nothing prints at install); `telemetry status` names `npx @opena2a/aim-sdk telemetry <subcommand>` for an install whose bin is not on PATH.
- The internal CLI's `register` guard (help and stray arguments never run it) moves into a module with its own test.

### Fixed — 1.3.1 release-test P2 findings

- **Token response shape**: `OAuthTokenManager` now reads both the RFC 6749
  snake_case token response (`access_token`/`token_type`/`expires_in`) and the
  historical camelCase form. Previously an RFC-shaped response produced
  `Authorization: Bearer undefined`. A missing or unparseable expiry no longer
  poisons the token cache into `NaN` (which silently forced a refetch on every
  request); it falls back to a conservative 5 minutes, and a response with no
  access token in either spelling raises `AuthenticationError` instead of
  caching `undefined`.
- **Typed `/oauth/token` errors**: a non-2xx from the token endpoint now
  surfaces as the SDK's typed error family (429 → `RateLimitError`, 401 →
  `AuthenticationError`, others → `AIMError` with `code` and `statusCode`)
  instead of a bare `Error`, and the embedded response-body excerpt is capped
  at 500 characters (a proxy's HTML error page ran to 2095).
- **Retries claim removed**: the README advertised "Automatic Retries:
  Built-in retry logic with exponential backoff", but no retry logic exists in
  the SDK. The Features line now describes what the code does (typed errors,
  `RateLimitError.retryAfter`); retry policy stays with the caller. Pinned by
  `src/readme-claims.test.ts` so claim and code cannot diverge again.
- **`Retry-After` honored**: `RateLimitError.retryAfter` now reflects the
  server's `Retry-After` response header (seconds or HTTP-date) when present,
  falling back to the body field then the 60s default only when absent.
  Previously the header was never read, so `Retry-After: 7` yielded 60.
- **Enforcement telemetry on wire 403**: a 403 from `POST /api/v1/verify` with
  telemetry enabled now produces a correlated deny enforcement record, exactly
  as the 200 `{actionAllowed: false}` path does. Previously the
  `AuthorizationError` propagated before the recording hook could run, so the
  403 path recorded nothing. (The allow-path half of this finding — an allowed
  verify carrying a detection produces a record — was already fixed by the
  post-1.3.1 correlation-joiner rework; it is now pinned by a test.)
- **Network-error context**: a connection failure now names the request URL
  and the underlying errno (e.g. `ECONNREFUSED`), unwrapped from the fetch
  error's cause chain, with an explicit `baseUrl` too — not only when the
  localhost default was in play. Previously an explicit-baseUrl failure read
  just "Network error: fetch failed".
- **Delegation lower bound**: `verifyDelegation`, `verifyDelegationChain` and
  `checkDelegationTemporalValidity` now enforce BOTH bounds of the signed
  window: an evaluation instant before `createdAt` is rejected as not yet
  valid (lower bound inclusive; expiry stays exclusive). Previously
  verify-at-epoch against a delegation created in 2026 returned `true`.
- **GTIN registry destination**: `submitGTINEvent` resolves its registry URL
  through the same chain as every other telemetry transport — explicit
  `registryUrl`, else `OPENA2A_REGISTRY_URL`, else the default — instead of
  hardcoding the production registry, which is how the release test's dummy
  event reached production.
- **Env-credential diagnostics**: `loadCredentialsFromEnv()` still returns
  `null` when any of `AIM_AGENT_ID`, `AIM_PRIVATE_KEY`, `AIM_PUBLIC_KEY`,
  `AIM_ORGANIZATION_ID` is missing, but a PARTIAL environment now warns naming
  the missing variable(s) instead of presenting exactly like a clean
  environment. (The README half of this finding was already fixed by #451.)

### Fixed — 1.3.1 release-test P3 findings

- **The npm package no longer ships the TypeScript source inside its source
  maps.** `sourcemap: true` embedded ~2.6 MB of full source (doc comments
  included) as `sourcesContent` in every published tarball while the README
  promised "source is not shipped in the npm package". Maps still ship for
  readable stack traces; `sourcesContent` is stripped, and a packaging test
  now packs the real tarball and pins the invariant.
- `aim-arp help` and `aim-arp version` (the bare words) now work like
  `--help`/`--version`, and `aim-arp telemetry --version` answers with the
  version instead of "Unknown telemetry subcommand". The internal `arp-guard`
  CLI accepts the bare words too.
- `aim-arp telemetry log abc` is now an error instead of silently showing 20
  records, `log 0` is refused (the count must be a positive integer), and
  extra positional arguments (`log 5 5`, `status extra`) are errors — matching
  the strictness the option side already had.
- Every ARP telemetry request identifies its build: sensor enrollment and
  purge (right-to-delete) requests now send `User-Agent: OpenA2A-ARP/<version>`
  like the signature emitter (they previously sent none, logging as "node"),
  and the causal-denial relay's User-Agent now carries the SDK version.
- The express and fastify `verifyAction`/`verifyRoute` hooks set
  `req.aim.agentId` from the verification result, so handlers behind the hook
  can attribute the request to the verified agent without a separate
  `getAgent()` round trip.
- `AIMClient` refuses a nonsensical `timeout`/`enforcementTimeout` (negative,
  zero, NaN, Infinity) with a `ConfigurationError` at construction, and
  `registerAgent` refuses an empty or non-string `name` before any request is
  made.
- `parseAPIError` caps a server-supplied error message at 512 characters and
  falls back to "Unknown error" when the body's `message` is not a string —
  the message lands in logs and terminals verbatim and was previously passed
  through whole.
- The local correlated-events telemetry log is created `0600` (owner-only),
  matching the credential writer; it was `0644`.
- `LocalVerifier` validates the shape of its trust anchors at construction
  (`trustedIssuers` an array of strings, `publicKeys` an array of key
  objects) and throws `ConfigurationError` instead of storing a mis-shaped
  config that would later fail closed with a misleading denial reason.
- README: documented that `getAgent()` returns `null` and `getTrustScore()`
  returns `0` when no credentials are loaded (no request is made), and added
  the previously undocumented exported clients (`A2AClient`, `SecretsClient`,
  `OAuthTokenManager`, `CrlCache`) to the API reference.

### Fixed — entry points share one class identity

- The entry points (`.`, `/arp`, `/express`, `/fastify`) share one copy of `AIMClient` and the error classes (#334). Each bundled its own copy, so an `ActionDeniedError` raised through `@opena2a/aim-sdk/express` was not `instanceof` the class imported from `@opena2a/aim-sdk`. The build now emits shared chunks in both formats. The ESM and CommonJS builds remain two copies; match on `error.code` where one process loads both.

## [1.3.1] - 2026-09-02

### Added

- `AgentType.DEMO`, the agent type the one-command demo registers (#431).

### Fixed

- **NetworkMonitor ss parser read the columns of `ss -tpn` while running
  `ss -tpn state established`, so the Linux monitor never reported an
  established connection.** Under the `state established` filter ss omits the
  `State` column, shifting every field left by one: the peer address was read
  as the local address and the `users:(...)` process field as the peer, so
  `remotePort` parsed to `NaN` and no reported connection ever carried the
  real destination — and because the garbage list was non-empty,
  `getConnections()` never fell through to the correct `/proc/net/tcp`
  fallback either. The parser now picks the address columns from the header
  line (`State` present or not) instead of a fixed index, and no longer drops
  the non-root shape where ss cannot name the owning process and the data
  lines end at the peer column.

### Known issues

Both reproduce on 1.3.0 and are unchanged by this release; scheduled for 1.3.2.

- The README's Express and Fastify examples answer 401 and never contact AIM unless
  `AIM_AGENT_ID`, `AIM_PRIVATE_KEY`, `AIM_PUBLIC_KEY` and `AIM_ORGANIZATION_ID` are all set;
  the middleware options carry no way to pass credentials
  ([#449](https://github.com/opena2a-org/agent-identity-management/issues/449)).
- `aimErrorHandler` passes an upstream 5xx to Express's default handler, which renders a
  stack trace with local paths outside `NODE_ENV=production`; the Fastify plugin answers the
  same case as JSON ([#450](https://github.com/opena2a-org/agent-identity-management/issues/450)).


## [1.3.0] - 2026-08-24

### Added

- **The telemetry consent CLI now actually ships** as the `aim-arp` bin
  ([#400](https://github.com/opena2a-org/agent-identity-management/issues/400)).
  The install-time disclosure has cited `arp telemetry log` / `status` /
  `opt-out` since 1.0.0, attributing them to the hackmyagent CLI — but no
  installed package registered any such command. The audit-log read path,
  status, opt-out, opt-in and the right-to-delete purge were designed and
  implemented, and unreachable. They are now real:

  ```
  aim-arp telemetry status|log|disclosure|opt-out|opt-in|purge
  npx @opena2a/aim-sdk telemetry <subcommand>   # same bin, no global install
  ```

  The bin is named `aim-arp`, not `arp`: `arp` is a system utility on macOS
  and Linux and an unrelated npm package. Sensor enrollment (`register`) is
  deliberately not part of the shipped surface. A test now walks every command
  citation in the shipped TypeScript sources and fails when one names a command the
  package does not register, so this class cannot silently return.

### Fixed

- **Asking a subcommand for help no longer runs it.** `aim-arp telemetry
  opt-out --help` performed a real opt-out — the marker was written by the
  question — `opt-in --help` removed one, and on a machine that had sent,
  `purge --help` would have fired the right-to-delete from a help query.
  The dispatcher chose what to run from the subcommand name alone and never
  validated the arguments, so `--help` never stopped execution and unknown
  options were silently swallowed. `--help` / `-h` anywhere in the arguments
  now print usage and change nothing, and an unrecognized option is an error
  naming the option instead of a no-op.
  A mistyped subcommand is still reported as the error even when `--help`
  rides along, so scripts keep the typo signal. The internal CLI's
  `register` subcommand had the same defect — its help query would have
  built and sent a real enrollment — and is guarded the same way.

- **`telemetry status` names every environment variable it attributes an
  opt-out to, and only when that variable is what opted you out.**
  `OPENA2A_TELEMETRY_OPTOUT` and `ARP_TELEMETRY_DISABLED` were reported as a
  bare "environment variable" with no clearing instruction, while
  `OPENA2A_TELEMETRY` already got "unset it to clear". All three now name
  the variable in effect, and attribution reads the same strict truthiness
  as the opt-out decision — `OPENA2A_TELEMETRY_OPTOUT=0` alongside a local
  marker names the marker, not a variable whose unsetting would clear
  nothing.

- **`telemetry status` no longer creates identity state.** Reading status on a
  clean machine minted three files — the sensor id, the org id, and the org
  root secret — as a side effect of looking. A read command now reads: identity
  fields report `none yet (created on first send)` until a send mints them.

- **Server-supplied text is sanitized before it can reach your terminal.**
  Registry error and message strings printed by `opt-out`, `purge` and
  enrollment failures are stripped of C0/C1 control bytes (ANSI escapes,
  carriage-return overwrite) and capped at 500 chars with an explicit
  truncation marker — the TypeScript half of the class filed as
  [#384](https://github.com/opena2a-org/agent-identity-management/issues/384).
  The manual-retry `curl` commands also single-quote-escape their interpolated
  values.

- **A registry-assigned sensor id is shape-validated before it is persisted.**
  Enrollment adopted the server's `sensorId` verbatim into a local file that
  `telemetry status` later prints; a hostile registry could place
  terminal-driving bytes there. Ids are now accepted only when they match
  `^[A-Za-z0-9._-]{1,128}$`, and status strips control bytes from anything it
  reads off disk regardless.

- **`opt-out` and `purge` no longer create a sensor identity on a machine that
  never sent.** Building the purge proof minted the identity it was about to
  purge. Both commands now check first and report that there is nothing to
  purge.

- **`@opena2a/aim-sdk/arp` now reports the version npm published.** Its
  `VERSION` export was a second hand-maintained literal that read `0.2.0` from
  the day the module landed and was never bumped, so 1.0.0 through 1.2.0 all
  reported the same ARP version while the root export reported the real one.
  Both subpaths now export one value.

  This constant goes on the wire. The two telemetry channels send it as
  `User-Agent: OpenA2A-ARP/<VERSION>`, which is the only build signal the
  registry records for a submission, so every ARP request it has ever logged
  looks like it came from the same sensor build. If you parse that header,
  expect the package version from this release on and `0.2.0` before it.

## [1.2.0] - 2026-08-21

### Changed — read this before upgrading

**Structural signature telemetry is now OFF by default.** It previously ran
unless you opted out, sending the structural shape of anomalous runtime events
to `https://api.oa2a.org` from inside your process. It now sends nothing until
you turn it on:

- `AIM_TELEMETRY=1` in the environment, or
- `signatureTelemetry: { enabled: true }` in your ARP config.

If you were relying on this channel, it stops on upgrade and you must opt in to
restore it. If you had already opted out, nothing changes for you.

**`OPENA2A_TELEMETRY=off` now works in this SDK.** That is the spelling
[opena2a.org/privacy](https://opena2a.org/privacy) documents and the one it
tells you to put in your shell profile, but no released version of this package
ever read it: 1.0.0 through 1.1.0 read only `OPENA2A_TELEMETRY_OPTOUT` and
`ARP_TELEMETRY_DISABLED`, neither of which appears in the policy. A user who did
exactly what the policy said was still emitting. `off`, `0`, `false` and `no`
are accepted, matching the CLIs. It is honored in the off direction only —
`OPENA2A_TELEMETRY=on` does not turn the channel on, because an ecosystem-wide
CLI setting should not start a network channel inside a library embedded in your
process.

An opt-out still beats an opt-in, so `OPENA2A_TELEMETRY_OPTOUT=1`,
`ARP_TELEMETRY_DISABLED=1`, `signatureTelemetry.enabled: false` and the
`~/.opena2a/telemetry-optout` marker all continue to work and now take
precedence over either opt-in above. Clearing an opt-out no longer starts the
channel by itself — refusing and never choosing are now distinct states, and
only an explicit opt-in starts it.

This SDK ships as a library inside other people's production processes, and
AIM's claim is that identity and audit stay on disk. A channel that phones home
unless the operator finds the switch is inconsistent with both.

### Fixed

- **The default config no longer enables the channel.** `defaultConfig()`
  carried `signatureTelemetry: { enabled: true }`, so every `AgentRuntimeProtection`
  built from defaults — which is every one that does not pass an explicit config
  — turned the channel on. That value is now absent rather than `false`:
  an explicit `false` is an opt-out, and because an opt-out beats an opt-in it
  would have made `AIM_TELEMETRY=1` silently do nothing.
- **`arp telemetry status` no longer reports "OFF (opted out)" when nobody opted
  out.** Off-because-refused and off-because-never-enabled are different states
  and only one of them has a reason to report; the status output and
  `arp telemetry opt-in` now distinguish them.
- The first-run disclosure text no longer says "ON by default" and now names how
  to turn the channel on as well as off.

- **A server-side denial is now reported as a denial, not as an opaque 500.**
  `parseAPIError` maps HTTP 403 to `AuthorizationError`, but the express
  middleware, the fastify preHandler, fastify's `verifyRoute` and
  `aimErrorHandler` each checked only for `ActionDeniedError` and
  `AuthenticationError`. An `AuthorizationError` matched no branch and fell
  through to `next(error)` / `throw`, so the ordinary case of AIM refusing an
  action over the wire surfaced to the caller with no status and no denial
  reason. All four now answer `403 Action denied`.

  Verification has always been fail-closed in these integrations -- a failed
  verification never called `next()` and never reached the route handler -- so
  this is a reporting defect, not a bypass. The Python SDK's fail-open denial
  defect has no analogue here.

### Changed

- `verifyRoute` (fastify) now delegates to `verifyAction` instead of repeating
  its body. Two copies of the same catch ladder in one file is how they came to
  disagree; express's `verifyRouter` already delegated this way.
- The error-to-response mapping lives in one place,
  `src/integrations/verification-outcome.ts`, so the integrations agree by
  construction rather than by three files being edited together.

## [1.1.0] - 2026-07-16

### Security

- **Delegation chains now enforce temporal narrowing: a child may not outlive its
  parent.** `verifyDelegationChain` rejects any chain in which a child's
  `expiresAt` is later than its parent's, evaluated independently of the current
  time (the per-hop expiry check added in 1.0.3 already fails the chain once a
  parent has actually expired; this closes the window before that and covers
  partial / as-of verification). This completes the delegation-expiry hardening
  from 1.0.3.

### Added

- `createDelegation` accepts an optional `parentExpiresAt`. When creating a
  sub-delegation, pass the parent's `expiresAt`: the child's default expiry is
  capped at the parent's, and an explicit `expiresAt` beyond the parent's is
  rejected (fails closed on an unparseable value). This prevents the default
  seven-day expiry from silently producing a child that outlives its parent.

### Note

- Stricter verification: a chain whose child outlives its parent — previously
  accepted — is now rejected. Chains built with `createDelegation` (using
  `parentExpiresAt` for sub-delegations) are unaffected. Cross-engine parity for
  this rule (Go / Java verifiers, kanoniv interop spec) is tracked as a follow-up;
  the TypeScript verifier being stricter is fail-closed-safe in the interim.

## [1.0.3] - 2026-07-15

### Security

- **Delegation verification now enforces temporal validity.** `verifyDelegation`
  and `verifyDelegationChain` previously checked signature, delegator identity and
  scope narrowing but never evaluated the signed `createdAt`/`expiresAt` window, so
  an expired delegation — and a child that outlived its parent — still verified as
  valid. Verification now rejects delegations that are expired, have an unparseable
  or missing timestamp, or have an inverted window (`createdAt` after `expiresAt`),
  and fails closed in every one of those cases. `verifyDelegationChain` evaluates
  every hop against a single shared instant.

### Added

- `verifyDelegation(delegation, { verifyAt })` and
  `verifyDelegationChain(chain, { verifyAt })` accept an explicit evaluation time
  (a `Date` or ISO-8601 string) for deterministic tests and offline / as-of
  verification. Defaults to the current time.
- `checkDelegationTemporalValidity(delegation, verifyAt?)` — the standalone
  temporal check, exported for callers that want the reason a delegation is
  temporally invalid.
- `verifyDelegationSignature(delegation)` — the raw Ed25519 signature check with
  no temporal evaluation, for archival/audit inspection where authenticity matters
  independent of time. `verifyDelegation` now composes signature + temporal checks.
- `DelegationChainResult` gains a `temporalValid` field.

### Notes

- The production ATX/ATC credential verifiers (Go backend, `LocalVerifier`, the
  Java SDK) already enforce credential expiry and are unaffected. This change
  concerns the cross-engine delegation-chain primitive in this package.
- Reported privately by Tymofii Pidlisnyi (Agent Passport System,
  agent-passport.org).
