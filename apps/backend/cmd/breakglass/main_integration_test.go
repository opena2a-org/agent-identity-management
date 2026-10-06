//go:build integration

package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/handlers"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record"
	recordstore "github.com/opena2a-org/agent-identity-management/apps/backend/internal/record/store"
)

// aim-breakglass chain status and GET /api/v1/admin/audit-logs/chain/head
// both report a chain through the record store's one chain-state read.
//
// Build-tag gated: requires Postgres reachable via TEST_DATABASE_URL with the
// AIM schema applied.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration ./cmd/breakglass/...

func testDatabase(t *testing.T) (*sql.DB, string) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping aim-breakglass integration test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping())
	return db, dsn
}

// seedOrganization inserts one organization and removes it and its chain on
// cleanup.
func seedOrganization(t *testing.T, db *sql.DB) string {
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
		id, "breakglass-org-"+suffix, "breakglass-"+suffix+".example.com")
	require.NoError(t, err)
	return id
}

type testKey struct {
	public  record.PublicKey
	private ed25519.PrivateKey
}

func (k testKey) SignPayload(_ context.Context, class record.PayloadClass, payload record.Payload) (string, []byte, error) {
	message, err := record.SigningInput(class, payload)
	if err != nil {
		return "", nil, err
	}
	return k.public.KeyID(), ed25519.Sign(k.private, message), nil
}

func (k testKey) PublicKey(context.Context) (record.PublicKey, error) {
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
	key := testKey{public: record.PublicKey{Alg: record.AlgEd25519, Key: public}, private: private}

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

// routeBody calls the admin route as an admin of organizationID and returns
// its body.
func routeBody(t *testing.T, db *sql.DB, organizationID string) string {
	t.Helper()
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.MustParse(organizationID))
		return c.Next()
	})
	app.Get("/api/v1/admin/audit-logs/chain/head", handlers.NewAuditChainHandler(db).GetChainHead)
	resp, err := app.Test(httptest.NewRequest("GET", "/api/v1/admin/audit-logs/chain/head", nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return string(raw)
}

// breakglass runs the command against the test database, configured the way
// aim-migrate is, through DATABASE_URL.
func breakglass(t *testing.T, dsn string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = run(context.Background(), args, envOf(map[string]string{"DATABASE_URL": dsn}), &out, &errOut)
	return code, out.String(), errOut.String()
}

// A cell plants notStarted, extendable and notExtendable, each in its own
// organization, and reads the same chainState from the record store's
// chain-state read, from aim-breakglass chain status and from the admin
// route. The command's --json output is the
// route's body, member for member.
func TestChainStatusReadsTheChainStateTheRouteReads(t *testing.T) {
	db, dsn := testDatabase(t)

	notStarted := seedOrganization(t, db)
	extendable := seedOrganization(t, db)
	extendableChain, extendableHead := plantChain(t, db, extendable)
	notExtendable := seedOrganization(t, db)
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
			status, err := recordstore.ReadChainState(context.Background(), db, tc.org)
			require.NoError(t, err)
			require.Equal(t, tc.want, status.State)

			code, stdout, stderr := breakglass(t, dsn, "chain", "status", "--organization", tc.org, "--json")
			require.Equal(t, exitOK, code, stderr)
			var command struct {
				ChainState record.ChainState `json:"chainState"`
			}
			require.NoError(t, json.Unmarshal([]byte(stdout), &command), stdout)
			require.Equal(t, status.State, command.ChainState, "the command reports the chain-state read")

			route := routeBody(t, db, tc.org)
			var routed struct {
				ChainState record.ChainState `json:"chainState"`
			}
			require.NoError(t, json.Unmarshal([]byte(route), &routed), route)
			require.Equal(t, routed.ChainState, command.ChainState, "the command and the route read the same chainState")
			require.JSONEq(t, route, stdout, "the command's --json output is the route's body")
		})
	}

	t.Run("notStarted prints null chainId, head and latestCheckpoint", func(t *testing.T) {
		_, stdout, _ := breakglass(t, dsn, "chain", "status", "--organization", notStarted, "--json")
		require.JSONEq(t, `{"chainState":"notStarted","chainId":null,"head":null,"latestCheckpoint":null}`, stdout)
	})

	t.Run("extendable prints the chain and its head for a person", func(t *testing.T) {
		code, stdout, stderr := breakglass(t, dsn, "chain", "status", "--organization", extendable)
		require.Equal(t, exitOK, code, stderr)
		require.Contains(t, stdout, "chainState        extendable")
		require.Contains(t, stdout, "chainId           "+extendableChain)
		require.Contains(t, stdout, "head              seq 0, hash "+extendableHead.Hash)
		require.NotContains(t, stdout, "reason")
	})

	t.Run("notExtendable prints the stored head and the reason", func(t *testing.T) {
		code, stdout, stderr := breakglass(t, dsn, "chain", "status", "--organization", notExtendable)
		require.Equal(t, exitOK, code, stderr)
		require.Contains(t, stdout, "chainState        notExtendable")
		require.Contains(t, stdout, "chainId           "+brokenChain)
		require.Contains(t, stdout, "head              seq 0, hash 0000000000000000000000000000000000000000000000000000000000000000")
		require.Contains(t, stdout, "reason            head_mismatch")
	})

	t.Run("an organization id is read in any case the UUID parser accepts", func(t *testing.T) {
		code, stdout, stderr := breakglass(t, dsn, "chain", "status", "--organization", strings.ToUpper(extendable), "--json")
		require.Equal(t, exitOK, code, stderr)
		require.JSONEq(t, routeBody(t, db, extendable), stdout)
	})
}

// An id that names no organization is refused with exit 1, never reported as
// a chain that has not started.
func TestChainStatusRefusesAnUnknownOrganization(t *testing.T) {
	_, dsn := testDatabase(t)
	unknown := uuid.NewString()
	code, stdout, stderr := breakglass(t, dsn, "chain", "status", "--organization", unknown, "--json")
	require.Equal(t, exitFailure, code)
	require.Empty(t, stdout)
	require.Contains(t, stderr, "no organization has id "+unknown)
}
