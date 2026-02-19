package integration

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminEndpoints(t *testing.T) {
	ensureAIMBackendRunning(t) // Skip if AIM backend not running
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)

	// Wait for backend and login as admin
	require.NoError(t, tc.WaitForBackend())
	require.NoError(t, tc.LoginAsAdmin())

	t.Run("GET /api/v1/admin/users - List all users", func(t *testing.T) {
		respBody := tc.AssertStatusCode("GET", "/api/v1/admin/users", nil, tc.AdminToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Contains(t, result, "users")
		users := result["users"].([]interface{})
		assert.GreaterOrEqual(t, len(users), 1)
	})

	t.Run("GET /api/v1/admin/dashboard/stats - Get system statistics", func(t *testing.T) {
		respBody := tc.AssertStatusCode("GET", "/api/v1/admin/dashboard/stats", nil, tc.AdminToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		// Check for correct field names (camelCase per API convention)
		assert.Contains(t, result, "totalUsers")
		assert.Contains(t, result, "totalAgents")
		assert.Contains(t, result, "organizationId")
	})

	t.Run("PUT /api/v1/admin/users/:id/role - Update user role", func(t *testing.T) {
		// Get admin's own user ID to test role update
		validResp, err := tc.Get("/api/v1/auth/me", tc.AdminToken)
		require.NoError(t, err)

		var validResult map[string]interface{}
		err = json.Unmarshal(validResp, &validResult)
		require.NoError(t, err)

		userID := validResult["id"].(string)

		// Confirm admin role (update to same role to verify endpoint works)
		path := fmt.Sprintf("/api/v1/admin/users/%s/role", userID)
		body := map[string]interface{}{
			"role": "admin",
		}

		respBody := tc.AssertStatusCode("PUT", path, body, tc.AdminToken, 200)

		var result map[string]interface{}
		err = json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Equal(t, "admin", result["role"])
	})

	t.Run("DELETE /api/v1/admin/users/:id - Deactivate and delete user", func(t *testing.T) {
		// Create a test user (will be in admin's org)
		email := fmt.Sprintf("delete-test-%d@example.com", time.Now().Unix())
		_, err := tc.CreateTestUser(email, "TestPass123!")
		require.NoError(t, err)

		// Find the user ID from admin user list
		usersResp := tc.AssertStatusCode("GET", "/api/v1/admin/users", nil, tc.AdminToken, 200)
		var usersResult map[string]interface{}
		require.NoError(t, json.Unmarshal(usersResp, &usersResult))

		users := usersResult["users"].([]interface{})
		var targetUserID string
		for _, u := range users {
			user := u.(map[string]interface{})
			if user["email"] == email {
				targetUserID = user["id"].(string)
				break
			}
		}
		require.NotEmpty(t, targetUserID, "Should find the created user in admin user list")

		// Step 1: Deactivate the user (required before permanent deletion)
		deactivatePath := fmt.Sprintf("/api/v1/admin/users/%s/deactivate", targetUserID)
		tc.AssertStatusCode("POST", deactivatePath, nil, tc.AdminToken, 200)

		// Step 2: Permanently delete the deactivated user
		deletePath := fmt.Sprintf("/api/v1/admin/users/%s", targetUserID)
		tc.AssertStatusCode("DELETE", deletePath, nil, tc.AdminToken, 200)
	})

	t.Run("GET /api/v1/admin/audit-logs - Get audit logs", func(t *testing.T) {
		respBody := tc.AssertStatusCode("GET", "/api/v1/admin/audit-logs", nil, tc.AdminToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Contains(t, result, "logs")
	})

	t.Run("Admin endpoints - Require admin role", func(t *testing.T) {
		// Create regular user
		email := fmt.Sprintf("regular-user-%d@example.com", time.Now().Unix())
		userToken, err := tc.CreateTestUser(email, "TestPass123!")
		require.NoError(t, err)

		// Try to access admin endpoint
		tc.AssertStatusCode("GET", "/api/v1/admin/users", nil, userToken, 403)
	})
}

func TestSecurityEndpoints(t *testing.T) {
	ensureAIMBackendRunning(t) // Skip if AIM backend not running
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)

	// Wait for backend and login as admin (security endpoints require manager/admin role)
	require.NoError(t, tc.WaitForBackend())
	require.NoError(t, tc.LoginAsAdmin())
	userToken := tc.AdminToken

	t.Run("GET /api/v1/security/alerts - Get security alerts", func(t *testing.T) {
		respBody := tc.AssertStatusCode("GET", "/api/v1/security/alerts", nil, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Contains(t, result, "alerts")
	})

	t.Run("GET /api/v1/security/threats - Get threat detection results", func(t *testing.T) {
		respBody := tc.AssertStatusCode("GET", "/api/v1/security/threats", nil, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Contains(t, result, "threats")
	})

	t.Run("GET /api/v1/security/dashboard - Get security dashboard", func(t *testing.T) {
		respBody := tc.AssertStatusCode("GET", "/api/v1/security/dashboard", nil, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Contains(t, result, "metrics")
		assert.Contains(t, result, "agents")
	})
}

func TestAnalyticsEndpoints(t *testing.T) {
	ensureAIMBackendRunning(t) // Skip if AIM backend not running
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)

	// Wait for backend and login as admin (analytics endpoints require manager/admin role)
	require.NoError(t, tc.WaitForBackend())
	require.NoError(t, tc.LoginAsAdmin())
	userToken := tc.AdminToken

	t.Run("GET /api/v1/analytics/dashboard - Get dashboard data", func(t *testing.T) {
		respBody := tc.AssertStatusCode("GET", "/api/v1/analytics/dashboard", nil, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Contains(t, result, "totalAgents")
		assert.Contains(t, result, "verifiedAgents")
		assert.Contains(t, result, "totalUsers")
	})

	t.Run("GET /api/v1/analytics/usage - Get usage statistics", func(t *testing.T) {
		respBody := tc.AssertStatusCode("GET", "/api/v1/analytics/usage?period=week", nil, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Contains(t, result, "period")
		assert.Contains(t, result, "totalAgents")
	})

	t.Run("GET /api/v1/analytics/trends - Get trust score trends", func(t *testing.T) {
		respBody := tc.AssertStatusCode("GET", "/api/v1/analytics/trends", nil, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.NotNil(t, result)
	})

	t.Run("GET /api/v1/analytics/activity - Get activity summary", func(t *testing.T) {
		respBody := tc.AssertStatusCode("GET", "/api/v1/analytics/activity", nil, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Contains(t, result, "period")
		assert.Contains(t, result, "summary")
	})
}
