# AIM Release Smoke Tests

This document is the per-repo release smoke test contract for `agent-identity-management`. Every PR that ships changes to a piece of the system MUST run the smoke test for that piece BEFORE asking a reviewer to merge — not "I ran the unit tests" or "I ran `docker compose config -q`," but **the actual binary boots, accepts traffic, and the externally-observable signal that the change targeted is verifiable**.

This was written 2026-04-28 after the OTel exporter PR (#114) was almost merged with five real bugs that no unit test, lint, or syntax check would have caught. The bugs:

| Bug | Where caught | Caught by |
|---|---|---|
| `otel/opentelemetry-collector-contrib:0.115.0` does not exist on Docker Hub | `docker compose up` | smoke harness |
| Stale named containers from prior failed runs blocked compose up | `docker compose up` | smoke harness |
| Loki rejected `index_label` under `scope_attributes` (only valid under `resource_attributes`) | `loki /ready` health probe | smoke harness |
| Collector 0.150 dropped `service.telemetry.metrics.address`, replaced with `readers` block | collector startup | smoke harness |
| Prometheus v3 renamed `--enable-feature=otlp-write-receiver` to `--web.enable-otlp-receiver` | warn log on startup | smoke harness |
| **Production**: OTel Go SDK v1.43 default temporality is Delta, but Prometheus OTLP receiver only accepts Cumulative — counters silently rejected | OTLP-to-Prometheus translation error log | smoke harness |

All six were syntactically valid. All six would have shipped if the verification stopped at "go build clean and YAML parses."

## The contract

For every component of the AIM platform, there is a smoke test that:

1. **Boots the real artifact** (binary, container, stack — not a mock, not a unit test)
2. **Sends real traffic** through the path the PR touched
3. **Verifies the externally-observable signal** the PR was supposed to deliver
4. **Cleans up after itself** so it is idempotent (re-runnable without manual reset)

If a smoke test does not exist for the path you touched, **write one as part of the PR** and add it to the table below.

## Component smoke matrix

| Component | What ships | Smoke test | What it verifies | Run command |
|---|---|---|---|---|
| **OTel demo stack** | docker-compose stack with collector + Tempo + Prometheus + Loki + Grafana | `apps/backend/deployments/otel-demo/smoke-test.sh` | All 5 services boot healthy; a synthetic OTLP trace + counter + log lands in Tempo, Prometheus, and Loki respectively | `cd apps/backend/deployments/otel-demo && ./smoke-test.sh` |
| **AIM backend OTel wiring** | telemetry.Init, FGA span wrap, drift gauge | (covered by OTel demo stack smoke) | Backend exports succeed against a real collector | (TODO: end-to-end test that boots backend AND demo stack, sends an HTTP request that triggers FGA, verifies trace/metric/log) |
| **AIM backend HTTP API** | Fiber routes, auth, FGA enforcement | `tests/integration/*.go` | Endpoints return correct status codes against a live backend | (currently broken: needs a running backend with admin login configured — pre-existing issue, not part of this contract yet) |
| **PQC migration** | ML-DSA / ML-KEM key generation | `internal/crypto/pqc/*_test.go` | Keys generate, sign, verify | `go test ./internal/crypto/pqc/` (passes today; not a real "boot" test but acceptable since the surface is pure crypto) |

(More rows to be added as components grow smoke tests.)

## Smoke test design rules

When writing a new smoke test for AIM:

### 1. Pre-flight, then cleanup, then port check

- Verify required tools are on PATH (`docker`, `curl`, `go`, etc.)
- Reap any orphan resources from prior failed runs (`docker rm -f <named-container>`, drop stale temp dirs) **before** the port-conflict check — otherwise your own prior run shows up as a conflict
- Then check ports/files/env you need

### 2. Boot the real thing, with a real readiness signal

Every service has a real readiness signal — find it.

| Service | Readiness probe |
|---|---|
| Tempo | `GET /ready` |
| Loki | `GET /ready` |
| Prometheus v3 | `GET /-/ready` |
| Grafana | `GET /api/health` |
| OTel collector | `GET /` on the `health_check` extension (default port 13133) |
| AIM backend | `GET /health` |
| Postgres | `pg_isready` |
| Redis | `redis-cli ping` |

Do not use "service self-metrics endpoint exists" as a readiness probe — it returns 200 before the service is actually serving its main port.

### 3. Send real traffic, not synthetic stubs

The synthetic OTLP signal generator (`gen-signal.sh`) sends an actual OTLP message via a throwaway Go program using the same SDK as the backend. This caught the Delta-vs-Cumulative bug because **the SDK's actual default behavior** is what hits Prometheus — not what the docs say it should be.

For HTTP smoke tests against the backend, use real `curl` against real endpoints. For database smoke tests, run real migrations against a real Postgres and `SELECT` the result.

### 4. Verify the externally-observable signal

If the PR adds a metric, query Prometheus for it (`/api/v1/query?query=<name>`). If it adds a span, query Tempo for the trace_id (`/api/traces/<id>`). If it adds a log line, query Loki (`/loki/api/v1/query_range?query=...`).

**Do not stop at "the request returned 200."** A 200 with empty `data.result.[]` in Prometheus means the metric did not land — that's exactly the failure mode that hid the temporality bug for hours.

### 5. Wait, but bound the wait

Async pipelines need flush time. Pattern: poll every 2s for up to 30s with a clear failure message that includes the last response body. Do not use a single `sleep 60` — when it fails you do not know whether you needed 5 more seconds or 5 minutes.

### 6. Make it port-overridable

Dev machines run other Docker stacks. The smoke test must work on a clean machine AND on a machine where the default ports are taken. Pattern: env vars with defaults (`OTEL_GRPC_PORT=${OTEL_GRPC_PORT:-4317}`), provide a `.env.example` operators can copy, document the conflict-check command in the comments at the top of compose.

### 7. Make it idempotent

A smoke test that fails halfway and leaves orphan containers/files/ports must be re-runnable without manual cleanup. Always do `docker rm -f <name>` for every named container in pre-flight, even if the previous run "succeeded" (in case it crashed between containers).

### 8. Exit codes are part of the contract

`0` = pass. Non-zero with a human-readable failure message and the relevant log snippet. Use distinct codes per failure class so CI can branch on them:

```
0 = all signals verified
1 = port conflict (no boot attempted)
2 = docker compose up failed
3 = a service failed health check
4 = a signal failed to land in its backend
```

## Running smoke tests

### Locally

Per-component, before pushing:

```bash
# OTel demo stack
cd apps/backend/deployments/otel-demo && ./smoke-test.sh
```

Expected runtime: 60-90 seconds on a warm machine, 3-5 minutes on first run (image pulls).

### In the pre-push gate

The `pre-push-review` skill should be extended (TODO) to run the smoke test for any component touched in the diff:

| Path touched | Smoke test to run |
|---|---|
| `apps/backend/deployments/otel-demo/**` | `apps/backend/deployments/otel-demo/smoke-test.sh` |
| `apps/backend/internal/telemetry/**` | `apps/backend/deployments/otel-demo/smoke-test.sh` (validates the SDK init that the backend uses) |
| `apps/backend/internal/application/fga_engine.go` | (TODO: backend HTTP smoke that triggers an FGA call) |

Until that wiring lands, the rule is: **before you push, manually run the smoke for any component touched in your diff**, and paste the `==> PASS` line into your PR description.

### In CI (TODO)

A GitHub Actions workflow per component that runs its smoke test on PRs touching its paths. Failure blocks merge.

## Anti-patterns to avoid

These were all observed during the OTel exporter PR review and represent the failure mode of "verification by syntax check":

- "`docker compose config -q` exits 0, so the stack is fine." It validates YAML schema, not whether image tags exist or services boot.
- "`go build` is clean, so the SDK call is correct." Build only validates types; it does not catch wrong default behaviors (Delta vs Cumulative, wrong package imports for option types).
- "`go test -race` passes, so the production behavior is correct." Tests use mocks; the production OTLP exporter that your code constructs at boot was never exercised.
- "The unit test calls `Authorize`, so the metrics emit." Authorize was wrapped in a `defer` that only `emitDecisionTelemetry` calls — no test ran against a real meter provider.
- "I read the docs, so I know the SDK default." OTel Go SDK v1.43 ships Delta temporality by default for synchronous counters, contradicting common assumption.

If you find yourself reasoning your way to "this should work," stop and run the smoke test. If the smoke test does not exist, write one — that **is** the work, not extra work on top of the work.

## Adding a new smoke test

Template:

```bash
#!/usr/bin/env bash
# Smoke test for <COMPONENT>.
# Verifies <WHAT EXTERNAL SIGNAL>.
# Exit codes: see docs/testing/release-smoke.md
set -euo pipefail

cd "$(dirname "$0")"

# 1. Pre-flight + cleanup
# 2. Boot real artifact
# 3. Wait for readiness (real probe per service)
# 4. Send real traffic
# 5. Verify externally-observable signal
# 6. Print PASS line with clickable URLs to the verified signal
```

Then add a row to the **Component smoke matrix** above.

## Cross-link

Global testing philosophy: `~/.claude/instructions/testing-philosophy.md`
Pre-push review skill: `~/workspace/claude-skills/skills/pre-push-review/SKILL.md`
This doc was written in response to bugs surfaced when the user asked "did you actually run these services?" after the original verification claimed PASS based on syntax checks alone.
