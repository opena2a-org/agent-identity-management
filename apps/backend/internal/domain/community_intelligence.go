package domain

import (
	"time"

	"github.com/google/uuid"
)

// CommunityIntelligenceReport tracks anonymized trust factor data sent to the Registry
type CommunityIntelligenceReport struct {
	ID               uuid.UUID              `json:"id"`
	OrganizationID   uuid.UUID              `json:"organizationId"`
	ReportType       string                 `json:"reportType"`
	AgentCount       int                    `json:"agentCount"`
	DataPoints       map[string]interface{} `json:"dataPoints"`
	SubmittedAt      time.Time              `json:"submittedAt"`
	RegistryAccepted bool                   `json:"registryAccepted"`
	CreatedAt        time.Time              `json:"createdAt"`
}

// TrustFactorDistribution represents anonymized aggregate trust factor data.
// No agent IDs, no PII -- only statistical distributions.
type TrustFactorDistribution struct {
	TotalAgents         int                    `json:"totalAgents"`
	FactorDistributions map[string]FactorStats `json:"factorDistributions"`
	ScoreDistribution   ScoreDistribution      `json:"scoreDistribution"`
	Timestamp           string                 `json:"timestamp"`
}

// FactorStats holds min/max/mean/median for a single trust factor
type FactorStats struct {
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
	Mean   float64 `json:"mean"`
	Median float64 `json:"median"`
	P25    float64 `json:"p25"`
	P75    float64 `json:"p75"`
}

// ScoreDistribution shows overall trust score distribution in buckets
type ScoreDistribution struct {
	Bucket0to20   int `json:"bucket0to20"`
	Bucket20to40  int `json:"bucket20to40"`
	Bucket40to60  int `json:"bucket40to60"`
	Bucket60to80  int `json:"bucket60to80"`
	Bucket80to100 int `json:"bucket80to100"`
}

// CommunityBenchmark represents aggregated benchmarks from all opted-in organizations
type CommunityBenchmark struct {
	TotalOptedInOrgs    int                    `json:"totalOptedInOrgs"`
	TotalAgentsReported int                    `json:"totalAgentsReported"`
	GlobalMedianScore   float64                `json:"globalMedianScore"`
	FactorBenchmarks    map[string]FactorStats `json:"factorBenchmarks"`
	ScoreDistribution   ScoreDistribution      `json:"scoreDistribution"`
	LastUpdated         string                 `json:"lastUpdated"`
	SufficientData      bool                   `json:"sufficientData"` // true when >= 10 agents opted in
}

// CommunityIntelligenceRepository defines persistence for CI reports
type CommunityIntelligenceRepository interface {
	SaveReport(report *CommunityIntelligenceReport) error
	GetReportsByOrg(orgID uuid.UUID, limit int) ([]*CommunityIntelligenceReport, error)
	GetOptedInOrgCount() (int, error)
	GetAllRecentReports(since time.Time) ([]*CommunityIntelligenceReport, error)
}
