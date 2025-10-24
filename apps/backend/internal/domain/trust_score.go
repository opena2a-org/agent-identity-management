package domain

import (
	"time"

	"github.com/google/uuid"
)

// TrustScoreFactors contains the individual factors contributing to trust score
// Based on 8-factor trust scoring algorithm (see documentation)
type TrustScoreFactors struct {
	// Factor 1: Verification Status (25% weight) - Ed25519 signature verification
	VerificationStatus float64 `json:"verification_status"` // 0-1

	// Factor 2: Uptime & Availability (15% weight) - Health check responsiveness
	Uptime float64 `json:"uptime"` // 0-1

	// Factor 3: Action Success Rate (15% weight) - Successful vs failed actions
	SuccessRate float64 `json:"success_rate"` // 0-1

	// Factor 4: Security Alerts (15% weight) - Active security alerts by severity
	SecurityAlerts float64 `json:"security_alerts"` // 0-1

	// Factor 5: Compliance Score (10% weight) - SOC 2, HIPAA, GDPR adherence
	Compliance float64 `json:"compliance"` // 0-1

	// Factor 6: Age & History (10% weight) - How long agent has been operating
	Age float64 `json:"age"` // 0-1

	// Factor 7: Drift Detection (5% weight) - Behavioral pattern changes
	DriftDetection float64 `json:"drift_detection"` // 0-1

	// Factor 8: User Feedback (5% weight) - Explicit user ratings
	UserFeedback float64 `json:"user_feedback"` // 0-1
}

// TrustScore represents a calculated trust score for an agent
type TrustScore struct {
	ID             uuid.UUID          `json:"id"`
	AgentID        uuid.UUID          `json:"agent_id"`
	Score          float64            `json:"score"` // 0-1
	Factors        TrustScoreFactors  `json:"factors"`
	Confidence     float64            `json:"confidence"` // 0-1
	LastCalculated time.Time          `json:"last_calculated"`
	CreatedAt      time.Time          `json:"created_at"`
}

// TrustScoreRepository defines the interface for trust score persistence
type TrustScoreRepository interface {
	Create(score *TrustScore) error
	GetByAgent(agentID uuid.UUID) (*TrustScore, error)
	GetLatest(agentID uuid.UUID) (*TrustScore, error)
	GetHistory(agentID uuid.UUID, limit int) ([]*TrustScore, error)
	GetHistoryAuditTrail(agentID uuid.UUID, limit int) ([]*TrustScoreHistoryEntry, error)
}

// TrustScoreHistoryEntry represents an audit trail entry for trust score changes
// Maps to trust_score_history table in database
type TrustScoreHistoryEntry struct {
	ID             uuid.UUID  `json:"id"`
	AgentID        uuid.UUID  `json:"agent_id"`
	OrganizationID uuid.UUID  `json:"organization_id"`
	TrustScore     float64    `json:"trust_score"` // 0-1
	PreviousScore  *float64   `json:"previous_score,omitempty"` // 0-1, nullable
	ChangeReason   string     `json:"reason"` // Frontend expects "reason" not "change_reason"
	ChangedBy      *uuid.UUID `json:"changed_by,omitempty"` // NULL for automated changes
	RecordedAt     time.Time  `json:"timestamp"` // Frontend expects "timestamp" not "recorded_at"
	CreatedAt      time.Time  `json:"created_at"`
}

// TrustScoreCalculator defines the interface for trust score calculation
type TrustScoreCalculator interface {
	Calculate(agent *Agent) (*TrustScore, error)
	CalculateFactors(agent *Agent) (*TrustScoreFactors, error)
}
