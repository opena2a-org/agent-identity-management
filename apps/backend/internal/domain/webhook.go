package domain

import (
	"time"

	"github.com/google/uuid"
)

// WebhookEvent represents the type of event that triggers a webhook
type WebhookEvent string

const (
	WebhookEventAgentCreated      WebhookEvent = "agent.created"
	WebhookEventAgentVerified     WebhookEvent = "agent.verified"
	WebhookEventAgentSuspended    WebhookEvent = "agent.suspended"
	WebhookEventTrustScoreChanged WebhookEvent = "trust_score.changed"
	WebhookEventAlertCreated      WebhookEvent = "alert.created"
	WebhookEventComplianceViolation WebhookEvent = "compliance.violation"
)

// Webhook represents a webhook subscription
type Webhook struct {
	ID             uuid.UUID      `json:"id"`
	OrganizationID uuid.UUID      `json:"organizationId"`
	Name           string         `json:"name"`
	URL            string         `json:"url"`
	Events         []WebhookEvent `json:"events"`
	Secret         string         `json:"secret"` // For webhook signature verification
	IsActive       bool           `json:"isActive"`
	LastTriggered  *time.Time     `json:"lastTriggered"`
	FailureCount   int            `json:"failureCount"`
	CreatedAt      time.Time      `json:"createdAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
	CreatedBy      uuid.UUID      `json:"createdBy"`
}

// WebhookDelivery represents a webhook delivery attempt
type WebhookDelivery struct {
	ID           uuid.UUID    `json:"id"`
	WebhookID    uuid.UUID    `json:"webhookId"`
	Event        WebhookEvent `json:"event"`
	Payload      string       `json:"payload"`
	StatusCode   int          `json:"statusCode"`
	ResponseBody string       `json:"responseBody"`
	Success      bool         `json:"success"`
	AttemptCount int          `json:"attemptCount"`
	CreatedAt    time.Time    `json:"createdAt"`
}

// WebhookRepository defines the interface for webhook persistence
type WebhookRepository interface {
	Create(webhook *Webhook) error
	GetByID(id uuid.UUID) (*Webhook, error)
	GetByOrganization(orgID uuid.UUID) ([]*Webhook, error)
	Update(webhook *Webhook) error
	Delete(id uuid.UUID) error
	RecordDelivery(delivery *WebhookDelivery) error
	GetDeliveries(webhookID uuid.UUID, limit, offset int) ([]*WebhookDelivery, error)
}
