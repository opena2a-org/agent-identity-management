package application

// MCPService.AddPublicKey on a store that answers every statement and records
// it. A refused key type or a key the helper refuses leaves the store with no
// insert and no update; an accepted key reaches it as exactly one insert.

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/base64"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	infracrypto "github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/crypto"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
)

// recordedStatement is one statement the store received, with its arguments.
type recordedStatement struct {
	query string
	args  []driver.Value
}

// statementStore is a database/sql driver that accepts every statement,
// records it, and answers a read of mcp_servers with one tool server row.
type statementStore struct {
	mu         sync.Mutex
	statements []recordedStatement
}

func (s *statementStore) record(query string, args []driver.NamedValue) {
	s.mu.Lock()
	defer s.mu.Unlock()
	values := make([]driver.Value, len(args))
	for i, a := range args {
		values[i] = a.Value
	}
	s.statements = append(s.statements, recordedStatement{query: strings.TrimSpace(query), args: values})
}

// writes returns the INSERT and UPDATE statements received so far.
func (s *statementStore) writes() []recordedStatement {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []recordedStatement
	for _, st := range s.statements {
		verb := strings.ToUpper(strings.Fields(st.query + " x")[0])
		if verb == "INSERT" || verb == "UPDATE" {
			out = append(out, st)
		}
	}
	return out
}

func (s *statementStore) Connect(context.Context) (driver.Conn, error) { return &statementConn{s}, nil }
func (s *statementStore) Driver() driver.Driver                        { return statementDriver{s} }

type statementDriver struct{ store *statementStore }

func (d statementDriver) Open(string) (driver.Conn, error) { return &statementConn{d.store}, nil }

type statementConn struct{ store *statementStore }

func (c *statementConn) Prepare(query string) (driver.Stmt, error) {
	return &statementStmt{conn: c, query: query}, nil
}
func (c *statementConn) Close() error              { return nil }
func (c *statementConn) Begin() (driver.Tx, error) { return statementTx{}, nil }

func (c *statementConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	c.store.record(query, args)
	return driver.RowsAffected(1), nil
}

func (c *statementConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	c.store.record(query, args)
	if strings.Contains(query, "FROM mcp_servers") {
		return &statementRows{values: [][]driver.Value{toolServerRowValues()}}, nil
	}
	return &statementRows{}, nil
}

type statementStmt struct {
	conn  *statementConn
	query string
}

func (s *statementStmt) Close() error  { return nil }
func (s *statementStmt) NumInput() int { return -1 }
func (s *statementStmt) Exec(args []driver.Value) (driver.Result, error) {
	return s.conn.ExecContext(context.Background(), s.query, named(args))
}
func (s *statementStmt) Query(args []driver.Value) (driver.Rows, error) {
	return s.conn.QueryContext(context.Background(), s.query, named(args))
}

func named(args []driver.Value) []driver.NamedValue {
	out := make([]driver.NamedValue, len(args))
	for i, v := range args {
		out[i] = driver.NamedValue{Ordinal: i + 1, Value: v}
	}
	return out
}

type statementTx struct{}

func (statementTx) Commit() error   { return nil }
func (statementTx) Rollback() error { return nil }

// toolServerColumns mirrors the SELECT list of MCPServerRepository.GetByID.
var toolServerColumns = []string{
	"id", "organization_id", "name", "description", "url", "version",
	"public_key", "status", "is_verified", "last_verified_at", "verification_url",
	"capabilities", "trust_score", "registered_by_agent", "created_by", "created_at", "updated_at",
	"verification_method", "attestation_count", "confidence_score", "last_attested_at",
	"created_by_name", "created_by_email", "created_by_sdk_token_id", "created_by_api_key_id",
	"updated_by", "updated_by_name", "updated_by_email",
}

func toolServerRowValues() []driver.Value {
	now := time.Now().UTC()
	return []driver.Value{
		uuid.NewString(), uuid.NewString(), "files", nil, "https://files.example.com", nil,
		nil, "pending", false, nil, nil,
		[]byte("[]"), 0.5, nil, uuid.NewString(), now, now,
		"", int64(0), 0.0, nil,
		"", "", nil, nil,
		nil, "", "",
	}
}

type statementRows struct {
	values [][]driver.Value
	next   int
}

func (r *statementRows) Columns() []string { return toolServerColumns }
func (r *statementRows) Close() error      { return nil }
func (r *statementRows) Next(dest []driver.Value) error {
	if r.next >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.next])
	r.next++
	return nil
}

// newStatementStoreService returns an MCPService whose repository talks to a
// statementStore.
func newStatementStoreService(t *testing.T) (*MCPService, *statementStore) {
	t.Helper()
	store := &statementStore{}
	db := sql.OpenDB(store)
	t.Cleanup(func() { _ = db.Close() })
	return &MCPService{
		mcpRepo:       repository.NewMCPServerRepository(db),
		cryptoService: infracrypto.NewED25519Service(),
		challenges:    make(map[string]ChallengeData),
	}, store
}

func TestToolServerKeyRefusedForItsTypeOrByTheKeyHelperReachesTheStoreAsNoWriteAndAnAcceptedKeyAsOneInsert(t *testing.T) {
	check := func(t *testing.T, refused *AddPublicKeyRequest, wantErr error) {
		svc, store := newStatementStoreService(t)
		serverID := uuid.New()

		err := svc.AddPublicKey(context.Background(), serverID, refused)

		require.ErrorIs(t, err, wantErr)
		assert.Empty(t, store.writes(), "the store received no insert and no update")

		// Control: an accepted key on the same store is exactly one insert.
		genuine := newEd25519PublicKey(t)
		require.NoError(t, svc.AddPublicKey(context.Background(), serverID, &AddPublicKeyRequest{PublicKey: genuine, KeyType: "ed25519"}))
		writes := store.writes()
		require.Len(t, writes, 1, "the store received exactly the insert")
		assert.True(t, strings.HasPrefix(writes[0].query, "INSERT INTO mcp_server_keys"), "statement: %s", writes[0].query)
		require.Len(t, writes[0].args, 5)
		assert.Equal(t, serverID.String(), writes[0].args[1])
		assert.Equal(t, genuine, writes[0].args[2], "the stored key is the accepted one")
		assert.Equal(t, "ed25519", writes[0].args[3])
	}

	genuine := newEd25519PublicKey(t)
	for name, keyType := range map[string]string{"empty type": "", "rsa": "rsa", "ecdsa": "ecdsa"} {
		keyType := keyType
		t.Run(name, func(t *testing.T) {
			check(t, &AddPublicKeyRequest{PublicKey: genuine, KeyType: keyType}, ErrPublicKeyTypeNotSupported)
		})
	}
	eachRefusedKey(t, func(t *testing.T, encoded, keyType string) {
		check(t, &AddPublicKeyRequest{PublicKey: encoded, KeyType: keyType}, ErrPublicKeyNotAccepted)
	})
	t.Run("not base64", func(t *testing.T) {
		check(t, &AddPublicKeyRequest{PublicKey: "not base64!", KeyType: "ed25519"}, ErrPublicKeyNotAccepted)
	})
	t.Run("31 bytes", func(t *testing.T) {
		raw, err := base64.StdEncoding.DecodeString(genuine)
		require.NoError(t, err)
		check(t, &AddPublicKeyRequest{PublicKey: base64.StdEncoding.EncodeToString(raw[:31]), KeyType: "ed25519"}, ErrPublicKeyNotAccepted)
	})
}
