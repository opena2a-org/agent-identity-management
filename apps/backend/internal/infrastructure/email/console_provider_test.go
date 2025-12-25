package email

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================
// NewConsoleProvider Tests
// ===========================

func TestNewConsoleProvider_Success(t *testing.T) {
	config := EmailConfig{
		FromAddress: "test@example.com",
		FromName:    "Test Sender",
	}

	provider, err := NewConsoleProvider(config)

	require.NoError(t, err)
	assert.NotNil(t, provider)
	assert.Equal(t, "Test Sender <test@example.com>", provider.from)
}

func TestNewConsoleProvider_EmptyFromName(t *testing.T) {
	config := EmailConfig{
		FromAddress: "test@example.com",
		FromName:    "",
	}

	provider, err := NewConsoleProvider(config)

	require.NoError(t, err)
	assert.NotNil(t, provider)
	assert.Equal(t, " <test@example.com>", provider.from)
}

// ===========================
// ValidateConfig Tests
// ===========================

func TestConsoleProvider_ValidateConfig(t *testing.T) {
	config := EmailConfig{
		FromAddress: "test@example.com",
		FromName:    "Test",
	}
	provider, _ := NewConsoleProvider(config)

	err := provider.ValidateConfig()

	assert.NoError(t, err) // Console provider has no validation
}

// ===========================
// GetProviderName Tests
// ===========================

func TestConsoleProvider_GetProviderName(t *testing.T) {
	config := EmailConfig{
		FromAddress: "test@example.com",
		FromName:    "Test",
	}
	provider, _ := NewConsoleProvider(config)

	name := provider.GetProviderName()

	assert.Equal(t, "Console", name)
}

// ===========================
// SendEmail Tests
// ===========================

func TestConsoleProvider_SendEmail_Basic(t *testing.T) {
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	config := EmailConfig{
		FromAddress: "sender@example.com",
		FromName:    "Sender",
	}
	provider, _ := NewConsoleProvider(config)

	params := EmailParams{
		From:     "sender@example.com",
		To:       []string{"recipient@example.com"},
		Subject:  "Test Subject",
		TextBody: "Test body content",
	}

	err := provider.SendEmail(context.Background(), params)

	// Restore stdout and read output
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "EMAIL")
	assert.Contains(t, output, "From: sender@example.com")
	assert.Contains(t, output, "To: recipient@example.com")
	assert.Contains(t, output, "Subject: Test Subject")
	assert.Contains(t, output, "Test body content")
}

func TestConsoleProvider_SendEmail_MultipleRecipients(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	config := EmailConfig{
		FromAddress: "sender@example.com",
		FromName:    "Sender",
	}
	provider, _ := NewConsoleProvider(config)

	params := EmailParams{
		From:     "sender@example.com",
		To:       []string{"user1@example.com", "user2@example.com"},
		Subject:  "Test",
		TextBody: "Body",
	}

	err := provider.SendEmail(context.Background(), params)

	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "user1@example.com, user2@example.com")
}

func TestConsoleProvider_SendEmail_WithCC(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	config := EmailConfig{
		FromAddress: "sender@example.com",
		FromName:    "Sender",
	}
	provider, _ := NewConsoleProvider(config)

	params := EmailParams{
		From:     "sender@example.com",
		To:       []string{"recipient@example.com"},
		CC:       []string{"cc1@example.com", "cc2@example.com"},
		Subject:  "Test",
		TextBody: "Body",
	}

	err := provider.SendEmail(context.Background(), params)

	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "CC: cc1@example.com, cc2@example.com")
}

func TestConsoleProvider_SendEmail_WithBCC(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	config := EmailConfig{
		FromAddress: "sender@example.com",
		FromName:    "Sender",
	}
	provider, _ := NewConsoleProvider(config)

	params := EmailParams{
		From:     "sender@example.com",
		To:       []string{"recipient@example.com"},
		BCC:      []string{"bcc@example.com"},
		Subject:  "Test",
		TextBody: "Body",
	}

	err := provider.SendEmail(context.Background(), params)

	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "BCC: bcc@example.com")
}

func TestConsoleProvider_SendEmail_WithReplyTo(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	config := EmailConfig{
		FromAddress: "sender@example.com",
		FromName:    "Sender",
	}
	provider, _ := NewConsoleProvider(config)

	params := EmailParams{
		From:     "sender@example.com",
		To:       []string{"recipient@example.com"},
		ReplyTo:  "replyto@example.com",
		Subject:  "Test",
		TextBody: "Body",
	}

	err := provider.SendEmail(context.Background(), params)

	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "Reply-To: replyto@example.com")
}

func TestConsoleProvider_SendEmail_WithHTMLBody(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	config := EmailConfig{
		FromAddress: "sender@example.com",
		FromName:    "Sender",
	}
	provider, _ := NewConsoleProvider(config)

	params := EmailParams{
		From:     "sender@example.com",
		To:       []string{"recipient@example.com"},
		Subject:  "Test",
		TextBody: "Plain text body",
		HTMLBody: "<html><body><h1>HTML Body</h1></body></html>",
	}

	err := provider.SendEmail(context.Background(), params)

	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	assert.NoError(t, err)
	assert.Contains(t, output, "HTML Body:")
	assert.Contains(t, output, "<html><body><h1>HTML Body</h1></body></html>")
	assert.Contains(t, output, "Text Body:")
	assert.Contains(t, output, "Plain text body")
}

func TestConsoleProvider_SendEmail_ContextCancellation(t *testing.T) {
	// Console provider doesn't use context, but should still work
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	config := EmailConfig{
		FromAddress: "sender@example.com",
		FromName:    "Sender",
	}
	provider, _ := NewConsoleProvider(config)

	params := EmailParams{
		From:     "sender@example.com",
		To:       []string{"recipient@example.com"},
		Subject:  "Test",
		TextBody: "Body",
	}

	// Suppress stdout
	old := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	err := provider.SendEmail(ctx, params)

	w.Close()
	os.Stdout = old

	// Console provider ignores context, so should succeed
	assert.NoError(t, err)
}
