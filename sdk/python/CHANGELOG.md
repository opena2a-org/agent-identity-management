# Changelog

All notable changes to the AIM Python SDK will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Planned
- JavaScript/TypeScript SDK
- GraphQL API support
- CLI tool for automation

## [1.0.0] - 2025-11-06

### Added
- **New Decorators**:
  - `@agent.track_action()` - Track and log actions without requiring approval (for monitoring and audit)
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
- `@agent.perform_action()` - Use `@agent.track_action()` or `@agent.require_approval()` instead
  - Will be removed in v2.0.0
  - Deprecation warnings will be added in v1.1.0

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
- Two new decorators available: `@track_action()` and `@require_approval()`

**Recommended Migration**:

```python
# OLD (0.9.x) - Still works but deprecated
@agent.perform_action("read_database", resource="users")
def query_users():
    return db.query("SELECT * FROM users")

# NEW (1.0.0) - Recommended for monitoring
@agent.track_action(risk_level="low", resource="database:users")
def query_users():
    return db.query("SELECT * FROM users")

# NEW (1.0.0) - For critical actions requiring approval
@agent.require_approval(risk_level="critical", resource="database:users")
def delete_all_users():
    return db.execute("DELETE FROM users")  # ⏸️ Pauses until admin approves
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

**Last Updated**: 2025-11-06
