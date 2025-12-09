# 🔮 Auto-Detection Guide

Let AIM automatically discover your agent type and MCP servers with **zero configuration**.

## What is Auto-Detection?

**Auto-Detection** scans your Python environment and system to automatically configure your agent:

**What it detects**:
- ✅ **Agent Type** - LangChain, CrewAI, Claude, GPT, and 15+ more from imports
- ✅ Claude Desktop MCP servers (`claude_desktop_config.json`)
- ✅ Custom MCP server configurations
- ✅ NPM-based MCP servers
- ✅ Python-based MCP servers
- ✅ Docker-based MCP servers

**Time to detect**: < 1 second
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

You can also detect the agent type without registering:

```python
from aim_sdk import auto_detect_agent_type

# Detect agent type from current imports
agent_type, reason = auto_detect_agent_type()

print(f"Detected: {agent_type}")
print(f"Reason: {reason}")
```

---

## MCP Server Auto-Detection

---

## Quick Start (10 Seconds)

```python
from aim_sdk import auto_detect_mcp_servers

# ONE LINE - Discover all MCP servers!
servers = auto_detect_mcp_servers()

print(f"Found {len(servers)} MCP servers:")
for server in servers:
    print(f"  - {server['name']}: {server['command']}")
```

**Output**:
```
Found 3 MCP servers:
  - filesystem: npx -y @modelcontextprotocol/server-filesystem
  - github: npx -y @modelcontextprotocol/server-github
  - postgres: npx -y @modelcontextprotocol/server-postgres
```

---

## How It Works

### Detection Process

```
┌────────────────────────────────────────────────────┐
│  1. Scan System for MCP Configurations            │
│     ├─ Claude Desktop config                      │
│     ├─ Custom config files                        │
│     └─ Environment variables                      │
└────────────────────────────────────────────────────┘
                      ↓
┌────────────────────────────────────────────────────┐
│  2. Parse Configuration Files                      │
│     ├─ Extract server names                       │
│     ├─ Extract commands                           │
│     ├─ Extract arguments                          │
│     └─ Extract environment variables              │
└────────────────────────────────────────────────────┘
                      ↓
┌────────────────────────────────────────────────────┐
│  3. Validate Servers                               │
│     ├─ Check command exists                       │
│     ├─ Verify server is accessible                │
│     └─ Test server responds to health checks      │
└────────────────────────────────────────────────────┘
                      ↓
┌────────────────────────────────────────────────────┐
│  4. Return Discovered Servers                      │
│     └─ Ready to register with AIM                 │
└────────────────────────────────────────────────────┘
```

---

## Claude Desktop Detection

### Default Configuration Location

**macOS**:
```
~/Library/Application Support/Claude/claude_desktop_config.json
```

**Windows**:
```
%APPDATA%/Claude/claude_desktop_config.json
```

**Linux**:
```
~/.config/Claude/claude_desktop_config.json
```

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
      "args": ["-y", "@modelcontextprotocol/server-github"],
      "env": {
        "GITHUB_TOKEN": "ghp_..."
      }
    },
    "postgres": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-postgres"],
      "env": {
        "DATABASE_URL": "postgresql://localhost/mydb"
      }
    }
  }
}
```

### Auto-Detect Claude Desktop Servers

```python
from aim_sdk import auto_detect_mcp_servers

# Automatically finds Claude Desktop config
servers = auto_detect_mcp_servers()

print(f"Discovered {len(servers)} servers from Claude Desktop:")
for server in servers:
    print(f"  Name: {server['name']}")
    print(f"  Command: {server['command']}")
    print(f"  Args: {server['args']}")
    print(f"  Environment: {list(server.get('env', {}).keys())}")
    print()
```

**Output**:
```
Discovered 3 servers from Claude Desktop:
  Name: filesystem
  Command: npx
  Args: ['-y', '@modelcontextprotocol/server-filesystem']
  Environment: ['ALLOWED_DIRECTORY']

  Name: github
  Command: npx
  Args: ['-y', '@modelcontextprotocol/server-github']
  Environment: ['GITHUB_TOKEN']

  Name: postgres
  Command: npx
  Args: ['-y', '@modelcontextprotocol/server-postgres']
  Environment: ['DATABASE_URL']
```

---

## Custom Configuration Files

### Specify Custom Config Path

```python
from aim_sdk import auto_detect_mcp_servers

# Use custom config file
servers = auto_detect_mcp_servers(
    config_path="/path/to/custom_mcp_config.json"
)

print(f"Found {len(servers)} servers in custom config")
```

### Multiple Configuration Files

```python
from aim_sdk import auto_detect_mcp_servers

# Scan multiple config files
config_paths = [
    "~/Library/Application Support/Claude/claude_desktop_config.json",
    "~/.config/my_mcp_servers.json",
    "/etc/aim/mcp_servers.json"
]

all_servers = []
for config_path in config_paths:
    try:
        servers = auto_detect_mcp_servers(config_path=config_path)
        all_servers.extend(servers)
        print(f"✅ Found {len(servers)} servers in {config_path}")
    except FileNotFoundError:
        print(f"⚠️  Config not found: {config_path}")

print(f"\nTotal: {len(all_servers)} MCP servers discovered")
```

---

## Register Discovered Servers

### Option 1: Register All at Once

```python
from aim_sdk import auto_detect_mcp_servers, register_all_mcp_servers

# Auto-detect all MCP servers
servers = auto_detect_mcp_servers()

# Register all with AIM
results = register_all_mcp_servers(servers)

print(f"✅ Registered {len(results)} MCP servers:")
for result in results:
    print(f"  - {result['name']}")
    print(f"    Server ID: {result['server_id']}")
    print(f"    Public Key: {result['public_key'][:32]}...")
    print()
```

### Option 2: Register Selectively

```python
from aim_sdk import auto_detect_mcp_servers, register_mcp_server

# Auto-detect all servers
servers = auto_detect_mcp_servers()

# Filter servers you want to register
production_servers = [
    s for s in servers
    if s['name'] in ['filesystem', 'github', 'postgres']
]

# Register selected servers
for server in production_servers:
    result = register_mcp_server(
        name=server['name'],
        command=server['command'],
        args=server.get('args', []),
        env=server.get('env', {})
    )
    print(f"✅ Registered: {server['name']}")
```

### Option 3: Automated Daily Registration

```python
from aim_sdk import auto_detect_mcp_servers, register_all_mcp_servers
import schedule
import time

def auto_register_new_servers():
    """Automatically register any new MCP servers"""
    print("Scanning for new MCP servers...")

    # Detect servers
    servers = auto_detect_mcp_servers()

    # Register new servers
    results = register_all_mcp_servers(servers)

    if results:
        print(f"✅ Registered {len(results)} new servers")
    else:
        print("No new servers found")

# Run daily at 9 AM
schedule.every().day.at("09:00").do(auto_register_new_servers)

while True:
    schedule.run_pending()
    time.sleep(60)
```

---

## Advanced Detection

### Detect Python MCP Servers

```python
from aim_sdk.detection import detect_python_mcp_servers

# Scan for Python-based MCP servers
servers = detect_python_mcp_servers(
    search_paths=[
        "~/mcp_servers",
        "/opt/mcp",
        "./custom_servers"
    ]
)

print(f"Found {len(servers)} Python MCP servers:")
for server in servers:
    print(f"  - {server['name']}: {server['file_path']}")
```

### Detect Docker MCP Servers

```python
from aim_sdk.detection import detect_docker_mcp_servers

# Scan for Docker-based MCP servers
servers = detect_docker_mcp_servers()

print(f"Found {len(servers)} Docker MCP servers:")
for server in servers:
    print(f"  - {server['name']}")
    print(f"    Image: {server['image']}")
    print(f"    Ports: {server['ports']}")
```

### Detect NPM MCP Servers

```python
from aim_sdk.detection import detect_npm_mcp_servers

# Scan for NPM-based MCP servers
servers = detect_npm_mcp_servers()

print(f"Found {len(servers)} NPM MCP servers:")
for server in servers:
    print(f"  - {server['package']}: {server['version']}")
```

---

## Validation

### Validate Before Registration

```python
from aim_sdk import auto_detect_mcp_servers
from aim_sdk.validation import validate_mcp_server

# Detect servers
servers = auto_detect_mcp_servers()

# Validate each server before registration
valid_servers = []
for server in servers:
    is_valid, issues = validate_mcp_server(server)

    if is_valid:
        print(f"✅ {server['name']}: Valid")
        valid_servers.append(server)
    else:
        print(f"❌ {server['name']}: Invalid")
        for issue in issues:
            print(f"   - {issue}")

print(f"\n{len(valid_servers)}/{len(servers)} servers are valid")
```

### Health Checks

```python
from aim_sdk.detection import health_check_server

# Check if server is responding
server = {
    "name": "filesystem",
    "command": "npx",
    "args": ["-y", "@modelcontextprotocol/server-filesystem"]
}

is_healthy = health_check_server(server)

if is_healthy:
    print(f"✅ {server['name']} is healthy")
else:
    print(f"❌ {server['name']} is not responding")
```

---

## Filtering

### Filter by Server Type

```python
from aim_sdk import auto_detect_mcp_servers

# Detect all servers
servers = auto_detect_mcp_servers()

# Filter by type
npm_servers = [s for s in servers if s['command'] == 'npx']
python_servers = [s for s in servers if s['command'] == 'python']
docker_servers = [s for s in servers if s['command'] == 'docker']

print(f"NPM servers: {len(npm_servers)}")
print(f"Python servers: {len(python_servers)}")
print(f"Docker servers: {len(docker_servers)}")
```

### Filter by Environment

```python
from aim_sdk import auto_detect_mcp_servers
import os

# Detect all servers
servers = auto_detect_mcp_servers()

# Filter by environment
environment = os.getenv("ENVIRONMENT", "development")

if environment == "production":
    # Only approved servers
    servers = [s for s in servers if s['name'] in ['github', 'postgres']]
elif environment == "staging":
    # Staging servers
    servers = [s for s in servers if s['name'] in ['github', 'postgres', 'filesystem']]
else:
    # All servers in development
    pass

print(f"{len(servers)} servers for {environment} environment")
```

---

## Configuration Management

### Export Configuration

```python
from aim_sdk import auto_detect_mcp_servers
import json

# Detect servers
servers = auto_detect_mcp_servers()

# Export to JSON
config = {
    "mcpServers": {
        server['name']: {
            "command": server['command'],
            "args": server.get('args', []),
            "env": server.get('env', {})
        }
        for server in servers
    }
}

# Save to file
with open("mcp_config_backup.json", "w") as f:
    json.dump(config, f, indent=2)

print(f"✅ Exported {len(servers)} servers to mcp_config_backup.json")
```

### Import Configuration

```python
from aim_sdk import register_mcp_server
import json

# Load configuration
with open("mcp_config_backup.json", "r") as f:
    config = json.load(f)

# Register servers from config
for name, server_config in config['mcpServers'].items():
    result = register_mcp_server(
        name=name,
        command=server_config['command'],
        args=server_config.get('args', []),
        env=server_config.get('env', {})
    )
    print(f"✅ Registered: {name}")
```

---

## Continuous Monitoring

### Monitor for New Servers

```python
from aim_sdk import auto_detect_mcp_servers, register_all_mcp_servers
import time

def monitor_new_servers():
    """Continuously monitor for new MCP servers"""
    known_servers = set()

    while True:
        # Detect current servers
        current_servers = auto_detect_mcp_servers()
        current_names = {s['name'] for s in current_servers}

        # Find new servers
        new_servers = current_names - known_servers

        if new_servers:
            print(f"🔔 Found {len(new_servers)} new servers: {new_servers}")

            # Register new servers
            servers_to_register = [
                s for s in current_servers
                if s['name'] in new_servers
            ]
            register_all_mcp_servers(servers_to_register)

            # Update known servers
            known_servers = current_names

        # Wait 5 minutes before next check
        time.sleep(300)

# Start monitoring
monitor_new_servers()
```

### Detect Configuration Changes

```python
from aim_sdk import auto_detect_mcp_servers
import hashlib
import json
import time

def hash_config(servers):
    """Generate hash of server configuration"""
    config_str = json.dumps(servers, sort_keys=True)
    return hashlib.sha256(config_str.encode()).hexdigest()

def monitor_config_changes():
    """Monitor for MCP configuration changes"""
    last_hash = None

    while True:
        # Detect servers
        servers = auto_detect_mcp_servers()
        current_hash = hash_config(servers)

        if last_hash and current_hash != last_hash:
            print("🔔 MCP configuration changed!")
            print(f"Detected {len(servers)} servers")

            # Re-register all servers to update configuration
            register_all_mcp_servers(servers)

        last_hash = current_hash

        # Wait 1 minute before next check
        time.sleep(60)

# Start monitoring
monitor_config_changes()
```

---

## Examples

### Complete Auto-Detection Workflow

```python
from aim_sdk import (
    auto_detect_mcp_servers,
    register_all_mcp_servers,
    validate_mcp_server
)
from aim_sdk.detection import health_check_server

def setup_mcp_servers():
    """
    Complete workflow:
    1. Auto-detect MCP servers
    2. Validate servers
    3. Health check servers
    4. Register with AIM
    """

    print("🔍 Step 1: Auto-detecting MCP servers...")
    servers = auto_detect_mcp_servers()
    print(f"✅ Found {len(servers)} servers\n")

    print("🔍 Step 2: Validating servers...")
    valid_servers = []
    for server in servers:
        is_valid, issues = validate_mcp_server(server)

        if is_valid:
            print(f"✅ {server['name']}: Valid")
            valid_servers.append(server)
        else:
            print(f"❌ {server['name']}: Invalid")
            for issue in issues:
                print(f"   - {issue}")

    print(f"\n{len(valid_servers)}/{len(servers)} servers passed validation\n")

    print("🔍 Step 3: Health checking servers...")
    healthy_servers = []
    for server in valid_servers:
        is_healthy = health_check_server(server)

        if is_healthy:
            print(f"✅ {server['name']}: Healthy")
            healthy_servers.append(server)
        else:
            print(f"❌ {server['name']}: Not responding")

    print(f"\n{len(healthy_servers)}/{len(valid_servers)} servers are healthy\n")

    print("🔍 Step 4: Registering servers with AIM...")
    results = register_all_mcp_servers(healthy_servers)

    print(f"✅ Registered {len(results)} servers:")
    for result in results:
        print(f"  - {result['name']}: {result['server_id']}")

    print("\n✅ All done! MCP servers are secured by AIM.")

# Run setup
setup_mcp_servers()
```

---

## Troubleshooting

### Issue: "No servers detected"

**Causes**:
1. Claude Desktop config file doesn't exist
2. Config file is in non-standard location
3. No MCP servers configured

**Solution**:
```python
# Check if Claude Desktop config exists
import os

config_paths = [
    "~/Library/Application Support/Claude/claude_desktop_config.json",  # macOS
    "%APPDATA%/Claude/claude_desktop_config.json",  # Windows
    "~/.config/Claude/claude_desktop_config.json"   # Linux
]

for path in config_paths:
    expanded_path = os.path.expanduser(path)
    if os.path.exists(expanded_path):
        print(f"✅ Config found: {expanded_path}")
    else:
        print(f"❌ Config not found: {expanded_path}")

# Specify custom path if needed
servers = auto_detect_mcp_servers(config_path="/custom/path/config.json")
```

### Issue: "Server validation failed"

**Error**: `ValidationError: Command not found`

**Solution**:
```python
# Check if command exists
import shutil

server = {
    "name": "filesystem",
    "command": "npx",
    "args": ["-y", "@modelcontextprotocol/server-filesystem"]
}

if shutil.which(server['command']):
    print(f"✅ {server['command']} is installed")
else:
    print(f"❌ {server['command']} is not installed")
    print(f"Install with: npm install -g {server['command']}")
```

### Issue: "Health check timeout"

**Error**: `TimeoutError: Server did not respond to health check`

**Solution**:
```python
# Increase timeout
from aim_sdk.detection import health_check_server

is_healthy = health_check_server(
    server,
    timeout=30  # Wait up to 30 seconds
)

if not is_healthy:
    print(f"Server {server['name']} is not responding")
    print("Check server logs for errors")
```

---

## Best Practices

### 1. Run Auto-Detection on Startup

```python
# ✅ GOOD - Detect servers on application startup
from aim_sdk import auto_detect_mcp_servers, register_all_mcp_servers

def init_mcp_servers():
    """Initialize MCP servers on startup"""
    servers = auto_detect_mcp_servers()
    register_all_mcp_servers(servers)
    print(f"✅ Initialized {len(servers)} MCP servers")

# Call during application initialization
init_mcp_servers()
```

### 2. Validate Before Registration

```python
# ✅ GOOD - Always validate before registering
from aim_sdk.validation import validate_mcp_server

servers = auto_detect_mcp_servers()

for server in servers:
    is_valid, issues = validate_mcp_server(server)
    if is_valid:
        register_mcp_server(**server)
    else:
        print(f"⚠️  Skipping invalid server: {server['name']}")
```

### 3. Monitor for Changes

```python
# ✅ GOOD - Monitor for configuration changes
import schedule

def check_for_new_servers():
    servers = auto_detect_mcp_servers()
    register_all_mcp_servers(servers)

schedule.every().hour.do(check_for_new_servers)
```

---

## Next Steps

- **[Python SDK Guide →](./python.md)** - Complete SDK reference
- **[Authentication Guide →](./authentication.md)** - Ed25519 cryptography
- **[Trust Scoring Guide →](./trust-scoring.md)** - 8-factor trust algorithm

---

<div align="center">

[🏠 Back to Home](../../README.md) • [📚 SDK Documentation](./index.md) • [💬 Get Help](https://discord.gg/opena2a)

</div>
