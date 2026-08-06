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
// `revocations` is what ATP-SPEC v1.0.0-rc1 §8.1 specifies — a document that is not in
// this repository, so the pointer is given in full:
// https://github.com/opena2a-standards/agent-trust-protocol (ATP-SPEC.md §8.1) with its
// schema at https://specs.opena2a.org/schemas/atp/revocation-list-v1.schema.json.
// Both SDK CRL types decode `entries`
// instead, so a fetcher wired naively against this body does not get a usable list.
//
// Serving both keys is the tempting tidy-up and it must stay a deliberate act with a
// failing test attached, because the SDK side has already been caught failing OPEN on a
// shape it could not read: the Java cache accepted a decoded `Crl(entries=null)` as
// valid, cached it FRESH, and the verifier read null entries as "nothing is revoked".
// Both SDKs are fixed now, but the ordering rule this test enforces is that neither end
// moves until the other is known to fail closed.
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

	revokedID, _ := seedAgentForRevocationList(t, db, ctx, "revoked")

	status, body := fetchRevocationList(t, db)
	require.Equal(t, fiber.StatusOK, status)

	ids := revokedAgentIDs(t, body)

	// Control. Without it this test asserts 0 == 0 and passes against an endpoint that
	// returns an empty list to everyone — which is the defect this file exists for. It
	// did exactly that: it was the one test in this file that stayed green on the
	// unfixed code.
	require.Contains(t, ids, revokedID.String(),
		"control: the seeded revoked agent must be listed, or the count below is vacuous")

	total, ok := body["total"].(float64)
	require.True(t, ok, "response carried no numeric total: %v", body)
	assert.Equal(t, len(ids), int(total),
		"reported total disagreed with the number of entries returned")
}

// fetchRevocationETag returns the ETag header the handler sets.
func fetchRevocationETag(t *testing.T, db *sql.DB) string {
	t.Helper()

	handler := NewLifecycleHandler(nil, repository.NewAgentRepository(db))
	app := fiber.New()
	app.Get("/api/v1/revocations", handler.GetRevocationList)

	resp, err := app.Test(httptest.NewRequest("GET", "/api/v1/revocations", nil), fiber.TestConfig{Timeout: 0})
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, fiber.StatusOK, resp.StatusCode)

	return resp.Header.Get("ETag")
}

// seedBulkRevokedAgents inserts n revoked agents in one organization and returns the org
// id and the set of agent ids, so a caller can assert on exactly its own rows rather than
// on whatever else the table holds.
func seedBulkRevokedAgents(t *testing.T, db *sql.DB, ctx context.Context, n int) (uuid.UUID, map[string]bool) {
	t.Helper()

	orgID, userID := uuid.New(), uuid.New()
	suffix := orgID.String()[:8]

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM agents WHERE organization_id = $1`, orgID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID)
		_, _ = db.ExecContext(ctx, `DELETE FROM organizations WHERE id = $1`, orgID)
	})

	_, err := db.ExecContext(ctx,
		`INSERT INTO organizations (id, name, domain, created_at, updated_at)
		 VALUES ($1, $2, $3, NOW(), NOW())`,
		orgID, "revbulk-org-"+suffix, "revbulk-"+suffix+".example.com")
	require.NoError(t, err)

	_, err = db.ExecContext(ctx,
		`INSERT INTO users (id, organization_id, email, name, password_hash, role,
		                    provider, provider_id, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, 'x', 'admin', 'local', $5, NOW(), NOW())`,
		userID, orgID, "revbulk-"+suffix+"@example.com", "revbulk-user", "local-"+suffix)
	require.NoError(t, err)

	// created_at is deliberately identical across every row, which makes `created_at DESC`
	// alone a non-total order — the condition the query's `, id` tie-breaker exists for.
	//
	// It does NOT follow that this fixture catches a dropped tie-breaker, and an earlier
	// version of this comment claimed it did. Removing `, id` leaves all of these tests
	// green: PostgreSQL happens to return the same permutation of tied rows across
	// consecutive page queries, so the defect is real but invisible from here. Proving it
	// needs a plan that genuinely reorders between pages, which this fixture does not force.
	//
	// The fixture is still worth its cost — it is what exercises multi-page assembly at all
	// — but do not read a green run as evidence the ordering is safe.
	rows, err := db.QueryContext(ctx,
		`INSERT INTO agents (id, organization_id, name, display_name, description, agent_type,
		                     status, public_key, trust_score, created_by, created_at, updated_at)
		 SELECT gen_random_uuid(), $1, 'revbulk-' || $2 || '-' || g, 'Bulk Agent', NULL,
		        'ai_agent', 'revoked', 'unused', 0.5, $3, NOW(), NOW()
		 FROM generate_series(1, $4) AS g
		 RETURNING id`,
		orgID, suffix, userID, n)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	ids := make(map[string]bool, n)
	for rows.Next() {
		var id uuid.UUID
		require.NoError(t, rows.Scan(&id))
		ids[id.String()] = true
	}
	require.NoError(t, rows.Err())
	require.Len(t, ids, n, "bulk seed did not insert the requested number of agents")

	return orgID, ids
}

// The ETag must identify the revocation SET, and nothing else.
//
// It decides whether a caching verifier refreshes its CRL, so a validator that fails to
// change on a real change tells the verifier a stale list is current. The previous one was
// built from the entry count and a five-minute wall-clock bucket, so it changed when
// nothing had, and collided whenever one agent was revoked and another left the list
// inside the same bucket.
func TestRevocationListETagTracksTheSetAndNotTheClock(t *testing.T) {
	ctx := context.Background()
	db := revocationTestDB(t)

	first, _ := seedAgentForRevocationList(t, db, ctx, "revoked")

	_, body := fetchRevocationList(t, db)
	require.Contains(t, revokedAgentIDs(t, body), first.String(),
		"control: the seeded agent must be listed, or these ETags describe an empty set")

	etagA := fetchRevocationETag(t, db)
	require.NotEmpty(t, etagA, "handler set no ETag")

	// Same content, later wall clock: the validator must not move.
	assert.Equal(t, etagA, fetchRevocationETag(t, db),
		"the ETag changed while the revocation set did not; a caching verifier refetches for nothing")

	// One more revocation: the validator must move.
	second, _ := seedAgentForRevocationList(t, db, ctx, "revoked")
	assert.NotEqual(t, etagA, fetchRevocationETag(t, db),
		"the ETag did not change after %s was revoked; a caching verifier would keep serving a list that omits it", second)
}

// The paging loop must assemble a list that crosses the page boundary.
//
// revocationListPageSize is 500 and every other test in this file runs against a handful
// of rows, so the loop executes exactly once and breaks — the offset arithmetic, the break
// condition and multi-page assembly are otherwise never exercised.
func TestRevocationListAssemblesAcrossPageBoundaries(t *testing.T) {
	ctx := context.Background()
	db := revocationTestDB(t)

	want := revocationListPageSize + 37
	_, seeded := seedBulkRevokedAgents(t, db, ctx, want)

	status, body := fetchRevocationList(t, db)
	require.Equal(t, fiber.StatusOK, status)

	returned := revokedAgentIDs(t, body)

	seen := make(map[string]bool, len(returned))
	found := 0
	for _, id := range returned {
		assert.False(t, seen[id], "agent %s appeared twice; the offset walk double-counted", id)
		seen[id] = true
		if seeded[id] {
			found++
		}
	}

	assert.Equal(t, want, found,
		"the revocation list dropped %d of the %d seeded revoked agents across the page boundary",
		want-found, want)
}
