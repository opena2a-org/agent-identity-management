# SDK Quickstart

**Time: 2 minutes** | **Difficulty: Beginner**

Secure your first AI agent with 3 lines of Python code. No configuration needed.

## Prerequisites

- AIM running at http://localhost:3000 (see [Installation](../guides/INSTALLATION.md))
- Python 3.8+

---

## Fastest Way: Run the Demo Agent (60 seconds!)

After downloading the SDK, you can see AIM in action immediately:

```bash
# Extract and install
cd ~/projects  # or wherever you keep projects
unzip ~/Downloads/aim-sdk-python.zip
cd aim-sdk-python
pip install -e .

# Run the interactive demo
python demo_agent.py
```

Open your [AIM Dashboard](http://localhost:3000/dashboard/agents) side-by-side and watch it update in real-time as you trigger actions!

The demo includes weather checks, product searches, user lookups, notifications, and refunds - each with different risk levels so you can see how AIM monitors them.

---

## Step 1: Download the SDK (30 seconds)

The SDK comes pre-configured with your credentials. No API keys to manage.

1. Login to AIM at http://localhost:3000
2. Go to **Settings → SDK Download**
3. Click **Download Python SDK**
4. Extract the ZIP file to your projects folder:

```bash
cd ~/projects  # or ~/dev, ~/Desktop, etc.
unzip ~/Downloads/aim-sdk-python.zip
cd aim-sdk-python
pip install -e .
```

> **Note**: There is NO pip package. The SDK must be downloaded from your AIM instance as it contains your personal credentials.

---

## Step 2: Register Your Agent (1 Line)

Create a new Python file in the extracted SDK folder:

```python
from aim_sdk import secure

# This single line:
# - Registers your agent with AIM
# - Generates Ed25519 cryptographic keys
# - Stores credentials securely
# - Detects capabilities automatically
agent = secure("my-first-agent")
```

Run it:

```bash
python my_agent.py
```

Check the [Agents page](http://localhost:3000/dashboard/agents) to see your agent!

---

## Step 3: Secure Your Actions

Add decorators to track and verify every action your agent takes:

```python
from aim_sdk import secure

agent = secure("my-first-agent")

# Low-risk action: Track and log
@agent.track_action(risk_level="low")
def fetch_weather(city):
    """Every call is verified, logged, and monitored"""
    return f"Weather in {city}: Sunny, 72°F"

# High-risk action: Requires higher trust score
@agent.track_action(risk_level="high")
def send_email(to, subject, body):
    """High-risk actions are flagged and closely monitored"""
    print(f"Sending email to {to}: {subject}")
    return True

# Critical action: Requires admin approval
@agent.require_approval(risk_level="critical")
def delete_database(db_name):
    """This will PAUSE until an admin approves!"""
    print(f"Deleting database: {db_name}")
    return True

# Use your functions normally
result = fetch_weather("San Francisco")
print(result)

# This will send and be logged
send_email("user@example.com", "Hello", "Test message")

# This will WAIT for admin approval before executing
# delete_database("production")  # Uncomment to test
```

---

## Step 4: View Activity in Dashboard

Open AIM at http://localhost:3000 and check:

- **Agents** → See your registered agent with trust score
- **Activity** → View all tracked actions with timestamps
- **Audit Logs** → Complete audit trail for compliance

---

## Complete Working Example

Copy this entire file to get started immediately:

```python
"""
AIM SDK Quick Start Example
Run this file after downloading the SDK from your AIM dashboard.
"""
from aim_sdk import secure

# Initialize your secure agent
agent = secure("weather-bot")

# Define secured actions
@agent.track_action(risk_level="low")
def get_weather(city: str) -> dict:
    """Fetch weather data - low risk, just tracking"""
    return {
        "city": city,
        "temperature": 72,
        "condition": "Sunny"
    }

@agent.track_action(risk_level="medium", resource="database:read")
def get_user_preferences(user_id: str) -> dict:
    """Read user data - medium risk, monitored for anomalies"""
    return {
        "user_id": user_id,
        "preferred_unit": "fahrenheit",
        "notifications": True
    }

@agent.track_action(risk_level="high", resource="notification:send")
def send_weather_alert(user_id: str, message: str) -> bool:
    """Send notifications - high risk, requires good trust score"""
    print(f"Sending to {user_id}: {message}")
    return True

# Main execution
if __name__ == "__main__":
    print("Weather Bot starting...")

    # These actions are all verified and logged
    weather = get_weather("San Francisco")
    print(f"Weather: {weather}")

    prefs = get_user_preferences("user_123")
    print(f"User preferences: {prefs}")

    send_weather_alert("user_123", f"It's {weather['temperature']}F in {weather['city']}!")

    print("All actions completed and logged to AIM!")
```

---

## What's Next?

- **[Register MCP Servers](./mcp-registration.md)** - Connect your agent to MCP servers
- **[Understanding Trust Scores](../quick-start.md#step-5-see-it-work-instant-feedback)** - Learn how AIM evaluates agent trustworthiness
- **[Security Policies](../guides/SECURITY.md)** - Configure automated security rules

---

<div align="center">

[← Back to Tutorials](./README.md) | [API Quickstart →](./api-quickstart.md)

</div>
