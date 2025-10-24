package email

import (
	"context"
	"fmt"
)

// AWSSESProvider implements email sending via AWS SES
type AWSSESProvider struct {
	region          string
	accessKeyID     string
	secretAccessKey string
	from            string
}

// NewAWSSESProvider creates a new AWS SES email provider
func NewAWSSESProvider(config EmailConfig) (*AWSSESProvider, error) {
	provider := &AWSSESProvider{
		region:          config.AWSRegion,
		accessKeyID:     config.AWSAccessKeyID,
		secretAccessKey: config.AWSSecretAccessKey,
		from:            fmt.Sprintf("%s <%s>", config.FromName, config.FromAddress),
	}

	if err := provider.ValidateConfig(); err != nil {
		return nil, err
	}

	return provider, nil
}

// ValidateConfig validates the AWS SES configuration
func (p *AWSSESProvider) ValidateConfig() error {
	if p.region == "" {
		return fmt.Errorf("AWS region is required")
	}
	if p.accessKeyID == "" {
		return fmt.Errorf("AWS access key ID is required")
	}
	if p.secretAccessKey == "" {
		return fmt.Errorf("AWS secret access key is required")
	}
	if p.from == "" {
		return fmt.Errorf("From address is required")
	}
	return nil
}

// GetProviderName returns the provider name
func (p *AWSSESProvider) GetProviderName() string {
	return "AWS SES"
}

// SendEmail sends an email via AWS SES
func (p *AWSSESProvider) SendEmail(ctx context.Context, params EmailParams) error {
	// TODO: Implement AWS SES integration
	// See: https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/ses-example-send-email.html
	//
	// Implementation steps:
	// 1. Import AWS SDK: github.com/aws/aws-sdk-go/service/ses
	// 2. Create SES client with credentials
	// 3. Build SendEmailInput with params
	// 4. Call client.SendEmail()

	return fmt.Errorf("AWS SES provider not yet implemented - contributions welcome!")
}
