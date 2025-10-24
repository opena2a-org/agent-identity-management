# ✈️ Flight Search Agent - AIM Integration Demo

A real-world AI agent that demonstrates complete integration with the AIM (Agent Identity Management) platform.

## 🎯 What This Demonstrates

This flight search agent showcases:
- ✅ **Auto-registration** - One line of code: `secure("agent-name")`
- ✅ **Auto-detection** - Automatically detects 5 capabilities from code
- ✅ **Cryptographic signing** - Ed25519 signatures for authentication
- ✅ **Action verification** - Requests approval before executing searches
- ✅ **Activity logging** - Logs all actions to AIM for audit trail
- ✅ **Trust scoring** - Builds trust score through verified actions
- ✅ **Dashboard integration** - Visible in AIM web dashboard

## 🚀 Quick Start

### Prerequisites

- AIM platform running (see root README.md)
- Python 3.11+
- SDK downloaded from AIM dashboard (Settings → SDK Download)

### Option 1: Automated QA Test (Recommended)

```bash
# This script guides you through the complete flow
./quick_qa_test.sh
```

This will:
1. Open browser for OAuth login
2. Guide you to download fresh SDK
3. Install credentials automatically
4. Run verification tests
5. Open dashboard to verify results

### Option 2: Manual Testing

```bash
# 1. Download SDK from AIM dashboard (Settings → SDK Download)
# 2. Extract SDK to this directory or add to PYTHONPATH

# Install dependencies
pip install -r requirements.txt

# Run interactive mode
python3 flight_agent.py

# Or run demo (one-shot search)
python3 demo_search.py

# Or run automated tests
python3 test_flight_agent.py
```

## Configuration

The agent uses OAuth credentials from the SDK download. Make sure you have:
- `.aim/credentials.json` in the same directory as the agent

## Usage

### Interactive Mode

```bash
python flight_agent.py
```

Available commands:
- `search <destination>` - Search flights to a destination (NYC, SFO, MIA)
- `status` - Show agent status and AIM connection
- `help` - Show available commands
- `quit` - Exit the agent

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
1. Calls `client.verify_action()` to request verification from AIM
2. AIM checks agent trust score and action risk level
3. If approved, executes flight search
4. Logs result with `client.log_action_result()`

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

✅ **Action Verification**
```python
verification = client.verify_action(
    action_type="search_flights",
    action_details={
        "destination": destination,
        "risk_level": "low"
    }
)
```

✅ **Activity Logging**
```python
client.log_action_result(
    action_type="search_flights",
    success=True,
    metadata={"flights_found": 4}
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
# Option 1: Use automated script
./quick_qa_test.sh

# Option 2: Manual process
# 1. Log in to portal
open http://localhost:3000/auth/login

# 2. Download fresh SDK
open http://localhost:3000/dashboard/sdk

# 3. Copy credentials
cp -r ./fresh-sdk/aim-sdk-python/.aim ~/.aim
```

**Why does this happen?**

AIM uses **token rotation** for enterprise security:
- When you use a refresh token → backend issues NEW token
- OLD token is immediately revoked → prevents reuse attacks
- This is SOC 2 / HIPAA compliant behavior

See `NEXT_STEPS.md` for detailed explanation.

### Empty Dashboard Tabs

If tabs like "Recent Activity" or "Trust History" are empty:

**This is expected** if:
1. Agent hasn't performed any verified actions yet, OR
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
1. Run `./quick_qa_test.sh` to complete QA
2. Verify all dashboard tabs populate
3. Test with different destinations
4. Review security logs in dashboard

### For Production
- Review `PRODUCTION_READINESS_REPORT.md`
- Complete documentation improvements
- Set up monitoring
- Deploy to enterprise environment

## 🚀 TL;DR - Get Started in 60 Seconds

```bash
# 1. Run automated QA test
./quick_qa_test.sh

# 2. Follow browser prompts to log in and download SDK

# 3. Watch verification tests pass

# 4. Open dashboard to see your agent in action
open http://localhost:3000/dashboard
```

**That's it!** The agent is registered, capabilities detected, and dashboard populated.

---

**Status**: Example Agent ✓
**Last Updated**: January 2025
