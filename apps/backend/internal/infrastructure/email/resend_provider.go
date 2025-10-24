package email

import (
	"context"
	"fmt"
)

// ResendProvider implements email sending via Resend
type ResendProvider struct {
	apiKey string
	from   string
}

// NewResendProvider creates a new Resend email provider
func NewResendProvider(config EmailConfig) (*ResendProvider, error) {
	provider := &ResendProvider{
		apiKey: config.ResendAPIKey,
		from:   fmt.Sprintf("%s <%s>", config.FromName, config.FromAddress),
	}

	if err := provider.ValidateConfig(); err != nil {
		return nil, err
	}

	return provider, nil
}

// ValidateConfig validates the Resend configuration
func (p *ResendProvider) ValidateConfig() error {
	if p.apiKey == "" {
		return fmt.Errorf("Resend API key is required")
	}
	if p.from == "" {
		return fmt.Errorf("From address is required")
	}
	return nil
}

// GetProviderName returns the provider name
func (p *ResendProvider) GetProviderName() string {
	return "Resend"
}

// SendEmail sends an email via Resend
func (p *ResendProvider) SendEmail(ctx context.Context, params EmailParams) error {
	// TODO: Implement Resend integration
	// See: https://resend.com/docs/send-with-go
	//
	// Implementation steps:
	// 1. Import Resend SDK: github.com/resendlabs/resend-go
	// 2. Create client with API key
	// 3. Build SendEmailRequest
	// 4. Call client.Emails.Send()

	return fmt.Errorf("Resend provider not yet implemented - contributions welcome!")
}
