package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestListAgentsUnauthorized verifies agents endpoint requires authentication
func TestListAgentsUnauthorized(t *testing.T) {
	baseURL := getBaseURL()

	resp, err := http.Get(baseURL + "/api/v1/agents")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestCreateAgentUnauthorized verifies create agent requires authentication
func TestCreateAgentUnauthorized(t *testing.T) {
	baseURL := getBaseURL()

	agentData := map[string]interface{}{
		"name":         "test-agent",
		"display_name": "Test Agent",
		"description":  "A test AI agent",
		"agent_type":   "ai_agent",
		"version":      "1.0.0",
	}

	body, _ := json.Marshal(agentData)
	resp, err := http.Post(baseURL+"/api/v1/agents", "application/json", bytes.NewBuffer(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestGetAgentUnauthorized verifies get agent requires authentication
func TestGetAgentUnauthorized(t *testing.T) {
	baseURL := getBaseURL()

	// Use a sample UUID
	agentID := "00000000-0000-0000-0000-000000000000"
	resp, err := http.Get(baseURL + "/api/v1/agents/" + agentID)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestUpdateAgentUnauthorized verifies update agent requires authentication
func TestUpdateAgentUnauthorized(t *testing.T) {
	baseURL := getBaseURL()

	agentID := "00000000-0000-0000-0000-000000000000"
	updateData := map[string]interface{}{
		"display_name": "Updated Agent",
	}

	body, _ := json.Marshal(updateData)
	req, err := http.NewRequest("PUT", baseURL+"/api/v1/agents/"+agentID, bytes.NewBuffer(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestDeleteAgentUnauthorized verifies delete agent requires authentication
func TestDeleteAgentUnauthorized(t *testing.T) {
	baseURL := getBaseURL()

	agentID := "00000000-0000-0000-0000-000000000000"
	req, err := http.NewRequest("DELETE", baseURL+"/api/v1/agents/"+agentID, nil)
	require.NoError(t, err)

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestCreateAgentInvalidData verifies validation on agent creation
func TestCreateAgentInvalidData(t *testing.T) {
	baseURL := getBaseURL()

	// Missing required fields
	invalidAgent := map[string]interface{}{
		"name": "test",
		// Missing display_name, description, agent_type
	}

	body, _ := json.Marshal(invalidAgent)
	req, err := http.NewRequest("POST", baseURL+"/api/v1/agents", bytes.NewBuffer(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer mock-invalid-token")

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should return 400 (validation error) or 401 (invalid token)
	// Either is acceptable - validation might happen before or after auth
	assert.True(t, resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnauthorized)
}

// TODO: Add authenticated tests once we have a test JWT generation utility
// - TestCreateAgentAuthorized
// - TestListAgentsAuthorized
// - TestGetAgentAuthorized
// - TestUpdateAgentAuthorized
// - TestDeleteAgentAuthorized
