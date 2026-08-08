//go:build integration

package repository

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A2AConsentRepository.ListAll and A2ATrustScoreRepository.ListAll, and the
// organization boundary they did not have.
//
// Both queries ran with no organization predicate whatsoever. GET
// /api/v1/a2a/consents therefore returned every tenant's consent records —
// user_id, purpose, data_types, both agent IDs, user_agent — to any
// authenticated caller, and GET /api/v1/a2a/trust-scores did the same for
// every tenant's agent IDs and behavioural metrics. Neither route carried a
// role middleware, unlike its neighbours on the same group.
//
// These tests run against a real database rather than a mock, because the
// defect lived entirely in SQL that a mock replaces. Each asserts three
// things, and all three are needed:
//
//   - presence: the caller's own rows ARE returned, so the test cannot pass
//     by returning nothing;
//   - absence: the other tenant's rows are NOT returned, which is the actual
//     security property;
//   - the reported total is the caller's count, not the global count. An
//     unscoped COUNT leaks how much data every other tenant holds without
//     returning a single row, so scoping the rows alone is not sufficient.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestA2AListAllTenantScope ./internal/infrastructure/repository/...

func a2aScopeTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping A2A ListAll tenant scope test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping())
	return db
}

// a2aScopeFixture stands up two organizations, one agent in each, and returns
// their IDs. Everything it creates is removed on cleanup.
func a2aScopeFixture(t *testing.T, db *sql.DB) (orgA, orgB, agentA, agentB uuid.UUID) {
	t.Helper()
	ctx := context.Background()

	orgA, orgB = uuid.New(), uuid.New()
	agentA, agentB = uuid.New(), uuid.New()
	suffix := uuid.NewString()[:8]

	for _, o := range []struct {
		id   uuid.UUID
		name string
	}{{orgA, "scope-test-a-" + suffix}, {orgB, "scope-test-b-" + suffix}} {
		_, err := db.ExecContext(ctx,
			`INSERT INTO organizations (id, name, domain) VALUES ($1, $2, $3)`,
			o.id, o.name, o.name+".invalid")
		require.NoError(t, err)
	}

	// agents.created_by is a real FK to users, so each org needs a user.
	userA, userB := uuid.New(), uuid.New()
	for _, u := range []struct {
		id  uuid.UUID
		org uuid.UUID
	}{{userA, orgA}, {userB, orgB}} {
		_, err := db.ExecContext(ctx,
			`INSERT INTO users (id, organization_id, email, name, provider, provider_id)
			 VALUES ($1, $2, $3, 'scope test user', 'local', $4)`,
			u.id, u.org, "scope-"+u.id.String()[:8]+"@example.invalid", u.id.String())
		require.NoError(t, err)
	}

	for _, a := range []struct {
		id      uuid.UUID
		org     uuid.UUID
		creator uuid.UUID
	}{{agentA, orgA, userA}, {agentB, orgB, userB}} {
		_, err := db.ExecContext(ctx,
			`INSERT INTO agents (id, name, display_name, agent_type, created_by, organization_id)
			 VALUES ($1, $2, $2, 'service', $3, $4)`,
			a.id, "scope-agent-"+a.id.String()[:8], a.creator, a.org)
		require.NoError(t, err)
	}

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM a2a_trust_scores WHERE agent_id = ANY($1)`,
			pqUUIDArray(agentA, agentB))
		_, _ = db.ExecContext(ctx, `DELETE FROM a2a_consent_records WHERE organization_id = ANY($1)`,
			pqUUIDArray(orgA, orgB))
		_, _ = db.ExecContext(ctx, `DELETE FROM agents WHERE id = ANY($1)`,
			pqUUIDArray(agentA, agentB))
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = ANY($1)`,
			pqUUIDArray(userA, userB))
		_, _ = db.ExecContext(ctx, `DELETE FROM organizations WHERE id = ANY($1)`,
			pqUUIDArray(orgA, orgB))
	})

	return orgA, orgB, agentA, agentB
}

func pqUUIDArray(ids ...uuid.UUID) interface{} {
	out := "{"
	for i, id := range ids {
		if i > 0 {
			out += ","
		}
		out += id.String()
	}
	return out + "}"
}

func TestA2AListAllTenantScope_Consents(t *testing.T) {
	db := a2aScopeTestDB(t)
	ctx := context.Background()
	orgA, orgB, agentA, agentB := a2aScopeFixture(t, db)

	// Two consent records for org A, three for org B. The asymmetry matters:
	// equal counts would let a wrong-but-plausible implementation (say, one
	// that scoped the rows but not the COUNT) pass the total assertion by
	// coincidence.
	insert := func(org, grantor, recipient uuid.UUID, userID string) {
		_, err := db.ExecContext(ctx,
			`INSERT INTO a2a_consent_records
			   (user_id, organization_id, grantor_agent_id, recipient_agent_id,
			    scope, purpose, data_types, consent_method)
			 VALUES ($1, $2, $3, $4, '["pii"]'::jsonb, 'test', '["email"]'::jsonb, 'api')`,
			userID, org, grantor, recipient)
		require.NoError(t, err)
	}
	insert(orgA, agentA, agentA, "user-a-1")
	insert(orgA, agentA, agentA, "user-a-2")
	insert(orgB, agentB, agentB, "user-b-1")
	insert(orgB, agentB, agentB, "user-b-2")
	insert(orgB, agentB, agentB, "user-b-3")

	repo := NewA2AConsentRepository(db)

	rowsA, totalA, err := repo.ListAll(ctx, orgA, 100, 0)
	require.NoError(t, err)

	assert.Len(t, rowsA, 2, "org A must see exactly its own two consent records")
	assert.Equal(t, 2, totalA,
		"total must be org A's count, not the global count; an unscoped COUNT "+
			"discloses how much consent data other tenants hold")
	for _, r := range rowsA {
		require.NotNil(t, r.OrganizationID)
		assert.Equal(t, orgA, *r.OrganizationID,
			"a row from another organization crossed the tenant boundary")
	}

	// Absence, stated over the field an attacker would actually read.
	seen := map[string]bool{}
	for _, r := range rowsA {
		seen[r.UserID] = true
	}
	assert.True(t, seen["user-a-1"] && seen["user-a-2"], "org A's own rows must be present")
	for _, leaked := range []string{"user-b-1", "user-b-2", "user-b-3"} {
		assert.False(t, seen[leaked], "org B's user_id %q was returned to org A", leaked)
	}

	// The boundary must hold from the other side too, so the test cannot pass
	// on an implementation that hardcodes or mixes up the parameter.
	rowsB, totalB, err := repo.ListAll(ctx, orgB, 100, 0)
	require.NoError(t, err)
	assert.Len(t, rowsB, 3)
	assert.Equal(t, 3, totalB)
	for _, r := range rowsB {
		require.NotNil(t, r.OrganizationID)
		assert.Equal(t, orgB, *r.OrganizationID)
	}

	// An organization with no records sees nothing and is told nothing about
	// the size of the table.
	rowsNone, totalNone, err := repo.ListAll(ctx, uuid.New(), 100, 0)
	require.NoError(t, err)
	assert.Empty(t, rowsNone)
	assert.Equal(t, 0, totalNone)
}

func TestA2AListAllTenantScope_TrustScores(t *testing.T) {
	db := a2aScopeTestDB(t)
	ctx := context.Background()
	orgA, orgB, agentA, agentB := a2aScopeFixture(t, db)

	// a2a_trust_scores carries no organization_id of its own — it is keyed on
	// agent_id — so the boundary can only be reached by joining agents. This
	// test is what proves that join is present and filters on the right side.
	for _, a := range []uuid.UUID{agentA, agentB} {
		_, err := db.ExecContext(ctx,
			`INSERT INTO a2a_trust_scores (agent_id, a2a_trust_score, data_points)
			 VALUES ($1, 0.5, 1)`, a)
		require.NoError(t, err)
	}

	repo := NewA2ATrustScoreRepository(db)

	scoresA, totalA, err := repo.ListAll(ctx, orgA, 100, 0)
	require.NoError(t, err)
	require.Len(t, scoresA, 1, "org A must see exactly its own agent's score")
	assert.Equal(t, agentA, scoresA[0].AgentID)
	assert.Equal(t, 1, totalA, "total must not count agents in other organizations")

	for _, s := range scoresA {
		assert.NotEqual(t, agentB, s.AgentID,
			"org B's agent ID was returned to org A")
	}

	scoresB, totalB, err := repo.ListAll(ctx, orgB, 100, 0)
	require.NoError(t, err)
	require.Len(t, scoresB, 1)
	assert.Equal(t, agentB, scoresB[0].AgentID)
	assert.Equal(t, 1, totalB)

	scoresNone, totalNone, err := repo.ListAll(ctx, uuid.New(), 100, 0)
	require.NoError(t, err)
	assert.Empty(t, scoresNone)
	assert.Equal(t, 0, totalNone)
}
