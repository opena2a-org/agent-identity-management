package domain

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// JSONBMap is a map type that implements sql.Scanner and driver.Valuer for JSONB columns
type JSONBMap map[string]interface{}

// Scan implements sql.Scanner for JSONBMap
func (j *JSONBMap) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to scan JSONBMap: expected []byte, got %T", value)
	}

	if len(bytes) == 0 {
		*j = nil
		return nil
	}

	return json.Unmarshal(bytes, j)
}

// Value implements driver.Valuer for JSONBMap
func (j JSONBMap) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// CapabilityRequestStatus represents the approval status of a capability request
type CapabilityRequestStatus string

const (
	CapabilityRequestStatusPending      CapabilityRequestStatus = "pending"
	CapabilityRequestStatusApproved     CapabilityRequestStatus = "approved"
	CapabilityRequestStatusAutoApproved CapabilityRequestStatus = "auto-approved"
	CapabilityRequestStatusRejected     CapabilityRequestStatus = "rejected"
)

// CapabilityRequest represents a request for additional agent capabilities after registration
type CapabilityRequest struct {
	ID             uuid.UUID               `json:"id" db:"id"`
	AgentID        uuid.UUID               `json:"agentId" db:"agent_id"`
	CapabilityType string                  `json:"capabilityType" db:"capability_type"`
	Reason         string                  `json:"reason" db:"reason"`
	Metadata       JSONBMap                `json:"metadata,omitempty" db:"metadata"`
	Status         CapabilityRequestStatus `json:"status" db:"status"`
	RequestedBy    uuid.UUID               `json:"requestedBy" db:"requested_by"`
	ReviewedBy     *uuid.UUID              `json:"reviewedBy,omitempty" db:"reviewed_by"`
	RequestedAt    time.Time               `json:"requestedAt" db:"requested_at"`
	ReviewedAt     *time.Time              `json:"reviewedAt,omitempty" db:"reviewed_at"`
	CreatedAt      time.Time               `json:"createdAt" db:"created_at"`
	UpdatedAt      time.Time               `json:"updatedAt" db:"updated_at"`
}

// CapabilityRequestWithDetails includes agent and user details for API responses
type CapabilityRequestWithDetails struct {
	CapabilityRequest
	AgentName        string  `json:"agentName" db:"agent_name"`
	AgentDisplayName string  `json:"agentDisplayName" db:"agent_display_name"`
	RequestedByEmail string  `json:"requestedByEmail" db:"requested_by_email"`
	ReviewedByEmail  *string `json:"reviewedByEmail,omitempty" db:"reviewed_by_email"`
}

// CreateCapabilityRequestInput represents input for creating a new capability request
type CreateCapabilityRequestInput struct {
	AgentID        uuid.UUID `json:"agentId" validate:"required"`
	CapabilityType string    `json:"capabilityType" validate:"required"`
	Reason         string    `json:"reason" validate:"required,min=10"`
	Metadata       JSONBMap  `json:"metadata,omitempty"`
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
