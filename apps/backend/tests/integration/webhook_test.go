package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCreateWebhookUnauthorized tests that creating webhook requires authentication
func TestCreateWebhookUnauthorized(t *testing.T) {
	baseURL := getBaseURL()

	payload := map[string]interface{}{
		"url":    "https://example.com/webhook",
		"events": []string{"agent.created", "agent.verified"},
	}
	jsonData, err := json.Marshal(payload)
	require.NoError(t, err)

	resp, err := http.Post(baseURL+"/api/v1/webhooks", "application/json", bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestListWebhooksUnauthorized tests that listing webhooks requires authentication
func TestListWebhooksUnauthorized(t *testing.T) {
	baseURL := getBaseURL()

	resp, err := http.Get(baseURL + "/api/v1/webhooks")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestGetWebhookUnauthorized tests that getting webhook requires authentication
func TestGetWebhookUnauthorized(t *testing.T) {
	baseURL := getBaseURL()
	webhookID := "123e4567-e89b-12d3-a456-426614174000"

	resp, err := http.Get(baseURL + "/api/v1/webhooks/" + webhookID)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestDeleteWebhookUnauthorized tests that deleting webhook requires authentication
func TestDeleteWebhookUnauthorized(t *testing.T) {
	baseURL := getBaseURL()
	webhookID := "123e4567-e89b-12d3-a456-426614174000"

	req, err := http.NewRequest("DELETE", baseURL+"/api/v1/webhooks/"+webhookID, nil)
	require.NoError(t, err)

	client := &http.Client{}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestTestWebhookUnauthorized tests that testing webhook requires authentication
func TestTestWebhookUnauthorized(t *testing.T) {
	baseURL := getBaseURL()
	webhookID := "123e4567-e89b-12d3-a456-426614174000"

	resp, err := http.Post(baseURL+"/api/v1/webhooks/"+webhookID+"/test", "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestCreateWebhookWithInvalidData tests creating webhook with invalid data
func TestCreateWebhookWithInvalidData(t *testing.T) {
	baseURL := getBaseURL()

	payload := map[string]interface{}{
		"url": "invalid-url",
	}
	jsonData, err := json.Marshal(payload)
	require.NoError(t, err)

	resp, err := http.Post(baseURL+"/api/v1/webhooks", "application/json", bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestCreateWebhookWithEmptyEvents tests creating webhook with empty events array
func TestCreateWebhookWithEmptyEvents(t *testing.T) {
	baseURL := getBaseURL()

	payload := map[string]interface{}{
		"url":    "https://example.com/webhook",
		"events": []string{},
	}
	jsonData, err := json.Marshal(payload)
	require.NoError(t, err)

	resp, err := http.Post(baseURL+"/api/v1/webhooks", "application/json", bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

// TestTestWebhookWithPayload tests webhook test with custom payload
func TestTestWebhookWithPayload(t *testing.T) {
	baseURL := getBaseURL()
	webhookID := "123e4567-e89b-12d3-a456-426614174000"

	payload := map[string]interface{}{
		"event": "agent.created",
		"data": map[string]interface{}{
			"agent_id": "test-123",
		},
	}
	jsonData, err := json.Marshal(payload)
	require.NoError(t, err)

	resp, err := http.Post(baseURL+"/api/v1/webhooks/"+webhookID+"/test", "application/json", bytes.NewBuffer(jsonData))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "Should return 401 without auth token")
}

