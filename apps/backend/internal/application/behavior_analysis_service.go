package application

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
)

// BehaviorAnalysisService handles behavioral anomaly detection
// using per-agent learned baselines for statistical analysis
type BehaviorAnalysisService struct {
	db           *sql.DB
	baselineRepo domain.BehaviorBaselineRepository
	alertService *AlertService
}

// NewBehaviorAnalysisService creates a new behavior analysis service
func NewBehaviorAnalysisService(
	db *sql.DB,
	baselineRepo domain.BehaviorBaselineRepository,
	alertService *AlertService,
) *BehaviorAnalysisService {
	return &BehaviorAnalysisService{
		db:           db,
		baselineRepo: baselineRepo,
		alertService: alertService,
	}
}

// Configuration constants for behavioral analysis
const (
	// Minimum samples required before baseline is considered "mature"
	DefaultMaturityThreshold = 50

	// Sigma thresholds for anomaly severity classification
	SigmaThresholdLow      = 2.0 // 2-sigma = ~5% probability
	SigmaThresholdMedium   = 3.0 // 3-sigma = ~0.3% probability
	SigmaThresholdHigh     = 4.0 // 4-sigma = ~0.006% probability
	SigmaThresholdCritical = 5.0 // 5-sigma = ~0.00006% probability

	// Minimum agent age before anomaly detection kicks in
	MinAgentAgeForDetection = 24 * time.Hour

	// Sliding window for calculating current velocity
	VelocityWindowHours = 1
)

// RecordActivity records an agent's activity and updates the behavioral baseline
// This should be called after each verification event
func (s *BehaviorAnalysisService) RecordActivity(
	ctx context.Context,
	agentID uuid.UUID,
	orgID uuid.UUID,
	action string,
	resource string,
	capability string,
) error {
	// Get or create baseline for this agent
	baseline, err := s.baselineRepo.GetByAgentID(agentID)
	if err != nil {
		return fmt.Errorf("failed to get baseline: %w", err)
	}

	now := time.Now()

	if baseline == nil {
		// Create new baseline
		baseline = &domain.AgentBehaviorBaseline{
			ID:                uuid.New(),
			AgentID:           agentID,
			OrganizationID:    orgID,
			BaselineStartAt:   now,
			BaselineEndAt:     now,
			SamplesCount:      0,
			AvgCallsPerHour:   0,
			StddevCallsPerHour: 0,
			MaxCallsPerHour:   0,
			CapabilityUsage:   make(map[string]int),
			ResourcePatterns:  make(map[string]int),
			ActionSequences:   []domain.ActionSequence{},
			AvgRiskScore:      0,
			StddevRiskScore:   0,
			IsMature:          false,
			MaturityThreshold: DefaultMaturityThreshold,
			CreatedAt:         now,
		}
	}

	// Update sample count
	baseline.SamplesCount++
	baseline.BaselineEndAt = now

	// Update capability usage
	if capability != "" {
		if baseline.CapabilityUsage == nil {
			baseline.CapabilityUsage = make(map[string]int)
		}
		baseline.CapabilityUsage[capability]++
	}

	// Update resource patterns (extract resource type from resource string)
	if resource != "" {
		resourceType := extractResourceType(resource)
		if baseline.ResourcePatterns == nil {
			baseline.ResourcePatterns = make(map[string]int)
		}
		baseline.ResourcePatterns[resourceType]++
	}

	// Calculate risk-weighted score for this action
	riskWeight := domain.GetRiskWeight(capability)

	// Update rolling statistics using Welford's algorithm for numerical stability
	updateBaselineRiskStats(baseline, riskWeight)

	// Update velocity stats (calls per hour)
	if err := s.updateVelocityStats(ctx, baseline, agentID); err != nil {
		// Log but don't fail - velocity stats are non-critical
		fmt.Printf("Warning: failed to update velocity stats: %v\n", err)
	}

	// Check if baseline is now mature
	if !baseline.IsMature && baseline.SamplesCount >= baseline.MaturityThreshold {
		baseline.IsMature = true
		fmt.Printf("✅ Agent %s baseline now mature (%d samples)\n", agentID, baseline.SamplesCount)
	}

	// Save updated baseline
	if err := s.baselineRepo.Upsert(baseline); err != nil {
		return fmt.Errorf("failed to save baseline: %w", err)
	}

	return nil
}

// DetectAnomalies analyzes current activity against the baseline and returns any anomalies
// Uses statistical deviation (sigma) from learned baseline patterns
func (s *BehaviorAnalysisService) DetectAnomalies(
	ctx context.Context,
	agentID uuid.UUID,
	orgID uuid.UUID,
	action string,
	resource string,
	capability string,
) (*domain.DetectionResult, error) {
	result := &domain.DetectionResult{
		IsAnomalous: false,
		Anomalies:   []domain.BehavioralAnomaly{},
		ShouldBlock: false,
		ShouldAlert: false,
	}

	// Get the agent's baseline
	baseline, err := s.baselineRepo.GetByAgentID(agentID)
	if err != nil {
		return result, fmt.Errorf("failed to get baseline: %w", err)
	}

	// If no baseline exists or not mature, skip anomaly detection
	if baseline == nil {
		result.Reason = "No baseline established yet - learning mode"
		return result, nil
	}

	if !baseline.IsMature {
		result.Reason = fmt.Sprintf("Baseline not mature (%d/%d samples) - learning mode",
			baseline.SamplesCount, baseline.MaturityThreshold)
		return result, nil
	}

	now := time.Now()

	// 1. Check for new capability (never seen before)
	if capability != "" && baseline.CapabilityUsage != nil {
		if _, seen := baseline.CapabilityUsage[capability]; !seen {
			anomaly := domain.BehavioralAnomaly{
				ID:                uuid.New(),
				AgentID:           agentID,
				OrganizationID:    orgID,
				AnomalyType:       domain.BehavioralAnomalyNewCapability,
				Severity:          s.severityForNewCapability(capability),
				Description:       fmt.Sprintf("First-time use of capability '%s' - never seen in baseline period", capability),
				TriggerCapability: capability,
				TriggerAction:     action,
				TriggerResource:   resource,
				Metadata: map[string]interface{}{
					"baselineSamples": baseline.SamplesCount,
					"knownCapabilities": len(baseline.CapabilityUsage),
				},
				DetectedAt: now,
			}
			result.Anomalies = append(result.Anomalies, anomaly)
			result.IsAnomalous = true
		}
	}

	// 2. Check for new resource type (never accessed before)
	if resource != "" && baseline.ResourcePatterns != nil {
		resourceType := extractResourceType(resource)
		if _, seen := baseline.ResourcePatterns[resourceType]; !seen {
			anomaly := domain.BehavioralAnomaly{
				ID:                uuid.New(),
				AgentID:           agentID,
				OrganizationID:    orgID,
				AnomalyType:       domain.BehavioralAnomalyNewResource,
				Severity:          domain.AnomalySeverityMedium,
				Description:       fmt.Sprintf("First-time access to resource type '%s'", resourceType),
				TriggerResource:   resource,
				TriggerAction:     action,
				TriggerCapability: capability,
				Metadata: map[string]interface{}{
					"resourceType": resourceType,
					"knownResourceTypes": len(baseline.ResourcePatterns),
				},
				DetectedAt: now,
			}
			result.Anomalies = append(result.Anomalies, anomaly)
			result.IsAnomalous = true
		}
	}

	// 3. Check for velocity spike (sudden increase in call rate)
	currentVelocity, err := s.getCurrentVelocity(ctx, agentID)
	if err == nil && baseline.StddevCallsPerHour > 0 {
		sigma := domain.CalculateSigma(currentVelocity, baseline.AvgCallsPerHour, baseline.StddevCallsPerHour)
		if domain.IsSignificantDeviation(sigma, SigmaThresholdMedium) {
			severity := s.severityFromSigma(sigma)
			expectedVal := baseline.AvgCallsPerHour
			actualVal := currentVelocity

			anomaly := domain.BehavioralAnomaly{
				ID:             uuid.New(),
				AgentID:        agentID,
				OrganizationID: orgID,
				AnomalyType:    domain.BehavioralAnomalyVelocitySpike,
				Severity:       severity,
				Description:    fmt.Sprintf("Unusual activity rate: %.1f calls/hour vs baseline %.1f±%.1f (%.1fσ deviation)", currentVelocity, baseline.AvgCallsPerHour, baseline.StddevCallsPerHour, sigma),
				ExpectedValue:  &expectedVal,
				ActualValue:    &actualVal,
				DeviationSigma: &sigma,
				Metadata: map[string]interface{}{
					"currentVelocity": currentVelocity,
					"avgVelocity":     baseline.AvgCallsPerHour,
					"stddevVelocity":  baseline.StddevCallsPerHour,
				},
				DetectedAt: now,
			}
			result.Anomalies = append(result.Anomalies, anomaly)
			result.IsAnomalous = true

			// High velocity spikes are more concerning
			if severity == domain.AnomalySeverityCritical || severity == domain.AnomalySeverityHigh {
				result.ShouldAlert = true
			}
		}
	}

	// 4. Check for risk score spike (sudden high-risk activity)
	if capability != "" && baseline.StddevRiskScore > 0 {
		currentRisk := domain.GetRiskWeight(capability)
		sigma := domain.CalculateSigma(currentRisk, baseline.AvgRiskScore, baseline.StddevRiskScore)

		if domain.IsSignificantDeviation(sigma, SigmaThresholdMedium) {
			severity := s.severityFromSigma(sigma)
			expectedVal := baseline.AvgRiskScore
			actualVal := currentRisk

			anomaly := domain.BehavioralAnomaly{
				ID:                uuid.New(),
				AgentID:           agentID,
				OrganizationID:    orgID,
				AnomalyType:       domain.BehavioralAnomalyRiskSpike,
				Severity:          severity,
				Description:       fmt.Sprintf("High-risk operation: risk weight %.1f vs baseline %.1f±%.1f (%.1fσ)", currentRisk, baseline.AvgRiskScore, baseline.StddevRiskScore, sigma),
				ExpectedValue:     &expectedVal,
				ActualValue:       &actualVal,
				DeviationSigma:    &sigma,
				TriggerCapability: capability,
				TriggerAction:     action,
				Metadata: map[string]interface{}{
					"riskWeight": currentRisk,
					"avgRisk":    baseline.AvgRiskScore,
					"stddevRisk": baseline.StddevRiskScore,
				},
				DetectedAt: now,
			}
			result.Anomalies = append(result.Anomalies, anomaly)
			result.IsAnomalous = true

			if severity == domain.AnomalySeverityCritical {
				result.ShouldAlert = true
				result.ShouldBlock = true // Block critical risk spikes
			}
		}
	}

	// 5. Check for capability drift (unusual pattern in capability usage)
	if capability != "" && baseline.CapabilityUsage != nil {
		if driftAnomaly := s.detectCapabilityDrift(baseline, capability, agentID, orgID); driftAnomaly != nil {
			result.Anomalies = append(result.Anomalies, *driftAnomaly)
			result.IsAnomalous = true
		}
	}

	// Set reason and determine final alert status
	if result.IsAnomalous {
		result.Reason = fmt.Sprintf("Detected %d behavioral anomalies", len(result.Anomalies))

		// Record all anomalies
		for _, anomaly := range result.Anomalies {
			if err := s.baselineRepo.RecordAnomaly(&anomaly); err != nil {
				fmt.Printf("Warning: failed to record anomaly: %v\n", err)
			}

			// Any high/critical severity should trigger alert
			if anomaly.Severity == domain.AnomalySeverityHigh || anomaly.Severity == domain.AnomalySeverityCritical {
				result.ShouldAlert = true
			}
		}
	}

	return result, nil
}

// updateVelocityStats updates the velocity (calls per hour) statistics
func (s *BehaviorAnalysisService) updateVelocityStats(
	ctx context.Context,
	baseline *domain.AgentBehaviorBaseline,
	agentID uuid.UUID,
) error {
	// Calculate current velocity based on recent verifications
	currentVelocity, err := s.getCurrentVelocity(ctx, agentID)
	if err != nil {
		return err
	}

	// Update max if needed
	if int(currentVelocity) > baseline.MaxCallsPerHour {
		baseline.MaxCallsPerHour = int(currentVelocity)
	}

	// Use Welford's algorithm for running mean and variance
	n := float64(baseline.SamplesCount)
	if n == 1 {
		baseline.AvgCallsPerHour = currentVelocity
		baseline.StddevCallsPerHour = 0
	} else {
		oldMean := baseline.AvgCallsPerHour
		baseline.AvgCallsPerHour = oldMean + (currentVelocity-oldMean)/n

		// Running variance calculation
		// This is a simplified version - for more accuracy we'd track M2
		// But for our purposes, this approximation is sufficient
		baseline.StddevCallsPerHour = math.Sqrt(
			(baseline.StddevCallsPerHour*baseline.StddevCallsPerHour*(n-1) +
				(currentVelocity-oldMean)*(currentVelocity-baseline.AvgCallsPerHour)) / n,
		)
	}

	return nil
}

// getCurrentVelocity calculates the current call rate (calls per hour)
func (s *BehaviorAnalysisService) getCurrentVelocity(ctx context.Context, agentID uuid.UUID) (float64, error) {
	// Count verifications in the last hour
	var count int
	query := `
		SELECT COUNT(*)
		FROM verification_events
		WHERE agent_id = $1
		  AND created_at > NOW() - INTERVAL '1 hour'
	`

	if err := s.db.QueryRowContext(ctx, query, agentID).Scan(&count); err != nil {
		return 0, err
	}

	return float64(count), nil
}

// updateBaselineRiskStats updates running statistics for risk scores on a baseline
func updateBaselineRiskStats(b *domain.AgentBehaviorBaseline, riskWeight float64) {
	n := float64(b.SamplesCount)
	if n == 1 {
		b.AvgRiskScore = riskWeight
		b.StddevRiskScore = 0
	} else {
		oldMean := b.AvgRiskScore
		b.AvgRiskScore = oldMean + (riskWeight-oldMean)/n

		// Running variance (Welford's)
		b.StddevRiskScore = math.Sqrt(
			(b.StddevRiskScore*b.StddevRiskScore*(n-1) +
				(riskWeight-oldMean)*(riskWeight-b.AvgRiskScore)) / n,
		)
	}
}

// severityFromSigma determines anomaly severity based on sigma deviation
func (s *BehaviorAnalysisService) severityFromSigma(sigma float64) domain.AnomalySeverity {
	absSigma := math.Abs(sigma)
	switch {
	case absSigma >= SigmaThresholdCritical:
		return domain.AnomalySeverityCritical
	case absSigma >= SigmaThresholdHigh:
		return domain.AnomalySeverityHigh
	case absSigma >= SigmaThresholdMedium:
		return domain.AnomalySeverityMedium
	default:
		return domain.AnomalySeverityLow
	}
}

// severityForNewCapability determines severity based on capability type
func (s *BehaviorAnalysisService) severityForNewCapability(capability string) domain.AnomalySeverity {
	riskWeight := domain.GetRiskWeight(capability)
	switch {
	case riskWeight >= 9.0: // admin, security
		return domain.AnomalySeverityCritical
	case riskWeight >= 7.0: // delete, system
		return domain.AnomalySeverityHigh
	case riskWeight >= 5.0: // write, execute
		return domain.AnomalySeverityMedium
	default:
		return domain.AnomalySeverityLow
	}
}

// detectCapabilityDrift checks for unusual capability usage patterns
func (s *BehaviorAnalysisService) detectCapabilityDrift(
	baseline *domain.AgentBehaviorBaseline,
	capability string,
	agentID uuid.UUID,
	orgID uuid.UUID,
) *domain.BehavioralAnomaly {
	if baseline.CapabilityUsage == nil {
		return nil
	}

	// Calculate total usage across all capabilities
	totalUsage := 0
	for _, count := range baseline.CapabilityUsage {
		totalUsage += count
	}

	if totalUsage < 10 {
		return nil // Not enough data
	}

	// Check if this capability's usage is unusually low or high relative to pattern
	capUsage, exists := baseline.CapabilityUsage[capability]
	if !exists {
		return nil // Already handled by new_capability check
	}

	// Calculate expected proportion based on historical usage
	expectedProportion := float64(capUsage) / float64(totalUsage)

	// If a previously rare capability is being used heavily, that's drift
	// For simplicity, flag if a capability that's <5% of history is suddenly active
	if expectedProportion < 0.05 {
		// Check recent usage (last hour)
		// This would require a more complex query - for MVP, skip this
		// Future enhancement: track recent vs baseline proportions
		return nil
	}

	return nil
}

// extractResourceType extracts a resource type from a resource string
// e.g., "s3://bucket/key" -> "s3", "/api/users/123" -> "api"
func extractResourceType(resource string) string {
	if resource == "" {
		return "unknown"
	}

	// Handle protocol-style resources (s3://, dynamodb://, etc.)
	for i, c := range resource {
		if c == ':' && i > 0 {
			return resource[:i]
		}
	}

	// Handle path-style resources (/api/..., /admin/..., etc.)
	if resource[0] == '/' && len(resource) > 1 {
		end := 1
		for ; end < len(resource) && resource[end] != '/'; end++ {
		}
		return resource[1:end]
	}

	// Default: use first segment
	for i, c := range resource {
		if c == '/' || c == ':' || c == '.' {
			return resource[:i]
		}
	}

	return resource
}

// GetBaselineStatus returns the baseline status for an agent
func (s *BehaviorAnalysisService) GetBaselineStatus(
	ctx context.Context,
	agentID uuid.UUID,
) (*domain.AgentBehaviorBaseline, error) {
	return s.baselineRepo.GetByAgentID(agentID)
}

// GetRecentAnomalies returns recent anomalies for an agent
func (s *BehaviorAnalysisService) GetRecentAnomalies(
	ctx context.Context,
	agentID uuid.UUID,
	limit int,
) ([]*domain.BehavioralAnomaly, error) {
	return s.baselineRepo.GetRecentAnomalies(agentID, limit)
}
