package domain

import (
	"fmt"
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
	ID               uuid.UUID      `json:"id"`
	AgentID          uuid.UUID      `json:"agentId"`
	MCPServerID      uuid.UUID      `json:"mcpServerId"`
	DetectionID      *uuid.UUID     `json:"detectionId"`
	ConnectionType   ConnectionType `json:"connectionType"`
	FirstConnectedAt time.Time      `json:"firstConnectedAt"`
	LastAttestedAt   *time.Time     `json:"lastAttestedAt"`
	AttestationCount int            `json:"attestationCount"`
	IsActive         bool           `json:"isActive"`
	CreatedAt        time.Time      `json:"createdAt"`
	UpdatedAt        time.Time      `json:"updatedAt"`
}

// AttestationPayload represents the data that an agent attests to about an MCP server
// IMPORTANT: Fields MUST be in alphabetical order by JSON key name to match SDK canonical JSON
// SDK uses Python's json.dumps(sort_keys=True) which produces alphabetically sorted keys
type AttestationPayload struct {
	AgentID              string   `json:"agent_id"`                // 1. agent_id
	CapabilitiesFound    []string `json:"capabilities_found"`      // 2. capabilities_found
	Challenge            string   `json:"challenge,omitempty"`     // 2.5. challenge (server nonce for proof of key possession)
	ConnectionLatencyMs  float64  `json:"connection_latency_ms"`   // 3. connection_latency_ms
	ConnectionSuccessful bool     `json:"connection_successful"`   // 4. connection_successful
	HealthCheckPassed    bool     `json:"health_check_passed"`     // 5. health_check_passed
	MCPName              string   `json:"mcp_name"`                // 6. mcp_name
	MCPURL               string   `json:"mcp_url"`                 // 7. mcp_url
	SDKVersion           string   `json:"sdk_version"`             // 8. sdk_version
	Timestamp            string   `json:"timestamp"`               // 9. timestamp
}

// AttestationChallenge represents a server-generated nonce for proof of private key possession
// This prevents replay attacks and proves the agent holds the private key at attestation time
type AttestationChallenge struct {
	ID          uuid.UUID  `json:"id"`
	Challenge   string     `json:"challenge"`    // Random nonce (32 bytes base64)
	AgentID     uuid.UUID  `json:"agentId"`      // Agent that requested this challenge
	MCPServerID uuid.UUID  `json:"mcpServerId"`  // MCP server being attested
	ExpiresAt   time.Time  `json:"expiresAt"`    // Challenge expires after 5 minutes
	UsedAt      *time.Time `json:"usedAt"`       // Set when challenge is consumed (prevents replay)
	CreatedAt   time.Time  `json:"createdAt"`
}

// ToCanonicalJSON converts attestation payload to canonical JSON for signature verification
// CRITICAL: Must match SDK's canonical JSON format exactly:
// - Sorted keys (alphabetically by JSON key name)
// - No whitespace (compact JSON)
// - Consistent float formatting (Python's json.dumps formats 35.0 as "35.0", not "35")
func (ap *AttestationPayload) ToCanonicalJSON() ([]byte, error) {
	// Build ordered map to match Python's json.dumps(sort_keys=True) exactly
	// We use a custom approach because:
	// 1. Go's json.Marshal uses struct field order, not alphabetical JSON key order
	// 2. Go's json.Marshal formats 35.0 as "35" but Python formats it as "35.0"

	parts := []string{}

	// Fields in alphabetical order by JSON key name
	parts = append(parts, formatJSONField("agent_id", ap.AgentID))
	parts = append(parts, formatJSONArray("capabilities_found", ap.CapabilitiesFound))

	// Challenge is optional - only include if non-empty (matches Python's behavior)
	if ap.Challenge != "" {
		parts = append(parts, formatJSONField("challenge", ap.Challenge))
	}

	// Format float with decimal point to match Python's behavior
	parts = append(parts, formatJSONFloat("connection_latency_ms", ap.ConnectionLatencyMs))
	parts = append(parts, formatJSONBool("connection_successful", ap.ConnectionSuccessful))
	parts = append(parts, formatJSONBool("health_check_passed", ap.HealthCheckPassed))
	parts = append(parts, formatJSONField("mcp_name", ap.MCPName))
	parts = append(parts, formatJSONField("mcp_url", ap.MCPURL))
	parts = append(parts, formatJSONField("sdk_version", ap.SDKVersion))
	parts = append(parts, formatJSONField("timestamp", ap.Timestamp))

	// Build final JSON object
	result := "{" + joinStrings(parts, ",") + "}"
	return []byte(result), nil
}

// Helper functions for canonical JSON formatting

func formatJSONField(key, value string) string {
	// Escape special characters in JSON string
	escaped := escapeJSONString(value)
	return `"` + key + `":"` + escaped + `"`
}

func formatJSONFloat(key string, value float64) string {
	// Format float to match Python's json.dumps behavior:
	// - Integer values like 35.0 are serialized as "35.0" (with decimal point)
	// - Non-integer values like 35.5 are serialized normally
	var formatted string
	if value == float64(int64(value)) {
		// Integer value - add .0 to match Python
		formatted = fmt.Sprintf("%.1f", value)
	} else {
		// Non-integer - use default formatting
		formatted = fmt.Sprintf("%g", value)
	}
	return `"` + key + `":` + formatted
}

func formatJSONBool(key string, value bool) string {
	if value {
		return `"` + key + `":true`
	}
	return `"` + key + `":false`
}

func formatJSONArray(key string, values []string) string {
	if len(values) == 0 {
		return `"` + key + `":[]`
	}

	var parts []string
	for _, v := range values {
		parts = append(parts, `"`+escapeJSONString(v)+`"`)
	}
	return `"` + key + `":[` + joinStrings(parts, ",") + `]`
}

func escapeJSONString(s string) string {
	// Basic JSON string escaping
	var result []byte
	for _, c := range s {
		switch c {
		case '"':
			result = append(result, '\\', '"')
		case '\\':
			result = append(result, '\\', '\\')
		case '\n':
			result = append(result, '\\', 'n')
		case '\r':
			result = append(result, '\\', 'r')
		case '\t':
			result = append(result, '\\', 't')
		default:
			if c < 32 {
				result = append(result, []byte(fmt.Sprintf("\\u%04x", c))...)
			} else {
				result = append(result, []byte(string(c))...)
			}
		}
	}
	return string(result)
}

func joinStrings(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += sep + parts[i]
	}
	return result
}

// MCPAttestation represents a cryptographically signed attestation from a verified agent
type MCPAttestation struct {
	ID                uuid.UUID          `json:"id"`
	MCPServerID       uuid.UUID          `json:"mcpServerId"`
	AgentID           *uuid.UUID         `json:"agentId"`          // Nullable for manual attestations
	AttestationData   AttestationPayload `json:"attestationData"`
	Signature         string             `json:"signature"`
	SignatureVerified bool               `json:"signatureVerified"`
	VerifiedAt        *time.Time         `json:"verifiedAt"`
	ExpiresAt         time.Time          `json:"expiresAt"`
	IsValid           bool               `json:"isValid"`
	CreatedAt         time.Time          `json:"createdAt"`

	// Populated via JOIN queries
	AgentName       string  `json:"agentName,omitempty"`
	AgentTrustScore float64 `json:"agentTrustScore,omitempty"`
}

// AttestationWithAgentDetails is returned from API endpoints that need agent info
type AttestationWithAgentDetails struct {
	ID                    uuid.UUID `json:"id"`
	AgentID               uuid.UUID `json:"agentId"`
	AgentName             string    `json:"agentName"`
	AgentTrustScore       float64   `json:"agentTrustScore"`
	VerifiedAt            string    `json:"verifiedAt"`
	ExpiresAt             string    `json:"expiresAt"`
	CapabilitiesConfirmed []string  `json:"capabilitiesConfirmed"`
	ConnectionLatencyMs   float64   `json:"connectionLatencyMs"`
	HealthCheckPassed     bool      `json:"healthCheckPassed"`
	IsValid               bool      `json:"isValid"`

	// Attestation metadata - who and how
	AttestationType      string    `json:"attestationType"`           // "sdk" or "manual"
	AttestedBy           string    `json:"attestedBy"`                // Agent name or User name
	AttesterType         string    `json:"attesterType"`              // "agent" or "user"
	SignatureVerified    bool      `json:"signatureVerified"`         // Whether cryptographic signature was verified
	SDKVersion           string    `json:"sdkVersion,omitempty"`      // SDK version used (if SDK attestation)
	ConnectionSuccessful bool      `json:"connectionSuccessful"`      // Whether connection test succeeded
	AgentOwnerName       string    `json:"agentOwnerName,omitempty"`  // Name of user who owns the agent (for SDK attestations)
	AgentOwnerID         uuid.UUID `json:"agentOwnerId,omitempty"`    // ID of user who owns the agent (for SDK attestations)
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
	GetAllAttestationsByOrganization(orgID uuid.UUID, limit int) ([]*MCPAttestation, error)
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
