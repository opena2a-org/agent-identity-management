package store

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The pre-chain state holds only while no chain has started and a foreign
// key removes audit rows by cascade. Once a chain has started, such a
// foreign key is a finding, and the deployment is not pre-chain again.
func TestRecordPathStartState(t *testing.T) {
	cascading := []string{
		"audit_logs.audit_logs_organization_id_fkey",
		"verification_events.verification_events_agent_id_fkey",
	}
	const named = "audit_logs.audit_logs_organization_id_fkey,verification_events.verification_events_agent_id_fkey"
	cases := []struct {
		name      string
		cascading []string
		genesis   bool
		mode      PathMode
		finding   bool
		line      string
	}{
		{
			name: "no chain started, cascades present", cascading: cascading,
			mode: ModePreChain,
			line: "record_path_start state=pre_chain genesis=none cascading_foreign_keys=" + named +
				" missing_migration=cascading_foreign_key_replacement",
		},
		{
			name: "no chain started, no cascade",
			mode: ModeChained,
			line: "record_path_start state=chained genesis=none cascading_foreign_keys=none",
		},
		{
			name: "a chain started, no cascade", genesis: true,
			mode: ModeChained,
			line: "record_path_start state=chained genesis=written cascading_foreign_keys=none",
		},
		{
			name: "a chain started, cascades present", cascading: cascading, genesis: true,
			mode: ModeChained, finding: true,
			line: "SECURITY record_path_start state=chained genesis=written cascading_foreign_keys=" + named +
				" finding=cascading_foreign_keys",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newStartState(tc.cascading, tc.genesis)
			assert.Equal(t, tc.mode, s.Mode)
			assert.Equal(t, tc.genesis, s.GenesisWritten)
			assert.Equal(t, tc.cascading, s.Cascading)
			assert.Equal(t, tc.finding, s.Finding())
			assert.Equal(t, tc.line, s.Line())
		})
	}
}

// The start gauges say whether the process is in the pre-chain state and
// name each cascading foreign key, and carry no other label.
func TestRecordPathStartMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	m, err := NewStartMetrics(reg)
	require.NoError(t, err)

	read := func() (preChain float64, constraints map[string]float64) {
		t.Helper()
		families, err := reg.Gather()
		require.NoError(t, err)
		constraints = map[string]float64{}
		for _, f := range families {
			for _, series := range f.GetMetric() {
				switch f.GetName() {
				case "aim_record_path_pre_chain":
					require.Empty(t, series.GetLabel())
					preChain = series.GetGauge().GetValue()
				case "aim_record_cascading_foreign_keys":
					require.Len(t, series.GetLabel(), 1)
					require.Equal(t, "constraint", series.GetLabel()[0].GetName())
					constraints[series.GetLabel()[0].GetValue()] = series.GetGauge().GetValue()
				default:
					t.Fatalf("unexpected series %s", f.GetName())
				}
			}
		}
		return preChain, constraints
	}

	cascading := []string{"audit_logs.audit_logs_organization_id_fkey", "audit_logs.audit_logs_user_id_fkey"}
	m.Set(newStartState(cascading, false))
	preChain, constraints := read()
	assert.Equal(t, 1.0, preChain)
	assert.Equal(t, map[string]float64{
		"audit_logs.audit_logs_organization_id_fkey": 1,
		"audit_logs.audit_logs_user_id_fkey":         1,
	}, constraints)

	m.Set(newStartState(cascading[:1], true))
	preChain, constraints = read()
	assert.Equal(t, 0.0, preChain)
	assert.Equal(t, map[string]float64{"audit_logs.audit_logs_organization_id_fkey": 1}, constraints)

	m.Set(newStartState(nil, true))
	preChain, constraints = read()
	assert.Equal(t, 0.0, preChain)
	assert.Empty(t, constraints)
}
