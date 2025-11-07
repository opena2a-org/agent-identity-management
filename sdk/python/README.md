# AIM Python SDK

**Enterprise-Grade AI Agent Security - One line of code. Complete protection.**

Enterprise-grade cryptographic verification with zero configuration.

## Quick Start - Zero Configuration 🚀

### Radical Simplicity Meets Enterprise Security

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

AIM provides two decorators for action verification:

#### 1. `@agent.track_action()` - Track and Log Actions
Best for: Monitoring, logging, and audit trails (doesn't require approval)

```python
# Low-risk action - just track and log
@agent.track_action(risk_level="low")
def get_weather(city):
    """Fetch weather data - safe operation"""
    return weather_api.get(city)

# Medium-risk action - track with context
@agent.track_action(risk_level="medium", resource="database:users")
def query_database(query):
    """Query database - monitored for anomalies"""
    return db.execute(query)

# High-risk action - tracked and flagged
@agent.track_action(risk_level="high", resource="payments:charge")
def charge_credit_card(amount, card_token):
    """Charge credit card - high-risk, closely monitored"""
    return stripe.charge(amount, card_token)
```

#### 2. `@agent.require_approval()` - Require Admin Approval
Best for: Dangerous actions that need human oversight

```python
# Delete user account - requires admin approval
@agent.require_approval(risk_level="critical", resource="user:account")
def delete_user_account(user_id):
    """Delete user - REQUIRES admin approval before execution"""
    return db.execute("DELETE FROM users WHERE id = ?", user_id)

# Transfer money - requires approval
@agent.require_approval(risk_level="critical", resource="financial:transfer")
def transfer_money(from_account, to_account, amount):
    """Transfer funds - BLOCKED until admin approves"""
    return banking_api.transfer(from_account, to_account, amount)

# Deploy to production - requires approval
@agent.require_approval(risk_level="high", resource="infrastructure:deploy")
def deploy_to_production(service_name, version):
    """Deploy to prod - admin must approve first"""
    return k8s.deploy(service_name, version)
```

#### Key Differences

| Decorator | Requires Approval? | When to Use |
|-----------|-------------------|-------------|
| `@track_action()` | ❌ No - executes immediately | Monitoring, logging, low-medium risk actions |
| `@require_approval()` | ✅ Yes - blocks until admin approves | Critical actions, destructive operations, high-risk |

#### What Happens During Verification

**Both decorators**:
1. ✅ Verify agent identity with Ed25519 signature
2. ✅ Check trust score (must be above threshold)
3. ✅ Log action to immutable audit trail
4. ✅ Monitor for behavioral anomalies
5. ✅ Update trust score based on result

**`@require_approval()` additionally**:
- ⏸️ Pauses execution and creates approval request
- 📧 Notifies admin with action details
- ⏳ Waits for admin decision (approve/reject)
- ✅ Executes only if approved
- ❌ Raises `ActionDeniedError` if rejected

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

## 📁 SDK Structure

```
sdk/python/
├── aim_sdk/              # Core SDK package
├── docs/                 # Integration guides and documentation
│   ├── CREWAI_INTEGRATION.md
│   ├── LANGCHAIN_INTEGRATION.md
│   ├── MCP_INTEGRATION.md
│   ├── MICROSOFT_COPILOT_INTEGRATION.md
│   ├── ENV_CONFIG.md
│   └── VERSIONING.md    # Versioning strategy
├── examples/             # Working code examples
│   ├── example.py
│   ├── example_auto_detection.py
│   └── example_one_line_setup.py
├── tests/                # Comprehensive test suite
├── demos/                # Demo projects
├── README.md             # This file
├── CHANGELOG.md          # Version history
├── VERSION               # Current SDK version (1.0.0)
├── requirements.txt      # Dependencies
└── setup.py              # Package setup
```

## Examples

### Quick Auto-Detection Demo (No Backend Required)
```bash
python examples/example_auto_detection.py
```
Demonstrates automatic capability and MCP server detection.

### Full Zero-Config Demo
```bash
python examples/example_one_line_setup.py
```
Shows zero-config registration and verified actions (requires backend running).

### Classic Example
```bash
python examples/example.py
```
Traditional example with decorator-based verification.

**See [examples/README.md](./examples/README.md) for detailed documentation of all examples.**

## Framework Integration Guides

- **[LangChain](./docs/LANGCHAIN_INTEGRATION.md)** - Complete LangChain integration guide
- **[CrewAI](./docs/CREWAI_INTEGRATION.md)** - CrewAI agent integration
- **[MCP Servers](./docs/MCP_INTEGRATION.md)** - Model Context Protocol integration
- **[Microsoft Copilot](./docs/MICROSOFT_COPILOT_INTEGRATION.md)** - Copilot Studio integration

**See [docs/README.md](./docs/README.md) for all integration guides.**

## Requirements

All dependencies auto-install with pip:

- Python 3.8+
- requests (HTTP client)
- PyNaCl (Ed25519 cryptography)
- cryptography (secure encryption)
- keyring (system keyring integration)

All dependencies are included in the downloaded SDK's `requirements.txt`.

## Versioning

The SDK follows [Semantic Versioning 2.0.0](https://semver.org/):

```
1.0.0
│ │ │
│ │ └─── PATCH: Bug fixes
│ └───── MINOR: New features (backward-compatible)
└─────── MAJOR: Breaking changes
```

**Current Version**: 1.0.0

**Version Compatibility**:
- SDK 1.x.x works with Backend 1.x.x ✅
- SDK 1.x.x does NOT work with Backend 2.x.x ❌

**Check Your Version**:
```python
import aim_sdk
print(aim_sdk.__version__)  # "1.0.0"
```

**See Also**:
- [CHANGELOG.md](./CHANGELOG.md) - Complete version history
- [docs/VERSIONING.md](./docs/VERSIONING.md) - Versioning strategy and support policy

## License

GNU Affero General Public License v3.0 (AGPL-3.0) - See [LICENSE](../../LICENSE) for details
