# AIM MCP Interception - Quick Reference

## Choose Your Approach

| **Approach** | **When to Use** | **Code Example** |
|--------------|-----------------|------------------|
| **Decorator** | Individual functions | `@aim_mcp_tool(...)` |
| **Context Manager** | Multi-step workflows | `with aim_mcp_session(...):` |
| **Interceptor** | Wrap existing client | `MCPProtocolInterceptor(...)` |

---

## 1. Decorator (`@aim_mcp_tool`)

### Minimal Example
```python
from aim_sdk.integrations.mcp import aim_mcp_tool

@aim_mcp_tool(
    aim_client=client,
    mcp_server_id=server_id,
    risk_level="low"
)
def my_tool(arg):
    return mcp_client.call_tool("tool_name", {"arg": arg})
```

### Auto-Load Example
```python
# Auto-loads from ~/.aim/credentials.json
@aim_mcp_tool(risk_level="medium", verbose=True)
def my_tool(arg):
    return mcp_client.call_tool("tool_name", {"arg": arg})
```

### Parameters
- `aim_client`: AIMClient instance (optional, auto-loads)
- `mcp_server_id`: Server UUID (optional with session)
- `risk_level`: "low", "medium", "high" (default: "medium")
- `verbose`: Print debug info (default: False)
- `graceful_fallback`: Run without AIM if unavailable (default: True)

---

## 2. Context Manager (`aim_mcp_session`)

### Basic Example
```python
from aim_sdk.integrations.mcp import aim_mcp_session, aim_mcp_tool

with aim_mcp_session(client, server_id, "session_name", verbose=True) as session:
    @aim_mcp_tool(risk_level="low")
    def tool1():
        return mcp_client.call_tool("tool1", {})

    result = tool1()
    session.log("Custom log message")

    stats = session.get_stats()
```

### Nested Sessions
```python
with aim_mcp_session(client, server1_id, "outer"):
    # Uses server1

    with aim_mcp_session(client, server2_id, "inner"):
        # Uses server2

    # Back to server1
```

### Session Methods
- `session.log(message)`: Add custom log
- `session.get_stats()`: Get session statistics
- `session.track_call(verification_id, success)`: Manual tracking

---

## 3. Protocol Interceptor (`MCPProtocolInterceptor`)

### Basic Example
```python
from aim_sdk.integrations.mcp import MCPProtocolInterceptor

# Wrap existing MCP client
verified_mcp = MCPProtocolInterceptor(
    mcp_client=mcp_client,
    aim_client=client,
    mcp_server_id=server_id,
    auto_verify=True
)

# Use as drop-in replacement
result = verified_mcp.call_tool("search", {"q": "test"})
```

### Selective Verification
```python
verified_mcp = MCPProtocolInterceptor(..., auto_verify=False)

# No verification
data = verified_mcp.call_tool("read", {}, verify=False)

# With verification
result = verified_mcp.call_tool("delete", {"id": 123}, verify=True, risk_level="high")
```

### Methods
- `call_tool(name, args, verify=None, risk_level=None)`
- `read_resource(uri, verify=None, risk_level=None)`
- `get_prompt(name, args, verify=None, risk_level=None)`
- `get_stats()`: Get interceptor statistics

---

## Common Patterns

### Pattern: Multi-Level Risk
```python
class RiskAwareTools:
    @aim_mcp_tool(risk_level="low")
    def read(self, resource):
        pass  # Auto-approved

    @aim_mcp_tool(risk_level="high")
    def delete(self, resource):
        pass  # Requires approval
```

### Pattern: Conditional Verification
```python
import os
IS_PROD = os.getenv("ENV") == "production"

@aim_mcp_tool(
    aim_client=client if IS_PROD else None,
    graceful_fallback=True
)
def tool():
    pass  # Verified in prod, unverified in dev
```

### Pattern: Retry Logic
```python
from tenacity import retry, stop_after_attempt

@retry(stop=stop_after_attempt(3))
@aim_mcp_tool(risk_level="medium")
def resilient_tool():
    return mcp_client.call_tool("api_call", {})
```

---

## Risk Levels

| Level | Use For | Approval |
|-------|---------|----------|
| **low** | Read-only, public data | Auto-approved |
| **medium** | Queries, non-critical writes | May require approval |
| **high** | Destructive ops, prod data | Likely requires approval |

---

## Imports

```python
from aim_sdk import AIMClient
from aim_sdk.integrations.mcp import (
    # Registration
    register_mcp_server,
    list_mcp_servers,

    # Manual verification
    verify_mcp_action,

    # Automatic verification
    aim_mcp_tool,           # Decorator
    aim_mcp_session,        # Context manager
    MCPProtocolInterceptor  # Interceptor
)
```

---

## Setup Flow

```python
# 1. Register agent
client = AIMClient.from_credentials("my-agent")

# 2. Register MCP server
server = register_mcp_server(
    aim_client=client,
    server_name="my-mcp",
    server_url="http://localhost:3000",
    public_key="ed25519_...",
    capabilities=["tools"]
)

# 3. Choose approach and use
@aim_mcp_tool(aim_client=client, mcp_server_id=server["id"])
def my_tool():
    pass
```

---

## Troubleshooting

### "mcp_server_id is required"
```python
# Solution 1: Provide explicitly
@aim_mcp_tool(mcp_server_id=server_id)

# Solution 2: Use session context
with aim_mcp_session(client, server_id):
    @aim_mcp_tool()  # Auto-detects
```

### "Verification failed"
```python
# Add verbose logging
@aim_mcp_tool(verbose=True)

# Or use graceful fallback
@aim_mcp_tool(graceful_fallback=True)
```

### Decorator not working
```python
# WRONG ❌
@aim_mcp_tool
def tool():
    pass

# CORRECT ✅
@aim_mcp_tool()
def tool():
    pass
```

---

## Testing

```bash
# Run tests
python test_mcp_call_interception.py

# Expected output:
# ✅ PASSED: Decorator-Based Auto-Detection
# ✅ PASSED: Context Manager Auto-Detection
# ✅ PASSED: Protocol Interceptor Auto-Detection
# ✅ PASSED: Graceful Fallback
```

---

## Full Documentation

- **Usage Examples**: `EXAMPLES_MCP_INTERCEPTION.md`
- **Implementation Details**: `MCP_INTERCEPTION_IMPLEMENTATION.md`
- **Source Code**: `aim_sdk/integrations/mcp/auto_detect.py`
- **Tests**: `test_mcp_call_interception.py`
