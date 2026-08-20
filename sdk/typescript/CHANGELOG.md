# Changelog

All notable changes to `@opena2a/aim-sdk` are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this package adheres to [Semantic Versioning](https://semver.org/).

## [1.2.0] - 2026-08-11

### Fixed

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
