package domain

import (
	"time"

	"github.com/google/uuid"
)

// MCPServerStatus represents the verification status of an MCP server
type MCPServerStatus string

const (
	MCPServerStatusPending   MCPServerStatus = "pending"
	MCPServerStatusVerified  MCPServerStatus = "verified"
	MCPServerStatusSuspended MCPServerStatus = "suspended"
	MCPServerStatusRevoked   MCPServerStatus = "revoked"
)

// MCPServer represents a Model Context Protocol server
type MCPServer struct {
	ID                uuid.UUID       `json:"id"`
	OrganizationID    uuid.UUID       `json:"organization_id"`
	Name              string          `json:"name"`
	Description       string          `json:"description"`
	URL               string          `json:"url"`
	Version           string          `json:"version"`
	PublicKey         string          `json:"public_key"`
	Status            MCPServerStatus `json:"status"`
	IsVerified        bool            `json:"is_verified"`
	LastVerifiedAt    *time.Time      `json:"last_verified_at"`
	VerificationURL   string          `json:"verification_url"`
	Capabilities      []string        `json:"capabilities"` // e.g., ["tools", "prompts", "resources"]
	TrustScore        float64         `json:"trust_score"`
	VerificationCount  int             `json:"verification_count,omitempty"` // Fetched via JOIN/COUNT
	RegisteredByAgent  *uuid.UUID      `json:"registered_by_agent"`          // Agent that registered this server (nullable)
	CreatedBy         uuid.UUID       `json:"created_by"`                   // User who created this server
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
	// ✅ NEW: Tags applied to this MCP server (populated by join)
	Tags               []Tag           `json:"tags"`
	// ✅ NEW: Agent Attestation fields
	VerificationMethod string          `json:"verification_method"` // agent_attestation, api_key, or manual
	AttestationCount   int             `json:"attestation_count"`   // Number of verified agent attestations
	ConfidenceScore    float64         `json:"confidence_score"`    // Calculated from attestations (0-100)
	LastAttestedAt     *time.Time      `json:"last_attested_at"`    // Most recent attestation timestamp
	// Populated via JOIN queries
	AttestedBy             []string `json:"attested_by,omitempty"`              // Agent names that have attested
	ConnectedAgentsCount   int      `json:"connected_agents_count,omitempty"`   // Number of connected agents
	CapabilitiesCount      int      `json:"capabilities_count,omitempty"`       // Number of capabilities
}

// MCPServerRepository defines the interface for MCP server persistence
type MCPServerRepository interface {
	Create(server *MCPServer) error
	GetByID(id uuid.UUID) (*MCPServer, error)
	GetByOrganization(orgID uuid.UUID) ([]*MCPServer, error)
	GetByURL(url string) (*MCPServer, error)
	Update(server *MCPServer) error
	Delete(id uuid.UUID) error
	List(limit, offset int) ([]*MCPServer, error)
	GetVerificationStatus(id uuid.UUID) (*MCPServerVerificationStatus, error)
}

// MCPServerVerificationStatus represents the verification status details
type MCPServerVerificationStatus struct {
	ServerID       uuid.UUID  `json:"server_id"`
	IsVerified     bool       `json:"is_verified"`
	LastVerifiedAt *time.Time `json:"last_verified_at"`
	TrustScore     float64    `json:"trust_score"`
	PublicKeyCount int        `json:"public_key_count"`
	Status         MCPServerStatus `json:"status"`
}
