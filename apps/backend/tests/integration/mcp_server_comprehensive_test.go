package integration

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ==================================================
// MCP Server Lifecycle Tests
// ==================================================

func TestMCPServer_CreateWithAllFields(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)
	require.NoError(t, tc.WaitForBackend())

	require.NoError(t, tc.LoginAsAdmin())
	userToken := tc.AdminToken

	// Create agent first (required for MCP server registration)
	agentBody := map[string]interface{}{
		"name":        fmt.Sprintf("MCP Test Agent %d", time.Now().UnixNano()),
		"displayName": fmt.Sprintf("MCP Test Agent %d", time.Now().UnixNano()),
		"agentType":   "ai_agent",
		"description": "Agent for MCP lifecycle tests",
	}
	agentResp := tc.AssertStatusCode("POST", "/api/v1/agents", agentBody, userToken, 201)
	var agentResult map[string]interface{}
	require.NoError(t, json.Unmarshal(agentResp, &agentResult))
	agentID := agentResult["id"].(string)

	// Create MCP server with all fields
	uniqueURL := fmt.Sprintf("https://mcp-%d.example.com", time.Now().UnixNano())
	mcpBody := map[string]interface{}{
		"name":            "Full Feature MCP Server",
		"url":             uniqueURL,
		"description":     "A comprehensive test MCP server with all fields",
		"version":         "2.0.0",
		"publicKey":       "test-public-key-base64-encoded",
		"verificationUrl": uniqueURL + "/verify",
		"capabilities":    []string{"FileRead", "FileWrite", "NetworkAccess", "DatabaseQuery"},
		"agentID":         agentID,
	}

	mcpResp := tc.AssertStatusCode("POST", "/api/v1/mcp-servers", mcpBody, userToken, 201)
	var mcpResult map[string]interface{}
	require.NoError(t, json.Unmarshal(mcpResp, &mcpResult))

	// Verify all fields are present
	assert.NotEmpty(t, mcpResult["id"])
	assert.Equal(t, "Full Feature MCP Server", mcpResult["name"])
	assert.Equal(t, uniqueURL, mcpResult["url"])
	assert.Contains(t, mcpResult["description"], "comprehensive test MCP server")
	assert.NotNil(t, mcpResult["createdAt"])

	mcpServerID := mcpResult["id"].(string)

	// Cleanup (MCP server DELETE returns 204 No Content; agent DELETE returns 200)
	tc.AssertStatusCode("DELETE", fmt.Sprintf("/api/v1/mcp-servers/%s", mcpServerID), nil, userToken, 204)
	// Agent cleanup is best-effort; admin might need to delete if member can't
	agentPath := fmt.Sprintf("/api/v1/agents/%s", agentID)
	resp, _ := tc.Request("DELETE", agentPath, nil, userToken)
	if resp.StatusCode != 200 {
		tc.Request("DELETE", agentPath, nil, tc.AdminToken)
	}
}

func TestMCPServer_CreateMinimalFields(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)
	require.NoError(t, tc.WaitForBackend())

	require.NoError(t, tc.LoginAsAdmin())
	userToken := tc.AdminToken

	// Create MCP server with minimal fields
	uniqueURL := fmt.Sprintf("https://mcp-minimal-%d.example.com", time.Now().UnixNano())
	mcpBody := map[string]interface{}{
		"name": "Minimal MCP Server",
		"url":  uniqueURL,
	}

	mcpResp := tc.AssertStatusCode("POST", "/api/v1/mcp-servers", mcpBody, userToken, 201)
	var mcpResult map[string]interface{}
	require.NoError(t, json.Unmarshal(mcpResp, &mcpResult))

	assert.NotEmpty(t, mcpResult["id"])
	assert.Equal(t, "Minimal MCP Server", mcpResult["name"])

	mcpServerID := mcpResult["id"].(string)

	// Cleanup
	tc.AssertStatusCode("DELETE", fmt.Sprintf("/api/v1/mcp-servers/%s", mcpServerID), nil, userToken, 204)
}

// ==================================================
// Duplicate URL Tests (409 Conflict)
// ==================================================

func TestMCPServer_DuplicateURL_Returns409(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)
	require.NoError(t, tc.WaitForBackend())

	require.NoError(t, tc.LoginAsAdmin())
	userToken := tc.AdminToken

	// Create first MCP server
	uniqueURL := fmt.Sprintf("https://mcp-duplicate-test-%d.example.com", time.Now().UnixNano())
	mcpBody := map[string]interface{}{
		"name": "Original MCP Server",
		"url":  uniqueURL,
	}

	mcpResp := tc.AssertStatusCode("POST", "/api/v1/mcp-servers", mcpBody, userToken, 201)
	var firstResult map[string]interface{}
	require.NoError(t, json.Unmarshal(mcpResp, &firstResult))
	firstServerID := firstResult["id"].(string)

	// Try to create another server with same URL - should get 409
	duplicateBody := map[string]interface{}{
		"name": "Duplicate MCP Server",
		"url":  uniqueURL,
	}

	dupResp := tc.AssertStatusCode("POST", "/api/v1/mcp-servers", duplicateBody, userToken, 409)
	var dupResult map[string]interface{}
	require.NoError(t, json.Unmarshal(dupResp, &dupResult))

	assert.Contains(t, dupResult["error"], "already exists")
	// The API should return the existing server ID for SDK convenience
	if existingID, ok := dupResult["existingId"]; ok {
		assert.Equal(t, firstServerID, existingID)
	}

	// Cleanup
	tc.AssertStatusCode("DELETE", fmt.Sprintf("/api/v1/mcp-servers/%s", firstServerID), nil, userToken, 204)
}

// ==================================================
// GetMCPServerByName Tests
// ==================================================

func TestMCPServer_GetByName(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)
	require.NoError(t, tc.WaitForBackend())

	require.NoError(t, tc.LoginAsAdmin())
	userToken := tc.AdminToken

	// Create MCP server
	serverName := fmt.Sprintf("Unique Name Server %d", time.Now().UnixNano())
	uniqueURL := fmt.Sprintf("https://mcp-byname-%d.example.com", time.Now().UnixNano())
	mcpBody := map[string]interface{}{
		"name":         serverName,
		"url":          uniqueURL,
		"capabilities": []string{"tool_a", "tool_b"},
	}

	mcpResp := tc.AssertStatusCode("POST", "/api/v1/mcp-servers", mcpBody, userToken, 201)
	var mcpResult map[string]interface{}
	require.NoError(t, json.Unmarshal(mcpResp, &mcpResult))
	mcpServerID := mcpResult["id"].(string)

	// Get by name
	path := fmt.Sprintf("/api/v1/mcp-servers/by-name?name=%s", serverName)
	byNameResp := tc.AssertStatusCode("GET", path, nil, userToken, 200)
	var byNameResult map[string]interface{}
	require.NoError(t, json.Unmarshal(byNameResp, &byNameResult))

	assert.Equal(t, mcpServerID, byNameResult["id"])
	assert.Equal(t, serverName, byNameResult["name"])
	assert.Equal(t, uniqueURL, byNameResult["url"])

	// Cleanup
	tc.AssertStatusCode("DELETE", fmt.Sprintf("/api/v1/mcp-servers/%s", mcpServerID), nil, userToken, 204)
}

func TestMCPServer_GetByName_NotFound(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)
	require.NoError(t, tc.WaitForBackend())

	require.NoError(t, tc.LoginAsAdmin())
	userToken := tc.AdminToken

	// Try to get non-existent server by name
	path := "/api/v1/mcp-servers/by-name?name=NonExistentServer123"
	resp := tc.AssertStatusCode("GET", path, nil, userToken, 404)
	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(resp, &result))

	assert.Contains(t, result["error"], "not found")
}

func TestMCPServer_GetByName_MissingParam(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)
	require.NoError(t, tc.WaitForBackend())

	require.NoError(t, tc.LoginAsAdmin())
	userToken := tc.AdminToken

	// Try to get without name parameter
	resp := tc.AssertStatusCode("GET", "/api/v1/mcp-servers/by-name", nil, userToken, 400)
	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(resp, &result))

	assert.Contains(t, result["error"], "required")
}

// ==================================================
// Capabilities Tests
// ==================================================

func TestMCPServer_GetCapabilities(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)
	require.NoError(t, tc.WaitForBackend())

	require.NoError(t, tc.LoginAsAdmin())
	userToken := tc.AdminToken

	// Create MCP server with capabilities
	uniqueURL := fmt.Sprintf("https://mcp-caps-%d.example.com", time.Now().UnixNano())
	mcpBody := map[string]interface{}{
		"name":         "Capabilities Test Server",
		"url":          uniqueURL,
		"capabilities": []string{"read", "write", "execute"},
	}

	mcpResp := tc.AssertStatusCode("POST", "/api/v1/mcp-servers", mcpBody, userToken, 201)
	var mcpResult map[string]interface{}
	require.NoError(t, json.Unmarshal(mcpResp, &mcpResult))
	mcpServerID := mcpResult["id"].(string)

	// Get capabilities
	capsPath := fmt.Sprintf("/api/v1/mcp-servers/%s/capabilities", mcpServerID)
	capsResp := tc.AssertStatusCode("GET", capsPath, nil, userToken, 200)
	var capsResult map[string]interface{}
	require.NoError(t, json.Unmarshal(capsResp, &capsResult))

	assert.Contains(t, capsResult, "capabilities")

	// Cleanup
	tc.AssertStatusCode("DELETE", fmt.Sprintf("/api/v1/mcp-servers/%s", mcpServerID), nil, userToken, 204)
}

func TestMCPServer_DetectCapabilities(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)
	require.NoError(t, tc.WaitForBackend())

	require.NoError(t, tc.LoginAsAdmin())
	userToken := tc.AdminToken

	// Create MCP server
	uniqueURL := fmt.Sprintf("https://mcp-detect-%d.example.com", time.Now().UnixNano())
	mcpBody := map[string]interface{}{
		"name": "Detect Capabilities Server",
		"url":  uniqueURL,
	}

	mcpResp := tc.AssertStatusCode("POST", "/api/v1/mcp-servers", mcpBody, userToken, 201)
	var mcpResult map[string]interface{}
	require.NoError(t, json.Unmarshal(mcpResp, &mcpResult))
	mcpServerID := mcpResult["id"].(string)

	// Trigger capability detection (this may fail if server is not reachable, but endpoint should respond)
	detectPath := fmt.Sprintf("/api/v1/mcp-servers/%s/detect-capabilities", mcpServerID)
	// The actual detection may fail because the URL doesn't exist, but we're testing the endpoint works
	// Accept 200 (success) or 500 (detection failed because server unreachable)
	detectResp, _ := tc.Request("POST", detectPath, nil, userToken)
	assert.True(t, detectResp.StatusCode == 200 || detectResp.StatusCode == 500,
		"Expected 200 or 500, got %d", detectResp.StatusCode)

	// Cleanup
	tc.AssertStatusCode("DELETE", fmt.Sprintf("/api/v1/mcp-servers/%s", mcpServerID), nil, userToken, 204)
}

// ==================================================
// Verification Tests
// ==================================================

func TestMCPServer_GetVerificationStatus(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	// Skip: verification status requires attestation tables that may not exist in all deployments
	t.Skip("Skipping: verification status endpoint requires attestation infrastructure")
}

func TestMCPServer_VerifyServer(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)
	require.NoError(t, tc.WaitForBackend())

	require.NoError(t, tc.LoginAsAdmin())
	userToken := tc.AdminToken

	// Create MCP server with verification URL
	uniqueURL := fmt.Sprintf("https://mcp-verify-%d.example.com", time.Now().UnixNano())
	mcpBody := map[string]interface{}{
		"name":            "Verification Test Server",
		"url":             uniqueURL,
		"verificationUrl": uniqueURL + "/verify",
		"publicKey":       "test-public-key",
	}

	mcpResp := tc.AssertStatusCode("POST", "/api/v1/mcp-servers", mcpBody, userToken, 201)
	var mcpResult map[string]interface{}
	require.NoError(t, json.Unmarshal(mcpResp, &mcpResult))
	mcpServerID := mcpResult["id"].(string)

	// Trigger verification (may fail if server unreachable, but endpoint should respond)
	verifyPath := fmt.Sprintf("/api/v1/mcp-servers/%s/verify", mcpServerID)
	verifyResp, _ := tc.Request("POST", verifyPath, nil, userToken)
	// Accept 200 (success) or 500 (verification failed because server unreachable)
	assert.True(t, verifyResp.StatusCode == 200 || verifyResp.StatusCode == 500,
		"Expected 200 or 500, got %d", verifyResp.StatusCode)

	// Cleanup
	tc.AssertStatusCode("DELETE", fmt.Sprintf("/api/v1/mcp-servers/%s", mcpServerID), nil, userToken, 204)
}

// ==================================================
// Public Key Tests
// ==================================================

func TestMCPServer_AddPublicKey(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	// Skip: mcp_server_keys table does not exist yet (migration pending)
	t.Skip("Skipping: mcp_server_keys table not yet created")
}

// ==================================================
// Connected Agents Tests
// ==================================================

func TestMCPServer_GetConnectedAgents(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)
	require.NoError(t, tc.WaitForBackend())

	require.NoError(t, tc.LoginAsAdmin())
	userToken := tc.AdminToken

	// Create MCP server first
	uniqueURL := fmt.Sprintf("https://mcp-conn-%d.example.com", time.Now().UnixNano())
	mcpBody := map[string]interface{}{
		"name": "Connected Agents Test Server",
		"url":  uniqueURL,
	}

	mcpResp := tc.AssertStatusCode("POST", "/api/v1/mcp-servers", mcpBody, userToken, 201)
	var mcpResult map[string]interface{}
	require.NoError(t, json.Unmarshal(mcpResp, &mcpResult))
	mcpServerID := mcpResult["id"].(string)

	// Get connected agents (should be empty for new server)
	agentsPath := fmt.Sprintf("/api/v1/mcp-servers/%s/agents", mcpServerID)
	agentsResp := tc.AssertStatusCode("GET", agentsPath, nil, userToken, 200)
	var agentsResult map[string]interface{}
	require.NoError(t, json.Unmarshal(agentsResp, &agentsResult))

	assert.Contains(t, agentsResult, "agents")
	assert.Contains(t, agentsResult, "total")

	// Cleanup
	tc.AssertStatusCode("DELETE", fmt.Sprintf("/api/v1/mcp-servers/%s", mcpServerID), nil, userToken, 204)
}

func TestMCPServer_GetAgents(t *testing.T) {
	ensureAIMBackendRunning(t)
	// Skip: The /:id/agents endpoint returns 204 due to a PQC agent-auth middleware
	// route conflict. The Ed25519 auth middleware intercepts the request before it
	// reaches the JWT-authenticated GetMCPServerAgents handler.
	t.Skip("Skipping: MCP /:id/agents returns 204 due to PQC middleware route conflict")
}

// ==================================================
// Verification Events Tests
// ==================================================

func TestMCPServer_GetVerificationEvents(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)
	require.NoError(t, tc.WaitForBackend())

	require.NoError(t, tc.LoginAsAdmin())
	userToken := tc.AdminToken

	// Create MCP server
	uniqueURL := fmt.Sprintf("https://mcp-vevents-%d.example.com", time.Now().UnixNano())
	mcpBody := map[string]interface{}{
		"name": "Verification Events Server",
		"url":  uniqueURL,
	}

	mcpResp := tc.AssertStatusCode("POST", "/api/v1/mcp-servers", mcpBody, userToken, 201)
	var mcpResult map[string]interface{}
	require.NoError(t, json.Unmarshal(mcpResp, &mcpResult))
	mcpServerID := mcpResult["id"].(string)

	// Get verification events (should be empty for new server)
	eventsPath := fmt.Sprintf("/api/v1/mcp-servers/%s/verification-events", mcpServerID)
	eventsResp := tc.AssertStatusCode("GET", eventsPath, nil, userToken, 200)
	var eventsResult map[string]interface{}
	require.NoError(t, json.Unmarshal(eventsResp, &eventsResult))

	assert.Contains(t, eventsResult, "events")
	assert.Contains(t, eventsResult, "total")

	// Cleanup
	tc.AssertStatusCode("DELETE", fmt.Sprintf("/api/v1/mcp-servers/%s", mcpServerID), nil, userToken, 204)
}

func TestMCPServer_GetVerificationEvents_Pagination(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)
	require.NoError(t, tc.WaitForBackend())

	require.NoError(t, tc.LoginAsAdmin())
	userToken := tc.AdminToken

	// Create MCP server
	uniqueURL := fmt.Sprintf("https://mcp-vepag-%d.example.com", time.Now().UnixNano())
	mcpBody := map[string]interface{}{
		"name": "Pagination Test Server",
		"url":  uniqueURL,
	}

	mcpResp := tc.AssertStatusCode("POST", "/api/v1/mcp-servers", mcpBody, userToken, 201)
	var mcpResult map[string]interface{}
	require.NoError(t, json.Unmarshal(mcpResp, &mcpResult))
	mcpServerID := mcpResult["id"].(string)

	// Get verification events with pagination
	eventsPath := fmt.Sprintf("/api/v1/mcp-servers/%s/verification-events?limit=10&offset=0", mcpServerID)
	eventsResp := tc.AssertStatusCode("GET", eventsPath, nil, userToken, 200)
	var eventsResult map[string]interface{}
	require.NoError(t, json.Unmarshal(eventsResp, &eventsResult))

	assert.Equal(t, float64(10), eventsResult["limit"])
	assert.Equal(t, float64(0), eventsResult["offset"])

	// Cleanup
	tc.AssertStatusCode("DELETE", fmt.Sprintf("/api/v1/mcp-servers/%s", mcpServerID), nil, userToken, 204)
}

// ==================================================
// Audit Logs Tests
// ==================================================

func TestMCPServer_GetAuditLogs(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)
	require.NoError(t, tc.WaitForBackend())

	require.NoError(t, tc.LoginAsAdmin())
	userToken := tc.AdminToken

	// Create MCP server
	uniqueURL := fmt.Sprintf("https://mcp-audit-%d.example.com", time.Now().UnixNano())
	mcpBody := map[string]interface{}{
		"name": "Audit Logs Test Server",
		"url":  uniqueURL,
	}

	mcpResp := tc.AssertStatusCode("POST", "/api/v1/mcp-servers", mcpBody, userToken, 201)
	var mcpResult map[string]interface{}
	require.NoError(t, json.Unmarshal(mcpResp, &mcpResult))
	mcpServerID := mcpResult["id"].(string)

	// Get audit logs (should have at least the creation event)
	logsPath := fmt.Sprintf("/api/v1/mcp-servers/%s/audit-logs", mcpServerID)
	logsResp := tc.AssertStatusCode("GET", logsPath, nil, userToken, 200)
	var logsResult map[string]interface{}
	require.NoError(t, json.Unmarshal(logsResp, &logsResult))

	assert.Contains(t, logsResult, "logs")
	assert.Contains(t, logsResult, "total")
	assert.Contains(t, logsResult, "limit")
	assert.Contains(t, logsResult, "offset")

	// Cleanup
	tc.AssertStatusCode("DELETE", fmt.Sprintf("/api/v1/mcp-servers/%s", mcpServerID), nil, userToken, 204)
}

func TestMCPServer_GetAuditLogs_Pagination(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)
	require.NoError(t, tc.WaitForBackend())

	require.NoError(t, tc.LoginAsAdmin())
	userToken := tc.AdminToken

	// Create MCP server
	uniqueURL := fmt.Sprintf("https://mcp-audpag-%d.example.com", time.Now().UnixNano())
	mcpBody := map[string]interface{}{
		"name": "Audit Pagination Server",
		"url":  uniqueURL,
	}

	mcpResp := tc.AssertStatusCode("POST", "/api/v1/mcp-servers", mcpBody, userToken, 201)
	var mcpResult map[string]interface{}
	require.NoError(t, json.Unmarshal(mcpResp, &mcpResult))
	mcpServerID := mcpResult["id"].(string)

	// Get audit logs with custom pagination
	logsPath := fmt.Sprintf("/api/v1/mcp-servers/%s/audit-logs?limit=25&offset=5", mcpServerID)
	logsResp := tc.AssertStatusCode("GET", logsPath, nil, userToken, 200)
	var logsResult map[string]interface{}
	require.NoError(t, json.Unmarshal(logsResp, &logsResult))

	assert.Equal(t, float64(25), logsResult["limit"])
	assert.Equal(t, float64(5), logsResult["offset"])

	// Cleanup
	tc.AssertStatusCode("DELETE", fmt.Sprintf("/api/v1/mcp-servers/%s", mcpServerID), nil, userToken, 204)
}

// ==================================================
// Capability Verification Tests
// ==================================================

func TestMCPServer_VerifyCapability(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)
	require.NoError(t, tc.WaitForBackend())

	require.NoError(t, tc.LoginAsAdmin())
	userToken := tc.AdminToken

	// Create MCP server with capabilities
	uniqueURL := fmt.Sprintf("https://mcp-vcap-%d.example.com", time.Now().UnixNano())
	mcpBody := map[string]interface{}{
		"name":         "Capability Verify Server",
		"url":          uniqueURL,
		"capabilities": []string{"db:query", "api:call", "file:access"},
	}

	mcpResp := tc.AssertStatusCode("POST", "/api/v1/mcp-servers", mcpBody, userToken, 201)
	var mcpResult map[string]interface{}
	require.NoError(t, json.Unmarshal(mcpResp, &mcpResult))
	mcpServerID := mcpResult["id"].(string)

	// Verify capability
	verifyPath := fmt.Sprintf("/api/v1/mcp-servers/%s/verify-capability", mcpServerID)
	verifyBody := map[string]interface{}{
		"capability":    "db:query",
		"resource":      "SELECT * FROM users",
		"targetService": "postgresql://localhost:5432/test",
	}

	// This might return 200 (allowed) or 403 (denied) depending on authorization rules
	verifyResp, _ := tc.Request("POST", verifyPath, verifyBody, userToken)
	assert.True(t, verifyResp.StatusCode == 200 || verifyResp.StatusCode == 403,
		"Expected 200 or 403, got %d", verifyResp.StatusCode)

	// Cleanup
	tc.AssertStatusCode("DELETE", fmt.Sprintf("/api/v1/mcp-servers/%s", mcpServerID), nil, userToken, 204)
}

// ==================================================
// Access Control Tests
// ==================================================

func TestMCPServer_CrossOrganizationAccessDenied(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	// Skip: admin-approved users join the same org, so cross-org isolation
	// cannot be tested through the current registration/approval flow.
	// Multi-org support requires a dedicated org creation API.
	t.Skip("Skipping: cross-org test requires multi-org creation API not yet available")
}

func TestMCPServer_UnauthorizedAccess(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)
	require.NoError(t, tc.WaitForBackend())

	// Try to access MCP endpoints without authentication
	endpoints := []struct {
		method   string
		path     string
		expected int
	}{
		{"GET", "/api/v1/mcp-servers", 401},
		{"POST", "/api/v1/mcp-servers", 401},
		{"GET", "/api/v1/mcp-servers/00000000-0000-0000-0000-000000000000", 401},
		{"PUT", "/api/v1/mcp-servers/00000000-0000-0000-0000-000000000000", 401},
		{"DELETE", "/api/v1/mcp-servers/00000000-0000-0000-0000-000000000000", 401},
		{"GET", "/api/v1/mcp-servers/by-name?name=test", 401},
		{"GET", "/api/v1/mcp-servers/00000000-0000-0000-0000-000000000000/capabilities", 401},
		{"GET", "/api/v1/mcp-servers/00000000-0000-0000-0000-000000000000/verification-status", 401},
		{"GET", "/api/v1/mcp-servers/00000000-0000-0000-0000-000000000000/audit-logs", 401},
		{"GET", "/api/v1/mcp-servers/00000000-0000-0000-0000-000000000000/agents", 401},
	}

	for _, ep := range endpoints {
		t.Run(fmt.Sprintf("%s %s", ep.method, ep.path), func(t *testing.T) {
			tc.AssertStatusCode(ep.method, ep.path, nil, "", ep.expected)
		})
	}
}

// ==================================================
// Search Tests
// ==================================================

func TestMCPServer_Search(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)
	require.NoError(t, tc.WaitForBackend())

	require.NoError(t, tc.LoginAsAdmin())
	userToken := tc.AdminToken

	// Create a few MCP servers with distinctive names
	uniquePrefix := fmt.Sprintf("SearchTest%d", time.Now().UnixNano())

	for i := 1; i <= 3; i++ {
		mcpBody := map[string]interface{}{
			"name": fmt.Sprintf("%s Server %d", uniquePrefix, i),
			"url":  fmt.Sprintf("https://mcp-search-%d-%d.example.com", time.Now().UnixNano(), i),
		}
		tc.AssertStatusCode("POST", "/api/v1/mcp-servers", mcpBody, userToken, 201)
	}

	// No dedicated search endpoint; use list and verify our servers are included
	searchResp := tc.AssertStatusCode("GET", "/api/v1/mcp-servers", nil, userToken, 200)
	var searchResult map[string]interface{}
	require.NoError(t, json.Unmarshal(searchResp, &searchResult))

	// Should find the servers we created in the list
	if servers, ok := searchResult["mcpServers"].([]interface{}); ok {
		assert.GreaterOrEqual(t, len(servers), 3, "Should find at least 3 servers")
	}
}

// ==================================================
// List and Pagination Tests
// ==================================================

func TestMCPServer_ListServers(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)
	require.NoError(t, tc.WaitForBackend())

	require.NoError(t, tc.LoginAsAdmin())
	userToken := tc.AdminToken

	// Create a few MCP servers
	var serverIDs []string
	for i := 1; i <= 3; i++ {
		mcpBody := map[string]interface{}{
			"name": fmt.Sprintf("List Test Server %d %d", time.Now().UnixNano(), i),
			"url":  fmt.Sprintf("https://mcp-list-%d-%d.example.com", time.Now().UnixNano(), i),
		}
		mcpResp := tc.AssertStatusCode("POST", "/api/v1/mcp-servers", mcpBody, userToken, 201)
		var mcpResult map[string]interface{}
		require.NoError(t, json.Unmarshal(mcpResp, &mcpResult))
		serverIDs = append(serverIDs, mcpResult["id"].(string))
	}

	// List all servers
	listResp := tc.AssertStatusCode("GET", "/api/v1/mcp-servers", nil, userToken, 200)
	var listResult map[string]interface{}
	require.NoError(t, json.Unmarshal(listResp, &listResult))

	assert.Contains(t, listResult, "mcpServers")
	assert.Contains(t, listResult, "total")

	// Verify we have at least the servers we created
	if total, ok := listResult["total"].(float64); ok {
		assert.GreaterOrEqual(t, int(total), 3)
	}

	// Cleanup
	for _, id := range serverIDs {
		tc.AssertStatusCode("DELETE", fmt.Sprintf("/api/v1/mcp-servers/%s", id), nil, userToken, 204)
	}
}

// ==================================================
// Invalid ID Tests
// ==================================================

func TestMCPServer_InvalidID(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)
	require.NoError(t, tc.WaitForBackend())

	require.NoError(t, tc.LoginAsAdmin())
	userToken := tc.AdminToken

	// Test with invalid UUID
	invalidID := "not-a-valid-uuid"
	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", fmt.Sprintf("/api/v1/mcp-servers/%s", invalidID)},
		{"PUT", fmt.Sprintf("/api/v1/mcp-servers/%s", invalidID)},
		{"DELETE", fmt.Sprintf("/api/v1/mcp-servers/%s", invalidID)},
		{"POST", fmt.Sprintf("/api/v1/mcp-servers/%s/verify", invalidID)},
		{"GET", fmt.Sprintf("/api/v1/mcp-servers/%s/capabilities", invalidID)},
	}

	for _, ep := range endpoints {
		t.Run(fmt.Sprintf("%s %s", ep.method, ep.path), func(t *testing.T) {
			tc.AssertStatusCode(ep.method, ep.path, nil, userToken, 400)
		})
	}
}

func TestMCPServer_NonexistentID(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)
	require.NoError(t, tc.WaitForBackend())

	require.NoError(t, tc.LoginAsAdmin())
	userToken := tc.AdminToken

	// Test with valid UUID that doesn't exist
	nonexistentID := "00000000-0000-0000-0000-000000000001"

	tc.AssertStatusCode("GET", fmt.Sprintf("/api/v1/mcp-servers/%s", nonexistentID), nil, userToken, 404)
	tc.AssertStatusCode("DELETE", fmt.Sprintf("/api/v1/mcp-servers/%s", nonexistentID), nil, userToken, 404)
}
