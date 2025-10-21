# AIM Python SDK

**Enterprise-grade identity and capability management for AI agents.**

One-line agent registration with automatic cryptographic verification.

## Quick Start - Zero Configuration 🚀

### SDK Download Mode (ZERO CONFIG!)

```python
from aim_sdk import secure

# ONE LINE - That's it! Everything auto-detected.
agent = secure("my-agent")

# ✅ Auto-detected: OAuth credentials, capabilities, MCP servers
# ✅ Auto-verified: Challenge-response verification
# ✅ Ready to use!

@agent.perform_action("send_email", resource="admin@example.com")
def send_critical_notification(message):
    send_email("admin@example.com", message)
```

### Manual Install Mode (Still Easy!)

```python
from aim_sdk import secure

# Requires API key, but still auto-detects capabilities + MCPs
agent = secure("my-agent", api_key="aim_abc123")

# ✅ Auto-detected: Capabilities, MCP servers
# ✅ Auto-verified: Challenge-response verification
```

## Installation

**Option 1: Download SDK from Dashboard (Recommended)**
- Visit your AIM dashboard
- Click "Download SDK" → Includes embedded OAuth credentials
- Extract and you're ready to go!

**Option 2: Manual Install via pip**
```bash
pip install aim-sdk
# or
pip install -r requirements.txt
```

## Features

- ✅ **Zero-config registration**: One line, everything auto-detected (SDK mode)
- ✅ **Automatic capability detection**: Scans imports, decorators, config files
- ✅ **Automatic MCP detection**: Finds MCP servers from Claude config & imports
- ✅ **Automatic key management**: Ed25519 keys generated and stored securely
- ✅ **Automatic verification**: Challenge-response auth, auto-approval
- ✅ **Decorator-based verification**: Simple `@agent.perform_action()` decorator
- ✅ **Trust scoring**: ML-powered risk assessment
- ✅ **Audit logging**: Complete action history
- ✅ **Secure storage**: Credentials saved to `~/.aim/credentials.json` (0600 permissions)

## Usage Modes

### Mode 1: SDK Download (ZERO CONFIG) ⭐ Recommended

```python
from aim_sdk import secure

# ONE LINE - Everything auto-detected!
agent = secure("my-agent")

# What happens behind the scenes:
# ✅ OAuth credentials loaded from .aim/credentials.json
# ✅ Capabilities auto-detected (imports, decorators, config)
# ✅ MCP servers auto-detected (Claude config, imports)
# ✅ Agent registered with AIM
# ✅ Challenge-response verification completed
# ✅ Auto-approved (if trust score ≥70)
```

### Mode 2: Manual Install (API Key)

```python
from aim_sdk import secure

# Requires API key, but still auto-detects capabilities + MCPs
agent = secure("my-agent", api_key="aim_abc123")

# What happens:
# ✅ Capabilities auto-detected
# ✅ MCP servers auto-detected
# ✅ Agent registered
# ✅ Auto-verified
```

### Mode 3: Power User (Full Control)

```python
from aim_sdk import secure

# Disable auto-detection, specify everything manually
agent = secure(
    name="my-agent",
    api_key="aim_abc123",
    auto_detect=False,  # Disable auto-detection
    capabilities=["custom_capability"],
    talks_to=["custom-mcp-server"],
    display_name="My Custom Agent",
    version="1.0.0",
    repository_url="https://github.com/myorg/my-agent"
)
```

### Mode 4: Existing Credentials

```python
from aim_sdk import AIMClient

# If you already have credentials from previous registration
client = AIMClient(
    agent_id="your-agent-id",
    public_key="base64-public-key",
    private_key="base64-private-key",
    aim_url="https://aim.example.com"
)
```

### Performing Verified Actions

```python
# Simple action verification
@agent.perform_action("read_database", resource="users_table")
def get_user_data(user_id):
    return database.query(f"SELECT * FROM users WHERE id = {user_id}")

# Action with additional context
@agent.perform_action(
    "modify_user", 
    resource="user:12345",
    metadata={"reason": "Account update requested by user"}
)
def update_user_email(user_id, new_email):
    return database.execute(
        "UPDATE users SET email = ? WHERE id = ?",
        new_email, user_id
    )

# High-risk action (requires higher trust score)
@agent.perform_action(
    "delete_data",
    resource="user:12345",
    risk_level="high"
)
def delete_user_account(user_id):
    return database.execute("DELETE FROM users WHERE id = ?", user_id)
```

## Capability Management - How Auto-Grant Works 🔒

### Initial Registration: Auto-Grant (No Approval Needed!)

When you register an agent, **capabilities are automatically granted** - no admin approval required!

```python
from aim_sdk import secure

# Capabilities detected and AUTO-GRANTED immediately
agent = secure("my-agent")

# ✅ Capabilities: Auto-detected from imports/decorators
# ✅ Granted: Automatically during registration
# ✅ Ready to use: Perform actions immediately!
```

**This is a game-changer**: Users can start using agents immediately without waiting for admin approval.

### Capability Updates: Admin Approval Required

If you need to add NEW capabilities after registration, admins must approve:

```python
from aim_sdk import AIMClient

client = AIMClient.from_credentials("my-agent")

# Request new capability (requires admin approval)
request = client.capabilities.request(
    capability_type="delete_email",
    reason="Need to clean up spam automatically"
)

print(f"Request created: {request['id']}")
print(f"Status: {request['status']}")  # "pending"

# Admin reviews and approves via dashboard
# Once approved, capability is automatically granted
```

**Why this workflow?**
- **Fast onboarding**: Users start immediately
- **Security**: Admins review capability expansions
- **Scalability**: No bottleneck for thousands of agents

### How Enforcement Works

AIM enforces capabilities using a **single source of truth**:

```python
# ✅ ENFORCEMENT: Only GRANTED capabilities are enforced
# - agent.capabilities (array) = DECLARED (reference only)
# - agent_capabilities (table) = GRANTED (enforcement)

@agent.perform_action("read_email")
def read_inbox():
    # ✅ Allowed if "read_email" was GRANTED
    # ❌ Denied if "read_email" not granted (even if declared)
    pass
```

**Security Benefits**:
- Prevents CVE-2025-32711 (EchoLeak) attacks
- Admin control over capability expansion
- Full audit trail (who granted what, when)

### Alternative: Delete and Re-register

Don't want to wait for admin approval? Delete your agent and re-register with updated capabilities:

```python
from aim_sdk import secure, AIMClient

# Delete existing agent
client = AIMClient.from_credentials("my-agent")
client.agents.delete(agent_id=client.agent_id)

# Re-register with updated capabilities
agent = secure(
    "my-agent",
    capabilities=["read_email", "send_email", "delete_email"]  # ✅ All auto-granted
)
```

**Trade-off**: Loses historical trust score and audit logs.

## Credential Storage

Credentials are automatically saved to `~/.aim/credentials.json` with secure permissions (0600).

**⚠️ Security Warning**: The private key is only returned ONCE during registration. Keep it safe!

```json
{
  "my-agent": {
    "agent_id": "550e8400-e29b-41d4-a716-446655440000",
    "public_key": "base64-encoded-public-key",
    "private_key": "base64-encoded-private-key",
    "aim_url": "http://localhost:8080",
    "status": "verified",
    "trust_score": 75.0,
    "registered_at": "2025-10-07T16:05:27.143786Z"
  }
}
```

## Auto-Detection: Fully Automated Setup 🎉

### Full Auto-Detection (NOW AVAILABLE!)

AIM now automatically detects **EVERYTHING**:

```python
from aim_sdk import secure

# ONE LINE - Zero configuration!
agent = secure("my-agent")

# AIM automatically detects:
# ✅ Agent capabilities (from imports, decorators, config files)
# ✅ MCP servers (from Claude config & Python imports)
# ✅ Authentication (OAuth from SDK download or API key)
# ✅ Trust scoring factors
```

### Capability Auto-Detection

**AIM detects capabilities from:**

1. **Import Analysis** - Infers capabilities from Python packages:
   ```python
   import requests      # → "make_api_calls"
   import smtplib       # → "send_email"
   import psycopg2      # → "access_database"
   import subprocess    # → "execute_code"
   ```

2. **Decorator Analysis** - Scans `@agent.perform_action()` calls:
   ```python
   @agent.perform_action("read_database")
   def get_users():       # → "access_database"
       pass
   ```

3. **Config File** - Explicit declarations in `~/.aim/capabilities.json`:
   ```json
   {
     "capabilities": ["custom_capability"],
     "version": "1.0.0"
   }
   ```

### MCP Server Auto-Detection

**AIM detects MCP servers from:**

1. **Claude Desktop Config** (`~/.claude/claude_desktop_config.json`):
   - 100% confidence for configured servers
   - Extracts command, args, and environment variables

2. **Python Imports** (module scanning):
   - Detects MCP packages in sys.modules
   - 90% confidence for imported packages

### Manual Override (Power Users)

You can always override auto-detection:

```python
from aim_sdk import secure

# Disable auto-detection entirely
agent = secure(
    "my-agent",
    api_key="aim_abc123",
    auto_detect=False,
    capabilities=["custom_capability"],
    talks_to=["custom-mcp-server"]
)

# Or mix auto-detection with manual specification
agent = secure(
    "my-agent",
    api_key="aim_abc123",
    capabilities=["read_files", "write_files"],  # Manual
    # talks_to will be auto-detected
)
```

### Convenience Functions

```python
from aim_sdk import auto_detect_capabilities, auto_detect_mcps

# Detect capabilities separately
capabilities = auto_detect_capabilities()
print(f"Your agent has: {capabilities}")

# Detect MCP servers separately
mcps = auto_detect_mcps()
print(f"Your agent talks to: {[m['mcpServer'] for m in mcps]}")
```

## Examples

### Quick Auto-Detection Demo (No Backend Required)
```bash
python example_auto_detection.py
```
Demonstrates automatic capability and MCP server detection.

### Full Zero-Config Demo
```bash
python example_zero_config.py
```
Shows zero-config registration and verified actions (requires backend running).

### Classic Example
```bash
python example.py
```
Traditional example with decorator-based verification.

## Requirements

All dependencies auto-install with pip:

- Python 3.8+
- requests (HTTP client)
- PyNaCl (Ed25519 cryptography)
- cryptography (secure encryption)
- keyring (system keyring integration) - **Now auto-installs!**

## License

MIT
