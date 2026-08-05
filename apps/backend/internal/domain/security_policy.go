package domain

import (
	"time"

	"github.com/google/uuid"
)

// PolicyType represents different types of security policies
type PolicyType string

const (
	PolicyTypeCapabilityViolation PolicyType = "capability_violation"
	PolicyTypeTrustScoreLow       PolicyType = "trust_score_low"
	PolicyTypeUnusualActivity     PolicyType = "unusual_activity"
	PolicyTypeUnauthorizedAccess  PolicyType = "unauthorized_access"
	PolicyTypeDataExfiltration    PolicyType = "data_exfiltration"
	PolicyTypeConfigDrift         PolicyType = "config_drift"

	// MCP-specific policy types
	PolicyTypeMCPAllowlist    PolicyType = "mcp_allowlist"    // Only allow listed MCPs
	PolicyTypeMCPBlocklist    PolicyType = "mcp_blocklist"    // Block specific MCPs
	PolicyTypeMCPCapabilities PolicyType = "mcp_capabilities" // Required/forbidden capabilities
	PolicyTypeMCPUnverified   PolicyType = "mcp_unverified"   // Action on unverified MCPs
)

// EnforcementAction defines what action to take when policy is triggered
type EnforcementAction string

const (
	EnforcementAlertOnly     EnforcementAction = "alert_only"      // Generate alert, allow action
	EnforcementBlockAndAlert EnforcementAction = "block_and_alert" // Generate alert, deny action
	EnforcementAllow         EnforcementAction = "allow"           // Permit action, no alert
)

// SecurityPolicy represents a configurable security policy
type SecurityPolicy struct {
	ID                uuid.UUID         `json:"id"`
	OrganizationID    uuid.UUID         `json:"organizationId"`
	Name              string            `json:"name"`
	Description       string            `json:"description"`
	PolicyType        PolicyType        `json:"policyType"`
	EnforcementAction EnforcementAction `json:"enforcementAction"`

	// Severity threshold - only trigger for alerts at or above this level
	SeverityThreshold AlertSeverity `json:"severityThreshold"`

	// Policy configuration (JSON)
	Rules map[string]interface{} `json:"rules"`

	// Scope
	AppliesTo string `json:"appliesTo"` // "all", "agent_id:xxx", "agent_type:ai", etc.

	// Status
	IsEnabled bool `json:"isEnabled"`
	Priority  int  `json:"priority"` // Higher priority policies evaluated first

	// Timestamps
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	CreatedBy uuid.UUID `json:"createdBy"`
}

// PolicyEvaluationResult represents the result of evaluating a policy
type PolicyEvaluationResult struct {
	PolicyID          uuid.UUID         `json:"policyId"`
	PolicyName        string            `json:"policyName"`
	Triggered         bool              `json:"triggered"`
	EnforcementAction EnforcementAction `json:"enforcementAction"`
	Reason            string            `json:"reason"`
	ShouldBlock       bool              `json:"shouldBlock"`
	ShouldAlert       bool              `json:"shouldAlert"`
}

// SecurityPolicyRepository defines the interface for security policy persistence
type SecurityPolicyRepository interface {
	Create(policy *SecurityPolicy) error
	GetByID(id uuid.UUID) (*SecurityPolicy, error)
	GetByOrganization(orgID uuid.UUID) ([]*SecurityPolicy, error)
	GetActiveByOrganization(orgID uuid.UUID) ([]*SecurityPolicy, error)
	GetByType(orgID uuid.UUID, policyType PolicyType) ([]*SecurityPolicy, error)
	Update(policy *SecurityPolicy) error
	Delete(id uuid.UUID) error
}

// ============================================================================
// MCP Policy Rule Structures
// ============================================================================

// MCPAllowlistRules defines rules for MCP allowlist policies
// Only MCPs matching these criteria are allowed to be used by agents
type MCPAllowlistRules struct {
	// AllowedDomains is a list of allowed domain patterns (supports wildcards)
	// Example: ["*.company.com", "github.com", "*.anthropic.com"]
	AllowedDomains []string `json:"allowedDomains,omitempty"`

	// AllowedNames is a list of allowed MCP server names (exact match)
	// Example: ["filesystem-mcp", "github-mcp", "slack-mcp"]
	AllowedNames []string `json:"allowedNames,omitempty"`

	// AllowedCapabilities is a list of allowed capability types
	// Example: ["tools", "resources", "prompts"]
	AllowedCapabilities []string `json:"allowedCapabilities,omitempty"`

	// RequireVerified requires MCP servers to be verified before use
	RequireVerified bool `json:"requireVerified"`

	// MinTrustScore is the minimum trust score required, on the canonical
	// [0,1] trust scale (e.g. 0.7, not 70). This comment previously said
	// 0-100, which disagreed with every value actually assigned to the field
	// and with `mcp_servers.trust_score` (CHECK 0..1, migration 104).
	MinTrustScore float64 `json:"minTrustScore"`

	// MinConfidenceScore is the minimum confidence score required (0-100)
	MinConfidenceScore float64 `json:"minConfidenceScore"`

	// MinAttestations is the minimum number of attestations required
	MinAttestations int `json:"minAttestations"`
}

// MCPBlocklistRules defines rules for MCP blocklist policies
// MCPs matching these criteria are blocked from use
type MCPBlocklistRules struct {
	// BlockedDomains is a list of blocked domain patterns (supports wildcards)
	// Example: ["malicious.com", "*.sketchy-domain.net"]
	BlockedDomains []string `json:"blockedDomains,omitempty"`

	// BlockedNames is a list of blocked MCP server names (exact match)
	// Example: ["compromised-mcp", "deprecated-server"]
	BlockedNames []string `json:"blockedNames,omitempty"`

	// BlockedURLs is a list of exact URL matches to block
	// Example: ["https://malicious.com/mcp", "http://insecure-server.com/api"]
	BlockedURLs []string `json:"blockedUrls,omitempty"`

	// BlockedCapabilities is a list of forbidden capability types
	// Example: ["dangerous-tool", "unrestricted-access"]
	BlockedCapabilities []string `json:"blockedCapabilities,omitempty"`
}

// MCPCapabilitiesRules defines rules for MCP capability policies
// Controls which capabilities are required or forbidden
type MCPCapabilitiesRules struct {
	// RequiredCapabilities are capabilities that MUST be present
	// Example: ["tools", "resources"]
	RequiredCapabilities []string `json:"requiredCapabilities,omitempty"`

	// ForbiddenCapabilities are capabilities that MUST NOT be present
	// Example: ["admin", "system-access", "file-write"]
	ForbiddenCapabilities []string `json:"forbiddenCapabilities,omitempty"`

	// RequireAllCapabilities requires all listed capabilities (AND logic)
	// If false, at least one required capability is sufficient (OR logic)
	RequireAllCapabilities bool `json:"requireAllCapabilities"`

	// MaxCapabilityCount limits the total number of capabilities
	// 0 means no limit
	MaxCapabilityCount int `json:"maxCapabilityCount"`
}

// MCPUnverifiedRules defines rules for handling unverified MCP servers
type MCPUnverifiedRules struct {
	// Action defines what to do when an unverified MCP is detected
	// "alert_only" - Generate alert but allow usage
	// "block_and_alert" - Block usage and generate alert
	// "quarantine" - Allow limited access with monitoring
	Action string `json:"action"`

	// GracePeriodDays allows newly registered MCPs a grace period before enforcement
	// 0 means no grace period
	GracePeriodDays int `json:"gracePeriodDays"`

	// RequireManualApproval requires admin approval for unverified MCPs
	RequireManualApproval bool `json:"requireManualApproval"`

	// AutoQuarantineAfterDays automatically quarantine MCPs that remain unverified
	// 0 means never auto-quarantine
	AutoQuarantineAfterDays int `json:"autoQuarantineAfterDays"`

	// AlertOnFirstUse generates an alert when an unverified MCP is first accessed
	AlertOnFirstUse bool `json:"alertOnFirstUse"`

	// NotifyOwners notifies MCP owners when their server is flagged as unverified
	NotifyOwners bool `json:"notifyOwners"`
}

// MCPPolicyEvaluationResult extends PolicyEvaluationResult with MCP-specific details
type MCPPolicyEvaluationResult struct {
	PolicyEvaluationResult

	// MCPServerID is the ID of the MCP server being evaluated
	MCPServerID uuid.UUID `json:"mcpServerId"`

	// MCPServerName is the name of the MCP server
	MCPServerName string `json:"mcpServerName"`

	// MCPServerURL is the URL of the MCP server
	MCPServerURL string `json:"mcpServerUrl"`

	// ViolatedRules contains details about which specific rules were violated
	ViolatedRules []string `json:"violatedRules,omitempty"`

	// MatchedPatterns contains the patterns that matched (for allowlist/blocklist)
	MatchedPatterns []string `json:"matchedPatterns,omitempty"`

	// Recommendations contains suggested actions to resolve policy violations
	Recommendations []string `json:"recommendations,omitempty"`
}
