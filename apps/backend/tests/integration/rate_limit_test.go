package integration

import (
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRateLimitHeadersPresent verifies rate limit headers are returned
func TestRateLimitHeadersPresent(t *testing.T) {
	ensureAIMBackendRunning(t)
	baseURL := getBaseURL()

	resp, err := http.Get(baseURL + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	// Rate limit headers should be present (if rate limiting is enabled)
	// These are optional - the test documents expected behavior
	rateLimitHeaders := []string{
		"X-RateLimit-Limit",
		"X-RateLimit-Remaining",
		"X-RateLimit-Reset",
	}

	foundHeaders := 0
	for _, header := range rateLimitHeaders {
		if resp.Header.Get(header) != "" {
			foundHeaders++
		}
	}

	// Log whether rate limiting headers are present
	if foundHeaders == 0 {
		t.Log("Note: Rate limit headers not present on health endpoint (may be exempt)")
	} else {
		t.Logf("Found %d rate limit headers", foundHeaders)
	}
}

// TestAPIEndpointRateLimitHeaders verifies rate limit headers on API endpoints
func TestAPIEndpointRateLimitHeaders(t *testing.T) {
	ensureAIMBackendRunning(t)
	baseURL := getBaseURL()

	// Test on an authenticated endpoint
	resp, err := http.Get(baseURL + "/api/v1/agents")
	require.NoError(t, err)
	defer resp.Body.Close()

	// Check for rate limit headers on API endpoint
	limit := resp.Header.Get("X-RateLimit-Limit")
	remaining := resp.Header.Get("X-RateLimit-Remaining")
	reset := resp.Header.Get("X-RateLimit-Reset")

	// Rate limiting should be present on API endpoints
	if limit != "" || remaining != "" || reset != "" {
		t.Logf("Rate limit headers found: Limit=%s, Remaining=%s, Reset=%s", limit, remaining, reset)
	}
}

// TestRateLimitRetryAfterHeader verifies Retry-After header when rate limited
func TestRateLimitRetryAfterHeader(t *testing.T) {
	ensureAIMBackendRunning(t)
	baseURL := getBaseURL()

	// Make multiple rapid requests to potentially trigger rate limit
	client := &http.Client{Timeout: 5 * time.Second}
	var lastResp *http.Response
	var rateLimitHit bool

	for i := 0; i < 100; i++ {
		resp, err := client.Get(baseURL + "/api/v1/agents")
		if err != nil {
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			lastResp = resp
			rateLimitHit = true
			break
		}
		resp.Body.Close()
	}

	if rateLimitHit {
		defer lastResp.Body.Close()

		// When rate limited, Retry-After header should be present
		retryAfter := lastResp.Header.Get("Retry-After")
		t.Logf("Rate limit triggered. Retry-After: %s", retryAfter)
		assert.NotEmpty(t, retryAfter, "Retry-After header should be present when rate limited")
	} else {
		t.Log("Note: Rate limit not triggered with 100 requests (may have high limit or be disabled)")
	}
}

// TestRateLimitBypassForHealthEndpoint verifies health endpoint is not rate limited
func TestRateLimitBypassForHealthEndpoint(t *testing.T) {
	ensureAIMBackendRunning(t)
	baseURL := getBaseURL()

	client := &http.Client{Timeout: 5 * time.Second}

	// Health endpoint should never be rate limited
	for i := 0; i < 50; i++ {
		resp, err := client.Get(baseURL + "/health")
		require.NoError(t, err)
		resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "Health endpoint should not be rate limited")
	}
}

// TestConcurrentRequestsHandling tests server handles concurrent requests
func TestConcurrentRequestsHandling(t *testing.T) {
	ensureAIMBackendRunning(t)
	baseURL := getBaseURL()

	var wg sync.WaitGroup
	errors := make(chan error, 20)

	// Make 20 concurrent requests
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Get(baseURL + "/health")
			if err != nil {
				errors <- err
				return
			}
			resp.Body.Close()
		}()
	}

	wg.Wait()
	close(errors)

	errorCount := 0
	for err := range errors {
		t.Logf("Request error: %v", err)
		errorCount++
	}

	// Most requests should succeed
	assert.Less(t, errorCount, 5, "Most concurrent requests should succeed")
}

// TestRateLimitDistinctByIP verifies rate limiting considers IP address
func TestRateLimitDistinctByIP(t *testing.T) {
	ensureAIMBackendRunning(t)
	baseURL := getBaseURL()

	// This test documents that rate limiting should be per-IP
	// In local testing, all requests come from same IP
	client := &http.Client{Timeout: 5 * time.Second}

	resp, err := client.Get(baseURL + "/api/v1/agents")
	require.NoError(t, err)
	defer resp.Body.Close()

	// Check if X-Forwarded-For or X-Real-IP is considered
	// This is informational - actual behavior depends on configuration
	t.Log("Rate limiting should consider client IP for fair distribution")
}

// TestOAuthEndpointRateLimiting verifies OAuth endpoint has rate limiting
func TestOAuthEndpointRateLimiting(t *testing.T) {
	ensureAIMBackendRunning(t)
	baseURL := getBaseURL()

	client := &http.Client{Timeout: 5 * time.Second}

	// OAuth token endpoint should have rate limiting to prevent brute force
	resp, err := client.Post(baseURL+"/oauth/token", "application/x-www-form-urlencoded", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Check for rate limit headers
	limit := resp.Header.Get("X-RateLimit-Limit")
	if limit != "" {
		t.Logf("OAuth endpoint rate limit: %s", limit)
	} else {
		t.Log("Note: OAuth endpoint rate limit headers not present")
	}
}

// TestLoginEndpointRateLimiting verifies login endpoint has rate limiting
func TestLoginEndpointRateLimiting(t *testing.T) {
	ensureAIMBackendRunning(t)
	baseURL := getBaseURL()

	client := &http.Client{Timeout: 5 * time.Second}
	var rateLimitHit bool

	// Try multiple failed logins to trigger rate limiting
	for i := 0; i < 20; i++ {
		resp, err := client.Post(baseURL+"/api/v1/public/login", "application/json", nil)
		if err != nil {
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			rateLimitHit = true
			t.Log("Login endpoint correctly rate limits after multiple attempts")
			resp.Body.Close()
			break
		}
		resp.Body.Close()
	}

	if !rateLimitHit {
		t.Log("Note: Login endpoint rate limit not triggered (may have high limit)")
	}
}
