package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ConnectionType represents how an agent-MCP connection was established
type ConnectionType string

const (
	ConnectionTypeAutoDetected  ConnectionType = "auto_detected"
	ConnectionTypeUserRegistered ConnectionType = "user_registered"
	ConnectionTypeAttested       ConnectionType = "attested"
)

// AgentMCPConnection represents a bidirectional relationship between an agent and MCP server
type AgentMCPConnection struct {
	ID                uuid.UUID      `json:"id"`
	AgentID           uuid.UUID      `json:"agent_id"`
	MCPServerID       uuid.UUID      `json:"mcp_server_id"`
	DetectionID       *uuid.UUID     `json:"detection_id"`
	ConnectionType    ConnectionType `json:"connection_type"`
	FirstConnectedAt  time.Time      `json:"first_connected_at"`
	LastAttestedAt    *time.Time     `json:"last_attested_at"`
	AttestationCount  int            `json:"attestation_count"`
	IsActive          bool           `json:"is_active"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
}

// AttestationPayload represents the data that an agent attests to about an MCP server
type AttestationPayload struct {
	AgentID              string   `json:"agent_id"`
	MCPURL               string   `json:"mcp_url"`
	MCPName              string   `json:"mcp_name"`
	CapabilitiesFound    []string `json:"capabilities_found"`
	ConnectionSuccessful bool     `json:"connection_successful"`
	HealthCheckPassed    bool     `json:"health_check_passed"`
	ConnectionLatencyMs  float64  `json:"connection_latency_ms"`
	Timestamp            string   `json:"timestamp"`
	SDKVersion           string   `json:"sdk_version"`
}

// ToCanonicalJSON converts attestation payload to canonical JSON for signature verification
func (ap *AttestationPayload) ToCanonicalJSON() ([]byte, error) {
	// Use json.Marshal with sorted keys for consistent signature
	return json.Marshal(ap)
}

// MCPAttestation represents a cryptographically signed attestation from a verified agent
type MCPAttestation struct {
	ID                uuid.UUID          `json:"id"`
	MCPServerID       uuid.UUID          `json:"mcp_server_id"`
	AgentID           uuid.UUID          `json:"agent_id"`
	AttestationData   AttestationPayload `json:"attestation_data"`
	Signature         string             `json:"signature"`
	SignatureVerified bool               `json:"signature_verified"`
	VerifiedAt        *time.Time         `json:"verified_at"`
	ExpiresAt         time.Time          `json:"expires_at"`
	IsValid           bool               `json:"is_valid"`
	CreatedAt         time.Time          `json:"created_at"`

	// Populated via JOIN queries
	AgentName       string  `json:"agent_name,omitempty"`
	AgentTrustScore float64 `json:"agent_trust_score,omitempty"`
}

// AttestationWithAgentDetails is returned from API endpoints that need agent info
type AttestationWithAgentDetails struct {
	ID                    uuid.UUID `json:"id"`
	AgentID               uuid.UUID `json:"agent_id"`
	AgentName             string    `json:"agent_name"`
	AgentTrustScore       float64   `json:"agent_trust_score"`
	VerifiedAt            string    `json:"verified_at"`
	ExpiresAt             string    `json:"expires_at"`
	CapabilitiesConfirmed []string  `json:"capabilities_confirmed"`
	ConnectionLatencyMs   float64   `json:"connection_latency_ms"`
	HealthCheckPassed     bool      `json:"health_check_passed"`
	IsValid               bool      `json:"is_valid"`
}

// VerificationMethod represents how an MCP server was verified
type VerificationMethod string

const (
	VerificationMethodAgentAttestation VerificationMethod = "agent_attestation"
	VerificationMethodAPIKey           VerificationMethod = "api_key"
	VerificationMethodManual           VerificationMethod = "manual"
)

// MCPAttestationRepository defines the interface for attestation persistence
type MCPAttestationRepository interface {
	// Attestation operations
	CreateAttestation(attestation *MCPAttestation) error
	GetAttestationByID(id uuid.UUID) (*MCPAttestation, error)
	GetAttestationsByMCP(mcpServerID uuid.UUID) ([]*MCPAttestation, error)
	GetValidAttestationsByMCP(mcpServerID uuid.UUID) ([]*MCPAttestation, error)
	GetAttestationsByAgent(agentID uuid.UUID) ([]*MCPAttestation, error)
	InvalidateAttestation(id uuid.UUID) error
	InvalidateExpiredAttestations() error // Background job

	// Connection operations
	CreateConnection(connection *AgentMCPConnection) error
	GetConnectionByID(id uuid.UUID) (*AgentMCPConnection, error)
	GetConnectionByAgentAndMCP(agentID, mcpServerID uuid.UUID) (*AgentMCPConnection, error)
	GetConnectionsByAgent(agentID uuid.UUID) ([]*AgentMCPConnection, error)
	GetConnectionsByMCP(mcpServerID uuid.UUID) ([]*AgentMCPConnection, error)
	UpdateConnection(connection *AgentMCPConnection) error
	DeleteConnection(id uuid.UUID) error

	// Confidence score operations
	UpdateMCPConfidenceScore(mcpServerID uuid.UUID, score float64, attestationCount int, lastAttestedAt time.Time) error
}
