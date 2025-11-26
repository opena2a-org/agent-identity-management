package domain

import (
	"time"

	"github.com/google/uuid"
)

// APIKey represents an API key for agent authentication
type APIKey struct {
	ID             uuid.UUID  `json:"id"`
	OrganizationID uuid.UUID  `json:"organizationId"`
	AgentID        uuid.UUID  `json:"agentId"`
	AgentName      string     `json:"agentName,omitempty"` // Fetched via JOIN
	Name           string     `json:"name"`
	KeyHash        string     `json:"keyHash"` // SHA-256 hash
	Prefix         string     `json:"prefix"`  // First 8 chars for identification
	LastUsedAt     *time.Time `json:"lastUsedAt"`
	ExpiresAt      *time.Time `json:"expiresAt"`
	IsActive       bool       `json:"isActive"`
	CreatedAt      time.Time  `json:"createdAt"`
	CreatedBy      uuid.UUID  `json:"createdBy"`
}

// APIKeyRepository defines the interface for API key persistence
type APIKeyRepository interface {
	Create(key *APIKey) error
	GetByID(id uuid.UUID) (*APIKey, error)
	GetByHash(hash string) (*APIKey, error)
	GetByAgent(agentID uuid.UUID) ([]*APIKey, error)
	GetByOrganization(orgID uuid.UUID) ([]*APIKey, error)
	Revoke(id uuid.UUID) error
	Delete(id uuid.UUID) error
	UpdateLastUsed(id uuid.UUID) error
}
