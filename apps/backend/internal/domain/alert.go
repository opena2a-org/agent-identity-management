package domain

import (
	"time"

	"github.com/google/uuid"
)

// AlertType represents the type of alert
type AlertType string

const (
	AlertCertificateExpiring  AlertType = "certificate_expiring"
	AlertAPIKeyExpiring       AlertType = "api_key_expiring"
	AlertTrustScoreLow        AlertType = "trust_score_low"
	AlertAgentOffline         AlertType = "agent_offline"
	AlertSecurityBreach       AlertType = "security_breach"
	AlertUnusualActivity      AlertType = "unusual_activity"
	AlertTypeConfigurationDrift AlertType = "configuration_drift"
)

// AlertSeverity represents alert severity level
type AlertSeverity string

const (
	AlertSeverityInfo     AlertSeverity = "info"
	AlertSeverityWarning  AlertSeverity = "warning"
	AlertSeverityHigh     AlertSeverity = "high"
	AlertSeverityCritical AlertSeverity = "critical"

	// Legacy constants for backward compatibility
	SeverityInfo     AlertSeverity = "info"
	SeverityWarning  AlertSeverity = "warning"
	SeverityCritical AlertSeverity = "critical"
)

// Alert represents a security or operational alert
type Alert struct {
	ID              uuid.UUID     `json:"id"`
	OrganizationID  uuid.UUID     `json:"organization_id"`
	AlertType       AlertType     `json:"alert_type"`
	Severity        AlertSeverity `json:"severity"`
	Title           string        `json:"title"`
	Description     string        `json:"description"`
	ResourceType    string        `json:"resource_type"`
	ResourceID      uuid.UUID     `json:"resource_id"`
	IsAcknowledged  bool          `json:"is_acknowledged"`
	AcknowledgedBy  *uuid.UUID    `json:"acknowledged_by"`
	AcknowledgedAt  *time.Time    `json:"acknowledged_at"`
	CreatedAt       time.Time     `json:"created_at"`
}

// AlertRepository defines the interface for alert persistence
type AlertRepository interface {
	Create(alert *Alert) error
	GetByID(id uuid.UUID) (*Alert, error)
	GetByOrganization(orgID uuid.UUID, limit, offset int) ([]*Alert, error)
	GetUnacknowledged(orgID uuid.UUID) ([]*Alert, error)
	Acknowledge(id, userID uuid.UUID) error
	Delete(id uuid.UUID) error
}
