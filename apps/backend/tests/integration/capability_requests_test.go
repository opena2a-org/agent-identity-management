package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCreateCapabilityRequestUnauthorized tests that creating capability request requires authentication
func TestCreateCapabilityRequestUnauthorized(t *testing.T) {
	baseURL := getBaseURL()

	payload := map[string]interface{}{
		"agent_id":     "123e4567-e89b-12d3-a456-426614174000",
		"capability":   "file_system_write",
		"justification": "Need write access for log files",
	}
	jsonData, err := json.Marshal(payload)
	require.NoError(t, err)

	resp, err := http.Post(baseURL+"/api/v1/capability-requests", "application/json", bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestListCapabilityRequestsUnauthorized tests that listing capability requests requires authentication
func TestListCapabilityRequestsUnauthorized(t *testing.T) {
	baseURL := getBaseURL()

	resp, err := http.Get(baseURL + "/api/v1/admin/capability-requests")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestGetCapabilityRequestUnauthorized tests that getting capability request requires authentication
func TestGetCapabilityRequestUnauthorized(t *testing.T) {
	baseURL := getBaseURL()
	requestID := "123e4567-e89b-12d3-a456-426614174000"

	resp, err := http.Get(baseURL + "/api/v1/admin/capability-requests/" + requestID)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestApproveCapabilityRequestUnauthorized tests that approving capability request requires authentication
func TestApproveCapabilityRequestUnauthorized(t *testing.T) {
	baseURL := getBaseURL()
	requestID := "123e4567-e89b-12d3-a456-426614174000"

	payload := map[string]interface{}{
		"comment": "Approved for production use",
	}
	jsonData, err := json.Marshal(payload)
	require.NoError(t, err)

	resp, err := http.Post(baseURL+"/api/v1/admin/capability-requests/"+requestID+"/approve", "application/json", bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestRejectCapabilityRequestUnauthorized tests that rejecting capability request requires authentication
func TestRejectCapabilityRequestUnauthorized(t *testing.T) {
	baseURL := getBaseURL()
	requestID := "123e4567-e89b-12d3-a456-426614174000"

	payload := map[string]interface{}{
		"reason": "Security concerns - capability too broad",
	}
	jsonData, err := json.Marshal(payload)
	require.NoError(t, err)

	resp, err := http.Post(baseURL+"/api/v1/admin/capability-requests/"+requestID+"/reject", "application/json", bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestCreateCapabilityRequestWithInvalidData tests creating capability request with invalid data
func TestCreateCapabilityRequestWithInvalidData(t *testing.T) {
	baseURL := getBaseURL()

	payload := map[string]interface{}{
		"agent_id": "",
	}
	jsonData, err := json.Marshal(payload)
	require.NoError(t, err)

	resp, err := http.Post(baseURL+"/api/v1/capability-requests", "application/json", bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestListCapabilityRequestsWithParams tests listing capability requests with query parameters
func TestListCapabilityRequestsWithParams(t *testing.T) {
	baseURL := getBaseURL()

	resp, err := http.Get(baseURL + "/api/v1/admin/capability-requests?status=pending&limit=10")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestApproveCapabilityRequestEmptyBody tests approving capability request with empty body
func TestApproveCapabilityRequestEmptyBody(t *testing.T) {
	baseURL := getBaseURL()
	requestID := "123e4567-e89b-12d3-a456-426614174000"

	resp, err := http.Post(baseURL+"/api/v1/admin/capability-requests/"+requestID+"/approve", "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestRejectCapabilityRequestEmptyBody tests rejecting capability request with empty body
func TestRejectCapabilityRequestEmptyBody(t *testing.T) {
	baseURL := getBaseURL()
	requestID := "123e4567-e89b-12d3-a456-426614174000"

	resp, err := http.Post(baseURL+"/api/v1/admin/capability-requests/"+requestID+"/reject", "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestGetCapabilityRequestWithInvalidID tests getting capability request with invalid ID
func TestGetCapabilityRequestWithInvalidID(t *testing.T) {
	baseURL := getBaseURL()

	resp, err := http.Get(baseURL + "/api/v1/admin/capability-requests/invalid-id")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

