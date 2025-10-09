# 🚨 CRITICAL: User Identity Mapping Gap in AIM

**Date**: October 8, 2025
**Issue**: Agent/MCP registrations are not properly mapped to actual users
**Impact**: HIGH - Affects traceability, security, compliance, and user experience
**Status**: ⚠️ **NEEDS IMMEDIATE ATTENTION**

---

## 🔍 Current State Analysis

### ✅ What's Already in Place

1. **Database Schema** - `CreatedBy` field exists:
   ```go
   // Agent struct (apps/backend/internal/domain/agent.go:50)
   CreatedBy uuid.UUID `json:"created_by"`

   // MCPServer struct (apps/backend/internal/domain/mcp_server.go:37)
   CreatedBy uuid.UUID `json:"created_by"`
   ```

2. **Users Table** - SSO authentication infrastructure:
   ```go
   // User struct with SSO support
   Provider      string  // google, microsoft, okta, local
   ProviderID    string  // OAuth provider user ID
   Email         string
   OrganizationID uuid.UUID
   Role          UserRole // admin, manager, member, viewer
   ```

3. **Organization Model** - Multi-tenancy support exists

### ❌ What's Missing (CRITICAL GAPS)

#### **Gap 1: SDK Registration Uses Hardcoded User ID**

**File**: `apps/backend/internal/interfaces/http/handlers/public_agent_handler.go`

```go
// Lines 93-94 - HARDCODED VALUES
defaultOrgID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
defaultUserID := uuid.MustParse("7661f186-1de3-4898-bcbd-11bc9490ece7")

// Line 105 - Used for all SDK registrations
}, defaultOrgID, defaultUserID)
```

**Problem**:
- ALL agents registered via SDK are assigned to the same fake user
- No way to know which developer created which agent
- Breaks traceability, security, and compliance requirements

---

#### **Gap 2: No API Key → User Association**

**Current Flow**:
```
Developer → Gets API Key → Registers Agent via SDK
                ↓
         WHERE IS USER ID?
```

**Problem**:
- API keys exist but don't carry user identity
- SDK doesn't send user credentials
- Backend doesn't validate user identity during SDK registration

---

#### **Gap 3: No Dashboard for Developers to View "My Agents"**

**Current State**:
- Admin page shows ALL agents in organization
- No user-specific filtering
- No "My Agents" dashboard for developers

**What's Needed**:
- `/dashboard/my-agents` page showing only agents created by logged-in user
- `/dashboard/my-mcp-servers` page for user's MCP servers
- Filter on admin page: "Show only my agents" toggle

---

#### **Gap 4: Frontend Registration Doesn't Set CreatedBy**

**Current authenticated agent registration** (if it exists):
- Frontend creates agent via authenticated API
- Backend might not be setting `CreatedBy` from JWT token

---

## 🎯 Your Questions Answered

### Q1: "Are we tracking user identity for agent/MCP registration?"

**Answer**: ❌ **NO - Currently using hardcoded default user ID**

**Evidence**: `public_agent_handler.go:93-94`

---

### Q2: "Does SSO integration require IT to rework their existing SSO?"

**Answer**: ✅ **NO - AIM's SSO is a bonus, not a replacement**

**How OAuth SSO Works** (No IT rework needed):

```
┌─────────────────────────────────────────────────────────┐
│ Option 1: AIM as Standalone (Most Common)              │
├─────────────────────────────────────────────────────────┤
│ Developer → Google OAuth → AIM Dashboard → Uses AIM    │
│                                                          │
│ No integration with company SSO needed!                 │
│ AIM has its own user database.                          │
└─────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────┐
│ Option 2: Enterprise Integration (Optional)             │
├─────────────────────────────────────────────────────────┤
│ Company SSO (Okta) → SAML/OIDC → AIM Dashboard         │
│                                                          │
│ Requires company to add AIM as an app in their IdP.    │
│ Common for large enterprises with strict compliance.    │
└─────────────────────────────────────────────────────────┘
```

**Your Plan is Correct**:
- Offer Google, Microsoft, Okta OAuth (Option 1) ✅
- Users sign in with their work email
- AIM maintains its own user database
- **No IT rework required** - just add users to AIM

**Optional**: For enterprises wanting SAML integration, they can configure it later.

---

### Q3: "Can developers see their own agents/MCP in the dashboard?"

**Answer**: ⚠️ **PARTIALLY - No user-specific filtering yet**

**Current State**:
- Admins can see ALL agents/MCP in their organization
- No way to filter by "Created By Me"
- No dedicated "My Agents" page for developers

**What Users Want**:
1. Login to AIM dashboard (SSO)
2. See dashboard showing ONLY their agents/MCP
3. Register new agents/MCP via frontend
4. View verification events for their agents
5. Manage API keys for their agents

---

## 🛠️ Solution Architecture

### **Proposed Flow: SDK Registration with User Identity**

```
┌──────────────────────────────────────────────────────────────┐
│ Step 1: Developer Gets API Key                               │
├──────────────────────────────────────────────────────────────┤
│ 1. Developer logs in to AIM dashboard (OAuth SSO)            │
│ 2. Goes to /dashboard/api-keys                               │
│ 3. Clicks "Generate API Key"                                 │
│ 4. Backend creates API key linked to user:                   │
│    - api_keys.user_id = current_user.id                      │
│    - api_keys.organization_id = current_user.organization_id │
│ 5. Developer copies API key (e.g., "aim_xxxxxxxxxxxx")       │
└──────────────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────────────┐
│ Step 2: SDK Registration with API Key                        │
├──────────────────────────────────────────────────────────────┤
│ Developer code:                                              │
│   from aim_sdk import register_agent                         │
│                                                              │
│   agent = register_agent(                                    │
│       name="my-agent",                                       │
│       api_key="aim_xxxxxxxxxxxx",  # <-- USER IDENTITY      │
│       aim_url="https://aim.company.com"                      │
│   )                                                          │
└──────────────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────────────┐
│ Step 3: Backend Validates API Key & Extracts User           │
├──────────────────────────────────────────────────────────────┤
│ Backend (public_agent_handler.go):                          │
│   1. Receive API key from SDK request header                │
│   2. Validate API key:                                       │
│      - Hash API key (SHA-256)                                │
│      - Look up in api_keys table                             │
│      - Check if active and not expired                       │
│   3. Extract user_id from api_keys.user_id                   │
│   4. Extract organization_id from api_keys.organization_id   │
│   5. Create agent with REAL user:                            │
│      - agent.created_by = user_id   ✅                       │
│      - agent.organization_id = organization_id  ✅           │
└──────────────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────────────┐
│ Step 4: Developer Views Their Agents                         │
├──────────────────────────────────────────────────────────────┤
│ Dashboard:                                                   │
│   - /dashboard/my-agents (filtered by created_by)           │
│   - Shows only agents where created_by = current_user.id    │
│   - Developer can see verification status, trust score      │
│   - Can register more agents via UI                          │
└──────────────────────────────────────────────────────────────┘
```

---

## 📋 Required Database Changes

### **1. Add `user_id` to API Keys Table** (CRITICAL)

```sql
-- Migration: Add user_id to api_keys table
ALTER TABLE api_keys ADD COLUMN user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE api_keys ADD COLUMN organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE;

-- Index for fast lookups
CREATE INDEX idx_api_keys_user_id ON api_keys(user_id);
CREATE INDEX idx_api_keys_organization_id ON api_keys(organization_id);
```

**Why**: API keys must be linked to the user who created them.

---

### **2. Ensure `created_by` is NOT NULL** (Enforce Traceability)

```sql
-- Migration: Make created_by required (if not already)
ALTER TABLE agents ALTER COLUMN created_by SET NOT NULL;
ALTER TABLE mcp_servers ALTER COLUMN created_by SET NOT NULL;

-- Add index for filtering "My Agents"
CREATE INDEX idx_agents_created_by ON agents(created_by);
CREATE INDEX idx_mcp_servers_created_by ON mcp_servers(created_by);
```

**Why**: Every agent MUST have an owner for compliance and security.

---

## 🔧 Required Code Changes

### **Change 1: Update PublicRegisterRequest to Accept API Key**

```go
// File: apps/backend/internal/interfaces/http/handlers/public_agent_handler.go

type PublicRegisterRequest struct {
	Name                string           `json:"name" validate:"required"`
	DisplayName         string           `json:"display_name" validate:"required"`
	Description         string           `json:"description" validate:"required"`
	AgentType           domain.AgentType `json:"agent_type" validate:"required"`
	Version             string           `json:"version"`
	RepositoryURL       string           `json:"repository_url"`
	DocumentationURL    string           `json:"documentation_url"`
	// REMOVED: OrganizationDomain, UserEmail (use API key instead)
}

// Update Register handler
func (h *PublicAgentHandler) Register(c fiber.Ctx) error {
	// 1. Extract API key from header
	apiKey := c.Get("X-AIM-API-Key")
	if apiKey == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "API key required (X-AIM-API-Key header)",
		})
	}

	// 2. Validate API key and get user + organization
	user, org, err := h.authService.ValidateAPIKey(c.Context(), apiKey)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid or expired API key",
		})
	}

	// 3. Create agent with REAL user identity
	agent, err := h.agentService.CreateAgent(c.Context(), &application.CreateAgentRequest{
		Name:             req.Name,
		DisplayName:      req.DisplayName,
		Description:      req.Description,
		AgentType:        req.AgentType,
		Version:          req.Version,
		RepositoryURL:    req.RepositoryURL,
		DocumentationURL: req.DocumentationURL,
	}, org.ID, user.ID) // ✅ Use REAL user and org

	// ... rest of handler
}
```

---

### **Change 2: Add `ValidateAPIKey` Method to AuthService**

```go
// File: apps/backend/internal/application/auth_service.go

func (s *AuthService) ValidateAPIKey(ctx context.Context, apiKey string) (*domain.User, *domain.Organization, error) {
	// 1. Hash API key (SHA-256)
	hashedKey := hashAPIKey(apiKey)

	// 2. Look up in database
	key, err := s.apiKeyRepo.GetByHash(hashedKey)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid API key")
	}

	// 3. Check if active and not expired
	if !key.IsActive {
		return nil, nil, fmt.Errorf("API key is inactive")
	}
	if key.ExpiresAt != nil && time.Now().After(*key.ExpiresAt) {
		return nil, nil, fmt.Errorf("API key expired")
	}

	// 4. Get user
	user, err := s.userRepo.GetByID(key.UserID)
	if err != nil {
		return nil, nil, fmt.Errorf("user not found")
	}

	// 5. Get organization
	org, err := s.orgRepo.GetByID(key.OrganizationID)
	if err != nil {
		return nil, nil, fmt.Errorf("organization not found")
	}

	// 6. Update last_used_at
	_ = s.apiKeyRepo.UpdateLastUsed(key.ID, time.Now())

	return user, org, nil
}
```

---

### **Change 3: Update SDK to Send API Key**

```python
# File: sdks/python/aim_sdk/client.py

def register_agent(
    name: str,
    api_key: str,  # ✅ NEW: Required API key
    aim_url: str = "http://localhost:8080",
    **kwargs
):
    """Register agent with user identity via API key."""

    headers = {
        "Content-Type": "application/json",
        "X-AIM-API-Key": api_key  # ✅ Send API key in header
    }

    response = requests.post(
        f"{aim_url}/api/v1/public/agents/register",
        json={
            "name": name,
            "display_name": kwargs.get("display_name", name),
            "description": kwargs.get("description", ""),
            "agent_type": kwargs.get("agent_type", "ai_agent"),
            "version": kwargs.get("version", "1.0.0"),
            "repository_url": kwargs.get("repository_url"),
            "documentation_url": kwargs.get("documentation_url"),
        },
        headers=headers
    )

    # ... rest of function
```

---

### **Change 4: Add "My Agents" Dashboard**

```typescript
// File: apps/web/app/dashboard/my-agents/page.tsx

'use client'

export default function MyAgentsPage() {
  const [myAgents, setMyAgents] = useState<Agent[]>([])

  useEffect(() => {
    fetchMyAgents()
  }, [])

  const fetchMyAgents = async () => {
    // New endpoint: GET /api/v1/users/me/agents
    const data = await api.getMyAgents()
    setMyAgents(data)
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">My Agents</h1>
        <p className="text-muted-foreground mt-1">
          Agents and MCP servers you've registered
        </p>
      </div>

      {/* Show only agents where created_by = current_user.id */}
      <AgentList agents={myAgents} />
    </div>
  )
}
```

**Backend Endpoint**:
```go
// GET /api/v1/users/me/agents
func (h *AgentHandler) GetMyAgents(c fiber.Ctx) error {
	userID := c.Locals("user_id").(uuid.UUID)

	agents, err := h.agentRepo.GetByCreator(userID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"agents": agents})
}
```

---

## 🎯 Implementation Priority

### **Phase 1: Critical Foundation** (Week 1 - ASAP)

1. ✅ Add `user_id` and `organization_id` to `api_keys` table
2. ✅ Add `ValidateAPIKey()` method to AuthService
3. ✅ Update `PublicAgentHandler.Register()` to use API key for user identity
4. ✅ Update SDK to send `X-AIM-API-Key` header
5. ✅ Test: Register agent via SDK and verify `created_by` is set correctly

**Deliverable**: SDK registrations now have proper user identity tracking

---

### **Phase 2: Developer Dashboard** (Week 2)

1. ✅ Add `GET /api/v1/users/me/agents` endpoint
2. ✅ Add `GET /api/v1/users/me/mcp-servers` endpoint
3. ✅ Create `/dashboard/my-agents` page
4. ✅ Create `/dashboard/my-mcp-servers` page
5. ✅ Add "Show only my agents" filter toggle on admin pages

**Deliverable**: Developers can view and manage their own agents

---

### **Phase 3: Compliance & Audit** (Week 3)

1. ✅ Add audit logging for all agent operations (created_by tracked)
2. ✅ Add "Owned By" column to admin agent list
3. ✅ Add user search/filter on admin pages
4. ✅ Generate compliance reports: "Who owns which agents?"

**Deliverable**: Full traceability and compliance reporting

---

## 📊 Security & Compliance Benefits

### **With User Identity Mapping**:

| Requirement | Before (Hardcoded User) | After (Real User Identity) |
|-------------|-------------------------|----------------------------|
| **Traceability** | ❌ All agents owned by fake user | ✅ Know exactly who created each agent |
| **Security** | ❌ Can't revoke access per user | ✅ Revoke user → revoke their agents |
| **Compliance (SOC 2)** | ❌ Fails audit (no user tracking) | ✅ Passes audit (full trail) |
| **Incident Response** | ❌ "Who created this agent?" → Unknown | ✅ Instant identification |
| **User Experience** | ❌ Users see all org agents | ✅ Users see only their agents |
| **API Key Management** | ❌ API keys not linked to users | ✅ API keys → Users → Agents |

---

## 🚨 Risks of NOT Implementing This

1. **Security Incident**: Can't identify which developer created a compromised agent
2. **Compliance Failure**: SOC 2, HIPAA, GDPR all require user traceability
3. **User Confusion**: Developers see ALL agents, not just theirs
4. **No Accountability**: Can't attribute actions to specific users
5. **Investment Risk**: VCs will ask "Who owns these agents?" and you can't answer

---

## ✅ Recommended Action Plan

### **Immediate Next Steps**:

1. **Create Database Migration** for `api_keys.user_id`
2. **Implement `ValidateAPIKey()` in AuthService**
3. **Update PublicAgentHandler** to use API key for user identity
4. **Update Python SDK** to require and send API key
5. **Test End-to-End**: SDK registration → Verify `created_by` is real user

### **Timeline**:
- **Phase 1** (Critical): 2-3 days
- **Phase 2** (Dashboard): 3-5 days
- **Phase 3** (Compliance): 2-3 days

**Total**: ~2 weeks to full implementation

---

## 📚 Related Documentation

- **SSO Integration**: No IT rework needed - AIM is standalone with OAuth
- **API Key Security**: Hash API keys (SHA-256) before storage
- **User Roles**: Developers can register agents; admins see all agents
- **Compliance**: User identity tracking is mandatory for SOC 2, HIPAA

---

## 🎉 Expected Outcome

**After Implementation**:

1. ✅ Developer logs in to AIM (Google/Microsoft/Okta OAuth)
2. ✅ Generates API key from dashboard
3. ✅ Uses SDK to register agent (API key sends user identity)
4. ✅ Agent is created with `created_by = developer.id`
5. ✅ Developer sees their agents on `/dashboard/my-agents`
6. ✅ Admin can see all agents with "Owned By" column
7. ✅ Full audit trail for compliance

**Investor Pitch**:
> "AIM tracks every agent registration to the exact developer who created it. Full traceability, security, and compliance built-in. Enterprise-ready from day one."

---

**Last Updated**: October 8, 2025
**Author**: Claude Code Analysis
**Priority**: 🚨 **CRITICAL - Implement Phase 1 ASAP**
**Status**: ⚠️ Awaiting Approval to Implement

---
