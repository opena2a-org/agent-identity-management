package domain

import (
	"time"

	"github.com/google/uuid"
)

// AgentCapability represents a registered capability for an agent
type AgentCapability struct {
	ID              uuid.UUID              `json:"id"`
	AgentID         uuid.UUID              `json:"agentId"`
	CapabilityType  string                 `json:"capabilityType"`
	CapabilityScope map[string]interface{} `json:"capabilityScope,omitempty"`
	GrantedBy       *uuid.UUID             `json:"grantedBy,omitempty"`
	GrantedAt       time.Time              `json:"grantedAt"`
	RevokedAt       *time.Time             `json:"revokedAt,omitempty"`
	CreatedAt       time.Time              `json:"createdAt"`
	UpdatedAt       time.Time              `json:"updatedAt"`
}

// CapabilityViolation represents an attempt to perform an action outside capability scope
type CapabilityViolation struct {
	ID                     uuid.UUID              `json:"id"`
	AgentID                uuid.UUID              `json:"agentId"`
	AgentName              *string                `json:"agentName,omitempty"`
	AttemptedCapability    string                 `json:"attemptedCapability"`
	RegisteredCapabilities map[string]interface{} `json:"registeredCapabilities,omitempty"`
	Severity               string                 `json:"severity"`
	TrustScoreImpact       int                    `json:"trustScoreImpact"`
	IsBlocked              bool                   `json:"isBlocked"`
	SourceIP               *string                `json:"sourceIp,omitempty"`
	RequestMetadata        map[string]interface{} `json:"requestMetadata,omitempty"`
	CreatedAt              time.Time              `json:"createdAt"`
}

// CapabilityRepository defines the interface for capability data access
type CapabilityRepository interface {
	// Capability CRUD
	CreateCapability(capability *AgentCapability) error
	GetCapabilityByID(id uuid.UUID) (*AgentCapability, error)
	GetCapabilitiesByAgentID(agentID uuid.UUID) ([]*AgentCapability, error)
	GetActiveCapabilitiesByAgentID(agentID uuid.UUID) ([]*AgentCapability, error)
	RevokeCapability(id uuid.UUID, revokedAt time.Time) error
	DeleteCapability(id uuid.UUID) error

	// Violation tracking
	CreateViolation(violation *CapabilityViolation) error
	GetViolationByID(id uuid.UUID) (*CapabilityViolation, error)
	GetViolationsByAgentID(agentID uuid.UUID, limit, offset int) ([]*CapabilityViolation, int, error)
	GetRecentViolations(orgID uuid.UUID, minutes int) ([]*CapabilityViolation, error)
	GetViolationsByOrganization(orgID uuid.UUID, limit, offset int) ([]*CapabilityViolation, int, error)
}

// Standard capability types
const (
	CapabilityFileRead        = "file:read"
	CapabilityFileWrite       = "file:write"
	CapabilityFileDelete      = "file:delete"
	CapabilityNetworkAccess   = "network:access"
	CapabilityAPICall         = "api:call"
	CapabilityDBQuery         = "db:query"
	CapabilityDBWrite         = "db:write"
	CapabilityUserImpersonate = "user:impersonate"
	CapabilityDataExport      = "data:export"
	CapabilitySystemAdmin     = "system:admin"
	CapabilityMCPToolUse      = "mcp:tool_use"
)

// Violation severity levels
const (
	ViolationSeverityLow      = "low"
	ViolationSeverityMedium   = "medium"
	ViolationSeverityHigh     = "high"
	ViolationSeverityCritical = "critical"
)
