package domain

import (
	"time"

	"github.com/google/uuid"
)

// CapabilityRequestStatus represents the approval status of a capability request
type CapabilityRequestStatus string

const (
	CapabilityRequestStatusPending  CapabilityRequestStatus = "pending"
	CapabilityRequestStatusApproved CapabilityRequestStatus = "approved"
	CapabilityRequestStatusRejected CapabilityRequestStatus = "rejected"
)

// CapabilityRequest represents a request for additional agent capabilities after registration
type CapabilityRequest struct {
	ID             uuid.UUID               `json:"id" db:"id"`
	AgentID        uuid.UUID               `json:"agent_id" db:"agent_id"`
	CapabilityType string                  `json:"capability_type" db:"capability_type"`
	Reason         string                  `json:"reason" db:"reason"`
	Status         CapabilityRequestStatus `json:"status" db:"status"`
	RequestedBy    uuid.UUID               `json:"requested_by" db:"requested_by"`
	ReviewedBy     *uuid.UUID              `json:"reviewed_by,omitempty" db:"reviewed_by"`
	RequestedAt    time.Time               `json:"requested_at" db:"requested_at"`
	ReviewedAt     *time.Time              `json:"reviewed_at,omitempty" db:"reviewed_at"`
	CreatedAt      time.Time               `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time               `json:"updated_at" db:"updated_at"`
}

// CapabilityRequestWithDetails includes agent and user details for API responses
type CapabilityRequestWithDetails struct {
	CapabilityRequest
	AgentName          string  `json:"agent_name" db:"agent_name"`
	AgentDisplayName   string  `json:"agent_display_name" db:"agent_display_name"`
	RequestedByEmail   string  `json:"requested_by_email" db:"requested_by_email"`
	ReviewedByEmail    *string `json:"reviewed_by_email,omitempty" db:"reviewed_by_email"`
}

// CreateCapabilityRequestInput represents input for creating a new capability request
type CreateCapabilityRequestInput struct {
	AgentID        uuid.UUID `json:"agent_id" validate:"required"`
	CapabilityType string    `json:"capability_type" validate:"required"`
	Reason         string    `json:"reason" validate:"required,min=10"`
	RequestedBy    uuid.UUID `json:"-"` // Set from authenticated user context
}

// CapabilityRequestRepository defines the interface for capability request data access
type CapabilityRequestRepository interface {
	Create(req *CapabilityRequest) error
	GetByID(id uuid.UUID) (*CapabilityRequestWithDetails, error)
	List(filter CapabilityRequestFilter) ([]*CapabilityRequestWithDetails, error)
	UpdateStatus(id uuid.UUID, status CapabilityRequestStatus, reviewedBy uuid.UUID) error
	Delete(id uuid.UUID) error
}

// CapabilityRequestFilter defines filtering options for capability request queries
type CapabilityRequestFilter struct {
	Status         *CapabilityRequestStatus
	AgentID        *uuid.UUID
	OrganizationID *uuid.UUID // Filter by organization for multi-tenancy
	Limit          int
	Offset         int
}
