# 🐍 Python SDK Guide - Complete Reference

The complete guide to the AIM Python SDK.

## Installation

```bash
# Install from PyPI
pip install aim-sdk

# Or install from source
git clone https://github.com/opena2a/agent-identity-management.git
cd agent-identity-management/sdk/python
pip install -e .
```

**Requirements**:
- Python 3.8+
- `cryptography` (Ed25519 support)
- `requests` (HTTP client)

---

## Quick Start (30 Seconds)

```python
from aim_sdk import secure

# ONE LINE - Secure your agent!
agent = secure(
    name="my-agent",
    aim_url="http://localhost:8080",
    private_key="your-private-key-here"
)

# Use your agent normally
# All actions are automatically verified and logged
```

**That's it!** Your agent is now secure.

---

## Core Functions

### `secure()` - The Magic Function

Register and secure an agent with one line.

```python
from aim_sdk import secure

agent = secure(
    name: str,                    # Agent name (required)
    aim_url: str = None,          # AIM backend URL (default: http://localhost:8080)
    private_key: str = None,      # Ed25519 private key (default: auto-generate)
    agent_type: str = "ai_agent", # Agent type (default: ai_agent)
    description: str = None,      # Agent description (optional)
    auto_verify: bool = True      # Auto-verify actions (default: True)
) -> AIMAgent
```

**Parameters**:
- **name** (required): Unique identifier for your agent
- **aim_url**: AIM backend URL (defaults to `http://localhost:8080`)
- **private_key**: Ed25519 private key (auto-generates if not provided)
- **agent_type**: Type of agent (`ai_agent`, `mcp_server`, `multi_agent_team`)
- **description**: Human-readable description
- **auto_verify**: Automatically verify all actions (recommended)

**Returns**: `AIMAgent` instance

**Example**:
```python
# Minimal (auto-generates keypair)
agent = secure("my-agent")

# Production (with existing key)
agent = secure(
    name="production-agent",
    aim_url="https://aim.yourcompany.com",
    private_key=os.getenv("AIM_PRIVATE_KEY"),
    agent_type="ai_agent",
    description="Production customer service agent"
)
```

---

### `AIMAgent` Class

The main class representing a secured agent.

#### Agent Properties

```python
# Access agent information
print(agent.id)              # UUID of agent
print(agent.name)            # Agent name
print(agent.public_key)      # Ed25519 public key
print(agent.trust_score)     # Current trust score (0.0 - 1.0)
print(agent.is_verified)     # Verification status
print(agent.created_at)      # Creation timestamp
```

#### Verify Action

Manually verify an action before execution.

```python
agent.verify_action(
    action_name: str,           # Name of the action
    parameters: dict = None,    # Action parameters (optional)
    risk_level: str = "low"     # Risk level (low/medium/high/critical)
) -> bool
```

**Example**:
```python
# Verify before executing sensitive operation
if agent.verify_action(
    action_name="delete_user",
    parameters={"user_id": 12345},
    risk_level="high"
):
    # Verification succeeded, execute action
    delete_user(12345)
else:
    # Verification failed
    print("Action not allowed or requires approval")
```

#### Log Action

Manually log an action after execution.

```python
agent.log_action(
    action_name: str,           # Name of the action
    parameters: dict = None,    # Action parameters
    result: any = None,         # Action result
    success: bool = True,       # Whether action succeeded
    error: str = None           # Error message if failed
) -> None
```

**Example**:
```python
# Execute and log action
result = fetch_weather("San Francisco")

agent.log_action(
    action_name="fetch_weather",
    parameters={"city": "San Francisco"},
    result=result,
    success=True
)
```

#### Get Trust Score

Get current trust score with breakdown.

```python
agent.get_trust_score(
    detailed: bool = False      # Include factor breakdown
) -> Union[float, dict]
```

**Example**:
```python
# Simple trust score
score = agent.get_trust_score()
print(f"Trust Score: {score}")  # 0.95

# Detailed breakdown
breakdown = agent.get_trust_score(detailed=True)
print(breakdown)
# {
#   "overall": 0.95,
#   "factors": {
#     "verification_status": 1.00,
#     "uptime": 1.00,
#     "success_rate": 0.98,
#     "security_alerts": 1.00,
#     "compliance": 1.00,
#     "age": 0.50,
#     "drift_detection": 1.00,
#     "user_feedback": 1.00
#   }
# }
```

#### Get Audit Logs

Retrieve audit trail for compliance.

```python
agent.get_audit_logs(
    limit: int = 100,           # Max number of logs
    offset: int = 0,            # Pagination offset
    start_date: str = None,     # Filter by start date (ISO 8601)
    end_date: str = None,       # Filter by end date (ISO 8601)
    action_name: str = None     # Filter by action name
) -> list[dict]
```

**Example**:
```python
# Get last 100 logs
logs = agent.get_audit_logs(limit=100)

# Get logs for specific action
delete_logs = agent.get_audit_logs(
    action_name="delete_user",
    start_date="2025-10-01T00:00:00Z"
)

# Print logs
for log in logs:
    print(f"{log['timestamp']} | {log['action_name']} | {log['status']}")
```

#### Export Compliance Report

Generate compliance reports for SOC 2, HIPAA, GDPR.

```python
agent.export_compliance_report(
    report_type: str = "soc2",  # Report type (soc2/hipaa/gdpr)
    start_date: str = None,     # Start date (ISO 8601)
    end_date: str = None,       # End date (ISO 8601)
    format: str = "json"        # Output format (json/csv/pdf)
) -> Union[dict, str, bytes]
```

**Example**:
```python
# SOC 2 compliance report
report = agent.export_compliance_report(
    report_type="soc2",
    start_date="2025-09-01T00:00:00Z",
    end_date="2025-09-30T00:00:00Z",
    format="pdf"
)

# Save to file
with open("soc2_report_sept.pdf", "wb") as f:
    f.write(report)
```

---

## Decorators

### `@agent.track_action`

Automatically track and verify function calls.

```python
from aim_sdk import secure

agent = secure("my-agent")

@agent.track_action(
    action_name: str = None,    # Custom action name (default: function name)
    risk_level: str = "low"     # Risk level (low/medium/high/critical)
)
def your_function(*args, **kwargs):
    # Function implementation
    pass
```

**Example**:
```python
from aim_sdk import secure
import requests

agent = secure("weather-agent")

@agent.track_action(action_name="get_weather", risk_level="low")
def get_weather(city: str) -> dict:
    """Fetch weather data for a city"""
    response = requests.get(
        "https://api.openweathermap.org/data/2.5/weather",
        params={"q": city, "appid": os.getenv("OPENWEATHER_API_KEY")}
    )
    return response.json()

# Calling this function automatically:
# 1. Verifies the action with AIM
# 2. Executes the function
# 3. Logs the result to AIM
weather = get_weather("San Francisco")
```

### `@agent.require_approval`

Require human approval before executing high-risk actions.

```python
from aim_sdk import secure

agent = secure("database-agent")

@agent.require_approval(
    risk_level: str = "high"    # Risk level (high/critical)
)
def delete_user(user_id: int):
    # Function implementation
    pass
```

**Example**:
```python
from aim_sdk import secure
import psycopg2

agent = secure("database-agent")

@agent.require_approval(risk_level="critical")
def delete_all_users():
    """
    Delete all users from database

    CRITICAL RISK - Requires urgent approval
    Execution pauses until human approves in dashboard
    """
    with psycopg2.connect(os.getenv("DATABASE_URL")) as conn:
        cursor = conn.cursor()
        cursor.execute("DELETE FROM users")
        return {"deleted": cursor.rowcount}

# This will pause and show approval request in AIM dashboard
# Only executes if approved by human
result = delete_all_users()  # Waits for approval
```

---

## MCP Integration

### Auto-Detect MCP Servers

Automatically discover MCP servers from Claude Desktop.

```python
from aim_sdk import auto_detect_mcp_servers

servers = auto_detect_mcp_servers(
    config_path: str = None     # Custom config path (default: auto-detect)
) -> list[dict]
```

**Example**:
```python
from aim_sdk import auto_detect_mcp_servers

# Auto-detect all MCP servers
servers = auto_detect_mcp_servers()

print(f"Found {len(servers)} MCP servers:")
for server in servers:
    print(f"  - {server['name']}: {server['command']}")

# Output:
# Found 3 MCP servers:
#   - filesystem: npx -y @modelcontextprotocol/server-filesystem
#   - github: npx -y @modelcontextprotocol/server-github
#   - postgres: npx -y @modelcontextprotocol/server-postgres
```

### Register MCP Server

Register an MCP server with AIM.

```python
from aim_sdk import register_mcp_server

result = register_mcp_server(
    name: str,                  # Server name
    command: str,               # Command to start server
    args: list[str] = None,     # Command arguments
    env: dict = None            # Environment variables
) -> dict
```

**Example**:
```python
from aim_sdk import register_mcp_server

# Register filesystem MCP server
result = register_mcp_server(
    name="filesystem",
    command="npx",
    args=["-y", "@modelcontextprotocol/server-filesystem"],
    env={"ALLOWED_DIRECTORY": "~/Documents"}
)

print(f"Server ID: {result['server_id']}")
print(f"Public Key: {result['public_key']}")
```

### Register All MCP Servers

Register all auto-detected MCP servers at once.

```python
from aim_sdk import register_all_mcp_servers

results = register_all_mcp_servers(
    servers: list[dict] = None  # List of servers (default: auto-detect)
) -> list[dict]
```

**Example**:
```python
from aim_sdk import auto_detect_mcp_servers, register_all_mcp_servers

# Auto-detect and register all MCP servers
servers = auto_detect_mcp_servers()
results = register_all_mcp_servers(servers)

print(f"Registered {len(results)} MCP servers:")
for result in results:
    print(f"  - {result['name']}: {result['server_id']}")
```

---

## Framework Integrations

### LangChain

```python
from aim_sdk import secure
from aim_sdk.integrations.langchain import AIMCallbackHandler
from langchain.agents import AgentExecutor

# Secure your LangChain agent
aim_agent = secure("langchain-agent")

# Add AIM callback
agent_executor = AgentExecutor(
    agent=agent,
    tools=tools,
    callbacks=[AIMCallbackHandler(aim_agent=aim_agent)]
)

# Run agent (all actions automatically verified and logged)
result = agent_executor.run("What's the weather in SF?")
```

### CrewAI

```python
from aim_sdk import secure
from aim_sdk.integrations.crewai import AIMCrewWrapper
from crewai import Crew

# Secure your CrewAI team
aim_crew = secure("research-crew")

# Create crew
crew = Crew(agents=[researcher, writer], tasks=[research_task, write_task])

# Wrap with AIM security
secured_crew = AIMCrewWrapper(crew, aim_agent=aim_crew)

# Run crew (all agent actions verified and logged)
result = secured_crew.kickoff(inputs={"topic": "AI in Healthcare"})
```

### Microsoft Copilot

```python
from aim_sdk import secure
from aim_sdk.integrations.copilot import CopilotPlugin

# Secure your Copilot plugin
aim_agent = secure("hr-copilot")

class HRPlugin(CopilotPlugin):
    def __init__(self):
        super().__init__(
            name="HR Assistant",
            version="1.0.0",
            aim_agent=aim_agent
        )

    async def check_leave_balance(self, employee_id: str):
        # Implementation
        pass

# Start plugin (all actions verified and logged)
plugin = HRPlugin()
plugin.start(host="0.0.0.0", port=8095)
```

---

## Configuration

### Environment Variables

```bash
# AIM Backend URL
export AIM_URL="http://localhost:8080"

# Agent Private Key (Ed25519)
export AIM_PRIVATE_KEY="your-private-key-here"

# Enable debug logging
export AIM_DEBUG="true"

# API timeout (seconds)
export AIM_TIMEOUT="30"

# Retry attempts for failed requests
export AIM_RETRY_ATTEMPTS="3"
```

### Configuration File

Create `~/.aim/config.yaml`:

```yaml
# AIM SDK Configuration
aim_url: "http://localhost:8080"
timeout: 30
retry_attempts: 3
debug: false

# Logging
log_level: "INFO"
log_file: "~/.aim/aim-sdk.log"

# Security
verify_ssl: true
```

Load configuration:

```python
from aim_sdk import load_config

config = load_config("~/.aim/config.yaml")
agent = secure("my-agent", **config)
```

---

## Error Handling

### Common Exceptions

```python
from aim_sdk.exceptions import (
    AIMAuthenticationError,     # Authentication failed
    AIMVerificationError,        # Action verification failed
    AIMConnectionError,          # Cannot connect to AIM backend
    AIMTimeoutError,             # Request timed out
    AIMRateLimitError           # Rate limit exceeded
)
```

### Example Error Handling

```python
from aim_sdk import secure
from aim_sdk.exceptions import AIMVerificationError, AIMConnectionError

try:
    agent = secure("my-agent")

    # Verify high-risk action
    agent.verify_action(
        action_name="delete_database",
        parameters={"database": "production"},
        risk_level="critical"
    )

except AIMAuthenticationError as e:
    print(f"Authentication failed: {e}")
    # Check AIM_PRIVATE_KEY environment variable

except AIMVerificationError as e:
    print(f"Action verification failed: {e}")
    # Action was rejected by AIM

except AIMConnectionError as e:
    print(f"Cannot connect to AIM backend: {e}")
    # Check if AIM backend is running

except AIMTimeoutError as e:
    print(f"Request timed out: {e}")
    # Increase AIM_TIMEOUT or check network
```

---

## Best Practices

### 1. Use Environment Variables

```python
# ✅ GOOD - Use environment variables
agent = secure(
    name="my-agent",
    aim_url=os.getenv("AIM_URL"),
    private_key=os.getenv("AIM_PRIVATE_KEY")
)

# ❌ BAD - Hardcode credentials
agent = secure(
    name="my-agent",
    aim_url="http://localhost:8080",
    private_key="hardcoded-key-123"
)
```

### 2. Use Decorators for Automatic Tracking

```python
# ✅ GOOD - Use decorators
@agent.track_action(risk_level="low")
def get_weather(city: str):
    return requests.get(f"https://api.weather.com/{city}").json()

# ❌ BAD - Manual tracking everywhere
def get_weather(city: str):
    agent.verify_action("get_weather", {"city": city})
    result = requests.get(f"https://api.weather.com/{city}").json()
    agent.log_action("get_weather", {"city": city}, result)
    return result
```

### 3. Use Risk Levels Appropriately

```python
# ✅ GOOD - Appropriate risk levels
@agent.track_action(risk_level="low")
def read_data(id: int):
    pass

@agent.require_approval(risk_level="high")
def update_data(id: int, data: dict):
    pass

@agent.require_approval(risk_level="critical")
def delete_all_data():
    pass

# ❌ BAD - Everything is low risk
@agent.track_action(risk_level="low")
def delete_all_data():  # Should be critical!
    pass
```

### 4. Export Compliance Reports Regularly

```python
# ✅ GOOD - Regular compliance reporting
def monthly_compliance_report():
    """Run on 1st of each month"""
    report = agent.export_compliance_report(
        report_type="soc2",
        start_date=first_day_of_last_month(),
        end_date=last_day_of_last_month(),
        format="pdf"
    )
    send_to_compliance_team(report)

# Schedule this to run monthly
```

### 5. Monitor Trust Scores

```python
# ✅ GOOD - Monitor trust scores
def check_agent_health():
    """Run daily health check"""
    breakdown = agent.get_trust_score(detailed=True)

    if breakdown["overall"] < 0.70:
        send_alert(f"Low trust score: {breakdown['overall']}")

        # Check which factors are low
        for factor, score in breakdown["factors"].items():
            if score < 0.70:
                print(f"⚠️  Low {factor}: {score}")
```

---

## Examples

### Complete Weather Agent

```python
from aim_sdk import secure
import requests
import os

# Secure agent
agent = secure(
    name="weather-agent",
    aim_url=os.getenv("AIM_URL", "http://localhost:8080"),
    private_key=os.getenv("AIM_PRIVATE_KEY")
)

class WeatherAgent:
    def __init__(self):
        self.api_key = os.getenv("OPENWEATHER_API_KEY")
        self.base_url = "https://api.openweathermap.org/data/2.5/weather"

    @agent.track_action(risk_level="low")
    def get_weather(self, city: str, units: str = "imperial") -> dict:
        """Get current weather for a city"""
        response = requests.get(
            self.base_url,
            params={"q": city, "appid": self.api_key, "units": units}
        )
        response.raise_for_status()
        return response.json()

    @agent.track_action(risk_level="low")
    def get_forecast(self, city: str) -> str:
        """Get human-readable weather forecast"""
        weather = self.get_weather(city)
        temp = weather['main']['temp']
        description = weather['weather'][0]['description']
        return f"🌤️  Weather in {city}: {temp}°F, {description}"

# Use agent
weather_agent = WeatherAgent()
forecast = weather_agent.get_forecast("San Francisco")
print(forecast)

# Check trust score
score = agent.get_trust_score()
print(f"Trust Score: {score}")
```

---

## API Reference

Complete SDK API documentation: [https://docs.opena2a.org/sdk/api](https://docs.opena2a.org/sdk/api)

---

## Troubleshooting

### Issue: "Authentication failed"

**Error**: `AIMAuthenticationError: Invalid private key`

**Solution**:
1. Check `AIM_PRIVATE_KEY` is set: `echo $AIM_PRIVATE_KEY`
2. Verify key matches agent registered in dashboard
3. Ensure key is valid Ed25519 private key

### Issue: "Connection refused"

**Error**: `AIMConnectionError: Connection refused to http://localhost:8080`

**Solution**:
1. Check AIM backend is running: `curl http://localhost:8080/health`
2. Verify `AIM_URL` is correct
3. Check firewall/network settings

### Issue: "Low trust score"

**Symptoms**: Trust score below 0.70

**Solution**:
```python
# Get detailed breakdown
breakdown = agent.get_trust_score(detailed=True)

# Check which factors are low
for factor, score in breakdown["factors"].items():
    if score < 0.70:
        print(f"Low {factor}: {score}")
        # Address the specific factor
```

---

## Next Steps

- **[Authentication Guide →](./authentication.md)** - Ed25519 cryptography deep dive
- **[Auto-Detection Guide →](./auto-detection.md)** - MCP server discovery
- **[Trust Scoring Guide →](./trust-scoring.md)** - 8-factor trust algorithm

---

<div align="center">

[🏠 Back to Home](../../README.md) • [📚 SDK Documentation](./index.md) • [💬 Get Help](https://discord.gg/opena2a)

</div>
