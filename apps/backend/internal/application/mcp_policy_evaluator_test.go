package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsMCPPolicy(t *testing.T) {
	tests := []struct {
		policyType domain.PolicyType
		expected   bool
	}{
		{domain.PolicyTypeMCPAllowlist, true},
		{domain.PolicyTypeMCPBlocklist, true},
		{domain.PolicyTypeMCPCapabilities, true},
		{domain.PolicyTypeMCPUnverified, true},
		{domain.PolicyTypeCapabilityViolation, false},
		{domain.PolicyTypeTrustScoreLow, false},
		{domain.PolicyTypeUnusualActivity, false},
		{domain.PolicyTypeDataExfiltration, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.policyType), func(t *testing.T) {
			result := isMCPPolicy(tt.policyType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractDomain(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "simple https URL",
			url:      "https://example.com",
			expected: "example.com",
		},
		{
			name:     "https URL with path",
			url:      "https://api.example.com/v1/endpoint",
			expected: "api.example.com",
		},
		{
			name:     "http URL with port",
			url:      "http://localhost:8080/api",
			expected: "localhost",
		},
		{
			name:     "https URL with subdomain",
			url:      "https://mcp.services.company.com",
			expected: "mcp.services.company.com",
		},
		{
			name:     "URL with query params",
			url:      "https://api.example.com?key=value",
			expected: "api.example.com",
		},
		{
			name:     "IP address URL",
			url:      "http://192.168.1.100:3000",
			expected: "192.168.1.100",
		},
		{
			name:     "invalid URL returns as-is",
			url:      "not-a-valid-url",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractDomain(tt.url)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMatchDomainPattern(t *testing.T) {
	tests := []struct {
		name     string
		domain   string
		pattern  string
		expected bool
	}{
		// Exact match tests
		{
			name:     "exact match",
			domain:   "example.com",
			pattern:  "example.com",
			expected: true,
		},
		{
			name:     "exact match case insensitive",
			domain:   "Example.COM",
			pattern:  "example.com",
			expected: true,
		},
		{
			name:     "no match different domain",
			domain:   "other.com",
			pattern:  "example.com",
			expected: false,
		},
		// Wildcard tests
		{
			name:     "wildcard matches subdomain",
			domain:   "api.example.com",
			pattern:  "*.example.com",
			expected: true,
		},
		{
			name:     "wildcard matches nested subdomain",
			domain:   "mcp.api.example.com",
			pattern:  "*.example.com",
			expected: true,
		},
		{
			name:     "wildcard matches exact base domain",
			domain:   "example.com",
			pattern:  "*.example.com",
			expected: true,
		},
		{
			name:     "wildcard no match different domain",
			domain:   "example.org",
			pattern:  "*.example.com",
			expected: false,
		},
		{
			name:     "wildcard no match partial",
			domain:   "notexample.com",
			pattern:  "*.example.com",
			expected: false,
		},
		// Bare wildcard. Migration 052 seeds allowedDomains ["*"] meaning "any domain"; before the
		// bare "*" case existed it fell through to the equality test and matched no host at all.
		{
			name:     "bare wildcard matches a host",
			domain:   "api.example.com",
			pattern:  "*",
			expected: true,
		},
		{
			name:     "bare wildcard matches a single-label host",
			domain:   "localhost",
			pattern:  "*",
			expected: true,
		},
		{
			name:     "bare wildcard matches an IP address",
			domain:   "192.168.1.100",
			pattern:  "*",
			expected: true,
		},
		{
			// extractDomain returns "" for a URL with no host. "Every server" includes it, so a
			// "*" blocklist cannot be stepped around by registering a host-less URL.
			name:     "bare wildcard matches a server with no parseable host",
			domain:   "",
			pattern:  "*",
			expected: true,
		},
		{
			name:     "a literal asterisk host is still only matched by a pattern",
			domain:   "*",
			pattern:  "example.com",
			expected: false,
		},
		// Host names compare without regard to case, and a trailing dot names the same host.
		{
			name:     "wildcard matches a host written in upper case",
			domain:   "x.EVIL.com",
			pattern:  "*.evil.com",
			expected: true,
		},
		{
			name:     "wildcard written in upper case matches a lower-case host",
			domain:   "evil.com",
			pattern:  "*.EVIL.com",
			expected: true,
		},
		{
			name:     "wildcard matches a host with a trailing dot",
			domain:   "x.evil.com.",
			pattern:  "*.evil.com",
			expected: true,
		},
		{
			name:     "exact pattern matches a host with a trailing dot",
			domain:   "evil.com.",
			pattern:  "evil.com",
			expected: true,
		},
		{
			name:     "exact pattern with a trailing dot matches the host without one",
			domain:   "evil.com",
			pattern:  "evil.com.",
			expected: true,
		},
		{
			name:     "a trailing dot does not make a lookalike match",
			domain:   "notevil.com.",
			pattern:  "*.evil.com",
			expected: false,
		},
		{
			name:     "a wildcard with no domain after it matches nothing",
			domain:   "evil.com",
			pattern:  "*.",
			expected: false,
		},
		// Edge cases
		{
			name:     "empty domain",
			domain:   "",
			pattern:  "example.com",
			expected: false,
		},
		{
			name:     "empty pattern",
			domain:   "example.com",
			pattern:  "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchDomainPattern(tt.domain, tt.pattern)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ========================================
// Constructor Tests
// ========================================

func TestNewMCPPolicyEvaluator(t *testing.T) {
	evaluator := NewMCPPolicyEvaluator(nil, nil)

	assert.NotNil(t, evaluator)
	assert.Nil(t, evaluator.policyRepo)
	assert.Nil(t, evaluator.mcpRepo)
}

// TestMCPPolicy_MinTrustScoreGateRejectsLowScoringServer is the security
// red-proof for the trust-score fabrication.
//
// `evaluateAllowlist` compares `mcpServer.TrustScore` against
// `rules.MinTrustScore`, and `MinTrustScore` is on the canonical [0,1] scale.
// While `mcp_service.go` stamped SDK-registered and verified servers with the
// literal 75.0, that comparison could never fail: 75.0 sits above every
// representable threshold, so an SDK-registered MCP server passed any
// organization's trust floor unconditionally, and only a genuinely-measured
// server could ever be rejected.
//
// With scores on one scale, the gate discriminates again.
func TestMCPPolicy_MinTrustScoreGateRejectsLowScoringServer(t *testing.T) {
	evaluator := &MCPPolicyEvaluator{}

	policy := &domain.SecurityPolicy{
		ID:         uuid.New(),
		PolicyType: domain.PolicyTypeMCPAllowlist,
		Rules: map[string]interface{}{
			"allowedDomains": []string{"mcp.example.com"},
			"minTrustScore":  0.7,
		},
	}

	tests := []struct {
		name            string
		trustScore      float64
		expectTriggered bool
		why             string
	}{
		{
			name:            "score below the floor is rejected",
			trustScore:      0.42,
			expectTriggered: true,
			why:             "0.42 is below the 0.7 floor",
		},
		{
			name:            "score above the floor is allowed",
			trustScore:      0.85,
			expectTriggered: false,
			why:             "0.85 clears the 0.7 floor",
		},
		{
			name:            "score exactly at the floor is allowed",
			trustScore:      0.7,
			expectTriggered: false,
			why:             "the comparison is strictly less-than",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := &domain.MCPServer{
				ID:         uuid.New(),
				Name:       "example-mcp",
				URL:        "https://mcp.example.com/sse",
				Status:     domain.MCPServerStatusVerified,
				IsVerified: true,
				TrustScore: tt.trustScore,
			}

			result := &domain.MCPPolicyEvaluationResult{}
			evaluator.evaluateAllowlist(server, policy, result)

			assert.Equal(t, tt.expectTriggered, result.Triggered, tt.why)
			if tt.expectTriggered {
				assert.Contains(t, result.ViolatedRules, "Trust score below minimum")
				// The threshold is rendered as a percentage alongside the
				// score. It used to be printed unscaled, turning a 0.7 floor
				// into "below minimum 0.7%".
				assert.Contains(t, result.Reason, "below minimum 70.0%")
			}
		})
	}
}

// TestMCPPolicy_FabricatedScoreWouldDefeatTheGate pins the failure directly:
// the exact literal the service used to write passes a floor that no
// calculated score could clear, because the calculator's maximum output is
// 1.0. If this ever passes with `triggered == false` for a value above 1.0,
// an out-of-scale writer has reappeared.
func TestMCPPolicy_FabricatedScoreWouldDefeatTheGate(t *testing.T) {
	evaluator := &MCPPolicyEvaluator{}

	policy := &domain.SecurityPolicy{
		ID:         uuid.New(),
		PolicyType: domain.PolicyTypeMCPAllowlist,
		Rules: map[string]interface{}{
			"allowedDomains": []string{"mcp.example.com"},
			"minTrustScore":  1.0, // the strictest representable floor
		},
	}

	server := &domain.MCPServer{
		ID:         uuid.New(),
		Name:       "example-mcp",
		URL:        "https://mcp.example.com/sse",
		Status:     domain.MCPServerStatusVerified,
		IsVerified: true,
		TrustScore: 75.0, // the removed literal
	}

	result := &domain.MCPPolicyEvaluationResult{}
	evaluator.evaluateAllowlist(server, policy, result)

	assert.False(t, result.Triggered,
		"documents the defect: 75.0 clears even a 1.0 floor. Migration 104's "+
			"CHECK constraint is what makes this value unstorable; this test "+
			"records why the constraint is load-bearing rather than cosmetic.")
}

// TestMCPPolicy_AllowlistEnforcementMatrix covers the remaining verdict
// surface of `evaluateAllowlist`. Each case violates exactly one field and
// asserts the verdict flips to rejected; the baseline case violates none and
// asserts it does not.
//
// Before this, the only tests in this file exercised the domain-matching
// helpers — `evaluateAllowlist` itself, which decides whether an MCP server
// is allowed, had no test at all. Enforcement that no test can distinguish
// from its absence is unproven.
func TestMCPPolicy_AllowlistEnforcementMatrix(t *testing.T) {
	evaluator := &MCPPolicyEvaluator{}

	// A server that satisfies every requirement below.
	compliant := func() *domain.MCPServer {
		return &domain.MCPServer{
			ID:               uuid.New(),
			Name:             "example-mcp",
			URL:              "https://mcp.example.com/sse",
			Status:           domain.MCPServerStatusVerified,
			IsVerified:       true,
			TrustScore:       0.85,
			ConfidenceScore:  90.0,
			AttestationCount: 5,
		}
	}

	rules := map[string]interface{}{
		"allowedDomains":     []string{"mcp.example.com"},
		"requireVerified":    true,
		"minTrustScore":      0.7,
		"minConfidenceScore": 80.0,
		"minAttestations":    3,
	}

	policy := &domain.SecurityPolicy{
		ID:         uuid.New(),
		PolicyType: domain.PolicyTypeMCPAllowlist,
		Rules:      rules,
	}

	tests := []struct {
		name           string
		violate        func(*domain.MCPServer)
		expectViolated string // "" means the request must be allowed
	}{
		{
			name:    "baseline: nothing violated",
			violate: func(*domain.MCPServer) {},
		},
		{
			name:           "domain not in allowlist",
			violate:        func(s *domain.MCPServer) { s.URL = "https://evil.example.net/sse" },
			expectViolated: "Not in allowlist",
		},
		{
			name:           "unverified while requireVerified is set",
			violate:        func(s *domain.MCPServer) { s.Status = domain.MCPServerStatusPending },
			expectViolated: "MCP server not verified",
		},
		{
			name:           "trust score below the floor",
			violate:        func(s *domain.MCPServer) { s.TrustScore = 0.2 },
			expectViolated: "Trust score below minimum",
		},
		{
			name:           "confidence score below the floor",
			violate:        func(s *domain.MCPServer) { s.ConfidenceScore = 10.0 },
			expectViolated: "Confidence score below minimum",
		},
		{
			name:           "too few attestations",
			violate:        func(s *domain.MCPServer) { s.AttestationCount = 1 },
			expectViolated: "Insufficient attestations",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := compliant()
			tt.violate(server)

			result := &domain.MCPPolicyEvaluationResult{}
			evaluator.evaluateAllowlist(server, policy, result)

			if tt.expectViolated == "" {
				assert.False(t, result.Triggered,
					"a fully compliant server must be allowed; if this fails the "+
						"other cases prove nothing, since they could be tripping "+
						"on the baseline rather than on the field they violate")
				return
			}

			assert.True(t, result.Triggered, "violating %q must trigger the policy", tt.name)
			assert.Contains(t, result.ViolatedRules, tt.expectViolated)
		})
	}
}

// TestMCPPolicy_AllowedCapabilitiesIsAnAllowlist covers MCPAllowlistRules.AllowedCapabilities,
// which was declared, editable in the admin form, seeded by migration 052 and read by nothing.
// For a server that the policy's allowedDomains admit, every capability it declares must appear
// on the list; a server that declares one outside it is rejected. The check does not run for a
// policy that sets neither allowedDomains nor allowedNames.
func TestMCPPolicy_AllowedCapabilitiesIsAnAllowlist(t *testing.T) {
	evaluator := &MCPPolicyEvaluator{}

	policy := &domain.SecurityPolicy{
		ID:         uuid.New(),
		PolicyType: domain.PolicyTypeMCPAllowlist,
		Rules: map[string]interface{}{
			"allowedDomains":      []string{"mcp.example.com"},
			"allowedCapabilities": []string{"tools", "resources", "prompts"},
		},
	}

	tests := []struct {
		name            string
		capabilities    []string
		expectTriggered bool
	}{
		{name: "every capability on the list is allowed", capabilities: []string{"tools", "prompts"}},
		{name: "matching is case-insensitive", capabilities: []string{"Tools", "RESOURCES"}},
		{name: "a server declaring no capabilities is allowed", capabilities: nil},
		{name: "one capability off the list is rejected", capabilities: []string{"tools", "sampling"}, expectTriggered: true},
		{name: "only capabilities off the list is rejected", capabilities: []string{"sampling"}, expectTriggered: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := &domain.MCPServer{
				ID:           uuid.New(),
				Name:         "example-mcp",
				URL:          "https://mcp.example.com/sse",
				Status:       domain.MCPServerStatusVerified,
				TrustScore:   0.9,
				Capabilities: tt.capabilities,
			}

			result := &domain.MCPPolicyEvaluationResult{}
			evaluator.evaluateAllowlist(server, policy, result)

			assert.Equal(t, tt.expectTriggered, result.Triggered)
			if tt.expectTriggered {
				assert.Contains(t, result.ViolatedRules, "Capability not in allowlist")
				assert.Contains(t, result.Reason, "sampling")
			}
		})
	}
}

// TestEvaluateMCPServer_RejectsServerViolatingEachRule drives the public entry point, through the
// policy repository, with policies configured the way migration 052 seeds them and the admin
// form saves them: a bare "*" domain pattern and a trust floor on the canonical [0,1] scale.
//
// Each case pairs a compliant server, which must pass, with the same server violating exactly one
// rule, which must be rejected for that rule. Against the evaluator before #355 every case fails:
// "*" matched no host, so the compliant server was rejected as "Not in allowlist" and no rule
// behind the domain match was ever reached; a "*" blocklist blocked nothing; and
// allowedCapabilities was never read.
func TestEvaluateMCPServer_RejectsServerViolatingEachRule(t *testing.T) {
	orgID := uuid.New()

	compliant := func() *domain.MCPServer {
		return &domain.MCPServer{
			ID:               uuid.New(),
			OrganizationID:   orgID,
			Name:             "example-mcp",
			URL:              "https://mcp.example.com/sse",
			Status:           domain.MCPServerStatusVerified,
			IsVerified:       true,
			TrustScore:       0.85,
			ConfidenceScore:  90.0,
			AttestationCount: 5,
			Capabilities:     []string{"tools", "resources"},
		}
	}

	tests := []struct {
		name       string
		policyType domain.PolicyType
		rules      map[string]interface{}
		// noCompliantServer is set when the rule rejects every server by design, so there is
		// no compliant control to assert against.
		noCompliantServer bool
		violate           func(*domain.MCPServer)
		expectViolated    string
	}{
		{
			name:           "unverified server under requireVerified",
			policyType:     domain.PolicyTypeMCPAllowlist,
			rules:          map[string]interface{}{"allowedDomains": []string{"*"}, "requireVerified": true},
			violate:        func(s *domain.MCPServer) { s.Status = domain.MCPServerStatusPending; s.IsVerified = false },
			expectViolated: "MCP server not verified",
		},
		{
			// The seeded "High-Risk MCP Server Block" floor of 30, rescaled to [0,1].
			name:           "trust score below minTrustScore",
			policyType:     domain.PolicyTypeMCPAllowlist,
			rules:          map[string]interface{}{"allowedDomains": []string{"*"}, "minTrustScore": 0.3},
			violate:        func(s *domain.MCPServer) { s.TrustScore = 0.1 },
			expectViolated: "Trust score below minimum",
		},
		{
			name:           "confidence score below minConfidenceScore",
			policyType:     domain.PolicyTypeMCPAllowlist,
			rules:          map[string]interface{}{"allowedDomains": []string{"*"}, "minConfidenceScore": 80.0},
			violate:        func(s *domain.MCPServer) { s.ConfidenceScore = 10.0 },
			expectViolated: "Confidence score below minimum",
		},
		{
			name:           "attestations below minAttestations",
			policyType:     domain.PolicyTypeMCPAllowlist,
			rules:          map[string]interface{}{"allowedDomains": []string{"*"}, "minAttestations": 2},
			violate:        func(s *domain.MCPServer) { s.AttestationCount = 1 },
			expectViolated: "Insufficient attestations",
		},
		{
			// An exact domain keeps this case's failure attributable to allowedCapabilities
			// alone, not to the bare "*" match.
			name:       "capability outside allowedCapabilities",
			policyType: domain.PolicyTypeMCPAllowlist,
			rules: map[string]interface{}{
				"allowedDomains":      []string{"mcp.example.com"},
				"allowedCapabilities": []string{"tools", "resources", "prompts"},
			},
			violate:        func(s *domain.MCPServer) { s.Capabilities = append(s.Capabilities, "sampling") },
			expectViolated: "Capability not in allowlist",
		},
		{
			name:              "every domain blocked by a bare * blocklist",
			policyType:        domain.PolicyTypeMCPBlocklist,
			rules:             map[string]interface{}{"blockedDomains": []string{"*"}},
			noCompliantServer: true,
			violate:           func(*domain.MCPServer) {},
			expectViolated:    "Domain is blocked",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := &domain.SecurityPolicy{
				ID:                uuid.New(),
				OrganizationID:    orgID,
				Name:              tt.name,
				PolicyType:        tt.policyType,
				EnforcementAction: domain.EnforcementBlockAndAlert,
				Rules:             tt.rules,
				IsEnabled:         true,
			}
			repo := new(MockSecurityPolicyRepository)
			repo.On("GetActiveByOrganization", orgID).Return([]*domain.SecurityPolicy{policy}, nil)
			evaluator := NewMCPPolicyEvaluator(repo, nil)

			if !tt.noCompliantServer {
				results, err := evaluator.EvaluateMCPServer(context.Background(), compliant())
				require.NoError(t, err)
				require.Len(t, results, 1)
				assert.False(t, results[0].Triggered,
					"the compliant server must pass, or the rejection below proves nothing; violated: %v",
					results[0].ViolatedRules)
			}

			server := compliant()
			tt.violate(server)
			results, err := evaluator.EvaluateMCPServer(context.Background(), server)
			require.NoError(t, err)
			require.Len(t, results, 1)
			assert.True(t, results[0].Triggered, "a server violating %q must be rejected", tt.name)
			assert.True(t, results[0].ShouldBlock, "a block_and_alert policy must report ShouldBlock")
			assert.Equal(t, []string{tt.expectViolated}, results[0].ViolatedRules,
				"the server must be rejected for the rule it violates, not for another")
		})
	}
}

// TestMCPPolicy_BlocklistIgnoresHostCaseAndTrailingDot drives the four server URL and pattern
// pairs of #593 through a blocklist policy. A host name in another case, or with the trailing
// dot of its fully qualified form, names the same host, so each must be blocked.
func TestMCPPolicy_BlocklistIgnoresHostCaseAndTrailingDot(t *testing.T) {
	evaluator := &MCPPolicyEvaluator{}

	tests := []struct {
		url     string
		pattern string
	}{
		{url: "https://x.EVIL.com/sse", pattern: "*.evil.com"},
		{url: "https://x.evil.com./sse", pattern: "*.evil.com"},
		{url: "https://evil.com./sse", pattern: "evil.com"},
		{url: "https://evil.com/sse", pattern: "*.EVIL.com"},
	}

	for _, tt := range tests {
		t.Run(tt.url+" against "+tt.pattern, func(t *testing.T) {
			policy := &domain.SecurityPolicy{
				ID:         uuid.New(),
				PolicyType: domain.PolicyTypeMCPBlocklist,
				Rules:      map[string]interface{}{"blockedDomains": []string{tt.pattern}},
			}
			server := &domain.MCPServer{ID: uuid.New(), Name: "example-mcp", URL: tt.url}

			result := &domain.MCPPolicyEvaluationResult{}
			evaluator.evaluateBlocklist(server, policy, result)

			assert.True(t, result.Triggered, "%s must be blocked by %s", tt.url, tt.pattern)
			assert.Equal(t, []string{"Domain is blocked"}, result.ViolatedRules)
		})
	}
}

// TestMCPPolicy_UndecodableRulesReportAViolation covers a policy whose rules hold a value of a
// JSON type its rule struct cannot take. Each evaluator returned without a verdict, so such a
// policy admitted every server: an allowlist with the string floor "50" let a server off its
// domain list through. The policy is now reported as violated, for its rules, so it gets fixed.
func TestMCPPolicy_UndecodableRulesReportAViolation(t *testing.T) {
	evaluator := &MCPPolicyEvaluator{}

	server := func() *domain.MCPServer {
		return &domain.MCPServer{
			ID:           uuid.New(),
			Name:         "example-mcp",
			URL:          "https://mcp.example.com/sse",
			Status:       domain.MCPServerStatusVerified,
			IsVerified:   true,
			TrustScore:   0.9,
			Capabilities: []string{"tools"},
		}
	}

	// The control from #593: the same allowlist without the string floor rejects the server.
	control := &domain.SecurityPolicy{
		ID:         uuid.New(),
		PolicyType: domain.PolicyTypeMCPAllowlist,
		Rules:      map[string]interface{}{"allowedDomains": []string{"only.example.org"}},
	}
	controlResult, err := evaluator.evaluatePolicy(context.Background(), server(), control)
	require.NoError(t, err)
	require.Equal(t, []string{"Not in allowlist"}, controlResult.ViolatedRules)

	tests := []struct {
		name       string
		policyType domain.PolicyType
		rules      map[string]interface{}
		field      string
	}{
		{
			name:       "allowlist with a string minTrustScore",
			policyType: domain.PolicyTypeMCPAllowlist,
			rules:      map[string]interface{}{"allowedDomains": []string{"only.example.org"}, "minTrustScore": "50"},
			field:      "minTrustScore",
		},
		{
			name:       "blocklist with a string where a list belongs",
			policyType: domain.PolicyTypeMCPBlocklist,
			rules:      map[string]interface{}{"blockedDomains": "mcp.example.com"},
			field:      "blockedDomains",
		},
		{
			name:       "capabilities with a string count",
			policyType: domain.PolicyTypeMCPCapabilities,
			rules:      map[string]interface{}{"forbiddenCapabilities": []string{"tools"}, "maxCapabilityCount": "1"},
			field:      "maxCapabilityCount",
		},
		{
			name:       "unverified with a string flag",
			policyType: domain.PolicyTypeMCPUnverified,
			rules:      map[string]interface{}{"requireManualApproval": "yes"},
			field:      "requireManualApproval",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := &domain.SecurityPolicy{
				ID:                uuid.New(),
				PolicyType:        tt.policyType,
				EnforcementAction: domain.EnforcementAlertOnly,
				Rules:             tt.rules,
			}

			result, err := evaluator.evaluatePolicy(context.Background(), server(), policy)
			require.NoError(t, err)

			assert.True(t, result.Triggered, "a policy whose rules cannot be read must not pass the server")
			assert.True(t, result.ShouldAlert)
			assert.False(t, result.ShouldBlock, "an alert_only policy alerts and does not block")
			assert.Equal(t, []string{"Policy rules could not be decoded"}, result.ViolatedRules)
			assert.Contains(t, result.Reason, "rules."+tt.field)
		})
	}
}

// TestMCPPolicy_AllowedNamesAdmitAServer covers allowedNames: a server whose domain is not on
// allowedDomains, or a policy with no allowedDomains, is admitted by its name (case-insensitive),
// and an admitted server still goes through the checks behind the admission.
func TestMCPPolicy_AllowedNamesAdmitAServer(t *testing.T) {
	evaluator := &MCPPolicyEvaluator{}

	tests := []struct {
		name           string
		rules          map[string]interface{}
		serverName     string
		capabilities   []string
		expectViolated []string
		expectMatched  []string
	}{
		{
			name:          "a name on the list admits a server off the domain list",
			rules:         map[string]interface{}{"allowedDomains": []string{"only.example.org"}, "allowedNames": []string{"example-mcp"}},
			serverName:    "Example-MCP",
			expectMatched: []string{"name:example-mcp"},
		},
		{
			name:          "a name on the list admits a server when the policy lists no domains",
			rules:         map[string]interface{}{"allowedNames": []string{"example-mcp"}},
			serverName:    "example-mcp",
			expectMatched: []string{"name:example-mcp"},
		},
		{
			name: "a server admitted by name is still held to allowedCapabilities",
			rules: map[string]interface{}{
				"allowedNames":        []string{"example-mcp"},
				"allowedCapabilities": []string{"tools"},
			},
			serverName:     "example-mcp",
			capabilities:   []string{"tools", "sampling"},
			expectViolated: []string{"Capability not in allowlist"},
			expectMatched:  []string{"name:example-mcp"},
		},
		{
			name:           "a name off the list is not admitted",
			rules:          map[string]interface{}{"allowedNames": []string{"example-mcp"}},
			serverName:     "other-mcp",
			expectViolated: []string{"Not in allowlist"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := &domain.SecurityPolicy{ID: uuid.New(), PolicyType: domain.PolicyTypeMCPAllowlist, Rules: tt.rules}
			server := &domain.MCPServer{
				ID:           uuid.New(),
				Name:         tt.serverName,
				URL:          "https://mcp.example.com/sse",
				Status:       domain.MCPServerStatusVerified,
				TrustScore:   0.9,
				Capabilities: tt.capabilities,
			}

			result := &domain.MCPPolicyEvaluationResult{}
			evaluator.evaluateAllowlist(server, policy, result)

			assert.Equal(t, len(tt.expectViolated) > 0, result.Triggered)
			assert.Equal(t, tt.expectViolated, result.ViolatedRules)
			assert.Equal(t, tt.expectMatched, result.MatchedPatterns)
		})
	}
}

// TestMCPPolicy_AllowlistWithNeitherListEnforcesNothing pins the documented limit of an
// mcp_allowlist policy that sets neither allowedDomains nor allowedNames: no server is admitted,
// so none is checked, and no rule of the policy reports a violation.
func TestMCPPolicy_AllowlistWithNeitherListEnforcesNothing(t *testing.T) {
	evaluator := &MCPPolicyEvaluator{}

	policy := &domain.SecurityPolicy{
		ID:         uuid.New(),
		PolicyType: domain.PolicyTypeMCPAllowlist,
		Rules: map[string]interface{}{
			"allowedCapabilities": []string{"tools"},
			"requireVerified":     true,
			"minTrustScore":       0.9,
		},
	}
	server := &domain.MCPServer{
		ID:           uuid.New(),
		Name:         "example-mcp",
		URL:          "https://mcp.example.com/sse",
		Status:       domain.MCPServerStatusPending,
		TrustScore:   0.1,
		Capabilities: []string{"sampling"},
	}

	result := &domain.MCPPolicyEvaluationResult{}
	evaluator.evaluateAllowlist(server, policy, result)

	assert.False(t, result.Triggered)
	assert.Empty(t, result.ViolatedRules)
}

// TestMCPPolicy_SeededTrustedDomainsAllowlistAdmitsLoggingAndCompletions evaluates the rules of
// the seeded "Trusted MCP Server Domains" policy as migration 052 wrote them and as migration 130
// leaves them, against a server on one of its domains that declares logging and completions.
func TestMCPPolicy_SeededTrustedDomainsAllowlistAdmitsLoggingAndCompletions(t *testing.T) {
	evaluator := &MCPPolicyEvaluator{}
	seededDomains := []string{"localhost", "127.0.0.1", "*.local", "*.internal", "*.amazonaws.com", "*.azure.com", "*.googleapis.com"}

	tests := []struct {
		name            string
		capabilities    []string
		expectTriggered bool
	}{
		{name: "as migration 052 seeded it", capabilities: []string{"tools", "resources", "prompts"}, expectTriggered: true},
		{name: "after migration 130", capabilities: []string{"tools", "resources", "prompts", "logging", "completions"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := &domain.SecurityPolicy{
				ID:         uuid.New(),
				PolicyType: domain.PolicyTypeMCPAllowlist,
				Rules: map[string]interface{}{
					"allowedDomains":      seededDomains,
					"allowedNames":        []string{},
					"allowedCapabilities": tt.capabilities,
					"requireVerified":     false,
					"minTrustScore":       0,
					"minAttestations":     0,
				},
			}
			server := &domain.MCPServer{
				ID:           uuid.New(),
				Name:         "example-mcp",
				URL:          "https://mcp.example.amazonaws.com/sse",
				Status:       domain.MCPServerStatusVerified,
				TrustScore:   0.9,
				Capabilities: []string{"tools", "logging", "completions"},
			}

			result := &domain.MCPPolicyEvaluationResult{}
			evaluator.evaluateAllowlist(server, policy, result)

			assert.Equal(t, tt.expectTriggered, result.Triggered, "violated: %v", result.ViolatedRules)
		})
	}
}

// TestMCPPolicy_CapabilityAllowlistStopsAtTheFirstCapabilityOffTheList pins that a server
// declaring several capabilities off the list is reported once, for the first of them.
func TestMCPPolicy_CapabilityAllowlistStopsAtTheFirstCapabilityOffTheList(t *testing.T) {
	evaluator := &MCPPolicyEvaluator{}

	policy := &domain.SecurityPolicy{
		ID:         uuid.New(),
		PolicyType: domain.PolicyTypeMCPAllowlist,
		Rules: map[string]interface{}{
			"allowedDomains":      []string{"mcp.example.com"},
			"allowedCapabilities": []string{"tools"},
		},
	}
	server := &domain.MCPServer{
		ID:           uuid.New(),
		Name:         "example-mcp",
		URL:          "https://mcp.example.com/sse",
		Status:       domain.MCPServerStatusVerified,
		TrustScore:   0.9,
		Capabilities: []string{"tools", "sampling", "elicitation"},
	}

	result := &domain.MCPPolicyEvaluationResult{}
	evaluator.evaluateAllowlist(server, policy, result)

	assert.True(t, result.Triggered)
	assert.Equal(t, []string{"Capability not in allowlist"}, result.ViolatedRules)
	assert.Equal(t, "MCP server has capability not in allowlist: sampling", result.Reason)
}
