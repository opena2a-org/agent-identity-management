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
	ID               uuid.UUID         `json:"id"`
	OrganizationID   uuid.UUID         `json:"organization_id"`
	Name             string            `json:"name"`
	Description      string            `json:"description"`
	PolicyType       PolicyType        `json:"policy_type"`
	EnforcementAction EnforcementAction `json:"enforcement_action"`

	// Severity threshold - only trigger for alerts at or above this level
	SeverityThreshold AlertSeverity `json:"severity_threshold"`

	// Policy configuration (JSON)
	Rules map[string]interface{} `json:"rules"`

	// Scope
	AppliesTo string `json:"applies_to"` // "all", "agent_id:xxx", "agent_type:ai", etc.

	// Status
	IsEnabled bool      `json:"is_enabled"`
	Priority  int       `json:"priority"` // Higher priority policies evaluated first

	// Timestamps
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	CreatedBy uuid.UUID  `json:"created_by"`
}

// PolicyEvaluationResult represents the result of evaluating a policy
type PolicyEvaluationResult struct {
	PolicyID          uuid.UUID         `json:"policy_id"`
	PolicyName        string            `json:"policy_name"`
	Triggered         bool              `json:"triggered"`
	EnforcementAction EnforcementAction `json:"enforcement_action"`
	Reason            string            `json:"reason"`
	ShouldBlock       bool              `json:"should_block"`
	ShouldAlert       bool              `json:"should_alert"`
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
