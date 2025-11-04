# AIM Python SDK

**The Stripe for AI Agent Identity - One line of code. Complete security.**

Enterprise-grade cryptographic verification with zero configuration.

## Quick Start - Zero Configuration 🚀

### The "Stripe Moment" for AI Security

```python
from aim_sdk import secure

# ONE LINE - Complete enterprise security
agent = secure("my-agent")

# That's it. Your agent now has:
# ✅ Ed25519 cryptographic signatures
# ✅ Real-time trust scoring
# ✅ Complete audit trail
# ✅ Zero configuration
```

### Manual Mode (With API Key)

```python
from aim_sdk import secure

# Still one line - just add your API key
agent = secure("my-agent", api_key="aim_abc123")
```

## Installation

**Download SDK from Dashboard (Only Method)**
1. Log in to AIM at http://localhost:3000 (or your AIM instance)
2. Go to Settings → SDK Download
3. Click "Download SDK" → Includes pre-configured credentials
4. Extract the downloaded SDK
5. You're ready to go!

**Note**: There is NO pip package. The SDK must be downloaded from your AIM instance as it contains your personal credentials and authentication tokens.

## Why AIM?

**Before AIM:** 50+ lines of boilerplate for basic agent security
**After AIM:** 1 line

### What You Get

| Feature | Description | Zero Config? |
|---------|-------------|--------------|
| **Cryptographic Identity** | Ed25519 signatures on every action | ✅ Automatic |
| **Trust Scoring** | Real-time ML risk assessment | ✅ Automatic |
| **Capability Detection** | Scans your code, finds what your agent does | ✅ Automatic |
| **MCP Server Detection** | Finds Claude Desktop configs automatically | ✅ Automatic |
| **Audit Trail** | SOC 2 compliant logging | ✅ Automatic |
| **Action Verification** | Every API call cryptographically signed | ✅ Automatic |

## Usage Examples

### 1. Zero Config (Downloaded SDK)
```python
from aim_sdk import secure
agent = secure("my-agent")  # Done. Complete security.
```

### 2. With API Key
```python
from aim_sdk import secure
agent = secure("my-agent", api_key="aim_abc123")
```

### 3. Custom Configuration
```python
from aim_sdk import secure
agent = secure(
    name="my-agent",
    api_key="aim_abc123",
    auto_detect=False,
    capabilities=["read_database", "send_email"],
    version="1.0.0"
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

## Capability Management

### How It Works

1. **Registration = Auto-Grant**: All capabilities detected during registration are automatically granted
2. **Updates = Admin Approval**: New capabilities after registration require admin review
3. **Security**: Prevents privilege escalation attacks (CVE-2025-32711)

```python
# Initial registration - capabilities auto-granted
agent = secure("my-agent")  # ✅ Can use all detected capabilities immediately

# Later, need new capability? Admin must approve
client.capabilities.request("delete_data", reason="Cleanup feature")
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

## Auto-Detection Magic 🎯

AIM automatically detects everything about your agent:

### What Gets Detected

| Source | What It Finds | Confidence |
|--------|--------------|------------|
| **Python Imports** | `requests` → API calls, `psycopg2` → Database access | 95% |
| **Claude Desktop Config** | MCP servers from `~/.claude/claude_desktop_config.json` | 100% |
| **Decorators** | `@agent.perform_action()` calls in your code | 100% |
| **Config Files** | Explicit capabilities in `~/.aim/capabilities.json` | 100% |

### Override When Needed

```python
# Full auto-detection (default)
agent = secure("my-agent")

# Partial override
agent = secure("my-agent", capabilities=["custom_capability"])

# Complete manual control
agent = secure("my-agent", auto_detect=False, capabilities=["read", "write"])
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
- keyring (system keyring integration)

All dependencies are included in the downloaded SDK's `requirements.txt`.

## License

GNU Affero General Public License v3.0 (AGPL-3.0) - See [LICENSE](../../LICENSE) for details
