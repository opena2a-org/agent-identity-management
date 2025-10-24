package email

import (
	"context"
	"fmt"
	"strings"
)

// ConsoleProvider implements email sending by printing to console (for development)
type ConsoleProvider struct {
	from string
}

// NewConsoleProvider creates a new console email provider
func NewConsoleProvider(config EmailConfig) (*ConsoleProvider, error) {
	return &ConsoleProvider{
		from: fmt.Sprintf("%s <%s>", config.FromName, config.FromAddress),
	}, nil
}

// ValidateConfig validates the console provider configuration
func (p *ConsoleProvider) ValidateConfig() error {
	return nil // No validation needed for console provider
}

// GetProviderName returns the provider name
func (p *ConsoleProvider) GetProviderName() string {
	return "Console"
}

// SendEmail "sends" an email by printing it to console
func (p *ConsoleProvider) SendEmail(ctx context.Context, params EmailParams) error {
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("📧 EMAIL (Console Provider - Development Only)")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("From: %s\n", params.From)
	fmt.Printf("To: %s\n", strings.Join(params.To, ", "))

	if len(params.CC) > 0 {
		fmt.Printf("CC: %s\n", strings.Join(params.CC, ", "))
	}
	if len(params.BCC) > 0 {
		fmt.Printf("BCC: %s\n", strings.Join(params.BCC, ", "))
	}
	if params.ReplyTo != "" {
		fmt.Printf("Reply-To: %s\n", params.ReplyTo)
	}

	fmt.Printf("Subject: %s\n", params.Subject)
	fmt.Println(strings.Repeat("-", 80))

	if params.HTMLBody != "" {
		fmt.Println("HTML Body:")
		fmt.Println(params.HTMLBody)
		fmt.Println(strings.Repeat("-", 80))
	}

	fmt.Println("Text Body:")
	fmt.Println(params.TextBody)
	fmt.Println(strings.Repeat("=", 80) + "\n")

	return nil
}
