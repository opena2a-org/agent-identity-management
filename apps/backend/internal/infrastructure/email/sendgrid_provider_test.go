package email

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================
// SendGridProvider Tests
// ===========================

func TestNewSendGridProvider_ValidConfig(t *testing.T) {
	config := EmailConfig{
		SendGridAPIKey: "SG.valid-api-key",
		FromAddress:    "noreply@example.com",
		FromName:       "Example",
	}

	provider, err := NewSendGridProvider(config)

	require.NoError(t, err)
	assert.NotNil(t, provider)
	assert.Equal(t, "SG.valid-api-key", provider.apiKey)
}

func TestNewSendGridProvider_MissingAPIKey(t *testing.T) {
	config := EmailConfig{
		FromAddress: "noreply@example.com",
		FromName:    "Example",
	}

	provider, err := NewSendGridProvider(config)

	assert.Error(t, err)
	assert.Nil(t, provider)
	assert.Contains(t, err.Error(), "SendGrid API key is required")
}

func TestNewSendGridProvider_FromFormat(t *testing.T) {
	// When FromAddress and FromName are provided, from field is set
	config := EmailConfig{
		SendGridAPIKey: "SG.valid-api-key",
		FromAddress:    "noreply@example.com",
		FromName:       "Example",
	}

	provider, err := NewSendGridProvider(config)

	require.NoError(t, err)
	assert.Contains(t, provider.from, "noreply@example.com")
	assert.Contains(t, provider.from, "Example")
}

func TestSendGridProvider_GetProviderName(t *testing.T) {
	config := EmailConfig{
		SendGridAPIKey: "SG.valid-api-key",
		FromAddress:    "noreply@example.com",
		FromName:       "Example",
	}

	provider, err := NewSendGridProvider(config)
	require.NoError(t, err)

	assert.Equal(t, "SendGrid", provider.GetProviderName())
}

func TestSendGridProvider_ValidateConfig_Valid(t *testing.T) {
	provider := &SendGridProvider{
		apiKey: "SG.valid-api-key",
		from:   "Example <noreply@example.com>",
	}

	err := provider.ValidateConfig()

	assert.NoError(t, err)
}

func TestSendGridProvider_ValidateConfig_MissingAPIKey(t *testing.T) {
	provider := &SendGridProvider{
		from: "Example <noreply@example.com>",
	}

	err := provider.ValidateConfig()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SendGrid API key is required")
}

func TestSendGridProvider_ValidateConfig_MissingFrom(t *testing.T) {
	provider := &SendGridProvider{
		apiKey: "SG.valid-api-key",
	}

	err := provider.ValidateConfig()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "From address is required")
}

func TestSendGridProvider_SendEmail_NotImplemented(t *testing.T) {
	config := EmailConfig{
		SendGridAPIKey: "SG.valid-api-key",
		FromAddress:    "noreply@example.com",
		FromName:       "Example",
	}

	provider, err := NewSendGridProvider(config)
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
