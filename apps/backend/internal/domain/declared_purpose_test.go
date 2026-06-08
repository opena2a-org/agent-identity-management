package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validPurpose is a coherent baseline declaration used as the starting point for
// mutation-based tests.
func validPurpose() *DeclaredPurpose {
	return &DeclaredPurpose{
		VocabVersion: "1",
		Statement:    "Processes customer billing inquiries and issues refunds.",
		Category:     "financial-operations",
		TaskScopes:   []string{"billing:inquiry", "billing:refund"},
		CapabilityJustification: map[string][]string{
			"db:read":  {"billing:inquiry"},
			"api:call": {"billing:refund"},
		},
		Autonomy:     AutonomySupervised,
		DataScopes:   []string{"customer.billing"},
		EgressScopes: []string{"api.stripe.com"},
	}
}

var validGrants = []string{"db:read", "api:call"}

func TestDeclaredPurpose_Validate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(p *DeclaredPurpose)
		grants  []string
		wantErr bool
	}{
		{"valid baseline", func(p *DeclaredPurpose) {}, validGrants, false},
		{"nil vocab version defaults ok", func(p *DeclaredPurpose) { p.VocabVersion = "" }, validGrants, false},
		{"unsupported vocab version", func(p *DeclaredPurpose) { p.VocabVersion = "2" }, validGrants, true},
		{"statement too long", func(p *DeclaredPurpose) {
			s := make([]rune, MaxPurposeStatementLength+1)
			for i := range s {
				s[i] = 'a'
			}
			p.Statement = string(s)
		}, validGrants, true},
		{"statement at limit ok", func(p *DeclaredPurpose) {
			s := make([]rune, MaxPurposeStatementLength)
			for i := range s {
				s[i] = 'a'
			}
			p.Statement = string(s)
		}, validGrants, false},
		{"empty category", func(p *DeclaredPurpose) { p.Category = "" }, validGrants, true},
		{"unknown bare category", func(p *DeclaredPurpose) { p.Category = "totally-made-up" }, validGrants, true},
		{"valid custom category", func(p *DeclaredPurpose) {
			p.Category = "acme.fx-trading"
		}, validGrants, false},
		{"malformed custom category (3 parts)", func(p *DeclaredPurpose) { p.Category = "a.b.c" }, validGrants, true},
		{"invalid autonomy", func(p *DeclaredPurpose) { p.Autonomy = "yolo" }, validGrants, true},
		{"empty autonomy ok", func(p *DeclaredPurpose) { p.Autonomy = "" }, validGrants, false},
		{"no task scopes", func(p *DeclaredPurpose) { p.TaskScopes = nil }, validGrants, true},
		{"bad task scope grammar", func(p *DeclaredPurpose) { p.TaskScopes = []string{"Billing:Inquiry"} }, validGrants, true},
		{"justification key not granted", func(p *DeclaredPurpose) {
			p.CapabilityJustification["secrets:resolve"] = []string{"billing:inquiry"}
		}, validGrants, true},
		{"justification wildcard key", func(p *DeclaredPurpose) {
			p.CapabilityJustification = map[string][]string{"db:*": {"billing:inquiry"}}
		}, validGrants, true},
		{"justification references undeclared task scope", func(p *DeclaredPurpose) {
			p.CapabilityJustification["db:read"] = []string{"billing:nonexistent"}
		}, validGrants, true},
		{"egress with scheme rejected", func(p *DeclaredPurpose) { p.EgressScopes = []string{"https://api.stripe.com"} }, validGrants, true},
		{"egress with path rejected", func(p *DeclaredPurpose) { p.EgressScopes = []string{"api.stripe.com/v1/charges"} }, validGrants, true},
		{"egress bare host ok", func(p *DeclaredPurpose) { p.EgressScopes = []string{"hooks.internal.acme.com"} }, validGrants, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := validPurpose()
			tc.mutate(p)
			err := p.Validate(tc.grants)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestDeclaredPurpose_DualUseCategory: a dual-use category must justify every
// granted capability explicitly; an unjustified grant is a hard error.
func TestDeclaredPurpose_DualUseCategory(t *testing.T) {
	grants := []string{"system:exec", "network:connect"}
	p := &DeclaredPurpose{
		Category:   "security-operations",
		TaskScopes: []string{"secops:scan"},
		CapabilityJustification: map[string][]string{
			"system:exec": {"secops:scan"},
			// network:connect deliberately left unjustified
		},
	}
	require.Error(t, p.Validate(grants), "dual-use category must reject an unjustified granted capability")

	p.CapabilityJustification["network:connect"] = []string{"secops:scan"}
	require.NoError(t, p.Validate(grants), "dual-use category passes once every grant is justified")
}

func TestDeclaredPurpose_NilValidateIsNoop(t *testing.T) {
	var p *DeclaredPurpose
	require.NoError(t, p.Validate(validGrants))
	assert.Nil(t, p.CoherenceWarnings(validGrants))
}

func TestDeclaredPurpose_DeclaredBreadth(t *testing.T) {
	p := &DeclaredPurpose{
		CapabilityJustification: map[string][]string{
			"db:read":         {"x:y"},
			"db:write":        {"x:y"}, // same namespace -> still 1
			"network:connect": {"x:y"}, // second namespace -> 2
		},
	}
	assert.Equal(t, 2, p.DeclaredBreadth())
}

func TestDeclaredPurpose_CoherenceWarnings(t *testing.T) {
	t.Run("clean coherent purpose has no warnings", func(t *testing.T) {
		assert.Empty(t, validPurpose().CoherenceWarnings(validGrants))
	})

	t.Run("granted capability justified by no task", func(t *testing.T) {
		p := validPurpose()
		grants := append([]string{}, validGrants...)
		grants = append(grants, "data:export") // granted but not in justification
		w := p.CoherenceWarnings(grants)
		require.NotEmpty(t, w)
		assert.Contains(t, w[0], "data:export")
	})

	t.Run("dead authority: capability justified by empty task list", func(t *testing.T) {
		p := validPurpose()
		p.CapabilityJustification["db:read"] = []string{}
		w := p.CoherenceWarnings(validGrants)
		joined := ""
		for _, s := range w {
			joined += s + "\n"
		}
		assert.Contains(t, joined, "dead authority")
	})

	t.Run("over-broad: narrow category exceeding namespace budget", func(t *testing.T) {
		// customer-support is narrow (max 1 capability namespace); justify two.
		p := &DeclaredPurpose{
			Category:   "customer-support",
			TaskScopes: []string{"support:triage"},
			CapabilityJustification: map[string][]string{
				"db:read":         {"support:triage"},
				"network:connect": {"support:triage"},
			},
		}
		grants := []string{"db:read", "network:connect"}
		require.NoError(t, p.Validate(grants))
		w := p.CoherenceWarnings(grants)
		joined := ""
		for _, s := range w {
			joined += s + "\n"
		}
		assert.Contains(t, joined, "exceeds")
	})

	t.Run("broad category never flags over-broad", func(t *testing.T) {
		p := &DeclaredPurpose{
			Category:   "agent-orchestration", // broad: unbounded
			TaskScopes: []string{"orchestrate:route"},
			CapabilityJustification: map[string][]string{
				"db:read":         {"orchestrate:route"},
				"network:connect": {"orchestrate:route"},
				"system:exec":     {"orchestrate:route"},
				"api:call":        {"orchestrate:route"},
			},
		}
		grants := []string{"db:read", "network:connect", "system:exec", "api:call"}
		require.NoError(t, p.Validate(grants))
		for _, s := range p.CoherenceWarnings(grants) {
			assert.NotContains(t, s, "exceeds")
		}
	})
}

func TestPurposeCategoryHelpers(t *testing.T) {
	assert.False(t, IsCustomPurposeCategory("financial-operations"), "core category is not custom")
	assert.True(t, IsCustomPurposeCategory("acme.fx-trading"))
	assert.False(t, IsCustomPurposeCategory("nodot"))
	assert.False(t, IsCustomPurposeCategory("a.b.c"))

	assert.Equal(t, PurposeBreadthBroad, CategoryBreadthClass("acme.fx-trading"), "custom defaults to broad")
	assert.Equal(t, PurposeBreadthNarrow, CategoryBreadthClass("customer-support"))
	assert.Equal(t, PurposeBreadthBroad, CategoryBreadthClass("devops-automation"))

	assert.True(t, IsReservedTaskScopeNamespace("billing"))
	assert.False(t, IsReservedTaskScopeNamespace("acme"))

	assert.Len(t, CorePurposeCategories, 15, "locked core vocab has 15 categories, no catch-all")
	assert.Len(t, ReservedTaskScopeNamespaces, 17, "locked core vocab has 17 reserved taskScope namespaces")
}
