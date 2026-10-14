package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"

	"github.com/lib/pq"
)

// The server opens two handles on the same login.
//
// The request pool serves requests and the background jobs. When
// POSTGRES_POOL_ROLE names a role, every connection the pool opens runs
// SET ROLE to it before database/sql receives the connection, so every
// statement on the pool runs with that role's privileges and under the
// row-level security policies that apply to it.
//
// The migration handle runs the schema migrations and the one-time startup
// data migrations. It never sets a role, so it acts as the login itself.

// OpenPool opens the request pool on dsn.
//
// When role is non-empty, every new connection runs SET ROLE with role quoted
// as an identifier, once, before database/sql receives it. A refused SET ROLE
// closes that connection and returns the refusal, so a connection is never
// handed out under the login's own role: the pool cannot open, and its first
// ping fails. When role is empty the pool is lib/pq's own.
func OpenPool(dsn, role string) (*sql.DB, error) {
	return openPool(dsn, role, nil)
}

// OpenCensusPool is OpenPool with census counting every statement the pool
// runs; see Census.
func OpenCensusPool(dsn, role string, census *Census) (*sql.DB, error) {
	if census == nil {
		return nil, errors.New("database: OpenCensusPool needs a census")
	}
	return openPool(dsn, role, census)
}

// OpenMigrationHandle opens the handle the migrations run on. It never sets a
// role.
func OpenMigrationHandle(dsn string) (*sql.DB, error) {
	connector, err := pq.NewConnector(dsn)
	if err != nil {
		return nil, err
	}
	return sql.OpenDB(connector), nil
}

func openPool(dsn, role string, census *Census) (*sql.DB, error) {
	connector, err := pq.NewConnector(dsn)
	if err != nil {
		return nil, err
	}
	return sql.OpenDB(newConnector(connector, role, census)), nil
}

// newConnector returns base unchanged when there is no role to set and no
// census to count, and otherwise a connector that sets the role on, and
// counts the statements of, every connection base opens.
func newConnector(base driver.Connector, role string, census *Census) driver.Connector {
	if role == "" && census == nil {
		return base
	}
	return &poolConnector{base: base, role: role, census: census}
}

type poolConnector struct {
	base   driver.Connector
	role   string
	census *Census
}

// Connect opens a connection, sets the role on it, and only then returns it.
// With no census the connection returned is the base driver's own.
func (c *poolConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.base.Connect(ctx)
	if err != nil {
		return nil, err
	}
	if c.role != "" {
		if err := setRole(ctx, conn, c.role); err != nil {
			_ = conn.Close()
			return nil, err
		}
	}
	if c.census != nil {
		return &censusConn{conn: conn, census: c.census}, nil
	}
	return conn, nil
}

func (c *poolConnector) Driver() driver.Driver {
	return c.base.Driver()
}

func setRole(ctx context.Context, conn driver.Conn, role string) error {
	quoted := pq.QuoteIdentifier(role)
	execer, ok := conn.(driver.ExecerContext)
	if !ok {
		return fmt.Errorf("SET ROLE %s: the driver connection cannot execute a statement", quoted)
	}
	if _, err := execer.ExecContext(ctx, "SET ROLE "+quoted, nil); err != nil {
		return fmt.Errorf("SET ROLE %s: %w", quoted, err)
	}
	return nil
}
