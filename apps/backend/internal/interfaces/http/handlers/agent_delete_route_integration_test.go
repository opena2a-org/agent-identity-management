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

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
)

// DELETE /api/v1/agents/:id through the real handler, service and repository.
//
// Delete agent is the only deletion a hosted user has. An agent named in an A2A task,
// message or consent record could not be deleted (the references had no ON DELETE rule)
// and the route answered 500 with the database's foreign-key message as `error`, which
// the dashboard showed to the person who pressed Delete. The repository suite asserts
// what the delete statement reaches; these tests assert what the route answers.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestAgentDeleteRoute ./internal/interfaces/http/handlers/...

func agentDeleteRouteDB(t *testing.T) *sql.DB {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping agent delete route integration test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping())
	return db
}

type agentDeleteRouteTenant struct {
	orgID, userID uuid.UUID
	suffix        string
}

func seedAgentDeleteRouteTenant(t *testing.T, db *sql.DB, ctx context.Context) agentDeleteRouteTenant {
	t.Helper()

	f := agentDeleteRouteTenant{orgID: uuid.New(), userID: uuid.New()}
	f.suffix = f.orgID.String()[:8]

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM agents WHERE organization_id = $1`, f.orgID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, f.userID)
		_, _ = db.ExecContext(ctx, `DELETE FROM organizations WHERE id = $1`, f.orgID)
	})

	_, err := db.ExecContext(ctx,
		`INSERT INTO organizations (id, name, domain, created_at, updated_at)
		 VALUES ($1, $2, $3, NOW(), NOW())`,
		f.orgID, "agent-delete-route-org-"+f.suffix, "agent-delete-route-"+f.suffix+".example.com")
	require.NoError(t, err)

	_, err = db.ExecContext(ctx,
		`INSERT INTO users (id, organization_id, email, name, password_hash, role,
		                    provider, provider_id, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, 'x', 'admin', 'local', $5, NOW(), NOW())`,
		f.userID, f.orgID, "agent-delete-route-"+f.suffix+"@example.com", "agent-delete-route-user", "local-"+f.suffix)
	require.NoError(t, err)

	return f
}

// seedAgentDeleteRouteAgent inserts one agent in the tenant. `description` is scanned
// into a plain string, so it is set to keep the handler's GetAgent read working.
func seedAgentDeleteRouteAgent(t *testing.T, db *sql.DB, ctx context.Context, f agentDeleteRouteTenant) uuid.UUID {
	t.Helper()

	id := uuid.New()
	_, err := db.ExecContext(ctx,
		`INSERT INTO agents (id, organization_id, name, display_name, description, agent_type,
		                     status, public_key, trust_score, created_by, created_at, updated_at)
		 VALUES ($1, $2, $3, 'Agent Delete Route Agent', 'seeded by the agent delete route suite',
		         'ai_agent', 'verified', 'unused-for-this-route', 0.5, $4, NOW(), NOW())`,
		id, f.orgID, "agent-delete-route-"+id.String()[:8], f.userID)
	require.NoError(t, err)
	return id
}

// deleteAgentThroughRoute mounts the real handler over the real service and repository
// at the route's path, with the org and user the auth middleware would set, and returns
// the status and raw body. The audit service is a stub: the route writes its audit row
// after the delete, and these tests are about the delete.
func deleteAgentThroughRoute(t *testing.T, db *sql.DB, f agentDeleteRouteTenant, agentID uuid.UUID) (int, []byte) {
	t.Helper()

	// GetAgent and DeleteAgent read the agent repository only.
	agentService := application.NewAgentService(
		repository.NewAgentRepository(db), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	handler := NewAgentHandlerWithInterfaces(
		agentService,
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)

	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Locals("organization_id", f.orgID)
		c.Locals("user_id", f.userID)
		return c.Next()
	})
	app.Delete("/api/v1/agents/:id", handler.DeleteAgent)

	resp, err := app.Test(httptest.NewRequest("DELETE", "/api/v1/agents/"+agentID.String(), nil), fiber.TestConfig{Timeout: 0})
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, raw
}

func agentDeleteRouteRowExists(t *testing.T, db *sql.DB, ctx context.Context, table string, id uuid.UUID) bool {
	t.Helper()
	var exists bool
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM `+table+` WHERE id = $1)`, id).Scan(&exists))
	return exists
}

func TestAgentDeleteRouteDeletesAnAgentWithA2ATaskMessageAndConsentRows(t *testing.T) {
	db := agentDeleteRouteDB(t)
	ctx := context.Background()

	f := seedAgentDeleteRouteTenant(t, db, ctx)
	deleted := seedAgentDeleteRouteAgent(t, db, ctx, f)
	counterpart := seedAgentDeleteRouteAgent(t, db, ctx, f)

	// Registered after the agents, so under t.Cleanup's LIFO order it runs before their
	// cleanup and a failing run leaves no row that would block it.
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM a2a_messages WHERE sender_agent_id = ANY($1::uuid[])`,
			"{"+deleted.String()+","+counterpart.String()+"}")
		_, _ = db.ExecContext(ctx, `DELETE FROM a2a_tasks WHERE client_agent_id = $1 OR remote_agent_id = $1`, deleted)
		_, _ = db.ExecContext(ctx, `DELETE FROM a2a_consent_records WHERE organization_id = $1`, f.orgID)
	})

	taskID, messageID, consentID := uuid.New(), uuid.New(), uuid.New()
	_, err := db.ExecContext(ctx,
		`INSERT INTO a2a_tasks (id, external_task_id, client_agent_id, remote_agent_id)
		 VALUES ($1, $2, $3, $4)`,
		taskID, "task-"+taskID.String()[:8], deleted, counterpart)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx,
		`INSERT INTO a2a_messages (id, task_id, role, sender_agent_id) VALUES ($1, $2, 'agent', $3)`,
		messageID, taskID, deleted)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx,
		`INSERT INTO a2a_consent_records
		   (id, user_id, organization_id, grantor_agent_id, recipient_agent_id,
		    scope, purpose, data_types, consent_method)
		 VALUES ($1, $2, $3, $4, $5, '["pii"]'::jsonb, 'agent delete route test', '["email"]'::jsonb, 'api')`,
		consentID, "subject-"+consentID.String()[:8], f.orgID, deleted, counterpart)
	require.NoError(t, err)

	status, raw := deleteAgentThroughRoute(t, db, f, deleted)

	require.Equal(t, fiber.StatusNoContent, status,
		"deleting an agent with A2A task, message and consent rows must succeed; body: %s", raw)
	assert.False(t, agentDeleteRouteRowExists(t, db, ctx, "agents", deleted), "the agent row must be gone")
	assert.False(t, agentDeleteRouteRowExists(t, db, ctx, "a2a_tasks", taskID), "its A2A task must be deleted with it")
	assert.False(t, agentDeleteRouteRowExists(t, db, ctx, "a2a_messages", messageID), "its A2A message must be deleted with it")
	assert.False(t, agentDeleteRouteRowExists(t, db, ctx, "a2a_consent_records", consentID), "its consent record must be deleted with it")
	assert.True(t, agentDeleteRouteRowExists(t, db, ctx, "agents", counterpart), "the other agent must not be deleted")
}

// A delete the database refuses must answer with a stated outcome and next step, and
// remove nothing. A reference with no ON DELETE rule, created by this test and dropped
// when it ends, stands in for the A2A references as they were before they followed the
// agent, so the refusal comes from the real database through the real route.
func TestAgentDeleteRouteFailureStatesAReasonNotTheDatabaseError(t *testing.T) {
	db := agentDeleteRouteDB(t)
	ctx := context.Background()

	f := seedAgentDeleteRouteTenant(t, db, ctx)
	agentID := seedAgentDeleteRouteAgent(t, db, ctx, f)

	blocker := "agent_delete_route_blocker_" + uuid.New().String()[:8]
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, `DROP TABLE IF EXISTS `+blocker) })
	_, err := db.ExecContext(ctx,
		`CREATE TABLE `+blocker+` (id UUID PRIMARY KEY, agent_id UUID NOT NULL REFERENCES agents(id))`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO `+blocker+` (id, agent_id) VALUES ($1, $2)`, uuid.New(), agentID)
	require.NoError(t, err)

	status, raw := deleteAgentThroughRoute(t, db, f, agentID)

	assert.Equal(t, fiber.StatusInternalServerError, status)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &body), "body: %s", raw)
	assert.Equal(t, agentDeleteFailedMessage, body["error"])
	for _, leaked := range []string{"pq:", "foreign key", "constraint", blocker} {
		assert.NotContains(t, string(raw), leaked, "the response must not carry the database error")
	}
	assert.True(t, agentDeleteRouteRowExists(t, db, ctx, "agents", agentID),
		"a refused delete must leave the agent in place, as the answer states")
}
