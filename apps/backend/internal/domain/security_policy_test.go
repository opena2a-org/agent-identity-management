package domain

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPolicyType_Constants(t *testing.T) {
	tests := []struct {
		constant PolicyType
		expected string
	}{
		{PolicyTypeCapabilityViolation, "capability_violation"},
		{PolicyTypeTrustScoreLow, "trust_score_low"},
		{PolicyTypeUnusualActivity, "unusual_activity"},
		{PolicyTypeUnauthorizedAccess, "unauthorized_access"},
		{PolicyTypeDataExfiltration, "data_exfiltration"},
		{PolicyTypeConfigDrift, "config_drift"},
		{PolicyTypeMCPAllowlist, "mcp_allowlist"},
		{PolicyTypeMCPBlocklist, "mcp_blocklist"},
		{PolicyTypeMCPCapabilities, "mcp_capabilities"},
		{PolicyTypeMCPUnverified, "mcp_unverified"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, PolicyType(tt.expected), tt.constant)
		})
	}
}

func TestEnforcementAction_Constants(t *testing.T) {
	tests := []struct {
		constant EnforcementAction
		expected string
	}{
		{EnforcementAlertOnly, "alert_only"},
		{EnforcementBlockAndAlert, "block_and_alert"},
		{EnforcementAllow, "allow"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, EnforcementAction(tt.expected), tt.constant)
		})
	}
}

func TestSecurityPolicy_Structure(t *testing.T) {
	now := time.Now()

	policy := SecurityPolicy{
		ID:                uuid.New(),
		OrganizationID:    uuid.New(),
		Name:              "Block Low Trust Agents",
		Description:       "Block agents with trust score below 50%",
		PolicyType:        PolicyTypeTrustScoreLow,
		EnforcementAction: EnforcementBlockAndAlert,
		SeverityThreshold: AlertSeverityHigh,
		Rules: map[string]interface{}{
			"min_trust_score": 0.5,
			"grace_period":    "24h",
		},
		AppliesTo: "all",
		IsEnabled: true,
		Priority:  10,
		CreatedAt: now,
		UpdatedAt: now,
		CreatedBy: uuid.New(),
	}

	assert.NotEqual(t, uuid.Nil, policy.ID)
	assert.Equal(t, "Block Low Trust Agents", policy.Name)
	assert.Equal(t, PolicyTypeTrustScoreLow, policy.PolicyType)
	assert.Equal(t, EnforcementBlockAndAlert, policy.EnforcementAction)
	assert.Equal(t, AlertSeverityHigh, policy.SeverityThreshold)
	assert.True(t, policy.IsEnabled)
	assert.Equal(t, 10, policy.Priority)
}

func TestSecurityPolicy_MCPAllowlist(t *testing.T) {
	policy := SecurityPolicy{
		ID:                uuid.New(),
		OrganizationID:    uuid.New(),
		Name:              "MCP Server Allowlist",
		Description:       "Only allow approved MCP servers",
		PolicyType:        PolicyTypeMCPAllowlist,
		EnforcementAction: EnforcementBlockAndAlert,
		SeverityThreshold: AlertSeverityCritical,
		Rules: map[string]interface{}{
			"allowedDomains":     []string{"*.company.com", "github.com"},
			"allowedNames":       []string{"internal-mcp", "github-mcp"},
			"requireVerified":    true,
			"minTrustScore":      0.7,
			"minConfidenceScore": 0.5,
		},
		AppliesTo: "all",
		IsEnabled: true,
		Priority:  1,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		CreatedBy: uuid.New(),
	}

	assert.Equal(t, PolicyTypeMCPAllowlist, policy.PolicyType)
	assert.Contains(t, policy.Rules, "allowedDomains")
	assert.Contains(t, policy.Rules, "requireVerified")
}

func TestSecurityPolicy_MCPBlocklist(t *testing.T) {
	policy := SecurityPolicy{
		ID:                uuid.New(),
		OrganizationID:    uuid.New(),
		Name:              "Block Malicious MCP Servers",
		Description:       "Block known malicious MCP servers",
		PolicyType:        PolicyTypeMCPBlocklist,
		EnforcementAction: EnforcementBlockAndAlert,
		SeverityThreshold: AlertSeverityCritical,
		Rules: map[string]interface{}{
			"blockedDomains": []string{"malicious.com", "*.sketchy.net"},
			"blockedUrls":    []string{"http://insecure-mcp.com/api"},
		},
		AppliesTo: "all",
		IsEnabled: true,
		Priority:  1,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		CreatedBy: uuid.New(),
	}

	assert.Equal(t, PolicyTypeMCPBlocklist, policy.PolicyType)
	assert.Contains(t, policy.Rules, "blockedDomains")
}

func TestSecurityPolicy_Disabled(t *testing.T) {
	policy := SecurityPolicy{
		ID:                uuid.New(),
		OrganizationID:    uuid.New(),
		Name:              "Disabled Policy",
		PolicyType:        PolicyTypeUnusualActivity,
		EnforcementAction: EnforcementAlertOnly,
		IsEnabled:         false,
		Priority:          100,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
		CreatedBy:         uuid.New(),
	}

	assert.False(t, policy.IsEnabled)
}

func TestPolicyEvaluationResult_Structure(t *testing.T) {
	result := PolicyEvaluationResult{
		PolicyID:          uuid.New(),
		PolicyName:        "Test Policy",
		Triggered:         true,
		EnforcementAction: EnforcementBlockAndAlert,
		Reason:            "Trust score below threshold",
		ShouldBlock:       true,
		ShouldAlert:       true,
	}

	assert.NotEqual(t, uuid.Nil, result.PolicyID)
	assert.True(t, result.Triggered)
	assert.True(t, result.ShouldBlock)
	assert.True(t, result.ShouldAlert)
}

func TestPolicyEvaluationResult_NotTriggered(t *testing.T) {
	result := PolicyEvaluationResult{
		PolicyID:          uuid.New(),
		PolicyName:        "Test Policy",
		Triggered:         false,
		EnforcementAction: EnforcementBlockAndAlert,
		Reason:            "",
		ShouldBlock:       false,
		ShouldAlert:       false,
	}

	assert.False(t, result.Triggered)
	assert.False(t, result.ShouldBlock)
	assert.False(t, result.ShouldAlert)
	assert.Empty(t, result.Reason)
}

func TestPolicyEvaluationResult_AlertOnly(t *testing.T) {
	result := PolicyEvaluationResult{
		PolicyID:          uuid.New(),
		PolicyName:        "Monitor Policy",
		Triggered:         true,
		EnforcementAction: EnforcementAlertOnly,
		Reason:            "Unusual activity detected",
		ShouldBlock:       false,
		ShouldAlert:       true,
	}

	assert.True(t, result.Triggered)
	assert.False(t, result.ShouldBlock)
	assert.True(t, result.ShouldAlert)
}

func TestMCPAllowlistRules_Structure(t *testing.T) {
	rules := MCPAllowlistRules{
		AllowedDomains:      []string{"*.company.com", "github.com", "*.anthropic.com"},
		AllowedNames:        []string{"internal-mcp", "github-mcp", "slack-mcp"},
		AllowedCapabilities: []string{"tools", "resources", "prompts"},
		RequireVerified:     true,
		MinTrustScore:       0.7,
		MinConfidenceScore:  0.5,
		MinAttestations:     2,
	}

	assert.Len(t, rules.AllowedDomains, 3)
	assert.Len(t, rules.AllowedNames, 3)
	assert.Len(t, rules.AllowedCapabilities, 3)
	assert.True(t, rules.RequireVerified)
	assert.Equal(t, 0.7, rules.MinTrustScore)
	assert.Equal(t, 0.5, rules.MinConfidenceScore)
	assert.Equal(t, 2, rules.MinAttestations)
}

func TestMCPBlocklistRules_Structure(t *testing.T) {
	rules := MCPBlocklistRules{
		BlockedDomains:      []string{"malicious.com", "*.sketchy-domain.net"},
		BlockedNames:        []string{"compromised-mcp", "deprecated-server"},
		BlockedURLs:         []string{"https://malicious.com/mcp", "http://insecure-server.com/api"},
		BlockedCapabilities: []string{"dangerous-tool", "unrestricted-access"},
	}

	assert.Len(t, rules.BlockedDomains, 2)
	assert.Len(t, rules.BlockedNames, 2)
	assert.Len(t, rules.BlockedURLs, 2)
	assert.Len(t, rules.BlockedCapabilities, 2)
}

func TestMCPCapabilitiesRules_Structure(t *testing.T) {
	rules := MCPCapabilitiesRules{
		RequiredCapabilities:   []string{"tools", "resources"},
		ForbiddenCapabilities:  []string{"admin", "system-access", "file-write"},
		RequireAllCapabilities: true,
		MaxCapabilityCount:     10,
	}

	assert.Len(t, rules.RequiredCapabilities, 2)
	assert.Len(t, rules.ForbiddenCapabilities, 3)
	assert.True(t, rules.RequireAllCapabilities)
	assert.Equal(t, 10, rules.MaxCapabilityCount)
}

func TestMCPCapabilitiesRules_OrLogic(t *testing.T) {
	rules := MCPCapabilitiesRules{
		RequiredCapabilities:   []string{"tools", "resources"},
		RequireAllCapabilities: false, // OR logic
		MaxCapabilityCount:     0,     // No limit
	}

	assert.False(t, rules.RequireAllCapabilities)
	assert.Equal(t, 0, rules.MaxCapabilityCount)
}

func TestMCPUnverifiedRules_Structure(t *testing.T) {
	rules := MCPUnverifiedRules{
		Action:                  "quarantine",
		GracePeriodDays:         7,
		RequireManualApproval:   true,
		AutoQuarantineAfterDays: 30,
		AlertOnFirstUse:         true,
		NotifyOwners:            true,
	}

	assert.Equal(t, "quarantine", rules.Action)
	assert.Equal(t, 7, rules.GracePeriodDays)
	assert.True(t, rules.RequireManualApproval)
	assert.Equal(t, 30, rules.AutoQuarantineAfterDays)
	assert.True(t, rules.AlertOnFirstUse)
	assert.True(t, rules.NotifyOwners)
}

func TestMCPUnverifiedRules_BlockAction(t *testing.T) {
	rules := MCPUnverifiedRules{
		Action:                "block_and_alert",
		GracePeriodDays:       0,     // No grace period
		RequireManualApproval: false,
	}

	assert.Equal(t, "block_and_alert", rules.Action)
	assert.Equal(t, 0, rules.GracePeriodDays)
}

func TestMCPPolicyEvaluationResult_Structure(t *testing.T) {
	result := MCPPolicyEvaluationResult{
		PolicyEvaluationResult: PolicyEvaluationResult{
			PolicyID:          uuid.New(),
			PolicyName:        "MCP Blocklist",
			Triggered:         true,
			EnforcementAction: EnforcementBlockAndAlert,
			Reason:            "MCP server matches blocklist pattern",
			ShouldBlock:       true,
			ShouldAlert:       true,
		},
		MCPServerID:     uuid.New(),
		MCPServerName:   "suspicious-mcp",
		MCPServerURL:    "https://suspicious.com/mcp",
		ViolatedRules:   []string{"blockedDomains: *.suspicious.com"},
		MatchedPatterns: []string{"*.suspicious.com"},
		Recommendations: []string{
			"Remove this MCP server from your organization",
			"Review all connections to this server",
		},
	}

	assert.NotEqual(t, uuid.Nil, result.MCPServerID)
	assert.Equal(t, "suspicious-mcp", result.MCPServerName)
	assert.True(t, result.Triggered)
	assert.True(t, result.ShouldBlock)
	assert.Len(t, result.ViolatedRules, 1)
	assert.Len(t, result.MatchedPatterns, 1)
	assert.Len(t, result.Recommendations, 2)
}

func TestSecurityPolicy_JSONSerialization(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)

	policy := SecurityPolicy{
		ID:                uuid.New(),
		OrganizationID:    uuid.New(),
		Name:              "Test Policy",
		Description:       "Test description",
		PolicyType:        PolicyTypeTrustScoreLow,
		EnforcementAction: EnforcementBlockAndAlert,
		SeverityThreshold: AlertSeverityHigh,
		Rules:             map[string]interface{}{"threshold": 0.5},
		AppliesTo:         "all",
		IsEnabled:         true,
		Priority:          5,
		CreatedAt:         now,
		UpdatedAt:         now,
		CreatedBy:         uuid.New(),
	}

	// Serialize to JSON
	data, err := json.Marshal(policy)
	require.NoError(t, err)

	// Deserialize back
	var restored SecurityPolicy
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)

	// Verify key fields
	assert.Equal(t, policy.ID, restored.ID)
	assert.Equal(t, policy.Name, restored.Name)
	assert.Equal(t, policy.PolicyType, restored.PolicyType)
	assert.Equal(t, policy.EnforcementAction, restored.EnforcementAction)
	assert.Equal(t, policy.IsEnabled, restored.IsEnabled)
}

func TestMCPAllowlistRules_JSONSerialization(t *testing.T) {
	rules := MCPAllowlistRules{
		AllowedDomains:  []string{"*.company.com"},
		AllowedNames:    []string{"internal-mcp"},
		RequireVerified: true,
		MinTrustScore:   0.7,
	}

	// Serialize to JSON
	data, err := json.Marshal(rules)
	require.NoError(t, err)

	// Deserialize back
	var restored MCPAllowlistRules
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)

	// Verify fields
	assert.Equal(t, rules.AllowedDomains, restored.AllowedDomains)
	assert.Equal(t, rules.RequireVerified, restored.RequireVerified)
	assert.Equal(t, rules.MinTrustScore, restored.MinTrustScore)
}

func TestSecurityPolicy_AllPolicyTypes(t *testing.T) {
	types := []PolicyType{
		PolicyTypeCapabilityViolation,
		PolicyTypeTrustScoreLow,
		PolicyTypeUnusualActivity,
		PolicyTypeUnauthorizedAccess,
		PolicyTypeDataExfiltration,
		PolicyTypeConfigDrift,
		PolicyTypeMCPAllowlist,
		PolicyTypeMCPBlocklist,
		PolicyTypeMCPCapabilities,
		PolicyTypeMCPUnverified,
	}

	for _, pType := range types {
		t.Run(string(pType), func(t *testing.T) {
			policy := SecurityPolicy{
				ID:                uuid.New(),
				OrganizationID:    uuid.New(),
				Name:              "Test " + string(pType),
				PolicyType:        pType,
				EnforcementAction: EnforcementAlertOnly,
				IsEnabled:         true,
				CreatedBy:         uuid.New(),
				CreatedAt:         time.Now(),
				UpdatedAt:         time.Now(),
			}

			assert.Equal(t, pType, policy.PolicyType)
			assert.NotEmpty(t, string(pType))
		})
	}
}

func TestSecurityPolicy_AllEnforcementActions(t *testing.T) {
	actions := []EnforcementAction{
		EnforcementAlertOnly,
		EnforcementBlockAndAlert,
		EnforcementAllow,
	}

	for _, action := range actions {
		t.Run(string(action), func(t *testing.T) {
			policy := SecurityPolicy{
				ID:                uuid.New(),
				OrganizationID:    uuid.New(),
				Name:              "Test",
				PolicyType:        PolicyTypeTrustScoreLow,
				EnforcementAction: action,
				IsEnabled:         true,
				CreatedBy:         uuid.New(),
				CreatedAt:         time.Now(),
				UpdatedAt:         time.Now(),
			}

			assert.Equal(t, action, policy.EnforcementAction)
			assert.NotEmpty(t, string(action))
		})
	}
}
