package domain

import (
	"time"

	"github.com/google/uuid"
)

// Organization represents a tenant organization
type Organization struct {
	ID        uuid.UUID              `json:"id"`
	Name      string                 `json:"name"`
	Domain    string                 `json:"domain"`
	PlanType  string                 `json:"-"` // internal use only, not exposed via API
	MaxAgents int                    `json:"maxAgents"`
	MaxUsers  int                    `json:"maxUsers"`
	IsActive  bool                   `json:"isActive"`
	Settings  map[string]interface{} `json:"settings"` // Additional org settings
	CreatedAt time.Time              `json:"createdAt"`
	UpdatedAt time.Time              `json:"updatedAt"`
}

// OrganizationRepository defines the interface for organization persistence
type OrganizationRepository interface {
	Create(org *Organization) error
	GetByID(id uuid.UUID) (*Organization, error)
	GetByDomain(domain string) (*Organization, error)
	Update(org *Organization) error
	Delete(id uuid.UUID) error
}
