> **[OpenA2A](https://github.com/opena2a-org/opena2a)**: [CLI](https://github.com/opena2a-org/opena2a) · [HackMyAgent](https://github.com/opena2a-org/hackmyagent) · [Secretless](https://github.com/opena2a-org/secretless-ai) · [AIM](https://github.com/opena2a-org/agent-identity-management) · [Browser Guard](https://github.com/opena2a-org/AI-BrowserGuard) · [DVAA](https://github.com/opena2a-org/damn-vulnerable-ai-agent)
# Agent Identity Management (AIM)

No way to audit what an agent did, control what it can do, or revoke access when something goes wrong. AIM fixes that -- open-source identity, governance, and access control for AI agents.

[![CI](https://github.com/opena2a-org/agent-identity-management/actions/workflows/ci.yml/badge.svg)](https://github.com/opena2a-org/agent-identity-management/actions/workflows/ci.yml)
[![Security](https://github.com/opena2a-org/agent-identity-management/actions/workflows/security.yml/badge.svg)](https://github.com/opena2a-org/agent-identity-management/actions/workflows/security.yml)
[![Docker](https://img.shields.io/docker/pulls/opena2a/aim-server?label=docker%20pulls)](https://hub.docker.com/r/opena2a/aim-server)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

[Website](https://opena2a.org) | [Demos](https://opena2a.org/demos) | [Discord](https://discord.gg/uRZa3KXgEn)

## Quick Start

```bash
npx opena2a-cli identity create --name my-agent
```

```
Agent created:
  ID:         aim_7f3a9c2e
  Name:       my-agent
  Public Key: ed25519:x8Kp...mQ4R
  Stored:     ~/.opena2a/aim-core/identities/my-agent.json
  Audit Log:  ~/.opena2a/aim-core/audit.jsonl
```

Your agent now has a cryptographic identity, an append-only audit log, and a trust score -- no server required.

## Two Ways to Start

**Solo developer, single agent, no infrastructure:**

```bash
npm install @opena2a/aim-core
```

Local Ed25519 keys, file-based audit log, YAML capability policies. Everything stays on your machine.

**Team, fleet of agents, full governance:**

```bash
docker pull opena2a/aim-server
docker pull opena2a/aim-dashboard
```

PostgreSQL-backed audit log, REST API, dashboard, OAuth 2.0 token endpoint, cross-machine fleet management.

Or use [AIM Cloud](https://aim.opena2a.org) -- no infrastructure required.

## CLI Commands

The CLI is the fastest path to managing agent identity. Install once:

```bash
npm install -g opena2a-cli
```

Then:

```bash
# Local identity
opena2a identity create --name my-agent    # Create Ed25519 identity
opena2a identity trust                     # Calculate trust score
opena2a identity sign --data "hello"       # Sign data
opena2a identity audit                     # View audit log
opena2a identity attach --all              # Connect to all detected tools

# Cloud (aim.opena2a.org)
opena2a login                              # Authenticate via browser
opena2a identity create --name my-agent --server cloud   # Register on server
opena2a identity list --server cloud       # List all agents
opena2a identity tag add production --server cloud       # Tag agents
opena2a identity mcp list --server cloud   # View MCP connections
opena2a identity activity --server cloud   # View activity log
opena2a whoami                             # Check auth status
```

For a full security dashboard across all your agents:

```bash
npx opena2a-cli review
```

```
Security Review: ~/my-project
  Identity:     aim_7f3a9c2e (my-agent)
  Trust Score:  0.85 (strong)
  Capabilities: 3 allowed, 1 denied
  Audit Events: 47 (last 24h)
  MCP Servers:  2 verified, 0 drifted
```

## What AIM Provides

Cryptographic identity, OAuth 2.0 auth, capability enforcement, audit trail, 8-factor trust scoring, MCP attestation, lifecycle management, policy engine, tag/MCP management, and a full web dashboard.

<details>
<summary>See all features</summary>

**Cryptographic identity** -- Ed25519 keypairs generated on agent creation. Every agent gets a verifiable identity with signing and verification capabilities. Post-quantum (ML-DSA-44/65/87) and hybrid Ed25519+ML-DSA modes available server-side.

**OAuth 2.0 and machine-to-machine auth** -- JWT-bearer grant for agent-to-server authentication. Device authorization flow (RFC 8628) for CLI login via browser. No API key management needed after `opena2a login`.

**Capability enforcement** -- Declare what each agent can do; block everything else at runtime. Per-capability trust thresholds (e.g., `system:admin` requires 70% trust, `file:read` requires 0%). Per-capability execution modes: `auto` (immediate), `notify` (execute + alert), `review` (queue for human approval). Policies defined in YAML (local) or via REST API (server).

**Audit trail** -- Append-only, tamper-evident log of every action. JSON-lines locally, PostgreSQL with full query API on the server. Audit events include action, target, result, timestamp, and tool attribution.

**Trust scoring** -- 8-factor weighted algorithm: verification status (25%), uptime (15%), action success rate (15%), security alerts (15%), compliance (10%), agent age (10%), drift detection (5%), and user feedback (5%). Historical trends and confidence levels tracked. Trust scores gate capability access via per-capability thresholds.

**Delegation chains** -- Cryptographically signed delegation with Ed25519. Scope narrowing enforced at each hop. Trust attenuation: each delegation hop reduces effective trust by a configurable factor (default 0.8x per hop, minimum floor of 0.3). Prevents deep delegation chains from bypassing trust requirements.

**MCP attestation** -- Agents attest to the quality and security of MCP servers they use. Multi-agent consensus protocol: 3+ unique attesters across 2+ owners = verified. Supply chain visualization on the dashboard.

**Lifecycle management** -- Full agent state machine: pending, verified, suspended, revoked. Suspend agents instantly when compromised, revoke permanently (30-day data retention). Status affects trust score automatically.

**Policy management** -- YAML-based local policies or server-managed via REST API. Default-deny or default-allow with granular capability rules. Plugin-scoped policies for per-tool control.

**Tag and MCP management** -- Organize agents with tags, attach/detach MCP server connections. Manageable via CLI (`identity tag add`, `identity mcp add`), REST API, or dashboard.

**Dashboard** -- Web UI for fleet management at [aim.opena2a.org](https://aim.opena2a.org). Agent overview, trust score breakdowns, MCP network graph, audit timeline, security violations, capability requests, and policy editor.

</details>

## SDKs

| SDK | Install | Status |
|-----|---------|--------|
| Python | `pip install -e sdk/python/` (local) or download from [AIM dashboard](https://aim.opena2a.org) | Stable |
| Java | `org.opena2a:aim-sdk:1.0.0` (Maven / Gradle) | Stable |
| TypeScript/Node.js | `npm install @opena2a/aim-core` | Stable |

## aim-core: For Library Developers

If you are building a tool or framework that needs to embed agent identity, `@opena2a/aim-core` provides programmatic access without requiring a running server.

```bash
npm install @opena2a/aim-core
```

```typescript
import { AIMCore } from '@opena2a/aim-core';

const aim = new AIMCore({ agentName: 'my-assistant' });

// Ed25519 identity -- created on first run, persisted to ~/.opena2a/aim-core/
const identity = aim.getIdentity();
console.log('Agent ID:', identity.agentId);

// Capability enforcement -- define what the agent can do
aim.loadPolicy({ allow: ['db:read', 'api:call'], deny: ['db:write'] });
aim.checkCapability('db:read');   // passes
// aim.checkCapability('db:write'); // throws CapabilityDenied

// Audit log -- append-only, tamper-evident
aim.logEvent({ action: 'db:read', target: 'customers', result: 'allowed', plugin: 'my-assistant' });

// Trust scoring -- 8-factor calculation
const score = aim.calculateTrust();
console.log('Trust:', score.overall); // e.g. 0.45
```

| Feature | aim-core (local) | Full AIM (server + dashboard) |
|---------|-----------------|-------------------------------|
| Ed25519 identity | Local keypair | Server-issued + PQC (ML-DSA) |
| OAuth 2.0 auth | N/A | JWT-bearer + device flow |
| Audit log | JSON-lines file | PostgreSQL + query API |
| Capability policy | YAML file | REST API + visual editor |
| Trust scoring | 8-factor local | Real-time + history + trends |
| MCP attestation | N/A | Multi-agent consensus |
| Lifecycle | N/A | Suspend, revoke, verify |
| Tags + MCPs | N/A | Organize + attach via CLI/API |
| Multi-agent | Per-machine | Cross-machine fleet |
| Dashboard | N/A | Full web UI |

This is the same library used by HackMyAgent's `--with-aim` flag to add agent identity during security remediation.

## Server Deployment

For team and fleet deployments, the AIM server provides a REST API, dashboard, and PostgreSQL-backed storage.

```bash
curl -sSL https://raw.githubusercontent.com/opena2a-org/agent-identity-management/main/scripts/quickstart.sh | bash
```

Opens dashboard at [localhost:3000](http://localhost:3000), API at [localhost:8080](http://localhost:8080). Secrets are auto-generated. Login credentials are printed at the end.

| Image | GHCR | Docker Hub |
|-------|------|------------|
| Backend API | `ghcr.io/opena2a-org/aim-server` | `opena2a/aim-server` |
| Dashboard | `ghcr.io/opena2a-org/aim-dashboard` | `opena2a/aim-dashboard` |

For production deployment (AWS, Azure, GCP, Kubernetes), image signing verification, and tag conventions, see [infrastructure/DEPLOYMENT.md](infrastructure/DEPLOYMENT.md).

## Using with HackMyAgent

[HackMyAgent](https://github.com/opena2a-org/hackmyagent) can add AIM agent identity as part of its security remediation flow:

```bash
hackmyagent fix-all --with-aim     # scan, fix, and add agent identity
hackmyagent fix-all --dry-run      # preview without modifying
```

## Use Cases

| Guide | Description | Time |
|-------|-------------|------|
| [Register my agent](docs/use-cases/register-my-agent.md) | Create an Ed25519 identity and attach tools | 2 min |
| [Audit agent actions](docs/use-cases/audit-agent-actions.md) | Track actions with a tamper-evident log | 5 min |
| [Enforce capabilities](docs/use-cases/enforce-capabilities.md) | Restrict what your agent can do with YAML policies | 5 min |
| [Embed in my app](docs/use-cases/embed-in-my-app.md) | Use aim-core SDK in your own framework | 10 min |
| [Fleet governance](docs/use-cases/fleet-governance.md) | Centralized management with AIM Server | 30 min |

See [docs/USE-CASES.md](docs/USE-CASES.md) for the full index.

## Links

- [Documentation](https://opena2a.org/docs) -- full guides, tutorials, API reference
- [SDK Quickstart](https://opena2a.org/docs/tutorials/sdk-quickstart) -- secure your first agent
- [MCP Registration](https://opena2a.org/docs/tutorials/mcp-registration) -- connect MCP servers
- [Contributing](CONTRIBUTING.md) -- how to contribute
- [Deployment Guide](infrastructure/DEPLOYMENT.md) -- production deployment

Part of the [OpenA2A](https://opena2a.org) security platform. See all tools at [opena2a.org](https://opena2a.org).

## License

Apache-2.0 -- See [LICENSE](LICENSE)
