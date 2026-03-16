> **[OpenA2A](https://github.com/opena2a-org/opena2a)**: [CLI](https://github.com/opena2a-org/opena2a) · [HackMyAgent](https://github.com/opena2a-org/hackmyagent) · [Secretless](https://github.com/opena2a-org/secretless-ai) · [AIM](https://github.com/opena2a-org/agent-identity-management) · [Browser Guard](https://github.com/opena2a-org/AI-BrowserGuard) · [DVAA](https://github.com/opena2a-org/damn-vulnerable-ai-agent) · Registry (April 2026)

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
opena2a identity create --name my-agent    # Create identity
opena2a identity trust                     # Calculate trust score
opena2a identity sign --data "hello"       # Sign data
opena2a identity audit                     # View audit log
opena2a identity attach --all              # Connect to all detected tools
```

For a full security dashboard across all your agents:

```bash
npx opena2a-cli review
```

```
Security Review: ~/my-project
  Identity:     aim_7f3a9c2e (my-agent)
  Trust Score:  85/100 (B)
  Capabilities: 3 allowed, 1 denied
  Audit Events: 47 (last 24h)
  MCP Servers:  2 verified, 0 drifted
```

## What AIM Provides

**Cryptographic identity** -- Ed25519 keypairs and OAuth 2.0 token endpoint for machine-to-machine auth. Every agent gets a verifiable identity on creation.

**Capability enforcement** -- Declare what each agent can do; block everything else at runtime. Policies defined in YAML (local) or via REST API (server).

**Audit trail** -- Append-only, tamper-evident log of every action. JSON-lines locally, PostgreSQL with full query API on the server.

**Trust scoring** -- 8-factor algorithm evaluating agent trustworthiness: identity strength, capability compliance, audit completeness, MCP attestation, policy adherence, lifecycle status, ownership verification, and behavioral analysis.

## SDKs

| SDK | Install | Status |
|-----|---------|--------|
| Python | `pip install aim-sdk` | Stable |
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
const identity = aim.getOrCreateIdentity();
console.log('Agent ID:', identity.agentId);

// Capability enforcement -- define what the agent can do
aim.loadPolicy({ allow: ['db:read', 'api:call'], deny: ['db:write'] });
aim.checkCapability('db:read');   // passes
// aim.checkCapability('db:write'); // throws CapabilityDenied

// Audit log -- append-only, tamper-evident
aim.logEvent({ action: 'db:read', target: 'customers', outcome: 'allowed' });

// Trust scoring -- 8-factor calculation
const score = aim.calculateTrust();
console.log('Trust:', score.score, score.grade); // e.g. 85, "B"
```

| Feature | aim-core (local) | Full AIM (server) |
|---------|-----------------|-------------------|
| Ed25519 identity | Local keypair | Server-issued + OIDC |
| Audit log | JSON-lines file | PostgreSQL + API |
| Capability policy | YAML file | REST API + dashboard |
| Trust scoring | 8-factor local | Real-time + history |
| Multi-agent | Per-machine | Cross-machine fleet |

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

## Links

- [Documentation](https://opena2a.org/docs) -- full guides, tutorials, API reference
- [SDK Quickstart](https://opena2a.org/docs/tutorials/sdk-quickstart) -- secure your first agent
- [MCP Registration](https://opena2a.org/docs/tutorials/mcp-registration) -- connect MCP servers
- [Contributing](CONTRIBUTING.md) -- how to contribute
- [Deployment Guide](infrastructure/DEPLOYMENT.md) -- production deployment

Part of the [OpenA2A](https://opena2a.org) security platform. See all tools at [opena2a.org](https://opena2a.org).

## License

Apache-2.0 -- See [LICENSE](LICENSE)
