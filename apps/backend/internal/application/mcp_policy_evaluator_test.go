package application

import (
	"testing"

	"github.com/opena2a/identity/backend/internal/domain"
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
