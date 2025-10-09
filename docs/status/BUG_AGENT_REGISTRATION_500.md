# 🐛 Critical Bug: Agent Registration Returns 500 Error

**Discovered**: October 6, 2025 during comprehensive production testing
**Severity**: HIGH (blocks user workflow)
**Status**: Frontend issue with request payload

---

## Summary

Agent registration form submits incorrect payload to backend, causing 500 Internal Server Error. Backend API works correctly when called with proper payload via curl.

---

## Reproduction Steps

1. Navigate to http://localhost:3000/dashboard/agents
2. Click "Register Agent" button
3. Fill form:
   - Agent Name: test-agent-1
   - Display Name: Test AI Agent
   - Description: A test agent for production readiness testing
   - Agent Type: AI Agent
   - Version: 1.0.0
4. Click "Register Agent" submit button

**Expected**: Agent created successfully (201 Created)
**Actual**: HTTP 500 error, fallback to mock mode

---

## Evidence

### Frontend Error
```
Console: "Failed to save agent: Error: HTTP 500"
Network: POST /api/v1/agents [failed - 500]
```

### Backend Log
```
[2025-10-06T17:39:54Z] 500 - 10.390542ms POST /api/v1/agents
```

### Successful curl Test
```bash
curl -X POST http://localhost:8080/api/v1/agents \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"test-agent-3","displayName":"Test Agent 3","description":"Test","agentType":"ai_agent","version":"1.0.0"}'

# Result: 201 Created ✅
# Agent appears in UI after page refresh
```

---

## Root Cause Analysis

**Problem**: Frontend sends incorrect field names or payload structure to backend.

**Files Involved**:
- Frontend: `apps/web/components/modals/register-agent-modal.tsx` (line 70)
- Frontend API: `apps/web/lib/api.ts` (agent registration method)
- Backend Handler: `apps/backend/internal/interfaces/http/handlers/agent_handler.go` (line 43-78)

**Backend Expects** (working payload):
```typescript
{
  "name": string,
  "displayName": string,
  "description": string,
  "agentType": string,  // "ai_agent" or "mcp_server"
  "version": string
}
```

**Frontend Likely Sends** (incorrect):
- Unknown - need to inspect network request details
- Possibly wrong field names (snake_case vs camelCase)
- Possibly missing required fields
- Possibly extra fields causing validation error

---

## Impact

**User Impact**: HIGH
- Users cannot register agents via UI
- Must use API directly (curl/Postman)
- Poor user experience

**Business Impact**: MEDIUM
- Blocks primary user workflow
- Reduces platform usability
- Not suitable for production launch

**Workaround Available**: Yes
- Frontend shows "Agent registered successfully!" despite error
- Uses mock data fallback
- Actual registration works via direct API call

---

## Fix Required

1. **Inspect Network Request**: Capture exact payload sent by frontend
2. **Compare with Backend**: Match against CreateAgentRequest struct
3. **Update Frontend**: Fix field names and payload structure
4. **Test**: Verify registration works end-to-end
5. **Regression Test**: Ensure other operations still work

**Priority**: HIGH - should be fixed before public launch

---

## Testing Recommendations

After fix:
1. Test agent registration with all field combinations
2. Test with minimum required fields only
3. Test with maximum fields filled
4. Verify error handling for invalid input
5. Verify success toast and UI update
6. Test agent appears immediately without page refresh

---

## Related Issues

- Frontend error handling masks actual problem (shows success despite 500)
- Mock data fallback helpful for development but hides bugs
- Need better error reporting to show actual backend error messages

---

**Reported by**: Claude Code (Comprehensive Testing Phase 2)
**Date**: October 6, 2025
**Test Phase**: Frontend Chrome DevTools Testing
