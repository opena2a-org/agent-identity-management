# Changelog

All notable changes to `@opena2a/aim-sdk` are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this package adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

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
