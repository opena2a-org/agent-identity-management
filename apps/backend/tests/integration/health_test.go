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
	baseURL := getBaseURL()

	resp, err := http.Post(baseURL+"/health", "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode, "Health endpoint should only accept GET")
}

// getBaseURL returns the base URL for the API server
// Override with BACKEND_URL environment variable
func getBaseURL() string {
	// Default to localhost:8080
	// In CI/CD, set BACKEND_URL environment variable
	return "http://localhost:8080"
}
