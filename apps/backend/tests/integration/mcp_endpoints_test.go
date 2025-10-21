package integration

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMCPServerEndpoints(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)

	// Wait for backend and login
	require.NoError(t, tc.WaitForBackend())

	email := fmt.Sprintf("mcp-test-%d@example.com", time.Now().Unix())
	userToken, err := tc.CreateTestUser(email, "TestPass123!")
	require.NoError(t, err)

	var createdServerID string
	var createdAgentID string

	// Create an agent first (required for MCP server registration)
	t.Run("Setup - Create agent for MCP tests", func(t *testing.T) {
		body := map[string]interface{}{
			"name":        "MCP Test Agent",
			"type":        "ai_agent",
			"description": "Agent for MCP testing",
		}

		respBody := tc.AssertStatusCode("POST", "/api/v1/agents", body, userToken, 201)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		createdAgentID = result["id"].(string)
	})

	t.Run("POST /api/v1/mcp-servers - Register MCP server", func(t *testing.T) {
		body := map[string]interface{}{
			"name":        "Test MCP Server",
			"url":         "https://mcp-server.example.com",
			"description": "Integration test MCP server",
			"agentID":     createdAgentID,
			"publicKey":   "test-public-key-data",
		}

		respBody := tc.AssertStatusCode("POST", "/api/v1/mcp-servers", body, userToken, 201)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Contains(t, result, "id")
		assert.Equal(t, "Test MCP Server", result["name"])
		assert.Equal(t, "https://mcp-server.example.com", result["url"])

		createdServerID = result["id"].(string)
	})

	t.Run("GET /api/v1/mcp-servers - List all MCP servers", func(t *testing.T) {
		respBody := tc.AssertStatusCode("GET", "/api/v1/mcp-servers", nil, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Contains(t, result, "servers")
		servers := result["servers"].([]interface{})
		assert.GreaterOrEqual(t, len(servers), 1)
	})

	t.Run("GET /api/v1/mcp-servers/:id - Get MCP server by ID", func(t *testing.T) {
		path := fmt.Sprintf("/api/v1/mcp-servers/%s", createdServerID)
		respBody := tc.AssertStatusCode("GET", path, nil, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Equal(t, createdServerID, result["id"])
		assert.Equal(t, "Test MCP Server", result["name"])
	})

	t.Run("PUT /api/v1/mcp-servers/:id - Update MCP server", func(t *testing.T) {
		path := fmt.Sprintf("/api/v1/mcp-servers/%s", createdServerID)
		body := map[string]interface{}{
			"name":        "Updated MCP Server",
			"description": "Updated MCP description",
		}

		respBody := tc.AssertStatusCode("PUT", path, body, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Equal(t, "Updated MCP Server", result["name"])
	})

	t.Run("POST /api/v1/mcp-servers/:id/verify - Verify MCP server", func(t *testing.T) {
		path := fmt.Sprintf("/api/v1/mcp-servers/%s/verify", createdServerID)
		body := map[string]interface{}{
			"signature": "test-signature",
			"nonce":     "test-nonce",
		}

		respBody := tc.AssertStatusCode("POST", path, body, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Contains(t, result, "verified")
	})

	t.Run("GET /api/v1/mcp-servers/:id/capabilities - Get MCP server capabilities", func(t *testing.T) {
		path := fmt.Sprintf("/api/v1/mcp-servers/%s/capabilities", createdServerID)
		respBody := tc.AssertStatusCode("GET", path, nil, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Contains(t, result, "capabilities")
	})

	t.Run("POST /api/v1/mcp-servers/:id/capabilities - Update capabilities", func(t *testing.T) {
		path := fmt.Sprintf("/api/v1/mcp-servers/%s/capabilities", createdServerID)
		body := map[string]interface{}{
			"capabilities": []string{"FileRead", "FileWrite", "NetworkAccess"},
		}

		respBody := tc.AssertStatusCode("POST", path, body, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Contains(t, result, "capabilities")
	})

	t.Run("GET /api/v1/mcp-servers/search - Search MCP servers", func(t *testing.T) {
		path := "/api/v1/mcp-servers/search?query=MCP"
		respBody := tc.AssertStatusCode("GET", path, nil, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Contains(t, result, "servers")
	})

	t.Run("DELETE /api/v1/mcp-servers/:id - Delete MCP server", func(t *testing.T) {
		path := fmt.Sprintf("/api/v1/mcp-servers/%s", createdServerID)
		tc.AssertStatusCode("DELETE", path, nil, userToken, 200)

		// Verify server is deleted
		tc.AssertStatusCode("GET", path, nil, userToken, 404)
	})

	t.Run("POST /api/v1/mcp-servers - Require authentication", func(t *testing.T) {
		body := map[string]interface{}{
			"name": "Unauthenticated Server",
			"url":  "https://example.com",
		}

		tc.AssertStatusCode("POST", "/api/v1/mcp-servers", body, "", 401)
	})

	// Cleanup
	t.Run("Cleanup - Delete test agent", func(t *testing.T) {
		path := fmt.Sprintf("/api/v1/agents/%s", createdAgentID)
		tc.AssertStatusCode("DELETE", path, nil, userToken, 200)
	})
}
