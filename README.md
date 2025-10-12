# 🛡️ AIM - Agent Identity Management

<div align="center">

**Enterprise-Grade Agent Identity Management**

*Secure your AI agents with 1 line of code*

[![License](https://img.shields.io/badge/license-AGPL--3.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.23+-blue.svg)](https://golang.org)
[![Node Version](https://img.shields.io/badge/node-22+-green.svg)](https://nodejs.org)
[![Python Version](https://img.shields.io/badge/python-3.8+-blue.svg)](https://python.org)
[![Docker](https://img.shields.io/badge/docker-ready-blue.svg)](https://docker.com)

[Documentation](docs/) • [Quick Start](#-quick-start) • [SDK](#-sdk-1-line-setup) • [API Reference](docs/API.md) • [Contributing](#-contributing)

</div>

---

## 🎯 What is AIM?

**AIM (Agent Identity Management)** is the first open-source platform designed specifically for securing AI agent ecosystems. It provides cryptographic identity verification, trust scoring, and compliance monitoring for autonomous AI agents and [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) servers.

### One Line of Code Philosophy

**AIM makes agent security effortless**:

```python
from aim_sdk import secure

# ONE LINE - Agent registered, verified, and secured! 🚀
agent = secure("my-agent")
```

**That's it.** No complex configuration. No key management. No security expertise required.

### Why AIM?

As AI agents become more autonomous and powerful, they need robust identity and security infrastructure:

- ✅ **Ed25519 Cryptographic Signing**: Military-grade authentication on every request
- ✅ **Auto-MCP Detection**: Automatically discovers and registers MCP servers from Claude Desktop config
- ✅ **Trust Scoring**: ML-powered 8-factor risk assessment
- ✅ **Real-time Behavior Analytics**: Monitor agent actions, drift, and anomalies
- ✅ **Framework Integrations**: Works with CrewAI, LangChain out of the box
- ✅ **Complete Audit Trail**: SOC 2, HIPAA, GDPR compliance ready
- ✅ **Enterprise SSO**: Google, Microsoft, Okta with auto-provisioning
- ✅ **Zero Code Changes**: Secure existing agents without refactoring

---

## 🚀 Quick Start

### One-Command Deployment

```bash
# Clone the repository
git clone https://github.com/opena2a/agent-identity-management.git
cd agent-identity-management

# Deploy everything (infrastructure + services)
./deploy.sh

# Or just infrastructure for development
./deploy.sh development
```

**Access your AIM instance:**
- 🌐 Dashboard: http://localhost:3000
- 🔌 Backend API: http://localhost:8080
- 📊 Grafana: http://localhost:3003

### Create Your First Agent (via Dashboard)

1. **Register**: Navigate to http://localhost:3000 and create an account
2. **Create Agent**: Click "Register New Agent" in the dashboard
3. **Download SDK**: Go to agent details → "SDK Setup" tab
4. **Copy & Run**: Copy the 1-line setup code and paste into your agent

---

## 🔥 SDK: 1-Line Setup

### Installation

```bash
# Download SDK from AIM Dashboard
# Navigate to http://localhost:3000/dashboard/sdk
# Click "Download SDK" → Extract and install:

unzip aim-sdk-python.zip
cd aim-sdk-python
pip install -e .
```

### The Magic: One Line

```python
from aim_sdk import secure
import os

# ONE LINE - Enterprise security enabled! 🚀
agent = secure(
    name="my-agent",
    aim_url="https://aim.opena2a.org",
    private_key=os.getenv("AIM_PRIVATE_KEY")  # Get from dashboard
)

# ✨ That's it! Your agent is now secure.

# Automatically enabled:
# ✅ Ed25519 cryptographic signing on every request
# ✅ Auto-MCP detection from Claude Desktop config
# ✅ Real-time trust scoring and behavior analytics
# ✅ Audit logging and compliance reporting
# ✅ Anomaly detection and security alerts
```

### Advanced: Full Client Control

```python
from aim_sdk import AIMClient
import os

# For existing agents or custom configurations
client = AIMClient(
    api_url="https://aim.opena2a.org",
    agent_id="your-agent-id",  # From dashboard
    private_key=os.getenv("AIM_PRIVATE_KEY"),
    auto_detect={
        "enabled": True,
        "config_path": "~/.config/claude/mcp_config.json"
    }
)

# Verify actions before execution
@client.perform_action("read_database", resource="users_table")
def get_user_data(user_id):
    return database.query("SELECT * FROM users WHERE id = ?", user_id)

# Auto-detect MCPs
detection = client.detect_mcps()
print(f"Detected {len(detection['mcps'])} MCP servers")
```

---

## 🔌 Framework Integrations

### CrewAI Integration

```python
from crewai import Agent, Task, Crew
from aim_sdk import secure
from aim_sdk.integrations.crewai import AIMCrewWrapper

# Register with AIM
aim_agent = secure("my-crew", "https://aim.opena2a.org")

# Wrap your crew
my_crew = Crew(
    agents=[researcher, writer],
    tasks=[research_task, write_task]
)

verified_crew = AIMCrewWrapper(
    crew=my_crew,
    aim_agent=aim_agent,
    risk_level="medium"
)

# All agent actions automatically verified and logged
result = verified_crew.kickoff(inputs={"topic": "AI safety"})
```

### LangChain Integration

```python
from langchain.agents import AgentExecutor, create_openai_functions_agent
from aim_sdk import secure
from aim_sdk.integrations.langchain import AIMAgentExecutor

# Register with AIM
aim_agent = secure("langchain-agent", "https://aim.opena2a.org")

# Create LangChain agent
agent = create_openai_functions_agent(llm, tools, prompt)
executor = AgentExecutor(agent=agent, tools=tools)

# Wrap with AIM verification
verified_executor = AIMAgentExecutor(
    agent_executor=executor,
    aim_agent=aim_agent
)

# Every tool use is verified before execution
result = verified_executor.run("What's the weather in San Francisco?")
```

### MCP Server Registration

```python
from aim_sdk import AIMClient
from aim_sdk.integrations.mcp import register_mcp_server

client = AIMClient(
    api_url="https://aim.opena2a.org",
    agent_id="your-agent-id",
    private_key=os.getenv("AIM_PRIVATE_KEY")
)

# Register MCP server with cryptographic verification
mcp_server = register_mcp_server(
    client=client,
    name="filesystem-mcp",
    endpoint="http://localhost:3000",
    public_key="base64-encoded-public-key"
)

# Verify MCP server identity
verification = mcp_server.verify_identity()
print(f"MCP Trust Score: {verification.trust_score}")
```

---

## 📦 Features

### 🔐 Core Identity Management

- **AI Agent Registration**: Ed25519-based cryptographic identity
- **MCP Server Verification**: Public key verification for Model Context Protocol servers
- **API Key Management**: SHA-256 hashed keys with expiration and usage tracking
- **Trust Scoring**: 8-factor ML-powered risk assessment
- **Drift Detection**: Automatic detection of behavioral anomalies
- **Challenge-Response Auth**: Military-grade cryptographic verification

### 🏢 Enterprise Features

- **Multi-Tenancy**: Organization-level isolation with RBAC
- **SSO Integration**: Google, Microsoft, Okta with auto-provisioning
- **Complete Audit Trail**: Every action logged with immutable history
- **Proactive Security Alerts**: Certificate expiry, anomaly detection, threat warnings
- **Compliance Reporting**: SOC 2, HIPAA, GDPR ready with exportable reports
- **Webhook Integration**: Real-time event notifications to external systems

### 🛠️ Developer Experience

- **1-Line SDK Setup**: `register_agent()` - that's it!
- **Auto-MCP Detection**: Discovers MCP servers from Claude Desktop config
- **Framework Integrations**: CrewAI, LangChain (Microsoft Copilot coming soon)
- **Zero Code Changes**: Secure existing agents without refactoring
- **REST API**: 60+ endpoints with full OpenAPI documentation
- **Docker Ready**: Production-grade containers and Docker Compose setup
- **Kubernetes Support**: Scalable deployment manifests included

---

## 🏗️ Architecture

### Technology Stack

#### Backend
- **Language**: Go 1.23+ with Fiber v3 (high performance web framework)
- **Database**: PostgreSQL 16 with TimescaleDB (time-series data)
- **Cache**: Redis 7 (session management & caching)
- **Search**: Elasticsearch 8 (audit log search)
- **Queue**: NATS JetStream (event streaming)
- **Storage**: MinIO (S3-compatible object storage)

#### Frontend
- **Framework**: Next.js 15 with App Router (React 19)
- **Language**: TypeScript 5.5+
- **Styling**: Tailwind CSS v4 + Shadcn/ui
- **State**: Zustand (lightweight state management)
- **Charts**: Recharts (data visualization)
- **Icons**: Lucide React

#### SDK
- **Python SDK**: Full-featured with CrewAI, LangChain, MCP integrations
- **Go SDK**: Coming soon
- **JavaScript SDK**: Coming soon

#### Infrastructure
- **Containers**: Docker + Docker Compose
- **Orchestration**: Kubernetes (production)
- **Monitoring**: Prometheus + Grafana
- **Logging**: Loki + Promtail (centralized logging)
- **CI/CD**: GitHub Actions

### System Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    AIM Platform                          │
├─────────────────────────────────────────────────────────┤
│                                                          │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐ │
│  │   Dashboard  │  │   Backend    │  │   Python     │ │
│  │   Next.js    │◄─┤   Go/Fiber   │◄─┤   SDK        │ │
│  └──────────────┘  └──────────────┘  └──────────────┘ │
│         │                  │                 │          │
│         ▼                  ▼                 ▼          │
│  ┌──────────────────────────────────────────────────┐ │
│  │           Core Services Layer                     │ │
│  ├──────────────────────────────────────────────────┤ │
│  │ • Authentication (JWT, OAuth, SSO)               │ │
│  │ • Agent Management (CRUD, Verification)          │ │
│  │ • Trust Scoring (8-factor ML algorithm)          │ │
│  │ • MCP Detection (Auto-discovery)                 │ │
│  │ • Audit Logging (Immutable trail)                │ │
│  │ • Alert Service (Proactive notifications)        │ │
│  │ • Compliance Reports (SOC 2, HIPAA, GDPR)        │ │
│  │ • Webhook Service (Event notifications)          │ │
│  └──────────────────────────────────────────────────┘ │
│         │                                              │
│         ▼                                              │
│  ┌──────────────────────────────────────────────────┐ │
│  │           Data & Infrastructure Layer             │ │
│  ├──────────────────────────────────────────────────┤ │
│  │ PostgreSQL 16  │ Redis 7      │ Elasticsearch 8  │ │
│  │ TimescaleDB    │ NATS         │ MinIO            │ │
│  │ Prometheus     │ Grafana      │ Loki             │ │
│  └──────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────┘
```

---

## 📊 API Overview

### REST API - 60+ Endpoints

**Authentication** (`/api/v1/auth`)
- `POST /login` - User login with JWT
- `POST /register` - User registration
- `GET /sso/{provider}` - SSO authentication (Google, Microsoft, Okta)
- `POST /refresh` - Refresh access token
- `POST /logout` - Logout and invalidate session

**Agents** (`/api/v1/agents`)
- `GET /agents` - List all agents
- `POST /agents` - Register new agent
- `GET /agents/{id}` - Get agent details
- `PUT /agents/{id}` - Update agent
- `DELETE /agents/{id}` - Delete agent
- `POST /agents/{id}/verify` - Cryptographic verification
- `GET /agents/{id}/trust-score` - Get trust score
- `GET /agents/{id}/capabilities` - Get detected capabilities

**MCP Servers** (`/api/v1/mcp-servers`)
- `GET /mcp-servers` - List all MCP servers
- `POST /mcp-servers` - Register MCP server
- `GET /mcp-servers/{id}` - Get MCP server details
- `POST /mcp-servers/{id}/verify` - Verify MCP server identity
- `GET /mcp-servers/{id}/trust-score` - Get MCP trust score

**API Keys** (`/api/v1/keys`)
- `GET /keys` - List API keys
- `POST /keys` - Generate new API key
- `DELETE /keys/{id}` - Revoke API key
- `GET /keys/{id}/usage` - Get API key usage stats

**Trust Scores** (`/api/v1/trust-scores`)
- `GET /trust-scores` - Get organization trust scores
- `GET /trust-scores/{agent_id}` - Get agent trust score
- `GET /trust-scores/{agent_id}/history` - Trust score history
- `POST /trust-scores/{agent_id}/recalculate` - Recalculate trust score

**Audit Logs** (`/api/v1/audit-logs`)
- `GET /audit-logs` - List audit logs (filterable)
- `GET /audit-logs/{id}` - Get specific log entry
- `POST /audit-logs/export` - Export audit logs (CSV, JSON)
- `GET /audit-logs/stats` - Get audit statistics

**Alerts** (`/api/v1/alerts`)
- `GET /alerts` - List security alerts
- `GET /alerts/{id}` - Get alert details
- `POST /alerts/{id}/acknowledge` - Acknowledge alert
- `DELETE /alerts/{id}` - Dismiss alert

**Compliance** (`/api/v1/compliance`)
- `GET /compliance/reports` - List compliance reports
- `POST /compliance/reports/generate` - Generate new report
- `GET /compliance/reports/{id}` - Get report details
- `POST /compliance/reports/{id}/export` - Export report (PDF, CSV)

### API Documentation

- **Swagger UI**: http://localhost:8080/swagger
- **OpenAPI Spec**: http://localhost:8080/openapi.json
- **Postman Collection**: Available in `/docs/postman/`

### Authentication Example

```bash
# Register user
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "SecurePass123!",
    "firstName": "John",
    "lastName": "Doe"
  }'

# Login
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "SecurePass123!"
  }'

# Response: { "token": "eyJhbGc...", "user": {...} }

# Use token for authenticated requests
curl http://localhost:8080/api/v1/agents \
  -H "Authorization: Bearer eyJhbGc..."
```

---

## 🔒 Security

### Cryptographic Verification Flow

AIM uses **Ed25519 digital signatures** for agent authentication:

1. **Agent Registration**:
   - Generate Ed25519 keypair (public + private)
   - Store public key in AIM platform
   - Private key stored securely in agent's environment

2. **Challenge-Response Authentication**:
   - Agent requests verification
   - Server sends random challenge (256-bit nonce)
   - Agent signs challenge with private key
   - Server verifies signature with public key
   - JWT token issued on successful verification

3. **Every Request Signed**:
   - SDK automatically signs all API requests
   - Server validates signature before processing
   - Prevents impersonation and replay attacks

### Trust Scoring Algorithm

**8-factor ML-powered assessment**:

1. **Verification Status** (25%) - Cryptographic verification success rate
2. **Uptime & Availability** (15%) - Agent reliability and responsiveness
3. **Action Success Rate** (15%) - Percentage of successful operations
4. **Security Alerts** (15%) - Number and severity of security alerts
5. **Compliance Score** (10%) - Adherence to security policies
6. **Age & History** (10%) - Time since registration, historical behavior
7. **Drift Detection** (5%) - Behavioral consistency over time
8. **User Feedback** (5%) - Manual reviews and ratings

**Trust Score Range**: 0.00 (untrusted) → 1.00 (fully trusted)

### Security Best Practices

- ✅ **Never commit secrets**: Use environment variables for all credentials
- ✅ **Rotate keys regularly**: API keys expire, private keys rotated quarterly
- ✅ **Use HTTPS/TLS**: Always use encrypted connections in production
- ✅ **Enable rate limiting**: 100 requests/minute default (configurable)
- ✅ **Monitor audit logs**: Review logs daily for suspicious activity
- ✅ **Update dependencies**: Keep all packages up-to-date
- ✅ **Run security scans**: Use Trivy, Snyk, or similar tools
- ✅ **Least privilege**: Grant minimum required permissions

---

## 🧪 Testing

### Backend Tests (Go)

```bash
cd apps/backend

# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run integration tests
go test ./tests/integration/...

# Run specific test
go test -v -run TestAgentService_CreateAgent ./internal/application/
```

### Frontend Tests (Next.js)

```bash
cd apps/web

# Run all tests
npm test

# Run with coverage
npm test -- --coverage

# Run in watch mode
npm test -- --watch

# E2E tests with Playwright
npm run test:e2e
```

### SDK Tests (Python)

```bash
cd sdks/python

# Run all tests
python3 -m pytest tests/ -v

# Run with coverage
python3 -m pytest --cov=aim_sdk tests/

# Run integration tests
python3 -m pytest tests/integration/

# Test framework integrations
python3 test_crewai_integration.py
python3 test_langchain_integration.py
```

---

## 🚦 Getting Started

### Prerequisites

- **Docker** 20.10+ and **Docker Compose** 2.0+
- **Go** 1.23+ (for backend development)
- **Node.js** 22+ (for frontend development)
- **Python** 3.8+ (for SDK usage)

### Local Development Setup

```bash
# 1. Clone repository
git clone https://github.com/opena2a/agent-identity-management.git
cd agent-identity-management

# 2. Deploy infrastructure (PostgreSQL, Redis, etc.)
./deploy.sh development

# 3. Start backend (new terminal)
cd apps/backend
cp .env.example .env  # Configure environment variables
go run cmd/server/main.go

# 4. Start frontend (new terminal)
cd apps/web
cp .env.local.example .env.local  # Configure environment variables
npm install
npm run dev

# 5. Access application
open http://localhost:3000
```

### Production Deployment

#### Docker Compose (Recommended for Single Server)

```bash
# Deploy with Docker Compose
./deploy.sh production

# Services will be available at:
# - Frontend: http://localhost:3000
# - Backend: http://localhost:8080
# - Grafana: http://localhost:3003
```

#### Kubernetes (Recommended for Scale)

```bash
# Deploy to Kubernetes cluster
kubectl apply -f infrastructure/k8s/namespace.yaml
kubectl apply -f infrastructure/k8s/configmap.yaml
kubectl apply -f infrastructure/k8s/secrets.yaml
kubectl apply -f infrastructure/k8s/

# Check deployment status
kubectl get pods -n aim-platform
kubectl get services -n aim-platform
```

#### Environment Variables

**Backend** (`.env`):
```bash
# Database
DATABASE_URL=postgresql://aim:password@localhost:5432/aim_db

# Redis
REDIS_URL=redis://localhost:6379

# JWT
JWT_SECRET=your-super-secret-jwt-key-change-in-production

# OAuth (optional)
GOOGLE_CLIENT_ID=your-google-client-id
GOOGLE_CLIENT_SECRET=your-google-client-secret

# API
API_PORT=8080
API_HOST=0.0.0.0
```

**Frontend** (`.env.local`):
```bash
# Backend API URL
NEXT_PUBLIC_API_URL=http://localhost:8080

# OAuth (optional)
NEXT_PUBLIC_GOOGLE_CLIENT_ID=your-google-client-id
```

---

## 🤝 Contributing

We welcome contributions! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

### Development Workflow

1. **Fork** the repository
2. **Create** a feature branch (`git checkout -b feature/amazing-feature`)
3. **Commit** your changes (`git commit -m 'feat: add amazing feature'`)
4. **Test** your changes (`go test ./...`, `npm test`, `pytest`)
5. **Push** to the branch (`git push origin feature/amazing-feature`)
6. **Open** a Pull Request

### Code Standards

- **Go**: Follow [Effective Go](https://golang.org/doc/effective_go) guidelines
- **TypeScript**: Follow [TypeScript Best Practices](https://typescript-lang.org/docs/)
- **Python**: Follow [PEP 8](https://pep8.org) style guide
- **Commits**: Use [Conventional Commits](https://conventionalcommits.org)
- **Documentation**: Update docs with code changes
- **Tests**: Add tests for all new features

---

## 📈 Roadmap

### v1.0.0 (Current - Released) ✅
- [x] Core agent identity management
- [x] Ed25519 cryptographic verification
- [x] MCP server registration & verification
- [x] Trust scoring system (8-factor algorithm)
- [x] Python SDK with 1-line setup
- [x] CrewAI integration
- [x] LangChain integration
- [x] MCP auto-detection
- [x] Complete audit logging
- [x] Security alerts & notifications
- [x] Docker Compose deployment
- [x] Next.js dashboard (AIVF-inspired design)
- [x] 60+ REST API endpoints
- [x] Enterprise SSO (Google, Microsoft, Okta)

### v1.1.0 (Q1 2025)
- [ ] **Action Verification**: Capture agent action context with behavioral baselines
- [ ] **Advanced RBAC**: Custom roles and fine-grained permissions
- [ ] **GraphQL API**: Complex queries and real-time subscriptions
- [ ] **CLI Tool**: Command-line interface for automation
- [ ] **Go SDK**: Full-featured Go client library
- [ ] **JavaScript SDK**: Browser and Node.js support
- [ ] **Advanced Analytics**: Agent behavior insights and trends
- [ ] **Webhook Management UI**: Configure webhooks from dashboard
- [ ] **Multi-region Deployment**: Global distribution support

### v1.2.0 (Q2 2025)
- [ ] **SAML 2.0 SSO**: Enterprise SAML authentication
- [ ] **Advanced Compliance**: Automated SOC 2, HIPAA, GDPR reporting
- [ ] **Behavioral Anomaly Detection**: ML-powered threat detection
- [ ] **Custom Trust Models**: Organization-specific trust algorithms
- [ ] **Agent-to-Agent Communication**: Secure inter-agent messaging
- [ ] **Microsoft Copilot Integration**: Copilot Studio support
- [ ] **SOC 2 Type II Certification**: Security compliance certification

### v2.0.0 (Q3 2025)
- [ ] **Federated Identity**: Cross-platform identity federation
- [ ] **Zero-Knowledge Proofs**: Privacy-preserving verification
- [ ] **Blockchain Anchoring**: Immutable audit trail on blockchain
- [ ] **Advanced Threat Detection**: Real-time attack prevention
- [ ] **Auto-Remediation**: Automated security responses
- [ ] **HIPAA Certification**: Healthcare compliance certification

---

## 📄 License

This project is licensed under the **GNU Affero General Public License v3.0 (AGPL-3.0)**.

**What this means**:
- ✅ You can use AIM for free, forever (even commercially)
- ✅ You can modify and distribute AIM
- ✅ You must open-source any modifications (if you provide AIM as a service)
- ✅ Perfect for self-hosted deployments

**Commercial Licensing**: Contact us for commercial licenses without AGPL requirements.

See the [LICENSE](LICENSE) file for full details.

---

## 🙏 Acknowledgments

Built with ❤️ by the [OpenA2A](https://opena2a.org) community.

**Special thanks to:**
- [Model Context Protocol](https://modelcontextprotocol.io) - MCP specification
- [Go](https://golang.org) - Backend language
- [Next.js](https://nextjs.org) - Frontend framework
- [CrewAI](https://crewai.com) - Multi-agent orchestration
- [LangChain](https://langchain.com) - Agent framework
- The open-source community

---

## 📞 Support & Community

- **Documentation**: https://docs.opena2a.org/aim
- **Discord**: https://discord.gg/opena2a
- **GitHub Issues**: https://github.com/opena2a/agent-identity-management/issues
- **Email**: support@opena2a.org
- **Twitter**: [@opena2a](https://twitter.com/opena2a)

---

## 🌟 Star History

If you find AIM useful, please give it a ⭐ on GitHub!

[![Star History Chart](https://api.star-history.com/svg?repos=opena2a/agent-identity-management&type=Date)](https://star-history.com/#opena2a/agent-identity-management&Date)

---

<div align="center">

**🛡️ Enterprise-Grade Agent Security 🛡️**

*Secure your agents with 1 line of code*

[Get Started](#-quick-start) • [Documentation](docs/) • [Community](https://discord.gg/opena2a)

</div>
