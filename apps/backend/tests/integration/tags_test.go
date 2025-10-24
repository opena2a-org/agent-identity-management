package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetTagsUnauthorized tests that getting tags requires authentication
func TestGetTagsUnauthorized(t *testing.T) {
	baseURL := getBaseURL()

	resp, err := http.Get(baseURL + "/api/v1/tags")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestCreateTagUnauthorized tests that creating tag requires authentication
func TestCreateTagUnauthorized(t *testing.T) {
	baseURL := getBaseURL()

	payload := map[string]interface{}{
		"name":        "production",
		"description": "Production environment tag",
		"color":       "#FF5733",
	}
	jsonData, err := json.Marshal(payload)
	require.NoError(t, err)

	resp, err := http.Post(baseURL+"/api/v1/tags", "application/json", bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestDeleteTagUnauthorized tests that deleting tag requires authentication
func TestDeleteTagUnauthorized(t *testing.T) {
	baseURL := getBaseURL()
	tagID := "123e4567-e89b-12d3-a456-426614174000"

	req, err := http.NewRequest("DELETE", baseURL+"/api/v1/tags/"+tagID, nil)
	require.NoError(t, err)

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestGetAgentTagsUnauthorized tests that getting agent tags requires authentication
func TestGetAgentTagsUnauthorized(t *testing.T) {
	baseURL := getBaseURL()
	agentID := "123e4567-e89b-12d3-a456-426614174000"

	resp, err := http.Get(baseURL + "/api/v1/agents/" + agentID + "/tags")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestAddTagsToAgentUnauthorized tests that adding tags to agent requires authentication
func TestAddTagsToAgentUnauthorized(t *testing.T) {
	baseURL := getBaseURL()
	agentID := "123e4567-e89b-12d3-a456-426614174000"

	payload := map[string]interface{}{
		"tag_ids": []string{"123e4567-e89b-12d3-a456-426614174000"},
	}
	jsonData, err := json.Marshal(payload)
	require.NoError(t, err)

	resp, err := http.Post(baseURL+"/api/v1/agents/"+agentID+"/tags", "application/json", bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestRemoveTagFromAgentUnauthorized tests that removing tag from agent requires authentication
func TestRemoveTagFromAgentUnauthorized(t *testing.T) {
	baseURL := getBaseURL()
	agentID := "123e4567-e89b-12d3-a456-426614174000"
	tagID := "123e4567-e89b-12d3-a456-426614174001"

	req, err := http.NewRequest("DELETE", baseURL+"/api/v1/agents/"+agentID+"/tags/"+tagID, nil)
	require.NoError(t, err)

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestSuggestTagsForAgentUnauthorized tests that suggesting tags for agent requires authentication
func TestSuggestTagsForAgentUnauthorized(t *testing.T) {
	baseURL := getBaseURL()
	agentID := "123e4567-e89b-12d3-a456-426614174000"

	resp, err := http.Get(baseURL + "/api/v1/agents/" + agentID + "/tags/suggestions")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestGetMCPServerTagsUnauthorized tests that getting MCP server tags requires authentication
func TestGetMCPServerTagsUnauthorized(t *testing.T) {
	baseURL := getBaseURL()
	mcpID := "123e4567-e89b-12d3-a456-426614174000"

	resp, err := http.Get(baseURL + "/api/v1/mcp-servers/" + mcpID + "/tags")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestAddTagsToMCPServerUnauthorized tests that adding tags to MCP server requires authentication
func TestAddTagsToMCPServerUnauthorized(t *testing.T) {
	baseURL := getBaseURL()
	mcpID := "123e4567-e89b-12d3-a456-426614174000"

	payload := map[string]interface{}{
		"tag_ids": []string{"123e4567-e89b-12d3-a456-426614174000"},
	}
	jsonData, err := json.Marshal(payload)
	require.NoError(t, err)

	resp, err := http.Post(baseURL+"/api/v1/mcp-servers/"+mcpID+"/tags", "application/json", bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestRemoveTagFromMCPServerUnauthorized tests that removing tag from MCP server requires authentication
func TestRemoveTagFromMCPServerUnauthorized(t *testing.T) {
	baseURL := getBaseURL()
	mcpID := "123e4567-e89b-12d3-a456-426614174000"
	tagID := "123e4567-e89b-12d3-a456-426614174001"

	req, err := http.NewRequest("DELETE", baseURL+"/api/v1/mcp-servers/"+mcpID+"/tags/"+tagID, nil)
	require.NoError(t, err)

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestSuggestTagsForMCPServerUnauthorized tests that suggesting tags for MCP server requires authentication
func TestSuggestTagsForMCPServerUnauthorized(t *testing.T) {
	baseURL := getBaseURL()
	mcpID := "123e4567-e89b-12d3-a456-426614174000"

	resp, err := http.Get(baseURL + "/api/v1/mcp-servers/" + mcpID + "/tags/suggestions")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestCreateTagWithInvalidData tests creating tag with invalid data
func TestCreateTagWithInvalidData(t *testing.T) {
	baseURL := getBaseURL()

	payload := map[string]interface{}{
		"name": "",
	}
	jsonData, err := json.Marshal(payload)
	require.NoError(t, err)

	resp, err := http.Post(baseURL+"/api/v1/tags", "application/json", bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestAddTagsToAgentWithEmptyArray tests adding empty tag array to agent
func TestAddTagsToAgentWithEmptyArray(t *testing.T) {
	baseURL := getBaseURL()
	agentID := "123e4567-e89b-12d3-a456-426614174000"

	payload := map[string]interface{}{
		"tag_ids": []string{},
	}
	jsonData, err := json.Marshal(payload)
	require.NoError(t, err)

	resp, err := http.Post(baseURL+"/api/v1/agents/"+agentID+"/tags", "application/json", bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

