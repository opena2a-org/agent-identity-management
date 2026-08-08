//go:build integration

package repository

import (
	"context"
	"database/sql"
	"testing"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Cross-tenant scoping at the repository layer, against a real database.
//
// Every defect this file covers had the same shape: the tenant identifier arrived from a
// channel nobody was watching — a query parameter or a request body — and the query that
// consumed it carried no organization predicate. A handler-level check can be forgotten by
// the next caller; a predicate in SQL cannot. These tests therefore assert at the layer the
// fix lives in, and they use a REAL database because the defects were in SQL that a mock
// replaces.
//
// The fixture is deliberately two organizations with equivalent rows. A single-org fixture
// cannot fail these tests: everything belongs to the caller, so an unscoped query and a
// scoped one return the same thing. The foreign org is what makes the assertions able to
// fire at all.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestCrossTenant ./internal/infrastructure/repository/...

type tenantFixture struct {
	orgID   uuid.UUID
	userID  uuid.UUID
	agentID uuid.UUID
}

// seedTenant creates one organization with a user and an agent.
func seedTenant(t *testing.T, db *sql.DB, ctx context.Context, label string) tenantFixture {
	t.Helper()

	f := tenantFixture{orgID: uuid.New(), userID: uuid.New(), agentID: uuid.New()}
	suffix := f.orgID.String()[:8]

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM agents WHERE organization_id = $1`, f.orgID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE organization_id = $1`, f.orgID)
		_, _ = db.ExecContext(ctx, `DELETE FROM organizations WHERE id = $1`, f.orgID)
	})

	_, err := db.ExecContext(ctx,
		`INSERT INTO organizations (id, name, domain, created_at, updated_at)
		 VALUES ($1, $2, $3, NOW(), NOW())`,
		f.orgID, label+"-org-"+suffix, label+"-"+suffix+".example.com")
	require.NoError(t, err)

	_, err = db.ExecContext(ctx,
		`INSERT INTO users (id, organization_id, email, name, password_hash, role,
		                    provider, provider_id, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, 'x', 'admin', 'local', $5, NOW(), NOW())`,
		f.userID, f.orgID, label+"-"+suffix+"@example.com", label+"-user", "local-"+suffix)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx,
		`INSERT INTO agents (id, organization_id, name, display_name, description, agent_type,
		                     status, public_key, trust_score, created_by, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, 'seeded by the cross-tenant suite', 'ai_agent',
		         'verified', 'unused', 0.5, $5, NOW(), NOW())`,
		f.agentID, f.orgID, label+"-agent-"+suffix, label+" Agent", f.userID)
	require.NoError(t, err)

	return f
}

// GetByIDs backs POST /api/v1/agents/bulk-status, whose agent ids come from the request
// body. Before the fix the query was `FROM agents WHERE id IN (...)` — it SELECTED
// organization_id and never filtered on it — so naming a foreign agent's UUID returned its
// status, trust score and last-active.
func TestCrossTenantBulkStatusCannotReadForeignAgents(t *testing.T) {
	ctx := context.Background()
	db := agentListTestDB(t)
	repo := NewAgentRepository(db)

	mine := seedTenant(t, db, ctx, "mine")
	theirs := seedTenant(t, db, ctx, "theirs")

	// Ask for BOTH, exactly as an attacker would: their id alongside a legitimate one, so
	// a half-applied filter shows up as a partial result rather than an error.
	agents, err := repo.GetByIDs(ctx, mine.orgID, []uuid.UUID{mine.agentID, theirs.agentID})
	require.NoError(t, err)

	got := make(map[uuid.UUID]bool, len(agents))
	for _, a := range agents {
		got[a.ID] = true
	}

	assert.True(t, got[mine.agentID], "control: my own agent must be returned, or this test proves nothing")
	assert.False(t, got[theirs.agentID],
		"another organization's agent was returned for an id supplied in the request body")
}

// A caller with no organization must get nothing rather than everything. uuid.Nil means the
// auth chain did not populate the org and the handler proceeded anyway; matching Nil against
// real rows would disclose them.
func TestCrossTenantBulkStatusFailsClosedWithoutAnOrg(t *testing.T) {
	ctx := context.Background()
	db := agentListTestDB(t)
	repo := NewAgentRepository(db)

	mine := seedTenant(t, db, ctx, "mine")

	agents, err := repo.GetByIDs(ctx, uuid.Nil, []uuid.UUID{mine.agentID})

	require.Error(t, err, "a nil caller organization must be refused, not treated as a wildcard")
	assert.Empty(t, agents, "no rows may be returned alongside the error")
}

// ListTasks backs GET /api/v1/a2a/tasks. Before the fix its query was
// `FROM a2a_tasks WHERE 1=1` with no organization predicate anywhere in the handler,
// service or repository — so any authenticated user could page through every task in the
// system without even naming a target.
func TestCrossTenantA2ATasksAreScopedToTheCallersOrg(t *testing.T) {
	ctx := context.Background()
	db := agentListTestDB(t)
	repo := &A2ATaskRepository{db: db}

	mine := seedTenant(t, db, ctx, "mine")
	theirs := seedTenant(t, db, ctx, "theirs")

	mineTask := seedA2ATask(t, db, ctx, mine.agentID, mine.agentID)
	theirsTask := seedA2ATask(t, db, ctx, theirs.agentID, theirs.agentID)

	// No agentId filter — this is the whole-table enumeration the defect allowed.
	tasks, total, err := repo.ListTasks(ctx, mine.orgID, nil, "", 100, 0)
	require.NoError(t, err)

	got := make(map[uuid.UUID]bool, len(tasks))
	for _, task := range tasks {
		got[task.ID] = true
	}

	assert.True(t, got[mineTask], "control: my own task must be listed")
	assert.False(t, got[theirsTask],
		"another organization's A2A task was listed to a caller who named no target at all")
	assert.Equal(t, len(tasks), total,
		"the reported total must describe the same scoped set as the rows, or a caller learns the foreign count")
}

// A task whose OTHER end is a foreign agent must still be visible to the org that owns one
// end. That is what a bilateral record means, and a fix that hid it would be over-scoped —
// breaking the feature while looking secure.
func TestCrossTenantA2ATaskIsVisibleToEitherParticipant(t *testing.T) {
	ctx := context.Background()
	db := agentListTestDB(t)
	repo := &A2ATaskRepository{db: db}

	mine := seedTenant(t, db, ctx, "mine")
	theirs := seedTenant(t, db, ctx, "theirs")

	shared := seedA2ATask(t, db, ctx, mine.agentID, theirs.agentID)

	for _, tc := range []struct {
		name  string
		orgID uuid.UUID
	}{
		{"client side", mine.orgID},
		{"remote side", theirs.orgID},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tasks, _, err := repo.ListTasks(ctx, tc.orgID, nil, "", 100, 0)
			require.NoError(t, err)

			found := false
			for _, task := range tasks {
				if task.ID == shared {
					found = true
				}
			}
			assert.True(t, found,
				"a participant could not see a task its own agent took part in; the scoping is too tight")
		})
	}
}

func TestCrossTenantA2ATasksFailClosedWithoutAnOrg(t *testing.T) {
	ctx := context.Background()
	db := agentListTestDB(t)
	repo := &A2ATaskRepository{db: db}

	_, _, err := repo.ListTasks(ctx, uuid.Nil, nil, "", 100, 0)
	require.Error(t, err, "a nil caller organization must be refused, not treated as a wildcard")
}

// seedA2ATask inserts one task between two agents and returns its id.
func seedA2ATask(t *testing.T, db *sql.DB, ctx context.Context, clientAgent, remoteAgent uuid.UUID) uuid.UUID {
	t.Helper()

	id := uuid.New()
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, `DELETE FROM a2a_tasks WHERE id = $1`, id) })

	_, err := db.ExecContext(ctx,
		`INSERT INTO a2a_tasks (id, external_task_id, client_agent_id, remote_agent_id, state, created_at)
		 VALUES ($1, $2, $3, $4, 'SUBMITTED', NOW())`,
		id, "task-"+id.String()[:8], clientAgent, remoteAgent)
	require.NoError(t, err)

	return id
}

// CheckConsent must answer only about the caller's own organization.
//
// The query matched on user_id, grantor_agent_id, recipient_agent_id and scope
// and nothing else, so its boolean described any row in the table whatever org
// owned it. user_id is what makes that sharp: an unvalidated VARCHAR(200) with
// no ownership relation to anything, filtered on directly, so a caller who
// guesses one gets a yes/no about another tenant's consent state.
func TestCrossTenantConsentCheckIsScopedToTheCallersOrg(t *testing.T) {
	ctx := context.Background()
	db := agentListTestDB(t)
	repo := NewA2AConsentRepository(db)

	mine := seedTenant(t, db, ctx, "mine")
	theirs := seedTenant(t, db, ctx, "theirs")

	const userID = "consent-oracle-subject"
	seedA2AConsent(t, db, ctx, theirs.orgID, theirs.agentID, theirs.agentID, userID)

	// The caller knows every matching value except the organization. Before the
	// predicate that was enough.
	got, err := repo.CheckConsent(ctx, mine.orgID, userID, theirs.agentID, theirs.agentID, "pii")
	require.NoError(t, err)
	assert.False(t, got,
		"CheckConsent answered true about another organization's consent record; the boolean is a cross-tenant oracle")

	// Control, so the test cannot pass by always returning false.
	got, err = repo.CheckConsent(ctx, theirs.orgID, userID, theirs.agentID, theirs.agentID, "pii")
	require.NoError(t, err)
	assert.True(t, got, "control: the owning organization must still see its own consent")
}

// Over-scoping guard: the RECIPIENT may be another organization's agent. That
// is the entire purpose of cross-agent consent, so a fix that scoped the
// recipient too would break the feature. This test fails if someone adds it.
func TestCrossTenantConsentCheckAllowsCrossOrgRecipient(t *testing.T) {
	ctx := context.Background()
	db := agentListTestDB(t)
	repo := NewA2AConsentRepository(db)

	mine := seedTenant(t, db, ctx, "mine")
	theirs := seedTenant(t, db, ctx, "theirs")

	const userID = "cross-org-consent-subject"
	seedA2AConsent(t, db, ctx, mine.orgID, mine.agentID, theirs.agentID, userID)

	got, err := repo.CheckConsent(ctx, mine.orgID, userID, mine.agentID, theirs.agentID, "pii")
	require.NoError(t, err)
	assert.True(t, got,
		"a consent grant naming another organization's agent as recipient was not found by its owner; the scoping is too tight")
}

// seedA2AConsent inserts one live consent record owned by orgID.
func seedA2AConsent(t *testing.T, db *sql.DB, ctx context.Context, orgID, grantor, recipient uuid.UUID, userID string) {
	t.Helper()

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM a2a_consent_records WHERE organization_id = $1`, orgID)
	})

	_, err := db.ExecContext(ctx,
		`INSERT INTO a2a_consent_records
		   (user_id, organization_id, grantor_agent_id, recipient_agent_id,
		    scope, purpose, data_types, consent_method)
		 VALUES ($1, $2, $3, $4, '["pii"]'::jsonb, 'scoping test', '["email"]'::jsonb, 'api')`,
		userID, orgID, grantor, recipient)
	require.NoError(t, err)
}

// GET /api/v1/a2a/cards returned every tenant's agent cards. The only predicate
// was is_valid = TRUE, so card_url, the full card_data document, agent_id and
// attestation_signature crossed the organization boundary to any authenticated
// caller. a2a_agent_cards has no organization_id of its own, so the boundary is
// reached by joining agents -- this test is what proves that join is present and
// filters on the right side.
func TestCrossTenantAgentCardsAreScopedToTheCallersOrg(t *testing.T) {
	ctx := context.Background()
	db := agentListTestDB(t)
	repo := NewA2AAgentCardRepository(db)

	mine := seedTenant(t, db, ctx, "mine")
	theirs := seedTenant(t, db, ctx, "theirs")

	// Asymmetric on purpose: one card for my org, two for theirs. Equal counts
	// would let a wrong-but-plausible implementation look right by coincidence.
	theirsSecondAgent := seedExtraAgent(t, db, ctx, theirs)
	mineCard := seedAgentCard(t, db, ctx, mine.agentID, true)
	theirsCard := seedAgentCard(t, db, ctx, theirs.agentID, true)
	theirsCard2 := seedAgentCard(t, db, ctx, theirsSecondAgent, true)

	cards, err := repo.GetValidCards(ctx, mine.orgID, 100, 0)
	require.NoError(t, err)

	got := make(map[uuid.UUID]bool, len(cards))
	for _, c := range cards {
		got[c.ID] = true
	}

	assert.True(t, got[mineCard], "control: my own agent card must be listed")
	assert.False(t, got[theirsCard],
		"another organization's agent card was returned to a caller who asked for nothing but a page")
	assert.False(t, got[theirsCard2],
		"another organization's second agent card was returned")
	assert.Len(t, cards, 1, "org mine owns exactly one valid card")

	// The boundary must hold from the other side too, so the test cannot pass on
	// an implementation that hardcodes or mixes up the parameter.
	theirCards, err := repo.GetValidCards(ctx, theirs.orgID, 100, 0)
	require.NoError(t, err)
	assert.Len(t, theirCards, 2, "org theirs owns exactly two valid cards")

	// An organization with no cards sees nothing.
	none, err := repo.GetValidCards(ctx, uuid.New(), 100, 0)
	require.NoError(t, err)
	assert.Empty(t, none)
}

// The org predicate must not have displaced the is_valid filter it was added
// alongside. A fix that scoped by organization and quietly started returning
// invalid cards would pass every assertion above.
func TestCrossTenantAgentCardsStillExcludeInvalidOnes(t *testing.T) {
	ctx := context.Background()
	db := agentListTestDB(t)
	repo := NewA2AAgentCardRepository(db)

	mine := seedTenant(t, db, ctx, "mine")
	secondAgent := seedExtraAgent(t, db, ctx, mine)

	validCard := seedAgentCard(t, db, ctx, mine.agentID, true)
	invalidCard := seedAgentCard(t, db, ctx, secondAgent, false)

	cards, err := repo.GetValidCards(ctx, mine.orgID, 100, 0)
	require.NoError(t, err)

	got := make(map[uuid.UUID]bool, len(cards))
	for _, c := range cards {
		got[c.ID] = true
	}
	assert.True(t, got[validCard], "control: the valid card must be listed")
	assert.False(t, got[invalidCard], "an invalid card was listed; the is_valid filter was lost")
}

// seedExtraAgent adds a second agent to an existing tenant and returns its id.
func seedExtraAgent(t *testing.T, db *sql.DB, ctx context.Context, f tenantFixture) uuid.UUID {
	t.Helper()

	id := uuid.New()
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, `DELETE FROM agents WHERE id = $1`, id) })

	_, err := db.ExecContext(ctx,
		`INSERT INTO agents (id, name, display_name, agent_type, created_by, organization_id)
		 VALUES ($1, $2, $2, 'service', $3, $4)`,
		id, "extra-agent-"+id.String()[:8], f.userID, f.orgID)
	require.NoError(t, err)

	return id
}

// seedAgentCard inserts one agent card for an agent and returns its id.
func seedAgentCard(t *testing.T, db *sql.DB, ctx context.Context, agentID uuid.UUID, valid bool) uuid.UUID {
	t.Helper()

	id := uuid.New()
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, `DELETE FROM a2a_agent_cards WHERE id = $1`, id) })

	_, err := db.ExecContext(ctx,
		`INSERT INTO a2a_agent_cards
		   (id, agent_id, card_url, card_data, card_hash, is_valid, attestation_signature)
		 VALUES ($1, $2, $3, $4::jsonb, $5, $6, $7)`,
		id, agentID,
		"https://cards.example.invalid/"+id.String()[:8],
		`{"name":"scoping-test","url":"https://example.invalid"}`,
		id.String()[:16], valid, "sig-"+id.String()[:16])
	require.NoError(t, err)

	return id
}
