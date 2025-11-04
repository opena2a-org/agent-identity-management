# Agent Identity Management (AIM)

<div align="center">

**Production-grade identity, verification, and security management for autonomous AI agents and MCP servers**

Real-time threat detection • Zero-trust architecture • Self-hosted & cloud-native

[![License: AGPL-3.0](https://img.shields.io/badge/License-AGPL%203.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?logo=go)](https://go.dev/)
[![Next.js](https://img.shields.io/badge/Next.js-15-black?logo=next.js)](https://nextjs.org/)
[![TypeScript](https://img.shields.io/badge/TypeScript-5.5+-3178C6?logo=typescript)](https://www.typescriptlang.org/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-336791?logo=postgresql)](https://www.postgresql.org/)
[![API Endpoints](https://img.shields.io/badge/API%20Endpoints-136-brightgreen.svg)](#-technical-reference)

[![GitHub Stars](https://img.shields.io/github/stars/opena2a-org/agent-identity-management?style=social)](https://github.com/opena2a-org/agent-identity-management/stargazers)
[![GitHub Forks](https://img.shields.io/github/forks/opena2a-org/agent-identity-management?style=social)](https://github.com/opena2a-org/agent-identity-management/network/members)
[![GitHub Issues](https://img.shields.io/github/issues/opena2a-org/agent-identity-management)](https://github.com/opena2a-org/agent-identity-management/issues)
[![GitHub Pull Requests](https://img.shields.io/github/issues-pr/opena2a-org/agent-identity-management)](https://github.com/opena2a-org/agent-identity-management/pulls)

[Documentation](https://opena2a.org/docs) • [Python SDK](sdk/python/README.md) • [Quick Start](#-quick-start)

</div>

---

## 🎯 Why AIM?

### The Problem: AI Agents Are Under Attack

In 2024-2025, AI security incidents hit unprecedented levels:

- **CVE-2025-32711 (EchoLeak)**: CVSS 9.3 critical vulnerability in Microsoft Copilot — zero-click exploit allowing remote code execution through malicious workspace files
- **73% of organizations** experienced AI security incidents with an average cost of **$4.8 million per incident**
- **41% of AI incidents** are prompt injection attacks targeting agent workflows
- Major 2024 breaches: GPT-Store bot API keys exposed, Vanna.AI arbitrary code execution (CVE-2024-5565), ChatGPT search vulnerability

**The hard truth**: Every AI agent deployed without identity management is a potential attack vector. One compromised agent can expose your entire infrastructure, customer data, and compliance standing.

### The Solution: AIM

AIM provides the security infrastructure AI agents need to operate safely in production:

- **🔒 Cryptographic Identity** — Ed25519 signing for agent authentication and action verification
- **🛡️ MCP Server Attestation** — Cryptographically verify every MCP server your agents connect to
- **⚡ One-Line Security** — `secure("my-agent")` protects agents instantly with zero config
- **📊 8-Factor Trust Scoring** — ML-powered risk assessment for every agent and action
- **👮 Automated Compliance** — Complete audit trails
- **🚨 Real-Time Threat Detection** — Behavioral anomaly detection and automatic policy enforcement
- **🔐 Zero-Trust Architecture** — Every action verified, every MCP attested, every risk scored

**Perfect for:** Teams deploying AI agents at scale, security teams managing agent fleets, compliance officers requiring audit trails, developers building agent-based systems.

---

## ⚡ Quick Start

### 1. Deploy AIM (One Command)

```bash
git clone https://github.com/opena2a-org/agent-identity-management.git
cd agent-identity-management
docker compose up -d
```

**Services**:
- Frontend: http://localhost:3000 (Login and download SDK)
- Backend API: http://localhost:8080
- PostgreSQL: localhost:5432
- Redis: localhost:6379

### 2. Download SDK from Dashboard
```bash
# 1. Log in to AIM at http://localhost:3000
# 2. Go to Settings → SDK Download
# 3. Download SDK with pre-configured credentials
# 4. Extract and you're ready to go!
```

**Note**: There is NO pip package. The SDK must be downloaded from your AIM instance as it contains your personal credentials.

### 3. Secure Your First Agent (One Line!)
```python
from aim_sdk import secure

# Secure your agent with one line
client = secure(
    "my-agent-name",
    agent_type="ai_agent",
    auto_detect_capabilities=True,  # ✨ Auto-detects what your agent can do
    auto_detect_mcps=True           # ✨ Auto-detects MCP servers from config
)

# Your agent is now:
# ✅ Registered with AIM
# ✅ Cryptographically signed (Ed25519)
# ✅ Trust score calculated
# ✅ Capabilities detected
# ✅ MCPs verified
# ✅ Ready for production
```

### 4. Verify Actions Before Execution
```python
# Before performing sensitive operations
verification = client.verify_action(
    action_type="database_query",
    action_details={
        "query": "SELECT * FROM users",
        "risk_level": "medium"
    }
)

if verification.approved:
    # Execute the action
    results = execute_query(verification.parameters["query"])

    # Log the result
    client.log_action_result(
        action_type="database_query",
        success=True,
        metadata={"rows_returned": len(results)}
    )
```

**That's it!** Your agent now has enterprise-grade security with complete audit trails.

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

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         AIM Platform                            │
│                                                                  │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐         │
│  │   Backend    │  │   Frontend   │  │   Database   │         │
│  │   (Go 1.23)  │  │  (Next.js)   │  │ (PostgreSQL) │         │
│  │   Fiber v3   │  │  React 19    │  │      16      │         │
│  └──────┬───────┘  └──────────────┘  └──────────────┘         │
│         │                                                        │
│         │  REST API (136 endpoints)                             │
│         │                                                        │
└─────────┼────────────────────────────────────────────────────────┘
          │
          │  HTTPS + Ed25519 Signing
          │
┌─────────▼────────────────────────────────────────────────────────┐
│                      Your AI Agents                              │
│                                                                  │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐            │
│  │  LangChain  │  │   CrewAI    │  │    Custom   │            │
│  │   Agents    │  │   Agents    │  │   Agents    │            │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘            │
│         │                 │                 │                   │
│         └─────────────────┴─────────────────┘                   │
│                           │                                      │
│                    AIM SDK (Python)                              │
│                   secure("agent-name")                           │
└──────────────────────────────────────────────────────────────────┘
```

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

## 🧪 Development

### Backend (Go)

```bash
cd backend

# Install dependencies
go mod download

# Run tests
go test ./...

# Run with hot reload
air

# Build
go build -o aim-backend cmd/server/main.go
```

### Frontend (Next.js)

```bash
cd frontend

# Install dependencies
npm install

# Run development server
npm run dev

# Build for production
npm run build

# Run production build
npm start
```

### Database Migrations

```bash
# Run migrations
cd backend
go run cmd/migrate/main.go up

# Rollback migration
go run cmd/migrate/main.go down

# Create new migration
go run cmd/migrate/main.go create <migration_name>
```

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

## 📄 License

GNU Affero General Public License v3.0 (AGPL-3.0) - See [LICENSE](LICENSE) for details.

**Why AGPL?** We believe in open-source security infrastructure. AGPL ensures that any modifications to AIM remain open-source, even when deployed as a service.

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

## 📞 Support & Resources

- **📖 Comprehensive Documentation**: [opena2a.org/docs](https://opena2a.org/docs) — Complete guides, tutorials, and API reference
- **📧 Email**: [info@opena2a.org](mailto:info@opena2a.org)
- **💬 Discord**: [Join our community](https://discord.gg/uRZa3KXgEn)
- **🔗 Website**: [opena2a.org](https://opena2a.org)

---

## 🗺️ Roadmap

### Q4 2025 ✅ (Completed)
- [x] Core platform with 136 API endpoints
- [x] MCP attestation and verification
- [x] 8-factor trust scoring
- [x] Capability request workflow
- [x] Python SDK with one-line `secure()`
- [x] Admin UI with real-time updates
- [x] Production deployment on Azure

### Q1 2026 🔄 (In Progress)
- [ ] GraphQL API
- [ ] CLI tool for automation
- [ ] Terraform provider
- [ ] JavaScript/TypeScript SDK

---

## 📚 Technical Reference

<details>
<summary><h3>📊 API Overview (136 Endpoints)</h3></summary>

### Agent Management (12 endpoints)
```
POST   /api/v1/agents/register          # Register new agent
GET    /api/v1/agents/:id                # Get agent details
PATCH  /api/v1/agents/:id                # Update agent
DELETE /api/v1/agents/:id                # Delete agent
POST   /api/v1/agents/:id/verify         # Verify agent signature
GET    /api/v1/agents/:id/credentials    # Get API credentials
POST   /api/v1/agents/:id/rotate-key     # Rotate agent keys
GET    /api/v1/agents/:id/trust-score    # Get trust score
GET    /api/v1/agents/:id/activity       # Get activity logs
GET    /api/v1/agents/:id/violations     # Get violations
GET    /api/v1/agents/:id/key-vault      # Get key vault info
GET    /api/v1/agents/:id/mcp-servers    # MCP connections
```

### MCP Server Management (15 endpoints)
```
POST   /api/v1/mcp-servers/register      # Register MCP server
GET    /api/v1/mcp-servers/:id           # Get MCP details
PATCH  /api/v1/mcp-servers/:id           # Update MCP
DELETE /api/v1/mcp-servers/:id           # Delete MCP
POST   /api/v1/mcp-servers/:id/attest    # Attest MCP server
GET    /api/v1/mcp-servers/:id/agents    # Connected agents
POST   /api/v1/mcp-servers/:id/verify    # Verify attestation
GET    /api/v1/mcp-servers/:id/capabilities  # Get capabilities
POST   /api/v1/mcp-servers/:id/revoke    # Revoke attestation
...
```

### Trust Scoring (6 endpoints)
```
GET    /api/v1/trust-scores/:agent_id     # Current score
GET    /api/v1/trust-scores/:agent_id/history  # Score history
POST   /api/v1/trust-scores/:agent_id/calculate  # Recalculate
GET    /api/v1/trust-scores/:agent_id/factors    # Score breakdown
GET    /api/v1/trust-scores/aggregate      # Aggregate scores
POST   /api/v1/trust-scores/:agent_id/override   # Manual override
```

### Security Monitoring (7 endpoints)
```
GET    /api/v1/security/threats            # List threats
GET    /api/v1/security/anomalies          # Detected anomalies
GET    /api/v1/security/alerts             # Active alerts
POST   /api/v1/security/alerts/:id/acknowledge  # Acknowledge alert
GET    /api/v1/security/metrics            # Security metrics
GET    /api/v1/security/policies           # Security policies
POST   /api/v1/security/policies           # Create policy
```

### Capability Management (8 endpoints)
```
POST   /api/v1/capabilities/grant          # Grant capability
POST   /api/v1/capabilities/revoke         # Revoke capability
GET    /api/v1/capabilities/:agent_id      # List capabilities
POST   /api/v1/capabilities/request        # Request capability
GET    /api/v1/capabilities/requests       # List requests
POST   /api/v1/capabilities/approve/:id    # Approve request
POST   /api/v1/capabilities/reject/:id     # Reject request
GET    /api/v1/capabilities/violations     # List violations
```

**Total**: 136 endpoints across 24 categories

See [API Documentation](https://opena2a.org/docs/api/rest) for complete reference.

</details>

<details>
<summary><h3>🗄️ Database Schema</h3></summary>

### Core Tables

**agents** — Agent registry
- `id`, `name`, `agent_type`, `owner_id`
- `public_key`, `key_algorithm`, `key_created_at`
- `trust_score`, `status`, `last_seen_at`

**mcp_servers** — MCP server registry
- `id`, `server_id`, `name`, `url`
- `public_key`, `attestation_signature`
- `capabilities`, `status`, `verified_at`

**agent_mcp_connections** — Agent-MCP relationships
- `agent_id`, `mcp_server_id`, `connected_at`
- `detection_method`, `confidence_score`

**verification_events** — Action verification log
- `id`, `agent_id`, `action_type`, `resource_type`
- `approved`, `risk_level`, `trust_score_at_time`

**trust_scores** — Trust score history
- `agent_id`, `score`, `factors`, `calculated_at`

**capabilities** — Agent capabilities
- `agent_id`, `capability_name`, `granted_by`
- `granted_at`, `expires_at`, `metadata`

**security_anomalies** — Behavioral anomaly detection
- `agent_id`, `anomaly_type`, `severity`
- `detected_at`, `resolved_at`, `metadata`

### Capability Management
- **capability_requests** — Pending capability requests
- **capability_violations** — Unauthorized action attempts

### MCP Attestation
- **mcp_attestations** — Cryptographic attestation records
- **mcp_capabilities** — MCP server capabilities

</details>

---

<div align="center">

**Built with ❤️ by the [OpenA2A](https://opena2a.org) team**

⭐ **Star us on GitHub** if AIM helps secure your AI agents!

</div>

---

## 🏷️ Search Topics

<div align="center">

`ai-security` `agent-identity` `ai-agent-management` `mcp-servers` `machine-learning-security` `zero-trust` `authentication` `authorization` `audit-logging` `compliance` `hipaa` `soc2` `gdpr` `langchain` `crewai` `autonomous-agents` `trust-scoring` `threat-detection` `anomaly-detection` `cryptography` `ed25519` `golang` `nextjs` `typescript` `postgresql` `kubernetes` `docker` `cybersecurity` `devops` `mlops` `aiops` `identity-management` `access-control` `rbac` `security-automation` `vulnerability-management` `risk-management` `ai-governance` `llm-security` `prompt-injection` `ai-safety`

</div>
