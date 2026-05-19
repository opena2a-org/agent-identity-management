# Hardening

AIM is pre-1.0 software in active hardening. This page tracks the work that needs to land before we recommend production deployment.

## Current status

**Pre-production.** AIM's architecture, threat model, and core enforcement primitives (capability authorization, trust scoring, monitoring/strict modes, cryptographic identity, audit logging) are in place. The integration layer between the SDK, the backend services, and the dashboard has known gaps that the team is actively closing.

We are documenting this openly because pretending a pre-1.0 system is production-ready is more harmful than saying it is not. AIM is suitable today for evaluation, development, internal testing, and demonstration. It is not suitable for production deployments handling untrusted tenants or sensitive data.

## What we are working on

The hardening work is organized into the following streams. Each is tracked internally and surfaces as patches to `main` over time. Specific defect details are kept private until fixes ship, per the disclosure policy in [SECURITY.md](SECURITY.md).

### Tenant scoping

We are auditing every authenticated endpoint to ensure that requests touching a resource by id verify the resource belongs to the caller's organization. Today this check is enforced ad-hoc per handler. The target state is enforcement at a middleware boundary so the check cannot be forgotten.

### Bootstrap secrets and configuration

We are removing default fallback values from environment variables that carry secrets. The current deployment configuration includes default values for development convenience; deployments that do not override them inherit weak credentials. The target state is fail-fast on any required secret that is not explicitly set, and first-boot generation of bootstrap credentials with a one-time print to the operator.

### API response contracts

We are auditing list-style API responses to ensure empty results return an empty array rather than null. Some handlers currently return null when the underlying query returns zero rows, which can crash strict consumers. The target state is consistent contract enforcement at the handler layer plus generated TypeScript types on the dashboard to prevent regressions.

### Data model invariants

Several entities have data stored in two places (a denormalized column on the parent table plus a richer detail table). Today these are kept in sync by application code. We are reviewing each pair to either enforce the invariant at the database layer (triggers, computed views) or refactor to a single source of truth.

### Capability lifecycle

The capability approval workflow exists in the backend and correctly distinguishes monitoring mode (auto-approve) from strict mode (require admin review). We are routing the SDK registration path through this workflow so that re-registering an agent with new capabilities goes through the same gate as any other capability change. The current SDK path grants declared capabilities directly.

### Dashboard rendering on empty state

We are auditing every dashboard tab on the agent and MCP server detail pages to ensure fresh resources with no history render the appropriate empty-state message rather than a generic error boundary. The target is a Playwright suite that covers every panel against a freshly-created resource.

### Post-quantum cryptography surfacing

The backend implements ML-DSA 44/65/87 (FIPS 204) and supports hybrid Ed25519 plus ML-DSA registration. The agent detail dashboard does not yet render these fields. We are adding the matching UI so that the cryptographic identity panel reflects the full key material the backend actually stores.

## Our process

We treat security and stability work as first-class commitments, not background polish.

- Every fix lands with a regression test that fails on the old behavior.
- Architectural changes (middleware introductions, schema refactors, contract changes) go through internal architecture review before merging.
- We use draft GitHub Security Advisories for issues that affect deployments running today. They become public after patches ship.
- We do not publish defect details that read like attack recipes ahead of fixes. We do publish category-level work, like this page, so the community can see active investment.
- Once each item in this list is closed, the corresponding entry will be marked complete here, the relevant Security Advisory (if any) will be published, and the change will appear in the CHANGELOG with a brief explanation.

## Reporting issues

If you find a security issue, please follow the responsible disclosure policy in [SECURITY.md](SECURITY.md). For non-security issues (correctness, stability, UX gaps), open a GitHub issue.

We particularly welcome reports on:

- Cross-tenant data exposure
- Authentication or authorization bypass
- Capability or trust score manipulation that does not require admin
- Cryptographic identity forgery
- Dashboard crashes on empty or unexpected state

## Roadmap to 1.0

The 1.0 milestone is defined by the completion of the streams above plus:

- A passing end-to-end test suite covering empty-state rendering on every dashboard panel
- A contract test asserting that no API endpoint returns null for an array field
- A regression test suite for the capability approval workflow under both monitoring and strict modes
- Middleware-level tenant scoping enforced and proven by negative tests
- No hardcoded secrets in the codebase, no default fallbacks for secret-shaped environment variables
- A clean third-party security review

We will update this page as items close. We will not move to 1.0 until they all do.

## Acknowledgements

This hardening agenda was informed by an internal end-to-end exercise running AIM against realistic demo scenarios. The exercise surfaced gaps the team had not yet prioritized, and we are using it as the input for the work plan above. We will publish a more detailed retrospective once the corresponding fixes have shipped.
