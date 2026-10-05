//go:build integration

package application

import (
	"context"
	"database/sql"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// POST /api/v1/sdk-api/agents/:id/heartbeat stores a heartbeat time and writes no other
// agent column. It used to read the row and write all of it back through
// AgentRepository.Update, whose statement has no last_heartbeat column, so no heartbeat
// time was stored, and a suspension or key rotation another request committed between
// that read and the write was overwritten from the read.
//
// The case commits a suspension and a key rotation right after the heartbeat's first
// agent read, lets the heartbeat finish, and reads the row back with SQL. The statements
// are what is under test, so this drives the real AgentRepository against a real
// Postgres rather than a mock. The name carries RoundTrip so the CI step that fails on a
// skipped integration test selects it.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestRecordHeartbeat ./internal/application/...

func heartbeatRaceDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping heartbeat race test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping())
	return db
}

// heartbeatCommitAfterRead wraps the repository the heartbeat goes through. Its first
// GetByID takes its read and then commits the competing change before returning it.
type heartbeatCommitAfterRead struct {
	domain.AgentRepository
	once   sync.Once
	commit func()
}

func (h *heartbeatCommitAfterRead) GetByID(id uuid.UUID) (*domain.Agent, error) {
	agent, err := h.AgentRepository.GetByID(id)
	h.once.Do(h.commit)
	return agent, err
}

func TestRecordHeartbeat_RoundTripKeepsConcurrentSuspensionAndKeyRotation(t *testing.T) {
	db := heartbeatRaceDB(t)
	ctx := context.Background()
	orgID, userID := seedOrgAndUser(t, db, ctx, "heartbeat-race")

	agentID := uuid.New()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM agents WHERE id = $1`, agentID)
	})
	_, err := db.ExecContext(ctx,
		`INSERT INTO agents (id, organization_id, name, display_name, agent_type, status,
		                     public_key, encrypted_private_key, key_algorithm, key_created_at,
		                     rotation_count, capabilities, created_by, created_at, updated_at)
		 VALUES ($1, $2, $3, 'Heartbeat Race Agent', 'ai_agent', 'verified',
		         'PUB', 'SECRET-ENCRYPTED-KEY', 'Ed25519', NOW() - INTERVAL '30 days',
		         1, '["cap:one"]'::jsonb, $4, NOW(), NOW())`,
		agentID, orgID, "heartbeat-race-"+agentID.String()[:8], userID)
	require.NoError(t, err)

	repo := &heartbeatCommitAfterRead{
		AgentRepository: repository.NewAgentRepository(db),
		commit: func() {
			_, err := db.ExecContext(ctx,
				`UPDATE agents
				    SET status = 'suspended',
				        previous_public_key = public_key, public_key = 'ROTATED-PUB',
				        encrypted_private_key = 'ROTATED-SECRET',
				        rotation_count = rotation_count + 1, key_created_at = NOW()
				  WHERE id = $1`, agentID)
			require.NoError(t, err)
		},
	}
	service := NewAgentService(repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	agent, err := service.RecordHeartbeat(ctx, agentID)
	require.NoError(t, err)

	var (
		status, publicKey, encryptedKey string
		previousKey                     sql.NullString
		rotationCount                   int
		lastHeartbeat                   sql.NullTime
	)
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT status, public_key, previous_public_key, encrypted_private_key,
		        rotation_count, last_heartbeat
		   FROM agents WHERE id = $1`, agentID,
	).Scan(&status, &publicKey, &previousKey, &encryptedKey, &rotationCount, &lastHeartbeat))

	assert.Equal(t, "suspended", status, "the heartbeat overwrote a committed suspension")
	assert.Equal(t, "ROTATED-PUB", publicKey, "the heartbeat overwrote a committed key rotation")
	assert.Equal(t, "PUB", previousKey.String)
	assert.Equal(t, "ROTATED-SECRET", encryptedKey)
	assert.Equal(t, 2, rotationCount)
	require.True(t, lastHeartbeat.Valid, "the heartbeat stored no last_heartbeat")
	assert.WithinDuration(t, time.Now(), lastHeartbeat.Time, time.Minute)

	require.NotNil(t, agent.LastHeartbeat)
	assert.True(t, agent.LastHeartbeat.Equal(lastHeartbeat.Time),
		"the response must report the stored heartbeat time, got %s want %s",
		agent.LastHeartbeat, lastHeartbeat.Time)
}
