//go:build integration

package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
)

// The federated revocation list (GET /api/v1/revocations).
//
// This endpoint is the post-issuance half of ATX revocation: `atx.revoked` is set at
// issuance and never updated afterwards, so a verifier learns that an already-issued
// credential was revoked only by reading this list. atx-spec core.md §3.3 puts the
// propagation target at under 60 seconds, hard upper bound 5 minutes.
//
// Before this suite existed the handler called `agentRepo.List(0, 0)`, which reaches
// `SELECT ... FROM agents ORDER BY created_at DESC LIMIT $1 OFFSET $2` with limit 0.
// PostgreSQL `LIMIT 0` returns no rows — it does not mean "unbounded" — so the endpoint
// answered 200 with an empty list no matter how many agents were revoked. A caching
// verifier treats that as a successful fetch and holds it as FRESH, so revocation was
// skipped with no error surfaced anywhere: no 4xx, no 5xx, no log line.
//
// These tests drive the REAL repository against a real Postgres and the REAL handler.
// A mocked repository cannot catch this defect — the bug is in the SQL the mock replaces.
//
// Build-tag gated: requires Postgres reachable via TEST_DATABASE_URL with the AIM schema
// applied.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestRevocationList ./internal/interfaces/http/handlers/...

func revocationTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping revocation list integration test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping())
	return db
}

// seedAgentForRevocationList inserts one agent in `status` and returns its id and name.
// Rows are removed child-first on cleanup so the foreign keys hold on the way out.
func seedAgentForRevocationList(t *testing.T, db *sql.DB, ctx context.Context, status string) (uuid.UUID, string) {
	t.Helper()

	orgID, userID, agentID := uuid.New(), uuid.New(), uuid.New()
	suffix := agentID.String()[:8]
	agentName := "revlist-agent-" + suffix

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM agents WHERE id = $1`, agentID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID)
		_, _ = db.ExecContext(ctx, `DELETE FROM organizations WHERE id = $1`, orgID)
	})

	_, err := db.ExecContext(ctx,
		`INSERT INTO organizations (id, name, domain, created_at, updated_at)
		 VALUES ($1, $2, $3, NOW(), NOW())`,
		orgID, "revlist-org-"+suffix, "revlist-"+suffix+".example.com")
	require.NoError(t, err)

	_, err = db.ExecContext(ctx,
		`INSERT INTO users (id, organization_id, email, name, password_hash, role,
		                    provider, provider_id, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, 'x', 'admin', 'local', $5, NOW(), NOW())`,
		userID, orgID, "revlist-"+suffix+"@example.com", "revlist-user", "local-"+suffix)
	require.NoError(t, err)

	// `description` is nullable with no default but is scanned into a plain string,
	// so a NULL there fails the row read rather than the assertion under test.
	_, err = db.ExecContext(ctx,
		`INSERT INTO agents (id, organization_id, name, display_name, description, agent_type,
		                     status, public_key, trust_score, created_by, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, 'ai_agent', $6, 'unused-for-this-endpoint', 0.5, $7, NOW(), NOW())`,
		agentID, orgID, agentName, "Revocation List Agent",
		"seeded by the revocation list suite", status, userID)
	require.NoError(t, err)

	return agentID, agentName
}

// fetchRevocationList runs the real handler through a real fiber app and returns the
// decoded body. No middleware is mounted: the endpoint is unauthenticated by design and
// this suite is about what the list CONTAINS.
func fetchRevocationList(t *testing.T, db *sql.DB) (int, map[string]any) {
	t.Helper()

	// agentService is not reached by GetRevocationList; it reads agentRepo only.
	handler := NewLifecycleHandler(nil, repository.NewAgentRepository(db))

	app := fiber.New()
	app.Get("/api/v1/revocations", handler.GetRevocationList)

	resp, err := app.Test(httptest.NewRequest("GET", "/api/v1/revocations", nil), fiber.TestConfig{Timeout: 0})
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var body map[string]any
	require.NoError(t, json.Unmarshal(raw, &body), "response was not JSON: %s", string(raw))
	return resp.StatusCode, body
}

// revocationEntries returns the entry objects under `key`.
func revocationEntries(t *testing.T, body map[string]any, key string) []map[string]any {
	t.Helper()

	raw, ok := body[key]
	require.True(t, ok, "response carried no %q key: %v", key, body)

	list, ok := raw.([]any)
	require.True(t, ok, "%q was not a list: %v", key, raw)

	entries := make([]map[string]any, 0, len(list))
	for _, e := range list {
		entry, ok := e.(map[string]any)
		require.True(t, ok, "entry was not an object: %v", e)
		entries = append(entries, entry)
	}
	return entries
}

// revokedAgentIDs pulls the agent ids out of the `revocations` key.
func revokedAgentIDs(t *testing.T, body map[string]any) []string {
	t.Helper()

	entries := revocationEntries(t, body, "revocations")
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		id, ok := entry["agentId"].(string)
		require.True(t, ok, "entry carried no agentId: %v", entry)
		ids = append(ids, id)
	}
	return ids
}

// A revoked agent MUST appear in the list. This is the assertion the empty-list defect
// failed: pre-fix the endpoint answered 200 with zero entries regardless of the table.
func TestRevocationListIncludesRevokedAgent(t *testing.T) {
	ctx := context.Background()
	db := revocationTestDB(t)

	revokedID, _ := seedAgentForRevocationList(t, db, ctx, "revoked")

	status, body := fetchRevocationList(t, db)
	require.Equal(t, fiber.StatusOK, status)

	assert.Contains(t, revokedAgentIDs(t, body), revokedID.String(),
		"a revoked agent was absent from the revocation list — a verifier reading this "+
			"list would keep accepting its credential until expiry")
}

// An agent that is NOT revoked must be absent. Without this, a fix that returns every
// agent row would pass the test above while telling verifiers to reject working agents.
func TestRevocationListExcludesActiveAgent(t *testing.T) {
	ctx := context.Background()
	db := revocationTestDB(t)

	activeID, _ := seedAgentForRevocationList(t, db, ctx, "active")
	revokedID, _ := seedAgentForRevocationList(t, db, ctx, "revoked")

	status, body := fetchRevocationList(t, db)
	require.Equal(t, fiber.StatusOK, status)

	ids := revokedAgentIDs(t, body)
	assert.Contains(t, ids, revokedID.String(), "control: the revoked agent must be listed")
	assert.NotContains(t, ids, activeID.String(),
		"an active agent appeared in the revocation list — verifiers would reject a valid credential")
}

// The list must carry the agent id and a reason code, and nothing else.
//
// `name` is an organization-internal namespace with no opt-in mechanism, and this
// endpoint is unauthenticated and spans every organization. `revokedAt` was sourced
// from `agents.updated_at`, which any later write to the row rewrites. Both were
// present and both were inert only because the list was always empty — so this asserts
// the absence directly rather than trusting that nothing puts them back.
func TestRevocationListCarriesNoAgentNameOrTimestamp(t *testing.T) {
	ctx := context.Background()
	db := revocationTestDB(t)

	_, agentName := seedAgentForRevocationList(t, db, ctx, "revoked")

	status, body := fetchRevocationList(t, db)
	require.Equal(t, fiber.StatusOK, status)

	entries := revocationEntries(t, body, "revocations")
	require.NotEmpty(t, entries, "control: the revoked agent must be listed for this to assert anything")

	for _, entry := range entries {
		assert.NotContains(t, entry, "name",
			"the revocation list exposed an agent name on an unauthenticated cross-organization endpoint")
		assert.NotContains(t, entry, "revokedAt",
			"the revocation list exposed a revocation time it cannot stand behind")

		keys := make([]string, 0, len(entry))
		for k := range entry {
			keys = append(keys, k)
		}
		assert.ElementsMatch(t, []string{"agentId", "reason"}, keys,
			"the revocation list carried a field with no consumer; add one only with a reader")
	}

	// The name must not have reached the response through any other field either.
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), agentName,
		"the seeded agent name appeared somewhere in the revocation response")
}

// The response must NOT carry an `entries` key, and this is deliberate.
//
// Both SDK CRL types decode `entries`, so a fetcher pointed straight at this body fails
// loudly today — the TypeScript cache throws rather than caching. That mismatch is doing
// real work: it is the only thing stopping a client from accepting this list as fresh and
// authoritative. Serving `entries` as well would make a naive fetcher succeed quietly,
// which is the exact silent fail-open the empty-list defect produced.
//
// Aligning server and SDK is correct eventually, but not as a standalone rename on either
// side, and not before a client can act on what it receives. This test is here so that
// adding the alias is a deliberate act with a failing test attached, rather than a tidy-up.
func TestRevocationListDoesNotServeTheSdkKey(t *testing.T) {
	ctx := context.Background()
	db := revocationTestDB(t)

	revokedID, _ := seedAgentForRevocationList(t, db, ctx, "revoked")

	status, body := fetchRevocationList(t, db)
	require.Equal(t, fiber.StatusOK, status)

	require.Contains(t, revokedAgentIDs(t, body), revokedID.String(),
		"control: the revoked agent must be listed for this to assert anything")
	assert.NotContains(t, body, "entries",
		"the response gained the SDK's key, which removes the mismatch that currently "+
			"stops a naive fetcher from silently accepting this list")
}

// A revoked agent with no description must still reach the list.
//
// `agents.description` is nullable, and AgentRepository.List scanned it straight into a
// string, which fails the whole row. That defect was unreachable while the limit bug
// held — no row was ever scanned — so fixing the limit alone converted the empty list
// into a 500 for any deployment holding one agent with a NULL description. The two
// defects had to be fixed together, and this is the test that says so.
//
// The other seeds in this file all set `description`, which is exactly why they did not
// catch it.
func TestRevocationListIncludesAgentWithNullDescription(t *testing.T) {
	ctx := context.Background()
	db := revocationTestDB(t)

	revokedID, _ := seedAgentForRevocationList(t, db, ctx, "revoked")
	_, err := db.ExecContext(ctx, `UPDATE agents SET description = NULL WHERE id = $1`, revokedID)
	require.NoError(t, err)

	status, body := fetchRevocationList(t, db)
	require.Equal(t, fiber.StatusOK, status,
		"a NULL description failed the row scan and took the whole revocation list with it")

	assert.Contains(t, revokedAgentIDs(t, body), revokedID.String())
}

// The count the endpoint reports must match the entries it actually returns. A caller
// that trusts `total` and ignores the array (or vice versa) must not see two answers.
func TestRevocationListTotalMatchesEntries(t *testing.T) {
	ctx := context.Background()
	db := revocationTestDB(t)

	seedAgentForRevocationList(t, db, ctx, "revoked")

	status, body := fetchRevocationList(t, db)
	require.Equal(t, fiber.StatusOK, status)

	total, ok := body["total"].(float64)
	require.True(t, ok, "response carried no numeric total: %v", body)
	assert.Equal(t, len(revokedAgentIDs(t, body)), int(total),
		"reported total disagreed with the number of entries returned")
}
