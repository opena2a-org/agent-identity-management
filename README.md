<div align="center">

# Agent Identity Management (AIM)

**IAM for AI agents — cryptographic identity, access control, and accountability.**

[![Backend Coverage](https://img.shields.io/badge/Backend%20Coverage-70%25+-brightgreen)](apps/backend)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.23+-00ADD8?logo=go)](https://go.dev/)
[![Python](https://img.shields.io/badge/Python-3.8+-3776AB?logo=python)](https://python.org/)

[📺 Demo Video](https://youtu.be/meD_LW5fc_A) • [📚 Docs](https://opena2a.org/docs) • [💬 Discord](https://discord.gg/uRZa3KXgEn)

</div>

---

## Quick Start

```bash
# 1. Clone and start
git clone https://github.com/opena2a-org/agent-identity-management.git
cd agent-identity-management
docker compose up -d

# 2. Open dashboard
open http://localhost:3000
# Login: admin@opena2a.org / AIM2025!Secure

# 3. Download SDK from Settings → SDK Download
```

**Or use AIM Cloud:** [aim.opena2a.org](https://aim.opena2a.org) — no infrastructure required.

---

## Secure Your Agent (One Line)

```python
from aim_sdk import secure

agent = secure("my-assistant", capabilities=["db:read", "api:call"])

@agent.perform_action(capability="db:read")
def get_customer(id):
    return db.query(id)
```

That's it. Your agent now has:
- Cryptographic identity (Ed25519)
- Capability enforcement
- Full audit trail
- Real-time monitoring

---

## What AIM Solves

| Problem | Without AIM | With AIM |
|---------|-------------|----------|
| Agent impersonation | Anyone can claim to be your agent | Cryptographic proof required |
| Prompt injection | Agent tricked into unauthorized actions | Capability enforcement blocks it |
| No visibility | What are your agents doing? | Complete audit trail |
| MCP supply chain | Unknown servers, no verification | MCP attestation + drift detection |

---

## Features

<details>
<summary><strong>Dashboard & Monitoring</strong></summary>

Real-time visibility into your AI agent fleet:

- **Trust Scoring** — 8-factor algorithm evaluates agent trustworthiness
- **Security Alerts** — Severity-based alerts with acknowledgment workflow
- **Activity Timeline** — Every action, verification, and MCP connection
- **Compliance Checks** — 10 automated checks for security compliance

![Dashboard](docs/images/dashboard-executive.png)

</details>

<details>
<summary><strong>MCP Server Attestation</strong></summary>

Verify and monitor Model Context Protocol servers:

- **Auto-Attestation** — SDK automatically attests MCP servers on first use
- **Drift Detection** — Alerts when server tools change unexpectedly
- **Supply Chain View** — See all MCP servers across your organization

![MCP Attestations](docs/images/mcp-attestations.png)

</details>

<details>
<summary><strong>Capability-Based Access Control</strong></summary>

Define what each agent can do. Block everything else.

- **Declared Capabilities** — `db:read`, `api:call`, `file:write`, etc.
- **Runtime Enforcement** — Unauthorized actions blocked at API layer
- **Risk Level Detection** — Automatic risk assessment from patterns

</details>

<details>
<summary><strong>Just-In-Time Access</strong></summary>

Sensitive operations require admin approval:

- **Request Workflow** — Agents request elevated access
- **Time-Limited** — Access expires automatically
- **Full Audit Trail** — Every request logged

![Capability Requests](docs/images/capability-requests.png)

</details>

<details>
<summary><strong>Security Policies</strong></summary>

- **MONITORING Mode** — Observe and alert without blocking (development)
- **STRICT Mode** — Enforce policies and block violations (production)
- **Data Exfiltration Detection** — Detect unusual data transfer patterns

![Security Policies](docs/images/security-policies.png)

</details>

---

## SDKs

| SDK | Install | Status |
|-----|---------|--------|
| **Python** | `pip install aim-sdk` | ✅ Stable |
| **Java** | Maven/Gradle | ✅ Stable |
| **TypeScript** | Coming soon | 🚧 In progress |

### Python Example

```python
from aim_sdk import secure, AgentType

agent = secure(
    "my-agent",
    agent_type=AgentType.LANGCHAIN,
    capabilities=["db:read", "api:call"],
    tags=["production"],
    metadata={"model": "gpt-4"}
)

@agent.perform_action(capability="db:read")
def query_users():
    return database.get_users()
```

### Java Example

```java
AIMClient agent = AIMClient.secure(
    "my-agent",
    Arrays.asList("db:read", "api:call"),
    AgentType.LANGCHAIN
);

User user = agent.performAction("db:read", "users", () -> {
    return userRepository.findById(userId);
});
```

---

## Architecture

AIM uses **direct observation** — not proxies. Agents report actions explicitly via SDK decorators while communicating directly with target systems.

```
┌─────────────────────────────────────────────────────────────┐
│  YOUR AGENT                                                  │
│  ┌────────────────────────────────────────────────────────┐ │
│  │ @agent.perform_action(capability="db:read")            │ │
│  │ def get_data():                                        │ │
│  │     return database.query()  ─────────────────────────────► Target
│  └────────────────────────────────────────────────────────┘ │  (Direct)
│                    │                                         │
│                    │ Reports action                          │
│                    ▼                                         │
│              AIM Backend                                     │
│         (Verify + Log + Enforce)                             │
└─────────────────────────────────────────────────────────────┘
```

**Key points:**
- Zero latency added to agent↔target communication
- No single point of failure (fail-open mode available)
- Works with any MCP server, API, or database

---

## Deployment

```bash
# Development
docker compose up -d

# Production (Kubernetes)
kubectl apply -f k8s/
```

See [infrastructure/DEPLOYMENT.md](infrastructure/DEPLOYMENT.md) for AWS, Azure, and GCP guides.

---

## Documentation

- [SDK Quickstart](https://opena2a.org/docs/tutorials/sdk-quickstart) — Secure your first agent
- [MCP Registration](https://opena2a.org/docs/tutorials/mcp-registration) — Connect MCP servers
- [Post-Quantum Cryptography](docs/guides/PQC.md) — ML-DSA signatures
- [Full Documentation](https://opena2a.org/docs)

---

## Contributing

We welcome contributions! See [CONTRIBUTING.md](CONTRIBUTING.md).

---

## Community

- **Discord**: [discord.gg/uRZa3KXgEn](https://discord.gg/uRZa3KXgEn)
- **Email**: [info@opena2a.org](mailto:info@opena2a.org)
- **Website**: [opena2a.org](https://opena2a.org)

---

## License

Apache-2.0 — See [LICENSE](LICENSE)

---

<div align="center">

⭐ **Star this repo** if AIM helps secure your AI agents!

</div>
