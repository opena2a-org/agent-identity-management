# Execution Isolation Trust Factor — Design Note v1.0

**Status:** Draft — implementation deferred pending `[CHIEF-CA]` decision on attestation source hierarchy
**Version:** 1.0.0
**Date:** 2026-05-24
**Closes investigation for:** #137

---

## 1. Why this document exists

The trust score breakdown advertises 9 factors that sum to 100%. Factor 9 — **Execution isolation, 10% weight** — is one of three factors the SECURITY model classifies as requiring external attestation, on the explicit basis that "an agent cannot self-attest into high trust."

This note records what Factor 9 actually does today, where the gap is between the SDK surface and a load-bearing backend signal, and what design decisions remain open before the factor can be wired to a non-self-attested source.

## 2. Current state

### 2.1 What exists in code

| Layer | Artifact | File:line |
|---|---|---|
| Schema | `isolation_attestations` table; `trust_scores.execution_isolation` column | `apps/backend/migrations/090_add_execution_isolation.sql:5-21` |
| Domain | `IsolationAttestation` struct; `IsolationAttestationRepository` interface; `ScoreIsolation` posture-to-score function | `apps/backend/internal/domain/isolation.go:58-149` |
| Trust calculator | `calculateExecutionIsolation` reads latest attestation; weight 0.10 | `apps/backend/internal/application/trust_calculator.go:126,497-509` |
| SDK (TypeScript) | `AIMClient.attestIsolation` posts to `/api/v1/sdk-api/agents/<id>/isolation-attestation` | `sdk/typescript/src/client/AIMClient.ts:432-465` |
| SDK (Python) | Equivalent path | `sdk/python/aim_sdk/client.py:1677` |
| Auto-detection | `autoDetectIsolation` reads `/proc/self/status`, `/.dockerenv`, `/proc/mounts` | `sdk/typescript/src/isolation/index.ts:109-159` |

### 2.2 What does NOT exist

| Missing piece | Consequence |
|---|---|
| Backend HTTP handler for `POST /api/v1/sdk-api/agents/<id>/isolation-attestation` | SDK calls to `attestIsolation` return 404 today |
| Production repository implementation of `IsolationAttestationRepository` | The table exists but no code writes to it from request handling |
| `TrustCalculator.SetIsolationRepo` is never called outside tests | `c.isolationRepo == nil` is always true in production, so `calculateExecutionIsolation` always returns the `0.3` baseline regardless of agent posture |
| External attestation ingestion (orchestrator / TEE / scanner) | Even with the SDK path wired, every score would be self-reported and a compromised agent could falsify it |

### 2.3 Net effect today

For every agent in every deployment, Factor 9 contributes a fixed `0.3 × 0.10 = 0.03` to the overall trust score. The 9-factor algorithm runs, the column populates, but no observable agent property — sandbox type, network egress, filesystem mode, MAC profile — can change the value.

This is honest if the doc says so. It was not previously documented. PR #220 (closing #132) made the factor's `0.3` baseline visible in the trust-scoring doc; this design note documents the deeper reason the baseline never moves.

## 3. Threat model — why self-attestation cannot be load-bearing

An agent claiming `sandbox=firecracker, network=airgap, filesystem=readonly, process=full` produces an isolation score of `1.0` from `ScoreIsolation`. The data is internally consistent; the values are valid enum members; nothing in the current path verifies that the agent process is actually inside a Firecracker VM, on an air-gapped network, with a read-only rootfs, under a seccomp + MAC profile.

A compromised agent — or a malicious operator — submits the same payload. The score is identical. The signal is non-falsifiable only if it comes from outside the agent's own process.

This is the position the issue body took, and the position this design note adopts: **Factor 9 should not move above its baseline on the strength of an SDK-submitted attestation alone.**

## 4. Proposed attestation sources (`[CHIEF-CA]` open question)

In order of trust:

| Tier | Source | What it can attest | Falsifiability |
|---|---|---|---|
| 1 | Confidential computing / TEE remote attestation (SEV-SNP, TDX, Nitro Enclaves) | Process is running inside a measured enclave with a known image | Cryptographically bound to vendor root of trust |
| 2 | Orchestrator security context report (Kubernetes API, Nomad job spec, ECS task definition) | Pod is running with declared securityContext, networkPolicy, mount restrictions | Requires the agent to be unable to forge orchestrator API responses (network identity check) |
| 3 | HackMyAgent scan of the deployed surface | The deployed artifact matches a known-isolated configuration | Bound to scanner identity; static — drifts if runtime configuration changes after scan |
| 4 | SDK self-attestation (current path) | Agent's process self-reports what it sees from `/proc` | Falsifiable; useful only as a diagnostic, not for scoring |

The open question — and the reason this is `[CHIEF-CA]` rather than a straightforward implementation — is which tier(s) AIM should support as Phase 1, and how the calculator should combine them when multiple sources are present. Specifically:

- Should missing higher-tier evidence pin the score at baseline (strict), or should a lower tier raise it modestly (lenient)?
- Where does the source identity (orchestrator URL, TEE measurement root, scanner pubkey) live, and who registers it as trusted?
- What is the freshness window for an attestation before it decays back to baseline?
- How are attestations bound to the agent identity such that a valid attestation from agent A is not replayable by agent B?

None of these are answered yet. The architecture decision belongs to `[CHIEF-CA]` and must precede the backend handler implementation.

## 5. Why the existing SDK path is still useful

Even if SDK self-attestation cannot move the trust score, the `attestIsolation` data has two legitimate uses:

1. **Diagnostic** — operators can see what the agent thinks its isolation looks like, compared against what the orchestrator reports. Disagreement is itself a signal.
2. **Drift hint** — repeated changes to the SDK-reported posture (e.g., the same agent reporting `sandbox=docker` one hour and `sandbox=none` the next) is suspicious independent of the absolute value.

A reasonable Phase 1 keeps the SDK ingestion endpoint, persists the attestation, and uses it for diagnostic surfaces — but `calculateExecutionIsolation` reads from a higher-tier source for the score.

## 6. Phasing — not a commitment, a shape

Recorded for the `[CHIEF-CA]` review, not as a roadmap:

- **Phase 1 (handler-only).** Implement the backend POST handler so `attestIsolation` stops 404'ing. Persist to `isolation_attestations`. Wire `SetIsolationRepo`. The score still reads from the SDK attestation under this phase, with the caveat that it is self-attested and therefore not adversarially robust. This unlocks the diagnostic use case.
- **Phase 2 (one external source).** Pick the first non-self source — likely Kubernetes orchestrator context, since most production agents run there — and wire its ingestion. Change `calculateExecutionIsolation` to prefer external evidence over self.
- **Phase 3 (multi-source combiner).** Add TEE and HMA tiers. Define the combiner. Define the freshness decay.

The choice of Phase 1 alone — that is, "stop the 404 but keep the score self-attested" — is not safe to ship without a clear product disclosure that the factor remains self-attested. The decision to ship Phase 1 alone, or to gate handler enablement on Phase 2 readiness, is the first `[CHIEF-CA]` question.

## 7. What this document does NOT do

- Does not change the trust calculator code path.
- Does not add a backend handler.
- Does not call `SetIsolationRepo`.
- Does not change the score for any agent.

It documents the gap so the next reader (or the next session) is not surprised by the `0.3` baseline, and so the design discussion can start from a shared statement of the current state.

## 8. References

- Issue #137 — Trust factor "Execution isolation" (10%): document and implement the external-attestation scoring path.
- PR #220 — `docs(#132): sync trust-scoring doc to 9-factor algorithm; add liveness matrix` (introduced the Factor 9 doc section and noted the `0.3` baseline).
- `docs/sdk/trust-scoring.md` — Factor 9 section.
