package domain

import (
	"time"

	"github.com/google/uuid"
)

// AgentType represents the type of agent
type AgentType string

const (
	AgentTypeAI  AgentType = "ai_agent"
	AgentTypeMCP AgentType = "mcp_server"
)

// AgentStatus represents the verification status
type AgentStatus string

const (
	AgentStatusPending   AgentStatus = "pending"
	AgentStatusVerified  AgentStatus = "verified"
	AgentStatusSuspended AgentStatus = "suspended"
	AgentStatusRevoked   AgentStatus = "revoked"
)

// Agent represents an AI agent or MCP server
type Agent struct {
	ID                       uuid.UUID   `json:"id"`
	OrganizationID           uuid.UUID   `json:"organization_id"`
	Name                     string      `json:"name"`
	DisplayName              string      `json:"display_name"`
	Description              string      `json:"description"`
	AgentType                AgentType   `json:"agent_type"`
	Status                   AgentStatus `json:"status"`
	Version                  string      `json:"version"`
	PublicKey                *string     `json:"public_key"`
	EncryptedPrivateKey      *string     `json:"-"` // ✅ NEW: Stored encrypted, never exposed in API
	KeyAlgorithm             string      `json:"key_algorithm"`
	CertificateURL           string      `json:"certificate_url"`
	RepositoryURL            string      `json:"repository_url"`
	DocumentationURL         string      `json:"documentation_url"`
	TrustScore               float64     `json:"trust_score"`
	VerifiedAt               *time.Time  `json:"verified_at"`
	LastCapabilityCheckAt    *time.Time  `json:"last_capability_check_at"`
	CapabilityViolationCount int         `json:"capability_violation_count"`
	IsCompromised            bool        `json:"is_compromised"`
	CreatedAt                time.Time   `json:"created_at"`
	UpdatedAt                time.Time   `json:"updated_at"`
	CreatedBy                uuid.UUID   `json:"created_by"`
}

// AgentRepository defines the interface for agent persistence
type AgentRepository interface {
	Create(agent *Agent) error
	GetByID(id uuid.UUID) (*Agent, error)
	GetByOrganization(orgID uuid.UUID) ([]*Agent, error)
	Update(agent *Agent) error
	Delete(id uuid.UUID) error
	List(limit, offset int) ([]*Agent, error)
	UpdateTrustScore(id uuid.UUID, newScore float64) error
	MarkAsCompromised(id uuid.UUID) error
}
