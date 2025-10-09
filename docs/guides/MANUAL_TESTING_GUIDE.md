# Manual Testing Guide - Agent Identity Management

This guide provides step-by-step instructions for manually testing the Agent Identity Management platform using Chrome DevTools MCP.

## Prerequisites

Before testing, ensure:
1. Docker Desktop is running
2. Database and Redis containers are up
3. Database migrations are applied
4. Backend server is running on :8080
5. Frontend dev server is running on :3000

## Quick Start Script

```bash
# 1. Start infrastructure
cd /Users/decimai/workspace/agent-identity-management
docker compose up -d postgres redis

# 2. Wait for services (30 seconds)
sleep 30

# 3. Run migrations
cd apps/backend
go run cmd/migrate/main.go up

# 4. Start backend (in one terminal)
go run cmd/server/main.go

# 5. Start frontend (in another terminal)
cd apps/web
npm run dev
```

## Chrome DevTools MCP Testing Workflow

### Test 1: Landing Page Load Test

```typescript
// Step 1: Navigate to landing page
mcp__chrome-devtools__navigate_page({
  url: "http://localhost:3000"
})

// Step 2: Take snapshot to see page structure
mcp__chrome-devtools__take_snapshot()

// Step 3: Take screenshot for visual verification
mcp__chrome-devtools__take_screenshot({
  filePath: "/Users/decimai/workspace/agent-identity-management/screenshots/landing-page.png"
})

// Step 4: Check for console errors
mcp__chrome-devtools__list_console_messages()

// Expected Results:
// - Page loads without errors
// - SSO buttons visible in snapshot
// - No error-level console messages
```

### Test 2: Google OAuth Initiation

```typescript
// Step 1: Navigate to landing page
mcp__chrome-devtools__navigate_page({
  url: "http://localhost:3000"
})

// Step 2: Get snapshot to find Google SSO button UID
mcp__chrome-devtools__take_snapshot()
// Look for button with text "Sign in with Google" or similar
// Note the UID from snapshot

// Step 3: Click Google SSO button
mcp__chrome-devtools__click({
  uid: "<google-button-uid-from-snapshot>"
})

// Step 4: Verify redirect to OAuth endpoint
mcp__chrome-devtools__list_network_requests({
  resourceTypes: ["document"]
})

// Expected Results:
// - Redirect to /api/v1/auth/login/google
// - Then redirect to accounts.google.com
// - Network requests show OAuth flow initiated
```

### Test 3: Dashboard Page (After Mock Login)

```typescript
// Note: For testing authenticated pages, you'll need to:
// 1. Either complete full OAuth flow with test credentials
// 2. Or manually set JWT token in localStorage via evaluate_script

// Step 1: Set mock authentication
mcp__chrome-devtools__navigate_page({
  url: "http://localhost:3000"
})

mcp__chrome-devtools__evaluate_script({
  function: `() => {
    localStorage.setItem('auth_token', 'mock-jwt-token');
    localStorage.setItem('user', JSON.stringify({
      id: '00000000-0000-0000-0000-000000000001',
      email: 'test@example.com',
      name: 'Test User',
      role: 'admin'
    }));
  }`
})

// Step 2: Navigate to dashboard
mcp__chrome-devtools__navigate_page({
  url: "http://localhost:3000/dashboard"
})

// Step 3: Take snapshot
mcp__chrome-devtools__take_snapshot()

// Step 4: Take screenshot
mcp__chrome-devtools__take_screenshot({
  filePath: "/Users/decimai/workspace/agent-identity-management/screenshots/dashboard.png"
})

// Expected Results:
// - Dashboard loads successfully
// - Stats cards visible (Total Agents, API Keys, etc.)
// - Sidebar navigation present
// - No console errors
```

### Test 4: Agent Registration Flow

```typescript
// Prerequisites: Mock authentication set (see Test 3)

// Step 1: Navigate to agent registration
mcp__chrome-devtools__navigate_page({
  url: "http://localhost:3000/dashboard/agents/new"
})

// Step 2: Get form snapshot
const snapshot = mcp__chrome-devtools__take_snapshot()
// Note UIDs for: AI Agent button, form inputs, submit button

// Step 3: Click AI Agent type
mcp__chrome-devtools__click({
  uid: "<ai-agent-button-uid>"
})

// Step 4: Fill form fields
mcp__chrome-devtools__fill_form({
  elements: [
    { uid: "<name-input-uid>", value: "test-agent" },
    { uid: "<display-name-input-uid>", value: "Test Agent" },
    { uid: "<description-textarea-uid>", value: "A comprehensive test agent" },
    { uid: "<version-input-uid>", value: "1.0.0" },
    { uid: "<repository-url-input-uid>", value: "https://github.com/test/agent" }
  ]
})

// Step 5: Take screenshot of filled form
mcp__chrome-devtools__take_screenshot({
  filePath: "/Users/decimai/workspace/agent-identity-management/screenshots/agent-form-filled.png"
})

// Step 6: Submit form
mcp__chrome-devtools__click({
  uid: "<submit-button-uid>"
})

// Step 7: Verify API request
mcp__chrome-devtools__list_network_requests({
  resourceTypes: ["xhr", "fetch"]
})

mcp__chrome-devtools__get_network_request({
  url: "/api/v1/agents"
})

// Expected Results:
// - Form fills correctly
// - POST request to /api/v1/agents with form data
// - Response 201 Created (or 401 if mock auth doesn't work)
// - Redirect to agent list page
```

### Test 5: API Key Generation

```typescript
// Prerequisites: Mock authentication set

// Step 1: Navigate to API keys page
mcp__chrome-devtools__navigate_page({
  url: "http://localhost:3000/dashboard/api-keys"
})

// Step 2: Get snapshot
mcp__chrome-devtools__take_snapshot()

// Step 3: Click "Generate API Key" button
mcp__chrome-devtools__click({
  uid: "<generate-key-button-uid>"
})

// Step 4: Verify modal appears
mcp__chrome-devtools__take_snapshot()

// Step 5: Fill key generation form
mcp__chrome-devtools__fill_form({
  elements: [
    { uid: "<agent-select-uid>", value: "<agent-id>" },
    { uid: "<key-name-input-uid>", value: "Production Key" },
    { uid: "<expiry-input-uid>", value: "90" }
  ]
})

// Step 6: Submit
mcp__chrome-devtools__click({
  uid: "<create-key-button-uid>"
})

// Step 7: Verify API request
mcp__chrome-devtools__list_network_requests()

// Step 8: Verify key display modal
mcp__chrome-devtools__take_snapshot()
mcp__chrome-devtools__take_screenshot({
  filePath: "/Users/decimai/workspace/agent-identity-management/screenshots/api-key-generated.png"
})

// Step 9: Test copy button
mcp__chrome-devtools__click({
  uid: "<copy-button-uid>"
})

mcp__chrome-devtools__evaluate_script({
  function: `async () => {
    return await navigator.clipboard.readText();
  }`
})

// Expected Results:
// - Modal appears with form
// - POST request to /api/v1/api-keys
// - Response contains API key
// - Key is displayed in modal
// - Copy button copies key to clipboard
```

### Test 6: Admin Dashboard

```typescript
// Prerequisites: Mock authentication with admin role

// Step 1: Navigate to admin panel
mcp__chrome-devtools__navigate_page({
  url: "http://localhost:3000/dashboard/admin"
})

// Step 2: Take snapshot
mcp__chrome-devtools__take_snapshot()

// Step 3: Take screenshot
mcp__chrome-devtools__take_screenshot({
  filePath: "/Users/decimai/workspace/agent-identity-management/screenshots/admin-dashboard.png"
})

// Step 4: Click Users tab
mcp__chrome-devtools__click({
  uid: "<users-tab-uid>"
})

// Step 5: Verify users table
mcp__chrome-devtools__take_snapshot()

// Step 6: Click Audit Logs tab
mcp__chrome-devtools__click({
  uid: "<audit-logs-tab-uid>"
})

// Step 7: Verify audit logs table
mcp__chrome-devtools__take_snapshot()
mcp__chrome-devtools__take_screenshot({
  filePath: "/Users/decimai/workspace/agent-identity-management/screenshots/audit-logs.png"
})

// Step 8: Click Alerts tab
mcp__chrome-devtools__click({
  uid: "<alerts-tab-uid>"
})

// Step 9: Verify alerts table
mcp__chrome-devtools__take_snapshot()

// Expected Results:
// - All tabs render correctly
// - Tables display data
// - No console errors
```

### Test 7: Responsive Design Testing

```typescript
// Test Mobile (375x667)
mcp__chrome-devtools__resize_page({
  width: 375,
  height: 667
})

mcp__chrome-devtools__navigate_page({
  url: "http://localhost:3000/dashboard"
})

mcp__chrome-devtools__take_screenshot({
  filePath: "/Users/decimai/workspace/agent-identity-management/screenshots/mobile-dashboard.png"
})

// Test Tablet (768x1024)
mcp__chrome-devtools__resize_page({
  width: 768,
  height: 1024
})

mcp__chrome-devtools__navigate_page({
  url: "http://localhost:3000/dashboard/agents"
})

mcp__chrome-devtools__take_screenshot({
  filePath: "/Users/decimai/workspace/agent-identity-management/screenshots/tablet-agents.png"
})

// Test Desktop (1920x1080)
mcp__chrome-devtools__resize_page({
  width: 1920,
  height: 1080
})

mcp__chrome-devtools__navigate_page({
  url: "http://localhost:3000/dashboard/admin"
})

mcp__chrome-devtools__take_screenshot({
  filePath: "/Users/decimai/workspace/agent-identity-management/screenshots/desktop-admin.png"
})

// Expected Results:
// - Layouts adapt to different screen sizes
// - All content accessible at each breakpoint
// - No horizontal scrolling
```

### Test 8: Performance Testing

```typescript
// Step 1: Start performance trace
mcp__chrome-devtools__performance_start_trace({
  reload: true,
  autoStop: true
})

// Step 2: Navigate to page
mcp__chrome-devtools__navigate_page({
  url: "http://localhost:3000/dashboard"
})

// Step 3: Stop trace (if not auto-stopped)
mcp__chrome-devtools__performance_stop_trace()

// Expected Results:
// - FCP < 1s
// - TTI < 2s
// - LCP < 2.5s
// - No long tasks blocking main thread
```

### Test 9: Error Handling

```typescript
// Test network error handling
mcp__chrome-devtools__navigate_page({
  url: "http://localhost:3000/dashboard/agents"
})

// Simulate slow network
mcp__chrome-devtools__emulate_network({
  throttlingOption: "Slow 3G"
})

// Try loading page
mcp__chrome-devtools__navigate_page({
  url: "http://localhost:3000/dashboard/agents/new"
})

// Verify loading states
mcp__chrome-devtools__take_snapshot()

// Reset network
mcp__chrome-devtools__emulate_network({
  throttlingOption: "No emulation"
})

// Expected Results:
// - Loading spinners appear during slow load
// - Page eventually loads
// - No crashes or blank screens
```

### Test 10: Console Error Check

```typescript
// Test all major pages for console errors
const pages = [
  "http://localhost:3000",
  "http://localhost:3000/dashboard",
  "http://localhost:3000/dashboard/agents",
  "http://localhost:3000/dashboard/agents/new",
  "http://localhost:3000/dashboard/api-keys",
  "http://localhost:3000/dashboard/admin"
];

for (const pageUrl of pages) {
  // Navigate to page
  mcp__chrome-devtools__navigate_page({ url: pageUrl })

  // Wait for load
  // (MCP handles this automatically)

  // Check console messages
  const messages = mcp__chrome-devtools__list_console_messages()

  // Take screenshot
  mcp__chrome-devtools__take_screenshot({
    filePath: `/Users/decimai/workspace/agent-identity-management/screenshots/${pageUrl.split('/').pop() || 'page'}.png`
  })
}

// Expected Results:
// - No error-level console messages on any page
// - Only info/log level messages if any
```

## Backend API Testing (via curl)

### Health Check
```bash
curl http://localhost:8080/api/v1/health

# Expected: {"status":"healthy","timestamp":"..."}
```

### OAuth Endpoints
```bash
# Google OAuth (should redirect)
curl -I http://localhost:8080/api/v1/auth/login/google

# Expected: 302 Found, Location: accounts.google.com/oauth2/...
```

### Authenticated Endpoints (Require JWT)
```bash
# List agents (unauthorized)
curl http://localhost:8080/api/v1/agents

# Expected: 401 Unauthorized

# Me endpoint (unauthorized)
curl http://localhost:8080/api/v1/auth/me

# Expected: 401 Unauthorized
```

## Test Results Checklist

### Backend Tests
- [ ] Health endpoint returns 200
- [ ] Database connection successful
- [ ] Redis connection successful
- [ ] OAuth endpoints redirect correctly
- [ ] Protected endpoints return 401 without auth
- [ ] No server errors in logs

### Frontend Tests (Chrome DevTools MCP)
- [ ] Landing page loads without errors
- [ ] Google OAuth flow initiates
- [ ] Dashboard displays (with mock auth)
- [ ] Agent registration form works
- [ ] API key generation modal works
- [ ] Admin panel loads (admin role)
- [ ] Responsive on mobile (375x667)
- [ ] Responsive on tablet (768x1024)
- [ ] Responsive on desktop (1920x1080)
- [ ] Performance metrics met (FCP < 1s, TTI < 2s)
- [ ] No console errors on any page
- [ ] Loading states display correctly
- [ ] Error states display correctly

### Integration Tests
- [ ] Agent creation end-to-end
- [ ] API key generation end-to-end
- [ ] Trust score calculation
- [ ] Audit log creation
- [ ] Alert generation

## Troubleshooting

### Issue: Backend won't start
**Solution:**
```bash
# Check if port 8080 is in use
lsof -i :8080

# Kill process if needed
kill -9 <PID>

# Restart backend
cd apps/backend
go run cmd/server/main.go
```

### Issue: Frontend won't start
**Solution:**
```bash
# Check if port 3000 is in use
lsof -i :3000

# Kill process if needed
kill -9 <PID>

# Restart frontend
cd apps/web
npm run dev
```

### Issue: Database connection failed
**Solution:**
```bash
# Check PostgreSQL container
docker ps | grep postgres

# Restart if needed
docker compose restart postgres

# Check logs
docker logs aim-postgres
```

### Issue: Mock auth doesn't work
**Solution:**
- Frontend might require real JWT validation
- Need to implement test JWT generation in backend
- Or modify frontend to accept mock tokens in dev mode
- Alternative: Complete full OAuth flow with test Google account

## Success Criteria

All tests pass when:
- ✅ All backend endpoints respond correctly
- ✅ All frontend pages load without errors
- ✅ All user flows work end-to-end
- ✅ Responsive design works on all breakpoints
- ✅ Performance metrics met
- ✅ No console errors
- ✅ Screenshots captured for visual verification

## Next Steps After Testing

1. Fix any bugs found during testing
2. Update code based on test results
3. Re-run tests to verify fixes
4. Document known issues
5. Prepare for production deployment
