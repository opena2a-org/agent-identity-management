package domain

import "time"

// EmailService defines the interface for sending emails
type EmailService interface {
	// SendEmail sends a plain text or HTML email
	SendEmail(to, subject, body string, isHTML bool) error

	// SendTemplatedEmail sends an email using a predefined template
	SendTemplatedEmail(template EmailTemplate, to string, data interface{}) error

	// SendBulkEmail sends the same email to multiple recipients
	SendBulkEmail(recipients []string, subject, body string, isHTML bool) error

	// ValidateConnection tests the email service connection
	ValidateConnection() error
}

// EmailTemplate represents a predefined email template
type EmailTemplate string

const (
	// User-related templates
	TemplateWelcome       EmailTemplate = "welcome"
	TemplateUserApproved  EmailTemplate = "user_approved"
	TemplateUserRejected  EmailTemplate = "user_rejected"
	TemplatePasswordReset EmailTemplate = "password_reset"

	// Agent-related templates
	TemplateAgentRegistered     EmailTemplate = "agent_registered"
	TemplateAgentVerified       EmailTemplate = "agent_verified"
	TemplateVerificationReminder EmailTemplate = "verification_reminder"
	TemplateVerificationFailed  EmailTemplate = "verification_failed"

	// Alert templates
	TemplateAlertCritical EmailTemplate = "alert_critical"
	TemplateAlertWarning  EmailTemplate = "alert_warning"
	TemplateAlertInfo     EmailTemplate = "alert_info"

	// MCP Server templates
	TemplateMCPServerRegistered EmailTemplate = "mcp_server_registered"
	TemplateMCPServerExpiring   EmailTemplate = "mcp_server_expiring"

	// API Key templates
	TemplateAPIKeyCreated  EmailTemplate = "api_key_created"
	TemplateAPIKeyExpiring EmailTemplate = "api_key_expiring"
	TemplateAPIKeyRevoked  EmailTemplate = "api_key_revoked"
)

// EmailTemplateData contains data for rendering email templates
type EmailTemplateData struct {
	// Common fields
	UserName    string
	UserEmail   string
	DashboardURL string
	SupportEmail string
	Timestamp   time.Time

	// Agent-specific fields
	AgentID        string
	AgentName      string
	AgentType      string
	TrustScore     float64
	VerificationURL string

	// Alert-specific fields
	AlertTitle       string
	AlertDescription string
	AlertSeverity    string
	AlertURL         string

	// MCP Server-specific fields
	MCPServerID   string
	MCPServerName string
	MCPServerURL  string
	PublicKey     string
	ExpiresAt     time.Time

	// API Key-specific fields
	APIKeyID     string
	APIKeyName   string
	APIKeyPrefix string
	CreatedAt    time.Time

	// Custom fields (for extensibility)
	CustomData map[string]interface{}
}

// EmailConfig holds email service configuration
type EmailConfig struct {
	// Provider: "azure" or "smtp"
	Provider string

	// Common configuration
	FromAddress string
	FromName    string

	// Azure Communication Services configuration
	Azure AzureEmailConfig

	// SMTP configuration
	SMTP SMTPConfig

	// Template directory (optional)
	TemplateDir string

	// Rate limiting (emails per minute)
	RateLimitPerMinute int
}

// AzureEmailConfig holds Azure Communication Services configuration
type AzureEmailConfig struct {
	// Connection string from Azure Communication Services
	ConnectionString string

	// Polling configuration for message status
	PollingEnabled  bool
	PollingInterval time.Duration
	PollingTimeout  time.Duration
}

// SMTPConfig holds SMTP server configuration
type SMTPConfig struct {
	Host       string
	Port       int
	Username   string
	Password   string
	TLSEnabled bool

	// Connection pool settings
	MaxConnections int
	IdleTimeout    time.Duration
}

// EmailMetrics tracks email sending metrics
type EmailMetrics struct {
	TotalSent       int64
	TotalFailed     int64
	LastSentAt      time.Time
	LastFailedAt    time.Time
	AverageLatency  time.Duration
	SuccessRate     float64
	FailuresByType  map[string]int64
	SentByTemplate  map[EmailTemplate]int64
}
