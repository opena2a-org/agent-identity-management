package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestListMCPServersUnauthorized tests that listing MCP servers requires authentication
func TestListMCPServersUnauthorized(t *testing.T) {
	baseURL := getBaseURL()

	resp, err := http.Get(baseURL + "/api/v1/mcp-servers")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestCreateMCPServerUnauthorized tests that creating MCP servers requires authentication
func TestCreateMCPServerUnauthorized(t *testing.T) {
	baseURL := getBaseURL()

	payload := map[string]interface{}{
		"name":        "test-mcp",
		"description": "Test MCP server",
		"url":         "https://test-mcp.example.com",
	}
	jsonData, err := json.Marshal(payload)
	require.NoError(t, err)

	resp, err := http.Post(baseURL+"/api/v1/mcp-servers", "application/json", bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestGetMCPServerUnauthorized tests that getting MCP server requires authentication
func TestGetMCPServerUnauthorized(t *testing.T) {
	baseURL := getBaseURL()
	mcpID := "123e4567-e89b-12d3-a456-426614174000"

	resp, err := http.Get(baseURL + "/api/v1/mcp-servers/" + mcpID)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestUpdateMCPServerUnauthorized tests that updating MCP server requires authentication
func TestUpdateMCPServerUnauthorized(t *testing.T) {
	baseURL := getBaseURL()
	mcpID := "123e4567-e89b-12d3-a456-426614174000"

	payload := map[string]interface{}{
		"name":        "updated-mcp",
		"description": "Updated description",
	}
	jsonData, err := json.Marshal(payload)
	require.NoError(t, err)

	req, err := http.NewRequest("PUT", baseURL+"/api/v1/mcp-servers/"+mcpID, bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestDeleteMCPServerUnauthorized tests that deleting MCP server requires authentication
func TestDeleteMCPServerUnauthorized(t *testing.T) {
	baseURL := getBaseURL()
	mcpID := "123e4567-e89b-12d3-a456-426614174000"

	req, err := http.NewRequest("DELETE", baseURL+"/api/v1/mcp-servers/"+mcpID, nil)
	require.NoError(t, err)

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestVerifyMCPServerUnauthorized tests that verifying MCP server requires authentication
func TestVerifyMCPServerUnauthorized(t *testing.T) {
	baseURL := getBaseURL()
	mcpID := "123e4567-e89b-12d3-a456-426614174000"

	req, err := http.NewRequest("POST", baseURL+"/api/v1/mcp-servers/"+mcpID+"/verify", nil)
	require.NoError(t, err)

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestAddPublicKeyUnauthorized tests that adding public key requires authentication
func TestAddPublicKeyUnauthorized(t *testing.T) {
	baseURL := getBaseURL()
	mcpID := "123e4567-e89b-12d3-a456-426614174000"

	payload := map[string]interface{}{
		"publicKey": "ed25519:AAAA...",
	}
	jsonData, err := json.Marshal(payload)
	require.NoError(t, err)

	req, err := http.NewRequest("POST", baseURL+"/api/v1/mcp-servers/"+mcpID+"/keys", bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestGetVerificationStatusUnauthorized tests that getting verification status requires authentication
func TestGetVerificationStatusUnauthorized(t *testing.T) {
	baseURL := getBaseURL()
	mcpID := "123e4567-e89b-12d3-a456-426614174000"

	resp, err := http.Get(baseURL + "/api/v1/mcp-servers/" + mcpID + "/verification-status")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestGetMCPServerCapabilitiesUnauthorized tests that getting capabilities requires authentication
func TestGetMCPServerCapabilitiesUnauthorized(t *testing.T) {
	baseURL := getBaseURL()
	mcpID := "123e4567-e89b-12d3-a456-426614174000"

	resp, err := http.Get(baseURL + "/api/v1/mcp-servers/" + mcpID + "/capabilities")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestGetMCPServerAgentsUnauthorized tests that getting MCP server agents requires authentication
func TestGetMCPServerAgentsUnauthorized(t *testing.T) {
	baseURL := getBaseURL()
	mcpID := "123e4567-e89b-12d3-a456-426614174000"

	resp, err := http.Get(baseURL + "/api/v1/mcp-servers/" + mcpID + "/agents")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestVerifyMCPActionUnauthorized tests that verifying MCP action requires authentication
func TestVerifyMCPActionUnauthorized(t *testing.T) {
	baseURL := getBaseURL()
	mcpID := "123e4567-e89b-12d3-a456-426614174000"

	payload := map[string]interface{}{
		"action":     "mcp.tools.call",
		"parameters": map[string]interface{}{},
	}
	jsonData, err := json.Marshal(payload)
	require.NoError(t, err)

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/mcp-servers/%s/verify-action", baseURL, mcpID), bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}
