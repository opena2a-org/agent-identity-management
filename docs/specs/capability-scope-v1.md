# Capability Scope (`capability_scope` JSONB) — Design Note v1.0

**Status:** Draft — design open pending `[CHIEF-CA]` decision on unification with FGA policies
**Version:** 1.0.0
**Date:** 2026-05-24
**Closes investigation for:** #130

---

## 1. Why this document exists

The 5-step Fine-Grained Authorization (FGA) model attributes attribute-level scoping ("which fields, which actions") to Step 2. The `agent_capabilities.capability_scope` JSONB column was added in migration 016 with the apparent intent of carrying that scoping data — `{"columns": ["name", "email"]}` for a `db:read` grant, `{"recipients": ["internal@example.com"]}` for an `email:send` grant, and similar.

This document records what `capability_scope` actually does today, that there are two parallel authorization paths in AIM with very different scope handling, and the open question of how (or whether) to unify them.

## 2. Current state — two enforcement paths

### 2.1 Path A: Simple matcher (`VerifyCapability` / SDK `verify_capability`)

This is the path the SDK has been hitting since v0.x: the agent calls `verify_capability(capability, resource, context)` and the backend resolves whether the agent holds a matching grant.

| Surface | File:line |
|---|---|
| SDK entry (Python) | `sdk/python/aim_sdk/client.py:587` |
| Backend service | `apps/backend/internal/application/agent_service.go:826` (`VerifyCapability`) |
| Matcher | `apps/backend/internal/application/agent_service.go:1369-1389` (`matchesCapability`) |

The matcher signature accepts `resource` as a parameter — but the function body **never reads it**. Behaviour is exact-match on the capability type string plus a trailing-`*` wildcard prefix (`file:*` matches `file:read`).

The matcher **never reads `capability.CapabilityScope`** either. A grant of `db:read` with `capability_scope = {"columns": ["name"]}` resolves identically to a grant of `db:read` with no scope. The grant API persists the JSON; the verification path silently ignores it.

The TODO comment at `agent_service.go:1383-1386` explicitly anticipates this:

```
// Future: Add more sophisticated pattern matching here
// - Resource-based matching (e.g., "file:read:/data/*")
// - Time-based capabilities
// - Context-aware matching
```

### 2.2 Path B: FGA engine (`/api/v1/fga/authorize`)

This is the newer, fuller path: explicit per-capability policy rows in `fga_policies`, evaluated by `FGAEngine.Authorize` through the 5-step model.

| Surface | File:line |
|---|---|
| HTTP entry | `apps/backend/internal/interfaces/http/handlers/authorize_handler.go:127` |
| Engine | `apps/backend/internal/application/fga_engine.go:309` (`Authorize`) |
| Step 2 attribute check | `apps/backend/internal/application/fga_engine.go:721-764` (`checkAttributes`) |
| Policy storage | `fga_policies` table with `allowed_attributes`, `denied_attributes`, `allowed_objects`, `allowed_actions` columns |

Here Step 2 IS implemented. `req.Attributes[]` is matched against `policy.AllowedAttributes[]` and `policy.DeniedAttributes[]` using `matchPattern`. An agent calling `/authorize` with `attributes: ["email", "phone"]` against a policy that lists `allowed_attributes: ["name"]` is denied with `DENY_ATTRIBUTE` and reason `Attribute 'email' not in allowed list`.

**But the FGA engine does not read `agent_capabilities.capability_scope`.** It reads `fga_policies`. The two storage locations are separate and unsynchronised; granting a capability with a scope does not create an FGA policy with corresponding attribute restrictions.

### 2.3 Summary

| Question | Path A (`VerifyCapability`) | Path B (`FGAEngine.Authorize`) |
|---|---|---|
| Reads `capability_scope` JSONB? | No | No |
| Reads `resource` parameter? | No (declared, ignored) | Yes, against `allowed_objects` |
| Enforces attribute scoping? | No | Yes (from `fga_policies`) |
| Used by SDK `verify_capability` | Yes | No (separate endpoint) |
| Used by `/api/v1/fga/authorize` | No | Yes |

The implication for operators: setting `capability_scope` on a grant is purely decorative under Path A, and Path B ignores it entirely. The attribute-level scoping that the 5-step deck promises only flows through Path B, and only when an `fga_policies` row exists for the capability — not from the JSON on the grant.

## 3. Workaround today

If you need attribute-level granularity under Path A — for example, because your agents call `verify_capability` rather than `/authorize` — encode it into the capability name:

| Instead of | Use |
|---|---|
| `db:read` with `scope: {"columns": ["name", "email"]}` | grants for `db:read:name` and `db:read:email` separately |
| `email:send` with `scope: {"recipients": ["internal@example.com"]}` | a grant for `email:send:internal@example.com` |
| `file:read` with `scope: {"paths": ["/data/public"]}` | a grant for `file:read:/data/public` |

The wildcard prefix matcher honours these (`db:read:*` covers all subtypes), and the exact-match path enforces the leaf strictly. The convention is non-obvious — that is the cost of relying on the legacy path.

For deployments that can move to Path B, write `fga_policies` rows directly. The FGA engine enforces attribute scoping there as designed.

## 4. Open questions (`[CHIEF-CA]`)

| Question | Why it matters |
|---|---|
| Should `capability_scope` be deprecated and removed, or wired into a unified matcher? | The column exists, is non-empty in some deployments, and represents an unmet promise to operators. Leaving it as decoration risks future confusion. |
| If wired in, should the simple matcher (`matchesCapability`) read it, or should `capability_scope` compile into `fga_policies` rows at grant time? | Wiring the simple matcher is local; compiling into FGA is more architecturally honest but requires a migration of existing rows. |
| Should `CapabilityService.GrantCapability` reject grants that supply a non-empty `capability_scope` until enforcement lands? | Converts silent bypass into refusal. Has API-contract impact for SDK callers who pass scope today. |
| What is the canonical migration path off Path A onto Path B for the SDK? | SDK callers use `verify_capability`; Path B is `/authorize`. Either the SDK adopts `/authorize`, or `VerifyCapability` is upgraded to consult `fga_policies` under the hood. |

These are entangled. The decision belongs to `[CHIEF-CA]` and should precede any code change.

## 5. What this document does NOT do

- Does not change `matchesCapability`.
- Does not add scope evaluation under Path A.
- Does not reject grants that include a non-empty scope.
- Does not migrate existing `capability_scope` JSON into `fga_policies` rows.
- Does not change the SDK contract.

It records the two-path reality so the next reader (or the next session) is not surprised by the decorative behaviour of `capability_scope`, and so the design discussion can start from a shared statement of the current state.

## 6. References

- Issue #130 — FGA Step 2 (Attribute): capability_scope JSONB is stored but not evaluated by the matcher.
- `SECURITY.md` NIST SP 800-53 AC-6 row — original disclosure that attribute-level scoping is plumbed but not enforced.
- Migration 016 — created `agent_capabilities.capability_scope`.
- `apps/backend/internal/application/agent_service.go:1369` — `matchesCapability` matcher (legacy path).
- `apps/backend/internal/application/fga_engine.go:309` — FGA engine `Authorize` (newer path).
