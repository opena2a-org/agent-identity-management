package domain

import (
	"time"

	"github.com/google/uuid"
)

// VerificationProtocol represents the protocol used for verification
type VerificationProtocol string

const (
	VerificationProtocolMCP   VerificationProtocol = "MCP"
	VerificationProtocolA2A   VerificationProtocol = "A2A"
	VerificationProtocolACP   VerificationProtocol = "ACP"
	VerificationProtocolDID   VerificationProtocol = "DID"
	VerificationProtocolOAuth VerificationProtocol = "OAuth"
	VerificationProtocolSAML  VerificationProtocol = "SAML"
)

// VerificationType represents the type of verification
type VerificationType string

const (
	VerificationTypeIdentity   VerificationType = "identity"
	VerificationTypeCapability VerificationType = "capability"
	VerificationTypePermission VerificationType = "permission"
	VerificationTypeTrust      VerificationType = "trust"
)

// VerificationEventStatus represents the status of a verification event
type VerificationEventStatus string

const (
	VerificationEventStatusSuccess VerificationEventStatus = "success"
	VerificationEventStatusFailed  VerificationEventStatus = "failed"
	VerificationEventStatusPending VerificationEventStatus = "pending"
	VerificationEventStatusTimeout VerificationEventStatus = "timeout"
)

// VerificationResult represents the result of a verification
type VerificationResult string

const (
	VerificationResultVerified VerificationResult = "verified"
	VerificationResultDenied   VerificationResult = "denied"
	VerificationResultExpired  VerificationResult = "expired"
)

// InitiatorType represents who initiated the verification
type InitiatorType string

const (
	InitiatorTypeUser      InitiatorType = "user"
	InitiatorTypeAgent     InitiatorType = "agent"
	InitiatorTypeSystem    InitiatorType = "system"
	InitiatorTypeScheduler InitiatorType = "scheduler"
)

// VerificationEvent represents a real-time verification event for monitoring
type VerificationEvent struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID uuid.UUID `json:"organizationId"`

	// Target can be either an Agent or MCP Server (one must be set, not both)
	AgentID       *uuid.UUID `json:"agentId,omitempty"`
	AgentName     *string    `json:"agentName,omitempty"`
	MCPServerID   *uuid.UUID `json:"mcpServerId,omitempty"`
	MCPServerName *string    `json:"mcpServerName,omitempty"`

	// Verification details
	Protocol         VerificationProtocol    `json:"protocol"`
	VerificationType VerificationType        `json:"verificationType"`
	Status           VerificationEventStatus `json:"status"`
	Result           *VerificationResult     `json:"result,omitempty"`

	// Cryptographic proof
	Signature   *string `json:"signature,omitempty"`
	MessageHash *string `json:"messageHash,omitempty"`
	Nonce       *string `json:"nonce,omitempty"`
	PublicKey   *string `json:"publicKey,omitempty"`

	// Metrics
	Confidence float64 `json:"confidence"`
	TrustScore float64 `json:"trustScore"`
	DurationMs int     `json:"durationMs"`

	// Error handling
	ErrorCode   *string `json:"errorCode,omitempty"`
	ErrorReason *string `json:"errorReason,omitempty"`

	// Initiator information
	InitiatorType InitiatorType `json:"initiatorType"`
	InitiatorID   *uuid.UUID    `json:"initiatorId,omitempty"`
	InitiatorName *string       `json:"initiatorName,omitempty"`
	InitiatorIP   *string       `json:"initiatorIp,omitempty"`

	// Context
	Action       *string `json:"action,omitempty"`
	ResourceType *string `json:"resourceType,omitempty"`
	ResourceID   *string `json:"resourceId,omitempty"`
	Location     *string `json:"location,omitempty"`

	// Configuration Drift Detection (WHO and WHAT)
	CurrentMCPServers    []string `json:"currentMcpServers,omitempty"`    // Runtime: MCP servers being communicated with
	CurrentCapabilities  []string `json:"currentCapabilities,omitempty"`  // Runtime: Capabilities being used
	DriftDetected        bool     `json:"driftDetected"`                  // Whether configuration drift was detected
	MCPServerDrift       []string `json:"mcpServerDrift,omitempty"`       // Unregistered MCP servers detected
	CapabilityDrift      []string `json:"capabilityDrift,omitempty"`      // Undeclared capabilities detected

	// Timestamps
	StartedAt   time.Time  `json:"startedAt"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`

	// Additional data
	Details  *string                `json:"details,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// VerificationEventRepository defines the interface for verification event storage
type VerificationEventRepository interface {
	Create(event *VerificationEvent) error
	GetByID(id uuid.UUID) (*VerificationEvent, error)
	GetByOrganization(orgID uuid.UUID, limit, offset int) ([]*VerificationEvent, int, error)
	GetByAgent(agentID uuid.UUID, limit, offset int) ([]*VerificationEvent, int, error)
	GetByMCPServer(mcpServerID uuid.UUID, limit, offset int) ([]*VerificationEvent, int, error)
	GetRecentEvents(orgID uuid.UUID, minutes int) ([]*VerificationEvent, error)
	GetPendingVerifications(orgID uuid.UUID) ([]*VerificationEvent, error)
	GetStatistics(orgID uuid.UUID, startTime, endTime time.Time) (*VerificationStatistics, error)
	GetAgentStatistics(agentID uuid.UUID, startTime, endTime time.Time) (*AgentVerificationStatistics, error)
	UpdateResult(id uuid.UUID, result VerificationResult, reason *string, metadata map[string]interface{}) error
	Delete(id uuid.UUID) error
}

// VerificationStatistics represents aggregated verification metrics
type VerificationStatistics struct {
	TotalVerifications      int            `json:"totalVerifications"`
	SuccessCount            int            `json:"successCount"`
	FailedCount             int            `json:"failedCount"`
	PendingCount            int            `json:"pendingCount"`
	TimeoutCount            int            `json:"timeoutCount"`
	SuccessRate             float64        `json:"successRate"`
	AvgDurationMs           float64        `json:"avgDurationMs"`
	AvgConfidence           float64        `json:"avgConfidence"`
	AvgTrustScore           float64        `json:"avgTrustScore"`
	VerificationsPerMinute  float64        `json:"verificationsPerMinute"`
	UniqueAgentsVerified    int            `json:"uniqueAgentsVerified"`
	ProtocolDistribution    map[string]int `json:"protocolDistribution"`
	TypeDistribution        map[string]int `json:"typeDistribution"`
	InitiatorDistribution   map[string]int `json:"initiatorDistribution"`
}

// AgentVerificationStatistics represents per-agent verification metrics for trust scoring
type AgentVerificationStatistics struct {
	AgentID            uuid.UUID `json:"agentId"`
	TotalVerifications int       `json:"totalVerifications"`
	SuccessCount       int       `json:"successCount"`
	FailedCount        int       `json:"failedCount"`
	SuccessRate        float64   `json:"successRate"` // 0.0-1.0
	AvgDurationMs      float64   `json:"avgDurationMs"`
	AvgConfidence      float64   `json:"avgConfidence"`
	LastVerification   time.Time `json:"lastVerification"`
}
