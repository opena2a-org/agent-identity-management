package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestWebhookEventConstants(t *testing.T) {
	// Alert events
	assert.Equal(t, WebhookEvent("alert.created"), WebhookEventAlertCreated)
	assert.Equal(t, WebhookEvent("alert.acknowledged"), WebhookEventAlertAcknowledged)

	// Agent events
	assert.Equal(t, WebhookEvent("agent.created"), WebhookEventAgentCreated)
	assert.Equal(t, WebhookEvent("agent.verified"), WebhookEventAgentVerified)
	assert.Equal(t, WebhookEvent("agent.updated"), WebhookEventAgentUpdated)
	assert.Equal(t, WebhookEvent("agent.suspended"), WebhookEventAgentSuspended)
	assert.Equal(t, WebhookEvent("agent.deleted"), WebhookEventAgentDeleted)

	// Trust score events
	assert.Equal(t, WebhookEvent("trust_score.changed"), WebhookEventTrustScoreChanged)
	assert.Equal(t, WebhookEvent("trust_score.low"), WebhookEventTrustScoreLow)

	// Security events
	assert.Equal(t, WebhookEvent("security.violation"), WebhookEventSecurityViolation)
	assert.Equal(t, WebhookEvent("compliance.violation"), WebhookEventComplianceViolation)
	assert.Equal(t, WebhookEvent("security.capability_drift"), WebhookEventCapabilityDrift)
	assert.Equal(t, WebhookEvent("security.data_exfiltration"), WebhookEventDataExfiltration)

	// API key events
	assert.Equal(t, WebhookEvent("api_key.created"), WebhookEventAPIKeyCreated)
	assert.Equal(t, WebhookEvent("api_key.revoked"), WebhookEventAPIKeyRevoked)
	assert.Equal(t, WebhookEvent("api_key.expired"), WebhookEventAPIKeyExpired)
}

func TestAllWebhookEvents(t *testing.T) {
	events := AllWebhookEvents()

	// Should return all 16 event types
	assert.Len(t, events, 16)

	// Verify all event types are included
	expectedEvents := []WebhookEvent{
		WebhookEventAlertCreated,
		WebhookEventAlertAcknowledged,
		WebhookEventAgentCreated,
		WebhookEventAgentVerified,
		WebhookEventAgentUpdated,
		WebhookEventAgentSuspended,
		WebhookEventAgentDeleted,
		WebhookEventTrustScoreChanged,
		WebhookEventTrustScoreLow,
		WebhookEventSecurityViolation,
		WebhookEventComplianceViolation,
		WebhookEventCapabilityDrift,
		WebhookEventDataExfiltration,
		WebhookEventAPIKeyCreated,
		WebhookEventAPIKeyRevoked,
		WebhookEventAPIKeyExpired,
	}

	for _, expected := range expectedEvents {
		assert.Contains(t, events, expected, "AllWebhookEvents should contain %s", expected)
	}
}

func TestWebhookStatusConstants(t *testing.T) {
	assert.Equal(t, WebhookStatus("active"), WebhookStatusActive)
	assert.Equal(t, WebhookStatus("paused"), WebhookStatusPaused)
	assert.Equal(t, WebhookStatus("disabled"), WebhookStatusDisabled)
}

func TestWebhookDeliveryStatusConstants(t *testing.T) {
	assert.Equal(t, WebhookDeliveryStatus("pending"), DeliveryStatusPending)
	assert.Equal(t, WebhookDeliveryStatus("success"), DeliveryStatusSuccess)
	assert.Equal(t, WebhookDeliveryStatus("failed"), DeliveryStatusFailed)
	assert.Equal(t, WebhookDeliveryStatus("retrying"), DeliveryStatusRetrying)
	assert.Equal(t, WebhookDeliveryStatus("abandoned"), DeliveryStatusAbandoned)
}

func TestWebhookStruct(t *testing.T) {
	now := time.Now()
	webhookID := uuid.New()
	orgID := uuid.New()
	createdBy := uuid.New()

	webhook := Webhook{
		ID:                webhookID,
		OrganizationID:    orgID,
		Name:              "test-webhook",
		URL:               "https://example.com/webhook",
		Events:            []WebhookEvent{WebhookEventAgentCreated, WebhookEventAgentVerified},
		Secret:            "webhook-secret",
		Status:            WebhookStatusActive,
		Description:       "Test webhook for notifications",
		TimeoutSeconds:    30,
		MaxRetries:        3,
		RetryDelaySeconds: 60,
		SuccessCount:      100,
		FailureCount:      5,
		CreatedAt:         now,
		UpdatedAt:         now,
		CreatedBy:         createdBy,
	}

	assert.Equal(t, webhookID, webhook.ID)
	assert.Equal(t, orgID, webhook.OrganizationID)
	assert.Equal(t, "test-webhook", webhook.Name)
	assert.Equal(t, "https://example.com/webhook", webhook.URL)
	assert.Len(t, webhook.Events, 2)
	assert.Equal(t, "webhook-secret", webhook.Secret)
	assert.Equal(t, WebhookStatusActive, webhook.Status)
	assert.Equal(t, 30, webhook.TimeoutSeconds)
	assert.Equal(t, 3, webhook.MaxRetries)
	assert.Equal(t, int64(100), webhook.SuccessCount)
	assert.Equal(t, int64(5), webhook.FailureCount)
}

func TestWebhookDeliveryStruct(t *testing.T) {
	now := time.Now()
	deliveryID := uuid.New()
	webhookID := uuid.New()
	orgID := uuid.New()

	delivery := WebhookDelivery{
		ID:             deliveryID,
		WebhookID:      webhookID,
		OrganizationID: orgID,
		Event:          WebhookEventAgentCreated,
		Payload:        `{"agent_id": "123"}`,
		Status:         DeliveryStatusSuccess,
		AttemptCount:   1,
		LastAttemptAt:  &now,
		StatusCode:     200,
		ResponseBody:   "OK",
		DurationMs:     150,
		Success:        true,
		CreatedAt:      now,
		CompletedAt:    &now,
	}

	assert.Equal(t, deliveryID, delivery.ID)
	assert.Equal(t, webhookID, delivery.WebhookID)
	assert.Equal(t, WebhookEventAgentCreated, delivery.Event)
	assert.Equal(t, DeliveryStatusSuccess, delivery.Status)
	assert.Equal(t, 1, delivery.AttemptCount)
	assert.Equal(t, 200, delivery.StatusCode)
	assert.Equal(t, int64(150), delivery.DurationMs)
	assert.True(t, delivery.Success)
}

func TestWebhookDeliveryRetryFields(t *testing.T) {
	now := time.Now()
	nextRetry := now.Add(60 * time.Second)

	delivery := WebhookDelivery{
		ID:           uuid.New(),
		Status:       DeliveryStatusRetrying,
		AttemptCount: 2,
		NextRetryAt:  &nextRetry,
		ErrorMessage: "Connection timeout",
	}

	assert.Equal(t, DeliveryStatusRetrying, delivery.Status)
	assert.Equal(t, 2, delivery.AttemptCount)
	assert.NotNil(t, delivery.NextRetryAt)
	assert.Equal(t, "Connection timeout", delivery.ErrorMessage)
}

func TestWebhookPayloadStruct(t *testing.T) {
	now := time.Now()
	deliveryID := uuid.New().String()
	orgID := uuid.New().String()

	payload := WebhookPayload{
		ID:             deliveryID,
		Event:          WebhookEventAgentCreated,
		Timestamp:      now,
		OrganizationID: orgID,
		Data: map[string]interface{}{
			"agentId":   "agent-123",
			"agentName": "test-agent",
			"status":    "verified",
		},
	}

	assert.Equal(t, deliveryID, payload.ID)
	assert.Equal(t, WebhookEventAgentCreated, payload.Event)
	assert.Equal(t, orgID, payload.OrganizationID)
	assert.NotNil(t, payload.Data)
	assert.Equal(t, "agent-123", payload.Data["agentId"])
}

func TestWebhookEventStringConversion(t *testing.T) {
	event := WebhookEventAgentCreated
	str := string(event)
	assert.Equal(t, "agent.created", str)

	// Convert back
	converted := WebhookEvent(str)
	assert.Equal(t, WebhookEventAgentCreated, converted)
}

func TestWebhookWithSoftDelete(t *testing.T) {
	now := time.Now()
	webhook := Webhook{
		ID:        uuid.New(),
		Name:      "deleted-webhook",
		DeletedAt: &now,
	}

	assert.NotNil(t, webhook.DeletedAt)
}

func TestWebhookWithLastTriggered(t *testing.T) {
	now := time.Now()
	webhook := Webhook{
		ID:                 uuid.New(),
		Name:               "active-webhook",
		LastTriggered:      &now,
		LastDeliveryStatus: "success",
	}

	assert.NotNil(t, webhook.LastTriggered)
	assert.Equal(t, "success", webhook.LastDeliveryStatus)
}
