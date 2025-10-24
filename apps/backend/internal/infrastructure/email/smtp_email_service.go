package email

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"
	"sync"
	"time"

	"github.com/opena2a/identity/backend/internal/domain"
)

// SMTPEmailService implements email sending using standard SMTP
type SMTPEmailService struct {
	host             string
	port             int
	username         string
	password         string
	fromAddress      string
	fromName         string
	tlsEnabled       bool
	templateRenderer *TemplateRenderer
	metrics          *emailMetrics
	mu               sync.RWMutex
}

// NewSMTPEmailService creates a new SMTP email provider
func NewSMTPEmailService(config domain.EmailConfig) (*SMTPEmailService, error) {
	if config.SMTP.Host == "" {
		return nil, fmt.Errorf("smtp host is required")
	}

	if config.SMTP.Port == 0 {
		return nil, fmt.Errorf("smtp port is required")
	}

	if config.FromAddress == "" {
		return nil, fmt.Errorf("from email address is required")
	}

	templateRenderer, err := NewTemplateRenderer(config.TemplateDir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize template renderer: %w", err)
	}

	return &SMTPEmailService{
		host:             config.SMTP.Host,
		port:             config.SMTP.Port,
		username:         config.SMTP.Username,
		password:         config.SMTP.Password,
		fromAddress:      config.FromAddress,
		fromName:         config.FromName,
		tlsEnabled:       config.SMTP.TLSEnabled,
		templateRenderer: templateRenderer,
		metrics: &emailMetrics{
			failuresByType: make(map[string]int64),
			sentByTemplate: make(map[domain.EmailTemplate]int64),
		},
	}, nil
}

// SendEmail sends a plain text or HTML email via SMTP
func (s *SMTPEmailService) SendEmail(to, subject, body string, isHTML bool) error {
	startTime := time.Now()

	// Build email message
	from := s.fromAddress
	if s.fromName != "" {
		from = fmt.Sprintf("%s <%s>", s.fromName, s.fromAddress)
	}

	// Construct email headers and body
	message := s.buildMessage(from, to, subject, body, isHTML)

	// Connect to SMTP server
	addr := fmt.Sprintf("%s:%d", s.host, s.port)

	// Setup authentication
	var auth smtp.Auth
	if s.username != "" && s.password != "" {
		auth = smtp.PlainAuth("", s.username, s.password, s.host)
	}

	// Send email
	var err error
	if s.tlsEnabled {
		// Use STARTTLS for modern SMTP servers (port 587)
		// This connects in plaintext first, then upgrades to TLS
		client, err := smtp.Dial(addr)
		if err != nil {
			s.recordFailure("smtp_dial_error")
			return fmt.Errorf("failed to dial SMTP server: %w", err)
		}
		defer client.Quit()

		// Upgrade to TLS using STARTTLS
		tlsConfig := &tls.Config{
			ServerName: s.host,
		}
		if err = client.StartTLS(tlsConfig); err != nil {
			s.recordFailure("starttls_error")
			return fmt.Errorf("failed to connect via TLS: %w", err)
		}

		// Authenticate
		if auth != nil {
			if err := client.Auth(auth); err != nil {
				s.recordFailure("auth_error")
				return fmt.Errorf("authentication failed: %w", err)
			}
		}

		// Send the message
		if err := client.Mail(s.fromAddress); err != nil {
			s.recordFailure("mail_from_error")
			return fmt.Errorf("MAIL FROM failed: %w", err)
		}

		if err := client.Rcpt(to); err != nil {
			s.recordFailure("rcpt_to_error")
			return fmt.Errorf("RCPT TO failed: %w", err)
		}

		w, err := client.Data()
		if err != nil {
			s.recordFailure("data_command_error")
			return fmt.Errorf("DATA command failed: %w", err)
		}

		_, err = w.Write([]byte(message))
		if err != nil {
			s.recordFailure("write_error")
			return fmt.Errorf("failed to write message: %w", err)
		}

		err = w.Close()
		if err != nil {
			s.recordFailure("close_error")
			return fmt.Errorf("failed to close writer: %w", err)
		}
	} else {
		// Use plain connection (less secure, mainly for local testing)
		err = smtp.SendMail(addr, auth, s.fromAddress, []string{to}, []byte(message))
		if err != nil {
			s.recordFailure("send_mail_error")
			return fmt.Errorf("failed to send email: %w", err)
		}
	}

	// Update metrics
	s.recordSuccess(time.Since(startTime), "")

	return nil
}

// SendTemplatedEmail sends an email using a predefined template
func (s *SMTPEmailService) SendTemplatedEmail(template domain.EmailTemplate, to string, data interface{}) error {
	// Render the template
	subject, body, err := s.templateRenderer.Render(template, data)
	if err != nil {
		s.recordFailure("template_render_error")
		return fmt.Errorf("failed to render template %s: %w", template, err)
	}

	// Send the email (always HTML for templates)
	if err := s.SendEmail(to, subject, body, true); err != nil {
		s.recordFailure("send_error")
		return err
	}

	// Track template usage
	s.metrics.mu.Lock()
	s.metrics.sentByTemplate[template]++
	s.metrics.mu.Unlock()

	return nil
}

// SendBulkEmail sends the same email to multiple recipients
func (s *SMTPEmailService) SendBulkEmail(recipients []string, subject, body string, isHTML bool) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(recipients))

	// Send emails sequentially to avoid overwhelming SMTP server
	// In production, implement proper rate limiting
	for _, recipient := range recipients {
		wg.Add(1)
		go func(to string) {
			defer wg.Done()
			if err := s.SendEmail(to, subject, body, isHTML); err != nil {
				errChan <- fmt.Errorf("failed to send to %s: %w", to, err)
			}
			// Small delay to avoid rate limiting
			time.Sleep(100 * time.Millisecond)
		}(recipient)
	}

	wg.Wait()
	close(errChan)

	// Collect errors
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to send %d/%d emails: %v", len(errors), len(recipients), errors[0])
	}

	return nil
}

// ValidateConnection tests the SMTP server connection
func (s *SMTPEmailService) ValidateConnection() error {
	addr := fmt.Sprintf("%s:%d", s.host, s.port)

	if s.tlsEnabled {
		// Test STARTTLS connection (for port 587)
		client, err := smtp.Dial(addr)
		if err != nil {
			return fmt.Errorf("failed to connect to SMTP server: %w", err)
		}
		defer client.Quit()

		// Upgrade to TLS using STARTTLS
		tlsConfig := &tls.Config{
			ServerName: s.host,
		}
		if err = client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("failed to start TLS: %w", err)
		}

		// Test authentication if configured
		if s.username != "" && s.password != "" {
			auth := smtp.PlainAuth("", s.username, s.password, s.host)
			if err := client.Auth(auth); err != nil {
				return fmt.Errorf("authentication failed: %w", err)
			}
		}
	} else {
		// Test plain connection
		client, err := smtp.Dial(addr)
		if err != nil {
			return fmt.Errorf("failed to connect to SMTP server: %w", err)
		}
		defer client.Quit()
	}

	return nil
}

// buildMessage constructs a RFC 822-compliant email message
func (s *SMTPEmailService) buildMessage(from, to, subject, body string, isHTML bool) string {
	var builder strings.Builder

	// Headers
	builder.WriteString(fmt.Sprintf("From: %s\r\n", from))
	builder.WriteString(fmt.Sprintf("To: %s\r\n", to))
	builder.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	builder.WriteString(fmt.Sprintf("Date: %s\r\n", time.Now().Format(time.RFC1123Z)))
	builder.WriteString("MIME-Version: 1.0\r\n")

	if isHTML {
		builder.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
	} else {
		builder.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n")
	}

	builder.WriteString("\r\n")

	// Body
	builder.WriteString(body)

	return builder.String()
}

// GetMetrics returns current email sending metrics
func (s *SMTPEmailService) GetMetrics() domain.EmailMetrics {
	s.metrics.mu.RLock()
	defer s.metrics.mu.RUnlock()

	failuresByType := make(map[string]int64)
	for k, v := range s.metrics.failuresByType {
		failuresByType[k] = v
	}

	sentByTemplate := make(map[domain.EmailTemplate]int64)
	for k, v := range s.metrics.sentByTemplate {
		sentByTemplate[k] = v
	}

	var successRate float64
	total := s.metrics.totalSent + s.metrics.totalFailed
	if total > 0 {
		successRate = float64(s.metrics.totalSent) / float64(total) * 100
	}

	return domain.EmailMetrics{
		TotalSent:      s.metrics.totalSent,
		TotalFailed:    s.metrics.totalFailed,
		LastSentAt:     s.metrics.lastSentAt,
		LastFailedAt:   s.metrics.lastFailedAt,
		SuccessRate:    successRate,
		FailuresByType: failuresByType,
		SentByTemplate: sentByTemplate,
	}
}

// recordSuccess updates metrics for successful email send
func (s *SMTPEmailService) recordSuccess(latency time.Duration, template domain.EmailTemplate) {
	s.metrics.mu.Lock()
	defer s.metrics.mu.Unlock()

	s.metrics.totalSent++
	s.metrics.lastSentAt = time.Now()
}

// recordFailure updates metrics for failed email send
func (s *SMTPEmailService) recordFailure(errorType string) {
	s.metrics.mu.Lock()
	defer s.metrics.mu.Unlock()

	s.metrics.totalFailed++
	s.metrics.lastFailedAt = time.Now()
	s.metrics.failuresByType[errorType]++
}
