package integration

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHealthEndpoint verifies the health check endpoint
func TestHealthEndpoint(t *testing.T) {
	ensureAIMBackendRunning(t) // Skip if AIM backend not running
	baseURL := getBaseURL()

	resp, err := http.Get(baseURL + "/health")
	require.NoError(t, err, "Health endpoint request should not error")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "Health endpoint should return 200 OK")

	var healthResp map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&healthResp)
	require.NoError(t, err, "Health response should be valid JSON")

	assert.Equal(t, "healthy", healthResp["status"], "Health status should be 'healthy'")
	assert.NotEmpty(t, healthResp["time"], "Health response should include time")
}

// TestHealthEndpointWithInvalidMethod verifies health endpoint only accepts GET
func TestHealthEndpointWithInvalidMethod(t *testing.T) {
	ensureAIMBackendRunning(t) // Skip if AIM backend not running
	baseURL := getBaseURL()

	resp, err := http.Post(baseURL+"/health", "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode, "Health endpoint should only accept GET")
}

// ensureAIMBackendRunning skips t unless BACKEND_URL or TEST_BASE_URL names an
// AIM backend that answers (skipUnlessAIMBackend), so neither an unset address
// nor another service on the port (like Jenkins) fails a test.
func ensureAIMBackendRunning(t *testing.T) {
	t.Helper()
	skipUnlessAIMBackend(t, http.DefaultClient, getBaseURL())
}
