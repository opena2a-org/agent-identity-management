# Integration Test Plan - Agent Identity Management

## Overview
This document outlines comprehensive integration and E2E testing procedures for the Agent Identity Management platform, following the CLAUDE_CONTEXT.md mandate to use Chrome DevTools MCP for frontend testing.

## Test Environment Setup

### Prerequisites
```bash
# 1. Start infrastructure services
docker compose up -d postgres redis

# 2. Wait for services to be healthy
docker compose ps

# 3. Run database migrations
cd apps/backend
go run cmd/migrate/main.go up

# 4. Start backend server
go run cmd/server/main.go

# 5. Start frontend (new terminal)
cd apps/web
npm run dev
```

### Environment Variables
Ensure both `.env` files are configured:
- `apps/backend/.env` - Backend configuration
- `apps/web/.env.local` - Frontend configuration

---

## Backend Integration Tests

### Test Suite 1: Health & Infrastructure

#### Test 1.1: Health Endpoint
```bash
curl http://localhost:8080/api/v1/health

Expected Response:
{
  "status": "healthy",
  "timestamp": "2025-01-XX..."
}
```

#### Test 1.2: Database Connection
```bash
docker exec -it aim-postgres psql -U postgres -d identity -c "\dt"

Expected: List of all tables (users, organizations, agents, api_keys, etc.)
```

#### Test 1.3: Redis Connection
```bash
docker exec -it aim-redis redis-cli ping

Expected: PONG
```

### Test Suite 2: Authentication Endpoints

#### Test 2.1: OAuth Initiation (Google)
```bash
curl -I http://localhost:8080/api/v1/auth/login/google

Expected: 302 redirect to Google OAuth
```

#### Test 2.2: OAuth Initiation (Microsoft)
```bash
curl -I http://localhost:8080/api/v1/auth/login/microsoft

Expected: 302 redirect to Microsoft OAuth (if configured)
```

#### Test 2.3: Me Endpoint (No Auth)
```bash
curl http://localhost:8080/api/v1/auth/me

Expected: 401 Unauthorized
```

### Test Suite 3: Agent Management (With Mock Auth)

#### Test 3.1: List Agents (Unauthorized)
```bash
curl http://localhost:8080/api/v1/agents

Expected: 401 Unauthorized
```

#### Test 3.2: Create Agent (Authorized)
```bash
curl -X POST http://localhost:8080/api/v1/agents \
  -H "Authorization: Bearer <valid-jwt>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "test-agent",
    "display_name": "Test Agent",
    "description": "A test AI agent",
    "agent_type": "ai_agent",
    "version": "1.0.0"
  }'

Expected: 201 Created with agent object
```

#### Test 3.3: Get Agent by ID
```bash
curl http://localhost:8080/api/v1/agents/:id \
  -H "Authorization: Bearer <valid-jwt>"

Expected: 200 OK with agent object
```

#### Test 3.4: Update Agent
```bash
curl -X PUT http://localhost:8080/api/v1/agents/:id \
  -H "Authorization: Bearer <valid-jwt>" \
  -H "Content-Type: application/json" \
  -d '{
    "display_name": "Updated Test Agent",
    "description": "An updated test AI agent"
  }'

Expected: 200 OK with updated agent
```

#### Test 3.5: Delete Agent
```bash
curl -X DELETE http://localhost:8080/api/v1/agents/:id \
  -H "Authorization: Bearer <valid-jwt>"

Expected: 204 No Content
```

### Test Suite 4: API Key Management

#### Test 4.1: Generate API Key
```bash
curl -X POST http://localhost:8080/api/v1/api-keys \
  -H "Authorization: Bearer <valid-jwt>" \
  -H "Content-Type: application/json" \
  -d '{
    "agent_id": "<agent-uuid>",
    "name": "Production Key",
    "expires_in_days": 90
  }'

Expected: 201 Created with api_key object (plaintext key shown once)
```

#### Test 4.2: List API Keys
```bash
curl http://localhost:8080/api/v1/api-keys \
  -H "Authorization: Bearer <valid-jwt>"

Expected: 200 OK with array of api_key objects
```

#### Test 4.3: Revoke API Key
```bash
curl -X DELETE http://localhost:8080/api/v1/api-keys/:id \
  -H "Authorization: Bearer <valid-jwt>"

Expected: 204 No Content
```

### Test Suite 5: Trust Scoring

#### Test 5.1: Calculate Trust Score
```bash
curl -X POST http://localhost:8080/api/v1/trust-scores/:agentId/calculate \
  -H "Authorization: Bearer <valid-jwt>"

Expected: 200 OK with trust_score object (score between 0.0-1.0)
```

#### Test 5.2: Get Trust Score History
```bash
curl http://localhost:8080/api/v1/trust-scores/:agentId \
  -H "Authorization: Bearer <valid-jwt>"

Expected: 200 OK with array of historical trust scores
```

### Test Suite 6: Admin Endpoints

#### Test 6.1: List Users (Admin Only)
```bash
curl http://localhost:8080/api/v1/admin/users \
  -H "Authorization: Bearer <admin-jwt>"

Expected: 200 OK with array of users
```

#### Test 6.2: Update User Role
```bash
curl -X PUT http://localhost:8080/api/v1/admin/users/:id/role \
  -H "Authorization: Bearer <admin-jwt>" \
  -H "Content-Type: application/json" \
  -d '{
    "role": "manager"
  }'

Expected: 200 OK with updated user
```

#### Test 6.3: Get Audit Logs
```bash
curl http://localhost:8080/api/v1/admin/audit-logs \
  -H "Authorization: Bearer <admin-jwt>"

Expected: 200 OK with array of audit logs
```

#### Test 6.4: Get Alerts
```bash
curl http://localhost:8080/api/v1/admin/alerts \
  -H "Authorization: Bearer <admin-jwt>"

Expected: 200 OK with array of alerts
```

### Test Suite 7: Compliance Endpoints

#### Test 7.1: Generate Compliance Report
```bash
curl -X POST http://localhost:8080/api/v1/compliance/reports \
  -H "Authorization: Bearer <admin-jwt>" \
  -H "Content-Type: application/json" \
  -d '{
    "report_type": "access_review",
    "start_date": "2025-01-01",
    "end_date": "2025-01-31"
  }'

Expected: 200 OK with report object
```

#### Test 7.2: Get Compliance Status
```bash
curl http://localhost:8080/api/v1/compliance/status \
  -H "Authorization: Bearer <admin-jwt>"

Expected: 200 OK with compliance status object
```

---

## Frontend E2E Tests (Chrome DevTools MCP)

### Test Suite 1: Landing Page & Navigation

#### Test 1.1: Landing Page Loads
```typescript
// Navigate to landing page
mcp__chrome-devtools__navigate_page({ url: "http://localhost:3000" })

// Take snapshot to verify content
mcp__chrome-devtools__take_snapshot()

// Expected: Landing page with "Sign in with Google" button visible
// Verify in snapshot: title, description, SSO button UIDs

// Take screenshot for visual verification
mcp__chrome-devtools__take_screenshot({ filePath: "screenshots/landing-page.png" })
```

#### Test 1.2: Navigation Elements
```typescript
// Check header/footer elements exist
mcp__chrome-devtools__take_snapshot()

// Verify: Logo, navigation menu, footer links present
```

### Test Suite 2: SSO Authentication Flow

#### Test 2.1: Google OAuth Initiation
```typescript
// Navigate to landing page
mcp__chrome-devtools__navigate_page({ url: "http://localhost:3000" })

// Take snapshot to get SSO button UID
const snapshot = mcp__chrome-devtools__take_snapshot()

// Click "Sign in with Google" button
mcp__chrome-devtools__click({ uid: "<google-sso-button-uid>" })

// Verify OAuth redirect
const requests = mcp__chrome-devtools__list_network_requests({
  resourceTypes: ["document"]
})

// Expected: Redirect to localhost:8080/api/v1/auth/login/google
// Then redirect to Google OAuth URL
```

#### Test 2.2: OAuth Callback Handling
```typescript
// Simulate successful OAuth callback
// (In real test, need valid OAuth credentials)

// Navigate to dashboard after mock login
mcp__chrome-devtools__navigate_page({ url: "http://localhost:3000/dashboard" })

// Verify dashboard loads
mcp__chrome-devtools__take_snapshot()

// Expected: Dashboard with user info, agent list
```

### Test Suite 3: Dashboard Features

#### Test 3.1: Dashboard Page Loads
```typescript
// Navigate to dashboard
mcp__chrome-devtools__navigate_page({ url: "http://localhost:3000/dashboard" })

// Take snapshot
mcp__chrome-devtools__take_snapshot()

// Verify elements: sidebar, stats cards, recent activity
// Take screenshot
mcp__chrome-devtools__take_screenshot({ filePath: "screenshots/dashboard.png" })
```

#### Test 3.2: Statistics Display
```typescript
// Check stats are rendered
const snapshot = mcp__chrome-devtools__take_snapshot()

// Expected: Total Agents, Total API Keys, Trust Score Average, Active Alerts
```

### Test Suite 4: Agent Registration Flow

#### Test 4.1: Navigate to New Agent Form
```typescript
// Click "Register Agent" or navigate directly
mcp__chrome-devtools__navigate_page({ url: "http://localhost:3000/dashboard/agents/new" })

// Take snapshot to get form field UIDs
const snapshot = mcp__chrome-devtools__take_snapshot()

// Verify: Form with all required fields visible
```

#### Test 4.2: Select Agent Type
```typescript
// Get snapshot with field UIDs
const snapshot = mcp__chrome-devtools__take_snapshot()

// Click "AI Agent" type button
mcp__chrome-devtools__click({ uid: "<ai-agent-button-uid>" })

// Verify selection
mcp__chrome-devtools__take_screenshot()

// Expected: AI Agent card has blue border/highlight
```

#### Test 4.3: Fill Agent Registration Form
```typescript
// Fill all form fields
mcp__chrome-devtools__fill_form({
  elements: [
    { uid: "<name-input-uid>", value: "test-agent" },
    { uid: "<display-name-input-uid>", value: "Test Agent" },
    { uid: "<description-textarea-uid>", value: "A comprehensive test agent for E2E testing" },
    { uid: "<version-input-uid>", value: "1.0.0" },
    { uid: "<repository-url-input-uid>", value: "https://github.com/test/agent" },
    { uid: "<documentation-url-input-uid>", value: "https://docs.test.com" }
  ]
})

// Take screenshot to verify form filled
mcp__chrome-devtools__take_screenshot({ filePath: "screenshots/agent-form-filled.png" })
```

#### Test 4.4: Submit Agent Registration
```typescript
// Click submit button
mcp__chrome-devtools__click({ uid: "<submit-button-uid>" })

// Verify API request sent
const requests = mcp__chrome-devtools__list_network_requests({
  resourceTypes: ["xhr", "fetch"]
})

// Expected: POST request to /api/v1/agents

// Check for success response
const agentRequest = mcp__chrome-devtools__get_network_request({
  url: "/api/v1/agents"
})

// Expected: 201 Created response

// Verify redirect to agent list or detail page
mcp__chrome-devtools__take_snapshot()
```

### Test Suite 5: API Key Management

#### Test 5.1: Navigate to API Keys Page
```typescript
// Navigate to API keys
mcp__chrome-devtools__navigate_page({ url: "http://localhost:3000/dashboard/api-keys" })

// Take snapshot
mcp__chrome-devtools__take_snapshot()

// Verify: "Generate API Key" button visible
```

#### Test 5.2: Generate API Key
```typescript
// Click "Generate API Key" button
mcp__chrome-devtools__click({ uid: "<generate-key-button-uid>" })

// Verify modal appears
mcp__chrome-devtools__take_snapshot()

// Fill key generation form
mcp__chrome-devtools__fill_form({
  elements: [
    { uid: "<agent-select-uid>", value: "<agent-id>" },
    { uid: "<key-name-input-uid>", value: "Production Key" },
    { uid: "<expiry-input-uid>", value: "90" }
  ]
})

// Submit
mcp__chrome-devtools__click({ uid: "<create-key-button-uid>" })

// Verify API request
const requests = mcp__chrome-devtools__list_network_requests()

// Expected: POST /api/v1/api-keys with 201 response
```

#### Test 5.3: Display Generated Key
```typescript
// Verify key display modal appears
mcp__chrome-devtools__take_snapshot()

// Expected: Modal with plaintext API key, copy button, download buttons

// Take screenshot
mcp__chrome-devtools__take_screenshot({ filePath: "screenshots/api-key-generated.png" })
```

#### Test 5.4: Copy API Key to Clipboard
```typescript
// Click copy button
mcp__chrome-devtools__click({ uid: "<copy-button-uid>" })

// Verify clipboard content
const clipboardContent = mcp__chrome-devtools__evaluate_script({
  function: "async () => { return await navigator.clipboard.readText(); }"
})

// Expected: Clipboard contains the API key

// Verify success toast
mcp__chrome-devtools__take_snapshot()
```

#### Test 5.5: Download API Key Files
```typescript
// Click "Download as .txt"
mcp__chrome-devtools__click({ uid: "<download-txt-button-uid>" })

// Click "Download as .env"
mcp__chrome-devtools__click({ uid: "<download-env-button-uid>" })

// Verify downloads initiated (check network tab)
const requests = mcp__chrome-devtools__list_network_requests()
```

### Test Suite 6: Trust Score Display

#### Test 6.1: View Agent Trust Score
```typescript
// Navigate to agent detail page
mcp__chrome-devtools__navigate_page({ url: "http://localhost:3000/dashboard/agents/<agent-id>" })

// Take snapshot
mcp__chrome-devtools__take_snapshot()

// Verify: Trust score badge, percentage, breakdown chart

// Take screenshot
mcp__chrome-devtools__take_screenshot({ filePath: "screenshots/trust-score.png" })
```

#### Test 6.2: Calculate New Trust Score
```typescript
// Click "Recalculate Trust Score" button
mcp__chrome-devtools__click({ uid: "<recalculate-button-uid>" })

// Verify API request
const requests = mcp__chrome-devtools__list_network_requests({
  resourceTypes: ["xhr", "fetch"]
})

// Expected: POST /api/v1/trust-scores/:agentId/calculate

// Verify UI updates with new score
mcp__chrome-devtools__take_snapshot()
```

### Test Suite 7: Admin Dashboard

#### Test 7.1: Navigate to Admin Panel (Admin Only)
```typescript
// Navigate to admin panel
mcp__chrome-devtools__navigate_page({ url: "http://localhost:3000/dashboard/admin" })

// Take snapshot
mcp__chrome-devtools__take_snapshot()

// Expected: Admin stats, tabs for Users/Audit Logs/Alerts

// Take screenshot
mcp__chrome-devtools__take_screenshot({ filePath: "screenshots/admin-dashboard.png" })
```

#### Test 7.2: User Management Tab
```typescript
// Click Users tab
mcp__chrome-devtools__click({ uid: "<users-tab-uid>" })

// Verify users table loads
mcp__chrome-devtools__take_snapshot()

// Expected: Table with user list, role badges, action buttons
```

#### Test 7.3: Update User Role
```typescript
// Click "Edit" on a user
mcp__chrome-devtools__click({ uid: "<edit-user-button-uid>" })

// Change role dropdown
mcp__chrome-devtools__fill({ uid: "<role-select-uid>", value: "manager" })

// Save changes
mcp__chrome-devtools__click({ uid: "<save-button-uid>" })

// Verify API request
const requests = mcp__chrome-devtools__list_network_requests()

// Expected: PUT /api/v1/admin/users/:id/role
```

#### Test 7.4: Audit Logs Tab
```typescript
// Click Audit Logs tab
mcp__chrome-devtools__click({ uid: "<audit-logs-tab-uid>" })

// Verify audit logs table loads
mcp__chrome-devtools__take_snapshot()

// Expected: Table with timestamp, user, action, resource columns
```

#### Test 7.5: Filter Audit Logs
```typescript
// Fill filter inputs
mcp__chrome-devtools__fill_form({
  elements: [
    { uid: "<action-filter-uid>", value: "create" },
    { uid: "<start-date-uid>", value: "2025-01-01" },
    { uid: "<end-date-uid>", value: "2025-01-31" }
  ]
})

// Click "Apply Filters"
mcp__chrome-devtools__click({ uid: "<apply-filters-button-uid>" })

// Verify filtered results
mcp__chrome-devtools__take_snapshot()
```

#### Test 7.6: Alerts Tab
```typescript
// Click Alerts tab
mcp__chrome-devtools__click({ uid: "<alerts-tab-uid>" })

// Verify alerts table loads
mcp__chrome-devtools__take_snapshot()

// Expected: Table with severity badges, alert messages, actions
```

### Test Suite 8: Error Handling

#### Test 8.1: Network Error Handling
```typescript
// Navigate to a page
mcp__chrome-devtools__navigate_page({ url: "http://localhost:3000/dashboard/agents" })

// Simulate network offline (use DevTools network throttling)
mcp__chrome-devtools__emulate_network({ throttlingOption: "No emulation" })

// Try action that requires API call
mcp__chrome-devtools__click({ uid: "<some-action-button-uid>" })

// Verify error message displays
mcp__chrome-devtools__take_snapshot()

// Expected: User-friendly error message with retry option
```

#### Test 8.2: Validation Errors
```typescript
// Navigate to agent form
mcp__chrome-devtools__navigate_page({ url: "http://localhost:3000/dashboard/agents/new" })

// Submit empty form
mcp__chrome-devtools__click({ uid: "<submit-button-uid>" })

// Verify validation errors
mcp__chrome-devtools__take_snapshot()

// Expected: Required field error messages
```

### Test Suite 9: Responsive Design

#### Test 9.1: Mobile View (375x667)
```typescript
// Resize to mobile
mcp__chrome-devtools__resize_page({ width: 375, height: 667 })

// Navigate to dashboard
mcp__chrome-devtools__navigate_page({ url: "http://localhost:3000/dashboard" })

// Take screenshot
mcp__chrome-devtools__take_screenshot({ filePath: "screenshots/mobile-dashboard.png" })

// Verify: Mobile menu, responsive layout
```

#### Test 9.2: Tablet View (768x1024)
```typescript
// Resize to tablet
mcp__chrome-devtools__resize_page({ width: 768, height: 1024 })

// Navigate to agents page
mcp__chrome-devtools__navigate_page({ url: "http://localhost:3000/dashboard/agents" })

// Take screenshot
mcp__chrome-devtools__take_screenshot({ filePath: "screenshots/tablet-agents.png" })
```

#### Test 9.3: Desktop View (1920x1080)
```typescript
// Resize to desktop
mcp__chrome-devtools__resize_page({ width: 1920, height: 1080 })

// Navigate to admin panel
mcp__chrome-devtools__navigate_page({ url: "http://localhost:3000/dashboard/admin" })

// Take screenshot
mcp__chrome-devtools__take_screenshot({ filePath: "screenshots/desktop-admin.png" })
```

### Test Suite 10: Performance Monitoring

#### Test 10.1: Page Load Performance
```typescript
// Start performance trace
mcp__chrome-devtools__performance_start_trace({ reload: true, autoStop: true })

// Navigate to dashboard
mcp__chrome-devtools__navigate_page({ url: "http://localhost:3000/dashboard" })

// Stop trace (if not auto-stopped)
mcp__chrome-devtools__performance_stop_trace()

// Analyze results
// Expected: FCP < 1s, TTI < 2s, LCP < 2.5s
```

#### Test 10.2: Console Error Check
```typescript
// Navigate through all pages
const pages = [
  "http://localhost:3000",
  "http://localhost:3000/dashboard",
  "http://localhost:3000/dashboard/agents",
  "http://localhost:3000/dashboard/admin"
]

for (const page of pages) {
  mcp__chrome-devtools__navigate_page({ url: page })
  const consoleMessages = mcp__chrome-devtools__list_console_messages()

  // Verify: No error-level console messages
}
```

---

## Test Execution Checklist

### Pre-Test Setup
- [ ] Docker containers running (postgres, redis)
- [ ] Database migrations applied
- [ ] Backend server running on :8080
- [ ] Frontend dev server running on :3000
- [ ] OAuth credentials configured (for SSO tests)

### Backend Tests
- [ ] Suite 1: Health & Infrastructure (3 tests)
- [ ] Suite 2: Authentication (3 tests)
- [ ] Suite 3: Agent Management (5 tests)
- [ ] Suite 4: API Key Management (3 tests)
- [ ] Suite 5: Trust Scoring (2 tests)
- [ ] Suite 6: Admin Endpoints (4 tests)
- [ ] Suite 7: Compliance (2 tests)

### Frontend E2E Tests (Chrome DevTools MCP)
- [ ] Suite 1: Landing Page (2 tests)
- [ ] Suite 2: SSO Authentication (2 tests)
- [ ] Suite 3: Dashboard (2 tests)
- [ ] Suite 4: Agent Registration (4 tests)
- [ ] Suite 5: API Key Management (5 tests)
- [ ] Suite 6: Trust Score Display (2 tests)
- [ ] Suite 7: Admin Dashboard (6 tests)
- [ ] Suite 8: Error Handling (2 tests)
- [ ] Suite 9: Responsive Design (3 tests)
- [ ] Suite 10: Performance (2 tests)

### Post-Test Verification
- [ ] All tests passed
- [ ] No console errors
- [ ] Performance metrics met
- [ ] Screenshots captured for visual verification
- [ ] Bug reports created for any failures

---

## Automated Test Implementation

### Backend Integration Tests (Go)

Location: `apps/backend/tests/integration/`

```go
// Example: apps/backend/tests/integration/health_test.go
package integration

import (
    "net/http"
    "testing"

    "github.com/stretchr/testify/assert"
)

func TestHealthEndpoint(t *testing.T) {
    resp, err := http.Get("http://localhost:8080/api/v1/health")
    assert.NoError(t, err)
    assert.Equal(t, http.StatusOK, resp.StatusCode)

    // Parse and verify response body
    // ...
}
```

### Frontend E2E Tests (Playwright)

Location: `apps/web/tests/e2e/`

```typescript
// Example: apps/web/tests/e2e/agent-registration.spec.ts
import { test, expect } from '@playwright/test';

test.describe('Agent Registration', () => {
  test('should register new agent successfully', async ({ page }) => {
    await page.goto('http://localhost:3000/dashboard/agents/new');

    // Fill form
    await page.fill('[name="name"]', 'test-agent');
    await page.fill('[name="display_name"]', 'Test Agent');
    // ...

    // Submit
    await page.click('button[type="submit"]');

    // Verify success
    await expect(page).toHaveURL(/\/dashboard\/agents/);
  });
});
```

---

## Performance Benchmarks

### Backend API Targets
- p50: < 50ms
- p95: < 100ms
- p99: < 500ms

### Frontend Metrics Targets
- First Contentful Paint: < 1s
- Time to Interactive: < 2s
- Largest Contentful Paint: < 2.5s
- Lighthouse Score: > 90

### Load Testing
- Concurrent Users: 10,000
- Requests per Day: 1,000,000
- Database Queries: < 50ms
- Cache Hit Rate: > 80%

---

## Bug Reporting Template

When tests fail, use this template:

```markdown
### Bug Report

**Test**: [Test Suite Name] - [Test Name]
**Severity**: Critical / High / Medium / Low
**Status**: New

**Description**:
[Clear description of the bug]

**Steps to Reproduce**:
1. Navigate to...
2. Click...
3. Fill...
4. Observe error

**Expected Behavior**:
[What should happen]

**Actual Behavior**:
[What actually happened]

**Screenshots**:
[Attach screenshots from Chrome DevTools MCP]

**Console Errors**:
```
[Console error messages]
```

**Network Requests**:
[Failed API calls, status codes]

**Environment**:
- Backend: Running on :8080
- Frontend: Running on :3000
- Browser: Chrome (via DevTools MCP)
```

---

## Success Criteria

### All Tests Pass
- ✅ All backend integration tests passing
- ✅ All frontend E2E tests passing
- ✅ Performance benchmarks met
- ✅ No console errors
- ✅ Responsive design verified

### Documentation Complete
- ✅ Test plan documented
- ✅ Test results recorded
- ✅ Screenshots captured
- ✅ Bug reports filed
- ✅ Performance metrics tracked

### Ready for Launch
- ✅ All critical bugs fixed
- ✅ User flows working end-to-end
- ✅ Security tested
- ✅ Performance optimized
- ✅ Documentation updated
