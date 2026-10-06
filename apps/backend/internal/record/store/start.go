package store

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

// EventRecordPathStart is the console line the record path writes once, at
// process start, with its start state. It names no organization.
const EventRecordPathStart = "record_path_start"

// PathMode is how the record path writes for an organization whose chain has
// not started. It is decided at process start. The set is closed.
type PathMode string

const (
	// ModePreChain: no chain has started in the deployment, and a foreign
	// key removes audit_logs or verification_events rows by cascade, so no
	// chain can start (Start refuses with *PreChainError). A write for an
	// organization with no chain commits its state change with no record and
	// no debt. The audit rows it writes are pre-genesis rows once that
	// organization's chain starts.
	ModePreChain PathMode = "pre_chain"
	// ModeChained: a write for an organization with no chain is refused with
	// reason chain_head.
	ModeChained PathMode = "chained"
)

// StartState is what the record path reads at process start.
type StartState struct {
	Mode PathMode
	// GenesisWritten is true when some organization's chain has started.
	GenesisWritten bool
	// Cascading names each foreign key that removes audit_logs or
	// verification_events rows by cascade, as table.constraint. While no
	// chain has started they keep the deployment in the pre-chain state
	// until a migration replaces them. Once a chain has started each one is a
	// finding: the deployment is not pre-chain again.
	Cascading []string
}

// newStartState decides the mode. The pre-chain state is entered only while
// no chain has started.
func newStartState(cascading []string, genesisWritten bool) StartState {
	mode := ModeChained
	if len(cascading) > 0 && !genesisWritten {
		mode = ModePreChain
	}
	return StartState{Mode: mode, GenesisWritten: genesisWritten, Cascading: cascading}
}

// Finding is true when a chain has started and a foreign key still removes
// audit rows by cascade: a cascade would remove chained rows with no record.
func (s StartState) Finding() bool {
	return s.GenesisWritten && len(s.Cascading) > 0
}

// Line is the start line. In the pre-chain state it names the foreign keys
// the missing migration replaces; with a finding it is a SECURITY line.
func (s StartState) Line() string {
	keys := "none"
	if len(s.Cascading) > 0 {
		keys = strings.Join(s.Cascading, ",")
	}
	genesis := "none"
	if s.GenesisWritten {
		genesis = "written"
	}
	line := fmt.Sprintf("%s state=%s genesis=%s cascading_foreign_keys=%s", EventRecordPathStart, s.Mode, genesis, keys)
	switch {
	case s.Mode == ModePreChain:
		return line + " missing_migration=cascading_foreign_key_replacement"
	case s.Finding():
		return "SECURITY " + line + " finding=cascading_foreign_keys"
	}
	return line
}

// genesisWrittenQuery reads whether any chain has started. A record_chains
// row is written in the same transaction as its chain's genesis. It reads
// one boolean and no organization.
const genesisWrittenQuery = `SELECT EXISTS (SELECT 1 FROM record_chains)`

// ReadStartState reads the record path's start state.
func ReadStartState(ctx context.Context, q Querier) (StartState, error) {
	cascading, err := CascadingForeignKeys(ctx, q)
	if err != nil {
		return StartState{}, err
	}
	var written bool
	if err := q.QueryRowContext(ctx, genesisWrittenQuery).Scan(&written); err != nil {
		return StartState{}, fmt.Errorf("store: read whether a chain has started: %w", err)
	}
	return newStartState(cascading, written), nil
}

// StartMetrics are the record path's start gauges. No series carries an
// organization label.
type StartMetrics struct {
	preChain  prometheus.Gauge
	cascading *prometheus.GaugeVec
}

// NewStartMetrics registers the start gauges with reg.
func NewStartMetrics(reg prometheus.Registerer) (*StartMetrics, error) {
	m := &StartMetrics{
		preChain: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "aim_record_path_pre_chain",
			Help: "1 while the record path is in the pre-chain state: no chain has started and a foreign key removes audit rows by cascade",
		}),
		cascading: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "aim_record_cascading_foreign_keys",
			Help: "1 for each foreign key that removes audit_logs or verification_events rows by cascade, read at process start; after a chain has started each is a finding",
		}, []string{"constraint"}),
	}
	for _, c := range []prometheus.Collector{m.preChain, m.cascading} {
		if err := reg.Register(c); err != nil {
			return nil, fmt.Errorf("store: register start metrics: %w", err)
		}
	}
	return m, nil
}

// Set sets the gauges to s.
func (m *StartMetrics) Set(s StartState) {
	if s.Mode == ModePreChain {
		m.preChain.Set(1)
	} else {
		m.preChain.Set(0)
	}
	m.cascading.Reset()
	for _, name := range s.Cascading {
		m.cascading.WithLabelValues(name).Set(1)
	}
}

// ReportStart reads the start state, sets m and writes the start line to
// logger. A writer of this process is configured with PreChain set when the
// state's mode is ModePreChain.
func ReportStart(ctx context.Context, q Querier, logger *log.Logger, m *StartMetrics) (StartState, error) {
	s, err := ReadStartState(ctx, q)
	if err != nil {
		return StartState{}, err
	}
	m.Set(s)
	logger.Print(s.Line())
	return s, nil
}
