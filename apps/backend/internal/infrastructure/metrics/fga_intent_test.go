package metrics

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fgaIntentSeries reads every series of family from the registry the /metrics
// endpoint serves, keyed by its label values joined with "/" in label-name order.
func fgaIntentSeries(t *testing.T, family string) map[string]float64 {
	t.Helper()
	families, err := registry.Gather()
	require.NoError(t, err)
	out := map[string]float64{}
	for _, f := range families {
		if f.GetName() != family {
			continue
		}
		for _, m := range f.GetMetric() {
			var values []string
			for _, pair := range m.GetLabel() {
				values = append(values, pair.GetValue())
			}
			out[strings.Join(values, "/")] = m.GetCounter().GetValue()
		}
	}
	return out
}

func TestFGAIntentSeriesExistAtZero(t *testing.T) {
	checks := fgaIntentSeries(t, "aim_fga_intent_checks_total")
	skipped := fgaIntentSeries(t, "aim_fga_intent_skipped_total")
	// Labels gather in name order: agreement, risk_tier, status.
	for _, tier := range fgaIntentRiskTiers {
		for _, o := range fgaIntentOutcomes {
			_, ok := checks[o[1]+"/"+tier+"/"+o[0]]
			assert.True(t, ok, "missing series risk_tier=%s status=%s agreement=%s", tier, o[0], o[1])
		}
		_, ok := skipped[tier]
		assert.True(t, ok, "missing skipped series risk_tier=%s", tier)
	}
	assert.Len(t, checks, len(fgaIntentRiskTiers)*len(fgaIntentOutcomes), "a series outside the closed set exists")
	assert.Len(t, skipped, len(fgaIntentRiskTiers))
}

func TestRecordFGAIntentCheckLabels(t *testing.T) {
	cases := []struct {
		name                     string
		riskLevel, status, class string
		key                      string // agreement/risk_tier/status
	}{
		{"confident benign agrees", "HIGH", FGAIntentStatusClassified, "", "agree/high/classified"},
		{"attack class disagrees", "MEDIUM", FGAIntentStatusClassified, "exfiltration_pattern", "disagree/medium/classified"},
		{"abstain has no verdict", "HIGH", FGAIntentStatusAbstain, "exfiltration_pattern", "none/high/abstain"},
		{"fail_open has no verdict", "medium", FGAIntentStatusFailOpen, "unknown", "none/medium/fail_open"},
		{"unknown status is fail_open", "HIGH", "something-else", "x", "none/high/fail_open"},
		{"unknown tier is other", "CRITICAL", FGAIntentStatusClassified, "", "agree/other/classified"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := fgaIntentSeries(t, "aim_fga_intent_checks_total")
			RecordFGAIntentCheck(tc.riskLevel, tc.status, tc.class)
			after := fgaIntentSeries(t, "aim_fga_intent_checks_total")
			for key, v := range after {
				want := before[key]
				if key == tc.key {
					want++
				}
				assert.Equal(t, want, v, "series %s", key)
			}
			assert.Len(t, after, len(before), "the record created a series outside the closed set")
		})
	}
}

func TestRecordFGAIntentSkipped(t *testing.T) {
	before := fgaIntentSeries(t, "aim_fga_intent_skipped_total")
	RecordFGAIntentSkipped("LOW")
	RecordFGAIntentSkipped("")
	after := fgaIntentSeries(t, "aim_fga_intent_skipped_total")
	assert.Equal(t, before["low"]+1, after["low"])
	assert.Equal(t, before["other"]+1, after["other"])
	assert.Equal(t, before["high"], after["high"])
	assert.Equal(t, before["medium"], after["medium"])
}

// The Step 5 families are served on /metrics.
func TestFGAIntentFamiliesServed(t *testing.T) {
	RecordFGAIntentCheck("HIGH", FGAIntentStatusClassified, "")
	app := fiber.New()
	app.Get("/metrics", PrometheusHandler())
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/metrics", nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	exposition := string(body)
	assert.Contains(t, exposition, "# TYPE aim_fga_intent_checks_total counter\n")
	assert.Contains(t, exposition, "# TYPE aim_fga_intent_skipped_total counter\n")
	assert.Contains(t, exposition, `aim_fga_intent_checks_total{agreement="agree",risk_tier="high",status="classified"}`)
	assert.Contains(t, exposition, `aim_fga_intent_skipped_total{risk_tier="low"}`)
}
