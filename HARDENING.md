# Hardening

AIM is pre-1.0 software in active hardening. This page tracks the work that needs to land before we recommend production deployment.

**Last updated:** 2026-05-28. Every enforcement stream and gate criterion is now closed in code, including the CI-integrated Playwright empty-state suite (PR #247 + #248). AIM is open source under Apache-2.0, so community review of the codebase is welcome before and after 1.0 per [SECURITY.md](SECURITY.md); a paid third-party engagement is a worthwhile post-1.0 investment for regulated deployments but is not a binding 1.0 gate.

## Current status

**Pre-production.** AIM's architecture, threat model, and core enforcement primitives (capability authorization, trust scoring, monitoring/strict modes, cryptographic identity, audit logging) are in place. The integration layer between the SDK, the backend services, and the dashboard has been audited and the contract enforcement gaps are largely closed.

We are documenting this openly because pretending a pre-1.0 system is production-ready is more harmful than saying it is not. AIM is suitable today for evaluation, development, internal testing, and demonstration. It is not suitable for production deployments handling untrusted tenants or sensitive data — see the **Roadmap to 1.0** section below for the specific remaining blockers.

## What we are working on

Each stream lists its current status. Specific defect details are kept private until fixes ship, per the disclosure policy in [SECURITY.md](SECURITY.md).

### Tenant scoping

**Status: in progress.** Every authenticated handler that touches a resource by id verifies the resource belongs to the caller's organization. Today this is enforced per-handler via a `LoadOwned` helper, with a CI lint (`scripts/lint-tenant-scoping.sh`) that fails the build on any handler reading a tenant-scoped URL parameter without invoking a registered tenant-scoping helper (`LoadOwned`, `LoadOwnedViaAgent`). Route-binding consistency (so the handler reads the parameter the route actually binds) is exercised by integration cross-tenant test suites rather than by the lint. The target state remains enforcement at a middleware boundary so the check cannot be bypassed by a future handler that forgets the lint pattern. The risk floor is mitigated by the lint plus the integration tests; the architectural target is not yet reached.

Coverage in main today: organization-scoped handlers for agents, MCP servers, capability requests, policies, A2A consents, attestations, and admin endpoints; cross-tenant negative tests across ~11 handler suites.

### Bootstrap secrets and configuration

**Status: done.** Default fallback values have been removed from secret-shaped environment variables. The configuration validator (`apps/backend/internal/config/config.go`) fails fast on missing `JWT_SECRET` and `KEYVAULT_MASTER_KEY`, enforces a 32-character minimum, and rejects known-dev secrets in every environment via a hash-only blocklist. `docker-compose.yml` uses the `${VAR:?...}` fail-fast pattern for required secrets, enforced by `scripts/lint-no-secret-fallbacks.sh`. First-boot bootstrap (`apps/backend/cmd/bootstrap/`) generates a random admin password and prints it once to the operator.

### API response contracts

**Status: done.** List-style API responses return `[]` for empty results, never `null`. Enforcement is layered: list-returning slices are initialized via `make([]T, 0)` across handlers, repositories, and application services, and `scripts/lint-jsonarray.sh` fails the build on any new `var xs []T` declaration in those layers. Regression tests pin the JSON shape on representative endpoints (trust score history, agent MCP servers, agent alerts).

### Data model invariants

**Status: done.** Each known data-model invariant — parent cache columns plus richer detail tables, and row-internal couplings where two columns on the same row must move together — is now enforced at the database layer rather than by application code. Six invariant-preserving triggers cover trust scores → agent cache, MCP trust scores → MCP server cache, capability violations → agent count, MCP server capabilities → cache, A2A peer trust aggregates, and agent status → `verified_at` timestamp. The A2A consent cross-tenant invariant is enforced by a complementary `NOT NULL` constraint on `a2a_consent_records.organization_id` (migration 097, backfilled from the grantor agent). Tests assert the trigger and constraint behavior under insert, update, and delete.

### Capability lifecycle

**Status: done.** The SDK registration path routes through the capability approval workflow. Monitoring mode auto-approves and grants; strict mode creates a pending request and refuses to grant until an admin approves. Both first-registration baseline and re-registration with new capabilities are covered. The 7-test regression suite at `apps/backend/internal/application/agent_service_capability_lock_test.go` exercises both modes, the baseline-grant case, the strict-mode pending-no-grant case, capability removal, and a fail-loudly assertion if the approval service is nil.

### Dashboard rendering on empty state

**Status: done.** Empty-state messages are implemented on the agent and MCP server detail pages in code, and the corresponding Playwright suite (`apps/web/tests/e2e/empty-state-agent.spec.ts` and `apps/web/tests/e2e/empty-state-mcp.spec.ts`) exercises every tab on a freshly-registered resource against a real backend in CI on every PR (the `e2e-empty-state` job in `.github/workflows/ci.yml`). Four panels — api-keys, activity, and trust on the agent detail page, and audit trail on the MCP detail page — are reclassified N/A-by-design for empty-state copy because backend auto-population on resource creation (default API key, audit event for the registration itself, default trust score, Activity Timeline summary) makes the empty branch unreachable through the fixture path; those four are still exercised for "panel renders without crash." Evidence: PR #247 introduced the suite; PR #248 closed the CI job over several iterations against the real backend.

### Post-quantum cryptography surfacing

**Status: done.** The backend implements ML-DSA 44/65/87 (FIPS 204) and supports hybrid Ed25519 plus ML-DSA registration. The Identity and Signing tab on the agent detail page renders the ML-DSA algorithm, a truncated post-quantum public key with copy-to-clipboard, the hybrid-mode badge, and the lifecycle dates. The agent detail API response and the key-vault endpoint both include the PQC fields. Regression tests pin the response shape so the surfacing cannot silently regress.

### Third-party security review

**Status: rescoped — post-1.0.** AIM is open source under Apache-2.0. The codebase, threat model, and architectural decisions are publicly auditable, and the community is welcome to review and report findings per [SECURITY.md](SECURITY.md). A paid third-party engagement is a worthwhile post-1.0 investment when the deployment surface includes regulated tenants, but it is not a binding 1.0 gate criterion. Decision recorded 2026-05-28.

## Our process

We treat security and stability work as first-class commitments, not background polish.

- Every fix lands with a regression test that fails on the old behavior.
- Architectural changes (middleware introductions, schema refactors, contract changes) go through internal architecture review before merging.
- We use draft GitHub Security Advisories for issues that affect deployments running today. They become public after patches ship.
- We do not publish defect details that read like attack recipes ahead of fixes. We do publish category-level work, like this page, so the community can see active investment.
- Each closed stream above carries a short evidence pointer. The corresponding security advisories (where applicable) will publish on the 1.0 cut.

## Reporting issues

If you find a security issue, please follow the responsible disclosure policy in [SECURITY.md](SECURITY.md). For non-security issues (correctness, stability, UX gaps), open a GitHub issue.

We particularly welcome reports on:

- Cross-tenant data exposure
- Authentication or authorization bypass
- Capability or trust score manipulation that does not require admin
- Cryptographic identity forgery
- Dashboard crashes on empty or unexpected state

## Roadmap to 1.0

The 1.0 milestone is defined by the gate criteria below. Status as of 2026-05-28:

- [x] A contract test asserting that no API endpoint returns null for an array field
- [x] A regression test suite for the capability approval workflow under both monitoring and strict modes
- [x] No hardcoded secrets in the codebase, no default fallbacks for secret-shaped environment variables
- [x] Tenant scoping enforced on every authenticated handler with cross-tenant negative tests (risk floor met via per-handler enforcement plus CI lint; architectural target of middleware-level enforcement remains a stretch goal)
- [x] A passing end-to-end Playwright suite covering empty-state rendering on every dashboard panel, integrated into CI

We will update this page as items close. As of 2026-05-28, every gate criterion above is met (the empty-state suite landed in PR #247 and was unblocked into a passing CI job by PR #248). A paid third-party security review was previously listed as a 1.0 criterion; it has been rescoped to a post-1.0 investment because AIM is open source and the codebase is publicly auditable today (see the "Third-party security review" stream above for the full decision).

## Acknowledgements

This hardening agenda was informed by an internal end-to-end exercise running AIM against realistic demo scenarios. The exercise surfaced gaps the team had not yet prioritized, and we used it as the input for the work plan above. We will publish a more detailed retrospective once the corresponding fixes have shipped.
