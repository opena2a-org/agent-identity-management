package application

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/google/uuid"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// MCPPolicyEvaluator evaluates MCP servers against security policies.
//
// NOT WIRED. Nothing outside _test.go constructs this type, so no request is gated by it and no
// mcp_* policy is enforced anywhere in the running system. SecurityPolicyService, the service
// built in cmd/server/main.go, reaches only the six non-MCP policy types through
// policyRepo.GetByType. The type is kept rather than deleted because the rest of the feature did
// ship -- the admin security-policy routes are mounted, the admin page creates and toggles all
// four mcp_* types, and migration 052 seeded six such policies -- so deleting the engine would
// leave every promise in place with no implementation left to honour it.
//
// Migration 105 disables the seeded policies so the surface stops presenting enforcement that
// does not happen. Three things must be fixed before this can be wired, or it will reject every
// MCP server:
//
//  1. security_policies.rules.minTrustScore is seeded and stored 0-100, while evaluateAllowlist
//     compares it against MCPServer.TrustScore, which migration 104 made canonical [0,1].
//  2. matchDomainPattern does not treat a bare "*" as a wildcard -- only the "*." prefix is
//     special-cased -- so the seeded 'allowedDomains': ['*'] matches no host at all.
//  3. MCPAllowlistRules.AllowedCapabilities is declared and read by nothing, so an organization
//     that configures it gets silent non-enforcement.
//
// Tracked in https://github.com/opena2a-org/agent-identity-management/issues/355.
type MCPPolicyEvaluator struct {
	policyRepo domain.SecurityPolicyRepository
	mcpRepo    domain.MCPServerRepository
}

// NewMCPPolicyEvaluator creates a new MCP policy evaluator
func NewMCPPolicyEvaluator(
	policyRepo domain.SecurityPolicyRepository,
	mcpRepo domain.MCPServerRepository,
) *MCPPolicyEvaluator {
	return &MCPPolicyEvaluator{
		policyRepo: policyRepo,
		mcpRepo:    mcpRepo,
	}
}

// EvaluateMCPServer evaluates an MCP server against all applicable policies
func (e *MCPPolicyEvaluator) EvaluateMCPServer(
	ctx context.Context,
	mcpServer *domain.MCPServer,
) ([]*domain.MCPPolicyEvaluationResult, error) {
	// Get all active policies for the organization
	policies, err := e.policyRepo.GetActiveByOrganization(mcpServer.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("failed to get active policies: %w", err)
	}

	results := make([]*domain.MCPPolicyEvaluationResult, 0)

	// Evaluate each MCP-related policy
	for _, policy := range policies {
		if !isMCPPolicy(policy.PolicyType) {
			continue
		}

		result, err := e.evaluatePolicy(ctx, mcpServer, policy)
		if err != nil {
			// Log error but continue with other policies
			continue
		}

		results = append(results, result)
	}

	return results, nil
}

// EvaluateAllMCPServers evaluates all MCP servers in an organization against policies
func (e *MCPPolicyEvaluator) EvaluateAllMCPServers(
	ctx context.Context,
	orgID uuid.UUID,
) (map[uuid.UUID][]*domain.MCPPolicyEvaluationResult, error) {
	// Get all MCP servers
	servers, err := e.mcpRepo.GetByOrganization(orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to list MCP servers: %w", err)
	}

	results := make(map[uuid.UUID][]*domain.MCPPolicyEvaluationResult)

	for _, server := range servers {
		serverResults, err := e.EvaluateMCPServer(ctx, server)
		if err != nil {
			continue
		}
		results[server.ID] = serverResults
	}

	return results, nil
}

// evaluatePolicy evaluates a single policy against an MCP server
func (e *MCPPolicyEvaluator) evaluatePolicy(
	ctx context.Context,
	mcpServer *domain.MCPServer,
	policy *domain.SecurityPolicy,
) (*domain.MCPPolicyEvaluationResult, error) {
	result := &domain.MCPPolicyEvaluationResult{
		PolicyEvaluationResult: domain.PolicyEvaluationResult{
			PolicyID:          policy.ID,
			PolicyName:        policy.Name,
			EnforcementAction: policy.EnforcementAction,
			Triggered:         false,
			ShouldBlock:       false,
			ShouldAlert:       false,
		},
		MCPServerID:   mcpServer.ID,
		MCPServerName: mcpServer.Name,
		MCPServerURL:  mcpServer.URL,
	}

	switch policy.PolicyType {
	case domain.PolicyTypeMCPAllowlist:
		e.evaluateAllowlist(mcpServer, policy, result)
	case domain.PolicyTypeMCPBlocklist:
		e.evaluateBlocklist(mcpServer, policy, result)
	case domain.PolicyTypeMCPCapabilities:
		e.evaluateCapabilities(mcpServer, policy, result)
	case domain.PolicyTypeMCPUnverified:
		e.evaluateUnverified(mcpServer, policy, result)
	}

	// Set enforcement actions based on policy
	if result.Triggered {
		result.ShouldAlert = policy.EnforcementAction == domain.EnforcementAlertOnly ||
			policy.EnforcementAction == domain.EnforcementBlockAndAlert
		result.ShouldBlock = policy.EnforcementAction == domain.EnforcementBlockAndAlert
	}

	return result, nil
}

// evaluateAllowlist checks if MCP server is in the allowlist
func (e *MCPPolicyEvaluator) evaluateAllowlist(
	mcpServer *domain.MCPServer,
	policy *domain.SecurityPolicy,
	result *domain.MCPPolicyEvaluationResult,
) {
	var rules domain.MCPAllowlistRules
	rulesJSON, _ := json.Marshal(policy.Rules)
	if err := json.Unmarshal(rulesJSON, &rules); err != nil {
		return
	}

	// Check if MCP server matches allowlist
	allowed := false

	// Check domain allowlist
	if len(rules.AllowedDomains) > 0 {
		serverDomain := extractDomain(mcpServer.URL)
		for _, pattern := range rules.AllowedDomains {
			if matchDomainPattern(serverDomain, pattern) {
				allowed = true
				result.MatchedPatterns = append(result.MatchedPatterns, "domain:"+pattern)
				break
			}
		}
	}

	// Check name allowlist
	if !allowed && len(rules.AllowedNames) > 0 {
		for _, name := range rules.AllowedNames {
			if strings.EqualFold(mcpServer.Name, name) {
				allowed = true
				result.MatchedPatterns = append(result.MatchedPatterns, "name:"+name)
				break
			}
		}
	}

	// Check additional requirements even if allowed
	if allowed {
		// Check verification requirement
		if rules.RequireVerified && mcpServer.Status != "verified" {
			result.Triggered = true
			result.ViolatedRules = append(result.ViolatedRules, "MCP server not verified")
			result.Reason = "MCP server must be verified"
			result.Recommendations = append(result.Recommendations, "Verify the MCP server through attestations")
			return
		}

		// Check minimum trust score
		if rules.MinTrustScore > 0 && mcpServer.TrustScore < rules.MinTrustScore {
			result.Triggered = true
			result.ViolatedRules = append(result.ViolatedRules, "Trust score below minimum")
			// Both values are on the canonical [0,1] scale, so both are scaled
			// for display. The threshold used to be printed unscaled, which
			// rendered a 0.7 floor as "below minimum 0.7%".
			result.Reason = fmt.Sprintf("Trust score %.1f%% is below minimum %.1f%%",
				mcpServer.TrustScore*100, rules.MinTrustScore*100)
			result.Recommendations = append(result.Recommendations, "Improve MCP server trust score through attestations")
			return
		}

		// Check minimum confidence score
		if rules.MinConfidenceScore > 0 && mcpServer.ConfidenceScore < rules.MinConfidenceScore {
			result.Triggered = true
			result.ViolatedRules = append(result.ViolatedRules, "Confidence score below minimum")
			result.Reason = fmt.Sprintf("Confidence score %.1f%% is below minimum %.1f%%",
				mcpServer.ConfidenceScore, rules.MinConfidenceScore)
			result.Recommendations = append(result.Recommendations, "Obtain more agent attestations")
			return
		}

		// Check minimum attestations
		if rules.MinAttestations > 0 && mcpServer.AttestationCount < rules.MinAttestations {
			result.Triggered = true
			result.ViolatedRules = append(result.ViolatedRules, "Insufficient attestations")
			result.Reason = fmt.Sprintf("Attestation count %d is below minimum %d",
				mcpServer.AttestationCount, rules.MinAttestations)
			result.Recommendations = append(result.Recommendations, "Obtain more agent attestations")
			return
		}
	} else if len(rules.AllowedDomains) > 0 || len(rules.AllowedNames) > 0 {
		// MCP server is not in allowlist
		result.Triggered = true
		result.ViolatedRules = append(result.ViolatedRules, "Not in allowlist")
		result.Reason = "MCP server is not in the organization's allowlist"
		result.Recommendations = append(result.Recommendations, "Add this MCP server to the allowlist or use an approved MCP server")
	}
}

// evaluateBlocklist checks if MCP server is in the blocklist
func (e *MCPPolicyEvaluator) evaluateBlocklist(
	mcpServer *domain.MCPServer,
	policy *domain.SecurityPolicy,
	result *domain.MCPPolicyEvaluationResult,
) {
	var rules domain.MCPBlocklistRules
	rulesJSON, _ := json.Marshal(policy.Rules)
	if err := json.Unmarshal(rulesJSON, &rules); err != nil {
		return
	}

	// Check domain blocklist
	if len(rules.BlockedDomains) > 0 {
		serverDomain := extractDomain(mcpServer.URL)
		for _, pattern := range rules.BlockedDomains {
			if matchDomainPattern(serverDomain, pattern) {
				result.Triggered = true
				result.ViolatedRules = append(result.ViolatedRules, "Domain is blocked")
				result.MatchedPatterns = append(result.MatchedPatterns, "blocked_domain:"+pattern)
				result.Reason = fmt.Sprintf("MCP server domain matches blocked pattern: %s", pattern)
				result.Recommendations = append(result.Recommendations, "Use an MCP server from an approved domain")
				return
			}
		}
	}

	// Check name blocklist
	if len(rules.BlockedNames) > 0 {
		for _, name := range rules.BlockedNames {
			if strings.EqualFold(mcpServer.Name, name) {
				result.Triggered = true
				result.ViolatedRules = append(result.ViolatedRules, "Name is blocked")
				result.MatchedPatterns = append(result.MatchedPatterns, "blocked_name:"+name)
				result.Reason = fmt.Sprintf("MCP server name '%s' is blocked", name)
				result.Recommendations = append(result.Recommendations, "Use a different MCP server")
				return
			}
		}
	}

	// Check URL blocklist
	if len(rules.BlockedURLs) > 0 {
		for _, blockedURL := range rules.BlockedURLs {
			if strings.EqualFold(mcpServer.URL, blockedURL) {
				result.Triggered = true
				result.ViolatedRules = append(result.ViolatedRules, "URL is blocked")
				result.MatchedPatterns = append(result.MatchedPatterns, "blocked_url:"+blockedURL)
				result.Reason = "MCP server URL is explicitly blocked"
				result.Recommendations = append(result.Recommendations, "Use a different MCP server")
				return
			}
		}
	}

	// Check capability blocklist
	if len(rules.BlockedCapabilities) > 0 && len(mcpServer.Capabilities) > 0 {
		for _, blockedCap := range rules.BlockedCapabilities {
			for _, serverCap := range mcpServer.Capabilities {
				if strings.EqualFold(serverCap, blockedCap) {
					result.Triggered = true
					result.ViolatedRules = append(result.ViolatedRules, "Blocked capability detected")
					result.MatchedPatterns = append(result.MatchedPatterns, "blocked_capability:"+blockedCap)
					result.Reason = fmt.Sprintf("MCP server has blocked capability: %s", blockedCap)
					result.Recommendations = append(result.Recommendations, "Remove the blocked capability or use a different MCP server")
					return
				}
			}
		}
	}
}

// evaluateCapabilities checks MCP server capabilities against policy
func (e *MCPPolicyEvaluator) evaluateCapabilities(
	mcpServer *domain.MCPServer,
	policy *domain.SecurityPolicy,
	result *domain.MCPPolicyEvaluationResult,
) {
	var rules domain.MCPCapabilitiesRules
	rulesJSON, _ := json.Marshal(policy.Rules)
	if err := json.Unmarshal(rulesJSON, &rules); err != nil {
		return
	}

	serverCaps := mcpServer.Capabilities

	// Check required capabilities
	if len(rules.RequiredCapabilities) > 0 {
		if rules.RequireAllCapabilities {
			// AND logic - all required capabilities must be present
			for _, required := range rules.RequiredCapabilities {
				found := false
				for _, serverCap := range serverCaps {
					if strings.EqualFold(serverCap, required) {
						found = true
						break
					}
				}
				if !found {
					result.Triggered = true
					result.ViolatedRules = append(result.ViolatedRules, "Missing required capability: "+required)
					result.Reason = fmt.Sprintf("MCP server missing required capability: %s", required)
					result.Recommendations = append(result.Recommendations, "Add the missing capability to the MCP server")
				}
			}
		} else {
			// OR logic - at least one required capability must be present
			found := false
			for _, required := range rules.RequiredCapabilities {
				for _, serverCap := range serverCaps {
					if strings.EqualFold(serverCap, required) {
						found = true
						break
					}
				}
				if found {
					break
				}
			}
			if !found {
				result.Triggered = true
				result.ViolatedRules = append(result.ViolatedRules, "No required capabilities present")
				result.Reason = "MCP server has none of the required capabilities"
				result.Recommendations = append(result.Recommendations, "Add at least one required capability")
			}
		}
	}

	// Check forbidden capabilities
	if len(rules.ForbiddenCapabilities) > 0 {
		for _, forbidden := range rules.ForbiddenCapabilities {
			for _, serverCap := range serverCaps {
				if strings.EqualFold(serverCap, forbidden) {
					result.Triggered = true
					result.ViolatedRules = append(result.ViolatedRules, "Forbidden capability present: "+forbidden)
					result.Reason = fmt.Sprintf("MCP server has forbidden capability: %s", forbidden)
					result.Recommendations = append(result.Recommendations, "Remove the forbidden capability")
				}
			}
		}
	}

	// Check max capability count
	if rules.MaxCapabilityCount > 0 && len(serverCaps) > rules.MaxCapabilityCount {
		result.Triggered = true
		result.ViolatedRules = append(result.ViolatedRules, "Too many capabilities")
		result.Reason = fmt.Sprintf("MCP server has %d capabilities, exceeding limit of %d",
			len(serverCaps), rules.MaxCapabilityCount)
		result.Recommendations = append(result.Recommendations, "Reduce the number of capabilities")
	}
}

// evaluateUnverified checks if unverified MCP server handling rules apply
func (e *MCPPolicyEvaluator) evaluateUnverified(
	mcpServer *domain.MCPServer,
	policy *domain.SecurityPolicy,
	result *domain.MCPPolicyEvaluationResult,
) {
	var rules domain.MCPUnverifiedRules
	rulesJSON, _ := json.Marshal(policy.Rules)
	if err := json.Unmarshal(rulesJSON, &rules); err != nil {
		return
	}

	// Only evaluate if server is not verified
	if mcpServer.Status == "verified" {
		return
	}

	// Check grace period
	// If within grace period, don't trigger
	// (grace period logic would check mcpServer.CreatedAt vs current time)

	result.Triggered = true
	result.ViolatedRules = append(result.ViolatedRules, "MCP server not verified")
	result.Reason = "MCP server has not been verified"

	// Set recommendations based on rules
	if rules.RequireManualApproval {
		result.Recommendations = append(result.Recommendations, "Request admin approval for this MCP server")
	} else {
		result.Recommendations = append(result.Recommendations, "Verify the MCP server through agent attestations")
	}
}

// Helper functions

func isMCPPolicy(policyType domain.PolicyType) bool {
	return policyType == domain.PolicyTypeMCPAllowlist ||
		policyType == domain.PolicyTypeMCPBlocklist ||
		policyType == domain.PolicyTypeMCPCapabilities ||
		policyType == domain.PolicyTypeMCPUnverified
}

func extractDomain(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	return parsed.Hostname()
}

func matchDomainPattern(domain, pattern string) bool {
	// Handle wildcard patterns like "*.company.com"
	if strings.HasPrefix(pattern, "*.") {
		suffix := pattern[1:] // ".company.com"
		return strings.HasSuffix(domain, suffix) || domain == pattern[2:] // exact match without wildcard
	}
	return strings.EqualFold(domain, pattern)
}
