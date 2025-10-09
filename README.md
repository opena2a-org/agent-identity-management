# 🛡️ AIM - Agent Identity Management

<div align="center">

**Enterprise-grade identity verification and security platform for AI agents and MCP servers**

[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.23+-blue.svg)](https://golang.org)
[![Node Version](https://img.shields.io/badge/node-22+-green.svg)](https://nodejs.org)
[![Python Version](https://img.shields.io/badge/python-3.8+-blue.svg)](https://python.org)
[![Docker](https://img.shields.io/badge/docker-ready-blue.svg)](https://docker.com)

[Documentation](docs/) • [Quick Start](#-quick-start) • [SDK](#-python-sdk) • [API Reference](docs/API.md) • [Contributing](#-contributing)

</div>

---

## 🎯 What is AIM?

**AIM (Agent Identity Management)** is the first open-source platform designed specifically for securing AI agent ecosystems. It provides cryptographic identity verification, trust scoring, and compliance monitoring for autonomous AI agents and [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) servers.

Think of AIM as **"Stripe for AI Agent Identity"** - one line of code to register, verify, and secure your agents.

### Why AIM?

As AI agents become more autonomous and powerful, they need robust identity and security infrastructure:

- ✅ **Cryptographic Verification**: Ed25519-based challenge-response authentication
- ✅ **Trust Scoring**: ML-powered assessment of agent trustworthiness
- ✅ **MCP Server Verification**: Built-in support for Model Context Protocol servers
- ✅ **Audit Logging**: Complete history of all agent actions for compliance
- ✅ **Enterprise SSO**: Google, Microsoft, Okta integration with auto-provisioning
- ✅ **Zero-Trust Security**: Verify every action before execution
- ✅ **Compliance Ready**: SOC 2, HIPAA, GDPR audit trails

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

**That's it!** AIM is now running at:
- 🌐 Frontend: http://localhost:3000
- 🔌 Backend API: http://localhost:8080
- 📊 Grafana Dashboard: http://localhost:3003

### One-Line Agent Registration

```python
from aim_sdk import register_agent

# Register agent with automatic key generation
agent = register_agent("my-agent", "http://localhost:8080")

# Use decorator-based verification
@agent.perform_action("read_database", resource="users_table")
def get_user_data(user_id):
    return database.query(f"SELECT * FROM users WHERE id = {user_id}")
```

---

## 📦 Features

### 🔐 Core Identity Management

- **AI Agent Registration**: Cryptographic identity for autonomous agents
- **MCP Server Verification**: Public key verification for MCP servers
- **API Key Management**: SHA-256 hashed keys with expiration
- **Trust Scoring**: 8-factor ML-powered risk assessment
- **Drift Detection**: Automatic detection of agent behavior changes

### 🏢 Enterprise Features

- **Multi-Tenancy**: Organization-level isolation with RBAC
- **SSO Integration**: Google, Microsoft, Okta with auto-provisioning
- **Audit Logging**: Every action logged with immutable trail
- **Proactive Alerts**: Certificate expiry, anomaly detection, security threats
- **Compliance Reporting**: SOC 2, HIPAA, GDPR ready
- **Webhook Integration**: Real-time event notifications

### 🛠️ Developer Experience

- **One-Line SDK**: Python SDK with automatic key management
- **Framework Integrations**: CrewAI, LangChain, Microsoft Copilot
- **REST API**: 60+ endpoints with OpenAPI documentation
- **GraphQL API**: Complex queries and subscriptions
- **CLI Tool**: Automation and CI/CD integration
- **Docker Ready**: Production-ready containers

---

## 📚 Documentation

### Getting Started

- [Installation & Deployment](docs/DEPLOYMENT.md) - Complete deployment guide
- [Architecture Overview](docs/ARCHITECTURE.md) - System design and components
- [API Documentation](docs/API.md) - REST API reference
- [Security Guide](docs/SECURITY.md) - Security best practices

### Python SDK

- [SDK Quick Start](sdks/python/README.md) - One-line agent registration
- [CrewAI Integration](sdks/python/CREWAI_INTEGRATION.md) - Multi-agent crews
- [LangChain Integration](sdks/python/LANGCHAIN_INTEGRATION.md) - LangChain agents
- [Microsoft Copilot](sdks/python/MICROSOFT_COPILOT_INTEGRATION.md) - Copilot Studio
- [MCP Integration](sdks/python/MCP_INTEGRATION.md) - Model Context Protocol

### Advanced Topics

- [Trust Scoring Algorithm](docs/TRUST_SCORING.md) - ML-powered risk assessment
- [Compliance & Audit](docs/COMPLIANCE.md) - SOC 2, HIPAA, GDPR
- [Performance Tuning](docs/PERFORMANCE.md) - Optimization guide
- [Production Deployment](docs/PRODUCTION.md) - Kubernetes, scaling, HA

---

## 🐍 Python SDK

### Installation

```bash
cd sdks/python
pip install -r requirements.txt
```

### Basic Usage

```python
from aim_sdk import register_agent

# Option 1: Auto-register with one line
agent = register_agent(
    name="my-agent",
    aim_url="http://localhost:8080",
    display_name="My Awesome Agent",
    description="Production agent for user management"
)

# Option 2: Load existing credentials
from aim_sdk import AIMClient
agent = AIMClient.load_from_credentials("my-agent")

# Verify actions before execution
@agent.perform_action("modify_user", resource="user:12345")
def update_user_email(user_id, new_email):
    return database.execute(
        "UPDATE users SET email = ? WHERE id = ?",
        new_email, user_id
    )

# Action is verified, logged, and executed
update_user_email("12345", "newemail@example.com")
```

### Framework Integrations

#### CrewAI

```python
from crewai import Agent, Task, Crew
from aim_sdk.integrations.crewai import AIMCrewWrapper

# Wrap entire crew with automatic verification
verified_crew = AIMCrewWrapper(
    crew=my_crew,
    aim_agent=agent,
    risk_level="medium"
)

# All executions automatically verified and logged
result = verified_crew.kickoff(inputs={"topic": "AI safety"})
```

#### LangChain

```python
from langchain.agents import AgentExecutor
from aim_sdk.integrations.langchain import AIMAgentExecutor

# Wrap LangChain agent with verification
verified_agent = AIMAgentExecutor(
    agent=my_langchain_agent,
    tools=tools,
    aim_agent=agent
)

# Every tool use verified before execution
result = verified_agent.run("What's the weather in SF?")
```

#### MCP Servers

```python
from aim_sdk import AIMClient
from aim_sdk.integrations.mcp import register_mcp_server

# Register MCP server with cryptographic verification
mcp_server = register_mcp_server(
    name="my-mcp-server",
    url="http://localhost:8080",
    endpoint="http://mcp-server:3000"
)

# Verify MCP server identity
verification = mcp_server.verify_identity()
print(f"Trust Score: {verification.trust_score}")
```

---

## 🏗️ Architecture

### Technology Stack

#### Backend
- **Language**: Go 1.23+ with Fiber v3
- **Database**: PostgreSQL 16 + TimescaleDB
- **Cache**: Redis 7
- **Search**: Elasticsearch 8
- **Queue**: NATS JetStream
- **Storage**: MinIO (S3-compatible)

#### Frontend
- **Framework**: Next.js 15 (App Router)
- **Language**: TypeScript 5.5+
- **Styling**: Tailwind CSS v4 + Shadcn/ui
- **State**: Zustand
- **Charts**: Recharts

#### Infrastructure
- **Containers**: Docker + Docker Compose
- **Orchestration**: Kubernetes (production)
- **Monitoring**: Prometheus + Grafana
- **Logging**: Loki + Promtail
- **CI/CD**: GitHub Actions

### System Components

```
┌─────────────────────────────────────────────────────────┐
│                    AIM Platform                          │
├─────────────────────────────────────────────────────────┤
│                                                          │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐ │
│  │   Frontend   │  │   Backend    │  │   SDK        │ │
│  │   Next.js    │◄─┤   Go/Fiber   │◄─┤   Python     │ │
│  └──────────────┘  └──────────────┘  └──────────────┘ │
│         │                  │                            │
│         ▼                  ▼                            │
│  ┌──────────────────────────────────────────────────┐ │
│  │           Core Services                           │ │
│  ├──────────────────────────────────────────────────┤ │
│  │ • Authentication  • Agent Management             │ │
│  │ • Trust Scoring   • API Key Management           │ │
│  │ • Audit Logging   • Alert Service                │ │
│  │ • Compliance      • Webhook Service              │ │
│  └──────────────────────────────────────────────────┘ │
│         │                                              │
│         ▼                                              │
│  ┌──────────────────────────────────────────────────┐ │
│  │           Infrastructure                          │ │
│  ├──────────────────────────────────────────────────┤ │
│  │ PostgreSQL │ Redis │ Elasticsearch │ MinIO       │ │
│  │ NATS       │ Prometheus │ Grafana │ Loki         │ │
│  └──────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────┘
```

---

## 🚦 Getting Started

### Prerequisites

- **Docker** 20.10+ and **Docker Compose** 2.0+
- **Go** 1.23+ (for backend development)
- **Node.js** 22+ (for frontend development)
- **Python** 3.8+ (for SDK)

### Development Setup

```bash
# 1. Clone repository
git clone https://github.com/opena2a/agent-identity-management.git
cd agent-identity-management

# 2. Deploy infrastructure
./deploy.sh development

# 3. Start backend (in new terminal)
cd apps/backend
go run cmd/server/main.go

# 4. Start frontend (in new terminal)
cd apps/web
npm install
npm run dev

# 5. Access application
open http://localhost:3000
```

### Production Deployment

```bash
# Deploy with Docker Compose
./deploy.sh production

# Or deploy to Kubernetes
kubectl apply -f infrastructure/k8s/

# Or use Terraform
cd infrastructure/terraform
terraform init
terraform apply
```

---

## 📊 API Overview

### REST API

**60+ endpoints** covering:

- **Authentication**: `/auth/login`, `/auth/register`, `/auth/sso/{provider}`
- **Agents**: `/api/agents`, `/api/agents/{id}`, `/api/agents/{id}/verify`
- **MCP Servers**: `/api/mcp-servers`, `/api/mcp-servers/{id}/verify`
- **API Keys**: `/api/keys`, `/api/keys/{id}/revoke`
- **Trust Scores**: `/api/trust-scores`, `/api/trust-scores/history`
- **Audit Logs**: `/api/audit-logs`, `/api/audit-logs/export`
- **Alerts**: `/api/alerts`, `/api/alerts/{id}/acknowledge`
- **Compliance**: `/api/compliance/reports`, `/api/compliance/export`

### Authentication

```bash
# Register user
curl -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"secure123"}'

# Login
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"secure123"}'

# Use token
curl http://localhost:8080/api/agents \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

### API Documentation

- **Swagger UI**: http://localhost:8080/swagger
- **OpenAPI Spec**: http://localhost:8080/openapi.json
- **Postman Collection**: [docs/postman/AIM.postman_collection.json](docs/postman/)

---

## 🔒 Security

### Cryptographic Verification

AIM uses **Ed25519 digital signatures** for agent verification:

1. **Agent Registration**: Generate Ed25519 keypair
2. **Challenge Request**: Server sends random challenge
3. **Sign Challenge**: Agent signs with private key
4. **Verify Response**: Server verifies with public key
5. **Grant Access**: Issue JWT token on success

### Trust Scoring

**8-factor ML algorithm** assessing:

1. **Verification Status** (25%) - Cryptographic verification success
2. **Uptime** (15%) - Agent availability and reliability
3. **Action Success Rate** (15%) - Percentage of successful operations
4. **Security Alerts** (15%) - Number and severity of alerts
5. **Compliance Score** (10%) - Adherence to policies
6. **Age & History** (10%) - Time since registration
7. **Drift Detection** (5%) - Behavioral consistency
8. **User Feedback** (5%) - Manual reviews and ratings

### Security Best Practices

- ✅ **Never commit** `.env` files or secrets
- ✅ **Rotate secrets** regularly (API keys, JWT secrets)
- ✅ **Use HTTPS/TLS** in production
- ✅ **Enable rate limiting** (100 requests/minute default)
- ✅ **Monitor audit logs** for suspicious activity
- ✅ **Update dependencies** regularly
- ✅ **Run security scans** with `trivy` or `snyk`

---

## 🧪 Testing

### Backend Tests

```bash
cd apps/backend

# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run integration tests
go test ./tests/integration/...

# Run specific test
go test -v ./internal/application/agent_service_test.go
```

### Frontend Tests

```bash
cd apps/web

# Run all tests
npm test

# Run with coverage
npm test -- --coverage

# Run in watch mode
npm test -- --watch
```

### SDK Tests

```bash
cd sdks/python

# Run all tests
python3 -m pytest tests/

# Run integration tests
python3 test_crewai_integration.py
python3 test_langchain_integration.py

# Run with coverage
python3 -m pytest --cov=aim_sdk tests/
```

---

## 🤝 Contributing

We welcome contributions! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

### Development Workflow

1. **Fork** the repository
2. **Create** a feature branch (`git checkout -b feature/amazing-feature`)
3. **Commit** your changes (`git commit -m 'Add amazing feature'`)
4. **Push** to the branch (`git push origin feature/amazing-feature`)
5. **Open** a Pull Request

### Code Standards

- **Go**: Follow [Effective Go](https://golang.org/doc/effective_go)
- **TypeScript**: Follow [TypeScript Guidelines](https://typescript-lang.org/docs/)
- **Python**: Follow [PEP 8](https://pep8.org)
- **Documentation**: Update docs with code changes
- **Tests**: Add tests for new features

---

## 📈 Roadmap

### v1.0.0 (Current) ✅
- [x] Core identity management
- [x] MCP server verification
- [x] Trust scoring system
- [x] Python SDK
- [x] CrewAI/LangChain integrations
- [x] Basic audit logging
- [x] Docker deployment

### v1.1.0 (Q1 2025)
- [ ] Advanced RBAC with custom roles
- [ ] GraphQL API
- [ ] CLI tool
- [ ] Advanced analytics dashboard
- [ ] Webhook management UI
- [ ] Multi-region deployment

### v1.2.0 (Q2 2025)
- [ ] SAML 2.0 SSO
- [ ] Advanced compliance reporting
- [ ] Behavioral anomaly detection
- [ ] Custom trust scoring models
- [ ] Agent-to-agent communication
- [ ] SOC 2 Type II certification

### v2.0.0 (Q3 2025)
- [ ] Federated identity
- [ ] Zero-knowledge proofs
- [ ] Blockchain anchoring
- [ ] Advanced threat detection
- [ ] Auto-remediation
- [ ] HIPAA certification

---

## 📄 License

This project is licensed under the **Apache License 2.0** - see the [LICENSE](LICENSE) file for details.

---

## 🙏 Acknowledgments

Built with ❤️ by the OpenA2A community.

**Special thanks to:**
- [Anthropic](https://anthropic.com) - Claude 4.5 for autonomous development
- [Model Context Protocol](https://modelcontextprotocol.io) - MCP specification
- [Go](https://golang.org) - Backend language
- [Next.js](https://nextjs.org) - Frontend framework
- The open-source community

---

## 📞 Support & Community

- **Documentation**: https://docs.opena2a.org
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

**🛡️ AIM - Secure the Agent-to-Agent Future 🛡️**

Made with 🤖 by AI, for AI

[Get Started](#-quick-start) • [Documentation](docs/) • [Community](https://discord.gg/opena2a)

</div>
