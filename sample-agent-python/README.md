# AIM Python Demo Agent

Production-ready Python demonstration agent showcasing the [AIM (Agent Identity Management)](https://github.com/opena2a/identity) SDK with enterprise security features.

## Overview

This demo demonstrates:
- **One-line agent registration** with automatic security
- **MCP (Model Context Protocol)** usage and detection
- **Capability management** and real-time trust scoring
- **Safe vs dangerous operation** handling
- **Cryptographic verification** with Ed25519
- **Local MCP server** for testing verification

## Quick Start

### Prerequisites

- Python 3.8 or higher
- AIM backend running (see main repository)
- API key from AIM dashboard

### Installation

```bash
# Install SDK and dependencies
./setup.sh

# Or manually:
cd aim-sdk-python
pip install -e .
cd ..
pip install -r requirements.txt
```

### Configuration

1. Get your API key from the AIM dashboard at `http://localhost:3000/dashboard/developers`
2. Update the API key in `demo.py`:

```python
API_KEY = 'your-api-key-here'
```

### Running the Demo

```bash
# Run with clean credentials
./run-demo.sh

# Or directly
python3 demo.py
```

## What the Demo Does

### 1. Agent Registration (`demo.py`)

Demonstrates the **one-line** `secure()` function:

```python
from aim_sdk import secure

# ONE LINE - Enterprise security enabled!
agent = secure(name="python-demo-agent", api_key=API_KEY, aim_url=AIM_URL)
```

This automatically:
- ✅ Generates Ed25519 cryptographic keys
- ✅ Registers agent with AIM platform
- ✅ Detects MCP servers from Claude Desktop config
- ✅ Reports capabilities to AIM
- ✅ Stores credentials securely in system keyring

### 2. Safe Operations

Shows approved actions that execute successfully:
- File reading operations
- Data processing
- API calls

### 3. Dangerous Operations

Demonstrates blocked actions with real-time alerts:
- System commands
- File deletion attempts
- Database modifications

### 4. MCP Usage

Simulates using an MCP server as a tool (reading a file via MCP).

**Note**: Agents **use** MCP servers as tools; they don't run their own MCP servers. AIM tracks this usage for governance.

## Testing MCP Cryptographic Verification

The `test-mcp-server.py` provides a local MCP server with Ed25519 cryptographic verification:

### Start the MCP Server

```bash
python3 test-mcp-server.py
```

The server will display its public key. Copy it for registration.

### Register in AIM Dashboard

1. Go to `http://localhost:3000/dashboard/mcp`
2. Click "Register MCP Server"
3. Fill in:
   - **Name**: `test-mcp-local`
   - **URL**: `http://localhost:5555`
   - **Public Key**: (from server output)
   - **Description**: `Local test MCP server`
4. Click "Save"
5. Click "Verify" to trigger cryptographic verification
6. Capabilities will be auto-detected (3 tools, 2 resources, 1 prompt)

### MCP Server Capabilities

The test server provides:

**Tools:**
- `echo` - Echo back text
- `calculate` - Perform math calculations
- `timestamp` - Get current UTC timestamp

**Resources:**
- `server://status` - Server health and statistics
- `server://config` - Server configuration

**Prompts:**
- `greeting` - Generate friendly greetings

## Files

- **`demo.py`** - Main demonstration script
- **`test-mcp-server.py`** - Local MCP server for testing
- **`setup.sh`** - Installation script
- **`run-demo.sh`** - Clean run script
- **`requirements.txt`** - Python dependencies
- **`aim-sdk-python/`** - AIM Python SDK

## Features Demonstrated

### Security
- ✅ Ed25519 cryptographic signing on every API request
- ✅ Secure credential storage (system keyring)
- ✅ Automatic duplicate prevention
- ✅ Real-time trust scoring (8-factor ML algorithm)

### Governance
- ✅ MCP usage detection and reporting
- ✅ Capability request and management
- ✅ Action verification (safe vs dangerous)
- ✅ Audit logging

### Automation
- ✅ One-line registration
- ✅ Auto MCP detection
- ✅ Auto capability reporting
- ✅ Credential reuse (no duplicates)

## Architecture

```
┌─────────────────┐
│  Python Agent   │  Uses MCP servers as tools
│   (demo.py)     │  Registers with AIM
└────────┬────────┘
         │
         ├─ Cryptographic Identity (Ed25519)
         ├─ MCP Usage Reporting
         ├─ Capability Management
         └─ Action Verification
         │
    ┌────▼─────┐
    │   AIM    │  Governance & Monitoring
    │ Platform │  Real-time Trust Scoring
    └──────────┘
```

## Dashboard Monitoring

All agent activity is visible in real-time:
- **Agent Status**: `http://localhost:3000/dashboard/agents`
- **MCP Servers**: `http://localhost:3000/dashboard/mcp`
- **Audit Logs**: `http://localhost:3000/dashboard/audit`
- **Trust Score**: Live updates based on behavior

## Troubleshooting

### "Registration failed: invalid API key"
- Generate a new API key from the dashboard
- Update `API_KEY` in `demo.py`

### "Agent already exists"
- The SDK automatically reuses existing agents
- To force new registration: `rm -rf ~/.aim`

### "MCP verification failed"
- Ensure the MCP server is running
- Check the public key matches
- Verify the URL is correct

## Production Usage

For production agents:

1. **Secure API Key Storage**: Use environment variables
   ```python
   import os
   API_KEY = os.getenv('AIM_API_KEY')
   ```

2. **Error Handling**: Wrap operations in try-except
3. **Logging**: Use proper logging instead of print statements
4. **MCP Detection**: Let SDK auto-detect from Claude Desktop
5. **Capability Requests**: Request capabilities before first use

## Learn More

- **Main Repository**: [AIM GitHub](https://github.com/opena2a/identity)
- **Python SDK Docs**: `aim-sdk-python/README.md`
- **API Reference**: See main repository
- **MCP Protocol**: [Model Context Protocol](https://modelcontextprotocol.io)

## License

See main repository for license information.

## Support

For issues or questions:
- Open an issue in the main repository
- Check the documentation
- Review audit logs in the dashboard

---

**Built with ❤️ for secure AI agent governance**
