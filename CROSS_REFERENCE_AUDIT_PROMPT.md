# Complete Frontend ↔ Backend Cross-Reference Audit

## Your Mission

Create a **comprehensive, verified cross-reference** mapping ALL backend endpoints to frontend UI pages using **Chrome DevTools MCP** to actually verify what exists in the production application.

## ⚠️ Critical Requirements

### 1. **ALWAYS Use Chrome DevTools for Verification**
- **DO NOT** just read frontend code - code can lie!
- **ALWAYS** navigate to the production URL and verify UI actually exists
- **ALWAYS** check network requests to see which endpoints are called
- **ALWAYS** take snapshots/screenshots to confirm UI elements

### 2. **Production URL**
```
https://aim-prod-frontend.graypebble-c7e67ab8.canadacentral.azurecontainerapps.io
```

**Login Credentials**:
- Email: `admin@opena2a.org`
- Password: `AIM2025!Secure`

### 3. **Backend Source of Truth**
```
/Users/decimai/workspace/agent-identity-management/apps/backend/cmd/server/main.go
```

This file contains ALL 116 backend endpoints (lines 700-1100).

### 4. **Endpoint Inventory Reference**
```
/Users/decimai/workspace/agent-identity-management/ENDPOINT_INVENTORY.md
```

Complete list of all 116 endpoints organized by category.

---

## Step-by-Step Process

### Phase 1: Login and Explore (MANDATORY)
1. Use Chrome DevTools to navigate to production URL
2. Login with admin credentials
3. Take snapshot of dashboard
4. List all navigation menu items
5. Document all accessible pages

### Phase 2: Map Each Backend Endpoint Category
For EACH of the 24 endpoint categories in `ENDPOINT_INVENTORY.md`:

#### Example: SDK Download Category
```markdown
## Category: SDK Download (1 endpoint)

**Backend Endpoint**: `GET /api/v1/sdk/download`

**Chrome DevTools Verification**:
1. Navigate to: https://...azurecontainerapps.io/dashboard/sdk
2. Take snapshot - confirm page exists
3. Click "Download SDK" button
4. Monitor network tab - confirm GET /api/v1/sdk/download is called
5. Screenshot of network request

**Frontend Mapping**:
- ✅ UI Page: `/dashboard/sdk`
- ✅ API Method: `downloadSDK()` in api.ts (line X)
- ✅ Component: `apps/web/app/dashboard/sdk/page.tsx`
- ✅ Network Request: Confirmed in Chrome DevTools

**Status**: VERIFIED - UI exists and endpoint is called
```

### Phase 3: Identify Gaps

#### A. Backend Endpoints WITHOUT UI
List endpoints that exist in backend but have NO UI:
```markdown
### Backend Endpoints Not Exposed in UI

| Endpoint | Reason |
|----------|--------|
| `POST /api/v1/agents/:id/capabilities` | Likely SDK-only |
| `GET /api/v1/compliance/metrics` | Admin feature not yet built |
```

#### B. Frontend Pages WITHOUT Backend
List UI pages that call non-existent endpoints:
```markdown
### Frontend Dead Code

| Frontend Method | Status |
|----------------|--------|
| `approveVerification()` | Backend endpoint doesn't exist |
```

### Phase 4: Create Final Cross-Reference Document

**Output File**: `FRONTEND_BACKEND_CROSS_REFERENCE.md`

**Structure**:
```markdown
# AIM Frontend ↔ Backend Complete Cross-Reference
**Verified**: [Date]
**Method**: Chrome DevTools + Code Analysis
**Production URL**: https://aim-prod-frontend.graypebble-c7e67ab8.canadacentral.azurecontainerapps.io

## Summary
- Total Backend Endpoints: 116
- Endpoints with UI: X
- Endpoints without UI: Y
- Frontend Dead Code: Z methods

## Verification Methodology
All UI pages verified using Chrome DevTools MCP by:
1. Logging into production application
2. Navigating to each page
3. Monitoring network requests
4. Confirming endpoints are called

---

## Category-by-Category Mapping

### 1. Health & Status (3 endpoints)
[Include Chrome DevTools verification for each]

### 2. SDK API (4 endpoints)
[Include Chrome DevTools verification for each]

... [Continue for all 24 categories]

---

## 🚨 Critical Findings

### Backend Endpoints NOT in UI
[List with reasons]

### Frontend Dead Code
[List methods to remove]

### Orphaned Endpoints
[Endpoints that exist but are never called]
```

---

## Chrome DevTools Workflow (CRITICAL!)

### For Each Endpoint Category:

1. **Navigate to suspected UI page**:
```typescript
mcp__chrome-devtools__navigate_page({
  url: "https://aim-prod-frontend.graypebble-c7e67ab8.canadacentral.azurecontainerapps.io/dashboard/sdk"
})
```

2. **Take snapshot to see page structure**:
```typescript
mcp__chrome-devtools__take_snapshot()
```

3. **Interact with page** (click buttons, fill forms):
```typescript
mcp__chrome-devtools__click({ uid: "download-button-uid" })
```

4. **Monitor network requests**:
```typescript
mcp__chrome-devtools__list_network_requests({
  resourceTypes: ["xhr", "fetch"],
  output_mode: "content"
})
```

5. **Screenshot as proof**:
```typescript
mcp__chrome-devtools__take_screenshot()
```

---

## Common Pages to Verify

### Dashboard Pages (Navigate and Verify Each):
- `/dashboard` - Main dashboard
- `/dashboard/agents` - Agent list
- `/dashboard/agents/new` - Create agent
- `/dashboard/agents/:id` - Agent details (tabs: Overview, Capabilities, Violations, Key Vault)
- `/dashboard/mcp-servers` - MCP server list
- `/dashboard/mcp-servers/new` - Create MCP
- `/dashboard/mcp-servers/:id` - MCP details
- `/dashboard/sdk` - SDK download (YOU MISSED THIS!)
- `/dashboard/api-keys` - API key management
- `/dashboard/settings` - Organization settings
- `/dashboard/admin/users` - User management
- `/dashboard/admin/capability-requests` - Capability approval
- `/dashboard/admin/audit-logs` - Audit logs
- `/dashboard/admin/alerts` - Security alerts
- `/dashboard/compliance` - Compliance reporting
- `/dashboard/security` - Security dashboard
- `/dashboard/analytics` - Analytics (check for trends tab!)

### Public Pages:
- `/login` - Login page
- `/register` - Registration page
- `/forgot-password` - Password reset

---

## Deliverables

1. **FRONTEND_BACKEND_CROSS_REFERENCE.md** - Complete verified mapping
2. **Screenshots** - Save to `/docs/screenshots/` for each major UI page
3. **Network Request Logs** - Document which endpoints are actually called
4. **Cleanup Recommendations** - List of frontend dead code to remove

---

## Success Criteria

✅ Every single one of 116 backend endpoints is accounted for
✅ Every claim of "UI exists" is verified with Chrome DevTools
✅ Every claim of "no UI" is verified by exhaustive navigation
✅ Network requests confirm which endpoints are actually used
✅ Screenshots prove UI pages exist
✅ Clear recommendations for cleanup

---

## Common Mistakes to Avoid (Learn from Previous Agent!)

❌ **DON'T** just read `api.ts` and assume UI exists
❌ **DON'T** claim "no UI" without checking production
❌ **DON'T** skip Chrome DevTools verification
❌ **DON'T** trust code - verify with actual browser testing

✅ **DO** navigate to every suspected page
✅ **DO** monitor network requests
✅ **DO** take screenshots as proof
✅ **DO** verify in production, not localhost

---

## Example of GOOD Verification

```markdown
### SDK Download Endpoint

**Backend**: `GET /api/v1/sdk/download` (line 753 in main.go)

**Chrome DevTools Verification**:
1. Navigated to: https://...azurecontainerapps.io/dashboard/sdk
2. Page exists - confirmed with snapshot
3. UI shows "Download SDK" button
4. Clicked button
5. Network request captured:
   ```
   GET /api/v1/sdk/download
   Status: 200
   Response: application/octet-stream (SDK ZIP file)
   ```
6. Screenshot saved: docs/screenshots/sdk-download-page.png

**Frontend Code**:
- UI Component: `apps/web/app/dashboard/sdk/page.tsx`
- API Method: `downloadSDK()` in api.ts (line 621)
- Network Request: CONFIRMED in Chrome DevTools

**Status**: ✅ VERIFIED - UI exists and endpoint is actively used
```

---

## Time Estimate

- Phase 1 (Login & Explore): 30 minutes
- Phase 2 (Map 24 categories): 3-4 hours
- Phase 3 (Identify gaps): 1 hour
- Phase 4 (Create document): 1 hour

**Total**: ~6 hours of thorough, verified work

---

## Final Note

**Quality over speed**. A thorough, verified cross-reference is more valuable than a quick, inaccurate one. The previous agent failed because they didn't verify - don't make the same mistake!

Use Chrome DevTools for EVERYTHING. If you can't verify it in the browser, you can't claim it exists.
