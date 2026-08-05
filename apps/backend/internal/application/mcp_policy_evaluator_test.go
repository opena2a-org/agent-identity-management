package application

import (
	"testing"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
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
