package email

import (
	"testing"

	"github.com/opena2a/identity/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================
// LoadEmailConfigFromEnv Tests
// ===========================

func TestLoadEmailConfigFromEnv_ConsoleProvider(t *testing.T) {
	t.Setenv("EMAIL_PROVIDER", "console")
	t.Setenv("EMAIL_FROM_ADDRESS", "test@example.com")
	t.Setenv("EMAIL_FROM_NAME", "Test Sender")

	config, err := LoadEmailConfigFromEnv()

	require.NoError(t, err)
	assert.Equal(t, "console", config.Provider)
	assert.Equal(t, "test@example.com", config.FromAddress)
	assert.Equal(t, "Test Sender", config.FromName)
}

func TestLoadEmailConfigFromEnv_AzureProvider(t *testing.T) {
	t.Setenv("EMAIL_PROVIDER", "azure")
	t.Setenv("EMAIL_FROM_ADDRESS", "noreply@example.com")
	t.Setenv("AZURE_EMAIL_CONNECTION_STRING", "endpoint=https://test.communication.azure.com/;accesskey=test123")

	config, err := LoadEmailConfigFromEnv()

	require.NoError(t, err)
	assert.Equal(t, "azure", config.Provider)
	assert.Equal(t, "endpoint=https://test.communication.azure.com/;accesskey=test123", config.Azure.ConnectionString)
}

func TestLoadEmailConfigFromEnv_AzureMissingConnectionString(t *testing.T) {
	t.Setenv("EMAIL_PROVIDER", "azure")
	t.Setenv("EMAIL_FROM_ADDRESS", "noreply@example.com")
	// Don't set AZURE_EMAIL_CONNECTION_STRING

	_, err := LoadEmailConfigFromEnv()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "AZURE_EMAIL_CONNECTION_STRING")
}

func TestLoadEmailConfigFromEnv_SMTPProvider(t *testing.T) {
	t.Setenv("EMAIL_PROVIDER", "smtp")
	t.Setenv("EMAIL_FROM_ADDRESS", "noreply@example.com")
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("SMTP_PORT", "587")
	t.Setenv("SMTP_USERNAME", "user")
	t.Setenv("SMTP_PASSWORD", "pass")

	config, err := LoadEmailConfigFromEnv()

	require.NoError(t, err)
	assert.Equal(t, "smtp", config.Provider)
	assert.Equal(t, "smtp.example.com", config.SMTP.Host)
	assert.Equal(t, 587, config.SMTP.Port)
	assert.Equal(t, "user", config.SMTP.Username)
	assert.Equal(t, "pass", config.SMTP.Password)
}

func TestLoadEmailConfigFromEnv_SMTPMissingHost(t *testing.T) {
	t.Setenv("EMAIL_PROVIDER", "smtp")
	t.Setenv("EMAIL_FROM_ADDRESS", "noreply@example.com")
	t.Setenv("SMTP_PORT", "587")

	_, err := LoadEmailConfigFromEnv()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SMTP_HOST")
}

func TestLoadEmailConfigFromEnv_MissingFromAddress(t *testing.T) {
	t.Setenv("EMAIL_PROVIDER", "console")
	// Don't set EMAIL_FROM_ADDRESS

	_, err := LoadEmailConfigFromEnv()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "EMAIL_FROM_ADDRESS")
}

func TestLoadEmailConfigFromEnv_UnsupportedProvider(t *testing.T) {
	t.Setenv("EMAIL_PROVIDER", "unsupported")
	t.Setenv("EMAIL_FROM_ADDRESS", "test@example.com")

	_, err := LoadEmailConfigFromEnv()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported")
}

func TestLoadEmailConfigFromEnv_DefaultValues(t *testing.T) {
	t.Setenv("EMAIL_FROM_ADDRESS", "test@example.com")
	// EMAIL_PROVIDER defaults to "azure"

	_, err := LoadEmailConfigFromEnv()

	// Should fail because azure connection string is not set
	assert.Error(t, err)
}

func TestLoadEmailConfigFromEnv_RateLimitCustom(t *testing.T) {
	t.Setenv("EMAIL_PROVIDER", "console")
	t.Setenv("EMAIL_FROM_ADDRESS", "test@example.com")
	t.Setenv("EMAIL_RATE_LIMIT_PER_MINUTE", "100")

	config, err := LoadEmailConfigFromEnv()

	require.NoError(t, err)
	assert.Equal(t, 100, config.RateLimitPerMinute)
}

// ===========================
// ValidateEmailConfig Tests
// ===========================

func TestValidateEmailConfig_ValidAzure(t *testing.T) {
	config := domain.EmailConfig{
		Provider:    "azure",
		FromAddress: "test@example.com",
		Azure: domain.AzureEmailConfig{
			ConnectionString: "valid-connection-string",
		},
	}

	err := ValidateEmailConfig(config)

	assert.NoError(t, err)
}

func TestValidateEmailConfig_ValidSMTP(t *testing.T) {
	config := domain.EmailConfig{
		Provider:    "smtp",
		FromAddress: "test@example.com",
		SMTP: domain.SMTPConfig{
			Host: "smtp.example.com",
			Port: 587,
		},
	}

	err := ValidateEmailConfig(config)

	assert.NoError(t, err)
}

func TestValidateEmailConfig_MissingFromAddress(t *testing.T) {
	config := domain.EmailConfig{
		Provider: "azure",
		Azure: domain.AzureEmailConfig{
			ConnectionString: "valid-connection-string",
		},
	}

	err := ValidateEmailConfig(config)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "from address")
}

func TestValidateEmailConfig_MissingProvider(t *testing.T) {
	config := domain.EmailConfig{
		FromAddress: "test@example.com",
	}

	err := ValidateEmailConfig(config)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "provider")
}

func TestValidateEmailConfig_AzureMissingConnectionString(t *testing.T) {
	config := domain.EmailConfig{
		Provider:    "azure",
		FromAddress: "test@example.com",
	}

	err := ValidateEmailConfig(config)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connection string")
}

func TestValidateEmailConfig_SMTPMissingHost(t *testing.T) {
	config := domain.EmailConfig{
		Provider:    "smtp",
		FromAddress: "test@example.com",
		SMTP: domain.SMTPConfig{
			Port: 587,
		},
	}

	err := ValidateEmailConfig(config)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "smtp host")
}

func TestValidateEmailConfig_SMTPMissingPort(t *testing.T) {
	config := domain.EmailConfig{
		Provider:    "smtp",
		FromAddress: "test@example.com",
		SMTP: domain.SMTPConfig{
			Host: "smtp.example.com",
		},
	}

	err := ValidateEmailConfig(config)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "smtp port")
}

func TestValidateEmailConfig_UnsupportedProvider(t *testing.T) {
	config := domain.EmailConfig{
		Provider:    "mailgun",
		FromAddress: "test@example.com",
	}

	err := ValidateEmailConfig(config)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported provider")
}

// ===========================
// NewEmailServiceWithConfig Tests
// ===========================

func TestNewEmailServiceWithConfig_Console(t *testing.T) {
	config := domain.EmailConfig{
		Provider:    "console",
		FromAddress: "test@example.com",
		FromName:    "Test",
	}

	service, err := NewEmailServiceWithConfig(config)

	require.NoError(t, err)
	assert.NotNil(t, service)
}

func TestNewEmailServiceWithConfig_UnsupportedProvider(t *testing.T) {
	config := domain.EmailConfig{
		Provider:    "invalid",
		FromAddress: "test@example.com",
	}

	_, err := NewEmailServiceWithConfig(config)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported")
}

// ===========================
// Helper Function Tests
// ===========================

func TestGetEnv_WithValue(t *testing.T) {
	t.Setenv("TEST_KEY", "test_value")

	result := getEnv("TEST_KEY", "default")

	assert.Equal(t, "test_value", result)
}

func TestGetEnv_WithDefault(t *testing.T) {
	result := getEnv("NONEXISTENT_KEY", "default_value")

	assert.Equal(t, "default_value", result)
}

func TestGetEnvAsInt_WithValidInt(t *testing.T) {
	t.Setenv("INT_KEY", "42")

	result := getEnvAsInt("INT_KEY", 10)

	assert.Equal(t, 42, result)
}

func TestGetEnvAsInt_WithInvalidInt(t *testing.T) {
	t.Setenv("INT_KEY", "not-a-number")

	result := getEnvAsInt("INT_KEY", 10)

	assert.Equal(t, 10, result)
}

func TestGetEnvAsInt_WithDefault(t *testing.T) {
	result := getEnvAsInt("NONEXISTENT_INT_KEY", 99)

	assert.Equal(t, 99, result)
}

func TestGetEnvAsBool_True(t *testing.T) {
	t.Setenv("BOOL_KEY", "true")

	result := getEnvAsBool("BOOL_KEY", false)

	assert.True(t, result)
}

func TestGetEnvAsBool_False(t *testing.T) {
	t.Setenv("BOOL_KEY", "false")

	result := getEnvAsBool("BOOL_KEY", true)

	assert.False(t, result)
}

func TestGetEnvAsBool_InvalidValue(t *testing.T) {
	t.Setenv("BOOL_KEY", "not-a-bool")

	result := getEnvAsBool("BOOL_KEY", true)

	assert.True(t, result) // Returns default
}

func TestGetEnvAsBool_Default(t *testing.T) {
	result := getEnvAsBool("NONEXISTENT_BOOL_KEY", true)

	assert.True(t, result)
}
