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
