package integration

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDataFlowEndpointsUnauthorized tests that data flow endpoints require authentication
func TestDataFlowEndpointsUnauthorized(t *testing.T) {
	ensureAIMBackendRunning(t)
	baseURL := getBaseURL()

	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/api/v1/data-flows"},
		{"POST", "/api/v1/data-flows"},
		{"GET", "/api/v1/data-flows/statistics"},
		{"GET", "/api/v1/data-sources"},
		{"POST", "/api/v1/data-sources"},
		{"GET", "/api/v1/data-flow-policies"},
		{"POST", "/api/v1/data-flow-policies"},
		{"GET", "/api/v1/data-flow-violations"},
		{"GET", "/api/v1/classifications"},
	}

	for _, ep := range endpoints {
		t.Run(fmt.Sprintf("%s %s requires auth", ep.method, ep.path), func(t *testing.T) {
			tc := NewTestContext(t)
			tc.AssertStatusCode(ep.method, ep.path, nil, "", 401)
		})
	}
}

// TestDataFlowCRUD tests the complete data flow lifecycle
func TestDataFlowCRUD(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)
	require.NoError(t, tc.WaitForBackend())

	// Create test user and get token
	email := fmt.Sprintf("dataflow-test-%d@example.com", time.Now().Unix())
	userToken, err := tc.CreateTestUser(email, "TestPass123!")
	if err != nil {
		// Fall back to admin login if user creation fails
		require.NoError(t, tc.LoginAsAdmin())
		userToken = tc.AdminToken
	}

	var createdAgentID string
	var createdFlowID string

	// First, create a test agent
	t.Run("Setup: Create test agent", func(t *testing.T) {
		body := map[string]interface{}{
			"name":        fmt.Sprintf("DataFlow Test Agent %d", time.Now().Unix()),
			"type":        "ai_agent",
			"description": "Agent for data flow integration tests",
			"version":     "1.0.0",
		}

		respBody := tc.AssertStatusCode("POST", "/api/v1/agents", body, userToken, 201)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)
		createdAgentID = result["id"].(string)
	})

	t.Run("POST /api/v1/data-flows - Record data flow", func(t *testing.T) {
		body := map[string]interface{}{
			"agentId":     createdAgentID,
			"sourceType":  "database",
			"sourceName":  "users.email",
			"destination": "openai-api",
			"dataTypes":   []string{"pii.email"},
			"metadata": map[string]interface{}{
				"action":   "llm_query",
				"purpose":  "customer_support",
				"recordId": "usr-12345",
			},
		}

		respBody := tc.AssertStatusCode("POST", "/api/v1/data-flows", body, userToken, 201)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Contains(t, result, "id")
		assert.Equal(t, "database", result["sourceType"])
		assert.Equal(t, "users.email", result["sourceName"])
		assert.Equal(t, "openai-api", result["destination"])

		createdFlowID = result["id"].(string)
	})

	t.Run("GET /api/v1/data-flows - List data flows", func(t *testing.T) {
		respBody := tc.AssertStatusCode("GET", "/api/v1/data-flows", nil, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Contains(t, result, "flows")
		flows := result["flows"].([]interface{})
		assert.GreaterOrEqual(t, len(flows), 1, "Should have at least one flow")
	})

	t.Run("GET /api/v1/data-flows/:id - Get specific data flow", func(t *testing.T) {
		path := fmt.Sprintf("/api/v1/data-flows/%s", createdFlowID)
		respBody := tc.AssertStatusCode("GET", path, nil, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Equal(t, createdFlowID, result["id"])
		assert.Equal(t, "database", result["sourceType"])
	})

	t.Run("GET /api/v1/data-flows?agentId=X - Filter by agent", func(t *testing.T) {
		path := fmt.Sprintf("/api/v1/data-flows?agentId=%s", createdAgentID)
		respBody := tc.AssertStatusCode("GET", path, nil, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		flows := result["flows"].([]interface{})
		for _, f := range flows {
			flow := f.(map[string]interface{})
			assert.Equal(t, createdAgentID, flow["agentId"], "All flows should belong to specified agent")
		}
	})

	// Cleanup
	t.Run("Cleanup: Delete test agent", func(t *testing.T) {
		path := fmt.Sprintf("/api/v1/agents/%s", createdAgentID)
		tc.AssertStatusCode("DELETE", path, nil, userToken, 200)
	})
}

// TestDataSourceRegistration tests data source registration and classification
func TestDataSourceRegistration(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)
	require.NoError(t, tc.WaitForBackend())

	// Get user token
	email := fmt.Sprintf("datasource-test-%d@example.com", time.Now().Unix())
	userToken, err := tc.CreateTestUser(email, "TestPass123!")
	if err != nil {
		require.NoError(t, tc.LoginAsAdmin())
		userToken = tc.AdminToken
	}

	var createdSourceID string

	t.Run("POST /api/v1/data-sources - Register explicit source (Tier 1)", func(t *testing.T) {
		body := map[string]interface{}{
			"name":               "customer_ssn",
			"sourceType":         "database_column",
			"connectionString":   "postgres://localhost/customers",
			"classificationType": "pii.ssn",
			"classificationTier": 1, // Explicit
			"metadata": map[string]interface{}{
				"table":  "customers",
				"column": "ssn",
			},
		}

		respBody := tc.AssertStatusCode("POST", "/api/v1/data-sources", body, userToken, 201)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Contains(t, result, "id")
		assert.Equal(t, float64(1), result["classificationTier"], "Explicit classification should be tier 1")
		assert.Equal(t, float64(1.0), result["classificationConfidence"], "Explicit should have 100% confidence")

		createdSourceID = result["id"].(string)
	})

	t.Run("POST /api/v1/data-sources - Auto-classify from column name (Tier 2)", func(t *testing.T) {
		body := map[string]interface{}{
			"name":             "email_column",
			"sourceType":       "database_column",
			"connectionString": "postgres://localhost/users",
			"metadata": map[string]interface{}{
				"table":  "users",
				"column": "email_address",
			},
		}

		respBody := tc.AssertStatusCode("POST", "/api/v1/data-sources", body, userToken, 201)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		// Should auto-classify as PII.email based on column name
		assert.Equal(t, float64(2), result["classificationTier"], "Schema inference should be tier 2")
		confidence := result["classificationConfidence"].(float64)
		assert.GreaterOrEqual(t, confidence, 0.9, "Schema-based should have high confidence")
	})

	t.Run("GET /api/v1/data-sources - List data sources", func(t *testing.T) {
		respBody := tc.AssertStatusCode("GET", "/api/v1/data-sources", nil, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Contains(t, result, "sources")
		sources := result["sources"].([]interface{})
		assert.GreaterOrEqual(t, len(sources), 1)
	})

	t.Run("GET /api/v1/data-sources/:id - Get specific source", func(t *testing.T) {
		path := fmt.Sprintf("/api/v1/data-sources/%s", createdSourceID)
		respBody := tc.AssertStatusCode("GET", path, nil, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Equal(t, createdSourceID, result["id"])
	})

	t.Run("PUT /api/v1/data-sources/:id - Update source classification", func(t *testing.T) {
		path := fmt.Sprintf("/api/v1/data-sources/%s", createdSourceID)
		body := map[string]interface{}{
			"classificationType": "pii.government_id",
			"classificationTier": 1,
		}

		respBody := tc.AssertStatusCode("PUT", path, body, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Equal(t, "pii.government_id", result["classificationType"])
	})

	t.Run("DELETE /api/v1/data-sources/:id - Delete source", func(t *testing.T) {
		path := fmt.Sprintf("/api/v1/data-sources/%s", createdSourceID)
		tc.AssertStatusCode("DELETE", path, nil, userToken, 200)

		// Verify deletion
		tc.AssertStatusCode("GET", path, nil, userToken, 404)
	})
}

// TestDataFlowPolicies tests policy creation and enforcement
func TestDataFlowPolicies(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)
	require.NoError(t, tc.WaitForBackend())

	email := fmt.Sprintf("policy-test-%d@example.com", time.Now().Unix())
	userToken, err := tc.CreateTestUser(email, "TestPass123!")
	if err != nil {
		require.NoError(t, tc.LoginAsAdmin())
		userToken = tc.AdminToken
	}

	var createdPolicyID string

	t.Run("POST /api/v1/data-flow-policies - Create block policy", func(t *testing.T) {
		body := map[string]interface{}{
			"name":                "Block SSN to External APIs",
			"description":         "Prevent SSN from flowing to any external API",
			"classificationType":  "pii.ssn",
			"destinationPattern":  "*-api",
			"action":              "block",
			"enabled":             true,
		}

		respBody := tc.AssertStatusCode("POST", "/api/v1/data-flow-policies", body, userToken, 201)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Contains(t, result, "id")
		assert.Equal(t, "block", result["action"])
		assert.Equal(t, true, result["enabled"])

		createdPolicyID = result["id"].(string)
	})

	t.Run("POST /api/v1/data-flow-policies - Create alert policy", func(t *testing.T) {
		body := map[string]interface{}{
			"name":               "Alert on PHI to LLMs",
			"description":        "Alert when PHI is sent to LLM providers",
			"classificationCategory": "phi",
			"destinationPattern": "openai*|anthropic*|google*",
			"action":             "alert",
			"enabled":            true,
		}

		respBody := tc.AssertStatusCode("POST", "/api/v1/data-flow-policies", body, userToken, 201)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Equal(t, "alert", result["action"])
	})

	t.Run("GET /api/v1/data-flow-policies - List policies", func(t *testing.T) {
		respBody := tc.AssertStatusCode("GET", "/api/v1/data-flow-policies", nil, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Contains(t, result, "policies")
		policies := result["policies"].([]interface{})
		assert.GreaterOrEqual(t, len(policies), 2, "Should have at least 2 policies")
	})

	t.Run("PUT /api/v1/data-flow-policies/:id - Disable policy", func(t *testing.T) {
		path := fmt.Sprintf("/api/v1/data-flow-policies/%s", createdPolicyID)
		body := map[string]interface{}{
			"enabled": false,
		}

		respBody := tc.AssertStatusCode("PUT", path, body, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Equal(t, false, result["enabled"])
	})

	t.Run("DELETE /api/v1/data-flow-policies/:id - Delete policy", func(t *testing.T) {
		path := fmt.Sprintf("/api/v1/data-flow-policies/%s", createdPolicyID)
		tc.AssertStatusCode("DELETE", path, nil, userToken, 200)
	})
}

// TestDataFlowViolations tests violation detection and recording
func TestDataFlowViolations(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)
	require.NoError(t, tc.WaitForBackend())

	email := fmt.Sprintf("violation-test-%d@example.com", time.Now().Unix())
	userToken, err := tc.CreateTestUser(email, "TestPass123!")
	if err != nil {
		require.NoError(t, tc.LoginAsAdmin())
		userToken = tc.AdminToken
	}

	var createdAgentID string
	var createdPolicyID string

	// Setup: Create agent and policy
	t.Run("Setup: Create test agent and blocking policy", func(t *testing.T) {
		// Create agent
		agentBody := map[string]interface{}{
			"name":        fmt.Sprintf("Violation Test Agent %d", time.Now().Unix()),
			"type":        "ai_agent",
			"description": "Agent for violation testing",
		}
		respBody := tc.AssertStatusCode("POST", "/api/v1/agents", agentBody, userToken, 201)
		var agentResult map[string]interface{}
		json.Unmarshal(respBody, &agentResult)
		createdAgentID = agentResult["id"].(string)

		// Create blocking policy for SSN
		policyBody := map[string]interface{}{
			"name":               "Block SSN Externally",
			"classificationType": "pii.ssn",
			"destinationPattern": "*",
			"action":             "block",
			"enabled":            true,
		}
		respBody = tc.AssertStatusCode("POST", "/api/v1/data-flow-policies", policyBody, userToken, 201)
		var policyResult map[string]interface{}
		json.Unmarshal(respBody, &policyResult)
		createdPolicyID = policyResult["id"].(string)
	})

	t.Run("POST /api/v1/data-flows - Trigger policy violation", func(t *testing.T) {
		body := map[string]interface{}{
			"agentId":     createdAgentID,
			"sourceType":  "database",
			"sourceName":  "customers.ssn",
			"destination": "external-api",
			"dataTypes":   []string{"pii.ssn"},
		}

		// This should still succeed (flow is recorded) but trigger a violation
		respBody := tc.AssertStatusCode("POST", "/api/v1/data-flows", body, userToken, 201)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		// Check if violation was triggered
		if violations, ok := result["violations"].([]interface{}); ok {
			assert.GreaterOrEqual(t, len(violations), 1, "Should have triggered at least one violation")
		}
	})

	t.Run("GET /api/v1/data-flow-violations - List violations", func(t *testing.T) {
		respBody := tc.AssertStatusCode("GET", "/api/v1/data-flow-violations", nil, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Contains(t, result, "violations")
	})

	t.Run("GET /api/v1/data-flow-violations?agentId=X - Filter by agent", func(t *testing.T) {
		path := fmt.Sprintf("/api/v1/data-flow-violations?agentId=%s", createdAgentID)
		respBody := tc.AssertStatusCode("GET", path, nil, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		violations := result["violations"].([]interface{})
		for _, v := range violations {
			violation := v.(map[string]interface{})
			assert.Equal(t, createdAgentID, violation["agentId"])
		}
	})

	t.Run("GET /api/v1/data-flow-violations?severity=critical - Filter by severity", func(t *testing.T) {
		path := "/api/v1/data-flow-violations?severity=critical"
		respBody := tc.AssertStatusCode("GET", path, nil, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		violations := result["violations"].([]interface{})
		for _, v := range violations {
			violation := v.(map[string]interface{})
			assert.Equal(t, "critical", violation["severity"])
		}
	})

	// Cleanup
	t.Run("Cleanup", func(t *testing.T) {
		tc.AssertStatusCode("DELETE", fmt.Sprintf("/api/v1/agents/%s", createdAgentID), nil, userToken, 200)
		tc.AssertStatusCode("DELETE", fmt.Sprintf("/api/v1/data-flow-policies/%s", createdPolicyID), nil, userToken, 200)
	})
}

// TestDataFlowStatistics tests statistics and aggregation
func TestDataFlowStatistics(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)
	require.NoError(t, tc.WaitForBackend())

	email := fmt.Sprintf("stats-test-%d@example.com", time.Now().Unix())
	userToken, err := tc.CreateTestUser(email, "TestPass123!")
	if err != nil {
		require.NoError(t, tc.LoginAsAdmin())
		userToken = tc.AdminToken
	}

	t.Run("GET /api/v1/data-flows/statistics - Get overall statistics", func(t *testing.T) {
		respBody := tc.AssertStatusCode("GET", "/api/v1/data-flows/statistics", nil, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		// Should contain summary statistics
		assert.Contains(t, result, "totalFlows")
		assert.Contains(t, result, "totalViolations")
		assert.Contains(t, result, "flowsByCategory")
	})

	t.Run("GET /api/v1/data-flows/statistics?period=24h - Filter by period", func(t *testing.T) {
		path := "/api/v1/data-flows/statistics?period=24h"
		respBody := tc.AssertStatusCode("GET", path, nil, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Contains(t, result, "period")
		assert.Equal(t, "24h", result["period"])
	})

	t.Run("GET /api/v1/data-flows/statistics?groupBy=agent - Group by agent", func(t *testing.T) {
		path := "/api/v1/data-flows/statistics?groupBy=agent"
		respBody := tc.AssertStatusCode("GET", path, nil, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Contains(t, result, "byAgent")
	})

	t.Run("GET /api/v1/data-flows/statistics?groupBy=destination - Group by destination", func(t *testing.T) {
		path := "/api/v1/data-flows/statistics?groupBy=destination"
		respBody := tc.AssertStatusCode("GET", path, nil, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Contains(t, result, "byDestination")
	})
}

// TestClassificationTypes tests classification type listing
func TestClassificationTypes(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)
	require.NoError(t, tc.WaitForBackend())

	email := fmt.Sprintf("class-test-%d@example.com", time.Now().Unix())
	userToken, err := tc.CreateTestUser(email, "TestPass123!")
	if err != nil {
		require.NoError(t, tc.LoginAsAdmin())
		userToken = tc.AdminToken
	}

	t.Run("GET /api/v1/classifications - List all classification types", func(t *testing.T) {
		respBody := tc.AssertStatusCode("GET", "/api/v1/classifications", nil, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Contains(t, result, "categories")
		categories := result["categories"].([]interface{})

		// Should have PII, PHI, PCI categories
		categoryNames := make([]string, 0)
		for _, c := range categories {
			cat := c.(map[string]interface{})
			categoryNames = append(categoryNames, cat["name"].(string))
		}

		assert.Contains(t, categoryNames, "pii", "Should have PII category")
		assert.Contains(t, categoryNames, "phi", "Should have PHI category")
		assert.Contains(t, categoryNames, "pci", "Should have PCI category")
	})

	t.Run("GET /api/v1/classifications?category=pii - Filter by category", func(t *testing.T) {
		respBody := tc.AssertStatusCode("GET", "/api/v1/classifications?category=pii", nil, userToken, 200)

		var result map[string]interface{}
		err := json.Unmarshal(respBody, &result)
		require.NoError(t, err)

		assert.Contains(t, result, "types")
		types := result["types"].([]interface{})

		// PII should include email, ssn, phone, address, etc.
		typeNames := make([]string, 0)
		for _, t := range types {
			typ := t.(map[string]interface{})
			typeNames = append(typeNames, typ["name"].(string))
		}

		assert.Contains(t, typeNames, "email", "PII should include email")
		assert.Contains(t, typeNames, "ssn", "PII should include SSN")
	})
}

// TestDataFlowWithClassificationTiers tests classification tier hierarchy
func TestDataFlowWithClassificationTiers(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)
	require.NoError(t, tc.WaitForBackend())

	email := fmt.Sprintf("tier-test-%d@example.com", time.Now().Unix())
	userToken, err := tc.CreateTestUser(email, "TestPass123!")
	if err != nil {
		require.NoError(t, tc.LoginAsAdmin())
		userToken = tc.AdminToken
	}

	testCases := []struct {
		name               string
		sourceName         string
		expectedTier       float64
		expectedConfidence float64
		explicitType       string
	}{
		{
			name:               "Tier 1 - Explicit classification",
			sourceName:         "explicit_ssn_field",
			expectedTier:       1,
			expectedConfidence: 1.0,
			explicitType:       "pii.ssn",
		},
		{
			name:               "Tier 2 - Schema inference from column name",
			sourceName:         "customer_email_address",
			expectedTier:       2,
			expectedConfidence: 0.95,
			explicitType:       "", // Let system infer
		},
		{
			name:               "Tier 2 - Schema inference from table.column",
			sourceName:         "patients.medical_record_number",
			expectedTier:       2,
			expectedConfidence: 0.95,
			explicitType:       "",
		},
	}

	for _, tc2 := range testCases {
		t.Run(tc2.name, func(t *testing.T) {
			body := map[string]interface{}{
				"name":             tc2.sourceName,
				"sourceType":       "database_column",
				"connectionString": "postgres://localhost/test",
			}

			if tc2.explicitType != "" {
				body["classificationType"] = tc2.explicitType
				body["classificationTier"] = tc2.expectedTier
			}

			respBody := tc.AssertStatusCode("POST", "/api/v1/data-sources", body, userToken, 201)

			var result map[string]interface{}
			err := json.Unmarshal(respBody, &result)
			require.NoError(t, err)

			assert.Equal(t, tc2.expectedTier, result["classificationTier"],
				"Classification tier should match expected")

			confidence := result["classificationConfidence"].(float64)
			assert.GreaterOrEqual(t, confidence, tc2.expectedConfidence-0.1,
				"Classification confidence should be >= expected")
		})
	}
}

// TestDataFlowRateLimiting tests that the endpoint handles high volume
func TestDataFlowRateLimiting(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)
	require.NoError(t, tc.WaitForBackend())

	email := fmt.Sprintf("ratelimit-test-%d@example.com", time.Now().Unix())
	userToken, err := tc.CreateTestUser(email, "TestPass123!")
	if err != nil {
		require.NoError(t, tc.LoginAsAdmin())
		userToken = tc.AdminToken
	}

	// Create test agent
	agentBody := map[string]interface{}{
		"name": fmt.Sprintf("Rate Limit Test Agent %d", time.Now().Unix()),
		"type": "ai_agent",
	}
	respBody := tc.AssertStatusCode("POST", "/api/v1/agents", agentBody, userToken, 201)
	var agentResult map[string]interface{}
	json.Unmarshal(respBody, &agentResult)
	agentID := agentResult["id"].(string)

	t.Run("Handle burst of data flow events", func(t *testing.T) {
		// Send 50 flow events rapidly
		successCount := 0
		for i := 0; i < 50; i++ {
			body := map[string]interface{}{
				"agentId":     agentID,
				"sourceType":  "database",
				"sourceName":  fmt.Sprintf("field_%d", i),
				"destination": "llm-api",
				"dataTypes":   []string{"pii.email"},
			}

			respBody := tc.AssertStatusCode("POST", "/api/v1/data-flows", body, userToken, 201)
			if len(respBody) > 0 {
				successCount++
			}
		}

		assert.Equal(t, 50, successCount, "All 50 events should be recorded")
	})

	// Cleanup
	tc.AssertStatusCode("DELETE", fmt.Sprintf("/api/v1/agents/%s", agentID), nil, userToken, 200)
}

// TestDataFlowInvalidInputs tests validation of inputs
func TestDataFlowInvalidInputs(t *testing.T) {
	ensureAIMBackendRunning(t)
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	tc := NewTestContext(t)
	require.NoError(t, tc.WaitForBackend())

	email := fmt.Sprintf("invalid-test-%d@example.com", time.Now().Unix())
	userToken, err := tc.CreateTestUser(email, "TestPass123!")
	if err != nil {
		require.NoError(t, tc.LoginAsAdmin())
		userToken = tc.AdminToken
	}

	t.Run("POST /api/v1/data-flows - Missing required fields", func(t *testing.T) {
		body := map[string]interface{}{
			// Missing agentId, sourceType, destination
			"sourceName": "some_field",
		}

		tc.AssertStatusCode("POST", "/api/v1/data-flows", body, userToken, 400)
	})

	t.Run("POST /api/v1/data-flows - Invalid agent ID", func(t *testing.T) {
		body := map[string]interface{}{
			"agentId":     uuid.New().String(), // Non-existent agent
			"sourceType":  "database",
			"sourceName":  "test_field",
			"destination": "api",
		}

		tc.AssertStatusCode("POST", "/api/v1/data-flows", body, userToken, 404)
	})

	t.Run("POST /api/v1/data-sources - Invalid classification tier", func(t *testing.T) {
		body := map[string]interface{}{
			"name":               "test_source",
			"sourceType":         "database_column",
			"classificationTier": 99, // Invalid tier
		}

		tc.AssertStatusCode("POST", "/api/v1/data-sources", body, userToken, 400)
	})

	t.Run("POST /api/v1/data-flow-policies - Invalid action", func(t *testing.T) {
		body := map[string]interface{}{
			"name":               "Invalid Policy",
			"classificationType": "pii.email",
			"action":             "invalid_action",
		}

		tc.AssertStatusCode("POST", "/api/v1/data-flow-policies", body, userToken, 400)
	})

	t.Run("GET /api/v1/data-flows/:id - Invalid UUID format", func(t *testing.T) {
		tc.AssertStatusCode("GET", "/api/v1/data-flows/not-a-uuid", nil, userToken, 400)
	})
}
