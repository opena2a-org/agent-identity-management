package email

import (
	"fmt"
	"strings"

	"github.com/opena2a/identity/backend/internal/domain"
)

// ConsoleEmailService implements email sending by printing to console (for development)
type ConsoleEmailService struct {
	fromAddress      string
	fromName         string
	templateRenderer *TemplateRenderer
}

// NewConsoleEmailService creates a new console email service
func NewConsoleEmailService(config domain.EmailConfig) (domain.EmailService, error) {
	// Initialize template renderer (empty string for default template dir)
	templateRenderer, err := NewTemplateRenderer("")
	if err != nil {
		return nil, fmt.Errorf("failed to initialize template renderer: %w", err)
	}

	return &ConsoleEmailService{
		fromAddress:      config.FromAddress,
		fromName:         config.FromName,
		templateRenderer: templateRenderer,
	}, nil
}

// SendEmail sends a plain text or HTML email by printing to console
func (s *ConsoleEmailService) SendEmail(to, subject, body string, isHTML bool) error {
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("📧 EMAIL (Console Provider - Development Only)")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("From: %s <%s>\n", s.fromName, s.fromAddress)
	fmt.Printf("To: %s\n", to)
	fmt.Printf("Subject: %s\n", subject)
	fmt.Println(strings.Repeat("-", 80))

	if isHTML {
		fmt.Println("HTML Body:")
		fmt.Println(body)
	} else {
		fmt.Println("Text Body:")
		fmt.Println(body)
	}

	fmt.Println(strings.Repeat("=", 80) + "\n")
	return nil
}

// SendTemplatedEmail sends an email using a predefined template
func (s *ConsoleEmailService) SendTemplatedEmail(template domain.EmailTemplate, to string, data interface{}) error {
	// Render template (returns subject, body, err)
	subject, body, err := s.templateRenderer.Render(template, data)
	if err != nil {
		return fmt.Errorf("failed to render template: %w", err)
	}

	// Print email to console
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("📧 TEMPLATED EMAIL (Console Provider - Development Only)")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("Template: %s\n", template)
	fmt.Printf("From: %s <%s>\n", s.fromName, s.fromAddress)
	fmt.Printf("To: %s\n", to)
	fmt.Printf("Subject: %s\n", subject)
	fmt.Println(strings.Repeat("-", 80))

	fmt.Println("Body:")
	fmt.Println(body)
	fmt.Println(strings.Repeat("=", 80) + "\n")

	return nil
}

// SendBulkEmail sends the same email to multiple recipients by printing to console
func (s *ConsoleEmailService) SendBulkEmail(recipients []string, subject, body string, isHTML bool) error {
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("📧 BULK EMAIL (Console Provider - Development Only)")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("From: %s <%s>\n", s.fromName, s.fromAddress)
	fmt.Printf("To: %s\n", strings.Join(recipients, ", "))
	fmt.Printf("Subject: %s\n", subject)
	fmt.Println(strings.Repeat("-", 80))

	if isHTML {
		fmt.Println("HTML Body:")
		fmt.Println(body)
	} else {
		fmt.Println("Text Body:")
		fmt.Println(body)
	}

	fmt.Println(strings.Repeat("=", 80) + "\n")
	return nil
}

// ValidateConnection validates the console provider connection (always succeeds)
func (s *ConsoleEmailService) ValidateConnection() error {
	fmt.Println("✅ Console email service initialized (emails will be printed to console)")
	fmt.Printf("   From: %s <%s>\n", s.fromName, s.fromAddress)
	fmt.Println("   ⚠️  This is for development only - no actual emails will be sent!")
	return nil
}
