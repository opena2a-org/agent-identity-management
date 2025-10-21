# Verification Events Testing Guide

**Date**: October 19, 2025
**Purpose**: Complete guide for testing protocol detection and verification events

---

## Overview

AIM supports **6 verification protocols** with automatic detection and tracking:

1. **MCP** (Model Context Protocol)
2. **A2A** (Agent-to-Agent)
3. **ACP** (Agent Communication Protocol)
4. **DID** (Decentralized Identity)
5. **OAuth** (OAuth 2.0 / OIDC)
6. **SAML** (Security Assertion Markup Language)

This guide provides two approaches to test verification events:

- **Approach 1**: Direct API calls (faster, no SDK required)
- **Approach 2**: Python SDK integration (production-like testing)

---

## Prerequisites

### Backend Requirements
```bash
# Ensure backend is running
docker compose up -d postgres backend

# Check backend health
curl http://localhost:8080/health
```

### Frontend Requirements (Optional)
```bash
# Start frontend to view monitoring dashboard
docker compose up -d frontend

# Access dashboard at:
# http://localhost:3000/dashboard/monitoring
```

### Python Requirements
```bash
# Install required packages
pip install requests
```

---

## Approach 1: Direct API Testing (Recommended for Quick Testing)

### Test Script: `test_verification_events.py`

**What it does**:
- ✅ Logs in as admin user
- ✅ Creates test agent
- ✅ Generates verification events for all 6 protocols
- ✅ Tests multiple verification types (identity, capability, permission, trust)
- ✅ Includes success and failure scenarios
- ✅ Displays real-time statistics
- ✅ Shows protocol distribution

### Usage:

```bash
# Run the test script
python test_verification_events.py

# Expected output:
# 🚀 AIM VERIFICATION EVENTS TEST SUITE
# 🔐 Logging in as admin@opena2a.org...
# ✅ Login successful! Org ID: xxx
# 🤖 Creating test agent: protocol-test-agent...
# ✅ Agent created: xxx
#
# 🧪 RUNNING PROTOCOL VERIFICATION TESTS
# ============================================================
# 🧪 Testing MCP Protocol
# ✅ MCP verification event created successfully
# ...
# ✅ TEST SUITE COMPLETE
# ✅ Created 24 verification events
# ✅ Tested all 6 protocols: MCP, A2A, ACP, DID, OAuth, SAML
# ✅ Success rate: 83.33%
```

### What Gets Created:

**MCP Protocol** (3 events):
- Identity verification (MCP server auth)
- Capability verification (list tools)
- Permission verification (execute tool)

**A2A Protocol** (3 events):
- Identity verification (agent handshake)
- Trust verification (verify signature)
- Permission verification (delegate task)

**ACP Protocol** (3 events):
- Identity verification (ACP connect)
- Capability verification (capability check)
- Permission verification (message send)

**DID Protocol** (3 events):
- Identity verification (DID resolution)
- Trust verification (DID signature)
- Capability verification (DID capability)

**OAuth Protocol** (3 events):
- Identity verification (token verify)
- Permission verification (scope check)
- Identity verification (OIDC ID token)

**SAML Protocol** (3 events):
- Identity verification (assertion verify)
- Permission verification (attribute check)
- Identity verification (SSO session)

**Mixed Failures** (4 events):
- MCP auth failure
- A2A signature invalid
- OAuth timeout
- DID not found

**Total**: 24 verification events across 6 protocols

---

## Approach 2: Python SDK Testing (Production-Like)

### Test Script: `test_sdk_verification_events.py`

**What it does**:
- ✅ Downloads Python SDK from backend
- ✅ Installs SDK with dependencies
- ✅ Initializes AIM SDK client
- ✅ Tests MCP protocol verification (via SDK)
- ✅ Tests capability verification (A2A protocol)
- ✅ Tests action verification (decorator pattern)
- ✅ Tests SDK integration reporting
- ✅ Verifies events were created

### Usage:

```bash
# Run the SDK test script
python test_sdk_verification_events.py

# The script will:
# 1. Login and create test agent
# 2. Download Python SDK as ZIP
# 3. Extract and install SDK
# 4. Run verification tests using actual SDK
# 5. Display statistics
# 6. Ask if you want to clean up

# Expected output:
# 🚀 AIM PYTHON SDK VERIFICATION EVENTS TEST
# 🔐 Logging in as admin@opena2a.org...
# ✅ Login successful!
# 🤖 Creating test agent: sdk-verification-tester...
# ✅ Agent created
# 📦 Downloading Python SDK...
# ✅ SDK downloaded (45,231 bytes)
# 📥 Installing SDK dependencies...
# ✅ SDK installed successfully
# 🔧 Initializing SDK client...
# ✅ SDK client initialized
#
# 🧪 RUNNING SDK VERIFICATION TESTS
# ✅ MCP verification triggered via SDK
# ✅ A2A capability verification triggered
# ✅ Action verification triggered via decorator
# ✅ SDK integration reporting triggered
#
# ✅ TEST SUITE COMPLETE
# Tests Passed: 4/4
# Verification Events Created: 4
```

### SDK Features Tested:

1. **MCP Registration** (`register_mcp`)
   - Protocol: MCP
   - Verification Type: Identity
   - Tests: Auto-detection and registration

2. **Capability Reporting** (`report_capabilities`)
   - Protocol: A2A
   - Verification Type: Capability
   - Tests: Agent capability discovery

3. **Action Verification** (`@perform_action` decorator)
   - Protocol: A2A
   - Verification Type: Permission
   - Tests: Decorator-based verification

4. **SDK Integration** (`report_sdk_integration`)
   - Protocol: A2A
   - Verification Type: Identity
   - Tests: SDK status reporting

---

## Viewing Results

### 1. Monitoring Dashboard (Recommended)

Visit the monitoring dashboard to see real-time verification analytics:

```
http://localhost:3000/dashboard/monitoring
```

**What you'll see**:
- ✅ **Total Verifications**: Count of all verification events
- ✅ **Success Rate**: Percentage of successful verifications
- ✅ **Average Latency**: Mean verification duration
- ✅ **Active Agents**: Number of unique agents verified
- ✅ **Protocol Distribution**: Bar chart showing protocol usage
- ✅ **Verification Type Distribution**: Bar chart by verification type
- ✅ **Status Breakdown**: Success vs. failed vs. timeout
- ✅ **Recent Events Feed**: Live event stream

### 2. API Endpoints

You can also query verification data via API:

```bash
# Get verification statistics
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/verification-events/statistics?period=24h

# Get recent verification events
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/verification-events/recent?minutes=15
```

### 3. Database Query

Directly query PostgreSQL to see raw data:

```bash
# Connect to database
docker compose exec -T postgres psql -U postgres -d identity

# Count total verifications
SELECT COUNT(*) FROM verification_events;

# Protocol distribution
SELECT protocol, COUNT(*) FROM verification_events GROUP BY protocol;

# Recent events
SELECT
  protocol,
  verification_type,
  status,
  duration_ms,
  created_at
FROM verification_events
ORDER BY created_at DESC
LIMIT 10;
```

---

## Protocol Detection Details

For complete protocol detection strategy, see: **PROTOCOL_DETECTION_STRATEGY.md**

### Current Detection Status

| Protocol | Status | Detection Method |
|----------|--------|------------------|
| MCP | ✅ Implemented | Claude config auto-detection |
| A2A | ✅ Implemented | Agent-to-agent authentication |
| ACP | ⏳ Partial | Message format detection (manual) |
| DID | ⏳ Partial | DID prefix detection (manual) |
| OAuth | ✅ Implemented | Bearer token JWT validation |
| SAML | ⏳ Partial | SAML assertion detection (manual) |

### Future Enhancements (v1.1)

- [ ] **Auto-detection middleware**: Automatic protocol detection from headers
- [ ] **ACP message parser**: Full ACP 1.0 specification support
- [ ] **DID resolver**: W3C DID document resolution
- [ ] **SAML parser**: Full SAML 2.0 assertion parsing
- [ ] **Protocol analytics**: Advanced protocol usage insights

---

## Troubleshooting

### Issue: "Backend not responding"

```bash
# Check backend status
docker compose ps backend

# View backend logs
docker compose logs -f backend

# Restart backend
docker compose restart backend
```

### Issue: "No verification events created"

**Possible causes**:
1. Database not running → `docker compose up -d postgres`
2. Backend not connected to DB → Check `DATABASE_URL` in backend logs
3. Agent not found → Verify agent was created successfully
4. Auth token expired → Re-login and get fresh token

### Issue: "SDK import error"

```bash
# Ensure SDK is installed
cd aim-sdk-test
pip install -e .

# Check if modules exist
python -c "import aim_sdk; print(aim_sdk.__file__)"
```

### Issue: "Protocol not detected"

Currently, protocols are **manually specified** when creating verification events.
Auto-detection is planned for v1.1. For now, explicitly set the protocol:

```python
# Manual protocol specification
client.create_verification_event(
    agent_id=agent_id,
    protocol="MCP",  # Explicitly set protocol
    verification_type="identity"
)
```

---

## Cleanup

### Delete Test Agents

```bash
# Via API
curl -X DELETE -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/agents/{agent_id}

# Or use the cleanup option in test scripts
```

### Clear Verification Events

```bash
# WARNING: This deletes ALL verification events!
docker compose exec -T postgres psql -U postgres -d identity \
  -c "DELETE FROM verification_events WHERE metadata->>'testEvent' = 'true';"
```

### Remove SDK Directory

```bash
# If you used SDK test script
rm -rf aim-sdk-test
```

---

## Next Steps

### 1. **Integration Testing**
   - Test verification events in production-like scenarios
   - Integrate with CI/CD pipeline
   - Add performance benchmarks

### 2. **Protocol Auto-Detection**
   - Implement detection middleware
   - Add protocol inference from request context
   - Support mixed-protocol scenarios

### 3. **Advanced Analytics**
   - Protocol success rates over time
   - Latency percentiles by protocol
   - Anomaly detection for unusual protocol patterns

### 4. **Documentation**
   - Add protocol detection examples
   - Document best practices
   - Create video tutorials

---

## References

- **Protocol Strategy**: `PROTOCOL_DETECTION_STRATEGY.md`
- **API Documentation**: `/api/v1/docs` (Swagger)
- **SDK Documentation**: Download SDK and see `README.md`
- **Monitoring Dashboard**: `http://localhost:3000/dashboard/monitoring`

---

**Last Updated**: October 19, 2025
**Maintained by**: OpenA2A Team
