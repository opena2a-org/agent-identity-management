package database

import (
	"context"
	"database/sql/driver"
	"errors"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// CensusInterval is how often the server prints the census while it runs.
const CensusInterval = 5 * time.Minute

// maxCachedQueries bounds how many distinct statement texts the census keeps
// the table lookup of.
const maxCachedQueries = 4096

// Census counts, on the request pool, the statements that would read or
// write nothing under row-level security: those whose text names a table in
// TenantScopedTables and that run outside a transaction in which set_config
// was called for TenantGUC or ServiceScopeGUC. It records where each one
// came from, as the first caller outside database/sql, sqlx, lib/pq and this
// package.
//
// Its output is counts and code locations only. It never keeps or prints
// statement text or argument values: it reads them to decide what to count,
// and keeps nothing but a per-text yes or no.
type Census struct {
	tables map[string]struct{}

	statements atomic.Int64
	undeclared atomic.Int64

	mu    sync.Mutex
	sites map[string]int64

	names       sync.Map // statement text -> whether it names a tenant-scoped table
	cachedNames atomic.Int64
}

// CensusSite is one code location and how many undeclared statements it ran.
type CensusSite struct {
	Site  string
	Count int64
}

// CensusSnapshot is the census at one moment. Statements counts the
// statements that named a tenant-scoped table; Undeclared counts those of
// them that ran outside a declared scope. Sites is ordered by count, highest
// first.
type CensusSnapshot struct {
	Statements int64
	Undeclared int64
	Sites      []CensusSite
}

// NewCensus returns an empty census over TenantScopedTables.
func NewCensus() *Census {
	tables := make(map[string]struct{}, len(TenantScopedTables))
	for _, t := range TenantScopedTables {
		tables[strings.ToLower(t.Table)] = struct{}{}
	}
	return &Census{tables: tables, sites: map[string]int64{}}
}

// Snapshot returns the counts so far.
func (c *Census) Snapshot() CensusSnapshot {
	undeclared := c.undeclared.Load()
	statements := c.statements.Load()
	c.mu.Lock()
	sites := make([]CensusSite, 0, len(c.sites))
	for site, count := range c.sites {
		sites = append(sites, CensusSite{Site: site, Count: count})
	}
	c.mu.Unlock()
	sort.Slice(sites, func(i, j int) bool {
		if sites[i].Count != sites[j].Count {
			return sites[i].Count > sites[j].Count
		}
		return sites[i].Site < sites[j].Site
	})
	return CensusSnapshot{Statements: statements, Undeclared: undeclared, Sites: sites}
}

// Report prints the census through logf: one line of totals, then one line
// per site.
func (c *Census) Report(logf func(format string, args ...any)) {
	s := c.Snapshot()
	logf("row security census: statements=%d undeclared=%d", s.Statements, s.Undeclared)
	for _, site := range s.Sites {
		logf("row security census: undeclared site=%s count=%d", site.Site, site.Count)
	}
}

// Start prints the census through logf every interval. The returned stop
// function ends that, prints the census a last time, and does nothing on a
// second call.
func (c *Census) Start(interval time.Duration, logf func(format string, args ...any)) (stop func()) {
	done := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				c.Report(logf)
			case <-done:
				return
			}
		}
	}()
	var once sync.Once
	return func() {
		once.Do(func() {
			close(done)
			<-finished
			c.Report(logf)
		})
	}
}

// observe counts one statement run on conn.
func (c *Census) observe(conn *censusConn, query string) {
	if !c.namesTenantScopedTable(query) {
		return
	}
	c.statements.Add(1)
	if conn.inTx && conn.declared {
		return
	}
	c.undeclared.Add(1)
	site := callerSite()
	c.mu.Lock()
	c.sites[site]++
	c.mu.Unlock()
}

func (c *Census) namesTenantScopedTable(query string) bool {
	if found, ok := c.names.Load(query); ok {
		return found.(bool)
	}
	found := namesTable(query, c.tables)
	if c.cachedNames.Add(1) <= maxCachedQueries {
		c.names.Store(query, found)
	}
	return found
}

// namesTable reports whether query names one of tables as a word outside
// string literals and comments, in any case, quoted or schema-qualified.
func namesTable(query string, tables map[string]struct{}) bool {
	for i := 0; i < len(query); {
		ch := query[i]
		switch {
		case ch == '\'':
			i = skipStringLiteral(query, i)
		case ch == '-' && i+1 < len(query) && query[i+1] == '-':
			for i < len(query) && query[i] != '\n' {
				i++
			}
		case ch == '/' && i+1 < len(query) && query[i+1] == '*':
			end := strings.Index(query[i+2:], "*/")
			if end < 0 {
				return false
			}
			i += 2 + end + 2
		case isWordByte(ch):
			start := i
			for i < len(query) && isWordByte(query[i]) {
				i++
			}
			if _, ok := tables[strings.ToLower(query[start:i])]; ok {
				return true
			}
		default:
			i++
		}
	}
	return false
}

// skipStringLiteral returns the index after the string literal that starts
// at s[i], a single quote.
func skipStringLiteral(s string, i int) int {
	for i++; i < len(s); i++ {
		if s[i] != '\'' {
			continue
		}
		if i+1 < len(s) && s[i+1] == '\'' {
			i++
			continue
		}
		return i + 1
	}
	return i
}

func isWordByte(ch byte) bool {
	return ch == '_' || ch == '$' || ch >= 0x80 ||
		('0' <= ch && ch <= '9') || ('a' <= ch && ch <= 'z') || ('A' <= ch && ch <= 'Z')
}

// declaresScope reports whether a statement calls set_config for the tenant
// identity or the service scope, naming the variable in its text or passing
// it as an argument.
func declaresScope(query string, args []driver.NamedValue) bool {
	lower := strings.ToLower(query)
	if !strings.Contains(lower, "set_config") {
		return false
	}
	if strings.Contains(lower, TenantGUC) || strings.Contains(lower, ServiceScopeGUC) {
		return true
	}
	for _, arg := range args {
		if s, ok := arg.Value.(string); ok && (s == TenantGUC || s == ServiceScopeGUC) {
			return true
		}
	}
	return false
}

// censusPackage is this package's import path.
var censusPackage = reflect.TypeOf((*Census)(nil)).Elem().PkgPath()

// sourceRoot is the directory this module's source files are compiled from,
// as runtime reports it, so a site reads internal/... rather than a build
// machine's absolute path.
var sourceRoot = func() string {
	const self = "internal/infrastructure/database/census.go"
	_, file, _, ok := runtime.Caller(0)
	if !ok || !strings.HasSuffix(file, self) {
		return ""
	}
	return strings.TrimSuffix(file, self)
}()

// callerSite returns file:line of the first caller outside database/sql,
// sqlx, lib/pq, this package and the runtime.
func callerSite() string {
	pcs := make([]uintptr, 64)
	frames := runtime.CallersFrames(pcs[:runtime.Callers(2, pcs)])
	for {
		frame, more := frames.Next()
		if frame.Function != "" && !isDatabaseLayer(functionPackage(frame.Function)) {
			return strings.TrimPrefix(frame.File, sourceRoot) + ":" + strconv.Itoa(frame.Line)
		}
		if !more {
			return "unknown"
		}
	}
}

func isDatabaseLayer(pkg string) bool {
	if pkg == censusPackage || pkg == "runtime" {
		return true
	}
	for _, layer := range []string{"database/sql", "github.com/jmoiron/sqlx", "github.com/lib/pq"} {
		if pkg == layer || strings.HasPrefix(pkg, layer+"/") {
			return true
		}
	}
	return false
}

// functionPackage returns the import path of a function name as runtime
// reports it, such as database/sql for database/sql.(*DB).QueryContext.
func functionPackage(function string) string {
	if i := strings.IndexByte(function, '['); i >= 0 {
		function = function[:i]
	}
	slash := strings.LastIndexByte(function, '/')
	if dot := strings.IndexByte(function[slash+1:], '.'); dot >= 0 {
		return function[:slash+1+dot]
	}
	return function
}

// censusConn counts the statements of one connection for the census. It
// implements the optional database/sql/driver interfaces lib/pq's connection
// implements, and only those, so database/sql drives it as it drives lib/pq.
//
// database/sql uses a connection from one goroutine at a time, so inTx and
// declared need no lock.
type censusConn struct {
	conn   driver.Conn
	census *Census

	inTx     bool // a transaction is open on the connection
	declared bool // set_config was called for a scope in that transaction
}

var (
	_ driver.Conn               = (*censusConn)(nil)
	_ driver.ExecerContext      = (*censusConn)(nil)
	_ driver.QueryerContext     = (*censusConn)(nil)
	_ driver.ConnPrepareContext = (*censusConn)(nil)
	_ driver.ConnBeginTx        = (*censusConn)(nil)
	_ driver.Pinger             = (*censusConn)(nil)
	_ driver.SessionResetter    = (*censusConn)(nil)
	_ driver.Validator          = (*censusConn)(nil)
)

// after records a statement that ran on the connection: it counts it, and
// marks the open transaction declared when the statement declared a scope.
// A statement the driver skipped is counted where it is run instead.
func (c *censusConn) after(query string, args []driver.NamedValue, err error) {
	if errors.Is(err, driver.ErrSkip) {
		return
	}
	c.census.observe(c, query)
	if err == nil && c.inTx && declaresScope(query, args) {
		c.declared = true
	}
}

func (c *censusConn) Prepare(query string) (driver.Stmt, error) {
	return c.PrepareContext(context.Background(), query)
}

func (c *censusConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	var (
		stmt driver.Stmt
		err  error
	)
	if preparer, ok := c.conn.(driver.ConnPrepareContext); ok {
		stmt, err = preparer.PrepareContext(ctx, query)
	} else {
		stmt, err = c.conn.Prepare(query)
	}
	if err != nil {
		return nil, err
	}
	return &censusStmt{stmt: stmt, conn: c, query: query}, nil
}

func (c *censusConn) Close() error {
	return c.conn.Close()
}

func (c *censusConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *censusConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	var (
		tx  driver.Tx
		err error
	)
	if beginner, ok := c.conn.(driver.ConnBeginTx); ok {
		tx, err = beginner.BeginTx(ctx, opts)
	} else if opts.Isolation == driver.IsolationLevel(0) && !opts.ReadOnly {
		tx, err = c.conn.Begin() //nolint:staticcheck // the fallback for a driver without ConnBeginTx
	} else {
		return nil, errors.New("database: the driver connection supports neither ConnBeginTx nor transaction options")
	}
	if err != nil {
		return nil, err
	}
	c.inTx, c.declared = true, false
	return &censusTx{tx: tx, conn: c}, nil
}

func (c *censusConn) Exec(query string, args []driver.Value) (driver.Result, error) {
	execer, ok := c.conn.(driver.Execer) //nolint:staticcheck // lib/pq implements it, so the wrapper does
	if !ok {
		return nil, driver.ErrSkip
	}
	result, err := execer.Exec(query, args)
	c.after(query, namedValues(args), err)
	return result, err
}

func (c *censusConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	execer, ok := c.conn.(driver.ExecerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	result, err := execer.ExecContext(ctx, query, args)
	c.after(query, args, err)
	return result, err
}

func (c *censusConn) Query(query string, args []driver.Value) (driver.Rows, error) {
	queryer, ok := c.conn.(driver.Queryer) //nolint:staticcheck // lib/pq implements it, so the wrapper does
	if !ok {
		return nil, driver.ErrSkip
	}
	rows, err := queryer.Query(query, args)
	c.after(query, namedValues(args), err)
	return rows, err
}

func (c *censusConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	queryer, ok := c.conn.(driver.QueryerContext)
	if !ok {
		return nil, driver.ErrSkip
	}
	rows, err := queryer.QueryContext(ctx, query, args)
	c.after(query, args, err)
	return rows, err
}

func (c *censusConn) Ping(ctx context.Context) error {
	if pinger, ok := c.conn.(driver.Pinger); ok {
		return pinger.Ping(ctx)
	}
	return nil
}

func (c *censusConn) ResetSession(ctx context.Context) error {
	if resetter, ok := c.conn.(driver.SessionResetter); ok {
		return resetter.ResetSession(ctx)
	}
	return nil
}

func (c *censusConn) IsValid() bool {
	if validator, ok := c.conn.(driver.Validator); ok {
		return validator.IsValid()
	}
	return true
}

// censusTx ends the connection's transaction state with the transaction.
type censusTx struct {
	tx   driver.Tx
	conn *censusConn
}

func (t *censusTx) Commit() error {
	defer t.end()
	return t.tx.Commit()
}

func (t *censusTx) Rollback() error {
	defer t.end()
	return t.tx.Rollback()
}

func (t *censusTx) end() {
	t.conn.inTx, t.conn.declared = false, false
}

// censusStmt counts each execution of a prepared statement on the
// connection that prepared it. It implements the optional interfaces lib/pq's
// statement implements, and only those.
type censusStmt struct {
	stmt  driver.Stmt
	conn  *censusConn
	query string
}

var (
	_ driver.StmtExecContext  = (*censusStmt)(nil)
	_ driver.StmtQueryContext = (*censusStmt)(nil)
)

func (s *censusStmt) Close() error {
	return s.stmt.Close()
}

func (s *censusStmt) NumInput() int {
	return s.stmt.NumInput()
}

func (s *censusStmt) Exec(args []driver.Value) (driver.Result, error) {
	result, err := s.stmt.Exec(args) //nolint:staticcheck // driver.Stmt requires it
	s.conn.after(s.query, namedValues(args), err)
	return result, err
}

func (s *censusStmt) Query(args []driver.Value) (driver.Rows, error) {
	rows, err := s.stmt.Query(args) //nolint:staticcheck // driver.Stmt requires it
	s.conn.after(s.query, namedValues(args), err)
	return rows, err
}

func (s *censusStmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	var (
		result driver.Result
		err    error
	)
	if execer, ok := s.stmt.(driver.StmtExecContext); ok {
		result, err = execer.ExecContext(ctx, args)
	} else if values, convErr := plainValues(args); convErr != nil {
		return nil, convErr
	} else {
		result, err = s.stmt.Exec(values) //nolint:staticcheck // the fallback for a driver without StmtExecContext
	}
	s.conn.after(s.query, args, err)
	return result, err
}

func (s *censusStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	var (
		rows driver.Rows
		err  error
	)
	if queryer, ok := s.stmt.(driver.StmtQueryContext); ok {
		rows, err = queryer.QueryContext(ctx, args)
	} else if values, convErr := plainValues(args); convErr != nil {
		return nil, convErr
	} else {
		rows, err = s.stmt.Query(values) //nolint:staticcheck // the fallback for a driver without StmtQueryContext
	}
	s.conn.after(s.query, args, err)
	return rows, err
}

func namedValues(args []driver.Value) []driver.NamedValue {
	named := make([]driver.NamedValue, len(args))
	for i, v := range args {
		named[i] = driver.NamedValue{Ordinal: i + 1, Value: v}
	}
	return named
}

func plainValues(args []driver.NamedValue) ([]driver.Value, error) {
	values := make([]driver.Value, len(args))
	for i, arg := range args {
		if arg.Name != "" {
			return nil, errors.New("database: the driver does not support named parameters")
		}
		values[i] = arg.Value
	}
	return values, nil
}
