# 🔮 Auto-Detection Guide

Let AIM automatically discover your agent type and MCP servers with **zero configuration**.

## What is Auto-Detection?

**Auto-Detection** scans your Python environment to configure your agent:

**What it detects**:
- ✅ **Agent Type** - LangChain, CrewAI, Claude, GPT, and 15+ more from imports
- ✅ **Capabilities** - inferred from the packages your agent imports
- ✅ **MCP servers** in the Claude Desktop config (`claude_desktop_config.json`)
- ✅ **MCP packages** your Python process has loaded or installed
- ✅ **MCP servers your agent calls**, when you record each call with `track_mcp_call()`

**Code required**: 1 line

---

## Agent Type Auto-Detection

The SDK automatically detects your agent type based on imported Python packages.

### How It Works

```python
from aim_sdk import secure
import langchain  # SDK detects this!

agent = secure("my-agent")  # Auto-detected as "langchain"
```

Output:
```
✅ Auto-detected agent type: langchain (detected 'langchain' framework import)
```

### Supported Agent Types

| Category | Types | Auto-Detected From |
|----------|-------|-------------------|
| **Frameworks** (highest priority) | LangChain, LlamaIndex, CrewAI, AutoGen, LangGraph, Haystack, Semantic Kernel | `langchain`, `llama_index`, `crewai`, `autogen`, `langgraph`, `haystack`, `semantic_kernel` |
| **Autonomous Agents** | AutoGPT, BabyAGI | `autogpt`, `babyagi` |
| **LLM Providers** (fallback) | Claude, GPT, Gemini, Llama, Mistral, Cohere | `anthropic`, `openai`, `google.generativeai`, `llama`, `mistralai`, `cohere` |
| **Copilots & Assistants** | Copilot, Assistant, Chatbot | For interactive assistants |
| **Generic** | Custom | When nothing specific detected |

### Detection Priority

Frameworks are detected **before** LLM providers. This makes sense because:

- If you import `langchain` + `anthropic`, you're building a **LangChain agent** that uses Claude
- The framework defines the agent's architecture and behavior
- The LLM is just the underlying model

```python
import langchain       # Framework
import anthropic       # LLM Provider

agent = secure("my-agent")
# Result: "langchain" (framework takes priority)
```

### Explicit Override

You can always specify the agent type manually:

```python
from aim_sdk import secure, AgentType

# Use AgentType constants for type safety
agent = secure("my-agent", agent_type=AgentType.CREWAI)

# Or use string directly
agent = secure("my-agent", agent_type="crewai")
```

### Available AgentType Constants

```python
from aim_sdk import AgentType

# LLM Providers
AgentType.CLAUDE        # "claude"
AgentType.GPT           # "gpt"
AgentType.GEMINI        # "gemini"
AgentType.LLAMA         # "llama"
AgentType.MISTRAL       # "mistral"
AgentType.COHERE        # "cohere"

# Frameworks
AgentType.LANGCHAIN     # "langchain"
AgentType.LLAMAINDEX    # "llamaindex"
AgentType.CREWAI        # "crewai"
AgentType.AUTOGEN       # "autogen"
AgentType.LANGGRAPH     # "langgraph"
AgentType.HAYSTACK      # "haystack"
AgentType.SEMANTIC_KERNEL  # "semantic_kernel"

# Copilots & Assistants
AgentType.COPILOT       # "copilot"
AgentType.ASSISTANT     # "assistant"
AgentType.CHATBOT       # "chatbot"

# Autonomous Agents
AgentType.AUTOGPT       # "autogpt"
AgentType.BABYAGI       # "babyagi"

# Generic
AgentType.CUSTOM        # "custom"
AgentType.AI_AGENT      # "ai_agent" (legacy)

# Helper methods
AgentType.all_types()              # List all types
AgentType.frameworks()             # List framework types
AgentType.llm_providers()          # List LLM provider types
```

### Programmatic Detection

You can also detect the agent type and capabilities without registering:

```python
from aim_sdk import auto_detect_agent_type, auto_detect_capabilities

# Detect agent type from current imports
agent_type, reason = auto_detect_agent_type()

print(f"Detected: {agent_type}")
print(f"Reason: {reason}")

# The capabilities secure() declares when auto_detect=True (the default)
print(auto_detect_capabilities())
```

---

## MCP Server Auto-Detection

### Quick Start (10 Seconds)

```python
from aim_sdk import auto_detect_mcps

# ONE LINE - Discover MCP servers!
detections = auto_detect_mcps()

print(f"Found {len(detections)} MCP servers:")
for detection in detections:
    print(f"  - {detection['mcpServer']} ({detection['detectionMethod']})")
```

**Output** (with the example config below):
```
Found 3 MCP servers:
  - filesystem (claude_config)
  - github (claude_config)
  - postgres (claude_config)
```

---

## How It Works

`auto_detect_mcps()` runs two detection methods and returns their detections
together. Neither starts a process or opens a connection.

| Method | `detectionMethod` | What it reads |
|--------|-------------------|---------------|
| Claude Desktop config | `claude_config` | The `mcpServers` entries of the first config file found (below) |
| Python imports | `sdk_import` | Loaded modules (`sys.modules`) and installed packages whose names are MCP servers |

A third method, `sdk_runtime`, reports the servers you recorded with
`track_mcp_call()`; see [Runtime Tracking](#runtime-tracking).

### Detection Format

Each detection is a dict:

```json
{
  "mcpServer": "filesystem",
  "detectionMethod": "claude_config",
  "confidence": 100.0,
  "details": {
    "configPath": "<home>/Library/Application Support/Claude/claude_desktop_config.json",
    "command": "npx",
    "args": ["-y", "@modelcontextprotocol/server-filesystem"]
  },
  "sdkVersion": "2.0.0",
  "timestamp": "2026-10-11T09:00:00+00:00"
}
```

---

## Claude Desktop Detection

### Configuration Locations

The SDK reads the first of these files that exists, in this order:

1. `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS)
2. `%APPDATA%\Claude\claude_desktop_config.json` (Windows)
3. `~/.config/Claude/claude_desktop_config.json`
4. `~/.claude/claude_desktop_config.json` (legacy)

It does not read Cursor's `~/.cursor/mcp.json` or any project-local
`mcp.json`: an agent's working directory is not a trustworthy source of server
commands. There is no option to pass a different config path.

An unreadable or malformed config produces one warning naming the file and the
parse error, and no detections; it never raises.

### Example Configuration File

```json
{
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem"],
      "env": {
        "ALLOWED_DIRECTORY": "~/Documents"
      }
    },
    "github": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-github"]
    },
    "postgres": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-postgres"]
    }
  }
}
```

---

## Discovering Tools

`auto_detect_mcps(discover_tools=True)` also asks each configured server for its
tools. **This executes every server command in the Claude Desktop config**: each
`command`/`args` pair is launched as a child process, queried over stdio and shut
down. `timeout_per_server` (default 10 seconds) bounds each server.

```python
from aim_sdk import auto_detect_mcps

detections = auto_detect_mcps(discover_tools=True, timeout_per_server=30.0)
```

Only use it on a machine whose Claude Desktop config you trust.

---

## Using MCPDetector Directly

`auto_detect_mcps()` is a shortcut for `MCPDetector().detect_all()`. Use the
detector to run one method at a time:

```python
from aim_sdk import MCPDetector

detector = MCPDetector()

config_detections = detector.detect_from_claude_config()
import_detections = detector.detect_from_imports()
all_detections = detector.detect_all()

# detect_all() plus the servers recorded with track_mcp_call()
with_runtime = detector.detect_all_with_runtime()
```

### Runtime Tracking

Record each MCP call your agent makes, and the detector reports those servers
too (`detectionMethod: "sdk_runtime"`):

```python
from aim_sdk import MCPDetector, track_mcp_call

def call_tool(server, tool, arguments):
    track_mcp_call(server, tool_name=tool)
    return mcp_session.call_tool(tool, arguments)

detections = MCPDetector().detect_all_with_runtime()
```

---

## Register Discovered Servers

### Option 1: Report Every Detection

```python
from aim_sdk import secure, auto_detect_mcps

agent = secure("my-agent")

result = agent.report_detections(auto_detect_mcps())
```

### Option 2: Register Selectively

```python
from aim_sdk import secure, auto_detect_mcps

agent = secure("my-agent")

# Only the servers in the Claude Desktop config, except postgres
for detection in auto_detect_mcps():
    if detection["detectionMethod"] == "claude_config" and detection["mcpServer"] != "postgres":
        agent.register_mcp(detection["mcpServer"], detection_method="auto_config")
```

`register_mcp()` takes an MCP server ID or name that is registered in your
organization; `detection_method` is one of `manual` (the default), `auto_sdk`,
`auto_config` or `cli`.

### Option 3: At Registration

```python
from aim_sdk import secure

# Scan the Claude Desktop config while registering (opt-in)
agent = secure("my-agent", auto_detect_mcp=True)

# Or name the servers the agent talks to
agent = secure("my-agent", mcp_servers=["github", "filesystem"])
```

---

## Troubleshooting

### Issue: "No servers detected"

**Solution**:
```python
from pathlib import Path

# Check which Claude Desktop config exists
for candidate in [
    Path.home() / "Library/Application Support/Claude/claude_desktop_config.json",
    Path.home() / ".config/Claude/claude_desktop_config.json",
    Path.home() / ".claude/claude_desktop_config.json",
]:
    print(candidate, candidate.exists())
```

If the file exists but nothing is detected, look for the SDK's warning: it names
the file and the JSON error. On Windows, check `%APPDATA%\Claude\`.

### Issue: "Tool discovery timed out"

**Solution**: Raise the per-server limit, or leave `discover_tools` off:

```python
from aim_sdk import auto_detect_mcps

detections = auto_detect_mcps(discover_tools=True, timeout_per_server=30.0)
```

---

## Best Practices

### 1. Detect on Startup

```python
from aim_sdk import secure, auto_detect_mcps

def setup_agent():
    agent = secure("my-agent")
    agent.report_detections(auto_detect_mcps())
    return agent

# Call during application initialization
agent = setup_agent()
```

### 2. Keep Tool Discovery Opt-In

`discover_tools=True` runs every configured server command. Leave it off unless
you need the tool lists and trust the config.

### 3. Track Runtime Calls

Calling `track_mcp_call()` on each MCP call lets AIM see servers that are not in
the Claude Desktop config.

---

## Next Steps

- **[Python SDK Guide →](./python.md)** - Complete SDK reference
- **[Authentication Guide →](./authentication.md)** - Ed25519 cryptography
- **[Trust Scoring Guide →](./trust-scoring.md)** - 9-factor trust algorithm

---

<div align="center">

[🏠 Back to Home](../../README.md) • [📚 SDK Documentation](./index.md) • [💬 Get Help](https://discord.gg/opena2a)

</div>
