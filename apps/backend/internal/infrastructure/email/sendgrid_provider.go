package email

import (
	"context"
	"fmt"
)

// SendGridProvider implements email sending via SendGrid
type SendGridProvider struct {
	apiKey string
	from   string
}

// NewSendGridProvider creates a new SendGrid email provider
func NewSendGridProvider(config EmailConfig) (*SendGridProvider, error) {
	provider := &SendGridProvider{
		apiKey: config.SendGridAPIKey,
		from:   fmt.Sprintf("%s <%s>", config.FromName, config.FromAddress),
	}

	if err := provider.ValidateConfig(); err != nil {
		return nil, err
	}

	return provider, nil
}

// ValidateConfig validates the SendGrid configuration
func (p *SendGridProvider) ValidateConfig() error {
	if p.apiKey == "" {
		return fmt.Errorf("SendGrid API key is required")
	}
	if p.from == "" {
		return fmt.Errorf("From address is required")
	}
	return nil
}

// GetProviderName returns the provider name
func (p *SendGridProvider) GetProviderName() string {
	return "SendGrid"
}

// SendEmail sends an email via SendGrid
func (p *SendGridProvider) SendEmail(ctx context.Context, params EmailParams) error {
	// TODO: Implement SendGrid integration
	// See: https://github.com/sendgrid/sendgrid-go
	//
	// Implementation steps:
	// 1. Import SendGrid SDK: github.com/sendgrid/sendgrid-go
	// 2. Create message with mail.NewV3Mail()
	// 3. Set personalization (to, cc, bcc)
	// 4. Send via sendgrid.NewSendClient(apiKey).Send()

	return fmt.Errorf("SendGrid provider not yet implemented - contributions welcome!")
}
