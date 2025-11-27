package domain

import (
	"time"

	"github.com/google/uuid"
)

// TrustScoreFactors contains the individual factors contributing to trust score
// Based on 8-factor trust scoring algorithm (see documentation)
type TrustScoreFactors struct {
	// Factor 1: Verification Status (25% weight) - Ed25519 signature verification
	VerificationStatus float64 `json:"verificationStatus"` // 0-1

	// Factor 2: Uptime & Availability (15% weight) - Health check responsiveness
	Uptime float64 `json:"uptime"` // 0-1

	// Factor 3: Action Success Rate (15% weight) - Successful vs failed actions
	SuccessRate float64 `json:"successRate"` // 0-1

	// Factor 4: Security Alerts (15% weight) - Active security alerts by severity
	SecurityAlerts float64 `json:"securityAlerts"` // 0-1

	// Factor 5: Compliance Score (10% weight) - SOC 2, HIPAA, GDPR adherence
	Compliance float64 `json:"compliance"` // 0-1

	// Factor 6: Age & History (10% weight) - How long agent has been operating
	Age float64 `json:"age"` // 0-1

	// Factor 7: Drift Detection (5% weight) - Behavioral pattern changes
	DriftDetection float64 `json:"driftDetection"` // 0-1

	// Factor 8: User Feedback (5% weight) - Explicit user ratings
	UserFeedback float64 `json:"userFeedback"` // 0-1
}

// TrustScore represents a calculated trust score for an agent
type TrustScore struct {
	ID             uuid.UUID         `json:"id"`
	AgentID        uuid.UUID         `json:"agentId"`
	Score          float64           `json:"score"` // 0-1
	Factors        TrustScoreFactors `json:"factors"`
	Confidence     float64           `json:"confidence"` // 0-1
	LastCalculated time.Time         `json:"lastCalculated"`
	CreatedAt      time.Time         `json:"createdAt"`
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
	AgentID        uuid.UUID  `json:"agentId"`
	OrganizationID uuid.UUID  `json:"organizationId"`
	TrustScore     float64    `json:"trustScore"` // 0-1
	PreviousScore  *float64   `json:"previousScore,omitempty"` // 0-1, nullable
	ChangeReason   string     `json:"reason"` // Frontend expects "reason"
	ChangedBy      *uuid.UUID `json:"changedBy,omitempty"` // NULL for automated changes
	RecordedAt     time.Time  `json:"timestamp"` // Frontend expects "timestamp"
	CreatedAt      time.Time  `json:"createdAt"`
}

// TrustScoreCalculator defines the interface for trust score calculation
type TrustScoreCalculator interface {
	Calculate(agent *Agent) (*TrustScore, error)
	CalculateFactors(agent *Agent) (*TrustScoreFactors, error)
}
