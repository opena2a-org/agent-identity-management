# MCP Auto-Detection - Quick Reference

## 🚀 Quick Start (30 seconds)

```python
from aim_sdk import AIMClient
from aim_sdk.integrations.mcp import detect_mcp_servers_from_config

# Initialize client
aim_client = AIMClient(
    agent_id="your-agent-id",
    public_key="your-public-key",
    private_key="your-private-key",
    aim_url="https://aim.example.com"
)

# Auto-detect and register
result = detect_mcp_servers_from_config(aim_client, "your-agent-id")

print(f"✅ Detected {len(result['detected_servers'])} servers")
```

## 📋 Function Signature

```python
detect_mcp_servers_from_config(
    aim_client: AIMClient,           # Required: AIM client instance
    agent_id: str,                   # Required: Agent UUID
    config_path: str = "~/.config/claude/claude_desktop_config.json",  # Optional
    auto_register: bool = True,      # Optional: Auto-register new servers
    dry_run: bool = False           # Optional: Preview without changes
) -> Dict[str, Any]
```

## 🎯 Common Use Cases

### 1. Basic Auto-Detection
```python
result = detect_mcp_servers_from_config(aim_client, agent_id)
```

### 2. Preview (Dry Run)
```python
result = detect_mcp_servers_from_config(aim_client, agent_id, dry_run=True)
```

### 3. Custom Config Path
```python
result = detect_mcp_servers_from_config(
    aim_client,
    agent_id,
    config_path="/custom/path/config.json"
)
```

### 4. Map Only (No Registration)
```python
result = detect_mcp_servers_from_config(
    aim_client,
    agent_id,
    auto_register=False
)
```

## 📦 Response Format

```python
{
    "detected_servers": [
        {
            "name": "filesystem",
            "command": "npx",
            "args": ["-y", "@modelcontextprotocol/server-filesystem"],
            "confidence": 100.0,
            "source": "claude_desktop_config"
        }
    ],
    "registered_count": 3,      # Newly registered
    "mapped_count": 3,          # Mapped to agent
    "total_talks_to": 5,        # Total MCP servers
    "dry_run": False,
    "errors_encountered": []
}
```

## 🔍 Helper Functions

### Find Config Automatically
```python
from aim_sdk.integrations.mcp import find_claude_config

config_path = find_claude_config()
if config_path:
    print(f"Found: {config_path}")
```

### Get Default Paths
```python
from aim_sdk.integrations.mcp import get_default_config_paths

for path in get_default_config_paths():
    print(path)
```

## 📁 Default Config Locations

| OS | Primary Path | Fallback Path |
|----|--------------|---------------|
| **macOS** | `~/Library/Application Support/Claude/` | `~/.config/claude/` |
| **Windows** | `~/AppData/Roaming/Claude/` | `~/.config/claude/` |
| **Linux** | `~/.config/claude/` | - |

Filename: `claude_desktop_config.json`

## ⚠️ Error Handling

```python
try:
    result = detect_mcp_servers_from_config(aim_client, agent_id)

    if result['errors_encountered']:
        print(f"⚠️ {len(result['errors_encountered'])} errors")

except FileNotFoundError:
    print("❌ Claude config not found")

except PermissionError:
    print("❌ Authentication failed")

except ValueError as e:
    print(f"❌ Invalid config: {e}")
```

## 🎨 Best Practices

### ✅ DO
- Start with dry run to preview
- Handle partial failures gracefully
- Use helper functions to find config
- Check `errors_encountered` in response

### ❌ DON'T
- Hardcode config paths
- Ignore authentication errors
- Skip error checking
- Use without testing first

## 🔗 Backend Endpoint

**Endpoint**: `POST /api/v1/agents/{agent_id}/mcp-servers/detect`

**Request**:
```json
{
  "config_path": "/path/to/config.json",
  "auto_register": true,
  "dry_run": false
}
```

## 📚 Documentation

- **Full Guide**: `docs/MCP_AUTO_DETECTION.md`
- **Examples**: `examples/mcp_auto_detection_example.py`
- **Tests**: `test_mcp_auto_detection.py`

## 🧪 Testing

```bash
# Set environment
export AIM_URL="http://localhost:8080"
export AGENT_ID="your-agent-id"
export PUBLIC_KEY="your-public-key"
export PRIVATE_KEY="your-private-key"

# Run tests
python3 test_mcp_auto_detection.py
```

## 💡 Quick Tips

1. **Auto-detect config**: Use `find_claude_config()` instead of hardcoding paths
2. **Preview first**: Use `dry_run=True` before actual registration
3. **Check errors**: Always inspect `errors_encountered` in response
4. **Partial success**: Function continues even if some servers fail

## 🆘 Troubleshooting

| Problem | Solution |
|---------|----------|
| Config not found | Install Claude Desktop or specify custom path |
| Auth failed | Verify agent credentials |
| Malformed config | Validate JSON with `python3 -m json.tool config.json` |
| Some servers failed | Check `errors_encountered` list for details |

## ⏱️ Performance

- **Config parsing**: < 100ms
- **API call**: < 500ms
- **Total time**: < 1 second for typical configs

## 🔐 Security

- ✅ No credentials stored in config
- ✅ Uses AIM client authentication
- ✅ Validates all file paths
- ✅ Environment variables secured

---

**Last Updated**: October 19, 2025
**Version**: 1.0.0
**Status**: Production Ready ✅
