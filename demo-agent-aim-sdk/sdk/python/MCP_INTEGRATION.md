# 🔌 AIM + MCP (Model Context Protocol) Integration Guide

**Status**: ✅ **SDK IMPLEMENTATION COMPLETE** - Backend endpoints exist, SDK ready
**Last Updated**: October 8, 2025
**Note**: Integration testing requires authentication setup

---

## 🎯 Overview

Seamless integration between **AIM (Agent Identity Management)** and **MCP (Model Context Protocol)** for registration, verification, and audit logging of MCP servers and their actions.

### What This Enables

- ✅ **MCP Server Registration** with cryptographic verification
- ✅ **Action Verification** for MCP tools, resources, and prompts
- ✅ **Trust Scoring** for MCP servers based on usage history
- ✅ **Audit Trail** for all MCP server interactions
- ✅ **Centralized Registry** of trusted MCP servers
- ✅ **Security Verification** before tool/resource access

---

## 📦 What is MCP?

**Model Context Protocol (MCP)** is an open standard introduced by Anthropic in November 2024 that enables AI systems (like LLMs) to integrate with external data sources and tools.

### MCP Architecture

```
┌─────────────┐          ┌─────────────┐          ┌──────────────┐
│             │          │             │          │              │
│ MCP Client  │◄────────►│ MCP Server  │◄────────►│   Data       │
│ (LLM App)   │  JSON-   │ (Provider)  │          │   Sources    │
│             │  RPC 2.0 │             │          │              │
└─────────────┘          └─────────────┘          └──────────────┘
```

### MCP Server Capabilities

1. **Resources**: Context and data for the AI model
2. **Tools**: Functions the AI model can execute
3. **Prompts**: Templated messages and workflows

---

## 🚀 Quick Start

### Step 1: Register an MCP Server

```python
from aim_sdk import secure
from aim_sdk.integrations.mcp import register_mcp_server

# Register AIM agent (one-time setup)
agent = secure("my-agent")

# Register MCP server with AIM
server_info = register_mcp_server(
    aim_client=agent,
    server_name="research-mcp",
    server_url="http://localhost:3000",
    public_key="ed25519_your_public_key_here",
    capabilities=["tools", "resources", "prompts"],
    description="Research assistant MCP server",
    version="1.0.0"
)

print(f"✅ Server registered: {server_info['id']}")
print(f"   Status: {server_info['status']}")
print(f"   Trust Score: {server_info['trust_score']}")
```

**What Happens**:
- MCP server is registered with AIM backend
- Cryptographic public key is stored for verification
- Initial trust score is assigned (default: 50.0)
- Server appears in AIM dashboard

---

### Step 2: Verify MCP Actions

```python
from aim_sdk.integrations.mcp import verify_mcp_action

# Verify MCP tool usage before execution
verification = verify_mcp_action(
    aim_client=agent,
    mcp_server_id=server_info['id'],
    action_type="mcp_tool:web_search",
    resource="search query: AI safety",
    context={
        "tool": "web_search",
        "params": {"q": "AI safety", "limit": 10}
    },
    risk_level="low"
)

print(f"✅ Action verified: {verification['verification_id']}")

# Execute MCP tool (your implementation)
results = mcp_server.tools.web_search(query="AI safety")

# Log result back to AIM
from aim_sdk.integrations.mcp.verification import log_mcp_action_result

log_mcp_action_result(
    aim_client=agent,
    verification_id=verification['verification_id'],
    success=True,
    result_summary=f"Found {len(results)} results"
)
```

---

### Step 3: Use Action Wrapper (Recommended)

```python
from aim_sdk.integrations.mcp.verification import MCPActionWrapper

# Create wrapper for automatic verification
mcp_wrapper = MCPActionWrapper(
    aim_client=agent,
    mcp_server_id=server_info['id'],
    default_risk_level="medium",
    verbose=True
)

# Execute MCP tool with automatic verification and logging
result = mcp_wrapper.execute_tool(
    tool_name="web_search",
    tool_function=lambda: mcp_server.tools.web_search("AI safety"),
    risk_level="low",
    context={"query": "AI safety"}
)

print(f"Results: {result}")
```

**Benefits**:
- ✅ Automatic verification before execution
- ✅ Automatic result logging after completion
- ✅ Error handling and logging
- ✅ Clean, simple API

---

## 🔧 API Reference

### register_mcp_server()

Register an MCP server with the AIM backend.

```python
def register_mcp_server(
    aim_client: AIMClient,
    server_name: str,
    server_url: str,
    public_key: str,
    capabilities: List[str],
    description: str = "",
    version: str = "1.0.0",
    verification_url: Optional[str] = None
) -> Dict[str, Any]
```

**Parameters**:
- **`aim_client`**: AIMClient instance for authentication
- **`server_name`**: Name of the MCP server (unique per organization)
- **`server_url`**: Base URL of the MCP server
- **`public_key`**: Ed25519 public key for cryptographic verification
- **`capabilities`**: List of server capabilities (e.g., `["tools", "resources", "prompts"]`)
- **`description`**: Optional description
- **`version`**: Server version (default: "1.0.0")
- **`verification_url`**: Optional URL for verification challenges

**Returns**:
```python
{
    "id": "uuid",
    "name": "server-name",
    "url": "http://localhost:3000",
    "status": "pending",  # or "verified", "suspended", "revoked"
    "trust_score": 50.0,
    "capabilities": ["tools", "resources"],
    "created_at": "2025-10-08T...",
    ...
}
```

**Raises**:
- `ValueError`: If server_name, public_key, or capabilities are invalid
- `PermissionError`: If authentication fails
- `requests.exceptions.RequestException`: If registration fails

---

### list_mcp_servers()

List all MCP servers registered for the current organization.

```python
def list_mcp_servers(
    aim_client: AIMClient,
    limit: int = 50,
    offset: int = 0
) -> List[Dict[str, Any]]
```

**Example**:
```python
servers = list_mcp_servers(aim_client, limit=10)
for server in servers:
    print(f"{server['name']}: {server['status']} (trust: {server['trust_score']})")
```

---

### verify_mcp_action()

Verify an MCP action (tool call, resource access, or prompt usage).

```python
def verify_mcp_action(
    aim_client: AIMClient,
    mcp_server_id: str,
    action_type: str,
    resource: str = "",
    context: Optional[Dict[str, Any]] = None,
    risk_level: str = "medium",
    timeout_seconds: int = 5
) -> Dict[str, Any]
```

**Parameters**:
- **`mcp_server_id`**: UUID of the MCP server
- **`action_type`**: Type of action (e.g., `"mcp_tool:web_search"`, `"mcp_resource:database"`)
- **`resource`**: Resource being accessed
- **`context`**: Additional context (tool params, etc.)
- **`risk_level`**: `"low"`, `"medium"`, or `"high"`

**Returns**:
```python
{
    "verification_id": "uuid",
    "status": "approved",  # or "denied"
    "timestamp": "2025-10-08T...",
    "trust_score_impact": 0.5
}
```

---

### MCPActionWrapper

Wrapper class for automatic verification and logging.

```python
class MCPActionWrapper:
    def __init__(
        self,
        aim_client: AIMClient,
        mcp_server_id: str,
        default_risk_level: str = "medium",
        verbose: bool = False
    )

    def execute_tool(
        self,
        tool_name: str,
        tool_function: callable,
        risk_level: Optional[str] = None,
        context: Optional[Dict[str, Any]] = None
    ) -> Any
```

**Example**:
```python
mcp_wrapper = MCPActionWrapper(
    aim_client=aim_client,
    mcp_server_id="server-uuid",
    verbose=True
)

result = mcp_wrapper.execute_tool(
    tool_name="file_search",
    tool_function=lambda: search_files("*.py"),
    risk_level="low"
)
```

---

## 📊 What Gets Logged to AIM

### MCP Server Registration

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "research-mcp",
  "url": "http://localhost:3000",
  "status": "verified",
  "trust_score": 75.5,
  "capabilities": ["tools", "resources", "prompts"],
  "verification_count": 142,
  "last_verified_at": "2025-10-08T02:48:34Z"
}
```

### MCP Action Verification

```json
{
  "verification_id": "abc123-def456",
  "mcp_server_id": "550e8400-...",
  "action_type": "mcp_tool:web_search",
  "resource": "search query: AI safety",
  "context": {
    "tool": "web_search",
    "params": {"q": "AI safety", "limit": 10}
  },
  "risk_level": "low",
  "status": "approved",
  "trust_score_before": 75.5,
  "trust_score_after": 76.0,
  "timestamp": "2025-10-08T02:48:34Z"
}
```

---

## 🔒 Security Best Practices

### 1. Verify All High-Risk MCP Actions

```python
# High-risk: Database modifications
verification = verify_mcp_action(
    aim_client=aim_client,
    mcp_server_id=server_id,
    action_type="mcp_tool:database_update",
    resource="users table",
    risk_level="high"  # ← Requires higher trust score
)
```

### 2. Use Separate MCP Servers for Different Risk Levels

```python
# Low-risk server for read operations
search_server = register_mcp_server(
    aim_client=aim_client,
    server_name="search-mcp",
    capabilities=["resources"],  # Read-only
    ...
)

# High-risk server for write operations
admin_server = register_mcp_server(
    aim_client=aim_client,
    server_name="admin-mcp",
    capabilities=["tools"],  # Write operations
    ...
)
```

### 3. Monitor Trust Scores

```python
# Get server details including trust score
server = get_mcp_server(aim_client, server_id)

if server['trust_score'] < 60.0:
    print(f"⚠️  Warning: Low trust score for {server['name']}")
    # Consider suspending or reviewing server
```

### 4. Regularly Review MCP Server Usage

```python
# List all servers and their verification counts
servers = list_mcp_servers(aim_client)
for server in servers:
    print(f"{server['name']}:")
    print(f"  Trust: {server['trust_score']}")
    print(f"  Verifications: {server.get('verification_count', 0)}")
    print(f"  Last verified: {server.get('last_verified_at', 'Never')}")
```

---

## 🎯 Real-World Examples

### Example 1: Research Assistant MCP Server

```python
from aim_sdk import secure
from aim_sdk.integrations.mcp import register_mcp_server, MCPActionWrapper

# Register AIM agent
agent = secure("research-agent", AIM_URL)

# Register MCP server
research_server = register_mcp_server(
    aim_client=aim_client,
    server_name="research-assistant-mcp",
    server_url="http://localhost:3000",
    public_key="ed25519_public_key",
    capabilities=["tools", "resources"],
    description="Research assistant with web search and document analysis"
)

# Create action wrapper
mcp_wrapper = MCPActionWrapper(
    aim_client=aim_client,
    mcp_server_id=research_server['id'],
    default_risk_level="low"
)

# Execute research tools with automatic verification
search_results = mcp_wrapper.execute_tool(
    tool_name="web_search",
    tool_function=lambda: search_web("AI safety best practices"),
    context={"query": "AI safety best practices", "limit": 20}
)

document_analysis = mcp_wrapper.execute_tool(
    tool_name="analyze_document",
    tool_function=lambda: analyze_pdf("research_paper.pdf"),
    risk_level="medium",
    context={"file": "research_paper.pdf"}
)
```

### Example 2: Database Admin MCP Server

```python
# Register high-security MCP server for database operations
db_server = register_mcp_server(
    aim_client=aim_client,
    server_name="database-admin-mcp",
    server_url="http://localhost:3001",
    public_key="ed25519_public_key",
    capabilities=["tools"],
    description="Database administration server (high security)"
)

mcp_wrapper = MCPActionWrapper(
    aim_client=aim_client,
    mcp_server_id=db_server['id'],
    default_risk_level="high",  # All operations high-risk by default
    verbose=True
)

# Read operation - medium risk
users = mcp_wrapper.execute_tool(
    tool_name="query_database",
    tool_function=lambda: db.query("SELECT * FROM users"),
    risk_level="medium"
)

# Write operation - high risk (requires higher trust score)
try:
    mcp_wrapper.execute_tool(
        tool_name="delete_user",
        tool_function=lambda: db.delete_user("user123"),
        risk_level="high",
        context={"user_id": "user123", "reason": "account closure"}
    )
except PermissionError as e:
    print(f"❌ Operation denied: {e}")
    # Handle denial (notify admin, log incident, etc.)
```

---

## 📈 MCP vs AIM Integration Benefits

| Without AIM | With AIM Integration |
|-------------|---------------------|
| ❌ No central registry of MCP servers | ✅ Centralized MCP server registry |
| ❌ No verification before tool execution | ✅ Cryptographic verification required |
| ❌ No audit trail of MCP actions | ✅ Complete audit trail of all actions |
| ❌ No trust scoring for servers | ✅ ML-powered trust scoring |
| ❌ Manual security reviews | ✅ Automatic security verification |
| ❌ No compliance reporting | ✅ SOC 2/HIPAA/GDPR compliance ready |

---

## 🐛 Troubleshooting

### "Authentication failed" Error

**Cause**: AIM client is not properly authenticated

**Solution**:
```python
# Ensure agent is registered and credentials are loaded
agent = secure("my-agent", AIM_URL)

# Verify credentials are loaded
print(f"Agent ID: {aim_client.agent_id}")
```

### "MCP server already exists" Error

**Cause**: Server with that name is already registered

**Solution**:
```python
# List existing servers
servers = list_mcp_servers(aim_client)
for server in servers:
    if server['name'] == "my-server":
        print(f"Found existing server: {server['id']}")
        # Use existing server or delete and re-register
```

### "Invalid public key" Error

**Cause**: Public key format is incorrect

**Solution**:
```python
# Ensure public key is Ed25519 format (base64-encoded, 32 bytes)
# Example: "ed25519_your_64_character_base64_string_here"

import base64
# If you have raw bytes:
public_key_b64 = base64.b64encode(public_key_bytes).decode()
```

---

## 🚀 Next Steps

1. **Register Your MCP Server**: Use `register_mcp_server()` to add your server
2. **Test Verification**: Try `verify_mcp_action()` with a sample action
3. **Use Action Wrapper**: Simplify with `MCPActionWrapper` for production
4. **Monitor Dashboard**: View all MCP servers at https://aim.company.com/dashboard/mcp
5. **Review Trust Scores**: Monitor and improve server trust over time

---

## 📚 Additional Resources

- **MCP Specification**: https://modelcontextprotocol.io/specification/2025-06-18
- **AIM Documentation**: [Main README](../../README.md)
- **LangChain Integration**: [LANGCHAIN_INTEGRATION.md](LANGCHAIN_INTEGRATION.md)
- **CrewAI Integration**: [CREWAI_INTEGRATION.md](CREWAI_INTEGRATION.md)

---

**Integration Status**: ✅ **SDK IMPLEMENTATION COMPLETE**
**Backend Status**: ✅ **ENDPOINTS IMPLEMENTED**
**Last Updated**: October 8, 2025
**AIM SDK Version**: 1.0.0

---

**Secure Your MCP Servers with AIM! 🔌🔒**
