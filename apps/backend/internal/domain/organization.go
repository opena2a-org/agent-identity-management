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
	PlanType  string                 `json:"plan_type"` // free, pro, enterprise
	MaxAgents int                    `json:"max_agents"`
	MaxUsers  int                    `json:"max_users"`
	IsActive  bool                   `json:"is_active"`
	Settings  map[string]interface{} `json:"settings"`   // Additional org settings
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
}

// OrganizationRepository defines the interface for organization persistence
type OrganizationRepository interface {
	Create(org *Organization) error
	GetByID(id uuid.UUID) (*Organization, error)
	GetByDomain(domain string) (*Organization, error)
	Update(org *Organization) error
	Delete(id uuid.UUID) error
}
