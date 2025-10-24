# AIM Architecture Documentation

**Welcome to the AIM (Agent Identity Management) Architecture Documentation**

This directory contains comprehensive architectural documentation for the AIM platform, including system design, technical decisions, and implementation patterns.

---

## 📚 Documentation Index

### Core Documents

1. **[SYSTEM_ARCHITECTURE.md](./SYSTEM_ARCHITECTURE.md)**
   - Complete system architecture overview
   - Component breakdown and interactions
   - Technology stack details
   - Performance targets and scalability strategy
   - Deployment architecture

### Architecture Decision Records (ADRs)

All significant architectural decisions are documented as ADRs following the [MADR format](https://adr.github.io/madr/):

1. **[ADR-001: Technology Stack Selection](./adr/001-technology-stack-selection.md)**
   - Why we chose Go, Fiber v3, Next.js 15, PostgreSQL, Redis
   - Alternatives considered and rejected
   - Performance and scalability justifications

2. **[ADR-002: Clean Architecture Pattern](./adr/002-clean-architecture-pattern.md)**
   - Domain-driven design implementation
   - Layer separation and dependency rules
   - Testing benefits and maintainability

3. **[ADR-003: Multi-Tenancy Strategy](./adr/003-multi-tenancy-strategy.md)**
   - Shared schema with PostgreSQL Row-Level Security
   - Organization-level data isolation
   - Performance and security considerations

4. **[ADR-004: Trust Scoring Algorithm](./adr/004-trust-scoring-algorithm.md)**
   - 8-factor ML-powered trust scoring system
   - Transparent, fair, and actionable scoring
   - Algorithm implementation and versioning

5. **[ADR-005: API Authentication & Authorization](./adr/005-api-authentication-authorization.md)**
   - OAuth2/OIDC for web users (Google, Microsoft, Okta)
   - JWT token management (access + refresh)
   - API key authentication for programmatic access
   - RBAC with 4 role levels (Admin, Manager, Member, Viewer)

6. **[ADR-006: Runtime Verification & Capability-Based Authorization](./adr/006-runtime-verification-capability-authorization.md)** ⭐️ **CORE MISSION**
   - Pre-execution authorization for all agent/MCP actions
   - Capability-based permission system with granular controls
   - Anomaly detection and phishing prevention
   - Complete audit trail for enterprise trust
   - Real-time verification API (<50ms latency target)

---

## 🏗️ System Overview

### High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        Client Layer                          │
├─────────────────────────────────────────────────────────────┤
│  Web App (Next.js 15)  │  Mobile App  │  CLI Tool  │  API   │
└───────────────┬─────────────────────────────────────────────┘
                │
                ├─── HTTPS/TLS ───┐
                │                  │
┌───────────────▼──────────────────▼──────────────────────────┐
│                   Load Balancer (Nginx)                      │
└───────────────┬──────────────────────────────────────────────┘
                │
                ├─── Backend Services ───┐
                │                          │
┌───────────────▼──────────────────────────▼──────────────────┐
│          Application Layer (Go + Fiber v3)                   │
├──────────────────────────────────────────────────────────────┤
│  Auth │ Agents │ Trust │ API Keys │ Audit │ Alerts │ MCP    │
└───────────────┬──────────────────────────────────────────────┘
                │
                ├─── Data Layer ───┐
                │                   │
┌───────────────▼───────────────────▼──────────────────────────┐
│  PostgreSQL 16     │     Redis 7      │    TimescaleDB       │
│  (Primary DB)      │    (Cache)       │  (Time-Series)       │
└──────────────────────────────────────────────────────────────┘
```

### Key Components

| Component | Technology | Purpose |
|-----------|-----------|---------|
| **Frontend** | Next.js 15 + React 19 | Enterprise UI, user experience |
| **Backend** | Go 1.23 + Fiber v3 | High-performance API layer |
| **Database** | PostgreSQL 16 | Primary data store |
| **Cache** | Redis 7 | Sessions, rate limiting |
| **Time-Series** | TimescaleDB | Audit logs, trust scores |

---

## 🎯 Design Principles

### 1. Security-First
- All endpoints require authentication
- OAuth2/OIDC for SSO (no password storage)
- API keys hashed with SHA-256
- JWT tokens with short expiry
- PostgreSQL RLS for multi-tenancy isolation

### 2. Clean Architecture
- Domain layer has zero dependencies
- Business logic separate from infrastructure
- Easy to test and maintain
- Flexible to swap components

### 3. Performance-Oriented
- **Target**: API response <100ms (p95)
- Connection pooling (max 100 connections)
- Redis caching for frequently accessed data
- Composite indexes for tenant-scoped queries

### 4. Scalability
- Horizontal scaling with Kubernetes
- Stateless backend services
- Multi-tenant database (single DB, all tenants)
- Load balancing with Nginx

### 5. Observability
- Comprehensive audit logging (TimescaleDB)
- Prometheus metrics collection
- Grafana dashboards
- ELK stack for centralized logging

---

## 📊 Current Status

### Phase 1: Foundation ✅ **COMPLETED**
- [x] Backend API (62+ endpoints implemented)
- [x] Clean Architecture pattern
- [x] Multi-tenancy with RLS
- [x] OAuth2/OIDC authentication
- [x] JWT token management
- [x] API key authentication
- [x] RBAC authorization
- [x] Trust scoring algorithm
- [x] Enterprise UI redesign
- [x] Docker Compose infrastructure
- [x] Integration tests (21/21 passing)

### Phase 2: Feature Completeness ✅ **COMPLETED**
- [x] Runtime verification endpoints (3 endpoints) ⭐️ **CORE MISSION**
- [x] MCP server registration (9 endpoints)
- [x] Security dashboard (6 endpoints)
- [x] Compliance reporting (12 endpoints)
- [x] Analytics & reporting (4 endpoints)
- [x] Webhook integration (5 endpoints)

**Achievement**: 62+ endpoints (103% of 60+ target)

---

## 🚀 Getting Started

### For Developers

1. **Read Core Documents First**:
   - [SYSTEM_ARCHITECTURE.md](./SYSTEM_ARCHITECTURE.md) - Understand the big picture
   - [ADR-002: Clean Architecture](./adr/002-clean-architecture-pattern.md) - Learn the code structure

2. **Understand Key Decisions**:
   - [ADR-001: Technology Stack](./adr/001-technology-stack-selection.md) - Why these technologies?
   - [ADR-003: Multi-Tenancy](./adr/003-multi-tenancy-strategy.md) - How does data isolation work?

3. **Implement Features**:
   - Follow Clean Architecture pattern
   - Add tests for all code
   - Document architectural decisions as ADRs

### For Architects

1. **Review System Architecture**:
   - [SYSTEM_ARCHITECTURE.md](./SYSTEM_ARCHITECTURE.md) - Complete technical design

2. **Understand Trade-offs**:
   - Read all ADRs to understand decisions and alternatives
   - Review "Consequences" sections for trade-offs

3. **Plan New Features**:
   - Create ADRs for significant decisions
   - Update system architecture document
   - Consider security, scalability, performance

---

## 📝 Contributing to Architecture Docs

### When to Create an ADR

Create a new ADR when making decisions about:
- Technology selection (libraries, frameworks, databases)
- Architecture patterns (MVC, Clean Architecture, Microservices)
- Security approaches (authentication, authorization, encryption)
- Performance strategies (caching, indexing, partitioning)
- Integration patterns (APIs, webhooks, events)

### ADR Template

```markdown
# ADR NNN: Decision Title

**Status**: Proposed | Accepted | Deprecated | Superseded by ADR-XXX
**Date**: YYYY-MM-DD
**Decision Makers**: Team/Role
**Stakeholders**: Affected parties

## Context
What is the issue that we're seeing that is motivating this decision or change?

## Decision
What is the change that we're proposing and/or doing?

## Consequences
What becomes easier or more difficult to do because of this change?

### Positive
### Negative
### Mitigation

## Alternatives Considered
What other options were considered and why were they rejected?

## References
- Links to relevant resources
```

### Updating Documentation

- **System Architecture**: Update when adding major components or changing data flows
- **ADRs**: Create new ADRs, never modify accepted ones (create superseding ADR instead)
- **README**: Update index when adding new documents

---

## 🔗 Related Documentation

- **[../AIM_VISION.md](../AIM_VISION.md)** - Product vision and investment strategy
- **[../claude.md](../claude.md)** - Development guidelines and workflow
- **[../README.md](../README.md)** - Project overview and setup
- **[../API_REFERENCE.md](../API_REFERENCE.md)** - API endpoint documentation
- **[../API_ENDPOINT_SUMMARY.md](../API_ENDPOINT_SUMMARY.md)** - Complete endpoint catalog (62+ endpoints)
- **[../PRODUCTION_READINESS.md](../PRODUCTION_READINESS.md)** - Deployment readiness

---

## 🤝 Questions or Feedback

- **GitHub Issues**: [https://github.com/opena2a/identity/issues](https://github.com/opena2a/identity/issues)
- **Discord**: [https://discord.gg/opena2a](https://discord.gg/opena2a)
- **Email**: info@opena2a.org

---

**Last Updated**: October 6, 2025
**Version**: 2.0.0
**Maintained By**: AIM Architecture Team
