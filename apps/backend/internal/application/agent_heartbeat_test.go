package application

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RecordHeartbeat stores a heartbeat time and writes nothing else to the agent row.
//
// heartbeatFakeRepo stores one agent and applies Update the way its SQL does: every
// column from the struct except last_heartbeat, which that statement does not name. Its
// first GetByID commits a suspension and a key rotation after taking its read, as a
// request landing between a heartbeat's read and its write would. The statements
// themselves are pinned against Postgres in heartbeat_race_integration_test.go.
type heartbeatFakeRepo struct {
	domain.AgentRepository
	stored       domain.Agent
	commitOnRead func(stored *domain.Agent)
	updates      int
}

func (r *heartbeatFakeRepo) GetByID(id uuid.UUID) (*domain.Agent, error) {
	if id != r.stored.ID {
		return nil, sql.ErrNoRows
	}
	read := r.stored
	if r.commitOnRead != nil {
		r.commitOnRead(&r.stored)
		r.commitOnRead = nil
	}
	return &read, nil
}

func (r *heartbeatFakeRepo) Update(agent *domain.Agent) error {
	r.updates++
	lastHeartbeat := r.stored.LastHeartbeat
	r.stored = *agent
	r.stored.LastHeartbeat = lastHeartbeat
	return nil
}

func (r *heartbeatFakeRepo) UpdateHeartbeat(_ context.Context, id uuid.UUID) (time.Time, error) {
	if id != r.stored.ID {
		return time.Time{}, sql.ErrNoRows
	}
	now := time.Now()
	r.stored.LastHeartbeat = &now
	return now, nil
}

func TestRecordHeartbeat_StoresHeartbeatTimeAndKeepsConcurrentChanges(t *testing.T) {
	oldKey, rotatedKey := "OLD-PUBLIC-KEY", "ROTATED-PUBLIC-KEY"
	id := uuid.New()
	repo := &heartbeatFakeRepo{stored: domain.Agent{
		ID:            id,
		Status:        domain.AgentStatusVerified,
		PublicKey:     &oldKey,
		RotationCount: 1,
	}}
	repo.commitOnRead = func(stored *domain.Agent) {
		stored.Status = domain.AgentStatusSuspended
		stored.PreviousPublicKey = stored.PublicKey
		stored.PublicKey = &rotatedKey
		stored.RotationCount = 2
	}
	service := &AgentService{agentRepo: repo}

	before := time.Now()
	agent, err := service.RecordHeartbeat(context.Background(), id)
	require.NoError(t, err)

	assert.Equal(t, domain.AgentStatusSuspended, repo.stored.Status,
		"the heartbeat overwrote a suspension committed after its read")
	require.NotNil(t, repo.stored.PublicKey)
	assert.Equal(t, rotatedKey, *repo.stored.PublicKey,
		"the heartbeat overwrote a key rotation committed after its read")
	assert.Equal(t, 2, repo.stored.RotationCount)
	assert.Zero(t, repo.updates, "a heartbeat must not write the agent row through Update")
	require.NotNil(t, repo.stored.LastHeartbeat, "the heartbeat stored no heartbeat time")
	assert.False(t, repo.stored.LastHeartbeat.Before(before))

	require.NotNil(t, agent.LastHeartbeat)
	assert.True(t, agent.LastHeartbeat.Equal(*repo.stored.LastHeartbeat),
		"the response must report the heartbeat time that was stored")
}

func TestRecordHeartbeat_UnknownAgentWritesNothing(t *testing.T) {
	repo := &heartbeatFakeRepo{stored: domain.Agent{ID: uuid.New(), Status: domain.AgentStatusVerified}}
	service := &AgentService{agentRepo: repo}

	_, err := service.RecordHeartbeat(context.Background(), uuid.New())
	require.Error(t, err)
	assert.Nil(t, repo.stored.LastHeartbeat)
	assert.Zero(t, repo.updates)
}
