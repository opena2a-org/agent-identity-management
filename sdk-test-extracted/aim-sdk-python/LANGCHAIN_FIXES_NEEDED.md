# LangChain Integration - Required Fixes

**Priority**: HIGH - Blocks production deployment
**Estimated Time**: 1-2 days
**Impact**: Will bring test pass rate from 87% → 100%

---

## Fix #1: Implement `AIMClient.from_credentials()` ⚠️ CRITICAL

**File**: `aim_sdk/client.py`
**Issue**: Method referenced in `decorators.py:79` but doesn't exist

### Implementation

```python
# Add to AIMClient class in aim_sdk/client.py

@classmethod
def from_credentials(cls, agent_name: str) -> 'AIMClient':
    """
    Load AIM agent credentials from ~/.aim/credentials.json

    This method allows agents to be automatically loaded without
    manually providing keys, enabling zero-friction integration.

    Args:
        agent_name: Name of the agent to load (e.g., "langchain-agent")

    Returns:
        Initialized AIMClient instance with loaded credentials

    Raises:
        FileNotFoundError: If credentials file doesn't exist or agent not found
        ConfigurationError: If credentials are invalid or corrupted

    Example:
        >>> client = AIMClient.from_credentials("my-agent")
        >>> client.verify_action("read_database", "users_table")
    """
    import json
    from pathlib import Path

    # Credentials file location
    creds_file = Path.home() / ".aim" / "credentials.json"

    # Check if file exists
    if not creds_file.exists():
        raise FileNotFoundError(
            f"Credentials file not found: {creds_file}\n"
            f"Run AIMClient.auto_register_or_load() to create it."
        )

    # Load credentials
    try:
        with open(creds_file, 'r') as f:
            all_credentials = json.load(f)
    except json.JSONDecodeError as e:
        raise ConfigurationError(f"Invalid credentials file format: {e}")

    # Find agent credentials
    if agent_name not in all_credentials:
        available = ", ".join(all_credentials.keys())
        raise FileNotFoundError(
            f"Agent '{agent_name}' not found in credentials.\n"
            f"Available agents: {available}\n"
            f"Run AIMClient.auto_register_or_load('{agent_name}', 'https://aim.example.com') to register."
        )

    creds = all_credentials[agent_name]

    # Validate required fields
    required_fields = ['agent_id', 'public_key', 'private_key', 'aim_url']
    missing = [f for f in required_fields if f not in creds]
    if missing:
        raise ConfigurationError(
            f"Invalid credentials for '{agent_name}': missing fields {missing}"
        )

    # Initialize client
    return cls(
        agent_id=creds['agent_id'],
        public_key=creds['public_key'],
        private_key=creds['private_key'],
        aim_url=creds['aim_url']
    )
```

### Credentials File Format

Expected format for `~/.aim/credentials.json`:
```json
{
  "langchain-agent": {
    "agent_id": "550e8400-e29b-41d4-a716-446655440000",
    "public_key": "base64-encoded-public-key",
    "private_key": "base64-encoded-private-key",
    "aim_url": "https://aim.example.com"
  },
  "another-agent": {
    "agent_id": "660e8400-e29b-41d4-a716-446655440001",
    "public_key": "base64-encoded-public-key-2",
    "private_key": "base64-encoded-private-key-2",
    "aim_url": "https://aim.example.com"
  }
}
```

### Tests to Add

```python
# In tests/test_client.py

def test_from_credentials_success(tmp_path):
    """Test loading credentials from file"""
    # Create credentials file
    creds_file = tmp_path / ".aim" / "credentials.json"
    creds_file.parent.mkdir(parents=True)

    credentials = {
        "test-agent": {
            "agent_id": "test-agent-id",
            "public_key": "test-public-key",
            "private_key": "test-private-key",
            "aim_url": "http://localhost:8080"
        }
    }

    with open(creds_file, 'w') as f:
        json.dump(credentials, f)

    # Mock Path.home() to return tmp_path
    with patch('pathlib.Path.home', return_value=tmp_path):
        client = AIMClient.from_credentials("test-agent")
        assert client.agent_id == "test-agent-id"


def test_from_credentials_file_not_found():
    """Test error when credentials file doesn't exist"""
    with pytest.raises(FileNotFoundError, match="Credentials file not found"):
        AIMClient.from_credentials("nonexistent-agent")


def test_from_credentials_agent_not_found(tmp_path):
    """Test error when agent not in credentials"""
    creds_file = tmp_path / ".aim" / "credentials.json"
    creds_file.parent.mkdir(parents=True)

    with open(creds_file, 'w') as f:
        json.dump({"other-agent": {}}, f)

    with patch('pathlib.Path.home', return_value=tmp_path):
        with pytest.raises(FileNotFoundError, match="Agent 'test-agent' not found"):
            AIMClient.from_credentials("test-agent")
```

---

## Fix #2: Implement `AIMClient.auto_register_or_load()` ⚠️ CRITICAL

**File**: `aim_sdk/client.py`
**Issue**: Referenced in all documentation but doesn't exist

### Implementation

```python
# Add to AIMClient class in aim_sdk/client.py

@classmethod
def auto_register_or_load(
    cls,
    agent_name: str,
    aim_url: str,
    agent_type: str = "ai_agent",
    capabilities: list = None,
    **kwargs
) -> 'AIMClient':
    """
    Automatically register a new agent or load existing credentials.

    This is the recommended way to initialize an AIMClient for new agents.
    It provides zero-friction setup by:
    1. Checking if agent already registered (credentials exist)
    2. If yes, loading existing credentials
    3. If no, registering new agent and saving credentials

    Args:
        agent_name: Name for the agent (e.g., "langchain-agent")
        aim_url: AIM server URL (e.g., "https://aim.example.com")
        agent_type: Type of agent (default: "ai_agent")
        capabilities: List of capabilities (default: [])
        **kwargs: Additional parameters for registration

    Returns:
        Initialized AIMClient instance

    Example:
        >>> # First time - registers new agent
        >>> client = AIMClient.auto_register_or_load(
        ...     "langchain-agent",
        ...     "https://aim.example.com"
        ... )
        Agent 'langchain-agent' registered successfully!

        >>> # Subsequent times - loads existing credentials
        >>> client = AIMClient.auto_register_or_load(
        ...     "langchain-agent",
        ...     "https://aim.example.com"
        ... )
        Agent 'langchain-agent' loaded from credentials
    """
    import json
    from pathlib import Path

    # Try to load existing credentials first
    try:
        client = cls.from_credentials(agent_name)
        print(f"✅ Agent '{agent_name}' loaded from credentials")
        return client
    except FileNotFoundError:
        # Agent not found - need to register
        pass

    # Register new agent
    print(f"🔧 Registering new agent '{agent_name}'...")

    # Generate Ed25519 key pair
    from nacl.signing import SigningKey
    from nacl.encoding import Base64Encoder

    signing_key = SigningKey.generate()
    private_key = signing_key.encode(encoder=Base64Encoder).decode('utf-8')
    public_key = signing_key.verify_key.encode(encoder=Base64Encoder).decode('utf-8')

    # Register with AIM server
    import requests

    registration_data = {
        "name": agent_name,
        "type": agent_type,
        "public_key": public_key,
        "capabilities": capabilities or [],
        **kwargs
    }

    try:
        response = requests.post(
            f"{aim_url.rstrip('/')}/api/v1/agents/register",
            json=registration_data,
            timeout=30
        )
        response.raise_for_status()

        registration_result = response.json()
        agent_id = registration_result.get('agent_id')

        if not agent_id:
            raise ConfigurationError("Registration succeeded but no agent_id returned")

    except requests.RequestException as e:
        raise ConfigurationError(f"Failed to register agent with AIM server: {e}")

    # Save credentials to file
    creds_dir = Path.home() / ".aim"
    creds_file = creds_dir / "credentials.json"

    # Create directory if needed
    creds_dir.mkdir(parents=True, exist_ok=True)

    # Load existing credentials or create new
    if creds_file.exists():
        with open(creds_file, 'r') as f:
            all_credentials = json.load(f)
    else:
        all_credentials = {}

    # Add new agent credentials
    all_credentials[agent_name] = {
        "agent_id": agent_id,
        "public_key": public_key,
        "private_key": private_key,
        "aim_url": aim_url,
        "registered_at": datetime.now(timezone.utc).isoformat()
    }

    # Save credentials
    with open(creds_file, 'w') as f:
        json.dump(all_credentials, f, indent=2)

    # Set secure permissions (owner read/write only)
    import os
    os.chmod(creds_file, 0o600)

    print(f"✅ Agent '{agent_name}' registered successfully!")
    print(f"   Agent ID: {agent_id}")
    print(f"   Credentials saved to: {creds_file}")

    # Initialize and return client
    return cls(
        agent_id=agent_id,
        public_key=public_key,
        private_key=private_key,
        aim_url=aim_url
    )
```

### Tests to Add

```python
# In tests/test_client.py

def test_auto_register_or_load_new_agent(tmp_path, mock_aim_server):
    """Test registering a new agent"""
    # Mock AIM server registration endpoint
    mock_aim_server.register('/api/v1/agents/register',
        json={'agent_id': 'new-agent-id'})

    with patch('pathlib.Path.home', return_value=tmp_path):
        client = AIMClient.auto_register_or_load(
            "test-agent",
            "http://localhost:8080"
        )

        # Verify client initialized
        assert client.agent_id == "new-agent-id"

        # Verify credentials saved
        creds_file = tmp_path / ".aim" / "credentials.json"
        assert creds_file.exists()

        with open(creds_file) as f:
            creds = json.load(f)

        assert "test-agent" in creds
        assert creds["test-agent"]["agent_id"] == "new-agent-id"


def test_auto_register_or_load_existing_agent(tmp_path):
    """Test loading existing agent"""
    # Create existing credentials
    creds_file = tmp_path / ".aim" / "credentials.json"
    creds_file.parent.mkdir(parents=True)

    existing_creds = {
        "test-agent": {
            "agent_id": "existing-agent-id",
            "public_key": "existing-public-key",
            "private_key": "existing-private-key",
            "aim_url": "http://localhost:8080"
        }
    }

    with open(creds_file, 'w') as f:
        json.dump(existing_creds, f)

    with patch('pathlib.Path.home', return_value=tmp_path):
        client = AIMClient.auto_register_or_load(
            "test-agent",
            "http://localhost:8080"
        )

        # Should load existing, not register new
        assert client.agent_id == "existing-agent-id"
```

---

## Fix #3: Update Documentation Examples 📝

**Files**: `LANGCHAIN_INTEGRATION.md`, all examples

### Issue
LangChain's `@tool` decorator requires either:
1. A docstring on the function, OR
2. A `description` parameter

### Fix All Examples

**Before (broken)**:
```python
@tool
@aim_verify(agent=aim_client, risk_level="low")
def read_data(id: str) -> str:  # ❌ No docstring
    return f"Data for {id}"
```

**After (works)**:
```python
@tool
@aim_verify(agent=aim_client, risk_level="low")
def read_data(id: str) -> str:
    '''Read data from database'''  # ✅ Docstring added
    return f"Data for {id}"
```

### Specific File Changes

#### LANGCHAIN_INTEGRATION.md

**Lines 48-50** - Add docstring:
```python
@tool
def search_database(query: str) -> str:
    '''Search the company database'''  # ✅ Add this
    return f"Results for: {query}"
```

**Lines 53-55** - Add docstring:
```python
@tool
def send_email(to: str, subject: str) -> str:
    '''Send an email'''  # ✅ Add this
    return f"Email sent to {to}"
```

**Lines 336-339** - Already have docstrings ✅

**Lines 451-453** - Add docstring:
```python
@tool
def search_tickets(query: str) -> str:
    '''Search support tickets'''  # ✅ Add this
    return tickets_db.search(query)
```

**Lines 456-458** - Add docstring:
```python
@tool
def update_ticket_status(ticket_id: str, status: str) -> str:
    '''Update ticket status'''  # ✅ Add this
    return tickets_db.update(ticket_id, status)
```

### Add Documentation Note

Add this section after "Installation" section:

```markdown
## ⚠️ Important: LangChain Tool Requirements

When using LangChain's `@tool` decorator, you **must** provide either:

1. **A docstring** (recommended):
   ```python
   @tool
   def my_tool(input: str) -> str:
       '''Tool description goes here'''  # ← Required
       return process(input)
   ```

2. **A description parameter**:
   ```python
   @tool(description="Tool description goes here")
   def my_tool(input: str) -> str:
       return process(input)
   ```

Without one of these, you'll get:
```
ValueError: Function must have a docstring if description not provided.
```

**Best Practice**: Always use docstrings - they serve as both tool descriptions
and code documentation.
```

---

## Fix #4: Update Test Suite 🧪

**File**: `test_langchain_integration_comprehensive.py`

### Add Docstrings to All Test Tools

**Lines affected**: Tests 5.5, 6.3, 6.4

**Fix**:
```python
# In test_documentation_examples(), test 5.5
@tool
@aim_verify(agent=mock_client, risk_level="low")
def read_data(id: str) -> str:
    '''Read data from database'''  # ✅ Add this
    return f"Data for {id}"

# In test_error_handling(), test 6.3
@tool
@aim_verify(agent=failing_client)
def failing_tool(input: str) -> str:
    '''Test tool that may fail'''  # ✅ Add this
    return f"Result: {input}"

# In test_error_handling(), test 6.4
@tool
@aim_verify(agent=normal_client)
def error_tool(input: str) -> str:
    '''Test tool that raises errors'''  # ✅ Add this
    raise ValueError("Tool execution failed")
```

### Fix Tool Invocation Pattern (Test 3.7)

**Before (broken)**:
```python
@tool
@aim_verify(agent=mock_client)
def resource_tool(resource_id: str, action: str) -> str:
    '''Resource tool'''
    return f"Action {action} on {resource_id}"

# ❌ Wrong - LangChain tools don't accept multiple positional args
result = resource_tool.invoke("resource-123", "delete")
```

**After (works)**:
```python
@tool
@aim_verify(agent=mock_client)
def resource_tool(input: str) -> str:
    '''Resource tool - input format: "resource_id action"'''
    parts = input.split()
    resource_id = parts[0] if len(parts) > 0 else ""
    action = parts[1] if len(parts) > 1 else ""
    return f"Action {action} on {resource_id}"

# ✅ Correct - single string argument
result = resource_tool.invoke("resource-123 delete")
```

---

## Fix #5: Add Import Statements 📦

**File**: `aim_sdk/client.py`

### Required Imports

Add these imports to the top of `client.py`:

```python
import json
import os
from pathlib import Path
from datetime import datetime, timezone
from typing import Optional, List, Dict, Any

# These may already be present, but ensure they are:
import base64
import hashlib
import time
import requests
from nacl.signing import SigningKey
from nacl.encoding import Base64Encoder
```

---

## Validation Checklist ✅

After implementing fixes, verify:

- [ ] `test_langchain_integration_comprehensive.py` passes 100% (39/39 tests)
- [ ] All documentation examples run without errors
- [ ] Credentials file created correctly at `~/.aim/credentials.json`
- [ ] Credentials file has secure permissions (0600)
- [ ] `from_credentials()` loads existing agents
- [ ] `auto_register_or_load()` registers new agents
- [ ] `auto_register_or_load()` loads existing agents on second call
- [ ] Error messages are helpful and actionable
- [ ] All docstrings present on `@tool` decorated functions

### Run Tests

```bash
# Run comprehensive test suite
cd /Users/decimai/workspace/agent-identity-management/sdk-test-extracted/aim-sdk-python
python3 test_langchain_integration_comprehensive.py

# Run unit tests
python3 -m pytest tests/ -v

# Verify documentation examples
python3 -c "
from langchain_core.tools import tool
from aim_sdk import AIMClient
from aim_sdk.integrations.langchain import aim_verify

# Test auto_register_or_load
client = AIMClient.auto_register_or_load('test-agent', 'http://localhost:8080')
print('✅ auto_register_or_load works')

# Test from_credentials
client2 = AIMClient.from_credentials('test-agent')
print('✅ from_credentials works')
"
```

---

## Expected Results After Fixes

```
================================================================================
COMPREHENSIVE TEST SUMMARY
================================================================================

Section Results:
--------------------------------------------------------------------------------
✅ PASS: Import Validation
✅ PASS: AIMCallbackHandler Tests
✅ PASS: @aim_verify Decorator Tests
✅ PASS: Tool Wrapper Tests
✅ PASS: Documentation Examples
✅ PASS: Error Handling Tests
✅ PASS: Feature Completeness

TOTAL: 39/39 tests passed (100.0%)

🎉 ALL TESTS PASSED - LangChain integration is fully functional!
```

---

## Estimated Timeline

| Task | Time | Priority |
|------|------|----------|
| Implement `from_credentials()` | 2 hours | HIGH |
| Implement `auto_register_or_load()` | 3 hours | HIGH |
| Write unit tests | 2 hours | HIGH |
| Update documentation | 1 hour | MEDIUM |
| Fix test suite | 1 hour | MEDIUM |
| Manual testing & validation | 1 hour | MEDIUM |

**Total**: 10 hours (~1-2 days)

---

## Questions & Answers

**Q: Can we ship without these fixes?**
A: ❌ No - all documentation examples are broken

**Q: What's the minimum viable fix?**
A: Implement both methods (`from_credentials`, `auto_register_or_load`) and update all examples with docstrings

**Q: Can users work around the missing methods?**
A: Yes, but it defeats the "zero-friction" goal:
```python
# Workaround (not user-friendly)
client = AIMClient(
    agent_id="manual-id",
    public_key="manual-key",
    private_key="manual-key",
    aim_url="https://aim.example.com"
)
```

**Q: Are there any security concerns?**
A: Yes - credentials file must have 0600 permissions (owner read/write only)

---

**Next Steps**: Implement fixes in priority order, run tests after each fix, validate all examples work end-to-end.
