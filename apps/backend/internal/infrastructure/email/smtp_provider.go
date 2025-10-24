package email

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"
)

// SMTPProvider implements email sending via SMTP
type SMTPProvider struct {
	host     string
	port     int
	user     string
	password string
	useTLS   bool
	from     string
}

// NewSMTPProvider creates a new SMTP email provider
func NewSMTPProvider(config EmailConfig) (*SMTPProvider, error) {
	provider := &SMTPProvider{
		host:     config.SMTPHost,
		port:     config.SMTPPort,
		user:     config.SMTPUser,
		password: config.SMTPPassword,
		useTLS:   config.SMTPTLS,
		from:     fmt.Sprintf("%s <%s>", config.FromName, config.FromAddress),
	}

	if err := provider.ValidateConfig(); err != nil {
		return nil, err
	}

	return provider, nil
}

// ValidateConfig validates the SMTP configuration
func (p *SMTPProvider) ValidateConfig() error {
	if p.host == "" {
		return fmt.Errorf("SMTP host is required")
	}
	if p.port == 0 {
		return fmt.Errorf("SMTP port is required")
	}
	if p.from == "" {
		return fmt.Errorf("From address is required")
	}
	return nil
}

// GetProviderName returns the provider name
func (p *SMTPProvider) GetProviderName() string {
	return "SMTP"
}

// SendEmail sends an email via SMTP
func (p *SMTPProvider) SendEmail(ctx context.Context, params EmailParams) error {
	// Build email message
	message := p.buildMessage(params)

	// Connect to SMTP server
	addr := fmt.Sprintf("%s:%d", p.host, p.port)

	// Determine authentication method
	var auth smtp.Auth
	if p.user != "" && p.password != "" {
		auth = smtp.PlainAuth("", p.user, p.password, p.host)
	}

	// Recipients
	to := append(params.To, params.CC...)
	to = append(to, params.BCC...)

	// Send email
	if p.useTLS {
		return p.sendWithTLS(addr, auth, params.From, to, message)
	}
	return smtp.SendMail(addr, auth, params.From, to, []byte(message))
}

// sendWithTLS sends email with TLS encryption
func (p *SMTPProvider) sendWithTLS(addr string, auth smtp.Auth, from string, to []string, message string) error {
	// Create TLS config
	tlsConfig := &tls.Config{
		ServerName: p.host,
	}

	// Connect to server
	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}
	defer conn.Close()

	// Create SMTP client
	client, err := smtp.NewClient(conn, p.host)
	if err != nil {
		return fmt.Errorf("failed to create SMTP client: %w", err)
	}
	defer client.Quit()

	// Authenticate
	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP authentication failed: %w", err)
		}
	}

	// Set sender
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}

	// Set recipients
	for _, recipient := range to {
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("failed to set recipient %s: %w", recipient, err)
		}
	}

	// Send message
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to get data writer: %w", err)
	}

	_, err = writer.Write([]byte(message))
	if err != nil {
		return fmt.Errorf("failed to write message: %w", err)
	}

	return writer.Close()
}

// buildMessage constructs the email message in MIME format
func (p *SMTPProvider) buildMessage(params EmailParams) string {
	var builder strings.Builder

	// Headers
	builder.WriteString(fmt.Sprintf("From: %s\r\n", params.From))
	builder.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(params.To, ", ")))

	if len(params.CC) > 0 {
		builder.WriteString(fmt.Sprintf("Cc: %s\r\n", strings.Join(params.CC, ", ")))
	}

	if params.ReplyTo != "" {
		builder.WriteString(fmt.Sprintf("Reply-To: %s\r\n", params.ReplyTo))
	}

	builder.WriteString(fmt.Sprintf("Subject: %s\r\n", params.Subject))
	builder.WriteString("MIME-Version: 1.0\r\n")

	// Custom headers
	for key, value := range params.Headers {
		builder.WriteString(fmt.Sprintf("%s: %s\r\n", key, value))
	}

	// Multipart message (text + HTML)
	if params.HTMLBody != "" {
		builder.WriteString("Content-Type: multipart/alternative; boundary=\"boundary-aim\"\r\n\r\n")

		// Plain text part
		builder.WriteString("--boundary-aim\r\n")
		builder.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
		builder.WriteString(params.TextBody)
		builder.WriteString("\r\n\r\n")

		// HTML part
		builder.WriteString("--boundary-aim\r\n")
		builder.WriteString("Content-Type: text/html; charset=UTF-8\r\n\r\n")
		builder.WriteString(params.HTMLBody)
		builder.WriteString("\r\n\r\n")

		builder.WriteString("--boundary-aim--\r\n")
	} else {
		// Plain text only
		builder.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
		builder.WriteString(params.TextBody)
		builder.WriteString("\r\n")
	}

	return builder.String()
}
