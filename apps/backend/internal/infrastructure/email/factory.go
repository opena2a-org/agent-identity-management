package email

import (
	"fmt"
	"os"
	"strconv"

	"github.com/opena2a/identity/backend/internal/domain"
)

// NewEmailService creates an email service based on configuration
// It automatically detects the provider from environment variables
func NewEmailService() (domain.EmailService, error) {
	config, err := LoadEmailConfigFromEnv()
	if err != nil {
		return nil, fmt.Errorf("failed to load email configuration: %w", err)
	}

	return NewEmailServiceWithConfig(config)
}

// NewEmailServiceWithConfig creates an email service with the provided configuration
func NewEmailServiceWithConfig(config domain.EmailConfig) (domain.EmailService, error) {
	switch config.Provider {
	case "console":
		return NewConsoleEmailService(config)
	case "azure":
		return NewAzureEmailService(config)
	case "smtp":
		return NewSMTPEmailService(config)
	default:
		return nil, fmt.Errorf("unsupported email provider: %s (use 'console', 'azure' or 'smtp')", config.Provider)
	}
}

// LoadEmailConfigFromEnv loads email configuration from environment variables
func LoadEmailConfigFromEnv() (domain.EmailConfig, error) {
	provider := getEnv("EMAIL_PROVIDER", "azure")

	config := domain.EmailConfig{
		Provider:     provider,
		FromAddress:  getEnv("EMAIL_FROM_ADDRESS", ""),
		FromName:     getEnv("EMAIL_FROM_NAME", "Agent Identity Management"),
		TemplateDir:  getEnv("EMAIL_TEMPLATES_DIR", ""),
		RateLimitPerMinute: getEnvAsInt("EMAIL_RATE_LIMIT_PER_MINUTE", 60),
	}

	// Validate required fields
	if config.FromAddress == "" {
		return config, fmt.Errorf("EMAIL_FROM_ADDRESS is required")
	}

	// Load provider-specific configuration
	switch provider {
	case "console":
		// No additional configuration needed for console provider
		// This is for development/testing only

	case "azure":
		config.Azure = domain.AzureEmailConfig{
			ConnectionString: getEnv("AZURE_EMAIL_CONNECTION_STRING", ""),
			PollingEnabled:   getEnvAsBool("AZURE_EMAIL_POLLING_ENABLED", false),
		}

		if config.Azure.ConnectionString == "" {
			return config, fmt.Errorf("AZURE_EMAIL_CONNECTION_STRING is required for Azure provider")
		}

	case "smtp":
		config.SMTP = domain.SMTPConfig{
			Host:       getEnv("SMTP_HOST", ""),
			Port:       getEnvAsInt("SMTP_PORT", 587),
			Username:   getEnv("SMTP_USERNAME", ""),
			Password:   getEnv("SMTP_PASSWORD", ""),
			TLSEnabled: getEnvAsBool("SMTP_TLS_ENABLED", true),
			MaxConnections: getEnvAsInt("SMTP_MAX_CONNECTIONS", 10),
		}

		if config.SMTP.Host == "" {
			return config, fmt.Errorf("SMTP_HOST is required for SMTP provider")
		}

		if config.SMTP.Port == 0 {
			return config, fmt.Errorf("SMTP_PORT is required for SMTP provider")
		}

	default:
		return config, fmt.Errorf("unsupported EMAIL_PROVIDER: %s (use 'console', 'azure' or 'smtp')", provider)
	}

	return config, nil
}

// Helper functions for environment variable handling

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}

	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}

	return value
}

func getEnvAsBool(key string, defaultValue bool) bool {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}

	value, err := strconv.ParseBool(valueStr)
	if err != nil {
		return defaultValue
	}

	return value
}

// ValidateEmailConfig validates the email configuration
func ValidateEmailConfig(config domain.EmailConfig) error {
	if config.FromAddress == "" {
		return fmt.Errorf("from address is required")
	}

	if config.Provider == "" {
		return fmt.Errorf("provider is required")
	}

	switch config.Provider {
	case "azure":
		if config.Azure.ConnectionString == "" {
			return fmt.Errorf("azure connection string is required")
		}

	case "smtp":
		if config.SMTP.Host == "" {
			return fmt.Errorf("smtp host is required")
		}

		if config.SMTP.Port == 0 {
			return fmt.Errorf("smtp port is required")
		}

	default:
		return fmt.Errorf("unsupported provider: %s", config.Provider)
	}

	return nil
}
