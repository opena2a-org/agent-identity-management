package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
)

// SecurityPolicyService handles security policy evaluation and management
type SecurityPolicyService struct {
	policyRepo domain.SecurityPolicyRepository
	alertRepo  domain.AlertRepository
}

// NewSecurityPolicyService creates a new security policy service
func NewSecurityPolicyService(
	policyRepo domain.SecurityPolicyRepository,
	alertRepo domain.AlertRepository,
) *SecurityPolicyService {
	return &SecurityPolicyService{
		policyRepo: policyRepo,
		alertRepo:  alertRepo,
	}
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
	// Default Policy 1: Alert on Capability Violations (HIGH priority)
	// NOTE: Default is alert-only. Admins can enable blocking with explicit confirmation.
	capabilityViolationPolicy := &domain.SecurityPolicy{
		OrganizationID:    orgID,
		Name:              "Monitor Capability Violations",
		Description:       "Generate alerts on any capability violations (e.g., EchoLeak attacks). This monitors unauthorized actions that exceed an agent's registered capabilities. Admins can enable blocking mode to prevent these actions.",
		PolicyType:        domain.PolicyTypeCapabilityViolation,
		EnforcementAction: domain.EnforcementAlertOnly,
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

// EvaluateUnusualActivity evaluates security policies for unusual activity patterns
// Returns enforcement decision and whether to create an alert
func (s *SecurityPolicyService) EvaluateUnusualActivity(
	ctx context.Context,
	agent *domain.Agent,
	actionType string,
	resource string,
	auditID uuid.UUID,
) (shouldBlock bool, shouldAlert bool, policyName string, err error) {
	// Get active unusual_activity policies for this organization
	policies, err := s.policyRepo.GetByType(agent.OrganizationID, domain.PolicyTypeUnusualActivity)
	if err != nil {
		return false, false, "", fmt.Errorf("failed to fetch unusual activity policies: %w", err)
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

		// Note: This policy type is available for future enhancement.
		// Currently acts as a placeholder for custom anomaly detection.
		// Admins can configure it but it won't trigger unless extended.

		fmt.Printf("✅ Unusual Activity Policy '%s' evaluated for agent %s (currently inactive)\n",
			policy.Name, agent.Name)
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

		// Note: This policy type is available for future enhancement.
		// Could be extended to detect capability changes, key rotations, etc.
		// Admins can configure it but it won't trigger unless extended.

		fmt.Printf("✅ Config Drift Policy '%s' evaluated for agent %s (currently inactive)\n",
			policy.Name, agent.Name)
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

	// Evaluate policies by priority (highest first)
	for _, policy := range policies {
		if !policy.IsEnabled {
			continue
		}

		// Check if policy applies to this agent
		if !s.policyAppliesToAgent(policy, agent) {
			continue
		}

		// Note: Most unauthorized access is handled by capability_violation policy.
		// This policy type is available for future custom access control rules.
		// Admins can configure it but it won't trigger unless extended.

		fmt.Printf("✅ Unauthorized Access Policy '%s' evaluated for agent %s (currently inactive)\n",
			policy.Name, agent.Name)
	}

	return false, false, "", nil
}
