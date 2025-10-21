package integration

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthEndpoints(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)

	// Wait for backend to be ready
	err := tc.WaitForBackend()
	require.NoError(t, err, "Backend should be ready")

	t.Run("POST /api/v1/auth/register - Register new user", func(t *testing.T) {
		email := fmt.Sprintf("test-%d@example.com", time.Now().Unix())
		body := map[string]interface{}{
			"email":    email,
			"password": "TestPass123!",
			"name":     "Integration Test User",
		}

		respBody := tc.AssertStatusCode("POST", "/api/v1/auth/register", body, "", 201)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Contains(t, result, "accessToken")
		assert.Contains(t, result, "user")

		user := result["user"].(map[string]interface{})
		assert.Equal(t, email, user["email"])
		assert.Equal(t, "Integration Test User", user["name"])
	})

	t.Run("POST /api/v1/auth/register - Duplicate email rejected", func(t *testing.T) {
		email := fmt.Sprintf("duplicate-%d@example.com", time.Now().Unix())

		// Create first user
		body := map[string]interface{}{
			"email":    email,
			"password": "TestPass123!",
			"name":     "First User",
		}
		tc.AssertStatusCode("POST", "/api/v1/auth/register", body, "", 201)

		// Try to create duplicate
		tc.AssertStatusCode("POST", "/api/v1/auth/register", body, "", 409)
	})

	t.Run("POST /api/v1/auth/login/local - Login with valid credentials", func(t *testing.T) {
		// Create user first
		email := fmt.Sprintf("login-test-%d@example.com", time.Now().Unix())
		password := "TestPass123!"

		regBody := map[string]interface{}{
			"email":    email,
			"password": password,
			"name":     "Login Test User",
		}
		tc.AssertStatusCode("POST", "/api/v1/auth/register", regBody, "", 201)

		// Now login
		loginBody := map[string]interface{}{
			"email":    email,
			"password": password,
		}

		respBody := tc.AssertStatusCode("POST", "/api/v1/auth/login/local", loginBody, "", 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Contains(t, result, "accessToken")
		assert.Contains(t, result, "user")
	})

	t.Run("POST /api/v1/auth/login/local - Login with invalid password", func(t *testing.T) {
		// Create user first
		email := fmt.Sprintf("invalid-pass-%d@example.com", time.Now().Unix())

		regBody := map[string]interface{}{
			"email":    email,
			"password": "CorrectPass123!",
			"name":     "Invalid Pass Test",
		}
		tc.AssertStatusCode("POST", "/api/v1/auth/register", regBody, "", 201)

		// Try wrong password
		loginBody := map[string]interface{}{
			"email":    email,
			"password": "WrongPass123!",
		}

		tc.AssertStatusCode("POST", "/api/v1/auth/login/local", loginBody, "", 401)
	})

	t.Run("POST /api/v1/auth/login/local - Login with non-existent user", func(t *testing.T) {
		loginBody := map[string]interface{}{
			"email":    "nonexistent@example.com",
			"password": "SomePass123!",
		}

		tc.AssertStatusCode("POST", "/api/v1/auth/login/local", loginBody, "", 401)
	})

	t.Run("POST /api/v1/auth/validate - Validate valid token", func(t *testing.T) {
		// Create user and get token
		email := fmt.Sprintf("validate-%d@example.com", time.Now().Unix())
		token, err := tc.CreateTestUser(email, "TestPass123!")
		require.NoError(t, err)

		// Validate token
		respBody := tc.AssertStatusCode("POST", "/api/v1/auth/validate", nil, token, 200)

		var result map[string]interface{}
		err = json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.True(t, result["valid"].(bool))
		assert.Contains(t, result, "user")
	})

	t.Run("POST /api/v1/auth/validate - Reject invalid token", func(t *testing.T) {
		tc.AssertStatusCode("POST", "/api/v1/auth/validate", nil, "invalid.token.here", 401)
	})

	t.Run("POST /api/v1/auth/refresh - Refresh access token", func(t *testing.T) {
		// Create user and get token
		email := fmt.Sprintf("refresh-%d@example.com", time.Now().Unix())
		token, err := tc.CreateTestUser(email, "TestPass123!")
		require.NoError(t, err)

		// Refresh token
		respBody := tc.AssertStatusCode("POST", "/api/v1/auth/refresh", nil, token, 200)

		var result map[string]interface{}
		err = json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Contains(t, result, "accessToken")
		newToken := result["accessToken"].(string)
		assert.NotEqual(t, token, newToken)
	})

	t.Run("POST /api/v1/auth/change-password - Change password successfully", func(t *testing.T) {
		// Create user
		email := fmt.Sprintf("change-pass-%d@example.com", time.Now().Unix())
		oldPassword := "OldPass123!"
		token, err := tc.CreateTestUser(email, oldPassword)
		require.NoError(t, err)

		// Change password
		changeBody := map[string]interface{}{
			"currentPassword": oldPassword,
			"newPassword":     "NewPass123!",
		}

		tc.AssertStatusCode("POST", "/api/v1/auth/change-password", changeBody, token, 200)

		// Verify old password no longer works
		loginBody := map[string]interface{}{
			"email":    email,
			"password": oldPassword,
		}
		tc.AssertStatusCode("POST", "/api/v1/auth/login/local", loginBody, "", 401)

		// Verify new password works
		loginBody["password"] = "NewPass123!"
		tc.AssertStatusCode("POST", "/api/v1/auth/login/local", loginBody, "", 200)
	})

	t.Run("POST /api/v1/auth/change-password - Reject wrong current password", func(t *testing.T) {
		// Create user
		email := fmt.Sprintf("wrong-current-%d@example.com", time.Now().Unix())
		token, err := tc.CreateTestUser(email, "CorrectPass123!")
		require.NoError(t, err)

		// Try to change with wrong current password
		changeBody := map[string]interface{}{
			"currentPassword": "WrongPass123!",
			"newPassword":     "NewPass123!",
		}

		tc.AssertStatusCode("POST", "/api/v1/auth/change-password", changeBody, token, 401)
	})
}
