# 🔌 MCP Integration - Secure Model Context Protocol Servers

Register your MCP servers with **automatic cryptographic verification**.

## What You'll Build

An MCP server registration that:
- ✅ Automatically registers with AIM platform
- ✅ Cryptographic verification with Ed25519 signatures
- ✅ Public key discovery via `.well-known/mcp/capabilities`
- ✅ Complete audit trail of all MCP operations
- ✅ Real-time trust scoring

**Integration Time**: 3 minutes
**Code Changes**: Auto-detection (0 lines) or Manual (2 lines)
**Use Case**: Claude Desktop, MCP servers, AI assistants

---

## What is MCP?

**Model Context Protocol (MCP)** is an open protocol that lets AI assistants (like Claude Desktop) securely connect to external data sources and tools.

**Examples of MCP Servers**:
- **Database MCP**: Lets Claude query your PostgreSQL/MySQL databases
- **Filesystem MCP**: Gives Claude access to local files
- **GitHub MCP**: Connects Claude to your GitHub repositories
- **Slack MCP**: Allows Claude to send messages, read channels
- **Google Drive MCP**: Access and search Google Drive files

**The Problem**: How do you know an MCP server is legitimate and hasn't been compromised?

**AIM's Solution**: Cryptographic verification + continuous monitoring

---

## Prerequisites

1. ✅ AIM platform running ([Quick Start Guide](../quick-start.md))
2. ✅ MCP server installed (e.g., Claude Desktop)
3. ✅ `aim-sdk` installed (`pip install aim-sdk`)
4. ✅ Python 3.8+ for custom MCP servers

---

## Integration Method 1: Auto-Detection (Easiest)

AIM automatically discovers MCP servers from Claude Desktop's configuration.

### How It Works

```python
from aim_sdk import auto_detect_mcp_servers

# 🔮 ZERO CONFIG - AIM finds all your MCP servers!
detected_servers = auto_detect_mcp_servers()

print(f"Found {len(detected_servers)} MCP servers:")
for server in detected_servers:
    print(f"  - {server['name']}: {server['command']}")
```

**Expected Output**:
```
Found 3 MCP servers:
  - filesystem: npx -y @modelcontextprotocol/server-filesystem
  - github: npx -y @modelcontextprotocol/server-github
  - postgres: npx -y @modelcontextprotocol/server-postgres
```

### Register Auto-Detected Servers

```python
from aim_sdk import auto_detect_mcp_servers, register_mcp_server

# Auto-detect all MCP servers
servers = auto_detect_mcp_servers()

# Register each one with AIM
for server_config in servers:
    result = register_mcp_server(
        name=server_config['name'],
        command=server_config['command'],
        args=server_config.get('args', []),
        env=server_config.get('env', {})
    )

    print(f"✅ Registered: {server_config['name']}")
    print(f"   Server ID: {result['server_id']}")
    print(f"   Public Key: {result['public_key'][:32]}...")
    print()
```

**That's it!** All your MCP servers are now registered and monitored by AIM.

---

## Integration Method 2: Manual Registration

For custom MCP servers or fine-grained control.

### Step 1: Create an MCP Server

Create `my_mcp_server.py`:

```python
"""
Custom MCP Server - Secured with AIM
Provides weather data capabilities
"""

from aim_sdk import secure
from aim_sdk.integrations.mcp import MCPServer, MCPCapability
import requests
import os
from typing import Dict, Any

# 🔐 ONE LINE - Secure your MCP server!
aim_agent = secure(
    name="weather-mcp-server",
    aim_url=os.getenv("AIM_URL", "http://localhost:8080"),
    private_key=os.getenv("AIM_PRIVATE_KEY")
)

# ═══════════════════════════════════════════════════════════
# DEFINE MCP SERVER
# ═══════════════════════════════════════════════════════════

class WeatherMCPServer(MCPServer):
    """MCP server providing weather capabilities"""

    def __init__(self):
        super().__init__(
            name="weather-mcp-server",
            version="1.0.0",
            aim_agent=aim_agent  # 🔐 AIM integration
        )
        self.api_key = os.getenv("OPENWEATHER_API_KEY")
        self.base_url = "https://api.openweathermap.org/data/2.5/weather"

    def get_capabilities(self) -> list[MCPCapability]:
        """Define what this MCP server can do"""
        return [
            MCPCapability(
                name="get_weather",
                description="Get current weather for a city",
                parameters={
                    "city": {"type": "string", "description": "City name"},
                    "units": {
                        "type": "string",
                        "description": "Temperature units (imperial/metric)",
                        "default": "imperial"
                    }
                },
                returns={"type": "object", "description": "Weather data"}
            ),
            MCPCapability(
                name="get_forecast",
                description="Get 5-day weather forecast",
                parameters={
                    "city": {"type": "string", "description": "City name"}
                },
                returns={"type": "array", "description": "5-day forecast"}
            )
        ]

    async def get_weather(self, city: str, units: str = "imperial") -> Dict[str, Any]:
        """
        Get current weather for a city

        This method is automatically verified and logged by AIM
        """
        response = requests.get(
            self.base_url,
            params={"q": city, "appid": self.api_key, "units": units}
        )
        response.raise_for_status()

        data = response.json()
        return {
            "city": city,
            "temperature": data['main']['temp'],
            "feels_like": data['main']['feels_like'],
            "conditions": data['weather'][0]['description'],
            "humidity": data['main']['humidity'],
            "wind_speed": data['wind']['speed']
        }

    async def get_forecast(self, city: str) -> list[Dict[str, Any]]:
        """Get 5-day weather forecast"""
        response = requests.get(
            "https://api.openweathermap.org/data/2.5/forecast",
            params={"q": city, "appid": self.api_key, "units": "imperial"}
        )
        response.raise_for_status()

        data = response.json()
        forecasts = []

        for item in data['list'][:5]:  # Next 5 entries
            forecasts.append({
                "datetime": item['dt_txt'],
                "temperature": item['main']['temp'],
                "conditions": item['weather'][0]['description']
            })

        return forecasts


# ═══════════════════════════════════════════════════════════
# START MCP SERVER
# ═══════════════════════════════════════════════════════════

if __name__ == "__main__":
    server = WeatherMCPServer()

    # Start server with AIM monitoring
    server.start(
        host="0.0.0.0",
        port=8090,
        expose_capabilities_endpoint=True  # Creates /.well-known/mcp/capabilities
    )

    print("🚀 Weather MCP Server running on http://localhost:8090")
    print("📊 Capabilities: http://localhost:8090/.well-known/mcp/capabilities")
    print("🔐 Secured by AIM - all requests verified and logged")
```

### Step 2: Start Your MCP Server

```bash
# Set environment variables
export AIM_PRIVATE_KEY="your-aim-private-key"
export OPENWEATHER_API_KEY="your-openweather-key"
export AIM_URL="http://localhost:8080"

# Start the server
python my_mcp_server.py
```

**Output**:
```
🚀 Weather MCP Server running on http://localhost:8090
📊 Capabilities: http://localhost:8090/.well-known/mcp/capabilities
🔐 Secured by AIM - all requests verified and logged
```

### Step 3: Verify Capabilities Endpoint

```bash
# Check the capabilities endpoint
curl http://localhost:8090/.well-known/mcp/capabilities | jq
```

**Response**:
```json
{
  "server": {
    "name": "weather-mcp-server",
    "version": "1.0.0",
    "aim_verified": true,
    "public_key": "302a300506032b6570032100a1b2c3d4e5f6...",
    "capabilities_url": "/.well-known/mcp/capabilities"
  },
  "capabilities": [
    {
      "name": "get_weather",
      "description": "Get current weather for a city",
      "parameters": {
        "city": {"type": "string", "description": "City name"},
        "units": {
          "type": "string",
          "description": "Temperature units (imperial/metric)",
          "default": "imperial"
        }
      },
      "returns": {"type": "object", "description": "Weather data"}
    },
    {
      "name": "get_forecast",
      "description": "Get 5-day weather forecast",
      "parameters": {
        "city": {"type": "string", "description": "City name"}
      },
      "returns": {"type": "array", "description": "5-day forecast"}
    }
  ]
}
```

---

## Step 4: Use MCP Server in Claude Desktop

### Configure Claude Desktop

Edit `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "weather": {
      "command": "python",
      "args": ["/path/to/my_mcp_server.py"],
      "env": {
        "AIM_PRIVATE_KEY": "your-aim-private-key",
        "OPENWEATHER_API_KEY": "your-openweather-key",
        "AIM_URL": "http://localhost:8080"
      }
    }
  }
}
```

### Restart Claude Desktop

```bash
# Kill Claude Desktop
killall Claude

# Restart Claude Desktop
open -a Claude
```

### Use in Claude

```
User: What's the weather in San Francisco?

Claude: I'll check the weather using the weather MCP server.

[Claude calls: get_weather(city="San Francisco")]

The current weather in San Francisco is 62°F with clear skies.
It feels like 60°F with 65% humidity and winds at 8 mph.
```

**Behind the scenes** (in AIM Dashboard):
```
✅ MCP Request: get_weather(city="San Francisco")
   Server: weather-mcp-server
   Verified: ✅ Yes (Ed25519 signature valid)
   Response Time: 245ms
   Status: SUCCESS
   Trust Score Impact: +0.002 (now 0.952)
```

---

## Step 5: Check Your Dashboard (MCP Monitoring)

Open http://localhost:3000 → MCP Servers → weather-mcp-server

### Server Status

```
MCP Server: weather-mcp-server
Version: 1.0.0
Status: ✅ ACTIVE
Trust Score: 0.95 (Excellent)
Last Verified: 15 seconds ago
Total Requests: 12
Success Rate: 100%
Avg Response Time: 198ms
```

### Capabilities

```
📋 Server Capabilities:

1. get_weather
   Description: Get current weather for a city
   Parameters: city (string), units (string, optional)
   Returns: Weather data object
   Usage Count: 8 requests

2. get_forecast
   Description: Get 5-day weather forecast
   Parameters: city (string)
   Returns: Array of forecast objects
   Usage Count: 4 requests
```

### Recent Activity

```
✅ get_weather("San Francisco")      |  15s ago  |  SUCCESS  |  245ms
✅ get_weather("New York")            |  2m ago   |  SUCCESS  |  198ms
✅ get_forecast("Los Angeles")        |  5m ago   |  SUCCESS  |  312ms
✅ get_weather("Seattle")             |  8m ago   |  SUCCESS  |  176ms
```

### Trust Score Breakdown

```
✅ Verification Status:     100%  (1.00)  [All requests verified]
✅ Uptime & Availability:   100%  (1.00)  [Server always up]
✅ Request Success Rate:    100%  (1.00)  [12/12 succeeded]
✅ Security Alerts:           0   (1.00)  [No anomalies]
✅ Compliance Score:        100%  (1.00)  [Following policies]
⚠️  Age & History:          New   (0.50)  [Improves over time]
✅ Drift Detection:         None  (1.00)  [Consistent behavior]
✅ User Feedback:           None  (1.00)  [No complaints]

Overall Trust Score: 0.95 / 1.00
```

### Security Monitoring

```
🔐 Cryptographic Verification
   Public Key: 302a300506032b6570032100a1b2c3d4e5f6...
   Algorithm: Ed25519
   Last Verified: 15 seconds ago
   Total Verifications: 12
   Failed Verifications: 0

🛡️ Security Alerts: None

📊 Anomaly Detection
   Unusual request patterns: None
   Suspicious parameters: None
   Unexpected responses: None
```

---

## 🎓 Understanding MCP Integration

### What is the `.well-known/mcp/capabilities` Endpoint?

This endpoint follows the **well-known URI** pattern (like `.well-known/security.txt`) and serves as the "identity card" for your MCP server:

```json
{
  "server": {
    "name": "weather-mcp-server",
    "version": "1.0.0",
    "aim_verified": true,           // ← AIM cryptographic verification
    "public_key": "302a300506...",  // ← Ed25519 public key
    "capabilities_url": "/.well-known/mcp/capabilities"
  },
  "capabilities": [...]
}
```

### How Does Cryptographic Verification Work?

1. **MCP Server Registers with AIM**:
   ```python
   aim_agent = secure("weather-mcp-server")
   # Generates Ed25519 keypair
   # Registers public key with AIM
   ```

2. **Client Discovers Server**:
   ```bash
   curl http://localhost:8090/.well-known/mcp/capabilities
   # Client retrieves public key from capabilities endpoint
   ```

3. **Every Request is Signed**:
   ```python
   # Client sends request
   POST /get_weather
   Headers:
     X-MCP-Signature: <Ed25519 signature>
     X-MCP-Timestamp: <Unix timestamp>
   Body: {"city": "San Francisco"}
   ```

4. **AIM Verifies Each Request**:
   ```python
   # Server verifies signature
   verify_signature(public_key, signature, request_body + timestamp)

   # If valid → process request
   # If invalid → reject with 401 Unauthorized
   ```

### Why Is This Important?

**Without AIM**: Anyone can impersonate an MCP server
```
❌ Malicious server could claim to be "github-mcp"
❌ Client has no way to verify authenticity
❌ Attacker could steal credentials, inject malicious data
```

**With AIM**: Cryptographic proof of identity
```
✅ Only the real server has the private key
✅ Signature proves server identity
✅ Tampering with responses is detected
✅ Complete audit trail of all interactions
```

---

## 🚀 Advanced Usage

### MCP Server with Database Access

```python
from aim_sdk import secure
from aim_sdk.integrations.mcp import MCPServer, MCPCapability
import psycopg2

aim_agent = secure("database-mcp-server")

class DatabaseMCPServer(MCPServer):
    """MCP server for database queries"""

    def __init__(self):
        super().__init__(
            name="database-mcp-server",
            version="1.0.0",
            aim_agent=aim_agent
        )

    def get_capabilities(self):
        return [
            MCPCapability(
                name="query_users",
                description="Query users table with filters",
                parameters={
                    "filters": {
                        "type": "object",
                        "description": "Filter criteria"
                    }
                }
            )
        ]

    async def query_users(self, filters: dict = None):
        """Query users (automatically verified by AIM)"""
        with psycopg2.connect(os.getenv("DATABASE_URL")) as conn:
            cursor = conn.cursor()
            query = "SELECT * FROM users WHERE 1=1"
            params = []

            if filters and "age__gte" in filters:
                query += " AND age >= %s"
                params.append(filters["age__gte"])

            cursor.execute(query, params)
            return [dict(zip(columns, row)) for row in cursor.fetchall()]


# Start server
server = DatabaseMCPServer()
server.start(host="0.0.0.0", port=8091)
```

### MCP Server with File System Access

```python
from aim_sdk import secure
from aim_sdk.integrations.mcp import MCPServer, MCPCapability
import os
from pathlib import Path

aim_agent = secure("filesystem-mcp-server")

class FilesystemMCPServer(MCPServer):
    """MCP server for file system operations"""

    def __init__(self, allowed_directory: str):
        super().__init__(
            name="filesystem-mcp-server",
            version="1.0.0",
            aim_agent=aim_agent
        )
        self.allowed_directory = Path(allowed_directory)

    def get_capabilities(self):
        return [
            MCPCapability(
                name="list_files",
                description="List files in a directory",
                parameters={
                    "path": {"type": "string", "description": "Directory path"}
                }
            ),
            MCPCapability(
                name="read_file",
                description="Read file contents",
                parameters={
                    "path": {"type": "string", "description": "File path"}
                }
            )
        ]

    async def list_files(self, path: str = "."):
        """List files (AIM verifies this is allowed)"""
        full_path = (self.allowed_directory / path).resolve()

        # Security: ensure path is within allowed directory
        if not str(full_path).startswith(str(self.allowed_directory)):
            raise PermissionError("Access denied: path outside allowed directory")

        return [f.name for f in full_path.iterdir()]

    async def read_file(self, path: str):
        """Read file (AIM logs all reads)"""
        full_path = (self.allowed_directory / path).resolve()

        if not str(full_path).startswith(str(self.allowed_directory)):
            raise PermissionError("Access denied")

        with open(full_path, 'r') as f:
            return f.read()


# Start server (only allows access to ~/safe_directory)
server = FilesystemMCPServer(allowed_directory="~/safe_directory")
server.start(host="0.0.0.0", port=8092)
```

### Multi-Server Discovery

```python
from aim_sdk import auto_detect_mcp_servers, register_all_mcp_servers

# Auto-detect all MCP servers from Claude Desktop config
servers = auto_detect_mcp_servers()

# Register all at once
results = register_all_mcp_servers(servers)

print(f"✅ Registered {len(results)} MCP servers:")
for result in results:
    print(f"  - {result['name']}: {result['server_id']}")
    print(f"    Public Key: {result['public_key'][:32]}...")
    print(f"    Capabilities: {result['capabilities_count']} capabilities")
    print()
```

---

## 💡 Real-World Use Cases

### 1. Secure Claude Desktop with Multiple MCP Servers

```json
{
  "mcpServers": {
    "filesystem": {
      "command": "python",
      "args": ["/path/to/filesystem_mcp.py"],
      "env": {
        "AIM_PRIVATE_KEY": "fs-private-key",
        "AIM_URL": "http://localhost:8080",
        "ALLOWED_DIRECTORY": "~/Documents"
      }
    },
    "database": {
      "command": "python",
      "args": ["/path/to/database_mcp.py"],
      "env": {
        "AIM_PRIVATE_KEY": "db-private-key",
        "AIM_URL": "http://localhost:8080",
        "DATABASE_URL": "postgresql://localhost/mydb"
      }
    },
    "github": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-github"],
      "env": {
        "GITHUB_TOKEN": "ghp_..."
      }
    }
  }
}
```

**Result**: All three servers monitored by AIM with separate trust scores

### 2. Enterprise MCP Server Fleet

```python
from aim_sdk import secure, register_mcp_server

# Register multiple production MCP servers
servers = [
    {
        "name": "prod-database-mcp",
        "command": "python",
        "args": ["/opt/mcp/database_server.py"],
        "env": {"DATABASE_URL": "postgresql://prod-db/main"}
    },
    {
        "name": "prod-slack-mcp",
        "command": "python",
        "args": ["/opt/mcp/slack_server.py"],
        "env": {"SLACK_TOKEN": "xoxb-..."}
    },
    {
        "name": "prod-gdrive-mcp",
        "command": "python",
        "args": ["/opt/mcp/gdrive_server.py"],
        "env": {"GOOGLE_CREDENTIALS": "/opt/creds/gdrive.json"}
    }
]

# Register all
for server_config in servers:
    result = register_mcp_server(**server_config)
    print(f"✅ {server_config['name']}: {result['server_id']}")

# AIM dashboard now shows all servers with trust scores
```

### 3. MCP Server with Approval Workflows

```python
from aim_sdk import secure
from aim_sdk.integrations.mcp import MCPServer, MCPCapability

aim_agent = secure("sensitive-data-mcp")

class SensitiveDataMCPServer(MCPServer):
    """MCP server requiring approval for sensitive operations"""

    @aim_agent.require_approval(risk_level="high")
    async def delete_user(self, user_id: int):
        """Delete user (requires human approval)"""
        # AIM will pause execution and request approval
        with psycopg2.connect(os.getenv("DATABASE_URL")) as conn:
            cursor = conn.cursor()
            cursor.execute("DELETE FROM users WHERE id = %s", (user_id,))
            return {"deleted": cursor.rowcount > 0}

    @aim_agent.require_approval(risk_level="critical")
    async def export_all_data(self):
        """Export all data (requires urgent approval)"""
        # High-risk operation, requires immediate human review
        # ...
```

---

## 🐛 Troubleshooting

### Issue: "MCP server not detected by Claude Desktop"

**Solution**:
1. Check `claude_desktop_config.json` syntax (must be valid JSON)
2. Verify file path: `~/Library/Application Support/Claude/claude_desktop_config.json`
3. Restart Claude Desktop completely
4. Check server logs for errors

### Issue: "Signature verification failed"

**Error**: `401 Unauthorized: Invalid MCP signature`

**Solution**:
1. Verify `AIM_PRIVATE_KEY` matches the registered server
2. Check server is using correct `aim_agent` instance
3. Ensure timestamps are synchronized (within 5 minutes)
4. Verify public key in `.well-known/mcp/capabilities` is correct

### Issue: "Capabilities endpoint returns 404"

**Solution**:
```python
# Ensure expose_capabilities_endpoint=True
server.start(
    host="0.0.0.0",
    port=8090,
    expose_capabilities_endpoint=True  # ← Must be True
)
```

---

## ✅ Checklist

- [ ] MCP server registered in AIM dashboard
- [ ] Private key saved securely
- [ ] `aim-sdk` installed with MCP support
- [ ] `.well-known/mcp/capabilities` endpoint working
- [ ] Capabilities endpoint returns valid JSON
- [ ] Server added to Claude Desktop config (if applicable)
- [ ] Claude Desktop restarted
- [ ] Dashboard shows server status
- [ ] Trust score visible (should be >0.90)
- [ ] Requests logged in audit trail
- [ ] Signature verification working

**All checked?** 🎉 **Your MCP server is cryptographically verified!**

---

## 🚀 Next Steps

### Explore More Integrations

- [LangChain Integration →](./langchain.md) - Secure LangChain agents
- [CrewAI Integration →](./crewai.md) - Multi-agent teams
- [Microsoft Copilot →](./copilot.md) - Enterprise AI assistants

### Learn Advanced Features

- [SDK Documentation](../sdk/python.md) - Complete SDK reference
- [Cryptographic Auth](../sdk/authentication.md) - Ed25519 deep dive
- [Auto-Detection](../sdk/auto-detection.md) - MCP discovery

### Deploy to Production

- [Azure Deployment](../deployment/azure.md) - Production setup
- [Security Best Practices](../security/best-practices.md) - Harden deployment

---

<div align="center">

**Next**: [Microsoft Copilot Integration →](./copilot.md)

[🏠 Back to Home](../../README.md) • [📚 All Integrations](./index.md) • [💬 Get Help](https://discord.gg/opena2a)

</div>
