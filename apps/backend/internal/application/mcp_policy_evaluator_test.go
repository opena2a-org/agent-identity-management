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
// Every capability a server declares must appear on the list; a server outside it is rejected.
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
