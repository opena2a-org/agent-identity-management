package email

import (
	"testing"
	"time"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================
// Azure Email Service Tests
// ===========================

func TestNewAzureEmailService_MissingConnectionString(t *testing.T) {
	config := domain.EmailConfig{
		FromAddress: "noreply@example.com",
		Azure: domain.AzureEmailConfig{
			ConnectionString: "",
		},
	}

	service, err := NewAzureEmailService(config)

	assert.Error(t, err)
	assert.Nil(t, service)
	assert.Contains(t, err.Error(), "connection string is required")
}

func TestNewAzureEmailService_MissingFromAddress(t *testing.T) {
	config := domain.EmailConfig{
		Azure: domain.AzureEmailConfig{
			ConnectionString: "endpoint=https://example.communication.azure.com/;accesskey=abc123==",
		},
	}

	service, err := NewAzureEmailService(config)

	assert.Error(t, err)
	assert.Nil(t, service)
	assert.Contains(t, err.Error(), "from email address is required")
}

func TestNewAzureEmailService_InvalidConnectionString(t *testing.T) {
	config := domain.EmailConfig{
		FromAddress: "noreply@example.com",
		Azure: domain.AzureEmailConfig{
			ConnectionString: "invalid-connection-string",
		},
	}

	service, err := NewAzureEmailService(config)

	assert.Error(t, err)
	assert.Nil(t, service)
	assert.Contains(t, err.Error(), "failed to parse")
}

func TestNewAzureEmailService_ValidConfig(t *testing.T) {
	config := domain.EmailConfig{
		FromAddress: "noreply@example.com",
		FromName:    "Example App",
		Azure: domain.AzureEmailConfig{
			ConnectionString: "endpoint=https://example.communication.azure.com;accesskey=dGVzdGFjY2Vzc2tleQ==",
		},
	}

	service, err := NewAzureEmailService(config)

	require.NoError(t, err)
	assert.NotNil(t, service)
	assert.Equal(t, "https://example.communication.azure.com", service.endpoint)
	assert.Equal(t, "dGVzdGFjY2Vzc2tleQ==", service.accessKey)
	assert.Equal(t, "noreply@example.com", service.fromAddress)
	assert.Equal(t, "Example App", service.fromName)
}

// ===========================
// parseAzureConnectionString Tests
// ===========================

func TestParseAzureConnectionString_ValidFormat(t *testing.T) {
	connStr := "endpoint=https://myresource.communication.azure.com;accesskey=myAccessKey123=="

	endpoint, accessKey, err := parseAzureConnectionString(connStr)

	require.NoError(t, err)
	assert.Equal(t, "https://myresource.communication.azure.com", endpoint)
	assert.Equal(t, "myAccessKey123==", accessKey)
}

func TestParseAzureConnectionString_ValidFormatWithSlash(t *testing.T) {
	connStr := "endpoint=https://myresource.communication.azure.com/;accesskey=myAccessKey123=="

	endpoint, accessKey, err := parseAzureConnectionString(connStr)

	require.NoError(t, err)
	// Endpoint should have trailing slash removed
	assert.Equal(t, "https://myresource.communication.azure.com", endpoint)
	assert.Equal(t, "myAccessKey123==", accessKey)
}

func TestParseAzureConnectionString_MissingEndpoint(t *testing.T) {
	connStr := "accesskey=myAccessKey123=="

	_, _, err := parseAzureConnectionString(connStr)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "endpoint")
}

func TestParseAzureConnectionString_MissingAccessKey(t *testing.T) {
	connStr := "endpoint=https://myresource.communication.azure.com"

	_, _, err := parseAzureConnectionString(connStr)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access key")
}

func TestParseAzureConnectionString_EmptyString(t *testing.T) {
	connStr := ""

	_, _, err := parseAzureConnectionString(connStr)

	assert.Error(t, err)
}

func TestParseAzureConnectionString_InvalidFormat(t *testing.T) {
	connStr := "not-a-valid-format"

	_, _, err := parseAzureConnectionString(connStr)

	assert.Error(t, err)
}

// ===========================
// emailMetrics Tests
// ===========================

func TestEmailMetrics_DefaultValues(t *testing.T) {
	metrics := &emailMetrics{
		failuresByType: make(map[string]int64),
		sentByTemplate: make(map[domain.EmailTemplate]int64),
	}

	assert.Equal(t, int64(0), metrics.totalSent)
	assert.Equal(t, int64(0), metrics.totalFailed)
	assert.True(t, metrics.lastSentAt.IsZero())
	assert.True(t, metrics.lastFailedAt.IsZero())
}

// ===========================
// Azure Email Request/Response Struct Tests
// ===========================

func TestAzureEmailRequest_Structure(t *testing.T) {
	req := azureEmailRequest{
		SenderAddress: "sender@example.com",
		Content: azureEmailContent{
			Subject:   "Test Subject",
			PlainText: "Plain text body",
			HTML:      "<html>HTML body</html>",
		},
		Recipients: azureEmailRecipients{
			To: []azureEmailAddress{
				{Address: "recipient@example.com", DisplayName: "Recipient"},
			},
		},
	}

	assert.Equal(t, "sender@example.com", req.SenderAddress)
	assert.Equal(t, "Test Subject", req.Content.Subject)
	assert.Len(t, req.Recipients.To, 1)
}

func TestAzureEmailResponse_Structure(t *testing.T) {
	resp := azureEmailResponse{
		ID:     "message-123",
		Status: "Queued",
	}

	assert.Equal(t, "message-123", resp.ID)
	assert.Equal(t, "Queued", resp.Status)
	assert.Nil(t, resp.Error)
}

func TestAzureEmailResponse_WithError(t *testing.T) {
	resp := azureEmailResponse{
		ID:     "",
		Status: "Failed",
		Error: &struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}{
			Code:    "InvalidRecipient",
			Message: "The recipient address is invalid",
		},
	}

	assert.NotNil(t, resp.Error)
	assert.Equal(t, "InvalidRecipient", resp.Error.Code)
}

// ===========================
// Azure Email Service Metrics Tests
// ===========================

func TestAzureEmailService_GetMetrics_Initial(t *testing.T) {
	config := domain.EmailConfig{
		FromAddress: "noreply@example.com",
		FromName:    "Example",
		Azure: domain.AzureEmailConfig{
			ConnectionString: "endpoint=https://test.communication.azure.com;accesskey=dGVzdGtleQ==",
		},
	}

	service, err := NewAzureEmailService(config)
	require.NoError(t, err)

	metrics := service.GetMetrics()

	assert.Equal(t, int64(0), metrics.TotalSent)
	assert.Equal(t, int64(0), metrics.TotalFailed)
	assert.Equal(t, float64(0), metrics.SuccessRate)
	assert.True(t, metrics.LastSentAt.IsZero())
	assert.True(t, metrics.LastFailedAt.IsZero())
}

func TestAzureEmailService_RecordSuccess(t *testing.T) {
	service := &AzureEmailService{
		metrics: &emailMetrics{
			failuresByType: make(map[string]int64),
			sentByTemplate: make(map[domain.EmailTemplate]int64),
		},
	}

	service.recordSuccess(100, domain.TemplateWelcome)
	service.recordSuccess(50, domain.TemplatePasswordReset)

	assert.Equal(t, int64(2), service.metrics.totalSent)
	assert.False(t, service.metrics.lastSentAt.IsZero())
}

func TestAzureEmailService_RecordFailure(t *testing.T) {
	service := &AzureEmailService{
		metrics: &emailMetrics{
			failuresByType: make(map[string]int64),
			sentByTemplate: make(map[domain.EmailTemplate]int64),
		},
	}

	service.recordFailure("auth_error")
	service.recordFailure("auth_error")
	service.recordFailure("timeout")

	assert.Equal(t, int64(3), service.metrics.totalFailed)
	assert.Equal(t, int64(2), service.metrics.failuresByType["auth_error"])
	assert.Equal(t, int64(1), service.metrics.failuresByType["timeout"])
	assert.False(t, service.metrics.lastFailedAt.IsZero())
}

func TestAzureEmailService_GetMetrics_WithData(t *testing.T) {
	service := &AzureEmailService{
		metrics: &emailMetrics{
			totalSent:      10,
			totalFailed:    5,
			failuresByType: map[string]int64{"dial_error": 3, "auth_error": 2},
			sentByTemplate: make(map[domain.EmailTemplate]int64),
		},
	}

	metrics := service.GetMetrics()

	assert.Equal(t, int64(10), metrics.TotalSent)
	assert.Equal(t, int64(5), metrics.TotalFailed)
	// Success rate: 10 / 15 * 100 = 66.67%
	assert.InDelta(t, 66.67, metrics.SuccessRate, 0.01)
	assert.Equal(t, int64(3), metrics.FailuresByType["dial_error"])
	assert.Equal(t, int64(2), metrics.FailuresByType["auth_error"])
}

func TestAzureEmailService_GetMetrics_ZeroTotal(t *testing.T) {
	service := &AzureEmailService{
		metrics: &emailMetrics{
			totalSent:      0,
			totalFailed:    0,
			failuresByType: make(map[string]int64),
			sentByTemplate: make(map[domain.EmailTemplate]int64),
		},
	}

	metrics := service.GetMetrics()

	// Success rate should be 0 when there are no emails
	assert.Equal(t, float64(0), metrics.SuccessRate)
}

func TestAzureEmailService_GetMetrics_WithSentByTemplate(t *testing.T) {
	service := &AzureEmailService{
		metrics: &emailMetrics{
			totalSent:      5,
			totalFailed:    0,
			failuresByType: make(map[string]int64),
			sentByTemplate: map[domain.EmailTemplate]int64{
				domain.TemplateWelcome:       3,
				domain.TemplatePasswordReset: 2,
			},
		},
	}

	metrics := service.GetMetrics()

	assert.Equal(t, int64(5), metrics.TotalSent)
	assert.Equal(t, int64(3), metrics.SentByTemplate[domain.TemplateWelcome])
	assert.Equal(t, int64(2), metrics.SentByTemplate[domain.TemplatePasswordReset])
}

func TestAzureEmailService_GetMetrics_WithTimestamps(t *testing.T) {
	now := time.Now()
	service := &AzureEmailService{
		metrics: &emailMetrics{
			totalSent:      3,
			totalFailed:    1,
			lastSentAt:     now,
			lastFailedAt:   now.Add(-time.Hour),
			failuresByType: make(map[string]int64),
			sentByTemplate: make(map[domain.EmailTemplate]int64),
		},
	}

	metrics := service.GetMetrics()

	assert.Equal(t, now, metrics.LastSentAt)
	assert.Equal(t, now.Add(-time.Hour), metrics.LastFailedAt)
}

// ===========================
// generateAuthHeader Tests
// ===========================

func TestAzureEmailService_GenerateAuthHeader(t *testing.T) {
	service := &AzureEmailService{
		accessKey: "dGVzdGFjY2Vzc2tleQ==",
	}

	header, err := service.generateAuthHeader("POST", "/emails:send", "2023-01-01T00:00:00Z", []byte(`{"test":"data"}`))

	require.NoError(t, err)
	assert.Contains(t, header, "HMAC-SHA256")
	assert.Contains(t, header, "SignedHeaders=date;host;x-ms-content-sha256")
	assert.Contains(t, header, "Signature=")
}

func TestAzureEmailService_GenerateAuthHeader_EmptyBody(t *testing.T) {
	service := &AzureEmailService{
		accessKey: "dGVzdGFjY2Vzc2tleQ==",
	}

	header, err := service.generateAuthHeader("POST", "/emails:send", "2023-01-01T00:00:00Z", []byte{})

	require.NoError(t, err)
	assert.Contains(t, header, "HMAC-SHA256")
}

// ===========================
// ValidateConnection Tests
// ===========================

func TestAzureEmailService_ValidateConnection_EmptyEndpoint(t *testing.T) {
	service := &AzureEmailService{
		endpoint: "",
	}

	err := service.ValidateConnection()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "endpoint")
}

func TestAzureEmailService_ValidateConnection_EmptyAccessKey(t *testing.T) {
	service := &AzureEmailService{
		endpoint:  "https://test.communication.azure.com",
		accessKey: "",
	}

	err := service.ValidateConnection()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "access key")
}

func TestAzureEmailService_ValidateConnection_EmptyFromAddress(t *testing.T) {
	service := &AzureEmailService{
		endpoint:    "https://test.communication.azure.com",
		accessKey:   "abc123",
		fromAddress: "",
	}

	err := service.ValidateConnection()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "from address")
}

func TestAzureEmailService_ValidateConnection_HTTPEndpoint(t *testing.T) {
	service := &AzureEmailService{
		endpoint:    "http://test.communication.azure.com",
		accessKey:   "abc123",
		fromAddress: "noreply@example.com",
	}

	err := service.ValidateConnection()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "HTTPS")
}

func TestAzureEmailService_ValidateConnection_Valid(t *testing.T) {
	service := &AzureEmailService{
		endpoint:    "https://test.communication.azure.com",
		accessKey:   "abc123",
		fromAddress: "noreply@example.com",
	}

	err := service.ValidateConnection()

	assert.NoError(t, err)
}
