# AIM SDK Versioning Strategy

## Overview

The AIM Python SDK uses semantic versioning (SemVer 2.0.0) to ensure predictable and transparent version management. This document outlines our versioning strategy, release process, and compatibility guarantees.

## Semantic Versioning

### Version Format: `MAJOR.MINOR.PATCH`

```
1.0.0
│ │ │
│ │ └─── PATCH: Backward-compatible bug fixes
│ └───── MINOR: Backward-compatible new features
└─────── MAJOR: Breaking changes
```

### Examples

- `1.0.0` → Initial stable release
- `1.0.1` → Bug fix (e.g., fixed encryption issue)
- `1.1.0` → New feature (e.g., added `@require_approval()` decorator)
- `2.0.0` → Breaking change (e.g., changed API signature)

## VERSION File

The SDK version is stored in a single source of truth: `/sdk/python/VERSION`

**Contents**:
```
1.0.0
```

**Why a VERSION file?**
- Single source of truth for version number
- Easy to read and update
- Works with automated release processes
- No need to parse Python code to extract version

## Download Naming Convention

When you download the SDK from the AIM dashboard, the filename includes the version:

```
aim-sdk-python-v1.0.0.zip
```

**Format**: `aim-sdk-{language}-v{version}.zip`

**Why include version in filename?**
- Allows multiple SDK versions to coexist on the same system
- Clear audit trail of which version was downloaded when
- Easier troubleshooting (users can tell us their version immediately)
- Prevents accidental overwrites

## Version Compatibility

### Backend API Compatibility

The SDK and backend API maintain backward compatibility within major versions:

| SDK Version | Compatible Backend API Versions |
|-------------|-------------------------------|
| 1.x.x       | 1.x.x (any 1.x backend)       |
| 2.x.x       | 2.x.x (any 2.x backend)       |

**Example**:
- SDK `1.0.0` works with backend `1.0.5` ✅
- SDK `1.2.3` works with backend `1.0.0` ✅ (SDK has newer features but degrades gracefully)
- SDK `1.x.x` does NOT work with backend `2.x.x` ❌ (major version mismatch)

### Python Version Requirements

```python
Python >= 3.8  # Minimum supported version
Python <= 3.12 # Maximum tested version
```

**Tested on**:
- Python 3.8 (minimum)
- Python 3.9
- Python 3.10
- Python 3.11
- Python 3.12 (recommended)

## Release Process

### 1. Version Bump

Update the VERSION file:
```bash
# Bug fix: 1.0.0 → 1.0.1
echo "1.0.1" > sdk/python/VERSION

# New feature: 1.0.1 → 1.1.0
echo "1.1.0" > sdk/python/VERSION

# Breaking change: 1.1.0 → 2.0.0
echo "2.0.0" > sdk/python/VERSION
```

### 2. Update Changelog

Document changes in `CHANGELOG.md`:

```markdown
## [1.1.0] - 2025-11-06

### Added
- `@agent.require_approval()` decorator for critical actions
- `@agent.track_action()` decorator for monitoring

### Fixed
- Ed25519 signature verification JSON formatting issue
- Credential encryption bug in secure_storage.py

### Changed
- SDK download filename now includes version (aim-sdk-python-v1.1.0.zip)
```

### 3. Test

Run full test suite:
```bash
cd sdk/python
pytest tests/ -v --cov=aim_sdk --cov-report=html
```

**Required**:
- ✅ 100% of unit tests pass
- ✅ Integration tests with backend pass
- ✅ Decorator tests pass
- ✅ Cryptographic signing tests pass

### 4. Build

Build the SDK package:
```bash
cd apps/backend
docker compose build backend
docker compose up -d backend
```

### 5. Tag Release

Create a Git tag:
```bash
git tag -a v1.1.0 -m "Release v1.1.0: Added approval decorators and fixed signature verification"
git push origin v1.1.0
```

## Breaking Changes Policy

### What Constitutes a Breaking Change?

**Breaking changes** (require major version bump):
- Removing or renaming public functions/methods
- Changing function signatures (adding required parameters)
- Removing decorator functionality
- Changing default behavior that users depend on
- Removing or renaming environment variables
- Changing API endpoint paths or request/response formats

**NOT breaking changes** (minor or patch version):
- Adding new optional parameters (with defaults)
- Adding new decorators or functions
- Fixing bugs that restore intended behavior
- Internal refactoring that doesn't affect public API
- Performance improvements
- Documentation updates

### Deprecation Timeline

Before introducing breaking changes:

1. **Version N**: Add deprecation warning
   ```python
   import warnings
   warnings.warn("perform_action() is deprecated, use track_action() instead", DeprecationWarning)
   ```

2. **Version N+1**: Continue supporting deprecated feature with warnings

3. **Version N+2 (Major)**: Remove deprecated feature

**Example**:
- v1.0.0: `@agent.perform_action()` works
- v1.1.0: Add `@agent.track_action()`, deprecate `perform_action()`
- v1.2.0: Both work, deprecation warning shown
- v2.0.0: Remove `@agent.perform_action()` entirely

## Version Checking

### In Code

```python
import aim_sdk

# Get SDK version
print(aim_sdk.__version__)  # "1.0.0"

# Check minimum version
from packaging import version
if version.parse(aim_sdk.__version__) < version.parse("1.1.0"):
    raise RuntimeError("This script requires AIM SDK >= 1.1.0")
```

### Via CLI

```bash
# Check SDK version
python -c "import aim_sdk; print(aim_sdk.__version__)"
```

## Upgrade Guide

### From 1.0.x to 1.1.x (Minor Upgrade)

**Changes**:
- New decorators available: `@track_action()` and `@require_approval()`
- `@perform_action()` still works but deprecated

**Action Required**:
- ✅ No breaking changes
- ⚠️ Update code to use new decorators (recommended but optional)

**Migration**:
```python
# OLD (still works)
@agent.perform_action("read_database", resource="users")
def query_users():
    return db.query("SELECT * FROM users")

# NEW (recommended)
@agent.track_action(risk_level="low", resource="database:users")
def query_users():
    return db.query("SELECT * FROM users")
```

### From 1.x to 2.x (Major Upgrade)

**When released, will include**:
- Breaking API changes
- Migration guide with detailed examples
- Automated migration tools where possible

## Security Patches

**Critical security fixes** are backported to all supported major versions:

- **Current major version**: All patches applied
- **Previous major version**: Security patches only (for 6 months)
- **Older versions**: No longer supported

**Example**:
- Current: v2.5.0
- Supported: v2.x.x (all patches), v1.x.x (security only)
- Unsupported: v0.x.x

## Support Policy

| Version | Support Level | Maintenance Period |
|---------|--------------|-------------------|
| Latest  | Full support | Indefinite |
| N-1     | Security patches only | 6 months after N+1 release |
| N-2+    | No support   | End of life |

**Example**:
- v2.0.0 released on 2026-01-01
- v1.x.x receives security patches until 2026-07-01
- v0.x.x is end-of-life immediately

## Changelog Format

We follow [Keep a Changelog](https://keepachangelog.com/en/1.0.0/) format:

```markdown
# Changelog

All notable changes to the AIM Python SDK will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- New features in development

## [1.1.0] - 2025-11-06

### Added
- `@agent.require_approval()` decorator for critical actions requiring admin approval
- `@agent.track_action()` decorator for monitoring and audit logging

### Fixed
- Ed25519 signature verification JSON formatting mismatch between Python and Go
- Credential encryption bug in secure_storage.py

### Changed
- SDK download filename now includes version (aim-sdk-python-v1.1.0.zip)
- Updated documentation with new decorator examples

## [1.0.0] - 2025-11-05

### Added
- Initial stable release
- `secure()` function for zero-config agent registration
- Ed25519 cryptographic signing
- Automatic capability detection
- MCP server detection
- OAuth token management
```

## Best Practices

### For SDK Developers

1. **Always update VERSION file** when releasing
2. **Document all changes** in CHANGELOG.md
3. **Tag releases** with `git tag -a vX.Y.Z`
4. **Run full test suite** before releasing
5. **Communicate breaking changes** clearly and early

### For SDK Users

1. **Pin SDK versions** in your code:
   ```python
   # requirements.txt
   aim-sdk==1.1.0  # Exact version pinning
   ```

2. **Test before upgrading** major versions
3. **Read CHANGELOG** before upgrading
4. **Use version checking** for critical features:
   ```python
   if version.parse(aim_sdk.__version__) >= version.parse("1.1.0"):
       @agent.require_approval(risk_level="critical")
       def critical_operation():
           pass
   ```

## Questions?

- **How do I check my SDK version?** Run `python -c "import aim_sdk; print(aim_sdk.__version__)"`
- **Can I use SDK 1.x with backend 2.x?** No, major versions must match
- **When should I upgrade?** Minor/patch versions: anytime. Major versions: plan and test first.
- **How long is each version supported?** Latest version: indefinite. N-1: 6 months security patches.

## Version History

| Version | Release Date | Major Changes |
|---------|-------------|---------------|
| 1.1.0   | 2025-11-06  | Added `@track_action()` and `@require_approval()` decorators |
| 1.0.0   | 2025-11-05  | Initial stable release |

---

**Last Updated**: 2025-11-06
