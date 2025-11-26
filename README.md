# Agent Identity Management (AIM)

<div align="center">

**Production-grade identity, verification, and security management for autonomous AI agents and MCP servers**

[![License: AGPL-3.0](https://img.shields.io/badge/License-AGPL%203.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?logo=go)](https://go.dev/)
[![Next.js](https://img.shields.io/badge/Next.js-15-black?logo=next.js)](https://nextjs.org/)
[![TypeScript](https://img.shields.io/badge/TypeScript-5.5+-3178C6?logo=typescript)](https://www.typescriptlang.org/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-336791?logo=postgresql)](https://www.postgresql.org/)

[![GitHub Stars](https://img.shields.io/github/stars/opena2a-org/agent-identity-management?style=social)](https://github.com/opena2a-org/agent-identity-management/stargazers)

[📚 Documentation](https://opena2a.org/docs) • [🚀 Tutorials](https://opena2a.org/docs/tutorials) • [💬 Discord](https://discord.gg/uRZa3KXgEn)

</div>

---

## 🚨 AI Agents Are a Security Blind Spot

Your AI agents are making decisions, calling APIs, and accessing data — but can you answer:

- **Who** authorized this agent to act?
- **What** actions is it taking right now?
- **Why** did it access that sensitive data?
- **How** do you stop a compromised agent?

Without visibility, a single rogue agent can exfiltrate data, rack up API bills, or delete production databases — and you won't know until it's too late.

**AIM gives you control:** Cryptographic identity • Real-time monitoring • Trust scoring • Complete audit trails

---

## ⚡ See AIM Working in 60 Seconds

**Just run and watch your dashboard update in real-time.**

### Step 1: Start AIM (30 seconds)

```bash
git clone https://github.com/opena2a-org/agent-identity-management.git
cd agent-identity-management
docker compose up -d
```

Open http://localhost:3000 → Login: `admin@opena2a.org` / `AIM2025!Secure`

### Step 2: Download & Run Demo Agent (30 seconds)

```bash
# In the AIM dashboard: Settings → SDK Download → Download Python SDK

# Then in your terminal:
unzip ~/Downloads/aim-sdk-python.zip
cd aim-sdk-python
pip install -e .

# Run the interactive demo!
python demo_agent.py
```

### Step 3: Watch Your Dashboard Update! 🎉

Open **[http://localhost:3000/dashboard/agents](http://localhost:3000/dashboard/agents)** side-by-side with your terminal.

Trigger actions from the demo menu and watch:
- ✅ Agent registration appear instantly
- ✅ Trust scores update in real-time
- ✅ Activity logs populate as you trigger actions
- ✅ Different risk levels (low/medium/high) monitored differently

**That's it!** You just secured your first AI agent. 🚀

---

## 🛠️ Build Your Own Agent

Ready to build your own? It's just 3 lines:

```python
from aim_sdk import secure

agent = secure("my-agent")  # That's it - agent is secured!

@agent.track_action(risk_level="low")
def my_function(data):
    return api.call(data)  # Verified, logged, monitored
```

**Pro tip:** Copy `demo_agent.py` from the SDK and modify it for your use case!

For more details, see the [SDK Quickstart Tutorial](https://opena2a.org/docs/tutorials/sdk-quickstart).

---

## 📚 Learn More

| Resource | Time | Description |
|----------|------|-------------|
| [**SDK Quickstart**](https://opena2a.org/docs/tutorials/sdk-quickstart) | 2 min | Build your own agent with 3 lines of Python |
| [**API Quickstart**](https://opena2a.org/docs/tutorials/api-quickstart) | 3 min | REST API examples with curl |
| [**Dashboard Walkthrough**](https://opena2a.org/docs/tutorials/ui-walkthrough) | 3 min | Navigate the AIM dashboard |
| [**Register MCP Servers**](https://opena2a.org/docs/tutorials/mcp-registration) | 3 min | Connect and attest MCP servers |
| [**Full Documentation**](https://opena2a.org/docs) | — | Complete guides and API reference |

---

## 🎬 Platform Walkthrough

[![AIM Platform Walkthrough](https://img.youtube.com/vi/jji5XbxRHfk/maxresdefault.jpg)](https://youtu.be/jji5XbxRHfk)

**📺 [Watch the 5-minute walkthrough →](https://youtu.be/jji5XbxRHfk)** — See dashboard, trust scoring, MCP registration, and security monitoring in action.

---

## 🎯 Key Features

**Agent Identity Management**
- Ed25519 cryptographic signing for all agent communications
- Automatic key generation and rotation
- Secure credential storage using OS keyrings
- Agent registration and verification workflows

**MCP Server Attestation**
- Cryptographic verification of MCP servers
- Automatic detection from Claude Desktop config
- Capability mapping and access control
- Real-time connection monitoring

**Trust Scoring (8 Factors)**
1. **Agent History** — Past behavior and reliability
2. **MCP Attestation** — Verified server connections
3. **Action Risk Level** — Severity of requested actions
4. **Capability Violations** — Attempts to exceed permissions
5. **Frequency Analysis** — Unusual activity patterns
6. **Temporal Patterns** — Time-based behavior analysis
7. **Geographic Signals** — Location-based risk assessment
8. **Community Feedback** — Peer validation and reporting

**Compliance & Audit**
- Complete audit trail for every agent action
- Automated security policy enforcement
- Real-time compliance reporting

**Security Monitoring**
- Behavioral anomaly detection using ML
- Real-time threat alerts and notifications
- Automatic policy enforcement
- Security dashboard with metrics
- **Bulk alert management** — Acknowledge all alerts at once
- **Configuration drift detection** — Detect unauthorized MCP server connections

**Advanced Security Policies** (3 Policy Types)
1. **Unusual Activity Detection**
   - API rate spike detection with configurable thresholds
   - Off-hours access monitoring (detect logins outside business hours)
   - Unusual access pattern detection (tracking diverse resource access)

2. **Configuration Drift Monitoring**
   - Capability change detection (alerts on permission modifications)
   - Public key rotation tracking with approval requirements
   - Permission escalation detection for dangerous capabilities

3. **Unauthorized Access Control**
   - IP-based restrictions with whitelist and wildcard support
   - Time-based access control (day-of-week and hour restrictions)
   - Resource-level and action-level access control with pattern matching

---

## 💼 Use Cases

### AI Governance & Security
- **AI Agent Fleet Management** — Centralized identity management for hundreds of AI agents
- **LLM Security & Compliance** — Audit trails and access controls for LangChain, CrewAI agents
- **Autonomous Agent Authentication** — Cryptographic verification for self-operating agents
- **AI Risk Management** — Real-time trust scoring and behavioral anomaly detection

### Industry Applications
- **Healthcare AI (HIPAA Compliance)** — Secure patient data access for medical AI agents
- **Financial Services (SOC 2)** — Compliance-ready AI for trading and advisory agents
- **Legal AI (Confidentiality)** — Audit trails for document-processing agents
- **Customer Service Automation** — Identity management for chatbot and support agents

### Developer Workflows
- **GitHub Copilot Security** — Track and verify AI coding assistant actions
- **VS Code Extensions** — Secure AI-powered development tools
- **CI/CD Automation** — Identity management for build and deployment agents
- **DevOps AI Agents** — Authentication for infrastructure automation agents

---

## 🚀 Deployment

### Docker Compose (Recommended)

```bash
git clone https://github.com/opena2a-org/agent-identity-management.git
cd agent-identity-management
docker compose up -d
```

### Kubernetes

```bash
kubectl apply -f k8s/namespace.yaml
kubectl apply -f k8s/postgres.yaml
kubectl apply -f k8s/redis.yaml
kubectl apply -f k8s/backend.yaml
kubectl apply -f k8s/frontend.yaml
```

### Cloud Deployment

See [infrastructure/DEPLOYMENT.md](infrastructure/DEPLOYMENT.md) for:
- AWS deployment with ECS
- Azure deployment with Container Apps
- GCP deployment with Cloud Run
- Production best practices

### Environment Variables

<details>
<summary>Backend (Go)</summary>

```env
# Database
DATABASE_URL=postgresql://user:password@localhost:5432/aim

# Server
PORT=8080
ENVIRONMENT=production

# Security
JWT_SECRET=your-secret-key-here
CORS_ORIGINS=http://localhost:3000

# Features
ENABLE_TRUST_SCORING=true
ENABLE_MCP_ATTESTATION=true
ENABLE_ANOMALY_DETECTION=true
```
</details>

<details>
<summary>Frontend (Next.js)</summary>

```env
# API
NEXT_PUBLIC_API_URL=http://localhost:8080

# Features
NEXT_PUBLIC_ENABLE_ANALYTICS=true
NEXT_PUBLIC_ENVIRONMENT=production
```
</details>

---

## 🔐 Security

### Cryptographic Design

**Ed25519 Signing**
- All agent communications signed with Ed25519
- 256-bit keys generated on agent registration
- Signatures verified on every API request
- Keys rotated automatically every 90 days

**MCP Attestation**
- MCP servers cryptographically attested
- Public key infrastructure for verification
- Certificate chain validation
- Revocation list maintained
- **Configuration drift detection** — Alerts when agents connect to unregistered MCP servers

**Session Security**
- Automatic session expiry detection with graceful redirect
- Toast notifications for expired sessions
- Secure token refresh handling

**Zero-Trust Architecture**
- No implicit trust between components
- Every action requires verification
- Least privilege access control
- Complete audit trail

### Threat Model

**Protected Against**:
- ✅ Prompt injection attacks
- ✅ Agent impersonation
- ✅ MCP server spoofing
- ✅ Unauthorized capability use
- ✅ Behavioral anomalies
- ✅ Credential theft
- ✅ Man-in-the-middle attacks

**Out of Scope**:
- ❌ Model jailbreaking (LLM provider responsibility)
- ❌ Physical server compromise (infrastructure responsibility)
- ❌ Browser-based attacks (user responsibility)

---

## 🤝 Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

### Development Setup

```bash
# Clone repository
git clone https://github.com/opena2a-org/agent-identity-management.git
cd agent-identity-management

# Start development environment
docker compose -f docker-compose.dev.yml up -d

# Run tests
./scripts/run-tests.sh
```

### Code Standards

- **Go**: Follow [Effective Go](https://golang.org/doc/effective_go)
- **TypeScript**: Use strict mode, follow Airbnb style guide
- **Testing**: Minimum 80% code coverage
- **Security**: All PRs scanned with Snyk and gosec

---

## 🆚 Comparison

### AIM vs. Traditional Security

| Traditional Security | AIM |
|---------------------|-----|
| ❌ Manual agent registration | ✅ One-line `secure()` registration |
| ❌ Static API keys | ✅ Cryptographic signatures (Ed25519) |
| ❌ No MCP verification | ✅ Cryptographic MCP attestation |
| ❌ No trust scoring | ✅ ML-powered 8-factor trust scoring |
| ❌ Reactive monitoring | ✅ Real-time anomaly detection |
| ❌ Compliance headaches | ✅ Automated audit trails |
| ❌ Scattered monitoring | ✅ Unified security dashboard |
| ❌ React after breaches | ✅ Prevent before they happen |

---

## Support & Resources

### Documentation

| Resource | Link |
|----------|------|
| **Full Documentation** | [**opena2a.org/docs**](https://opena2a.org/docs) |
| 5-Minute Tutorials | [opena2a.org/docs/tutorials](https://opena2a.org/docs/tutorials) |
| API Reference | [opena2a.org/docs/aim/api-reference](https://opena2a.org/docs/aim/api-reference) |
| SDK Guide | [opena2a.org/docs/api/sdks](https://opena2a.org/docs/api/sdks) |

### Community

- **📧 Email**: [info@opena2a.org](mailto:info@opena2a.org)
- **💬 Discord**: [Join our community](https://discord.gg/uRZa3KXgEn)
- **🔗 Website**: [opena2a.org](https://opena2a.org)

---

## Roadmap

### Q4 2025 ✅ (Completed)
- [x] Core platform with 160 API endpoints
- [x] MCP attestation and verification
- [x] 8-factor trust scoring
- [x] Capability request workflow
- [x] Python SDK with one-line `secure()`
- [x] Admin UI with real-time updates
- [x] Production deployment on Azure

### Q1-Q2 2026 🔄 (In Progress)
- [ ] GraphQL API
- [ ] CLI tool for automation
- [ ] Terraform provider
- [ ] JavaScript/TypeScript SDK

---

<div align="center">

⭐ **Star us on GitHub** if AIM helps secure your AI agents!

</div>

---
## 📄 License

GNU Affero General Public License v3.0 (AGPL-3.0) - See [LICENSE](LICENSE) for details.
---

## 🏷️ Search Topics

<div align="center">

`ai-security` `agent-identity` `ai-agent-management` `mcp-servers` `machine-learning-security` `zero-trust` `authentication` `authorization` `audit-logging` `compliance` `hipaa` `soc2` `gdpr` `langchain` `crewai` `autonomous-agents` `trust-scoring` `threat-detection` `anomaly-detection` `cryptography` `ed25519` `golang` `nextjs` `typescript` `postgresql` `kubernetes` `docker` `cybersecurity` `devops` `mlops` `aiops` `identity-management` `access-control` `rbac` `security-automation` `vulnerability-management` `risk-management` `ai-governance` `llm-security` `prompt-injection` `ai-safety`

</div>
