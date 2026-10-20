package metrics

import (
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const v2RefusalFamily = "aim_agent_request_v2_refusals_total"

// The reason label values, written out so this file states them independently of the
// declarations it checks.
var wantAgentRequestV2RefusalReasons = []string{
	"audience_not_configured", "conflicting_authentication", "statement_malformed", "nonce_missing",
	"signature_header_malformed", "audience_mismatch", "method_mismatch", "target_mismatch", "body_mismatch",
	"agent_lookup_failed", "no_registered_key", "registered_key_malformed", "signature_invalid_ed25519",
	"signature_invalid_mldsa", "hybrid_required", "single_signature_required", "skew_past", "skew_future",
	"nonce_reused", "freshness_check_unavailable", "agent_status_denied", "unclassified",
}

// v2RefusalSeries gathers aim_agent_request_v2_refusals_total from the registry /metrics
// serves, each series as "reason.sdk". A series with a label other than reason and sdk
// fails the test.
func v2RefusalSeries(t *testing.T) map[string]float64 {
	t.Helper()
	families, err := registry.Gather()
	require.NoError(t, err)
	series := map[string]float64{}
	for _, family := range families {
		if family.GetName() != v2RefusalFamily {
			continue
		}
		for _, metric := range family.GetMetric() {
			labels := map[string]string{}
			for _, pair := range metric.GetLabel() {
				labels[pair.GetName()] = pair.GetValue()
			}
			require.Len(t, labels, 2, "the labels are reason and sdk, nothing else: %v", labels)
			require.Contains(t, labels, "reason")
			require.Contains(t, labels, "sdk")
			series[labels["reason"]+"."+labels["sdk"]] = metric.GetCounter().GetValue()
		}
	}
	return series
}

func TestAgentRequestV2RefusalReasons_AreTheDeclaredConstants(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "agent_request_v2_refusals.go", nil, 0)
	require.NoError(t, err)
	var declared []string
	for _, decl := range f.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			vs := spec.(*ast.ValueSpec)
			if typ, ok := vs.Type.(*ast.Ident); !ok || typ.Name != "AgentRequestV2RefusalReason" {
				continue
			}
			for _, value := range vs.Values {
				lit, ok := value.(*ast.BasicLit)
				require.True(t, ok)
				s, err := strconv.Unquote(lit.Value)
				require.NoError(t, err)
				declared = append(declared, s)
			}
		}
	}
	var accepted []string
	for reason := range agentRequestV2RefusalReasons {
		accepted = append(accepted, string(reason))
	}
	want := append([]string(nil), wantAgentRequestV2RefusalReasons...)
	sort.Strings(want)
	sort.Strings(declared)
	sort.Strings(accepted)
	assert.Equal(t, want, declared, "the constants of AgentRequestV2RefusalReason")
	assert.Equal(t, want, accepted, "the reasons RecordAgentRequestV2Refusal records as given")
}

func TestRecordAgentRequestV2Refusal_CountsByReasonAndSDK(t *testing.T) {
	for _, tc := range []struct{ name, userAgent, sdk string }{
		{"Python SDK", "AIM-Python-SDK/2.0.2", "python"},
		{"TypeScript SDK", "AIM-SDK-TypeScript/1.4.0", "typescript"},
		{"Java SDK", "AIM-Java-SDK/1.0.0", "java"},
		{"curl", "curl/8.7.1", "other"},
		{"no User-Agent", "", "other"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := v2RefusalSeries(t)
			RecordAgentRequestV2Refusal(AgentRequestV2ReasonAudienceMismatch, tc.userAgent)
			assert.Equal(t, map[string]float64{"audience_mismatch." + tc.sdk: 1},
				authenticatedMoved(before, v2RefusalSeries(t)))
		})
	}
}

// A value outside the set, which only a caller mistake can pass, is counted as
// unclassified and never becomes a label.
func TestRecordAgentRequestV2Refusal_CountsAReasonOutsideTheSetAsUnclassified(t *testing.T) {
	before := v2RefusalSeries(t)
	RecordAgentRequestV2Refusal(AgentRequestV2RefusalReason("planted-not-a-reason"), "curl/8.7.1")
	RecordAgentRequestV2Refusal("", "curl/8.7.1")
	after := v2RefusalSeries(t)
	assert.Equal(t, map[string]float64{"unclassified.other": 2}, authenticatedMoved(before, after))
	for series := range after {
		assert.NotContains(t, series, "planted-not-a-reason")
	}
}

// A refusal is not counted as an authenticated request.
func TestRecordAgentRequestV2Refusal_MovesNoAuthenticatedSeries(t *testing.T) {
	before := authenticatedSeries(t, registry, authenticatedFamily)
	RecordAgentRequestV2Refusal(AgentRequestV2ReasonNonceReused, "AIM-SDK-TypeScript/1.4.0")
	assert.Empty(t, authenticatedMoved(before, authenticatedSeries(t, registry, authenticatedFamily)))
}
