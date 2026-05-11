# ✈️ Flight Search Agent - AIM Integration Demo

A real-world AI agent that demonstrates complete integration with the AIM (Agent Identity Management) platform.

## 🎯 What This Demonstrates

This flight search agent showcases:
- ✅ **Auto-registration** - One line of code: `secure("agent-name")`
- ✅ **Auto-detection** - Automatically detects 5 capabilities from code
- ✅ **Cryptographic signing** - Ed25519 signatures for authentication
- ✅ **Capability verification** - Requests approval before executing searches
- ✅ **Prompt-injection denial** - The `inject` subcommand drives three deterministic injection scenarios (data exfiltration, privilege escalation, sandbox escape) through AIM. Each one asks for a capability the agent never declared; AIM denies at FGA Step 1 (capability_check). Used as the stage demo for the May 2026 LF Open Source Summit + Observability Summit talks.
- ✅ **Activity logging** - Logs all capabilities to AIM for audit trail
- ✅ **Trust scoring** - Builds trust score through verified capabilities
- ✅ **Dashboard integration** - Visible in AIM web dashboard

## 🚀 Quick Start

### Prerequisites

- AIM platform running (see root README.md)
- Python 3.11+
- SDK downloaded from AIM dashboard (Settings → SDK Download)

### Option 1: Repo checkout (recommended for contributors)

If you have the `agent-identity-management` repo cloned, the SDK at `../../sdk/python` is already on `sys.path` (see `flight_agent.py:25-27`). Run:

```bash
# From the agent-identity-management repo root, start the AIM backend if it
# isn't already running. The agent talks to localhost:8080 by default.
docker compose up -d aim-postgres aim-redis aim-backend

# From this directory:
pip install -r requirements.txt
python3 flight_agent.py
```

On first run the agent will register with AIM, generate an Ed25519 keypair, and drop a credentials file under `~/.aim/`. The same OAuth/SDK-token flow used by external SDK consumers — exercising every path the dashboard-bundle flow exercises.

### Option 2: Dashboard-bundle SDK (for AIM Cloud users)

```bash
# 1. Download the Python SDK bundle from your AIM dashboard:
#    Settings → SDK Downloads → Python
# 2. Extract the bundle here so the agent sees ./aim-sdk-python/:
#    mv ~/Downloads/aim-sdk-python ./aim-sdk-python

# Install dependencies
pip install -r requirements.txt

# Run interactive mode
python3 flight_agent.py

# Or run demo (one-shot search)
python3 demo_search.py

# Or run automated tests
python3 test_flight_agent.py
```

The auto-prepended `sys.path` picks up either layout — no env vars required.

## Configuration

The agent uses OAuth credentials from the SDK download. Make sure you have:
- `.aim/credentials.json` in the same directory as the agent

## Usage

### Interactive Mode

```bash
python flight_agent.py
```

Available commands:
- `search <destination>` - Search flights to a destination (NYC, SFO, MIA). Routes through `verify_capability("flights:search", ...)`, which AIM approves because the agent has the capability declared.
- `inject <scenario>` - Run a deterministic prompt-injection demo. The agent attempts a capability it never declared; AIM denies at FGA Step 1 (capability_check). Scenarios:
  - `data-exfil` — exfiltration via `email:send`
  - `priv-esc` — privilege escalation via `admin:create_user`
  - `sandbox-escape` — sandbox escape via `os:exec`
- `status` - Show agent status and AIM connection
- `help` - Show available commands
- `quit` - Exit the agent

The `inject` command is what the Linux Foundation Open Source Summit + Observability Summit stage demos drive on Beat 4 of `presentations/linux-foundation/demo-script.md`. Each scenario is repeatable and produces the same DENY output every time, which is exactly what you want under stage lights.

### Stage prep (run once before the talk)

The flight agent uses an OAuth-backed SDK token that expires after 90 days. If your token is stale, the agent falls into standalone mode and the `inject` command can no longer demonstrate the live deny path. To refresh:

```bash
# 1. Open the AIM dashboard (community: https://aim.opena2a.org)
# 2. Settings → SDK Downloads → download a fresh Python SDK
# 3. Extract over the existing credentials directory:
mv ~/Downloads/aim-sdk-python ./aim-sdk-python
# 4. The agent's auto-prepended sys.path picks this up — no env vars needed.

# Smoke-test that the deny path is live:
python3 flight_agent.py
flightagent> inject data-exfil
# Expected: "🛡️  AIM DENIED the capability request." block.
```

For backend-side verification without the SDK (faster, doesn't need a token), run the hermetic OTel smoke from `apps/backend/deployments/otel-demo/smoke-backend.sh`. It proves the same fga.authorize → DENY path that `inject` exercises through the SDK.

### Example Session

```
flightagent> search NYC

🔍 Searching flights to NYC...
🔐 Requesting verification from AIM...
✅ Verification status: approved
   Found 4 flights to NYC

✈️  Available Flights (sorted by price):
================================================================================

1. JetBlue - B6 3456
   Route: LAX → JFK
   Time: 14:00 - 22:30 (5h 30m)
   Stops: Direct
   💰 Price: $179.00

2. Delta Airlines - DL 9012
   Route: LAX → LGA
   Time: 12:30 - 21:15 (5h 45m)
   Stops: 1 stop(s)
   💰 Price: $199.99

...
```

## How It Works

### 1. Registration (First Run)

On first run, the agent:
- Calls `secure("flight-search-agent")` to register with AIM
- Auto-detects capabilities from code (e.g., `search_flights`, `api_calls`)
- Auto-detects MCPs from Claude Desktop configuration
- Generates Ed25519 keypair for signing
- Receives agent ID from AIM

### 2. Flight Search

For each search:
1. Calls `client.verify_capability()` to request verification from AIM
2. AIM checks agent trust score and capability risk level
3. If approved, executes flight search
4. Logs result with `client.log_capability_result()`

### 3. Dashboard Visibility

After running the agent, you can see:
- Agent registration in the Agents page
- Verification requests in the Verifications page
- Activity logs in the Analytics dashboard
- Trust score changes based on behavior

## AIM Integration Points

This agent demonstrates:

✅ **Agent Registration**
```python
client = secure(
    "flight-search-agent",
    agent_type="ai_agent",
    auto_detect_capabilities=True,
    auto_detect_mcps=True
)
```

✅ **Capability Verification**
```python
verification = client.verify_capability(
    capability="flights:search",
    resource=destination,
    context={"risk_level": "low"}
)
```

✅ **Activity Logging**
```python
client.log_capability_result(
    audit_id=verification.audit_id,
    success=True,
    result_summary="Found 4 flights"
)
```

## Testing the Integration

1. **Start the agent**:
   ```bash
   python flight_agent.py
   ```

2. **Search for flights**:
   ```
   flightagent> search NYC
   ```

3. **Check the AIM dashboard**:
   - Open http://localhost:3000/dashboard
   - View the agent in the Agents page
   - See verification requests in real-time
   - Check activity in Analytics

## 🐛 Troubleshooting

### "Authentication failed" Error

**This is expected behavior** if your SDK credentials have expired due to token rotation.

**Solution**: Get fresh credentials

```bash
# 1. Log in to portal
open http://localhost:3000/auth/login

# 2. Download fresh SDK from the dashboard
open http://localhost:3000/dashboard/sdk

# 3. Copy credentials
cp -r ./fresh-sdk/aim-sdk-python/.aim ~/.aim
```

**Why does this happen?**

AIM uses **token rotation** for security:
- When you use a refresh token → backend issues NEW token
- OLD token is immediately revoked → prevents reuse attacks
- This is SOC 2 / HIPAA compliant behavior

See `NEXT_STEPS.md` for detailed explanation.

### Empty Dashboard Tabs

If tabs like "Recent Activity" or "Trust History" are empty:

**This is expected** if:
1. Agent hasn't performed any verified capabilities yet, OR
2. Your credentials have expired (token rotation)

**Solution**:
1. Get fresh credentials (see above)
2. Run the agent to perform searches
3. Tabs will populate with verification events

### Verification Tests Failing

```bash
# Run diagnostic verification
python3 verify_qa_complete.py

# This will check:
# - Credentials are valid
# - Agent can authenticate
# - Verification flow works
# - Activity logging works
# - Dashboard data populates
```

## 📚 Documentation

### Core Documents
- **NEXT_STEPS.md** - Complete guide for fresh OAuth login
- **QA_COMPLETE_SUMMARY.md** - Comprehensive QA results and findings
- **PRODUCTION_READINESS_REPORT.md** - Production deployment assessment
- **SECURITY_REVIEW.md** - Security architecture analysis

### Quick Links
- **Agent Detail**: http://localhost:3000/dashboard/agents/[your-agent-id]
- **Dashboard**: http://localhost:3000/dashboard
- **Portal Login**: http://localhost:3000/auth/login
- **SDK Download**: http://localhost:3000/dashboard/sdk

## 🎉 Success Metrics

After running the agent, you should see:

- ✅ Agent registered with AIM
- ✅ 5 capabilities auto-detected
- ✅ Trust score: 51%
- ✅ Status: Verified
- ✅ Flight search results displayed
- ✅ Dashboard populated with data

## 💡 Next Steps

### For Development
- Add more search parameters (dates, passenger count, etc.)
- Integrate with real flight APIs (Amadeus, Skyscanner)
- Add booking functionality
- Implement multi-city searches

### For Testing
1. Run `python3 test_flight_agent.py` to exercise registration + benign search + verification logging end-to-end.
2. Run `python3 flight_agent.py`, then in the REPL: `inject data-exfil`, `inject priv-esc`, `inject sandbox-escape` — each should produce a clean AIM DENY block (FGA capability_check denial).
3. Verify all dashboard tabs populate (`http://localhost:3000/dashboard/agents/<agent-id>`).
4. Review security logs and audit events in dashboard.

### For Production
- Review `PRODUCTION_READINESS_REPORT.md`
- Complete documentation improvements
- Set up monitoring
- Deploy to your environment

## 🚀 TL;DR - Get Started in 60 Seconds

```bash
# 1. Bring AIM up locally (one-time)
docker compose -f ../../docker-compose.yml up -d aim-postgres aim-redis aim-backend aim-frontend

# 2. Follow browser prompts at http://localhost:3000 to log in and download the Python SDK

# 3. Watch verification tests pass

# 4. Open dashboard to see your agent in action
open http://localhost:3000/dashboard
```

**That's it!** The agent is registered, capabilities detected, and dashboard populated.

---

**Status**: Example Agent ✓
**Last Updated**: January 2025
