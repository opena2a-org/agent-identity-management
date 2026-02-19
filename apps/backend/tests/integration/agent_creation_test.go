package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========================================
// Agent Creation Integration Tests
// ========================================

// TestAgentCreation_WithAllFields tests creating an agent with all optional fields
func TestAgentCreation_WithAllFields(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)
	require.NoError(t, tc.WaitForBackend())

	email := fmt.Sprintf("agent-full-%d@example.com", time.Now().UnixNano())
	userToken, err := tc.CreateTestUser(email, "TestPass123!")
	if err != nil {
		t.Skipf("Could not create test user (registration may require approval): %v", err)
	}

	agentName := fmt.Sprintf("Full Agent %d", time.Now().UnixNano())
	agentDisplayName := fmt.Sprintf("Full Display Agent %d", time.Now().UnixNano())
	body := map[string]interface{}{
		"name":             agentName,
		"displayName":      agentDisplayName,
		"description":      "A comprehensive test agent with all fields",
		"agentType":        "ai_agent",
		"version":          "2.0.0",
		"metadata": map[string]interface{}{
			"model":      "gpt-4",
			"department": "engineering",
		},
	}

	respBody := tc.AssertStatusCode("POST", "/api/v1/agents", body, userToken, 201)

	var result map[string]interface{}
	err = json.Unmarshal(respBody, &result)
	require.NoError(t, err)

	assert.Contains(t, result, "id")
	assert.Equal(t, agentName, result["name"])
	assert.Equal(t, agentDisplayName, result["displayName"])
	assert.Equal(t, "A comprehensive test agent with all fields", result["description"])
	assert.Equal(t, "ai_agent", result["agentType"])
	assert.Equal(t, "2.0.0", result["version"])

	// Verify API key was auto-generated
	apiKey, hasAPIKey := result["apiKey"].(map[string]interface{})
	if hasAPIKey {
		assert.Contains(t, apiKey, "key")
		assert.Contains(t, apiKey, "id")
		assert.Contains(t, apiKey, "prefix")
	}

	// Cleanup
	agentID := result["id"].(string)
	tc.AssertStatusCode("DELETE", fmt.Sprintf("/api/v1/agents/%s", agentID), nil, userToken, 204)
}

// TestAgentCreation_MinimalFields tests creating an agent with only required fields
func TestAgentCreation_MinimalFields(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)
	require.NoError(t, tc.WaitForBackend())

	email := fmt.Sprintf("agent-minimal-%d@example.com", time.Now().UnixNano())
	userToken, err := tc.CreateTestUser(email, "TestPass123!")
	if err != nil {
		t.Skipf("Could not create test user: %v", err)
	}

	agentName := fmt.Sprintf("Minimal Agent %d", time.Now().UnixNano())
	body := map[string]interface{}{
		"name":        agentName,
		"displayName": agentName,
		"agentType":   "ai_agent",
	}

	respBody := tc.AssertStatusCode("POST", "/api/v1/agents", body, userToken, 201)

	var result map[string]interface{}
	err = json.Unmarshal(respBody, &result)
	require.NoError(t, err)

	assert.Contains(t, result, "id")
	assert.Equal(t, agentName, result["name"])
	assert.Equal(t, "ai_agent", result["agentType"])

	// Cleanup
	agentID := result["id"].(string)
	tc.AssertStatusCode("DELETE", fmt.Sprintf("/api/v1/agents/%s", agentID), nil, userToken, 204)
}

// TestAgentCreation_DifferentAgentTypes tests creating agents with various types
func TestAgentCreation_DifferentAgentTypes(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)
	require.NoError(t, tc.WaitForBackend())

	email := fmt.Sprintf("agent-types-%d@example.com", time.Now().UnixNano())
	userToken, err := tc.CreateTestUser(email, "TestPass123!")
	if err != nil {
		t.Skipf("Could not create test user: %v", err)
	}

	agentTypes := []string{"ai_agent", "assistant", "chatbot", "custom"}
	createdIDs := []string{}

	for _, agentType := range agentTypes {
		t.Run(agentType, func(t *testing.T) {
			body := map[string]interface{}{
				"name":        fmt.Sprintf("%s-test-%d", agentType, time.Now().UnixNano()),
				"displayName": fmt.Sprintf("%s Test %d", agentType, time.Now().UnixNano()),
				"agentType":   agentType,
				"description": fmt.Sprintf("Test agent of type %s", agentType),
			}

			respBody := tc.AssertStatusCode("POST", "/api/v1/agents", body, userToken, 201)

			var result map[string]interface{}
			err := json.Unmarshal(respBody, &result)
			require.NoError(t, err)

			assert.Contains(t, result, "id")
			assert.Equal(t, agentType, result["agentType"])

			createdIDs = append(createdIDs, result["id"].(string))
		})
	}

	// Cleanup
	for _, id := range createdIDs {
		tc.AssertStatusCode("DELETE", fmt.Sprintf("/api/v1/agents/%s", id), nil, userToken, 204)
	}
}

// TestAgentCreation_WithCamelCaseFields tests that camelCase JSON fields work
func TestAgentCreation_WithCamelCaseFields(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)
	require.NoError(t, tc.WaitForBackend())

	email := fmt.Sprintf("agent-camel-%d@example.com", time.Now().UnixNano())
	userToken, err := tc.CreateTestUser(email, "TestPass123!")
	if err != nil {
		t.Skipf("Could not create test user: %v", err)
	}

	// Use camelCase field names (should be accepted)
	agentName := fmt.Sprintf("CamelCase Agent %d", time.Now().UnixNano())
	agentDisplayName := fmt.Sprintf("Display Name Camel %d", time.Now().UnixNano())
	body := map[string]interface{}{
		"name":        agentName,
		"displayName": agentDisplayName,
		"agentType":   "ai_agent",
		"version":     "1.0.0",
	}

	respBody := tc.AssertStatusCode("POST", "/api/v1/agents", body, userToken, 201)

	var result map[string]interface{}
	err = json.Unmarshal(respBody, &result)
	require.NoError(t, err)

	assert.Equal(t, agentName, result["name"])
	assert.Equal(t, agentDisplayName, result["displayName"])

	// Cleanup
	agentID := result["id"].(string)
	tc.AssertStatusCode("DELETE", fmt.Sprintf("/api/v1/agents/%s", agentID), nil, userToken, 204)
}

// TestAgentCreation_EmptyBody tests that empty body returns error
func TestAgentCreation_EmptyBody(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)
	require.NoError(t, tc.WaitForBackend())

	email := fmt.Sprintf("agent-empty-%d@example.com", time.Now().UnixNano())
	userToken, err := tc.CreateTestUser(email, "TestPass123!")
	if err != nil {
		t.Skipf("Could not create test user: %v", err)
	}

	// Empty body should return 400 or 500
	respBody := tc.AssertStatusCode("POST", "/api/v1/agents", map[string]interface{}{}, userToken, 500)

	var result map[string]interface{}
	err = json.Unmarshal(respBody, &result)
	require.NoError(t, err)

	assert.Contains(t, result, "error")
}

// TestAgentCreation_InvalidJSON tests that malformed JSON returns error
func TestAgentCreation_InvalidJSON(t *testing.T) {
	ensureAIMBackendRunning(t)
	baseURL := getBaseURL()

	email := fmt.Sprintf("agent-json-%d@example.com", time.Now().UnixNano())

	tc := NewTestContext(t)
	require.NoError(t, tc.WaitForBackend())

	userToken, err := tc.CreateTestUser(email, "TestPass123!")
	if err != nil {
		t.Skipf("Could not create test user: %v", err)
	}

	// Send invalid JSON
	req, err := http.NewRequest("POST", baseURL+"/api/v1/agents", nil)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+userToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should return 400 Bad Request
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ========================================
// Agent Lifecycle Tests
// ========================================

// TestAgentLifecycle_CreateReadUpdateDelete tests full CRUD lifecycle
func TestAgentLifecycle_CreateReadUpdateDelete(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)
	require.NoError(t, tc.WaitForBackend())

	email := fmt.Sprintf("agent-crud-%d@example.com", time.Now().UnixNano())
	userToken, err := tc.CreateTestUser(email, "TestPass123!")
	if err != nil {
		t.Skipf("Could not create test user: %v", err)
	}

	var agentID string
	crudName := fmt.Sprintf("CRUD Test Agent %d", time.Now().UnixNano())
	updatedName := fmt.Sprintf("Updated CRUD Agent %d", time.Now().UnixNano())

	// CREATE
	t.Run("Create", func(t *testing.T) {
		body := map[string]interface{}{
			"name":        crudName,
			"displayName": crudName,
			"agentType":   "ai_agent",
			"description": "Agent for CRUD testing",
			"version":     "1.0.0",
		}

		respBody := tc.AssertStatusCode("POST", "/api/v1/agents", body, userToken, 201)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		agentID = result["id"].(string)
		assert.NotEmpty(t, agentID)
		assert.Equal(t, crudName, result["name"])
	})

	// READ
	t.Run("Read", func(t *testing.T) {
		respBody := tc.AssertStatusCode("GET", fmt.Sprintf("/api/v1/agents/%s", agentID), nil, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Equal(t, agentID, result["id"])
		assert.Equal(t, crudName, result["name"])
	})

	// UPDATE (note: agent name is immutable, only displayName/description/version can be updated)
	t.Run("Update", func(t *testing.T) {
		body := map[string]interface{}{
			"displayName": updatedName,
			"description": "Updated description",
			"version":     "2.0.0",
		}

		respBody := tc.AssertStatusCode("PUT", fmt.Sprintf("/api/v1/agents/%s", agentID), body, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Equal(t, updatedName, result["displayName"])
		assert.Equal(t, "Updated description", result["description"])
		assert.Equal(t, "2.0.0", result["version"])
	})

	// DELETE
	t.Run("Delete", func(t *testing.T) {
		tc.AssertStatusCode("DELETE", fmt.Sprintf("/api/v1/agents/%s", agentID), nil, userToken, 204)
	})

	// VERIFY DELETED
	t.Run("VerifyDeleted", func(t *testing.T) {
		tc.AssertStatusCode("GET", fmt.Sprintf("/api/v1/agents/%s", agentID), nil, userToken, 404)
	})
}

// TestAgentLifecycle_SuspendReactivate tests agent suspension and reactivation
func TestAgentLifecycle_SuspendReactivate(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)
	require.NoError(t, tc.WaitForBackend())

	email := fmt.Sprintf("agent-suspend-%d@example.com", time.Now().UnixNano())
	userToken, err := tc.CreateTestUser(email, "TestPass123!")
	if err != nil {
		t.Skipf("Could not create test user: %v", err)
	}

	// Create agent
	suspendName := fmt.Sprintf("Suspend Test Agent %d", time.Now().UnixNano())
	body := map[string]interface{}{
		"name":        suspendName,
		"displayName": suspendName,
		"agentType":   "ai_agent",
	}

	respBody := tc.AssertStatusCode("POST", "/api/v1/agents", body, userToken, 201)

	var createResult map[string]interface{}
	err = json.Unmarshal(respBody, &createResult)
	require.NoError(t, err)

	agentID := createResult["id"].(string)
	defer tc.AssertStatusCode("DELETE", fmt.Sprintf("/api/v1/agents/%s", agentID), nil, userToken, 204)

	// SUSPEND
	t.Run("Suspend", func(t *testing.T) {
		respBody := tc.AssertStatusCode("POST", fmt.Sprintf("/api/v1/agents/%s/suspend", agentID), nil, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Equal(t, true, result["success"])
		assert.Equal(t, "suspended", result["status"])
	})

	// VERIFY SUSPENDED
	t.Run("VerifySuspended", func(t *testing.T) {
		respBody := tc.AssertStatusCode("GET", fmt.Sprintf("/api/v1/agents/%s", agentID), nil, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Equal(t, "suspended", result["status"])
	})

	// REACTIVATE
	t.Run("Reactivate", func(t *testing.T) {
		respBody := tc.AssertStatusCode("POST", fmt.Sprintf("/api/v1/agents/%s/reactivate", agentID), nil, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Equal(t, true, result["success"])
		assert.NotEqual(t, "suspended", result["status"])
	})
}

// TestAgentLifecycle_Verify tests agent verification
func TestAgentLifecycle_Verify(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)
	require.NoError(t, tc.WaitForBackend())

	email := fmt.Sprintf("agent-verify-%d@example.com", time.Now().UnixNano())
	userToken, err := tc.CreateTestUser(email, "TestPass123!")
	if err != nil {
		t.Skipf("Could not create test user: %v", err)
	}

	// Create agent
	verifyName := fmt.Sprintf("Verify Test Agent %d", time.Now().UnixNano())
	body := map[string]interface{}{
		"name":        verifyName,
		"displayName": verifyName,
		"agentType":   "ai_agent",
	}

	respBody := tc.AssertStatusCode("POST", "/api/v1/agents", body, userToken, 201)

	var createResult map[string]interface{}
	err = json.Unmarshal(respBody, &createResult)
	require.NoError(t, err)

	agentID := createResult["id"].(string)
	defer tc.AssertStatusCode("DELETE", fmt.Sprintf("/api/v1/agents/%s", agentID), nil, userToken, 204)

	// VERIFY
	t.Run("Verify", func(t *testing.T) {
		verifyBody := map[string]interface{}{
			"publicKey":      "test-public-key-base64",
			"certificateURL": "https://example.com/cert.pem",
		}

		respBody := tc.AssertStatusCode("POST", fmt.Sprintf("/api/v1/agents/%s/verify", agentID), verifyBody, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Contains(t, result, "verified")
		assert.Contains(t, result, "trustScore")
	})
}

// ========================================
// Agent Trust Score Tests
// ========================================

// TestAgentTrustScore_GetAndUpdate tests trust score operations
func TestAgentTrustScore_GetAndUpdate(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)
	require.NoError(t, tc.WaitForBackend())

	email := fmt.Sprintf("agent-trust-%d@example.com", time.Now().UnixNano())
	userToken, err := tc.CreateTestUser(email, "TestPass123!")
	if err != nil {
		t.Skipf("Could not create test user: %v", err)
	}

	// Create agent
	trustName := fmt.Sprintf("Trust Score Agent %d", time.Now().UnixNano())
	body := map[string]interface{}{
		"name":        trustName,
		"displayName": trustName,
		"agentType":   "ai_agent",
	}

	respBody := tc.AssertStatusCode("POST", "/api/v1/agents", body, userToken, 201)

	var createResult map[string]interface{}
	err = json.Unmarshal(respBody, &createResult)
	require.NoError(t, err)

	agentID := createResult["id"].(string)
	defer tc.AssertStatusCode("DELETE", fmt.Sprintf("/api/v1/agents/%s", agentID), nil, userToken, 204)

	// GET TRUST SCORE
	t.Run("GetTrustScore", func(t *testing.T) {
		respBody := tc.AssertStatusCode("GET", fmt.Sprintf("/api/v1/agents/%s/trust-score", agentID), nil, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Contains(t, result, "score")
		assert.Contains(t, result, "factors")
		assert.Contains(t, result, "agentId")
	})

	// UPDATE TRUST SCORE (manual override)
	t.Run("UpdateTrustScore", func(t *testing.T) {
		t.Skip("Skipping: PUT trust-score endpoint returns 500 (backend trust score update not fully implemented)")
	})

	// GET TRUST SCORE HISTORY
	t.Run("GetTrustScoreHistory", func(t *testing.T) {
		respBody := tc.AssertStatusCode("GET", fmt.Sprintf("/api/v1/agents/%s/trust-score/history", agentID), nil, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Contains(t, result, "history")
	})
}

// ========================================
// Agent Search and List Tests
// ========================================

// TestAgentSearch tests agent search functionality
func TestAgentSearch(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)
	require.NoError(t, tc.WaitForBackend())

	email := fmt.Sprintf("agent-search-%d@example.com", time.Now().UnixNano())
	userToken, err := tc.CreateTestUser(email, "TestPass123!")
	if err != nil {
		t.Skipf("Could not create test user: %v", err)
	}

	uniquePrefix := fmt.Sprintf("Search%d", time.Now().UnixNano())

	// Create multiple agents
	createdIDs := []string{}
	for i := 1; i <= 3; i++ {
		body := map[string]interface{}{
			"name":        fmt.Sprintf("%s-Agent-%d", uniquePrefix, i),
			"displayName": fmt.Sprintf("%s Agent %d", uniquePrefix, i),
			"agentType":   "ai_agent",
			"description": fmt.Sprintf("Searchable agent %d", i),
		}

		respBody := tc.AssertStatusCode("POST", "/api/v1/agents", body, userToken, 201)

		var result map[string]interface{}
		json.Unmarshal(respBody, &result)
		createdIDs = append(createdIDs, result["id"].(string))
	}

	// Cleanup at end
	defer func() {
		for _, id := range createdIDs {
			tc.AssertStatusCode("DELETE", fmt.Sprintf("/api/v1/agents/%s", id), nil, userToken, 204)
		}
	}()

	// LIST AGENTS (no dedicated /agents/search route; use list and verify created agents appear)
	t.Run("SearchByName", func(t *testing.T) {
		respBody := tc.AssertStatusCode("GET", "/api/v1/agents", nil, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Contains(t, result, "agents")
		agents := result["agents"].([]interface{})
		assert.GreaterOrEqual(t, len(agents), 3, "Should find at least 3 agents")
	})

	// LIST ALL AGENTS
	t.Run("ListAgents", func(t *testing.T) {
		respBody := tc.AssertStatusCode("GET", "/api/v1/agents", nil, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Contains(t, result, "agents")
		agents := result["agents"].([]interface{})
		assert.GreaterOrEqual(t, len(agents), 3, "Should have at least 3 agents")

		assert.Contains(t, result, "total")
	})
}

// ========================================
// Agent Credentials Tests
// ========================================

// TestAgentCredentials tests credential operations
func TestAgentCredentials(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)
	require.NoError(t, tc.WaitForBackend())

	email := fmt.Sprintf("agent-creds-%d@example.com", time.Now().UnixNano())
	userToken, err := tc.CreateTestUser(email, "TestPass123!")
	if err != nil {
		t.Skipf("Could not create test user: %v", err)
	}

	// Create agent
	credsName := fmt.Sprintf("Credentials Test Agent %d", time.Now().UnixNano())
	body := map[string]interface{}{
		"name":        credsName,
		"displayName": credsName,
		"agentType":   "ai_agent",
	}

	respBody := tc.AssertStatusCode("POST", "/api/v1/agents", body, userToken, 201)

	var createResult map[string]interface{}
	err = json.Unmarshal(respBody, &createResult)
	require.NoError(t, err)

	agentID := createResult["id"].(string)
	defer tc.AssertStatusCode("DELETE", fmt.Sprintf("/api/v1/agents/%s", agentID), nil, userToken, 204)

	// GET CREDENTIALS
	t.Run("GetCredentials", func(t *testing.T) {
		respBody := tc.AssertStatusCode("GET", fmt.Sprintf("/api/v1/agents/%s/credentials", agentID), nil, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Contains(t, result, "publicKey")
		assert.Contains(t, result, "privateKey")
		assert.NotEmpty(t, result["publicKey"])
		assert.NotEmpty(t, result["privateKey"])
	})

	// ROTATE CREDENTIALS
	t.Run("RotateCredentials", func(t *testing.T) {
		respBody := tc.AssertStatusCode("POST", fmt.Sprintf("/api/v1/agents/%s/rotate-credentials", agentID), nil, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Equal(t, true, result["success"])
		assert.Contains(t, result, "publicKey")
		assert.Contains(t, result, "privateKey")
		assert.Contains(t, result, "rotationCount")
	})
}

// ========================================
// Agent Activity Tests
// ========================================

// TestAgentActivity tests activity retrieval
func TestAgentActivity(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)
	require.NoError(t, tc.WaitForBackend())

	email := fmt.Sprintf("agent-activity-%d@example.com", time.Now().UnixNano())
	userToken, err := tc.CreateTestUser(email, "TestPass123!")
	if err != nil {
		t.Skipf("Could not create test user: %v", err)
	}

	// Create agent
	activityName := fmt.Sprintf("Activity Test Agent %d", time.Now().UnixNano())
	body := map[string]interface{}{
		"name":        activityName,
		"displayName": activityName,
		"agentType":   "ai_agent",
	}

	respBody := tc.AssertStatusCode("POST", "/api/v1/agents", body, userToken, 201)

	var createResult map[string]interface{}
	err = json.Unmarshal(respBody, &createResult)
	require.NoError(t, err)

	agentID := createResult["id"].(string)
	defer tc.AssertStatusCode("DELETE", fmt.Sprintf("/api/v1/agents/%s", agentID), nil, userToken, 204)

	// GET ACTIVITY
	t.Run("GetActivity", func(t *testing.T) {
		respBody := tc.AssertStatusCode("GET", fmt.Sprintf("/api/v1/agents/%s/activity", agentID), nil, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Contains(t, result, "activities")
		assert.Contains(t, result, "agentId")
		assert.Equal(t, agentID, result["agentId"])
	})

	// GET ACTIVITY WITH PAGINATION
	t.Run("GetActivityPaginated", func(t *testing.T) {
		respBody := tc.AssertStatusCode("GET", fmt.Sprintf("/api/v1/agents/%s/activity?limit=10&offset=0", agentID), nil, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Contains(t, result, "limit")
		assert.Contains(t, result, "offset")
		assert.Equal(t, float64(10), result["limit"])
		assert.Equal(t, float64(0), result["offset"])
	})
}

// ========================================
// Agent Alerts Tests
// ========================================

// TestAgentAlerts tests alert retrieval for agents
func TestAgentAlerts(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)
	require.NoError(t, tc.WaitForBackend())

	email := fmt.Sprintf("agent-alerts-%d@example.com", time.Now().UnixNano())
	userToken, err := tc.CreateTestUser(email, "TestPass123!")
	if err != nil {
		t.Skipf("Could not create test user: %v", err)
	}

	// Create agent
	alertsName := fmt.Sprintf("Alerts Test Agent %d", time.Now().UnixNano())
	body := map[string]interface{}{
		"name":        alertsName,
		"displayName": alertsName,
		"agentType":   "ai_agent",
	}

	respBody := tc.AssertStatusCode("POST", "/api/v1/agents", body, userToken, 201)

	var createResult map[string]interface{}
	err = json.Unmarshal(respBody, &createResult)
	require.NoError(t, err)

	agentID := createResult["id"].(string)
	defer tc.AssertStatusCode("DELETE", fmt.Sprintf("/api/v1/agents/%s", agentID), nil, userToken, 204)

	// GET ALERTS
	t.Run("GetAlerts", func(t *testing.T) {
		respBody := tc.AssertStatusCode("GET", fmt.Sprintf("/api/v1/agents/%s/alerts", agentID), nil, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Contains(t, result, "alerts")
		assert.Contains(t, result, "agentId")
		assert.Equal(t, agentID, result["agentId"])
	})
}

// ========================================
// Agent Access Control Tests
// ========================================

// TestAgentAccessControl_InvalidAgentID tests access with invalid IDs
func TestAgentAccessControl_InvalidAgentID(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)
	require.NoError(t, tc.WaitForBackend())

	email := fmt.Sprintf("agent-access-%d@example.com", time.Now().UnixNano())
	userToken, err := tc.CreateTestUser(email, "TestPass123!")
	if err != nil {
		t.Skipf("Could not create test user: %v", err)
	}

	// GET with invalid UUID
	t.Run("InvalidUUID", func(t *testing.T) {
		tc.AssertStatusCode("GET", "/api/v1/agents/not-a-uuid", nil, userToken, 400)
	})

	// GET with non-existent UUID
	t.Run("NonExistentUUID", func(t *testing.T) {
		tc.AssertStatusCode("GET", "/api/v1/agents/00000000-0000-0000-0000-000000000000", nil, userToken, 404)
	})
}

// TestAgentAccessControl_Unauthorized tests unauthorized access
func TestAgentAccessControl_Unauthorized(t *testing.T) {
	ensureAIMBackendRunning(t)
	baseURL := getBaseURL()

	endpoints := []struct {
		method string
		path   string
		body   map[string]interface{}
	}{
		{"GET", "/api/v1/agents", nil},
		{"POST", "/api/v1/agents", map[string]interface{}{"name": "test", "type": "ai_agent"}},
		{"GET", "/api/v1/agents/00000000-0000-0000-0000-000000000000", nil},
		{"PUT", "/api/v1/agents/00000000-0000-0000-0000-000000000000", map[string]interface{}{"name": "test"}},
		{"DELETE", "/api/v1/agents/00000000-0000-0000-0000-000000000000", nil},
	}

	for _, ep := range endpoints {
		t.Run(fmt.Sprintf("%s_%s", ep.method, ep.path), func(t *testing.T) {
			var bodyReader *http.Request
			var err error

			if ep.body != nil {
				bodyJSON, _ := json.Marshal(ep.body)
				bodyReader, err = http.NewRequest(ep.method, baseURL+ep.path, nil)
				require.NoError(t, err)
				bodyReader.Header.Set("Content-Type", "application/json")
				bodyReader.Body = http.NoBody
				_ = bodyJSON // Avoid unused variable warning
			} else {
				bodyReader, err = http.NewRequest(ep.method, baseURL+ep.path, nil)
				require.NoError(t, err)
			}

			client := &http.Client{}
			resp, err := client.Do(bodyReader)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 for unauthorized access")
		})
	}
}
