# MCP Server Creation 500 Error - Root Cause & Fix

## Problem Summary
The `POST /api/v1/mcp-servers` endpoint was returning a **500 Internal Server Error** when attempting to create an MCP server, despite the first creation succeeding (201).

## Root Causes Identified

### 1. **Field Name Mismatch** (Primary Issue)
**Location**: `test_all_endpoints.sh` line 288

**Problem**: The test script was sending `base_url` but the backend expects `url`

```bash
# BEFORE (Wrong field name)
'{"name":"Test MCP","base_url":"https://test.mcp.com","public_key":"test-key"}'

# AFTER (Correct field name)
'{"name":"Test MCP","url":"https://test.mcp.com","public_key":"test-key"}'
```

**Why this caused 500**: When `url` was missing (empty string), the validation failed, but since `base_url` was provided, the request body was considered valid. However, when trying to create the MCP server with an empty URL, it likely caused database constraint violations or service-level errors.

### 2. **Duplicate URL Detection** (Secondary Issue)
**Location**: `apps/backend/internal/application/mcp_service.go` line 79

**Problem**: The service correctly detected duplicate URLs, but the handler was returning 500 instead of 409 Conflict

```
❌ Error creating MCP server: mcp server with this URL already exists
[500] POST /api/v1/mcp-servers
```

**Backend Schema**:
```go
type CreateMCPServerRequest struct {
    Name            string   `json:"name" validate:"required"`
    Description     string   `json:"description"`
    URL             string   `json:"url" validate:"required,url"` // ✅ Expects "url" not "base_url"
    Version         string   `json:"version"`
    PublicKey       string   `json:"public_key"`
    VerificationURL string   `json:"verification_url"`
    Capabilities    []string `json:"capabilities"`
}
```

**Database Schema**:
```sql
CREATE TABLE mcp_servers (
    ...
    url VARCHAR(500) NOT NULL, -- ✅ Uses "url" not "base_url"
    ...
);
```

### 3. **Test Re-runs Creating Duplicates**
**Problem**: Running the test multiple times would fail because it tried to create an MCP server with the same URL

## Fixes Applied

### Fix 1: Correct Field Name in Test Script
**File**: `test_all_endpoints.sh`

```bash
# Use timestamp to ensure unique URL for each test run
TIMESTAMP=$(date +%s)
test_endpoint "POST" "/api/v1/mcp-servers" "201" "Create MCP server" \
  '{"name":"Test MCP","url":"https://test-'$TIMESTAMP'.mcp.com","public_key":"test-key"}'
```

**Changes**:
- ✅ Changed `base_url` → `url` (matches backend expectation)
- ✅ Added timestamp to URL to make it unique for each test run

### Fix 2: Proper HTTP Status Code for Duplicate URLs
**File**: `apps/backend/internal/interfaces/http/handlers/mcp_handler.go`

```go
server, err := h.mcpService.CreateMCPServer(c.Context(), &req, orgID, userID)
if err != nil {
    // Log the actual error for debugging
    fmt.Printf("❌ Error creating MCP server: %v\n", err)

    // Return 409 Conflict for duplicate URL errors
    if err.Error() == "mcp server with this URL already exists" {
        return c.Status(fiber.StatusConflict).JSON(fiber.Map{
            "error": err.Error(),
        })
    }

    return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
        "error": err.Error(),
    })
}
```

**Changes**:
- ✅ Return **409 Conflict** for duplicate URL errors (proper REST semantics)
- ✅ Keep 500 for actual server errors

## Testing Status

### Before Fix
```
❌ Error creating MCP server: mcp server with this URL already exists
[500] POST /api/v1/mcp-servers
```

### After Fix (Expected Behavior)
```
# First run
✅ [201] POST /api/v1/mcp-servers - Create MCP server

# Second run (different timestamp URL)
✅ [201] POST /api/v1/mcp-servers - Create MCP server

# If duplicate URL somehow occurs
✅ [409] POST /api/v1/mcp-servers - "mcp server with this URL already exists"
```

## Lessons Learned

### Naming Consistency is CRITICAL
This bug is a **perfect example** of the naming consistency rule in `CLAUDE.md`:

> **PROBLEM**: Using different names for the same concept causes bugs that are hard to find.

**In this case**:
- ❌ Test used: `base_url`
- ✅ Backend expected: `url`
- ✅ Database used: `url`

**Prevention**:
1. Always check existing code for similar concepts before naming fields
2. Use exact same naming across backend, frontend, and database
3. Document naming conventions and follow them strictly

### Proper HTTP Status Codes Matter
- **409 Conflict**: Resource already exists (duplicate)
- **500 Internal Server Error**: Actual server failure
- **400 Bad Request**: Invalid input

## Verification Checklist

- [x] Go code compiles without errors
- [x] Test script syntax is valid
- [x] Field names match backend expectations
- [x] HTTP status codes are semantically correct
- [x] URL uniqueness guaranteed with timestamp
- [x] Documentation updated

## Ready to Test

The fix is **ready to test**. Run:

```bash
./test_all_endpoints.sh
```

**Expected Result**:
- ✅ POST /api/v1/mcp-servers returns 201 (Created)
- ✅ No 500 errors
- ✅ Each test run creates a new MCP server with unique URL
- ✅ If duplicate URL is manually tested, returns 409 (Conflict)
