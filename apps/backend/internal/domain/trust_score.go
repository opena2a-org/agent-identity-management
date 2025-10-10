package domain

import (
	"time"

	"github.com/google/uuid"
)

// TrustScoreFactors contains the individual factors contributing to trust score
type TrustScoreFactors struct {
	VerificationStatus  float64 `json:"verification_status"`  // 0-1
	CertificateValidity float64 `json:"certificate_validity"` // 0-1
	RepositoryQuality   float64 `json:"repository_quality"`   // 0-1
	DocumentationScore  float64 `json:"documentation_score"`  // 0-1
	CommunityTrust      float64 `json:"community_trust"`      // 0-1
	SecurityAudit       float64 `json:"security_audit"`       // 0-1
	UpdateFrequency     float64 `json:"update_frequency"`     // 0-1
	AgeScore            float64 `json:"age_score"`            // 0-1
	CapabilityRisk      float64 `json:"capability_risk"`      // 0-1 (1 = low risk, 0 = high risk)
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
}

// TrustScoreCalculator defines the interface for trust score calculation
type TrustScoreCalculator interface {
	Calculate(agent *Agent) (*TrustScore, error)
	CalculateFactors(agent *Agent) (*TrustScoreFactors, error)
}
