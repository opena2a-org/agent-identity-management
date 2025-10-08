# 🧪 End-to-End Testing Guide: SDK Tokens Dashboard

**Purpose**: Comprehensive testing of the SDK Tokens Dashboard feature
**Branch**: `feature/sdk-tokens-dashboard`
**Estimated Time**: 30-45 minutes
**Date**: October 8, 2025

---

## 📋 Prerequisites

Before starting, ensure you have:
- ✅ Backend running on `http://localhost:8080`
- ✅ Frontend running on `http://localhost:3000`
- ✅ PostgreSQL database running
- ✅ Valid authentication token
- ✅ Chrome DevTools MCP available

---

## 🎯 Test Overview

This E2E test will verify:
1. **SDK Token Generation** - Tokens created on SDK download
2. **Dashboard UI** - Page loads and displays correctly
3. **Token List** - All token metadata displays correctly
4. **Single Token Revocation** - Can revoke individual tokens
5. **Bulk Revocation** - Can revoke all tokens at once
6. **Filtering** - Show/hide revoked tokens
7. **Navigation** - Sidebar links work correctly
8. **API Integration** - All endpoints respond correctly

---

## 🚀 Phase 1: Setup & Authentication

### Step 1.1: Verify Services Running

```bash
# Check backend
curl http://localhost:8080/api/v1/auth/providers

# Expected: List of OAuth providers (Google, Microsoft, Okta)

# Check frontend
curl http://localhost:3000

# Expected: HTML response (Next.js app)

# Check database
psql -U aim_user -d aim_dev -c "SELECT COUNT(*) FROM sdk_tokens;"

# Expected: Number of tokens in database
```

### Step 1.2: Get Authentication Token

**Option A: Use Existing Token**
```bash
# If you have a token saved
export AUTH_TOKEN="your-jwt-token-here"
```

**Option B: Login via Chrome DevTools**
```bash
# Use Chrome DevTools MCP to login and extract token
# (Instructions below in Phase 2)
```

### Step 1.3: Verify Token Works

```bash
curl -X GET http://localhost:8080/api/v1/auth/me \
  -H "Authorization: Bearer $AUTH_TOKEN"

# Expected Output:
{
  "id": "uuid",
  "email": "user@example.com",
  "name": "Test User",
  "role": "admin",
  "organization_id": "uuid"
}
```

**✅ Success Criteria**: API returns user info without errors

---

## 🎨 Phase 2: UI Testing with Chrome DevTools MCP

### Step 2.1: Navigate to Dashboard

```javascript
// Use Chrome DevTools MCP
mcp__chrome-devtools__navigate_page({
  url: "http://localhost:3000/dashboard/sdk-tokens"
})
```

**Expected**: Page loads without 404 error

### Step 2.2: Take Initial Snapshot

```javascript
mcp__chrome-devtools__take_snapshot()
```

**Verify in Snapshot**:
- [ ] Page title: "SDK Tokens"
- [ ] Subtitle: "Manage your SDK authentication tokens..."
- [ ] Three statistics cards visible
- [ ] Navigation sidebar with "SDK Tokens" link highlighted

### Step 2.3: Check Console for Errors

```javascript
mcp__chrome-devtools__list_console_messages()
```

**Expected**: No errors (warnings are okay)

**❌ If Errors Found**:
- Note the error message
- Check if it's a network error (API down?)
- Check if it's a JavaScript error (bug in code?)

### Step 2.4: Verify Statistics Cards

```javascript
mcp__chrome-devtools__take_screenshot({
  format: "png"
})
```

**Check Screenshot For**:
- [ ] "Active Tokens" card with green shield icon
- [ ] "Total Usage" card with blue key icon
- [ ] "Revoked Tokens" card with red trash icon
- [ ] Numbers display correctly (not NaN or undefined)

---

## 🔑 Phase 3: Generate Test Token

### Step 3.1: Navigate to SDK Download Page

```javascript
mcp__chrome-devtools__navigate_page({
  url: "http://localhost:3000/dashboard/sdk"
})
```

### Step 3.2: Take Snapshot of SDK Page

```javascript
mcp__chrome-devtools__take_snapshot()
```

**Find in Snapshot**:
- Download SDK button UID (e.g., `uid="button-123"`)

### Step 3.3: Click Download Button

```javascript
mcp__chrome-devtools__click({
  uid: "button-123"  // Replace with actual UID from snapshot
})
```

**Expected**: SDK download starts

### Step 3.4: Monitor Network Request

```javascript
mcp__chrome-devtools__list_network_requests({
  resourceTypes: ["xhr", "fetch"],
  pageSize: 10
})
```

**Verify**:
- [ ] Request to `/api/v1/sdk/download`
- [ ] Status: 200 OK
- [ ] Response type: application/zip

### Step 3.5: Navigate Back to SDK Tokens Page

```javascript
mcp__chrome-devtools__navigate_page({
  url: "http://localhost:3000/dashboard/sdk-tokens"
})
```

### Step 3.6: Wait for Page Load

```javascript
mcp__chrome-devtools__wait_for({
  text: "Active Tokens",
  timeout: 5000
})
```

### Step 3.7: Verify New Token Appears

```javascript
mcp__chrome-devtools__take_snapshot()
```

**Check Snapshot**:
- [ ] New token card visible
- [ ] Token ID displayed (format: uuid or similar)
- [ ] Status badge shows "Active"
- [ ] IP address displayed
- [ ] User agent displayed
- [ ] Usage count shows 0 or low number
- [ ] "Revoke" button visible

**✅ Success Criteria**: New token appears in list with correct metadata

---

## 🔍 Phase 4: Token Display Verification

### Step 4.1: Inspect Token Card Details

From the snapshot in Step 3.7, verify each token card shows:

**Required Fields**:
- [ ] Device Name (or "Unknown Device")
- [ ] Token ID (format: `Token ID: xyz-abc-123`)
- [ ] Status Badge (Active/Revoked/Expired)
- [ ] IP Address with MapPin icon
- [ ] User Agent with Monitor icon
- [ ] Last Used timestamp with Clock icon
- [ ] Usage Count with Key icon
- [ ] Created timestamp ("Created X ago")
- [ ] Expires timestamp ("Expires X ago")

**Interactive Elements**:
- [ ] "Revoke" button visible (if token is active)
- [ ] "Revoke" button disabled (if token is revoked)

### Step 4.2: Verify Statistics Update

After generating new token, check statistics cards:

```javascript
mcp__chrome-devtools__take_snapshot()
```

**Expected**:
- [ ] "Active Tokens" count increased by 1
- [ ] "Total Usage" shows cumulative usage
- [ ] "Revoked Tokens" unchanged (or shows count)

---

## 🗑️ Phase 5: Single Token Revocation

### Step 5.1: Take Snapshot and Find Revoke Button

```javascript
mcp__chrome-devtools__take_snapshot()
```

**Find in Snapshot**:
- Revoke button UID for the token you want to revoke
- Example: `uid="revoke-button-xyz"`

### Step 5.2: Click Revoke Button

```javascript
mcp__chrome-devtools__click({
  uid: "revoke-button-xyz"  // Replace with actual UID
})
```

**Expected**: Revocation dialog appears

### Step 5.3: Verify Dialog Appeared

```javascript
mcp__chrome-devtools__wait_for({
  text: "Revoke SDK Token",
  timeout: 3000
})
```

### Step 5.4: Take Dialog Snapshot

```javascript
mcp__chrome-devtools__take_snapshot()
```

**Verify Dialog Contains**:
- [ ] Title: "Revoke SDK Token"
- [ ] Description: "This will immediately invalidate..."
- [ ] Textarea for revocation reason
- [ ] Cancel button
- [ ] "Revoke Token" button (should be disabled until reason entered)

### Step 5.5: Fill Revocation Reason

```javascript
mcp__chrome-devtools__fill({
  uid: "reason-textarea-uid",  // Replace with actual UID
  value: "Testing revocation workflow - E2E test"
})
```

### Step 5.6: Take Snapshot After Filling

```javascript
mcp__chrome-devtools__take_snapshot()
```

**Verify**:
- [ ] Textarea contains the text
- [ ] "Revoke Token" button is now enabled

### Step 5.7: Click Revoke Token Button

Find the button UID from snapshot, then:

```javascript
mcp__chrome-devtools__click({
  uid: "revoke-token-button-uid"  // Replace with actual UID
})
```

### Step 5.8: Monitor Network Request

```javascript
mcp__chrome-devtools__list_network_requests({
  resourceTypes: ["xhr", "fetch"],
  pageSize: 10
})
```

**Verify**:
- [ ] POST request to `/api/v1/users/me/sdk-tokens/{id}/revoke`
- [ ] Status: 200 OK
- [ ] Request body contains: `{"reason": "Testing revocation..."}`

### Step 5.9: Verify Token Status Changed

```javascript
mcp__chrome-devtools__wait_for({
  text: "Revoked",
  timeout: 5000
})
```

### Step 5.10: Take Final Snapshot

```javascript
mcp__chrome-devtools__take_snapshot()
```

**Verify**:
- [ ] Token status badge changed to "Revoked" (red color)
- [ ] Revoke button no longer visible
- [ ] Token card has reduced opacity
- [ ] Revocation reason displayed at bottom
- [ ] "Active Tokens" count decreased by 1
- [ ] "Revoked Tokens" count increased by 1

**✅ Success Criteria**: Token successfully revoked and UI updated

---

## 🗂️ Phase 6: Revoked Token Filtering

### Step 6.1: Count Current Tokens

```javascript
mcp__chrome-devtools__take_snapshot()
```

**Note**: Count how many token cards are visible

### Step 6.2: Click "Hide Revoked" Button

Find button UID from snapshot:

```javascript
mcp__chrome-devtools__click({
  uid: "hide-revoked-button-uid"  // Replace with actual UID
})
```

### Step 6.3: Verify Revoked Tokens Hidden

```javascript
mcp__chrome-devtools__take_snapshot()
```

**Verify**:
- [ ] Revoked token cards no longer visible
- [ ] Only active tokens displayed
- [ ] Button text changed to "Show Revoked"
- [ ] Token count decreased

### Step 6.4: Click "Show Revoked" Button

```javascript
mcp__chrome-devtools__click({
  uid: "show-revoked-button-uid"  // Replace with actual UID
})
```

### Step 6.5: Verify Revoked Tokens Reappear

```javascript
mcp__chrome-devtools__take_snapshot()
```

**Verify**:
- [ ] Revoked tokens visible again
- [ ] Button text changed back to "Hide Revoked"
- [ ] All tokens displayed (active + revoked)

**✅ Success Criteria**: Filtering toggles correctly

---

## 💥 Phase 7: Bulk Revocation

### Step 7.1: Generate Multiple Test Tokens

**Skip this if you already have 2+ active tokens**

```bash
# Download SDK multiple times to generate tokens
for i in {1..3}; do
  curl -X GET http://localhost:8080/api/v1/sdk/download \
    -H "Authorization: Bearer $AUTH_TOKEN" \
    -o /tmp/sdk-test-$i.zip
  sleep 1
done
```

### Step 7.2: Refresh Page

```javascript
mcp__chrome-devtools__navigate_page({
  url: "http://localhost:3000/dashboard/sdk-tokens"
})
```

### Step 7.3: Verify Multiple Active Tokens

```javascript
mcp__chrome-devtools__take_snapshot()
```

**Verify**:
- [ ] "Active Tokens" count shows 2 or more
- [ ] Multiple token cards visible
- [ ] "Revoke All" button visible in top right

### Step 7.4: Click "Revoke All" Button

```javascript
mcp__chrome-devtools__click({
  uid: "revoke-all-button-uid"  // Replace with actual UID
})
```

### Step 7.5: Verify Security Warning Dialog

```javascript
mcp__chrome-devtools__wait_for({
  text: "Revoke All SDK Tokens",
  timeout: 3000
})
```

### Step 7.6: Take Dialog Snapshot

```javascript
mcp__chrome-devtools__take_snapshot()
```

**Verify Dialog Contains**:
- [ ] Title: "Revoke All SDK Tokens"
- [ ] Warning message: "This will immediately invalidate all X active tokens"
- [ ] Red alert box with AlertCircle icon
- [ ] Alert text: "All applications using SDK tokens will immediately lose access..."
- [ ] Textarea for reason
- [ ] Cancel button
- [ ] "Revoke All X Tokens" button (disabled until reason entered)

### Step 7.7: Fill Revocation Reason

```javascript
mcp__chrome-devtools__fill({
  uid: "reason-all-textarea-uid",  // Replace with actual UID
  value: "E2E testing - bulk revocation test"
})
```

### Step 7.8: Click "Revoke All X Tokens" Button

```javascript
mcp__chrome-devtools__click({
  uid: "revoke-all-confirm-button-uid"  // Replace with actual UID
})
```

### Step 7.9: Monitor Network Request

```javascript
mcp__chrome-devtools__list_network_requests({
  resourceTypes: ["xhr", "fetch"],
  pageSize: 10
})
```

**Verify**:
- [ ] POST request to `/api/v1/users/me/sdk-tokens/revoke-all`
- [ ] Status: 200 OK
- [ ] Request body: `{"reason": "E2E testing..."}`

### Step 7.10: Verify All Tokens Revoked

```javascript
mcp__chrome-devtools__wait_for({
  text: "Active Tokens",
  timeout: 5000
})
```

### Step 7.11: Take Final Snapshot

```javascript
mcp__chrome-devtools__take_snapshot()
```

**Verify**:
- [ ] "Active Tokens" count is now 0
- [ ] "Revoked Tokens" count shows all tokens
- [ ] All token cards show "Revoked" badge
- [ ] No "Revoke" buttons visible
- [ ] All cards have reduced opacity
- [ ] "Revoke All" button no longer visible (no active tokens)

**✅ Success Criteria**: All tokens successfully revoked

---

## 🧭 Phase 8: Navigation Testing

### Step 8.1: Test Sidebar Link

```javascript
mcp__chrome-devtools__navigate_page({
  url: "http://localhost:3000/dashboard"
})
```

### Step 8.2: Take Sidebar Snapshot

```javascript
mcp__chrome-devtools__take_snapshot()
```

**Find in Snapshot**:
- [ ] "SDK Tokens" link with Lock icon
- [ ] Located after "Download SDK" link
- Note the UID of the link

### Step 8.3: Click SDK Tokens Link

```javascript
mcp__chrome-devtools__click({
  uid: "sdk-tokens-link-uid"  // Replace with actual UID
})
```

**Expected**: Navigates to `/dashboard/sdk-tokens`

### Step 8.4: Verify Active State

```javascript
mcp__chrome-devtools__take_snapshot()
```

**Verify**:
- [ ] URL is `/dashboard/sdk-tokens`
- [ ] "SDK Tokens" link highlighted in sidebar
- [ ] Dashboard page displayed

### Step 8.5: Test SDK Download Page Link

```javascript
mcp__chrome-devtools__navigate_page({
  url: "http://localhost:3000/dashboard/sdk"
})
```

### Step 8.6: Find Security Notice

```javascript
mcp__chrome-devtools__take_snapshot()
```

**Verify**:
- [ ] Blue security notice box visible
- [ ] Shield icon present
- [ ] Text: "Security Best Practices"
- [ ] Link: "Manage SDK Tokens →" with Lock icon
- [ ] Note the link UID

### Step 8.7: Click "Manage SDK Tokens" Link

```javascript
mcp__chrome-devtools__click({
  uid: "manage-tokens-link-uid"  // Replace with actual UID
})
```

**Expected**: Navigates to `/dashboard/sdk-tokens`

### Step 8.8: Verify Navigation

```javascript
mcp__chrome-devtools__take_snapshot()
```

**Verify**:
- [ ] URL changed to `/dashboard/sdk-tokens`
- [ ] SDK Tokens page loaded

**✅ Success Criteria**: All navigation links work correctly

---

## 🔌 Phase 9: API Integration Testing

### Step 9.1: Test List Tokens Endpoint

```bash
curl -X GET "http://localhost:8080/api/v1/users/me/sdk-tokens?include_revoked=true" \
  -H "Authorization: Bearer $AUTH_TOKEN" \
  -H "Content-Type: application/json"
```

**Expected Response**:
```json
{
  "tokens": [
    {
      "id": "uuid",
      "userId": "uuid",
      "organizationId": "uuid",
      "tokenId": "unique-token-id",
      "deviceName": "string or null",
      "ipAddress": "192.168.1.1",
      "userAgent": "Mozilla/5.0...",
      "lastUsedAt": "2025-10-08T10:00:00Z",
      "lastIpAddress": "192.168.1.1",
      "usageCount": 5,
      "createdAt": "2025-10-08T09:00:00Z",
      "expiresAt": "2026-01-06T09:00:00Z",
      "revokedAt": "2025-10-08T10:30:00Z",
      "revokeReason": "Testing revocation",
      "metadata": {}
    }
  ]
}
```

**Verify**:
- [ ] Status: 200 OK
- [ ] Response contains "tokens" array
- [ ] Each token has all required fields
- [ ] No authentication errors

### Step 9.2: Test Active Token Count Endpoint

```bash
curl -X GET "http://localhost:8080/api/v1/users/me/sdk-tokens/count" \
  -H "Authorization: Bearer $AUTH_TOKEN"
```

**Expected Response**:
```json
{
  "count": 0
}
```

**Verify**:
- [ ] Status: 200 OK
- [ ] Count matches number of active tokens

### Step 9.3: Test Revoke Token Endpoint

First, generate a new token:
```bash
curl -X GET http://localhost:8080/api/v1/sdk/download \
  -H "Authorization: Bearer $AUTH_TOKEN" \
  -o /tmp/test-sdk.zip
```

Get the token ID from the list endpoint, then:

```bash
TOKEN_ID="uuid-from-list-response"

curl -X POST "http://localhost:8080/api/v1/users/me/sdk-tokens/$TOKEN_ID/revoke" \
  -H "Authorization: Bearer $AUTH_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"reason": "API test - single token revocation"}'
```

**Expected Response**:
```json
{
  "success": true,
  "message": "Token revoked successfully"
}
```

**Verify**:
- [ ] Status: 200 OK
- [ ] Response indicates success
- [ ] Token marked as revoked in database

### Step 9.4: Test Revoke All Endpoint

Generate 2-3 new tokens, then:

```bash
curl -X POST "http://localhost:8080/api/v1/users/me/sdk-tokens/revoke-all" \
  -H "Authorization: Bearer $AUTH_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"reason": "API test - bulk revocation"}'
```

**Expected Response**:
```json
{
  "success": true,
  "message": "All tokens revoked successfully",
  "count": 3
}
```

**Verify**:
- [ ] Status: 200 OK
- [ ] Response indicates success
- [ ] All tokens marked as revoked

### Step 9.5: Verify Database State

```bash
psql -U aim_user -d aim_dev -c "
SELECT
  id,
  token_id,
  usage_count,
  revoked_at IS NOT NULL as is_revoked,
  revoke_reason
FROM sdk_tokens
WHERE user_id = (SELECT id FROM users WHERE email = 'your-email@example.com')
ORDER BY created_at DESC
LIMIT 5;
"
```

**Verify**:
- [ ] All expected tokens present
- [ ] Revocation status matches UI
- [ ] Revocation reasons stored correctly
- [ ] Usage counts reasonable

**✅ Success Criteria**: All API endpoints working correctly

---

## 🎨 Phase 10: Responsive Design Testing

### Step 10.1: Test Mobile Viewport (375x667)

```javascript
mcp__chrome-devtools__resize_page({
  width: 375,
  height: 667
})

mcp__chrome-devtools__navigate_page({
  url: "http://localhost:3000/dashboard/sdk-tokens"
})

mcp__chrome-devtools__take_screenshot({
  format: "png"
})
```

**Verify in Screenshot**:
- [ ] Statistics cards stack vertically
- [ ] Token cards display correctly
- [ ] No horizontal scrolling
- [ ] Text readable (not truncated badly)
- [ ] Buttons accessible

### Step 10.2: Test Tablet Viewport (768x1024)

```javascript
mcp__chrome-devtools__resize_page({
  width: 768,
  height: 1024
})

mcp__chrome-devtools__take_screenshot({
  format: "png"
})
```

**Verify**:
- [ ] Statistics cards in 2-column or 3-column layout
- [ ] Token cards display well
- [ ] Sidebar behavior appropriate

### Step 10.3: Test Desktop Viewport (1920x1080)

```javascript
mcp__chrome-devtools__resize_page({
  width: 1920,
  height: 1080
})

mcp__chrome-devtools__take_screenshot({
  format: "png"
})
```

**Verify**:
- [ ] Statistics cards in 3-column layout
- [ ] Token cards not too wide
- [ ] Proper use of space
- [ ] Content centered appropriately

**✅ Success Criteria**: UI looks good on all viewport sizes

---

## 🐛 Phase 11: Error Handling Testing

### Step 11.1: Test Unauthenticated Access

```bash
# Navigate to page without token
curl -X GET http://localhost:8080/api/v1/users/me/sdk-tokens

# Expected: 401 Unauthorized
```

**Verify**:
- [ ] API returns 401
- [ ] Frontend redirects to login (if applicable)

### Step 11.2: Test Invalid Token ID

```bash
curl -X POST "http://localhost:8080/api/v1/users/me/sdk-tokens/invalid-uuid/revoke" \
  -H "Authorization: Bearer $AUTH_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"reason": "Testing error handling"}'

# Expected: 400 Bad Request or 404 Not Found
```

**Verify**:
- [ ] API returns appropriate error
- [ ] Error message clear

### Step 11.3: Test Empty Revocation Reason

```bash
curl -X POST "http://localhost:8080/api/v1/users/me/sdk-tokens/$TOKEN_ID/revoke" \
  -H "Authorization: Bearer $AUTH_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"reason": ""}'

# Expected: 400 Bad Request
```

**Verify**:
- [ ] API rejects empty reason
- [ ] Frontend validation prevents submission

### Step 11.4: Test Network Error Simulation

In Chrome DevTools:
```javascript
// Simulate offline
mcp__chrome-devtools__emulate_network({
  throttlingOption: "No emulation"  // Then manually go offline
})

// Try to load page
mcp__chrome-devtools__navigate_page({
  url: "http://localhost:3000/dashboard/sdk-tokens"
})
```

**Verify**:
- [ ] Loading spinner appears
- [ ] Error message displays after timeout
- [ ] User can retry

**✅ Success Criteria**: Errors handled gracefully

---

## 📊 Phase 12: Performance Testing

### Step 12.1: Measure Page Load Time

```javascript
mcp__chrome-devtools__navigate_page({
  url: "http://localhost:3000/dashboard/sdk-tokens"
})

// Check performance in console
mcp__chrome-devtools__list_console_messages()
```

**Acceptance Criteria**:
- [ ] Page loads in < 2 seconds
- [ ] No memory leaks
- [ ] No console warnings

### Step 12.2: Test with Many Tokens

Generate 20+ tokens:
```bash
for i in {1..20}; do
  curl -X GET http://localhost:8080/api/v1/sdk/download \
    -H "Authorization: Bearer $AUTH_TOKEN" \
    -o /tmp/sdk-$i.zip
  sleep 0.5
done
```

Navigate to page:
```javascript
mcp__chrome-devtools__navigate_page({
  url: "http://localhost:3000/dashboard/sdk-tokens"
})

mcp__chrome-devtools__take_screenshot()
```

**Verify**:
- [ ] Page still loads quickly
- [ ] Scrolling smooth
- [ ] All tokens render correctly
- [ ] No UI lag

**✅ Success Criteria**: Performs well with many tokens

---

## ✅ Final Verification Checklist

### **Functionality** (All Must Pass)
- [ ] Token list displays correctly
- [ ] Statistics cards show accurate data
- [ ] Single token revocation works
- [ ] Bulk revocation works
- [ ] Filter toggle works (show/hide revoked)
- [ ] Navigation links work
- [ ] Security notice displays on SDK page
- [ ] All API endpoints respond correctly

### **UI/UX** (All Must Pass)
- [ ] No console errors
- [ ] Loading states display
- [ ] Error messages display when needed
- [ ] Confirmation dialogs work
- [ ] Responsive design works (mobile, tablet, desktop)
- [ ] Icons display correctly
- [ ] Colors and styling match AIVF aesthetics
- [ ] Text readable and not truncated

### **Data Integrity** (All Must Pass)
- [ ] Token metadata accurate
- [ ] Revocation status persists
- [ ] Usage counts update
- [ ] Timestamps correct
- [ ] Database state matches UI

### **Security** (All Must Pass)
- [ ] Authentication required
- [ ] Revocation reasons mandatory
- [ ] Tokens properly invalidated
- [ ] No sensitive data leaked in console
- [ ] HTTPS recommended in production

---

## 📝 Test Results Template

```markdown
## E2E Test Results: SDK Tokens Dashboard

**Date**: [Date]
**Tester**: [Name/Claude Session ID]
**Branch**: feature/sdk-tokens-dashboard
**Duration**: [Time taken]

### Summary
- Total Tests: 60+
- Passed: XX
- Failed: XX
- Skipped: XX

### Phase Results

#### Phase 1: Setup & Authentication
- [ ] PASS / FAIL - Services running
- [ ] PASS / FAIL - Authentication works
- Notes: ___

#### Phase 2: UI Testing
- [ ] PASS / FAIL - Page loads correctly
- [ ] PASS / FAIL - No console errors
- [ ] PASS / FAIL - Statistics display
- Notes: ___

#### Phase 3: Token Generation
- [ ] PASS / FAIL - SDK download creates token
- [ ] PASS / FAIL - Token appears in list
- Notes: ___

#### Phase 4: Token Display
- [ ] PASS / FAIL - All metadata displays
- [ ] PASS / FAIL - Formatting correct
- Notes: ___

#### Phase 5: Single Revocation
- [ ] PASS / FAIL - Revocation dialog works
- [ ] PASS / FAIL - Token revoked successfully
- [ ] PASS / FAIL - UI updates correctly
- Notes: ___

#### Phase 6: Filtering
- [ ] PASS / FAIL - Show/hide revoked works
- Notes: ___

#### Phase 7: Bulk Revocation
- [ ] PASS / FAIL - Revoke all works
- [ ] PASS / FAIL - Warning displays
- [ ] PASS / FAIL - All tokens revoked
- Notes: ___

#### Phase 8: Navigation
- [ ] PASS / FAIL - Sidebar link works
- [ ] PASS / FAIL - SDK page link works
- Notes: ___

#### Phase 9: API Integration
- [ ] PASS / FAIL - List endpoint
- [ ] PASS / FAIL - Count endpoint
- [ ] PASS / FAIL - Revoke endpoint
- [ ] PASS / FAIL - Revoke all endpoint
- Notes: ___

#### Phase 10: Responsive Design
- [ ] PASS / FAIL - Mobile
- [ ] PASS / FAIL - Tablet
- [ ] PASS / FAIL - Desktop
- Notes: ___

#### Phase 11: Error Handling
- [ ] PASS / FAIL - Auth errors
- [ ] PASS / FAIL - Invalid input
- [ ] PASS / FAIL - Network errors
- Notes: ___

#### Phase 12: Performance
- [ ] PASS / FAIL - Load time acceptable
- [ ] PASS / FAIL - Handles many tokens
- Notes: ___

### Issues Found

1. **Issue**: [Description]
   - **Severity**: Critical / High / Medium / Low
   - **Location**: [File:line or UI location]
   - **Steps to Reproduce**: [Steps]
   - **Expected**: [What should happen]
   - **Actual**: [What happened]

2. [Add more issues...]

### Recommendations

1. [Recommendation 1]
2. [Recommendation 2]
3. [Recommendation 3]

### Overall Assessment

**Status**: ✅ PASS / ❌ FAIL / ⚠️ CONDITIONAL PASS

**Comments**:
[Overall comments and conclusion]

**Approved for Merge**: YES / NO / WITH CHANGES
```

---

## 🚀 Post-Testing Actions

### If All Tests Pass ✅

1. **Document Results**
   ```bash
   # Save test results
   cp test-results.md TEST_RESULTS_$(date +%Y%m%d).md
   git add TEST_RESULTS_*.md
   git commit -m "test: SDK tokens dashboard E2E test results"
   ```

2. **Create Pull Request**
   - Go to GitHub PR URL
   - Add test results to PR description
   - Request review from team

3. **Merge to Main**
   ```bash
   git checkout main
   git merge feature/sdk-tokens-dashboard
   git push origin main
   ```

### If Tests Fail ❌

1. **Document Issues**
   - Take screenshots of failures
   - Save console logs
   - Note exact error messages

2. **Create Bug Report**
   ```bash
   # Create issue template
   echo "Bug found during E2E testing" > BUG_REPORT.md
   # Add details
   ```

3. **Fix Issues**
   - Create bug fix branch
   - Implement fixes
   - Re-run tests

4. **Retest**
   - Run this E2E test again
   - Verify fixes work
   - Update PR

---

## 📚 Additional Resources

- **Backend Endpoints**: See `SECURITY.md` for API documentation
- **Frontend Components**: Check `apps/web/components/ui/` for reusable components
- **Test Data**: Use `scripts/seed-test-data.sql` for sample data
- **Troubleshooting**: See `SDK_TOKENS_DASHBOARD_COMPLETE.md` for common issues

---

## 🎯 Success Criteria Summary

**Must Pass All**:
- ✅ Page loads without errors
- ✅ All CRUD operations work
- ✅ UI matches design specifications
- ✅ No console errors or warnings
- ✅ Responsive on all devices
- ✅ API endpoints return correct data
- ✅ Error handling works
- ✅ Security validations in place
- ✅ Database state consistent with UI
- ✅ Navigation flows work

**Overall Goal**: Prove that the SDK Tokens Dashboard is production-ready and provides complete token management functionality.

---

**End of E2E Testing Guide**

Good luck with testing! 🚀
