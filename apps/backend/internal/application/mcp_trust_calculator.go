package application

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// MCPTrustCalculator implements domain.MCPTrustScoreCalculator
// Implements 8-factor trust scoring algorithm for MCP servers
//
// Factor weights (total 100%):
//   - Attestation Consensus (25%): Multi-agent verification strength
//   - Connection Health (15%): Uptime and response times
//   - Capability Stability (15%): Schema changes, deprecations
//   - Security Posture (15%): TLS, authentication, known vulns
//   - Organization Compliance (10%): Meets policy requirements
//   - Age & History (10%): Operating duration without issues
//   - Usage Patterns (5%): Normal vs anomalous usage
//   - User Feedback (5%): Admin ratings and notes
type MCPTrustCalculator struct {
	mcpServerRepo       domain.MCPServerRepository
	attestationRepo     domain.MCPAttestationRepository
	capabilityRepo      domain.MCPServerCapabilityRepository
	alertRepo           domain.AlertRepository
	mcpTrustScoreRepo   domain.MCPTrustScoreRepository
}

// NewMCPTrustCalculator creates a new MCP trust calculator
func NewMCPTrustCalculator(
	mcpServerRepo domain.MCPServerRepository,
	attestationRepo domain.MCPAttestationRepository,
	capabilityRepo domain.MCPServerCapabilityRepository,
	alertRepo domain.AlertRepository,
) *MCPTrustCalculator {
	return &MCPTrustCalculator{
		mcpServerRepo:   mcpServerRepo,
		attestationRepo: attestationRepo,
		capabilityRepo:  capabilityRepo,
		alertRepo:       alertRepo,
	}
}

// NewMCPTrustCalculatorWithRepo creates a new MCP trust calculator with trust score repository
func NewMCPTrustCalculatorWithRepo(
	mcpServerRepo domain.MCPServerRepository,
	attestationRepo domain.MCPAttestationRepository,
	capabilityRepo domain.MCPServerCapabilityRepository,
	alertRepo domain.AlertRepository,
	mcpTrustScoreRepo domain.MCPTrustScoreRepository,
) *MCPTrustCalculator {
	return &MCPTrustCalculator{
		mcpServerRepo:     mcpServerRepo,
		attestationRepo:   attestationRepo,
		capabilityRepo:    capabilityRepo,
		alertRepo:         alertRepo,
		mcpTrustScoreRepo: mcpTrustScoreRepo,
	}
}

// Factor weights for MCP trust score calculation
var mcpFactorWeights = map[string]float64{
	"attestation_consensus":    0.25, // Factor 1 - 25%
	"connection_health":        0.15, // Factor 2 - 15%
	"capability_stability":     0.15, // Factor 3 - 15%
	"security_posture":         0.15, // Factor 4 - 15%
	"organization_compliance":  0.10, // Factor 5 - 10%
	"age":                      0.10, // Factor 6 - 10%
	"usage_patterns":           0.05, // Factor 7 - 5%
	"user_feedback":            0.05, // Factor 8 - 5%
}

// Calculate calculates trust score for an MCP server
// Returns the weighted average of all 8 factors
func (c *MCPTrustCalculator) Calculate(server *domain.MCPServer) (*domain.MCPTrustScore, error) {
	factors, err := c.CalculateFactors(server)
	if err != nil {
		return nil, err
	}

	// 8-factor weighted average (totaling 100%)
	score := factors.AttestationConsensus*mcpFactorWeights["attestation_consensus"] +
		factors.ConnectionHealth*mcpFactorWeights["connection_health"] +
		factors.CapabilityStability*mcpFactorWeights["capability_stability"] +
		factors.SecurityPosture*mcpFactorWeights["security_posture"] +
		factors.OrganizationCompliance*mcpFactorWeights["organization_compliance"] +
		factors.Age*mcpFactorWeights["age"] +
		factors.UsagePatterns*mcpFactorWeights["usage_patterns"] +
		factors.UserFeedback*mcpFactorWeights["user_feedback"]

	// Ensure score is within bounds [0, 1]
	score = math.Max(0.0, math.Min(1.0, score))

	// Calculate confidence based on available data
	confidence := c.calculateConfidence(server, factors)

	return &domain.MCPTrustScore{
		ID:             uuid.New(),
		MCPServerID:    server.ID,
		Score:          score,
		Factors:        *factors,
		Confidence:     confidence,
		LastCalculated: time.Now(),
		CreatedAt:      time.Now(),
	}, nil
}

// CalculateFactors calculates individual trust factors for an MCP server
func (c *MCPTrustCalculator) CalculateFactors(server *domain.MCPServer) (*domain.MCPTrustScoreFactors, error) {
	factors := &domain.MCPTrustScoreFactors{}

	// Factor 1: Attestation Consensus (25% weight)
	factors.AttestationConsensus = c.calculateAttestationConsensus(server)

	// Factor 2: Connection Health (15% weight)
	factors.ConnectionHealth = c.calculateConnectionHealth(server)

	// Factor 3: Capability Stability (15% weight)
	factors.CapabilityStability = c.calculateCapabilityStability(server)

	// Factor 4: Security Posture (15% weight)
	factors.SecurityPosture = c.calculateSecurityPosture(server)

	// Factor 5: Organization Compliance (10% weight)
	factors.OrganizationCompliance = c.calculateOrganizationCompliance(server)

	// Factor 6: Age & History (10% weight)
	factors.Age = c.calculateAge(server)

	// Factor 7: Usage Patterns (5% weight)
	factors.UsagePatterns = c.calculateUsagePatterns(server)

	// Factor 8: User Feedback (5% weight)
	factors.UserFeedback = c.calculateUserFeedback(server)

	return factors, nil
}

// Factor 1: Attestation Consensus (25% weight)
// Measures multi-agent verification strength
// Requires 3+ unique agents and 2+ unique owners for full score
func (c *MCPTrustCalculator) calculateAttestationConsensus(server *domain.MCPServer) float64 {
	// Use existing attestation data from server
	// This is populated via joins when fetching the server
	attestationCount := server.AttestationCount
	confidenceScore := server.ConfidenceScore

	// Thresholds for full verification
	const requiredAgents = 3    // Need 3+ unique agents
	const requiredOwners = 2    // Need 2+ unique owners
	const requiredConfidence = 60.0 // Need 60%+ confidence score

	// If we have the attestation repository, get more accurate counts
	if c.attestationRepo != nil {
		// Get unique agent count using type assertion
		// The MCPAttestationRepository concrete implementation has additional methods
		if attestationRepo, ok := c.attestationRepo.(attestationRepoWithCounts); ok {
			uniqueAgents, err := attestationRepo.GetUniqueAttestingAgentCount(server.ID)
			if err == nil {
				attestationCount = uniqueAgents
			}
		}
	}

	// Calculate score components
	agentScore := math.Min(1.0, float64(attestationCount)/float64(requiredAgents))
	confidenceScoreNorm := math.Min(1.0, confidenceScore/requiredConfidence)

	// Combine: agent count matters more (60%) than confidence (40%)
	score := agentScore*0.6 + confidenceScoreNorm*0.4

	// Apply verification status modifier
	switch server.Status {
	case domain.MCPServerStatusVerified:
		// Keep full score
	case domain.MCPServerStatusPending:
		score *= 0.5 // Reduce pending servers
	case domain.MCPServerStatusSuspended:
		score *= 0.2 // Heavily penalize suspended
	case domain.MCPServerStatusRevoked:
		score = 0.0 // Revoked servers get zero
	}

	return score
}

// Factor 2: Connection Health (15% weight)
// Measures uptime and response times based on attestation health checks
func (c *MCPTrustCalculator) calculateConnectionHealth(server *domain.MCPServer) float64 {
	// Use attestation data to measure connection health
	// Attestations include health_check_passed and connection_latency_ms

	if c.attestationRepo == nil {
		// Fallback: use server status as proxy
		if server.IsVerified {
			return 0.95
		}
		return 0.70
	}

	// Get valid attestations to analyze health check data
	attestations, err := c.attestationRepo.GetValidAttestationsByMCP(server.ID)
	if err != nil || len(attestations) == 0 {
		// No attestation data - return baseline
		if server.IsVerified {
			return 0.90
		}
		return 0.60
	}

	// Calculate health check pass rate and average latency
	healthPassed := 0
	totalLatency := 0.0
	connectionSuccesses := 0

	for _, attestation := range attestations {
		if attestation.AttestationData.HealthCheckPassed {
			healthPassed++
		}
		if attestation.AttestationData.ConnectionSuccessful {
			connectionSuccesses++
		}
		totalLatency += attestation.AttestationData.ConnectionLatencyMs
	}

	total := len(attestations)
	healthRate := float64(healthPassed) / float64(total)
	connectionRate := float64(connectionSuccesses) / float64(total)
	avgLatency := totalLatency / float64(total)

	// Score components:
	// - Health check pass rate (40%)
	// - Connection success rate (40%)
	// - Latency score (20%) - lower is better
	latencyScore := 1.0
	if avgLatency > 0 {
		// 100ms = perfect, 500ms = 0.5, 1000ms = 0.1
		latencyScore = math.Max(0.1, 1.0-(avgLatency/1000.0))
	}

	return healthRate*0.4 + connectionRate*0.4 + latencyScore*0.2
}

// Factor 3: Capability Stability (15% weight)
// Measures how stable the MCP's capabilities are over time
func (c *MCPTrustCalculator) calculateCapabilityStability(server *domain.MCPServer) float64 {
	// For now, use a simplified approach based on capability count
	// A server with defined capabilities is more stable than one without
	capCount := len(server.Capabilities)

	if capCount == 0 {
		// No capabilities defined - lower trust
		return 0.50
	}

	// Check detailed capabilities if available
	if c.capabilityRepo != nil {
		capabilities, err := c.capabilityRepo.GetByServerID(server.ID)
		if err == nil && len(capabilities) > 0 {
			// Count active capabilities
			activeCount := 0
			verifiedCount := 0
			for _, cap := range capabilities {
				if cap.IsActive {
					activeCount++
				}
				if cap.LastVerifiedAt != nil {
					verifiedCount++
				}
			}

			// More active and verified capabilities = higher stability
			activeRate := float64(activeCount) / float64(len(capabilities))
			verifiedRate := float64(verifiedCount) / float64(len(capabilities))

			return activeRate*0.6 + verifiedRate*0.4
		}
	}

	// Fallback: more capabilities = more mature/stable server
	// Cap at 10 capabilities for full score
	return math.Min(1.0, float64(capCount)/10.0)
}

// Factor 4: Security Posture (15% weight)
// Measures TLS, authentication, and known vulnerabilities
func (c *MCPTrustCalculator) calculateSecurityPosture(server *domain.MCPServer) float64 {
	score := 0.0
	maxScore := 4.0 // 4 security criteria

	// 1. TLS/HTTPS (1.0 point)
	if strings.HasPrefix(server.URL, "https://") {
		score += 1.0
	} else if strings.HasPrefix(server.URL, "http://localhost") ||
		strings.HasPrefix(server.URL, "http://127.0.0.1") {
		// Local servers get partial credit
		score += 0.5
	}

	// 2. Has public key for verification (1.0 point)
	if server.PublicKey != "" {
		score += 1.0
	}

	// 3. Has been verified (1.0 point)
	if server.IsVerified {
		score += 1.0
	}

	// 4. No security alerts (1.0 point)
	if c.alertRepo != nil {
		alerts, err := c.alertRepo.GetUnacknowledgedByResourceID(server.ID)
		if err == nil {
			// Check for security-related alerts
			securityAlerts := 0
			for _, alert := range alerts {
				if alert.Severity == domain.AlertSeverityCritical ||
					alert.Severity == domain.AlertSeverityHigh {
					securityAlerts++
				}
			}
			if securityAlerts == 0 {
				score += 1.0
			} else if securityAlerts <= 2 {
				score += 0.5
			}
			// More than 2 security alerts = 0 points
		} else {
			// If we can't check alerts, give partial credit
			score += 0.75
		}
	} else {
		// No alert repo - give baseline
		score += 0.75
	}

	return score / maxScore
}

// Factor 5: Organization Compliance (10% weight)
// Measures whether MCP meets organizational policy requirements
func (c *MCPTrustCalculator) calculateOrganizationCompliance(server *domain.MCPServer) float64 {
	// For MVP: Check basic compliance indicators
	// Full implementation would check against security policies

	score := 0.0
	maxScore := 3.0

	// 1. Has valid status
	if server.Status == domain.MCPServerStatusVerified ||
		server.Status == domain.MCPServerStatusPending {
		score += 1.0
	}

	// 2. Has creator audit trail
	if server.CreatedBy != uuid.Nil {
		score += 1.0
	}

	// 3. Has proper description/documentation
	if server.Description != "" {
		score += 1.0
	}

	return score / maxScore
}

// Factor 6: Age & History (10% weight)
// Measures how long the MCP server has been operating without issues
func (c *MCPTrustCalculator) calculateAge(server *domain.MCPServer) float64 {
	// Age scoring from documentation:
	// < 7 days: 0.30
	// 7-30 days: 0.50
	// 30-90 days: 0.75
	// 90+ days: 1.00
	daysSinceCreation := time.Since(server.CreatedAt).Hours() / 24

	if daysSinceCreation < 7 {
		return 0.30
	} else if daysSinceCreation < 30 {
		return 0.50
	} else if daysSinceCreation < 90 {
		return 0.75
	}
	return 1.0
}

// Factor 7: Usage Patterns (5% weight)
// Measures normal vs anomalous usage patterns
func (c *MCPTrustCalculator) calculateUsagePatterns(server *domain.MCPServer) float64 {
	// Check connection count as proxy for usage
	connectedAgents := server.ConnectedAgentsCount

	if connectedAgents == 0 {
		// No agents connected - lower score but not zero
		return 0.50
	}

	// More connected agents = more trust (up to a point)
	// 1 agent: 0.60
	// 2-3 agents: 0.75
	// 4-5 agents: 0.90
	// 6+ agents: 1.00
	if connectedAgents == 1 {
		return 0.60
	} else if connectedAgents <= 3 {
		return 0.75
	} else if connectedAgents <= 5 {
		return 0.90
	}
	return 1.0
}

// Factor 8: User Feedback (5% weight)
// Measures explicit admin feedback and ratings
func (c *MCPTrustCalculator) calculateUserFeedback(server *domain.MCPServer) float64 {
	// For MVP: Return baseline score
	// Future: Query user feedback/ratings table
	return 0.75
}

// calculateConfidence determines how confident we are in the score
// Based on available data points
func (c *MCPTrustCalculator) calculateConfidence(server *domain.MCPServer, factors *domain.MCPTrustScoreFactors) float64 {
	dataPoints := 0.0
	total := 8.0 // 8 factors

	// Base data points from server properties
	if server.Status != "" {
		dataPoints++
	}
	if server.PublicKey != "" {
		dataPoints++
	}
	if server.URL != "" {
		dataPoints++
	}

	// Attestation data provides multiple data points
	if server.AttestationCount > 0 {
		dataPoints += 2 // Covers attestation and connection health
		if server.AttestationCount >= 3 {
			dataPoints += 0.5 // Multiple attestations = more confidence
		}
	}

	// Has capabilities defined
	if len(server.Capabilities) > 0 {
		dataPoints++
	}

	// Has age (always true, but older = more confidence)
	if time.Since(server.CreatedAt).Hours() > 24*7 {
		dataPoints++
	}

	// Has connections
	if server.ConnectedAgentsCount > 0 {
		dataPoints++
	}

	confidence := dataPoints / total
	return math.Min(1.0, confidence)
}

// CalculateTrustScore calculates and optionally stores trust score for an MCP server
func (c *MCPTrustCalculator) CalculateTrustScore(ctx context.Context, mcpServerID uuid.UUID) (*domain.MCPTrustScore, error) {
	// Fetch the MCP server
	server, err := c.mcpServerRepo.GetByID(mcpServerID)
	if err != nil {
		return nil, fmt.Errorf("failed to get MCP server: %w", err)
	}

	// Calculate trust score
	score, err := c.Calculate(server)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate trust score: %w", err)
	}

	// Store the score if repository is available.
	//
	// This insert is also what updates the `mcp_servers.trust_score` cache: the
	// migration 094 trigger `mirror_mcp_trust_score_to_server` fires AFTER INSERT
	// on `mcp_trust_scores` and mirrors NEW.score onto the server row, in the same
	// transaction. There is deliberately no second write here.
	//
	// There used to be one — `server.TrustScore = score.Score` followed by
	// `mcpServerRepo.Update(server)`. It was redundant once 094 landed, and it was
	// the reason `Update` carried `trust_score` in its SET list at all, which made
	// every unrelated caller of `Update` a silent writer of the cache. See
	// `MCPServerRepository.Update` for why that column is now absent there.
	//
	// When `mcpTrustScoreRepo` is nil this function calculates without persisting
	// anything, which is what "optionally stores" means: no score row, and
	// therefore no cache update. Production always wires the repository
	// (`NewMCPTrustCalculatorWithRepo` in `cmd/server/main.go`); the nil-repo
	// constructor has no non-test callers.
	if c.mcpTrustScoreRepo != nil {
		if err := c.mcpTrustScoreRepo.Create(score); err != nil {
			return nil, fmt.Errorf("failed to store trust score: %w", err)
		}
	}

	return score, nil
}

// GetLatestTrustScore retrieves the latest trust score for an MCP server
func (c *MCPTrustCalculator) GetLatestTrustScore(ctx context.Context, mcpServerID uuid.UUID) (*domain.MCPTrustScore, error) {
	if c.mcpTrustScoreRepo == nil {
		return nil, fmt.Errorf("trust score repository not configured")
	}
	return c.mcpTrustScoreRepo.GetLatest(mcpServerID)
}

// GetTrustScoreHistory retrieves trust score history for an MCP server
func (c *MCPTrustCalculator) GetTrustScoreHistory(ctx context.Context, mcpServerID uuid.UUID, limit int) ([]*domain.MCPTrustScore, error) {
	if c.mcpTrustScoreRepo == nil {
		return nil, fmt.Errorf("trust score repository not configured")
	}
	return c.mcpTrustScoreRepo.GetHistory(mcpServerID, limit)
}

// Interface for attestation repo with count methods (type assertion helper)
type attestationRepoWithCounts interface {
	domain.MCPAttestationRepository
	GetUniqueAttestingAgentCount(mcpServerID uuid.UUID) (int, error)
	GetUniqueAgentOwnerCount(mcpServerID uuid.UUID) (int, error)
}
