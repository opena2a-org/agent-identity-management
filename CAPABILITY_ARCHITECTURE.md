# 🔐 Capability System Architecture

**Last Updated**: October 11, 2025
**Status**: Implemented (Enterprise Ready)

## Overview

AIM uses a **two-phase capability system** for enterprise-grade security:

1. **Declaration Phase**: Agent declares needed capabilities during registration
2. **Grant Phase**: Admin explicitly grants capabilities after review

This architecture prevents attacks like **CVE-2025-32711 (EchoLeak)** by ensuring admins control what agents can actually do.

---

## 📊 System Components

### 1. **Declared Capabilities** (`agent.capabilities` array)

**Purpose**: Agent's requested/declared capabilities (reference only)

**Database**: `agents` table → `capabilities` JSONB field
**API Field**: `Agent.Capabilities []string`

**Lifecycle**:
- Set during agent registration (`POST /api/v1/agents`)
- Updated when agent requirements change
- **NOT used for enforcement** - display/reference only

**Example**:
```json
{
  "capabilities": ["read_email", "fetch_external_url", "send_email"]
}
```

**Use Cases**:
- Show what agent needs in UI
- Admins review before granting
- Documentation/audit trail

---

### 2. **Granted Capabilities** (`agent_capabilities` table)

**Purpose**: Capabilities explicitly granted by admins (**ENFORCEMENT SOURCE OF TRUTH**)

**Database**: `agent_capabilities` table with full audit trail

**Schema**:
```sql
CREATE TABLE agent_capabilities (
    id UUID PRIMARY KEY,
    agent_id UUID NOT NULL REFERENCES agents(id),
    capability_type VARCHAR(255) NOT NULL,
    capability_scope JSONB,
    granted_by UUID REFERENCES users(id),
    granted_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
```

**Fields**:
- `capability_type`: e.g., "read_email", "fetch_external_url"
- `capability_scope`: Fine-grained permissions (JSON)
- `granted_by`: Which admin approved this capability
- `granted_at`: When it was granted
- `revoked_at`: NULL = active, timestamp = revoked

**API**: `POST /api/v1/agents/{id}/capabilities/grant`

**Example**:
```json
{
  "capabilityType": "read_email",
  "capabilityScope": {
    "maxEmails": 10,
    "allowedFolders": ["inbox", "sent"]
  },
  "grantedBy": "83018b76-39b0-4dea-bc1b-67c53bb03fc7",
  "grantedAt": "2025-10-11T15:30:00Z",
  "revokedAt": null
}
```

---

## 🔒 Enforcement Logic

### **CRITICAL**: Only `agent_capabilities` records are checked for enforcement

```go
// ✅ SINGLE SOURCE OF TRUTH for enforcement
activeCapabilities, err := s.capabilityRepo.GetActiveCapabilitiesByAgentID(agentID)

// Check if action matches granted capabilities
hasCapability := false
for _, capability := range activeCapabilities {
    if s.matchesCapability(actionType, resource, capability.CapabilityType) {
        hasCapability = true
        break
    }
}

if !hasCapability {
    // 🚨 CAPABILITY VIOLATION - Security policy enforcement
    return false, "Action denied: capability not granted", auditID, nil
}
```

**Key Point**: `agent.Capabilities` array is **NEVER checked** during enforcement. Only `agent_capabilities` records matter.

---

## 🎯 Workflow Example

### Scenario: Microsoft Copilot wants email access

#### Step 1: Agent Registration (Declaration + Auto-Grant)
```bash
POST /api/v1/agents
{
  "name": "microsoft-copilot",
  "capabilities": ["read_email", "send_email"]  # ✅ DECLARED
}
```

**Result**:
- Agent created with declared capabilities (stored in `agent.capabilities` for reference)
- **Capabilities automatically granted** (agent_capabilities records created)
- User can start using agent immediately (no admin approval bottleneck!)

**Auto-Grant Logic**:
```go
// During CreateAgent(), system automatically grants declared capabilities
for _, capability := range req.Capabilities {
    capabilityRecord := &domain.AgentCapability{
        AgentID:        agent.ID,
        CapabilityType: capability,
        GrantedBy:      &userID,  // Granted by user who created agent
        GrantedAt:      time.Now(),
    }
    capabilityRepo.Create(capabilityRecord)
}
```

#### Step 2: Agent Works Immediately (No Approval Needed)
```bash
POST /api/v1/agents/{copilot-id}/verify-action
{
  "actionType": "read_email",
  "resource": "inbox"
}
```

**Enforcement Check**:
1. Fetch active capabilities from `agent_capabilities` table
2. Check if "read_email" was granted (✅ auto-granted during registration)
3. ✅ ALLOWED

#### Step 3: User Wants to Add New Capability (Requires Approval)
```bash
POST /api/v1/agents/{copilot-id}/capabilities/request
{
  "capabilityType": "delete_email",
  "reason": "Need to clean up spam automatically"
}
```

**Result**: Request created, admin must approve

#### Step 4: Admin Reviews and Approves
```bash
POST /api/v1/admin/capability-requests/{request-id}/approve
```

**Result**: `agent_capabilities` record created (enforcement now allows delete_email)

#### Step 4: Agent Attempts Action
```bash
POST /api/v1/agents/{copilot-id}/verify-action
{
  "actionType": "read_email",
  "resource": "inbox"
}
```

**Enforcement Check**:
1. Fetch active capabilities from `agent_capabilities` table
2. Check if "read_email" was granted
3. ✅ ALLOWED (capability granted)

#### Step 5: Agent Attempts Unauthorized Action (EchoLeak Attack)
```bash
POST /api/v1/agents/{copilot-id}/verify-action
{
  "actionType": "fetch_external_url",  # ❌ NOT GRANTED
  "resource": "https://attacker.com"
}
```

**Enforcement Check**:
1. Fetch active capabilities from `agent_capabilities` table
2. Check if "fetch_external_url" was granted
3. ❌ DENIED (capability not granted)
4. 🚨 **Security Alert Created** (CVE-2025-32711 EchoLeak pattern detected)
5. 🛡️ **Security Policy Enforced** (action blocked)

---

## 🔄 Migration Strategy

### For Existing Agents (Backward Compatibility)

**Problem**: Existing agents have declared capabilities but no granted capabilities.

**Solution**: Auto-grant declared capabilities during migration

```sql
-- Migration: Auto-grant declared capabilities for existing agents
INSERT INTO agent_capabilities (
    id, agent_id, capability_type, granted_by, granted_at
)
SELECT
    gen_random_uuid(),
    a.id,
    jsonb_array_elements_text(a.capabilities),
    a.created_by,  -- Auto-grant by agent creator
    NOW()
FROM agents a
WHERE a.capabilities IS NOT NULL
  AND jsonb_array_length(a.capabilities) > 0
  AND NOT EXISTS (
      SELECT 1 FROM agent_capabilities ac
      WHERE ac.agent_id = a.id
      AND ac.revoked_at IS NULL
  );
```

---

## 📝 SDK Usage

### Python SDK
```python
from aim_sdk import AIMClient

# 1. Create agent (declares capabilities)
agent = client.agents.create(
    name="my-agent",
    capabilities=["read_email", "send_email"]  # DECLARED
)

# 2. Admin grants capabilities (via dashboard or API)
client.agents.grant_capability(
    agent_id=agent.id,
    capability_type="read_email",
    capability_scope={"maxEmails": 10}
)

# 3. Agent verifies actions (checks GRANTED capabilities)
allowed = client.agents.verify_action(
    agent_id=agent.id,
    action_type="read_email",
    resource="inbox"
)
```

### Node.js SDK
```javascript
const { AIMClient } = require('@opena2a/aim-sdk');

// 1. Create agent (declares capabilities)
const agent = await client.agents.create({
  name: 'my-agent',
  capabilities: ['read_email', 'send_email']  // DECLARED
});

// 2. Admin grants capabilities
await client.agents.grantCapability({
  agentId: agent.id,
  capabilityType: 'read_email',
  capabilityScope: { maxEmails: 10 }
});

// 3. Agent verifies actions
const { allowed } = await client.agents.verifyAction({
  agentId: agent.id,
  actionType: 'read_email',
  resource: 'inbox'
});
```

### Go SDK
```go
import "github.com/opena2a/aim-sdk-go"

// 1. Create agent (declares capabilities)
agent, err := client.Agents.Create(&aim.CreateAgentRequest{
    Name:         "my-agent",
    Capabilities: []string{"read_email", "send_email"},  // DECLARED
})

// 2. Admin grants capabilities
err = client.Agents.GrantCapability(agent.ID, &aim.GrantCapabilityRequest{
    CapabilityType: "read_email",
    CapabilityScope: map[string]interface{}{
        "maxEmails": 10,
    },
})

// 3. Agent verifies actions
result, err := client.Agents.VerifyAction(agent.ID, &aim.VerifyActionRequest{
    ActionType: "read_email",
    Resource:   "inbox",
})
```

---

## 🎯 Benefits

### Security
✅ **Explicit approval workflow** - admins must grant capabilities
✅ **Audit trail** - know who granted what and when
✅ **Revocation support** - revoke capabilities without deleting agent
✅ **Prevents scope creep** - agents can't self-authorize new actions

### Compliance
✅ **SOC 2 compliant** - full audit trail of capability grants
✅ **GDPR compliant** - clear access control documentation
✅ **Zero Trust** - never trust, always verify

### Developer Experience
✅ **Clear separation** - declare vs grant phases
✅ **Backward compatible** - existing agents continue working
✅ **Migration path** - auto-grant for existing capabilities

---

## ⚠️ Common Pitfalls

### ❌ WRONG: Assuming declared capabilities are enforced
```python
# Agent registers with capabilities
agent = client.agents.create(
    capabilities=["read_email"]
)

# ❌ WRONG: This will FAIL (capability not granted)
result = client.agents.verify_action(
    agent_id=agent.id,
    action_type="read_email"
)
# Error: Capability violation - "read_email" not granted
```

### ✅ CORRECT: Grant capabilities before using
```python
# Agent registers with capabilities
agent = client.agents.create(
    capabilities=["read_email"]
)

# ✅ CORRECT: Admin grants capability
client.agents.grant_capability(
    agent_id=agent.id,
    capability_type="read_email"
)

# ✅ NOW IT WORKS
result = client.agents.verify_action(
    agent_id=agent.id,
    action_type="read_email"
)
```

---

## 📚 Related Documentation

- **Security Policies**: `/docs/SECURITY_POLICIES.md`
- **EchoLeak Prevention**: `/demos/echoleak-attack-prevention/README.md`
- **API Reference**: `/docs/API.md#capability-management`
- **Trust Score Impact**: `/docs/TRUST_SCORING.md#capability-violations`

---

## 🚀 Summary

| Component | Purpose | Enforcement |
|-----------|---------|-------------|
| `agent.capabilities` array | Declared/requested capabilities | ❌ NO (reference only) |
| `agent_capabilities` table | Granted capabilities | ✅ YES (source of truth) |

**Remember**: Only granted capabilities are enforced. Declare first, grant second, enforce always.
