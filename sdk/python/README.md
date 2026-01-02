# AIM Python SDK

**Open Source AI Agent Security - One line of code. Complete protection.**

Cryptographic verification with zero configuration.

---

## See It Work in 60 Seconds!

```bash
# After downloading and extracting the SDK:
cd aim-sdk-python
pip install -e .
python demo_agent.py
```

Open your AIM dashboard side-by-side and watch it update in real-time as you trigger actions!

**Dashboard URL:** http://localhost:3000/dashboard/agents

---

## Quick Start - Zero Configuration

### One Line. Complete Security.

```python
from aim_sdk import secure
import langchain  # SDK auto-detects your framework!

# ONE LINE - Complete security
agent = secure("my-agent")  # Auto-detected as "langchain" agent

# That's it. Your agent now has:
# ✅ Agent type auto-detected from imports
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

### Step 1: Download SDK from Dashboard
1. Log in to AIM at http://localhost:3000 (or your AIM instance)
2. Go to **Settings → SDK Download**
3. Click **"Download SDK"** → Includes pre-configured credentials

### Step 2: Extract to Your Projects Folder
```bash
# Extract anywhere you keep your projects
cd ~/projects  # or ~/dev, ~/Desktop, etc.
unzip ~/Downloads/aim-sdk-python.zip
cd aim-sdk-python
pip install -e .
```

### Step 3: Run the Demo Agent!
```bash
python demo_agent.py
```

The demo agent is an interactive menu that lets you trigger different actions (weather checks, product searches, user lookups, etc.) and watch your AIM dashboard update in real-time.

**Note**: There is NO pip package. The SDK must be downloaded from your AIM instance as it contains your personal credentials and authentication tokens.

## Why AIM?

**Before AIM:** 50+ lines of boilerplate for basic agent security
**After AIM:** 1 line

### What You Get

| Feature | Description | Zero Config? |
|---------|-------------|--------------|
| **Agent Type Detection** | Auto-detects LangChain, CrewAI, Claude, GPT, etc. from imports | ✅ Automatic |
| **Cryptographic Identity** | Ed25519 signatures on every action | ✅ Automatic |
| **Trust Scoring** | Real-time ML risk assessment | ✅ Automatic |
| **Capability Detection** | Scans your code, finds what your agent does | ✅ Automatic |
| **MCP Server Detection** | Finds Claude Desktop configs automatically | ✅ Automatic |
| **MCP Capability Discovery** | Queries MCP servers for actual tools | ✅ Automatic |
| **Audit Trail** | SOC 2 compliant logging | ✅ Automatic |
| **Action Verification** | Every API call cryptographically signed | ✅ Automatic |

### Supported Agent Types

The SDK automatically detects your agent type from imported packages:

| Category | Types | Auto-Detected From |
|----------|-------|-------------------|
| **Frameworks** | LangChain, LlamaIndex, CrewAI, AutoGen, LangGraph, Haystack, Semantic Kernel | `langchain`, `crewai`, `autogen`, etc. |
| **LLM Providers** | Claude, GPT, Gemini, Llama, Mistral, Cohere | `anthropic`, `openai`, `google.generativeai`, etc. |
| **Copilots** | Copilot, Assistant, Chatbot | For interactive assistants |
| **Autonomous** | AutoGPT, BabyAGI | Self-directed agents |

**Priority**: Frameworks are detected before LLM providers. If you import both `langchain` and `anthropic`, the agent type will be `langchain`.

**Explicit Override**:
```python
from aim_sdk import secure, AgentType

agent = secure("my-agent", agent_type=AgentType.CREWAI)
```

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

### 3. Full-Featured Registration
```python
from aim_sdk import secure, AgentType

agent = secure(
    "my-ai-assistant",
    agent_type=AgentType.LANGCHAIN,  # CREWAI, AUTOGEN, GPT, CLAUDE, etc.
    capabilities=["db:read", "api:call"],
    mcp_servers=["filesystem"],
    version="1.0.0",  # Note: version defaults to "1.0.0" if undeclared
    description="Customer support AI agent",
    tags=["production", "customer-facing", "gpt-4", "support-team"],
    metadata={
        "model": "gpt-4",
        "department": "support"
    }
)
```

### Performing Verified Actions

Use `@agent.perform_action()` for all action verification. Risk level is **auto-detected** from the capability name:

```python
# Auto risk detection - db:read detected as "low" risk
@agent.perform_action(capability="db:read")
def get_customer(customer_id: str):
    return {"id": customer_id, "name": "Jane Doe"}

# Auto risk detection - db:delete detected as "high" risk
@agent.perform_action(capability="db:delete")
def delete_customer(customer_id: str):
    return db.execute("DELETE FROM customers WHERE id = ?", customer_id)

# Explicit risk level override when needed
@agent.perform_action(capability="api:call", risk_level="medium", resource="external-api")
def call_external_api(endpoint):
    return requests.get(endpoint)

# Critical action with JIT access - requires admin approval
@agent.perform_action(capability="payment:refund", jit_access=True)
def process_refund(order_id: str, amount: float):
    """Waits for admin approval before executing"""
    return stripe.refund(order_id, amount)
```

#### Risk Levels

| Risk Level | Approval Required? | When to Use |
|------------|-------------------|-------------|
| `low` | No - executes immediately | Read operations, safe actions |
| `medium` | No - executes immediately | Write operations, data modification |
| `high` | No (unless `jit_access=True`) | Sensitive operations, closely monitored |
| `critical` | Recommended with `jit_access=True` | Destructive operations, requires approval |

#### JIT (Just-In-Time) Access

For critical operations that require human oversight, add `jit_access=True`:

```python
@agent.perform_action(risk_level="critical", jit_access=True, timeout_seconds=300)
def transfer_money(from_account, to_account, amount):
    """Transfer funds - BLOCKED until admin approves"""
    return banking_api.transfer(from_account, to_account, amount)
```

With `jit_access=True`:
- Pauses execution and creates approval request in AIM dashboard
- Notifies admin with action details
- Waits for admin decision (approve/reject)
- Times out after `timeout_seconds` if no decision

#### What Happens During Verification

Every `@agent.perform_action()` call:
1. Verifies agent identity with Ed25519 signature
2. Checks trust score (must be above threshold)
3. Logs action to immutable audit trail
4. Monitors for behavioral anomalies
5. Updates trust score based on result

## Enforcement Mode: Strict vs Monitoring

AIM supports two enforcement modes that control what happens when verification fails. **As of v1.4.0, enforcement mode is configured by admins in the dashboard**, not via environment variables.

### Configure in Dashboard (Recommended)

Go to **Settings → Security → Policies** in your AIM dashboard to set your organization's enforcement mode:

| Mode | What Happens on Verification Failure | Best For |
|------|-------------------------------------|----------|
| **Monitoring** (default) | ⚠️ Warning logged, function executes anyway | Development, testing, gradual rollout |
| **Strict** | ❌ Exception raised, function blocked | Production, compliance (SOC 2, HIPAA) |

The SDK automatically reads your organization's enforcement setting from the backend - no code changes needed.

### Monitoring Mode (Default)

In **monitoring mode**, verification failures are logged but **functions still execute**:
- ⚠️ Verification failures log a warning
- ✅ Functions execute anyway (safe for testing)
- 📊 All events are tracked in the dashboard
- 🔔 Dashboard shows: "Executed despite denial (monitoring mode)"

This is ideal for:
- Development and testing
- Gradual rollout of verification
- Understanding your agent's behavior before enforcement

### Strict Mode (Production)

In **strict mode**, verification failures **block execution**:
- ❌ Verification failures raise `PermissionError`
- 🛑 Functions are NOT executed if verification fails
- 📊 Dashboard shows: "Action BLOCKED (strict mode enforced)"
- 🔐 Provides actual security enforcement

This is required for:
- Production deployments
- Security-critical applications
- Compliance requirements (SOC 2, HIPAA)

### Environment Variable Override (Testing Only)

For local testing, you can override the backend setting with the `AIM_STRICT_MODE` environment variable:

```bash
# Force strict mode locally (overrides backend setting)
export AIM_STRICT_MODE=true
python my_agent.py

# Force monitoring mode locally (overrides backend setting)
export AIM_STRICT_MODE=false
python my_agent.py

# Use backend setting (default - no env var needed)
unset AIM_STRICT_MODE
python my_agent.py
```

**Note**: The environment variable is for testing purposes only. In production, configure enforcement mode in the dashboard for consistent behavior across all agents.

### Example Code

```python
from aim_sdk import secure
from aim_sdk.decorators import aim_verify

agent = secure("my-agent")

@aim_verify(agent, action_type="database_delete", risk_level="critical")
def delete_all_records():
    """Behavior depends on org's enforcement mode (set in dashboard)"""
    db.execute("DELETE FROM records")

# If org is in strict mode, this raises PermissionError on verification failure
# If org is in monitoring mode, this logs a warning but executes anyway
try:
    delete_all_records()
except PermissionError as e:
    print(f"Action blocked by strict mode: {e}")
```

### Execution Status Tracking

The SDK automatically reports execution status back to the AIM backend, so you can see exactly what happened:

1. **Verified + Executed**: Action was approved and ran successfully
2. **Denied + Executed (Monitoring Mode)**: Action was denied but ran anyway (warning logged)
3. **Denied + Blocked (Strict Mode)**: Action was denied and prevented from running

This information is visible in the AIM dashboard's verification history and alert details.

## Capability Management

### How It Works

1. **Registration = Auto-Grant**: All capabilities detected during registration are automatically granted
2. **Updates = Admin Approval**: New capabilities after registration require admin review
3. **Security**: Prevents privilege escalation attacks (CVE-2025-32711)

```python
# Initial registration - capabilities auto-granted
agent = secure("my-agent")  # ✅ Can use all detected capabilities immediately

# Later, need new capability? Admin must approve
client.capabilities.request("data:delete", reason="Cleanup feature")
```

### Request Additional Capabilities

Use `request_capability()` to request capabilities that weren't detected during registration:

```python
# Request a new capability (requires admin approval)
result = agent.request_capability(
    capability_type="db:write",  # namespace:action format
    reason="Need to update user preferences"
)

if result["status"] == "pending":
    print(f"Request {result['id']} submitted - awaiting admin approval")
elif result["status"] == "approved":
    print("Capability granted!")
```

## MCP Server Registration

### Automatic Registration with Dynamic Capability Discovery

When you specify MCP servers, the SDK **automatically discovers their capabilities** by querying each server using the MCP protocol:

```python
from aim_sdk import secure

# Just provide server names - capabilities discovered automatically!
agent = secure(
    "my-agent",
    mcp_servers=["filesystem", "github"]  # Capabilities auto-discovered
)

# What happens:
# 1. SDK finds these servers in Claude Desktop config
# 2. SDK queries each server via MCP protocol (tools/list)
# 3. Agent auto-attests with discovered capabilities
# 4. Console shows: "Auto-attested MCP server 'filesystem' with 14 capabilities"
```

### Register MCP Servers Programmatically

Use `register_mcp()` for manual registration:

```python
# Register an MCP server
mcp_result = agent.register_mcp(
    server_name="my-database-server",
    server_url="http://localhost:3001",
    capabilities=["db:read", "db:write", "data:delete"]  # namespace:action format
)

print(f"MCP Server registered: {mcp_result['id']}")
```

This is useful when:
- Auto-detection doesn't find your MCP servers
- You're connecting to dynamically provisioned MCP servers
- You want to pre-register servers before connecting

### Discover MCP Capabilities Programmatically

```python
from aim_sdk.detection import discover_mcp_capabilities

# Discover what tools MCP servers actually have
caps = discover_mcp_capabilities(["filesystem", "github"])

for server, tools in caps.items():
    print(f"{server}: {len(tools)} tools - {tools[:3]}...")
# filesystem: 14 tools - ['read_file', 'write_file', 'edit_file']...
# github: 26 tools - ['create_repository', 'create_issue', 'create_pull_request']...
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

## Capability Declaration 🎯

AIM uses two reliable methods to detect your agent's capabilities:

### How to Declare Capabilities

| Method | Description | Confidence |
|--------|-------------|------------|
| **Decorators** | Use `@agent.perform_action(capability="...")` in your code | 100% |
| **Config File** | Explicit capabilities in `~/.aim/capabilities.json` | 100% |

> **Note**: Import-based detection (scanning `sys.modules`) is disabled by default because it's too noisy—almost every Python agent would show the same generic capabilities. Use decorators or config files for accurate capability declaration.

### Option 1: Use Decorators (Recommended)

```python
from aim_sdk import secure

agent = secure("my-agent")

# Declare capabilities with decorators
@agent.perform_action(capability="db:read")
def read_database():
    pass

@agent.perform_action(capability="email:send")
def send_email():
    pass
```

### Option 2: Use Config File

Create `~/.aim/capabilities.json`:
```json
{
  "capabilities": ["db:read", "db:write", "email:send"]
}
```

### Option 3: Explicit Declaration

```python
# Declare capabilities at registration time
agent = secure("my-agent", capabilities=["api:call", "db:read"])

# Or disable auto-detection entirely
agent = secure("my-agent", auto_detect=False, capabilities=["db:read", "db:write"])
```

## Advanced Options

### Force New Credentials (`force_new`)

By default, the SDK caches credentials locally in `~/.aim/agents/{name}.json`. Use `force_new=True` to bypass cached credentials and reconnect to the backend:

```python
# Force a fresh connection (ignores local credential cache)
agent = secure(
    "my-agent",
    force_new=True  # Bypasses local cache, reconnects to backend
)
```

**When to use `force_new=True`:**
- Your cached credentials became stale (e.g., after database reset)
- You want to update agent metadata or capabilities on the server
- You're debugging credential issues
- Testing credential rotation

**Note:** If an agent with the same name already exists on the server, `force_new=True` will reconnect to it (with fresh local credentials). To create a completely new agent, use a different name.

## 📁 SDK Structure

```
sdk/python/
├── aim_sdk/              # Core SDK package
├── docs/                 # Integration guides and documentation
│   ├── CREWAI_INTEGRATION.md
│   ├── LANGCHAIN_INTEGRATION.md
│   ├── MCP_INTEGRATION.md
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
├── VERSION               # Current SDK version (1.14.0)
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
1.1.0
│ │ │
│ │ └─── PATCH: Bug fixes
│ └───── MINOR: New features (backward-compatible)
└─────── MAJOR: Breaking changes
```

**Current Version**: 1.14.0

**Version Compatibility**:
- SDK 1.x.x works with Backend 1.x.x ✅
- SDK 1.x.x does NOT work with Backend 2.x.x ❌

**Check Your Version**:
```python
import aim_sdk
print(aim_sdk.__version__)  # "1.14.0"
```

**See Also**:
- [CHANGELOG.md](./CHANGELOG.md) - Complete version history
- [docs/VERSIONING.md](./docs/VERSIONING.md) - Versioning strategy and support policy

## License

Apache License 2.0 (Apache-2.0) - See [LICENSE](../../LICENSE) for details
