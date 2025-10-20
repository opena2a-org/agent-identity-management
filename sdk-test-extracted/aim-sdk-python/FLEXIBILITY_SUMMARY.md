# AIM SDK - Flexibility Spectrum Summary

## TL;DR ✨

The AIM SDK supports **three modes** of operation, from "just works" to "full control":

```
EASY MODE          BALANCED MODE          EXPERT MODE
   ↓                    ↓                       ↓
Auto-Detect      Manual + Auto          100% Manual
  3 lines          15 lines               50+ lines
  1 minute         10 minutes             30 minutes

Perfect for:     Perfect for:           Perfect for:
• Learning       • Production           • Compliance
• Prototypes     • Most agents          • Security-critical
```

---

## Quick Examples

### EASY MODE (Auto-Detect Everything)
```python
from aim_sdk import register_agent
from aim_sdk.integrations.mcp import detect_mcp_servers_from_config, auto_detect_capabilities

# One-line registration
agent = register_agent("my-agent", "https://aim.example.com", "aim_api_key")

# Auto-detect MCP servers from Claude Desktop
detect_mcp_servers_from_config(agent, agent.agent_id, auto_register=True)

# Auto-detect capabilities from MCP tools
auto_detect_capabilities(agent, agent.agent_id, auto_detect_from_mcp=True)

# Done! Everything detected automatically ✨
```

---

### BALANCED MODE (Manual + Auto) - RECOMMENDED
```python
from aim_sdk import register_agent
from aim_sdk.integrations.mcp import register_mcp_server, detect_mcp_servers_from_config, auto_detect_capabilities

# Register with manual declarations for critical components
agent = register_agent(
    name="my-agent",
    aim_url="https://aim.example.com",
    api_key="aim_api_key",
    talks_to=["custom-database-mcp"],      # Manual: Critical MCP
    capabilities=["process_payments"]     # Manual: Critical capability
)

# Auto-detect standard MCP servers (adds to manual)
detect_mcp_servers_from_config(agent, agent.agent_id, auto_register=True)

# Manually register custom MCP
register_mcp_server(
    agent, "custom-database-mcp", "http://localhost:5000",
    "ed25519_key", ["database", "query"]
)

# Combine manual + auto capabilities
manual_caps = [{"capability_type": "database_write", "risk_level": "HIGH"}]
auto_detect_capabilities(
    agent, agent.agent_id,
    detected_capabilities=manual_caps,  # Manual
    auto_detect_from_mcp=True          # Also auto-detect
)

# Best of both worlds! 🎯
```

---

### EXPERT MODE (Full Manual Control)
```python
from aim_sdk import register_agent
from aim_sdk.integrations.mcp import register_mcp_server, auto_detect_capabilities

# Exhaustive manual registration
agent = register_agent(
    name="my-agent",
    aim_url="https://aim.example.com",
    api_key="aim_api_key",
    talks_to=["mcp-1", "mcp-2", "mcp-3"],             # All MCPs
    capabilities=["cap-1", "cap-2", "cap-3", "cap-4"]  # All capabilities
)

# Manually register each MCP server
for mcp_config in all_mcp_servers:
    register_mcp_server(agent, **mcp_config)

# Manually report all capabilities (NO auto-detection)
all_capabilities = [
    {"capability_type": "file_read", "risk_level": "LOW"},
    {"capability_type": "database_write", "risk_level": "HIGH"},
    {"capability_type": "code_execution", "risk_level": "CRITICAL"}
]

auto_detect_capabilities(
    agent, agent.agent_id,
    detected_capabilities=all_capabilities,
    auto_detect_from_mcp=False  # Disable auto-detection
)

# Full control! 🔒
```

---

## Decision Tree

```
┌─────────────────────────────────────────────┐
│  What are you building?                     │
└─────────────────────────────────────────────┘
                    │
        ┌───────────┼───────────┐
        │           │           │
    Prototype   Production  Compliance
        │           │           │
        ↓           ↓           ↓
   EASY MODE   BALANCED    EXPERT MODE
               MODE (★)
```

**★ BALANCED MODE is recommended for most production agents**

---

## Feature Matrix

| Feature | Easy | Balanced | Expert |
|---------|------|----------|--------|
| Setup Time | 1 min | 10 min | 30 min |
| Code Lines | 3 | 15 | 50+ |
| Auto-Detect MCPs | ✅ | ✅ | ❌ |
| Manual MCPs | ❌ | ✅ | ✅ |
| Auto-Detect Capabilities | ✅ | ✅ | ❌ |
| Manual Capabilities | ❌ | ✅ | ✅ |
| Compliance Ready | ❌ | Partial | ✅ |
| Audit Trail | Basic | Good | Complete |

---

## Core Functions

### 1. Manual MCP Server Registration
```python
from aim_sdk.integrations.mcp import register_mcp_server

register_mcp_server(
    aim_client=agent,
    server_name="custom-mcp",
    server_url="http://localhost:5000",
    public_key="ed25519_key",
    capabilities=["tools", "resources"],
    description="Custom MCP server",
    version="1.0.0"
)
```

### 2. Auto-Detect MCP Servers
```python
from aim_sdk.integrations.mcp import detect_mcp_servers_from_config

detect_mcp_servers_from_config(
    aim_client=agent,
    agent_id=agent.agent_id,
    auto_register=True,  # Auto-register detected servers
    dry_run=False        # Execute (not just preview)
)
```

### 3. Manual Capability Declaration
```python
from aim_sdk.integrations.mcp import auto_detect_capabilities

manual_caps = [
    {
        "capability_type": "database_write",
        "capability_scope": {"database": "postgres://prod", "tables": ["users"]},
        "risk_level": "HIGH",
        "detected_via": "manual_declaration"
    }
]

auto_detect_capabilities(
    aim_client=agent,
    agent_id=agent.agent_id,
    detected_capabilities=manual_caps,
    auto_detect_from_mcp=False  # Disable auto-detection
)
```

---

## When to Use Each Mode

### Use EASY MODE When:
- 🎓 Learning the AIM platform
- 🚀 Prototyping quickly
- 🧪 Experimenting with agents
- 🏠 Development environment
- ⏰ Time-constrained demos

### Use BALANCED MODE When: (RECOMMENDED)
- 🏭 Building production agents
- 🔧 Using custom/internal MCPs
- 🎯 Need specific security controls
- 📊 Want auto-detection convenience
- ⚖️ Balancing ease + control

### Use EXPERT MODE When:
- 🔒 Security-critical applications
- 📋 Compliance requirements (SOC 2, HIPAA, GDPR)
- 🏢 Enterprise security policies
- 🔍 Maximum auditability needed
- 🎖️ Zero-trust architecture

---

## Philosophy

> **"Auto-detection makes our platform as easy as possible to use and secure agents, but manual declaration gives developers full control when they need it."**

The AIM SDK follows **progressive disclosure**:
1. **Easy to start**: Auto-detect everything (Easy Mode)
2. **Easy to customize**: Add manual declarations (Balanced Mode)
3. **Easy to secure**: Full manual control (Expert Mode)

You can **start simple and grow complex** as your security requirements evolve.

---

## Common Patterns

### Pattern 1: Start Easy, Add Manual Later
```python
# Development: Easy mode
agent = register_agent("agent", aim_url, api_key)
detect_mcp_servers_from_config(agent, agent.agent_id)

# Production: Add critical manual declarations
agent = register_agent(
    "agent", aim_url, api_key,
    talks_to=["critical-mcp"],
    capabilities=["critical_capability"],
    force_new=True  # Re-register
)
```

### Pattern 2: Preview Before Registering
```python
# Dry run to see what will be detected
result = detect_mcp_servers_from_config(
    agent, agent.agent_id,
    dry_run=True  # Preview only
)

print(f"Would detect {len(result['detected_servers'])} servers")

# Review and confirm before actual registration
```

### Pattern 3: Environment-Based Modes
```python
import os

if os.getenv("ENV") == "development":
    # Easy mode for dev
    agent = register_agent("agent", aim_url, api_key)
    detect_mcp_servers_from_config(agent, agent.agent_id)

elif os.getenv("ENV") == "production":
    # Expert mode for prod
    agent = register_agent(
        "agent", aim_url, api_key,
        talks_to=PRODUCTION_MCPS,
        capabilities=PRODUCTION_CAPABILITIES
    )
```

---

## Files in This Repository

### 1. `/examples/manual_vs_auto_registration.py`
**580+ lines** of runnable examples demonstrating:
- Easy mode (full auto-detection)
- Balanced mode (manual + auto)
- Expert mode (full manual control)
- Mode comparison table

**Run it:**
```bash
python examples/manual_vs_auto_registration.py
```

### 2. `/MANUAL_VS_AUTO_DETECTION.md`
**600+ lines** of comprehensive documentation:
- Detailed mode explanations
- API reference
- Best practices
- Troubleshooting guide
- Decision tree

### 3. `/VERIFICATION_MANUAL_DECLARATION.md`
**Complete verification report** confirming:
- ✅ Manual MCP registration works
- ✅ Manual capability declaration works
- ✅ Auto-detection can be disabled
- ✅ Hybrid mode works
- ✅ All user requirements met

---

## Quick Reference

### Registration with Manual Declarations
```python
agent = register_agent(
    name="my-agent",
    aim_url="https://aim.example.com",
    api_key="aim_1234567890abcdef",
    talks_to=["mcp-1", "mcp-2"],           # Manual MCPs
    capabilities=["cap-1", "cap-2"]        # Manual capabilities
)
```

### Disable Auto-Detection
```python
# Don't call detect_mcp_servers_from_config() - no MCP auto-detection

# Disable capability auto-detection
auto_detect_capabilities(
    agent, agent.agent_id,
    detected_capabilities=manual_caps,
    auto_detect_from_mcp=False  # NO auto-detection
)
```

### Enable Hybrid Mode
```python
# Manual declarations at registration
agent = register_agent(
    name="agent", aim_url=url, api_key=key,
    talks_to=["manual-mcp"],
    capabilities=["manual-cap"]
)

# Auto-detect additional components
detect_mcp_servers_from_config(agent, agent.agent_id)  # Adds to manual

auto_detect_capabilities(
    agent, agent.agent_id,
    detected_capabilities=manual_caps,  # Manual
    auto_detect_from_mcp=True          # Also auto-detect
)
```

---

## Summary

The AIM SDK provides **complete flexibility**:

✅ **Easy Mode**: Auto-detect everything (fastest)
✅ **Balanced Mode**: Manual + auto (recommended)
✅ **Expert Mode**: Full manual control (most secure)

Choose the mode that fits your needs, and **you can always migrate** from easy to expert as your requirements grow.

**User Requirement Met**: ✅
- Auto-detection makes the platform easy to use ✓
- Manual declaration gives developers control ✓
- Both work seamlessly together ✓

---

## Next Steps

1. **Try Easy Mode** - Get started in 1 minute
2. **Read the Guide** - See `/MANUAL_VS_AUTO_DETECTION.md`
3. **Run Examples** - Execute `/examples/manual_vs_auto_registration.py`
4. **Choose Your Mode** - Pick the one that fits your needs
5. **Build Your Agent** - Start securing your AI agents with AIM!

---

**Documentation**: `/MANUAL_VS_AUTO_DETECTION.md`
**Examples**: `/examples/manual_vs_auto_registration.py`
**Verification**: `/VERIFICATION_MANUAL_DECLARATION.md`

**Status**: ✅ COMPLETE - All functionality verified and documented
