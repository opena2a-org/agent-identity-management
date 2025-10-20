# Quick Fix Guide - CrewAI Integration

**Problem**: All documentation examples fail
**Root Cause**: 2 missing methods in `AIMClient`
**Fix Time**: 4-6 hours
**Fix Complexity**: Medium

---

## TL;DR - What's Broken?

```python
# ❌ THIS FAILS (documented but doesn't exist):
aim_client = AIMClient.auto_register_or_load("my-crew", "http://localhost:8080")
# AttributeError: type object 'AIMClient' has no attribute 'auto_register_or_load'

# ❌ THIS ALSO FAILS (used internally but doesn't exist):
aim_client = AIMClient.from_credentials("my-crew")
# AttributeError: type object 'AIMClient' has no attribute 'from_credentials'
```

---

## Fix #1: Add `auto_register_or_load()` Method

**File**: `aim_sdk/client.py`
**Location**: Inside `AIMClient` class (after `__init__`)
**Lines to add**: ~40

```python
@classmethod
def auto_register_or_load(
    cls,
    agent_name: str,
    base_url: str,
    organization_id: Optional[str] = None
) -> "AIMClient":
    """
    Auto-register agent or load from existing credentials.

    This is the recommended one-time setup method that:
    1. Checks if agent is already registered (credentials exist)
    2. If yes: loads from credentials
    3. If no: registers new agent and saves credentials

    Args:
        agent_name: Name for the agent
        base_url: AIM server URL
        organization_id: Optional organization ID

    Returns:
        Configured AIMClient instance

    Example:
        >>> client = AIMClient.auto_register_or_load(
        ...     "my-crew",
        ...     "https://aim.company.com"
        ... )
    """
    # Try to load existing credentials
    try:
        credentials = _load_credentials(agent_name)
        if credentials:
            return cls(
                agent_id=credentials["agent_id"],
                agent_name=credentials["agent_name"],
                private_key=credentials["private_key"],
                public_key=credentials["public_key"],
                base_url=base_url
            )
    except FileNotFoundError:
        pass

    # Register new agent
    agent_data = register_agent(
        name=agent_name,
        agent_type="ai_agent",
        base_url=base_url,
        organization_id=organization_id
    )

    # Save credentials for future use
    _save_credentials(agent_name, {
        "agent_id": agent_data["id"],
        "agent_name": agent_name,
        "private_key": agent_data["private_key"],
        "public_key": agent_data["public_key"]
    })

    # Return configured client
    return cls(
        agent_id=agent_data["id"],
        agent_name=agent_name,
        private_key=agent_data["private_key"],
        public_key=agent_data["public_key"],
        base_url=base_url
    )
```

---

## Fix #2: Add `from_credentials()` Method

**File**: `aim_sdk/client.py`
**Location**: Inside `AIMClient` class (after `auto_register_or_load`)
**Lines to add**: ~25

```python
@classmethod
def from_credentials(
    cls,
    agent_name: str,
    base_url: str = "http://localhost:8080"
) -> "AIMClient":
    """
    Load AIMClient from saved credentials.

    Args:
        agent_name: Name of the agent to load
        base_url: AIM server URL (default: http://localhost:8080)

    Returns:
        Configured AIMClient instance

    Raises:
        FileNotFoundError: If no credentials found for agent_name

    Example:
        >>> client = AIMClient.from_credentials("my-crew")
    """
    credentials = _load_credentials(agent_name)
    if not credentials:
        raise FileNotFoundError(
            f"No credentials found for agent '{agent_name}'. "
            f"Register first using AIMClient.auto_register_or_load()"
        )

    return cls(
        agent_id=credentials["agent_id"],
        agent_name=credentials["agent_name"],
        private_key=credentials["private_key"],
        public_key=credentials["public_key"],
        base_url=base_url
    )
```

---

## Verify the Fix

### Step 1: Check Syntax
```bash
cd /Users/decimai/workspace/agent-identity-management/sdk-test-extracted/aim-sdk-python
python3 -m py_compile aim_sdk/client.py
```

Expected: No output (success)

### Step 2: Test Import
```bash
python3 -c "from aim_sdk import AIMClient; print('✅ auto_register_or_load:', hasattr(AIMClient, 'auto_register_or_load')); print('✅ from_credentials:', hasattr(AIMClient, 'from_credentials'))"
```

Expected output:
```
✅ auto_register_or_load: True
✅ from_credentials: True
```

### Step 3: Run Comprehensive Tests
```bash
python3 test_crewai_integration_comprehensive.py
```

Expected output:
```
================================================================================
                                  TEST SUMMARY
================================================================================

✅ 1.1: Import Validation
✅ 2.1: AIMCrewWrapper - Basic
✅ 2.2: AIMCrewWrapper - Parameters
✅ 2.3: AIMCrewWrapper - Risk Levels
✅ 3.1: @aim_verified_task - Basic
✅ 3.2: @aim_verified_task - Parameters
✅ 3.3: @aim_verified_task - Graceful Degradation  ← Should now PASS
✅ 4.1: AIMTaskCallback - Basic
✅ 4.2: AIMTaskCallback - Parameters
✅ 5.1: Doc Example - Quick Start Option 1
✅ 5.2: Doc Example - Quick Start Option 2
✅ 5.3: Doc Example - Quick Start Option 3
✅ 6.1: Context Logging
✅ 7.1: Edge Case - Empty Crew
✅ 7.2: Edge Case - Long Output
✅ 7.3: Error Handling - Missing Verification ID
✅ 8.1: Feature Completeness

Total: 17/17 tests passed

🎉 ALL TESTS PASSED!
CrewAI integration is fully validated and production-ready.
```

---

## Testing Checklist

- [ ] Syntax check passes
- [ ] Import test passes
- [ ] Test 3.3 now passes (was failing before)
- [ ] All 17/17 tests pass
- [ ] No AttributeError exceptions

---

## What These Methods Do

### `auto_register_or_load()`
**Purpose**: One-time setup for new users
**Behavior**:
1. Checks `~/.aim/credentials.json` for existing agent
2. If found → loads and returns client
3. If not found → registers new agent, saves credentials, returns client

**Usage**:
```python
# First time: registers new agent
client = AIMClient.auto_register_or_load("my-crew", "http://localhost:8080")

# Second time: loads from credentials
client = AIMClient.auto_register_or_load("my-crew", "http://localhost:8080")
```

### `from_credentials()`
**Purpose**: Load existing agent from saved credentials
**Behavior**:
1. Reads `~/.aim/credentials.json`
2. If agent found → returns client
3. If not found → raises `FileNotFoundError`

**Usage**:
```python
# Load existing agent (must be registered first)
client = AIMClient.from_credentials("my-crew")
```

---

## Common Issues After Fix

### Issue: `register_agent()` not defined
**Cause**: Import missing
**Fix**: Add to imports at top of `client.py`:
```python
from aim_sdk.registration import register_agent
```

### Issue: `_load_credentials()` not defined
**Cause**: Function defined later in file
**Fix**: Move `auto_register_or_load()` and `from_credentials()` to after credential functions

### Issue: Type hints fail
**Cause**: Missing imports
**Fix**: Add to imports:
```python
from typing import Optional
```

---

## Files to Modify

1. **`aim_sdk/client.py`** (MUST fix)
   - Add `auto_register_or_load()` classmethod
   - Add `from_credentials()` classmethod
   - Total lines added: ~65

2. **`CREWAI_INTEGRATION.md`** (Optional)
   - Lines 165-175: Clarify callback usage
   - Add note: "Callbacks must be invoked manually or use AIMCrewWrapper"
   - Total changes: ~10 lines

---

## Estimated Timeline

| Task | Time | Priority |
|------|------|----------|
| Implement `auto_register_or_load()` | 2-3h | P0 |
| Implement `from_credentials()` | 1-2h | P0 |
| Test and verify | 30m | P0 |
| Update documentation | 30m | P1 |
| **TOTAL** | **4-6h** | - |

---

## Success Criteria

✅ **All tests pass** (17/17)
✅ **No AttributeError exceptions**
✅ **Documentation examples work**
✅ **Users can follow Quick Start guides**
✅ **Integration is production-ready**

---

## Need Help?

**Test Reports**:
- Full details: `CREWAI_INTEGRATION_TEST_REPORT.md`
- Executive summary: `TEST_SUMMARY.md`
- Issues only: `CREWAI_INTEGRATION_ISSUES.md`

**Test Suite**:
- Run: `python3 test_crewai_integration_comprehensive.py`
- File: `test_crewai_integration_comprehensive.py`

---

**Created**: October 19, 2025
**Priority**: CRITICAL (P0)
**Status**: Ready to implement
