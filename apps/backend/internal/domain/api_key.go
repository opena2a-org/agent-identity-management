package domain

import (
	"time"

	"github.com/google/uuid"
)

// APIKey represents an API key for agent authentication
type APIKey struct {
	ID             uuid.UUID  `json:"id"`
	OrganizationID uuid.UUID  `json:"organization_id"`
	AgentID        uuid.UUID  `json:"agent_id"`
	AgentName      string     `json:"agent_name,omitempty"` // Fetched via JOIN
	Name           string     `json:"name"`
	KeyHash        string     `json:"key_hash"` // SHA-256 hash
	Prefix         string     `json:"prefix"`   // First 8 chars for identification
	LastUsedAt     *time.Time `json:"last_used_at"`
	ExpiresAt      *time.Time `json:"expires_at"`
	IsActive       bool       `json:"is_active"`
	CreatedAt      time.Time  `json:"created_at"`
	CreatedBy      uuid.UUID  `json:"created_by"`
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
