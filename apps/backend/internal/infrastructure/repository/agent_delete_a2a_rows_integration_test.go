//go:build integration

package repository

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Deleting an agent must reach its A2A task, message and consent rows.
//
// DELETE /api/v1/agents/:id runs `DELETE FROM agents WHERE id = $1`. The five agent
// references on a2a_tasks, a2a_messages and a2a_consent_records were created with no
// ON DELETE rule, so an agent that had taken part in a single A2A task, sent a single
// message or been named in a single consent record could not be deleted at all: the
// statement failed with a foreign-key violation and the route answered 500. Delete agent
// is the only deletion a hosted user has, so for those agents there was no way to
// delete anything.
//
// A refusal was not the fix because nothing lets a user remove those rows first, so it
// would have had no next step. The rows now go with the agent, as every other per-agent
// A2A table already did. An attestation that pointed at one of the deleted tasks is a
// statement between two agents that still exist, so it is kept and only loses the link.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestAgentDelete ./internal/infrastructure/repository/...
func TestAgentDeleteReachesItsA2ATaskMessageAndConsentRows(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping agent delete integration test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping())

	ctx := context.Background()

	f := seedTenant(t, db, ctx, "agent-delete-a2a")
	deleted := f.agentID
	counterpart := seedExtraAgent(t, db, ctx, f)
	bystander := seedExtraAgent(t, db, ctx, f)
	attested := seedExtraAgent(t, db, ctx, f)
	agents := []uuid.UUID{deleted, counterpart, bystander, attested}

	// Registered after the agents, so under t.Cleanup's LIFO order it runs before their
	// cleanup and a failing run leaves no row that would block it.
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx,
			`DELETE FROM a2a_agent_attestations WHERE attesting_agent_id = ANY($1::uuid[]) OR attested_agent_id = ANY($1::uuid[])`,
			uuidArray(agents))
		_, _ = db.ExecContext(ctx, `DELETE FROM a2a_messages WHERE sender_agent_id = ANY($1::uuid[])`, uuidArray(agents))
		_, _ = db.ExecContext(ctx,
			`DELETE FROM a2a_tasks WHERE client_agent_id = ANY($1::uuid[]) OR remote_agent_id = ANY($1::uuid[])`,
			uuidArray(agents))
		_, _ = db.ExecContext(ctx,
			`DELETE FROM a2a_consent_records WHERE grantor_agent_id = ANY($1::uuid[]) OR recipient_agent_id = ANY($1::uuid[])`,
			uuidArray(agents))
	})

	// The deleted agent on each side of every reference.
	asClient := insertA2ATask(t, db, ctx, deleted, counterpart)
	asRemote := insertA2ATask(t, db, ctx, counterpart, deleted)
	inItsTask := insertA2AMessage(t, db, ctx, &asClient, counterpart)
	sentByIt := insertA2AMessage(t, db, ctx, nil, deleted)
	asGrantor := insertA2AConsent(t, db, ctx, f.orgID, deleted, counterpart)
	asRecipient := insertA2AConsent(t, db, ctx, f.orgID, counterpart, deleted)

	// An attestation between two other agents that cites one of the deleted agent's tasks.
	attestation := insertA2AAttestation(t, db, ctx, bystander, attested, asClient)

	// Rows the deleted agent is not part of.
	otherTask := insertA2ATask(t, db, ctx, counterpart, bystander)
	otherMessage := insertA2AMessage(t, db, ctx, &otherTask, counterpart)
	otherConsent := insertA2AConsent(t, db, ctx, f.orgID, counterpart, bystander)

	require.NoError(t, NewAgentRepository(db).Delete(deleted),
		"deleting an agent that has A2A task, message and consent rows must succeed")

	assert.False(t, rowExists(t, db, ctx, "agents", deleted), "the agent row must be gone")
	for name, id := range map[string]uuid.UUID{"task as client": asClient, "task as remote": asRemote} {
		assert.False(t, rowExists(t, db, ctx, "a2a_tasks", id), "%s must be deleted with the agent", name)
	}
	for name, id := range map[string]uuid.UUID{"message in its task": inItsTask, "message it sent": sentByIt} {
		assert.False(t, rowExists(t, db, ctx, "a2a_messages", id), "%s must be deleted with the agent", name)
	}
	for name, id := range map[string]uuid.UUID{"consent as grantor": asGrantor, "consent as recipient": asRecipient} {
		assert.False(t, rowExists(t, db, ctx, "a2a_consent_records", id), "%s must be deleted with the agent", name)
	}

	var taskID sql.NullString
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT task_id FROM a2a_agent_attestations WHERE id = $1`, attestation).Scan(&taskID),
		"an attestation between two remaining agents must be kept")
	assert.False(t, taskID.Valid, "the kept attestation must no longer point at the deleted task")

	for _, id := range []uuid.UUID{counterpart, bystander, attested} {
		assert.True(t, rowExists(t, db, ctx, "agents", id), "other agents must not be deleted")
	}
	assert.True(t, rowExists(t, db, ctx, "a2a_tasks", otherTask), "a task the agent was not part of must be kept")
	assert.True(t, rowExists(t, db, ctx, "a2a_messages", otherMessage), "a message the agent was not part of must be kept")
	assert.True(t, rowExists(t, db, ctx, "a2a_consent_records", otherConsent), "a consent record the agent was not part of must be kept")
}

func insertA2ATask(t *testing.T, db *sql.DB, ctx context.Context, client, remote uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := db.ExecContext(ctx,
		`INSERT INTO a2a_tasks (id, external_task_id, client_agent_id, remote_agent_id)
		 VALUES ($1, $2, $3, $4)`,
		id, "task-"+id.String()[:8], client, remote)
	require.NoError(t, err)
	return id
}

func insertA2AMessage(t *testing.T, db *sql.DB, ctx context.Context, taskID *uuid.UUID, sender uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := db.ExecContext(ctx,
		`INSERT INTO a2a_messages (id, task_id, role, sender_agent_id) VALUES ($1, $2, 'agent', $3)`,
		id, taskID, sender)
	require.NoError(t, err)
	return id
}

func insertA2AConsent(t *testing.T, db *sql.DB, ctx context.Context, orgID, grantor, recipient uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := db.ExecContext(ctx,
		`INSERT INTO a2a_consent_records
		   (id, user_id, organization_id, grantor_agent_id, recipient_agent_id,
		    scope, purpose, data_types, consent_method)
		 VALUES ($1, $2, $3, $4, $5, '["pii"]'::jsonb, 'agent delete test', '["email"]'::jsonb, 'api')`,
		id, "subject-"+id.String()[:8], orgID, grantor, recipient)
	require.NoError(t, err)
	return id
}

func insertA2AAttestation(t *testing.T, db *sql.DB, ctx context.Context, attesting, attested, taskID uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := db.ExecContext(ctx,
		`INSERT INTO a2a_agent_attestations
		   (id, attesting_agent_id, attested_agent_id, task_id, attestation_signature, expires_at)
		 VALUES ($1, $2, $3, $4, 'unused', NOW() + INTERVAL '1 day')`,
		id, attesting, attested, taskID)
	require.NoError(t, err)
	return id
}

// rowExists reports whether table holds a row with the given id. table is always a
// literal from this file.
func rowExists(t *testing.T, db *sql.DB, ctx context.Context, table string, id uuid.UUID) bool {
	t.Helper()
	var exists bool
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM `+table+` WHERE id = $1)`, id).Scan(&exists))
	return exists
}

func uuidArray(ids []uuid.UUID) interface{} {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.String()
	}
	return pq.Array(out)
}
