# Talks To & Capabilities Feature - Implementation Complete

**Date**: October 8, 2025
**Status**: ✅ Phase 1 Complete - Manual Declaration Implemented
**Next Phase**: WHO/WHAT Verification Logic

---

## 🎯 What Was Accomplished

### 1. Backend Implementation (Go)

#### **Agent Domain Model** (`internal/domain/agent.go`)
- Added `TalksTo []string` field to Agent struct
- Field stores list of MCP server IDs/names the agent can communicate with
- Example: `["filesystem-mcp", "github-mcp", "database-mcp"]`

#### **Agent Repository** (`internal/infrastructure/repository/agent_repository.go`)
- Implemented JSONB marshaling/unmarshaling for `talks_to` field
- Updated all repository methods:
  - ✅ `Create()` - Marshal talks_to to JSONB before insert
  - ✅ `GetByID()` - Unmarshal talks_to from JSONB
  - ✅ `GetByOrganization()` - Include talks_to in results
  - ✅ `List()` - Include talks_to in pagination
  - ✅ `Update()` - Marshal and update talks_to

#### **API Response**
```json
{
  "id": "899ca61d-b05f-49ce-b43e-22a73ab717e4",
  "name": "test-agent",
  "talks_to": null  // or ["filesystem-mcp", "github-mcp"]
}
```

### 2. Python SDK Enhancement

#### **Updated `register_agent()` Function**
```python
from aim_sdk import register_agent

# Register agent with WHO and WHAT
agent = register_agent(
    "my-agent",
    talks_to=["filesystem-mcp", "github-mcp", "database-mcp"],
    capabilities=["read_files", "create_pull_requests", "query_database"]
)
```

**New Parameters**:
- `talks_to`: List of MCP server IDs/names (WHO the agent talks to)
- `capabilities`: List of capability strings (WHAT the agent can do)

### 3. Frontend UI (Next.js + TypeScript)

#### **Agent Interface** (`apps/web/lib/api.ts`)
```typescript
export interface Agent {
  // ... existing fields
  talks_to?: string[]
}
```

#### **Agent Detail Modal** (`components/modals/agent-detail-modal.tsx`)
- Added "Talks To (MCP Servers)" section
- Displays purple badges for each MCP server
- Shows "No MCP servers configured" when empty
- Clean, professional UI matching AIVF design

**Screenshot**: Successfully tested and verified in browser ✅

---

## 📊 Database Schema

The `agents` table already has the `talks_to` column:

```sql
talks_to | jsonb | | | '[]'::jsonb
```

With GIN index:
```sql
"idx_agents_talks_to" gin (talks_to)
```

---

## 🔐 Next Phase: WHO/WHAT Verification Logic

### User's Requirement:
> "During verifications, WHO and WHAT should also be verified. Because AIM has historic knowledge of WHO and WHAT, if those values change, that could be a potential red flag that we should alert admins about."

### Implementation Plan:

#### 1. **Verification Event Enhancement**

When an agent performs an action, compare runtime values against registered values:

```go
// During verification
type VerificationRequest struct {
    AgentID      uuid.UUID
    ActionType   string
    Resource     string
    // NEW: Runtime values from the agent's current execution
    CurrentMCPServers  []string  // WHO is this agent talking to right now?
    CurrentCapabilities []string  // WHAT is this agent claiming it can do?
}
```

#### 2. **Drift Detection**

```go
func (s *VerificationService) CheckForDrift(agentID uuid.UUID, runtimeData VerificationRequest) (*DriftAlert, error) {
    // 1. Get agent's registered talks_to and capabilities
    agent, err := s.agentRepo.GetByID(agentID)
    if err != nil {
        return nil, err
    }

    // 2. Compare runtime values vs registered values
    mcpDrift := detectDrift(agent.TalksTo, runtimeData.CurrentMCPServers)
    capabilityDrift := detectDrift(agent.Capabilities, runtimeData.CurrentCapabilities)

    // 3. If drift detected, create HIGH severity alert
    if len(mcpDrift) > 0 || len(capabilityDrift) > 0 {
        alert := &Alert{
            Severity: "high",
            Type: "configuration_drift",
            Title: "Agent Configuration Drift Detected",
            Message: fmt.Sprintf(
                "Agent %s is communicating with unregistered MCP servers or using undeclared capabilities",
                agent.Name,
            ),
            Metadata: map[string]interface{}{
                "agent_id": agentID,
                "mcp_drift": mcpDrift,
                "capability_drift": capabilityDrift,
            },
        }

        // Save alert
        s.alertRepo.Create(alert)

        return &DriftAlert{
            MCPServerDrift: mcpDrift,
            CapabilityDrift: capabilityDrift,
        }, nil
    }

    return nil, nil
}

func detectDrift(registered []string, runtime []string) []string {
    drift := []string{}
    for _, item := range runtime {
        if !contains(registered, item) {
            drift = append(drift, item)
        }
    }
    return drift
}
```

#### 3. **Alert Examples**

**Scenario 1: Unauthorized MCP Server Communication**
```
⚠️ HIGH SEVERITY ALERT
Agent: customer-support-bot
Issue: Communicating with unregistered MCP server
Details:
  - Registered: ["filesystem-mcp", "database-mcp"]
  - Runtime: ["filesystem-mcp", "database-mcp", "external-api-mcp"]
  - Drift: ["external-api-mcp"] ← Not registered!
Action: Investigate why agent is calling external-api-mcp
```

**Scenario 2: Undeclared Capability Usage**
```
⚠️ HIGH SEVERITY ALERT
Agent: data-analyzer-bot
Issue: Using undeclared capabilities
Details:
  - Registered: ["read_database", "analyze_data"]
  - Runtime: ["read_database", "analyze_data", "write_database"]
  - Drift: ["write_database"] ← Not declared!
Action: Agent is attempting write operations when only read was authorized
```

#### 4. **Admin Dashboard Alert**

The alert would show up in `/dashboard/alerts` with:
- **Severity**: HIGH (red)
- **Type**: Configuration Drift
- **Agent**: Link to agent detail
- **Changes Detected**:
  - New MCP servers: `external-api-mcp`
  - New capabilities: `write_database`
- **Actions**:
  - Approve & Update Registration
  - Investigate & Block
  - View Audit Trail

---

## 🚀 Benefits

### 1. **Zero-Friction Developer Experience**
Developers manually declare WHO and WHAT during registration:
```python
agent = register_agent(
    "my-bot",
    talks_to=["filesystem", "github"],
    capabilities=["read", "write"]
)
```

### 2. **Automatic Drift Detection**
AIM monitors runtime behavior and alerts admins when:
- Agent talks to MCP servers not in `talks_to`
- Agent uses capabilities not declared

### 3. **Security & Compliance**
- **Prevents Privilege Escalation**: Agent can't silently gain new capabilities
- **Detects Compromised Agents**: Unusual MCP communication patterns trigger alerts
- **Audit Trail**: Full history of who talked to what and when

### 4. **Trust Score Impact**
Configuration drift should reduce trust score:
- First violation: -5 points + warning
- Repeated violations: -10 points + suspension
- Approved drift: Update registration, restore trust score

---

## 📝 Implementation Checklist

### ✅ Completed (Phase 1)
- [x] Add `talks_to` field to Agent domain model
- [x] Implement JSONB marshaling in repository
- [x] Update all repository methods (Create, Get, Update, List)
- [x] Add `talks_to` parameter to Python SDK
- [x] Add `capabilities` parameter to Python SDK
- [x] Display `talks_to` in agent detail modal UI
- [x] Test end-to-end functionality

### ⏳ TODO (Phase 2 - Verification Logic)
- [ ] Add `current_mcp_servers` to VerificationRequest
- [ ] Add `current_capabilities` to VerificationRequest
- [ ] Implement drift detection function
- [ ] Create alerts for configuration drift
- [ ] Add "Approve Drift" action in admin UI
- [ ] Update agent registration when drift is approved
- [ ] Impact trust score based on drift severity
- [ ] Add drift metrics to security dashboard

---

## 🧪 Testing Scenarios

### Test 1: Normal Operation (No Drift)
```python
# Registration
agent = register_agent("test-bot", talks_to=["filesystem"])

# Runtime verification
agent.verify_action(
    action_type="read_file",
    current_mcp_servers=["filesystem"]  # Matches registration ✅
)
# Result: No alert, verification proceeds normally
```

### Test 2: Drift Detection
```python
# Registration
agent = register_agent("test-bot", talks_to=["filesystem"])

# Runtime verification
agent.verify_action(
    action_type="read_file",
    current_mcp_servers=["filesystem", "github"]  # NEW: github not registered!
)
# Result: ⚠️ HIGH alert created
# Admin sees: "Agent test-bot is communicating with unregistered MCP server: github"
```

### Test 3: Approved Drift
```python
# Admin approves drift and updates registration
# talks_to updated to: ["filesystem", "github"]

# Next verification
agent.verify_action(
    action_type="read_file",
    current_mcp_servers=["filesystem", "github"]
)
# Result: ✅ No alert, drift now approved and registered
```

---

## 💡 Future Enhancements (Auto-Capture)

For a future release, we could add automatic detection:

```python
# SDK automatically captures MCP calls
@auto_capture_mcp  # Decorator detects MCP server calls
def process_data():
    filesystem.read("data.txt")  # Auto-detected: "filesystem"
    github.create_pr("feature")  # Auto-detected: "github"
    # SDK automatically updates talks_to: ["filesystem", "github"]
```

But for now, manual declaration keeps things simple and gives developers explicit control.

---

## 📊 Metrics to Track

1. **Drift Detection Rate**: % of agents with configuration drift
2. **Time to Approve Drift**: How long admins take to review
3. **False Positive Rate**: Drift alerts that were legitimate
4. **Security Incidents**: Drift that indicated actual compromise

---

## 🎉 Summary

We've successfully implemented the foundation for WHO/WHAT tracking:
- ✅ Backend stores `talks_to` and `capabilities`
- ✅ SDK accepts manual declarations
- ✅ UI displays the information

**Next step**: Implement drift detection during verification events to create security alerts when agents deviate from their registered configuration.

This provides a powerful security layer that helps detect:
- Compromised agents
- Privilege escalation attempts
- Unauthorized MCP server communication
- Configuration changes that should be reviewed

---

**Commits**:
- `fbc8daa` - Add talks_to and capabilities support to agent registration (backend + SDK)
- `dd4e7e2` - Display talks_to in agent detail modal UI (frontend)
