package domain

import (
	"context"
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
	OrganizationID           uuid.UUID   `json:"organizationId"`
	Name                     string      `json:"name"`
	DisplayName              string      `json:"displayName"`
	Description              string      `json:"description"`
	AgentType                AgentType   `json:"agentType"`
	Status                   AgentStatus `json:"status"`
	Version                  string      `json:"version"`
	PublicKey                *string     `json:"publicKey"`
	EncryptedPrivateKey      *string     `json:"-"` // Stored encrypted, never exposed in API
	KeyAlgorithm             string      `json:"keyAlgorithm"`
	CertificateURL           string      `json:"certificateUrl"`
	RepositoryURL            string      `json:"repositoryUrl"`
	DocumentationURL         string      `json:"documentationUrl"`
	TrustScore               float64     `json:"trustScore"`
	VerifiedAt               *time.Time  `json:"verifiedAt"`
	LastCapabilityCheckAt    *time.Time  `json:"lastCapabilityCheckAt"`
	CapabilityViolationCount int         `json:"capabilityViolationCount"`
	IsCompromised            bool        `json:"isCompromised"`
	// Capability-based access control (simple MVP)
	TalksTo                  []string    `json:"talksTo"` // List of MCP server names/IDs this agent can communicate with
	Capabilities             []string    `json:"capabilities"` // Agent capabilities (e.g., ["file:read", "api:call"])
	// Key rotation support
	KeyCreatedAt             *time.Time  `json:"keyCreatedAt"`
	KeyExpiresAt             *time.Time  `json:"keyExpiresAt"`
	KeyRotationGraceUntil    *time.Time  `json:"keyRotationGraceUntil,omitempty"`
	PreviousPublicKey        *string     `json:"-"` // Not exposed in API, used for grace period verification
	RotationCount            int         `json:"rotationCount"`
	CreatedAt                time.Time   `json:"createdAt"`
	UpdatedAt                time.Time   `json:"updatedAt"`
	CreatedBy                uuid.UUID   `json:"createdBy"`
	// Tags applied to this agent (populated by join)
	Tags                     []Tag       `json:"tags"`
	// Track when agent last performed an action (updated on every verify-action call)
	LastActive               *time.Time  `json:"lastActive"`
}

// AgentRepository defines the interface for agent persistence
type AgentRepository interface {
	Create(agent *Agent) error
	GetByID(id uuid.UUID) (*Agent, error)
	GetByName(orgID uuid.UUID, name string) (*Agent, error)
	GetByOrganization(orgID uuid.UUID) ([]*Agent, error)
	Update(agent *Agent) error
	Delete(id uuid.UUID) error
	List(limit, offset int) ([]*Agent, error)
	UpdateTrustScore(id uuid.UUID, newScore float64) error
	MarkAsCompromised(id uuid.UUID) error
	UpdateLastActive(ctx context.Context, agentID uuid.UUID) error
}
