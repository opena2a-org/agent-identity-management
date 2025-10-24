package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetAgentCapabilitiesUnauthorized tests that getting agent capabilities requires authentication
func TestGetAgentCapabilitiesUnauthorized(t *testing.T) {
	baseURL := getBaseURL()
	agentID := "123e4567-e89b-12d3-a456-426614174000"

	resp, err := http.Get(baseURL + "/api/v1/agents/" + agentID + "/capabilities")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestGrantCapabilityUnauthorized tests that granting capability requires authentication
func TestGrantCapabilityUnauthorized(t *testing.T) {
	baseURL := getBaseURL()
	agentID := "123e4567-e89b-12d3-a456-426614174000"

	payload := map[string]interface{}{
		"capability": "file_system_read",
		"reason":     "Required for project analysis",
	}
	jsonData, err := json.Marshal(payload)
	require.NoError(t, err)

	resp, err := http.Post(baseURL+"/api/v1/agents/"+agentID+"/capabilities", "application/json", bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestRevokeCapabilityUnauthorized tests that revoking capability requires authentication
func TestRevokeCapabilityUnauthorized(t *testing.T) {
	baseURL := getBaseURL()
	agentID := "123e4567-e89b-12d3-a456-426614174000"
	capabilityID := "123e4567-e89b-12d3-a456-426614174001"

	req, err := http.NewRequest("DELETE", baseURL+"/api/v1/agents/"+agentID+"/capabilities/"+capabilityID, nil)
	require.NoError(t, err)

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestGetViolationsByAgentUnauthorized tests that getting violations requires authentication
func TestGetViolationsByAgentUnauthorized(t *testing.T) {
	baseURL := getBaseURL()
	agentID := "123e4567-e89b-12d3-a456-426614174000"

	resp, err := http.Get(baseURL + "/api/v1/agents/" + agentID + "/violations")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestGrantCapabilityWithInvalidData tests granting capability with invalid data
func TestGrantCapabilityWithInvalidData(t *testing.T) {
	baseURL := getBaseURL()
	agentID := "123e4567-e89b-12d3-a456-426614174000"

	payload := map[string]interface{}{
		"capability": "",
	}
	jsonData, err := json.Marshal(payload)
	require.NoError(t, err)

	resp, err := http.Post(baseURL+"/api/v1/agents/"+agentID+"/capabilities", "application/json", bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestGrantMultipleCapabilities tests granting multiple capabilities
func TestGrantMultipleCapabilities(t *testing.T) {
	baseURL := getBaseURL()
	agentID := "123e4567-e89b-12d3-a456-426614174000"

	payload := map[string]interface{}{
		"capabilities": []string{"file_system_read", "network_access"},
		"reason":       "Required for data synchronization",
	}
	jsonData, err := json.Marshal(payload)
	require.NoError(t, err)

	resp, err := http.Post(baseURL+"/api/v1/agents/"+agentID+"/capabilities", "application/json", bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestGetViolationsByAgentWithParams tests getting violations with query parameters
func TestGetViolationsByAgentWithParams(t *testing.T) {
	baseURL := getBaseURL()
	agentID := "123e4567-e89b-12d3-a456-426614174000"

	resp, err := http.Get(baseURL + "/api/v1/agents/" + agentID + "/violations?limit=10&status=pending")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestRevokeCapabilityWithInvalidID tests revoking capability with invalid ID
func TestRevokeCapabilityWithInvalidID(t *testing.T) {
	baseURL := getBaseURL()
	agentID := "123e4567-e89b-12d3-a456-426614174000"

	req, err := http.NewRequest("DELETE", baseURL+"/api/v1/agents/"+agentID+"/capabilities/invalid-id", nil)
	require.NoError(t, err)

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestGetAgentCapabilitiesWithInvalidAgentID tests getting capabilities with invalid agent ID
func TestGetAgentCapabilitiesWithInvalidAgentID(t *testing.T) {
	baseURL := getBaseURL()

	resp, err := http.Get(baseURL + "/api/v1/agents/invalid-id/capabilities")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

