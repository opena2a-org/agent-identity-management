package domain

import (
	"time"

	"github.com/google/uuid"
)

// WebhookEvent represents the type of event that triggers a webhook
type WebhookEvent string

const (
	// Alert events
	WebhookEventAlertCreated      WebhookEvent = "alert.created"
	WebhookEventAlertAcknowledged WebhookEvent = "alert.acknowledged"

	// Agent events
	WebhookEventAgentCreated   WebhookEvent = "agent.created"
	WebhookEventAgentVerified  WebhookEvent = "agent.verified"
	WebhookEventAgentUpdated   WebhookEvent = "agent.updated"
	WebhookEventAgentSuspended WebhookEvent = "agent.suspended"
	WebhookEventAgentDeleted   WebhookEvent = "agent.deleted"

	// Trust score events
	WebhookEventTrustScoreChanged WebhookEvent = "trust_score.changed"
	WebhookEventTrustScoreLow     WebhookEvent = "trust_score.low"

	// Security events
	WebhookEventSecurityViolation   WebhookEvent = "security.violation"
	WebhookEventComplianceViolation WebhookEvent = "compliance.violation"
	WebhookEventCapabilityDrift     WebhookEvent = "security.capability_drift"
	WebhookEventDataExfiltration    WebhookEvent = "security.data_exfiltration"

	// API key events
	WebhookEventAPIKeyCreated WebhookEvent = "api_key.created"
	WebhookEventAPIKeyRevoked WebhookEvent = "api_key.revoked"
	WebhookEventAPIKeyExpired WebhookEvent = "api_key.expired"
)

// AllWebhookEvents returns all available webhook event types
func AllWebhookEvents() []WebhookEvent {
	return []WebhookEvent{
		WebhookEventAlertCreated,
		WebhookEventAlertAcknowledged,
		WebhookEventAgentCreated,
		WebhookEventAgentVerified,
		WebhookEventAgentUpdated,
		WebhookEventAgentSuspended,
		WebhookEventAgentDeleted,
		WebhookEventTrustScoreChanged,
		WebhookEventTrustScoreLow,
		WebhookEventSecurityViolation,
		WebhookEventComplianceViolation,
		WebhookEventCapabilityDrift,
		WebhookEventDataExfiltration,
		WebhookEventAPIKeyCreated,
		WebhookEventAPIKeyRevoked,
		WebhookEventAPIKeyExpired,
	}
}

// WebhookStatus represents the status of a webhook endpoint
type WebhookStatus string

const (
	WebhookStatusActive   WebhookStatus = "active"
	WebhookStatusPaused   WebhookStatus = "paused"
	WebhookStatusDisabled WebhookStatus = "disabled"
)

// Webhook represents a webhook subscription
type Webhook struct {
	ID             uuid.UUID      `json:"id"`
	OrganizationID uuid.UUID      `json:"organizationId"`
	Name           string         `json:"name"`
	URL            string         `json:"url"`
	Events         []WebhookEvent `json:"events"`
	Secret         string         `json:"-"` // Never expose in JSON responses
	Status         WebhookStatus  `json:"status"`
	Description    string         `json:"description,omitempty"`

	// Delivery settings
	TimeoutSeconds    int `json:"timeoutSeconds"`    // Request timeout (default: 30)
	MaxRetries        int `json:"maxRetries"`        // Max retry attempts (default: 3)
	RetryDelaySeconds int `json:"retryDelaySeconds"` // Initial delay between retries (default: 60)

	// Statistics
	LastTriggered      *time.Time `json:"lastTriggered,omitempty"`
	LastDeliveryStatus string     `json:"lastDeliveryStatus,omitempty"`
	SuccessCount       int64      `json:"successCount"`
	FailureCount       int64      `json:"failureCount"`

	// Legacy field (deprecated, use Status instead)
	IsActive bool `json:"isActive"`

	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	CreatedBy uuid.UUID  `json:"createdBy"`
	DeletedAt *time.Time `json:"-"` // Soft delete
}

// WebhookDeliveryStatus represents the status of a webhook delivery
type WebhookDeliveryStatus string

const (
	DeliveryStatusPending   WebhookDeliveryStatus = "pending"
	DeliveryStatusSuccess   WebhookDeliveryStatus = "success"
	DeliveryStatusFailed    WebhookDeliveryStatus = "failed"
	DeliveryStatusRetrying  WebhookDeliveryStatus = "retrying"
	DeliveryStatusAbandoned WebhookDeliveryStatus = "abandoned" // Max retries exceeded
)

// WebhookDelivery represents a webhook delivery attempt
type WebhookDelivery struct {
	ID             uuid.UUID             `json:"id"`
	WebhookID      uuid.UUID             `json:"webhookId"`
	OrganizationID uuid.UUID             `json:"organizationId"`
	Event          WebhookEvent          `json:"event"`
	Payload        string                `json:"payload"`
	Status         WebhookDeliveryStatus `json:"status"`

	// Delivery details
	AttemptCount   int        `json:"attemptCount"`
	LastAttemptAt  *time.Time `json:"lastAttemptAt,omitempty"`
	NextRetryAt    *time.Time `json:"nextRetryAt,omitempty"`
	StatusCode     int        `json:"statusCode,omitempty"`
	ResponseBody   string     `json:"responseBody,omitempty"`
	ErrorMessage   string     `json:"errorMessage,omitempty"`
	DurationMs     int64      `json:"durationMs,omitempty"`

	// Legacy field
	Success bool `json:"success"`

	CreatedAt   time.Time  `json:"createdAt"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
}

// WebhookPayload represents the standard payload sent to webhooks
type WebhookPayload struct {
	ID             string                 `json:"id"`             // Unique delivery ID
	Event          WebhookEvent           `json:"event"`          // Event type
	Timestamp      time.Time              `json:"timestamp"`      // When the event occurred
	OrganizationID string                 `json:"organizationId"` // Organization context
	Data           map[string]interface{} `json:"data"`           // Event-specific data
}

// WebhookRepository defines the interface for webhook persistence
type WebhookRepository interface {
	// Webhook CRUD
	Create(webhook *Webhook) error
	GetByID(id uuid.UUID) (*Webhook, error)
	GetByOrganization(orgID uuid.UUID, limit, offset int) ([]*Webhook, error)
	CountByOrganization(orgID uuid.UUID) (int, error)
	Update(webhook *Webhook) error
	Delete(id uuid.UUID) error

	// Get webhooks subscribed to a specific event
	GetByEvent(orgID uuid.UUID, event WebhookEvent) ([]*Webhook, error)
	GetActiveByEvent(orgID uuid.UUID, event WebhookEvent) ([]*Webhook, error)

	// Delivery management
	RecordDelivery(delivery *WebhookDelivery) error
	GetDeliveryByID(id uuid.UUID) (*WebhookDelivery, error)
	GetDeliveries(webhookID uuid.UUID, limit, offset int) ([]*WebhookDelivery, error)
	GetPendingDeliveries(limit int) ([]*WebhookDelivery, error)
	GetRetryableDeliveries(limit int) ([]*WebhookDelivery, error)
	UpdateDelivery(delivery *WebhookDelivery) error

	// Statistics
	UpdateWebhookStats(webhookID uuid.UUID, success bool, deliveredAt time.Time) error

	// Cleanup
	DeleteOldDeliveries(olderThan time.Duration) (int64, error)
}
