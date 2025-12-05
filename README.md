# AIM — Agent Identity Management

<div align="center">

**Stop your AI agents from going rogue.**

[![License: AGPL-3.0](https://img.shields.io/badge/License-AGPL%203.0-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.23+-00ADD8?logo=go)](https://go.dev/)
[![Next.js](https://img.shields.io/badge/Next.js-15-black?logo=next.js)](https://nextjs.org/)

[📚 Docs](https://opena2a.org/docs) • [🔒 Security](https://opena2a.org/docs/security-assessment) • [🚀 Tutorials](https://opena2a.org/docs/tutorials) • [💬 Discord](https://discord.gg/uRZa3KXgEn) • [📺 Demo Video](https://youtu.be/jji5XbxRHfk)

</div>

---

## The Problem

Your AI agents are making decisions, calling APIs, and accessing data. But can you answer:

- **Who** authorized this agent?
- **What** is it doing right now?
- **How** do you stop a compromised agent?

Without visibility, a single rogue agent can exfiltrate data, rack up API bills, or delete production databases.

**AIM gives you control.**

---

## 60-Second Demo

```bash
# 1. Start AIM
git clone https://github.com/opena2a-org/agent-identity-management.git && cd agent-identity-management
docker compose up -d

# 2. Open dashboard
open http://localhost:3000   # Login: admin@opena2a.org / AIM2025!Secure

# 3. Download SDK (Dashboard → Settings → SDK Download)
# 4. Run the demo
cd aim-sdk-python && pip install -e . && python demo_agent.py
```

Watch your dashboard update in real-time as the agent registers, gets verified, and performs actions.

---

## Secure Any Agent in 3 Lines

```python
from aim_sdk import secure, perform_action

agent = secure("my-agent")

@perform_action(capability="api:call", risk_level="low")
def call_api(data):
    return requests.get(data["url"])  # Verified, logged, monitored
```

That's it. Your agent now has:
- ✅ **Cryptographic identity** (Ed25519 keys)
- ✅ **Capability enforcement** (blocks unauthorized actions)
- ✅ **Trust scoring** (behavioral risk assessment)
- ✅ **Complete audit trail** (who did what, when)

---

## What AIM Does

| Capability | What It Means |
|-----------|---------------|
| **Cryptographic Identity** | Every agent gets unforgeable Ed25519 keys. No impersonation possible. |
| **Capability Enforcement** | Agents can only do what they're declared to do. Prompt injection? Blocked. |
| **MCP Server Attestation** | Verify the tool servers your agents connect to. Detect configuration drift. |
| **Trust Scoring** | 8-factor algorithm tracks agent behavior. Violations reduce trust. |
| **JIT Access** | Sensitive operations require real-time admin approval. |
| **Security Dashboard** | Real-time alerts, threat detection, source IP tracking. |

---

## Why AIM?

| Without AIM | With AIM |
|-------------|----------|
| Attacker says "I'm agent-007" → System trusts it | Attacker can't sign the cryptographic challenge → Blocked |
| Prompt injection tricks agent into exfiltrating data | Capability enforcement blocks the action at API layer |
| Agent connects to malicious MCP server | MCP attestation detects unregistered server → Alert |
| You find out after the breach | Real-time monitoring catches anomalies immediately |

---

## Key Features

- **🔐 Ed25519 Cryptographic Signing** — Unforgeable agent identity
- **🛡️ Capability-Based Access Control** — Blocks prompt injection attacks
- **🔌 MCP Server Attestation** — Verify tool servers, detect drift
- **📊 8-Factor Trust Scoring** — Behavioral risk assessment
- **⏱️ JIT Access** — Real-time admin approval for sensitive ops
- **🚨 Security Dashboard** — Alerts, threat detection, IP tracking
- **✅ 10 Compliance Checks** — Automated security & operations checks
- **📝 Complete Audit Trail** — Who did what, when, why

---

## Get Started

### Option 1: Docker (Recommended)

```bash
git clone https://github.com/opena2a-org/agent-identity-management.git
cd agent-identity-management
docker compose up -d
```

Open http://localhost:3000 — Login: `admin@opena2a.org` / `AIM2025!Secure`

### Option 2: Kubernetes

```bash
kubectl apply -f k8s/
```

See [deployment docs](infrastructure/DEPLOYMENT.md) for AWS, Azure, GCP.

---

## Documentation

| Resource | Description |
|----------|-------------|
| [**SDK Quickstart**](https://opena2a.org/docs/tutorials/sdk-quickstart) | Build your first secured agent (2 min) |
| [**API Quickstart**](https://opena2a.org/docs/tutorials/api-quickstart) | REST API examples with curl (3 min) |
| [**MCP Registration**](https://opena2a.org/docs/tutorials/mcp-registration) | Connect and attest MCP servers (3 min) |
| [**Full Docs**](https://opena2a.org/docs) | Complete guides and API reference |

---

## Platform Walkthrough

[![AIM Platform Walkthrough](https://img.youtube.com/vi/jji5XbxRHfk/maxresdefault.jpg)](https://youtu.be/jji5XbxRHfk)

**[Watch the 5-minute walkthrough →](https://youtu.be/jji5XbxRHfk)**

---

## Use Cases

- **AI Agent Fleet Management** — Centralized identity for hundreds of agents
- **LLM Security & Compliance** — Audit trails for LangChain, CrewAI agents
- **Healthcare AI (HIPAA)** — Secure patient data access
- **Financial Services (SOC 2)** — Compliance-ready AI agents
- **DevOps Automation** — Identity for CI/CD and infrastructure agents

---

## Contributing

We welcome contributions! See [CONTRIBUTING.md](CONTRIBUTING.md).

---

## Community

- **📧 Email**: [info@opena2a.org](mailto:info@opena2a.org)
- **💬 Discord**: [Join us](https://discord.gg/uRZa3KXgEn)
- **🔗 Website**: [opena2a.org](https://opena2a.org)

---

## License

AGPL-3.0 — See [LICENSE](LICENSE)

---

<div align="center">

⭐ **Star us** if AIM helps secure your AI agents!

</div>
