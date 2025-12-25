package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLocalLoginWithInvalidCredentials verifies local login rejects invalid credentials
func TestLocalLoginWithInvalidCredentials(t *testing.T) {
	ensureAIMBackendRunning(t) // Skip if AIM backend not running
	baseURL := getBaseURL()

	loginReq := map[string]string{
		"email":    "nonexistent@example.com",
		"password": "wrongpassword",
	}
	body, _ := json.Marshal(loginReq)

	resp, err := http.Post(baseURL+"/api/v1/auth/login/local", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should return 401 for invalid credentials
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 for invalid credentials")
}

// TestLocalLoginWithEmptyCredentials verifies local login validates input
func TestLocalLoginWithEmptyCredentials(t *testing.T) {
	ensureAIMBackendRunning(t) // Skip if AIM backend not running
	baseURL := getBaseURL()

	loginReq := map[string]string{
		"email":    "",
		"password": "",
	}
	body, _ := json.Marshal(loginReq)

	resp, err := http.Post(baseURL+"/api/v1/auth/login/local", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should return 400 for empty credentials or 401
	assert.True(t, resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnauthorized,
		"Should return 400 or 401 for empty credentials, got %d", resp.StatusCode)
}

// TestMeEndpointUnauthorized verifies /me endpoint requires authentication
func TestMeEndpointUnauthorized(t *testing.T) {
	ensureAIMBackendRunning(t) // Skip if AIM backend not running
	baseURL := getBaseURL()

	resp, err := http.Get(baseURL + "/api/v1/auth/me")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestLogoutEndpoint verifies logout endpoint works
func TestLogoutEndpoint(t *testing.T) {
	ensureAIMBackendRunning(t) // Skip if AIM backend not running
	baseURL := getBaseURL()

	req, err := http.NewRequest("POST", baseURL+"/api/v1/auth/logout", nil)
	require.NoError(t, err)

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Logout does not require authentication - returns success even without token
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Should return 200 OK")

	var logoutResp map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&logoutResp)
	require.NoError(t, err, "Response should be valid JSON")

	message, ok := logoutResp["message"].(string)
	require.True(t, ok, "Response should contain message")
	assert.Equal(t, "Logged out successfully", message, "Should return success message")
}

// TestPublicLoginWithInvalidCredentials verifies public login endpoint
func TestPublicLoginWithInvalidCredentials(t *testing.T) {
	ensureAIMBackendRunning(t) // Skip if AIM backend not running
	baseURL := getBaseURL()

	loginReq := map[string]string{
		"email":    "nonexistent@example.com",
		"password": "wrongpassword",
	}
	body, _ := json.Marshal(loginReq)

	resp, err := http.Post(baseURL+"/api/v1/public/login", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should return 401 for invalid credentials
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 for invalid credentials")
}

// TestPublicRegistrationEndpointExists verifies registration endpoint is accessible
func TestPublicRegistrationEndpointExists(t *testing.T) {
	ensureAIMBackendRunning(t) // Skip if AIM backend not running
	baseURL := getBaseURL()

	// Empty registration should fail validation
	registerReq := map[string]string{}
	body, _ := json.Marshal(registerReq)

	resp, err := http.Post(baseURL+"/api/v1/public/register", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should return 400 for invalid registration data, not 404
	assert.NotEqual(t, http.StatusNotFound, resp.StatusCode, "Registration endpoint should exist")
}
