# ADR 006: Runtime Verification & Capability-Based Authorization

**Status**: ✅ Accepted
**Date**: 2025-10-06
**Decision Makers**: AIM Architecture Team, Security Team, Product Team
**Stakeholders**: Enterprise Customers, Developers, Compliance Officers

---

## Context

### The Core Problem

Enterprises are increasingly deploying AI agents and MCP (Model Context Protocol) servers in their environments, but face critical security and governance challenges:

1. **Agent Trust Gap**: How do employees know if an AI agent is legitimate and safe to use?
2. **Capability Drift**: How do we prevent agents from exceeding their authorized scope?
3. **MCP Security**: How do we verify MCP servers aren't accessing unauthorized data?
4. **Audit Trail**: How do we track what AI tools are doing in our environment?
5. **Phishing Detection**: How do we detect when an agent is behaving abnormally?

### Current State (Without AIM)

- ❌ Employees use AI agents without verification
- ❌ No visibility into agent capabilities or permissions
- ❌ MCPs can access services without authorization checks
- ❌ No audit trail of AI/MCP activities
- ❌ Security teams blind to AI-related threats

### AIM's Mission

**AIM is a runtime verification and capability-based authorization platform for AI agents and MCP servers.**

**Key Principle**: Every action by an AI agent or MCP must be verified against its registered capabilities BEFORE execution.

---

## Decision

We will implement a **Real-Time Verification Architecture** with three core flows:

### 1. Agent/MCP Registration Flow
### 2. Runtime Verification Flow (Per-Action)
### 3. Anomaly Detection & Response Flow

---

## Architecture Design

### Flow 1: Agent Registration & Capability Definition

```
┌─────────────┐
│  Employee   │
└──────┬──────┘
       │ 1. Register Agent
       ▼
┌─────────────────────────────────────────────────────┐
│              AIM Registration Portal                 │
├─────────────────────────────────────────────────────┤
│  • Agent Name                                        │
│  • Description                                       │
│  • Vendor/Source                                     │
│  • CAPABILITIES (Critical):                          │
│    ✓ Can read files: Yes/No                         │
│    ✓ Can write files: Yes/No                        │
│    ✓ Can execute code: Yes/No                       │
│    ✓ Can access network: Yes/No                     │
│    ✓ Can access databases: Which ones?              │
│    ✓ Can call APIs: Which endpoints?                │
│    ✓ Allowed file paths: ["/data/reports/*"]       │
│    ✓ Max file size: 10MB                            │
│    ✓ Rate limits: 100 req/min                       │
└──────────────────┬──────────────────────────────────┘
                   │ 2. Store in AIM
                   ▼
┌─────────────────────────────────────────────────────┐
│         AIM Database (PostgreSQL)                    │
├─────────────────────────────────────────────────────┤
│  agents:                                             │
│    - id: agent_123                                   │
│    - name: "Data Analyst Agent"                      │
│    - organization_id: org_xyz                        │
│    - capabilities: {                                 │
│        "can_read_files": true,                       │
│        "allowed_paths": ["/data/reports/*"],         │
│        "can_execute_code": false,                    │
│        "can_access_network": false,                  │
│        "rate_limit": 100                             │
│      }                                               │
│    - is_verified: true                               │
│    - trust_score: 85                                 │
└─────────────────────────────────────────────────────┘
```

### Flow 2: Runtime Verification (Every Agent Action)

**CRITICAL**: This is the core value proposition of AIM!

```
Employee requests AI agent to perform action
           │
           ▼
┌─────────────────────────────────────────────────────┐
│         AI Agent Runtime (Before Execution)          │
└──────────────────┬──────────────────────────────────┘
                   │
                   │ STEP 1: Call AIM Verification API
                   ▼
┌─────────────────────────────────────────────────────┐
│   POST /api/v1/agents/:id/verify-action              │
│   {                                                   │
│     "agent_id": "agent_123",                         │
│     "action_type": "read_file",                      │
│     "resource": "/data/reports/sales.csv",           │
│     "metadata": {                                    │
│       "file_size": "5MB",                            │
│       "user_id": "user_456"                          │
│     }                                                │
│   }                                                   │
└──────────────────┬──────────────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────────────────┐
│          AIM Verification Engine                     │
├─────────────────────────────────────────────────────┤
│  1. Fetch agent capabilities from database           │
│  2. Check: Does action match capabilities?           │
│     ✓ can_read_files = true                         │
│     ✓ resource matches allowed_paths pattern?       │
│     ✓ file_size < max_file_size?                    │
│     ✓ rate_limit not exceeded?                      │
│  3. Run anomaly detection:                           │
│     • Is this normal behavior for this agent?        │
│     • Has agent pattern changed suddenly?            │
│     • Is this outside business hours?                │
│  4. Make decision: ALLOW or DENY                     │
│  5. Log audit trail                                  │
└──────────────────┬──────────────────────────────────┘
                   │
                   ├─── ALLOW ───┐
                   │              │
                   │              ▼
                   │     ┌──────────────────┐
                   │     │  Execute Action  │
                   │     │  + Log Success   │
                   │     └──────────────────┘
                   │
                   └─── DENY ────┐
                                 │
                                 ▼
                        ┌─────────────────────┐
                        │  Block Action        │
                        │  + Alert Security    │
                        │  + Log Denial        │
                        └─────────────────────┘
```

### Flow 3: MCP Runtime Verification

**Same pattern but for MCP servers accessing services:**

```
MCP Server attempts to access service (e.g., database query)
           │
           ▼
┌─────────────────────────────────────────────────────┐
│   POST /api/v1/mcp-servers/:id/verify-action        │
│   {                                                   │
│     "mcp_id": "mcp_789",                             │
│     "action_type": "database_query",                 │
│     "resource": "SELECT * FROM customers",           │
│     "target_service": "postgresql://prod-db",        │
│     "metadata": {                                    │
│       "user_id": "user_456",                         │
│       "query_type": "SELECT"                         │
│     }                                                │
│   }                                                   │
└──────────────────┬──────────────────────────────────┘
                   │
                   ▼
┌─────────────────────────────────────────────────────┐
│          AIM MCP Verification Engine                 │
├─────────────────────────────────────────────────────┤
│  1. Fetch MCP capabilities:                          │
│     • Allowed databases: [prod-db, staging-db]      │
│     • Allowed query types: [SELECT only]            │
│     • Forbidden tables: [employees, payroll]        │
│     • Max result rows: 1000                          │
│  2. Parse SQL query and validate:                   │
│     ✓ Query type is SELECT (allowed)                │
│     ✓ Table 'customers' not in forbidden list       │
│     ✓ No subqueries or JOINs to forbidden tables    │
│  3. Check anomalies:                                 │
│     • Is this MCP suddenly querying new tables?     │
│     • Query pattern changed?                         │
│  4. ALLOW or DENY                                    │
│  5. Audit log with full query                        │
└─────────────────────────────────────────────────────┘
```

---

## Implementation

### Required Endpoints (Runtime Verification)

#### Agent Runtime Verification
```go
// POST /api/v1/agents/:id/verify-action
// Verify if agent can perform requested action
func (h *AgentHandler) VerifyAction(c *fiber.Ctx) error {
    request := VerifyActionRequest{
        AgentID:    c.Params("id"),
        ActionType: "read_file" | "write_file" | "execute_code" | "network_request" | "database_query",
        Resource:   "/path/to/resource",
        Metadata:   map[string]interface{}{},
    }

    // 1. Fetch agent capabilities
    agent := agentRepo.GetByID(request.AgentID)

    // 2. Verify action against capabilities
    decision := verifyActionAgainstCapabilities(agent.Capabilities, request)

    // 3. Anomaly detection
    isAnomaly := detectAnomaly(agent.ID, request)

    // 4. Audit log
    auditLog.LogVerification(agent.ID, request, decision, isAnomaly)

    // 5. Return decision
    return c.JSON(VerificationResponse{
        Allowed: decision == ALLOW,
        Reason:  "Action matches registered capabilities",
        AuditID: "audit_123",
    })
}
```

#### MCP Runtime Verification
```go
// POST /api/v1/mcp-servers/:id/verify-action
// Verify if MCP can perform requested action
func (h *MCPHandler) VerifyAction(c *fiber.Ctx) error {
    request := VerifyMCPActionRequest{
        MCPID:         c.Params("id"),
        ActionType:    "database_query" | "api_call" | "file_access",
        Resource:      "SELECT * FROM table",
        TargetService: "postgresql://prod-db",
        Metadata:      map[string]interface{}{},
    }

    // 1. Fetch MCP capabilities
    mcp := mcpRepo.GetByID(request.MCPID)

    // 2. Parse and validate resource (e.g., SQL query)
    parsed := parseResource(request.ActionType, request.Resource)

    // 3. Verify against capabilities
    decision := verifyMCPAction(mcp.Capabilities, parsed)

    // 4. Anomaly detection
    isAnomaly := detectMCPAnomaly(mcp.ID, request)

    // 5. Audit log
    auditLog.LogMCPVerification(mcp.ID, request, decision, isAnomaly)

    return c.JSON(VerificationResponse{
        Allowed: decision == ALLOW,
        Reason:  "MCP authorized for this database query",
    })
}
```

### Capability Definition Schema

```go
type AgentCapabilities struct {
    // File operations
    CanReadFiles     bool     `json:"can_read_files"`
    CanWriteFiles    bool     `json:"can_write_files"`
    AllowedPaths     []string `json:"allowed_paths"`      // Glob patterns: ["/data/reports/*"]
    ForbiddenPaths   []string `json:"forbidden_paths"`    // ["/etc/*", "/root/*"]
    MaxFileSize      int64    `json:"max_file_size"`      // Bytes

    // Code execution
    CanExecuteCode   bool     `json:"can_execute_code"`
    AllowedLanguages []string `json:"allowed_languages"`  // ["python", "javascript"]

    // Network access
    CanAccessNetwork bool     `json:"can_access_network"`
    AllowedDomains   []string `json:"allowed_domains"`    // ["api.company.com"]
    ForbiddenDomains []string `json:"forbidden_domains"`  // ["*.external.com"]

    // Database access
    CanQueryDatabase    bool     `json:"can_query_database"`
    AllowedDatabases    []string `json:"allowed_databases"`    // ["analytics_db"]
    AllowedQueryTypes   []string `json:"allowed_query_types"`  // ["SELECT"]
    ForbiddenTables     []string `json:"forbidden_tables"`     // ["employees", "payroll"]
    MaxResultRows       int      `json:"max_result_rows"`      // 1000

    // Rate limits
    MaxActionsPerMinute int      `json:"max_actions_per_minute"` // 100
    MaxActionsPerHour   int      `json:"max_actions_per_hour"`   // 1000

    // Business hours restriction
    AllowedHours        []string `json:"allowed_hours"`          // ["9:00-17:00"]
    AllowedDaysOfWeek   []string `json:"allowed_days_of_week"`   // ["Mon", "Tue", "Wed", "Thu", "Fri"]
}

type MCPCapabilities struct {
    // Similar to AgentCapabilities but MCP-specific
    AllowedServices     []string `json:"allowed_services"`      // ["postgresql://prod-db"]
    AllowedOperations   []string `json:"allowed_operations"`    // ["read", "query"]
    ForbiddenOperations []string `json:"forbidden_operations"`  // ["delete", "drop"]
    // ... etc
}
```

---

## Consequences

### Positive

1. **Enterprise Trust in AI**:
   - Organizations can confidently deploy AI knowing all actions are verified
   - Complete audit trail of every AI/MCP action
   - Automatic capability enforcement (no human errors)

2. **Security & Compliance**:
   - Detect phishing attempts (agent acting outside scope)
   - Prevent data exfiltration (block unauthorized file access)
   - Meet SOC 2, HIPAA, GDPR requirements
   - Real-time anomaly detection

3. **Visibility & Control**:
   - Security teams see all AI/MCP activities in real-time
   - Can instantly revoke agent access if compromised
   - Trend analysis to detect capability drift

4. **Developer Experience**:
   - Simple SDK integration: `aim.verify_action(agent_id, "read_file", "/data/file.csv")`
   - Clear error messages when action blocked
   - Automatic rate limiting

### Negative

1. **Performance Overhead**:
   - Every action requires API call to AIM (adds ~10-50ms latency)
   - **Mitigation**: Cache capability checks, edge deployment, async verification for low-risk actions

2. **Single Point of Failure**:
   - If AIM is down, all agents/MCPs blocked
   - **Mitigation**: High availability (99.99% uptime), circuit breaker pattern, fail-open for trusted agents

3. **Capability Management Complexity**:
   - Defining granular capabilities requires thought
   - **Mitigation**: Template library, capability wizard UI, sane defaults

### Mitigation Strategies

1. **Performance Optimization**:
   ```go
   // Cache capability checks (30s TTL)
   cacheKey := fmt.Sprintf("capability:%s:%s", agentID, actionType)
   if cached := cache.Get(cacheKey); cached != nil {
       return cached
   }

   // Edge deployment (AIM deployed close to workloads)
   // Async verification for low-risk actions
   ```

2. **High Availability**:
   ```yaml
   # Kubernetes deployment
   replicas: 3
   affinity: antiAffinity  # Different zones
   livenessProbe: /health/ready
   ```

3. **Fail-Safe Modes**:
   ```go
   // Circuit breaker: If AIM unhealthy, fail open for trusted agents
   if isAIMUnhealthy() && agent.TrustScore > 90 {
       auditLog.LogFailOpen(agent.ID, action)
       return ALLOW
   }
   ```

---

## Alternatives Considered

### 1. Policy-Based Access Control (PBAC) Only
**Rejected because**: Static policies don't detect anomalies or runtime behavior changes.

### 2. Agent Self-Reporting (Honor System)
**Rejected because**: Agents could lie about their actions. No verification.

### 3. Manual Approval for Each Action
**Rejected because**: Too slow, doesn't scale. Enterprises need real-time verification.

---

## Integration Example

### Python Agent SDK

```python
from aim_sdk import AIMClient

aim = AIMClient(api_key="aim_sk_...")

def read_file(path: str):
    # BEFORE reading file, verify with AIM
    decision = aim.verify_action(
        agent_id="agent_123",
        action_type="read_file",
        resource=path,
        metadata={"user_id": "user_456"}
    )

    if not decision.allowed:
        raise PermissionError(f"AIM blocked action: {decision.reason}")

    # Action approved, proceed
    with open(path, 'r') as f:
        content = f.read()

    # Report success back to AIM
    aim.log_action_success(decision.audit_id)

    return content
```

### TypeScript MCP SDK

```typescript
import { AIMClient } from '@aim/sdk';

const aim = new AIMClient({ apiKey: 'aim_sk_...' });

async function queryDatabase(sql: string) {
    // Verify with AIM before executing query
    const decision = await aim.verifyMCPAction({
        mcpId: 'mcp_789',
        actionType: 'database_query',
        resource: sql,
        targetService: 'postgresql://prod-db',
    });

    if (!decision.allowed) {
        throw new Error(`AIM blocked query: ${decision.reason}`);
    }

    // Execute query
    const result = await db.query(sql);

    // Report success
    await aim.logActionSuccess(decision.auditId);

    return result;
}
```

---

## Success Metrics

1. **Verification Latency**: <50ms p99
2. **False Positives**: <1% (legitimate actions blocked)
3. **Anomaly Detection Accuracy**: >95% (catch suspicious behavior)
4. **Audit Trail Coverage**: 100% (every action logged)
5. **Developer Adoption**: 80% of agents use AIM verification

---

## References

- [NIST Zero Trust Architecture](https://www.nist.gov/publications/zero-trust-architecture)
- [OAuth2 Scopes Best Practices](https://oauth.net/2/scope/)
- [Google BeyondCorp](https://cloud.google.com/beyondcorp)

---

**Last Updated**: October 6, 2025
**Related ADRs**: ADR-002 (Clean Architecture), ADR-004 (Trust Scoring), ADR-005 (Authentication)
