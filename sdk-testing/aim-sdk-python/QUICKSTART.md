# AIM Python SDK - Quick Start

This SDK is pre-configured with your credentials!

## Installation

1. Unzip this file:
   ```bash
   unzip aim-sdk-python.zip
   cd aim-sdk-python
   ```

2. Install the SDK:
   ```bash
   pip install -e .
   ```

## Usage

The SDK is already configured with your identity. Just use it!

```python
from aim_sdk import AIMClient

# Zero configuration needed! Your credentials are embedded.
client = AIMClient()

# Register an agent
agent = client.register_agent(
    name="my-awesome-agent",
    agent_type="ai_agent",
    description="An agent that does amazing things"
)

print(f"Agent registered! ID: {agent['id']}")
print(f"Trust Score: {agent.get('trust_score', 'N/A')}")
```

## Automatic Authentication

Your SDK contains embedded OAuth credentials that automatically:
- ✅ Authenticate your agent registrations
- ✅ Link agents to your user account
- ✅ Refresh tokens when they expire
- ✅ Work for 90 days without re-authentication

## Security

Your credentials are stored in `.aim/credentials.json`. Keep this file secure!

⚠️ **Important Security Notes:**
- Credentials are valid for 90 days
- Never commit credentials to Git
- Revoke tokens from dashboard if compromised
- Tokens can be revoked at any time from your dashboard

For more examples, see the included test files.
