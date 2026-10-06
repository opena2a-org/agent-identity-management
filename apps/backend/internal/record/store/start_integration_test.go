//go:build integration

package store

import (
	"context"
	"database/sql"
	"log"
	"testing"
	"time"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// shippedCascades are the foreign keys of the schema as shipped that remove
// audit_logs and verification_events rows by cascade.
var shippedCascades = []string{
	"audit_logs.audit_logs_organization_id_fkey",
	"audit_logs.audit_logs_user_id_fkey",
	"verification_events.verification_events_agent_id_fkey",
	"verification_events.verification_events_mcp_server_id_fkey",
	"verification_events.verification_events_organization_id_fkey",
}

// session opens a transaction that is rolled back when the test ends. A
// temporary table created in it shadows the table of the same name for that
// transaction alone and is gone at the rollback, so a cell can plant "no
// chain started" or "cascades replaced" without changing what any other test
// sees.
func session(t *testing.T, db *sql.DB) *sql.Tx {
	t.Helper()
	tx, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	return tx
}

// The start state as read from the database: on the shipped schema with no
// chain started the deployment is pre-chain, and the start line and gauges
// name the cascading foreign keys; once a chain has started each of them is
// a finding; with the cascades replaced and no chain started it is chained.
func TestRecordPathStartStateReadFromTheDatabase(t *testing.T) {
	plain := openPlain(t)
	ctx := context.Background()

	t.Run("shipped schema, no chain started", func(t *testing.T) {
		tx := session(t, plain)
		_, err := tx.ExecContext(ctx, `CREATE TEMPORARY TABLE record_chains (id UUID)`)
		require.NoError(t, err)

		reg := prometheus.NewRegistry()
		m, err := NewStartMetrics(reg)
		require.NoError(t, err)
		logs := &syncBuffer{}
		state, err := ReportStart(ctx, tx, log.New(logs, "", 0), m)
		require.NoError(t, err)

		assert.Equal(t, StartState{Mode: ModePreChain, Cascading: shippedCascades}, state)
		assert.False(t, state.Finding())
		assert.Equal(t, []string{state.Line()}, logs.lines())
		assert.Contains(t, state.Line(), "state=pre_chain")
		for _, name := range shippedCascades {
			assert.Contains(t, state.Line(), name)
		}
		families, err := reg.Gather()
		require.NoError(t, err)
		named := 0
		for _, f := range families {
			for _, series := range f.GetMetric() {
				switch f.GetName() {
				case "aim_record_path_pre_chain":
					assert.Equal(t, 1.0, series.GetGauge().GetValue())
				case "aim_record_cascading_foreign_keys":
					assert.Contains(t, shippedCascades, series.GetLabel()[0].GetValue())
					named++
				}
			}
		}
		assert.Equal(t, len(shippedCascades), named)
	})

	t.Run("shipped schema, a chain started", func(t *testing.T) {
		tx := session(t, plain)
		_, err := tx.ExecContext(ctx, `CREATE TEMPORARY TABLE record_chains (id UUID)`)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `INSERT INTO record_chains (id) VALUES (gen_random_uuid())`)
		require.NoError(t, err)

		state, err := ReadStartState(ctx, tx)
		require.NoError(t, err)
		assert.Equal(t, StartState{Mode: ModeChained, GenesisWritten: true, Cascading: shippedCascades}, state)
		assert.True(t, state.Finding())
		assert.Contains(t, state.Line(), "SECURITY record_path_start state=chained genesis=written")
		assert.Contains(t, state.Line(), "finding=cascading_foreign_keys")
	})

	t.Run("cascades replaced, no chain started", func(t *testing.T) {
		tx := session(t, plain)
		for _, stmt := range []string{
			`CREATE TEMPORARY TABLE record_chains (id UUID)`,
			`CREATE TEMPORARY TABLE organizations (id UUID PRIMARY KEY)`,
			`CREATE TEMPORARY TABLE audit_logs (
			     id UUID PRIMARY KEY,
			     organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE RESTRICT)`,
			`CREATE TEMPORARY TABLE verification_events (
			     id UUID PRIMARY KEY,
			     organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE RESTRICT)`,
		} {
			_, err := tx.ExecContext(ctx, stmt)
			require.NoError(t, err)
		}

		state, err := ReadStartState(ctx, tx)
		require.NoError(t, err)
		assert.Equal(t, StartState{Mode: ModeChained}, state)
		assert.False(t, state.Finding())
		assert.Equal(t, "record_path_start state=chained genesis=none cascading_foreign_keys=none", state.Line())
	})
}

// newPreChainHarness is newHarness for a writer started in the pre-chain
// state.
func newPreChainHarness(t *testing.T, db *sql.DB, keys *testKeys) harness {
	t.Helper()
	reg := prometheus.NewRegistry()
	m, err := NewMetrics(reg)
	require.NoError(t, err)
	logs := &syncBuffer{}
	w, err := NewWriter(Config{
		DB: db, Keys: keys, Key: keys.publicKey(), Metrics: m,
		Logger: log.New(logs, "", 0), PreChain: true,
	})
	require.NoError(t, err)
	return harness{w: w, reg: reg, logs: logs, keys: keys}
}

// In the pre-chain state a write for an organization whose chain has not
// started commits its state change with no record and no debt, in every
// class, and counts and logs no failure. An organization whose chain has
// started is appended to as before. Once the writer writes a genesis, the
// pre-chain state is over: a write for an organization with no chain is
// refused again.
func TestRecordWriterInThePreChainState(t *testing.T) {
	db, _ := openTapped(t, 0)
	plain := openPlain(t)
	ctx := context.Background()
	keys := newTestKeys(t)
	h := newPreChainHarness(t, db, keys)

	for _, class := range classes {
		t.Run("no chain, "+string(class), func(t *testing.T) {
			org := seedOrg(t, plain)
			draft := testDraft()
			out, err := h.w.Write(ctx, Write{Class: class, OrganizationID: org, Draft: draft, Apply: markOrg(org)})
			require.NoError(t, err)
			assert.Equal(t, Appended{EventID: draft.EventID, Unchained: true}, out)
			assert.True(t, marked(t, plain, org), "the state change committed")
			assert.Zero(t, positions(t, plain, org), "no record was appended")
			assert.Empty(t, debtRows(t, plain, org), "no debt was written")
			status, err := ReadChainState(ctx, plain, org)
			require.NoError(t, err)
			assert.Equal(t, record.ChainNotStarted, status.State)
		})
	}
	_, failures := h.failures(t, ClassExpansion, ReasonChainHead)
	assert.Zero(t, failures, "no write failure was counted")
	assert.Zero(t, h.counter(t, "aim_record_debts_written_total"))
	assert.Empty(t, h.logs.lines(), "no failure or debt line was written")
	assert.Equal(t, PathUnknown, h.w.RecordPath(time.Now()).State,
		"a write with no record is neither a success nor a failure of the record path")

	t.Run("a started chain is appended to", func(t *testing.T) {
		org := seedOrg(t, plain)
		other := newHarness(t, db, keys)
		_, err := other.w.start(ctx, org)
		require.NoError(t, err)
		out, err := h.w.Write(ctx, Write{Class: ClassExpansion, OrganizationID: org, Draft: testDraft(), Apply: markOrg(org)})
		require.NoError(t, err)
		assert.False(t, out.Unchained)
		assert.Equal(t, int64(1), out.Seq)
		assert.Equal(t, 2, positions(t, plain, org))
		assert.Equal(t, PathOK, h.w.RecordPath(time.Now()).State)
	})

	t.Run("a genesis ends the pre-chain state", func(t *testing.T) {
		started := seedOrg(t, plain)
		_, err := h.w.start(ctx, started)
		require.NoError(t, err)

		org := seedOrg(t, plain)
		_, err = h.w.Write(ctx, Write{Class: ClassExpansion, OrganizationID: org, Draft: testDraft(), Apply: markOrg(org)})
		wantWriteError(t, err, ClassExpansion, ReasonChainHead)
		assert.False(t, marked(t, plain, org), "the refused write committed nothing")
	})
}
