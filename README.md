> **[OpenA2A](https://github.com/opena2a-org)**: [CLI](https://github.com/opena2a-org/opena2a) · [HackMyAgent](https://github.com/opena2a-org/hackmyagent) · [Secretless](https://github.com/opena2a-org/secretless-ai) · [Browser Guard](https://github.com/opena2a-org/AI-BrowserGuard)

<div align="center">

# Agent Identity Management (AIM)

**Open-source identity, governance, and access control for AI agents.**

AI agents are non-human identities operating with real permissions. Without identity management, there is no way to audit what they did, control what they can do, or revoke access when something goes wrong.

[![CI](https://github.com/opena2a-org/agent-identity-management/actions/workflows/ci.yml/badge.svg)](https://github.com/opena2a-org/agent-identity-management/actions/workflows/ci.yml)
[![Security](https://github.com/opena2a-org/agent-identity-management/actions/workflows/security.yml/badge.svg)](https://github.com/opena2a-org/agent-identity-management/actions/workflows/security.yml)
[![Docker](https://img.shields.io/docker/pulls/opena2a/aim-server?label=docker%20pulls)](https://hub.docker.com/r/opena2a/aim-server)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

</div>

## Quick Start

```bash
curl -sSL https://raw.githubusercontent.com/opena2a-org/agent-identity-management/main/scripts/quickstart.sh | bash
```

Opens dashboard at [localhost:3000](http://localhost:3000), API at [localhost:8080](http://localhost:8080). Secrets are auto-generated. Login credentials are printed at the end.

Or pull directly:

```bash
docker pull opena2a/aim-server
docker pull opena2a/aim-dashboard
```

**AIM Cloud:** [aim.opena2a.org](https://aim.opena2a.org) -- no infrastructure required.

## Secure Your Agent

```python
from aim_sdk import secure

agent = secure("my-assistant", capabilities=["db:read", "api:call"])

@agent.perform_action(capability="db:read")
def get_customer(id):
    return db.query(id)
```

Your agent gets cryptographic identity (Ed25519), capability enforcement, and a full audit trail.

## What AIM Does

- **Cryptographic identity** -- Ed25519 keypairs and OAuth 2.0 token endpoint for machine-to-machine auth
- **Capability enforcement** -- declare what each agent can do; block everything else at runtime
- **MCP attestation** -- verify Model Context Protocol servers, detect tool drift
- **NHI governance** -- ownership tracking, lifecycle management, shadow agent discovery, ABOM export
- **Trust scoring** -- 8-factor algorithm evaluating agent trustworthiness in real time
- **Security policies** -- monitoring or strict mode, data exfiltration detection, just-in-time access

## Docker Images

| Image | GHCR | Docker Hub |
|-------|------|------------|
| Backend API | `ghcr.io/opena2a-org/aim-server` | `opena2a/aim-server` |
| Dashboard | `ghcr.io/opena2a-org/aim-dashboard` | `opena2a/aim-dashboard` |

| Tag | Description |
|-----|-------------|
| `latest` | Latest stable release |
| `edge` | Built from `main` on every push |
| `0.5.2` | Specific release version |
| `0.5` | Latest patch in the 0.5 series |
| `0` | Latest minor in the 0.x series |

All images are signed with [cosign](https://github.com/sigstore/cosign) (keyless, OIDC-based). See [infrastructure/DEPLOYMENT.md](infrastructure/DEPLOYMENT.md) for verification and production deployment.

## SDKs

| SDK | Install | Status |
|-----|---------|--------|
| Python | `pip install aim-sdk` | Stable |
| Java | Maven / Gradle | Stable |
| TypeScript/Node.js | `npm install @opena2a/aim-core` | Stable |

The `@opena2a/aim-core` package provides programmatic access to AIM from Node.js projects. It is the same integration used by HackMyAgent's `--with-aim` flag to add agent identity and audit logging during security remediation.

## aim-core: Local-First Agent Identity

Most AIM features require a running server. `@opena2a/aim-core` is the exception -- a lightweight library that gives any agent cryptographic identity, audit logging, capability enforcement, and trust scoring without a server, database, or network call.

**Why local identity matters**: AI agents execute code on your machine with your permissions. Without identity, there is no audit trail, no capability boundary, and no way to prove which agent did what.

| Feature | aim-core (local) | Full AIM (server) |
|---------|-----------------|-------------------|
| Ed25519 identity | Local keypair | Server-issued + OIDC |
| Audit log | JSON-lines file | PostgreSQL + API |
| Capability policy | YAML file | REST API + dashboard |
| Trust scoring | 8-factor local | Real-time + history |
| Multi-agent | Per-machine | Cross-machine fleet |

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

<p align="center">
  <img src="docs/vhs/aim-core.gif" alt="aim-core demo" width="700" />
</p>

Start local, upgrade to the full AIM platform when you need multi-agent governance.

## Usage via OpenA2A CLI

The [OpenA2A CLI](https://github.com/opena2a-org/opena2a) provides an `identity` adapter that wraps the AIM server API, giving you quick terminal access to identity management without writing code or calling the REST API directly.

**Install the CLI:**

```bash
npm install -g opena2a-cli
# or run without installing:
npx opena2a
```

**Commands:**

```bash
# List all registered agents
opena2a identity list

# Register a new agent
opena2a identity register --name my-agent

# Check an agent's trust score
opena2a identity trust <agent>
```

The CLI adapter connects to your local AIM server (default `http://localhost:8080`) or AIM Cloud. Configure the target with `opena2a config set aim.endpoint <url>`.

For the full CLI reference, see the [CLI documentation](https://opena2a.org/docs/cli/).

## Using with HackMyAgent fix-all

[HackMyAgent](https://github.com/opena2a-org/hackmyagent) `fix-all` runs all security plugins in sequence — credential vault, file signing, skill guard — and can optionally add AIM agent identity without requiring the full server and SDK setup.

```bash
hackmyagent fix-all                     # scan and fix
hackmyagent fix-all --dry-run           # preview without modifying
hackmyagent fix-all --with-aim          # add agent identity + audit logging
hackmyagent fix-all --json              # JSON output
```

**Plugins:**

| Plugin | What it does |
|--------|-------------|
| SkillGuard | Hash pinning, tamper detection, dangerous pattern scanning |
| SignCrypt | Ed25519 signing, SHA-256 hash pinning, signature verification |
| CredVault | Credential detection, env var replacement, AES-256-GCM encrypted store |

**`--with-aim`** adds Ed25519 agent identity, cryptographic audit log, and capability policy enforcement. This is a lightweight way to use AIM without deploying the full server — it uses the [`@opena2a/aim-core`](#aim-core-local-first-agent-identity) library under the hood.

## Ecosystem Integration

AIM connects to the broader OpenA2A security platform through multiple interfaces:

| Method | Command / Package | What It Does |
|--------|-------------------|--------------|
| CLI | `opena2a identity list\|register\|trust` | Terminal access to agent identity management |
| HackMyAgent | `hackmyagent fix-all --with-aim` | Adds agent identity and audit logging during security remediation |
| Node.js SDK | `npm install @opena2a/aim-core` | Programmatic integration for TypeScript/Node.js projects |
| Python SDK | `pip install aim-sdk` | Programmatic integration for Python projects |
| REST API | `http://localhost:8080/api/v1/` | Direct HTTP access to all AIM features |

## Links

- [Documentation](https://opena2a.org/docs) -- full guides, tutorials, API reference
- [SDK Quickstart](https://opena2a.org/docs/tutorials/sdk-quickstart) -- secure your first agent
- [MCP Registration](https://opena2a.org/docs/tutorials/mcp-registration) -- connect MCP servers
- [Contributing](CONTRIBUTING.md) -- how to contribute
- [Deployment Guide](infrastructure/DEPLOYMENT.md) -- production deployment (AWS, Azure, GCP, K8s)

### Ecosystem

| Project | Description | Install |
|---------|-------------|---------|
| [OpenA2A CLI](https://github.com/opena2a-org/opena2a) | Unified security CLI for all OpenA2A tools | `npx opena2a` |
| [HackMyAgent](https://github.com/opena2a-org/hackmyagent) | Security scanner and red-team toolkit (includes OASB + ARP) | `npx hackmyagent secure` |
| [Secretless AI](https://github.com/opena2a-org/secretless-ai) | Credential management -- keep secrets out of AI context | `npx secretless-ai init` |
| [AI Browser Guard](https://github.com/opena2a-org/AI-BrowserGuard) | Browser agent detection and control (Chrome MV3) | Chrome Web Store |
| [DVAA](https://github.com/opena2a-org/damn-vulnerable-ai-agent) | Deliberately vulnerable AI agent for security training | `docker pull opena2a/dvaa` |
| [Registry](https://registry.opena2a.org) | Agent trust registry and supply chain verification | [registry.opena2a.org](https://registry.opena2a.org) |

## License

Apache-2.0 -- See [LICENSE](LICENSE)
