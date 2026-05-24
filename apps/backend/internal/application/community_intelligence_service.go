package application

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

const communityIntelligenceMinThreshold = 10

// CommunityIntelligenceService handles opt-in telemetry aggregation and push
type CommunityIntelligenceService struct {
	orgRepo     domain.OrganizationRepository
	agentRepo   domain.AgentRepository
	trustRepo   domain.TrustScoreRepository
	ciRepo      domain.CommunityIntelligenceRepository
	registryURL string
	httpClient  *http.Client
}

// NewCommunityIntelligenceService creates a new community intelligence service
func NewCommunityIntelligenceService(
	orgRepo domain.OrganizationRepository,
	agentRepo domain.AgentRepository,
	trustRepo domain.TrustScoreRepository,
	ciRepo domain.CommunityIntelligenceRepository,
	registryURL string,
) *CommunityIntelligenceService {
	return &CommunityIntelligenceService{
		orgRepo:     orgRepo,
		agentRepo:   agentRepo,
		trustRepo:   trustRepo,
		ciRepo:      ciRepo,
		registryURL: registryURL,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
	}
}

// EnableOptIn enables community intelligence for an organization
func (s *CommunityIntelligenceService) EnableOptIn(ctx context.Context, orgID uuid.UUID) error {
	org, err := s.orgRepo.GetByID(orgID)
	if err != nil {
		return fmt.Errorf("failed to get organization: %w", err)
	}

	now := time.Now().UTC()
	org.CommunityIntelligenceEnabled = true
	org.CommunityIntelligenceConsentedAt = &now

	if err := s.orgRepo.Update(org); err != nil {
		return fmt.Errorf("failed to update organization: %w", err)
	}

	log.Printf("Community intelligence enabled for org %s", orgID)
	return nil
}

// DisableOptIn disables community intelligence for an organization
func (s *CommunityIntelligenceService) DisableOptIn(ctx context.Context, orgID uuid.UUID) error {
	org, err := s.orgRepo.GetByID(orgID)
	if err != nil {
		return fmt.Errorf("failed to get organization: %w", err)
	}

	org.CommunityIntelligenceEnabled = false

	if err := s.orgRepo.Update(org); err != nil {
		return fmt.Errorf("failed to update organization: %w", err)
	}

	log.Printf("Community intelligence disabled for org %s", orgID)
	return nil
}

// GetOptInStatus returns the current opt-in status for an organization
func (s *CommunityIntelligenceService) GetOptInStatus(ctx context.Context, orgID uuid.UUID) (bool, *time.Time, error) {
	org, err := s.orgRepo.GetByID(orgID)
	if err != nil {
		return false, nil, fmt.Errorf("failed to get organization: %w", err)
	}
	return org.CommunityIntelligenceEnabled, org.CommunityIntelligenceConsentedAt, nil
}

// CollectAndPushTelemetry aggregates trust factor distributions from all opted-in orgs
// and pushes anonymized data to the Registry
func (s *CommunityIntelligenceService) CollectAndPushTelemetry(ctx context.Context) error {
	orgs, err := s.orgRepo.ListAll()
	if err != nil {
		return fmt.Errorf("failed to list organizations: %w", err)
	}

	secret := os.Getenv("REGISTRY_BRIDGE_SECRET")

	var totalPushed int
	for _, org := range orgs {
		if !org.IsActive || !org.CommunityIntelligenceEnabled {
			continue
		}

		distribution, err := s.buildTrustDistribution(ctx, org.ID)
		if err != nil {
			log.Printf("Community intelligence: failed to build distribution for org %s: %v", org.ID, err)
			continue
		}

		if distribution.TotalAgents == 0 {
			continue
		}

		// Save report locally
		dataPoints, _ := structToMap(distribution)
		report := &domain.CommunityIntelligenceReport{
			ID:             uuid.New(),
			OrganizationID: org.ID,
			ReportType:     "trust_factor_distribution",
			AgentCount:     distribution.TotalAgents,
			DataPoints:     dataPoints,
			SubmittedAt:    time.Now().UTC(),
		}

		// Push to registry
		if s.registryURL != "" && secret != "" {
			if err := s.pushToRegistry(ctx, distribution, org.ID, secret); err != nil {
				log.Printf("Community intelligence: failed to push to registry for org %s: %v", org.ID, err)
				report.RegistryAccepted = false
			} else {
				report.RegistryAccepted = true
				totalPushed++
			}
		}

		if err := s.ciRepo.SaveReport(report); err != nil {
			log.Printf("Community intelligence: failed to save report for org %s: %v", org.ID, err)
		}
	}

	log.Printf("Community intelligence: pushed %d org distributions to registry", totalPushed)
	return nil
}

// GetCommunityBenchmarks returns aggregated benchmarks from all opted-in orgs.
// Only returns data when threshold (10+ agents) is met.
func (s *CommunityIntelligenceService) GetCommunityBenchmarks(ctx context.Context) (*domain.CommunityBenchmark, error) {
	since := time.Now().UTC().Add(-24 * time.Hour) // Last 24h of reports
	reports, err := s.ciRepo.GetAllRecentReports(since)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent reports: %w", err)
	}

	optedInCount, err := s.ciRepo.GetOptedInOrgCount()
	if err != nil {
		return nil, fmt.Errorf("failed to get opted-in org count: %w", err)
	}

	totalAgents := 0
	allScores := make([]float64, 0)
	factorAggregates := make(map[string][]float64)
	var scoreDist domain.ScoreDistribution

	for _, report := range reports {
		totalAgents += report.AgentCount

		if dist, ok := report.DataPoints["scoreDistribution"].(map[string]interface{}); ok {
			if v, ok := dist["bucket0to20"].(float64); ok {
				scoreDist.Bucket0to20 += int(v)
			}
			if v, ok := dist["bucket20to40"].(float64); ok {
				scoreDist.Bucket20to40 += int(v)
			}
			if v, ok := dist["bucket40to60"].(float64); ok {
				scoreDist.Bucket40to60 += int(v)
			}
			if v, ok := dist["bucket60to80"].(float64); ok {
				scoreDist.Bucket60to80 += int(v)
			}
			if v, ok := dist["bucket80to100"].(float64); ok {
				scoreDist.Bucket80to100 += int(v)
			}
		}

		if factors, ok := report.DataPoints["factorDistributions"].(map[string]interface{}); ok {
			for factorName, factorData := range factors {
				if fd, ok := factorData.(map[string]interface{}); ok {
					if mean, ok := fd["mean"].(float64); ok {
						factorAggregates[factorName] = append(factorAggregates[factorName], mean)
						allScores = append(allScores, mean)
					}
				}
			}
		}
	}

	sufficientData := totalAgents >= communityIntelligenceMinThreshold

	benchmark := &domain.CommunityBenchmark{
		TotalOptedInOrgs:    optedInCount,
		TotalAgentsReported: totalAgents,
		ScoreDistribution:   scoreDist,
		LastUpdated:         time.Now().UTC().Format(time.RFC3339),
		SufficientData:      sufficientData,
	}

	if sufficientData && len(allScores) > 0 {
		sort.Float64s(allScores)
		benchmark.GlobalMedianScore = allScores[len(allScores)/2]

		factorBenchmarks := make(map[string]domain.FactorStats)
		for name, values := range factorAggregates {
			factorBenchmarks[name] = computeStats(values)
		}
		benchmark.FactorBenchmarks = factorBenchmarks
	}

	return benchmark, nil
}

// buildTrustDistribution computes anonymized trust factor stats for an org
func (s *CommunityIntelligenceService) buildTrustDistribution(ctx context.Context, orgID uuid.UUID) (*domain.TrustFactorDistribution, error) {
	agents, err := s.agentRepo.GetByOrganization(orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agents: %w", err)
	}

	if len(agents) == 0 {
		return &domain.TrustFactorDistribution{TotalAgents: 0}, nil
	}

	// Collect trust scores for all agents
	scores := make([]float64, 0)
	factorValues := map[string][]float64{
		"verificationStatus": {},
		"uptime":             {},
		"successRate":        {},
		"securityAlerts":     {},
		"compliance":         {},
		"age":                {},
		"driftDetection":     {},
		"userFeedback":       {},
	}

	for _, agent := range agents {
		trustScore, err := s.trustRepo.GetLatest(agent.ID)
		if err != nil {
			// Agent may not have trust score yet
			scores = append(scores, agent.TrustScore)
			continue
		}

		scores = append(scores, trustScore.Score)
		factorValues["verificationStatus"] = append(factorValues["verificationStatus"], trustScore.Factors.VerificationStatus)
		factorValues["uptime"] = append(factorValues["uptime"], trustScore.Factors.Uptime)
		factorValues["successRate"] = append(factorValues["successRate"], trustScore.Factors.SuccessRate)
		factorValues["securityAlerts"] = append(factorValues["securityAlerts"], trustScore.Factors.SecurityAlerts)
		factorValues["compliance"] = append(factorValues["compliance"], trustScore.Factors.Compliance)
		factorValues["age"] = append(factorValues["age"], trustScore.Factors.Age)
		factorValues["driftDetection"] = append(factorValues["driftDetection"], trustScore.Factors.DriftDetection)
		factorValues["userFeedback"] = append(factorValues["userFeedback"], trustScore.Factors.UserFeedback)
	}

	// Compute distributions
	factorDistributions := make(map[string]domain.FactorStats)
	for name, values := range factorValues {
		if len(values) > 0 {
			factorDistributions[name] = computeStats(values)
		}
	}

	// Score distribution buckets
	var scoreDist domain.ScoreDistribution
	for _, score := range scores {
		normalized := score
		if normalized <= 1.0 {
			normalized *= 100 // Normalize 0-1 to 0-100
		}
		switch {
		case normalized < 20:
			scoreDist.Bucket0to20++
		case normalized < 40:
			scoreDist.Bucket20to40++
		case normalized < 60:
			scoreDist.Bucket40to60++
		case normalized < 80:
			scoreDist.Bucket60to80++
		default:
			scoreDist.Bucket80to100++
		}
	}

	return &domain.TrustFactorDistribution{
		TotalAgents:         len(agents),
		FactorDistributions: factorDistributions,
		ScoreDistribution:   scoreDist,
		Timestamp:           time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// pushToRegistry sends anonymized trust factor distribution to the OpenA2A Registry
func (s *CommunityIntelligenceService) pushToRegistry(ctx context.Context, dist *domain.TrustFactorDistribution, orgID uuid.UUID, secret string) error {
	payload := map[string]interface{}{
		"type":              "community_intelligence",
		"tool":              "aim-server",
		"toolVersion":       "1.5.0",
		"timestamp":         dist.Timestamp,
		"trustDistribution": dist,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	url := s.registryURL + "/api/v1/aim/community-intelligence"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "aim-server/1.5.0")
	// Use contributor token for anonymous correlation
	token := generateContributorToken(orgID, secret)
	req.Header.Set("X-Contributor-Token", token)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("registry returned status %d", resp.StatusCode)
	}

	return nil
}

// computeStats calculates statistical summary for a slice of float64 values
func computeStats(values []float64) domain.FactorStats {
	if len(values) == 0 {
		return domain.FactorStats{}
	}

	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	sum := 0.0
	min := math.MaxFloat64
	max := -math.MaxFloat64
	for _, v := range sorted {
		sum += v
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}

	n := len(sorted)
	return domain.FactorStats{
		Min:    min,
		Max:    max,
		Mean:   sum / float64(n),
		Median: sorted[n/2],
		P25:    sorted[n/4],
		P75:    sorted[(3*n)/4],
	}
}

func structToMap(v interface{}) (map[string]interface{}, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	err = json.Unmarshal(data, &m)
	return m, err
}
