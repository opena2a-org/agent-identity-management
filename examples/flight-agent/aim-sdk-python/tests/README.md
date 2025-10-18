# AIM Python SDK - Test Suite

This directory contains integration tests for the AIM Python SDK.

## Test Files

### Core Functionality
- **test_client.py** - Core client functionality tests (existing)
- **test_new_agent.py** - Basic agent registration test

### Phase 2: Auto-Verification
- **test_phase2_flow.py** ✅ - Complete auto-registration with challenge-response verification
  - Tests one-line registration
  - Verifies automatic cryptographic verification
  - Validates auto-approval logic
  - **Status**: Production-ready

### Key Management
- **test_key_rotation.py** - Ed25519 key rotation tests
- **test_real_key_rotation.py** - Real-world key rotation scenarios
- **test_auto_registration.py** - Automatic registration with key generation

## Running Tests

### Quick Test (Phase 2 Auto-Verification)
```bash
cd /Users/decimai/workspace/agent-identity-management/sdks/python
python3 tests/test_phase2_flow.py
```

Expected output:
```
================================================================================
🧪 Phase 2: Auto-Registration + Challenge-Response Test
================================================================================

📋 Test: High Trust Agent (Repo URL + Docs URL = 75 points)
--------------------------------------------------------------------------------

🔐 Signing challenge for automatic verification...
✅ Challenge verified successfully!
   ✅ Agent auto-approved! Trust score: 100

🎉 Agent registered successfully!
   Status: verified
   Trust Score: 100

✅ TEST PASSED!
```

### Run All Tests
```bash
pytest tests/
```

## Prerequisites

1. **Backend Server Running**:
   ```bash
   cd apps/backend
   ./server
   ```

2. **Dependencies Installed**:
   ```bash
   pip install -r requirements.txt
   ```

## Test Environment

- **API URL**: http://localhost:8080
- **Database**: PostgreSQL (via Docker)
- **Redis**: localhost:6379 (via Docker)

## Test Coverage

| Feature | Test File | Status |
|---------|-----------|--------|
| Agent Registration | test_new_agent.py | ✅ |
| Auto-Verification | test_phase2_flow.py | ✅ |
| Key Rotation | test_key_rotation.py | ✅ |
| Client Operations | test_client.py | ✅ |

## Notes

- Tests create real agents in the database
- Each test run generates unique agent names using timestamps
- Credentials are saved to `~/.aim/credentials.json`
- Tests are idempotent and can be run multiple times

## Troubleshooting

**Issue**: Connection refused
**Solution**: Ensure backend server is running on port 8080

**Issue**: Database errors
**Solution**: Run `docker compose up -d` to start PostgreSQL

**Issue**: Module not found
**Solution**: Install SDK in development mode: `pip install -e .`
