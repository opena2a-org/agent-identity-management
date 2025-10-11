# AIM Python SDK

**"AIM is Stripe for AI Agent Identity"**

One-line agent registration with automatic cryptographic verification.

## Quick Start - The "Stripe Moment" 🚀

### SDK Download Mode (ZERO CONFIG!)

```python
from aim_sdk import register_agent

# ONE LINE - That's it! Everything auto-detected.
agent = register_agent("my-agent")

# ✅ Auto-detected: OAuth credentials, capabilities, MCP servers
# ✅ Auto-verified: Challenge-response verification
# ✅ Ready to use!

@agent.perform_action("send_email", resource="admin@example.com")
def send_critical_notification(message):
    send_email("admin@example.com", message)
```

### Manual Install Mode (Still Easy!)

```python
from aim_sdk import register_agent

# Requires API key, but still auto-detects capabilities + MCPs
agent = register_agent("my-agent", api_key="aim_abc123")

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
from aim_sdk import register_agent

# ONE LINE - Everything auto-detected!
agent = register_agent("my-agent")

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
from aim_sdk import register_agent

# Requires API key, but still auto-detects capabilities + MCPs
agent = register_agent("my-agent", api_key="aim_abc123")

# What happens:
# ✅ Capabilities auto-detected
# ✅ MCP servers auto-detected
# ✅ Agent registered
# ✅ Auto-verified
```

### Mode 3: Power User (Full Control)

```python
from aim_sdk import register_agent

# Disable auto-detection, specify everything manually
agent = register_agent(
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

## Auto-Detection: The "Stripe Moment" is HERE! 🎉

### Full Auto-Detection (NOW AVAILABLE!)

AIM now automatically detects **EVERYTHING**:

```python
from aim_sdk import register_agent

# ONE LINE - Zero configuration!
agent = register_agent("my-agent")

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
from aim_sdk import register_agent

# Disable auto-detection entirely
agent = register_agent(
    "my-agent",
    api_key="aim_abc123",
    auto_detect=False,
    capabilities=["custom_capability"],
    talks_to=["custom-mcp-server"]
)

# Or mix auto-detection with manual specification
agent = register_agent(
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

### Full "Stripe Moment" Demo
```bash
python example_stripe_moment.py
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
