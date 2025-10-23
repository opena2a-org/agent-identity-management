# 🐛 CRITICAL BUG: SDK OAuth Token Failure Breaks "One Line" Promise

**Severity:** 🔴 CRITICAL - Breaks core product promise
**Impact:** Users cannot use `secure()` function out-of-the-box
**Status:** 🔍 IDENTIFIED - Needs immediate fix
**Date Reported:** October 22, 2025
**Reported By:** SDK Testing Suite

## 📋 Executive Summary

The AIM Python SDK **fails to work out-of-the-box** after download, breaking our core "Stripe moment" promise:

```python
# THIS SHOULD WORK (but doesn't):
from aim_sdk import secure
agent = secure("my-agent")  # ❌ FAILS: "Failed to obtain OAuth access token"
```

**Root Cause:** OAuth token manager fails to load SDK credentials from current directory

**Business Impact:**
- ❌ Breaks "ONE LINE" marketing promise
- ❌ Users must manually configure credentials
- ❌ SDK is NOT "zero configuration"
- ❌ Major barrier to adoption

## 🔍 Technical Analysis

### Error Message
```
ConfigurationError: Registration failed: Failed to obtain OAuth access token
```

### Call Stack
```
File "aim_sdk/client.py", line 1027, in register_agent
    return _register_via_oauth(...)
File "aim_sdk/client.py", line 1103, in _register_via_oauth
    raise ConfigurationError("Failed to obtain OAuth access token")
```

### Root Cause

**Location:** `aim_sdk/oauth.py`, line 132-140

```python
def get_access_token(self) -> Optional[str]:
    """Get a valid access token, refreshing if necessary."""
    if not self.credentials:  # ❌ THIS CHECK FAILS
        return None  # ❌ RETURNS NONE

    # Check if current token is still valid
    if self.access_token and self.access_token_expiry:
        if time.time() < (self.access_token_expiry - 60):
            return self.access_token

    # Need to refresh token
    return self._refresh_token()
```

**Problem:** `self.credentials` is `None` because:
1. SDK looks in current working directory: `./.aim/credentials.json`
2. If not found, falls back to: `~/.aim/credentials.json`
3. BUT the actual SDK credentials are in: `/workspace/aim-sdk-python/.aim/credentials.json`
4. Result: Credentials never loaded, `get_access_token()` returns `None`

### Code Flow

```
User downloads SDK
└── SDK package includes: /aim-sdk-python/.aim/credentials.json

User creates project in different directory
└── User project: /my-project/weather_agent.py

User runs:
    from aim_sdk import secure
    agent = secure("my-agent")

SDK checks:
    1. ❌ Current dir: /my-project/.aim/credentials.json (not found)
    2. ❌ Home dir: ~/.aim/credentials.json (not found)
    3. ✅ Actual location: /aim-sdk-python/.aim/credentials.json (NOT CHECKED!)

Result: ❌ OAuth fails, registration fails
```

## 💥 Why This is CRITICAL

### Breaks Core Value Proposition

Our README.md promises:
> "**Zero Configuration** 🚀"
> "Download SDK from Dashboard (Recommended)"
> "- Visit your AIM dashboard"
> "- Click 'Download SDK' → Includes embedded authentication tokens"
> "- Extract and you're ready to go!"

**Reality:** ❌ NOT ready to go - OAuth fails immediately

### User Experience Impact

**Expected:** (What we promise)
```python
# 1. Download SDK from dashboard
# 2. Extract ZIP
# 3. One line of code:
from aim_sdk import secure
agent = secure("my-agent")  # ✅ WORKS IMMEDIATELY
```

**Actual:** (What happens)
```python
# 1. Download SDK from dashboard
# 2. Extract ZIP
# 3. One line of code:
from aim_sdk import secure
agent = secure("my-agent")  # ❌ ConfigurationError

# 4. User is confused
# 5. User reads error message (unclear)
# 6. User gives up or opens support ticket
```

## 🔧 Required Fix

### Option 1: Auto-Detect SDK Package Location (RECOMMENDED)

**Approach:** SDK should check its own installation directory for credentials

```python
# In oauth.py __init__():
def __init__(self, credentials_path: Optional[str] = None, use_secure_storage: bool = True):
    if credentials_path:
        self.credentials_path = Path(credentials_path)
    else:
        # Priority order:
        # 1. SDK package directory (where we're installed)
        # 2. Current working directory
        # 3. Home directory

        import aim_sdk
        sdk_package_dir = Path(aim_sdk.__file__).parent / ".aim"

        if (sdk_package_dir / "credentials.json").exists():
            self.credentials_path = sdk_package_dir / "credentials.json"  # ✅ FOUND!
        elif (Path.cwd() / ".aim" / "credentials.json").exists():
            self.credentials_path = Path.cwd() / ".aim" / "credentials.json"
        else:
            self.credentials_path = Path.home() / ".aim" / "credentials.json"
```

**Pros:**
- ✅ Works out-of-the-box
- ✅ No user configuration needed
- ✅ Maintains "Stripe moment" promise
- ✅ Backwards compatible

**Cons:**
- Needs to import `aim_sdk` module
- Slightly more complex logic

### Option 2: Copy Credentials on First Import

**Approach:** SDK copies credentials from package to home directory on first import

```python
# In __init__.py:
import shutil
from pathlib import Path

def _ensure_credentials():
    """Copy SDK credentials to home directory if they don't exist."""
    home_creds = Path.home() / ".aim" / "credentials.json"

    if not home_creds.exists():
        # Find SDK package credentials
        sdk_creds = Path(__file__).parent / ".aim" / "credentials.json"

        if sdk_creds.exists():
            home_creds.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy(sdk_creds, home_creds)
            print(f"✅ SDK credentials installed to {home_creds}")

# Call on import
_ensure_credentials()
```

**Pros:**
- ✅ Simple implementation
- ✅ Credentials in standard location
- ✅ Works across all user projects

**Cons:**
- Copies credentials on every import (unless we add check)
- User might accidentally delete from home directory

### Option 3: Environment Variable

**Approach:** Set `AIM_SDK_PATH` environment variable

```python
# Users would need to:
export AIM_SDK_PATH="/path/to/aim-sdk-python/.aim/credentials.json"
```

**Pros:**
- Flexible

**Cons:**
- ❌ Requires user configuration
- ❌ Breaks "zero config" promise
- ❌ NOT ACCEPTABLE

## ✅ Recommended Solution

**Implement Option 1 (Auto-Detect) + Option 2 (Auto-Copy) Hybrid:**

```python
def __init__(self, credentials_path: Optional[str] = None, use_secure_storage: bool = True):
    """
    Initialize OAuth token manager with intelligent credential discovery.

    Search priority:
    1. Explicit path (if provided)
    2. Home directory (~/.aim/credentials.json) - standard location
    3. SDK package directory - for downloaded SDKs
    4. Current working directory - for local projects

    If found in SDK package, auto-copy to home directory for persistence.
    """
    if credentials_path:
        self.credentials_path = Path(credentials_path)
    else:
        home_creds = Path.home() / ".aim" / "credentials.json"

        # Check home directory first (standard location)
        if home_creds.exists():
            self.credentials_path = home_creds
        else:
            # Look for SDK package credentials
            import aim_sdk
            sdk_creds = Path(aim_sdk.__file__).parent / ".aim" / "credentials.json"

            if sdk_creds.exists():
                # Auto-copy to home directory (one-time setup)
                home_creds.parent.mkdir(parents=True, exist_ok=True)
                import shutil
                shutil.copy(sdk_creds, home_creds)
                print(f"✅ SDK credentials installed to {home_creds}")
                self.credentials_path = home_creds
            else:
                # Fall back to current directory
                self.credentials_path = Path.cwd() / ".aim" / "credentials.json"
```

**This solution:**
- ✅ Works out-of-the-box (auto-finds SDK credentials)
- ✅ Auto-copies to standard location (one-time)
- ✅ Zero configuration required
- ✅ Maintains "Stripe moment" promise
- ✅ User-friendly (credentials in predictable location)
- ✅ Backwards compatible

## 📊 Testing Requirements

After fix is implemented, verify:

1. **Zero-Config Mode Works**
   ```python
   from aim_sdk import secure
   agent = secure("my-agent")  # ✅ MUST WORK
   ```

2. **Credentials Auto-Copied**
   ```bash
   ls ~/.aim/credentials.json  # ✅ MUST EXIST
   ```

3. **Works in Different Directories**
   ```bash
   cd /tmp
   python3 -c "from aim_sdk import secure; secure('test')"  # ✅ MUST WORK
   ```

4. **No Manual Configuration Required**
   - ✅ No environment variables needed
   - ✅ No config files to edit
   - ✅ No credential copying needed

## 🎯 Acceptance Criteria

- [ ] `secure("my-agent")` works immediately after SDK download
- [ ] No configuration or setup steps required
- [ ] Credentials auto-discovered from SDK package
- [ ] Credentials auto-copied to home directory
- [ ] Works in any directory
- [ ] Clear success message printed on first use
- [ ] All existing tests pass
- [ ] New test added for zero-config scenario

## 💼 Business Priority

**CRITICAL** - This must be fixed before:
- ❌ Any demo to investors
- ❌ Any marketing materials published
- ❌ Any public release
- ❌ Claiming "zero configuration"

**Timeline:** ASAP (within 24 hours)

**Owner:** SDK Team Lead

## 📝 Related Files

Files that need changes:
1. `aim_sdk/oauth.py` - Fix OAuthTokenManager.__init__()
2. `aim_sdk/client.py` - Verify it works with fix
3. `tests/test_zero_config.py` - Add comprehensive test
4. `README.md` - Update if any clarification needed

## 🚀 Success Metrics

After fix:
- ✅ 100% of users can use `secure()` without configuration
- ✅ Zero support tickets about "OAuth token failure"
- ✅ Demo works perfectly every time
- ✅ "Stripe moment" promise is REAL, not marketing

---

**Status:** 🔴 CRITICAL - NEEDS IMMEDIATE FIX
**Impact:** Breaks core product value
**Priority:** P0 (Highest)
**Assigned:** SDK Engineering Team
