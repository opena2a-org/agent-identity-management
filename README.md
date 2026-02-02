<div align="center">

# AIM — Identity Management for AI Agents

**Secure your AI agents with one line of code.**

Cryptographic identity. Capability enforcement. Complete audit trail.

[![Backend Coverage](https://img.shields.io/badge/Coverage-70%25+-brightgreen)](apps/backend)
[![Go](https://img.shields.io/badge/Go-1.23+-00ADD8?logo=go)](https://go.dev/)
[![Python](https://img.shields.io/badge/Python-3.8+-3776AB?logo=python)](https://python.org/)
[![Java](https://img.shields.io/badge/Java-17+-ED8B00?logo=openjdk)](https://openjdk.org/)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

[Live Demo](https://aim.opena2a.org) • [Documentation](https://opena2a.org/docs) • [Discord](https://discord.gg/uRZa3KXgEn)

</div>

---

## Quick Start

```bash
pip install aim-sdk
```

```python
from aim_sdk import secure

agent = secure("my-agent")

@agent.perform_action("db:read")
def get_customer(id):
    return database.query(id)
```

That's it. Your agent now has cryptographic identity, capability enforcement, and full audit logging.

**Java SDK also available** — see [Java Quick Start](#java-quick-start) below.

---

## Why AIM?

Your AI agents call APIs, access databases, and make autonomous decisions. But right now:

| Problem | Risk |
|---------|------|
| No agent identity | Attackers can impersonate your agents |
| No capability enforcement | Prompt injection bypasses your guardrails |
| No audit trail | You find out about breaches weeks later |

**AIM fixes this** — without proxies, without latency, without rewriting your code.

---

## Features

| Feature | Description |
|---------|-------------|
| **Cryptographic Identity** | Ed25519 signatures + post-quantum ML-DSA |
| **Capability Enforcement** | Block unauthorized actions at runtime |
| **MCP Attestation** | Verify MCP server integrity and detect drift |
| **Trust Scoring** | 8-factor behavioral analysis |
| **Audit Trail** | SOC 2 / HIPAA / GDPR ready |
| **Supply Chain Visibility** | ABOM (Agent Bill of Materials) |

---

## Self-Hosted Deployment

```bash
git clone https://github.com/opena2a-org/agent-identity-management.git
cd agent-identity-management
docker compose up -d
```

Open http://localhost:3000

Login: `admin@opena2a.org` / `AIM2025!Secure`

---

## Java Quick Start

```xml
<dependency>
    <groupId>org.opena2a</groupId>
    <artifactId>aim-sdk</artifactId>
    <version>0.1.0</version>
</dependency>
```

```java
import org.opena2a.aim.client.AIMClient;

AIMClient agent = AIMClient.secure("my-agent",
    Arrays.asList("db:read", "api:call"));

User user = agent.performAction("db:read", "users", () -> {
    return userRepository.findById(userId);
});
```

---

## How It Works

AIM uses **direct observation**, not proxies. Your agents communicate directly with target systems — AIM just observes and enforces.

```
┌─────────────────────────────────────────────────────────────┐
│  @agent.perform_action("db:read")                           │
│  def get_customer(id):                                      │
│      return database.query(id)  ─────────────────────────────┼──► Database
│                    │                                        │    (Direct)
│                    │ Reports action                         │
│                    ▼                                        │
│              AIM Backend                                    │
│         (Verify + Log + Enforce)                            │
└─────────────────────────────────────────────────────────────┘
```

**Zero latency added** to agent↔target communication. Verification happens in parallel.

---

## Screenshots

<details>
<summary><b>Executive Dashboard</b> — Real-time monitoring, trust scores, alerts</summary>

![Executive Dashboard](docs/images/dashboard-executive.png)

</details>

<details>
<summary><b>Security Command Center</b> — Threat detection, blocked actions, risk analysis</summary>

![Security Command Center](docs/images/dashboard-security.png)

</details>

<details>
<summary><b>MCP Supply Chain</b> — Server attestation, drift detection, dependencies</summary>

![MCP Supply Chain](docs/images/supply-chain.png)

</details>

<details>
<summary><b>Trust Scoring</b> — 8-factor behavioral analysis</summary>

![Trust Score](docs/images/agent-trust-score.png)

</details>

<details>
<summary><b>Capability Requests</b> — Just-in-time access with admin approval</summary>

![Capability Requests](docs/images/capability-requests.png)

</details>

---

## Documentation

| Resource | Description |
|----------|-------------|
| [SDK Quickstart](https://opena2a.org/docs/tutorials/sdk-quickstart) | Secure your first agent (2 min) |
| [API Reference](https://opena2a.org/docs/api) | REST API documentation |
| [MCP Attestation](https://opena2a.org/docs/tutorials/mcp-registration) | Connect and verify MCP servers |
| [Post-Quantum Crypto](docs/guides/PQC.md) | ML-DSA signatures and hybrid mode |
| [Full Documentation](https://opena2a.org/docs) | Complete guides |

---

## Roadmap

**Current (v1.0):** Ed25519 + ML-DSA identity, capability enforcement, MCP attestation, trust scoring, Python/Java/TypeScript SDKs, OAuth, ABOM

**Next:** GitHub Copilot integration, GraphQL API, CLI tool, webhook integrations

See [ROADMAP.md](ROADMAP.md) for details.

---

## Community

- [Discord](https://discord.gg/uRZa3KXgEn) — Chat with the team
- [GitHub Issues](https://github.com/opena2a-org/agent-identity-management/issues) — Bug reports & feature requests
- [Twitter](https://twitter.com/opena2a) — Updates
- [Email](mailto:info@opena2a.org) — info@opena2a.org

---

## Contributing

We welcome contributions! See [CONTRIBUTING.md](CONTRIBUTING.md).

---

## License

Apache-2.0 — See [LICENSE](LICENSE)

---

<div align="center">

**Built by the founders of [CyberSecurity NonProfit](https://csnp.org) (12,000+ security professionals)**

⭐ Star this repo if AIM helps secure your AI agents

</div>
