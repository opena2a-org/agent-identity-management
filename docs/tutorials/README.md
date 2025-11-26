# 5-Minute Tutorials

Get started with AIM quickly. Each tutorial is designed to deliver value in under 5 minutes.

## Quick Start Tutorials

| Tutorial | Time | Description |
|----------|------|-------------|
| [SDK Quickstart](./sdk-quickstart.md) | 2 min | Secure your first agent with 3 lines of Python |
| [API Quickstart](./api-quickstart.md) | 3 min | REST API examples with curl |
| [Dashboard Walkthrough](./dashboard-walkthrough.md) | 3 min | Navigate the AIM dashboard |
| [MCP Registration](./mcp-registration.md) | 3 min | Register and attest MCP servers |

## Zero to Secured in 60 Seconds

```python
from aim_sdk import secure

# 1. Register your agent (1 line)
agent = secure("my-agent")

# 2. Secure your actions (1 decorator)
@agent.track_action(risk_level="low")
def call_api(data):
    return requests.post("https://api.example.com", json=data)

# That's it! Every call is now:
# - Verified before execution
# - Logged to audit trail
# - Monitored for anomalies
# - Trust score updated
```

## Prerequisites

Before starting any tutorial, ensure:

1. **AIM is running** - See [Installation](../guides/INSTALLATION.md)
   ```bash
   docker compose up -d
   ```

2. **Access the dashboard** - http://localhost:3000
   - Email: `admin@opena2a.org`
   - Password: `AIM2025!Secure` (change on first login)

3. **Download the SDK** - Settings → SDK Download

## Tutorial Goals

Each tutorial focuses on one thing:

- **SDK Quickstart** → Register an agent and track actions programmatically
- **API Quickstart** → Use REST API for automation and integrations
- **Dashboard Walkthrough** → Navigate UI and manage agents visually
- **MCP Registration** → Connect agents to MCP servers with drift detection

## Next Steps

After completing the tutorials:

1. **[Full Quick Start Guide](../quick-start.md)** - Comprehensive 10-minute guide
2. **[Python SDK Reference](../sdk/python.md)** - Complete SDK documentation
3. **[API Reference](../API.md)** - All 160 endpoints
4. **[Security Best Practices](../guides/SECURITY.md)** - Production hardening
