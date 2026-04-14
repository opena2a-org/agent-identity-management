# Bump fiber v3.1.0 + apps/web CVEs — public `agent-identity-management`

**Date:** 2026-04-14
**Chief:** CA (architecture, deps/build chain) with CSR sign-off (CVEs)
**Driver:** Port aim-cloud PR #8 (merged 2026-04-14) to the public repo. Flagged as follow-up in that PR body because `go.mod` + `package.json` are not in `.sync-protect` and the next sync would regress aim-cloud.
**Playbook:** `~/.claude/projects/-Users-ecolibria-workspace-opena2a-org/memory/project_fiber_v3_1_0_migration_playbook.md` (applied once, validated in CI + prod deploy of aim-cloud).

## CVEs closed

Backend (`apps/backend/go.mod`):
- **CRITICAL** `github.com/gofiber/utils/v2 v2.0.0-beta.4` → **v2.0.2** (CVE-2025-66565, UUID silent fallback to predictable values — blast radius: any code calling `utils.UUIDv4()` on this repo gets deterministic-looking UUIDs on error).
- **HIGH** `github.com/gofiber/fiber/v3 v3.0.0-beta.2` → **v3.1.0** (CVE-2026-25891, CVE-2026-25899).
- `golang.org/x/crypto` is already at **v0.49.0** (safe; CVE-2025-22869 patched at 0.35.0). No action.

Frontend (`apps/web/package.json`):
- **HIGH** `next ^16.1.0` → **^16.2.3** (GHSA-h25m-26qc-wcjf, GHSA-q4gf-8mx6-v5v3).
- **HIGH** Add `overrides`: `lodash ^4.18.1` (CVE-2026-4800), `picomatch` `^2.3.2` / `^4.0.4` (CVE-2026-33671).

## Repo-specific deltas vs aim-cloud

Confirmed by grep before drafting:

| Item | aim-cloud state | public repo state | Work needed |
| --- | --- | --- | --- |
| `go.mod` go directive | `go 1.23` → `1.25.0` | already `1.25.0` | none |
| CI `setup-go` | `1.23` → `1.25` | already `1.25` (release/security/ci/sync workflows) | none |
| `Dockerfile.backend` | `golang:1.23-alpine` → `1.25-alpine` | **still `golang:1.23-alpine`** (drift) | bump to `1.25-alpine` |
| `cors.Config` comma-joins | 1 site | 1 site (`middleware/cors.go:59-62`, 4 fields + `strings.Join(safeOrigins, ",")`) | convert all 4 to `[]string`, drop `strings.Join` |
| `c.Context().Time()` | 1 handler | **2 sites only** (`handlers/agent_handler.go:560`, `:579`) | replace with `time.Now()` at handler entry + `time.Since(start)` for duration |
| Content-Type exact-match tests | 1 test | **0 sites** found | none |

**Net code change surface:** `~3 files, ~8 lines` of Go + `package.json` + 1 Dockerfile line.

## Non-goals (do NOT touch in this PR)

- `fix/deploy-azure-digest-pinning` branch (session-68, active). Stay on a separate branch off `main`.
- `.sync-protect` wiring. Follow-up, separate PR. Adding `go.mod`/`package.json` to `.sync-protect` is a policy decision that deserves its own review (it would block aim-cloud→public divergence, but a future-version-of-aim-cloud fiber bump would then stall on this repo).
- Migration-number drift (`aim-cloud` 081-084 missing). Out of scope.
- Go 1.25 anywhere outside Dockerfile.backend. All CI already on 1.25.

## Risks + mitigations

1. **Large transitive churn.** fiber v3.1.0 pulls `x/net v0.50.0`, `x/sys v0.41.0`, `x/text v0.34.0`, `x/crypto ≥ 0.48.0`. Mitigation: already validated in aim-cloud (same Go code paths, same downstream deps). `go mod tidy` + `go test -race ./internal/...` locally before push.
2. **Contributor Go toolchain friction.** Public repo → external contributors may run Go 1.24 or earlier locally. `go.mod` already pins 1.25.0 (pre-existing), so this PR does not change the floor. No new friction.
3. **Dockerfile `1.23-alpine`** is pre-existing drift — if it's building on CI, CI must be downloading 1.25 toolchain inside a 1.23 base. Verify by checking latest successful `main` image build or confirming the `go 1.25.0` directive triggers auto-toolchain download. Either way, bumping the base image is strictly an improvement.
4. **aim-cloud / public drift after merge.** aim-cloud was bumped yesterday; this PR brings public to parity. `.sync-protect` follow-up in a separate PR.

## Sequence

1. Branch `fix/fiber-v3.1.0-bump` off `origin/main`.
2. Edit `apps/backend/go.mod`: bump `gofiber/fiber/v3` to `v3.1.0`. `go mod tidy` — let it pull `utils/v2 v2.0.2` transitively.
3. Edit `apps/backend/internal/interfaces/http/middleware/cors.go`: convert 4 cors.Config string fields to `[]string`, drop `strings.Join(safeOrigins, ",")`.
4. Edit `apps/backend/internal/interfaces/http/handlers/agent_handler.go:560-579`: replace `c.Context().Time()` pattern (`startTime := c.Context().Time()` + `c.Context().Time().Sub(startTime)`) with `startTime := time.Now()` + `time.Since(startTime)`. Add `"time"` import if missing.
5. Edit `apps/backend/infrastructure/docker/Dockerfile.backend`: `golang:1.23-alpine` → `golang:1.25-alpine`.
6. Edit `apps/web/package.json`: `next ^16.1.0` → `^16.2.3`; add `"overrides": {"lodash": "^4.18.1", "picomatch": "^2.3.2"}`.
7. `cd apps/backend && go build ./... && go vet ./... && go test -race ./internal/...`.
8. `cd apps/web && npm install && npm run build && npm run lint`.
9. Commit, push, open PR. Self-review via `/review`, address findings, `/pre-push-review`, merge.

## Definition of done

- Trivy CRITICAL + HIGH count on this PR ≤ the count on current `main` for Go deps (specifically: CVE-2025-66565, -2026-25891, -2026-25899 gone; next/lodash/picomatch GHSAs gone).
- `go test -race ./internal/...` passes locally.
- `npm run build` + `npm run lint` pass in `apps/web`.
- PR self-review via `/review` addressed.
- Pre-push gate marker present before push.

## Chief call

[CHIEF-CA] DECISION: apply this bump via a dep-only PR on `fix/fiber-v3.1.0-bump` off main, independent of session-68's CI-workflow branch. RATIONALE: CRITICAL CVE on public repo; playbook validated in aim-cloud 24h ago; code surface is 3 files; toolchain floor already 1.25 so no contributor-facing regression. ALTERNATIVES REJECTED: (a) wait for `.sync-protect` reshuffle — leaves CRITICAL exposed; (b) fold into session-68's branch — mixes concerns, blocks either landing; (c) patch-bump `fiber/v3 v3.0.0-beta.2` to a later beta — no beta carries the CVE-2025-66565 fix. ESCALATION: loop in CSR only if Trivy surfaces a new CVE post-bump.
