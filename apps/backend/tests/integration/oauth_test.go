package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOAuthTokenEndpointExists verifies OAuth token endpoint is available
func TestOAuthTokenEndpointExists(t *testing.T) {
	ensureAIMBackendRunning(t)
	baseURL := getBaseURL()

	// Empty request should fail with validation error, not 404
	resp, err := http.Post(baseURL+"/oauth/token", "application/x-www-form-urlencoded", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.NotEqual(t, http.StatusNotFound, resp.StatusCode, "OAuth token endpoint should exist")
}

// TestOAuthTokenWithInvalidGrantType tests token request with invalid grant type
func TestOAuthTokenWithInvalidGrantType(t *testing.T) {
	ensureAIMBackendRunning(t)
	baseURL := getBaseURL()

	data := url.Values{}
	data.Set("grant_type", "invalid_grant")
	data.Set("client_id", "test-client")

	resp, err := http.Post(baseURL+"/oauth/token", "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should return 400 Bad Request for invalid grant type
	assert.True(t, resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnauthorized,
		"Should return 400 or 401 for invalid grant type, got %d", resp.StatusCode)
}

// TestOAuthTokenWithMissingClientId tests token request without client_id
func TestOAuthTokenWithMissingClientId(t *testing.T) {
	ensureAIMBackendRunning(t)
	baseURL := getBaseURL()

	data := url.Values{}
	data.Set("grant_type", "client_credentials")

	resp, err := http.Post(baseURL+"/oauth/token", "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should return 400 or 401 for missing client_id
	assert.True(t, resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnauthorized,
		"Should return 400 or 401 for missing client_id, got %d", resp.StatusCode)
}

// TestOAuthTokenWithInvalidClientAssertion tests token request with invalid client assertion
func TestOAuthTokenWithInvalidClientAssertion(t *testing.T) {
	ensureAIMBackendRunning(t)
	baseURL := getBaseURL()

	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", "nonexistent-agent-id")
	data.Set("client_assertion_type", "urn:ietf:params:oauth:client-assertion-type:jwt-bearer")
	data.Set("client_assertion", "invalid.jwt.token")

	resp, err := http.Post(baseURL+"/oauth/token", "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should return 401 for invalid client assertion
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 for invalid client assertion")

	var errorResp map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&errorResp)
	require.NoError(t, err, "Response should be valid JSON")

	// OAuth error response should have "error" field
	_, hasError := errorResp["error"]
	assert.True(t, hasError, "Response should contain 'error' field")
}

// TestOAuthTokenWithMalformedAssertion tests token request with malformed JWT
func TestOAuthTokenWithMalformedAssertion(t *testing.T) {
	ensureAIMBackendRunning(t)
	baseURL := getBaseURL()

	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", "test-agent-id")
	data.Set("client_assertion_type", "urn:ietf:params:oauth:client-assertion-type:jwt-bearer")
	data.Set("client_assertion", "not-a-jwt")

	resp, err := http.Post(baseURL+"/oauth/token", "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should return 400 or 401 for malformed assertion
	assert.True(t, resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnauthorized,
		"Should return 400 or 401 for malformed assertion, got %d", resp.StatusCode)
}

// TestOAuthTokenWithUnsupportedAssertionType tests unsupported assertion type
func TestOAuthTokenWithUnsupportedAssertionType(t *testing.T) {
	ensureAIMBackendRunning(t)
	baseURL := getBaseURL()

	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", "test-agent-id")
	data.Set("client_assertion_type", "unsupported-type")
	data.Set("client_assertion", "some-assertion")

	resp, err := http.Post(baseURL+"/oauth/token", "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should return 400 for unsupported assertion type
	assert.True(t, resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnauthorized,
		"Should return 400 or 401 for unsupported assertion type, got %d", resp.StatusCode)
}

// TestOAuthTokenContentType tests that endpoint accepts correct content type
func TestOAuthTokenContentType(t *testing.T) {
	ensureAIMBackendRunning(t)
	baseURL := getBaseURL()

	// Test with JSON content type (should be rejected or handled)
	jsonBody := map[string]string{
		"grant_type": "client_credentials",
		"client_id":  "test-client",
	}
	jsonData, _ := json.Marshal(jsonBody)

	resp, err := http.Post(baseURL+"/oauth/token", "application/json", bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	defer resp.Body.Close()

	// OAuth spec requires application/x-www-form-urlencoded
	// Server might accept JSON but form-urlencoded is standard
	assert.NotEqual(t, http.StatusNotFound, resp.StatusCode, "Endpoint should exist")
}

// TestOAuthTokenEmptyBody tests endpoint with empty body
func TestOAuthTokenEmptyBody(t *testing.T) {
	ensureAIMBackendRunning(t)
	baseURL := getBaseURL()

	resp, err := http.Post(baseURL+"/oauth/token", "application/x-www-form-urlencoded", strings.NewReader(""))
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should return 400 for empty request
	assert.True(t, resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnauthorized,
		"Should return 400 or 401 for empty body, got %d", resp.StatusCode)
}

// TestOAuthTokenResponseFormat verifies error response follows OAuth spec
func TestOAuthTokenResponseFormat(t *testing.T) {
	ensureAIMBackendRunning(t)
	baseURL := getBaseURL()

	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", "invalid-client")
	data.Set("client_assertion_type", "urn:ietf:params:oauth:client-assertion-type:jwt-bearer")
	data.Set("client_assertion", "invalid.jwt.assertion")

	resp, err := http.Post(baseURL+"/oauth/token", "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
	require.NoError(t, err)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errorResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&errorResp)
		require.NoError(t, err, "Error response should be valid JSON")

		// OAuth 2.0 error responses should have these fields
		_, hasError := errorResp["error"]
		assert.True(t, hasError, "OAuth error response should contain 'error' field")
	}
}
