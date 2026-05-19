# Agent Identity Management (AIM)

> **[OpenA2A](https://github.com/opena2a-org/opena2a)**: [CLI](https://github.com/opena2a-org/opena2a) · [HackMyAgent](https://github.com/opena2a-org/hackmyagent) · [Secretless](https://github.com/opena2a-org/secretless-ai) · [AIM](https://github.com/opena2a-org/agent-identity-management) · [Browser Guard](https://github.com/opena2a-org/AI-BrowserGuard) · [DVAA](https://github.com/opena2a-org/damn-vulnerable-ai-agent)

Cryptographic identity, capability authorization, and audit trails for AI agents. Apache 2.0.

[![CI](https://github.com/opena2a-org/agent-identity-management/actions/workflows/ci.yml/badge.svg)](https://github.com/opena2a-org/agent-identity-management/actions/workflows/ci.yml)
[![Security](https://github.com/opena2a-org/agent-identity-management/actions/workflows/security.yml/badge.svg)](https://github.com/opena2a-org/agent-identity-management/actions/workflows/security.yml)
[![Docker](https://img.shields.io/docker/pulls/opena2a/aim-server?label=docker%20pulls)](https://hub.docker.com/r/opena2a/aim-server)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

[Website](https://opena2a.org) · [Demos](https://opena2a.org/demos) · [Discord](https://discord.gg/uRZa3KXgEn)

> **Pre-1.0.** AIM is suitable today for evaluation, development, and internal testing. Production deployment with untrusted tenants or sensitive data is not yet recommended. Active hardening work is tracked in [HARDENING.md](HARDENING.md).

## Quick start

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

The agent has an Ed25519 keypair, a local audit log at `~/.opena2a/aim-core/audit.jsonl`, and a trust score. No server required.

After an incident:

```bash
opena2a identity audit
```

Reads back credential injections, file accesses, config changes, and capability checks captured by the OpenA2A toolchain.

## Three deployment modes

| Mode | When | Includes |
|---|---|---|
| **Local-only** | Solo developer, single machine | Ed25519 keypair, `audit.jsonl`, YAML capability policies, 8-factor trust score, cross-tool event bridges |
| **Self-hosted** | Team or fleet | Above plus PostgreSQL audit, REST API, dashboard, OAuth, 5-step FGA, 9-factor real-time trust, MCP attestation, PAM, SIEM adapters |
| **AIM Cloud** | Same as self-hosted with no infrastructure to operate | Managed at [aim.opena2a.org/get-started](https://aim.opena2a.org/get-started) |

All three share the same audit-event schema. Local agents can push history to a server via `AIMCore.enableReporting()`.

## Dashboard

Available in Self-hosted and AIM Cloud modes.

![Fleet overview](docs/images/dashboard-security.png)
*Fleet overview: agents monitored, actions blocked, and risk by category.*

![Agent registry](docs/images/agents.png)
*Agent registry with trust scores and verification status per agent.*

![Per-agent trust score breakdown](docs/images/agent-trust-score.png)
*Per-agent 8-factor trust score breakdown with weighted signal contributions.*

![MCP supply chain](docs/images/supply-chain.png)
*MCP server dependencies with multi-agent attestation status.*

## Install

### npm

```bash
npm install -g opena2a-cli      # CLI
npm install @opena2a/aim-core   # TypeScript library
```

### Homebrew

```bash
brew install opena2a-org/tap/opena2a
```

### Docker

```bash
curl -sSLO https://raw.githubusercontent.com/opena2a-org/agent-identity-management/main/scripts/quickstart.sh
shasum -a 256 quickstart.sh     # verify against the SHA in the latest release notes
bash quickstart.sh
```

Brings up `aim-server`, `aim-dashboard`, PostgreSQL, and Redis. Dashboard at `localhost:3000`, API at `localhost:8080`. Login credentials print at the end of the run.

Production deployment (Azure, GCP, AWS): [infrastructure/DEPLOYMENT.md](infrastructure/DEPLOYMENT.md).

### From source

Prerequisites: Docker, Go 1.22+, Node 20+, Python 3.11+.

```bash
git clone https://github.com/opena2a-org/agent-identity-management.git
cd agent-identity-management

# Minimal dev stack
docker compose up -d aim-postgres aim-redis aim-backend aim-frontend

# Health check
curl -fsS localhost:8080/healthz

# Python SDK editable for examples
pip install -e sdk/python

# Try the flight-search-agent demo
cd examples/flight-search-agent && python3 flight_agent.py
```

The full `docker-compose.yml` also brings up Elasticsearch, MinIO, NATS, Prometheus, Grafana, and Loki. Skip those services with the minimal command above.

### Verifying what was installed

Every release publishes via npm Trusted Publishing with SLSA v1 provenance. No long-lived `NPM_TOKEN`. GitHub Actions exchanges its OIDC token with npm at publish time.

```bash
npm view @opena2a/aim-core dist.attestations --json
# Expects non-empty result with predicateType "https://slsa.dev/provenance/v1"
```

Identity files (`~/.opena2a/aim-core/identity.json`) are written `mode 0600`. OAuth tokens live in the OS keychain by default. `~/.opena2a/auth.json` stores metadata only.

## How the local audit log fills up

`opena2a identity attach --all` installs cross-tool bridges. Each OpenA2A tool already writes its own event log. The bridges read those logs and re-emit each event through `aim.logEvent()` into one unified JSONL.

```text
Secretless events    ──┐
HMA scan findings    ──┤
HMA ARP runtime      ──┼─→ ~/.opena2a/aim-core/audit.jsonl
ConfigGuard events   ──┤
Shield events        ──┘
```

No decorator. No library import in agent code. Run `attach --all` once, work normally, and after an incident the audit log holds a deduplicated, timestamp-ordered trail of credential injections, file accesses, network calls, config tampering, and scan findings.

Capability authorization (deny-before-execute, FGA, intent classification) requires the server. See [Server features](#server-features).

## SDKs

| SDK | Install | Talks to |
|---|---|---|
| TypeScript | `npm install @opena2a/aim-core` | Local files or server |
| Python | `pip install -e sdk/python`, or download from [dashboard](https://aim.opena2a.org) Settings → SDK Downloads | Server |
| Java | `org.opena2a:aim-sdk:1.0.0`, or `cd sdk/java && mvn package` | Server |

TypeScript is the only SDK that runs without a server. It backs local mode. Python and Java SDKs talk to the AIM backend over HTTP. Working examples for all three live in [`examples/`](examples/).

### Python

Register an agent in one line. Add `@agent.perform_action` to each function that performs a capability.

```python
from aim_sdk import secure

agent = secure("my-first-agent")

@agent.perform_action(capability="weather:fetch")
def fetch_weather(city):
    return f"Weather in {city}: Sunny"
```

`secure()` generates an Ed25519 keypair, registers the agent with the AIM backend, stores credentials, and auto-detects the framework from imports (`langchain`, `crewai`, `llama_index`, `anthropic`, `openai`). When both a framework and an LLM provider are present, the framework wins.

`@agent.perform_action` signs each invocation, runs it through 5-step FGA on the server, and records the outcome in the audit log. Risk level auto-detects from the capability string using two lookup tables in [`sdk/python/aim_sdk/risk_detector.py`](sdk/python/aim_sdk/risk_detector.py):

- **Namespace prefix.** `payment:`, `admin:`, `system:`, `billing:`, `finance:` map to critical. `email:`, `notification:`, `sms:`, `user:`, `auth:`, `secret:`, `credential:` map to high. `db:`, `database:`, `file:`, `storage:`, `cache:` map to medium. `api:`, `weather:`, `search:`, `geocode:`, `translate:`, `time:`, `math:`, `util:` map to low.
- **Action suffix.** `:read`, `:fetch`, `:get`, `:list`, `:query`, `:view`, `:check`, `:validate` map to low. `:write`, `:update`, `:create`, `:modify`, `:save`, `:upload` map to medium. `:delete`, `:send`, `:execute`, `:run`, `:invoke`, `:export`, `:transfer` map to high. `:process`, `:refund`, `:charge`, `:approve`, `:drop`, `:truncate`, `:wipe`, `:terminate` map to critical.

When namespace and action disagree the higher risk wins. A `SPECIFIC_CAPABILITY_MAP` in the same file overrides both for known patterns (for example `user:delete` escalates to critical). Pass `risk_level="critical"` to override, and `jit_access=True` to pause execution until a human approves in the dashboard.

Full example: [`examples/flight-search-agent/flight_agent.py`](examples/flight-search-agent/flight_agent.py).

## Server features

### 5-step Fine-Grained Authorization

Every privileged action runs through five checks before execution.

| Step | Check | Latency budget |
|---|---|---|
| 1 | Capability | <10ms |
| 2 | Attribute | <10ms |
| 3 | Context | <10ms |
| 4 | Chain | <10ms |
| 5 | Intent (NanoMind) | up to 800ms on HIGH-risk operations |

Step 5 uses the [NanoMind security classifier](https://huggingface.co/opena2a/nanomind-security-classifier), a 3M-parameter local Mamba model. No external calls.

### Trust-gated capabilities

9-factor real-time trust scoring runs on every action. Per-capability thresholds gate access.

### MCP attestation

Multi-agent consensus. 3+ attesters across 2+ owners equals verified.

### Privileged Access Management

Three tiers: STANDARD, PRIVILEGED, SUPER_PRIVILEGED. Human approval gates, break-glass override, and certification campaigns.

### CyberArk integration

CCP for vaulted credential retrieval. PSM for privileged session recording.

### SIEM adapters

Splunk HEC and Microsoft Sentinel Data Collector. Buffered batch delivery, retry, severity filtering.

### Web dashboard

Agent registry, trust score breakdowns, MCP network graph, audit timeline, capability requests, policy editor.

## Trust scoring

The local and server trust scores measure different things.

**Local (8 factors)** answers "is the agent's security posture set up correctly?" Computed from local files. Source: [`packages/aim-core/src/trust.ts`](https://github.com/opena2a-org/opena2a/blob/main/packages/aim-core/src/trust.ts).

| Factor | Weight | Signal |
|---|---|---|
| Identity | 20% | `identity.json` exists |
| Capabilities | 15% | `policy.yaml` exists |
| Audit log | 10% | `audit.jsonl` exists |
| Secrets managed | 15% | Secretless integration active |
| Config signed | 10% | ConfigGuard signatures present |
| Skills verified | 10% | HMA verification on skills |
| Network controlled | 10% | Egress policy detected |
| Heartbeat monitored | 10% | ARP runtime heartbeat present |

**Server (9 factors)** answers "is the agent behaving in a way that should still be trusted right now?" Updates on every action. Source: [`apps/backend/internal/domain/trust_score.go`](apps/backend/internal/domain/trust_score.go).

| Factor | Weight | Source |
|---|---|---|
| Verification status | 25% | Externally attested |
| Uptime | 15% | Observed |
| Action success rate | 15% | Observed |
| Security alerts | 15% | NanoMind-modulated |
| Compliance | 10% | Externally attested |
| Execution isolation | 10% | Externally attested |
| Agent age | 5% | Server-recorded |
| Drift detection | 3% | Observed |
| User feedback | 2% | Human input |

Both can run for the same agent when local-to-server reporting is enabled.

## Observability

The backend emits OpenTelemetry traces, metrics, and logs. A hermetic demo stack lives at [`apps/backend/deployments/otel-demo/`](apps/backend/deployments/otel-demo/): OpenTelemetry Collector, Tempo, Prometheus, Loki, Grafana.

```bash
cd apps/backend/deployments/otel-demo
docker compose up -d
./smoke-backend.sh
```

Every `fga.authorize` decision lands as a parent span with 5 child spans (one per FGA step). 9 SemConv attributes ride the parent: `agent.id`, `agent.public_key.algorithm`, `agent.trust_score`, `agent.drift_score`, `agent.scan_verdict`, `agent.capability`, `fga.step`, `fga.outcome`, `fga.denied_by`. The attribute set is proposed to the OpenTelemetry Semantic Conventions WG. Full design notes: [`apps/backend/docs/OBSERVABILITY.md`](apps/backend/docs/OBSERVABILITY.md).

## Demos

Four runnable demos in [`examples/`](examples/):

| Demo | Shows | Stack |
|---|---|---|
| [`flight-search-agent`](examples/flight-search-agent/) | Python SDK with three deterministic prompt-injection scenarios (`inject data-exfil`, `inject priv-esc`, `inject sandbox-escape`) | Python + AIM server |
| [`langchain-crud-agent`](examples/langchain-crud-agent/) | LangChain agent secured by `@perform_action` | Python + LangChain + AIM server |
| [`mcp-server-demo`](examples/mcp-server-demo/) | MCP server with Ed25519 signing | Python + Flask + PyNaCl |
| [`a2a-multi-agent-demo`](examples/a2a-multi-agent-demo/) | A2A collaboration: discovery, GDPR consent, request signing, skill attestation | Python + Java + AIM server |

## Use cases

| Guide | Time |
|---|---|
| [Register an agent](docs/use-cases/register-my-agent.md) | 2 min |
| [Audit agent actions](docs/use-cases/audit-agent-actions.md) | 5 min |
| [Enforce capabilities](docs/use-cases/enforce-capabilities.md) | 5 min |
| [Embed in an app](docs/use-cases/embed-in-my-app.md) | 10 min |
| [Fleet governance](docs/use-cases/fleet-governance.md) | 30 min |
| [SIEM forwarding](docs/use-cases/siem-forwarding.md) | 15 min |
| [CyberArk vault integration](docs/use-cases/cyberark-integration.md) | 20 min |

Full index: [docs/USE-CASES.md](docs/USE-CASES.md).

## Contributing

Apache 2.0. PRs from outside the org welcome. [CONTRIBUTING.md](CONTRIBUTING.md) has the dev loop, test conventions, and pre-push review gates.

Security issues: `security@opena2a.org` (coordinated disclosure, response within 24 hours).

## Links

- [Documentation](https://opena2a.org/docs)
- [SDK Quickstart](https://opena2a.org/docs/tutorials/sdk-quickstart)
- [MCP Registration](https://opena2a.org/docs/tutorials/mcp-registration)
- [Deployment Guide](infrastructure/DEPLOYMENT.md)
- [Research](https://research.opena2a.org)

Part of the [OpenA2A](https://opena2a.org) security platform.

## License

Apache-2.0. See [LICENSE](LICENSE).
