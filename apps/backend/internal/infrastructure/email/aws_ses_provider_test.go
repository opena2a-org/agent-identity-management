package email

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================
// AWSSESProvider Tests
// ===========================

func TestNewAWSSESProvider_ValidConfig(t *testing.T) {
	config := EmailConfig{
		AWSRegion:          "us-east-1",
		AWSAccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
		AWSSecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		FromAddress:        "noreply@example.com",
		FromName:           "Example",
	}

	provider, err := NewAWSSESProvider(config)

	require.NoError(t, err)
	assert.NotNil(t, provider)
	assert.Equal(t, "us-east-1", provider.region)
	assert.Equal(t, "AKIAIOSFODNN7EXAMPLE", provider.accessKeyID)
}

func TestNewAWSSESProvider_MissingRegion(t *testing.T) {
	config := EmailConfig{
		AWSAccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
		AWSSecretAccessKey: "secret",
		FromAddress:        "noreply@example.com",
		FromName:           "Example",
	}

	provider, err := NewAWSSESProvider(config)

	assert.Error(t, err)
	assert.Nil(t, provider)
	assert.Contains(t, err.Error(), "AWS region is required")
}

func TestNewAWSSESProvider_MissingAccessKeyID(t *testing.T) {
	config := EmailConfig{
		AWSRegion:          "us-east-1",
		AWSSecretAccessKey: "secret",
		FromAddress:        "noreply@example.com",
		FromName:           "Example",
	}

	provider, err := NewAWSSESProvider(config)

	assert.Error(t, err)
	assert.Nil(t, provider)
	assert.Contains(t, err.Error(), "AWS access key ID is required")
}

func TestNewAWSSESProvider_MissingSecretAccessKey(t *testing.T) {
	config := EmailConfig{
		AWSRegion:      "us-east-1",
		AWSAccessKeyID: "AKIAIOSFODNN7EXAMPLE",
		FromAddress:    "noreply@example.com",
		FromName:       "Example",
	}

	provider, err := NewAWSSESProvider(config)

	assert.Error(t, err)
	assert.Nil(t, provider)
	assert.Contains(t, err.Error(), "AWS secret access key is required")
}

func TestNewAWSSESProvider_FromFormat(t *testing.T) {
	// When FromAddress and FromName are provided, from field is set
	config := EmailConfig{
		AWSRegion:          "us-east-1",
		AWSAccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
		AWSSecretAccessKey: "secret",
		FromAddress:        "noreply@example.com",
		FromName:           "Example",
	}

	provider, err := NewAWSSESProvider(config)

	require.NoError(t, err)
	assert.Contains(t, provider.from, "noreply@example.com")
	assert.Contains(t, provider.from, "Example")
}

func TestAWSSESProvider_GetProviderName(t *testing.T) {
	config := EmailConfig{
		AWSRegion:          "us-east-1",
		AWSAccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
		AWSSecretAccessKey: "secret",
		FromAddress:        "noreply@example.com",
		FromName:           "Example",
	}

	provider, err := NewAWSSESProvider(config)
	require.NoError(t, err)

	assert.Equal(t, "AWS SES", provider.GetProviderName())
}

func TestAWSSESProvider_ValidateConfig_Valid(t *testing.T) {
	provider := &AWSSESProvider{
		region:          "us-east-1",
		accessKeyID:     "AKIAIOSFODNN7EXAMPLE",
		secretAccessKey: "secret",
		from:            "Example <noreply@example.com>",
	}

	err := provider.ValidateConfig()

	assert.NoError(t, err)
}

func TestAWSSESProvider_ValidateConfig_MissingRegion(t *testing.T) {
	provider := &AWSSESProvider{
		accessKeyID:     "AKIAIOSFODNN7EXAMPLE",
		secretAccessKey: "secret",
		from:            "Example <noreply@example.com>",
	}

	err := provider.ValidateConfig()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "AWS region is required")
}

func TestAWSSESProvider_ValidateConfig_MissingAccessKeyID(t *testing.T) {
	provider := &AWSSESProvider{
		region:          "us-east-1",
		secretAccessKey: "secret",
		from:            "Example <noreply@example.com>",
	}

	err := provider.ValidateConfig()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "AWS access key ID is required")
}

func TestAWSSESProvider_ValidateConfig_MissingSecretAccessKey(t *testing.T) {
	provider := &AWSSESProvider{
		region:      "us-east-1",
		accessKeyID: "AKIAIOSFODNN7EXAMPLE",
		from:        "Example <noreply@example.com>",
	}

	err := provider.ValidateConfig()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "AWS secret access key is required")
}

func TestAWSSESProvider_ValidateConfig_MissingFrom(t *testing.T) {
	provider := &AWSSESProvider{
		region:          "us-east-1",
		accessKeyID:     "AKIAIOSFODNN7EXAMPLE",
		secretAccessKey: "secret",
	}

	err := provider.ValidateConfig()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "From address is required")
}

func TestAWSSESProvider_SendEmail_NotImplemented(t *testing.T) {
	config := EmailConfig{
		AWSRegion:          "us-east-1",
		AWSAccessKeyID:     "AKIAIOSFODNN7EXAMPLE",
		AWSSecretAccessKey: "secret",
		FromAddress:        "noreply@example.com",
		FromName:           "Example",
	}

	provider, err := NewAWSSESProvider(config)
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
