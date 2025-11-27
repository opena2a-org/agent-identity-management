package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// VerificationStatus represents the status of a verification
type VerificationStatus string

const (
	VerificationStatusPending  VerificationStatus = "pending"
	VerificationStatusApproved VerificationStatus = "approved"
	VerificationStatusDenied   VerificationStatus = "denied"
)

// Verification represents an agent verification request
type Verification struct {
	ID             uuid.UUID              `json:"id"`
	OrganizationID uuid.UUID              `json:"organizationId"`
	AgentID        uuid.UUID              `json:"agentId"`
	AgentName      string                 `json:"agentName"`
	Action         string                 `json:"action"`
	Status         VerificationStatus     `json:"status"`
	DurationMs     int                    `json:"durationMs"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt      time.Time              `json:"timestamp"` // Mapped to timestamp for frontend compatibility
	UpdatedAt      time.Time              `json:"-"`
}

// VerificationRepository defines the interface for verification data access
type VerificationRepository interface {
	Create(ctx context.Context, verification *Verification) error
	GetByID(ctx context.Context, id uuid.UUID) (*Verification, error)
	GetByOrganization(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]*Verification, int, error)
	GetByAgent(ctx context.Context, agentID uuid.UUID, limit, offset int) ([]*Verification, int, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status VerificationStatus) error
	Delete(ctx context.Context, id uuid.UUID) error
}
