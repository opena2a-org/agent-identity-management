# ADR 002: Clean Architecture Pattern

**Status**: ✅ Accepted
**Date**: 2025-10-06
**Decision Makers**: AIM Architecture Team
**Stakeholders**: Backend Team, QA Team

---

## Context

AIM requires a backend architecture that:
1. Enables independent testing of business logic
2. Allows swapping infrastructure components (database, cache, etc.) without affecting core logic
3. Supports 60+ endpoints with consistent structure
4. Facilitates team collaboration and code maintainability
5. Enables gradual migration and refactoring

The AIVF codebase suffered from tight coupling between business logic and infrastructure (database, external APIs), making it difficult to test and maintain.

---

## Decision

We will implement **Clean Architecture** (also known as Hexagonal Architecture or Ports & Adapters) for the backend, organized as follows:

```
apps/backend/internal/
├── domain/              # Layer 1: Pure Business Logic (Zero Dependencies)
│   ├── entities/        # Business entities (User, Agent, Organization)
│   ├── value_objects/   # Value objects (Email, TrustScore, AgentType)
│   └── interfaces/      # Repository interfaces (contracts)
│
├── application/         # Layer 2: Use Cases / Business Rules
│   ├── auth_service.go
│   ├── agent_service.go
│   ├── trust_service.go
│   └── audit_service.go
│
├── infrastructure/      # Layer 3: External Dependencies
│   ├── persistence/     # Database implementations
│   │   ├── postgres/    # PostgreSQL repository implementations
│   │   └── redis/       # Redis cache implementations
│   ├── auth/            # OAuth, JWT implementations
│   ├── email/           # Email service implementations
│   └── config/          # Configuration management
│
└── interfaces/          # Layer 4: Delivery Mechanisms
    ├── http/            # HTTP handlers
    │   ├── handlers/    # Route handlers
    │   ├── middleware/  # Auth, logging, CORS
    │   └── dto/         # Data Transfer Objects
    └── cli/             # CLI commands (future)
```

### Dependency Rule

**Dependencies point inward only**:
```
Interfaces → Application → Domain
Infrastructure → Application → Domain

Domain depends on: NOTHING (pure Go)
Application depends on: Domain
Infrastructure depends on: Domain, Application
Interfaces depends on: Domain, Application, Infrastructure
```

### Example Implementation

#### Domain Layer (Pure Business Logic)

```go
// internal/domain/entities/agent.go
package entities

import (
    "time"
    "github.com/google/uuid"
)

// Agent represents an AI agent or MCP server
type Agent struct {
    ID              uuid.UUID
    OrganizationID  uuid.UUID
    Name            string
    Type            AgentType
    IsActive        bool
    TrustScore      float64
    CreatedAt       time.Time
    UpdatedAt       time.Time
    LastVerifiedAt  *time.Time
}

// Business rules live in domain
func (a *Agent) Verify() error {
    if !a.IsActive {
        return errors.New("cannot verify inactive agent")
    }
    now := time.Now()
    a.LastVerifiedAt = &now
    return nil
}
```

#### Repository Interface (Domain Layer)

```go
// internal/domain/interfaces/agent_repository.go
package interfaces

import (
    "context"
    "github.com/google/uuid"
    "github.com/opena2a/identity/backend/internal/domain/entities"
)

// AgentRepository defines what the domain needs from persistence
type AgentRepository interface {
    GetByID(ctx context.Context, id uuid.UUID) (*entities.Agent, error)
    Create(ctx context.Context, agent *entities.Agent) error
    Update(ctx context.Context, agent *entities.Agent) error
    Delete(ctx context.Context, id uuid.UUID) error
    List(ctx context.Context, orgID uuid.UUID, filters Filters) ([]*entities.Agent, error)
}
```

#### Application Layer (Use Cases)

```go
// internal/application/agent_service.go
package application

import (
    "context"
    "github.com/google/uuid"
    "github.com/opena2a/identity/backend/internal/domain/entities"
    "github.com/opena2a/identity/backend/internal/domain/interfaces"
)

type AgentService struct {
    agentRepo    interfaces.AgentRepository
    trustService *TrustService
    auditLogger  interfaces.AuditLogger
}

func NewAgentService(
    agentRepo interfaces.AgentRepository,
    trustService *TrustService,
    auditLogger interfaces.AuditLogger,
) *AgentService {
    return &AgentService{
        agentRepo:    agentRepo,
        trustService: trustService,
        auditLogger:  auditLogger,
    }
}

func (s *AgentService) VerifyAgent(ctx context.Context, agentID uuid.UUID) error {
    // 1. Fetch agent from repository
    agent, err := s.agentRepo.GetByID(ctx, agentID)
    if err != nil {
        return err
    }

    // 2. Apply business rule (domain logic)
    if err := agent.Verify(); err != nil {
        return err
    }

    // 3. Recalculate trust score
    if err := s.trustService.RecalculateTrustScore(ctx, agentID); err != nil {
        return err
    }

    // 4. Update in repository
    if err := s.agentRepo.Update(ctx, agent); err != nil {
        return err
    }

    // 5. Log audit event
    s.auditLogger.Log(ctx, "agent_verified", agentID)

    return nil
}
```

#### Infrastructure Layer (Database Implementation)

```go
// internal/infrastructure/persistence/postgres/agent_repository.go
package postgres

import (
    "context"
    "github.com/google/uuid"
    "github.com/jmoiron/sqlx"
    "github.com/opena2a/identity/backend/internal/domain/entities"
    "github.com/opena2a/identity/backend/internal/domain/interfaces"
)

type AgentRepository struct {
    db *sqlx.DB
}

func NewAgentRepository(db *sqlx.DB) interfaces.AgentRepository {
    return &AgentRepository{db: db}
}

func (r *AgentRepository) GetByID(ctx context.Context, id uuid.UUID) (*entities.Agent, error) {
    var agent entities.Agent
    query := `
        SELECT id, organization_id, name, type, is_active, trust_score,
               created_at, updated_at, last_verified_at
        FROM agents
        WHERE id = $1
    `
    err := r.db.GetContext(ctx, &agent, query, id)
    return &agent, err
}

func (r *AgentRepository) Update(ctx context.Context, agent *entities.Agent) error {
    query := `
        UPDATE agents
        SET name = $1, type = $2, is_active = $3, trust_score = $4,
            updated_at = NOW(), last_verified_at = $5
        WHERE id = $6
    `
    _, err := r.db.ExecContext(ctx, query,
        agent.Name, agent.Type, agent.IsActive, agent.TrustScore,
        agent.LastVerifiedAt, agent.ID,
    )
    return err
}
```

#### Interfaces Layer (HTTP Handler)

```go
// internal/interfaces/http/handlers/agent_handler.go
package handlers

import (
    "github.com/gofiber/fiber/v3"
    "github.com/google/uuid"
    "github.com/opena2a/identity/backend/internal/application"
)

type AgentHandler struct {
    agentService *application.AgentService
}

func NewAgentHandler(agentService *application.AgentService) *AgentHandler {
    return &AgentHandler{agentService: agentService}
}

func (h *AgentHandler) VerifyAgent(c fiber.Ctx) error {
    agentID, err := uuid.Parse(c.Params("id"))
    if err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
            "error": "Invalid agent ID",
        })
    }

    if err := h.agentService.VerifyAgent(c.Context(), agentID); err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "error": "Failed to verify agent",
        })
    }

    return c.JSON(fiber.Map{
        "message": "Agent verified successfully",
    })
}
```

---

## Consequences

### Positive

1. **Testability**:
   - Domain logic can be tested in isolation (zero dependencies)
   - Easy to mock interfaces for unit tests
   - No database required for business logic tests

2. **Flexibility**:
   - Can swap PostgreSQL for MySQL without touching business logic
   - Can swap Redis for Memcached without affecting use cases
   - Can add new delivery mechanisms (gRPC, GraphQL) easily

3. **Maintainability**:
   - Clear separation of concerns
   - Easy to locate code (handler → service → repository)
   - Business rules centralized in domain layer

4. **Team Collaboration**:
   - Multiple developers can work on different layers simultaneously
   - Interfaces define contracts between layers
   - Less merge conflicts

5. **Scalability**:
   - Services are stateless and can be scaled horizontally
   - No tight coupling to specific infrastructure

### Negative

1. **Initial Complexity**:
   - More files and directories than traditional MVC
   - Developers must understand layer boundaries
   - More boilerplate code (interfaces, DTOs)

2. **Learning Curve**:
   - Team needs to understand Clean Architecture principles
   - Requires discipline to maintain layer boundaries

3. **Overhead for Simple Features**:
   - Simple CRUD operations require more code
   - May feel over-engineered for trivial endpoints

### Mitigation

1. **Documentation**:
   - This ADR explains the pattern clearly
   - Code comments reference layer responsibilities
   - Examples provided for common scenarios

2. **Code Reviews**:
   - Enforce layer boundaries during PR reviews
   - Check for domain layer purity (no infrastructure imports)

3. **Templates**:
   - Provide templates for new features (entity, repository, service, handler)
   - Reduce boilerplate with code generation tools

---

## Alternatives Considered

### 1. Traditional MVC (Model-View-Controller)
**Rejected because**:
- Tight coupling between models and database
- Difficult to test business logic
- Controllers often become bloated with business logic

### 2. Microservices Architecture
**Rejected because**:
- Overkill for current scale (60+ endpoints)
- Increased operational complexity (multiple deployments)
- Network latency between services
- Can migrate to microservices later if needed

### 3. Modular Monolith
**Considered but less flexible**:
- Similar benefits but less strict boundaries
- Easier to accidentally violate module boundaries
- Clean Architecture provides stronger guarantees

---

## Testing Benefits

### Unit Tests (Domain Layer)

```go
func TestAgent_Verify(t *testing.T) {
    agent := &entities.Agent{
        ID:       uuid.New(),
        IsActive: true,
    }

    err := agent.Verify()
    assert.NoError(t, err)
    assert.NotNil(t, agent.LastVerifiedAt)
}
```

### Integration Tests (Application Layer)

```go
func TestAgentService_VerifyAgent(t *testing.T) {
    // Setup mock repository
    mockRepo := &mock.AgentRepository{}
    mockTrust := &mock.TrustService{}
    mockAudit := &mock.AuditLogger{}

    service := application.NewAgentService(mockRepo, mockTrust, mockAudit)

    // Test
    err := service.VerifyAgent(context.Background(), agentID)
    assert.NoError(t, err)

    // Verify interactions
    mockRepo.AssertCalled(t, "GetByID", agentID)
    mockRepo.AssertCalled(t, "Update", mock.Anything)
    mockTrust.AssertCalled(t, "RecalculateTrustScore", agentID)
}
```

---

## References

- [The Clean Architecture by Robert C. Martin](https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html)
- [Hexagonal Architecture by Alistair Cockburn](https://alistair.cockburn.us/hexagonal-architecture/)
- [Domain-Driven Design by Eric Evans](https://www.domainlanguage.com/ddd/)

---

**Last Updated**: October 6, 2025
**Related ADRs**: ADR-001 (Technology Stack), ADR-003 (Multi-Tenancy)
