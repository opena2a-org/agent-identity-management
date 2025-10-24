package email

import (
	"context"
	"fmt"
)

// AzureEmailProvider implements email sending via Azure Communication Services
type AzureEmailProvider struct {
	connectionString string
	from             string
}

// NewAzureEmailProvider creates a new Azure email provider
func NewAzureEmailProvider(config EmailConfig) (*AzureEmailProvider, error) {
	provider := &AzureEmailProvider{
		connectionString: config.AzureConnectionString,
		from:             fmt.Sprintf("%s <%s>", config.FromName, config.FromAddress),
	}

	if err := provider.ValidateConfig(); err != nil {
		return nil, err
	}

	return provider, nil
}

// ValidateConfig validates the Azure configuration
func (p *AzureEmailProvider) ValidateConfig() error {
	if p.connectionString == "" {
		return fmt.Errorf("Azure connection string is required")
	}
	if p.from == "" {
		return fmt.Errorf("From address is required")
	}
	return nil
}

// GetProviderName returns the provider name
func (p *AzureEmailProvider) GetProviderName() string {
	return "Azure Communication Services"
}

// SendEmail sends an email via Azure Communication Services
func (p *AzureEmailProvider) SendEmail(ctx context.Context, params EmailParams) error {
	// TODO: Implement Azure Communication Services integration
	// See: https://learn.microsoft.com/en-us/azure/communication-services/quickstarts/email/send-email
	//
	// Implementation steps:
	// 1. Import Azure SDK: github.com/Azure/azure-sdk-for-go/sdk/communication/azemail
	// 2. Create email client with connection string
	// 3. Build email message
	// 4. Send via client.Send()

	return fmt.Errorf("Azure Communication Services provider not yet implemented - contributions welcome!")
}
