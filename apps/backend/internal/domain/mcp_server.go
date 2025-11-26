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
	ID                   uuid.UUID       `json:"id"`
	OrganizationID       uuid.UUID       `json:"organizationId"`
	Name                 string          `json:"name"`
	Description          string          `json:"description"`
	URL                  string          `json:"url"`
	Version              string          `json:"version"`
	PublicKey            string          `json:"publicKey"`
	Status               MCPServerStatus `json:"status"`
	IsVerified           bool            `json:"isVerified"`
	LastVerifiedAt       *time.Time      `json:"lastVerifiedAt"`
	VerificationURL      string          `json:"verificationUrl"`
	Capabilities         []string        `json:"capabilities"` // e.g., ["tools", "prompts", "resources"]
	TrustScore           float64         `json:"trustScore"`
	VerificationCount    int             `json:"verificationCount,omitempty"` // Fetched via JOIN/COUNT
	RegisteredByAgent    *uuid.UUID      `json:"registeredByAgent"`           // Agent that registered this server (nullable)
	CreatedBy            uuid.UUID       `json:"createdBy"`                   // User who created this server
	CreatedAt            time.Time       `json:"createdAt"`
	UpdatedAt            time.Time       `json:"updatedAt"`
	// ✅ NEW: Tags applied to this MCP server (populated by join)
	Tags []Tag `json:"tags"`
	// ✅ NEW: Agent Attestation fields
	VerificationMethod   string     `json:"verificationMethod"` // agent_attestation, api_key, or manual
	AttestationCount     int        `json:"attestationCount"`   // Number of verified agent attestations
	ConfidenceScore      float64    `json:"confidenceScore"`    // Calculated from attestations (0-100)
	LastAttestedAt       *time.Time `json:"lastAttestedAt"`     // Most recent attestation timestamp
	// Populated via JOIN queries
	AttestedBy           []string `json:"attestedBy,omitempty"`           // Agent names that have attested
	ConnectedAgentsCount int      `json:"connectedAgentsCount,omitempty"` // Number of connected agents
	CapabilitiesCount    int      `json:"capabilitiesCount,omitempty"`    // Number of capabilities
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
	ServerID       uuid.UUID       `json:"serverId"`
	IsVerified     bool            `json:"isVerified"`
	LastVerifiedAt *time.Time      `json:"lastVerifiedAt"`
	TrustScore     float64         `json:"trustScore"`
	PublicKeyCount int             `json:"publicKeyCount"`
	Status         MCPServerStatus `json:"status"`
}
