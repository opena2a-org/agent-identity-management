# End-to-End Testing Prompt for OAuth SDK Download Feature

## Context
You are testing the newly implemented OAuth-first SDK download feature in the AIM (Agent Identity Management) system. This feature allows users to download a pre-configured Python SDK with embedded OAuth credentials for zero-configuration agent registration.

## What Was Built
A complete OAuth-first architecture that enables:
1. Users login via OAuth (Google/Microsoft/Okta)
2. Download pre-configured SDK from dashboard with embedded 1-year refresh token
3. SDK automatically authenticates and links agents to real users
4. Zero configuration required - just one line of code!

## Your Task
Test the complete end-to-end flow from login to agent registration to dashboard verification.

## Project Structure
```
agent-identity-management/
├── apps/
│   ├── backend/          # Go backend (Fiber v3)
│   │   └── server        # Compiled binary
│   └── web/              # Next.js 15 frontend
│       └── .next/        # Built assets
└── sdks/
    └── python/           # Python SDK
        └── aim_sdk/
```

## Prerequisites Check
Before starting, verify:
- [ ] Backend server binary exists: `apps/backend/server`
- [ ] Frontend is built: `apps/web/.next/` directory exists
- [ ] PostgreSQL is running on port 5432
- [ ] Redis is running on port 6379
- [ ] Environment variables are set (see below)

## Environment Setup

### Backend (.env file location: `apps/backend/.env`)
```bash
# Database
DATABASE_HOST=localhost
DATABASE_PORT=5432
DATABASE_USER=postgres
DATABASE_PASSWORD=postgres
DATABASE_NAME=aim
DATABASE_SSLMODE=disable

# Redis
REDIS_HOST=localhost
REDIS_PORT=6379

# JWT
JWT_SECRET=dev-secret-change-in-production

# OAuth (at least one provider needed)
GOOGLE_CLIENT_ID=your-google-client-id
GOOGLE_CLIENT_SECRET=your-google-client-secret
GOOGLE_REDIRECT_URI=http://localhost:8080/api/v1/oauth/google/callback

# Server
AIM_PUBLIC_URL=http://localhost:8080
```

### Frontend (.env.local file location: `apps/web/.env.local`)
```bash
NEXT_PUBLIC_API_URL=http://localhost:8080
```

## Step-by-Step Testing Instructions

### Step 1: Start Services

#### 1a. Start Backend
```bash
cd /Users/decimai/workspace/agent-identity-management/apps/backend

# Check if server binary exists
ls -la server

# If it doesn't exist, build it:
go build -o server ./cmd/server

# Run the server
./server
```

**Expected output**:
```
🚀 Agent Identity Management API starting on port 8080
✅ Database connected
✅ Redis connected
✅ KeyVault initialized for automatic key generation
🔐 OAuth Providers: Google=true, Microsoft=false, Okta=false
```

**Troubleshooting**:
- If database connection fails: Check PostgreSQL is running with `psql -U postgres -d aim -c "SELECT 1"`
- If Redis fails: Check Redis is running with `redis-cli ping`
- If port 8080 in use: Kill process with `lsof -ti:8080 | xargs kill -9`

#### 1b. Start Frontend
```bash
cd /Users/decimai/workspace/agent-identity-management/apps/web

# Check if built
ls -la .next/

# If not built:
npm run build

# Start the server
npm run dev
```

**Expected output**:
```
ready - started server on 0.0.0.0:3000, url: http://localhost:3000
```

### Step 2: Login via OAuth

#### 2a. Open Browser
```bash
open http://localhost:3000/login
```

#### 2b. Login with OAuth
1. Click "Sign in with Google" (or your configured provider)
2. Complete OAuth flow
3. Should redirect to `http://localhost:3000/dashboard`

**Expected result**:
- Dashboard loads successfully
- Sidebar shows "Download SDK" link
- No console errors in browser DevTools

**Troubleshooting**:
- If OAuth fails: Check `GOOGLE_CLIENT_ID` and redirect URI match Google Cloud Console
- If stuck on login: Check backend logs for OAuth errors
- If 401 errors: Check JWT token is saved in localStorage (`auth_token` key)

### Step 3: Download SDK

#### 3a. Navigate to SDK Page
1. Click "Download SDK" in the sidebar
2. Should navigate to `/dashboard/sdk`

**Expected UI**:
- Page title: "Download SDK"
- Python SDK card with download button
- Quick Start guide with code examples
- Feature cards (Zero Config, Auto-Auth, One-Line)

#### 3b. Download SDK
1. Click "Download SDK" button
2. Button should show "Downloading..." state
3. File `aim-sdk-python.zip` should download

**Verify download**:
```bash
# Check file exists in Downloads folder
ls -la ~/Downloads/aim-sdk-python.zip

# Should be around 50-100KB
du -h ~/Downloads/aim-sdk-python.zip
```

**Troubleshooting**:
- If download fails with 401: Check JWT token in localStorage
- If download fails with 500: Check backend logs for error
- If no file downloads: Check browser console for errors

### Step 4: Extract and Inspect SDK

#### 4a. Extract ZIP
```bash
cd ~/Downloads
unzip aim-sdk-python.zip
cd aim-sdk-python
```

**Verify contents**:
```bash
ls -la
# Should see:
# - aim_sdk/          (Python package)
# - setup.py          (Installation file)
# - README.md         (Documentation)
# - QUICKSTART.md     (Setup guide)
# - .aim/             (Credentials directory)
```

#### 4b. Verify Credentials File
```bash
cat .aim/credentials.json
```

**Expected JSON structure**:
```json
{
  "aim_url": "http://localhost:8080",
  "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user_id": "some-uuid-here",
  "email": "your-email@example.com"
}
```

**Verify token fields**:
- `aim_url`: Should match your backend URL
- `refresh_token`: Should be a long JWT string (starts with `eyJ`)
- `user_id`: Should be a valid UUID
- `email`: Should be your OAuth email

**Troubleshooting**:
- If credentials.json missing: SDK download endpoint failed to embed credentials
- If refresh_token is empty: JWT generation failed on backend
- If user_id is wrong: OptionalAuthMiddleware not extracting user correctly

### Step 5: Install SDK

#### 5a. Create Test Directory
```bash
mkdir -p ~/aim-sdk-test
cp -r ~/Downloads/aim-sdk-python ~/aim-sdk-test/
cd ~/aim-sdk-test/aim-sdk-python
```

#### 5b. Install SDK
```bash
# Install in development mode
pip install -e .

# Verify installation
python -c "import aim_sdk; print(aim_sdk.__version__)"
```

**Expected output**:
```
1.0.0
```

**Troubleshooting**:
- If import fails: Check Python 3.8+ is installed
- If dependencies fail: Install manually with `pip install requests pynacl`

### Step 6: Copy Credentials to Home Directory

The SDK looks for credentials in `~/.aim/credentials.json`:

```bash
mkdir -p ~/.aim
cp .aim/credentials.json ~/.aim/
cat ~/.aim/credentials.json
```

**Verify**:
- File exists at `~/.aim/credentials.json`
- Contains valid JSON with refresh_token

### Step 7: Test Zero-Config Registration

#### 7a. Create Test Script
```bash
cd ~/aim-sdk-test
cat > test_agent.py << 'EOF'
"""Test OAuth-authenticated agent registration."""

from aim_sdk import register_agent

# Zero configuration - URL and auth auto-detected!
print("🚀 Registering agent with zero config...")

agent = register_agent(
    name="test-oauth-agent",
    display_name="Test OAuth Agent",
    description="Testing OAuth-first SDK download feature",
    agent_type="ai_agent",
    version="1.0.0"
)

print(f"\n✅ Success!")
print(f"   Agent ID: {agent.agent_id}")
print(f"   Dashboard: {agent.aim_url}/dashboard/agents")
print(f"\nAgent registered and linked to your user account!")
EOF
```

#### 7b. Run Test
```bash
python test_agent.py
```

**Expected output**:
```
🚀 Registering agent with zero config...
✨ Auto-detected AIM URL: http://localhost:8080
🔐 Using OAuth authentication from SDK credentials

🎉 Agent registered successfully!
   Agent ID: <some-uuid>
   Status: verified
   Trust Score: 70.0

✅ Success!
   Agent ID: <same-uuid>
   Dashboard: http://localhost:8080/dashboard/agents

Agent registered and linked to your user account!
```

**Troubleshooting**:
- If "aim_url parameter is required": Credentials not found at `~/.aim/credentials.json`
- If "Failed to refresh token": Backend `/api/v1/auth/refresh` endpoint issue
- If "Registration failed": Check backend logs for detailed error
- If agent_id is None: Registration succeeded but response parsing failed

### Step 8: Verify in Dashboard

#### 8a. Open Agents Dashboard
```bash
open http://localhost:3000/dashboard/agents
```

#### 8b. Find Your Agent
1. Look for "test-oauth-agent" in the list
2. Click on it to view details

**Expected details**:
- Name: "test-oauth-agent"
- Display Name: "Test OAuth Agent"
- Type: "AI Agent"
- Status: "Verified" (green badge)
- Trust Score: ~70.0
- Version: "1.0.0"
- Created timestamp: Recent

#### 8c. Verify User Ownership
The agent should be linked to YOUR user account (not the default user).

**How to verify**:
1. Check backend database directly:
```bash
psql -U postgres -d aim -c "
SELECT
    a.id,
    a.name,
    a.display_name,
    u.email as owner_email,
    o.name as organization
FROM agents a
JOIN users u ON a.created_by = u.id
JOIN organizations o ON a.organization_id = o.id
WHERE a.name = 'test-oauth-agent'
ORDER BY a.created_at DESC
LIMIT 1;
"
```

**Expected result**:
```
          id          |       name        |   display_name    |    owner_email     | organization
----------------------+-------------------+-------------------+--------------------+--------------
 <uuid>              | test-oauth-agent  | Test OAuth Agent  | your@email.com     | example.com
```

**Key verification**: `owner_email` should be YOUR email, NOT a default/fake user!

**Troubleshooting**:
- If owner_email is wrong: OptionalAuthMiddleware not extracting JWT correctly
- If organization is "Unknown": Organization auto-provisioning failed
- If agent not found: Registration failed silently, check backend logs

### Step 9: Verify Audit Trail

Check that the registration action was recorded with correct user:

```bash
psql -U postgres -d aim -c "
SELECT
    al.action,
    al.resource_type,
    al.resource_id,
    u.email as performed_by,
    al.created_at
FROM audit_logs al
JOIN users u ON al.user_id = u.id
WHERE al.resource_type = 'agent'
  AND al.action = 'create'
ORDER BY al.created_at DESC
LIMIT 5;
"
```

**Expected result**:
```
 action | resource_type | resource_id | performed_by      | created_at
--------+---------------+-------------+-------------------+-------------------
 create | agent         | <uuid>      | your@email.com    | 2025-10-08 ...
```

**Key verification**: Audit log shows YOUR email, not default user!

### Step 10: Test Multiple Registrations

Verify that multiple agents are all linked to the same user:

```bash
cat > test_multiple_agents.py << 'EOF'
from aim_sdk import register_agent

for i in range(3):
    agent = register_agent(
        name=f"multi-test-{i}",
        display_name=f"Multi Test Agent {i}",
        description="Testing multiple registrations",
        force_new=True  # Force new registration each time
    )
    print(f"✅ Registered: {agent.name} (ID: {agent.agent_id})")
EOF

python test_multiple_agents.py
```

**Expected output**:
```
🔐 Using OAuth authentication from SDK credentials
✅ Registered: multi-test-0 (ID: <uuid-1>)
🔐 Using OAuth authentication from SDK credentials
✅ Registered: multi-test-1 (ID: <uuid-2>)
🔐 Using OAuth authentication from SDK credentials
✅ Registered: multi-test-2 (ID: <uuid-3>)
```

**Verify in database**:
```bash
psql -U postgres -d aim -c "
SELECT
    a.name,
    u.email as owner_email
FROM agents a
JOIN users u ON a.created_by = u.id
WHERE a.name LIKE 'multi-test-%'
ORDER BY a.created_at DESC;
"
```

**Expected**: All 3 agents owned by YOUR email.

---

## Success Criteria

### ✅ All Tests Pass If:

1. **Backend & Frontend Running**:
   - [ ] Backend started without errors
   - [ ] Frontend accessible at localhost:3000
   - [ ] No crash logs in either service

2. **OAuth Login**:
   - [ ] OAuth flow completes successfully
   - [ ] JWT token saved in localStorage
   - [ ] Dashboard loads after login

3. **SDK Download**:
   - [ ] Download button works
   - [ ] ZIP file downloaded (~50-100KB)
   - [ ] ZIP contains all expected files
   - [ ] credentials.json has valid refresh_token

4. **SDK Installation**:
   - [ ] `pip install -e .` succeeds
   - [ ] Can import aim_sdk
   - [ ] Credentials copied to ~/.aim/

5. **Zero-Config Registration**:
   - [ ] `register_agent("name")` works (no aim_url needed!)
   - [ ] Prints "Auto-detected AIM URL"
   - [ ] Prints "Using OAuth authentication"
   - [ ] Returns valid agent_id

6. **Dashboard Verification**:
   - [ ] Agent appears in dashboard
   - [ ] Agent has correct details
   - [ ] Agent status is "Verified"
   - [ ] Trust score is ~70.0

7. **User Attribution** (CRITICAL):
   - [ ] Database shows YOUR email as owner
   - [ ] NOT showing default/fake user
   - [ ] Audit log has YOUR email
   - [ ] Multiple agents all owned by YOU

---

## Expected Issues & Solutions

### Issue 1: "aim_url parameter is required"
**Cause**: Credentials not found at `~/.aim/credentials.json`
**Solution**:
```bash
cp ~/aim-sdk-test/aim-sdk-python/.aim/credentials.json ~/.aim/
```

### Issue 2: "Token refresh failed"
**Cause**: Backend `/api/v1/auth/refresh` endpoint not implemented
**Solution**: This endpoint needs to be added to backend (not implemented yet!)

**Backend code needed** (add to `apps/backend/internal/interfaces/http/handlers/auth_handler.go`):
```go
func (h *AuthHandler) RefreshToken(c fiber.Ctx) error {
    var req struct {
        RefreshToken string `json:"refresh_token"`
    }

    if err := c.Bind().JSON(&req); err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request"})
    }

    newAccessToken, err := h.jwtService.RefreshAccessToken(req.RefreshToken)
    if err != nil {
        return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid refresh token"})
    }

    return c.JSON(fiber.Map{"access_token": newAccessToken})
}
```

**Route needed** (add to `apps/backend/cmd/server/main.go`):
```go
auth.Post("/refresh", h.Auth.RefreshToken)  // Add this line
```

### Issue 3: Agent owned by default user
**Cause**: OptionalAuthMiddleware not extracting JWT correctly
**Solution**: Check that refresh token is being exchanged for access token correctly

### Issue 4: "Registration failed: ..."
**Cause**: Various backend validation errors
**Solution**: Check backend logs for detailed error message

---

## Reporting Results

After completing all tests, create a report with:

### Test Results Summary
```markdown
## E2E Test Results - OAuth SDK Download

**Date**: [Current Date]
**Tester**: [Your Name]
**Branch**: feature/oauth-sdk-download

### Test Status
- [ ] Backend Started: ✅/❌
- [ ] Frontend Started: ✅/❌
- [ ] OAuth Login: ✅/❌
- [ ] SDK Download: ✅/❌
- [ ] SDK Installation: ✅/❌
- [ ] Zero-Config Registration: ✅/❌
- [ ] Dashboard Verification: ✅/❌
- [ ] User Attribution: ✅/❌ (CRITICAL!)

### Issues Found
1. [Issue description]
   - Steps to reproduce
   - Expected behavior
   - Actual behavior
   - Error logs

### Screenshots
- OAuth login page
- SDK download page
- Agent in dashboard
- Database query results

### Overall Result
✅ PASS - All tests successful, ready for production
❌ FAIL - Issues found, needs fixes before merge
```

---

## Quick Test Command

For quick testing without reading everything:

```bash
# 1. Start services
cd /Users/decimai/workspace/agent-identity-management/apps/backend && ./server &
cd /Users/decimai/workspace/agent-identity-management/apps/web && npm run dev &

# 2. Login
open http://localhost:3000/login

# 3. Download SDK from dashboard (/dashboard/sdk)

# 4. Install and test
cd ~/Downloads
unzip aim-sdk-python.zip
cd aim-sdk-python
pip install -e .
mkdir -p ~/.aim && cp .aim/credentials.json ~/.aim/

# 5. Register agent
python -c "from aim_sdk import register_agent; agent = register_agent('quick-test'); print(f'Agent ID: {agent.agent_id}')"

# 6. Verify in dashboard
open http://localhost:3000/dashboard/agents

# 7. Check database
psql -U postgres -d aim -c "SELECT a.name, u.email FROM agents a JOIN users u ON a.created_by = u.id WHERE a.name = 'quick-test';"
```

---

## Additional Notes

### Token Refresh Endpoint
The Python SDK expects a `/api/v1/auth/refresh` endpoint that may not be implemented yet. If you get token refresh errors, you'll need to add this endpoint to the backend first.

### Browser DevTools
Keep browser DevTools open (F12) on the Network tab to see:
- API calls to `/api/v1/sdk/download`
- Response status codes
- Response headers (Content-Type, Content-Disposition)

### Backend Logs
Watch backend logs for:
- "Using OAuth authentication from SDK credentials" (good!)
- JWT token validation success/failure
- User ID extraction from JWT claims
- Agent creation with correct user_id

### Success Indicator
The ultimate success indicator is seeing YOUR email address in the database query, not a default user. This proves the OAuth authentication chain is working end-to-end.

---

**Good luck with testing! This is the final piece to prove the OAuth-first architecture works perfectly.** 🚀
