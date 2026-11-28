package handlers

// POST /api/v1/mcp-servers/:id/keys through the handler and the real
// MCPService, on a store that answers every statement and records it. A key
// type other than Ed25519, or a key the key helper refuses, is answered 400
// with the handler's error, and the store receives no insert and no update; an
// accepted key is answered 201 and reaches the store as exactly one insert.

import (
	"context"
	"crypto/ed25519"
	"database/sql"
	"database/sql/driver"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto/ed25519key"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto/ed25519key/ed25519keytest"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
)

// keyStatement is one statement the store received, with its arguments.
type keyStatement struct {
	query string
	args  []driver.Value
}

// keyStatementStore is a database/sql driver that accepts every statement,
// records it, and answers a read of mcp_servers with one tool server of orgID.
type keyStatementStore struct {
	orgID      uuid.UUID
	mu         sync.Mutex
	statements []keyStatement
}

func (s *keyStatementStore) record(query string, args []driver.NamedValue) {
	s.mu.Lock()
	defer s.mu.Unlock()
	values := make([]driver.Value, len(args))
	for i, a := range args {
		values[i] = a.Value
	}
	s.statements = append(s.statements, keyStatement{query: strings.TrimSpace(query), args: values})
}

// writes returns the INSERT and UPDATE statements received so far.
func (s *keyStatementStore) writes() []keyStatement {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []keyStatement
	for _, st := range s.statements {
		verb := strings.ToUpper(strings.Fields(st.query + " x")[0])
		if verb == "INSERT" || verb == "UPDATE" {
			out = append(out, st)
		}
	}
	return out
}

func (s *keyStatementStore) Connect(context.Context) (driver.Conn, error) {
	return &keyStatementConn{s}, nil
}
func (s *keyStatementStore) Driver() driver.Driver { return keyStatementDriver{s} }

type keyStatementDriver struct{ store *keyStatementStore }

func (d keyStatementDriver) Open(string) (driver.Conn, error) { return &keyStatementConn{d.store}, nil }

type keyStatementConn struct{ store *keyStatementStore }

func (c *keyStatementConn) Prepare(query string) (driver.Stmt, error) {
	return &keyStatementStmt{conn: c, query: query}, nil
}
func (c *keyStatementConn) Close() error              { return nil }
func (c *keyStatementConn) Begin() (driver.Tx, error) { return keyStatementTx{}, nil }

func (c *keyStatementConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	c.store.record(query, args)
	return driver.RowsAffected(1), nil
}

func (c *keyStatementConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	c.store.record(query, args)
	if strings.Contains(query, "FROM mcp_servers") && len(args) == 1 {
		now := time.Now().UTC()
		return &keyStatementRows{values: [][]driver.Value{{
			args[0].Value, c.store.orgID.String(), "files", nil, "https://files.example.com", nil,
			nil, "pending", false, nil, nil,
			[]byte("[]"), 0.5, nil, uuid.NewString(), now, now,
			"", int64(0), 0.0, nil,
			"", "", nil, nil,
			nil, "", "",
		}}}, nil
	}
	return &keyStatementRows{}, nil
}

type keyStatementStmt struct {
	conn  *keyStatementConn
	query string
}

func (s *keyStatementStmt) Close() error  { return nil }
func (s *keyStatementStmt) NumInput() int { return -1 }
func (s *keyStatementStmt) Exec(args []driver.Value) (driver.Result, error) {
	return s.conn.ExecContext(context.Background(), s.query, keyStatementArgs(args))
}
func (s *keyStatementStmt) Query(args []driver.Value) (driver.Rows, error) {
	return s.conn.QueryContext(context.Background(), s.query, keyStatementArgs(args))
}

func keyStatementArgs(args []driver.Value) []driver.NamedValue {
	out := make([]driver.NamedValue, len(args))
	for i, v := range args {
		out[i] = driver.NamedValue{Ordinal: i + 1, Value: v}
	}
	return out
}

type keyStatementTx struct{}

func (keyStatementTx) Commit() error   { return nil }
func (keyStatementTx) Rollback() error { return nil }

type keyStatementRows struct {
	values [][]driver.Value
	next   int
}

func (r *keyStatementRows) Columns() []string { return mcpServerRowColumns }
func (r *keyStatementRows) Close() error      { return nil }
func (r *keyStatementRows) Next(dest []driver.Value) error {
	if r.next >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.next])
	r.next++
	return nil
}

// postToolServerKey posts body to the add-key route of serverID as a signed-in
// user of the store's organization, and returns the status and the answer.
func postToolServerKey(t *testing.T, store *keyStatementStore, serverID uuid.UUID, body map[string]string) (int, map[string]any) {
	t.Helper()
	db := sql.OpenDB(store)
	t.Cleanup(func() { _ = db.Close() })
	mcpService := application.NewMCPService(repository.NewMCPServerRepository(db), nil, nil, nil, nil, nil, nil, nil, nil, nil)
	handler := NewMCPHandler(mcpService, nil, application.NewAuditService(listOffsetAuditRepo{}), nil, nil, nil, nil)
	app := newTestApp()
	app.Use(func(c fiber.Ctx) error {
		c.Locals("organization_id", store.orgID)
		c.Locals("user_id", uuid.New())
		return c.Next()
	})
	app.Post("/api/v1/mcp-servers/:id/keys", handler.AddPublicKey)

	raw, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest("POST", "/api/v1/mcp-servers/"+serverID.String()+"/keys", strings.NewReader(string(raw)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var answer map[string]any
	require.NoError(t, json.Unmarshal(out, &answer), "the answer is JSON: %s", out)
	return resp.StatusCode, answer
}

func TestMCPHandler_AddPublicKey_ARefusedKeyIs400WithTheHandlersErrorAndNothingIsStored(t *testing.T) {
	genuine, _, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	enc := base64.StdEncoding.EncodeToString
	keyError := map[string]any{"error": "publicKey must be a base64-encoded 32-byte Ed25519 public key"}
	typeError := map[string]any{"error": `keyType must be "ed25519"`}

	type refusal struct {
		body   map[string]string
		answer map[string]any
	}
	cases := map[string]refusal{
		"keyType rsa":     {map[string]string{"publicKey": enc(genuine), "keyType": "rsa"}, typeError},
		"keyType ecdsa":   {map[string]string{"publicKey": enc(genuine), "keyType": "ecdsa"}, typeError},
		"keyType omitted": {map[string]string{"publicKey": enc(genuine)}, typeError},
		"off the curve":   {map[string]string{"publicKey": enc(ed25519keytest.OffCurve()), "keyType": "ed25519"}, keyError},
		"not base64":      {map[string]string{"publicKey": "not base64!", "keyType": "ed25519"}, keyError},
	}
	for i, small := range ed25519key.SmallOrderEncodings() {
		cases["small order "+string(rune('a'+i))] = refusal{map[string]string{"publicKey": enc(small), "keyType": "ed25519"}, keyError}
	}
	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			store := &keyStatementStore{orgID: uuid.New()}
			serverID := uuid.New()

			status, answer := postToolServerKey(t, store, serverID, tc.body)

			assert.Equal(t, fiber.StatusBadRequest, status, "answer: %v", answer)
			assert.Equal(t, tc.answer, answer)
			assert.Empty(t, store.writes(), "the store received no insert and no update")

			// Control: an accepted key on the same store is answered 201 and
			// stored as exactly one insert.
			pub, _, err := ed25519.GenerateKey(nil)
			require.NoError(t, err)
			status, answer = postToolServerKey(t, store, serverID, map[string]string{"publicKey": enc(pub), "keyType": "ed25519"})
			assert.Equal(t, fiber.StatusCreated, status, "answer: %v", answer)
			writes := store.writes()
			require.Len(t, writes, 1, "the store received exactly the insert")
			assert.True(t, strings.HasPrefix(writes[0].query, "INSERT INTO mcp_server_keys"), "statement: %s", writes[0].query)
			require.Len(t, writes[0].args, 5)
			assert.Equal(t, enc(pub), writes[0].args[2], "the stored key is the accepted one")
		})
	}
}
