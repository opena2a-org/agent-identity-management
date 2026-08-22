# Changelog

All notable changes to the AIM Python SDK will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [2.0.1] - 2026-08-22

Closes both defects 2.0.0 carried as known issues, and hardens every path a
server-sent string takes to a developer's terminal.

### Security

- **Server-sent text is sanitized before it can reach a terminal**
  ([#384](https://github.com/opena2a-org/agent-identity-management/issues/384)).
  A denial reason flowed from the server into exception messages and the
  monitoring-mode warning with no control-byte stripping and no length bound,
  so a hostile or compromised AIM server could carry ANSI escapes that rewrite
  what a developer sees, or scroll the real message away. Decision-borne
  strings are sanitized where they enter `VerificationDecision` (total over
  any JSON value — a non-string reason is coerced, never raised on, because a
  constructor raise would turn an explicit DENY into UNKNOWN). The pre-decision
  console warnings, the JIT poll warning, the 401 exception message and the
  403 denial-detail parse sanitize at their own entry sites, and the console's
  free-text methods strip control bytes as defence in depth. C0 bytes other
  than newline and tab, DEL and the C1 range are removed; text over 500 chars
  is truncated with an explicit marker.

### Fixed

- **The registration panel no longer reports a trust score the server did not
  send** ([#390](https://github.com/opena2a-org/agent-identity-management/issues/390)).
  Three defects sat on this one path, each of which independently put a number on
  screen that nothing measured:
  - `client.py` defaulted to `0.85`, so an unscored agent was announced as
    `Trust Score: 85%` in the same form as a real reading.
  - The same line used `or` rather than a presence test, so a genuine server-sent
    `0` — a real, maximally-untrusted score — is falsy and fell through to that
    same fabricated 85%. Verified before fixing: a payload of
    `{"trust_score": 0}` printed `Trust: 85%`.
  - `AIMConsole._normalize_trust_score` mapped `None` to `0.0`, which rendered an
    unknown score as a measured, alarming red `0%` — and, because callers
    normalize before formatting, made the `N/A` branch in both formatters
    unreachable. That branch existed and looked like a control; nothing could
    reach it.

  An absent score now stays absent and prints `N/A`. A genuine `0` still prints
  `0%` and remains distinguishable from unknown.

- **Emoji removed from user-facing output**
  ([#391](https://github.com/opena2a-org/agent-identity-management/issues/391)).
  U+23F3 hourglass becomes U+25CB, the mark this console already uses for
  pending; U+26A0 warning sign (usually carrying U+FE0F, which forces colour
  emoji presentation) becomes the word `Warning:`, which most of those lines
  already carried; U+2139 is dropped, since the label states what it is. U+2713,
  U+2717 and U+25CB are the house style and are unchanged.

  Characters are named by codepoint here rather than shown, because the
  regression guard below scans this file too.

  The sweep went wider than the issue's table, which listed three codepoints.
  The same standard was being broken by U+2705 heavy check mark (23 uses), U+274C
  cross mark (12) and sixteen pictographs in the same files — fixing only the
  tabled three would have shipped a panel with house-style U+2713 and emoji
  U+2705 on adjacent lines. U+2705 and U+274C map to U+2713 and U+2717; the
  pictographs are dropped.

  Correction to the issue's framing: `docs/*.md` are described there as shipped.
  They are not — `MANIFEST.in` ships `VERSION`, `README.md`, `CHANGELOG.md`,
  `LICENSE` and `aim_sdk/*.py` only. They are swept anyway because they are
  public on GitHub, but no pip user was receiving them.

### Added

- A regression guard that enumerates the package and the prose from disk and
  fails on any emoji, rather than checking the sites that were once wrong.
  Checking only known-bad sites is how this class returns. The guard is
  property-based for the same reason: the first pass at this fix used a
  hand-written list of pictographs to strip, and the list was the defect — it
  missed ten others sitting in the same documents.

## [2.0.0] - 2026-08-11

**On the deprecation window.** `docs/VERSIONING.md` documents a mandatory N+1 minor
release — one version carrying a `DeprecationWarning` for a change before the
change itself lands — before any breaking removal. This release does not have
that precursor: there was no prior minor version warning that a previously
falsy-returning path would start raising. The exception is deliberate, not an
oversight. Every published version silently fails open on a denial; the fix
*is* the raise. Holding it for a warning-only release would mean shipping a
known fail-open bug for one more minor version to satisfy a process meant to
protect callers, which is the opposite of what the process is for. Treat this
as a named, one-time exception to the timeline in `docs/VERSIONING.md`, not a
change to the policy itself.

### Changed — read this before upgrading

**Strict mode now actually blocks. Check which enforcement mode your organization
is in before you upgrade** (dashboard: Settings → Security → Policies).

Before this release the organization's enforcement mode never reached the Python
SDK at all, so a strict-mode organization behaved exactly like a monitoring-mode
one and denied actions executed. Three things can therefore newly stop a running
agent, and all three are the policy you already configured finally taking effect:

1. **A denied action now raises, in EVERY enforcement mode — including
   monitoring, which is the default.** This is the change most likely to affect
   you, because monitoring is the schema default and every organization created
   before this release was backfilled to it.

   Monitoring mode governs what happens when AIM cannot give an answer, not what
   happens when AIM says no. A verification that could not be completed is logged
   and the action proceeds; an explicit denial blocks in every mode, because it is
   a decision AIM already made with your organization's enforcement mode in hand.

   Concretely: the backend applies your enforcement mode before it answers, so
   under monitoring a policy refusal is already converted to an approval and never
   reaches your process as a denial. What does still arrive as a denial is the
   short list the server refuses to override — an agent that is not found, an
   agent **marked compromised**, a capability lookup that failed — plus an
   administrator pressing Deny on a JIT request. Until this release the SDK
   overrode all of those a second time and ran the action anyway.

   If you rely on monitoring mode to observe without blocking, that still works
   for everything except an explicit denial. If an agent of yours is currently
   denied and executing regardless, it will now stop; grant the capability in the
   dashboard under Agents, or check the agent's status there.
2. **`AIM_STRICT_MODE=1`, `=yes` and `=on` now enforce.** They previously did
   nothing — only the literal `true` was recognised — while our own
   `demo_agent.py` accepted them and printed `Strict Mode: ENABLED -
   Unauthorized actions WILL BE BLOCKED`. If you set one of these believing the
   control was on, it is on now.
3. **`AIM_STRICT_MODE=false` no longer disables enforcement.** The variable is a
   ratchet: it can only raise enforcement, never lower it. It is ignored with a
   warning rather than being a hard error. Remove it from your configuration —
   it no longer does what `docs/ENV_CONFIG.md` previously described. Monitoring
   mode is not a way to run without enforcement either: it stops AIM blocking on
   verifications it could not complete, and does not stop it blocking on ones it
   refused. There is no setting that makes a denied action execute.

**The return value of `verify_capability()` keeps its shape in 2.0.0** on the
approved path. `result["verified"]`, `result["verification_id"]`,
`result["approved_by"]` and `result["expires_at"]` all keep working, and two keys
are added: `mode` and `mode_source`.

Two of those keys change *value* rather than shape. `approved_by` and
`expires_at` were read from the response as `approved_by` and `expires_at`, while
the backend has only ever sent `approvedBy` and `expiresAt`, so both were `None`
on every call in every published version. They now carry the values the server
sends. Code that treated them as always-absent — `if not result["approved_by"]`,
or a `None` check standing in for "auto-approved" — will take a different branch.

What *has* changed is that paths which previously returned a falsy dict now
raise. `verify_capability()` returns only when the action is permitted:

| Situation | Before | Now |
|---|---|---|
| Permitted | dict with `verified: True` | unchanged, plus `mode` / `mode_source` |
| Explicitly denied | raised `ActionDeniedError` | unchanged |
| 403 from the server | raised `AuthenticationError` | raises `ActionDeniedError` |
| 404, 429, 5xx, bad JSON | returned `{"verified": False, ...}` | raises `VerificationUnavailableError` |
| Timeout, connection, DNS, TLS | returned `{"verified": False, ...}` | raises `VerificationUnavailableError` |

The broadest compatible catch is `except AIMError`, which every SDK exception
still descends from.

#### One narrow break, stated plainly

If you run with `AIM_STRICT_MODE=true` and catch `PermissionError` around a
decorated function, an *unavailable* AIM no longer lands in that handler.
`VerificationUnavailableError` deliberately does not subclass `PermissionError`,
because a handler written for "AIM said no" must not silently absorb "AIM was
never asked". The action is blocked either way — only the label changed, and the
old label was false.

```python
# before
except PermissionError:
    ...
# after
except (PermissionError, VerificationUnavailableError):
    ...
```

#### Coming in 3.0.0

When AIM cannot be reached **and** this process has no cached enforcement mode,
the action currently executes and emits a `PendingEnforcementChange` warning. In
3.0.0 it will be blocked instead. To adopt that behaviour now, set
`AIM_STRICT_MODE=true`.

### Fixed

- **A denial arriving as HTTP 403 — the only wire form a real denial takes — was
  parsed as if it were an error envelope.** `CreateVerification` returns a full
  verification response with status 403: `id`, `denialReason`, `enforcementMode`.
  The SDK read a key called `error`, which that route has never sent, and nothing
  else. Three consequences, all fixed together because they share six lines:
  every production denial reached the developer as the literal string
  `insufficient permissions`, naming no policy and no capability; the
  organization's enforcement mode was resolved from a hard-coded `None` on the
  one response that states it; and the verification id was dropped, so
  `report_execution_status` returned at its first line and AIM never recorded
  that the blocked action was in fact blocked. Those execution columns are not
  read by anything today, so nothing was displaying the wrong answer; what was
  missing is the evidence itself, for every verification written by every
  published version.
- **The administrator's stated reason now reaches the developer on a JIT denial.**
  The polling path read `denial_reason`; `writeVerificationResponse` sends
  `denialReason`. A developer stopped by a human decision was told only
  `Action denied`, never why. Same root cause as the 403 above: the SDK read
  snake_case keys off a response whose fields are all camelCase. Test fixtures
  reproduced the wrong shape, which is why 165 tests could not see any of it.
- **An explicit denial no longer executes the action.** `ActionDeniedError` was
  not a `PermissionError`, and the decorators' only two handlers were
  `except PermissionError` and `except Exception` — so every denial fell into the
  fail-open branch and the wrapped function ran. No outage, no attacker, no
  misconfiguration required; this was the normal denial path in every published
  version. `ActionDeniedError` now also subclasses `PermissionError`, so the
  contract the README always documented is true. (Note that `PermissionError`
  descends from `OSError`, so a denial now also satisfies `except OSError`.)
- **The organization's enforcement mode now reaches the SDK.** The decorator read
  `enforcementMode` off the client's return value; no return site in `client.py`
  ever set that key, so it fell back to `"monitoring"` on every call and the
  dashboard setting had no effect. An absent or unrecognised mode is now
  `unknown`, never `monitoring` — a degraded upstream must not be able to emit
  the lenient value by failing.
- **Execution reports now actually send.** The decorator read the id as
  `verificationId`/`id` while the client emits `verification_id`, so
  `report_execution_status` returned early on a falsy id before building a URL.
  All eight call sites were fed by that one value: AIM had never received an
  execution report from the Python SDK. The report also now carries its own short
  timeout instead of inheriting the client's 30-second request timeout.
- **Which entry points send an execution report, and which do not.** Reporting runs
  from `@aim_verify` in `aim_sdk/decorators.py`, and therefore also from the four
  convenience wrappers `aim_verify_api_call`, `aim_verify_database`,
  `aim_verify_file_access` and `aim_verify_external_service`, each of which
  delegates to it. It does **not** run from `perform_action`, `track_action`,
  `require_approval`, or the separate `aim_verify` in
  `aim_sdk/integrations/langchain/decorators.py`. Those four resolve a decision and
  act on it without writing an execution row, so an integration built only on them
  produces no execution evidence in 2.0.0. Widening them is deferred deliberately
  rather than done piecemeal, so that all four change in one release under one
  contract.
- **A rate-limited verification no longer disables verification.** A 429 is a
  status ≥ 400, so it returned `verified: False` and the action ran — 101
  requests in a minute from the agent's own IP turned verification off for that
  IP, with no credentials and no exploit, and it also fired by accident behind a
  NAT. A 429 is now retried once or twice honouring `Retry-After`, and if no
  decision is obtained it is not permissive. A 5xx is never retried, because the
  server may have partly executed the request.
- **`@require_approval` works at all.** It called `console.show_jit_awaiting()`,
  `show_jit_denied()` and `show_jit_approved()`, none of which exist — the real
  names are `jit_waiting`, `jit_denied` and `jit_approved`. It raised
  `AttributeError` on its first line on every call, including an approval, so it
  had never executed a wrapped function. Fixing the crash exposed a defect it had
  been hiding: `@require_approval` printed "approved - executing..." whenever the
  wrapped function ran, which is also true when a monitoring-mode organization
  ran it after an explicit denial, and when the 2.0.0 warning-window cell above
  ran it unverified after a transport failure. Neither of those is an approval.
  It now prints "approved" only on an explicit `ALLOW`, and something honest
  otherwise. The waiting panel's "Approve in dashboard" link was also always
  `http://localhost:3000/...` regardless of the configured server; it now uses
  the client's own `aim_url`.
- **`aim_verify`'s auto-init failure is now an `AIMError`.** With no client
  passed and no `AIM_AGENT_NAME` to auto-initialize one, it raised a bare
  `ValueError`, which the SDK's own documented `except AIMError:` pattern does
  not catch. It is now `ConfigurationError`, the category already used for every
  other setup failure (missing credentials, bad URL).
- **The LangChain `@aim_verify` no longer runs tools unverified.** It never
  inspected the verification result, so a 404, 429, 5xx or timeout fell through
  to the execute path. It also converted every exception into `PermissionError`,
  telling developers they had been denied when AIM was unreachable.
- **`@track_action` and `@require_approval` no longer substitute a sentinel dict
  for your function's return value** when verification does not permit the
  action. A caller that ignored the return proceeded as if the work had happened.

### Added

- `aim_sdk.decision.VerificationDecision` — the typed three-state decision
  (`ALLOW` / `DENY` / `UNKNOWN`) carrying the enforcement mode and its source.
  Attached to `ActionDeniedError` and `VerificationUnavailableError` as
  `.decision`, `.mode` and `.mode_source`.
- `aim_sdk.exceptions.VerificationUnavailableError`.
- `aim_sdk.strict_mode` — one `AIM_STRICT_MODE` parser, used by the SDK and by
  `demo_agent.py`, accepting `true|1|yes|on` case-insensitively.
- A process-scoped, in-memory enforcement-mode cache with a 300-second TTL on a
  monotonic clock. It is never written to disk, a keyring, or any cross-process
  store: a persisted cache is forgeable in the lenient direction.

### Deprecated

- The dict return from `verify_capability()`; 3.0.0 returns a
  `VerificationDecision`. Emits a `DeprecationWarning` once per process.
- The `verified` key, superseded by the decision's outcome.

### Security

All previously published versions of `aim-sdk` are affected by the denial
fail-open. Upgrading to 2.0.0 is the fix. If you cannot upgrade, set
`AIM_STRICT_MODE=true`.

Note the honest bound: everything the SDK enforces is inside the agent's own
process, and is therefore advisory with respect to the agent. This release means
an honest operator's strict-mode organization actually blocks denied actions, a
compromised agent still routing through the SDK is blocked, and the AIM console
stops implying enforcement it never performed. It does **not** enforce anything
against a hostile operator.

### Known issues

Two defects found by this release's fresh-user walkthrough are **not** fixed here.
Both reproduce identically on 1.24.1, so neither is introduced by 2.0.0 and
upgrading does not expose you to anything you were not already exposed to. Both
are scheduled for **2.0.1**. They are recorded here rather than left silent
because this release's own subject matter is the SDK reporting things that were
not true.

- **The registration panel prints `Trust Score: 85%` when the server sent no trust
  score at all** ([#390](https://github.com/opena2a-org/agent-identity-management/issues/390)).
  `client.py` reads `credentials.get("trust_score") or credentials.get("trustScore", 0.85)`,
  so an absent field renders as `85%` in the same form as a measured value, on the
  first thing a new user sees. The `or` also means a real server-sent score of `0`
  displays as `85%`. The SDK's own stored record is correct — `~/.aim/agents/<name>.json`
  holds `"trust_score": null` for the same registration — so the display contradicts
  the file. Treat the trust score shown at registration as unverified; read it from
  the dashboard instead.
- **Emoji-class characters in console output**
  ([#391](https://github.com/opena2a-org/agent-identity-management/issues/391)).
  U+23F3 in the `@require_approval` panel, U+26A0 on warning lines, and U+2139 on
  the API-key mode line. Cosmetic; no behavioural effect.

## [1.24.1] - 2026-06-08

### Fixed

- **CLI login no longer doubles the "Token exchange failed:" prefix.** `aim-sdk
  login` printed `Token exchange failed: Token exchange failed: 405` when the
  token endpoint returned a non-JSON error — the classic symptom of pointing the
  SDK at the dashboard URL (`https://aim.opena2a.org`) instead of the API base.
  The message now surfaces the server's `error_description` and, on a non-JSON
  response, a one-line hint that the URL should be the AIM API base. (#286)

### Internal

- Guard `tests/test_sdk_verification.py`'s manual smoke script under `__main__`
  so it no longer makes a network call during `pytest` collection. (#283)

## [1.24.0] - 2026-06-06

### Added

- **Causal-denial telemetry (opt-in, parity with the TypeScript SDK).** A new
  `aim_sdk.telemetry` package joins the injection cause (detection inference),
  the classified intent, and the authorization outcome (enforcement fact) into
  one correlated record so you can see *why* an action was blocked. Off by
  default, best-effort, and off the enforcement critical path — it never changes
  a verdict and never raises into `verify_capability`.
  - `AIMClient(..., telemetry={...})` enables it via two independent opt-ins:
    `telemetry.enabled` (stage 1: capture records locally) and
    `telemetry.relay.enabled` (stage 2: share anonymized indicators).
  - `verify_capability(..., telemetry={"intent": ..., "detection": ...})` is the
    seam the injection detector / intent classifier populate.
  - Two tiers: the full record stays local (`~/.opena2a/correlated-events.jsonl`,
    authoritative); only an anonymized `SharedIndicator` may leave, carrying no
    identifiers, payloads, paths, credentials, or the correlation key. The relay
    egress-validates technique fields (`^T-\d{1,6}$` + source allowlist +
    confidence in [0,1]) and uploads only `denied_injection_attempt` indicators
    to the Registry's public, count-only endpoint.
  - New public exports: `CorrelationJoiner`, `CorrelatedRelay`,
    `EnforcementInput`, `IntentInput`, `DetectionInput`, `build_correlated_record`,
    `to_shared_indicator`, `interim_technique_fields`, `mint_correlation_id`.
  - `AIMClient.close()` / `close_telemetry()` stop the managed joiner and relay
    threads.
  - Hardening: the sensor-token salt file is read/created with `O_NOFOLLOW` +
    `fstat()` on the held descriptor and an `O_EXCL` re-create, so a pre-planted
    symlink at the salt path is never followed (no TOCTOU, no write-through). The
    deferred-write queue drops + counts (`dropped_writes`) under extreme
    saturation instead of falling back to a synchronous write, keeping disk
    latency strictly off the enforcement path.

## [1.23.0] - 2026-06-01

### Added (provisional)

- **AAP grant surface — experimental.** `@perform_action(grant="grant://...")`,
  plus `BrokerClient`, `GrantSession`, `current_grant`, `BrokerGrantError`, and
  `GrantDeniedError` (#266). An agent references a grant; the Secretless broker
  verifies the ATX, authorizes, resolves a scoped credential, performs the
  operation in an ephemeral worker, and returns only the result — no credential
  value or backend identifier ever enters the agent process. **Provisional:** the
  Agent Authorization Protocol is at spec v0.1, so this surface and the broker
  wire format it depends on may change in a future minor release without a major
  bump. Opt-in only: omitting `grant` leaves all existing behavior unchanged.

### Changed

- **Richer `aim-sdk login` output and install-first README quickstart** (#270).
- **OAuth login callback pages redesigned to match the AIM dashboard.** The
  browser "Login successful" / "Login failed" pages now use the OpenA2A logo,
  the dashboard's blue-600-on-white design language, and inline SVG status icons
  (replacing the previous purple-gradient card and emoji). Fully self-contained
  (logo inlined as a data URI; the Inter webfont degrades to the system stack
  offline).
- **`aim-sdk` terminal output is now emoji-free** and reads as clean professional
  prose (`login`, `logout`, `status`); exit codes carry the success/failure signal.
- **`aim-sdk version` output unified** with the new flag: both print `aim-sdk X.Y.Z`.

### Fixed

- **`tests/` no longer ships inside the wheel.** `setup.py` now passes
  `find_packages(exclude=["tests", "tests.*"])`, so the published distribution
  contains only the `aim_sdk` package.

- **`aim-sdk --version` / `aim-sdk -V`** now print the SDK version at the top
  level, complementing the existing `aim-sdk version` subcommand.

## [1.22.1] - 2026-06-01

First PyPI release since 1.21.0 (1.22.0 was prepared but never published), so
this release also carries the 1.22.0 changes listed below.

### Fixed

- **`register_agent` no longer doubles the error prefix.** When the backend
  rejected a registration, the message read `Registration failed: Registration
  failed: <reason>` because the outer handler re-wrapped a `ConfigurationError`
  the inner helper had already raised. `AIMError` subclasses are now re-raised
  unwrapped; generic exceptions are still wrapped exactly once. Regression tests
  added in `tests/test_register_agent.py`.
- **`aim-sdk login` callback pages now render UTF-8 correctly.** The local OAuth
  callback responses sent `Content-Type: text/html` with no charset, so browsers
  decoded the page as Latin-1 and the `✓` showed up as `âœ…`. Both the success
  and error responses now send `text/html; charset=utf-8`.

## [1.22.0] - 2026-05-25

### Security

- **Verification polling now Ed25519-signed and agent-scoped** (#160). The SDK route
  `GET /api/v1/sdk-api/verifications/<id>` was unauthenticated and returned the full
  verification event to any holder of the UUID, regardless of organization. The SDK
  now signs each poll with three required headers: `X-AIM-Agent-ID`,
  `X-AIM-Timestamp`, `X-AIM-Signature`. The backend rejects requests that lack the
  signature, fall outside a +/-5-minute clock-skew window, or reference a
  verification owned by a different agent (404, no existence oracle).

### Breaking

- **`_wait_for_approval` requires `self.signing_key` and `self.agent_id`.** Agents
  created without an Ed25519 keypair can no longer poll for verification approval;
  the SDK raises `VerificationError` with recovery guidance instead of issuing
  unauthenticated requests. The normal `secure(...)` / cached-credentials paths
  already provide both; only ad-hoc clients that constructed `AIMClient` without
  registering keys are affected.
- **`api_key`-only consumers can no longer poll the SDK verification route.**
  Pre-1.22 SDKs polled `GET /api/v1/sdk-api/verifications/<id>` with `X-API-Key`
  only; this endpoint is now signature-authed (defect #160 closure) and the
  API-key fallback no longer satisfies it. Migration:
    1. Register the consumer as an AIM agent (see `secure(...)` quickstart).
       This generates an Ed25519 keypair that `_wait_for_approval` will use
       automatically.
    2. If the consumer must remain API-key-only (CI service account, etc.),
       call `/api/v1/verifications/<id>` (the JWT-authed, org-scoped route)
       instead of the SDK route. Note the JWT route requires a user-session
       JWT, not an API key, so a dashboard auth flow is required.
  CI consumers that previously held `api_key` AND polled via `verify_action`
  with a non-`None` `_wait_for_approval` timeout will now raise
  `VerificationError` on `pending` responses.

### Fixed
- **Stale agent credentials no longer trigger silent re-registration** (#178). When the
  cached `~/.aim/agents/<name>.json` references an agent that no longer exists in the
  backend, the SDK now raises `StaleCredentialsError` with the local file path and
  recovery steps instead of deleting the file and registering a fresh agent under the
  same name. Re-registering against admin-curated agent state (status, trust score,
  granted capabilities, MCP grants) was wiping that state without notice; recovery is
  now an explicit operator action.
- **Bundled-SDK install path now prints a visible adoption warning** (#178). When the
  SDK adopts credentials from a bundled `.aim/sdk_credentials.json` (e.g., a fresh
  dashboard download in the working directory), the install prints which truncated
  token ID it is adopting and the long-term home path so operators can see when stale
  bundled tokens are entering the home store.

### Changed
- **Token rotation message names the prior refresh token as revoked** (#174). The
  print after a successful rotation now reads `Token rotated successfully — old
  refresh token revoked (new id: <8-char-prefix>...)` so the cause of out-of-band
  copies failing on the next refresh is obvious.
- **`print_token_expired_error` explains the root cause and surfaces the local
  path** (#174). The error text names refresh-token rotation as the most common
  cause, lists the four concrete situations that trigger it, and includes the exact
  `rm <path>` command instead of pointing the user at the dashboard UI alone.

### Removed
- **Encrypted shadow credential file** (`~/.aim/sdk_credentials.encrypted`, audit
  #12). The shadow file written alongside the JSON store became a write-only
  secondary source that no other code path read, and operators had to delete it
  manually to recover from corruption. `OAuthTokenManager` now treats
  `~/.aim/sdk_credentials.json` (mode 0600) as the single source of truth and
  performs a one-time migration on first instantiation: any leftover `.encrypted`
  file is decrypted into the JSON store and removed, or — if decryption fails —
  preserved in place so the operator can recover manually. The `use_secure_storage`
  and `allow_plaintext_fallback` constructor parameters are preserved for ABI
  stability and ignored.

### Added
- **`StaleCredentialsError`** (subclass of `ConfigurationError`). Raised from
  `register_agent()` when cached agent credentials reference an agent missing from
  the backend.

### Planned
- JavaScript/TypeScript SDK
- GraphQL API support

## [1.21.0] - 2026-02-03

### Changed
- **CLI Authentication**: Replaced password prompt with OAuth 2.0 + PKCE browser flow
  - `aim-sdk login` now opens browser for secure authentication (Google, etc.)
  - Uses PKCE (Proof Key for Code Exchange) per RFC 8252 - same as AWS CLI
  - No more password prompts or browser permission dialogs
  - Browser redirects directly to localhost - seamless experience

### Security
- PKCE prevents authorization code interception attacks
- State parameter prevents CSRF attacks
- Authorization codes are one-time use with 5-minute TTL

## [1.8.0] - 2025-12-10

### Added
- **Smart MCP Attestation System**: Intelligent, automatic attestation that builds trust in MCP servers for supply chain security
  - Attestations triggered on: first use, new tool, stale cache (>24h), capability drift
  - Zero friction - attestations happen automatically without developer effort
  - Caching prevents redundant attestations while maintaining security
- **`AttestationCache` Class**: New persistent cache for tracking attestation state
  - `should_attest()` - Determines if attestation is needed based on triggers
  - `record_attestation()` - Records successful attestations
  - `record_tool_usage()` - Tracks tool usage for analytics
  - `get_supply_chain_report()` - Generates supply chain analytics report
- **Capability Drift Detection**: Automatically detects when MCP servers change their tools
  - Severity levels: low (tools added), medium (tools removed), high (>30% change)
  - Triggers re-attestation when drift is detected
- **Supply Chain Analytics**: Track MCP tool usage for security visibility
  - `get_mcp_supply_chain_report()` - Local analytics for this agent
  - `report_mcp_supply_chain()` - Sync analytics to backend for dashboard
  - Usage patterns visible in AIM dashboard
- **Enhanced `use_mcp_tool()`**: Now includes smart attestation
  - `auto_attest` parameter (default: True) - Enable smart attestation
  - `force_attest` parameter - Force attestation even if cached
  - Returns attestation info and tool usage stats
- **New AIMClient Methods**:
  - `use_mcp_tool()` - Convenience method with smart attestation
  - `get_attestation_cache()` - Access attestation cache
  - `get_mcp_supply_chain_report()` - Local supply chain report
  - `report_mcp_supply_chain()` - Sync to backend

### Changed
- `use_mcp_tool()` now automatically triggers attestations when appropriate
- Tool usage is tracked locally for supply chain analytics
- Documentation updated with smart attestation and supply chain features

### Supply Chain Security Value
This release positions AIM as an MCP supply chain security platform:
- **Trust Graphs**: Visualize which agents trust which MCP servers
- **Anomaly Detection**: Alert when MCP servers suddenly change capabilities
- **Compliance**: Full audit trail of tool usage across the organization
- **Risk Assessment**: Identify high-risk MCP servers (low attestations, frequent changes)

## [1.7.0] - 2025-12-10

### Added
- **Dynamic MCP Capability Discovery**: SDK now automatically queries MCP servers for their actual tools, resources, and prompts using the official MCP protocol (`tools/list`, `resources/list`, `prompts/list`)
  - No more hardcoded capability lists - capabilities are discovered at runtime
  - Works with any MCP server, not just known ones
  - `discover_mcp_capabilities()` function for programmatic discovery
  - `auto_detect_mcps(discover_tools=True)` for full detection with tools
- **Auto-Attestation on Registration**: When agents register MCP servers via `secure()` or `register_mcp()`, the SDK automatically:
  - Discovers actual capabilities from the MCP server
  - Creates a cryptographically signed attestation
  - Submits the attestation to build trust in the MCP server
- **New `attest_mcp()` method**: Manual attestation API on AIMClient for attesting to MCP server capabilities
- **MCP Connection Recording**: `use_mcp_tool()` function to record agent-MCP connections

### Changed
- Removed hardcoded `KNOWN_MCP_CAPABILITIES` lookup table - capabilities now always discovered dynamically
- `_detect_mcp_capabilities_from_config()` now uses dynamic discovery exclusively
- MCP server registration flow now includes automatic attestation with discovered capabilities

### Improved
- MCP integration documentation with new sections on dynamic discovery and auto-attestation
- README updated with MCP capability discovery examples

## [1.4.0] - 2025-12-05

### Added
- **Server-Side Enforcement Control**: Admins can now configure enforcement mode in the dashboard UI (Settings → Security → Policies)
  - **Monitoring Mode** (default): Actions are logged but allowed to proceed when verification fails
  - **Strict Mode**: Actions are blocked immediately when verification fails
- SDK now respects the organization's enforcement mode from the backend
- Environment variable `AIM_STRICT_MODE` can still override the backend setting for testing purposes

### Changed
- Verification response now includes `enforcementMode` field to inform SDK of the organization's setting
- `@aim_verify` decorator uses backend enforcement mode by default, with env var as optional override
- Dashboard Policies page now shows enforcement mode toggle with clear explanations of each mode

### Fixed
- SDK no longer requires `AIM_STRICT_MODE` environment variable - it now reads the setting from the backend

## [1.3.0] - 2025-12-05

### Added
- **Execution Status Reporting**: SDK now reports whether decorated functions actually executed back to the backend
  - New `report_execution_status()` method on `AIMClient`
  - Decorators automatically report execution status after function calls
  - Dashboard shows accurate status: "Executed", "Blocked", or "Executed despite denial"
- **Strict Mode Documentation**: Comprehensive documentation for `AIM_STRICT_MODE` environment variable
  - Explains difference between monitoring mode (default) and strict mode (production)
  - Code examples for production deployments

### Changed
- `@aim_verify` decorator now tracks and reports:
  - Whether function was executed
  - Whether strict mode was enabled
  - Any execution errors that occurred
- Alert detail panel now displays execution status with clear messaging

### Fixed
- Dashboard alert messaging now accurately reflects what actually happened (blocked vs allowed)

## [1.2.4] - 2025-12-04

### Fixed
- OAuth token refresh flow fixed (camelCase consistency)
- Improved error handling in credential storage

## [1.1.0] - 2025-12-03

### Added
- **MCP Server Registration**: `agent.register_mcp()` method for programmatic MCP server registration
- **Capability Requests**: `agent.request_capability()` method for requesting additional capabilities
- **JIT Access Demo**: Interactive demo showing just-in-time access request workflows
- **Consolidated Demo Agent**: Single `demo_agent.py` with comprehensive feature demonstrations
- **Standardized Capability Format**: All capabilities now use `namespace:action` format consistently

### Changed
- Demo agents consolidated into single interactive `demo_agent.py`
- Improved credential migration and stale credential handling
- Enhanced demo UX with better credential discovery and alerts

### Fixed
- Trust score nil pointer handling
- Duplicate plaintext file deletion in credential migration
- Stale encrypted credentials cleared on new SDK install

## [1.0.0] - 2025-11-06

### Added
- **New Decorators**:
  - `@agent.perform_action()` - Verify and track actions with optional JIT access for admin approval
  - `@agent.require_approval()` - Require admin approval before executing critical actions
- **Versioning**: SDK download filename now includes version (e.g., `aim-sdk-python-v1.0.0.zip`)
- **VERSION File**: Single source of truth for SDK version at `/sdk/python/VERSION`
- **Documentation**:
  - Comprehensive decorator documentation with examples
  - Versioning strategy guide at `docs/VERSIONING.md`
  - Updated README with new decorator usage patterns

### Fixed
- **Critical: Ed25519 Signature Verification**:
  - Fixed JSON formatting mismatch between Python SDK and Go backend
  - SDK now uses `json.dumps(sort_keys=True, separators=(', ', ': '))` for consistent message format
  - Backend uses custom `customJSONFormat()` function to match Python exactly
  - Resolves signature verification failures caused by:
    - `"resource": null` vs `"resource": ""` differences
    - Space placement inconsistencies in JSON serialization
- **Critical: Credential Encryption**:
  - Fixed encryption bug in `secure_storage.py` when storing agent credentials
  - Credentials now properly encrypted and saved to `~/.aim/credentials.json`
- **API Key Middleware**:
  - Fixed middleware blocking verification endpoints with 401 errors
  - Verification endpoints now correctly use Ed25519 signature auth instead of API keys
- **Public Key Handling**:
  - Backend now accepts and uses SDK-provided public keys during agent registration
  - Fixes public key mismatch errors where backend was generating its own keys
- **Decorator Response Parsing**:
  - Fixed `AttributeError: 'dict' object has no attribute 'approved'`
  - Decorators now use `dict.get("verified", False)` for response parsing

### Changed
- **SDK Download Format**: Filename changed from `aim-sdk-python.zip` to `aim-sdk-python-v{version}.zip`
- **Agent Registration**: Backend now supports optional `public_key` field in `CreateAgentRequest`
- **Verification Flow**: Improved decorator implementation with proper error handling

### Deprecated
- `@agent.track_action()` - Use `@agent.perform_action()` instead
  - Will be removed in v2.0.0
  - Deprecation warnings added in v1.6.0

## [0.9.0] - 2025-11-05 (Pre-release)

### Added
- Initial SDK implementation
- `secure()` function for zero-config agent registration
- Ed25519 cryptographic signing
- Automatic capability detection
- MCP server detection from Claude Desktop config
- OAuth token management
- Basic decorator support with `@agent.perform_action()`

### Security
- Ed25519 cryptographic signatures for all agent communications
- Secure credential storage using OS keyrings
- Encrypted private key storage
- SHA-256 API key hashing

---

## Version Support

| Version | Status | Support Level | End of Support |
|---------|--------|---------------|----------------|
| 1.0.x   | ✓ Current | Full support | N/A |
| 0.9.x   | Warning: Pre-release | No support | Immediately |

## Migration Guides

### Upgrading from 0.9.x to 1.0.0

**Breaking Changes**: None

**New Features**:
- `@agent.perform_action()` decorator for verification and tracking
- `@agent.require_approval()` decorator for critical actions

**Recommended Usage**:

```python
# Standard usage - all actions verified and tracked
@agent.perform_action(risk_level="low", resource="database:users")
def query_users():
    return db.query("SELECT * FROM users")

# Critical actions with JIT access - requires admin approval
@agent.perform_action(risk_level="critical", jit_access=True, resource="database:users")
def delete_all_users():
    return db.execute("DELETE FROM users")  # ○ Pauses until admin approves

# Alternative: use @require_approval for critical actions
@agent.require_approval(risk_level="critical", resource="database:users")
def purge_data():
    return db.execute("TRUNCATE TABLE users")
```

**Action Required**:
- ✓ No immediate action required - 0.9.x code continues to work
- Warning: Update decorators to new style before v2.0.0 (recommended)

---

## Reporting Issues

Found a bug? Please report it:
- **GitHub Issues**: https://github.com/opena2a-org/agent-identity-management/issues
- **Email**: info@opena2a.org
- **Discord**: https://discord.gg/uRZa3KXgEn

---

**Last Updated**: 2025-12-10
