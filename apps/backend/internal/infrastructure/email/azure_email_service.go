package email

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/opena2a/identity/backend/internal/domain"
)

// AzureEmailService implements email sending using Azure Communication Services
type AzureEmailService struct {
	connectionString string
	fromAddress      string
	fromName         string
	templateRenderer *TemplateRenderer
	metrics          *emailMetrics
	mu               sync.RWMutex
}

// emailMetrics tracks internal metrics
type emailMetrics struct {
	totalSent      int64
	totalFailed    int64
	lastSentAt     time.Time
	lastFailedAt   time.Time
	failuresByType map[string]int64
	sentByTemplate map[domain.EmailTemplate]int64
	mu             sync.RWMutex
}

// NewAzureEmailService creates a new Azure Communication Services email provider
func NewAzureEmailService(config domain.EmailConfig) (*AzureEmailService, error) {
	if config.Azure.ConnectionString == "" {
		return nil, fmt.Errorf("azure email connection string is required")
	}

	if config.FromAddress == "" {
		return nil, fmt.Errorf("from email address is required")
	}

	templateRenderer, err := NewTemplateRenderer(config.TemplateDir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize template renderer: %w", err)
	}

	return &AzureEmailService{
		connectionString: config.Azure.ConnectionString,
		fromAddress:      config.FromAddress,
		fromName:         config.FromName,
		templateRenderer: templateRenderer,
		metrics: &emailMetrics{
			failuresByType: make(map[string]int64),
			sentByTemplate: make(map[domain.EmailTemplate]int64),
		},
	}, nil
}

// SendEmail sends a plain text or HTML email
func (s *AzureEmailService) SendEmail(to, subject, body string, isHTML bool) error {
	_ = context.Background() // Will be used when Azure SDK is implemented
	startTime := time.Now()

	// TODO: Implement actual Azure Communication Services SDK call
	// For now, this is a placeholder that demonstrates the interface
	//
	// The actual implementation will use:
	// - github.com/Azure/azure-sdk-for-go/sdk/communication/azcommunication
	// - Create EmailClient from connection string
	// - Build EmailMessage with sender, recipient, subject, body
	// - Call client.Send() with context and message

	// Simulate email sending (remove this in production)
	time.Sleep(50 * time.Millisecond)

	// For demonstration purposes, we'll log the email details
	fmt.Printf("[Azure Email] Sending email:\n")
	fmt.Printf("  To: %s\n", to)
	fmt.Printf("  Subject: %s\n", subject)
	fmt.Printf("  HTML: %v\n", isHTML)
	fmt.Printf("  Body length: %d chars\n", len(body))

	// Update metrics
	s.recordSuccess(time.Since(startTime), "")

	return nil
}

// SendTemplatedEmail sends an email using a predefined template
func (s *AzureEmailService) SendTemplatedEmail(template domain.EmailTemplate, to string, data interface{}) error {
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
func (s *AzureEmailService) SendBulkEmail(recipients []string, subject, body string, isHTML bool) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(recipients))

	// Send emails concurrently (with rate limiting in production)
	for _, recipient := range recipients {
		wg.Add(1)
		go func(to string) {
			defer wg.Done()
			if err := s.SendEmail(to, subject, body, isHTML); err != nil {
				errChan <- fmt.Errorf("failed to send to %s: %w", to, err)
			}
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

// ValidateConnection tests the Azure Communication Services connection
func (s *AzureEmailService) ValidateConnection() error {
	// TODO: Implement actual connection validation
	// This should attempt to create an EmailClient and verify the connection string

	if s.connectionString == "" {
		return fmt.Errorf("connection string is empty")
	}

	// Validate connection string format
	// Azure Connection String format: endpoint=https://...;accesskey=...
	if len(s.connectionString) < 50 {
		return fmt.Errorf("connection string appears to be invalid (too short)")
	}

	return nil
}

// GetMetrics returns current email sending metrics
func (s *AzureEmailService) GetMetrics() domain.EmailMetrics {
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
func (s *AzureEmailService) recordSuccess(latency time.Duration, template domain.EmailTemplate) {
	s.metrics.mu.Lock()
	defer s.metrics.mu.Unlock()

	s.metrics.totalSent++
	s.metrics.lastSentAt = time.Now()
}

// recordFailure updates metrics for failed email send
func (s *AzureEmailService) recordFailure(errorType string) {
	s.metrics.mu.Lock()
	defer s.metrics.mu.Unlock()

	s.metrics.totalFailed++
	s.metrics.lastFailedAt = time.Now()
	s.metrics.failuresByType[errorType]++
}
