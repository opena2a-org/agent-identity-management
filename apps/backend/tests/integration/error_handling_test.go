package integration

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestErrorResponseFormat verifies error responses have consistent format
func TestErrorResponseFormat(t *testing.T) {
	ensureAIMBackendRunning(t)
	baseURL := getBaseURL()

	// Make a request that will fail
	resp, err := http.Get(baseURL + "/api/v1/agents/nonexistent-agent-id")
	require.NoError(t, err)
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var errorResp map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&errorResp)
		require.NoError(t, err, "Error response should be valid JSON")

		// Error responses should have structured format
		_, hasError := errorResp["error"]
		_, hasMessage := errorResp["message"]

		assert.True(t, hasError || hasMessage,
			"Error response should contain 'error' or 'message' field")
	}
}

// TestNotFoundReturns404 verifies non-existent resources return 404
func TestNotFoundReturns404(t *testing.T) {
	ensureAIMBackendRunning(t)
	baseURL := getBaseURL()

	// Non-existent endpoint
	resp, err := http.Get(baseURL + "/api/v1/nonexistent/endpoint")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode, "Non-existent endpoint should return 404")
}

// TestInvalidJSONReturns400 verifies malformed JSON returns 400
func TestInvalidJSONReturns400(t *testing.T) {
	ensureAIMBackendRunning(t)
	baseURL := getBaseURL()

	invalidJSON := []byte("{invalid json}")

	resp, err := http.Post(baseURL+"/api/v1/agents", "application/json", bytes.NewBuffer(invalidJSON))
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should return 400 for malformed JSON (or 401 if auth required first)
	assert.True(t, resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnauthorized,
		"Invalid JSON should return 400 or 401, got %d", resp.StatusCode)
}

// TestEmptyBodyHandling verifies endpoints handle empty body gracefully
func TestEmptyBodyHandling(t *testing.T) {
	ensureAIMBackendRunning(t)
	baseURL := getBaseURL()

	endpoints := []string{
		"/api/v1/agents",
		"/api/v1/public/login",
		"/api/v1/public/register",
	}

	for _, endpoint := range endpoints {
		t.Run(endpoint, func(t *testing.T) {
			resp, err := http.Post(baseURL+endpoint, "application/json", nil)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Should not return 500 for empty body
			assert.NotEqual(t, http.StatusInternalServerError, resp.StatusCode,
				"Empty body should not cause 500 error for %s", endpoint)
		})
	}
}

// TestMethodNotAllowed verifies correct method returns 405
func TestMethodNotAllowed(t *testing.T) {
	ensureAIMBackendRunning(t)
	baseURL := getBaseURL()

	// PUT to health endpoint (which only supports GET)
	req, err := http.NewRequest("PUT", baseURL+"/health", nil)
	require.NoError(t, err)

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should return 405 Method Not Allowed
	assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode, "Wrong method should return 405")

	// Should include Allow header with permitted methods
	allowHeader := resp.Header.Get("Allow")
	if allowHeader != "" {
		t.Logf("Allowed methods: %s", allowHeader)
	}
}

// TestContentTypeEnforcement verifies content type is checked
func TestContentTypeEnforcement(t *testing.T) {
	ensureAIMBackendRunning(t)
	baseURL := getBaseURL()

	body := []byte(`{"email": "test@example.com", "password": "test"}`)

	// Send JSON body with wrong content type
	req, err := http.NewRequest("POST", baseURL+"/api/v1/public/login", bytes.NewBuffer(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "text/plain")

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should either reject or handle gracefully (not 500)
	assert.NotEqual(t, http.StatusInternalServerError, resp.StatusCode,
		"Wrong content type should not cause 500 error")
}

// TestLargePayloadHandling verifies server handles large payloads
func TestLargePayloadHandling(t *testing.T) {
	ensureAIMBackendRunning(t)
	baseURL := getBaseURL()

	// Create a large payload (1MB)
	largeData := make([]byte, 1024*1024)
	for i := range largeData {
		largeData[i] = 'a'
	}

	payload := map[string]string{
		"data": string(largeData),
	}
	jsonData, _ := json.Marshal(payload)

	resp, err := http.Post(baseURL+"/api/v1/agents", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		t.Logf("Large payload rejected at network level: %v", err)
		return
	}
	defer resp.Body.Close()

	// Should return 413 or 400 for oversized payload, not 500
	assert.True(t, resp.StatusCode == http.StatusRequestEntityTooLarge ||
		resp.StatusCode == http.StatusBadRequest ||
		resp.StatusCode == http.StatusUnauthorized,
		"Large payload should return 413, 400, or 401, got %d", resp.StatusCode)
}

// TestUnicodeHandling verifies server handles unicode correctly
func TestUnicodeHandling(t *testing.T) {
	ensureAIMBackendRunning(t)
	baseURL := getBaseURL()

	payload := map[string]string{
		"email":    "test@example.com",
		"password": "password123",
		"name":     "Test User with unicode: \u4e2d\u6587\u65e5\u672c\u8a9e",
	}
	jsonData, _ := json.Marshal(payload)

	resp, err := http.Post(baseURL+"/api/v1/public/register", "application/json", bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should handle unicode gracefully (not 500)
	assert.NotEqual(t, http.StatusInternalServerError, resp.StatusCode,
		"Unicode content should not cause 500 error")
}

// TestSpecialCharactersInPath verifies path special characters are handled
func TestSpecialCharactersInPath(t *testing.T) {
	ensureAIMBackendRunning(t)
	baseURL := getBaseURL()

	// Test various special characters in path
	specialPaths := []string{
		"/api/v1/agents/../agents",
		"/api/v1/agents/../../etc/passwd",
		"/api/v1/agents/%00",
		"/api/v1/agents/test%20space",
		"/api/v1/agents/test<script>alert(1)</script>",
	}

	for _, path := range specialPaths {
		t.Run(path, func(t *testing.T) {
			resp, err := http.Get(baseURL + path)
			if err != nil {
				t.Logf("Request failed (expected for some paths): %v", err)
				return
			}
			defer resp.Body.Close()

			// Should not return 500
			assert.NotEqual(t, http.StatusInternalServerError, resp.StatusCode,
				"Special characters in path should not cause 500 error")
		})
	}
}

// TestDuplicateHeadersHandling verifies duplicate headers are handled
func TestDuplicateHeadersHandling(t *testing.T) {
	ensureAIMBackendRunning(t)
	baseURL := getBaseURL()

	req, err := http.NewRequest("GET", baseURL+"/health", nil)
	require.NoError(t, err)

	// Add duplicate headers
	req.Header.Add("Authorization", "Bearer token1")
	req.Header.Add("Authorization", "Bearer token2")

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should handle gracefully
	assert.NotEqual(t, http.StatusInternalServerError, resp.StatusCode,
		"Duplicate headers should not cause 500 error")
}

// TestNullBytesInBody verifies null bytes in body are handled
func TestNullBytesInBody(t *testing.T) {
	ensureAIMBackendRunning(t)
	baseURL := getBaseURL()

	bodyWithNull := []byte(`{"email": "test\x00@example.com", "password": "test"}`)

	resp, err := http.Post(baseURL+"/api/v1/public/login", "application/json", bytes.NewBuffer(bodyWithNull))
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should handle gracefully (not 500)
	assert.NotEqual(t, http.StatusInternalServerError, resp.StatusCode,
		"Null bytes in body should not cause 500 error")
}

// TestVeryLongURLHandling verifies very long URLs are handled
func TestVeryLongURLHandling(t *testing.T) {
	ensureAIMBackendRunning(t)
	baseURL := getBaseURL()

	// Create a very long path (8KB)
	longPath := strings.Repeat("a", 8192)

	resp, err := http.Get(baseURL + "/api/v1/" + longPath)
	if err != nil {
		t.Logf("Very long URL rejected at network level: %v", err)
		return
	}
	defer resp.Body.Close()

	// Should return 414 URI Too Long or 404, not 500
	assert.True(t, resp.StatusCode == http.StatusRequestURITooLong ||
		resp.StatusCode == http.StatusNotFound ||
		resp.StatusCode == http.StatusBadRequest,
		"Very long URL should return 414, 404, or 400, got %d", resp.StatusCode)
}

// TestEmptyAuthorizationHeader verifies empty auth header is handled
func TestEmptyAuthorizationHeader(t *testing.T) {
	ensureAIMBackendRunning(t)
	baseURL := getBaseURL()

	req, err := http.NewRequest("GET", baseURL+"/api/v1/agents", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "")

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should return 401, not 500
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
		"Empty Authorization header should return 401")
}

// TestMalformedAuthorizationHeader verifies malformed auth header is handled
func TestMalformedAuthorizationHeader(t *testing.T) {
	ensureAIMBackendRunning(t)
	baseURL := getBaseURL()

	malformedHeaders := []string{
		"Bearer",
		"Bearer ",
		"bearer token",
		"Basic invalid-base64",
		"InvalidScheme token",
	}

	for _, header := range malformedHeaders {
		t.Run(header, func(t *testing.T) {
			req, err := http.NewRequest("GET", baseURL+"/api/v1/agents", nil)
			require.NoError(t, err)
			req.Header.Set("Authorization", header)

			client := &http.Client{}
			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Should return 401, not 500
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
				"Malformed Authorization header '%s' should return 401", header)
		})
	}
}

// TestErrorResponseDoesNotLeakInternals verifies errors don't leak internal info
func TestErrorResponseDoesNotLeakInternals(t *testing.T) {
	ensureAIMBackendRunning(t)
	baseURL := getBaseURL()

	resp, err := http.Get(baseURL + "/api/v1/agents/nonexistent-id")
	require.NoError(t, err)
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		bodyStr := strings.ToLower(string(body))

		// Should not contain internal details
		sensitivePatterns := []string{
			"stack trace",
			"stacktrace",
			"sql",
			"postgres",
			"mysql",
			"redis",
			"/home/",
			"/var/",
			"goroutine",
			"panic",
		}

		for _, pattern := range sensitivePatterns {
			assert.NotContains(t, bodyStr, pattern,
				"Error response should not contain sensitive pattern: %s", pattern)
		}
	}
}
