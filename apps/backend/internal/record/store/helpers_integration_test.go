//go:build integration

package store

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"database/sql/driver"
	"errors"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record/trace"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

// These tests run against the repository's migrated Postgres:
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration ./internal/record/store/...

// tap wraps the database connections a test's writer uses. It keeps the
// statements they send, in order, with BEGIN, COMMIT and ROLLBACK, and can
// make a chosen statement block at the server first.
type tap struct {
	mu         sync.Mutex
	statements []string
	sleepFirst map[string]time.Duration
}

func (t *tap) note(s string) {
	t.mu.Lock()
	t.statements = append(t.statements, s)
	t.mu.Unlock()
}

func (t *tap) reset() {
	t.mu.Lock()
	t.statements = nil
	t.mu.Unlock()
}

func (t *tap) recorded() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]string(nil), t.statements...)
}

// blockBefore makes every later run of query first run pg_sleep for d on the
// same connection, inside the same transaction, so that the server's
// statement timeout applies to it as to the query.
func (t *tap) blockBefore(query string, d time.Duration) {
	t.mu.Lock()
	if t.sleepFirst == nil {
		t.sleepFirst = map[string]time.Duration{}
	}
	t.sleepFirst[query] = d
	t.mu.Unlock()
}

func (t *tap) sleepFor(query string) time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.sleepFirst[query]
}

type tapConnector struct {
	base driver.Connector
	tap  *tap
}

func (c tapConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.base.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &tapConn{Conn: conn, tap: c.tap}, nil
}

func (c tapConnector) Driver() driver.Driver { return c.base.Driver() }

type tapConn struct {
	driver.Conn
	tap *tap
}

func (c *tapConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	c.tap.note("BEGIN")
	tx, err := c.Conn.(driver.ConnBeginTx).BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &tapTx{Tx: tx, tap: c.tap}, nil
}

func (c *tapConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if err := c.block(ctx, query); err != nil {
		return nil, err
	}
	c.tap.note(query)
	return c.Conn.(driver.QueryerContext).QueryContext(ctx, query, args)
}

func (c *tapConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if err := c.block(ctx, query); err != nil {
		return nil, err
	}
	c.tap.note(query)
	return c.Conn.(driver.ExecerContext).ExecContext(ctx, query, args)
}

func (c *tapConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	return c.Conn.(driver.ConnPrepareContext).PrepareContext(ctx, query)
}

func (c *tapConn) Ping(ctx context.Context) error {
	return c.Conn.(driver.Pinger).Ping(ctx)
}

func (c *tapConn) ResetSession(ctx context.Context) error {
	return c.Conn.(driver.SessionResetter).ResetSession(ctx)
}

func (c *tapConn) IsValid() bool {
	return c.Conn.(driver.Validator).IsValid()
}

func (c *tapConn) block(ctx context.Context, query string) error {
	d := c.tap.sleepFor(query)
	if d == 0 {
		return nil
	}
	secs := strconv.FormatFloat(d.Seconds(), 'f', 3, 64)
	_, err := c.Conn.(driver.ExecerContext).ExecContext(ctx, "SELECT pg_sleep("+secs+")", nil)
	return err
}

type tapTx struct {
	driver.Tx
	tap *tap
}

func (t *tapTx) Commit() error {
	t.tap.note("COMMIT")
	return t.Tx.Commit()
}

func (t *tapTx) Rollback() error {
	t.tap.note("ROLLBACK")
	return t.Tx.Rollback()
}

func testDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping record store test")
	}
	return dsn
}

// openTapped opens a pool whose connections go through a tap.
func openTapped(t *testing.T, maxOpen int) (*sql.DB, *tap) {
	t.Helper()
	base, err := pq.NewConnector(testDSN(t))
	require.NoError(t, err)
	tp := &tap{}
	db := sql.OpenDB(tapConnector{base: base, tap: tp})
	if maxOpen > 0 {
		db.SetMaxOpenConns(maxOpen)
	}
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping())
	return db, tp
}

// openPlain opens a pool outside any writer, for planting and checking rows.
func openPlain(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("postgres", testDSN(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping())
	return db
}

// seedOrg inserts one organization and removes it, its chain and its audit
// rows on cleanup.
func seedOrg(t *testing.T, db *sql.DB) string {
	t.Helper()
	id := uuid.NewString()
	suffix := id[:8]
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM record_debts WHERE organization_id = $1`, id)
		_, _ = db.Exec(`DELETE FROM audit_records WHERE chain_id IN (SELECT id FROM record_chains WHERE organization_id = $1)`, id)
		_, _ = db.Exec(`DELETE FROM record_chains WHERE organization_id = $1`, id)
		_, _ = db.Exec(`DELETE FROM audit_logs WHERE organization_id = $1`, id)
		_, _ = db.Exec(`DELETE FROM organizations WHERE id = $1`, id)
	})
	_, err := db.Exec(
		`INSERT INTO organizations (id, name, domain, created_at, updated_at)
		 VALUES ($1, $2, $3, NOW(), NOW())`,
		id, "recordstore-org-"+suffix, "recordstore-"+suffix+".example.com")
	require.NoError(t, err)
	return id
}

// testKeys is a record key generated when the test runs. A non-nil fail
// makes every signing request fail.
type testKeys struct {
	public  ed25519.PublicKey
	private ed25519.PrivateKey
	fail    error
}

func newTestKeys(t *testing.T) *testKeys {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	return &testKeys{public: public, private: private}
}

func (k *testKeys) publicKey() record.PublicKey {
	return record.PublicKey{Alg: record.AlgEd25519, Key: append([]byte(nil), k.public...)}
}

func (k *testKeys) PublicKey(context.Context) (record.PublicKey, error) {
	return k.publicKey(), nil
}

func (k *testKeys) SignPayload(_ context.Context, class record.PayloadClass, payload record.Payload) (string, []byte, error) {
	if k.fail != nil {
		return "", nil, k.fail
	}
	message, err := record.SigningInput(class, payload)
	if err != nil {
		return "", nil, err
	}
	return k.publicKey().KeyID(), ed25519.Sign(k.private, message), nil
}

// syncBuffer is a log destination several goroutines write to.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) lines() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	text := strings.TrimRight(s.b.String(), "\n")
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

type harness struct {
	w    *Writer
	reg  *prometheus.Registry
	logs *syncBuffer
	keys *testKeys
}

func newHarness(t *testing.T, db *sql.DB, keys *testKeys) harness {
	t.Helper()
	if keys == nil {
		keys = newTestKeys(t)
	}
	reg := prometheus.NewRegistry()
	m, err := NewMetrics(reg)
	require.NoError(t, err)
	logs := &syncBuffer{}
	w, err := NewWriter(Config{
		DB: db, Keys: keys, Key: keys.publicKey(), Metrics: m,
		Logger: log.New(logs, "", 0),
	})
	require.NoError(t, err)
	return harness{w: w, reg: reg, logs: logs, keys: keys}
}

// failures returns the value of aim_record_write_failures_total for one
// class and reason, and the sum over every series.
func (h harness) failures(t *testing.T, class Class, reason Reason) (one, total float64) {
	t.Helper()
	families, err := h.reg.Gather()
	require.NoError(t, err)
	for _, f := range families {
		if f.GetName() != "aim_record_write_failures_total" {
			continue
		}
		for _, m := range f.GetMetric() {
			labels := map[string]string{}
			for _, l := range m.GetLabel() {
				labels[l.GetName()] = l.GetValue()
			}
			v := m.GetCounter().GetValue()
			total += v
			if labels["class"] == string(class) && labels["reason"] == string(reason) {
				one = v
			}
		}
	}
	return one, total
}

// testDraft is an administrative record that is the first of a trace minted
// for it alone.
func testDraft() record.Draft {
	d := record.Draft{
		EventID: uuid.NewString(),
		Type:    "opena2a.administrative",
		Retained: map[string]any{"opena2a": map[string]any{
			"admin_action":  "tag_created",
			"resource_type": "tag",
		}},
		Tenant: map[string]any{"opena2a": map[string]any{
			"resource_id": uuid.NewString(),
		}},
		Personal: map[string]any{"actor": uuid.NewString()},
	}
	ctx, err := trace.Begin(context.Background())
	if err == nil {
		err = trace.Stamp(ctx, &d, nil)
	}
	if err != nil {
		panic(err)
	}
	return d
}

func wantWriteError(t *testing.T, err error, class Class, reason Reason) {
	t.Helper()
	var we *WriteError
	require.True(t, errors.As(err, &we), "want a *WriteError, got %v", err)
	require.Equal(t, class, we.Class, "class of %v", err)
	require.Equal(t, reason, we.Reason, "reason of %v", err)
}

// waitForLockWaiters polls the server until n sessions wait on the append
// lock statement.
func waitForLockWaiters(t *testing.T, db *sql.DB, n int, within time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		var waiting int
		require.NoError(t, db.QueryRow(
			`SELECT count(*) FROM pg_stat_activity
			  WHERE datname = current_database() AND wait_event_type = 'Lock' AND query = $1`,
			lockChainQuery).Scan(&waiting))
		if waiting >= n {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// value returns the value of the one series of a counter or gauge family
// that carries no label.
func (h harness) value(t *testing.T, name string) float64 {
	t.Helper()
	families, err := h.reg.Gather()
	require.NoError(t, err)
	for _, f := range families {
		if f.GetName() != name {
			continue
		}
		require.Len(t, f.GetMetric(), 1, "%s has one series", name)
		m := f.GetMetric()[0]
		if m.GetCounter() != nil {
			return m.GetCounter().GetValue()
		}
		return m.GetGauge().GetValue()
	}
	t.Fatalf("no series named %s", name)
	return 0
}

func (h harness) counter(t *testing.T, name string) float64 { return h.value(t, name) }

// breakHead makes the organization's stored head name no record, so its
// chain reads notExtendable, and returns the function that restores it.
func breakHead(t *testing.T, db *sql.DB, organizationID string) (restore func()) {
	t.Helper()
	var hash string
	require.NoError(t, db.QueryRow(`SELECT head_hash FROM record_chains WHERE organization_id = $1`, organizationID).Scan(&hash))
	_, err := db.Exec(`UPDATE record_chains SET head_hash = repeat('0', 64) WHERE organization_id = $1`, organizationID)
	require.NoError(t, err)
	return func() {
		t.Helper()
		_, err := db.Exec(`UPDATE record_chains SET head_hash = $2 WHERE organization_id = $1`, organizationID, hash)
		require.NoError(t, err)
	}
}

// debtRows returns the organization's debt rows as JSON objects, by id.
func debtRows(t *testing.T, db *sql.DB, organizationID string) map[string]string {
	t.Helper()
	rows, err := db.Query(`SELECT id, row_to_json(d)::text FROM record_debts d WHERE organization_id = $1`, organizationID)
	require.NoError(t, err)
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var id, row string
		require.NoError(t, rows.Scan(&id, &row))
		out[id] = row
	}
	require.NoError(t, rows.Err())
	return out
}

// positions counts the records of the organization's chain.
func positions(t *testing.T, db *sql.DB, organizationID string) int {
	t.Helper()
	var n int
	require.NoError(t, db.QueryRow(`
SELECT count(*) FROM audit_records r JOIN record_chains c ON c.id = r.chain_id
 WHERE c.organization_id = $1`, organizationID).Scan(&n))
	return n
}

// markOrg is a state change a test can see: it sets the organization's
// max_agents to markedMaxAgents.
const markedMaxAgents = 4321

func markOrg(organizationID string) func(context.Context, *sql.Tx) error {
	return func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `UPDATE organizations SET max_agents = $2 WHERE id = $1`, organizationID, markedMaxAgents)
		return err
	}
}

func marked(t *testing.T, db *sql.DB, organizationID string) bool {
	t.Helper()
	var n sql.NullInt64
	require.NoError(t, db.QueryRow(`SELECT max_agents FROM organizations WHERE id = $1`, organizationID).Scan(&n))
	return n.Valid && n.Int64 == markedMaxAgents
}

func unmark(t *testing.T, db *sql.DB, organizationID string) {
	t.Helper()
	_, err := db.Exec(`UPDATE organizations SET max_agents = 1 WHERE id = $1`, organizationID)
	require.NoError(t, err)
}
