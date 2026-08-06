//go:build integration

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every read path on AgentRepository must survive a NULL `agents.description`.
//
// `description` is nullable with no default. Six methods select it; five scanned it
// straight into a plain string, which fails the ENTIRE row with
// "converting NULL to string is unsupported" — not just the one field. Only
// GetByOrganizationPaged used a NullString.
//
// The consequences differ per method and the worst is GetByID, which the agent auth
// middlewares call. A failed read there surfaces as "Agent not found", which is
// indistinguishable from a revocation denial: an agent with no description cannot
// authenticate, and the operator is told it was not found. `agent_revocation_integration_test.go`
// documented that in a comment and worked around it by always setting `description` in its
// own seed, which is why no test ever caught it.
//
// It reached the revocation list too: GetRevocationList could not fail this way only
// because its LIMIT 0 defect meant no row was ever scanned. Fixing that alone turned the
// endpoint into a 500 for any deployment holding one such agent.
//
// This suite asserts the whole class rather than the one method the revocation work
// needed, so a future method that selects `description` has a test that already covers it.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestNullDescription ./internal/infrastructure/repository/...

// seedNullDescriptionAgent inserts one agent with description explicitly NULL and returns
// the ids needed to reach it from every read path.
func seedNullDescriptionAgent(t *testing.T, db *sql.DB, ctx context.Context) (orgID, agentID, mcpServerID uuid.UUID, agentName, mcpServerName string) {
	t.Helper()

	userID := uuid.New()
	orgID, agentID, mcpServerID = uuid.New(), uuid.New(), uuid.New()
	suffix := agentID.String()[:8]
	agentName = "nulldesc-agent-" + suffix
	mcpServerName = "nulldesc-mcp-" + suffix

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM agents WHERE id = $1`, agentID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID)
		_, _ = db.ExecContext(ctx, `DELETE FROM organizations WHERE id = $1`, orgID)
	})

	_, err := db.ExecContext(ctx,
		`INSERT INTO organizations (id, name, domain, created_at, updated_at)
		 VALUES ($1, $2, $3, NOW(), NOW())`,
		orgID, "nulldesc-org-"+suffix, "nulldesc-"+suffix+".example.com")
	require.NoError(t, err)

	_, err = db.ExecContext(ctx,
		`INSERT INTO users (id, organization_id, email, name, password_hash, role,
		                    provider, provider_id, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, 'x', 'admin', 'local', $5, NOW(), NOW())`,
		userID, orgID, "nulldesc-"+suffix+"@example.com", "nulldesc-user", "local-"+suffix)
	require.NoError(t, err)

	// talks_to carries both the MCP server's id and its name, which is what
	// GetByMCPServer and GetByMCPServerName match on with the @> containment operator.
	talksTo := fmt.Sprintf(`["%s", "%s"]`, mcpServerID.String(), mcpServerName)

	// description is NULL. That is the entire point of the fixture, so it is written
	// explicitly rather than omitted, and a future NOT NULL constraint fails here loudly
	// instead of quietly making this suite vacuous.
	_, err = db.ExecContext(ctx,
		`INSERT INTO agents (id, organization_id, name, display_name, description, agent_type,
		                     status, public_key, trust_score, talks_to, created_by,
		                     created_at, updated_at)
		 VALUES ($1, $2, $3, $4, NULL, 'ai_agent', 'active', 'unused', 0.5, $5::jsonb, $6,
		         NOW(), NOW())`,
		agentID, orgID, agentName, "NullDesc Agent", talksTo, userID)
	require.NoError(t, err,
		"seeding a NULL description failed — if the column became NOT NULL this suite no "+
			"longer tests anything and must be revisited, not deleted")

	return orgID, agentID, mcpServerID, agentName, mcpServerName
}

func TestNullDescriptionIsReadableByEveryAgentReadPath(t *testing.T) {
	ctx := context.Background()
	db := agentListTestDB(t)
	repo := NewAgentRepository(db)

	orgID, agentID, mcpServerID, agentName, mcpServerName := seedNullDescriptionAgent(t, db, ctx)

	// GetByID is first because it is the one on the auth path.
	t.Run("GetByID", func(t *testing.T) {
		agent, err := repo.GetByID(agentID)
		require.NoError(t, err,
			"GetByID failed on a NULL description; the auth middlewares report this as "+
				"\"Agent not found\", which reads exactly like a revocation denial")
		require.NotNil(t, agent)
		assert.Equal(t, agentID, agent.ID)
		assert.Empty(t, agent.Description, "a NULL description must read as empty, not fail the row")
	})

	t.Run("GetByName", func(t *testing.T) {
		agent, err := repo.GetByName(orgID, agentName)
		require.NoError(t, err)
		require.NotNil(t, agent)
		assert.Equal(t, agentID, agent.ID)
	})

	t.Run("List", func(t *testing.T) {
		agents, err := repo.List(1000, 0)
		require.NoError(t, err)
		assert.True(t, containsAgent(agents, agentID), "the agent was absent from List")
	})

	t.Run("GetByOrganizationPaged", func(t *testing.T) {
		agents, _, err := repo.GetByOrganizationPaged(orgID, 1000, 0)
		require.NoError(t, err)
		assert.True(t, containsAgent(agents, agentID))
	})

	t.Run("GetByMCPServer", func(t *testing.T) {
		agents, err := repo.GetByMCPServer(mcpServerID, orgID)
		require.NoError(t, err)
		assert.True(t, containsAgent(agents, agentID))
	})

	t.Run("GetByMCPServerName", func(t *testing.T) {
		agents, err := repo.GetByMCPServerName(mcpServerName, orgID)
		require.NoError(t, err)
		assert.True(t, containsAgent(agents, agentID))
	})
}

// The control. The same six paths must still return a non-NULL description intact, so the
// suite above is proving NULL-tolerance rather than a method that ignores the column.
func TestNonNullDescriptionSurvivesEveryAgentReadPath(t *testing.T) {
	ctx := context.Background()
	db := agentListTestDB(t)
	repo := NewAgentRepository(db)

	orgID, agentID, mcpServerID, agentName, mcpServerName := seedNullDescriptionAgent(t, db, ctx)

	const want = "a description that must survive the round trip"
	_, err := db.ExecContext(ctx, `UPDATE agents SET description = $1 WHERE id = $2`, want, agentID)
	require.NoError(t, err)

	byID, err := repo.GetByID(agentID)
	require.NoError(t, err)
	assert.Equal(t, want, byID.Description)

	byName, err := repo.GetByName(orgID, agentName)
	require.NoError(t, err)
	assert.Equal(t, want, byName.Description)

	listed, err := repo.List(1000, 0)
	require.NoError(t, err)
	assert.Equal(t, want, findAgent(t, listed, agentID).Description)

	paged, _, err := repo.GetByOrganizationPaged(orgID, 1000, 0)
	require.NoError(t, err)
	assert.Equal(t, want, findAgent(t, paged, agentID).Description)

	byMCP, err := repo.GetByMCPServer(mcpServerID, orgID)
	require.NoError(t, err)
	assert.Equal(t, want, findAgent(t, byMCP, agentID).Description)

	byMCPName, err := repo.GetByMCPServerName(mcpServerName, orgID)
	require.NoError(t, err)
	assert.Equal(t, want, findAgent(t, byMCPName, agentID).Description)
}

func containsAgent(agents []*domain.Agent, id uuid.UUID) bool {
	for _, a := range agents {
		if a.ID == id {
			return true
		}
	}
	return false
}

func findAgent(t *testing.T, agents []*domain.Agent, id uuid.UUID) *domain.Agent {
	t.Helper()
	for _, a := range agents {
		if a.ID == id {
			return a
		}
	}
	require.FailNow(t, "agent not found in result set", "id=%s", id)
	return nil
}
