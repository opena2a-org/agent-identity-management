package application

import (
	"context"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
)

// TrustCalculator implements domain.TrustScoreCalculator
// Implements 8-factor trust scoring algorithm (see documentation)
type TrustCalculator struct {
	trustScoreRepo   domain.TrustScoreRepository
	apiKeyRepo       domain.APIKeyRepository
	auditRepo        domain.AuditLogRepository
	capabilityRepo   domain.CapabilityRepository
	agentRepo        domain.AgentRepository
	alertRepo        domain.AlertRepository
}

// NewTrustCalculator creates a new trust calculator
func NewTrustCalculator(
	trustScoreRepo domain.TrustScoreRepository,
	apiKeyRepo domain.APIKeyRepository,
	auditRepo domain.AuditLogRepository,
	capabilityRepo domain.CapabilityRepository,
	agentRepo domain.AgentRepository,
	alertRepo domain.AlertRepository,
) *TrustCalculator {
	return &TrustCalculator{
		trustScoreRepo:   trustScoreRepo,
		apiKeyRepo:       apiKeyRepo,
		auditRepo:        auditRepo,
		capabilityRepo:   capabilityRepo,
		agentRepo:        agentRepo,
		alertRepo:        alertRepo,
	}
}

// Calculate calculates trust score for an agent
// Implements the 8-factor algorithm with weighted average
func (c *TrustCalculator) Calculate(agent *domain.Agent) (*domain.TrustScore, error) {
	factors, err := c.CalculateFactors(agent)
	if err != nil {
		return nil, err
	}

	// 8-factor weighted average (totaling 100%)
	// Formula from documentation:
	// Trust Score =
	//     (0.25 × Verification Status) +
	//     (0.15 × Uptime & Availability) +
	//     (0.15 × Action Success Rate) +
	//     (0.15 × Security Alerts) +
	//     (0.10 × Compliance Score) +
	//     (0.10 × Age & History) +
	//     (0.05 × Drift Detection) +
	//     (0.05 × User Feedback)
	weights := map[string]float64{
		"verification":    0.25, // Factor 1
		"uptime":          0.15, // Factor 2
		"success_rate":    0.15, // Factor 3
		"security_alerts": 0.15, // Factor 4
		"compliance":      0.10, // Factor 5
		"age":             0.10, // Factor 6
		"drift_detection": 0.05, // Factor 7
		"user_feedback":   0.05, // Factor 8
	}

	score := factors.VerificationStatus*weights["verification"] +
		factors.Uptime*weights["uptime"] +
		factors.SuccessRate*weights["success_rate"] +
		factors.SecurityAlerts*weights["security_alerts"] +
		factors.Compliance*weights["compliance"] +
		factors.Age*weights["age"] +
		factors.DriftDetection*weights["drift_detection"] +
		factors.UserFeedback*weights["user_feedback"]

	// Ensure score is within bounds [0, 1]
	score = math.Max(0.0, math.Min(1.0, score))

	// Calculate confidence based on available data
	confidence := c.calculateConfidence(agent, factors)

	return &domain.TrustScore{
		ID:             uuid.New(),
		AgentID:        agent.ID,
		Score:          score,
		Factors:        *factors,
		Confidence:     confidence,
		LastCalculated: time.Now(),
		CreatedAt:      time.Now(),
	}, nil
}

// CalculateFactors calculates individual trust factors
func (c *TrustCalculator) CalculateFactors(agent *domain.Agent) (*domain.TrustScoreFactors, error) {
	factors := &domain.TrustScoreFactors{}

	// Factor 1: Verification Status (25% weight)
	// Ed25519 signature verification for all actions
	factors.VerificationStatus = c.calculateVerificationStatus(agent)

	// Factor 2: Uptime & Availability (15% weight)
	// Health check responsiveness over time
	factors.Uptime = c.calculateUptime(agent)

	// Factor 3: Action Success Rate (15% weight)
	// Percentage of actions that complete successfully
	factors.SuccessRate = c.calculateSuccessRate(agent)

	// Factor 4: Security Alerts (15% weight)
	// Active security alerts by severity
	factors.SecurityAlerts = c.calculateSecurityAlerts(agent)

	// Factor 5: Compliance Score (10% weight)
	// SOC 2, HIPAA, GDPR adherence
	factors.Compliance = c.calculateCompliance(agent)

	// Factor 6: Age & History (10% weight)
	// How long agent has been operating successfully
	factors.Age = c.calculateAge(agent)

	// Factor 7: Drift Detection (5% weight)
	// Behavioral pattern changes
	factors.DriftDetection = c.calculateDriftDetection(agent)

	// Factor 8: User Feedback (5% weight)
	// Explicit user ratings
	factors.UserFeedback = c.calculateUserFeedback(agent)

	return factors, nil
}

// Factor 1: Verification Status (25% weight)
// Measures percentage of actions successfully verified with Ed25519 signatures
func (c *TrustCalculator) calculateVerificationStatus(agent *domain.Agent) float64 {
	// TODO: Query agent_actions table for verification statistics
	// For MVP: Use agent verification status as proxy
	switch agent.Status {
	case domain.AgentStatusVerified:
		return 1.0
	case domain.AgentStatusPending:
		return 0.3
	case domain.AgentStatusSuspended:
		return 0.1
	case domain.AgentStatusRevoked:
		return 0.0
	default:
		return 0.3
	}
}

// Factor 2: Uptime & Availability (15% weight)
// Measures how often agent responds to health checks
func (c *TrustCalculator) calculateUptime(agent *domain.Agent) float64 {
	// TODO: Query agent_health_checks table
	// Calculate: successful_health_checks / total_health_checks
	// For MVP: Return baseline based on agent status
	if agent.Status == domain.AgentStatusVerified {
		return 0.98 // Assume 98% uptime for verified agents
	} else if agent.Status == domain.AgentStatusPending {
		return 0.75 // Lower baseline for pending agents
	}
	return 0.50
}

// Factor 3: Action Success Rate (15% weight)
// Measures percentage of actions that complete successfully
func (c *TrustCalculator) calculateSuccessRate(agent *domain.Agent) float64 {
	// TODO: Query agent_actions table
	// Calculate: successful_actions / total_actions
	// For MVP: Return baseline score
	return 0.95 // Assume 95% success rate
}

// Factor 4: Security Alerts (15% weight)
// Measures active security alerts by severity
func (c *TrustCalculator) calculateSecurityAlerts(agent *domain.Agent) float64 {
	// TODO: Query alerts table for agent-specific alerts
	// Implementation from documentation:
	// - Critical alerts: score = 0.0
	// - High alerts: score = 0.50
	// - Medium alerts: score = 0.75
	// - Low/no alerts: score = 1.0

	// For MVP: Check capability violations as proxy for security alerts
	violations, _, err := c.capabilityRepo.GetViolationsByAgentID(agent.ID, 100, 0)
	if err != nil || len(violations) == 0 {
		return 1.0 // No violations = perfect security score
	}

	// Count violations by severity in last 30 days
	thirtyDaysAgo := time.Now().AddDate(0, 0, -30)
	criticalCount := 0
	highCount := 0
	mediumCount := 0

	for _, v := range violations {
		if v.CreatedAt.After(thirtyDaysAgo) {
			switch v.Severity {
			case domain.ViolationSeverityCritical:
				criticalCount++
			case domain.ViolationSeverityHigh:
				highCount++
			case domain.ViolationSeverityMedium:
				mediumCount++
			}
		}
	}

	// Apply scoring logic from documentation
	if criticalCount > 0 {
		return 0.0
	} else if highCount > 0 {
		return 0.50
	} else if mediumCount > 0 {
		return 0.75
	}
	return 1.0
}

// Factor 5: Compliance Score (10% weight)
// Measures adherence to compliance policies (SOC 2, HIPAA, GDPR)
func (c *TrustCalculator) calculateCompliance(agent *domain.Agent) float64 {
	// TODO: Query agent_compliance_events table
	// Calculate: compliant_actions / total_actions_requiring_compliance
	// For MVP: Return baseline score
	return 1.0 // Assume full compliance for MVP
}

// Factor 6: Age & History (10% weight)
// Measures how long agent has been operating successfully
func (c *TrustCalculator) calculateAge(agent *domain.Agent) float64 {
	// Implementation from documentation:
	// < 7 days: 0.30
	// 7-30 days: 0.50
	// 30-90 days: 0.75
	// 90+ days: 1.00
	daysSinceCreation := time.Since(agent.CreatedAt).Hours() / 24

	if daysSinceCreation < 7 {
		return 0.30
	} else if daysSinceCreation < 30 {
		return 0.50
	} else if daysSinceCreation < 90 {
		return 0.75
	}
	return 1.0
}

// Factor 7: Drift Detection (5% weight)
// Measures changes in agent behavior patterns
func (c *TrustCalculator) calculateDriftDetection(agent *domain.Agent) float64 {
	// TODO: Query agent_behavioral_baselines table
	// Check if is_anomaly = true for recent records
	// For MVP: Return baseline (no drift detected)
	return 1.0
}

// Factor 8: User Feedback (5% weight)
// Measures explicit feedback from users
func (c *TrustCalculator) calculateUserFeedback(agent *domain.Agent) float64 {
	// TODO: Query agent_user_feedback table
	// Implementation from documentation:
	// - negative_feedback > 5: 0.0
	// - negative_feedback > 2: 0.50
	// - positive_feedback > 10: 1.0
	// - else: 0.75
	// For MVP: Return baseline score
	return 0.75
}

// calculateConfidence determines confidence level based on available data
func (c *TrustCalculator) calculateConfidence(agent *domain.Agent, factors *domain.TrustScoreFactors) float64 {
	// Count available data points
	dataPoints := 0.0
	total := 8.0 // 8 factors

	// Each factor that's not at baseline counts as a data point
	// For MVP, we have limited data, so confidence will be lower
	if agent.Status != "" {
		dataPoints++
	}
	if agent.PublicKey != nil && *agent.PublicKey != "" {
		dataPoints++
	}
	if agent.CreatedAt.Before(time.Now().AddDate(0, -1, 0)) {
		dataPoints++ // Agent has some history
	}

	// TODO: Increment dataPoints when we have actual operational metrics

	return dataPoints / total
}

// CalculateTrustScore calculates and stores trust score for an agent
func (c *TrustCalculator) CalculateTrustScore(ctx context.Context, agentID uuid.UUID) (*domain.TrustScore, error) {
	// Fetch the agent
	agent, err := c.agentRepo.GetByID(agentID)
	if err != nil {
		return nil, err
	}

	// Calculate trust score
	score, err := c.Calculate(agent)
	if err != nil {
		return nil, err
	}

	// Store the score
	if err := c.trustScoreRepo.Create(score); err != nil {
		return nil, err
	}

	return score, nil
}

// GetLatestTrustScore retrieves the latest trust score for an agent
func (c *TrustCalculator) GetLatestTrustScore(ctx context.Context, agentID uuid.UUID) (*domain.TrustScore, error) {
	return c.trustScoreRepo.GetLatest(agentID)
}

// GetTrustScoreHistory retrieves trust score history for an agent
func (c *TrustCalculator) GetTrustScoreHistory(ctx context.Context, agentID uuid.UUID, limit int) ([]*domain.TrustScore, error) {
	return c.trustScoreRepo.GetHistory(agentID, limit)
}
