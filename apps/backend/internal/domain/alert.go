package domain

import (
	"time"

	"github.com/google/uuid"
)

// AlertType represents the type of alert
type AlertType string

const (
	AlertCertificateExpiring    AlertType = "certificate_expiring"
	AlertAPIKeyExpiring         AlertType = "api_key_expiring"
	AlertTrustScoreLow          AlertType = "trust_score_low"
	AlertTrustScoreDrop         AlertType = "trust_score_drop" // Significant decrease in trust score
	AlertAgentOffline           AlertType = "agent_offline"
	AlertSecurityBreach         AlertType = "security_breach"
	AlertUnusualActivity        AlertType = "unusual_activity"
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
	ID             uuid.UUID     `json:"id"`
	OrganizationID uuid.UUID     `json:"organizationId"`
	AlertType      AlertType     `json:"alertType"`
	Severity       AlertSeverity `json:"severity"`
	Title          string        `json:"title"`
	Description    string        `json:"description"`
	ResourceType   string        `json:"resourceType"`
	ResourceID     uuid.UUID     `json:"resourceId"`
	IsAcknowledged bool          `json:"isAcknowledged"`
	AcknowledgedBy *uuid.UUID    `json:"acknowledgedBy"`
	AcknowledgedAt *time.Time    `json:"acknowledgedAt"`
	CreatedAt      time.Time     `json:"createdAt"`
}

// AlertRepository defines the interface for alert persistence
type AlertRepository interface {
	Create(alert *Alert) error
	GetByID(id uuid.UUID) (*Alert, error)
	GetByOrganization(orgID uuid.UUID, limit, offset int) ([]*Alert, error)
	GetByOrganizationFiltered(orgID uuid.UUID, status string, limit, offset int) ([]*Alert, error)
	CountByOrganization(orgID uuid.UUID) (int, error)
	CountByOrganizationFiltered(orgID uuid.UUID, status string) (int, error)
	GetUnacknowledged(orgID uuid.UUID) ([]*Alert, error)
	GetByResourceID(resourceID uuid.UUID, limit, offset int) ([]*Alert, error)
	GetUnacknowledgedByResourceID(resourceID uuid.UUID) ([]*Alert, error)
	Acknowledge(id, userID uuid.UUID) error
	BulkAcknowledge(orgID uuid.UUID, userID uuid.UUID) (int, error)
	Delete(id uuid.UUID) error
}
