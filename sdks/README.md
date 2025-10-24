# AIM SDK - Agent Identity Management

**Version**: 1.0.0
**Status**: Python ✅ Production Ready
**Public Release**: Python SDK Only

---

## 📦 Python SDK (✅ Production Ready)

**Status**: 100% Complete - Reference Implementation
**Location**: `sdks/python/`

The AIM Python SDK is the official, production-ready SDK for Agent Identity Management. It provides comprehensive features for agent registration, MCP server management, and intelligent capability detection.

### ✨ Features

- ✅ **Ed25519 Cryptographic Signing** - Enterprise-grade security
- ✅ **OAuth/OIDC Integration** - Google, Microsoft, Okta support
- ✅ **Automatic MCP Detection** - AI-powered capability detection
- ✅ **Secure Credential Storage** - OS keyring integration
- ✅ **Agent Registration** - Complete workflow automation
- ✅ **MCP Detection Reporting** - Real-time capability reporting
- ✅ **SDK Token Management** - Secure token handling
- ✅ **Action Verification** - Backend verification support
- ✅ **Message Signing** - Cryptographic message integrity

### 📦 Installation

```bash
pip install aim-sdk
```

### 🚀 Quick Start

```python
from aim_sdk import AIMClient

# Register new agent
client = AIMClient(api_url="http://localhost:8080")
result = client.register_agent_with_oauth(
    provider="google",
    agent_name="my-ai-agent"
)

# Auto-detect and report MCPs
client.auto_detect_mcps()

# Register specific MCP server
client.register_mcp(
    mcp_server_id="filesystem-mcp-server",
    detection_method="manual",
    confidence=100.0
)

# Report SDK integration
client.report_sdk_integration(
    sdk_version="aim-sdk-python@1.0.0",
    platform="python",
    capabilities=["auto_detect_mcps", "capability_detection"]
)
```

---

## 📚 SDK Methods

### Core Methods

#### `register_agent_with_oauth(provider, agent_name)`
Register a new agent using OAuth authentication.

**Parameters**:
- `provider` (str): OAuth provider ("google", "microsoft", "okta")
- `agent_name` (str): Name for the agent

**Returns**: Registration result with agent ID and credentials

**Example**:
```python
result = client.register_agent_with_oauth(
    provider="google",
    agent_name="my-ai-agent"
)
print(f"Agent ID: {result['agent_id']}")
```

---

#### `auto_detect_mcps()`
Automatically detect and report MCP servers.

**Returns**: Detection results with found MCP servers

**Example**:
```python
results = client.auto_detect_mcps()
print(f"Found {len(results['mcps'])} MCP servers")
```

---

#### `register_mcp(mcp_server_id, detection_method, confidence, metadata=None)`
Register an MCP server to the agent's "talks_to" list.

**Parameters**:
- `mcp_server_id` (str): MCP server identifier
- `detection_method` (str): Detection method ("auto", "manual", "config")
- `confidence` (float): Confidence score (0.0-100.0)
- `metadata` (dict, optional): Additional metadata

**Returns**: Registration result

**Example**:
```python
result = client.register_mcp(
    mcp_server_id="filesystem-mcp-server",
    detection_method="manual",
    confidence=100.0,
    metadata={"source": "config"}
)
print(f"Registered {result['added']} MCP server(s)")
```

---

#### `report_sdk_integration(sdk_version, platform, capabilities)`
Report SDK installation status to AIM dashboard.

**Parameters**:
- `sdk_version` (str): SDK version identifier
- `platform` (str): Platform name ("python", "node", etc.)
- `capabilities` (list): List of SDK capabilities

**Returns**: Integration report confirmation

**Example**:
```python
result = client.report_sdk_integration(
    sdk_version="aim-sdk-python@1.0.0",
    platform="python",
    capabilities=["auto_detect_mcps", "capability_detection"]
)
print(f"SDK integration reported: {result['message']}")
```

**What This Does**:
- Updates the **Detection tab** in AIM dashboard
- Shows SDK installation status: ✅ "Installed"
- Displays SDK version and platform
- Enables auto-detection features
- Tracks SDK capabilities

---

#### `verify_action(action_type, resource_type, parameters)`
Verify an action with the backend before execution.

**Parameters**:
- `action_type` (str): Type of action ("execute", "read", "write")
- `resource_type` (str): Type of resource ("database", "file", "api")
- `parameters` (dict): Action parameters

**Returns**: Verification result with approval status

**Example**:
```python
result = client.verify_action(
    action_type="execute",
    resource_type="database",
    parameters={"query": "SELECT * FROM users"}
)
if result['approved']:
    # Proceed with action
    execute_query(result['parameters']['query'])
```

---

#### `sign_message(message)`
Sign a message using Ed25519 cryptography.

**Parameters**:
- `message` (str): Message to sign

**Returns**: Cryptographic signature

**Example**:
```python
signature = client.sign_message("important message")
print(f"Signature: {signature}")
```

---

## 🧪 Testing

### Unit Tests
```bash
cd sdks/python
pytest
```

### Integration Tests
```bash
# Requires backend running on localhost:8080
cd sdks/python
pytest tests/test_e2e.py
```

### Coverage
```bash
cd sdks/python
pytest --cov=aim_sdk --cov-report=html
```

---

## 📖 Documentation

- **Full API Reference**: [sdks/python/README.md](./python/README.md)
- **Intelligent Detection**: [INTELLIGENT_AGENT_CAPABILITY_DETECTION.md](./INTELLIGENT_AGENT_CAPABILITY_DETECTION.md)
- **Implementation Guide**: [SDK_FEATURE_PARITY_IMPLEMENTATION_GUIDE.md](./SDK_FEATURE_PARITY_IMPLEMENTATION_GUIDE.md)
- **Examples**: [sdks/python/examples/](./python/examples/)

---

## 🔮 Future SDK Releases

### Go SDK (Planned Q1 2026)
Enterprise-ready Go SDK with Ed25519 signing, keyring storage, and agent registration.

### JavaScript SDK (Planned Q2 2026)
Full TypeScript support with browser and Node.js compatibility.

**To restore archived SDKs**:
```bash
# Restore Go SDK
git checkout archive/go-javascript-sdks -- sdks/go

# Restore JavaScript SDK
git checkout archive/go-javascript-sdks -- sdks/javascript
```

**Archived SDKs Status**: Both Go and JavaScript SDKs are production-ready (75% feature parity) and safely archived for future release.

---

## 🐛 Known Issues

### Python SDK
- ✅ No known issues
- 100% feature complete
- All tests passing

---

## 🤝 Contributing

To contribute to the Python SDK:

1. Fork the repository
2. Create a feature branch
3. Write tests for your changes
4. Ensure all tests pass
5. Submit a pull request

See [CONTRIBUTING.md](../CONTRIBUTING.md) for detailed guidelines.

---

## 📄 License

GNU Affero General Public License v3.0 (AGPL-3.0) - See [LICENSE](../LICENSE) for details

---

## 🔗 Related Links

- **Main Repository**: https://github.com/opena2a-org/agent-identity-management
- **Documentation**: https://docs.opena2a.org
- **Issue Tracker**: https://github.com/opena2a-org/agent-identity-management/issues
- **Discussions**: https://github.com/opena2a-org/agent-identity-management/discussions

---

**Last Updated**: October 19, 2025
**Maintainer**: OpenA2A Team
**Public Release**: Python SDK Only

## 📋 Recent Changes

### October 19, 2025 - Python-Only Public Release
- ✅ **Go SDK Archived** - Safely stored for Q1 2026 release
- ✅ **JavaScript SDK Archived** - Safely stored for Q2 2026 release
- ✅ **Python SDK** - 100% feature complete and production-ready
- ✅ **Comprehensive Testing** - All Python SDK endpoints verified
- ✅ **Documentation Updated** - Reflects Python-only approach

### October 10, 2025 - SDK Feature Additions
- ✅ Added `register_mcp()` method for MCP registration
- ✅ Added `report_sdk_integration()` method for SDK detection
- ✅ Intelligent MCP detection implemented
- ✅ Auto-detection capabilities enhanced
