# AIM SDK - Manual vs Auto-Detection Guide

## Overview

The AIM SDK supports **three modes of operation** to accommodate different use cases, from quick prototyping to enterprise security requirements. This guide explains when and how to use each mode.

## Philosophy

> "Auto-detection makes our platform as easy as possible to use and secure agents, but manual declaration gives developers full control when they need it."

The AIM SDK is designed with **progressive disclosure** in mind:
- **Easy to start**: Auto-detection gets you up and running in minutes
- **Easy to customize**: Add manual declarations as your needs grow
- **Easy to secure**: Full manual control for compliance and security-critical applications

---

## The Three Modes

### 1. EASY MODE - Full Auto-Detection ✨

**When to use:**
- Quick prototyping and experimentation
- Learning the AIM platform
- Simple agents with standard MCP servers
- Development environments

**Pros:**
- ✅ Zero configuration required
- ✅ Setup in < 1 minute
- ✅ Automatically detects MCP servers from Claude Desktop
- ✅ Automatically detects capabilities from MCP tools
- ✅ Perfect for getting started

**Cons:**
- ⚠️ Less control over what gets registered
- ⚠️ May detect unwanted MCP servers
- ⚠️ Requires Claude Desktop config file

**Code Example:**

```python
from aim_sdk import register_agent
from aim_sdk.integrations.mcp import (
    detect_mcp_servers_from_config,
    auto_detect_capabilities
)

# Step 1: Register agent (no manual declarations)
agent = register_agent(
    name="my-easy-agent",
    aim_url="https://aim.example.com",
    api_key="aim_1234567890abcdef"
    # talks_to: NOT specified - will be auto-detected
    # capabilities: NOT specified - will be auto-detected
)

# Step 2: Auto-detect MCP servers from Claude Desktop
detection_result = detect_mcp_servers_from_config(
    aim_client=agent,
    agent_id=agent.agent_id,
    auto_register=True  # Automatically register new MCP servers
)

# Step 3: Auto-detect capabilities from MCP tools
capability_result = auto_detect_capabilities(
    aim_client=agent,
    agent_id=agent.agent_id,
    auto_detect_from_mcp=True  # Detect from registered MCP servers
)

# Done! Everything is automatically detected and secured
```

---

### 2. BALANCED MODE - Auto-Detect + Manual Additions ⚖️ (RECOMMENDED)

**When to use:**
- Production agents
- Agents with custom/internal MCP servers
- When you need specific security controls
- Mixed environments (standard + custom components)

**Pros:**
- ✅ Best of both worlds
- ✅ Auto-detect common components, manually declare critical ones
- ✅ Production-ready
- ✅ Flexible and scalable
- ✅ Full control where you need it

**Cons:**
- ⚠️ Requires some manual configuration
- ⚠️ Need to understand which components to declare manually

**Code Example:**

```python
from aim_sdk import register_agent
from aim_sdk.integrations.mcp import (
    register_mcp_server,
    detect_mcp_servers_from_config,
    auto_detect_capabilities
)

# Step 1: Register agent with manual declarations for critical components
agent = register_agent(
    name="my-balanced-agent",
    aim_url="https://aim.example.com",
    api_key="aim_1234567890abcdef",

    # MANUAL: Explicitly declare custom/internal MCP servers
    talks_to=[
        "custom-database-mcp",    # Internal database MCP
        "internal-api-mcp",        # Company API MCP
        "payment-gateway-mcp"      # Payment processing MCP
    ],

    # MANUAL: Explicitly declare critical capabilities
    capabilities=[
        "execute_sql_queries",     # Database access
        "process_payments",        # Payment processing
        "access_pii"               # Personal information access
    ]
)

# Step 2: Auto-detect ADDITIONAL MCP servers from Claude Desktop
# These are ADDED to the manual declarations above
detection_result = detect_mcp_servers_from_config(
    aim_client=agent,
    agent_id=agent.agent_id,
    auto_register=True  # Auto-register standard MCP servers
)
# Result: Manual MCPs + Auto-detected MCPs

# Step 3: Manually register custom MCP server
custom_mcp = register_mcp_server(
    aim_client=agent,
    server_name="custom-database-mcp",
    server_url="http://localhost:5000",
    public_key="ed25519_custom_key_here",
    capabilities=["database", "query", "transactions"],
    description="Custom PostgreSQL MCP for production database",
    version="2.1.0"
)

# Step 4: Combine manual + auto-detected capabilities
manual_capabilities = [
    {
        "capability_type": "database_write",
        "capability_scope": {
            "database": "postgres://prod-db:5432/main",
            "tables": ["users", "transactions"]
        },
        "risk_level": "HIGH",
        "detected_via": "manual_declaration"
    }
]

capability_result = auto_detect_capabilities(
    aim_client=agent,
    agent_id=agent.agent_id,
    detected_capabilities=manual_capabilities,  # Manual capabilities
    auto_detect_from_mcp=True  # ALSO auto-detect from MCP
)
# Result: Manual capabilities + Auto-detected capabilities
```

---

### 3. EXPERT MODE - Full Manual Control 🔒

**When to use:**
- Security-critical applications
- Compliance requirements (SOC 2, HIPAA, GDPR)
- Agents that don't use standard MCP servers
- Maximum control and auditability needed

**Pros:**
- ✅ 100% control over every component
- ✅ No surprises - everything is explicit
- ✅ Perfect for audit trails
- ✅ Meets strictest security requirements
- ✅ Deterministic and reproducible

**Cons:**
- ⚠️ Requires extensive manual configuration
- ⚠️ Longer setup time (20+ minutes)
- ⚠️ More code to maintain
- ⚠️ No auto-detection convenience

**Code Example:**

```python
from aim_sdk import register_agent
from aim_sdk.integrations.mcp import (
    register_mcp_server,
    auto_detect_capabilities
)

# Step 1: Register agent with COMPLETE manual declarations
agent = register_agent(
    name="my-expert-agent",
    aim_url="https://aim.example.com",
    api_key="aim_1234567890abcdef",
    agent_type="ai_agent",
    version="1.0.0",

    # MANUAL: Exhaustively list ALL MCP servers
    talks_to=[
        "filesystem-mcp-v2",
        "database-mcp-postgres",
        "github-mcp-enterprise",
        "slack-mcp-notifications",
        "email-mcp-sendgrid"
    ],

    # MANUAL: Exhaustively list ALL capabilities
    capabilities=[
        "read_files",
        "write_files",
        "execute_code",
        "database_read",
        "database_write",
        "git_operations",
        "send_notifications",
        "call_webhooks",
        "process_payments"
    ]
)

# Step 2: Manually register EACH MCP server
mcp_servers = [
    {
        "name": "filesystem-mcp-v2",
        "url": "http://localhost:3001",
        "public_key": "ed25519_filesystem_key",
        "capabilities": ["read", "write", "list"],
        "description": "File system access (read/write)",
        "version": "2.0.0"
    },
    {
        "name": "database-mcp-postgres",
        "url": "http://localhost:3002",
        "public_key": "ed25519_database_key",
        "capabilities": ["query", "transactions", "migrations"],
        "description": "PostgreSQL database access",
        "version": "1.5.0"
    }
    # ... more MCP servers
]

for mcp_config in mcp_servers:
    register_mcp_server(
        aim_client=agent,
        server_name=mcp_config["name"],
        server_url=mcp_config["url"],
        public_key=mcp_config["public_key"],
        capabilities=mcp_config["capabilities"],
        description=mcp_config["description"],
        version=mcp_config["version"]
    )

# Step 3: Manually report ALL capabilities with precise risk levels
manual_capabilities = [
    {
        "capability_type": "file_read",
        "capability_scope": {
            "paths": ["/home/user/workspace"],
            "permissions": "read"
        },
        "risk_level": "LOW",
        "detected_via": "manual_declaration"
    },
    {
        "capability_type": "database_write",
        "capability_scope": {
            "database": "postgres://prod-db:5432/main",
            "tables": ["users", "orders"],
            "operations": ["INSERT", "UPDATE", "DELETE"]
        },
        "risk_level": "CRITICAL",
        "detected_via": "manual_declaration"
    }
    # ... more capabilities
]

capability_result = auto_detect_capabilities(
    aim_client=agent,
    agent_id=agent.agent_id,
    detected_capabilities=manual_capabilities,
    auto_detect_from_mcp=False  # DISABLE auto-detection
)
```

---

## Feature Comparison

| Feature | Easy Mode | Balanced Mode | Expert Mode |
|---------|-----------|---------------|-------------|
| **Setup Time** | 1 minute | 5-10 minutes | 20+ minutes |
| **Code Required** | 3 lines | 10-20 lines | 50+ lines |
| **Auto-Detection** | 100% Auto | Hybrid | 0% Auto |
| **Manual Control** | None | Partial | 100% |
| **Security Level** | Good | Better | Best |
| **Flexibility** | Low | High | Maximum |
| **Audit Trail** | Basic | Detailed | Complete |
| **Compliance Ready** | No | Partial | Yes |
| **Recommended For** | Quick Start, Prototyping | Production, Most Agents | Compliance, Critical Ops |

---

## Decision Tree

```
Are you prototyping or learning AIM?
├─ YES → Use EASY MODE
└─ NO
    └─ Do you have compliance/security requirements?
        ├─ YES → Use EXPERT MODE
        └─ NO
            └─ Do you have custom/internal components?
                ├─ YES → Use BALANCED MODE (RECOMMENDED)
                └─ NO → Use EASY MODE or BALANCED MODE
```

---

## Common Patterns

### Pattern 1: Start Easy, Add Manual Later

```python
# Step 1: Start with easy mode during development
agent = register_agent(
    name="my-agent",
    aim_url="https://aim.example.com",
    api_key=api_key
)

detect_mcp_servers_from_config(agent, agent.agent_id, auto_register=True)
auto_detect_capabilities(agent, agent.agent_id, auto_detect_from_mcp=True)

# Step 2: Later, add critical manual declarations for production
# Re-register with force_new=True
agent = register_agent(
    name="my-agent",
    aim_url="https://aim.example.com",
    api_key=api_key,
    talks_to=["critical-database-mcp"],  # Add critical components
    capabilities=["process_payments"],    # Add critical capabilities
    force_new=True
)
```

### Pattern 2: Dry Run Before Registration

```python
# Preview what will be detected without registering
detection_result = detect_mcp_servers_from_config(
    aim_client=agent,
    agent_id=agent.agent_id,
    auto_register=False,  # Don't register yet
    dry_run=True          # Just preview
)

# Review detected servers
for server in detection_result['detected_servers']:
    print(f"Found: {server['name']} (confidence: {server['confidence']}%)")

# Decide if you want to register them or declare manually
```

### Pattern 3: Different Modes for Different Environments

```python
import os

# Development: Easy mode with auto-detection
if os.getenv("ENV") == "development":
    agent = register_agent(name="agent", aim_url=aim_url, api_key=api_key)
    detect_mcp_servers_from_config(agent, agent.agent_id, auto_register=True)

# Production: Expert mode with full manual control
elif os.getenv("ENV") == "production":
    agent = register_agent(
        name="agent",
        aim_url=aim_url,
        api_key=api_key,
        talks_to=PRODUCTION_MCP_SERVERS,
        capabilities=PRODUCTION_CAPABILITIES
    )
    # No auto-detection in production
```

---

## API Reference

### Manual MCP Server Registration

```python
from aim_sdk.integrations.mcp import register_mcp_server

register_mcp_server(
    aim_client: AIMClient,
    server_name: str,              # Unique name
    server_url: str,               # Base URL
    public_key: str,               # Ed25519 public key
    capabilities: List[str],       # List of capabilities
    description: str = "",         # Optional description
    version: str = "1.0.0",       # Optional version
    verification_url: Optional[str] = None  # Optional verification URL
) -> Dict[str, Any]
```

### Auto-Detect MCP Servers

```python
from aim_sdk.integrations.mcp import detect_mcp_servers_from_config

detect_mcp_servers_from_config(
    aim_client: AIMClient,
    agent_id: str,
    config_path: str = "~/.config/claude/claude_desktop_config.json",
    auto_register: bool = True,   # Auto-register new servers
    dry_run: bool = False         # Preview without changes
) -> Dict[str, Any]
```

### Manual Capability Declaration

```python
from aim_sdk.integrations.mcp import auto_detect_capabilities

auto_detect_capabilities(
    aim_client: AIMClient,
    agent_id: str,
    detected_capabilities: Optional[List[Dict[str, Any]]] = None,  # Manual capabilities
    auto_detect_from_mcp: bool = True  # Also auto-detect from MCP
) -> Dict[str, Any]
```

**Manual Capability Format:**

```python
{
    "capability_type": str,        # e.g., "file_read", "database_write"
    "capability_scope": Dict,      # Scope details (paths, database, etc.)
    "risk_level": str,            # "LOW", "MEDIUM", "HIGH", "CRITICAL"
    "detected_via": str           # e.g., "manual_declaration", "static_analysis"
}
```

---

## Best Practices

### 1. Start Simple, Grow Complex
- Begin with EASY MODE to learn the platform
- Move to BALANCED MODE as you add custom components
- Use EXPERT MODE only when compliance/security demands it

### 2. Manual Declarations for Critical Components
Always manually declare:
- Database connections (especially write access)
- Payment processing capabilities
- PII/sensitive data access
- Internal/proprietary APIs
- Custom security controls

### 3. Auto-Detection for Standard Components
Let auto-detection handle:
- Standard MCP servers (filesystem, github, etc.)
- Common capabilities (read files, list directories)
- Development tools
- Non-critical integrations

### 4. Document Your Choices
```python
# GOOD: Clear documentation of why manual declaration is used
agent = register_agent(
    name="payment-processor",
    aim_url=aim_url,
    api_key=api_key,

    # MANUAL: Payment processing requires explicit declaration for SOC 2 compliance
    talks_to=["stripe-mcp", "paypal-mcp"],

    # MANUAL: PCI DSS requires explicit capability tracking
    capabilities=["process_payments", "store_credit_cards", "refund_transactions"]
)
```

### 5. Use Dry Runs in Production
```python
# Always dry run before production changes
result = detect_mcp_servers_from_config(
    agent, agent.agent_id,
    dry_run=True  # Preview only
)

# Review results
print(f"Would register {result['registered_count']} new servers")

# Then execute if satisfied
if user_confirms():
    detect_mcp_servers_from_config(agent, agent.agent_id, dry_run=False)
```

---

## Troubleshooting

### "Claude Desktop config not found"
**Problem:** Auto-detection can't find Claude Desktop config file.

**Solutions:**
1. Install Claude Desktop
2. Use manual mode instead
3. Specify custom config path:
```python
detect_mcp_servers_from_config(
    agent, agent.agent_id,
    config_path="~/custom/path/claude_desktop_config.json"
)
```

### "No capabilities detected"
**Problem:** Auto-detection found no capabilities.

**Solutions:**
1. Ensure MCP servers are registered first
2. Use manual capability declaration:
```python
auto_detect_capabilities(
    agent, agent.agent_id,
    detected_capabilities=[{
        "capability_type": "file_read",
        "capability_scope": {"paths": ["/workspace"]},
        "risk_level": "LOW",
        "detected_via": "manual"
    }],
    auto_detect_from_mcp=False
)
```

### "MCP server already exists"
**Problem:** Trying to register a duplicate MCP server.

**Solutions:**
1. Check existing servers first:
```python
from aim_sdk.integrations.mcp import list_mcp_servers
servers = list_mcp_servers(agent)
```
2. Skip duplicate registration
3. Delete and re-register if needed

---

## Examples

See `/examples/manual_vs_auto_registration.py` for complete working examples of all three modes.

Run the examples:
```bash
cd aim-sdk-python
python examples/manual_vs_auto_registration.py
```

---

## Summary

The AIM SDK provides **progressive disclosure** through three modes:

1. **EASY MODE**: Perfect for getting started - auto-detect everything
2. **BALANCED MODE**: Best for production - mix auto-detection with manual control
3. **EXPERT MODE**: Maximum security - full manual control

Choose the mode that fits your use case, and remember: **you can always start simple and add manual declarations later** as your security requirements grow.

The philosophy is simple:
> "Make it as easy as possible to get started, but give experts full control when they need it."

---

## Additional Resources

- **API Reference**: https://docs.aim.example.com/api
- **SDK Guide**: https://docs.aim.example.com/sdk
- **Security Best Practices**: https://docs.aim.example.com/security
- **Compliance Guide**: https://docs.aim.example.com/compliance
- **Examples Repository**: https://github.com/opena2a-org/aim-sdk-examples
