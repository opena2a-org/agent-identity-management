# ADR 003: Multi-Tenancy Strategy

**Status**: ✅ Accepted
**Date**: 2025-10-06
**Decision Makers**: AIM Architecture Team, Security Team
**Stakeholders**: Backend Team, Database Team, Enterprise Customers

---

## Context

AIM must support multiple enterprise customers (organizations) on the same infrastructure while ensuring:
1. **Data Isolation**: No cross-organization data access
2. **Security**: Each organization's data is completely isolated
3. **Scalability**: Single database serves all tenants efficiently
4. **Cost-Effectiveness**: No per-tenant infrastructure
5. **Performance**: Query performance remains fast as tenants grow
6. **Compliance**: Meet SOC 2, HIPAA, GDPR isolation requirements

### Multi-Tenancy Approaches

| Approach | Description | Pros | Cons |
|----------|-------------|------|------|
| **Separate Databases** | Each tenant gets own database | Perfect isolation | Expensive, complex backups |
| **Separate Schemas** | Each tenant gets own schema | Good isolation | Complex migrations |
| **Shared Schema** | All tenants in same schema with `organization_id` | Cost-effective, simple | Requires strict RLS |

---

## Decision

We will implement **Shared Schema Multi-Tenancy** with **PostgreSQL Row-Level Security (RLS)** for data isolation.

### Implementation Strategy

#### 1. Database Schema Design

**Every table includes `organization_id`**:

```sql
-- Core multi-tenant table structure
CREATE TABLE agents (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    type            TEXT NOT NULL CHECK (type IN ('ai_agent', 'mcp_server')),
    is_active       BOOLEAN DEFAULT true,
    trust_score     DECIMAL(5,2) DEFAULT 0,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW(),
    last_verified_at TIMESTAMPTZ,

    -- Indexes for multi-tenant queries
    CONSTRAINT agents_name_org_unique UNIQUE (organization_id, name)
);

-- Composite indexes for tenant-scoped queries
CREATE INDEX idx_agents_org_id ON agents(organization_id);
CREATE INDEX idx_agents_org_active ON agents(organization_id, is_active);
CREATE INDEX idx_agents_org_trust ON agents(organization_id, trust_score DESC);
```

#### 2. Row-Level Security (RLS)

**PostgreSQL RLS enforces data isolation**:

```sql
-- Enable RLS on all multi-tenant tables
ALTER TABLE agents ENABLE ROW LEVEL SECURITY;

-- Policy 1: Users can only see their organization's data
CREATE POLICY org_isolation_select ON agents
    FOR SELECT
    USING (organization_id = current_setting('app.current_org_id')::UUID);

-- Policy 2: Users can only insert into their organization
CREATE POLICY org_isolation_insert ON agents
    FOR INSERT
    WITH CHECK (organization_id = current_setting('app.current_org_id')::UUID);

-- Policy 3: Users can only update their organization's data
CREATE POLICY org_isolation_update ON agents
    FOR UPDATE
    USING (organization_id = current_setting('app.current_org_id')::UUID)
    WITH CHECK (organization_id = current_setting('app.current_org_id')::UUID);

-- Policy 4: Users can only delete their organization's data
CREATE POLICY org_isolation_delete ON agents
    FOR DELETE
    USING (organization_id = current_setting('app.current_org_id')::UUID);
```

#### 3. Application-Level Enforcement

**Backend sets organization context for every request**:

```go
// internal/infrastructure/persistence/postgres/connection.go
package postgres

import (
    "context"
    "fmt"
    "github.com/google/uuid"
    "github.com/jmoiron/sqlx"
)

// SetOrganizationContext sets the current organization for RLS
func SetOrganizationContext(ctx context.Context, db *sqlx.DB, orgID uuid.UUID) error {
    query := fmt.Sprintf("SET LOCAL app.current_org_id = '%s'", orgID.String())
    _, err := db.ExecContext(ctx, query)
    return err
}

// WithOrganizationContext wraps a database operation with organization context
func WithOrganizationContext(ctx context.Context, db *sqlx.DB, orgID uuid.UUID, fn func() error) error {
    // Start transaction
    tx, err := db.BeginTxx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()

    // Set organization context
    query := fmt.Sprintf("SET LOCAL app.current_org_id = '%s'", orgID.String())
    if _, err := tx.ExecContext(ctx, query); err != nil {
        return err
    }

    // Execute function
    if err := fn(); err != nil {
        return err
    }

    // Commit transaction
    return tx.Commit()
}
```

**Middleware extracts organization from JWT**:

```go
// internal/interfaces/http/middleware/organization_middleware.go
package middleware

import (
    "github.com/gofiber/fiber/v3"
    "github.com/google/uuid"
)

func OrganizationMiddleware() fiber.Handler {
    return func(c fiber.Ctx) error {
        // 1. Extract organization_id from JWT (set by auth middleware)
        orgIDValue := c.Locals("organization_id")
        if orgIDValue == nil {
            return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
                "error": "No organization context",
            })
        }

        orgID, ok := orgIDValue.(uuid.UUID)
        if !ok {
            return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
                "error": "Invalid organization context",
            })
        }

        // 2. Store in context for use in handlers
        c.Locals("organization_id", orgID)

        return c.Next()
    }
}
```

**Handlers use organization context**:

```go
// internal/interfaces/http/handlers/agent_handler.go
package handlers

import (
    "github.com/gofiber/fiber/v3"
    "github.com/google/uuid"
    "github.com/opena2a/identity/backend/internal/application"
    "github.com/opena2a/identity/backend/internal/infrastructure/persistence/postgres"
)

type AgentHandler struct {
    agentService *application.AgentService
    db          *sqlx.DB
}

func (h *AgentHandler) ListAgents(c fiber.Ctx) error {
    // 1. Get organization from context
    orgID := c.Locals("organization_id").(uuid.UUID)

    // 2. Set PostgreSQL RLS context
    err := postgres.SetOrganizationContext(c.Context(), h.db, orgID)
    if err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "error": "Failed to set organization context",
        })
    }

    // 3. Query agents (RLS automatically filters by organization_id)
    agents, err := h.agentService.ListAgents(c.Context(), orgID, filters)
    if err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "error": "Failed to list agents",
        })
    }

    return c.JSON(agents)
}
```

#### 4. Organization Table

```sql
CREATE TABLE organizations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT NOT NULL UNIQUE,
    plan            TEXT NOT NULL CHECK (plan IN ('community', 'pro', 'enterprise')),
    max_agents      INTEGER NOT NULL DEFAULT 50,
    is_active       BOOLEAN DEFAULT true,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW(),

    -- Enterprise features
    sso_enabled     BOOLEAN DEFAULT false,
    custom_domain   TEXT,
    whitelabel      BOOLEAN DEFAULT false
);

-- Users belong to organizations
CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    email           TEXT NOT NULL,
    name            TEXT NOT NULL,
    role            TEXT NOT NULL CHECK (role IN ('admin', 'manager', 'member', 'viewer')),
    is_active       BOOLEAN DEFAULT true,
    created_at      TIMESTAMPTZ DEFAULT NOW(),

    CONSTRAINT users_email_org_unique UNIQUE (organization_id, email)
);

CREATE INDEX idx_users_org_id ON users(organization_id);
```

---

## Consequences

### Positive

1. **Cost-Effective**:
   - Single database serves all tenants
   - No per-tenant infrastructure costs
   - Shared connection pool
   - Efficient resource utilization

2. **Simple Operations**:
   - Single migration applies to all tenants
   - Single backup covers all organizations
   - Single monitoring dashboard

3. **Performance**:
   - Composite indexes optimize tenant queries: `(organization_id, other_columns)`
   - PostgreSQL query planner uses organization_id effectively
   - Connection pooling shared across tenants

4. **Security**:
   - **PostgreSQL RLS** provides database-level enforcement
   - Even if application bug bypasses checks, RLS prevents data leaks
   - Audit logs automatically include organization_id

5. **Compliance**:
   - Data isolation meets SOC 2 requirements
   - GDPR data deletion simple (DELETE WHERE organization_id = X)
   - Audit trail preserved per organization

### Negative

1. **Noisy Neighbor Problem**:
   - One tenant's heavy queries can affect others
   - **Mitigation**: Query timeouts, connection limits per organization

2. **Schema Changes Impact All Tenants**:
   - Migration downtime affects all organizations
   - **Mitigation**: Zero-downtime migrations, blue-green deployments

3. **Data Recovery Complexity**:
   - Cannot restore single tenant from backup without filtering
   - **Mitigation**: Point-in-time recovery per organization_id

4. **Query Performance Degradation**:
   - As data grows, queries may slow down
   - **Mitigation**: Partitioning by organization_id when needed

### Mitigation Strategies

1. **Connection Pooling Limits**:
   ```go
   // Limit connections per organization
   type OrgConnectionPool struct {
       pools map[uuid.UUID]*pgxpool.Pool
       limits map[uuid.UUID]int
   }
   ```

2. **Query Timeout Enforcement**:
   ```sql
   -- Set statement timeout per organization
   SET LOCAL statement_timeout = '30s';
   ```

3. **Table Partitioning (Future)**:
   ```sql
   -- If single table grows too large, partition by organization_id
   CREATE TABLE agents PARTITION OF agents_partitioned
       FOR VALUES IN ('org-uuid-1', 'org-uuid-2', ...);
   ```

---

## Alternatives Considered

### 1. Separate Databases Per Tenant
**Rejected because**:
- **Cost**: Infrastructure per tenant (expensive)
- **Operations**: Complex migrations (must run per tenant)
- **Backups**: N separate backups needed
- **Monitoring**: N separate monitoring setups

**When to reconsider**:
- Enterprise customers demanding dedicated infrastructure
- Regulatory requirements mandate physical separation
- Performance degradation at massive scale

### 2. Separate Schemas Per Tenant
**Rejected because**:
- **Migrations**: Must run per schema (complex)
- **Connection Management**: Separate connection pools per schema
- **Limited Scalability**: PostgreSQL schema limits (~10,000)

**When to reconsider**:
- Customers require schema-level isolation
- Regulatory compliance mandates logical separation

### 3. Sharding by Organization
**Rejected for now (future consideration)**:
- **Complexity**: Distributed transactions, shard routing
- **Premature**: Not needed until 10,000+ organizations

**When to implement**:
- 10,000+ organizations
- Database size > 1TB
- Query performance degrades despite optimization

---

## Testing Multi-Tenancy

### Integration Tests

```go
func TestMultiTenancy_DataIsolation(t *testing.T) {
    // Create two organizations
    org1 := createTestOrganization(t, "Org 1")
    org2 := createTestOrganization(t, "Org 2")

    // Create agents for each organization
    agent1 := createTestAgent(t, org1.ID, "Agent 1")
    agent2 := createTestAgent(t, org2.ID, "Agent 2")

    // Query as Org 1 (should only see Agent 1)
    SetOrganizationContext(ctx, db, org1.ID)
    agents, err := agentRepo.List(ctx, org1.ID, Filters{})
    require.NoError(t, err)
    assert.Len(t, agents, 1)
    assert.Equal(t, agent1.ID, agents[0].ID)

    // Query as Org 2 (should only see Agent 2)
    SetOrganizationContext(ctx, db, org2.ID)
    agents, err = agentRepo.List(ctx, org2.ID, Filters{})
    require.NoError(t, err)
    assert.Len(t, agents, 1)
    assert.Equal(t, agent2.ID, agents[0].ID)
}
```

### Security Tests

```go
func TestMultiTenancy_CrossTenantAccessDenied(t *testing.T) {
    org1 := createTestOrganization(t, "Org 1")
    org2 := createTestOrganization(t, "Org 2")

    agent1 := createTestAgent(t, org1.ID, "Agent 1")

    // Attempt to access Org 1's agent while authenticated as Org 2
    SetOrganizationContext(ctx, db, org2.ID)
    agent, err := agentRepo.GetByID(ctx, agent1.ID)

    // Should return "not found" (RLS blocks access)
    assert.Error(t, err)
    assert.Nil(t, agent)
}
```

---

## References

- [PostgreSQL Row-Level Security](https://www.postgresql.org/docs/16/ddl-rowsecurity.html)
- [Multi-Tenancy Database Patterns](https://www.microsoft.com/en-us/download/details.aspx?id=29263)
- [Designing Multi-Tenant SaaS](https://aws.amazon.com/blogs/architecture/saas-tenant-isolation-strategies/)

---

**Last Updated**: October 6, 2025
**Related ADRs**: ADR-001 (Technology Stack), ADR-002 (Clean Architecture)
