package email

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================
// SMTPProvider Tests
// ===========================

func TestNewSMTPProvider_ValidConfig(t *testing.T) {
	config := EmailConfig{
		SMTPHost:     "smtp.example.com",
		SMTPPort:     587,
		SMTPUser:     "user",
		SMTPPassword: "password",
		SMTPTLS:      true,
		FromAddress:  "noreply@example.com",
		FromName:     "Example",
	}

	provider, err := NewSMTPProvider(config)

	require.NoError(t, err)
	assert.NotNil(t, provider)
	assert.Equal(t, "smtp.example.com", provider.host)
	assert.Equal(t, 587, provider.port)
	assert.Equal(t, "user", provider.user)
	assert.Equal(t, "password", provider.password)
	assert.True(t, provider.useTLS)
}

func TestNewSMTPProvider_MissingHost(t *testing.T) {
	config := EmailConfig{
		SMTPPort:    587,
		FromAddress: "noreply@example.com",
		FromName:    "Example",
	}

	provider, err := NewSMTPProvider(config)

	assert.Error(t, err)
	assert.Nil(t, provider)
	assert.Contains(t, err.Error(), "SMTP host is required")
}

func TestNewSMTPProvider_MissingPort(t *testing.T) {
	config := EmailConfig{
		SMTPHost:    "smtp.example.com",
		FromAddress: "noreply@example.com",
		FromName:    "Example",
	}

	provider, err := NewSMTPProvider(config)

	assert.Error(t, err)
	assert.Nil(t, provider)
	assert.Contains(t, err.Error(), "SMTP port is required")
}

func TestNewSMTPProvider_FromFormat(t *testing.T) {
	// When FromAddress and FromName are provided, from field is set
	config := EmailConfig{
		SMTPHost:    "smtp.example.com",
		SMTPPort:    587,
		FromAddress: "noreply@example.com",
		FromName:    "Example",
	}

	provider, err := NewSMTPProvider(config)

	require.NoError(t, err)
	assert.Contains(t, provider.from, "noreply@example.com")
	assert.Contains(t, provider.from, "Example")
}

func TestSMTPProvider_GetProviderName(t *testing.T) {
	config := EmailConfig{
		SMTPHost:    "smtp.example.com",
		SMTPPort:    587,
		FromAddress: "noreply@example.com",
		FromName:    "Example",
	}

	provider, err := NewSMTPProvider(config)
	require.NoError(t, err)

	assert.Equal(t, "SMTP", provider.GetProviderName())
}

func TestSMTPProvider_ValidateConfig_Valid(t *testing.T) {
	provider := &SMTPProvider{
		host: "smtp.example.com",
		port: 587,
		from: "Example <noreply@example.com>",
	}

	err := provider.ValidateConfig()

	assert.NoError(t, err)
}

func TestSMTPProvider_ValidateConfig_MissingHost(t *testing.T) {
	provider := &SMTPProvider{
		port: 587,
		from: "Example <noreply@example.com>",
	}

	err := provider.ValidateConfig()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SMTP host is required")
}

func TestSMTPProvider_ValidateConfig_MissingPort(t *testing.T) {
	provider := &SMTPProvider{
		host: "smtp.example.com",
		from: "Example <noreply@example.com>",
	}

	err := provider.ValidateConfig()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SMTP port is required")
}

func TestSMTPProvider_ValidateConfig_MissingFrom(t *testing.T) {
	provider := &SMTPProvider{
		host: "smtp.example.com",
		port: 587,
	}

	err := provider.ValidateConfig()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "From address is required")
}

func TestSMTPProvider_BuildMessage_TextOnly(t *testing.T) {
	provider := &SMTPProvider{
		host: "smtp.example.com",
		port: 587,
		from: "Example <noreply@example.com>",
	}

	params := EmailParams{
		From:     "sender@example.com",
		To:       []string{"recipient@example.com"},
		Subject:  "Test Subject",
		TextBody: "This is a test message",
	}

	message := provider.buildMessage(params)

	assert.Contains(t, message, "From: sender@example.com")
	assert.Contains(t, message, "To: recipient@example.com")
	assert.Contains(t, message, "Subject: Test Subject")
	assert.Contains(t, message, "This is a test message")
	assert.Contains(t, message, "text/plain")
}

func TestSMTPProvider_BuildMessage_WithHTML(t *testing.T) {
	provider := &SMTPProvider{
		host: "smtp.example.com",
		port: 587,
		from: "Example <noreply@example.com>",
	}

	params := EmailParams{
		From:     "sender@example.com",
		To:       []string{"recipient@example.com"},
		Subject:  "Test Subject",
		TextBody: "Plain text version",
		HTMLBody: "<html><body>HTML version</body></html>",
	}

	message := provider.buildMessage(params)

	assert.Contains(t, message, "multipart/alternative")
	assert.Contains(t, message, "boundary-aim")
	assert.Contains(t, message, "Plain text version")
	assert.Contains(t, message, "HTML version")
}

func TestSMTPProvider_BuildMessage_WithCC(t *testing.T) {
	provider := &SMTPProvider{
		host: "smtp.example.com",
		port: 587,
		from: "Example <noreply@example.com>",
	}

	params := EmailParams{
		From:     "sender@example.com",
		To:       []string{"recipient@example.com"},
		CC:       []string{"cc@example.com"},
		Subject:  "Test Subject",
		TextBody: "Test message",
	}

	message := provider.buildMessage(params)

	assert.Contains(t, message, "Cc: cc@example.com")
}

func TestSMTPProvider_BuildMessage_WithReplyTo(t *testing.T) {
	provider := &SMTPProvider{
		host: "smtp.example.com",
		port: 587,
		from: "Example <noreply@example.com>",
	}

	params := EmailParams{
		From:     "sender@example.com",
		To:       []string{"recipient@example.com"},
		ReplyTo:  "reply@example.com",
		Subject:  "Test Subject",
		TextBody: "Test message",
	}

	message := provider.buildMessage(params)

	assert.Contains(t, message, "Reply-To: reply@example.com")
}

func TestSMTPProvider_BuildMessage_WithCustomHeaders(t *testing.T) {
	provider := &SMTPProvider{
		host: "smtp.example.com",
		port: 587,
		from: "Example <noreply@example.com>",
	}

	params := EmailParams{
		From:     "sender@example.com",
		To:       []string{"recipient@example.com"},
		Subject:  "Test Subject",
		TextBody: "Test message",
		Headers: map[string]string{
			"X-Custom-Header": "custom-value",
		},
	}

	message := provider.buildMessage(params)

	assert.Contains(t, message, "X-Custom-Header: custom-value")
}

func TestSMTPProvider_SendEmail_FailsWithoutServer(t *testing.T) {
	config := EmailConfig{
		SMTPHost:    "invalid.host.example.com",
		SMTPPort:    12345,
		FromAddress: "noreply@example.com",
		FromName:    "Example",
	}

	provider, err := NewSMTPProvider(config)
	require.NoError(t, err)

	params := EmailParams{
		From:     "sender@example.com",
		To:       []string{"recipient@example.com"},
		Subject:  "Test",
		TextBody: "Test message",
	}

	err = provider.SendEmail(context.Background(), params)

	// Should fail because the host doesn't exist
	assert.Error(t, err)
}
