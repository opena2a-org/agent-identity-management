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
