# AIM + MCP (Model Context Protocol) Integration Guide

**Status**: ✓ **SDK IMPLEMENTATION COMPLETE** - Backend endpoints exist, SDK ready
**Last Updated**: December 10, 2025
**Note**: Integration testing requires authentication setup

---

## Overview

Seamless integration between **AIM (Agent Identity Management)** and **MCP (Model Context Protocol)** for registration, verification, and audit logging of MCP servers and their actions.

### What This Enables

- ✓ **MCP Server Registration** with cryptographic verification
- ✓ **Dynamic Capability Discovery** - Automatically queries MCP servers for their tools
- ✓ **Auto-Attestation** - Agents automatically attest to MCP capabilities on registration
- ✓ **Action Verification** for MCP tools, resources, and prompts
- ✓ **Trust Scoring** for MCP servers based on usage history
- ✓ **Audit Trail** for all MCP server interactions
- ✓ **Centralized Registry** of trusted MCP servers
- ✓ **Security Verification** before tool/resource access

---

## What is MCP?

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

## Quick Start

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

print(f"✓ Server registered: {server_info['id']}")
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

print(f"✓ Action verified: {verification['verification_id']}")

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
- ✓ Automatic verification before execution
- ✓ Automatic result logging after completion
- ✓ Error handling and logging
- ✓ Clean, simple API

---

## Dynamic Capability Discovery

The SDK automatically discovers MCP server capabilities by querying each server using the official MCP protocol. **No hardcoded capability lists** - capabilities are discovered at runtime directly from the servers.

### How It Works

1. **SDK reads Claude Desktop config** (`~/Library/Application Support/Claude/claude_desktop_config.json`)
2. **For each MCP server**, the SDK:
   - Spawns the MCP server process
   - Sends `tools/list`, `resources/list`, `prompts/list` requests
   - Collects all available capabilities
3. **Capabilities are used for attestation** when registering MCP servers

### Example: Discover Capabilities

```python
from aim_sdk.detection import discover_mcp_capabilities

# Discover capabilities for specific MCP servers
caps = discover_mcp_capabilities(["filesystem", "github"])

for server_name, tools in caps.items():
    print(f"{server_name}: {len(tools)} capabilities")
    print(f"  Tools: {tools[:5]}...")  # First 5 tools

# Example output:
# filesystem: 14 capabilities
#   Tools: ['read_file', 'read_text_file', 'write_file', 'edit_file', 'create_directory']...
# github: 26 capabilities
#   Tools: ['create_or_update_file', 'search_repositories', 'create_repository', ...]...
```

### Auto-Discovery During Registration

When you register an agent with MCP servers, capabilities are **automatically discovered**:

```python
from aim_sdk import secure

# MCP capabilities discovered automatically!
agent = secure(
    "my-agent",
    mcp_servers=["filesystem", "github"]  # Just names - capabilities auto-discovered
)

# What happens behind the scenes:
# 1. SDK finds "filesystem" and "github" in Claude Desktop config
# 2. SDK queries each server for its actual tools via MCP protocol
# 3. Agent automatically attests to discovered capabilities
# 4. Dashboard shows real tool names (read_file, write_file, etc.)
```

### Benefits

| Without Dynamic Discovery | With Dynamic Discovery |
|---------------------------|------------------------|
| ✗ Hardcoded capability lists | ✓ Real capabilities from servers |
| ✗ Lists become outdated | ✓ Always up-to-date |
| ✗ Manual maintenance | ✓ Zero maintenance |
| ✗ Limited to known servers | ✓ Works with any MCP server |
| ✗ Users must list all tools | ✓ Automatic - just provide server names |

### Advanced: Full Detection with Tools

```python
from aim_sdk import auto_detect_mcps

# Detect all MCP servers AND their tools
detections = auto_detect_mcps(discover_tools=True, timeout_per_server=30.0)

for d in detections:
    server = d['mcpServer']
    tools = d['details'].get('capabilities', [])
    print(f"{server}: {len(tools)} tools")
```

---

## Smart Attestation System

The SDK provides **smart attestation** - intelligent, automatic attestation that builds trust in MCP servers through cryptographic verification while minimizing overhead.

### Two Types of Attestation

1. **Registration-Time Attestation**: When agents register MCP servers via `secure()` or `register_mcp()`
2. **Smart Attestation on Tool Use**: When agents use MCP tools via `use_mcp_tool()` (NEW in v1.8.0)

### Smart Attestation Triggers

Smart attestation automatically creates attestations when:

| Trigger | Description |
|---------|-------------|
| **First Use** | First time this agent uses an MCP server |
| **New Tool** | First use of a NEW tool on a known server |
| **Stale Attestation** | Attestation is >24 hours old |
| **Capability Drift** | MCP server tools have changed (added/removed) |

### Example: Smart Attestation on Tool Use

```python
from aim_sdk import secure

agent = secure("my-agent")

# Smart attestation happens automatically!
result = agent.use_mcp_tool(
    server_id="04531081-dd02-43aa-9067-a4e656de5591",
    tool_name="read_file",
    mcp_url="npx -y @modelcontextprotocol/server-filesystem /tmp",
    mcp_name="filesystem-mcp"
)

# Check if attestation was created
if result.get("attestation"):
    print(f"Attestation triggered: {result['attestation']['reason']}")
    print(f"New confidence score: {result['attestation']['confidenceScore']}%")

# Tool usage is always tracked for analytics
print(f"Tool usage count: {result['toolUsage']['count']}")
```

### Capability Drift Detection

The SDK automatically detects when MCP servers change their capabilities:

```python
# If capability drift is detected, you'll see:
# Warning: Capability drift detected (severity: medium): +2 added, -1 removed

# Drift severity levels:
# - low: New tools added (expansion)
# - medium: Tools removed (contraction)
# - high: >30% change in capabilities
```

### Registration-Time Attestation

Attestations also happen automatically at registration:

```python
from aim_sdk import secure

# Registration automatically includes attestation
agent = secure(
    "my-agent",
    mcp_servers=["filesystem"]  # Capabilities auto-discovered and attested
)

# Console output:
# ✓ Registered MCP server: filesystem
# ✓ Auto-attested MCP server 'filesystem' with 14 capabilities
```

### Manual Attestation

You can also manually attest to MCP servers:

```python
# Attest to an MCP server's capabilities
result = agent.attest_mcp(
    mcp_server_id="550e8400-e29b-41d4-a716-446655440000",
    capabilities=["read_file", "write_file", "list_directory"],
    connection_successful=True,
    health_check_passed=True,
    connection_latency_ms=45
)

print(f"Attestation ID: {result['attestationId']}")
print(f"MCP Trust Score: {result['mcpConfidenceScore']}%")
```

### Force Re-Attestation

To force an attestation even if cached:

```python
result = agent.use_mcp_tool(
    server_id="...",
    tool_name="write_file",
    mcp_url="...",
    force_attest=True  # Force attestation even if cached
)
```

---

## Supply Chain Analytics

The SDK tracks MCP tool usage for supply chain visibility, enabling:

- Dashboard visualization of MCP usage patterns
- Anomaly detection (sudden spike in dangerous tool usage)
- Compliance reporting (which agents used which tools when)
- Trust graph visualization

### View Local Analytics

```python
# Get supply chain report for this agent
report = agent.get_mcp_supply_chain_report()

print(f"MCP Servers Used: {report['mcpServerCount']}")
print(f"Total Tools: {report['totalToolsUsed']}")
print(f"Total Invocations: {report['totalToolInvocations']}")

for server_id, stats in report["servers"].items():
    print(f"\n{server_id}:")
    print(f"  Attestation Age: {stats['attestationAge']}")
    print(f"  Invocations: {stats['invocationCount']}")
    print(f"  Top Tools:")
    for tool, count in stats["topTools"]:
        print(f"    - {tool}: {count} uses")
```

### Report to Dashboard

```python
# Sync analytics to AIM backend for dashboard visualization
result = agent.report_mcp_supply_chain()

print(f"Reported {result['serversReported']} MCP servers")
print(f"Total invocations: {result['totalInvocations']}")
```

### Access Attestation Cache

```python
from aim_sdk import AttestationCache

# Access the cache directly
cache = AttestationCache(agent_id=agent.agent_id)

# Check attestation status for a server
stats = cache.get_server_stats("server-uuid")
if stats:
    print(f"Last attested: {stats['lastAttestedAt']}")
    print(f"Tools attested: {stats['capabilitiesAttested']}")
    print(f"Confidence: {stats['confidenceScore']}%")

# Check if attestation is needed
decision = cache.should_attest("server-uuid", "new_tool")
if decision["shouldAttest"]:
    print(f"Attestation needed: {decision['reason']}")
```

---

## API Reference

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

### use_mcp_tool() (NEW in v1.8.0)

Record MCP tool usage with smart attestation for supply chain security.

```python
def use_mcp_tool(
    server_id: str,
    tool_name: str,
    mcp_url: str = "",
    mcp_name: str = "",
    auto_attest: bool = True,
    force_attest: bool = False,
    discovery_timeout: float = 30.0
) -> Dict[str, Any]
```

**Parameters**:
- **`server_id`**: UUID of the MCP server being used
- **`tool_name`**: Name of the tool being used (e.g., `"read_file"`, `"search"`)
- **`mcp_url`**: URL of the MCP server (required for attestation on first use)
- **`mcp_name`**: Name of the MCP server
- **`auto_attest`**: Enable smart attestation (default: True)
- **`force_attest`**: Force attestation even if cached (default: False)
- **`discovery_timeout`**: Timeout for capability discovery in seconds (default: 30.0)

**Returns**:
```python
{
    "success": True,
    "connection_id": "uuid",
    "agent_id": "agent-uuid",
    "mcp_server_id": "server-uuid",
    "attestation": {  # Only present if attestation occurred
        "id": "attestation-uuid",
        "reason": "first_use",  # or "new_tool", "stale", "capability_drift", "forced"
        "confidenceScore": 85.5,
        "capabilitiesAttested": ["read_file", "write_file", ...],
        "driftInfo": {...}  # Only if capability drift detected
    },
    "toolUsage": {
        "count": 5,
        "firstUsed": "2025-12-01T00:00:00Z",
        "lastUsed": "2025-12-10T15:30:00Z"
    }
}
```

---

### AttestationCache

Manage attestation state for smart re-attestation decisions.

```python
from aim_sdk import AttestationCache

cache = AttestationCache(
    agent_id="agent-uuid",
    cache_dir=None,     # Optional: defaults to ~/.aim
    ttl_hours=24        # Hours before attestation is considered stale
)

# Check if attestation is needed
decision = cache.should_attest(
    mcp_server_id="server-uuid",
    tool_name="read_file",
    current_capabilities=["read_file", "write_file"]  # For drift detection
)
# Returns: {"shouldAttest": bool, "reason": str, "driftInfo": {...}}

# Detect capability drift
drift = cache.detect_capability_drift("server-uuid", ["read_file", "write_file", "new_tool"])
# Returns: {"driftDetected": bool, "addedCapabilities": [...], "removedCapabilities": [...], "severity": str}

# Record attestation
cache.record_attestation(
    mcp_server_id="server-uuid",
    attestation_id="attestation-uuid",
    capabilities=["read_file", "write_file"],
    confidence_score=85.5
)

# Record tool usage
cache.record_tool_usage("server-uuid", "read_file")

# Get supply chain report
report = cache.get_supply_chain_report()
```

---

## What Gets Logged to AIM

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

## Security Best Practices

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
    print(f"Warning: Warning: Low trust score for {server['name']}")
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

## Real-World Examples

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
    print(f"✗ Operation denied: {e}")
    # Handle denial (notify admin, log incident, etc.)
```

---

## MCP vs AIM Integration Benefits

| Without AIM | With AIM Integration |
|-------------|---------------------|
| ✗ No central registry of MCP servers | ✓ Centralized MCP server registry |
| ✗ No verification before tool execution | ✓ Cryptographic verification required |
| ✗ No audit trail of MCP actions | ✓ Complete audit trail of all actions |
| ✗ No trust scoring for servers | ✓ ML-powered trust scoring |
| ✗ Manual security reviews | ✓ Automatic security verification |
| ✗ No compliance reporting | ✓ Exportable audit record for compliance reporting |

---

## Troubleshooting

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

## Next Steps

1. **Register Your MCP Server**: Use `register_mcp_server()` to add your server
2. **Test Verification**: Try `verify_mcp_action()` with a sample action
3. **Use Action Wrapper**: Simplify with `MCPActionWrapper` for production
4. **Monitor Dashboard**: View all MCP servers at https://aim.company.com/dashboard/mcp
5. **Review Trust Scores**: Monitor and improve server trust over time

---

## Additional Resources

- **MCP Specification**: https://modelcontextprotocol.io/specification/2025-06-18
- **AIM Documentation**: [Main README](../../README.md)
- **LangChain Integration**: [LANGCHAIN_INTEGRATION.md](LANGCHAIN_INTEGRATION.md)
- **CrewAI Integration**: [CREWAI_INTEGRATION.md](CREWAI_INTEGRATION.md)

---

**Integration Status**: ✓ **SDK IMPLEMENTATION COMPLETE**
**Backend Status**: ✓ **ENDPOINTS IMPLEMENTED**
**Last Updated**: December 10, 2025
**AIM SDK Version**: 1.8.0

---

**Secure Your MCP Servers with AIM! **
