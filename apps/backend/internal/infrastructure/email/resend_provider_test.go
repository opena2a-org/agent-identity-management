package email

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================
// ResendProvider Tests
// ===========================

func TestNewResendProvider_ValidConfig(t *testing.T) {
	config := EmailConfig{
		ResendAPIKey: "re_valid-api-key",
		FromAddress:  "noreply@example.com",
		FromName:     "Example",
	}

	provider, err := NewResendProvider(config)

	require.NoError(t, err)
	assert.NotNil(t, provider)
	assert.Equal(t, "re_valid-api-key", provider.apiKey)
}

func TestNewResendProvider_MissingAPIKey(t *testing.T) {
	config := EmailConfig{
		FromAddress: "noreply@example.com",
		FromName:    "Example",
	}

	provider, err := NewResendProvider(config)

	assert.Error(t, err)
	assert.Nil(t, provider)
	assert.Contains(t, err.Error(), "Resend API key is required")
}

func TestNewResendProvider_FromFormat(t *testing.T) {
	// When FromAddress and FromName are provided, from field is set
	config := EmailConfig{
		ResendAPIKey: "re_valid-api-key",
		FromAddress:  "noreply@example.com",
		FromName:     "Example",
	}

	provider, err := NewResendProvider(config)

	require.NoError(t, err)
	assert.Contains(t, provider.from, "noreply@example.com")
	assert.Contains(t, provider.from, "Example")
}

func TestResendProvider_GetProviderName(t *testing.T) {
	config := EmailConfig{
		ResendAPIKey: "re_valid-api-key",
		FromAddress:  "noreply@example.com",
		FromName:     "Example",
	}

	provider, err := NewResendProvider(config)
	require.NoError(t, err)

	assert.Equal(t, "Resend", provider.GetProviderName())
}

func TestResendProvider_ValidateConfig_Valid(t *testing.T) {
	provider := &ResendProvider{
		apiKey: "re_valid-api-key",
		from:   "Example <noreply@example.com>",
	}

	err := provider.ValidateConfig()

	assert.NoError(t, err)
}

func TestResendProvider_ValidateConfig_MissingAPIKey(t *testing.T) {
	provider := &ResendProvider{
		from: "Example <noreply@example.com>",
	}

	err := provider.ValidateConfig()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Resend API key is required")
}

func TestResendProvider_ValidateConfig_MissingFrom(t *testing.T) {
	provider := &ResendProvider{
		apiKey: "re_valid-api-key",
	}

	err := provider.ValidateConfig()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "From address is required")
}

func TestResendProvider_SendEmail_NotImplemented(t *testing.T) {
	config := EmailConfig{
		ResendAPIKey: "re_valid-api-key",
		FromAddress:  "noreply@example.com",
		FromName:     "Example",
	}

	provider, err := NewResendProvider(config)
	require.NoError(t, err)

	params := EmailParams{
		From:     "sender@example.com",
		To:       []string{"recipient@example.com"},
		Subject:  "Test",
		TextBody: "Test message",
	}

	err = provider.SendEmail(context.Background(), params)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not yet implemented")
}
