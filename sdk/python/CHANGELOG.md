# Changelog

All notable changes to the AIM Python SDK will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Planned
- JavaScript/TypeScript SDK
- GraphQL API support

## [1.21.0] - 2026-02-03

### Changed
- **CLI Authentication**: Replaced password prompt with OAuth 2.0 + PKCE browser flow
  - `aim-sdk login` now opens browser for secure authentication (Google, etc.)
  - Uses PKCE (Proof Key for Code Exchange) per RFC 8252 - same as AWS CLI
  - No more password prompts or browser permission dialogs
  - Browser redirects directly to localhost - seamless experience

### Security
- PKCE prevents authorization code interception attacks
- State parameter prevents CSRF attacks
- Authorization codes are one-time use with 5-minute TTL

## [1.8.0] - 2025-12-10

### Added
- **Smart MCP Attestation System**: Intelligent, automatic attestation that builds trust in MCP servers for supply chain security
  - Attestations triggered on: first use, new tool, stale cache (>24h), capability drift
  - Zero friction - attestations happen automatically without developer effort
  - Caching prevents redundant attestations while maintaining security
- **`AttestationCache` Class**: New persistent cache for tracking attestation state
  - `should_attest()` - Determines if attestation is needed based on triggers
  - `record_attestation()` - Records successful attestations
  - `record_tool_usage()` - Tracks tool usage for analytics
  - `get_supply_chain_report()` - Generates supply chain analytics report
- **Capability Drift Detection**: Automatically detects when MCP servers change their tools
  - Severity levels: low (tools added), medium (tools removed), high (>30% change)
  - Triggers re-attestation when drift is detected
- **Supply Chain Analytics**: Track MCP tool usage for security visibility
  - `get_mcp_supply_chain_report()` - Local analytics for this agent
  - `report_mcp_supply_chain()` - Sync analytics to backend for dashboard
  - Usage patterns visible in AIM dashboard
- **Enhanced `use_mcp_tool()`**: Now includes smart attestation
  - `auto_attest` parameter (default: True) - Enable smart attestation
  - `force_attest` parameter - Force attestation even if cached
  - Returns attestation info and tool usage stats
- **New AIMClient Methods**:
  - `use_mcp_tool()` - Convenience method with smart attestation
  - `get_attestation_cache()` - Access attestation cache
  - `get_mcp_supply_chain_report()` - Local supply chain report
  - `report_mcp_supply_chain()` - Sync to backend

### Changed
- `use_mcp_tool()` now automatically triggers attestations when appropriate
- Tool usage is tracked locally for supply chain analytics
- Documentation updated with smart attestation and supply chain features

### Supply Chain Security Value
This release positions AIM as an MCP supply chain security platform:
- **Trust Graphs**: Visualize which agents trust which MCP servers
- **Anomaly Detection**: Alert when MCP servers suddenly change capabilities
- **Compliance**: Full audit trail of tool usage across the organization
- **Risk Assessment**: Identify high-risk MCP servers (low attestations, frequent changes)

## [1.7.0] - 2025-12-10

### Added
- **Dynamic MCP Capability Discovery**: SDK now automatically queries MCP servers for their actual tools, resources, and prompts using the official MCP protocol (`tools/list`, `resources/list`, `prompts/list`)
  - No more hardcoded capability lists - capabilities are discovered at runtime
  - Works with any MCP server, not just known ones
  - `discover_mcp_capabilities()` function for programmatic discovery
  - `auto_detect_mcps(discover_tools=True)` for full detection with tools
- **Auto-Attestation on Registration**: When agents register MCP servers via `secure()` or `register_mcp()`, the SDK automatically:
  - Discovers actual capabilities from the MCP server
  - Creates a cryptographically signed attestation
  - Submits the attestation to build trust in the MCP server
- **New `attest_mcp()` method**: Manual attestation API on AIMClient for attesting to MCP server capabilities
- **MCP Connection Recording**: `use_mcp_tool()` function to record agent-MCP connections

### Changed
- Removed hardcoded `KNOWN_MCP_CAPABILITIES` lookup table - capabilities now always discovered dynamically
- `_detect_mcp_capabilities_from_config()` now uses dynamic discovery exclusively
- MCP server registration flow now includes automatic attestation with discovered capabilities

### Improved
- MCP integration documentation with new sections on dynamic discovery and auto-attestation
- README updated with MCP capability discovery examples

## [1.4.0] - 2025-12-05

### Added
- **Server-Side Enforcement Control**: Admins can now configure enforcement mode in the dashboard UI (Settings → Security → Policies)
  - **Monitoring Mode** (default): Actions are logged but allowed to proceed when verification fails
  - **Strict Mode**: Actions are blocked immediately when verification fails
- SDK now respects the organization's enforcement mode from the backend
- Environment variable `AIM_STRICT_MODE` can still override the backend setting for testing purposes

### Changed
- Verification response now includes `enforcementMode` field to inform SDK of the organization's setting
- `@aim_verify` decorator uses backend enforcement mode by default, with env var as optional override
- Dashboard Policies page now shows enforcement mode toggle with clear explanations of each mode

### Fixed
- SDK no longer requires `AIM_STRICT_MODE` environment variable - it now reads the setting from the backend

## [1.3.0] - 2025-12-05

### Added
- **Execution Status Reporting**: SDK now reports whether decorated functions actually executed back to the backend
  - New `report_execution_status()` method on `AIMClient`
  - Decorators automatically report execution status after function calls
  - Dashboard shows accurate status: "Executed", "Blocked", or "Executed despite denial"
- **Strict Mode Documentation**: Comprehensive documentation for `AIM_STRICT_MODE` environment variable
  - Explains difference between monitoring mode (default) and strict mode (production)
  - Code examples for production deployments

### Changed
- `@aim_verify` decorator now tracks and reports:
  - Whether function was executed
  - Whether strict mode was enabled
  - Any execution errors that occurred
- Alert detail panel now displays execution status with clear messaging

### Fixed
- Dashboard alert messaging now accurately reflects what actually happened (blocked vs allowed)

## [1.2.4] - 2025-12-04

### Fixed
- OAuth token refresh flow fixed (camelCase consistency)
- Improved error handling in credential storage

## [1.1.0] - 2025-12-03

### Added
- **MCP Server Registration**: `agent.register_mcp()` method for programmatic MCP server registration
- **Capability Requests**: `agent.request_capability()` method for requesting additional capabilities
- **JIT Access Demo**: Interactive demo showing just-in-time access request workflows
- **Consolidated Demo Agent**: Single `demo_agent.py` with comprehensive feature demonstrations
- **Standardized Capability Format**: All capabilities now use `namespace:action` format consistently

### Changed
- Demo agents consolidated into single interactive `demo_agent.py`
- Improved credential migration and stale credential handling
- Enhanced demo UX with better credential discovery and alerts

### Fixed
- Trust score nil pointer handling
- Duplicate plaintext file deletion in credential migration
- Stale encrypted credentials cleared on new SDK install

## [1.0.0] - 2025-11-06

### Added
- **New Decorators**:
  - `@agent.perform_action()` - Verify and track actions with optional JIT access for admin approval
  - `@agent.require_approval()` - Require admin approval before executing critical actions
- **Versioning**: SDK download filename now includes version (e.g., `aim-sdk-python-v1.0.0.zip`)
- **VERSION File**: Single source of truth for SDK version at `/sdk/python/VERSION`
- **Documentation**:
  - Comprehensive decorator documentation with examples
  - Versioning strategy guide at `docs/VERSIONING.md`
  - Updated README with new decorator usage patterns

### Fixed
- **Critical: Ed25519 Signature Verification**:
  - Fixed JSON formatting mismatch between Python SDK and Go backend
  - SDK now uses `json.dumps(sort_keys=True, separators=(', ', ': '))` for consistent message format
  - Backend uses custom `customJSONFormat()` function to match Python exactly
  - Resolves signature verification failures caused by:
    - `"resource": null` vs `"resource": ""` differences
    - Space placement inconsistencies in JSON serialization
- **Critical: Credential Encryption**:
  - Fixed encryption bug in `secure_storage.py` when storing agent credentials
  - Credentials now properly encrypted and saved to `~/.aim/credentials.json`
- **API Key Middleware**:
  - Fixed middleware blocking verification endpoints with 401 errors
  - Verification endpoints now correctly use Ed25519 signature auth instead of API keys
- **Public Key Handling**:
  - Backend now accepts and uses SDK-provided public keys during agent registration
  - Fixes public key mismatch errors where backend was generating its own keys
- **Decorator Response Parsing**:
  - Fixed `AttributeError: 'dict' object has no attribute 'approved'`
  - Decorators now use `dict.get("verified", False)` for response parsing

### Changed
- **SDK Download Format**: Filename changed from `aim-sdk-python.zip` to `aim-sdk-python-v{version}.zip`
- **Agent Registration**: Backend now supports optional `public_key` field in `CreateAgentRequest`
- **Verification Flow**: Improved decorator implementation with proper error handling

### Deprecated
- `@agent.track_action()` - Use `@agent.perform_action()` instead
  - Will be removed in v2.0.0
  - Deprecation warnings added in v1.6.0

## [0.9.0] - 2025-11-05 (Pre-release)

### Added
- Initial SDK implementation
- `secure()` function for zero-config agent registration
- Ed25519 cryptographic signing
- Automatic capability detection
- MCP server detection from Claude Desktop config
- OAuth token management
- Basic decorator support with `@agent.perform_action()`

### Security
- Ed25519 cryptographic signatures for all agent communications
- Secure credential storage using OS keyrings
- Encrypted private key storage
- SHA-256 API key hashing

---

## Version Support

| Version | Status | Support Level | End of Support |
|---------|--------|---------------|----------------|
| 1.0.x   | ✅ Current | Full support | N/A |
| 0.9.x   | ⚠️ Pre-release | No support | Immediately |

## Migration Guides

### Upgrading from 0.9.x to 1.0.0

**Breaking Changes**: None

**New Features**:
- `@agent.perform_action()` decorator for verification and tracking
- `@agent.require_approval()` decorator for critical actions

**Recommended Usage**:

```python
# Standard usage - all actions verified and tracked
@agent.perform_action(risk_level="low", resource="database:users")
def query_users():
    return db.query("SELECT * FROM users")

# Critical actions with JIT access - requires admin approval
@agent.perform_action(risk_level="critical", jit_access=True, resource="database:users")
def delete_all_users():
    return db.execute("DELETE FROM users")  # ⏸️ Pauses until admin approves

# Alternative: use @require_approval for critical actions
@agent.require_approval(risk_level="critical", resource="database:users")
def purge_data():
    return db.execute("TRUNCATE TABLE users")
```

**Action Required**:
- ✅ No immediate action required - 0.9.x code continues to work
- ⚠️ Update decorators to new style before v2.0.0 (recommended)

---

## Reporting Issues

Found a bug? Please report it:
- **GitHub Issues**: https://github.com/opena2a-org/agent-identity-management/issues
- **Email**: info@opena2a.org
- **Discord**: https://discord.gg/uRZa3KXgEn

---

**Last Updated**: 2025-12-10
