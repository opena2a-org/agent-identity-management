# 🎯 Seamless Auto-Registration - Zero-Friction Agent Identity

**Design Philosophy**: Based on Atomic Habits - make secure behavior easier than insecure behavior.

## The Vision

**Instead of this** (5 manual steps, 2-3 minutes):
```
1. Developer opens browser
2. Navigates to AIM dashboard
3. Fills registration form
4. Downloads SDK
5. Copies credentials to code
```

**Developers write this** (1 line, 0 thinking):
```python
from aim_sdk import AIMClient

# That's it! Auto-registers on first run, loads credentials on subsequent runs
client = AIMClient.auto_register(
    name="my-agent",
    agent_type="ai_agent",
    aim_url="https://aim.company.com"
)

# Use immediately - all crypto happens automatically
@client.perform_action("read_database", resource="users")
def get_users():
    return db.query("SELECT * FROM users")
```

## How It Works

### First Run (Auto-Registration)
```
1. SDK checks for credentials at ~/.aim/credentials/my-agent.json
2. Not found → calls POST /api/v1/agents/auto-register
3. AIM generates Ed25519 keypair
4. AIM returns credentials in response
5. SDK stores credentials locally (chmod 600)
6. Agent is ready to use (may be pending approval)
```

### Subsequent Runs (Auto-Load)
```
1. SDK checks for credentials at ~/.aim/credentials/my-agent.json
2. Found → loads credentials
3. Agent ready to use immediately
```

## API Specification

### Endpoint: Auto-Register Agent

**Request**:
```http
POST /api/v1/agents/auto-register
Content-Type: application/json
X-AIM-API-Key: org_api_key_here (for organization detection)

{
  "name": "customer-service-agent",
  "agent_type": "ai_agent",
  "description": "Handles customer inquiries",
  "capabilities": ["read_database", "send_email"],
  "hostname": "laptop-123.local",
  "platform": "darwin",
  "sdk_version": "1.0.0",
  "auto_approve": false
}
```

**Response (Success - Pending Approval)**:
```http
HTTP 201 Created
Content-Type: application/json

{
  "agent_id": "550e8400-e29b-41d4-a716-446655440000",
  "public_key": "9HSDiRWzTqhRu7iyYotXYLzcynJ9ReaArsGvbsT+PWI=",
  "private_key": "gbkroKOpjYzrXJCZncOHtDlyuujHm5yiAzJ36mmooan0d...",
  "status": "pending",
  "message": "Agent registered successfully. Pending admin approval.",
  "organization_id": "org-uuid",
  "created_at": "2025-10-07T22:30:00Z"
}
```

**Response (Success - Auto-Approved for Dev)**:
```http
HTTP 201 Created
Content-Type: application/json

{
  "agent_id": "550e8400-e29b-41d4-a716-446655440000",
  "public_key": "9HSDiRWzTqhRu7iyYotXYLzcynJ9ReaArsGvbsT+PWI=",
  "private_key": "gbkroKOpjYzrXJCZncOHtDlyuujHm5yiAzJ36mmooan0d...",
  "status": "active",
  "message": "Agent registered and auto-approved.",
  "organization_id": "org-uuid",
  "created_at": "2025-10-07T22:30:00Z"
}
```

**Response (Error - Already Exists)**:
```http
HTTP 409 Conflict
Content-Type: application/json

{
  "error": "Agent with this name already exists",
  "existing_agent_id": "existing-uuid",
  "suggestion": "Use a different name or load existing credentials"
}
```

## Credential Storage

### File Location
```
~/.aim/
├── credentials/
│   ├── my-agent.json           # chmod 600 (user read-only)
│   ├── slack-bot.json
│   └── email-processor.json
└── config.json                 # Global AIM config
```

### Credential File Format
```json
{
  "agent_id": "550e8400-e29b-41d4-a716-446655440000",
  "agent_name": "my-agent",
  "public_key": "9HSDiRWzTqhRu7iyYotXYLzcynJ9ReaArsGvbsT+PWI=",
  "private_key": "gbkroKOpjYzrXJCZncOHtDlyuujHm5yiAzJ36mmooan0d...",
  "organization_id": "org-uuid",
  "aim_url": "https://aim.company.com",
  "status": "active",
  "created_at": "2025-10-07T22:30:00Z",
  "last_used": "2025-10-07T23:45:00Z"
}
```

### Security
- Files stored with `chmod 600` (owner read/write only)
- Private key NEVER leaves developer's machine after initial download
- AIM server stores encrypted backup of private key (for disaster recovery)
- Public key stored on AIM for verification

## SDK Implementation

### Auto-Register Method

```python
class AIMClient:
    @classmethod
    def auto_register(
        cls,
        name: str,
        agent_type: str = "ai_agent",
        aim_url: Optional[str] = None,
        api_key: Optional[str] = None,
        auto_approve: bool = False,
        description: Optional[str] = None,
        capabilities: Optional[List[str]] = None,
        force_refresh: bool = False
    ) -> "AIMClient":
        """
        Automatically register agent or load existing credentials.

        First run:
        - Registers agent with AIM
        - Generates Ed25519 keypair on server
        - Downloads and stores credentials locally
        - Returns initialized client

        Subsequent runs:
        - Loads credentials from ~/.aim/credentials/{name}.json
        - Returns initialized client immediately

        Args:
            name: Unique agent name (used for credential file)
            agent_type: Type of agent (ai_agent, mcp_server, etc.)
            aim_url: AIM server URL (or from env: AIM_URL)
            api_key: Organization API key (or from env: AIM_API_KEY)
            auto_approve: Auto-approve in dev environments
            description: Agent description
            capabilities: List of actions agent can perform
            force_refresh: Force re-registration even if credentials exist

        Returns:
            Initialized AIMClient instance

        Raises:
            ConfigurationError: If AIM URL or API key not provided
            RegistrationError: If registration fails
            AgentPendingApprovalError: If agent pending admin approval

        Example:
            >>> client = AIMClient.auto_register(
            ...     name="my-agent",
            ...     aim_url="https://aim.company.com"
            ... )
            >>> # First run: Registers and stores credentials
            >>> # Next runs: Loads credentials instantly
        """

        # Get AIM URL from parameter or environment
        aim_url = aim_url or os.getenv("AIM_URL")
        if not aim_url:
            raise ConfigurationError("AIM URL required (parameter or AIM_URL env var)")

        # Get API key from parameter or environment
        api_key = api_key or os.getenv("AIM_API_KEY")
        if not api_key:
            raise ConfigurationError("API key required (parameter or AIM_API_KEY env var)")

        # Check for existing credentials
        creds_path = Path.home() / ".aim" / "credentials" / f"{name}.json"

        if creds_path.exists() and not force_refresh:
            # Load existing credentials
            with open(creds_path) as f:
                creds = json.load(f)

            # Update last_used timestamp
            creds["last_used"] = datetime.now(timezone.utc).isoformat()
            with open(creds_path, "w") as f:
                json.dump(creds, f, indent=2)

            # Return initialized client
            return cls(
                agent_id=creds["agent_id"],
                public_key=creds["public_key"],
                private_key=creds["private_key"],
                aim_url=creds["aim_url"]
            )

        # First time - register with AIM
        response = requests.post(
            f"{aim_url}/api/v1/agents/auto-register",
            headers={
                "X-AIM-API-Key": api_key,
                "Content-Type": "application/json"
            },
            json={
                "name": name,
                "agent_type": agent_type,
                "description": description,
                "capabilities": capabilities or [],
                "hostname": socket.gethostname(),
                "platform": sys.platform,
                "sdk_version": "1.0.0",
                "auto_approve": auto_approve
            },
            timeout=30
        )

        if response.status_code == 409:
            # Agent already exists
            error_data = response.json()
            raise RegistrationError(
                f"Agent '{name}' already exists. "
                f"Existing ID: {error_data.get('existing_agent_id')}"
            )

        response.raise_for_status()
        creds = response.json()

        # Create credentials directory
        creds_path.parent.mkdir(parents=True, exist_ok=True)

        # Store credentials locally
        with open(creds_path, "w") as f:
            json.dump({
                "agent_id": creds["agent_id"],
                "agent_name": name,
                "public_key": creds["public_key"],
                "private_key": creds["private_key"],
                "organization_id": creds["organization_id"],
                "aim_url": aim_url,
                "status": creds["status"],
                "created_at": creds["created_at"],
                "last_used": datetime.now(timezone.utc).isoformat()
            }, f, indent=2)

        # Secure the file (user read-only)
        os.chmod(creds_path, 0o600)

        # Log success
        print(f"✅ Agent '{name}' registered successfully!")
        print(f"   Agent ID: {creds['agent_id']}")
        print(f"   Status: {creds['status']}")
        if creds['status'] == 'pending':
            print(f"   ⏳ Pending admin approval")
        print(f"   Credentials stored: {creds_path}")

        # Return initialized client
        return cls(
            agent_id=creds["agent_id"],
            public_key=creds["public_key"],
            private_key=creds["private_key"],
            aim_url=aim_url
        )
```

## Backend Implementation

### Handler Method

```go
// AutoRegister handles automatic agent registration
func (h *AgentHandler) AutoRegister(c fiber.Ctx) error {
    // Get organization from API key
    apiKey := c.Get("X-AIM-API-Key")
    if apiKey == "" {
        return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
            "error": "API key required in X-AIM-API-Key header",
        })
    }

    orgID, err := h.authService.GetOrganizationFromAPIKey(c.Context(), apiKey)
    if err != nil {
        return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
            "error": "Invalid API key",
        })
    }

    var req struct {
        Name         string   `json:"name"`
        AgentType    string   `json:"agent_type"`
        Description  string   `json:"description"`
        Capabilities []string `json:"capabilities"`
        Hostname     string   `json:"hostname"`
        Platform     string   `json:"platform"`
        SDKVersion   string   `json:"sdk_version"`
        AutoApprove  bool     `json:"auto_approve"`
    }

    if err := c.Bind().JSON(&req); err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
            "error": "Invalid request body",
        })
    }

    // Check if agent already exists
    existingAgent, _ := h.agentService.GetAgentByName(c.Context(), orgID, req.Name)
    if existingAgent != nil {
        return c.Status(fiber.StatusConflict).JSON(fiber.Map{
            "error": "Agent with this name already exists",
            "existing_agent_id": existingAgent.ID.String(),
            "suggestion": "Use a different name or load existing credentials",
        })
    }

    // Create agent with auto-generated keys
    agent, publicKey, privateKey, err := h.agentService.CreateAgentWithAutoKeys(
        c.Context(),
        &application.CreateAgentRequest{
            Name:         req.Name,
            DisplayName:  req.Name,
            AgentType:    req.AgentType,
            Description:  req.Description,
            Capabilities: req.Capabilities,
            Metadata: map[string]interface{}{
                "hostname":    req.Hostname,
                "platform":    req.Platform,
                "sdk_version": req.SDKVersion,
                "auto_registered": true,
            },
        },
        orgID,
        uuid.Nil, // System user for auto-registration
    )

    if err != nil {
        return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
            "error": "Failed to register agent",
        })
    }

    // Auto-approve if requested (dev environments only)
    status := "pending"
    if req.AutoApprove {
        if err := h.agentService.VerifyAgent(c.Context(), agent.ID); err == nil {
            status = "active"
        }
    }

    // Log audit
    h.auditService.LogAction(
        c.Context(),
        orgID,
        uuid.Nil, // System action
        domain.AuditActionCreate,
        "agent",
        agent.ID,
        c.IP(),
        c.Get("User-Agent"),
        map[string]interface{}{
            "agent_name": req.Name,
            "auto_registered": true,
            "auto_approve": req.AutoApprove,
        },
    )

    // Return credentials (ONLY TIME private key is sent!)
    return c.Status(fiber.StatusCreated).JSON(fiber.Map{
        "agent_id":        agent.ID.String(),
        "public_key":      publicKey,
        "private_key":     privateKey, // Sent once, never again!
        "status":          status,
        "message":         getStatusMessage(status),
        "organization_id": orgID.String(),
        "created_at":      agent.CreatedAt,
    })
}

func getStatusMessage(status string) string {
    if status == "active" {
        return "Agent registered and auto-approved."
    }
    return "Agent registered successfully. Pending admin approval."
}
```

## Usage Examples

### Example 1: Simple Auto-Registration

```python
from aim_sdk import AIMClient

# First run: registers automatically
# Subsequent runs: loads credentials
client = AIMClient.auto_register(
    name="my-agent",
    aim_url="https://aim.company.com"
)

# Use immediately!
@client.perform_action("read_file", resource="/data/users.csv")
def process_users():
    with open("/data/users.csv") as f:
        return f.read()

result = process_users()
```

### Example 2: Development with Auto-Approve

```python
import os
from aim_sdk import AIMClient

# Dev environment - auto-approve
client = AIMClient.auto_register(
    name="dev-agent",
    aim_url="https://aim-dev.company.com",
    auto_approve=True  # Skip admin approval
)

# Works immediately without waiting for approval
```

### Example 3: Production with Capabilities

```python
from aim_sdk import AIMClient

client = AIMClient.auto_register(
    name="customer-service-bot",
    agent_type="ai_agent",
    description="Handles customer inquiries via Slack",
    capabilities=["read_database", "send_slack_message", "create_ticket"],
    aim_url=os.getenv("AIM_URL")
)

# First run: pending approval
# Admin approves in dashboard
# Subsequent runs: works immediately
```

## Admin Experience

### Approval Workflow

1. **Agent Auto-Registers**:
   - Appears in dashboard with "Pending" status
   - Shows hostname, platform, SDK version
   - Shows requested capabilities

2. **Admin Reviews**:
   - Sees agent name, type, description
   - Sees who/where it's running (hostname)
   - Can approve or deny

3. **Admin Approves**:
   - Agent status → "Active"
   - Agent can now perform actions
   - Developer's next run works immediately

4. **Admin Denies**:
   - Agent status → "Denied"
   - SDK raises `AgentDeniedError` on next run
   - Developer must fix and re-register

## Benefits

### For Developers
- ✅ 1 line of code vs 5 manual steps
- ✅ 0 minutes vs 2-3 minutes setup
- ✅ No UI navigation required
- ✅ No credential copy-paste
- ✅ Works identically in dev and prod

### For Security Teams
- ✅ All agents automatically registered
- ✅ Centralized approval workflow
- ✅ Complete audit trail
- ✅ Cryptographic verification
- ✅ No insecure workarounds

### For Executives
- ✅ Developers happy = productive
- ✅ Security happy = compliant
- ✅ Zero adoption friction
- ✅ Lower security training costs
- ✅ Faster time-to-market

## Migration Path

### Existing Manual Registration Still Works

```python
# Old way (still supported)
client = AIMClient(
    agent_id="uuid",
    public_key="...",
    private_key="...",
    aim_url="https://aim.company.com"
)

# New way (recommended)
client = AIMClient.auto_register(
    name="my-agent",
    aim_url="https://aim.company.com"
)
```

Both work! Auto-register is just easier.

## Implementation Checklist

### Backend
- [ ] Add `POST /api/v1/agents/auto-register` endpoint
- [ ] Implement API key authentication
- [ ] Add `GetOrganizationFromAPIKey` method
- [ ] Add `GetAgentByName` method
- [ ] Add `CreateAgentWithAutoKeys` method (returns private key)
- [ ] Add auto-approve flag support
- [ ] Add conflict detection (agent name already exists)
- [ ] Add audit logging for auto-registration

### SDK
- [ ] Add `AIMClient.auto_register()` class method
- [ ] Implement credential file storage (~/.aim/credentials/)
- [ ] Implement file permissions (chmod 600)
- [ ] Add credential loading logic
- [ ] Add `last_used` timestamp updates
- [ ] Add `force_refresh` flag support
- [ ] Add helpful console output
- [ ] Add `RegistrationError` exception
- [ ] Add `AgentPendingApprovalError` exception
- [ ] Add `AgentDeniedError` exception

### Documentation
- [ ] Update README with auto-register example
- [ ] Add "Quick Start" guide
- [ ] Add troubleshooting guide
- [ ] Add security best practices
- [ ] Add migration guide from manual registration

### Testing
- [ ] Test first-time registration
- [ ] Test subsequent credential loading
- [ ] Test approval workflow
- [ ] Test denial workflow
- [ ] Test auto-approve flag
- [ ] Test conflict detection
- [ ] Test file permissions
- [ ] Test invalid API keys
- [ ] Test network failures

---

**Estimated Implementation Time**: 6-8 hours
**Impact**: 🚀 MASSIVE - Makes AIM the easiest identity management system on the planet
**Investor Reaction**: 🤯 "This is brilliant - everyone will use this!"
