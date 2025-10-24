package integration

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOAuthGoogleInitiation verifies Google OAuth login endpoint
func TestOAuthGoogleInitiation(t *testing.T) {
	baseURL := getBaseURL()

	resp, err := http.Get(baseURL + "/api/v1/auth/login/google")
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should return JSON with redirect_url
	assert.Equal(t, http.StatusOK, resp.StatusCode, "Should return 200 OK")

	var loginResp map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&loginResp)
	require.NoError(t, err, "Response should be valid JSON")

	redirectURL, ok := loginResp["redirect_url"].(string)
	require.True(t, ok, "Response should contain redirect_url")
	assert.Contains(t, redirectURL, "accounts.google.com", "Should redirect to Google OAuth URL")
	assert.Contains(t, redirectURL, "oauth2", "Should be OAuth2 flow")
}

// TestOAuthMicrosoftInitiation verifies Microsoft OAuth login endpoint
func TestOAuthMicrosoftInitiation(t *testing.T) {
	baseURL := getBaseURL()

	resp, err := http.Get(baseURL + "/api/v1/auth/login/microsoft")
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should return JSON with redirect_url or error if provider not configured
	if resp.StatusCode == http.StatusOK {
		var loginResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&loginResp)
		require.NoError(t, err, "Response should be valid JSON")

		redirectURL, ok := loginResp["redirect_url"].(string)
		require.True(t, ok, "Response should contain redirect_url")
		assert.Contains(t, redirectURL, "microsoft", "Should redirect to Microsoft OAuth URL")
	} else {
		// Provider not configured is acceptable in test environment
		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode, "Should return 500 if provider not configured")
	}
}

// TestMeEndpointUnauthorized verifies /me endpoint requires authentication
func TestMeEndpointUnauthorized(t *testing.T) {
	baseURL := getBaseURL()

	resp, err := http.Get(baseURL + "/api/v1/auth/me")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestLogoutEndpoint verifies logout endpoint clears authentication
func TestLogoutEndpoint(t *testing.T) {
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

// TestInvalidOAuthProvider verifies invalid OAuth provider returns error
func TestInvalidOAuthProvider(t *testing.T) {
	baseURL := getBaseURL()

	resp, err := http.Get(baseURL + "/api/v1/auth/login/invalid-provider")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "Should return 400 for invalid provider")

	var errorResp map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&errorResp)
	require.NoError(t, err, "Response should be valid JSON")

	errorMsg, ok := errorResp["error"].(string)
	require.True(t, ok, "Response should contain error message")
	assert.Equal(t, "Invalid OAuth provider", errorMsg, "Should return proper error message")
}
