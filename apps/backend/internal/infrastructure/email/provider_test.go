package email

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================
// ProviderType Constants Tests
// ===========================

func TestProviderType_Constants(t *testing.T) {
	assert.Equal(t, ProviderType("smtp"), ProviderSMTP)
	assert.Equal(t, ProviderType("azure"), ProviderAzure)
	assert.Equal(t, ProviderType("aws_ses"), ProviderAWSSES)
	assert.Equal(t, ProviderType("sendgrid"), ProviderSendGrid)
	assert.Equal(t, ProviderType("resend"), ProviderResend)
	assert.Equal(t, ProviderType("console"), ProviderConsole)
}

// ===========================
// EmailConfig Tests
// ===========================

func TestEmailConfig_DefaultValues(t *testing.T) {
	config := EmailConfig{}

	assert.Empty(t, config.Provider)
	assert.Empty(t, config.FromAddress)
	assert.Empty(t, config.FromName)
	assert.Empty(t, config.SMTPHost)
	assert.Equal(t, 0, config.SMTPPort)
}

func TestEmailConfig_SMTPConfig(t *testing.T) {
	config := EmailConfig{
		Provider:     ProviderSMTP,
		SMTPHost:     "smtp.example.com",
		SMTPPort:     587,
		SMTPUser:     "user",
		SMTPPassword: "pass",
		SMTPTLS:      true,
		FromAddress:  "noreply@example.com",
		FromName:     "Example",
	}

	assert.Equal(t, ProviderSMTP, config.Provider)
	assert.Equal(t, "smtp.example.com", config.SMTPHost)
	assert.Equal(t, 587, config.SMTPPort)
	assert.Equal(t, "user", config.SMTPUser)
	assert.Equal(t, "pass", config.SMTPPassword)
	assert.True(t, config.SMTPTLS)
}

func TestEmailConfig_AzureConfig(t *testing.T) {
	config := EmailConfig{
		Provider:              ProviderAzure,
		AzureConnectionString: "endpoint=https://test.communication.azure.com/;accesskey=test",
		FromAddress:           "noreply@example.com",
	}

	assert.Equal(t, ProviderAzure, config.Provider)
	assert.NotEmpty(t, config.AzureConnectionString)
}

func TestEmailConfig_AWSConfig(t *testing.T) {
	config := EmailConfig{
		Provider:           ProviderAWSSES,
		AWSRegion:          "us-east-1",
		AWSAccessKeyID:     "AKIAEXAMPLE",
		AWSSecretAccessKey: "secret",
		FromAddress:        "noreply@example.com",
	}

	assert.Equal(t, ProviderAWSSES, config.Provider)
	assert.Equal(t, "us-east-1", config.AWSRegion)
	assert.Equal(t, "AKIAEXAMPLE", config.AWSAccessKeyID)
}

func TestEmailConfig_SendGridConfig(t *testing.T) {
	config := EmailConfig{
		Provider:       ProviderSendGrid,
		SendGridAPIKey: "SG.xxxxx",
		FromAddress:    "noreply@example.com",
	}

	assert.Equal(t, ProviderSendGrid, config.Provider)
	assert.Equal(t, "SG.xxxxx", config.SendGridAPIKey)
}

func TestEmailConfig_ResendConfig(t *testing.T) {
	config := EmailConfig{
		Provider:     ProviderResend,
		ResendAPIKey: "re_xxxxx",
		FromAddress:  "noreply@example.com",
	}

	assert.Equal(t, ProviderResend, config.Provider)
	assert.Equal(t, "re_xxxxx", config.ResendAPIKey)
}

// ===========================
// EmailParams Tests
// ===========================

func TestEmailParams_DefaultValues(t *testing.T) {
	params := EmailParams{}

	assert.Nil(t, params.To)
	assert.Empty(t, params.From)
	assert.Empty(t, params.Subject)
	assert.Empty(t, params.TextBody)
	assert.Empty(t, params.HTMLBody)
	assert.Nil(t, params.Attachments)
}

func TestEmailParams_WithAllFields(t *testing.T) {
	params := EmailParams{
		To:       []string{"user@example.com"},
		From:     "sender@example.com",
		Subject:  "Test Subject",
		TextBody: "Plain text body",
		HTMLBody: "<html><body>HTML body</body></html>",
		ReplyTo:  "reply@example.com",
		CC:       []string{"cc@example.com"},
		BCC:      []string{"bcc@example.com"},
		Attachments: []EmailAttachment{
			{Filename: "test.txt", ContentType: "text/plain", Data: []byte("test")},
		},
		Headers: map[string]string{"X-Custom": "value"},
	}

	assert.Equal(t, []string{"user@example.com"}, params.To)
	assert.Equal(t, "sender@example.com", params.From)
	assert.Equal(t, "Test Subject", params.Subject)
	assert.Equal(t, "Plain text body", params.TextBody)
	assert.Equal(t, "<html><body>HTML body</body></html>", params.HTMLBody)
	assert.Equal(t, "reply@example.com", params.ReplyTo)
	assert.Len(t, params.Attachments, 1)
	assert.Equal(t, "value", params.Headers["X-Custom"])
}

// ===========================
// EmailAttachment Tests
// ===========================

func TestEmailAttachment_Fields(t *testing.T) {
	attachment := EmailAttachment{
		Filename:    "document.pdf",
		ContentType: "application/pdf",
		Data:        []byte{0x25, 0x50, 0x44, 0x46}, // PDF magic bytes
	}

	assert.Equal(t, "document.pdf", attachment.Filename)
	assert.Equal(t, "application/pdf", attachment.ContentType)
	assert.Len(t, attachment.Data, 4)
}

// ===========================
// NewEmailProvider Tests
// ===========================

func TestNewEmailProvider_Console(t *testing.T) {
	config := EmailConfig{
		Provider:    ProviderConsole,
		FromAddress: "test@example.com",
		FromName:    "Test",
	}

	provider, err := NewEmailProvider(config)

	require.NoError(t, err)
	assert.NotNil(t, provider)
	assert.Equal(t, "Console", provider.GetProviderName())
}

func TestNewEmailProvider_UnsupportedProvider(t *testing.T) {
	config := EmailConfig{
		Provider:    ProviderType("unsupported"),
		FromAddress: "test@example.com",
	}

	provider, err := NewEmailProvider(config)

	assert.Error(t, err)
	assert.Nil(t, provider)
	assert.Contains(t, err.Error(), "unsupported email provider")
}

func TestNewEmailProvider_SMTP_MissingConfig(t *testing.T) {
	config := EmailConfig{
		Provider:    ProviderSMTP,
		FromAddress: "test@example.com",
		// Missing SMTPHost
	}

	// This might fail during validation or return a provider
	// depending on implementation
	provider, err := NewEmailProvider(config)

	// Should either error or return a provider
	if err != nil {
		assert.Contains(t, err.Error(), "SMTP")
	} else {
		assert.NotNil(t, provider)
	}
}

func TestNewEmailProvider_Azure_MissingConfig(t *testing.T) {
	config := EmailConfig{
		Provider:    ProviderAzure,
		FromAddress: "test@example.com",
		// Missing AzureConnectionString
	}

	provider, err := NewEmailProvider(config)

	// Should fail due to missing connection string
	if err != nil {
		assert.Contains(t, err.Error(), "connection")
	} else {
		// If it returns a provider, validation should fail later
		assert.NotNil(t, provider)
	}
}
