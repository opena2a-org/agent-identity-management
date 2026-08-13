# Status: stable

**Stage:** stable
**Maintenance horizon:** actively maintained through 2026-12-31, status reviewed quarterly
**Maintainer wanted:** no

## Stage rationale

AIM 1.0. Every gate criterion in [HARDENING.md](HARDENING.md)'s "Roadmap to 1.0" is met as of 2026-05-28 (last criterion — the CI-integrated Playwright empty-state suite — closed in PR #247 + PR #248 + PR #250). Semver is honored from this release forward; breaking changes go through a deprecation cycle and ship in a major bump. Security reports are handled per the policy in [SECURITY.md](SECURITY.md).

A paid third-party security review was previously listed as a 1.0 criterion but has been [rescoped to a post-1.0 investment](HARDENING.md#third-party-security-review) (CHIEF-CSR decision 2026-05-28): AIM is open source under Apache-2.0, so the codebase itself is the audit surface, and a community-review path through SECURITY.md is welcome before and after 1.0. A paid engagement is appropriate when a regulated customer requires formal attestation.

## Stage definitions

- **stable**: production-ready, semver honored, breaking changes documented, security reports handled per [SECURITY.md](SECURITY.md).
- **beta**: feature-complete or near, breaking changes possible with notice, actively developed.
- **experimental**: early stage, breaking changes expected, use at your own risk.
- **reference-only**: spec or reference implementation, not intended for production use.

## Status changes

| Date | Stage | Reason |
|---|---|---|
| 2026-05-24 | beta | initial STATUS.md (org lifecycle introduction). Maps to existing pre-1.0 disclaimer in README. |
| 2026-05-28 | stable | every Roadmap to 1.0 gate criterion in HARDENING.md met. Empty-state suite landed (#247) and CI-integrated (#248); HARDENING.md ☐ → ☑ in #250. Per-package SDK publish wiring complete (#251, #253, #254). Container image publish wiring aligned to convention (#255). Ready for `platform-v1.0.0` tag. |
