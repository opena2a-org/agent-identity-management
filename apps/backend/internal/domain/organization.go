package domain

import (
	"time"

	"github.com/google/uuid"
)

// EnforcementMode represents how the SDK handles verification failures
type EnforcementMode string

const (
	// EnforcementModeStrict blocks execution when verification fails
	EnforcementModeStrict EnforcementMode = "strict"
	// EnforcementModeMonitoring logs failures but allows execution (default)
	EnforcementModeMonitoring EnforcementMode = "monitoring"
)

// Organization represents a tenant organization
type Organization struct {
	ID              uuid.UUID              `json:"id"`
	Name            string                 `json:"name"`
	Domain          string                 `json:"domain"`
	PlanType        string                 `json:"-"` // internal use only, not exposed via API
	MaxAgents       int                    `json:"maxAgents"`
	MaxUsers        int                    `json:"maxUsers"`
	IsActive        bool                   `json:"isActive"`
	EnforcementMode     EnforcementMode        `json:"enforcementMode"` // strict or monitoring
	Settings            map[string]interface{} `json:"settings"`        // Additional org settings
	// Post-Quantum Cryptography (PQC) settings
	DefaultPQCAlgorithm *string                `json:"defaultPqcAlgorithm,omitempty"` // ML-DSA-44, ML-DSA-65, ML-DSA-87
	RequirePQC          bool                   `json:"requirePqc"`                    // Require PQC keys for new agents
	CreatedAt           time.Time              `json:"createdAt"`
	UpdatedAt           time.Time              `json:"updatedAt"`
}

// OrganizationRepository defines the interface for organization persistence
type OrganizationRepository interface {
	Create(org *Organization) error
	GetByID(id uuid.UUID) (*Organization, error)
	GetByDomain(domain string) (*Organization, error)
	ListAll() ([]*Organization, error)
	Update(org *Organization) error
	Delete(id uuid.UUID) error
}
