package domain

import (
	"time"

	"github.com/google/uuid"
)

// BehavioralAnomalyType represents the type of behavioral anomaly detected
type BehavioralAnomalyType string

const (
	BehavioralAnomalyVelocitySpike   BehavioralAnomalyType = "velocity_spike"    // Sudden increase in activity rate
	BehavioralAnomalyNewCapability   BehavioralAnomalyType = "new_capability"    // First-time use of a capability
	BehavioralAnomalyNewResource     BehavioralAnomalyType = "new_resource"      // Access to never-before-seen resource
	BehavioralAnomalyPatternBreak    BehavioralAnomalyType = "pattern_break"     // Unusual action sequence
	BehavioralAnomalyCrossAgent      BehavioralAnomalyType = "cross_agent"       // Coordinated unusual activity
	BehavioralAnomalyRiskSpike       BehavioralAnomalyType = "risk_spike"        // Sudden increase in risk-weighted score
	BehavioralAnomalyCapabilityDrift BehavioralAnomalyType = "capability_drift"  // Unusual capability usage pattern
)

// AnomalySeverity represents the severity level of a detected anomaly
type AnomalySeverity string

const (
	AnomalySeverityLow      AnomalySeverity = "low"
	AnomalySeverityMedium   AnomalySeverity = "medium"
	AnomalySeverityHigh     AnomalySeverity = "high"
	AnomalySeverityCritical AnomalySeverity = "critical"
)

// AgentBehaviorBaseline stores the learned behavioral patterns for an agent
type AgentBehaviorBaseline struct {
	ID             uuid.UUID `json:"id"`
	AgentID        uuid.UUID `json:"agentId"`
	OrganizationID uuid.UUID `json:"organizationId"`

	// Baseline learning period
	BaselineStartAt time.Time `json:"baselineStartAt"`
	BaselineEndAt   time.Time `json:"baselineEndAt"`
	SamplesCount    int       `json:"samplesCount"`

	// Velocity metrics (calls per hour)
	AvgCallsPerHour    float64 `json:"avgCallsPerHour"`
	StddevCallsPerHour float64 `json:"stddevCallsPerHour"`
	MaxCallsPerHour    int     `json:"maxCallsPerHour"`

	// Capability usage patterns (map of capability -> usage count)
	CapabilityUsage map[string]int `json:"capabilityUsage"`

	// Resource access patterns (map of resource type -> access count)
	ResourcePatterns map[string]int `json:"resourcePatterns"`

	// Common action sequences
	ActionSequences []ActionSequence `json:"actionSequences"`

	// Risk-weighted activity score
	AvgRiskScore    float64 `json:"avgRiskScore"`
	StddevRiskScore float64 `json:"stddevRiskScore"`

	// Metadata
	IsMature          bool      `json:"isMature"`
	MaturityThreshold int       `json:"maturityThreshold"`
	LastUpdatedAt     time.Time `json:"lastUpdatedAt"`
	CreatedAt         time.Time `json:"createdAt"`
}

// ActionSequence represents a common sequence of actions
type ActionSequence struct {
	Sequence []string `json:"sequence"`
	Count    int      `json:"count"`
}

// BehavioralAnomaly represents a detected deviation from normal behavior
type BehavioralAnomaly struct {
	ID             uuid.UUID `json:"id"`
	AgentID        uuid.UUID `json:"agentId"`
	OrganizationID uuid.UUID `json:"organizationId"`

	// Anomaly classification
	AnomalyType BehavioralAnomalyType `json:"anomalyType"`
	Severity    AnomalySeverity `json:"severity"`

	// Anomaly details
	Description string `json:"description"`

	// Statistical details
	ExpectedValue  *float64 `json:"expectedValue,omitempty"`
	ActualValue    *float64 `json:"actualValue,omitempty"`
	DeviationSigma *float64 `json:"deviationSigma,omitempty"`

	// Context
	TriggerAction     string `json:"triggerAction,omitempty"`
	TriggerResource   string `json:"triggerResource,omitempty"`
	TriggerCapability string `json:"triggerCapability,omitempty"`

	// Metadata
	Metadata   map[string]interface{} `json:"metadata"`
	DetectedAt time.Time              `json:"detectedAt"`

	// Link to alert if one was created
	AlertID *uuid.UUID `json:"alertId,omitempty"`
}

// DetectionResult represents the result of behavioral anomaly detection
type DetectionResult struct {
	IsAnomalous bool               `json:"isAnomalous"`
	Anomalies   []BehavioralAnomaly `json:"anomalies,omitempty"`
	ShouldBlock bool               `json:"shouldBlock"`
	ShouldAlert bool               `json:"shouldAlert"`
	Reason      string             `json:"reason,omitempty"`
}

// BehaviorBaselineRepository defines the interface for baseline storage
type BehaviorBaselineRepository interface {
	// Get the baseline for an agent (returns nil if not exists)
	GetByAgentID(agentID uuid.UUID) (*AgentBehaviorBaseline, error)

	// Create or update a baseline
	Upsert(baseline *AgentBehaviorBaseline) error

	// Get all mature baselines for an organization
	GetMatureByOrganization(orgID uuid.UUID) ([]*AgentBehaviorBaseline, error)

	// Delete baseline for an agent
	Delete(agentID uuid.UUID) error

	// Record a detected anomaly
	RecordAnomaly(anomaly *BehavioralAnomaly) error

	// Get recent anomalies for an agent
	GetRecentAnomalies(agentID uuid.UUID, limit int) ([]*BehavioralAnomaly, error)

	// Get anomalies by type for an organization
	GetAnomaliesByType(orgID uuid.UUID, anomalyType BehavioralAnomalyType, limit int) ([]*BehavioralAnomaly, error)
}

// CalculateSigma calculates how many standard deviations a value is from the mean
func CalculateSigma(value, mean, stddev float64) float64 {
	if stddev == 0 {
		return 0
	}
	return (value - mean) / stddev
}

// IsSignificantDeviation returns true if the sigma value indicates a significant deviation
// Using 3-sigma rule: values more than 3 standard deviations from mean are significant
func IsSignificantDeviation(sigma float64, threshold float64) bool {
	if threshold == 0 {
		threshold = 3.0 // Default to 3-sigma
	}
	return sigma > threshold || sigma < -threshold
}

// GetRiskWeight returns a risk weight for a capability/action
// Higher weights indicate more sensitive operations
func GetRiskWeight(capability string) float64 {
	// Risk weights for different capability categories
	riskWeights := map[string]float64{
		"admin":    10.0, // Administrative actions
		"delete":   8.0,  // Deletion operations
		"write":    5.0,  // Write operations
		"execute":  6.0,  // Execution operations
		"security": 9.0,  // Security-related
		"system":   8.0,  // System-level access
		"read":     1.0,  // Read operations (low risk)
		"list":     1.0,  // List operations (low risk)
		"api":      2.0,  // API calls
	}

	// Check for prefix matches
	for prefix, weight := range riskWeights {
		if len(capability) >= len(prefix) && capability[:len(prefix)] == prefix {
			return weight
		}
	}

	// Default weight for unknown capabilities
	return 3.0
}
