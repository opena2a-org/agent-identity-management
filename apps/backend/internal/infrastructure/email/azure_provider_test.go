package email

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================
// AzureEmailProvider Tests
// ===========================

func TestNewAzureEmailProvider_ValidConfig(t *testing.T) {
	config := EmailConfig{
		AzureConnectionString: "endpoint=https://test.communication.azure.com;accesskey=abc123==",
		FromAddress:           "noreply@example.com",
		FromName:              "Example",
	}

	provider, err := NewAzureEmailProvider(config)

	require.NoError(t, err)
	assert.NotNil(t, provider)
	assert.Equal(t, "endpoint=https://test.communication.azure.com;accesskey=abc123==", provider.connectionString)
}

func TestNewAzureEmailProvider_MissingConnectionString(t *testing.T) {
	config := EmailConfig{
		FromAddress: "noreply@example.com",
		FromName:    "Example",
	}

	provider, err := NewAzureEmailProvider(config)

	assert.Error(t, err)
	assert.Nil(t, provider)
	assert.Contains(t, err.Error(), "Azure connection string is required")
}

func TestNewAzureEmailProvider_FromFormat(t *testing.T) {
	config := EmailConfig{
		AzureConnectionString: "endpoint=https://test.communication.azure.com;accesskey=abc123==",
		FromAddress:           "noreply@example.com",
		FromName:              "Example",
	}

	provider, err := NewAzureEmailProvider(config)

	require.NoError(t, err)
	assert.Contains(t, provider.from, "noreply@example.com")
	assert.Contains(t, provider.from, "Example")
}

func TestAzureEmailProvider_GetProviderName(t *testing.T) {
	config := EmailConfig{
		AzureConnectionString: "endpoint=https://test.communication.azure.com;accesskey=abc123==",
		FromAddress:           "noreply@example.com",
		FromName:              "Example",
	}

	provider, err := NewAzureEmailProvider(config)
	require.NoError(t, err)

	assert.Equal(t, "Azure Communication Services", provider.GetProviderName())
}

func TestAzureEmailProvider_ValidateConfig_Valid(t *testing.T) {
	provider := &AzureEmailProvider{
		connectionString: "endpoint=https://test.communication.azure.com;accesskey=abc123==",
		from:             "Example <noreply@example.com>",
	}

	err := provider.ValidateConfig()

	assert.NoError(t, err)
}

func TestAzureEmailProvider_ValidateConfig_MissingConnectionString(t *testing.T) {
	provider := &AzureEmailProvider{
		from: "Example <noreply@example.com>",
	}

	err := provider.ValidateConfig()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Azure connection string is required")
}

func TestAzureEmailProvider_ValidateConfig_MissingFrom(t *testing.T) {
	provider := &AzureEmailProvider{
		connectionString: "endpoint=https://test.communication.azure.com;accesskey=abc123==",
	}

	err := provider.ValidateConfig()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "From address is required")
}

func TestAzureEmailProvider_SendEmail_NotImplemented(t *testing.T) {
	config := EmailConfig{
		AzureConnectionString: "endpoint=https://test.communication.azure.com;accesskey=abc123==",
		FromAddress:           "noreply@example.com",
		FromName:              "Example",
	}

	provider, err := NewAzureEmailProvider(config)
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
