package email

import (
	"context"
	"fmt"
)

// EmailProvider defines the interface that all email providers must implement
// This allows AIM to support multiple email services (SMTP, Azure, AWS SES, SendGrid, Resend)
type EmailProvider interface {
	// SendEmail sends an email with the given parameters
	SendEmail(ctx context.Context, params EmailParams) error

	// ValidateConfig validates the provider's configuration
	ValidateConfig() error

	// GetProviderName returns the name of the email provider
	GetProviderName() string
}

// EmailParams contains all the parameters needed to send an email
type EmailParams struct {
	To          []string          // Recipient email addresses
	From        string            // Sender email address
	Subject     string            // Email subject line
	TextBody    string            // Plain text version of email
	HTMLBody    string            // HTML version of email (optional but recommended)
	ReplyTo     string            // Reply-to address (optional)
	CC          []string          // CC recipients (optional)
	BCC         []string          // BCC recipients (optional)
	Attachments []EmailAttachment // File attachments (optional)
	Headers     map[string]string // Custom headers (optional)
}

// EmailAttachment represents a file attachment
type EmailAttachment struct {
	Filename    string
	ContentType string
	Data        []byte
}

// ProviderType represents the type of email provider
type ProviderType string

const (
	ProviderSMTP     ProviderType = "smtp"      // Standard SMTP server
	ProviderAzure    ProviderType = "azure"     // Azure Communication Services
	ProviderAWSSES   ProviderType = "aws_ses"   // Amazon SES
	ProviderSendGrid ProviderType = "sendgrid"  // SendGrid
	ProviderResend   ProviderType = "resend"    // Resend
	ProviderConsole  ProviderType = "console"   // Console output (development only)
)

// EmailConfig holds configuration for email providers
type EmailConfig struct {
	Provider ProviderType `json:"provider"`

	// SMTP Configuration
	SMTPHost     string `json:"smtp_host,omitempty"`
	SMTPPort     int    `json:"smtp_port,omitempty"`
	SMTPUser     string `json:"smtp_user,omitempty"`
	SMTPPassword string `json:"smtp_password,omitempty"`
	SMTPTLS      bool   `json:"smtp_tls,omitempty"`

	// Azure Communication Services
	AzureConnectionString string `json:"azure_connection_string,omitempty"`

	// AWS SES
	AWSRegion          string `json:"aws_region,omitempty"`
	AWSAccessKeyID     string `json:"aws_access_key_id,omitempty"`
	AWSSecretAccessKey string `json:"aws_secret_access_key,omitempty"`

	// SendGrid
	SendGridAPIKey string `json:"sendgrid_api_key,omitempty"`

	// Resend
	ResendAPIKey string `json:"resend_api_key,omitempty"`

	// Common Settings
	FromAddress string `json:"from_address"`
	FromName    string `json:"from_name"`
}

// NewEmailProvider creates a new email provider based on the configuration
func NewEmailProvider(config EmailConfig) (EmailProvider, error) {
	switch config.Provider {
	case ProviderSMTP:
		return NewSMTPProvider(config)
	case ProviderAzure:
		return NewAzureEmailProvider(config)
	case ProviderAWSSES:
		return NewAWSSESProvider(config)
	case ProviderSendGrid:
		return NewSendGridProvider(config)
	case ProviderResend:
		return NewResendProvider(config)
	case ProviderConsole:
		return NewConsoleProvider(config)
	default:
		return nil, fmt.Errorf("unsupported email provider: %s", config.Provider)
	}
}
