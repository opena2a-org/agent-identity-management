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

		// Check for correct field names from actual API response
		assert.Contains(t, result, "total_users")
		assert.Contains(t, result, "total_agents")
		assert.Contains(t, result, "organization_id")
	})

	t.Run("PUT /api/v1/admin/users/:id/role - Update user role", func(t *testing.T) {
		// Create a test user first
		email := fmt.Sprintf("role-test-%d@example.com", time.Now().Unix())
		token, err := tc.CreateTestUser(email, "TestPass123!")
		require.NoError(t, err)

		// Get user ID from token validation
		validResp, err := tc.Post("/api/v1/auth/validate", nil, token)
		require.NoError(t, err)

		var validResult map[string]interface{}
		err = json.Unmarshal(validResp, &validResult)
		require.NoError(t, err)

		user := validResult["user"].(map[string]interface{})
		userID := user["id"].(string)

		// Update role to admin
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

	t.Run("DELETE /api/v1/admin/users/:id - Deactivate user", func(t *testing.T) {
		// Create a test user
		email := fmt.Sprintf("deactivate-%d@example.com", time.Now().Unix())
		token, err := tc.CreateTestUser(email, "TestPass123!")
		require.NoError(t, err)

		// Get user ID
		validResp, err := tc.Post("/api/v1/auth/validate", nil, token)
		require.NoError(t, err)

		var validResult map[string]interface{}
		err = json.Unmarshal(validResp, &validResult)
		require.NoError(t, err)

		user := validResult["user"].(map[string]interface{})
		userID := user["id"].(string)

		// Deactivate user
		path := fmt.Sprintf("/api/v1/admin/users/%s", userID)
		tc.AssertStatusCode("DELETE", path, nil, tc.AdminToken, 200)
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
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)

	// Wait for backend and login
	require.NoError(t, tc.WaitForBackend())

	email := fmt.Sprintf("security-test-%d@example.com", time.Now().Unix())
	userToken, err := tc.CreateTestUser(email, "TestPass123!")
	require.NoError(t, err)

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

	t.Run("POST /api/v1/security/scan - Run security scan", func(t *testing.T) {
		body := map[string]interface{}{
			"scanType": "comprehensive",
		}

		respBody := tc.AssertStatusCode("POST", "/api/v1/security/scan", body, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Contains(t, result, "scanID")
	})

	t.Run("GET /api/v1/security/scans/:id - Get scan results", func(t *testing.T) {
		// Run a scan first
		scanBody := map[string]interface{}{
			"scanType": "quick",
		}

		scanResp, err := tc.Post("/api/v1/security/scan", scanBody, userToken)
		require.NoError(t, err)

		var scanResult map[string]interface{}
		err = json.Unmarshal(scanResp, &scanResult)
		require.NoError(t, err)

		scanID := scanResult["scanID"].(string)

		// Get scan results
		path := fmt.Sprintf("/api/v1/security/scans/%s", scanID)
		respBody := tc.AssertStatusCode("GET", path, nil, userToken, 200)

		var result map[string]interface{}
		err = json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Contains(t, result, "scan")
	})

	t.Run("PUT /api/v1/security/alerts/:id/acknowledge - Acknowledge alert", func(t *testing.T) {
		// Get alerts first
		alertsResp, err := tc.Get("/api/v1/security/alerts", userToken)
		require.NoError(t, err)

		var alertsResult map[string]interface{}
		err = json.Unmarshal(alertsResp, &alertsResult)
		require.NoError(t, err)

		alerts := alertsResult["alerts"].([]interface{})
		if len(alerts) > 0 {
			alert := alerts[0].(map[string]interface{})
			alertID := alert["id"].(string)

			path := fmt.Sprintf("/api/v1/security/alerts/%s/acknowledge", alertID)
			respBody := tc.AssertStatusCode("PUT", path, nil, userToken, 200)

			var result map[string]interface{}
			err = json.Unmarshal(respBody, &result)
			require.NoError(t, err)

			assert.Contains(t, result, "acknowledged")
		}
	})
}

func TestAnalyticsEndpoints(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)

	// Wait for backend and login
	require.NoError(t, tc.WaitForBackend())

	email := fmt.Sprintf("analytics-test-%d@example.com", time.Now().Unix())
	userToken, err := tc.CreateTestUser(email, "TestPass123!")
	require.NoError(t, err)

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
		assert.Contains(t, result, "data")
	})

	t.Run("GET /api/v1/analytics/trust-trends - Get trust score trends", func(t *testing.T) {
		respBody := tc.AssertStatusCode("GET", "/api/v1/analytics/trust-trends", nil, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Contains(t, result, "trends")
	})

	t.Run("GET /api/v1/analytics/agent-distribution - Get agent distribution", func(t *testing.T) {
		respBody := tc.AssertStatusCode("GET", "/api/v1/analytics/agent-distribution", nil, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Contains(t, result, "distribution")
	})

	t.Run("GET /api/v1/analytics/top-agents - Get top performing agents", func(t *testing.T) {
		respBody := tc.AssertStatusCode("GET", "/api/v1/analytics/top-agents?limit=10", nil, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Contains(t, result, "agents")
	})

	t.Run("GET /api/v1/analytics/compliance-report - Get compliance report", func(t *testing.T) {
		respBody := tc.AssertStatusCode("GET", "/api/v1/analytics/compliance-report", nil, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Contains(t, result, "report")
	})
}
