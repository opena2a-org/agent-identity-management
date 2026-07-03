package application

import (
	"context"
	"fmt"
	"log"
	"math"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// Factor exclusion reasons (AIP-SPEC §6.1: "Factors with no data are excluded
// and their weights redistributed proportionally"). A factor excluded for
// exclReasonNotWired or a query failure is a deployment defect and is logged;
// exclReasonNoData is a normal lifecycle state (e.g. an agent whose org has no
// compliance snapshot yet) and is not logged.
const (
	exclReasonNotWired = "repository not configured"
	exclReasonNoData   = "no data"
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
	mcpConnRepo           MCPConnectionLister
}

// MCPConnectionLister resolves the MCP servers an agent is connected to. The drift
// factor needs it because MCP drift alerts are keyed by server ID, not agent ID, so
// an agent's drift signal must be gathered across the servers it actually uses (#314).
type MCPConnectionLister interface {
	ListByAgent(ctx context.Context, agentID uuid.UUID) ([]*domain.AgentMCPConnection, error)
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

// SetMCPConnectionRepo sets the agent-MCP connection repository so the drift factor
// can attribute server-keyed MCP drift alerts to the agents connected to those servers
// (#314). Optional; when not set, the drift factor only sees agent-keyed alerts (the
// pre-#314 behavior, in which MCP drift never affected trust).
func (c *TrustCalculator) SetMCPConnectionRepo(repo MCPConnectionLister) {
	c.mcpConnRepo = repo
}

// SetTMEProvider sets the NanoMind TME provider for behavioral trust enrichment.
// Optional; when not set, TME has no effect on scoring.
func (c *TrustCalculator) SetTMEProvider(provider NanoMindTMEProvider) {
	c.tmeProvider = provider
}

// Calculate calculates trust score for an agent
// Implements the 9-factor algorithm with weighted average
func (c *TrustCalculator) Calculate(agent *domain.Agent) (*domain.TrustScore, error) {
	factors, excluded, err := c.calculateFactorsDetailed(agent)
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

	// AIP-SPEC §6.1 composition rule: factors with no data are excluded from
	// the weighted sum and their weights redistributed proportionally across
	// the factors that do have data. A factor whose repository is un-wired or
	// whose query failed must not contribute a fabricated neutral value.
	contributions := []struct {
		key   string
		value float64
	}{
		{"verification", factors.VerificationStatus},
		{"uptime", factors.Uptime},
		{"success_rate", factors.SuccessRate},
		{"security_alerts", factors.SecurityAlerts},
		{"compliance", factors.Compliance},
		{"age", factors.Age},
		{"drift_detection", factors.DriftDetection},
		{"user_feedback", factors.UserFeedback},
		{"execution_isolation", factors.ExecutionIsolation},
	}

	var weightedSum, includedWeight, imputedSum float64
	for _, f := range contributions {
		w, ok := weights[f.key]
		if !ok {
			// A key drift between contributions and weights would silently
			// zero a factor; fail loudly instead.
			return nil, fmt.Errorf("trust: factor %q has no weight entry", f.key)
		}
		// The neutral-imputed sum uses every factor at its struct value —
		// for excluded factors that is the neutral display placeholder.
		imputedSum += f.value * w
		if reason, isExcluded := excluded[f.key]; isExcluded {
			if reason != exclReasonNoData {
				// Un-wired repository or query failure: a deployment defect in
				// the reference implementation, not a data-lifecycle state.
				log.Printf("WARN trust: factor %q excluded from composite for agent %s: %s", f.key, agent.ID, reason)
			}
			continue
		}
		weightedSum += f.value * w
		includedWeight += w
	}

	// Factors 1-4 and 6 always produce data (status-based fallbacks are part
	// of their scoring functions), so includedWeight is at least 0.75.
	score := weightedSum / includedWeight

	// Anti-gaming ceiling: renormalization alone would hand an excluded
	// factor the agent's own included average, so an org could RAISE a good
	// agent's score by withholding compliance snapshots or never collecting
	// feedback (no-data would outscore a measured 0.9). Cap the published
	// composite at the neutral-imputed sum — a factor with no data can never
	// help more than a neutral measurement would. Bad agents keep the honest
	// renormalized value (exclusion no longer props them up toward neutral).
	// With no exclusions both sums are identical and the cap is a no-op.
	score = math.Min(score, imputedSum)

	// Ensure score is within bounds [0, 1]
	score = math.Max(0.0, math.Min(1.0, score))

	excludedNames := make([]string, 0, len(excluded))
	for name := range excluded {
		excludedNames = append(excludedNames, name)
	}
	sort.Strings(excludedNames)

	// Calculate confidence based on available data; excluded factors count
	// against it (an exclusion must not be confidence-free).
	confidence := c.calculateConfidence(agent, factors, excluded)

	return &domain.TrustScore{
		ID:              uuid.New(),
		AgentID:         agent.ID,
		Score:           score,
		Factors:         *factors,
		ExcludedFactors: excludedNames,
		Confidence:      confidence,
		LastCalculated:  time.Now(),
		CreatedAt:       time.Now(),
	}, nil
}

// CalculateFactors calculates individual trust factors
func (c *TrustCalculator) CalculateFactors(agent *domain.Agent) (*domain.TrustScoreFactors, error) {
	factors, _, err := c.calculateFactorsDetailed(agent)
	return factors, err
}

// calculateFactorsDetailed computes the 9 factors plus the exclusion set: the
// factors whose data source is un-wired, failed, or empty. An excluded factor
// carries a neutral placeholder value in the returned struct (display
// continuity for stored breakdowns) but MUST NOT contribute to the composite —
// Calculate redistributes its weight per AIP-SPEC §6.1.
func (c *TrustCalculator) calculateFactorsDetailed(agent *domain.Agent) (*domain.TrustScoreFactors, map[string]string, error) {
	factors := &domain.TrustScoreFactors{}
	excluded := make(map[string]string)

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
	var reason string
	factors.Compliance, reason = c.calculateCompliance(agent)
	if reason != "" {
		excluded["compliance"] = reason
	}

	// Factor 6: Age & History (5% weight)
	// How long agent has been operating successfully
	factors.Age = c.calculateAge(agent)

	// Factor 7: Drift Detection (3% weight)
	// Behavioral pattern changes
	factors.DriftDetection, reason = c.calculateDriftDetection(agent)
	if reason != "" {
		excluded["drift_detection"] = reason
	}

	// Factor 8: User Feedback (2% weight)
	// Explicit user ratings
	factors.UserFeedback, reason = c.calculateUserFeedback(agent)
	if reason != "" {
		excluded["user_feedback"] = reason
	}

	// Factor 9: Execution Isolation (10% weight)
	// Runtime isolation posture (sandbox, network, filesystem, process)
	factors.ExecutionIsolation, reason = c.calculateExecutionIsolation(agent)
	if reason != "" {
		excluded["execution_isolation"] = reason
	}

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

	return factors, excluded, nil
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
// With no snapshot data the factor is EXCLUDED from the composite (AIP §6.1);
// the returned 0.5 is a display placeholder only, never a contribution.
func (c *TrustCalculator) calculateCompliance(agent *domain.Agent) (float64, string) {
	if c.snapshotRepo == nil {
		return 0.5, exclReasonNotWired
	}

	// Query the latest AIM compliance snapshot for the agent's organization
	snapshot, err := c.snapshotRepo.GetLatest(agent.OrganizationID, domain.FrameworkAIM)
	if err != nil {
		return 0.5, "compliance snapshot query failed: " + err.Error()
	}
	if snapshot == nil {
		return 0.5, exclReasonNoData
	}

	// Snapshot score is 0-100, normalize to 0.0-1.0
	normalized := snapshot.Score / 100.0
	return math.Max(0.0, math.Min(1.0, normalized)), ""
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
// configuration drift alerts. No alerts = 1.0 (perfect): a wired alert
// repository returning zero alerts is a measurement, not missing data.
// Each drift alert reduces the score proportionally by severity.
// With no alert repository (or a failed agent-alert query) the factor is
// EXCLUDED from the composite (AIP §6.1); the returned 0.5 is a display
// placeholder only.
func (c *TrustCalculator) calculateDriftDetection(agent *domain.Agent) (float64, string) {
	if c.alertRepo == nil {
		return 0.5, exclReasonNotWired
	}

	// Gather drift-relevant alerts from two key spaces:
	//   - agent-keyed alerts (AlertTypeConfigurationDrift is created with ResourceID = agentID)
	//   - server-keyed MCP drift alerts for every MCP server this agent is connected to
	//     (AlertMCPCapabilityDrift / AlertMCPManifestDrift are created with ResourceID = serverID).
	// MCP drift is a property of the server, so its alerts are server-keyed; an agent
	// connected to a drifted server inherits that supply-chain risk. Without the
	// connection repo wired we fall back to agent-keyed alerts only (#314).
	seen := make(map[uuid.UUID]bool)
	alerts := make([]*domain.Alert, 0)
	collect := func(in []*domain.Alert) {
		for _, a := range in {
			if a == nil || seen[a.ID] {
				continue
			}
			seen[a.ID] = true
			alerts = append(alerts, a)
		}
	}

	agentAlerts, err := c.alertRepo.GetByResourceID(agent.ID, 100, 0)
	if err != nil {
		return 0.5, "agent alert query failed: " + err.Error()
	}
	collect(agentAlerts)

	if c.mcpConnRepo != nil {
		if conns, connErr := c.mcpConnRepo.ListByAgent(context.Background(), agent.ID); connErr == nil {
			for _, conn := range conns {
				if conn == nil {
					continue
				}
				if serverAlerts, sErr := c.alertRepo.GetByResourceID(conn.MCPServerID, 100, 0); sErr == nil {
					collect(serverAlerts)
				}
			}
		}
	}

	if len(alerts) == 0 {
		return 1.0, "" // No alerts = no drift = perfect score
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
			alert.AlertType != domain.AlertMCPCapabilityDrift &&
			alert.AlertType != domain.AlertMCPManifestDrift {
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

	return math.Max(0.0, score), ""
}

// Factor 8: User Feedback (2% weight)
// Measures explicit feedback from users, scored by sentiment-bucket counts.
//
// With no feedback repository wired, a failed query, or zero feedback rows,
// the factor is EXCLUDED from the composite (AIP §6.1), so agents are neither
// penalized nor rewarded without real data; the returned 0.5 is a display
// placeholder only.
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
func (c *TrustCalculator) calculateUserFeedback(agent *domain.Agent) (float64, string) {
	if c.userFeedbackRepo == nil {
		return 0.5, exclReasonNotWired
	}

	stats, err := c.userFeedbackRepo.GetStats(agent.ID)
	if err != nil {
		return 0.5, "feedback stats query failed: " + err.Error()
	}
	if stats == nil || stats.Total == 0 {
		return 0.5, exclReasonNoData
	}

	if stats.NegativeCount > 5 {
		return 0.0, ""
	}
	if stats.NegativeCount > 2 {
		return 0.5, ""
	}
	if stats.PositiveCount > 10 {
		return 1.0, ""
	}
	return 0.75, ""
}

// Factor 9: Execution Isolation (10% weight)
// Measures the runtime isolation posture of the agent.
// Agents self-report their isolation via SDK; the score is computed from
// sandbox type, network isolation, filesystem isolation, and process isolation.
// Returns 0.3 (low baseline) when no attestation exists: unlike the no-data
// factors this is a deliberate per-factor scoring choice (permitted by AIP
// §6.1) that incentivizes agents to report their posture, and issuance
// surfaces it via IsolationSelfReported. Only an un-wired repository excludes
// the factor from the composite.
func (c *TrustCalculator) calculateExecutionIsolation(agent *domain.Agent) (float64, string) {
	if c.isolationRepo == nil {
		return 0.3, exclReasonNotWired
	}

	attestation, err := c.isolationRepo.GetLatest(agent.ID)
	if err != nil || attestation == nil {
		return 0.3, "" // No attestation submitted yet: scored low baseline by design
	}

	// Use the pre-computed score from the attestation
	return attestation.Score, ""
}

// calculateConfidence determines confidence level based on available data.
// excluded is the AIP §6.1 exclusion set from calculateFactorsDetailed: a
// factor that was excluded for lack of data contributes no confidence, and a
// measured compliance/feedback factor now does (previously neither was
// counted at all, so exclusion was invisible to confidence).
func (c *TrustCalculator) calculateConfidence(agent *domain.Agent, factors *domain.TrustScoreFactors, excluded map[string]string) float64 {
	// Count available data points (each real data source adds confidence)
	dataPoints := 0.0
	total := 9.0 // 9 factors

	if c.snapshotRepo != nil {
		if _, isExcluded := excluded["compliance"]; !isExcluded {
			dataPoints++ // Real compliance snapshot measured
		}
	}
	if c.userFeedbackRepo != nil {
		if _, isExcluded := excluded["user_feedback"]; !isExcluded {
			dataPoints++ // Real user feedback measured
		}
	}

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
