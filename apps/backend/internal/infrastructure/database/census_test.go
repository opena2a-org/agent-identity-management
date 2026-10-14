package database_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/database"
)

// memConnector is an in-memory driver that accepts every statement and
// returns no rows, so the census can be driven through database/sql without
// a database.
type memConnector struct{}

func (memConnector) Connect(context.Context) (driver.Conn, error) { return memConn{}, nil }
func (memConnector) Driver() driver.Driver                        { return memDriver{} }

type memDriver struct{}

func (memDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("memDriver: open through the connector")
}

type memConn struct{}

func (memConn) Prepare(string) (driver.Stmt, error) { return memStmt{}, nil }
func (memConn) Close() error                        { return nil }
func (memConn) Begin() (driver.Tx, error)           { return memTx{}, nil }
func (memConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	return memTx{}, nil
}
func (memConn) ExecContext(context.Context, string, []driver.NamedValue) (driver.Result, error) {
	return driver.RowsAffected(1), nil
}
func (memConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	return memRows{}, nil
}

type memTx struct{}

func (memTx) Commit() error   { return nil }
func (memTx) Rollback() error { return nil }

type memStmt struct{}

func (memStmt) Close() error                               { return nil }
func (memStmt) NumInput() int                              { return -1 }
func (memStmt) Exec([]driver.Value) (driver.Result, error) { return driver.RowsAffected(1), nil }
func (memStmt) Query([]driver.Value) (driver.Rows, error)  { return memRows{}, nil }

type memRows struct{}

func (memRows) Columns() []string         { return []string{"n"} }
func (memRows) Close() error              { return nil }
func (memRows) Next([]driver.Value) error { return io.EOF }

func openCensusTestPool(t *testing.T) (*sql.DB, *database.Census) {
	t.Helper()
	census := database.NewCensus()
	db := sql.OpenDB(database.WrapConnector(memConnector{}, "", census))
	db.SetMaxOpenConns(1) // one connection, so a transaction's state is seen to end with it
	t.Cleanup(func() { _ = db.Close() })
	return db, census
}

// nextLine returns the line after the caller's.
func nextLine() int {
	_, _, line, _ := runtime.Caller(1)
	return line + 1
}

const censusTestFile = "internal/infrastructure/database/census_test.go"

// (d) A statement naming agents on the bare pool is undeclared, and is
// recorded at the line that ran it; the same statement inside WithTenant or
// WithServiceScope is not. A transaction that declares nothing is
// undeclared, and a declaration ends with its transaction.
func TestCensus_CountsTenantScopedStatementsOutsideADeclaredScope(t *testing.T) {
	db, census := openCensusTestPool(t)
	ctx := context.Background()
	orgID := uuid.New()

	bareLine := nextLine()
	_, err := db.ExecContext(ctx, `UPDATE agents SET status = $1 WHERE id = $2`, "suspended", uuid.NewString())
	require.NoError(t, err)
	snapshot := census.Snapshot()
	assert.Equal(t, int64(1), snapshot.Statements)
	assert.Equal(t, int64(1), snapshot.Undeclared, "a statement naming agents on the bare pool is undeclared")
	require.Equal(t, []database.CensusSite{{Site: fmt.Sprintf("%s:%d", censusTestFile, bareLine), Count: 1}}, snapshot.Sites)

	require.NoError(t, database.WithTenant(ctx, db, orgID, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `UPDATE agents SET status = $1 WHERE id = $2`, "active", uuid.NewString())
		return err
	}))
	snapshot = census.Snapshot()
	assert.Equal(t, int64(2), snapshot.Statements)
	assert.Equal(t, int64(1), snapshot.Undeclared, "the same statement inside WithTenant is declared")

	require.NoError(t, database.WithServiceScope(ctx, db, "census test", func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `SELECT id FROM public.agents`)
		if err != nil {
			return err
		}
		defer rows.Close()
		return rows.Err()
	}))
	snapshot = census.Snapshot()
	assert.Equal(t, int64(3), snapshot.Statements)
	assert.Equal(t, int64(1), snapshot.Undeclared, "a statement inside WithServiceScope is declared")

	// The declaration went with the WithServiceScope transaction: a bare
	// statement, and a new transaction that declares nothing, on the same
	// connection are undeclared.
	afterLine := nextLine()
	_, err = db.ExecContext(ctx, `SELECT id FROM agents`)
	require.NoError(t, err)
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	txLine := nextLine()
	_, err = tx.ExecContext(ctx, `DELETE FROM "agents" WHERE id = $1`, uuid.NewString())
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	// A prepared statement counts at each execution.
	stmt, err := db.PrepareContext(ctx, `SELECT count(*) FROM agents`)
	require.NoError(t, err)
	defer stmt.Close()
	stmtLine := nextLine()
	_, err = stmt.ExecContext(ctx)
	require.NoError(t, err)

	snapshot = census.Snapshot()
	assert.Equal(t, int64(6), snapshot.Statements)
	assert.Equal(t, int64(4), snapshot.Undeclared)
	assert.ElementsMatch(t, []database.CensusSite{
		{Site: fmt.Sprintf("%s:%d", censusTestFile, bareLine), Count: 1},
		{Site: fmt.Sprintf("%s:%d", censusTestFile, afterLine), Count: 1},
		{Site: fmt.Sprintf("%s:%d", censusTestFile, txLine), Count: 1},
		{Site: fmt.Sprintf("%s:%d", censusTestFile, stmtLine), Count: 1},
	}, snapshot.Sites)
}

// Only text that names a tenant-scoped table as a word, outside string
// literals and comments, is counted.
func TestCensus_CountsOnlyStatementsNamingATenantScopedTable(t *testing.T) {
	db, census := openCensusTestPool(t)
	ctx := context.Background()

	for _, query := range []string{
		`SELECT 1`,
		`SELECT 'agents'`,
		`SELECT 1 -- agents`,
		`SELECT 1 /* FROM agents */`,
		`SELECT id FROM agents_archive`,
		`SELECT id FROM record_chains`,
		`SELECT set_config('app.current_organization_id', $1, true)`,
	} {
		_, err := db.ExecContext(ctx, query, "x")
		require.NoError(t, err, query)
	}
	snapshot := census.Snapshot()
	assert.Zero(t, snapshot.Statements, "none of these statements names a tenant-scoped table")
	assert.Zero(t, snapshot.Undeclared)

	for _, table := range database.TenantScopedTables {
		_, err := db.ExecContext(ctx, "SELECT count(*) FROM "+strings.ToUpper(table.Table))
		require.NoError(t, err)
	}
	snapshot = census.Snapshot()
	assert.Equal(t, int64(len(database.TenantScopedTables)), snapshot.Statements)
	assert.Equal(t, int64(len(database.TenantScopedTables)), snapshot.Undeclared)
}

// The report is counts and code locations only: neither statement text nor
// argument values appear in it.
func TestCensus_ReportPrintsCountsAndSitesOnly(t *testing.T) {
	db, census := openCensusTestPool(t)
	ctx := context.Background()
	const secretArgument = "argument-value-never-printed"

	first := nextLine()
	_, err := db.ExecContext(ctx, `UPDATE users SET email = $1`, secretArgument)
	require.NoError(t, err)
	var second int
	for i := 0; i < 2; i++ {
		second = nextLine()
		_, err = db.ExecContext(ctx, `SELECT id FROM api_keys WHERE key_hash = $1`, secretArgument)
		require.NoError(t, err)
	}
	require.NoError(t, database.WithTenant(ctx, db, uuid.New(), func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `SELECT id FROM tags`)
		return err
	}))

	var lines []string
	census.Report(func(format string, args ...any) { lines = append(lines, fmt.Sprintf(format, args...)) })
	assert.Equal(t, []string{
		"row security census: statements=4 undeclared=3",
		fmt.Sprintf("row security census: undeclared site=%s:%d count=2", censusTestFile, second),
		fmt.Sprintf("row security census: undeclared site=%s:%d count=1", censusTestFile, first),
	}, lines)
	for _, line := range lines {
		assert.NotContains(t, line, secretArgument)
		assert.NotContains(t, line, "UPDATE")
		assert.NotContains(t, line, "SELECT")
	}
}

// Start reports at the interval, and its stop function reports once more
// and stops.
func TestCensus_StartReportsAtTheIntervalAndAtStop(t *testing.T) {
	census := database.NewCensus()
	var (
		mu    sync.Mutex
		lines []string
	)
	reported := make(chan struct{}, 64)
	logf := func(format string, args ...any) {
		mu.Lock()
		lines = append(lines, fmt.Sprintf(format, args...))
		mu.Unlock()
		select {
		case reported <- struct{}{}:
		default:
		}
	}

	stop := census.Start(5*time.Millisecond, logf)
	select {
	case <-reported:
	case <-time.After(5 * time.Second):
		t.Fatal("no report within 5s at a 5ms interval")
	}
	stop()
	mu.Lock()
	afterStop := len(lines)
	last := lines[afterStop-1]
	mu.Unlock()
	assert.Equal(t, "row security census: statements=0 undeclared=0", last)

	stop()
	time.Sleep(30 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, afterStop, len(lines), "nothing is reported after stop, and a second stop reports nothing")
}

func TestCensusInterval_IsFixed(t *testing.T) {
	assert.Equal(t, 5*time.Minute, database.CensusInterval)
}
