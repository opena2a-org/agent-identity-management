# 🚀 Quick Start Guide - 5 Minutes to Secure Agent

Welcome! This guide will get you from zero to a fully secure AI agent in **just 5 minutes**.

## What You'll Build

By the end of this guide, you'll have:
- ✅ AIM platform running (local or Azure)
- ✅ Your first agent registered and secured
- ✅ Real-time trust scoring active
- ✅ Complete audit trail capturing actions
- ✅ Security dashboard monitoring your agent

**Time required**: 5 minutes
**Difficulty**: Beginner
**Prerequisites**: Docker (for local) or Azure account (for cloud)

---

## Step 1: Deploy AIM (2 minutes)

### Option A: Local Development (Fastest) ⚡

```bash
# Clone the repository
git clone https://github.com/opena2a/agent-identity-management.git
cd agent-identity-management

# Start with Docker Compose
docker compose up -d

# Wait ~60 seconds for services to start
```

**Access Points**:
- 🌐 Dashboard: http://localhost:3000
- 🔌 Backend API: http://localhost:8080
- 📊 Grafana: http://localhost:3003

**Default Admin Login**:
- Email: `admin@opena2a.org`
- Password: `AIM2025!Secure` (⚠️ Change on first login!)

### Option B: Azure Production (One Command) ☁️

```bash
# Clone the repository
git clone https://github.com/opena2a/agent-identity-management.git
cd agent-identity-management

# Deploy to Azure (creates all infrastructure)
./scripts/deploy-azure-production.sh

# Wait ~10 minutes for deployment
```

**What Gets Created**:
- PostgreSQL database (with auto-initialization)
- Redis cache
- Backend API (Container App)
- Frontend dashboard (Container App)
- SSL/TLS certificates
- Health monitoring

**Access Points** (from deployment output):
- 🌐 Dashboard: `https://aim-prod-frontend.*.azurecontainerapps.io`
- 🔌 Backend API: `https://aim-prod-backend.*.azurecontainerapps.io`

---

## Step 2: Create Your First Agent (30 seconds)

### 2.1 Register Agent in Dashboard

1. **Login** to the AIM dashboard (http://localhost:3000)
2. **Navigate** to "Agents" → "Register New Agent"
3. **Fill in**:
   - **Agent Name**: `weather-agent`
   - **Agent Type**: `AI Agent`
   - **Description**: `Fetches weather data from API`
4. **Click** "Register Agent"

**✅ Success!** You'll see a private key displayed. **Copy this immediately** (it's only shown once).

### 2.2 Save Your Private Key

```bash
# Save to environment variable (recommended)
export AIM_PRIVATE_KEY="your-private-key-from-dashboard"

# Or save to .env file
echo "AIM_PRIVATE_KEY=your-private-key-from-dashboard" >> .env
```

**⚠️ Important**: Never commit private keys to version control!

---

## Step 3: Download AIM SDK (15 seconds)

1. **Login** to AIM dashboard
2. **Navigate** to Settings → SDK Download
3. **Click** "Download SDK" (includes pre-configured credentials)
4. **Extract** the downloaded ZIP file

```bash
# Extract SDK
unzip aim-sdk-python.zip
cd aim-sdk-python

# Install dependencies
pip install -r requirements.txt
```

**Note**: There is NO pip package. The SDK must be downloaded from your AIM instance as it contains your personal credentials.

**Verify Installation**:
```bash
python -c "from aim_sdk import secure; print('✅ AIM SDK installed!')"
```

---

## Step 4: Secure Your Agent (3 lines of code!)

### Create a Simple Weather Agent

Create a file called `weather_agent.py`:

```python
from aim_sdk import secure
import requests
import os

# LINE 1: Register your agent (zero config!)
agent = secure("weather-agent")

# LINE 2: Add @agent.track_action to verify every call
@agent.track_action(risk_level="low")
def get_weather(city: str):
    """Fetch weather data for a city"""
    # LINE 3: Your normal code - AIM verifies BEFORE this runs
    response = requests.get(
        f"https://api.openweathermap.org/data/2.5/weather",
        params={
            "q": city,
            "appid": "your-openweather-api-key",  # Get free key: https://openweathermap.org/api
            "units": "imperial"
        }
    )
    return response.json()

# Use your agent
if __name__ == "__main__":
    # This action is verified BEFORE execution
    # Logged to audit trail AUTOMATICALLY
    # Trust score updated AUTOMATICALLY
    weather = get_weather("San Francisco")

    print(f"🌤️  Weather in San Francisco:")
    print(f"   Temperature: {weather['main']['temp']}°F")
    print(f"   Conditions: {weather['weather'][0]['description']}")
    print(f"   Humidity: {weather['main']['humidity']}%")
```

**Run it**:
```bash
python weather_agent.py
```

**Expected Output**:
```
🌤️  Weather in San Francisco:
   Temperature: 62.5°F
   Conditions: clear sky
   Humidity: 65%
```

---

## Step 5: See It Work (Instant Feedback!)

### 5.1 Check Your Dashboard

Open the AIM dashboard (http://localhost:3000) and navigate to "Agents" → "weather-agent"

**You'll see**:

**Agent Status Card**:
```
✅ ACTIVE
Trust Score: 0.95 (Excellent)
Last Verified: 3 seconds ago
Total Actions: 1
```

**Recent Activity**:
```
✅ get_weather("San Francisco")  |  3 seconds ago  |  SUCCESS
   Response time: 245ms
   Resource: api.openweathermap.org
```

**Trust Score Breakdown**:
```
✅ Verification Status:     100% (1.00)  [25%]
✅ Uptime & Availability:   100% (1.00)  [15%]
✅ Action Success Rate:     100% (1.00)  [15%]
✅ Security Alerts:           0  (1.00)  [15%]
✅ Compliance Score:        100% (1.00)  [10%]
✅ Age & History:           New  (0.50)  [10%]
✅ Drift Detection:         None (1.00)  [ 5%]
✅ User Feedback:           None (1.00)  [ 5%]

Overall Trust Score: 0.95 / 1.00
```

**Audit Trail**:
```
📝 2025-10-21 14:32:15 UTC  |  Agent registered
📝 2025-10-21 14:35:42 UTC  |  Action verified: get_weather
```

### 5.2 Security Alerts (None! 🎉)

```
No security alerts. Your agent is behaving normally.
```

### 5.3 Compliance Reports

```
✅ SOC 2 Compliance:  100%
✅ HIPAA Compliance:  100%
✅ GDPR Compliance:   100%

Export Report: [CSV] [PDF] [JSON]
```

---

## 🎉 Congratulations!

You've just secured your first AI agent in **5 minutes**!

### What Just Happened?

Behind those **3 lines of code**, AIM prevents your agent from going rogue:

1. ✅ **Line 1** (`secure("weather-agent")`) - Cryptographic identity created
2. ✅ **Line 2** (`@agent.track_action`) - **Verification BEFORE execution** (prevents malicious actions!)
3. ✅ **Line 3** (your code) - Runs ONLY if verification passes

**Every time `get_weather()` is called**, AIM automatically:

- 🛡️ **Verifies** the action BEFORE it executes (prevents unauthorized API calls!)
- 📝 **Logs** to immutable audit trail (who, what, when, why)
- 📊 **Updates** real-time trust score (8-factor algorithm)
- 🚨 **Monitors** for anomalies (unusual patterns trigger alerts)
- 🔐 **Signs** with Ed25519 cryptography (tamper-proof verification)

**Without `@agent.track_action`?** Your agent can do ANYTHING without oversight. ❌
**With `@agent.track_action`?** Every action verified, logged, monitored. ✅

This is **the difference between a rogue agent and a trusted agent**.

---

## 🚀 Next Steps

### 1. Explore More Examples

- [Weather Agent Example](./examples/weather-agent.md) - Complete tutorial (what you just built!)
- [Flight Tracker Agent](./examples/flight-tracker.md) - Real-time flight tracking
- [Database Agent](./examples/database-agent.md) - Advanced security for DB access

### 2. Integrate with Your Framework

- [CrewAI Integration](./integrations/crewai.md) - Secure multi-agent teams
- [LangChain Integration](./integrations/langchain.md) - Secure agent frameworks
- [MCP Servers](./integrations/mcp.md) - Model Context Protocol

### 3. Learn the SDK

- [Python SDK Guide](./sdk/python.md) - Complete SDK reference
- [Authentication](./sdk/authentication.md) - Ed25519 deep dive
- [Auto-Detection](./sdk/auto-detection.md) - MCP auto-discovery
- [Trust Scoring](./sdk/trust-scoring.md) - How trust works

### 4. Deploy to Production

- [Azure Deployment](./deployment/azure.md) - Production-ready Azure setup
- [Kubernetes](./deployment/kubernetes.md) - Large scale
- [Security Best Practices](./security/best-practices.md) - Harden your deployment

---

## 🎓 Understanding Decorators (The Secret Sauce)

###  What's a Decorator?

**Think of it like airport security**: Your function is a passenger, and `@agent.track_action` is the security checkpoint.

```
┌─────────────────────────────────────────────────────────────────────┐
│                    WITHOUT @agent.track_action                      │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  User Request  →  Agent  →  External API  →  Done                  │
│                     ▲                                               │
│                     │                                               │
│                 NO CHECKS                                           │
│                 NO LOGS                                             │
│                 NO ALERTS                                           │
│                                                                     │
│  ❌ Agent can call ANY API                                          │
│  ❌ Agent can exfiltrate data                                       │
│  ❌ No audit trail                                                  │
│  ❌ No anomaly detection                                            │
│  ❌ Attacker has free reign                                         │
└─────────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────────┐
│                     WITH @agent.track_action                        │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  User Request  →  Agent  →  🛡️  AIM VERIFICATION  →  External API  │
│                              │                                      │
│                              ├──> ✅ Cryptographic signature         │
│                              ├──> ✅ Trust score check               │
│                              ├──> ✅ Anomaly detection               │
│                              ├──> ✅ Rate limiting                   │
│                              ├──> ✅ Audit logging                   │
│                              │                                      │
│                              └──> ⛔ BLOCK if suspicious             │
│                                                                     │
│  ✅ Every action verified BEFORE execution                          │
│  ✅ Complete audit trail (who, what, when, why)                     │
│  ✅ Anomaly detection catches attacks                               │
│  ✅ Trust score drops trigger alerts                                │
│  ✅ Admin notified immediately                                      │
└─────────────────────────────────────────────────────────────────────┘
```

```python
# WITHOUT decorator = No security checkpoint
def get_weather(city):
    return call_api(city)  # ❌ Anyone can board the plane

# WITH decorator = Security checkpoint BEFORE boarding
@agent.track_action(risk_level="low")
def get_weather(city):
    return call_api(city)  # ✅ Verified passenger only
```

### How Does It Work?

```python
from aim_sdk import secure

agent = secure("my-agent")

# The @ symbol means "wrap this function with verification"
@agent.track_action(risk_level="low")
def send_email(to, subject, body):
    # AIM does this AUTOMATICALLY:
    # 1. Verify: "Is this agent allowed to send email?"
    # 2. Log: "Agent 'my-agent' wants to send email to {to}"
    # 3. Check: "Any suspicious patterns?"
    # 4. Execute: Only if all checks pass
    # 5. Record: "Email sent successfully at {timestamp}"
    email_service.send(to, subject, body)
```

### Why This Prevents Rogue Agents

**Scenario: Agent Gets Compromised**

```python
# WITHOUT decorator - Attacker can do anything:
def delete_database():
    db.drop_all_tables()  # ❌ No verification, no audit trail, no alerts

# WITH decorator - Attacker is caught:
@agent.track_action(risk_level="critical")
def delete_database():
    # AIM catches this:
    # 🚨 Alert: "Critical action attempted"
    # 🚨 Alert: "No recent history of this action"
    # 🚨 Alert: "Trust score: 0.95 → 0.12 (SUSPICIOUS!)"
    # 🚨 Action BLOCKED automatically
    # 📧 Admin notified immediately
    db.drop_all_tables()  # ← Never executes
```

### Risk Levels Explained

```python
# LOW RISK - Read operations, safe actions
@agent.track_action(risk_level="low")
def get_weather(city):
    return weather_api.get(city)

# MEDIUM RISK - Writes, data modification
@agent.track_action(risk_level="medium")
def update_user_profile(user_id, data):
    return db.update(user_id, data)

# HIGH RISK - Sensitive operations
@agent.track_action(risk_level="high")
def transfer_money(from_account, to_account, amount):
    return bank_api.transfer(from_account, to_account, amount)

# CRITICAL RISK - Destructive operations (requires human approval)
@agent.require_approval(risk_level="critical")
def delete_all_users():
    # Execution PAUSES here
    # Admin gets notification: "Agent wants to delete all users - Approve?"
    # Agent waits for human decision
    # Only proceeds if approved
    return db.delete_all("users")
```

### The Golden Rule

**If your agent calls an API, database, or external service → Use a decorator!**

```python
# ✅ GOOD - All actions verified
@agent.track_action(risk_level="low")
def search_products(query):
    return api.search(query)

@agent.track_action(risk_level="medium")
def add_to_cart(product_id):
    return cart.add(product_id)

@agent.require_approval(risk_level="high")
def place_order(cart_id, payment_method):
    return orders.create(cart_id, payment_method)

# ❌ BAD - No verification = Agent can run wild
def charge_credit_card(amount):
    return stripe.charge(amount)  # Disaster waiting to happen!
```

---

## 💡 Pro Tips

### Tip 1: Zero Configuration is the Default

**Downloaded SDK = Ready to Go**:
```python
# ✅ RECOMMENDED - Zero config (OAuth credentials embedded in SDK)
agent = secure("my-agent")

# 🔧 ADVANCED - Manual mode with API key (if needed)
agent = secure("my-agent", api_key="aim_abc123")
```

### Tip 2: Declare Capabilities with Decorators

**Capabilities are detected from decorators and config files**:
```python
agent = secure("my-agent")

# ✅ Declare capabilities using decorators (recommended)
@agent.perform_action(capability="db:read")
def read_database():
    pass

# ✅ Or use a config file: ~/.aim/capabilities.json
# { "capabilities": ["db:read", "db:write"] }

# ✅ MCP servers are auto-detected from ~/.claude/claude_desktop_config.json
```

### Tip 3: Use Decorators for Actions

**Explicit verification for critical actions**:
```python
from aim_sdk import secure

agent = secure("database-agent")

@agent.perform_action("delete_user", risk_level="high")
def delete_user(user_id: int):
    """Delete user from database - requires approval"""
    # This action requires admin approval before execution
    database.delete(user_id)
```

### Tip 4: Monitor Your Dashboard Daily

- Check trust scores
- Review security alerts
- Audit recent actions
- Export compliance reports

---

## 🆘 Troubleshooting

### Issue: "Connection refused" to AIM backend

**Solution**:
```bash
# Check if backend is running
docker ps | grep aim-backend

# If not running, restart:
docker compose restart aim-backend

# Check logs:
docker compose logs aim-backend
```

### Issue: "Invalid private key"

**Solution**:
```bash
# Verify your private key is correct
echo $AIM_PRIVATE_KEY

# Re-generate key from dashboard:
# 1. Go to Agents → your-agent → Settings
# 2. Click "Regenerate Private Key"
# 3. Copy new key and update environment variable
```

### Issue: "Agent not found"

**Solution**:
```python
# Verify agent name matches dashboard exactly
agent = secure("weather-agent")  # ✅ Correct (lowercase, hyphen)
agent = secure("Weather Agent")  # ❌ Wrong (spaces, capitals)
```

### Issue: Trust score is low

**Reason**: New agents start with lower trust scores

**Solution**:
- Wait for agent to build history (trust improves over time)
- Ensure all actions succeed (failures lower trust)
- Avoid security alerts (fix any detected issues)

---

## 📞 Need Help?

- 💬 **Discord**: https://discord.gg/opena2a
- 📧 **Email**: info@opena2a.org
- 🐛 **GitHub Issues**: https://github.com/opena2a/agent-identity-management/issues
- 📚 **Documentation**: https://opena2a.org

---

## ✅ Quick Start Checklist

- [ ] AIM platform running (local or Azure)
- [ ] Admin login works
- [ ] Agent registered in dashboard
- [ ] Private key saved securely
- [ ] SDK downloaded from dashboard
- [ ] Sample agent running (`weather_agent.py`)
- [ ] Dashboard shows agent status
- [ ] Trust score visible
- [ ] Audit trail capturing actions
- [ ] No security alerts

**All checked?** 🎉 **You're ready to build secure AI agents!**

---

<div align="center">

**Next**: [Weather Agent Example →](./examples/weather-agent.md)

[🏠 Back to Home](../README.md) • [📚 All Guides](./index.md) • [💬 Get Help](https://discord.gg/opena2a)

</div>
