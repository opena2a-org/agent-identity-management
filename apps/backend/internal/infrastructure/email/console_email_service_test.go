package email

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/opena2a/identity/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================
// NewConsoleEmailService Tests
// ===========================

func TestNewConsoleEmailService_Success(t *testing.T) {
	config := domain.EmailConfig{
		Provider:    "console",
		FromAddress: "test@example.com",
		FromName:    "Test Sender",
	}

	service, err := NewConsoleEmailService(config)

	require.NoError(t, err)
	assert.NotNil(t, service)
}

func TestNewConsoleEmailService_WithEmptyFromName(t *testing.T) {
	config := domain.EmailConfig{
		Provider:    "console",
		FromAddress: "test@example.com",
		FromName:    "",
	}

	service, err := NewConsoleEmailService(config)

	require.NoError(t, err)
	assert.NotNil(t, service)
}

// ===========================
// SendEmail Tests
// ===========================

func TestConsoleEmailService_SendEmail_PlainText(t *testing.T) {
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	config := domain.EmailConfig{
		Provider:    "console",
		FromAddress: "sender@example.com",
		FromName:    "Sender",
	}
	service, err := NewConsoleEmailService(config)
	require.NoError(t, err)

	err = service.SendEmail("recipient@example.com", "Test Subject", "Test body", false)

	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "EMAIL")
	assert.Contains(t, output, "sender@example.com")
	assert.Contains(t, output, "recipient@example.com")
	assert.Contains(t, output, "Test Subject")
	assert.Contains(t, output, "Test body")
	assert.Contains(t, output, "Text Body:")
}

func TestConsoleEmailService_SendEmail_HTML(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	config := domain.EmailConfig{
		Provider:    "console",
		FromAddress: "sender@example.com",
		FromName:    "Sender",
	}
	service, err := NewConsoleEmailService(config)
	require.NoError(t, err)

	err = service.SendEmail("recipient@example.com", "Test Subject", "<html>Body</html>", true)

	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "HTML Body:")
	assert.Contains(t, output, "<html>Body</html>")
}

// ===========================
// SendTemplatedEmail Tests
// ===========================

func TestConsoleEmailService_SendTemplatedEmail_Success(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	config := domain.EmailConfig{
		Provider:    "console",
		FromAddress: "sender@example.com",
		FromName:    "Sender",
	}
	service, err := NewConsoleEmailService(config)
	require.NoError(t, err)

	data := map[string]interface{}{
		"UserName":     "Test User",
		"DashboardURL": "https://example.com/dashboard",
	}

	err = service.SendTemplatedEmail(domain.TemplateWelcome, "recipient@example.com", data)

	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "TEMPLATED EMAIL")
	assert.Contains(t, output, "Template: welcome")
}

// ===========================
// SendBulkEmail Tests
// ===========================

func TestConsoleEmailService_SendBulkEmail_Success(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	config := domain.EmailConfig{
		Provider:    "console",
		FromAddress: "sender@example.com",
		FromName:    "Sender",
	}
	service, err := NewConsoleEmailService(config)
	require.NoError(t, err)

	recipients := []string{"user1@example.com", "user2@example.com", "user3@example.com"}
	err = service.SendBulkEmail(recipients, "Bulk Subject", "Bulk body", false)

	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "BULK EMAIL")
	assert.Contains(t, output, "user1@example.com, user2@example.com, user3@example.com")
}

func TestConsoleEmailService_SendBulkEmail_HTML(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	config := domain.EmailConfig{
		Provider:    "console",
		FromAddress: "sender@example.com",
		FromName:    "Sender",
	}
	service, err := NewConsoleEmailService(config)
	require.NoError(t, err)

	recipients := []string{"user1@example.com"}
	err = service.SendBulkEmail(recipients, "Bulk Subject", "<html>Body</html>", true)

	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "HTML Body:")
}

// ===========================
// ValidateConnection Tests
// ===========================

func TestConsoleEmailService_ValidateConnection(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	config := domain.EmailConfig{
		Provider:    "console",
		FromAddress: "sender@example.com",
		FromName:    "Sender",
	}
	service, err := NewConsoleEmailService(config)
	require.NoError(t, err)

	err = service.ValidateConnection()

	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "Console email service initialized")
	assert.Contains(t, output, "sender@example.com")
}
