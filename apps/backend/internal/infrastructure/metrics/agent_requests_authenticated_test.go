package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const authenticatedFamily = "aim_agent_requests_authenticated_total"

// The scheme and sdk label values, written out so this file states them
// independently of the declarations it checks.
var (
	wantAgentRequestSchemes = []string{
		"agent-request-v1", "agent-request-v2", "verification-read", "verification-body", "action-request-v1",
	}
	wantSDKLabels = []string{"python", "typescript", "java", "other"}
)

// authenticatedSeries gathers the series of the named family from gatherer, each as
// "type.sdk". A series with a label other than type and sdk fails the test.
func authenticatedSeries(t *testing.T, gatherer prometheus.Gatherer, name string) map[string]float64 {
	t.Helper()
	families, err := gatherer.Gather()
	require.NoError(t, err)
	series := map[string]float64{}
	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		for _, metric := range family.GetMetric() {
			labels := map[string]string{}
			for _, pair := range metric.GetLabel() {
				labels[pair.GetName()] = pair.GetValue()
			}
			require.Len(t, labels, 2, "the labels are type and sdk, nothing else: %v", labels)
			require.Contains(t, labels, "type")
			require.Contains(t, labels, "sdk")
			series[labels["type"]+"."+labels["sdk"]] = metric.GetCounter().GetValue()
		}
	}
	return series
}

func authenticatedMoved(before, after map[string]float64) map[string]float64 {
	moved := map[string]float64{}
	for series, value := range after {
		if delta := value - before[series]; delta != 0 {
			moved[series] = delta
		}
	}
	return moved
}

func TestAgentRequestSchemes_AreTheFiveSchemesTheAuthenticatorsPass(t *testing.T) {
	var got []string
	for _, scheme := range agentRequestSchemes {
		got = append(got, string(scheme))
	}
	assert.Equal(t, wantAgentRequestSchemes, got)
	assert.Equal(t, "unclassified", string(AgentRequestSchemeUnclassified))
	assert.NotContains(t, agentRequestSchemes, AgentRequestSchemeUnclassified)
}

func TestAgentRequestsAuthenticated_EverySchemeAndSDKSeriesStartsAtZero(t *testing.T) {
	want := map[string]float64{}
	for _, scheme := range wantAgentRequestSchemes {
		for _, sdk := range wantSDKLabels {
			want[scheme+"."+sdk] = 0
		}
	}

	fresh := prometheus.NewRegistry()
	fresh.MustRegister(withAgentRequestSeries(prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "test_agent_requests_authenticated_total", Help: "test"},
		[]string{"type", "sdk"},
	)))
	assert.Equal(t, want, authenticatedSeries(t, fresh, "test_agent_requests_authenticated_total"),
		"before any request, every scheme and sdk label has a series, at 0, and there is no other series")

	// The registry /metrics serves holds the same series. Other tests in this package
	// may have counted requests already, so only their presence is compared here.
	served := authenticatedSeries(t, registry, authenticatedFamily)
	for series := range want {
		assert.Contains(t, served, series)
	}
}

func TestRecordAgentRequestAuthenticated_CountsOneRequestUnderItsSchemeAndSDK(t *testing.T) {
	userAgents := []struct{ header, sdk string }{
		{"AIM-Python-SDK/2.0.2 (A2A)", "python"},
		{"AIM-SDK-TypeScript/1.4.0", "typescript"},
		{"AIM-Java-SDK/1.0.0", "java"},
		{"", "other"},
		{"curl/8.7.1 AIM-Python-SDK/2.0.2", "other"},
	}
	for _, scheme := range agentRequestSchemes {
		for _, ua := range userAgents {
			before := authenticatedSeries(t, registry, authenticatedFamily)
			RecordAgentRequestAuthenticated(scheme, ua.header)
			after := authenticatedSeries(t, registry, authenticatedFamily)
			assert.Equal(t, map[string]float64{string(scheme) + "." + ua.sdk: 1}, authenticatedMoved(before, after),
				"scheme %s, User-Agent %q", scheme, ua.header)
		}
	}
}

// A value outside the closed set never becomes a label: an authenticator passes a
// constant, and a mistake that passed request data is counted as unclassified.
func TestRecordAgentRequestAuthenticated_RecordsASchemeOutsideTheSetAsUnclassified(t *testing.T) {
	const requestValue = "6f1c2a54-2f0e-4c39-9d4f-3b8e2c7a9d10"

	before := authenticatedSeries(t, registry, authenticatedFamily)
	RecordAgentRequestAuthenticated(AgentRequestScheme(requestValue), "AIM-Java-SDK/1.0.0")
	RecordAgentRequestAuthenticated(AgentRequestSchemeUnclassified, "AIM-Java-SDK/1.0.0")
	after := authenticatedSeries(t, registry, authenticatedFamily)

	assert.Equal(t, map[string]float64{"unclassified.java": 2}, authenticatedMoved(before, after))
	for series := range after {
		assert.NotContains(t, series, requestValue)
	}
}
