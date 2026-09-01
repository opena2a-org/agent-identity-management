# Changelog

All notable changes to `@opena2a/aim-sdk` are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this package adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

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
