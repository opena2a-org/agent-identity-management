# AIM Enterprise SSO Implementation Plan

## Vision
Enable **zero-friction enterprise integration** where:
1. **Employees self-register** via Google/Microsoft/Okta SSO (no admin tickets)
2. **Admins approve access** in AIM dashboard (simple approve/reject workflow)
3. **Full observability** - admins see who runs what, where, what it talks to, what data it can share
4. **Seamless runtime verification** - agents/MCPs verify themselves automatically (every 5min/30min/1hr/8hr/24hr)

## Architecture

### Phase 1: OAuth/OIDC Backend Infrastructure
**Files to Create/Modify:**
- `apps/backend/internal/domain/oauth_provider.go` - OAuth provider models
- `apps/backend/internal/application/oauth_service.go` - OAuth business logic
- `apps/backend/internal/infrastructure/oauth/google_provider.go` - Google OAuth implementation
- `apps/backend/internal/infrastructure/oauth/microsoft_provider.go` - Microsoft OAuth implementation
- `apps/backend/internal/infrastructure/oauth/okta_provider.go` - Okta OAuth implementation
- `apps/backend/internal/interfaces/http/handlers/oauth_handler.go` - OAuth HTTP endpoints
- `apps/backend/internal/domain/user_registration.go` - User registration request model

**Database Migration:**
```sql
CREATE TABLE user_registration_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL,
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    oauth_provider VARCHAR(50) NOT NULL, -- google, microsoft, okta
    oauth_user_id VARCHAR(255) NOT NULL,
    organization_id UUID REFERENCES organizations(id),
    status VARCHAR(50) NOT NULL DEFAULT 'pending', -- pending, approved, rejected
    requested_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reviewed_at TIMESTAMPTZ,
    reviewed_by UUID REFERENCES users(id),
    rejection_reason TEXT,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(oauth_provider, oauth_user_id)
);

CREATE TABLE oauth_connections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    provider VARCHAR(50) NOT NULL, -- google, microsoft, okta
    provider_user_id VARCHAR(255) NOT NULL,
    access_token_hash VARCHAR(255), -- SHA-256 hash
    refresh_token_hash VARCHAR(255), -- SHA-256 hash
    token_expires_at TIMESTAMPTZ,
    profile_data JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(provider, provider_user_id)
);

CREATE INDEX idx_registration_requests_status ON user_registration_requests(status);
CREATE INDEX idx_registration_requests_org ON user_registration_requests(organization_id);
CREATE INDEX idx_oauth_connections_user ON oauth_connections(user_id);
```

### Phase 2: Frontend Self-Registration Flow
**Components to Create:**
- `apps/web/app/auth/register/page.tsx` - Self-registration page with SSO buttons
- `apps/web/components/auth/sso-button.tsx` - Reusable SSO button component
- `apps/web/components/auth/registration-success.tsx` - "Pending approval" success state
- `apps/web/app/admin/registrations/page.tsx` - Admin registration approval dashboard

**User Flow:**
1. User visits `/auth/register`
2. Sees options: "Sign up with Google" | "Sign up with Microsoft" | "Sign up with Okta"
3. Clicks SSO button → OAuth flow → Callback
4. Backend creates `user_registration_request` with status "pending"
5. User sees: "Registration submitted! An admin will review your request."
6. Admin notification sent (email + in-app alert)

### Phase 3: Admin Approval Dashboard
**Features:**
- List pending registration requests
- View requester's email, name, OAuth provider
- One-click approve/reject with optional reason
- Automatic user account creation on approval
- Email notification to user on approval/rejection
- Audit log of all approval decisions

**UI Design:**
```
┌─────────────────────────────────────────────────────────────┐
│ Pending Registration Requests (3)                           │
├─────────────────────────────────────────────────────────────┤
│ ✉ john.doe@company.com                                      │
│ 📅 Requested 2 hours ago via Google SSO                     │
│ 👤 John Doe | john.doe.google.12345                         │
│ [Approve] [Reject]                                          │
├─────────────────────────────────────────────────────────────┤
│ ✉ jane.smith@company.com                                    │
│ 📅 Requested 1 day ago via Microsoft SSO                    │
│ 👤 Jane Smith | jane.smith.microsoft.67890                  │
│ [Approve] [Reject]                                          │
└─────────────────────────────────────────────────────────────┘
```

### Phase 4: Admin Observability Dashboard
**New Page: `/dashboard/observability`**

Shows complete visibility:
- **Who**: Which agents/MCPs are running
- **Where**: Deployment location, network, cloud region
- **What it does**: Registered capabilities
- **Who/What it talks to**: Inter-service communication logs
- **Data sharing**: What data it can potentially access/share

**Data Model:**
```typescript
interface ServiceObservability {
  id: string;
  name: string;
  type: 'agent' | 'mcp_server';
  owner: {
    userId: string;
    email: string;
    name: string;
  };
  deployment: {
    location: string; // IP, region, cluster
    environment: string; // prod, dev, staging
    version: string;
  };
  capabilities: Array<{
    name: string;
    scope: string[];
    dataAccess: string[]; // databases, APIs accessed
  }>;
  communications: Array<{
    targetId: string;
    targetName: string;
    frequency: number; // calls per hour
    lastCommunication: string;
    dataSharingRisk: 'low' | 'medium' | 'high';
  }>;
  trustScore: number;
  verificationHistory: Array<{
    timestamp: string;
    result: 'approved' | 'denied';
    reason?: string;
  }>;
}
```

### Phase 5: Runtime Verification Settings
**New Page: `/dashboard/verification-policy`**

Admin-configurable verification frequency:
- [ ] Every 5 minutes (high security)
- [ ] Every 30 minutes (recommended)
- [x] Every 1 hour (default)
- [ ] Every 8 hours (low-frequency services)
- [ ] Every 24 hours (minimal overhead)

**Per-service override:**
- Allow admins to set different frequencies for different service types
- High-risk services: shorter intervals
- Low-risk services: longer intervals

### Phase 6: AIM SDK/Client Library
**Package: `aim-client` (npm/pip/go)**

**Supported Frameworks:**
- ✅ **LangChain** (Python & JavaScript)
- ✅ **CrewAI** (Python)
- ✅ **AutoGen** (Python)
- ✅ **Google AI SDK** (Python & JavaScript)
- ✅ **Microsoft Copilot Studio** (Power Platform connectors)
- ✅ **Microsoft Semantic Kernel** (.NET & Python)
- ✅ **OpenAI Assistants API**
- ✅ **Anthropic Claude SDK**
- ✅ **Vanilla Python/Node.js**

**Example: Basic Usage**
```typescript
// Usage in agent/MCP code
import { AIMClient } from 'aim-client';

const aim = new AIMClient({
  aimUrl: process.env.AIM_URL,
  publicKey: process.env.AGENT_PUBLIC_KEY,
  privateKey: process.env.AGENT_PRIVATE_KEY,
  verificationInterval: '30m', // auto-verify every 30 minutes
});

// Seamless integration - developers don't think about it
async function callOtherService(serviceId: string, action: string, data: any) {
  // AIM client automatically verifies before making call
  await aim.verifyAndCall(serviceId, action, data);
}

// Start automatic verification background process
aim.startAutoVerification();
```

**Example: Microsoft Copilot Studio Integration**
```typescript
// Custom connector for Copilot Studio
// File: copilot-aim-connector.ts
import { AIMClient } from 'aim-client';

export class AIMCopilotConnector {
  private aim: AIMClient;

  constructor(config: AIMConfig) {
    this.aim = new AIMClient(config);
  }

  // Power Platform action: Verify Agent
  async verifyAgent(copilotId: string, action: string): Promise<{
    verified: boolean;
    trustScore: number;
    capabilities: string[];
  }> {
    return await this.aim.verify({
      agentId: copilotId,
      action: action,
      source: 'microsoft_copilot_studio'
    });
  }

  // Power Platform action: Log Activity
  async logActivity(copilotId: string, activity: any) {
    await this.aim.logActivity({
      agentId: copilotId,
      activity: activity,
      timestamp: new Date().toISOString()
    });
  }
}
```

**Example: LangChain Integration**
```python
# aim_langchain/callbacks.py
from langchain.callbacks.base import BaseCallbackHandler
from aim_client import AIMClient

class AIMVerificationCallback(BaseCallbackHandler):
    def __init__(self, aim_client: AIMClient):
        self.aim = aim_client

    def on_tool_start(self, tool: str, **kwargs):
        # Verify before tool execution
        result = self.aim.verify_action(
            action=tool,
            scope=kwargs.get('scope', [])
        )
        if not result.approved:
            raise PermissionError(f"AIM denied tool execution: {result.reason}")

    def on_agent_action(self, action, **kwargs):
        # Log all agent actions to AIM
        self.aim.log_activity({
            'action': action.tool,
            'input': action.tool_input
        })

# Usage in LangChain
from aim_langchain import AIMVerificationCallback

aim = AIMClient(url=os.getenv('AIM_URL'), ...)
agent = initialize_agent(
    tools=tools,
    llm=llm,
    callbacks=[AIMVerificationCallback(aim)]
)
```

**Example: CrewAI Integration**
```python
# aim_crewai/integrations.py
from crewai import Agent, Task, Crew
from aim_client import AIMClient

class AIMSecuredAgent(Agent):
    def __init__(self, aim_client: AIMClient, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self.aim = aim_client
        self.aim.register_agent(
            name=kwargs.get('role'),
            capabilities=kwargs.get('tools', [])
        )

    def execute_task(self, task: Task):
        # Verify before executing
        self.aim.verify_and_execute(
            task_id=task.id,
            callback=lambda: super().execute_task(task)
        )

# Usage
aim = AIMClient(url=os.getenv('AIM_URL'), ...)
researcher = AIMSecuredAgent(
    aim_client=aim,
    role='Researcher',
    goal='Find relevant information',
    tools=[search_tool, scrape_tool]
)
```

**Features:**
- Automatic signature generation
- Token caching (avoid re-verification every call)
- Background verification scheduler
- Graceful degradation if AIM is unreachable
- Comprehensive logging for debugging
- **Framework-specific adapters** (LangChain, CrewAI, Copilot, etc.)
- **Power Platform connector** for Microsoft Copilot Studio
- **Middleware/interceptors** for popular frameworks

## Environment Variables

**Backend:**
```env
# Google OAuth
GOOGLE_CLIENT_ID=your-client-id.apps.googleusercontent.com
GOOGLE_CLIENT_SECRET=your-secret
GOOGLE_REDIRECT_URI=http://localhost:8080/api/v1/oauth/google/callback

# Microsoft OAuth
MICROSOFT_CLIENT_ID=your-app-id
MICROSOFT_CLIENT_SECRET=your-secret
MICROSOFT_TENANT_ID=common  # or specific tenant
MICROSOFT_REDIRECT_URI=http://localhost:8080/api/v1/oauth/microsoft/callback

# Okta OAuth
OKTA_DOMAIN=your-domain.okta.com
OKTA_CLIENT_ID=your-client-id
OKTA_CLIENT_SECRET=your-secret
OKTA_REDIRECT_URI=http://localhost:8080/api/v1/oauth/okta/callback

# Email notifications
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USER=noreply@your-company.com
SMTP_PASSWORD=your-smtp-password
```

**Frontend:**
```env
NEXT_PUBLIC_GOOGLE_CLIENT_ID=your-client-id.apps.googleusercontent.com
NEXT_PUBLIC_MICROSOFT_CLIENT_ID=your-app-id
NEXT_PUBLIC_OKTA_DOMAIN=your-domain.okta.com
```

## API Endpoints

### OAuth/Registration
- `GET /api/v1/oauth/google/login` - Initiate Google OAuth
- `GET /api/v1/oauth/google/callback` - Google OAuth callback
- `GET /api/v1/oauth/microsoft/login` - Initiate Microsoft OAuth
- `GET /api/v1/oauth/microsoft/callback` - Microsoft OAuth callback
- `GET /api/v1/oauth/okta/login` - Initiate Okta OAuth
- `GET /api/v1/oauth/okta/callback` - Okta OAuth callback

### Admin Registration Management
- `GET /api/v1/admin/registration-requests` - List pending requests
- `POST /api/v1/admin/registration-requests/:id/approve` - Approve request
- `POST /api/v1/admin/registration-requests/:id/reject` - Reject request

### Observability
- `GET /api/v1/admin/observability/services` - List all services with details
- `GET /api/v1/admin/observability/services/:id` - Service details
- `GET /api/v1/admin/observability/communications` - Inter-service communications
- `GET /api/v1/admin/observability/data-access` - Data access patterns

### Verification Policy
- `GET /api/v1/admin/verification-policy` - Get current policy
- `PUT /api/v1/admin/verification-policy` - Update global policy
- `PUT /api/v1/admin/verification-policy/services/:id` - Override for specific service

## Security Considerations

1. **OAuth Token Storage**: Hash all access/refresh tokens with SHA-256
2. **CSRF Protection**: Use state parameter in OAuth flows
3. **Email Verification**: Require email verification before admin approval
4. **Rate Limiting**: Limit registration attempts per IP
5. **Audit Logging**: Log all OAuth events and approval decisions
6. **RBAC**: Only admins can approve registrations
7. **Token Expiration**: Refresh tokens regularly, expire after inactivity

## Implementation Order

1. ✅ Database migrations (registration_requests, oauth_connections)
2. ✅ Backend OAuth providers (Google first, then Microsoft, then Okta)
3. ✅ Backend OAuth endpoints and handlers
4. ✅ Frontend self-registration page with SSO buttons
5. ✅ Admin registration approval dashboard
6. ✅ Email notifications
7. ✅ Observability dashboard
8. ✅ Verification policy settings
9. ✅ AIM SDK client library
10. ✅ Documentation and examples

## Success Metrics

- ✅ Users can register via SSO without admin intervention
- ✅ Admins can approve/reject in < 30 seconds
- ✅ 100% visibility into service communications
- ✅ Verification happens automatically (zero manual work)
- ✅ SDK makes AIM integration trivial (< 5 lines of code)

## Next Steps

Starting with **Phase 1: OAuth Backend Infrastructure** - implementing Google OAuth first as proof of concept, then Microsoft and Okta.
