package application

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/opena2a/identity/backend/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestDefaultWebhookConfig(t *testing.T) {
	config := DefaultWebhookConfig()

	assert.NotNil(t, config)
	assert.Equal(t, 30*time.Second, config.DefaultTimeout)
	assert.Equal(t, 3, config.DefaultMaxRetries)
	assert.Equal(t, 60*time.Second, config.DefaultRetryDelay)
	assert.Equal(t, 30*time.Minute, config.MaxRetryDelay)
	assert.Equal(t, 2.0, config.RetryBackoffFactor)
	assert.Equal(t, 5, config.DeliveryWorkerCount)
	assert.Equal(t, 30*time.Second, config.RetryWorkerInterval)
}

func TestWebhookConfig_CustomValues(t *testing.T) {
	config := &WebhookConfig{
		DefaultTimeout:       60 * time.Second,
		DefaultMaxRetries:    5,
		DefaultRetryDelay:    120 * time.Second,
		MaxRetryDelay:        60 * time.Minute,
		RetryBackoffFactor:   3.0,
		DeliveryWorkerCount:  10,
		RetryWorkerInterval:  60 * time.Second,
	}

	assert.Equal(t, 60*time.Second, config.DefaultTimeout)
	assert.Equal(t, 5, config.DefaultMaxRetries)
	assert.Equal(t, 120*time.Second, config.DefaultRetryDelay)
	assert.Equal(t, 60*time.Minute, config.MaxRetryDelay)
	assert.Equal(t, 3.0, config.RetryBackoffFactor)
	assert.Equal(t, 10, config.DeliveryWorkerCount)
	assert.Equal(t, 60*time.Second, config.RetryWorkerInterval)
}

func TestGenerateSecret(t *testing.T) {
	secret1, err := generateSecret()
	assert.NoError(t, err)
	assert.NotEmpty(t, secret1)
	assert.Len(t, secret1, 64) // 32 bytes = 64 hex chars

	// Verify it's valid hex
	_, err = hex.DecodeString(secret1)
	assert.NoError(t, err)

	// Generate another and ensure they're different
	secret2, err := generateSecret()
	assert.NoError(t, err)
	assert.NotEqual(t, secret1, secret2)
}

func TestCreateSignature(t *testing.T) {
	payload := []byte(`{"event":"test","data":"hello"}`)
	secret := "mysecretkey123"

	signature := createSignature(payload, secret)

	// Verify signature is valid hex
	assert.NotEmpty(t, signature)
	_, err := hex.DecodeString(signature)
	assert.NoError(t, err)

	// Verify HMAC-SHA256 manually
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))
	assert.Equal(t, expected, signature)
}

func TestCreateSignature_DifferentPayloads(t *testing.T) {
	secret := "testsecret"

	sig1 := createSignature([]byte("payload1"), secret)
	sig2 := createSignature([]byte("payload2"), secret)

	assert.NotEqual(t, sig1, sig2)
}

func TestCreateSignature_DifferentSecrets(t *testing.T) {
	payload := []byte("same payload")

	sig1 := createSignature(payload, "secret1")
	sig2 := createSignature(payload, "secret2")

	assert.NotEqual(t, sig1, sig2)
}

func TestCreateSignature_EmptyPayload(t *testing.T) {
	secret := "mysecret"
	signature := createSignature([]byte{}, secret)

	assert.NotEmpty(t, signature)
	_, err := hex.DecodeString(signature)
	assert.NoError(t, err)
}

func TestCreateSignature_EmptySecret(t *testing.T) {
	payload := []byte("test payload")
	signature := createSignature(payload, "")

	assert.NotEmpty(t, signature)
	_, err := hex.DecodeString(signature)
	assert.NoError(t, err)
}

func TestCreateWebhookRequest_Structure(t *testing.T) {
	isActive := true
	req := &CreateWebhookRequest{
		Name:     "Test Webhook",
		URL:      "https://example.com/webhook",
		Events:   []domain.WebhookEvent{domain.WebhookEventAgentCreated},
		IsActive: &isActive,
	}

	assert.Equal(t, "Test Webhook", req.Name)
	assert.Equal(t, "https://example.com/webhook", req.URL)
	assert.Len(t, req.Events, 1)
	assert.True(t, *req.IsActive)
}

func TestCreateWebhookRequest_NilIsActive(t *testing.T) {
	req := &CreateWebhookRequest{
		Name:   "Test Webhook",
		URL:    "https://example.com/webhook",
		Events: []domain.WebhookEvent{domain.WebhookEventAgentCreated},
		// IsActive not set
	}

	assert.Nil(t, req.IsActive)
}

func TestWebhookTestResult_Success(t *testing.T) {
	result := &WebhookTestResult{
		Success:    true,
		StatusCode: 200,
	}

	assert.True(t, result.Success)
	assert.Equal(t, 200, result.StatusCode)
	assert.Empty(t, result.ErrorMessage)
}

func TestWebhookTestResult_Failure(t *testing.T) {
	result := &WebhookTestResult{
		Success:      false,
		StatusCode:   500,
		ErrorMessage: "Internal Server Error",
	}

	assert.False(t, result.Success)
	assert.Equal(t, 500, result.StatusCode)
	assert.Equal(t, "Internal Server Error", result.ErrorMessage)
}

func TestWebhookTestResult_ConnectionError(t *testing.T) {
	result := &WebhookTestResult{
		Success:      false,
		StatusCode:   0, // No response received
		ErrorMessage: "connection refused",
	}

	assert.False(t, result.Success)
	assert.Equal(t, 0, result.StatusCode)
	assert.Contains(t, result.ErrorMessage, "connection")
}

func TestSignatureVerification(t *testing.T) {
	// Simulate what a webhook receiver would do
	payload := []byte(`{"event":"agent.created","data":{"id":"123"}}`)
	secret := "webhook_secret_key"

	// Create signature (what the service does)
	signature := createSignature(payload, secret)

	// Receiver verification (what the client does)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	assert.Equal(t, expectedSig, signature)

	// Verify using constant-time comparison
	sigBytes, _ := hex.DecodeString(signature)
	expectedBytes, _ := hex.DecodeString(expectedSig)
	assert.True(t, hmac.Equal(sigBytes, expectedBytes))
}

func TestSignatureVerification_InvalidSecret(t *testing.T) {
	payload := []byte(`{"event":"agent.created"}`)
	correctSecret := "correct_secret"
	wrongSecret := "wrong_secret"

	// Create signature with correct secret
	signature := createSignature(payload, correctSecret)

	// Try to verify with wrong secret
	mac := hmac.New(sha256.New, []byte(wrongSecret))
	mac.Write(payload)
	wrongExpected := hex.EncodeToString(mac.Sum(nil))

	assert.NotEqual(t, wrongExpected, signature)
}

func TestSignatureVerification_TamperedPayload(t *testing.T) {
	originalPayload := []byte(`{"amount":100}`)
	tamperedPayload := []byte(`{"amount":1000}`)
	secret := "secret"

	// Create signature with original payload
	signature := createSignature(originalPayload, secret)

	// Try to verify with tampered payload
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(tamperedPayload)
	tamperedSig := hex.EncodeToString(mac.Sum(nil))

	assert.NotEqual(t, tamperedSig, signature)
}

// ========================================
// NewWebhookService Tests
// ========================================

func TestNewWebhookService(t *testing.T) {
	service := NewWebhookService(nil)

	assert.NotNil(t, service)
	assert.Nil(t, service.webhookRepo)
	assert.NotNil(t, service.config)
	assert.NotNil(t, service.stopChan)

	// Verify default config was applied
	assert.Equal(t, 30*time.Second, service.config.DefaultTimeout)
	assert.Equal(t, 3, service.config.DefaultMaxRetries)
}

func TestWebhookService_SetConfig(t *testing.T) {
	service := NewWebhookService(nil)

	customConfig := &WebhookConfig{
		DefaultTimeout:      60 * time.Second,
		DefaultMaxRetries:   5,
		DefaultRetryDelay:   120 * time.Second,
		MaxRetryDelay:       60 * time.Minute,
		RetryBackoffFactor:  3.0,
		DeliveryWorkerCount: 10,
	}

	service.SetConfig(customConfig)

	assert.Equal(t, customConfig, service.config)
	assert.Equal(t, 60*time.Second, service.config.DefaultTimeout)
	assert.Equal(t, 5, service.config.DefaultMaxRetries)
}

// ========================================
// WebhookConfig Edge Cases
// ========================================

func TestWebhookConfig_ZeroValues(t *testing.T) {
	config := &WebhookConfig{}

	// Zero values should be distinguishable
	assert.Equal(t, time.Duration(0), config.DefaultTimeout)
	assert.Equal(t, 0, config.DefaultMaxRetries)
	assert.Equal(t, time.Duration(0), config.DefaultRetryDelay)
	assert.Equal(t, 0.0, config.RetryBackoffFactor)
}

func TestWebhookConfig_BackoffCalculation(t *testing.T) {
	config := DefaultWebhookConfig()

	// Simulate backoff calculation
	delay := config.DefaultRetryDelay
	for i := 0; i < 5; i++ {
		delay = time.Duration(float64(delay) * config.RetryBackoffFactor)
		if delay > config.MaxRetryDelay {
			delay = config.MaxRetryDelay
		}
	}

	// After 5 retries with 2x backoff, should be capped at MaxRetryDelay
	assert.Equal(t, config.MaxRetryDelay, delay)
}

// ========================================
// CreateWebhookRequest Tests
// ========================================

func TestCreateWebhookRequest_MultipleEvents(t *testing.T) {
	isActive := true
	req := &CreateWebhookRequest{
		Name: "Multi-event Webhook",
		URL:  "https://example.com/webhook",
		Events: []domain.WebhookEvent{
			domain.WebhookEventAgentCreated,
			domain.WebhookEventAgentUpdated,
			domain.WebhookEventAgentDeleted,
		},
		IsActive: &isActive,
	}

	assert.Len(t, req.Events, 3)
	assert.Contains(t, req.Events, domain.WebhookEventAgentCreated)
	assert.Contains(t, req.Events, domain.WebhookEventAgentUpdated)
	assert.Contains(t, req.Events, domain.WebhookEventAgentDeleted)
}

func TestCreateWebhookRequest_IsActiveFalse(t *testing.T) {
	isActive := false
	req := &CreateWebhookRequest{
		Name:     "Inactive Webhook",
		URL:      "https://example.com/webhook",
		Events:   []domain.WebhookEvent{domain.WebhookEventAgentCreated},
		IsActive: &isActive,
	}

	assert.NotNil(t, req.IsActive)
	assert.False(t, *req.IsActive)
}

func TestCreateWebhookRequest_EmptyEvents(t *testing.T) {
	req := &CreateWebhookRequest{
		Name:   "Empty Events Webhook",
		URL:    "https://example.com/webhook",
		Events: []domain.WebhookEvent{},
	}

	assert.Empty(t, req.Events)
}

// ========================================
// WebhookTestResult Tests
// ========================================

func TestWebhookTestResult_VariousStatusCodes(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		isSuccess  bool
	}{
		{"200 OK", 200, true},
		{"201 Created", 201, true},
		{"204 No Content", 204, true},
		{"299 Edge success", 299, true},
		{"300 Redirect", 300, false},
		{"400 Bad Request", 400, false},
		{"401 Unauthorized", 401, false},
		{"404 Not Found", 404, false},
		{"500 Internal Error", 500, false},
		{"502 Bad Gateway", 502, false},
		{"503 Service Unavailable", 503, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Determine success based on status code (2xx = success)
			isSuccess := tt.statusCode >= 200 && tt.statusCode < 300
			assert.Equal(t, tt.isSuccess, isSuccess)
		})
	}
}

// ========================================
// Signature Length and Format Tests
// ========================================

func TestSignature_Length(t *testing.T) {
	payload := []byte(`{"test":"data"}`)
	secret := "secret"

	signature := createSignature(payload, secret)

	// SHA256 produces 32 bytes = 64 hex characters
	assert.Len(t, signature, 64)
}

func TestSignature_Consistency(t *testing.T) {
	payload := []byte(`{"consistent":"payload"}`)
	secret := "consistent-secret"

	// Same input should always produce same output
	sig1 := createSignature(payload, secret)
	sig2 := createSignature(payload, secret)
	sig3 := createSignature(payload, secret)

	assert.Equal(t, sig1, sig2)
	assert.Equal(t, sig2, sig3)
}

func TestSignature_LargePayload(t *testing.T) {
	// Test with a large payload
	largePayload := make([]byte, 10000)
	for i := range largePayload {
		largePayload[i] = byte('a' + (i % 26))
	}
	secret := "secret"

	signature := createSignature(largePayload, secret)

	// Should still produce valid 64-char hex
	assert.Len(t, signature, 64)
	_, err := hex.DecodeString(signature)
	assert.NoError(t, err)
}

// ========================================
// generateSecret Tests
// ========================================

func TestGenerateSecret_Uniqueness(t *testing.T) {
	secrets := make(map[string]bool)

	// Generate 100 secrets and ensure they're all unique
	for i := 0; i < 100; i++ {
		secret, err := generateSecret()
		assert.NoError(t, err)
		assert.False(t, secrets[secret], "Duplicate secret generated")
		secrets[secret] = true
	}
}

func TestGenerateSecret_Format(t *testing.T) {
	secret, err := generateSecret()
	assert.NoError(t, err)

	// Should be valid hex
	decoded, err := hex.DecodeString(secret)
	assert.NoError(t, err)

	// Should be 32 bytes when decoded
	assert.Len(t, decoded, 32)
}

// ========================================
// Webhook Event Types Tests
// ========================================

func TestWebhookEvents_AllEventTypes(t *testing.T) {
	events := []domain.WebhookEvent{
		domain.WebhookEventAgentCreated,
		domain.WebhookEventAgentUpdated,
		domain.WebhookEventAgentDeleted,
		domain.WebhookEventAgentVerified,
		domain.WebhookEventAgentSuspended,
		domain.WebhookEventTrustScoreChanged,
		domain.WebhookEventAlertCreated,
		domain.WebhookEventCapabilityDrift,
	}

	for _, event := range events {
		assert.NotEmpty(t, string(event))
	}
}

func TestWebhookEvent_Matching(t *testing.T) {
	// Test that we can match events correctly
	subscribedEvents := []domain.WebhookEvent{
		domain.WebhookEventAgentCreated,
		domain.WebhookEventAlertCreated,
	}

	incomingEvent := domain.WebhookEventAgentCreated

	// Check if event is subscribed
	found := false
	for _, e := range subscribedEvents {
		if e == incomingEvent {
			found = true
			break
		}
	}

	assert.True(t, found)

	// Check for an event not subscribed
	incomingEvent = domain.WebhookEventAgentDeleted
	found = false
	for _, e := range subscribedEvents {
		if e == incomingEvent {
			found = true
			break
		}
	}

	assert.False(t, found)
}
