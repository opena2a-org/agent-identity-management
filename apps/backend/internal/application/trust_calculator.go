package application

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// TrustCalculator implements domain.TrustScoreCalculator
// Implements 9-factor trust scoring algorithm (see documentation)
type TrustCalculator struct {
	trustScoreRepo        domain.TrustScoreRepository
	apiKeyRepo            domain.APIKeyRepository
	auditRepo             domain.AuditLogRepository
	capabilityRepo        domain.CapabilityRepository
	agentRepo             domain.AgentRepository
	alertRepo             domain.AlertRepository
	verificationEventRepo domain.VerificationEventRepository
	snapshotRepo          domain.ComplianceSnapshotRepository
	isolationRepo         domain.IsolationAttestationRepository
	userFeedbackRepo      domain.UserFeedbackRepository
	tmeProvider           NanoMindTMEProvider
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
		trustScoreRepo: trustScoreRepo,
		apiKeyRepo:     apiKeyRepo,
		auditRepo:      auditRepo,
		capabilityRepo: capabilityRepo,
		agentRepo:      agentRepo,
		alertRepo:      alertRepo,
	}
}

// NewTrustCalculatorWithVerification creates a new trust calculator with verification event repo
func NewTrustCalculatorWithVerification(
	trustScoreRepo domain.TrustScoreRepository,
	apiKeyRepo domain.APIKeyRepository,
	auditRepo domain.AuditLogRepository,
	capabilityRepo domain.CapabilityRepository,
	agentRepo domain.AgentRepository,
	alertRepo domain.AlertRepository,
	verificationEventRepo domain.VerificationEventRepository,
) *TrustCalculator {
	return &TrustCalculator{
		trustScoreRepo:        trustScoreRepo,
		apiKeyRepo:            apiKeyRepo,
		auditRepo:             auditRepo,
		capabilityRepo:        capabilityRepo,
		agentRepo:             agentRepo,
		alertRepo:             alertRepo,
		verificationEventRepo: verificationEventRepo,
	}
}

// NanoMindTMEProvider supplies Threat Model Evaluation scores from NanoMind.
// TME scores modify the security alerts factor as a behavioral signal.
type NanoMindTMEProvider interface {
	// GetLatestTMEScore returns the latest TME score for an agent (0-1, higher = safer).
	// Returns -1 if no evaluation exists.
	GetLatestTMEScore(agentID uuid.UUID) (float64, error)
}

// SetSnapshotRepo sets the compliance snapshot repository for compliance factor calculation.
// This is optional; when not set, the compliance factor returns a neutral 0.5 score.
func (c *TrustCalculator) SetSnapshotRepo(repo domain.ComplianceSnapshotRepository) {
	c.snapshotRepo = repo
}

// SetIsolationRepo sets the isolation attestation repository for execution isolation factor.
// Optional; when not set, the execution isolation factor returns a neutral 0.3 score.
func (c *TrustCalculator) SetIsolationRepo(repo domain.IsolationAttestationRepository) {
	c.isolationRepo = repo
}

// SetUserFeedbackRepo sets the user feedback repository for the user feedback factor.
// Optional; when not set, the user feedback factor returns a neutral 0.5 score.
func (c *TrustCalculator) SetUserFeedbackRepo(repo domain.UserFeedbackRepository) {
	c.userFeedbackRepo = repo
}

// SetTMEProvider sets the NanoMind TME provider for behavioral trust enrichment.
// Optional; when not set, TME has no effect on scoring.
func (c *TrustCalculator) SetTMEProvider(provider NanoMindTMEProvider) {
	c.tmeProvider = provider
}

// Calculate calculates trust score for an agent
// Implements the 9-factor algorithm with weighted average
func (c *TrustCalculator) Calculate(agent *domain.Agent) (*domain.TrustScore, error) {
	factors, err := c.CalculateFactors(agent)
	if err != nil {
		return nil, err
	}

	// 9-factor weighted average (totaling 100%)
	// Formula:
	// Trust Score =
	//     (0.25 × Verification Status) +
	//     (0.15 × Uptime & Availability) +
	//     (0.15 × Action Success Rate) +
	//     (0.15 × Security Alerts) +
	//     (0.10 × Compliance Score) +
	//     (0.05 × Age & History) +
	//     (0.03 × Drift Detection) +
	//     (0.02 × User Feedback) +
	//     (0.10 × Execution Isolation)
	//
	// Weight rebalance: Age reduced from 10% to 5%, Drift from 5% to 3%,
	// User Feedback from 5% to 2% to make room for 10% Execution Isolation.
	weights := map[string]float64{
		"verification":        0.25, // Factor 1
		"uptime":              0.15, // Factor 2
		"success_rate":        0.15, // Factor 3
		"security_alerts":     0.15, // Factor 4
		"compliance":          0.10, // Factor 5
		"age":                 0.05, // Factor 6 (reduced from 0.10)
		"drift_detection":     0.03, // Factor 7 (reduced from 0.05)
		"user_feedback":       0.02, // Factor 8 (reduced from 0.05)
		"execution_isolation": 0.10, // Factor 9 (new)
	}

	score := factors.VerificationStatus*weights["verification"] +
		factors.Uptime*weights["uptime"] +
		factors.SuccessRate*weights["success_rate"] +
		factors.SecurityAlerts*weights["security_alerts"] +
		factors.Compliance*weights["compliance"] +
		factors.Age*weights["age"] +
		factors.DriftDetection*weights["drift_detection"] +
		factors.UserFeedback*weights["user_feedback"] +
		factors.ExecutionIsolation*weights["execution_isolation"]

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

	// Factor 6: Age & History (5% weight)
	// How long agent has been operating successfully
	factors.Age = c.calculateAge(agent)

	// Factor 7: Drift Detection (3% weight)
	// Behavioral pattern changes
	factors.DriftDetection = c.calculateDriftDetection(agent)

	// Factor 8: User Feedback (2% weight)
	// Explicit user ratings
	factors.UserFeedback = c.calculateUserFeedback(agent)

	// Factor 9: Execution Isolation (10% weight)
	// Runtime isolation posture (sandbox, network, filesystem, process)
	factors.ExecutionIsolation = c.calculateExecutionIsolation(agent)

	// NanoMind TME enrichment: adjust security alerts factor based on threat model evaluation
	if c.tmeProvider != nil {
		tmeScore, err := c.tmeProvider.GetLatestTMEScore(agent.ID)
		if err == nil && tmeScore >= 0 {
			// TME score (0-1, higher = safer) blends into security alerts factor
			// Weight: 30% TME influence on the security alerts component
			factors.SecurityAlerts = factors.SecurityAlerts*0.7 + tmeScore*0.3
			if factors.SecurityAlerts > 1.0 {
				factors.SecurityAlerts = 1.0
			}
		}
	}

	return factors, nil
}

// Factor 1: Verification Status (25% weight)
// Measures percentage of actions successfully verified with Ed25519 signatures
func (c *TrustCalculator) calculateVerificationStatus(agent *domain.Agent) float64 {
	// Try to query real verification statistics from verification_events table
	if c.verificationEventRepo != nil {
		endTime := time.Now()
		startTime := endTime.AddDate(0, 0, -30) // Last 30 days

		stats, err := c.verificationEventRepo.GetAgentStatistics(agent.ID, startTime, endTime)
		if err == nil && stats.TotalVerifications > 0 {
			// Use real success rate from verification events
			// Blend with agent status for a more nuanced score
			verificationScore := stats.SuccessRate

			// Apply status modifier
			statusModifier := 1.0
			switch agent.Status {
			case domain.AgentStatusVerified:
				statusModifier = 1.0
			case domain.AgentStatusPending:
				statusModifier = 0.7
			case domain.AgentStatusSuspended:
				statusModifier = 0.3
			case domain.AgentStatusRevoked:
				statusModifier = 0.0
			}

			return verificationScore * statusModifier
		}
	}

	// Fallback: Use agent verification status as proxy
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
	// Try to calculate uptime from verification event response times
	if c.verificationEventRepo != nil {
		endTime := time.Now()
		startTime := endTime.AddDate(0, 0, -30) // Last 30 days

		stats, err := c.verificationEventRepo.GetAgentStatistics(agent.ID, startTime, endTime)
		if err == nil && stats.TotalVerifications > 0 {
			// Use verification success rate as proxy for availability
			// If agent is responding to verifications, it's available
			uptime := stats.SuccessRate

			// Boost score if there are recent verifications (agent is active)
			if time.Since(stats.LastVerification) < 24*time.Hour {
				uptime = math.Min(1.0, uptime+0.1)
			} else if time.Since(stats.LastVerification) > 7*24*time.Hour {
				// Penalize if no recent activity
				uptime = uptime * 0.8
			}

			return uptime
		}
	}

	// Fallback: Return baseline based on agent status
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
	// Query verification events for action success rate
	if c.verificationEventRepo != nil {
		endTime := time.Now()
		startTime := endTime.AddDate(0, 0, -30) // Last 30 days

		stats, err := c.verificationEventRepo.GetAgentStatistics(agent.ID, startTime, endTime)
		if err == nil && stats.TotalVerifications > 0 {
			// Return actual success rate from verification events
			return stats.SuccessRate
		}
	}

	// Fallback: Return baseline score based on status
	switch agent.Status {
	case domain.AgentStatusVerified:
		return 0.95
	case domain.AgentStatusPending:
		return 0.80
	default:
		return 0.70
	}
}

// Factor 4: Security Alerts (15% weight)
// Measures security posture based on violations with CUMULATIVE impact
// Each violation reduces score; blocked violations have 50% impact
func (c *TrustCalculator) calculateSecurityAlerts(agent *domain.Agent) float64 {
	score := 1.0

	// Check active security alerts first
	if c.alertRepo != nil {
		alerts, err := c.alertRepo.GetUnacknowledgedByResourceID(agent.ID)
		if err == nil && len(alerts) > 0 {
			for _, alert := range alerts {
				switch alert.Severity {
				case domain.AlertSeverityCritical:
					score -= 0.25 // Critical alerts have major impact
				case domain.AlertSeverityHigh:
					score -= 0.10
				case domain.AlertSeverityWarning:
					score -= 0.05
				}
			}
		}
	}

	// Check capability violations with cumulative impact
	violations, _, err := c.capabilityRepo.GetViolationsByAgentID(agent.ID, 500, 0)
	if err != nil || len(violations) == 0 {
		return math.Max(0.0, score)
	}

	// Only count violations from last 90 days
	ninetyDaysAgo := time.Now().AddDate(0, 0, -90)

	for _, v := range violations {
		if v.CreatedAt.After(ninetyDaysAgo) {
			// Base impact by severity
			var impact float64
			switch v.Severity {
			case domain.ViolationSeverityCritical:
				impact = 0.25
			case domain.ViolationSeverityHigh:
				impact = 0.10
			case domain.ViolationSeverityMedium:
				impact = 0.05
			case domain.ViolationSeverityLow:
				impact = 0.02
			default:
				impact = 0.02
			}

			// Blocked violations have 50% impact (system successfully prevented the attack)
			if v.IsBlocked {
				impact *= 0.5
			}

			score -= impact
		}
	}

	return math.Max(0.0, score)
}

// Factor 5: Compliance Score (10% weight)
// Measures adherence to compliance policies (SOC 2, HIPAA, GDPR)
// Queries the latest compliance snapshot for the agent's organization.
// Snapshot score is 0-100, normalized to 0.0-1.0.
// Returns 0.5 (neutral) when no snapshot data is available.
func (c *TrustCalculator) calculateCompliance(agent *domain.Agent) float64 {
	if c.snapshotRepo == nil {
		return 0.5 // Neutral when snapshot repo not configured
	}

	// Query the latest AIM compliance snapshot for the agent's organization
	snapshot, err := c.snapshotRepo.GetLatest(agent.OrganizationID, domain.FrameworkAIM)
	if err != nil || snapshot == nil {
		return 0.5 // Neutral when no compliance data exists
	}

	// Snapshot score is 0-100, normalize to 0.0-1.0
	normalized := snapshot.Score / 100.0
	return math.Max(0.0, math.Min(1.0, normalized))
}

// Factor 6: Age & History (5% weight)
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

// Factor 7: Drift Detection (3% weight)
// Measures changes in agent behavior patterns by checking for
// configuration drift alerts. No alerts = 1.0 (perfect).
// Each drift alert reduces the score proportionally by severity.
func (c *TrustCalculator) calculateDriftDetection(agent *domain.Agent) float64 {
	if c.alertRepo == nil {
		return 0.5 // Neutral when alert repo not available
	}

	// Query recent alerts for this agent
	alerts, err := c.alertRepo.GetByResourceID(agent.ID, 100, 0)
	if err != nil {
		return 0.5 // Neutral on error
	}

	if len(alerts) == 0 {
		return 1.0 // No alerts = no drift = perfect score
	}

	// Filter for drift-related alerts from the last 30 days
	score := 1.0
	thirtyDaysAgo := time.Now().AddDate(0, 0, -30)

	for _, alert := range alerts {
		if alert.CreatedAt.Before(thirtyDaysAgo) {
			continue
		}

		// Only consider drift-related alert types
		if alert.AlertType != domain.AlertTypeConfigurationDrift &&
			alert.AlertType != domain.AlertMCPCapabilityDrift {
			continue
		}

		// Reduce score based on severity
		switch alert.Severity {
		case domain.AlertSeverityCritical:
			score -= 0.30
		case domain.AlertSeverityHigh:
			score -= 0.15
		case domain.AlertSeverityWarning:
			score -= 0.05
		default:
			score -= 0.02
		}
	}

	return math.Max(0.0, score)
}

// Factor 8: User Feedback (2% weight)
// Measures explicit feedback from users, scored by sentiment-bucket counts.
//
// Returns 0.5 (neutral) when no feedback repository is wired or no feedback
// exists, so agents are neither penalized nor rewarded without real data.
//
// Scoring formula (sentiment buckets from domain.FeedbackSentiment, rating 1-5):
//
//	negative_feedback > 5:  0.00  (sustained dissatisfaction)
//	negative_feedback > 2:  0.50  (some dissatisfaction)
//	positive_feedback > 10: 1.00  (strong positive track record)
//	else:                   0.75  (mild positive / mixed)
//
// Negative checks precede positive so that an agent with many positives AND
// many negatives is not rewarded; sustained complaints dominate.
func (c *TrustCalculator) calculateUserFeedback(agent *domain.Agent) float64 {
	if c.userFeedbackRepo == nil {
		return 0.5 // Neutral when feedback collection not configured
	}

	stats, err := c.userFeedbackRepo.GetStats(agent.ID)
	if err != nil || stats == nil || stats.Total == 0 {
		return 0.5 // Neutral when no feedback data exists
	}

	if stats.NegativeCount > 5 {
		return 0.0
	}
	if stats.NegativeCount > 2 {
		return 0.5
	}
	if stats.PositiveCount > 10 {
		return 1.0
	}
	return 0.75
}

// Factor 9: Execution Isolation (10% weight)
// Measures the runtime isolation posture of the agent.
// Agents self-report their isolation via SDK; the score is computed from
// sandbox type, network isolation, filesystem isolation, and process isolation.
// Returns 0.3 (low baseline) when no attestation exists, incentivizing agents
// to report their isolation posture for a higher score.
func (c *TrustCalculator) calculateExecutionIsolation(agent *domain.Agent) float64 {
	if c.isolationRepo == nil {
		return 0.3 // Low baseline when no isolation data available
	}

	attestation, err := c.isolationRepo.GetLatest(agent.ID)
	if err != nil || attestation == nil {
		return 0.3 // No attestation submitted yet
	}

	// Use the pre-computed score from the attestation
	return attestation.Score
}

// calculateConfidence determines confidence level based on available data
func (c *TrustCalculator) calculateConfidence(agent *domain.Agent, factors *domain.TrustScoreFactors) float64 {
	// Count available data points (each real data source adds confidence)
	dataPoints := 0.0
	total := 9.0 // 9 factors

	// Base data points from agent properties
	if agent.Status != "" {
		dataPoints++
	}
	if agent.PublicKey != nil && *agent.PublicKey != "" {
		dataPoints++
	}
	if agent.CreatedAt.Before(time.Now().AddDate(0, -1, 0)) {
		dataPoints++ // Agent has some history
	}

	// Check if we have real verification event data
	if c.verificationEventRepo != nil {
		endTime := time.Now()
		startTime := endTime.AddDate(0, 0, -30)

		stats, err := c.verificationEventRepo.GetAgentStatistics(agent.ID, startTime, endTime)
		if err == nil && stats.TotalVerifications > 0 {
			// Real verification data available - higher confidence
			dataPoints += 3 // Covers verification, uptime, and success rate factors

			// Even higher confidence if significant sample size
			if stats.TotalVerifications >= 10 {
				dataPoints += 0.5
			}
			if stats.TotalVerifications >= 50 {
				dataPoints += 0.5
			}
		}
	}

	// Check if we have real alert data
	if c.alertRepo != nil {
		alerts, err := c.alertRepo.GetByResourceID(agent.ID, 100, 0)
		if err == nil {
			// Having alert data (even if empty) increases confidence
			dataPoints++
			if len(alerts) > 0 {
				dataPoints += 0.5 // More data = more confidence in the score
			}
		}
	}

	// Check if we have isolation attestation data
	if c.isolationRepo != nil {
		att, err := c.isolationRepo.GetLatest(agent.ID)
		if err == nil && att != nil {
			dataPoints++ // Isolation attestation submitted
		}
	}

	// Check if we have NanoMind TME data
	if c.tmeProvider != nil {
		tmeScore, err := c.tmeProvider.GetLatestTMEScore(agent.ID)
		if err == nil && tmeScore >= 0 {
			dataPoints += 0.5 // TME data enriches security alerts confidence
		}
	}

	confidence := dataPoints / total
	return math.Min(1.0, confidence) // Cap at 1.0
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

	// Store the score breakdown in trust_scores table
	if err := c.trustScoreRepo.Create(score); err != nil {
		return nil, err
	}

	// Update the agent's trust_score field to keep it in sync
	// This ensures agents.trust_score matches the calculated score from trust_scores table
	if err := c.agentRepo.UpdateTrustScore(agentID, score.Score); err != nil {
		return nil, fmt.Errorf("failed to update agent trust score: %w", err)
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

// GetTrustScoreHistoryAuditTrail returns audit trail from trust_score_history table
// This includes who changed it, when, and why - for frontend UI display
func (c *TrustCalculator) GetTrustScoreHistoryAuditTrail(ctx context.Context, agentID uuid.UUID, limit int) ([]*domain.TrustScoreHistoryEntry, error) {
	return c.trustScoreRepo.GetHistoryAuditTrail(agentID, limit)
}

// RecordUserFeedback persists a user's explicit rating for an agent. This is the
// collection side of the user feedback factor (factor 8): without it the factor
// has nothing to read and stays at the neutral 0.5 baseline. Rating must be 1-5.
func (c *TrustCalculator) RecordUserFeedback(ctx context.Context, agentID, orgID uuid.UUID, userID *uuid.UUID, rating int, comment string) (*domain.UserFeedback, error) {
	if c.userFeedbackRepo == nil {
		return nil, fmt.Errorf("user feedback repository not configured")
	}
	if rating < 1 || rating > 5 {
		return nil, fmt.Errorf("rating must be between 1 and 5, got %d", rating)
	}

	feedback := &domain.UserFeedback{
		ID:             uuid.New(),
		AgentID:        agentID,
		OrganizationID: orgID,
		UserID:         userID,
		Rating:         rating,
		Comment:        comment,
		CreatedAt:      time.Now(),
	}

	if err := c.userFeedbackRepo.Create(feedback); err != nil {
		return nil, fmt.Errorf("failed to record user feedback: %w", err)
	}

	return feedback, nil
}

// RecordIsolationAttestation persists an agent's self-reported runtime isolation
// posture. This is the collection side of the execution isolation factor (factor
// 9): without it the factor has nothing to read and stays at the 0.3 baseline.
//
// The posture is self-asserted by the agent and is NOT independently verified;
// the score is computed server-side via domain.ScoreIsolation so the agent
// cannot inject an arbitrary score, and unrecognized posture values are rejected
// before anything is written. Independent verification of the reported posture is
// a separate follow-up (see roadmap aim-isolation-verification).
func (c *TrustCalculator) RecordIsolationAttestation(ctx context.Context, agentID uuid.UUID, sandbox domain.SandboxType, network domain.NetworkIsolation, filesystem domain.FilesystemIsolation, process domain.ProcessIsolation) (*domain.IsolationAttestation, error) {
	if c.isolationRepo == nil {
		return nil, fmt.Errorf("isolation attestation repository not configured")
	}
	if err := domain.ValidateIsolationPosture(sandbox, network, filesystem, process); err != nil {
		return nil, err
	}

	now := time.Now()
	attestation := &domain.IsolationAttestation{
		ID:         uuid.New(),
		AgentID:    agentID,
		Sandbox:    sandbox,
		Network:    network,
		Filesystem: filesystem,
		Process:    process,
		Score:      domain.ScoreIsolation(sandbox, network, filesystem, process),
		ReportedAt: now,
		CreatedAt:  now,
	}

	if err := c.isolationRepo.Create(attestation); err != nil {
		return nil, fmt.Errorf("failed to record isolation attestation: %w", err)
	}

	return attestation, nil
}
