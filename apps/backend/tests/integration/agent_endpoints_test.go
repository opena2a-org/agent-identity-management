package integration

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentEndpoints(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)

	// Wait for backend and login
	require.NoError(t, tc.WaitForBackend())

	// Create test user and get token
	email := fmt.Sprintf("agent-test-%d@example.com", time.Now().Unix())
	userToken, err := tc.CreateTestUser(email, "TestPass123!")
	require.NoError(t, err)

	var createdAgentID string

	t.Run("POST /api/v1/agents - Create agent", func(t *testing.T) {
		body := map[string]interface{}{
			"name":        "Test Agent",
			"type":        "ai_agent",
			"description": "Integration test agent",
			"version":     "1.0.0",
		}

		respBody := tc.AssertStatusCode("POST", "/api/v1/agents", body, userToken, 201)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Contains(t, result, "id")
		assert.Equal(t, "Test Agent", result["name"])
		assert.Equal(t, "ai_agent", result["type"])

		createdAgentID = result["id"].(string)
	})

	t.Run("GET /api/v1/agents - List all agents", func(t *testing.T) {
		respBody := tc.AssertStatusCode("GET", "/api/v1/agents", nil, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Contains(t, result, "agents")
		agents := result["agents"].([]interface{})
		assert.GreaterOrEqual(t, len(agents), 1)
	})

	t.Run("GET /api/v1/agents/:id - Get agent by ID", func(t *testing.T) {
		path := fmt.Sprintf("/api/v1/agents/%s", createdAgentID)
		respBody := tc.AssertStatusCode("GET", path, nil, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Equal(t, createdAgentID, result["id"])
		assert.Equal(t, "Test Agent", result["name"])
	})

	t.Run("PUT /api/v1/agents/:id - Update agent", func(t *testing.T) {
		path := fmt.Sprintf("/api/v1/agents/%s", createdAgentID)
		body := map[string]interface{}{
			"name":        "Updated Test Agent",
			"description": "Updated description",
		}

		respBody := tc.AssertStatusCode("PUT", path, body, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Equal(t, "Updated Test Agent", result["name"])
		assert.Equal(t, "Updated description", result["description"])
	})

	t.Run("GET /api/v1/agents/:id/trust-score - Get trust score", func(t *testing.T) {
		path := fmt.Sprintf("/api/v1/agents/%s/trust-score", createdAgentID)
		respBody := tc.AssertStatusCode("GET", path, nil, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Contains(t, result, "trustScore")
		assert.Contains(t, result, "confidence")
		assert.Contains(t, result, "factors")
	})

	t.Run("POST /api/v1/agents/:id/verify - Verify agent", func(t *testing.T) {
		path := fmt.Sprintf("/api/v1/agents/%s/verify", createdAgentID)
		body := map[string]interface{}{
			"publicKey":      "test-public-key",
			"certificateURL": "https://example.com/cert.pem",
		}

		respBody := tc.AssertStatusCode("POST", path, body, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Contains(t, result, "verified")
	})

	t.Run("GET /api/v1/agents/:id/activity - Get agent activity", func(t *testing.T) {
		path := fmt.Sprintf("/api/v1/agents/%s/activity", createdAgentID)
		respBody := tc.AssertStatusCode("GET", path, nil, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Contains(t, result, "activities")
	})

	t.Run("GET /api/v1/agents/search - Search agents", func(t *testing.T) {
		path := "/api/v1/agents/search?query=Test"
		respBody := tc.AssertStatusCode("GET", path, nil, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Contains(t, result, "agents")
	})

	t.Run("DELETE /api/v1/agents/:id - Delete agent", func(t *testing.T) {
		path := fmt.Sprintf("/api/v1/agents/%s", createdAgentID)
		tc.AssertStatusCode("DELETE", path, nil, userToken, 200)

		// Verify agent is deleted
		tc.AssertStatusCode("GET", path, nil, userToken, 404)
	})

	t.Run("POST /api/v1/agents - Require authentication", func(t *testing.T) {
		body := map[string]interface{}{
			"name": "Unauthenticated Agent",
			"type": "ai_agent",
		}

		tc.AssertStatusCode("POST", "/api/v1/agents", body, "", 401)
	})
}
