# 🐍 Python SDK Guide - Complete Reference

The complete guide to the AIM Python SDK.

## Installation

The SDK is published on PyPI as `aim-sdk`. Install it and sign in to your AIM instance:

```bash
pip install aim-sdk                        # or "aim-sdk[rich]" for compact console output
aim-sdk login --url http://localhost:8080
```

pip installs the SDK's dependencies with it (Python 3.8+). `aim-sdk login` prints a code; approve it on your dashboard's `/device` page. The SDK saves its credentials to `~/.aim/`.

### Alternative: Download a Pre-Configured SDK

The dashboard can also give you the SDK as a ZIP with your credentials embedded, so you skip `aim-sdk login`.

**Steps to Download**:
1. **Login** to AIM Dashboard (http://localhost:3000)
   - Email: `admin@opena2a.org`
   - Password: captured from `aim-bootstrap --default` stdout on first deploy (pre-B2 stacks: `AIM2025!Secure`)
2. **Navigate** to Developers → SDK & docs (`/dashboard/sdk`)
3. **Click** "Download Python SDK" button
4. **Extract** the downloaded `aim-sdk-python-v<version>.zip` to your project directory; it unpacks into an `aim-sdk-python` folder

**Install Dependencies**:

After downloading the SDK, install the required dependencies:

```bash
# Install dependencies
pip install keyring PyNaCl requests cryptography PyJWT

# Or with Poetry
poetry add keyring PyNaCl requests cryptography PyJWT

# Or with pipenv
pipenv install keyring PyNaCl requests cryptography PyJWT
```

**Requirements**:
- Python 3.8+
- `keyring` (secure key storage)
- `PyNaCl` (Ed25519 cryptography)
- `requests` (HTTP client)
- `cryptography` (additional crypto support)
- `PyJWT` (reads the sign-in tokens; `aim-sdk login` and `from aim_sdk import secure` fail without it)

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
    aim_url="http://localhost:8080"
)

# Use your agent normally
# Actions you decorate with @agent.perform_action are verified and logged
```

**That's it!** Your agent is now registered. The agent's Ed25519 keypair is
generated locally and saved in `~/.aim/agents/<name>.json`; the next call to
`secure("my-agent")` loads it instead of registering again.

---

## Core Functions

### `secure()` - The Magic Function

Register and secure an agent with one line. The SDK auto-detects your agent type from imports.

```python
from aim_sdk import secure
import langchain  # SDK detects this!

agent = secure(
    name: str,                    # Agent name (required)
    aim_url: str = None,          # AIM backend URL (default: the URL in your SDK credentials)
    api_key: str = None,          # Agent API key (default: use your SDK credentials)
    agent_type: str = "ai_agent", # Agent type (default: auto-detected from imports)
    description: str = None,      # Agent description (optional)
) -> AIMClient
```

**Parameters**:
- **name** (required): Unique identifier for your agent
- **aim_url**: AIM backend URL (defaults to the URL in your SDK credentials; required with `api_key`)
- **api_key**: Agent API key (`aim_live_...`); when given, `secure()` authenticates with it instead of your SDK credentials
- **agent_type**: Type of agent (auto-detected if not specified). See [Supported Agent Types](#supported-agent-types)
- **description**: Human-readable description

`secure()` takes no private key. The first time it registers an agent, it generates the agent's Ed25519 keypair locally and saves it, with the agent ID, to `~/.aim/agents/<name>.json`; later calls with the same name load that file. To sign with a keypair you hold elsewhere, construct `AIMClient(agent_id=..., public_key=..., private_key=..., aim_url=...)` instead (see [Best Practices](#1-use-environment-variables)).

**Returns**: `AIMClient` instance

**Example**:
```python
# Minimal - agent type auto-detected from imports
from aim_sdk import secure
import langchain  # SDK detects LangChain!

agent = secure("my-agent")  # Auto-detected as "langchain"

# Explicit agent type
from aim_sdk import secure, AgentType
agent = secure("my-agent", agent_type=AgentType.CREWAI)

# Production (with an agent API key instead of SDK credentials)
agent = secure(
    name="production-agent",
    aim_url="https://aim.yourcompany.com",
    api_key=os.getenv("AIM_API_KEY"),
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

### `AIMClient` Class

`secure()` returns an `AIMClient`, the object every call below is made on.

#### Agent Properties

```python
print(agent.agent_id)        # UUID of the agent
print(agent.aim_url)         # AIM backend URL
print(agent.public_key)      # Ed25519 public key (base64)

# The rest of the agent's record (name, status, trust score) is on the server
details = agent.get_agent_details()
```

#### Verify Capability

Manually verify a capability before execution. The call returns only when the
action is permitted; a denial raises `ActionDeniedError`, and no decision
raises `VerificationUnavailableError` (see [Error Handling](#error-handling)).

| Parameter | Default | Description |
|-----------|---------|-------------|
| `capability` | (required) | Capability in `namespace:action` format (e.g. `"user:delete"`) |
| `resource` | `None` | Resource being accessed |
| `context` | `None` | Additional context dict |
| `timeout_seconds` | `300` | Longest wait for an approval |

**Example**:
```python
from aim_sdk.exceptions import ActionDeniedError

try:
    result = agent.verify_capability(
        capability="user:delete",
        resource="user_12345",
        context={"reason": "account_cleanup"}
    )
except ActionDeniedError:
    print("Capability not allowed")
else:
    delete_user(12345)
```

#### Get Trust Score

Get the agent's current trust score and its factors. Scores run from 0.0 to 1.0.

```python
result = agent.get_trust_score()
print(f"Trust Score: {result['score']:.2f}")  # 0.93
print(result["factors"])
# {
#   "verificationStatus": 0.98,
#   "uptime": 1.00,
#   "successRate": 0.98,
#   "securityAlerts": 1.00,
#   "compliance": 1.00,
#   "age": 0.50,
#   "driftDetection": 1.00,
#   "userFeedback": 1.00,
#   "executionIsolation": 0.65
# }

# Another agent in your organization
other = agent.get_trust_score("uuid-of-other-agent")
```

The values are for a verified agent 7 to 30 days old, 98% of whose verifications in the last 30
days succeeded, the latest within a day, with an AIM compliance snapshot of 100 for its
organization, more than ten positive ratings and an unverified isolation attestation (held to
0.65). The nine factors and their weights are in the [Trust Scoring Guide](./trust-scoring.md).

#### Get Trust Score History

Get the recorded score changes, newest first. Without `limit` the server
returns up to 30 entries.

```python
history = agent.get_trust_score_history(limit=10)
for entry in history["history"]:
    print(f"{entry['timestamp']} | {entry['trustScore']:.2f} | {entry['reason']}")
```

#### Get Audit Logs

Get the agent's audit log: the records whose resource is the agent, such as its
creation, updates, verification and suspension. Without `limit` the server
returns up to 50 records, and it returns at most 100; `offset` skips records.

```python
result = agent.get_audit_logs(limit=20)
for record in result["logs"]:
    print(f"{record['timestamp']} | {record['action']} | {record['actorType']}")
print(f"{result['total']} records")
```

The method calls `GET /api/v1/agents/{id}/audit-logs`. A client that signs its
requests may name only its own agent's ID: another agent's ID is answered with
403, which the SDK raises as `AuthenticationError`.

#### Organization Audit Logs and Compliance Reports

The SDK has no method for these: the server routes that serve them need an
administrator's session. Read them in the dashboard, or call the REST API with
an administrator's access token:

- Organization audit logs: `GET /api/v1/admin/audit-logs`
- Compliance report export: `GET /api/v1/compliance/export`

---

## Decorators

### `@agent.perform_action`

Automatically verify and track function calls. Capabilities are auto-registered on first use.

| Parameter | Default | Description |
|-----------|---------|-------------|
| `capability` | function name | Capability name |
| `resource` | `None` | Resource being accessed |
| `context` | `None` | Additional context dict |
| `timeout_seconds` | `300` | Longest wait for an approval |
| `jit_access` | `False` | Require admin approval before each call |
| `risk_level` | `None` | `low`, `medium`, `high` or `critical` |
| `auto_register` | `True` | Register the capability on first use |
| `grant` | `None` | Check the call against this grant |

> **Note**: The `auto_register` parameter (default: `True`) automatically registers capabilities with AIM on first use. This works seamlessly with MONITORING mode, where actions are logged but not blocked.

**Example**:
```python
from aim_sdk import secure
import os
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

| Parameter | Default | Description |
|-----------|---------|-------------|
| `risk_level` | `"high"` | `high` or `critical`; any other value raises `ValueError` |
| `action_name` | function name | Action name shown to the approver |
| `resource` | `None` | Resource being accessed |
| `timeout_seconds` | `3600` | Longest wait for the approval |

**Example**:
```python
from aim_sdk import secure
import os
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

| Parameter | Default | Description |
|-----------|---------|-------------|
| `capability_type` | (required) | Capability name (e.g. `"api:weather"`, `"db:read"`) |
| `description` | `""` | What the capability does |
| `risk_level` | `"medium"` | `low`, `medium`, `high` or `critical` |

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

Request a new capability that requires admin approval. Takes `capability_type`,
`reason` (business justification, min 10 characters) and an optional
`metadata` dict.

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

### Detect MCP Servers

`auto_detect_mcps()` reads the Claude Desktop config and the loaded Python
modules and returns one detection per MCP server it finds. It runs nothing
unless you pass `discover_tools=True`, which launches each configured server
command to ask it for its tools (`timeout_per_server`, default 10 seconds, bounds
each one).

```python
from aim_sdk import auto_detect_mcps

detections = auto_detect_mcps()

print(f"Found {len(detections)} MCP servers:")
for detection in detections:
    print(f"  - {detection['mcpServer']} ({detection['detectionMethod']})")
```

### Report MCP Servers to AIM

Send the detections to AIM, or register one server by name:

```python
from aim_sdk import secure, auto_detect_mcps

agent = secure("my-agent")

# Report everything detected
agent.report_detections(auto_detect_mcps())

# Or register one MCP server with this agent
agent.register_mcp("filesystem")
```

To do both at registration, pass `auto_detect_mcp=True` (scan the Claude
Desktop config) or `mcp_servers=["github", "filesystem"]` to `secure()`.

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
    callbacks=[AIMCallbackHandler(agent=aim_agent)]
)

# Run agent (tool calls are logged to AIM)
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

The SDK has no configuration file. Pass settings to `secure()`, or construct an
`AIMClient` with `timeout` (seconds per request, default 30), `auto_retry`
(default `True`), `max_retries` (default 3) and `enforcement_timeout` (seconds).

### Environment Variables

```bash
# Deadline for one enforcement call, in integer milliseconds
export AIM_ENFORCEMENT_TIMEOUT_MS="5000"

# Agent ID and Ed25519 keypair, for code that passes them to AIMClient
# (see Best Practices; the values are in ~/.aim/agents/<name>.json)
export AIM_AGENT_ID="your-agent-id"
export AIM_PUBLIC_KEY="your-base64-public-key"
export AIM_PRIVATE_KEY="your-base64-private-key"

# Print the SDK's debug messages
export AIM_DEBUG="true"

# Security event log
export AIM_SECURITY_LOG_FILE="$HOME/.aim/security.log"
export AIM_SECURITY_LOG_LEVEL="INFO"
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
    RevocationUnknownError,         # No decision: an ATC's revocation state could not be read
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
- `RevocationUnknownError` subclasses `VerificationUnavailableError`, so a handler
  for that class catches it too; put a handler for `RevocationUnknownError` first.
  `client.secrets.resolve()` raises it when AIM answers `503` with `error`
  `atc_revocation_unknown`: AIM could not read the revocation state of the ATC the
  request presented, so it neither accepted the ATC nor judged it revoked. It
  carries `code`, `status` (503), `retry_after` (the answer's `Retry-After` in whole
  seconds, or `None`), `revocation_state` and `reason`. The client raises it at the
  first such answer and does not wait or send the request again itself. Send it
  again after `retry_after` seconds with the same ATC while it is valid; a new ATC
  does not help.

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
from aim_sdk import AIMClient
import os

# ✅ GOOD - Use environment variables
agent = AIMClient(
    agent_id=os.getenv("AIM_AGENT_ID"),
    public_key=os.getenv("AIM_PUBLIC_KEY"),
    private_key=os.getenv("AIM_PRIVATE_KEY"),
    aim_url=os.getenv("AIM_URL")
)

# ❌ BAD - Hardcode credentials
agent = AIMClient(
    agent_id="550e8400-e29b-41d4-a716-446655440000",
    public_key="hardcoded-public-key",
    private_key="hardcoded-key-123",
    aim_url="http://localhost:8080"
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
    verification = agent.verify_capability("weather:read", resource=city)
    result = requests.get(f"https://api.weather.com/{city}").json()
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

### 4. Monitor Trust Scores

```python
# ✅ GOOD - Monitor trust scores
def check_agent_health():
    """Run daily health check"""
    result = agent.get_trust_score()

    if result["score"] < 0.70:
        send_alert(f"Low trust score: {result['score']:.2f}")

        # Check which factors are low
        for factor, score in result["factors"].items():
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
    aim_url=os.getenv("AIM_URL", "http://localhost:8080")
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
score = agent.get_trust_score()["score"]
print(f"Trust Score: {score:.2f}")
```

---

## API Reference

Complete SDK API documentation: [https://opena2a.org/docs/sdk/api](https://opena2a.org/docs/sdk/api)

---

## Troubleshooting

### Issue: "Authentication failed"

**Error**: `AuthenticationError: Authentication failed - invalid agent credentials`

**Solution**:
1. Check the agent's credentials file exists: `ls ~/.aim/agents/`
2. Verify the agent still exists in the dashboard and is not revoked
3. Register again with `secure("my-agent", force_new=True)` if the saved keys no longer match

### Issue: "Connection refused"

**Error**: `VerificationUnavailableError: Connection refused to http://localhost:8080`

**Solution**:
1. Check AIM backend is running: `curl http://localhost:8080/health`
2. Verify the `aim_url` you pass (or the one in the SDK credentials) is correct
3. Check firewall/network settings

### Issue: "Low trust score"

**Symptoms**: Trust score below 0.70

**Solution**:
```python
result = agent.get_trust_score()

# Check which factors are low
for factor, score in result["factors"].items():
    if score < 0.70:
        print(f"Low {factor}: {score}")
        # Address the specific factor
```

---

## Next Steps

- **[Authentication Guide →](./authentication.md)** - Ed25519 cryptography deep dive
- **[Auto-Detection Guide →](./auto-detection.md)** - MCP server discovery
- **[Trust Scoring Guide →](./trust-scoring.md)** - 9-factor trust algorithm

---

<div align="center">

[🏠 Back to Home](../../README.md) • [📚 SDK Documentation](./index.md) • [💬 Get Help](https://discord.gg/opena2a)

</div>
