package database_test

import (
	"bufio"
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/database"
)

// fakePostgres speaks enough of the PostgreSQL wire protocol for lib/pq to
// connect, run simple queries and prepare and close a statement. It trusts
// every login and records, per connection, each statement it receives, in
// order, before it answers it. SET ROLE is answered SET, or refused with
// SQLSTATE 42501 when refuseSetRole is true.
type fakePostgres struct {
	ln            net.Listener
	refuseSetRole bool

	mu       sync.Mutex
	sessions [][]string
	conns    []net.Conn
	wg       sync.WaitGroup
}

func startFakePostgres(t *testing.T, refuseSetRole bool) *fakePostgres {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := &fakePostgres{ln: ln, refuseSetRole: refuseSetRole}
	srv.wg.Add(1)
	go srv.accept()
	t.Cleanup(func() {
		_ = ln.Close()
		srv.mu.Lock()
		for _, c := range srv.conns {
			_ = c.Close()
		}
		srv.mu.Unlock()
		srv.wg.Wait()
	})
	return srv
}

func (s *fakePostgres) dsn() string {
	port := s.ln.Addr().(*net.TCPAddr).Port
	return fmt.Sprintf("host=127.0.0.1 port=%d user=aim_login dbname=aim sslmode=disable connect_timeout=5", port)
}

// statements returns a copy of the statements each connection received, one
// slice per connection in the order the connections were accepted.
func (s *fakePostgres) statements() [][]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([][]string, len(s.sessions))
	for i, session := range s.sessions {
		out[i] = append([]string(nil), session...)
	}
	return out
}

func (s *fakePostgres) accept() {
	defer s.wg.Done()
	for {
		c, err := s.ln.Accept()
		if err != nil {
			return
		}
		s.mu.Lock()
		s.conns = append(s.conns, c)
		s.sessions = append(s.sessions, nil)
		index := len(s.sessions) - 1
		s.mu.Unlock()
		s.wg.Add(1)
		go s.serve(c, index)
	}
}

func (s *fakePostgres) record(index int, statement string) {
	s.mu.Lock()
	s.sessions[index] = append(s.sessions[index], statement)
	s.mu.Unlock()
}

func (s *fakePostgres) serve(c net.Conn, index int) {
	defer s.wg.Done()
	defer c.Close()
	r := bufio.NewReader(c)

	// Startup: answer an SSL request with N, then trust the login.
	for {
		var length int32
		if err := binary.Read(r, binary.BigEndian, &length); err != nil {
			return
		}
		body := make([]byte, length-4)
		if _, err := io.ReadFull(r, body); err != nil {
			return
		}
		if binary.BigEndian.Uint32(body[:4]) == 80877103 {
			if _, err := c.Write([]byte{'N'}); err != nil {
				return
			}
			continue
		}
		break
	}
	if writeMessage(c, 'R', []byte{0, 0, 0, 0}) != nil || writeMessage(c, 'Z', []byte{'I'}) != nil {
		return
	}

	var pending [][2]any // message type and body, sent at the next Sync
	for {
		typ, err := r.ReadByte()
		if err != nil {
			return
		}
		var length int32
		if err := binary.Read(r, binary.BigEndian, &length); err != nil {
			return
		}
		body := make([]byte, length-4)
		if _, err := io.ReadFull(r, body); err != nil {
			return
		}
		switch typ {
		case 'Q':
			query := strings.TrimSuffix(string(body), "\x00")
			s.record(index, query)
			if err := s.answerSimpleQuery(c, query); err != nil {
				return
			}
		case 'P': // Parse: statement name, query, parameter types
			parts := strings.SplitN(string(body), "\x00", 3)
			if len(parts) >= 2 {
				s.record(index, parts[1])
			}
			pending = append(pending, [2]any{byte('1'), []byte(nil)})
		case 'D': // Describe: no parameters and no result columns
			pending = append(pending, [2]any{byte('t'), []byte{0, 0}}, [2]any{byte('n'), []byte(nil)})
		case 'C': // Close
			pending = append(pending, [2]any{byte('3'), []byte(nil)})
		case 'S': // Sync
			for _, m := range pending {
				if writeMessage(c, m[0].(byte), m[1].([]byte)) != nil {
					return
				}
			}
			pending = nil
			if writeMessage(c, 'Z', []byte{'I'}) != nil {
				return
			}
		case 'X':
			return
		default:
			return
		}
	}
}

func (s *fakePostgres) answerSimpleQuery(c net.Conn, query string) error {
	if strings.HasPrefix(query, "SET ROLE ") {
		if s.refuseSetRole {
			var e []byte
			for _, field := range []string{"SERROR", "VERROR", "C42501", "Mpermission denied to set role"} {
				e = append(e, field...)
				e = append(e, 0)
			}
			e = append(e, 0)
			if err := writeMessage(c, 'E', e); err != nil {
				return err
			}
			return writeMessage(c, 'Z', []byte{'I'})
		}
		if err := writeMessage(c, 'C', []byte("SET\x00")); err != nil {
			return err
		}
		return writeMessage(c, 'Z', []byte{'I'})
	}
	if err := writeMessage(c, 'I', nil); err != nil {
		return err
	}
	return writeMessage(c, 'Z', []byte{'I'})
}

func writeMessage(w io.Writer, typ byte, body []byte) error {
	msg := make([]byte, 5+len(body))
	msg[0] = typ
	binary.BigEndian.PutUint32(msg[1:5], uint32(4+len(body)))
	copy(msg[5:], body)
	_, err := w.Write(msg)
	return err
}

// (a) With a role, every new connection runs SET ROLE, quoted as an
// identifier, as its first statement, and runs it once: a connection the
// pool hands out again does not run it again.
func TestOpenPool_RunsSetRoleOncePerNewConnection(t *testing.T) {
	srv := startFakePostgres(t, false)
	role := `aim runtime "pool"`
	db, err := database.OpenPool(srv.dsn(), role)
	require.NoError(t, err)
	defer db.Close()
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(2)
	ctx := context.Background()

	first, err := db.Conn(ctx)
	require.NoError(t, err)
	second, err := db.Conn(ctx)
	require.NoError(t, err)
	require.NoError(t, first.PingContext(ctx))
	_, err = second.ExecContext(ctx, "SELECT 1")
	require.NoError(t, err)
	require.NoError(t, first.Close())
	require.NoError(t, second.Close())
	for i := 0; i < 4; i++ {
		_, err := db.ExecContext(ctx, "SELECT 1")
		require.NoError(t, err)
	}

	setRole := "SET ROLE " + pq.QuoteIdentifier(role)
	require.Equal(t, `SET ROLE "aim runtime ""pool"""`, setRole)
	sessions := srv.statements()
	require.Len(t, sessions, 2, "the pool opened two connections")
	for i, statements := range sessions {
		require.NotEmpty(t, statements, "connection %d", i)
		assert.Equal(t, setRole, statements[0], "SET ROLE is the first statement on connection %d", i)
		count := 0
		for _, s := range statements {
			if strings.HasPrefix(s, "SET ROLE") {
				count++
			}
		}
		assert.Equal(t, 1, count, "connection %d ran SET ROLE once; it ran %v", i, statements)
		assert.Greater(t, len(statements), 1, "connection %d ran work after SET ROLE", i)
	}
}

// (a) A refused SET ROLE fails the connection: the refusal reaches the
// caller, nothing else runs on that connection, and the pool holds no
// connection afterwards. The census pool behaves the same way.
func TestOpenPool_RefusedSetRoleFailsTheConnection(t *testing.T) {
	for _, withCensus := range []bool{false, true} {
		t.Run(fmt.Sprintf("census=%t", withCensus), func(t *testing.T) {
			srv := startFakePostgres(t, true)
			var (
				db  *sql.DB
				err error
			)
			if withCensus {
				db, err = database.OpenCensusPool(srv.dsn(), "aim_runtime", database.NewCensus())
			} else {
				db, err = database.OpenPool(srv.dsn(), "aim_runtime")
			}
			require.NoError(t, err)
			defer db.Close()
			ctx := context.Background()

			err = db.PingContext(ctx)
			require.Error(t, err)
			var pqErr *pq.Error
			require.True(t, errors.As(err, &pqErr), "the refusal reaches the caller: %v", err)
			assert.Equal(t, pq.ErrorCode("42501"), pqErr.Code)

			_, err = db.ExecContext(ctx, "SELECT 1")
			require.Error(t, err, "a statement on the pool fails while SET ROLE is refused")

			assert.Zero(t, db.Stats().OpenConnections, "the pool holds no connection after a refused SET ROLE")
			sessions := srv.statements()
			require.NotEmpty(t, sessions)
			for i, statements := range sessions {
				assert.Equal(t, []string{`SET ROLE "aim_runtime"`}, statements,
					"connection %d ran nothing under the login's own role", i)
			}
		})
	}
}

// OpenPool with no role, and the migration handle, never set a role.
func TestOpenPoolWithoutARoleAndOpenMigrationHandle_NeverSetARole(t *testing.T) {
	srv := startFakePostgres(t, true) // a SET ROLE would be refused, so a ping would fail
	ctx := context.Background()

	pool, err := database.OpenPool(srv.dsn(), "")
	require.NoError(t, err)
	require.NoError(t, pool.PingContext(ctx))
	require.NoError(t, pool.Close())

	handle, err := database.OpenMigrationHandle(srv.dsn())
	require.NoError(t, err)
	require.NoError(t, handle.PingContext(ctx))
	_, err = handle.ExecContext(ctx, "SELECT 1")
	require.NoError(t, err)
	require.NoError(t, handle.Close())

	sessions := srv.statements()
	require.Len(t, sessions, 2)
	for i, statements := range sessions {
		require.NotEmpty(t, statements)
		for _, s := range statements {
			assert.False(t, strings.HasPrefix(s, "SET ROLE"), "connection %d ran %q", i, s)
		}
	}
}

// connInterfaces is every optional interface database/sql looks for on a
// driver.Conn.
var connInterfaces = map[string]func(any) bool{
	"driver.Execer":             func(c any) bool { _, ok := c.(driver.Execer); return ok }, //nolint:staticcheck // the comparison covers the deprecated interfaces too
	"driver.ExecerContext":      func(c any) bool { _, ok := c.(driver.ExecerContext); return ok },
	"driver.Queryer":            func(c any) bool { _, ok := c.(driver.Queryer); return ok }, //nolint:staticcheck // the comparison covers the deprecated interfaces too
	"driver.QueryerContext":     func(c any) bool { _, ok := c.(driver.QueryerContext); return ok },
	"driver.ConnPrepareContext": func(c any) bool { _, ok := c.(driver.ConnPrepareContext); return ok },
	"driver.ConnBeginTx":        func(c any) bool { _, ok := c.(driver.ConnBeginTx); return ok },
	"driver.Pinger":             func(c any) bool { _, ok := c.(driver.Pinger); return ok },
	"driver.SessionResetter":    func(c any) bool { _, ok := c.(driver.SessionResetter); return ok },
	"driver.Validator":          func(c any) bool { _, ok := c.(driver.Validator); return ok },
	"driver.NamedValueChecker":  func(c any) bool { _, ok := c.(driver.NamedValueChecker); return ok },
}

// stmtInterfaces is every optional interface database/sql looks for on a
// driver.Stmt.
var stmtInterfaces = map[string]func(any) bool{
	"driver.StmtExecContext":   func(s any) bool { _, ok := s.(driver.StmtExecContext); return ok },
	"driver.StmtQueryContext":  func(s any) bool { _, ok := s.(driver.StmtQueryContext); return ok },
	"driver.NamedValueChecker": func(s any) bool { _, ok := s.(driver.NamedValueChecker); return ok },
	"driver.ColumnConverter":   func(s any) bool { _, ok := s.(driver.ColumnConverter); return ok }, //nolint:staticcheck // the comparison covers the deprecated interfaces too
}

// rawConnAndStmt returns the driver connection db hands out, and a statement
// prepared on it.
func rawConnAndStmt(t *testing.T, db *sql.DB) (conn any, stmt any) {
	t.Helper()
	ctx := context.Background()
	c, err := db.Conn(ctx)
	require.NoError(t, err)
	defer c.Close()
	require.NoError(t, c.Raw(func(dc any) error {
		conn = dc
		preparer, ok := dc.(driver.ConnPrepareContext)
		if !ok {
			return errors.New("the driver connection does not implement driver.ConnPrepareContext")
		}
		st, err := preparer.PrepareContext(ctx, "SELECT 1")
		if err != nil {
			return err
		}
		stmt = st
		return st.Close()
	}))
	return conn, stmt
}

// (b) The connection the census pool hands out, and the statements it
// prepares, implement the same optional database/sql/driver interfaces as
// lib/pq's own. The role-only pool hands out lib/pq's connection itself.
func TestPoolConnections_ImplementTheOptionalInterfacesOfLibPq(t *testing.T) {
	srv := startFakePostgres(t, false)

	handle, err := database.OpenMigrationHandle(srv.dsn())
	require.NoError(t, err)
	defer handle.Close()
	libpqConn, libpqStmt := rawConnAndStmt(t, handle)
	require.Equal(t, "github.com/lib/pq", reflect.TypeOf(libpqConn).Elem().PkgPath(),
		"the migration handle hands out lib/pq's connection")
	for _, name := range []string{"driver.ExecerContext", "driver.QueryerContext", "driver.ConnBeginTx", "driver.SessionResetter", "driver.Validator"} {
		require.True(t, connInterfaces[name](libpqConn), "lib/pq's connection implements %s", name)
	}
	require.True(t, stmtInterfaces["driver.StmtExecContext"](libpqStmt), "lib/pq's statement implements driver.StmtExecContext")

	rolePool, err := database.OpenPool(srv.dsn(), "aim_runtime")
	require.NoError(t, err)
	defer rolePool.Close()
	roleConn, _ := rawConnAndStmt(t, rolePool)
	assert.Equal(t, reflect.TypeOf(libpqConn), reflect.TypeOf(roleConn),
		"the role-only pool hands out lib/pq's connection after SET ROLE")

	censusPool, err := database.OpenCensusPool(srv.dsn(), "aim_runtime", database.NewCensus())
	require.NoError(t, err)
	defer censusPool.Close()
	censusConn, censusStmt := rawConnAndStmt(t, censusPool)
	require.NotEqual(t, reflect.TypeOf(libpqConn), reflect.TypeOf(censusConn), "the census pool wraps the connection")

	for name, implements := range connInterfaces {
		assert.Equal(t, implements(libpqConn), implements(censusConn), "connection: %s", name)
	}
	for name, implements := range stmtInterfaces {
		assert.Equal(t, implements(libpqStmt), implements(censusStmt), "statement: %s", name)
	}
}
