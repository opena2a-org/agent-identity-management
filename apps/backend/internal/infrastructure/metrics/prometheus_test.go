package metrics

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeMethod(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"standard GET", "GET", "GET"},
		{"standard POST", "POST", "POST"},
		{"standard PUT", "PUT", "PUT"},
		{"standard DELETE", "DELETE", "DELETE"},
		{"standard PATCH", "PATCH", "PATCH"},
		{"standard HEAD", "HEAD", "HEAD"},
		{"standard OPTIONS", "OPTIONS", "OPTIONS"},
		{"garbled GETT", "GETT", "UNKNOWN"},
		{"garbled POS", "POS", "UNKNOWN"},
		{"garbled POSTT", "POSTT", "UNKNOWN"},
		{"lowercase get", "get", "GET"},
		{"mixed case Get", "Get", "GET"},
		{"trailing whitespace", "GET ", "GET"},
		{"leading whitespace", " GET", "GET"},
		{"trailing newline", "GET\n", "GET"},
		{"trailing tab", "GET\t", "GET"},
		{"empty string", "", "UNKNOWN"},
		{"random string", "FOOBAR", "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeMethod(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple path", "/api/v1/agents", "/api/v1/agents"},
		{"path with UUID", "/api/v1/agents/550e8400-e29b-41d4-a716-446655440000", "/api/v1/agents/:id"},
		{"path with numeric ID", "/api/v1/agents/12345", "/api/v1/agents/:id"},
		{"root path", "/", "/"},
		{"metrics path", "/metrics", "/metrics"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizePath(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRegistryGatherNoDuplicates(t *testing.T) {
	// Record some metrics to exercise collectors with realistic label combinations
	httpRequestsTotal.WithLabelValues("GET", "/api/v1/agents", "200").Inc()
	httpRequestsTotal.WithLabelValues("POST", "/api/v1/agents", "201").Inc()
	httpRequestDuration.WithLabelValues("GET", "/api/v1/agents", "200").Observe(0.05)
	RecordSecurityAlert("high", "brute_force")
	RecordAgentOperation("create", "success")
	RecordComplianceCheck("policy", "pass")
	RecordAuditLog("create", "agent")

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("registry.Gather() returned error (duplicate registration?): %v", err)
	}

	if len(families) == 0 {
		t.Fatal("registry.Gather() returned zero metric families")
	}

	// Verify no duplicate metric family names
	seen := make(map[string]bool)
	for _, f := range families {
		name := f.GetName()
		if seen[name] {
			t.Errorf("duplicate metric family: %s", name)
		}
		seen[name] = true
	}
}

func TestIsNumeric(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"all digits", "12345", true},
		{"single digit", "0", true},
		{"with letters", "123abc", false},
		{"empty string", "", true}, // edge case: empty has no non-digit chars
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isNumeric(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// perEntityLabelNames are label names that would identify one agent, MCP
// server, organization or person. On a small deployment such a series narrows
// to one organization, so no metric may carry them.
var perEntityLabelNames = []string{
	"agent_id", "agent_name", "mcp_id", "org_id", "organization_id",
	"user_id", "email", "did", "name",
}

// allowedLabelNames is the closed set of label names the package registers.
// Each one takes values from a fixed vocabulary, never from an identifier.
var allowedLabelNames = map[string]bool{
	"method": true, "path": true, "status": true,
	"severity": true, "type": true, "operation": true,
	"event_type": true, "check_type": true, "query_type": true,
	"action": true, "resource_type": true,
	// aim_s1_refusals_total: reason is an S1RefusalReason and sdk one of the
	// s1SDKUserAgentPrefixes labels or "other" (s1_refusals.go).
	"reason": true, "sdk": true,
}

var labelVecConstructor = regexp.MustCompile(`^New(Counter|Gauge|Histogram|Summary)Vec$`)

// registeredLabelNames parses the package's non-test sources and returns every
// label name in a label-name position (the last argument of a New*Vec call,
// the variable labels of NewDesc, the keys of ConstLabels), with a count. A
// position whose names are not string literals is reported as an error, so a
// name cannot reach the registry without this census seeing it.
func registeredLabelNames(t *testing.T) map[string]int {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}
	fset := token.NewFileSet()
	names := map[string]int{}
	literalNames := func(where string, expr ast.Expr) {
		lit, ok := expr.(*ast.CompositeLit)
		if !ok {
			t.Errorf("%s: label names are not written as a literal list", where)
			return
		}
		for _, elt := range lit.Elts {
			if kv, ok := elt.(*ast.KeyValueExpr); ok {
				elt = kv.Key
			}
			bl, ok := elt.(*ast.BasicLit)
			if !ok || bl.Kind != token.STRING {
				t.Errorf("%s: a label name is not a string literal", where)
				continue
			}
			name, err := strconv.Unquote(bl.Value)
			if err != nil {
				t.Errorf("%s: unquote %s: %v", where, bl.Value, err)
				continue
			}
			names[name]++
		}
	}
	parsed := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, e.Name(), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", e.Name(), err)
		}
		parsed++
		ast.Inspect(f, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.CallExpr:
				sel, ok := n.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				where := fset.Position(n.Pos()).String()
				switch {
				case labelVecConstructor.MatchString(sel.Sel.Name) && len(n.Args) > 0:
					literalNames(where, n.Args[len(n.Args)-1])
				case sel.Sel.Name == "NewDesc" && len(n.Args) >= 3:
					literalNames(where, n.Args[2])
				}
			case *ast.KeyValueExpr:
				if id, ok := n.Key.(*ast.Ident); ok && (id.Name == "ConstLabels" || id.Name == "VariableLabels") {
					literalNames(fset.Position(n.Pos()).String(), n.Value)
				}
			}
			return true
		})
	}
	if parsed == 0 {
		t.Fatal("no non-test Go source parsed in the metrics package")
	}
	return names
}

func TestMetricLabelNamesCarryNoPerEntityIdentifier(t *testing.T) {
	names := registeredLabelNames(t)

	// Positive control: the census reads the HTTP request families.
	for _, want := range []string{"method", "path", "status"} {
		assert.Positive(t, names[want], "label %q not found; the census is not reading the label lists", want)
	}

	for _, banned := range perEntityLabelNames {
		assert.Zero(t, names[banned], "label %q identifies an entity and must not be registered", banned)
	}

	var unknown []string
	for name := range names {
		if !allowedLabelNames[name] {
			unknown = append(unknown, name)
		}
	}
	sort.Strings(unknown)
	assert.Empty(t, unknown, "label names outside the reviewed set; a new name must take its values from a fixed vocabulary, never from an identifier")
}

func TestGatheredFamiliesCarryNoPerEntityLabel(t *testing.T) {
	// Observe every recorder so each family has a series to gather.
	httpRequestsTotal.WithLabelValues("GET", "/api/v1/agents", "200").Inc()
	httpRequestDuration.WithLabelValues("GET", "/api/v1/agents", "200").Observe(0.01)
	RecordSecurityAlert("high", "brute_force")
	RecordSecurityThreat("high", "injection")
	UpdateTrustScore(80)
	RecordAgentOperation("create", "success")
	UpdateActiveAgents(3)
	UpdateMCPServersTotal(2)
	RecordVerificationEvent("signature", "success")
	ObserveVerificationDuration("signature", 0.01)
	RecordComplianceCheck("policy", "pass")
	RecordComplianceViolation()
	UpdateDatabaseConnections(4)
	ObserveDatabaseQueryDuration("select", 0.01)
	RecordAPIKeyOperation("create", "success")
	UpdateActiveAPIKeys(1)
	RecordAuditLog("create", "agent")

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("registry.Gather(): %v", err)
	}

	banned := map[string]bool{}
	for _, n := range perEntityLabelNames {
		banned[n] = true
	}
	seen := map[string]bool{}
	for _, f := range families {
		seen[f.GetName()] = true
		for _, m := range f.GetMetric() {
			for _, lp := range m.GetLabel() {
				assert.False(t, banned[lp.GetName()], "family %s carries label %q", f.GetName(), lp.GetName())
				assert.True(t, allowedLabelNames[lp.GetName()], "family %s carries label %q outside the reviewed set", f.GetName(), lp.GetName())
			}
		}
	}

	assert.True(t, seen["aim_http_requests_total"], "positive control: aim_http_requests_total not gathered")
	assert.True(t, seen["aim_trust_score_distribution"], "positive control: aim_trust_score_distribution not gathered")
	assert.False(t, seen["aim_trust_score"], "the per-agent trust score gauge is gathered")
	assert.False(t, seen["aim_mcp_attestations_total"], "the per-agent MCP attestation counter is gathered")
}
