package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestConfig holds configuration for integration tests
type TestConfig struct {
	BaseURL      string
	AdminEmail   string
	AdminPassword string
	TestTimeout  time.Duration
}

// GetTestConfig returns test configuration from environment or defaults
func GetTestConfig() *TestConfig {
	baseURL := os.Getenv("TEST_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	return &TestConfig{
		BaseURL:      baseURL,
		AdminEmail:   "admin@opena2a.org",
		AdminPassword: "AIM2025!Secure",
		TestTimeout:  30 * time.Second,
	}
}

// TestContext holds test state and helpers
type TestContext struct {
	Config      *TestConfig
	AdminToken  string
	UserToken   string
	Client      *http.Client
	T           *testing.T
}

// NewTestContext creates a new test context
func NewTestContext(t *testing.T) *TestContext {
	return &TestContext{
		Config: GetTestConfig(),
		Client: &http.Client{
			Timeout: 30 * time.Second,
		},
		T: t,
	}
}

// WaitForBackend waits for backend to be ready
func (tc *TestContext) WaitForBackend() error {
	maxRetries := 30
	for i := 0; i < maxRetries; i++ {
		resp, err := tc.Client.Get(tc.Config.BaseURL + "/health")
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("backend not ready after %d retries", maxRetries)
}

// LoginAsAdmin logs in as admin and stores token
func (tc *TestContext) LoginAsAdmin() error {
	body := map[string]interface{}{
		"email":    tc.Config.AdminEmail,
		"password": tc.Config.AdminPassword,
	}

	// Use public login endpoint (doesn't require authentication)
	resp, err := tc.Post("/api/v1/public/login", body, "")
	if err != nil {
		return fmt.Errorf("admin login failed: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		return fmt.Errorf("failed to parse admin login response: %w", err)
	}

	token, ok := result["accessToken"].(string)
	if !ok {
		return fmt.Errorf("no access token in admin login response")
	}

	tc.AdminToken = token
	return nil
}

// CreateTestUser creates a test user and returns token
func (tc *TestContext) CreateTestUser(email, password string) (string, error) {
	body := map[string]interface{}{
		"email":    email,
		"password": password,
		"name":     "Test User",
	}

	// Use public registration endpoint (doesn't require authentication)
	resp, err := tc.Post("/api/v1/public/register", body, "")
	if err != nil {
		return "", fmt.Errorf("user registration failed: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", fmt.Errorf("failed to parse registration response: %w", err)
	}

	// Note: Public registration requires admin approval, so it won't return a token immediately
	// For tests, we should use admin token to create users directly or approve the registration
	token, ok := result["accessToken"].(string)
	if !ok {
		// Registration pending approval - not an error in production flow
		// For testing, we'll need to handle this differently
		return "", fmt.Errorf("registration pending approval (expected for public registration)")
	}

	return token, nil
}

// Post makes a POST request
func (tc *TestContext) Post(path string, body interface{}, token string) ([]byte, error) {
	return tc.doRequest("POST", path, body, token)
}

// Get makes a GET request
func (tc *TestContext) Get(path string, token string) ([]byte, error) {
	return tc.doRequest("GET", path, nil, token)
}

// Put makes a PUT request
func (tc *TestContext) Put(path string, body interface{}, token string) ([]byte, error) {
	return tc.doRequest("PUT", path, body, token)
}

// Delete makes a DELETE request
func (tc *TestContext) Delete(path string, token string) ([]byte, error) {
	return tc.doRequest("DELETE", path, nil, token)
}

// doRequest performs HTTP request with optional authentication
func (tc *TestContext) doRequest(method, path string, body interface{}, token string) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequest(method, tc.Config.BaseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := tc.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Return body even on error status for error checking in tests
	if resp.StatusCode >= 400 {
		return respBody, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// AssertStatusCode makes a request and asserts the status code
func (tc *TestContext) AssertStatusCode(method, path string, body interface{}, token string, expectedStatus int) []byte {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		require.NoError(tc.T, err)
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequest(method, tc.Config.BaseURL+path, bodyReader)
	require.NoError(tc.T, err)

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := tc.Client.Do(req)
	require.NoError(tc.T, err)
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	require.NoError(tc.T, err)

	require.Equal(tc.T, expectedStatus, resp.StatusCode, "Response: %s", string(respBody))

	return respBody
}

// CleanupTestData removes test data (agents, users, etc.)
func (tc *TestContext) CleanupTestData() error {
	// In a real implementation, this would clean up test data
	// For now, we rely on database resets between test runs
	return nil
}
