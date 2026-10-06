//go:build integration

package handlers

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record"
	recordstore "github.com/opena2a-org/agent-identity-management/apps/backend/internal/record/store"
)

// GET /api/v1/admin/audit-logs/chain/head reports an organization's chain
// state through the record store's one chain-state read.
//
// Build-tag gated: requires Postgres reachable via TEST_DATABASE_URL with the
// AIM schema applied.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestAuditChainHead ./internal/interfaces/http/handlers/...

func auditChainTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping audit chain head integration test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping())
	return db
}

// seedAuditChainOrg inserts one organization and removes it and its chain on
// cleanup.
func seedAuditChainOrg(t *testing.T, db *sql.DB) string {
	t.Helper()
	id := uuid.NewString()
	suffix := id[:8]
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM audit_records WHERE chain_id IN (SELECT id FROM record_chains WHERE organization_id = $1)`, id)
		_, _ = db.Exec(`DELETE FROM record_chains WHERE organization_id = $1`, id)
		_, _ = db.Exec(`DELETE FROM organizations WHERE id = $1`, id)
	})
	_, err := db.Exec(
		`INSERT INTO organizations (id, name, domain, created_at, updated_at)
		 VALUES ($1, $2, $3, NOW(), NOW())`,
		id, "auditchain-org-"+suffix, "auditchain-"+suffix+".example.com")
	require.NoError(t, err)
	return id
}

type auditChainTestKey struct {
	public  record.PublicKey
	private ed25519.PrivateKey
}

func (k auditChainTestKey) SignPayload(_ context.Context, class record.PayloadClass, payload record.Payload) (string, []byte, error) {
	message, err := record.SigningInput(class, payload)
	if err != nil {
		return "", nil, err
	}
	return k.public.KeyID(), ed25519.Sign(k.private, message), nil
}

func (k auditChainTestKey) PublicKey(context.Context) (record.PublicKey, error) {
	return k.public, nil
}

// plantChain writes a signed genesis for the organization with the same two
// rows the record store's writer writes. The writer itself refuses to start
// a chain on the schema as shipped, whose foreign keys still remove audit
// rows by cascade.
func plantChain(t *testing.T, db *sql.DB, organizationID string) (chainID string, head record.Head) {
	t.Helper()
	ctx := context.Background()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	key := auditChainTestKey{public: record.PublicKey{Alg: record.AlgEd25519, Key: public}, private: private}

	chainID, eventID := uuid.NewString(), uuid.NewString()
	timestamp := time.Now().UTC().Truncate(time.Microsecond)
	body, err := record.NewGenesis(record.Genesis{
		ChainID:  chainID,
		FirstKey: key.public,
		Draft:    record.Draft{EventID: eventID, Timestamp: timestamp},
	})
	require.NoError(t, err)
	rec, err := record.Sign(ctx, body, key)
	require.NoError(t, err)
	signature, err := base64.StdEncoding.DecodeString(rec.Envelope.Signatures[0].Sig)
	require.NoError(t, err)
	head = body.Head()

	_, err = db.ExecContext(ctx, `
INSERT INTO record_chains (id, organization_id, key_id, head_seq, head_hash)
VALUES ($1, $2, $3, $4, $5)`, chainID, organizationID, key.public.KeyID(), head.Seq, head.Hash)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `
INSERT INTO audit_records (chain_id, seq, event_id, record_type, recorded_at, record_hash,
                           payload_type, payload, key_id, signature)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		chainID, head.Seq, eventID, record.TypeChainGenesis, timestamp,
		head.Hash, rec.Envelope.PayloadType, body.Canonical(), rec.Envelope.Signatures[0].KeyID, signature)
	require.NoError(t, err)
	return chainID, head
}

// getChainHead calls the route as an admin of organizationID and returns
// the status and each top-level member of the body as raw JSON.
func getChainHead(t *testing.T, h *AuditChainHandler, organizationID string) (int, map[string]json.RawMessage) {
	t.Helper()
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.MustParse(organizationID))
		return c.Next()
	})
	app.Get("/api/v1/admin/audit-logs/chain/head", h.GetChainHead)
	resp, err := app.Test(httptest.NewRequest("GET", "/api/v1/admin/audit-logs/chain/head", nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var body map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &body), string(raw))
	return resp.StatusCode, body
}

// A cell plants notStarted, extendable and notExtendable, each in its own
// organization, and reads the same chainState from the route and from the
// record store's chain-state read.
func TestAuditChainHeadReportsTheChainStateReadChainStateReads(t *testing.T) {
	db := auditChainTestDB(t)
	ctx := context.Background()
	h := NewAuditChainHandler(db)

	notStarted := seedAuditChainOrg(t, db)
	extendable := seedAuditChainOrg(t, db)
	extendableChain, extendableHead := plantChain(t, db, extendable)
	notExtendable := seedAuditChainOrg(t, db)
	brokenChain, _ := plantChain(t, db, notExtendable)
	_, err := db.Exec(`UPDATE record_chains SET head_hash = repeat('0', 64) WHERE organization_id = $1`, notExtendable)
	require.NoError(t, err)

	for _, tc := range []struct {
		org  string
		want record.ChainState
	}{
		{notStarted, record.ChainNotStarted},
		{extendable, record.ChainExtendable},
		{notExtendable, record.ChainNotExtendable},
	} {
		t.Run(string(tc.want), func(t *testing.T) {
			status, err := recordstore.ReadChainState(ctx, db, tc.org)
			require.NoError(t, err)
			require.Equal(t, tc.want, status.State)

			code, body := getChainHead(t, h, tc.org)
			require.Equal(t, fiber.StatusOK, code)
			var state record.ChainState
			require.NoError(t, json.Unmarshal(body["chainState"], &state))
			require.Equal(t, status.State, state, "the route and the chain-state read agree")
			require.JSONEq(t, `null`, string(body["latestCheckpoint"]))
		})
	}

	t.Run("notStarted answers null chainId, head and latestCheckpoint", func(t *testing.T) {
		_, body := getChainHead(t, h, notStarted)
		for _, member := range []string{"chainId", "head", "latestCheckpoint"} {
			raw, ok := body[member]
			require.True(t, ok, "%s is present", member)
			require.JSONEq(t, `null`, string(raw), member)
		}
		require.NotContains(t, body, "reason")
	})

	t.Run("extendable answers the chain and its head", func(t *testing.T) {
		_, body := getChainHead(t, h, extendable)
		require.JSONEq(t, `"`+extendableChain+`"`, string(body["chainId"]))
		require.JSONEq(t, `{"seq":0,"hash":"`+extendableHead.Hash+`"}`, string(body["head"]))
		require.NotContains(t, body, "reason")
	})

	t.Run("notExtendable answers the stored head and the reason", func(t *testing.T) {
		_, body := getChainHead(t, h, notExtendable)
		require.JSONEq(t, `"`+brokenChain+`"`, string(body["chainId"]))
		require.JSONEq(t, `{"seq":0,"hash":"`+"0000000000000000000000000000000000000000000000000000000000000000"+`"}`, string(body["head"]))
		require.JSONEq(t, `"head_mismatch"`, string(body["reason"]))
	})
}
