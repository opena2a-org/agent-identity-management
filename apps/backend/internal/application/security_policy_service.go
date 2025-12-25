package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
)

// SecurityPolicyService handles security policy evaluation and management
type SecurityPolicyService struct {
	policyRepo       domain.SecurityPolicyRepository
	alertRepo        domain.AlertRepository
	auditLogRepo     domain.AuditLogRepository
	agentRepo        domain.AgentRepository             // For suspending agents on critical trust score
	behaviorAnalysis *BehaviorAnalysisService           // Intelligent behavioral anomaly detection
}

// NewSecurityPolicyService creates a new security policy service
func NewSecurityPolicyService(
	policyRepo domain.SecurityPolicyRepository,
	alertRepo domain.AlertRepository,
	auditLogRepo domain.AuditLogRepository,
) *SecurityPolicyService {
	return &SecurityPolicyService{
		policyRepo:       policyRepo,
		alertRepo:        alertRepo,
		auditLogRepo:     auditLogRepo,
		behaviorAnalysis: nil, // Will be set via SetBehaviorAnalysis
	}
}

// SetBehaviorAnalysis sets the behavior analysis service for intelligent anomaly detection
// This uses setter injection to break circular dependency between services
func (s *SecurityPolicyService) SetBehaviorAnalysis(behaviorService *BehaviorAnalysisService) {
	s.behaviorAnalysis = behaviorService
}

// SetAgentRepository sets the agent repository for suspending agents on critical trust score
// This uses setter injection to break circular dependency between services
func (s *SecurityPolicyService) SetAgentRepository(agentRepo domain.AgentRepository) {
	s.agentRepo = agentRepo
}

// EvaluateCapabilityViolation evaluates security policies for capability violations
// Returns enforcement decision and whether to create an alert
func (s *SecurityPolicyService) EvaluateCapabilityViolation(
	ctx context.Context,
	agent *domain.Agent,
	actionType string,
	resource string,
	auditID uuid.UUID,
) (shouldBlock bool, shouldAlert bool, policyName string, err error) {
	// 1. Get active capability_violation policies for this organization
	policies, err := s.policyRepo.GetByType(agent.OrganizationID, domain.PolicyTypeCapabilityViolation)
	if err != nil {
		return false, false, "", fmt.Errorf("failed to fetch policies: %w", err)
	}

	// 2. If no policies configured, use safe defaults (block + alert)
	if len(policies) == 0 {
		fmt.Printf("⚠️  No security policies configured for org %s, using default: block + alert\n", agent.OrganizationID)
		return true, true, "default_policy", nil
	}

	// 3. Evaluate policies by priority (highest first)
	for _, policy := range policies {
		// Check if policy applies to this agent
		if !s.policyAppliesToAgent(policy, agent) {
			continue
		}

		// Policy matches - return enforcement action
		fmt.Printf("✅ Security Policy '%s' triggered for agent %s (action: %s)\n",
			policy.Name, agent.Name, policy.EnforcementAction)

		switch policy.EnforcementAction {
		case domain.EnforcementBlockAndAlert:
			return true, true, policy.Name, nil
		case domain.EnforcementAlertOnly:
			return false, true, policy.Name, nil
		case domain.EnforcementAllow:
			return false, false, policy.Name, nil
		default:
			// Unknown enforcement action - use safe default
			return true, true, policy.Name, nil
		}
	}

	// 4. No matching policy found - use safe default (block + alert)
	fmt.Printf("⚠️  No matching security policy for agent %s, using default: block + alert\n", agent.Name)
	return true, true, "default_policy", nil
}

// policyAppliesToAgent checks if a policy applies to a specific agent
func (s *SecurityPolicyService) policyAppliesToAgent(policy *domain.SecurityPolicy, agent *domain.Agent) bool {
	appliesTo := policy.AppliesTo

	// Apply to all agents
	if appliesTo == "all" {
		return true
	}

	// Apply to specific agent ID
	if strings.HasPrefix(appliesTo, "agent_id:") {
		targetID := strings.TrimPrefix(appliesTo, "agent_id:")
		return targetID == agent.ID.String()
	}

	// Apply to specific agent type
	if strings.HasPrefix(appliesTo, "agent_type:") {
		targetType := strings.TrimPrefix(appliesTo, "agent_type:")
		return targetType == string(agent.AgentType)
	}

	// Apply to agents with trust score below threshold
	if strings.HasPrefix(appliesTo, "trust_score_below:") {
		var threshold float64
		fmt.Sscanf(appliesTo, "trust_score_below:%f", &threshold)
		return agent.TrustScore < threshold
	}

	// Default: apply to all
	return true
}

// CreateDefaultPolicies creates default security policies for a new organization
func (s *SecurityPolicyService) CreateDefaultPolicies(ctx context.Context, orgID, userID uuid.UUID) error {
	// Default Policy 1: Block and Alert on Capability Violations (HIGH priority)
	// SECURITY: Default is to BLOCK unauthorized actions - this prevents attacks like EchoLeak (CVE-2025-32711)
	capabilityViolationPolicy := &domain.SecurityPolicy{
		OrganizationID:    orgID,
		Name:              "Block Capability Violations",
		Description:       "Block and alert on any capability violations (e.g., EchoLeak attacks). This prevents unauthorized actions that exceed an agent's registered capabilities. This is the secure default - admins can change to alert-only mode if needed.",
		PolicyType:        domain.PolicyTypeCapabilityViolation,
		EnforcementAction: domain.EnforcementBlockAndAlert,
		SeverityThreshold: domain.AlertSeverityHigh,
		Rules: map[string]interface{}{
			"attack_patterns": []string{"echoleak", "bulk_access", "data_exfiltration"},
		},
		AppliesTo: "all",
		IsEnabled: true,
		Priority:  1000, // Highest priority
		CreatedBy: userID,
	}

	if err := s.policyRepo.Create(capabilityViolationPolicy); err != nil {
		return fmt.Errorf("failed to create capability violation policy: %w", err)
	}

	// Default Policy 2: Alert Only for Low Trust Score Agents
	lowTrustPolicy := &domain.SecurityPolicy{
		OrganizationID:    orgID,
		Name:              "Monitor Low Trust Score Agents",
		Description:       "Generate alerts for agents with trust scores below 0.3 (30%). Does not block actions, but provides visibility into potentially risky agents.",
		PolicyType:        domain.PolicyTypeTrustScoreLow,
		EnforcementAction: domain.EnforcementAlertOnly,
		SeverityThreshold: domain.AlertSeverityWarning,
		Rules: map[string]interface{}{
			"trust_threshold": 0.3,
		},
		AppliesTo: "trust_score_below:0.3",
		IsEnabled: true,
		Priority:  500, // Medium priority
		CreatedBy: userID,
	}

	if err := s.policyRepo.Create(lowTrustPolicy); err != nil {
		return fmt.Errorf("failed to create low trust policy: %w", err)
	}

	// Default Policy 3: Alert on Data Exfiltration Attempts
	// NOTE: Default is alert-only. Admins can enable blocking with explicit confirmation.
	dataExfiltrationPolicy := &domain.SecurityPolicy{
		OrganizationID:    orgID,
		Name:              "Monitor Data Exfiltration",
		Description:       "Generate alerts on suspected data exfiltration attempts (e.g., external URL fetching, bulk data access). This monitors potential data leakage. Admins can enable blocking mode to prevent these actions.",
		PolicyType:        domain.PolicyTypeDataExfiltration,
		EnforcementAction: domain.EnforcementAlertOnly,
		SeverityThreshold: domain.AlertSeverityCritical,
		Rules: map[string]interface{}{
			"patterns": []string{"fetch_external_url", "bulk_export", "mass_download"},
		},
		AppliesTo: "all",
		IsEnabled: true,
		Priority:  900, // High priority
		CreatedBy: userID,
	}

	if err := s.policyRepo.Create(dataExfiltrationPolicy); err != nil {
		return fmt.Errorf("failed to create data exfiltration policy: %w", err)
	}

	fmt.Printf("✅ Created 3 default security policies for organization %s\n", orgID)
	return nil
}

// ListPolicies retrieves all security policies for an organization
func (s *SecurityPolicyService) ListPolicies(ctx context.Context, orgID uuid.UUID) ([]*domain.SecurityPolicy, error) {
	return s.policyRepo.GetByOrganization(orgID)
}

// GetPolicy retrieves a security policy by ID
func (s *SecurityPolicyService) GetPolicy(ctx context.Context, id uuid.UUID) (*domain.SecurityPolicy, error) {
	return s.policyRepo.GetByID(id)
}

// CreatePolicy creates a new security policy
func (s *SecurityPolicyService) CreatePolicy(ctx context.Context, policy *domain.SecurityPolicy) error {
	return s.policyRepo.Create(policy)
}

// UpdatePolicy updates a security policy
func (s *SecurityPolicyService) UpdatePolicy(ctx context.Context, policy *domain.SecurityPolicy) error {
	return s.policyRepo.Update(policy)
}

// DeletePolicy deletes a security policy
func (s *SecurityPolicyService) DeletePolicy(ctx context.Context, id uuid.UUID) error {
	return s.policyRepo.Delete(id)
}

// EnablePolicy enables a security policy
func (s *SecurityPolicyService) EnablePolicy(ctx context.Context, id uuid.UUID) error {
	policy, err := s.policyRepo.GetByID(id)
	if err != nil {
		return err
	}

	policy.IsEnabled = true
	return s.policyRepo.Update(policy)
}

// DisablePolicy disables a security policy
func (s *SecurityPolicyService) DisablePolicy(ctx context.Context, id uuid.UUID) error {
	policy, err := s.policyRepo.GetByID(id)
	if err != nil {
		return err
	}

	policy.IsEnabled = false
	return s.policyRepo.Update(policy)
}

// EvaluateTrustScoreLow evaluates security policies for low trust score agents
// Returns enforcement decision and whether to create an alert
func (s *SecurityPolicyService) EvaluateTrustScoreLow(
	ctx context.Context,
	agent *domain.Agent,
	actionType string,
	resource string,
	auditID uuid.UUID,
) (shouldBlock bool, shouldAlert bool, policyName string, err error) {
	// Get active trust_score_low policies for this organization
	policies, err := s.policyRepo.GetByType(agent.OrganizationID, domain.PolicyTypeTrustScoreLow)
	if err != nil {
		return false, false, "", fmt.Errorf("failed to fetch trust score policies: %w", err)
	}

	// If no policies configured, don't enforce (allow by default)
	if len(policies) == 0 {
		return false, false, "", nil
	}

	// Evaluate policies by priority (highest first)
	for _, policy := range policies {
		if !policy.IsEnabled {
			continue
		}

		// Check if policy applies to this agent
		if !s.policyAppliesToAgent(policy, agent) {
			continue
		}

		// Check trust score threshold from rules
		threshold, ok := policy.Rules["trust_threshold"].(float64)
		if !ok {
			threshold = 0.3 // Default threshold
		}

		// Trigger if agent trust score is below threshold
		if agent.TrustScore < threshold {
			fmt.Printf("✅ Trust Score Policy '%s' triggered for agent %s (score: %.2f < %.2f)\n",
				policy.Name, agent.Name, agent.TrustScore, threshold)

			switch policy.EnforcementAction {
			case domain.EnforcementBlockAndAlert:
				return true, true, policy.Name, nil
			case domain.EnforcementAlertOnly:
				return false, true, policy.Name, nil
			case domain.EnforcementAllow:
				return false, false, policy.Name, nil
			}
		}
	}

	// No policy triggered
	return false, false, "", nil
}

// TrustScoreEnforcementThresholds defines thresholds for trust score enforcement
const (
	TrustScoreThresholdWarning  = 0.70 // 70% - Generate warning alert
	TrustScoreThresholdCritical = 0.50 // 50% - Suspend agent and create critical alert
)

// TrustScoreEnforcementResult contains the result of trust score evaluation
type TrustScoreEnforcementResult struct {
	ShouldAlert   bool
	ShouldSuspend bool
	AlertSeverity domain.AlertSeverity
	PolicyName    string
	Message       string
}

// EvaluateTrustScoreOnUpdate evaluates trust score policies after a score update.
// This is called after every trust score change (recalculation, manual update, violation impact).
//
// Enforcement rules:
// - Score < 70%: Create warning alert (agent continues to operate)
// - Score < 50%: Create critical alert AND suspend agent
//
// Returns enforcement result with actions to take.
func (s *SecurityPolicyService) EvaluateTrustScoreOnUpdate(
	ctx context.Context,
	agent *domain.Agent,
	previousScore float64,
	currentScore float64,
) (*TrustScoreEnforcementResult, error) {
	result := &TrustScoreEnforcementResult{}

	// Only evaluate if score dropped (don't alert on improvements)
	if currentScore >= previousScore {
		return result, nil
	}

	// Only evaluate active/verified agents
	if agent.Status != domain.AgentStatusVerified && agent.Status != domain.AgentStatusPending {
		return result, nil
	}

	// Check for critical threshold breach (< 50%)
	if currentScore < TrustScoreThresholdCritical {
		result.ShouldAlert = true
		result.ShouldSuspend = true
		result.AlertSeverity = domain.SeverityCritical
		result.PolicyName = "Critical Trust Score Enforcement"
		result.Message = fmt.Sprintf(
			"Agent trust score dropped to %.1f%% (below critical threshold of %.0f%%). Agent has been automatically suspended.",
			currentScore*100, TrustScoreThresholdCritical*100,
		)

		// Create critical alert
		if err := s.createTrustScoreAlert(ctx, agent, result); err != nil {
			fmt.Printf("⚠️  Failed to create critical trust score alert: %v\n", err)
		}

		// Suspend the agent
		if err := s.suspendAgentForLowTrustScore(ctx, agent); err != nil {
			fmt.Printf("⚠️  Failed to suspend agent %s: %v\n", agent.Name, err)
		} else {
			fmt.Printf("🛑 Agent '%s' suspended due to critical trust score (%.1f%%)\n",
				agent.Name, currentScore*100)
		}

		return result, nil
	}

	// Check for warning threshold breach (< 70%)
	if currentScore < TrustScoreThresholdWarning {
		result.ShouldAlert = true
		result.ShouldSuspend = false
		result.AlertSeverity = domain.SeverityWarning
		result.PolicyName = "Low Trust Score Warning"
		result.Message = fmt.Sprintf(
			"Agent trust score dropped to %.1f%% (below warning threshold of %.0f%%). Consider investigating recent activity.",
			currentScore*100, TrustScoreThresholdWarning*100,
		)

		// Create warning alert
		if err := s.createTrustScoreAlert(ctx, agent, result); err != nil {
			fmt.Printf("⚠️  Failed to create trust score warning alert: %v\n", err)
		} else {
			fmt.Printf("⚠️  Low trust score alert created for agent '%s' (%.1f%%)\n",
				agent.Name, currentScore*100)
		}

		return result, nil
	}

	return result, nil
}

// createTrustScoreAlert creates an alert for trust score threshold breach
func (s *SecurityPolicyService) createTrustScoreAlert(
	ctx context.Context,
	agent *domain.Agent,
	result *TrustScoreEnforcementResult,
) error {
	// Check for recent duplicate alerts (within last hour)
	recentAlerts, err := s.alertRepo.GetByOrganization(agent.OrganizationID, 50, 0)
	if err == nil {
		cutoff := time.Now().Add(-1 * time.Hour)
		for _, existing := range recentAlerts {
			if existing.ResourceID == agent.ID &&
				existing.AlertType == domain.AlertTrustScoreLow &&
				existing.CreatedAt.After(cutoff) {
				// Duplicate alert within the last hour, skip
				return nil
			}
		}
	}

	alert := &domain.Alert{
		OrganizationID: agent.OrganizationID,
		AlertType:      domain.AlertTrustScoreLow,
		Severity:       result.AlertSeverity,
		Title:          fmt.Sprintf("%s: %s", result.PolicyName, agent.DisplayName),
		Description:    result.Message,
		ResourceType:   "agent",
		ResourceID:     agent.ID,
		Metadata: map[string]interface{}{
			"agentName":        agent.Name,
			"trustScore":       agent.TrustScore,
			"policyName":       result.PolicyName,
			"actionTaken":      result.ShouldSuspend,
			"enforcementLevel": string(result.AlertSeverity),
		},
	}

	return s.alertRepo.Create(alert)
}

// suspendAgentForLowTrustScore suspends an agent due to critical trust score
func (s *SecurityPolicyService) suspendAgentForLowTrustScore(ctx context.Context, agent *domain.Agent) error {
	if s.agentRepo == nil {
		return fmt.Errorf("agent repository not configured")
	}

	agent.Status = domain.AgentStatusSuspended
	return s.agentRepo.Update(agent)
}

// EvaluateUnusualActivity evaluates security policies for unusual activity patterns
// Uses baseline-based statistical detection for accurate anomaly identification
// Returns enforcement decision and whether to create an alert
func (s *SecurityPolicyService) EvaluateUnusualActivity(
	ctx context.Context,
	agent *domain.Agent,
	actionType string,
	resource string,
	auditID uuid.UUID,
) (shouldBlock bool, shouldAlert bool, policyName string, err error) {
	// ============================================================================
	// INTELLIGENT BEHAVIORAL ANALYSIS (Primary Detection Method)
	// ============================================================================
	// Uses per-agent learned baselines for statistical anomaly detection
	if s.behaviorAnalysis != nil {
		// First, record this activity to update the agent's baseline
		if err := s.behaviorAnalysis.RecordActivity(ctx, agent.ID, agent.OrganizationID, actionType, resource, actionType); err != nil {
			fmt.Printf("⚠️  Failed to record activity for agent %s: %v\n", agent.Name, err)
			// Don't fail - continue with detection
		}

		// Run intelligent anomaly detection against learned baseline
		result, err := s.behaviorAnalysis.DetectAnomalies(ctx, agent.ID, agent.OrganizationID, actionType, resource, actionType)
		if err != nil {
			fmt.Printf("⚠️  Behavioral analysis failed for agent %s: %v\n", agent.Name, err)
			// Fall through to legacy detection
		} else if result.IsAnomalous {
			// Get the policy for enforcement action
			policies, err := s.policyRepo.GetByType(agent.OrganizationID, domain.PolicyTypeUnusualActivity)
			if err == nil && len(policies) > 0 {
				// Use first enabled policy for enforcement
				for _, policy := range policies {
					if policy.IsEnabled && s.policyAppliesToAgent(policy, agent) {
						// Log the intelligent detection
						fmt.Printf("🧠 INTELLIGENT DETECTION: Agent %s - %s\n", agent.Name, result.Reason)
						for _, anomaly := range result.Anomalies {
							fmt.Printf("   ↳ %s [%s]: %s\n", anomaly.AnomalyType, anomaly.Severity, anomaly.Description)
						}

						switch policy.EnforcementAction {
						case domain.EnforcementBlockAndAlert:
							return result.ShouldBlock || true, true, policy.Name + " (Intelligent Detection)", nil
						case domain.EnforcementAlertOnly:
							return false, true, policy.Name + " (Intelligent Detection)", nil
						case domain.EnforcementAllow:
							// Log but don't enforce
							return false, false, policy.Name + " (Intelligent Detection - Logged)", nil
						}
					}
				}
			}

			// No policy configured but anomaly detected - use defaults
			if result.ShouldBlock {
				return true, true, "intelligent_detection_block", nil
			}
			if result.ShouldAlert {
				return false, true, "intelligent_detection_alert", nil
			}
		} else if result.Reason != "" {
			// Baseline learning in progress - log and allow
			fmt.Printf("📊 Baseline: Agent %s - %s\n", agent.Name, result.Reason)
			return false, false, "", nil
		}
	}

	// ============================================================================
	// LEGACY DETECTION (Fallback when intelligent analysis not available)
	// ============================================================================
	// Note: This code is kept for backwards compatibility but is not recommended
	// The intelligent baseline-based detection above should be preferred

	// Get active unusual_activity policies for this organization
	policies, err := s.policyRepo.GetByType(agent.OrganizationID, domain.PolicyTypeUnusualActivity)
	if err != nil {
		return false, false, "", fmt.Errorf("failed to fetch unusual activity policies: %w", err)
	}

	// If no policies configured, don't enforce
	if len(policies) == 0 {
		return false, false, "", nil
	}

	// BASELINE CHECK: Skip legacy detection for new agents
	const minAgentAgeHours = 24
	const minVerifications = 10

	agentAge := time.Since(agent.CreatedAt)
	if agentAge < time.Duration(minAgentAgeHours)*time.Hour {
		fmt.Printf("ℹ️  Skipping legacy unusual activity check for new agent %s (age: %v < %dh)\n",
			agent.Name, agentAge.Round(time.Minute), minAgentAgeHours)
		return false, false, "", nil
	}

	recentActions, err := s.auditLogRepo.GetRecentActionsByAgent(agent.ID, minVerifications+1)
	if err == nil && len(recentActions) < minVerifications {
		fmt.Printf("ℹ️  Skipping legacy unusual activity check for agent %s (only %d verifications, need %d)\n",
			agent.Name, len(recentActions), minVerifications)
		return false, false, "", nil
	}

	// Evaluate policies by priority (highest first)
	for _, policy := range policies {
		if !policy.IsEnabled {
			continue
		}

		if !s.policyAppliesToAgent(policy, agent) {
			continue
		}

		// Legacy: Check for API rate spikes (fixed threshold)
		if apiRateThreshold, ok := policy.Rules["api_rate_threshold"].(float64); ok {
			timeWindowMinutes, _ := policy.Rules["time_window_minutes"].(float64)
			if timeWindowMinutes == 0 {
				timeWindowMinutes = 60
			}

			actionCount, err := s.auditLogRepo.CountActionsByAgentInTimeWindow(
				agent.ID,
				domain.AuditAction(actionType),
				int(timeWindowMinutes),
			)
			if err != nil {
				fmt.Printf("⚠️  Failed to count actions for agent %s: %v\n", agent.Name, err)
				continue
			}

			if actionCount > int(apiRateThreshold) {
				fmt.Printf("⚠️  LEGACY Detection: API rate spike (count: %d > threshold: %.0f)\n",
					actionCount, apiRateThreshold)

				switch policy.EnforcementAction {
				case domain.EnforcementBlockAndAlert:
					return true, true, policy.Name + " (Legacy)", nil
				case domain.EnforcementAlertOnly:
					return false, true, policy.Name + " (Legacy)", nil
				case domain.EnforcementAllow:
					return false, false, policy.Name + " (Legacy)", nil
				}
			}
		}
	}

	return false, false, "", nil
}

// EvaluateDataExfiltration evaluates security policies for data exfiltration attempts
// Returns enforcement decision and whether to create an alert
func (s *SecurityPolicyService) EvaluateDataExfiltration(
	ctx context.Context,
	agent *domain.Agent,
	actionType string,
	resource string,
	auditID uuid.UUID,
) (shouldBlock bool, shouldAlert bool, policyName string, err error) {
	// Get active data_exfiltration policies for this organization
	policies, err := s.policyRepo.GetByType(agent.OrganizationID, domain.PolicyTypeDataExfiltration)
	if err != nil {
		return false, false, "", fmt.Errorf("failed to fetch data exfiltration policies: %w", err)
	}

	// If no policies configured, don't enforce
	if len(policies) == 0 {
		return false, false, "", nil
	}

	// Evaluate policies by priority (highest first)
	for _, policy := range policies {
		if !policy.IsEnabled {
			continue
		}

		// Check if policy applies to this agent
		if !s.policyAppliesToAgent(policy, agent) {
			continue
		}

		// Check for data exfiltration patterns in action
		patterns, ok := policy.Rules["patterns"].([]interface{})
		if ok {
			for _, p := range patterns {
				pattern, ok := p.(string)
				if !ok {
					continue
				}

				// Check if action matches exfiltration pattern
				if strings.Contains(strings.ToLower(actionType), pattern) ||
					strings.Contains(strings.ToLower(resource), pattern) {
					fmt.Printf("✅ Data Exfiltration Policy '%s' triggered for agent %s (pattern: %s)\n",
						policy.Name, agent.Name, pattern)

					switch policy.EnforcementAction {
					case domain.EnforcementBlockAndAlert:
						return true, true, policy.Name, nil
					case domain.EnforcementAlertOnly:
						return false, true, policy.Name, nil
					case domain.EnforcementAllow:
						return false, false, policy.Name, nil
					}
				}
			}
		}
	}

	return false, false, "", nil
}

// EvaluateConfigDrift evaluates security policies for configuration drift
// Returns enforcement decision and whether to create an alert
func (s *SecurityPolicyService) EvaluateConfigDrift(
	ctx context.Context,
	agent *domain.Agent,
	actionType string,
	resource string,
	auditID uuid.UUID,
) (shouldBlock bool, shouldAlert bool, policyName string, err error) {
	// Get active config_drift policies for this organization
	policies, err := s.policyRepo.GetByType(agent.OrganizationID, domain.PolicyTypeConfigDrift)
	if err != nil {
		return false, false, "", fmt.Errorf("failed to fetch config drift policies: %w", err)
	}

	// If no policies configured, don't enforce
	if len(policies) == 0 {
		return false, false, "", nil
	}

	// Evaluate policies by priority (highest first)
	for _, policy := range policies {
		if !policy.IsEnabled {
			continue
		}

		// Check if policy applies to this agent
		if !s.policyAppliesToAgent(policy, agent) {
			continue
		}

		// Check for capability changes (compare current vs. baseline)
		if checkCapabilityChanges, ok := policy.Rules["check_capability_changes"].(bool); ok && checkCapabilityChanges {
			// Baseline capabilities are stored in policy rules
			baselineCapabilities, ok := policy.Rules["baseline_capabilities"].([]interface{})
			if ok && len(baselineCapabilities) > 0 {
				// Convert to string slice
				baseline := make(map[string]bool)
				for _, cap := range baselineCapabilities {
					if capStr, ok := cap.(string); ok {
						baseline[capStr] = true
					}
				}

				// Check for added or removed capabilities
				currentCaps := make(map[string]bool)
				for _, cap := range agent.Capabilities {
					currentCaps[cap] = true
				}

				// Detect new capabilities (not in baseline)
				var addedCaps []string
				for cap := range currentCaps {
					if !baseline[cap] {
						addedCaps = append(addedCaps, cap)
					}
				}

				// Detect removed capabilities (in baseline but not current)
				var removedCaps []string
				for cap := range baseline {
					if !currentCaps[cap] {
						removedCaps = append(removedCaps, cap)
					}
				}

				if len(addedCaps) > 0 || len(removedCaps) > 0 {
					fmt.Printf("✅ Config Drift Policy '%s' triggered: Capability changes detected (added: %v, removed: %v)\n",
						policy.Name, addedCaps, removedCaps)

					switch policy.EnforcementAction {
					case domain.EnforcementBlockAndAlert:
						return true, true, policy.Name, nil
					case domain.EnforcementAlertOnly:
						return false, true, policy.Name, nil
					case domain.EnforcementAllow:
						return false, false, policy.Name, nil
					}
				}
			}
		}

		// Check for public key rotations
		if checkKeyRotations, ok := policy.Rules["check_key_rotations"].(bool); ok && checkKeyRotations {
			// Get recent audit logs for this agent to detect key changes
			recentActions, err := s.auditLogRepo.GetRecentActionsByAgent(agent.ID, 50)
			if err != nil {
				fmt.Printf("⚠️  Failed to get recent actions for agent %s: %v\n", agent.Name, err)
				continue
			}

			// Check for key update actions in recent history
			for _, action := range recentActions {
				if action.Action == domain.AuditActionUpdate {
					// Check metadata for public_key_changed flag
					if metadata, ok := action.Metadata["public_key_changed"].(bool); ok && metadata {
						// Check if this key rotation was approved
						requireApproval, _ := policy.Rules["require_key_rotation_approval"].(bool)
						if requireApproval {
							// Check if approval metadata exists
							if approved, ok := action.Metadata["key_rotation_approved"].(bool); !ok || !approved {
								fmt.Printf("✅ Config Drift Policy '%s' triggered: Unapproved public key rotation detected\n",
									policy.Name)

								switch policy.EnforcementAction {
								case domain.EnforcementBlockAndAlert:
									return true, true, policy.Name, nil
								case domain.EnforcementAlertOnly:
									return false, true, policy.Name, nil
								case domain.EnforcementAllow:
									return false, false, policy.Name, nil
								}
							}
						}
					}
				}
			}
		}

		// Check for permission escalations
		if checkPermissionEscalation, ok := policy.Rules["check_permission_escalation"].(bool); ok && checkPermissionEscalation {
			// Compare current capabilities against high-privilege capability patterns
			dangerousCapabilities, ok := policy.Rules["dangerous_capabilities"].([]interface{})
			if !ok || len(dangerousCapabilities) == 0 {
				// Default dangerous capabilities
				dangerousCapabilities = []interface{}{
					"admin:*",
					"*:delete",
					"system:*",
					"security:*",
				}
			}

			// Check if agent has any dangerous capabilities
			var foundDangerousCaps []string
			for _, cap := range agent.Capabilities {
				for _, dangerousCap := range dangerousCapabilities {
					if dangerousCapStr, ok := dangerousCap.(string); ok {
						// Simple wildcard matching
						if strings.HasSuffix(dangerousCapStr, "*") {
							prefix := strings.TrimSuffix(dangerousCapStr, "*")
							if strings.HasPrefix(cap, prefix) {
								foundDangerousCaps = append(foundDangerousCaps, cap)
								break
							}
						} else if cap == dangerousCapStr {
							foundDangerousCaps = append(foundDangerousCaps, cap)
							break
						}
					}
				}
			}

			if len(foundDangerousCaps) > 0 {
				fmt.Printf("✅ Config Drift Policy '%s' triggered: Dangerous capabilities detected: %v\n",
					policy.Name, foundDangerousCaps)

				switch policy.EnforcementAction {
				case domain.EnforcementBlockAndAlert:
					return true, true, policy.Name, nil
				case domain.EnforcementAlertOnly:
					return false, true, policy.Name, nil
				case domain.EnforcementAllow:
					return false, false, policy.Name, nil
				}
			}
		}
	}

	return false, false, "", nil
}

// EvaluateUnauthorizedAccess evaluates security policies for unauthorized access attempts
// Returns enforcement decision and whether to create an alert
func (s *SecurityPolicyService) EvaluateUnauthorizedAccess(
	ctx context.Context,
	agent *domain.Agent,
	actionType string,
	resource string,
	auditID uuid.UUID,
) (shouldBlock bool, shouldAlert bool, policyName string, err error) {
	// Get active unauthorized_access policies for this organization
	policies, err := s.policyRepo.GetByType(agent.OrganizationID, domain.PolicyTypeUnauthorizedAccess)
	if err != nil {
		return false, false, "", fmt.Errorf("failed to fetch unauthorized access policies: %w", err)
	}

	// If no policies configured, don't enforce
	if len(policies) == 0 {
		return false, false, "", nil
	}

	// Get the current audit log to extract IP address
	var currentIPAddress string

	// Try to get the audit log that triggered this evaluation
	if auditID != uuid.Nil {
		recentActions, err := s.auditLogRepo.GetRecentActionsByAgent(agent.ID, 10)
		if err == nil {
			for _, action := range recentActions {
				if action.ID == auditID {
					currentIPAddress = action.IPAddress
					break
				}
			}
		}
	}

	// Evaluate policies by priority (highest first)
	for _, policy := range policies {
		if !policy.IsEnabled {
			continue
		}

		// Check if policy applies to this agent
		if !s.policyAppliesToAgent(policy, agent) {
			continue
		}

		// Check for IP-based restrictions
		if checkIPRestrictions, ok := policy.Rules["check_ip_restrictions"].(bool); ok && checkIPRestrictions {
			allowedIPs, ok := policy.Rules["allowed_ips"].([]interface{})
			if ok && len(allowedIPs) > 0 && currentIPAddress != "" {
				// Check if current IP is in allowed list
				isAllowed := false
				for _, allowedIP := range allowedIPs {
					if allowedIPStr, ok := allowedIP.(string); ok {
						// Simple exact match (could be extended to support CIDR ranges)
						if currentIPAddress == allowedIPStr {
							isAllowed = true
							break
						}
						// Support wildcard matching (e.g., "192.168.*")
						if strings.HasSuffix(allowedIPStr, "*") {
							prefix := strings.TrimSuffix(allowedIPStr, "*")
							if strings.HasPrefix(currentIPAddress, prefix) {
								isAllowed = true
								break
							}
						}
					}
				}

				if !isAllowed {
					fmt.Printf("✅ Unauthorized Access Policy '%s' triggered: IP address %s not in allowed list\n",
						policy.Name, currentIPAddress)

					switch policy.EnforcementAction {
					case domain.EnforcementBlockAndAlert:
						return true, true, policy.Name, nil
					case domain.EnforcementAlertOnly:
						return false, true, policy.Name, nil
					case domain.EnforcementAllow:
						return false, false, policy.Name, nil
					}
				}
			}
		}

		// Check for time-based access restrictions
		if checkTimeRestrictions, ok := policy.Rules["check_time_restrictions"].(bool); ok && checkTimeRestrictions {
			allowedDays, _ := policy.Rules["allowed_days"].([]interface{})
			allowedHoursStart, _ := policy.Rules["allowed_hours_start"].(float64)
			allowedHoursEnd, _ := policy.Rules["allowed_hours_end"].(float64)

			now := time.Now()
			currentDay := now.Weekday().String()
			currentHour := now.Hour()

			// Check day restrictions
			if len(allowedDays) > 0 {
				isDayAllowed := false
				for _, day := range allowedDays {
					if dayStr, ok := day.(string); ok && strings.EqualFold(dayStr, currentDay) {
						isDayAllowed = true
						break
					}
				}

				if !isDayAllowed {
					fmt.Printf("✅ Unauthorized Access Policy '%s' triggered: Access not allowed on %s\n",
						policy.Name, currentDay)

					switch policy.EnforcementAction {
					case domain.EnforcementBlockAndAlert:
						return true, true, policy.Name, nil
					case domain.EnforcementAlertOnly:
						return false, true, policy.Name, nil
					case domain.EnforcementAllow:
						return false, false, policy.Name, nil
					}
				}
			}

			// Check hour restrictions
			if allowedHoursStart > 0 || allowedHoursEnd > 0 {
				if allowedHoursEnd == 0 {
					allowedHoursEnd = 24
				}

				if currentHour < int(allowedHoursStart) || currentHour >= int(allowedHoursEnd) {
					fmt.Printf("✅ Unauthorized Access Policy '%s' triggered: Access not allowed at hour %d (allowed: %.0f-%.0f)\n",
						policy.Name, currentHour, allowedHoursStart, allowedHoursEnd)

					switch policy.EnforcementAction {
					case domain.EnforcementBlockAndAlert:
						return true, true, policy.Name, nil
					case domain.EnforcementAlertOnly:
						return false, true, policy.Name, nil
					case domain.EnforcementAllow:
						return false, false, policy.Name, nil
					}
				}
			}
		}

		// Check for resource-level access control
		if checkResourceAccess, ok := policy.Rules["check_resource_access"].(bool); ok && checkResourceAccess {
			restrictedResources, ok := policy.Rules["restricted_resources"].([]interface{})
			if ok && len(restrictedResources) > 0 {
				// Check if current resource is in restricted list
				for _, restrictedResource := range restrictedResources {
					if restrictedResourceStr, ok := restrictedResource.(string); ok {
						// Simple pattern matching
						if strings.Contains(resource, restrictedResourceStr) ||
							strings.Contains(restrictedResourceStr, resource) {
							fmt.Printf("✅ Unauthorized Access Policy '%s' triggered: Access to restricted resource %s\n",
								policy.Name, resource)

							switch policy.EnforcementAction {
							case domain.EnforcementBlockAndAlert:
								return true, true, policy.Name, nil
							case domain.EnforcementAlertOnly:
								return false, true, policy.Name, nil
							case domain.EnforcementAllow:
								return false, false, policy.Name, nil
							}
						}
					}
				}
			}
		}

		// Check for action-level restrictions
		if checkActionRestrictions, ok := policy.Rules["check_action_restrictions"].(bool); ok && checkActionRestrictions {
			restrictedActions, ok := policy.Rules["restricted_actions"].([]interface{})
			if ok && len(restrictedActions) > 0 {
				// Check if current action is in restricted list
				for _, restrictedAction := range restrictedActions {
					if restrictedActionStr, ok := restrictedAction.(string); ok {
						if strings.EqualFold(actionType, restrictedActionStr) {
							fmt.Printf("✅ Unauthorized Access Policy '%s' triggered: Restricted action %s attempted\n",
								policy.Name, actionType)

							switch policy.EnforcementAction {
							case domain.EnforcementBlockAndAlert:
								return true, true, policy.Name, nil
							case domain.EnforcementAlertOnly:
								return false, true, policy.Name, nil
							case domain.EnforcementAllow:
								return false, false, policy.Name, nil
							}
						}
					}
				}
			}
		}
	}

	return false, false, "", nil
}

// EvaluateAuthFailures handles authentication failure events and creates appropriate alerts.
// This is called by AuthService when:
// - failureCount >= AlertThreshold (default 3): Creates a warning alert about multiple failures
// - isLocked == true: Creates a high-severity alert that account was locked
//
// This enables security teams to:
// - Monitor for brute force attack patterns
// - Track account lockouts across the organization
// - Investigate repeated authentication failures
func (s *SecurityPolicyService) EvaluateAuthFailures(email string, orgID uuid.UUID, failureCount int, isLocked bool) {
	if s.alertRepo == nil {
		fmt.Printf("⚠️  Alert repository not configured, skipping auth failure alert\n")
		return
	}

	// Check for recent duplicate alerts (within last 15 minutes for failures, 1 hour for lockouts)
	recentAlerts, err := s.alertRepo.GetByOrganization(orgID, 50, 0)
	if err == nil {
		var cutoff time.Time
		if isLocked {
			cutoff = time.Now().Add(-1 * time.Hour)
		} else {
			cutoff = time.Now().Add(-15 * time.Minute)
		}

		for _, existing := range recentAlerts {
			// Check if there's a recent similar alert for this email
			if existingEmail, ok := existing.Metadata["email"].(string); ok && existingEmail == email {
				if (isLocked && existing.AlertType == domain.AlertAccountLocked) ||
					(!isLocked && existing.AlertType == domain.AlertAuthFailurePattern) {
					if existing.CreatedAt.After(cutoff) {
						// Duplicate alert within the cooldown period, skip
						return
					}
				}
			}
		}
	}

	if isLocked {
		// Account locked - high severity alert
		alert := &domain.Alert{
			OrganizationID: orgID,
			AlertType:      domain.AlertAccountLocked,
			Severity:       domain.AlertSeverityHigh,
			Title:          fmt.Sprintf("Account Locked: %s", email),
			Description:    fmt.Sprintf("Account for '%s' has been locked after %d failed authentication attempts. This may indicate a brute force attack. The account will be automatically unlocked after the lockout period, or can be manually unlocked by an administrator.", email, failureCount),
			ResourceType:   "user",
			ResourceID:     uuid.Nil, // We don't have user ID for unknown users
			Metadata: map[string]interface{}{
				"email":          email,
				"failureCount":   failureCount,
				"eventType":      "account_locked",
				"recommendation": "Review authentication logs and consider blocking the source IP if this appears malicious",
			},
		}

		if err := s.alertRepo.Create(alert); err != nil {
			fmt.Printf("⚠️  Failed to create account locked alert: %v\n", err)
		} else {
			fmt.Printf("🔒 Account locked alert created for %s (after %d failures)\n", email, failureCount)
		}
	} else {
		// Multiple failures but not yet locked - warning alert
		alert := &domain.Alert{
			OrganizationID: orgID,
			AlertType:      domain.AlertAuthFailurePattern,
			Severity:       domain.AlertSeverityWarning,
			Title:          fmt.Sprintf("Multiple Auth Failures: %s", email),
			Description:    fmt.Sprintf("Detected %d failed authentication attempts for '%s'. This may indicate a brute force attempt. Account will be locked after additional failures.", failureCount, email),
			ResourceType:   "user",
			ResourceID:     uuid.Nil,
			Metadata: map[string]interface{}{
				"email":          email,
				"failureCount":   failureCount,
				"eventType":      "auth_failure_pattern",
				"recommendation": "Monitor for additional failures and review source IPs",
			},
		}

		if err := s.alertRepo.Create(alert); err != nil {
			fmt.Printf("⚠️  Failed to create auth failure pattern alert: %v\n", err)
		} else {
			fmt.Printf("⚠️  Auth failure pattern alert created for %s (%d failures)\n", email, failureCount)
		}
	}
}
