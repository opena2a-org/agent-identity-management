# AIM Python SDK

**"AIM is Stripe for AI Agent Identity"**

One-line agent registration with automatic cryptographic verification.

## Quick Start (ONE LINE!)

```python
from aim_sdk import register_agent

# That's it! Agent is registered, verified, and ready to use
agent = register_agent("my-agent", "http://localhost:8080")

# Use decorator-based verification for sensitive actions
@agent.perform_action("send_email", resource="admin@example.com")
def send_critical_notification(message):
    # AIM automatically verifies this action before execution
    send_email("admin@example.com", message)
    
# Call the function - AIM handles verification automatically
send_critical_notification("System alert!")
```

## Installation

```bash
pip install -r requirements.txt
```

## Features

- ✅ **One-line registration**: No manual key generation needed
- ✅ **Automatic key management**: Ed25519 keys generated and stored securely
- ✅ **Local credential storage**: Credentials saved to `~/.aim/credentials.json`
- ✅ **Decorator-based verification**: Simple `@agent.perform_action()` decorator
- ✅ **Challenge-response auth**: Cryptographic proof without exposing private keys
- ✅ **Trust scoring**: ML-powered risk assessment
- ✅ **Audit logging**: Complete action history

## Usage

### Option 1: One-Line Registration (Recommended)

```python
from aim_sdk import register_agent

# Register agent with minimal configuration
agent = register_agent(
    name="my-agent",
    aim_url="http://localhost:8080"
)

# Advanced registration with metadata
agent = register_agent(
    name="my-agent",
    aim_url="http://localhost:8080",
    display_name="My Awesome Agent",
    description="Production agent for user management",
    version="1.0.0",
    repository_url="https://github.com/myorg/my-agent",
    documentation_url="https://docs.myorg.com"
)
```

### Option 2: Manual Initialization (If you have existing credentials)

```python
from aim_sdk import AIMClient

client = AIMClient(
    agent_id="your-agent-id",
    public_key="base64-public-key",
    private_key="base64-private-key",
    aim_url="http://localhost:8080"
)
```

### Performing Verified Actions

```python
# Simple action verification
@agent.perform_action("read_database", resource="users_table")
def get_user_data(user_id):
    return database.query(f"SELECT * FROM users WHERE id = {user_id}")

# Action with additional context
@agent.perform_action(
    "modify_user", 
    resource="user:12345",
    metadata={"reason": "Account update requested by user"}
)
def update_user_email(user_id, new_email):
    return database.execute(
        "UPDATE users SET email = ? WHERE id = ?",
        new_email, user_id
    )

# High-risk action (requires higher trust score)
@agent.perform_action(
    "delete_data",
    resource="user:12345",
    risk_level="high"
)
def delete_user_account(user_id):
    return database.execute("DELETE FROM users WHERE id = ?", user_id)
```

## Credential Storage

Credentials are automatically saved to `~/.aim/credentials.json` with secure permissions (0600).

**⚠️ Security Warning**: The private key is only returned ONCE during registration. Keep it safe!

```json
{
  "my-agent": {
    "agent_id": "550e8400-e29b-41d4-a716-446655440000",
    "public_key": "base64-encoded-public-key",
    "private_key": "base64-encoded-private-key",
    "aim_url": "http://localhost:8080",
    "status": "verified",
    "trust_score": 75.0,
    "registered_at": "2025-10-07T16:05:27.143786Z"
  }
}
```

## Examples

See `example.py` for a complete working example.

```bash
python example.py
```

## Requirements

- Python 3.8+
- requests
- pynacl (for Ed25519 cryptography)

## License

MIT
