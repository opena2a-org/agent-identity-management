# AIM MCP Call Interception - Usage Examples

This document provides comprehensive examples for using AIM's automatic MCP tool call detection and interception.

## Table of Contents
1. [Overview](#overview)
2. [Approach 1: Decorator-Based](#approach-1-decorator-based-aim_mcp_tool)
3. [Approach 2: Context Manager](#approach-2-context-manager-aim_mcp_session)
4. [Approach 3: Protocol Interceptor](#approach-3-protocol-interceptor-mcpprotocolinterceptor)
5. [Advanced Patterns](#advanced-patterns)
6. [Best Practices](#best-practices)

---

## Overview

AIM provides **three approaches** for automatic MCP tool call interception and verification:

| Approach | Best For | Granularity | Ease of Use |
|----------|----------|-------------|-------------|
| **Decorator** | Individual tool functions | Fine-grained | ⭐⭐⭐ Easy |
| **Context Manager** | Session-based workflows | Medium | ⭐⭐ Moderate |
| **Protocol Interceptor** | MCP client wrapping | Coarse-grained | ⭐ Advanced |

All approaches automatically:
- ✅ Verify MCP tool calls with AIM before execution
- ✅ Log execution results back to AIM for audit trails
- ✅ Handle errors gracefully with optional fallback
- ✅ Provide verbose logging for debugging

---

## Approach 1: Decorator-Based (@aim_mcp_tool)

**Best for:** Explicit control over individual MCP tool functions

### Basic Usage

```python
from aim_sdk import AIMClient
from aim_sdk.integrations.mcp import register_mcp_server, aim_mcp_tool

# Setup AIM
aim_client = AIMClient.from_credentials("my-agent")
server_info = register_mcp_server(
    aim_client=aim_client,
    server_name="research-mcp",
    server_url="http://localhost:3000",
    public_key="ed25519_...",
    capabilities=["tools"]
)

# Decorate MCP tool functions
@aim_mcp_tool(
    aim_client=aim_client,
    mcp_server_id=server_info["id"],
    risk_level="low",
    verbose=True
)
def web_search(query: str) -> dict:
    """Search the web via MCP (automatically verified by AIM)"""
    return mcp_client.call_tool("web_search", {"query": query})

# Use normally - verification happens automatically
results = web_search("AI safety best practices")
```

### Auto-Load Configuration

```python
from aim_sdk.integrations.mcp import aim_mcp_tool

# Decorator auto-loads agent and server from credentials
@aim_mcp_tool(
    risk_level="medium",
    auto_load_agent="my-agent",  # Load from ~/.aim/credentials.json
    verbose=True
)
def database_query(sql: str) -> list:
    """Execute database query via MCP (auto-verified)"""
    return mcp_client.call_tool("database_query", {"sql": sql})

# Just call the function - AIM handles the rest
users = database_query("SELECT * FROM users LIMIT 10")
```

### Graceful Fallback

```python
# Run without verification if AIM not configured (dev/test environments)
@aim_mcp_tool(
    graceful_fallback=True,
    verbose=True
)
def read_file(path: str) -> str:
    """Read file via MCP (runs without verification if AIM unavailable)"""
    return mcp_client.call_tool("read_file", {"path": path})

# Works even if AIM is down
content = read_file("/etc/config.json")
```

### Risk Levels

```python
# Low-risk: Read-only operations
@aim_mcp_tool(risk_level="low")
def get_weather(city: str):
    return mcp_client.call_tool("weather", {"city": city})

# Medium-risk: Data queries
@aim_mcp_tool(risk_level="medium")
def search_database(query: str):
    return mcp_client.call_tool("db_search", {"query": query})

# High-risk: Write operations
@aim_mcp_tool(risk_level="high")
def delete_records(ids: list):
    return mcp_client.call_tool("db_delete", {"ids": ids})
```

---

## Approach 2: Context Manager (aim_mcp_session)

**Best for:** Session-based MCP workflows with multiple tool calls

### Basic Usage

```python
from aim_sdk import AIMClient
from aim_sdk.integrations.mcp import aim_mcp_session, aim_mcp_tool

aim_client = AIMClient.from_credentials("my-agent")

# Context manager provides MCP server context
with aim_mcp_session(
    aim_client=aim_client,
    mcp_server_id=server_info["id"],
    session_name="research_pipeline",
    verbose=True
) as session:
    # Tools inside session automatically use session's MCP server
    @aim_mcp_tool(risk_level="low")
    def search_papers(topic: str):
        return mcp_client.call_tool("search", {"topic": topic})

    @aim_mcp_tool(risk_level="medium")
    def analyze_papers(papers: list):
        return mcp_client.call_tool("analyze", {"papers": papers})

    # Execute workflow
    papers = search_papers("quantum computing")
    session.log(f"Found {len(papers)} papers")

    analysis = analyze_papers(papers)
    session.log(f"Generated analysis")

    # Get session statistics
    stats = session.get_stats()
    print(f"Session: {stats['total_calls']} calls, {stats['successful_calls']} successful")
```

### Multi-Session Workflows

```python
# Different MCP servers for different tasks
with aim_mcp_session(aim_client, research_server_id, "research"):
    @aim_mcp_tool(risk_level="low")
    def search_docs(query: str):
        return mcp_client.call_tool("search", {"query": query})

    docs = search_docs("neural networks")

with aim_mcp_session(aim_client, database_server_id, "database"):
    @aim_mcp_tool(risk_level="medium")
    def query_users(sql: str):
        return mcp_client.call_tool("query", {"sql": sql})

    users = query_users("SELECT * FROM users")
```

### Nested Sessions

```python
# Outer session for main workflow
with aim_mcp_session(aim_client, server1_id, "main") as main:
    @aim_mcp_tool(risk_level="low")
    def get_data():
        return mcp_client.call_tool("get_data", {})

    data = get_data()
    main.log("Retrieved data")

    # Inner session for processing with different server
    with aim_mcp_session(aim_client, server2_id, "processing") as processing:
        @aim_mcp_tool(risk_level="medium")
        def process_data(data):
            return mcp_client.call_tool("process", {"data": data})

        processed = process_data(data)
        processing.log("Processed data")

    # Back to main session
    main.log("Processing complete")
```

### Session Logging and Tracking

```python
with aim_mcp_session(aim_client, server_id, "analytics", verbose=True) as session:
    @aim_mcp_tool(risk_level="low")
    def fetch_metrics():
        return mcp_client.call_tool("metrics", {})

    @aim_mcp_tool(risk_level="medium")
    def generate_report(metrics):
        return mcp_client.call_tool("report", {"metrics": metrics})

    # Custom logging
    session.log("Starting analytics pipeline")

    metrics = fetch_metrics()
    session.log(f"Fetched {len(metrics)} metrics")

    report = generate_report(metrics)
    session.log("Report generated")

    # Get detailed stats
    stats = session.get_stats()
    print(f"Session '{stats['session_name']}' complete:")
    print(f"  Total calls: {stats['total_calls']}")
    print(f"  Successful: {stats['successful_calls']}")
    print(f"  Failed: {stats['failed_calls']}")
    print(f"  Verification IDs: {stats['verification_ids']}")
    print(f"  Logs: {stats['logs']}")
```

---

## Approach 3: Protocol Interceptor (MCPProtocolInterceptor)

**Best for:** Protocol-level integration, wrapping existing MCP clients

### Basic Usage

```python
from aim_sdk import AIMClient
from aim_sdk.integrations.mcp import MCPProtocolInterceptor
from mcp import Client  # MCP SDK client

# Create standard MCP client
mcp_client = Client("http://localhost:3000")

# Wrap with AIM interceptor for automatic verification
verified_mcp = MCPProtocolInterceptor(
    mcp_client=mcp_client,
    aim_client=aim_client,
    mcp_server_id=server_info["id"],
    auto_verify=True,  # Verify all calls automatically
    verbose=True
)

# Use verified client instead of mcp_client
# All calls are automatically verified with AIM
results = verified_mcp.call_tool("web_search", {"query": "AI safety"})
```

### Drop-In Replacement

```python
# Original code using MCP client
# mcp_client = Client("http://localhost:3000")
# results = mcp_client.call_tool("search", {"q": "test"})

# Replace with verified client - no other changes needed!
verified_mcp = MCPProtocolInterceptor(
    mcp_client=Client("http://localhost:3000"),
    aim_client=aim_client,
    mcp_server_id=server_info["id"],
    auto_verify=True
)

# Same interface, automatic verification
results = verified_mcp.call_tool("search", {"q": "test"})
```

### Selective Verification

```python
# Create interceptor with auto_verify disabled
verified_mcp = MCPProtocolInterceptor(
    mcp_client=mcp_client,
    aim_client=aim_client,
    mcp_server_id=server_info["id"],
    auto_verify=False,  # Manual control
    verbose=True
)

# Low-risk calls - no verification
config = verified_mcp.call_tool("read_config", {}, verify=False)

# High-risk calls - require verification
result = verified_mcp.call_tool(
    "delete_database",
    {"db": "production"},
    verify=True,
    risk_level="high"
)
```

### All MCP Protocol Operations

```python
verified_mcp = MCPProtocolInterceptor(
    mcp_client=mcp_client,
    aim_client=aim_client,
    mcp_server_id=server_info["id"],
    auto_verify=True
)

# Tool calls
search_result = verified_mcp.call_tool(
    "web_search",
    {"query": "AI safety"},
    risk_level="low"
)

# Resource access
config_data = verified_mcp.read_resource(
    "config.json",
    risk_level="low"
)

# Prompt retrieval
prompt_data = verified_mcp.get_prompt(
    "system_prompt",
    {"role": "assistant"},
    risk_level="low"
)

# Get statistics
stats = verified_mcp.get_stats()
print(f"Interceptor stats: {stats}")
```

### Transparent Proxying

```python
# MCPProtocolInterceptor proxies all other attributes to wrapped client
verified_mcp = MCPProtocolInterceptor(
    mcp_client=mcp_client,
    aim_client=aim_client,
    mcp_server_id=server_info["id"]
)

# Intercepted methods (verified)
verified_mcp.call_tool("search", {})

# Non-intercepted methods (pass-through)
verified_mcp.connect()  # Proxied to mcp_client.connect()
verified_mcp.disconnect()  # Proxied to mcp_client.disconnect()
verified_mcp.get_capabilities()  # Proxied to mcp_client.get_capabilities()
```

---

## Advanced Patterns

### Pattern 1: Multi-Level Risk Assessment

```python
class RiskAwareMCPTools:
    """MCP tools with automatic risk-based verification"""

    def __init__(self, aim_client, mcp_server_id):
        self.aim_client = aim_client
        self.mcp_server_id = mcp_server_id

    @aim_mcp_tool(risk_level="low")
    def read_operation(self, resource: str):
        """Low-risk read operation"""
        return mcp_client.call_tool("read", {"resource": resource})

    @aim_mcp_tool(risk_level="medium")
    def write_operation(self, resource: str, data: dict):
        """Medium-risk write operation"""
        return mcp_client.call_tool("write", {"resource": resource, "data": data})

    @aim_mcp_tool(risk_level="high")
    def delete_operation(self, resource: str):
        """High-risk delete operation - requires approval"""
        return mcp_client.call_tool("delete", {"resource": resource})

# Usage
tools = RiskAwareMCPTools(aim_client, server_id)
data = tools.read_operation("users.json")  # Auto-approved (low risk)
tools.write_operation("logs.txt", {"entry": "test"})  # May require approval
tools.delete_operation("production.db")  # Likely requires approval (high risk)
```

### Pattern 2: Conditional Verification

```python
import os

# Only verify in production
is_production = os.getenv("ENVIRONMENT") == "production"

@aim_mcp_tool(
    aim_client=aim_client if is_production else None,
    mcp_server_id=server_id if is_production else None,
    graceful_fallback=True,
    verbose=is_production
)
def database_query(sql: str):
    """Verified in production, unverified in dev/test"""
    return mcp_client.call_tool("query", {"sql": sql})
```

### Pattern 3: Retry Logic with Verification

```python
from tenacity import retry, stop_after_attempt, wait_exponential

@retry(stop=stop_after_attempt(3), wait=wait_exponential(multiplier=1, min=2, max=10))
@aim_mcp_tool(
    aim_client=aim_client,
    mcp_server_id=server_id,
    risk_level="medium"
)
def resilient_api_call(endpoint: str, params: dict):
    """MCP tool with automatic retry and AIM verification"""
    return mcp_client.call_tool("api_call", {"endpoint": endpoint, "params": params})

# Will retry up to 3 times, each attempt verified by AIM
result = resilient_api_call("/users", {"limit": 10})
```

### Pattern 4: Batched Tool Calls

```python
with aim_mcp_session(aim_client, server_id, "batch_processing") as session:
    @aim_mcp_tool(risk_level="low")
    def process_item(item):
        return mcp_client.call_tool("process", {"item": item})

    # Process items in batches
    items = ["item1", "item2", "item3", "item4", "item5"]
    results = []

    for i, item in enumerate(items, 1):
        result = process_item(item)
        results.append(result)
        session.log(f"Processed {i}/{len(items)} items")

    stats = session.get_stats()
    session.log(f"Batch complete: {stats['successful_calls']}/{stats['total_calls']} successful")
```

---

## Best Practices

### 1. Choose the Right Approach

- **Decorator (`@aim_mcp_tool`)**: Use when you want explicit control over individual functions
- **Context Manager (`aim_mcp_session`)**: Use for multi-step workflows with logical grouping
- **Interceptor (`MCPProtocolInterceptor`)**: Use when wrapping existing MCP clients or for protocol-level control

### 2. Set Appropriate Risk Levels

```python
# Low risk: Read-only, public data, no side effects
@aim_mcp_tool(risk_level="low")
def get_public_data():
    pass

# Medium risk: Write operations, non-critical data
@aim_mcp_tool(risk_level="medium")
def update_cache():
    pass

# High risk: Destructive operations, production data
@aim_mcp_tool(risk_level="high")
def delete_production_data():
    pass
```

### 3. Use Verbose Logging During Development

```python
# Development
@aim_mcp_tool(verbose=True)  # See what's happening
def debug_tool():
    pass

# Production
@aim_mcp_tool(verbose=False)  # Silent operation
def production_tool():
    pass
```

### 4. Enable Graceful Fallback for Resilience

```python
# Allow operation to continue if AIM is unavailable
@aim_mcp_tool(
    graceful_fallback=True,
    verbose=True  # Log when fallback is used
)
def resilient_tool():
    pass
```

### 5. Track Sessions for Complex Workflows

```python
with aim_mcp_session(aim_client, server_id, "complex_workflow", verbose=True) as session:
    # Step 1
    session.log("Starting step 1")
    result1 = step1()

    # Step 2
    session.log("Starting step 2")
    result2 = step2(result1)

    # Step 3
    session.log("Starting step 3")
    result3 = step3(result2)

    # Final stats
    stats = session.get_stats()
    session.log(f"Workflow complete: {stats}")
```

### 6. Combine with Existing Decorators

```python
from functools import lru_cache

# Cache + AIM verification
@lru_cache(maxsize=128)
@aim_mcp_tool(risk_level="low")
def cached_lookup(key: str):
    return mcp_client.call_tool("lookup", {"key": key})

# Async + AIM verification
import asyncio

@aim_mcp_tool(risk_level="medium")
async def async_mcp_call(data):
    # Note: Current implementation is sync, but pattern works
    return await async_mcp_client.call_tool("process", {"data": data})
```

---

## Troubleshooting

### Issue: "mcp_server_id is required"

**Solution:** Either provide `mcp_server_id` explicitly or use `aim_mcp_session` context manager:

```python
# Option 1: Explicit server ID
@aim_mcp_tool(mcp_server_id=server_id)
def tool():
    pass

# Option 2: Use session context
with aim_mcp_session(aim_client, server_id):
    @aim_mcp_tool()  # Auto-detects server ID from session
    def tool():
        pass
```

### Issue: "Verification failed"

**Solution:** Check AIM server status and agent credentials:

```python
# Add verbose logging to see details
@aim_mcp_tool(verbose=True)
def tool():
    pass

# Or use graceful fallback during debugging
@aim_mcp_tool(graceful_fallback=True, verbose=True)
def tool():
    pass
```

### Issue: "Function not being verified"

**Solution:** Ensure decorator is applied correctly:

```python
# CORRECT ✅
@aim_mcp_tool(aim_client=client, mcp_server_id=server_id)
def my_tool():
    pass

# WRONG ❌ (missing parentheses)
@aim_mcp_tool
def my_tool():
    pass
```

---

## Next Steps

- See `test_mcp_call_interception.py` for complete working examples
- Check `auto_detect.py` for full implementation details
- Read MCP integration docs for more on server registration

For questions or issues, please open a GitHub issue or contact the AIM team.
