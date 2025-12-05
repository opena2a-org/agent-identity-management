package domain

import (
	"time"

	"github.com/google/uuid"
)

// MCPTrustScoreFactors contains the 8 individual factors contributing to MCP server trust score
// Based on 8-factor weighted algorithm specifically designed for MCP servers
type MCPTrustScoreFactors struct {
	// Factor 1: Attestation Consensus (25% weight) - Multi-agent verification strength
	// Measures: valid_attestations / required_threshold
	// More attestations from diverse agents = higher score
	AttestationConsensus float64 `json:"attestationConsensus"` // 0-1

	// Factor 2: Connection Health (15% weight) - Uptime and response times
	// Measures: successful_pings / total_pings over 30 days
	// Based on attestation health check results
	ConnectionHealth float64 `json:"connectionHealth"` // 0-1

	// Factor 3: Capability Stability (15% weight) - Schema changes, deprecations
	// Measures: unchanged_capabilities / total_capabilities
	// Penalizes frequent capability changes
	CapabilityStability float64 `json:"capabilityStability"` // 0-1

	// Factor 4: Security Posture (15% weight) - TLS, authentication, known vulns
	// Measures: TLS enabled, auth required, no known CVEs
	// Based on URL scheme and security metadata
	SecurityPosture float64 `json:"securityPosture"` // 0-1

	// Factor 5: Organization Compliance (10% weight) - Meets policy requirements
	// Measures: Meets allowlist rules, doesn't match blocklist
	// Evaluates against active security policies
	OrganizationCompliance float64 `json:"organizationCompliance"` // 0-1

	// Factor 6: Age & History (10% weight) - Operating duration without issues
	// Measures: Days since registration with successful operation
	// < 7 days: 0.30, 7-30: 0.50, 30-90: 0.75, 90+: 1.00
	Age float64 `json:"age"` // 0-1

	// Factor 7: Usage Patterns (5% weight) - Normal vs anomalous usage
	// Measures: Connection frequency, attestation patterns
	// Anomalous patterns reduce score
	UsagePatterns float64 `json:"usagePatterns"` // 0-1

	// Factor 8: User Feedback (5% weight) - Admin ratings and notes
	// Measures: Explicit admin feedback and ratings
	// Negative feedback reduces score
	UserFeedback float64 `json:"userFeedback"` // 0-1
}

// MCPTrustScore represents a calculated trust score for an MCP server
type MCPTrustScore struct {
	ID             uuid.UUID             `json:"id"`
	MCPServerID    uuid.UUID             `json:"mcpServerId"`
	Score          float64               `json:"score"`      // 0-1
	Factors        MCPTrustScoreFactors  `json:"factors"`
	Confidence     float64               `json:"confidence"` // 0-1, how confident we are in the score
	LastCalculated time.Time             `json:"lastCalculated"`
	CreatedAt      time.Time             `json:"createdAt"`
}

// MCPTrustScoreRepository defines the interface for MCP trust score persistence
type MCPTrustScoreRepository interface {
	Create(score *MCPTrustScore) error
	GetByMCPServer(mcpServerID uuid.UUID) (*MCPTrustScore, error)
	GetLatest(mcpServerID uuid.UUID) (*MCPTrustScore, error)
	GetHistory(mcpServerID uuid.UUID, limit int) ([]*MCPTrustScore, error)
	GetHistoryAuditTrail(mcpServerID uuid.UUID, limit int) ([]*MCPTrustScoreHistoryEntry, error)
}

// MCPTrustScoreHistoryEntry represents an audit trail entry for MCP trust score changes
type MCPTrustScoreHistoryEntry struct {
	ID             uuid.UUID  `json:"id"`
	MCPServerID    uuid.UUID  `json:"mcpServerId"`
	OrganizationID uuid.UUID  `json:"organizationId"`
	TrustScore     float64    `json:"trustScore"`              // 0-1
	PreviousScore  *float64   `json:"previousScore,omitempty"` // 0-1, nullable
	ChangeReason   string     `json:"reason"`                  // Why the score changed
	ChangedBy      *uuid.UUID `json:"changedBy,omitempty"`     // NULL for automated changes
	RecordedAt     time.Time  `json:"timestamp"`               // When recorded
	CreatedAt      time.Time  `json:"createdAt"`
}

// MCPTrustScoreCalculator defines the interface for MCP trust score calculation
type MCPTrustScoreCalculator interface {
	Calculate(server *MCPServer) (*MCPTrustScore, error)
	CalculateFactors(server *MCPServer) (*MCPTrustScoreFactors, error)
}
