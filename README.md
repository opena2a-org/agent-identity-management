# Agent Identity Management (AIM)

> **[OpenA2A](https://github.com/opena2a-org/opena2a)**: [CLI](https://github.com/opena2a-org/opena2a) · [HackMyAgent](https://github.com/opena2a-org/hackmyagent) · [Secretless](https://github.com/opena2a-org/secretless-ai) · [AIM](https://github.com/opena2a-org/agent-identity-management) · [Browser Guard](https://github.com/opena2a-org/AI-BrowserGuard) · [DVAA](https://github.com/opena2a-org/damn-vulnerable-ai-agent)

Your AI agents act on your behalf — calling APIs, writing to your filesystem, spending your budget, reading your private data. Today you have no way to prove which agent did what, restrict what they can do, or revoke access when something goes wrong. AIM fixes that.

Open-source. Apache 2.0. Local-first or fleet-scale. Same wire format, your call.

[![CI](https://github.com/opena2a-org/agent-identity-management/actions/workflows/ci.yml/badge.svg)](https://github.com/opena2a-org/agent-identity-management/actions/workflows/ci.yml)
[![Security](https://github.com/opena2a-org/agent-identity-management/actions/workflows/security.yml/badge.svg)](https://github.com/opena2a-org/agent-identity-management/actions/workflows/security.yml)
[![Docker](https://img.shields.io/docker/pulls/opena2a/aim-server?label=docker%20pulls)](https://hub.docker.com/r/opena2a/aim-server)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

[Website](https://opena2a.org) · [Demos](https://opena2a.org/demos) · [Discord](https://discord.gg/uRZa3KXgEn)

---

## 30 seconds in

```bash
npx opena2a-cli identity create --name my-agent
```

```
Identity created
  Agent ID:    aim_xxxxxxxx
  Name:        my-agent
  Public Key:  <base64-Ed25519-public-key>
  Stored in:   ~/.opena2a/aim-core/
```

Your agent now has an Ed25519 identity, a local audit log, and a trust score. No server. No account. The files live under `~/.opena2a/aim-core/` and stay on your machine. When an incident happens, run:

```bash
opena2a identity audit
```

…and read back every credential injection, file access, config change, and capability check the OpenA2A toolchain captured for that agent. That's the local-first promise.

---

## Three ways to run AIM

| Mode | When to use it | What you get |
|---|---|---|
| **Local-only** (`@opena2a/aim-core` + CLI) | Solo developer, single machine. Want incident review without standing up infrastructure. | Ed25519 keypair, append-only `audit.jsonl`, YAML capability policies, 8-factor trust score. Events from Secretless, HMA, ARP, ConfigGuard, Shield flow into the audit log automatically via [identity bridges](#how-the-local-audit-log-fills-up). |
| **Self-hosted server** | Team or fleet. Need a dashboard, OAuth login, multi-machine state, 9-factor real-time trust scoring with NanoMind. | Everything above, plus PostgreSQL audit, REST API, web dashboard, 5-step FGA, MCP attestation, PAM, SIEM adapters. |
| **AIM Cloud** | Same as self-hosted, but you don't want to run infrastructure. | Managed deployment at [aim.opena2a.org](https://aim.opena2a.org). Same SDKs, same dashboard, no Postgres to babysit. |

All three modes share the same audit-event schema. A local-only agent can later enable `AIMCore.enableReporting()` and push its history to a server — no rewrite, no migration script.

---

## Install

### 1. npm — for users and contributors writing agent code

The library + CLI ship as two packages. Install one or both.

```bash
# CLI only — `opena2a identity ...` and the rest of the OpenA2A toolchain
npm install -g opena2a-cli

# Library only — embed AIM in your own TypeScript / Node.js agent
npm install @opena2a/aim-core

# Both, in a brand-new project
mkdir my-agent && cd my-agent
npm init -y
npm install @opena2a/aim-core
npm install -g opena2a-cli
opena2a identity create --name my-agent
```

Python and Java SDKs ship server-side — see [SDKs](#sdks).

### 2. Docker — for fleet operators

```bash
# Quickstart: dashboard at localhost:3000, API at localhost:8080
curl -sSLO https://raw.githubusercontent.com/opena2a-org/agent-identity-management/main/scripts/quickstart.sh
shasum -a 256 quickstart.sh   # verify against the SHA in the latest release notes
bash quickstart.sh
```

Pulls `opena2a/aim-server` and `opena2a/aim-dashboard` from Docker Hub. PostgreSQL, Redis, and the backend come up in one stack. Secrets auto-generated. Login credentials printed at the end of the run.

Production deployment (Azure / GCP / AWS): [infrastructure/DEPLOYMENT.md](infrastructure/DEPLOYMENT.md).

### 3. From source — for contributors, security teams, or self-hosters who want to inspect every line

Clone the repo and run the full stack locally. This is what we use for development and what HackMyAgent's `--with-aim` flag bundles when shipping AIM as part of a remediation.

**Prerequisites:** Docker Desktop (or Docker Engine + Compose v2), Go 1.22+, Node 20+, Python 3.11+ (for SDK + examples).

```bash
git clone https://github.com/opena2a-org/agent-identity-management.git
cd agent-identity-management

# Option A: start everything via docker-compose (recommended — matches CI)
docker compose up -d aim-postgres aim-redis aim-backend aim-frontend
# → backend on :8080, frontend on :3000, Postgres on :5432

# Option B: backend from source, dependencies in docker
docker compose up -d aim-postgres aim-redis
cd apps/backend
go build ./...
./aim-server   # picks up DATABASE_URL from .env, or set it explicitly

# Frontend from source
cd apps/web
npm install
npm run dev   # → localhost:3000 with hot reload

# Install the Python SDK editable for the examples
pip install -e sdk/python

# Run the flight-search-agent demo against your local stack
cd examples/flight-search-agent
python3 flight_agent.py
```

Health check before you do anything else:

```bash
curl -fsS localhost:8080/healthz   # → {"service":"agent-identity-management","status":"healthy",...}
```

If the backend returns healthy, you can hit the SDK API at `/api/v1/sdk-api/verifications`. The dashboard's first-run flow walks you through creating an admin account (default password printed to stdout — change it before exposing the port).

The `docker-compose.yml` at the repo root also brings up Elasticsearch, MinIO, NATS, Prometheus, Grafana, and Loki for observability. Skip those services for a minimal dev loop:

```bash
docker compose up -d aim-postgres aim-redis aim-backend aim-frontend
# leaves the heavy observability stack idle until you need it
```

A separate, hermetic OTel demo lives at `apps/backend/deployments/otel-demo/` — that one is for verifying the trace/metric/log exporters end-to-end (see [Observability](#observability)).

---

## How the local audit log fills up

This is the value prop you don't get from the marketing snippet.

`opena2a identity attach --all` installs **cross-tool bridges**. Each tool in the OpenA2A toolchain already writes its own local event log. The bridges read those logs and re-emit each event through `aim.logEvent()` into the unified AIM audit JSONL.

```text
Secretless events    ──┐
HMA scan findings    ──┤
HMA ARP runtime      ──┤   bridges.importAllToolEvents()
ConfigGuard events   ──┼─→ AIMCore.logEvent() ─→ ~/.opena2a/aim-core/audit.jsonl
Shield events        ──┘
                            ↑ "what happened around the agent, on this machine"
```

You don't add a decorator. You don't import a library into your agent. You run `opena2a identity attach --all` once, work normally, and after an incident you have a deduplicated, timestamp-ordered, signed audit trail of everything the toolchain saw the agent do — credential injections, file accesses, network calls, config tampering, scan findings.

**This works without a running AIM server.** Local-first means local-first.

If you ALSO want capability authorization (deny-before-execute, FGA, intent classification) — that requires the server-backed AIM. See [What the server adds](#what-the-server-adds).

---

## SDKs

| SDK | Install | Stable | Talks to |
|---|---|---|---|
| TypeScript / Node.js | `npm install @opena2a/aim-core` | Yes | Local files OR server (via `enableReporting`) |
| Python | `pip install -e sdk/python` (clone) · or download from the [dashboard](https://aim.opena2a.org) → Settings → SDK Downloads | Yes | Server (Go backend) |
| Java | Maven: `org.opena2a:aim-sdk:1.0.0` · or `cd sdk/java && mvn package` from source | Yes | Server (Go backend) |

The TypeScript SDK is the only one that runs without a server (it's `@opena2a/aim-core`, the library backing local mode). Python and Java SDKs always talk to the AIM backend over HTTP — they're meant for production agents that need real-time capability authorization.

```typescript
// TypeScript: local-first, file-based
import { AIMCore } from '@opena2a/aim-core';

const aim = new AIMCore({ agentName: 'flight-search' });
aim.loadPolicy({ allow: ['flights:search'], deny: ['email:send', 'os:exec'] });

if (aim.checkCapability('flights:search')) {
  // execute the action
  aim.logEvent({ plugin: 'flight-search', action: 'flights:search', target: 'NYC', result: 'allowed' });
}
```

```python
# Python: server-backed, every decorated method auto-logs
from aim_sdk import secure

agent = secure("flight-search")

@agent.perform_action("flights:search")
def search_flights(destination):
    return MOCK_FLIGHTS[destination]
# Every call routes through AIM's /api/v1/sdk-api/verifications endpoint,
# runs the 5-step FGA, and lands in the dashboard audit timeline.
```

```java
// Java: same shape, server-backed, manual call
AIMClient client = AIMClient.fromCredentials("flight-search");
VerificationResult result = client.verifyCapability("flights:search", "NYC", Map.of("riskLevel", "low"));
if (result.isApproved()) { /* execute */ }
```

Working examples for all three SDKs live in [`examples/`](examples/). The flight-search-agent demo is the same one we run at Linux Foundation Open Source Summit (May 2026) — including three deterministic prompt-injection scenarios via `inject data-exfil`, `inject priv-esc`, and `inject sandbox-escape`.

---

## What the local CLI gets you (without the server)

```bash
# Identity lifecycle
opena2a identity create --name my-agent       # generate Ed25519 keypair
opena2a identity sign --data "hello"          # sign arbitrary bytes
opena2a identity trust                        # local 8-factor trust score
opena2a identity audit [--limit 50]           # read the audit JSONL
opena2a identity attach --all                 # turn on the cross-tool bridges
opena2a identity policy load policy.yaml      # load a capability policy

# Other OpenA2A surfaces, same CLI
opena2a review                                # full security dashboard for this project
opena2a scan                                  # HMA security checks
opena2a protect                               # auto-fix common misconfigurations
opena2a runtime tail [-c 50]                  # tail HMA's ARP runtime events
opena2a secrets add OPENAI_API_KEY            # Secretless credential management
```

All of the above work offline. The CLI binary IS the front door — no server connection required.

---

## What the server adds

If you run the AIM server (Docker or self-hosted from source), you get:

**5-step Fine-Grained Authorization (FGA)** — Every privileged action runs through `capability → attribute → context → chain → intent` before execution. The intent check is powered by [NanoMind](https://huggingface.co/opena2a/nanomind-security-analyst), a 3M-parameter local language model. Sub-10ms for the cheap steps, up to 800ms for the intent check on HIGH-risk operations.

**9-factor real-time trust scoring** — Server-side trust includes verification status, uptime, action success rate, security alerts (NanoMind-modulated), compliance, agent age, drift detection, user feedback, and execution isolation. Updates on every action. The trust score gates capability access via per-capability thresholds (`system:admin` requires ≥0.70, `file:read` no minimum, etc.).

**MCP server attestation** — Multi-agent consensus protocol. 3+ unique attesters across 2+ owners = verified. Supply-chain visualization on the dashboard.

**Privileged Access Management (PAM)** — Three tiers (STANDARD / PRIVILEGED / SUPER_PRIVILEGED). Human approval gates on SUPER_PRIVILEGED. Break-glass override with tamper-evident logging. Certification campaigns for periodic privilege review.

**CyberArk integration** — CCP for vaulted credential retrieval, PSM for privileged session recording.

**SIEM adapters** — Splunk HEC and Microsoft Sentinel Data Collector. Buffered batch delivery, retry with backoff, severity filtering.

**Web dashboard** — Agent registry, trust score breakdowns, MCP network graph, audit timeline, security violations, capability requests, policy editor.

```yaml
# aim-config.yaml — SIEM forwarding
siem:
  adapter: splunk
  hecUrl: https://splunk.internal:8088
  token: ${SPLUNK_HEC_TOKEN}
  index: aim_agent_audit
  sourcetype: aim:audit
  minSeverity: warning
```

---

## Trust scoring — local vs server

The two trust scores measure different things on purpose.

### Local: `@opena2a/aim-core` (8 factors)

The local trust score answers: **"how well is this agent's security posture set up?"** It's a one-shot computation from local files — does the agent have a keypair? Is there a policy? An audit log? Are secrets managed via Secretless? Are configs signed by ConfigGuard?

| Factor | Weight | What signals it |
|---|---|---|
| Identity | 20% | `identity.json` exists |
| Capabilities | 15% | `policy.yaml` exists |
| Audit log | 10% | `audit.jsonl` exists |
| Secrets managed | 15% | Secretless integration active |
| Config signed | 10% | ConfigGuard signatures present |
| Skills verified | 10% | HMA verification on skills |
| Network controlled | 10% | Egress policy detected |
| Heartbeat monitored | 10% | ARP runtime heartbeat present |

Plus up to 0.05 extended bonus for comprehensive posture. Score range 0.0–1.0. Source: [`packages/aim-core/src/trust.ts`](https://github.com/opena2a-org/opena2a/blob/main/packages/aim-core/src/trust.ts) in the `opena2a` repo.

### Server: `aim-server` (9 factors)

The server trust score answers: **"is this agent behaving in a way I should still trust at this moment?"** It updates on every action and includes behavioral signals the local mode can't observe.

| Factor | Weight | Source |
|---|---|---|
| Verification status | 25% | Externally attested (signed) |
| Uptime | 15% | Observed behavior |
| Action success rate | 15% | Observed behavior |
| Security alerts | 15% | NanoMind-modulated |
| Compliance | 10% | Externally attested |
| Execution isolation | 10% | Externally attested (added in PR #95, 2026-04-15) |
| Agent age | 5% | Server-recorded |
| Drift detection | 3% | Observed behavior |
| User feedback | 2% | Human input |

Source: [`apps/backend/internal/domain/trust_score.go`](apps/backend/internal/domain/trust_score.go) in this repo.

The two scores are complementary — local says "you set the agent up right", server says "the agent is still acting right." Both can be active for the same agent if you've enabled local-to-server reporting via `AIMCore.enableReporting()`.

---

## Observability

AIM emits OpenTelemetry traces, metrics, and logs from the Go backend. The hermetic demo stack lives at [`apps/backend/deployments/otel-demo/`](apps/backend/deployments/otel-demo/) — a one-command boot of OpenTelemetry Collector + Tempo + Prometheus + Loki + Grafana, wired to the backend.

```bash
cd apps/backend/deployments/otel-demo
docker compose up -d                # → Grafana on :3001, Tempo on :3200
./smoke-backend.sh                  # end-to-end verification
```

Every `fga.authorize` decision lands as a parent span with 5 children (one per FGA step), all 9 Slide 14 SemConv attributes (`agent.id`, `agent.public_key.algorithm`, `agent.trust_score`, `agent.drift_score`, `agent.scan_verdict`, `agent.capability`, `fga.step`, `fga.outcome`, `fga.denied_by`), plus a structured log line in Loki and a real-time gauge in Prometheus. See [Observing the Observers](https://opena2a.org/talks/observability-summit-2026) for the talk-length version.

We're proposing these attributes for the OpenTelemetry Semantic Conventions WG — there are currently zero standard attributes for AI agents, and every vendor that ships agent telemetry ships incompatible attributes. The proposal is in `briefs/otel-exporters-aim-backend.md`.

---

## Demos

Four runnable demos in [`examples/`](examples/):

| Demo | Shows | Stack |
|---|---|---|
| [`flight-search-agent`](examples/flight-search-agent/) | AIM Python SDK end-to-end with three deterministic prompt-injection scenarios | Python + AIM server |
| [`langchain-crud-agent`](examples/langchain-crud-agent/) | LangChain agent secured by `@perform_action` decorator | Python + LangChain + AIM server |
| [`mcp-server-demo`](examples/mcp-server-demo/) | MCP server with Ed25519 signing + capabilities endpoint | Python + Flask + PyNaCl |
| [`a2a-multi-agent-demo`](examples/a2a-multi-agent-demo/) | Agent-to-Agent collaboration: discovery, GDPR consent, request signing, skill attestation | Python + Java + AIM server |

`flight-search-agent` is the live demo from the May 2026 Linux Foundation Open Source Summit and Observability Summit talks. The eight-beat demo script lives in `opena2a-org/presentations/linux-foundation/demo-script.md` if you want to reproduce the stage flow.

---

## Use cases

| Guide | Description | Time |
|---|---|---|
| [Register my agent](docs/use-cases/register-my-agent.md) | Create an Ed25519 identity and attach tools | 2 min |
| [Audit agent actions](docs/use-cases/audit-agent-actions.md) | Track actions with a tamper-evident log | 5 min |
| [Enforce capabilities](docs/use-cases/enforce-capabilities.md) | Restrict what your agent can do with YAML policies | 5 min |
| [Embed in my app](docs/use-cases/embed-in-my-app.md) | Use `aim-core` SDK in your own framework | 10 min |
| [Fleet governance](docs/use-cases/fleet-governance.md) | Centralized management with AIM Server | 30 min |
| [SIEM forwarding](docs/use-cases/siem-forwarding.md) | Stream audit events to Splunk / Sentinel | 15 min |
| [CyberArk vault integration](docs/use-cases/cyberark-integration.md) | Retrieve secrets at runtime without exposing values | 20 min |

See [docs/USE-CASES.md](docs/USE-CASES.md) for the full index.

---

## Using with HackMyAgent

[HackMyAgent](https://github.com/opena2a-org/hackmyagent) can add AIM agent identity as part of its security remediation flow:

```bash
hackmyagent fix-all --with-aim     # scan, fix, and add agent identity
hackmyagent fix-all --dry-run      # preview without modifying
```

ARP runtime events flow into AIM's audit log via the identity bridges, so the local-only "what happened during this scan + fix" story is unified across both tools.

---

## Contributing

We ship Apache 2.0, develop in the open, and merge PRs from outside the org regularly. [CONTRIBUTING.md](CONTRIBUTING.md) covers the dev loop, test conventions, and pre-push review gates.

If you found a security issue, please email `security@opena2a.org` — coordinated disclosure preferred. We respond within 24 hours.

---

## Links

- [Documentation](https://opena2a.org/docs) — full guides, tutorials, API reference
- [SDK Quickstart](https://opena2a.org/docs/tutorials/sdk-quickstart) — secure your first agent
- [MCP Registration](https://opena2a.org/docs/tutorials/mcp-registration) — connect MCP servers
- [Deployment Guide](infrastructure/DEPLOYMENT.md) — production deployment (Azure / GCP / AWS)
- [Contributing](CONTRIBUTING.md) — how to contribute
- [Research](https://research.opena2a.org) — published threat data, scanner benchmarks, exposure sweeps

Part of the [OpenA2A](https://opena2a.org) security platform.

---

## License

Apache-2.0 — see [LICENSE](LICENSE).
