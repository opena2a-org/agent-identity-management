> **[OpenA2A](https://github.com/opena2a-org/opena2a)**: [Secretless](https://github.com/opena2a-org/secretless-ai) · [HackMyAgent](https://github.com/opena2a-org/hackmyagent) · [ABG](https://github.com/opena2a-org/AI-BrowserGuard) · [OASB](https://github.com/opena2a-org/oasb) · [ARP](https://github.com/opena2a-org/arp) · [DVAA](https://github.com/opena2a-org/damn-vulnerable-ai-agent)

<div align="center">

# Agent Identity Management (AIM)

**Open-source identity, governance, and access control for AI agents.**

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
| TypeScript | Coming soon | In progress |

## Usage via OpenA2A CLI

The [OpenA2A CLI](https://github.com/opena2a-org/opena2a) provides an `identity` adapter that wraps the AIM server API, giving you quick terminal access to identity management without writing code or calling the REST API directly.

**Install the CLI:**

```bash
npm install -g @opena2a/cli
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

## Links

- [Documentation](https://opena2a.org/docs) -- full guides, tutorials, API reference
- [SDK Quickstart](https://opena2a.org/docs/tutorials/sdk-quickstart) -- secure your first agent
- [MCP Registration](https://opena2a.org/docs/tutorials/mcp-registration) -- connect MCP servers
- [Contributing](CONTRIBUTING.md) -- how to contribute
- [Deployment Guide](infrastructure/DEPLOYMENT.md) -- production deployment (AWS, Azure, GCP, K8s)

### Ecosystem

| Project | Description | Install |
|---------|-------------|---------|
| [HackMyAgent](https://github.com/opena2a-org/hackmyagent) | Security scanner -- 147 checks, attack mode, auto-fix | `npx hackmyagent secure` |
| [OASB](https://github.com/opena2a-org/oasb) | Open Agent Security Benchmark -- 222 attack scenarios | Included in `hackmyagent` |
| [ARP](https://github.com/opena2a-org/arp) | Agent Runtime Protection -- process, network, filesystem monitoring | Included in `hackmyagent` |
| [Secretless AI](https://github.com/opena2a-org/secretless-ai) | Keep credentials out of AI context windows | `npx secretless-ai init` |
| [DVAA](https://github.com/opena2a-org/damn-vulnerable-ai-agent) | Damn Vulnerable AI Agent -- security training | `docker pull opena2a/dvaa` |

## License

Apache-2.0 -- See [LICENSE](LICENSE)
