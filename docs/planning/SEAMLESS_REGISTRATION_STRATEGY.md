# AIM Seamless Registration Strategy

**Date**: October 7, 2025
**Vision**: Zero-friction registration where developers never think about cryptography after initial setup

---

## 🎯 Core Philosophy

**"Developers focus on workflows, not identity management"**

After initial AIM setup, developers should:
- ✅ Register agents in 30 seconds (3 fields max)
- ✅ Never see or touch cryptographic keys
- ✅ Never worry about verification protocols
- ✅ Get automatic security without configuration
- ✅ Focus 100% on building their agent logic

AIM handles all cryptographic complexity behind the scenes.

---

## 🚨 CRITICAL ISSUE: Current UX Violates Core Philosophy

### Current Agent Registration Form (`/dashboard/agents/new`)

**Problems Identified**:

```tsx
// ❌ CURRENT IMPLEMENTATION (BAD UX)
{/* Public Key */}
<div>
  <label className="block text-sm font-medium text-gray-700 mb-2">
    Public Key (Optional)
  </label>
  <textarea
    rows={6}
    placeholder="-----BEGIN PUBLIC KEY-----&#10;...&#10;-----END PUBLIC KEY-----"
    className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-600 focus:border-transparent font-mono text-sm"
    value={formData.public_key}
    onChange={(e) => setFormData({ ...formData, public_key: e.target.value })}
  />
  <p className="mt-1 text-sm text-gray-500">
    Provide a PEM-encoded public key for cryptographic verification
  </p>
</div>
```

**Why This is Bad**:
1. **Cognitive Load**: User sees cryptographic jargon ("PEM-encoded", "public key")
2. **Confusion**: What should they paste? How do they get a key?
3. **Friction**: Breaks the "register in 30 seconds" promise
4. **Inconsistent**: Says "optional" but then why show it at all?
5. **Anti-Pattern**: Violates "never think about AIM after setup" principle

### Current MCP Registration Modal (`register-mcp-modal.tsx`)

**Same Problems**:

```tsx
// ❌ CURRENT IMPLEMENTATION (BAD UX)
{/* Public Key */}
<div>
  <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
    Public Key <span className="text-gray-500">(optional)</span>
  </label>
  <textarea
    value={formData.public_key}
    onChange={(e) => setFormData({ ...formData, public_key: e.target.value })}
    placeholder="-----BEGIN PUBLIC KEY-----&#10;...&#10;-----END PUBLIC KEY-----"
    rows={6}
    className="w-full px-3 py-2 bg-gray-50 dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900 dark:text-gray-100 font-mono text-xs"
    disabled={loading || success}
  />
  <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">
    Paste PEM-formatted public key for cryptographic verification
  </p>
</div>

{/* Key Type */}
<div>
  <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
    Key Type
  </label>
  <select
    value={formData.key_type}
    onChange={(e) => setFormData({ ...formData, key_type: e.target.value as any })}
    className="w-full px-3 py-2 bg-gray-50 dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900 dark:text-gray-100"
    disabled={loading || success}
  >
    <option value="RSA-2048">RSA-2048</option>
    <option value="RSA-4096">RSA-4096</option>
    <option value="Ed25519">Ed25519</option>
    <option value="ECDSA-P256">ECDSA P-256</option>
  </select>
</div>

{/* Verification URL */}
<div>
  <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
    Verification URL <span className="text-gray-500">(optional)</span>
  </label>
  <input
    type="url"
    value={formData.verification_url}
    onChange={(e) => setFormData({ ...formData, verification_url: e.target.value })}
    placeholder="https://mcp-server.example.com/verify"
    className="w-full px-3 py-2 bg-gray-50 dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900 dark:text-gray-100"
    disabled={loading || success}
  />
  <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">
    Endpoint for cryptographic challenge-response verification
  </p>
</div>
```

**Additional Problems**:
1. User is asked to choose key type (RSA-2048 vs Ed25519?) - they don't care!
2. "Verification URL" - what is that? Why do they need it?
3. "Challenge-response verification" - too technical

---

## ✅ SOLUTION: Automatic Key Generation & Management

### Philosophy Shift

**OLD WAY (Manual)**:
1. User generates key pair locally
2. User pastes public key into AIM
3. User stores private key somewhere
4. User configures verification endpoint
5. User worries about key rotation

**NEW WAY (Automatic)**:
1. User clicks "Register Agent" ✅
2. AIM generates key pair server-side ✅
3. AIM stores public key in database ✅
4. AIM provides SDK with embedded private key ✅
5. Developer never sees cryptographic details ✅

---

## 🛠️ Implementation Plan

### Phase 1: Remove Public Key Fields from UI

**Agent Registration** (`/dashboard/agents/new/page.tsx`):

```tsx
// ✅ NEW IMPLEMENTATION (GOOD UX)
export default function NewAgentPage() {
  const [formData, setFormData] = useState({
    name: '',              // Required - e.g., "customer-support-agent"
    display_name: '',      // Required - e.g., "Customer Support Agent"
    description: '',       // Required - What the agent does
    agent_type: 'ai_agent', // ai_agent or mcp_server
    version: '',           // Optional - e.g., "1.0.0"
    repository_url: '',    // Optional - improves trust score
    documentation_url: '', // Optional
    // ❌ REMOVED: public_key field
  });

  // Backend generates keys automatically
}
```

**MCP Registration** (`register-mcp-modal.tsx`):

```tsx
// ✅ NEW IMPLEMENTATION (GOOD UX)
interface FormData {
  name: string;          // Required - Server name
  url: string;           // Required - Server URL
  description: string;   // Optional - What it provides
  status: 'active' | 'inactive'; // Default: 'active'
  // ❌ REMOVED: public_key
  // ❌ REMOVED: key_type
  // ❌ REMOVED: verification_url
}
```

**User Experience**:
- **Before**: 9 fields, cryptographic jargon, 3+ minutes
- **After**: 3 required fields, plain English, 30 seconds ✅

---

### Phase 2: Backend Automatic Key Generation

**Create Agent Endpoint** (Backend: `POST /api/v1/agents`):

```go
// internal/application/agent_service.go

func (s *AgentService) CreateAgent(ctx context.Context, req *CreateAgentRequest) (*domain.Agent, error) {
    // 1. Generate key pair automatically
    privateKey, publicKey, err := crypto.GenerateKeyPair("Ed25519")
    if err != nil {
        return nil, fmt.Errorf("failed to generate key pair: %w", err)
    }

    // 2. Create agent with generated public key
    agent := &domain.Agent{
        ID:              uuid.New(),
        OrganizationID:  req.OrganizationID,
        Name:            req.Name,
        DisplayName:     req.DisplayName,
        Description:     req.Description,
        AgentType:       req.AgentType,
        PublicKey:       publicKey,     // ✅ Auto-generated
        KeyType:         "Ed25519",      // ✅ AIM chooses best algorithm
        Status:          "active",
        CreatedAt:       time.Now(),
    }

    // 3. Store agent in database
    if err := s.agentRepo.Create(agent); err != nil {
        return nil, err
    }

    // 4. Store private key in secure vault (encrypted)
    if err := s.keyVault.StorePrivateKey(agent.ID, privateKey); err != nil {
        return nil, err
    }

    // 5. Return agent with download link for SDK
    agent.SDKDownloadURL = fmt.Sprintf("/api/v1/agents/%s/sdk", agent.ID)

    return agent, nil
}
```

**Create MCP Server Endpoint** (Backend: `POST /api/v1/mcp-servers`):

```go
// internal/application/mcp_service.go

func (s *MCPService) CreateMCPServer(ctx context.Context, req *CreateMCPServerRequest) (*domain.MCPServer, error) {
    // Same automatic key generation logic
    privateKey, publicKey, err := crypto.GenerateKeyPair("Ed25519")
    if err != nil {
        return nil, fmt.Errorf("failed to generate key pair: %w", err)
    }

    mcpServer := &domain.MCPServer{
        ID:             uuid.New(),
        OrganizationID: req.OrganizationID,
        Name:           req.Name,
        URL:            req.URL,
        Description:    req.Description,
        PublicKey:      publicKey,      // ✅ Auto-generated
        KeyType:        "Ed25519",       // ✅ Best practice default
        Status:         req.Status,
        CreatedAt:      time.Now(),
    }

    // Store in database
    if err := s.mcpRepo.Create(mcpServer); err != nil {
        return nil, err
    }

    // Store private key securely
    if err := s.keyVault.StorePrivateKey(mcpServer.ID, privateKey); err != nil {
        return nil, err
    }

    return mcpServer, nil
}
```

---

### Phase 3: SDK with Embedded Private Key

**Post-Registration Experience**:

After user registers an agent, they see:

```tsx
// Registration Success Screen
<div className="bg-green-50 border border-green-200 rounded-lg p-6">
  <CheckCircle className="h-12 w-12 text-green-600 mb-4" />
  <h3 className="text-lg font-semibold text-green-900">
    Agent Registered Successfully! 🎉
  </h3>
  <p className="mt-2 text-sm text-green-800">
    Your agent "Customer Support Agent" is ready to use.
  </p>

  {/* Download SDK with embedded keys */}
  <div className="mt-6 space-y-3">
    <h4 className="text-sm font-medium text-gray-900">
      Next Steps:
    </h4>

    {/* Language-specific SDK download */}
    <div className="flex gap-3">
      <Button onClick={() => downloadSDK('python')}>
        Download Python SDK
      </Button>
      <Button onClick={() => downloadSDK('nodejs')}>
        Download Node.js SDK
      </Button>
      <Button onClick={() => downloadSDK('go')}>
        Download Go SDK
      </Button>
    </div>

    {/* Simple integration code */}
    <div className="mt-4 bg-gray-900 rounded-lg p-4">
      <code className="text-sm text-green-400">
        # Install SDK
        pip install aim-python-sdk

        # That's it! Your agent is authenticated.
        from aim_sdk import AIMAgent

        agent = AIMAgent()  # ✅ Auto-authenticated
        agent.perform_action("send_email", {...})
      </code>
    </div>
  </div>
</div>
```

**SDK Features**:
1. **Embedded Credentials**: Private key baked into SDK package
2. **Automatic Signing**: All requests signed transparently
3. **Zero Configuration**: Developer never sees keys
4. **Easy Updates**: `aim-sdk update` rotates keys automatically

**Example Python SDK**:

```python
# aim_sdk/__init__.py

class AIMAgent:
    def __init__(self):
        # ✅ Private key embedded in SDK package (encrypted)
        self._private_key = self._load_embedded_key()
        self._agent_id = self._load_agent_id()
        self._aim_api = "https://aim.example.com/api/v1"

    def perform_action(self, action_type: str, params: dict):
        """
        Perform an action with automatic verification.
        Developer never thinks about signing or verification.
        """
        # 1. Create action payload
        payload = {
            "action_type": action_type,
            "params": params,
            "timestamp": int(time.time())
        }

        # 2. Sign automatically (developer doesn't see this)
        signature = self._sign_payload(payload)

        # 3. Send to AIM for verification
        response = requests.post(
            f"{self._aim_api}/agents/{self._agent_id}/verify-action",
            json=payload,
            headers={"X-AIM-Signature": signature}
        )

        # 4. Return result
        return response.json()

    def _sign_payload(self, payload: dict) -> str:
        """Internal method - developer never calls this"""
        message = json.dumps(payload, sort_keys=True)
        signature = ed25519.sign(self._private_key, message.encode())
        return base64.b64encode(signature).decode()
```

**Developer Experience**:

```python
# Developer's code - ZERO cryptography!

from aim_sdk import AIMAgent

agent = AIMAgent()  # ✅ Just works

# Perform actions - all verified automatically
result = agent.perform_action("send_email", {
    "to": "customer@example.com",
    "subject": "Your order has shipped",
    "body": "..."
})

# Developer focuses on business logic, not identity
```

---

### Phase 4: Automatic Key Rotation

**Backend Service** (Runs daily):

```go
// internal/application/key_rotation_service.go

func (s *KeyRotationService) RotateExpiredKeys(ctx context.Context) error {
    // 1. Find keys older than 90 days
    expiredAgents, err := s.agentRepo.GetWithOldKeys(90)
    if err != nil {
        return err
    }

    for _, agent := range expiredAgents {
        // 2. Generate new key pair
        newPrivateKey, newPublicKey, err := crypto.GenerateKeyPair("Ed25519")
        if err != nil {
            log.Printf("Failed to rotate key for agent %s: %v", agent.ID, err)
            continue
        }

        // 3. Update agent's public key
        agent.PublicKey = newPublicKey
        agent.KeyRotatedAt = time.Now()
        if err := s.agentRepo.Update(agent); err != nil {
            log.Printf("Failed to update agent %s: %v", agent.ID, err)
            continue
        }

        // 4. Update private key in vault
        if err := s.keyVault.StorePrivateKey(agent.ID, newPrivateKey); err != nil {
            log.Printf("Failed to store new key for agent %s: %v", agent.ID, err)
            continue
        }

        // 5. Notify user via email
        s.notifyKeyRotation(agent)

        log.Printf("Rotated key for agent %s", agent.ID)
    }

    return nil
}

func (s *KeyRotationService) notifyKeyRotation(agent *domain.Agent) {
    // Email user with update instructions
    email := fmt.Sprintf(`
        Your AIM agent "%s" has had its security keys rotated.

        To update your application, run:
        $ aim-sdk update

        This is done automatically every 90 days for security.
        No action needed if you're using the latest SDK.
    `, agent.DisplayName)

    s.emailService.Send(agent.UserEmail, "AIM Security Key Rotation", email)
}
```

**SDK Auto-Update**:

```bash
# User runs (or cron job runs automatically)
$ aim-sdk update

Checking for updates...
✓ New security keys available
✓ Downloaded updated credentials
✓ SDK updated successfully

Your agent is now using the latest security keys.
```

---

## 🎭 Role-Based Dashboard Views

### Problem Statement

**Current Issue**: All users see same dashboard regardless of role.

**Required Behavior**:
- **Admins**: See everything (user management, security, compliance)
- **Managers**: See operational features (agents, alerts, monitoring)
- **Members**: See their own agents and basic stats
- **Viewers**: Read-only access to dashboards

---

### Implementation Strategy

#### 1. Navigation Menu (Role-Based)

**Current** (`apps/web/components/navigation/sidebar.tsx` or similar):

```tsx
// ❌ CURRENT (EVERYONE SEES EVERYTHING)
const menuItems = [
  { name: 'Dashboard', path: '/dashboard', icon: Home },
  { name: 'Agents', path: '/dashboard/agents', icon: Users },
  { name: 'MCP Servers', path: '/dashboard/mcp', icon: Server },
  { name: 'API Keys', path: '/dashboard/api-keys', icon: Key },
  { name: 'Security', path: '/dashboard/security', icon: Shield },
  { name: 'Monitoring', path: '/dashboard/monitoring', icon: Activity },
  { name: 'Admin', path: '/dashboard/admin', icon: Settings },
];
```

**New** (Role-Based):

```tsx
// ✅ NEW (ROLE-BASED NAVIGATION)

interface User {
  role: 'admin' | 'manager' | 'member' | 'viewer';
}

function getMenuItemsForRole(role: User['role']) {
  // Base items (everyone can see)
  const baseItems = [
    { name: 'Dashboard', path: '/dashboard', icon: Home, roles: ['admin', 'manager', 'member', 'viewer'] },
    { name: 'Agents', path: '/dashboard/agents', icon: Users, roles: ['admin', 'manager', 'member', 'viewer'] },
  ];

  // Member+ items
  const memberItems = [
    { name: 'MCP Servers', path: '/dashboard/mcp', icon: Server, roles: ['admin', 'manager', 'member'] },
    { name: 'API Keys', path: '/dashboard/api-keys', icon: Key, roles: ['admin', 'manager', 'member'] },
  ];

  // Manager+ items
  const managerItems = [
    { name: 'Monitoring', path: '/dashboard/monitoring', icon: Activity, roles: ['admin', 'manager'] },
    { name: 'Security', path: '/dashboard/security', icon: Shield, roles: ['admin', 'manager'] },
    { name: 'Alerts', path: '/dashboard/admin/alerts', icon: AlertTriangle, roles: ['admin', 'manager'] },
  ];

  // Admin-only items
  const adminItems = [
    { name: 'User Management', path: '/dashboard/admin/users', icon: Users, roles: ['admin'] },
    { name: 'Audit Logs', path: '/dashboard/admin/audit-logs', icon: FileText, roles: ['admin'] },
    { name: 'Compliance', path: '/dashboard/admin/compliance', icon: CheckCircle, roles: ['admin'] },
    { name: 'Settings', path: '/dashboard/admin/settings', icon: Settings, roles: ['admin'] },
  ];

  const allItems = [...baseItems, ...memberItems, ...managerItems, ...adminItems];

  // Filter based on role
  return allItems.filter(item => item.roles.includes(role));
}

// Usage
export function Sidebar({ user }: { user: User }) {
  const menuItems = getMenuItemsForRole(user.role);

  return (
    <nav>
      {menuItems.map(item => (
        <Link key={item.path} href={item.path}>
          <item.icon /> {item.name}
        </Link>
      ))}
    </nav>
  );
}
```

---

#### 2. Dashboard Stats (Role-Specific)

**Admin Dashboard** (`/dashboard` for admins):

```tsx
// ✅ ADMIN VIEW - Comprehensive Stats
const adminStats = [
  { name: 'Total Users', value: '247', icon: Users },
  { name: 'Total Agents', value: '1,429', icon: Bot },
  { name: 'Total MCP Servers', value: '156', icon: Server },
  { name: 'Pending Approvals', value: '12', icon: Clock, urgent: true },
  { name: 'Active Alerts', value: '3', icon: AlertTriangle, urgent: true },
  { name: 'Avg Trust Score', value: '87.2', icon: Shield },
  { name: 'Security Incidents', value: '0', icon: ShieldAlert },
  { name: 'Compliance Status', value: '98%', icon: CheckCircle },
];
```

**Manager Dashboard** (`/dashboard` for managers):

```tsx
// ✅ MANAGER VIEW - Operational Stats
const managerStats = [
  { name: 'My Team Agents', value: '42', icon: Bot },
  { name: 'Active Alerts', value: '2', icon: AlertTriangle },
  { name: 'Avg Trust Score', value: '89.5', icon: Shield },
  { name: 'Verifications Today', value: '1,247', icon: CheckCircle },
];
```

**Member Dashboard** (`/dashboard` for members):

```tsx
// ✅ MEMBER VIEW - Personal Stats
const memberStats = [
  { name: 'My Agents', value: '7', icon: Bot },
  { name: 'My API Keys', value: '3', icon: Key },
  { name: 'Avg Trust Score', value: '92.0', icon: Shield },
  { name: 'Verifications Today', value: '523', icon: CheckCircle },
];
```

**Viewer Dashboard** (`/dashboard` for viewers):

```tsx
// ✅ VIEWER VIEW - Read-Only Stats
const viewerStats = [
  { name: 'Organization Agents', value: '1,429', icon: Bot },
  { name: 'Organization Trust Score', value: '87.2', icon: Shield },
  { name: 'Verifications Today', value: '15,247', icon: CheckCircle },
];
```

---

#### 3. Feature Access Control

**Agent Creation**:

```tsx
// ✅ MEMBER+ CAN CREATE AGENTS
export default function AgentsPage({ user }: { user: User }) {
  const canCreateAgent = ['admin', 'manager', 'member'].includes(user.role);

  return (
    <div>
      <div className="flex justify-between items-center">
        <h1>Agents</h1>
        {canCreateAgent && (
          <Button onClick={() => router.push('/dashboard/agents/new')}>
            Create Agent
          </Button>
        )}
      </div>

      {/* Viewers see read-only list */}
      {user.role === 'viewer' && (
        <p className="text-sm text-gray-500">
          You have read-only access. Contact your admin to create agents.
        </p>
      )}
    </div>
  );
}
```

**User Management**:

```tsx
// ✅ ADMIN-ONLY ACCESS
export default function UsersPage({ user }: { user: User }) {
  if (user.role !== 'admin') {
    return (
      <div className="text-center py-12">
        <Shield className="h-12 w-12 text-red-500 mx-auto" />
        <h3 className="mt-4 text-lg font-semibold">Access Denied</h3>
        <p className="mt-2 text-sm text-gray-500">
          Only administrators can access user management.
        </p>
      </div>
    );
  }

  // Admin user management UI
  return <UserManagementDashboard />;
}
```

---

## ⚡ Automatic Runtime Verification (Zero Developer Effort)

### Current Problem

**Backend has verification endpoints**:
- `POST /api/v1/agents/:id/verify-action` - Verify agent action
- `POST /api/v1/mcp-servers/:id/verify-action` - Verify MCP action

But these require **manual integration** by developers, which violates our philosophy.

### Solution: SDK Handles Everything Automatically

**Developer Experience** (What it should feel like):

```python
# Developer's code - NO verification logic needed!

from aim_sdk import AIMAgent

# 1. Initialize agent (keys embedded, auto-authenticated)
agent = AIMAgent()

# 2. Perform action - verification happens transparently
result = agent.perform_action("send_email", {
    "to": "customer@example.com",
    "subject": "Order confirmation",
    "body": "Your order #12345 has been received"
})

# 3. Done! Verification happened behind the scenes
# Developer never called verify(), sign(), or authenticate()
```

**What Happens Behind the Scenes** (Transparent to developer):

```python
# aim_sdk/agent.py (Internal SDK code)

class AIMAgent:
    def perform_action(self, action_type: str, params: dict):
        """
        Performs action with AUTOMATIC verification.
        Developer never sees this complexity.
        """

        # Step 1: Create action payload
        payload = {
            "agent_id": self._agent_id,
            "action_type": action_type,
            "params": params,
            "timestamp": int(time.time()),
            "nonce": secrets.token_hex(16)
        }

        # Step 2: Sign payload with embedded private key
        # (Developer never touches this key!)
        signature = self._sign_payload(payload)

        # Step 3: Send to AIM for verification BEFORE executing
        try:
            verification_response = requests.post(
                f"{self._aim_api}/agents/{self._agent_id}/verify-action",
                json=payload,
                headers={
                    "Authorization": f"Bearer {self._auth_token}",
                    "X-AIM-Signature": signature
                },
                timeout=2  # Fast verification
            )

            if verification_response.status_code != 200:
                raise VerificationFailedError(
                    f"AIM verification failed: {verification_response.json()['message']}"
                )

            # Get audit_id from verification response
            audit_id = verification_response.json()["audit_id"]

        except requests.RequestException as e:
            # Graceful degradation: log but don't block
            log.warning(f"AIM verification failed: {e}")
            # Optional: Fallback to local verification or alert admin
            audit_id = None

        # Step 4: Execute the actual action (developer's business logic)
        action_result = self._execute_action(action_type, params)

        # Step 5: Log action result back to AIM
        if audit_id:
            self._log_action_result(audit_id, action_result)

        # Step 6: Return result to developer
        return action_result

    def _execute_action(self, action_type: str, params: dict):
        """
        This is where the actual business logic happens.
        Could be sending email, calling API, updating database, etc.
        """
        # Developer's custom action handlers
        if action_type == "send_email":
            return self._send_email(params)
        elif action_type == "update_database":
            return self._update_database(params)
        else:
            raise ValueError(f"Unknown action type: {action_type}")

    def _log_action_result(self, audit_id: str, result: dict):
        """
        Report action outcome back to AIM for audit trail.
        Developer never calls this manually.
        """
        try:
            requests.post(
                f"{self._aim_api}/agents/{self._agent_id}/log-action/{audit_id}",
                json={
                    "success": result.get("success", True),
                    "result": result,
                    "timestamp": int(time.time())
                },
                headers={"Authorization": f"Bearer {self._auth_token}"},
                timeout=2
            )
        except requests.RequestException as e:
            log.warning(f"Failed to log action result: {e}")
            # Non-blocking: don't fail user's action if logging fails
```

### MCP Server Automatic Verification

**Same seamless experience for MCP servers**:

```typescript
// Developer's MCP server code - NO verification logic!

import { MCPServer } from '@aim/mcp-sdk';

const server = new MCPServer();  // ✅ Auto-authenticated with embedded keys

// Define tools - verification happens automatically
server.addTool('read_file', async (params) => {
  // AIM verification already happened before this executes!
  const content = fs.readFileSync(params.path, 'utf-8');
  return { content };
});

server.addTool('write_file', async (params) => {
  // AIM verification already happened before this executes!
  fs.writeFileSync(params.path, params.content);
  return { success: true };
});

// Start server - all requests auto-verified
server.listen(3000);
```

**Behind the scenes** (MCP SDK):

```typescript
// @aim/mcp-sdk/server.ts (Internal SDK code)

class MCPServer {
  async handleRequest(toolName: string, params: any): Promise<any> {
    // 1. Sign request with embedded private key
    const signature = this.signRequest(toolName, params);

    // 2. Verify with AIM BEFORE executing tool
    const verificationResult = await fetch(
      `${this.aimAPI}/mcp-servers/${this.serverId}/verify-action`,
      {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${this.authToken}`,
          'X-AIM-Signature': signature
        },
        body: JSON.stringify({
          tool_name: toolName,
          params: params,
          timestamp: Date.now()
        })
      }
    );

    if (!verificationResult.ok) {
      throw new Error('AIM verification failed - action blocked');
    }

    const { audit_id } = await verificationResult.json();

    // 3. Execute the tool (developer's handler)
    const result = await this.toolHandlers[toolName](params);

    // 4. Log result back to AIM
    await this.logActionResult(audit_id, result);

    // 5. Return result
    return result;
  }
}
```

### Verification Flow Diagram

```
Developer's Code:
  agent.perform_action("send_email", {...})
    ↓

SDK (Automatic):
  1. Create payload
  2. Sign with embedded private key
  3. → POST /api/v1/agents/:id/verify-action
    ← { verified: true, audit_id: "123" }
  4. Execute developer's action handler
  5. → POST /api/v1/agents/:id/log-action/123
    ← { logged: true }
  6. Return result to developer
    ↓

Developer receives:
  { success: true, ... }

DEVELOPER NEVER SAW:
  ❌ Signing
  ❌ Verification
  ❌ Audit logging
  ❌ Private keys
  ❌ Signatures
```

### Trust Score Updates (Also Automatic)

**Backend automatically calculates trust scores** based on verified actions:

```go
// Backend: After successful verification
func (s *AgentService) VerifyAction(ctx context.Context, req *VerifyActionRequest) error {
    // ... signature verification ...

    // Automatically update trust score factors
    go s.updateTrustScoreFactors(req.AgentID, TrustScoreFactors{
        SuccessfulVerification: true,
        Uptime: calculateUptime(req.AgentID),
        SecurityCompliance: checkSecurityCompliance(req.AgentID),
    })

    return nil
}
```

**Result**: Trust scores improve automatically as agents perform verified actions. Developer never manually calculates or updates trust scores.

### Developer Onboarding Flow

**Complete onboarding in 3 steps**:

1. **Register agent** (30 seconds)
   ```bash
   # In AIM dashboard: Click "Create Agent"
   # Fill: Name, Description, Type
   # Click "Create"
   ```

2. **Download SDK** (10 seconds)
   ```bash
   # AIM provides download link
   pip install aim-python-sdk
   ```

3. **Write code** (5 minutes)
   ```python
   from aim_sdk import AIMAgent

   agent = AIMAgent()  # Auto-authenticated

   # All actions auto-verified
   agent.perform_action("send_email", {...})
   agent.perform_action("update_db", {...})
   ```

**Total time to production**: < 10 minutes
**Lines of security/verification code**: 0
**Cryptographic knowledge required**: 0

---

## 📊 Role Permissions Matrix

| Feature | Admin | Manager | Member | Viewer |
|---------|-------|---------|--------|--------|
| **Dashboard** | Full stats | Team stats | Personal stats | Org stats |
| **View Agents** | All org agents | Team agents | Own agents | All org agents (RO) |
| **Create Agent** | ✅ | ✅ | ✅ | ❌ |
| **Edit Agent** | All agents | Team agents | Own agents | ❌ |
| **Delete Agent** | ✅ | Team agents | ❌ | ❌ |
| **View MCP Servers** | ✅ | ✅ | ✅ | ✅ (RO) |
| **Create MCP Server** | ✅ | ✅ | ✅ | ❌ |
| **Manage API Keys** | All keys | Team keys | Own keys | ❌ |
| **View Security Dashboard** | ✅ | ✅ | ❌ | ❌ |
| **Manage Alerts** | ✅ | ✅ | ❌ | ❌ |
| **View Audit Logs** | ✅ | ❌ | ❌ | ❌ |
| **User Management** | ✅ | ❌ | ❌ | ❌ |
| **Compliance Reports** | ✅ | ❌ | ❌ | ❌ |
| **Organization Settings** | ✅ | ❌ | ❌ | ❌ |

---

## 🚀 Implementation Checklist

### Immediate Actions (This Week)

- [ ] **Remove public key fields from agent registration form**
  - File: `apps/web/app/dashboard/agents/new/page.tsx`
  - Remove: `public_key` textarea
  - Result: 3 required fields only

- [ ] **Remove cryptographic fields from MCP registration modal**
  - File: `apps/web/components/modals/register-mcp-modal.tsx`
  - Remove: `public_key`, `key_type`, `verification_url`
  - Result: Name + URL + Description only

- [ ] **Implement automatic key generation in backend**
  - File: `apps/backend/internal/application/agent_service.go`
  - Add: `GenerateKeyPair()` call in `CreateAgent()`
  - Add: `KeyVault.StorePrivateKey()` integration

- [ ] **Implement role-based navigation**
  - Create: `apps/web/lib/permissions.ts`
  - Update: Navigation/Sidebar component
  - Add: Role-based menu filtering

- [ ] **Implement role-based dashboard stats**
  - File: `apps/web/app/dashboard/page.tsx`
  - Add: `getStatsForRole()` function
  - Display different stats per role

### Short-Term (Next 2 Weeks)

- [ ] Create Python SDK with embedded keys
- [ ] Create Node.js SDK with embedded keys
- [ ] Create Go SDK with embedded keys
- [ ] Build SDK download endpoint (`GET /api/v1/agents/:id/sdk`)
- [ ] Add post-registration success screen with SDK download
- [ ] Implement access control on all admin pages
- [ ] Add "Access Denied" screens for unauthorized roles

### Medium-Term (1 Month)

- [ ] Implement automatic key rotation service
- [ ] Build SDK auto-update mechanism
- [ ] Create key rotation notification emails
- [ ] Add SDK usage documentation
- [ ] Build role-based filtering for agents list (team vs personal)

---

## 📈 Success Metrics

**Developer Experience**:
- ✅ Agent registration time: <30 seconds (currently 3+ minutes)
- ✅ Fields required: 3 (currently 9)
- ✅ Cryptographic knowledge needed: 0 (currently high)
- ✅ Post-setup thinking about AIM: 0 (goal: "set it and forget it")

**Security**:
- ✅ All agents automatically have cryptographic verification
- ✅ Key rotation happens automatically every 90 days
- ✅ No unencrypted private keys on developer machines
- ✅ 100% of agents use best-practice algorithms (Ed25519)

**Investment Appeal**:
- ✅ "Zero-friction security" is a compelling value proposition
- ✅ Demonstrates enterprise-grade UX thinking
- ✅ Shows deep understanding of developer pain points
- ✅ Differentiator vs competitors who expose crypto complexity

---

## 🎯 Vision Statement

**"AIM makes identity verification invisible"**

After registering with AIM, developers should feel like they have a magic SDK that:
- Authenticates their agents automatically
- Signs requests without configuration
- Rotates keys without intervention
- Maintains security without complexity

The best identity system is one developers never think about. That's AIM.
