package email

import (
	"testing"

	"github.com/opena2a/identity/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================
// SMTPEmailService Tests
// ===========================

func TestNewSMTPEmailService_ValidConfig(t *testing.T) {
	config := domain.EmailConfig{
		FromAddress: "noreply@example.com",
		FromName:    "Example",
		SMTP: domain.SMTPConfig{
			Host:       "smtp.example.com",
			Port:       587,
			Username:   "user",
			Password:   "password",
			TLSEnabled: true,
		},
	}

	service, err := NewSMTPEmailService(config)

	require.NoError(t, err)
	assert.NotNil(t, service)
	assert.Equal(t, "smtp.example.com", service.host)
	assert.Equal(t, 587, service.port)
	assert.Equal(t, "user", service.username)
	assert.Equal(t, "password", service.password)
	assert.True(t, service.tlsEnabled)
	assert.Equal(t, "noreply@example.com", service.fromAddress)
	assert.Equal(t, "Example", service.fromName)
}

func TestNewSMTPEmailService_MissingHost(t *testing.T) {
	config := domain.EmailConfig{
		FromAddress: "noreply@example.com",
		FromName:    "Example",
		SMTP: domain.SMTPConfig{
			Port: 587,
		},
	}

	service, err := NewSMTPEmailService(config)

	assert.Error(t, err)
	assert.Nil(t, service)
	assert.Contains(t, err.Error(), "smtp host is required")
}

func TestNewSMTPEmailService_MissingPort(t *testing.T) {
	config := domain.EmailConfig{
		FromAddress: "noreply@example.com",
		FromName:    "Example",
		SMTP: domain.SMTPConfig{
			Host: "smtp.example.com",
		},
	}

	service, err := NewSMTPEmailService(config)

	assert.Error(t, err)
	assert.Nil(t, service)
	assert.Contains(t, err.Error(), "smtp port is required")
}

func TestNewSMTPEmailService_MissingFromAddress(t *testing.T) {
	config := domain.EmailConfig{
		FromName: "Example",
		SMTP: domain.SMTPConfig{
			Host: "smtp.example.com",
			Port: 587,
		},
	}

	service, err := NewSMTPEmailService(config)

	assert.Error(t, err)
	assert.Nil(t, service)
	assert.Contains(t, err.Error(), "from email address is required")
}

func TestSMTPEmailService_BuildMessage_PlainText(t *testing.T) {
	service := &SMTPEmailService{
		host:        "smtp.example.com",
		port:        587,
		fromAddress: "noreply@example.com",
		fromName:    "Example",
	}

	message := service.buildMessage("Example <noreply@example.com>", "recipient@example.com", "Test Subject", "This is a test message", false)

	assert.Contains(t, message, "From: Example <noreply@example.com>")
	assert.Contains(t, message, "To: recipient@example.com")
	assert.Contains(t, message, "Subject: Test Subject")
	assert.Contains(t, message, "text/plain")
	assert.Contains(t, message, "This is a test message")
}

func TestSMTPEmailService_BuildMessage_HTML(t *testing.T) {
	service := &SMTPEmailService{
		host:        "smtp.example.com",
		port:        587,
		fromAddress: "noreply@example.com",
		fromName:    "Example",
	}

	message := service.buildMessage("Example <noreply@example.com>", "recipient@example.com", "Test Subject", "<html><body>HTML content</body></html>", true)

	assert.Contains(t, message, "From: Example <noreply@example.com>")
	assert.Contains(t, message, "To: recipient@example.com")
	assert.Contains(t, message, "Subject: Test Subject")
	assert.Contains(t, message, "text/html")
	assert.Contains(t, message, "<html><body>HTML content</body></html>")
}

func TestSMTPEmailService_BuildMessage_HasMIMEHeaders(t *testing.T) {
	service := &SMTPEmailService{
		host:        "smtp.example.com",
		port:        587,
		fromAddress: "noreply@example.com",
	}

	message := service.buildMessage("sender@example.com", "recipient@example.com", "Test", "Body", false)

	assert.Contains(t, message, "MIME-Version: 1.0")
	assert.Contains(t, message, "charset=\"UTF-8\"")
	assert.Contains(t, message, "Date:")
}

func TestSMTPEmailService_GetMetrics_Initial(t *testing.T) {
	config := domain.EmailConfig{
		FromAddress: "noreply@example.com",
		FromName:    "Example",
		SMTP: domain.SMTPConfig{
			Host: "smtp.example.com",
			Port: 587,
		},
	}

	service, err := NewSMTPEmailService(config)
	require.NoError(t, err)

	metrics := service.GetMetrics()

	assert.Equal(t, int64(0), metrics.TotalSent)
	assert.Equal(t, int64(0), metrics.TotalFailed)
	assert.Equal(t, float64(0), metrics.SuccessRate)
	assert.True(t, metrics.LastSentAt.IsZero())
	assert.True(t, metrics.LastFailedAt.IsZero())
}

func TestSMTPEmailService_RecordSuccess(t *testing.T) {
	service := &SMTPEmailService{
		metrics: &emailMetrics{
			failuresByType: make(map[string]int64),
			sentByTemplate: make(map[domain.EmailTemplate]int64),
		},
	}

	service.recordSuccess(100, "")

	assert.Equal(t, int64(1), service.metrics.totalSent)
	assert.False(t, service.metrics.lastSentAt.IsZero())
}

func TestSMTPEmailService_RecordFailure(t *testing.T) {
	service := &SMTPEmailService{
		metrics: &emailMetrics{
			failuresByType: make(map[string]int64),
			sentByTemplate: make(map[domain.EmailTemplate]int64),
		},
	}

	service.recordFailure("smtp_dial_error")
	service.recordFailure("smtp_dial_error")
	service.recordFailure("auth_error")

	assert.Equal(t, int64(3), service.metrics.totalFailed)
	assert.Equal(t, int64(2), service.metrics.failuresByType["smtp_dial_error"])
	assert.Equal(t, int64(1), service.metrics.failuresByType["auth_error"])
	assert.False(t, service.metrics.lastFailedAt.IsZero())
}

func TestSMTPEmailService_GetMetrics_WithData(t *testing.T) {
	service := &SMTPEmailService{
		metrics: &emailMetrics{
			totalSent:      8,
			totalFailed:    2,
			failuresByType: map[string]int64{"dial_error": 2},
			sentByTemplate: map[domain.EmailTemplate]int64{domain.TemplateWelcome: 3},
		},
	}

	metrics := service.GetMetrics()

	assert.Equal(t, int64(8), metrics.TotalSent)
	assert.Equal(t, int64(2), metrics.TotalFailed)
	assert.Equal(t, float64(80), metrics.SuccessRate)
	assert.Equal(t, int64(2), metrics.FailuresByType["dial_error"])
	assert.Equal(t, int64(3), metrics.SentByTemplate[domain.TemplateWelcome])
}

func TestSMTPEmailService_SendEmail_FailsWithInvalidHost(t *testing.T) {
	config := domain.EmailConfig{
		FromAddress: "noreply@example.com",
		FromName:    "Example",
		SMTP: domain.SMTPConfig{
			Host:       "invalid.host.that.does.not.exist.example.com",
			Port:       12345,
			TLSEnabled: false,
		},
	}

	service, err := NewSMTPEmailService(config)
	require.NoError(t, err)

	err = service.SendEmail("recipient@example.com", "Test", "Test message", false)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to send email")
}

func TestSMTPEmailService_SendEmail_FailsWithTLS(t *testing.T) {
	config := domain.EmailConfig{
		FromAddress: "noreply@example.com",
		FromName:    "Example",
		SMTP: domain.SMTPConfig{
			Host:       "invalid.host.that.does.not.exist.example.com",
			Port:       12345,
			TLSEnabled: true,
		},
	}

	service, err := NewSMTPEmailService(config)
	require.NoError(t, err)

	err = service.SendEmail("recipient@example.com", "Test", "Test message", false)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to dial SMTP server")
}

func TestSMTPEmailService_ValidateConnection_Fails(t *testing.T) {
	config := domain.EmailConfig{
		FromAddress: "noreply@example.com",
		FromName:    "Example",
		SMTP: domain.SMTPConfig{
			Host:       "invalid.host.example.com",
			Port:       12345,
			TLSEnabled: false,
		},
	}

	service, err := NewSMTPEmailService(config)
	require.NoError(t, err)

	err = service.ValidateConnection()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect to SMTP server")
}

func TestSMTPEmailService_ValidateConnection_TLS_Fails(t *testing.T) {
	config := domain.EmailConfig{
		FromAddress: "noreply@example.com",
		FromName:    "Example",
		SMTP: domain.SMTPConfig{
			Host:       "invalid.host.example.com",
			Port:       12345,
			TLSEnabled: true,
		},
	}

	service, err := NewSMTPEmailService(config)
	require.NoError(t, err)

	err = service.ValidateConnection()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect to SMTP server")
}

func TestSMTPEmailService_SendEmail_WithFromName(t *testing.T) {
	config := domain.EmailConfig{
		FromAddress: "noreply@example.com",
		FromName:    "Example App",
		SMTP: domain.SMTPConfig{
			Host:       "invalid.host.example.com",
			Port:       12345,
			TLSEnabled: false,
		},
	}

	service, err := NewSMTPEmailService(config)
	require.NoError(t, err)

	err = service.SendEmail("recipient@example.com", "Test", "Test message", false)

	// Should fail because host doesn't exist
	assert.Error(t, err)
}

func TestSMTPEmailService_SendEmail_WithAuth(t *testing.T) {
	config := domain.EmailConfig{
		FromAddress: "noreply@example.com",
		FromName:    "Example",
		SMTP: domain.SMTPConfig{
			Host:       "invalid.host.example.com",
			Port:       12345,
			Username:   "testuser",
			Password:   "testpass",
			TLSEnabled: false,
		},
	}

	service, err := NewSMTPEmailService(config)
	require.NoError(t, err)

	err = service.SendEmail("recipient@example.com", "Test", "Test message", false)

	// Should fail because host doesn't exist
	assert.Error(t, err)
}
