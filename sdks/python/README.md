# AIM Python SDK

[![PyPI version](https://badge.fury.io/py/aim-sdk.svg)](https://badge.fury.io/py/aim-sdk)
[![Python Support](https://img.shields.io/pypi/pyversions/aim-sdk.svg)](https://pypi.org/project/aim-sdk/)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

Python SDK for **AIM (Agent Identity Management)** - Automatic identity verification for AI agents and MCP servers.

## Features

✅ **Zero-friction identity verification** - Automatic cryptographic signing and verification
✅ **Ed25519 signatures** - Industry-standard elliptic curve cryptography
✅ **Decorator-based API** - Simple `@perform_action` wrapper for automatic verification
✅ **Automatic retry** - Built-in retry logic for transient failures
✅ **Type hints** - Full type annotations for IDE support
✅ **Context manager support** - Clean resource management with `with` statements

## Installation

```bash
pip install aim-sdk
```

## Quick Start

### 1. Register Your Agent with AIM

First, register your agent at your AIM dashboard (e.g., `https://aim.example.com/dashboard/agents/new`).

AIM will automatically generate cryptographic keys for your agent - no manual key management required!

### 2. Download SDK with Embedded Keys

After registration, download the pre-configured SDK from the success page. Your SDK will come with keys already embedded.

### 3. Use the SDK

```python
from aim_sdk import AIMClient

# Initialize client (keys are already embedded in downloaded SDK)
client = AIMClient(
    agent_id="550e8400-e29b-41d4-a716-446655440000",
    public_key="base64-encoded-public-key",
    private_key="base64-encoded-private-key",
    aim_url="https://aim.example.com"
)

# Automatic verification with decorator
@client.perform_action("read_database", resource="users_table")
def get_user_data(user_id):
    """
    This function is automatically verified before execution.

    1. SDK signs request with your private key
    2. Sends verification request to AIM
    3. Waits for approval (auto-approved based on trust score)
    4. Executes function if approved
    5. Logs result back to AIM
    """
    return database.query("SELECT * FROM users WHERE id = ?", user_id)

# Just call the function - verification happens automatically!
users = get_user_data("123")
```

## Advanced Usage

### Manual Verification

If you need more control over the verification process:

```python
from aim_sdk import AIMClient, ActionDeniedError, VerificationError

client = AIMClient(...)

try:
    # Request verification manually
    verification = client.verify_action(
        action_type="send_email",
        resource="admin@example.com",
        context={
            "subject": "System Alert",
            "recipients": ["admin@example.com"],
            "priority": "high"
        },
        timeout_seconds=300  # Wait up to 5 minutes for approval
    )

    print(f"Verified by: {verification['approved_by']}")
    print(f"Expires at: {verification['expires_at']}")

    # Perform your action
    result = send_email(...)

    # Log the result
    client.log_action_result(
        verification_id=verification['verification_id'],
        success=True,
        result_summary="Email sent successfully"
    )

except ActionDeniedError as e:
    print(f"Action denied: {e}")

except VerificationError as e:
    print(f"Verification failed: {e}")
```

### Context Manager

Use the client as a context manager for automatic cleanup:

```python
from aim_sdk import AIMClient

with AIMClient(...) as client:
    @client.perform_action("delete_file", resource="/tmp/sensitive.txt")
    def cleanup_temp_files():
        os.remove("/tmp/sensitive.txt")

    cleanup_temp_files()
# Client automatically closed
```

### Custom Configuration

```python
client = AIMClient(
    agent_id="...",
    public_key="...",
    private_key="...",
    aim_url="https://aim.example.com",
    timeout=60,        # Request timeout in seconds
    auto_retry=True,   # Automatically retry on failure
    max_retries=5      # Maximum retry attempts
)
```

## Action Types

Common action types you can use:

- `read_database` - Database read operations
- `write_database` - Database write operations
- `send_email` - Email sending
- `access_api` - External API calls
- `read_file` - File system reads
- `write_file` - File system writes
- `execute_command` - Shell command execution
- `access_secret` - Secret/credential access

You can define custom action types based on your organization's policies.

## Error Handling

The SDK provides specific exceptions for different failure scenarios:

```python
from aim_sdk import (
    AIMError,              # Base exception
    AuthenticationError,   # Invalid credentials
    VerificationError,     # Verification request failed
    ActionDeniedError,     # Action explicitly denied
    ConfigurationError     # SDK misconfigured
)

try:
    result = get_user_data("123")

except AuthenticationError:
    print("Invalid agent credentials - check your keys")

except ActionDeniedError as e:
    print(f"Action denied by AIM: {e}")

except VerificationError as e:
    print(f"Verification failed: {e}")

except ConfigurationError as e:
    print(f"SDK configuration error: {e}")
```

## How It Works

1. **Registration**: You register your agent with AIM through the web dashboard
2. **Automatic Keys**: AIM generates Ed25519 key pair automatically (no manual crypto!)
3. **SDK Download**: You download pre-configured SDK with embedded keys
4. **Runtime Verification**: Every `@perform_action` decorated function:
   - Creates signed verification request
   - Sends to AIM server
   - Waits for approval (auto-approved based on trust score or requires manual approval)
   - Executes function if approved
   - Logs result to build trust score

## Security

- **Ed25519 signatures** - Industry-standard elliptic curve cryptography
- **Private keys never transmitted** - Only signatures are sent to AIM
- **Automatic key generation** - Prevents weak or predictable keys
- **Encrypted storage** - Private keys encrypted with AES-256-GCM on AIM server
- **Trust scoring** - Agent behavior tracked to enable auto-approval for trusted agents

## Requirements

- Python 3.8 or higher
- Internet connection to AIM server

## Development

```bash
# Clone repository
git clone https://github.com/opena2a-org/agent-identity-management
cd agent-identity-management/sdks/python

# Install with dev dependencies
pip install -e ".[dev]"

# Run tests
pytest tests/ -v --cov=aim_sdk

# Format code
black aim_sdk/

# Type checking
mypy aim_sdk/
```

## License

Apache License 2.0 - See [LICENSE](LICENSE) for details.

## Support

- **Documentation**: https://docs.opena2a.org/aim
- **Issues**: https://github.com/opena2a-org/agent-identity-management/issues
- **Discussions**: https://github.com/opena2a-org/agent-identity-management/discussions

## Contributing

Contributions are welcome! Please read our [Contributing Guide](CONTRIBUTING.md) for details.

---

Built with ❤️ by [OpenA2A](https://opena2a.org)
