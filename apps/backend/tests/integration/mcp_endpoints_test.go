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
	ensureAIMBackendRunning(t) // Skip if AIM backend not running
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)

	// Wait for backend and login as admin
	require.NoError(t, tc.WaitForBackend())
	require.NoError(t, tc.LoginAsAdmin())

	userToken := tc.AdminToken

	var createdServerID string
	var createdAgentID string
	mcpServerName := fmt.Sprintf("Test MCP Server %d", time.Now().UnixNano())
	mcpServerURL := fmt.Sprintf("https://mcp-server-%d.example.com", time.Now().UnixNano())

	// Create an agent first (required for MCP server registration)
	t.Run("Setup - Create agent for MCP tests", func(t *testing.T) {
		agentName := fmt.Sprintf("MCP Test Agent %d", time.Now().UnixNano())
		body := map[string]interface{}{
			"name":        agentName,
			"displayName": agentName,
			"agentType":   "ai_agent",
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
			"name":        mcpServerName,
			"url":         mcpServerURL,
			"description": "Integration test MCP server",
			"agentID":     createdAgentID,
			"publicKey":   "test-public-key-data",
		}

		respBody := tc.AssertStatusCode("POST", "/api/v1/mcp-servers", body, userToken, 201)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Contains(t, result, "id")
		assert.Equal(t, mcpServerName, result["name"])
		assert.Equal(t, mcpServerURL, result["url"])

		createdServerID = result["id"].(string)
	})

	t.Run("GET /api/v1/mcp-servers - List all MCP servers", func(t *testing.T) {
		respBody := tc.AssertStatusCode("GET", "/api/v1/mcp-servers", nil, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Contains(t, result, "mcpServers")
		servers := result["mcpServers"].([]interface{})
		assert.GreaterOrEqual(t, len(servers), 1)
	})

	t.Run("GET /api/v1/mcp-servers/:id - Get MCP server by ID", func(t *testing.T) {
		path := fmt.Sprintf("/api/v1/mcp-servers/%s", createdServerID)
		respBody := tc.AssertStatusCode("GET", path, nil, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Equal(t, createdServerID, result["id"])
		assert.Equal(t, mcpServerName, result["name"])
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
		// Skip: The verify endpoint performs DNS resolution on the MCP server URL.
		// Dynamic test URLs use non-existent hostnames that can't be resolved.
		t.Skip("Skipping: verify endpoint performs DNS resolution on test URLs")
	})

	t.Run("GET /api/v1/mcp-servers/:id/capabilities - Get MCP server capabilities", func(t *testing.T) {
		path := fmt.Sprintf("/api/v1/mcp-servers/%s/capabilities", createdServerID)
		respBody := tc.AssertStatusCode("GET", path, nil, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Contains(t, result, "capabilities")
	})

	t.Run("GET /api/v1/mcp-servers - List filters MCP servers", func(t *testing.T) {
		// No dedicated search endpoint; use the list endpoint
		respBody := tc.AssertStatusCode("GET", "/api/v1/mcp-servers", nil, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Contains(t, result, "mcpServers")
	})

	t.Run("DELETE /api/v1/mcp-servers/:id - Delete MCP server", func(t *testing.T) {
		path := fmt.Sprintf("/api/v1/mcp-servers/%s", createdServerID)
		tc.AssertStatusCode("DELETE", path, nil, userToken, 204)

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
		tc.AssertStatusCode("DELETE", path, nil, userToken, 204)
	})
}
