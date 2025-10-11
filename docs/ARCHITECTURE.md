# AIM System Architecture

**Version**: 1.0.0
**Last Updated**: October 10, 2025
**Status**: Production Ready

---

## 📋 Table of Contents

1. [Overview](#overview)
2. [High-Level Architecture](#high-level-architecture)
3. [Core Components](#core-components)
4. [Technology Stack](#technology-stack)
5. [Data Flow](#data-flow)
6. [Security Architecture](#security-architecture)
7. [Deployment Architecture](#deployment-architecture)
8. [Integration Points](#integration-points)

---

## Overview

AIM (Agent Identity Management) is built on a modern, scalable microservices architecture designed for enterprise-grade AI agent identity verification and security monitoring. The system is designed around three core principles:

1. **Security First**: Cryptographic verification, zero-trust architecture, and comprehensive audit logging
2. **Developer Experience**: One-line SDK integration with automatic configuration
3. **Enterprise Ready**: Multi-tenancy, RBAC, SSO, and compliance reporting

---

## High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Client Layer                              │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐         │
│  │   Web UI     │  │   Python     │  │   Go/JS      │         │
│  │   Next.js    │  │   SDK        │  │   SDKs       │         │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘         │
│         │                  │                  │                  │
└─────────┼──────────────────┼──────────────────┼──────────────────┘
          │                  │                  │
          └──────────────────┴──────────────────┘
                             │
          ┌──────────────────▼──────────────────┐
          │      API Gateway (Nginx)             │
          │    Load Balancer + Rate Limiting     │
          └──────────────────┬──────────────────┘
                             │
          ┌──────────────────▼──────────────────┐
          │         Backend Services             │
          │         Go + Fiber v3                │
          ├──────────────────────────────────────┤
          │                                       │
          │  ┌─────────────────────────────┐    │
          │  │   Authentication Service    │    │
          │  │   - JWT tokens              │    │
          │  │   - OAuth/OIDC              │    │
          │  │   - SSO providers           │    │
          │  └─────────────────────────────┘    │
          │                                       │
          │  ┌─────────────────────────────┐    │
          │  │   Agent Management Service  │    │
          │  │   - Registration            │    │
          │  │   - Verification            │    │
          │  │   - Trust scoring           │    │
          │  └─────────────────────────────┘    │
          │                                       │
          │  ┌─────────────────────────────┐    │
          │  │   MCP Server Service        │    │
          │  │   - Server registration     │    │
          │  │   - Capability detection    │    │
          │  │   - Cryptographic verify    │    │
          │  └─────────────────────────────┘    │
          │                                       │
          │  ┌─────────────────────────────┐    │
          │  │   Security Service          │    │
          │  │   - Threat detection        │    │
          │  │   - Anomaly monitoring      │    │
          │  │   - Compliance checks       │    │
          │  └─────────────────────────────┘    │
          │                                       │
          │  ┌─────────────────────────────┐    │
          │  │   Audit Service             │    │
          │  │   - Activity logging        │    │
          │  │   - Compliance reporting    │    │
          │  │   - Export capabilities     │    │
          │  └─────────────────────────────┘    │
          │                                       │
          └───────────────┬───────────────────────┘
                          │
          ┌───────────────▼───────────────────┐
          │      Data Layer                    │
          ├────────────────────────────────────┤
          │                                     │
          │  ┌──────────┐  ┌──────────┐       │
          │  │PostgreSQL│  │  Redis   │       │
          │  │ Primary  │  │  Cache   │       │
          │  └──────────┘  └──────────┘       │
          │                                     │
          └─────────────────────────────────────┘
```

---

## Core Components

### 1. Backend Services (Go + Fiber)

#### Authentication Service
**Responsibility**: User authentication, authorization, and session management

**Key Features**:
- JWT token generation and validation
- OAuth 2.0 / OIDC integration (Google, Microsoft, Okta)
- API key authentication with SHA-256 hashing
- Role-based access control (RBAC)
- Multi-factor authentication support

**Endpoints**:
- `POST /api/v1/auth/register` - User registration
- `POST /api/v1/auth/login` - User login
- `POST /api/v1/auth/refresh` - Token refresh
- `GET /api/v1/auth/sso/{provider}` - SSO authentication
- `POST /api/v1/auth/logout` - Session termination

#### Agent Management Service
**Responsibility**: AI agent lifecycle management and verification

**Key Features**:
- Ed25519 cryptographic key pair generation
- Challenge-response verification protocol
- Trust score calculation (8-factor algorithm)
- Agent capability tracking
- MCP server connection management

**Endpoints**:
- `POST /api/v1/agents` - Create agent
- `GET /api/v1/agents` - List agents
- `GET /api/v1/agents/{id}` - Get agent details
- `PUT /api/v1/agents/{id}` - Update agent
- `DELETE /api/v1/agents/{id}` - Delete agent
- `POST /api/v1/agents/{id}/verify` - Verify agent identity
- `GET /api/v1/agents/{id}/trust-score` - Get trust score

#### MCP Server Service
**Responsibility**: Model Context Protocol server registration and verification

**Key Features**:
- MCP server registration with public key
- Capability auto-detection
- Server health monitoring
- Trust score calculation
- Agent-to-MCP relationship tracking

**Endpoints**:
- `POST /api/v1/mcp-servers` - Register MCP server
- `GET /api/v1/mcp-servers` - List MCP servers
- `GET /api/v1/mcp-servers/{id}` - Get server details
- `POST /api/v1/mcp-servers/{id}/verify` - Verify server
- `GET /api/v1/mcp-servers/{id}/capabilities` - List capabilities

#### Security Service
**Responsibility**: Threat detection, anomaly monitoring, and security enforcement

**Key Features**:
- Real-time threat detection
- Behavioral anomaly analysis
- Trust score drift monitoring
- Security alert generation
- Automated threat response

**Endpoints**:
- `GET /api/v1/security/threats` - List threats
- `GET /api/v1/security/metrics` - Security metrics
- `POST /api/v1/security/threats/{id}/block` - Block threat
- `GET /api/v1/security/anomalies` - List anomalies

#### Audit Service
**Responsibility**: Comprehensive activity logging and compliance reporting

**Key Features**:
- Immutable audit trail
- Action-level logging
- Metadata enrichment
- Compliance report generation
- Export capabilities (CSV, JSON, PDF)

**Endpoints**:
- `GET /api/v1/audit-logs` - List audit logs
- `GET /api/v1/audit-logs/{id}` - Get log details
- `POST /api/v1/audit-logs/export` - Export logs
- `GET /api/v1/audit-logs/stats` - Audit statistics

### 2. Frontend (Next.js 15 + TypeScript)

**Architecture**: App Router with Server Components and Client Components

**Key Pages**:
- `/dashboard` - Overview dashboard with metrics
- `/dashboard/agents` - Agent management
- `/dashboard/mcp` - MCP server management
- `/dashboard/monitoring` - Activity monitoring
- `/dashboard/security` - Security threats
- `/dashboard/api-keys` - API key management
- `/dashboard/sdk` - SDK download portal
- `/dashboard/admin` - Admin controls

**Components**:
- `components/ui/` - Shadcn/ui component library
- `components/modals/` - Modal dialogs
- `components/charts/` - Recharts visualization
- `lib/api.ts` - API client with automatic token refresh

### 3. SDKs

#### Python SDK
**Features**:
- One-line agent registration
- Automatic OAuth configuration
- Ed25519 cryptographic signing
- System keyring credential storage
- Auto-detect MCPs from Claude config
- Capability auto-detection

#### Go SDK
**Features**:
- Ed25519 signing
- OS keyring integration
- Agent registration workflow
- Message signing and verification

#### JavaScript SDK
**Features**:
- Ed25519 signing (KeyPair class)
- OAuth integration
- Keyring credential storage
- Agent registration
- MCP detection reporting

---

## Technology Stack

### Backend
| Component | Technology | Version | Purpose |
|-----------|-----------|---------|---------|
| Language | Go | 1.23+ | High-performance backend |
| Web Framework | Fiber | v3 | Fast HTTP framework |
| Database | PostgreSQL | 16+ | Primary data store |
| Cache | Redis | 7+ | Session cache, rate limiting |
| Crypto | crypto/ed25519 | stdlib | Digital signatures |
| JWT | golang-jwt | v5 | Token management |

### Frontend
| Component | Technology | Version | Purpose |
|-----------|-----------|---------|---------|
| Framework | Next.js | 15 | React framework |
| Language | TypeScript | 5.5+ | Type safety |
| Styling | Tailwind CSS | v4 | Utility-first CSS |
| UI Library | Shadcn/ui | latest | Component library |
| State | Zustand | latest | State management |
| Charts | Recharts | latest | Data visualization |

### Infrastructure
| Component | Technology | Purpose |
|-----------|-----------|---------|
| Containerization | Docker | Application packaging |
| Orchestration | Kubernetes | Production deployment |
| Reverse Proxy | Nginx | Load balancing, SSL |
| Monitoring | Prometheus | Metrics collection |
| Visualization | Grafana | Metrics dashboards |
| Logging | Loki + Promtail | Log aggregation |

---

## Data Flow

### 1. Agent Registration Flow

```
SDK Client → POST /api/v1/agents
    ↓
Authentication Middleware (verify JWT)
    ↓
Agent Service
    ├─ Generate Ed25519 keypair
    ├─ Store agent in PostgreSQL
    ├─ Calculate initial trust score
    └─ Log audit event
    ↓
Response: { agent_id, public_key, trust_score }
    ↓
SDK stores credentials in keyring
```

### 2. Agent Verification Flow

```
SDK Client → POST /api/v1/agents/{id}/verify
    ↓
Agent Service
    ├─ Generate random challenge
    ├─ Send challenge to agent
    └─ Wait for signed response
    ↓
Agent (SDK)
    ├─ Load private key from keyring
    ├─ Sign challenge with Ed25519
    └─ Return signature
    ↓
Agent Service
    ├─ Verify signature with public key
    ├─ Update trust score
    ├─ Create verification event
    └─ Log audit event
    ↓
Response: { verified: true, trust_score }
```

### 3. MCP Auto-Detection Flow

```
SDK Client → Auto-detect MCPs
    ↓
Read ~/.config/Claude/claude_desktop_config.json
    ↓
Parse MCP server configurations
    ↓
For each MCP:
    POST /api/v1/agents/{id}/mcp-servers
    ↓
Agent Service
    ├─ Verify MCP server exists (create if not)
    ├─ Add to agent's "talks_to" array
    ├─ Detect capabilities
    ├─ Calculate trust score
    └─ Log audit event
    ↓
Response: { registered_mcps: [...] }
```

### 4. Security Threat Detection Flow

```
Security Service (Background Job)
    ↓
Monitor agent activities
    ↓
Analyze patterns:
    ├─ Trust score drops
    ├─ Verification failures
    ├─ Anomalous actions
    ├─ Compliance violations
    └─ Behavioral drift
    ↓
If threat detected:
    ├─ Create threat record
    ├─ Calculate severity
    ├─ Send alert notification
    └─ Log security event
    ↓
Store in PostgreSQL
    ↓
Dashboard displays real-time threats
```

---

## Security Architecture

### 1. Authentication & Authorization

**Multi-Layer Security**:
```
Layer 1: Network (Nginx rate limiting)
Layer 2: Authentication (JWT tokens)
Layer 3: Authorization (RBAC)
Layer 4: Resource-level (ownership checks)
```

**Token Management**:
- JWT tokens with 24-hour expiration
- Refresh tokens with 7-day expiration
- Automatic token rotation
- Token revocation support

**RBAC Roles**:
- **Admin**: Full system access
- **Manager**: Monitoring, security, alerts
- **Member**: Agent/MCP management, API keys
- **Viewer**: Read-only access

### 2. Cryptographic Verification

**Ed25519 Digital Signatures**:
- Public key stored in database
- Private key stored in OS keyring (SDK)
- Challenge-response protocol
- 256-bit security level

**Challenge-Response Protocol**:
1. Server generates 32-byte random challenge
2. Agent signs challenge with private key
3. Server verifies signature with public key
4. On success, trust score increases

### 3. Data Protection

**At Rest**:
- PostgreSQL encryption at rest
- API key hashing (SHA-256)
- Private keys never stored (keyring only)
- Sensitive data encrypted in database

**In Transit**:
- TLS 1.3 for all connections
- Certificate pinning (production)
- HTTPS-only in production

### 4. Audit & Compliance

**Immutable Audit Trail**:
- Every API call logged
- User, IP, timestamp, action
- Metadata for context
- Append-only storage

**Compliance Features**:
- SOC 2 audit trail
- HIPAA compliance logging
- GDPR data export
- Access review reports

---

## Deployment Architecture

### Development
```
Docker Compose
├── PostgreSQL (port 5432)
├── Redis (port 6379)
├── Backend API (port 8080)
├── Frontend (port 3000)
└── Grafana (port 3003)
```

### Production (Kubernetes)
```
Kubernetes Cluster
├── Namespace: aim-production
├── Ingress: HTTPS load balancer
├── Deployments:
│   ├── backend (3 replicas)
│   ├── frontend (2 replicas)
│   └── worker (2 replicas)
├── StatefulSets:
│   ├── postgresql (3 replicas)
│   └── redis (3 replicas)
└── Services:
    ├── backend-service (ClusterIP)
    ├── frontend-service (NodePort)
    └── database-service (ClusterIP)
```

**High Availability**:
- Multi-region deployment
- Database replication
- Redis cluster
- Auto-scaling (HPA)
- Health checks and probes

---

## Integration Points

### 1. OAuth Providers
- **Google**: OAuth 2.0 + OpenID Connect
- **Microsoft**: Azure AD + OIDC
- **Okta**: SAML 2.0 + OIDC

### 2. AI Frameworks
- **CrewAI**: Wrapper for multi-agent verification
- **LangChain**: Agent executor wrapper
- **Microsoft Copilot**: Copilot Studio integration

### 3. MCP Servers
- Auto-detection from Claude config
- Public key verification
- Capability discovery
- Health monitoring

### 4. Monitoring & Logging
- **Prometheus**: Metrics collection
- **Grafana**: Dashboard visualization
- **Loki**: Log aggregation
- **Alertmanager**: Alert routing

---

## Performance Characteristics

### API Response Times (p95)
- `GET /agents`: < 50ms
- `POST /agents`: < 100ms
- `POST /agents/{id}/verify`: < 200ms (includes crypto)
- `GET /audit-logs`: < 100ms

### Throughput
- 1000+ requests/second (single instance)
- 10,000+ requests/second (clustered)

### Scalability
- Horizontal scaling: Add backend replicas
- Vertical scaling: Increase resources
- Database: Read replicas for queries
- Cache: Redis cluster for sessions

---

## Future Architecture Enhancements

### Phase 2 (Q1 2025)
- GraphQL API alongside REST
- WebSocket real-time updates
- Advanced caching strategies
- Event-driven architecture (NATS)

### Phase 3 (Q2 2025)
- Federated identity
- Zero-knowledge proofs
- Blockchain anchoring
- AI-powered threat detection

---

## References

- [Deployment Guide](DEPLOYMENT.md)
- [Security Best Practices](SECURITY.md)
- [API Documentation](API.md)
- [Performance Tuning](PERFORMANCE.md)
- [Production Deployment](PRODUCTION.md)

---

**Maintained by**: OpenA2A Team
**Last Review**: October 10, 2025
**Next Review**: January 10, 2026
