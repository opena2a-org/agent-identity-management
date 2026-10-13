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
git clone https://github.com/opena2a-org/agent-identity-management.git
cd agent-identity-management

# Generate local secrets into .env (compose does not start without them)
./scripts/gen-dev-secrets.sh > .env

# Start with Docker Compose
docker compose up -d

# Wait ~60 seconds for services to start, then create the admin account
docker compose run --rm bootstrap
```

**Access Points**:
- 🌐 Dashboard: http://localhost:3000
- 🔌 Backend API: http://localhost:8080
- 📊 Grafana: http://localhost:3003

**Default Admin Login**:
- Email: `admin@opena2a.org`
- Password: the `DEFAULT_ADMIN_PASSWORD` value in `.env`. The bootstrap step does not print it. If `.env` sets no `DEFAULT_ADMIN_PASSWORD`, the bootstrap step generates a password and prints it once. (⚠️ Pre-B2 stacks used `AIM2025!Secure` — rotate immediately if you inherited it; first login forces a change either way.)

### Option B: Azure Production (One Command) ☁️

```bash
# Clone the repository
git clone https://github.com/opena2a-org/agent-identity-management.git
cd agent-identity-management

# Deploy to Azure (creates all infrastructure)
./infrastructure/scripts/deploy-azure-production.sh

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

## Step 3: Install the AIM SDK (15 seconds)

The SDK is published on PyPI as `aim-sdk`. Install it and sign in to your AIM instance:

```bash
pip install aim-sdk
aim-sdk login --url http://localhost:8080
```

`aim-sdk login` prints a code; approve it on your dashboard's `/device` page. The SDK saves its credentials to `~/.aim/`.

**Alternative: download a pre-configured SDK.** The dashboard can also give you the SDK as a ZIP with your personal credentials embedded, so you skip `aim-sdk login`:

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

# LINE 2: Add @agent.perform_action to verify every call
@agent.perform_action(risk_level="low")
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
Trust Score: 0.835 (Good)
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
➖ Compliance Score:    No snapshot (excluded)  [10%]
✅ Age & History:           New  (0.30)  [ 5%]
✅ Drift Detection:         None (1.00)  [ 3%]
➖ User Feedback:          None  (excluded)  [ 2%]
⚠️  Execution Isolation:  Not attested (0.30)  [10%]

Overall Trust Score: 0.835 / 1.00
```

The breakdown is for an agent verified today whose verifications so far all succeeded, with no
compliance snapshot for its organization, no user feedback and no isolation attestation. Compliance
and user feedback have no data, so they are left out and their weight is spread over the other seven
factors (0.881); the published score is capped at the sum with each of the two counted at 0.50
(0.835), so that missing data never raises a score. A new agent that is still pending starts at 0.5925.
See the [Trust Scoring Guide](./sdk/trust-scoring.md) for each factor.

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
2. ✅ **Line 2** (`@agent.perform_action`) - **Verification BEFORE execution** (prevents malicious actions!)
3. ✅ **Line 3** (your code) - Runs ONLY if verification passes

**Every time `get_weather()` is called**, AIM automatically:

- 🛡️ **Verifies** the action BEFORE it executes (prevents unauthorized API calls!)
- 📝 **Logs** to the audit trail (who, what, when, why)
- 📊 **Updates** real-time trust score (9-factor algorithm)
- 🚨 **Monitors** for anomalies (unusual patterns trigger alerts)
- 🔐 **Signs** the verification request with the agent's Ed25519 key

**Without `@agent.perform_action`?** Your agent can do ANYTHING without oversight. ❌
**With `@agent.perform_action`?** Every action verified, logged, monitored. ✅

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

- [Cloud Deployment](../infrastructure/deployment/README.md) - Azure, Google Cloud and AWS
- [Kubernetes](./DEPLOYMENT.md#-kubernetes-deployment) - Steps for manifests you write (the repository ships none)
- [Security Best Practices](./guides/SECURITY.md) - Harden your deployment

---

## 🎓 Understanding Decorators (The Secret Sauce)

###  What's a Decorator?

**Think of it like airport security**: Your function is a passenger, and `@agent.perform_action` is the security checkpoint.

```
┌─────────────────────────────────────────────────────────────────────┐
│                   WITHOUT @agent.perform_action                     │
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
│                    WITH @agent.perform_action                       │
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
@agent.perform_action(risk_level="low")
def get_weather(city):
    return call_api(city)  # ✅ Verified passenger only
```

### How Does It Work?

```python
from aim_sdk import secure

agent = secure("my-agent")

# The @ symbol means "wrap this function with verification"
@agent.perform_action(risk_level="low")
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
@agent.perform_action(risk_level="critical")
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
@agent.perform_action(risk_level="low")
def get_weather(city):
    return weather_api.get(city)

# MEDIUM RISK - Writes, data modification
@agent.perform_action(risk_level="medium")
def update_user_profile(user_id, data):
    return db.update(user_id, data)

# HIGH RISK - Sensitive operations
@agent.perform_action(risk_level="high")
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
@agent.perform_action(risk_level="low")
def search_products(query):
    return api.search(query)

@agent.perform_action(risk_level="medium")
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

# 🔧 ADVANCED - Manual mode with an agent API key (issued for an existing agent
# in your organization; aim_url is required)
agent = secure("my-agent", aim_url="http://localhost:8080", api_key="aim_live_...")
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
docker compose restart backend

# Check logs:
docker compose logs backend
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
- 🐛 **GitHub Issues**: https://github.com/opena2a-org/agent-identity-management/issues
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

## See it work

The two calls in the root README's Quick start, one allowed `db:read` and one refused `db:write`, are the whole model: every decorated call is checked against the agent's grant and logged; in strict mode a capability the agent was not granted is refused before the function body executes, with the reason. Nothing in your code changes between the two, only the grant.

Verify the agent before its first call: on a strict-mode organization, calls refused while pending are recorded against the agent's trust score, and a denied call after that can suspend it (measured 2026-09-23; an agent verified first is not affected). On a new organization, which starts in monitoring mode, a pending agent's call runs and AIM raises an `Unverified Agent Action (Monitoring Mode)` alert for it; on a strict-mode organization every call is refused with `Agent not verified - all actions denied` until the agent is verified.

Check the mode before the refused call: the Global Enforcement Mode badge under Security, then Policies, reads STRICT, and `GET /api/v1/admin/enforcement-settings` with an administrator's token returns `"enforcementMode": "strict"`.

The agent's audit log in the dashboard records each decision: the denied `db:write` appears with `denialReason` set to the same text the SDK printed, the capability, the risk level and the time (that row was read back through the API on the measured run of 2026-09-22). Open `http://localhost:3000` (self-hosted) or the AIM Cloud dashboard, then Agents, then your agent, then its activity. To permit `db:write`, grant it to the agent there.

Want to see the same boundary stop a real attack? The [Demos](DOCUMENTATION_INDEX.md#demos) end with an intentionally vulnerable agent run with and without AIM.

### Notes on the root README's Quick start

The first request after sign-in presents the refresh token `aim-sdk login` saved. A server with Redis records it as revoked, answers with a new one, and the SDK prints a line: `Token rotated successfully — old refresh token revoked (new id: 461d130d...)`. Without Redis the server logs `Token revocation disabled (no Redis configured)` and the SDK prints no such line.

`aim-sdk login` uses the OAuth 2.0 device grant (RFC 8628): the CLI prints a code and you approve it on your dashboard's `/device` page. `secure()` generates an Ed25519 keypair, registers the agent with the AIM backend, and stores credentials at `~/.aim/`.

The SDK built from this repository printed `[OK]` above where the release on PyPI prints `✓`. Without the `rich` extra (`pip install aim-sdk`), the same fields print in a panel.

The decorator files a capability request for `db:write`, the server refuses the call, and the agent's audit log records the denial with the same reason. The release on PyPI also prints `✓ Registered db:write (medium)` after the first line; the SDK built from this repository does not.

A stack started from the quickstart defaults has no security policy configured, so there the reason names AIM's built-in `default_policy` where the output above reads 'Capability Violation Detection'. An `ActionDeniedError` is a `PermissionError`, so an existing `except PermissionError` handler catches it.

New to AIM? The [SDK quickstart tutorial](https://opena2a.org/docs/tutorials/sdk-quickstart) walks through this end to end. The same one-line shape works in [Java](../sdk/java/README.md). TypeScript has no `secure()` call: `@opena2a/aim-core` checks a capability against a local policy with no server, and the server client `@opena2a/aim-sdk` registers an agent and verifies each action with separate calls ([TypeScript](../sdk/typescript/README.md#local-mode-with-opena2aaim-core)).

---

<div align="center">

**Next**: [Weather Agent Example →](./examples/weather-agent.md)

[🏠 Back to Home](../README.md) • [📚 All Guides](./DOCUMENTATION_INDEX.md) • [💬 Get Help](https://discord.gg/opena2a)

</div>
