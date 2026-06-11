package domain

import (
	"time"

	"github.com/google/uuid"
)

// Execution mode constants for per-capability enforcement
const (
	ExecutionModeAuto   = "auto"   // Execute immediately (default)
	ExecutionModeNotify = "notify" // Execute + create alert
	ExecutionModeReview = "review" // Queue for human approval before execution
)

// AgentCapability represents a registered capability for an agent
type AgentCapability struct {
	ID              uuid.UUID              `json:"id"`
	AgentID         uuid.UUID              `json:"agentId"`
	CapabilityType  string                 `json:"capabilityType"`
	CapabilityScope map[string]interface{} `json:"capabilityScope,omitempty"`
	ExecutionMode   string                 `json:"executionMode"`
	GrantedBy       *uuid.UUID             `json:"grantedBy,omitempty"`
	GrantedAt       time.Time              `json:"grantedAt"`
	RevokedAt       *time.Time             `json:"revokedAt,omitempty"`
	CreatedAt       time.Time              `json:"createdAt"`
	UpdatedAt       time.Time              `json:"updatedAt"`
}

// CapabilityViolation represents an attempt to perform an action outside capability scope
type CapabilityViolation struct {
	ID                     uuid.UUID              `json:"id"`
	AgentID                uuid.UUID              `json:"agent_id"`
	AgentName              *string                `json:"agent_name,omitempty"`
	AttemptedCapability    string                 `json:"attempted_capability"`
	RegisteredCapabilities map[string]interface{} `json:"registered_capabilities,omitempty"`
	Severity               string                 `json:"severity"`
	TrustScoreImpact       int                    `json:"trust_score_impact"`
	IsBlocked              bool                   `json:"is_blocked"`
	SourceIP               *string                `json:"source_ip,omitempty"`
	RequestMetadata        map[string]interface{} `json:"request_metadata,omitempty"`
	CreatedAt              time.Time              `json:"created_at"`
}

// CapabilityRepository defines the interface for capability data access
type CapabilityRepository interface {
	// Capability CRUD
	CreateCapability(capability *AgentCapability) error
	GetCapabilityByID(id uuid.UUID) (*AgentCapability, error)
	GetCapabilitiesByAgentID(agentID uuid.UUID) ([]*AgentCapability, error)
	GetActiveCapabilitiesByAgentID(agentID uuid.UUID) ([]*AgentCapability, error)
	GetCapabilitiesByAgentIDs(agentIDs []uuid.UUID, activeOnly bool) (map[uuid.UUID][]*AgentCapability, error)
	RevokeCapability(id uuid.UUID, revokedAt time.Time) error
	DeleteCapability(id uuid.UUID) error

	// Violation tracking
	CreateViolation(violation *CapabilityViolation) error
	GetViolationByID(id uuid.UUID) (*CapabilityViolation, error)
	GetViolationsByAgentID(agentID uuid.UUID, limit, offset int) ([]*CapabilityViolation, int, error)
	GetRecentViolations(orgID uuid.UUID, minutes int) ([]*CapabilityViolation, error)
	GetViolationsByOrganization(orgID uuid.UUID, limit, offset int) ([]*CapabilityViolation, int, error)

	// Capability Definitions (registry)
	ListCapabilityDefinitions(orgID *uuid.UUID) ([]*CapabilityDefinition, error)
	GetCapabilityDefinition(namespace, action string, orgID *uuid.UUID) (*CapabilityDefinition, error)
	CreateCapabilityDefinition(def *CapabilityDefinition) error
	UpdateCapabilityDefinition(def *CapabilityDefinition) error
}

// CapabilityDefinition represents a capability type in the registry
type CapabilityDefinition struct {
	ID             uuid.UUID  `json:"id"`
	Namespace      string     `json:"namespace"`
	Action         string     `json:"action"`
	CapabilityType string     `json:"capabilityType"` // "core" or "custom"
	RiskLevel      string     `json:"riskLevel"`      // "low", "medium", "high", "critical"
	MinTrustScore  float64    `json:"minTrustScore"`  // Minimum trust score (0.0-1.0) required
	DisplayName    string     `json:"displayName"`
	Description    string     `json:"description,omitempty"`
	Category       string     `json:"category,omitempty"`
	OrganizationID *uuid.UUID `json:"organizationId,omitempty"` // NULL for core capabilities
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

// DefaultMinTrustScoreForRiskLevel returns the default minimum trust score for a risk level
func DefaultMinTrustScoreForRiskLevel(riskLevel string) float64 {
	switch riskLevel {
	case RiskLevelLow:
		return 0.0
	case RiskLevelMedium:
		return 0.3
	case RiskLevelHigh:
		return 0.5
	case RiskLevelCritical:
		return 0.7
	default:
		return 0.0
	}
}

// DefaultExecutionModeForRiskLevel returns the default execution mode for a risk level
func DefaultExecutionModeForRiskLevel(riskLevel string) string {
	switch riskLevel {
	case RiskLevelLow, RiskLevelMedium:
		return ExecutionModeAuto
	case RiskLevelHigh:
		return ExecutionModeNotify
	case RiskLevelCritical:
		return ExecutionModeReview
	default:
		return ExecutionModeAuto
	}
}

// Type returns the full capability type string (namespace:action)
func (c *CapabilityDefinition) Type() string {
	return c.Namespace + ":" + c.Action
}

// Reserved namespaces that only core capabilities can use
var ReservedNamespaces = []string{
	"file", "db", "api", "network", "system", "user", "mcp", "data", "secrets",
}

// CapabilityValidationPattern is the regex pattern for valid capability strings
const CapabilityValidationPattern = `^[a-z][a-z0-9]*:[a-z][a-z0-9_]*$`

// Standard capability types
const (
	CapabilityFileRead        = "file:read"
	CapabilityFileWrite       = "file:write"
	CapabilityFileDelete      = "file:delete"
	CapabilityFileExecute     = "file:execute"
	CapabilityNetworkAccess   = "network:access"
	CapabilityNetworkConnect  = "network:connect"
	CapabilityNetworkListen   = "network:listen"
	CapabilityAPICall         = "api:call"
	CapabilityAPIWebhook      = "api:webhook"
	CapabilityDBRead          = "db:read"
	CapabilityDBQuery         = "db:query" // Alias for db:read
	CapabilityDBWrite         = "db:write"
	CapabilityDBAdmin         = "db:admin"
	CapabilityUserRead        = "user:read"
	CapabilityUserWrite       = "user:write"
	CapabilityUserImpersonate = "user:impersonate"
	CapabilityDataExport      = "data:export"
	CapabilityDataEncrypt     = "data:encrypt"
	CapabilitySystemExec      = "system:exec"
	CapabilitySystemAdmin     = "system:admin"
	CapabilityMCPToolUse      = "mcp:tool_use"
	CapabilityMCPResourceRead = "mcp:resource_read"

	// Secrets management capabilities
	CapabilitySecretsResolve = "secrets:resolve"
	CapabilitySecretsStore   = "secrets:store"
	CapabilitySecretsRotate  = "secrets:rotate"
	CapabilitySecretsDelete  = "secrets:delete"
)

// Violation severity levels
const (
	ViolationSeverityLow      = "low"
	ViolationSeverityMedium   = "medium"
	ViolationSeverityHigh     = "high"
	ViolationSeverityCritical = "critical"
)

// Risk levels
const (
	RiskLevelLow      = "low"
	RiskLevelMedium   = "medium"
	RiskLevelHigh     = "high"
	RiskLevelCritical = "critical"
)

// CapabilityError represents a capability validation error
type CapabilityError struct {
	Capability string
	Message    string
}

func (e *CapabilityError) Error() string {
	return e.Message
}

// ValidateCapabilityFormat validates that a capability string matches namespace:action format
func ValidateCapabilityFormat(capability string) error {
	if len(capability) == 0 {
		return &CapabilityError{Capability: capability, Message: "capability cannot be empty"}
	}

	// Check for exactly one colon
	colonCount := 0
	colonIndex := -1
	for i, c := range capability {
		if c == ':' {
			colonCount++
			colonIndex = i
		}
	}

	if colonCount != 1 {
		return &CapabilityError{
			Capability: capability,
			Message:    "capability must match format 'namespace:action' (e.g., 'file:read', 'payment:process')",
		}
	}

	namespace := capability[:colonIndex]
	action := capability[colonIndex+1:]

	// Validate namespace (lowercase alphanumeric, starts with letter)
	if len(namespace) == 0 {
		return &CapabilityError{Capability: capability, Message: "namespace cannot be empty"}
	}
	if !isLowerLetter(rune(namespace[0])) {
		return &CapabilityError{Capability: capability, Message: "namespace must start with a lowercase letter"}
	}
	for _, c := range namespace {
		if !isLowerAlphanumeric(c) {
			return &CapabilityError{Capability: capability, Message: "namespace must contain only lowercase letters and numbers"}
		}
	}

	// Validate action (lowercase alphanumeric with underscores, starts with letter)
	if len(action) == 0 {
		return &CapabilityError{Capability: capability, Message: "action cannot be empty"}
	}
	if !isLowerLetter(rune(action[0])) {
		return &CapabilityError{Capability: capability, Message: "action must start with a lowercase letter"}
	}
	for _, c := range action {
		if !isLowerAlphanumeric(c) && c != '_' {
			return &CapabilityError{Capability: capability, Message: "action must contain only lowercase letters, numbers, and underscores"}
		}
	}

	return nil
}

// ParseCapability splits a capability string into namespace and action
func ParseCapability(capability string) (namespace, action string, err error) {
	if err := ValidateCapabilityFormat(capability); err != nil {
		return "", "", err
	}

	for i, c := range capability {
		if c == ':' {
			return capability[:i], capability[i+1:], nil
		}
	}

	return "", "", &CapabilityError{Capability: capability, Message: "invalid capability format"}
}

// IsReservedNamespace checks if a namespace is reserved for core capabilities
func IsReservedNamespace(namespace string) bool {
	for _, reserved := range ReservedNamespaces {
		if namespace == reserved {
			return true
		}
	}
	return false
}

func isLowerLetter(c rune) bool {
	return c >= 'a' && c <= 'z'
}

func isLowerAlphanumeric(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')
}
