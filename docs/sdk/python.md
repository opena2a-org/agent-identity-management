# 🐍 Python SDK Guide - Complete Reference

The complete guide to the AIM Python SDK.

## Installation

<div className="bg-red-50 border border-red-200 rounded-lg p-4 mb-6">
  <h3 className="text-lg font-semibold text-red-900 mb-2">⚠️ NO pip install Available</h3>
  <p className="text-sm text-red-800 mb-2">
    <strong>There is NO pip package for the AIM SDK.</strong> You must download it from your AIM dashboard with pre-configured credentials.
  </p>
</div>

### Download Pre-Configured SDK (ONLY Option)

The AIM SDK comes with your embedded credentials - zero configuration required.

**Steps to Download**:
1. **Login** to AIM Dashboard (http://localhost:3000)
   - Email: `admin@opena2a.org`
   - Password: captured from `aim-bootstrap --default` stdout on first deploy (pre-B2 stacks: `AIM2025!Secure`)
2. **Navigate** to Settings → SDK Download
3. **Click** "Download Python SDK" button
4. **Extract** the downloaded ZIP file to your project directory

### Install Dependencies

After downloading the SDK, install the required dependencies:

```bash
# Install dependencies
pip install keyring PyNaCl requests cryptography

# Or with Poetry
poetry add keyring PyNaCl requests cryptography

# Or with pipenv
pipenv install keyring PyNaCl requests cryptography
```

**Requirements**:
- Python 3.8+
- `keyring` (secure key storage)
- `PyNaCl` (Ed25519 cryptography)
- `requests` (HTTP client)
- `cryptography` (additional crypto support)

### Verify Installation

```bash
python -c "from aim_sdk import secure; print('✅ AIM SDK installed!')"
```

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

Register and secure an agent with one line. The SDK auto-detects your agent type from imports.

```python
from aim_sdk import secure
import langchain  # SDK detects this!

agent = secure(
    name: str,                    # Agent name (required)
    aim_url: str = None,          # AIM backend URL (default: http://localhost:8080)
    private_key: str = None,      # Ed25519 private key (default: auto-generate)
    agent_type: str = None,       # Agent type (default: auto-detected from imports)
    description: str = None,      # Agent description (optional)
    auto_verify: bool = True      # Auto-verify actions (default: True)
) -> AIMAgent
```

**Parameters**:
- **name** (required): Unique identifier for your agent
- **aim_url**: AIM backend URL (defaults to `http://localhost:8080`)
- **private_key**: Ed25519 private key (auto-generates if not provided)
- **agent_type**: Type of agent (auto-detected if not specified). See [Supported Agent Types](#supported-agent-types)
- **description**: Human-readable description
- **auto_verify**: Automatically verify all actions (recommended)

**Returns**: `AIMAgent` instance

**Example**:
```python
# Minimal - agent type auto-detected from imports
from aim_sdk import secure
import langchain  # SDK detects LangChain!

agent = secure("my-agent")  # Auto-detected as "langchain"

# Explicit agent type
from aim_sdk import secure, AgentType
agent = secure("my-agent", agent_type=AgentType.CREWAI)

# Production (with existing key)
agent = secure(
    name="production-agent",
    aim_url="https://aim.yourcompany.com",
    private_key=os.getenv("AIM_PRIVATE_KEY"),
    agent_type=AgentType.CLAUDE,
    description="Production customer service agent"
)
```

---

## Supported Agent Types

The SDK automatically detects your agent type based on imported packages:

| Category | Types | Auto-Detected From |
|----------|-------|-------------------|
| **Frameworks** | LangChain, LlamaIndex, CrewAI, AutoGen, LangGraph, Haystack, Semantic Kernel | `langchain`, `llama_index`, `crewai`, `autogen`, etc. |
| **LLM Providers** | Claude, GPT, Gemini, Llama, Mistral, Cohere | `anthropic`, `openai`, `google.generativeai`, etc. |
| **Copilots** | Copilot, Assistant, Chatbot | For interactive assistants |
| **Autonomous** | AutoGPT, BabyAGI | Self-directed agents |

**Detection Priority**: Frameworks > Autonomous > LLM Providers

If you import both `langchain` and `anthropic`, the agent type will be `langchain` (frameworks take precedence).

### Using AgentType Constants

```python
from aim_sdk import secure, AgentType

# Available constants
AgentType.CLAUDE      # "claude"
AgentType.GPT         # "gpt"
AgentType.GEMINI      # "gemini"
AgentType.LLAMA       # "llama"
AgentType.MISTRAL     # "mistral"
AgentType.COHERE      # "cohere"
AgentType.LANGCHAIN   # "langchain"
AgentType.LLAMAINDEX  # "llamaindex"
AgentType.CREWAI      # "crewai"
AgentType.AUTOGEN     # "autogen"
AgentType.LANGGRAPH   # "langgraph"
AgentType.HAYSTACK    # "haystack"
AgentType.SEMANTIC_KERNEL  # "semantic_kernel"
AgentType.COPILOT     # "copilot"
AgentType.ASSISTANT   # "assistant"
AgentType.CHATBOT     # "chatbot"
AgentType.AUTOGPT     # "autogpt"
AgentType.BABYAGI     # "babyagi"
AgentType.CUSTOM      # "custom"

# Use in registration
agent = secure("my-agent", agent_type=AgentType.CREWAI)
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

#### Verify Capability

Manually verify a capability before execution.

```python
agent.verify_capability(
    capability: str,            # Capability in namespace:action format (e.g., "user:delete")
    resource: str = None,       # Resource being accessed (optional)
    context: dict = None        # Additional context (optional)
) -> dict
```

**Example**:
```python
# Verify before executing sensitive operation
result = agent.verify_capability(
    capability="user:delete",
    resource="user_12345",
    context={"reason": "account_cleanup"}
)
if result.get("verified"):
    # Verification succeeded, execute action
    delete_user(12345)
else:
    # Verification failed
    print("Capability not allowed or requires approval")
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

### `@agent.perform_action`

Automatically verify and track function calls. Capabilities are auto-registered on first use.

```python
from aim_sdk import secure

agent = secure("my-agent")

@agent.perform_action(
    capability: str = None,     # Capability name (default: function name)
    risk_level: str = "medium", # Risk level (low/medium/high/critical)
    resource: str = None,       # Resource being accessed (optional)
    jit_access: bool = False,   # Require admin approval (for critical actions)
    auto_register: bool = True  # Auto-register capability on first use (default: True)
)
def your_function(*args, **kwargs):
    # Function implementation
    pass
```

> **Note**: The `auto_register` parameter (default: `True`) automatically registers capabilities with AIM on first use. This works seamlessly with MONITORING mode, where actions are logged but not blocked.

**Example**:
```python
from aim_sdk import secure
import requests

agent = secure("weather-agent")

@agent.perform_action(capability="get_weather", risk_level="low")
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

## Capability Management

### `agent.register_capability()`

Explicitly register a capability with AIM. This is optional when using `@agent.perform_action` with `auto_register=True` (the default).

```python
agent.register_capability(
    capability_type: str,       # Capability name (e.g., "api:weather", "db:read")
    description: str = "",      # Description of what this capability does
    risk_level: str = "medium", # Risk level (low/medium/high/critical)
    auto_approve: bool = False  # Request auto-approval for low-risk capabilities
) -> dict
```

**Example**:
```python
from aim_sdk import secure

agent = secure("my-agent")

# Explicitly register capabilities on startup
agent.register_capability("api:weather", "Check weather data", risk_level="low")
agent.register_capability("db:read", "Read from database", risk_level="medium")
agent.register_capability("file:write", "Write files to disk", risk_level="high")

# Now use the capabilities
@agent.perform_action(capability="api:weather", risk_level="low")
def get_weather(city: str):
    return {"temp": 72, "city": city}
```

> **When to use**: Use `register_capability()` when you want to declare all your agent's capabilities upfront for visibility. The `@agent.perform_action` decorator will auto-register capabilities on first use if `auto_register=True` (default).

### `agent.request_capability()`

Request a new capability that requires admin approval.

```python
agent.request_capability(
    capability_type: str,       # Capability being requested
    reason: str                 # Business justification (min 10 characters)
) -> dict
```

**Example**:
```python
from aim_sdk import secure

agent = secure("my-agent")

# Request elevated capability
result = agent.request_capability(
    capability_type="db:admin",
    reason="Need database admin access to run migration scripts"
)

print(f"Request status: {result['status']}")  # "pending", "approved", or "rejected"
```

---

## Enforcement Modes

AIM supports two global enforcement modes that affect how capabilities are verified:

### MONITORING Mode (Default)

In MONITORING mode, all actions are **allowed** but logged for visibility:

- Actions proceed even without pre-registered capabilities
- Violations are logged and alerted but NOT blocked
- Individual security policies generate alerts but don't block
- Ideal for development, testing, and initial deployment

### STRICT Mode

In STRICT mode, capabilities must be granted before use:

- Actions are **blocked** if the agent lacks the required capability
- Individual security policies can block actions
- Admin must grant capabilities before agents can use them
- Ideal for production environments with strict security requirements

> **Tip**: Start with MONITORING mode to see how your agents behave, then switch to STRICT mode once you've configured the appropriate capabilities and policies.

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
    AIMError,                       # Base class for every SDK exception
    AuthenticationError,            # Credentials rejected
    ActionDeniedError,              # AIM ANSWERED and refused the action
    VerificationUnavailableError,   # No decision obtained (2.0.0+)
    VerificationError,              # Verification request failed
    ConfigurationError,             # SDK is misconfigured
)
```

`ActionDeniedError` and `VerificationUnavailableError` are the two outcomes that
must never be confused, and the class hierarchy enforces it:

- `ActionDeniedError` also subclasses `PermissionError`, so `except PermissionError`
  catches a denial. (It therefore also satisfies `except OSError`, which
  `PermissionError` descends from.)
- `VerificationUnavailableError` deliberately does **not** subclass
  `VerificationError`, `ActionDeniedError` or `PermissionError`. A handler written
  for "AIM said no" must not silently absorb "AIM was never asked".
- Both subclass `AIMError`, which is the broadest compatible catch.

### Example Error Handling

```python
from aim_sdk import secure
from aim_sdk.exceptions import (
    ActionDeniedError,
    AuthenticationError,
    VerificationUnavailableError,
)

try:
    agent = secure("my-agent")

    # Returns only when the action is permitted.
    agent.verify_capability(
        capability="db:delete",
        resource="production-database",
        context={"reason": "maintenance"}
    )

except ActionDeniedError as e:
    print(f"AIM denied the action: {e}")
    # A decision was made and it was "no". Grant the capability to this agent
    # in the dashboard, or accept the denial.

except VerificationUnavailableError as e:
    print(f"AIM returned no decision: {e}")
    # NOT a denial. AIM was unreachable, rate-limited, or errored. Whether the
    # action was blocked depends on the organization's enforcement mode.

except AuthenticationError as e:
    print(f"Authentication failed: {e}")
    # The agent's credentials were rejected. Check the credentials file.
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
@agent.perform_action(risk_level="low")
def get_weather(city: str):
    return requests.get(f"https://api.weather.com/{city}").json()

# ❌ BAD - Manual tracking everywhere
def get_weather(city: str):
    agent.verify_capability("weather:read", resource=city)
    result = requests.get(f"https://api.weather.com/{city}").json()
    agent.log_capability_result(verification_id, success=True)
    return result
```

### 3. Use Risk Levels Appropriately

```python
# ✅ GOOD - Appropriate risk levels
@agent.perform_action(risk_level="low")
def read_data(id: int):
    pass

@agent.require_approval(risk_level="high")
def update_data(id: int, data: dict):
    pass

@agent.require_approval(risk_level="critical")
def delete_all_data():
    pass

# ❌ BAD - Everything is low risk
@agent.perform_action(risk_level="low")
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

    @agent.perform_action(risk_level="low")
    def get_weather(self, city: str, units: str = "imperial") -> dict:
        """Get current weather for a city"""
        response = requests.get(
            self.base_url,
            params={"q": city, "appid": self.api_key, "units": units}
        )
        response.raise_for_status()
        return response.json()

    @agent.perform_action(risk_level="low")
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

Complete SDK API documentation: [https://opena2a.org/docs/sdk/api](https://opena2a.org/docs/sdk/api)

---

## Troubleshooting

### Issue: "Authentication failed"

**Error**: `AuthenticationError: Invalid private key`

**Solution**:
1. Check `AIM_PRIVATE_KEY` is set: `echo $AIM_PRIVATE_KEY`
2. Verify key matches agent registered in dashboard
3. Ensure key is valid Ed25519 private key

### Issue: "Connection refused"

**Error**: `VerificationUnavailableError: Connection refused to http://localhost:8080`

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
