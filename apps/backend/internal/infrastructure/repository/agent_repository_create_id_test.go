package repository

import (
	"database/sql/driver"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// agentInsertArgs expects the agent ID as the first INSERT argument and accepts
// any value for the other 37, verified_at last.
func agentInsertArgs(id driver.Value) []driver.Value {
	args := []driver.Value{id}
	for i := 0; i < 37; i++ {
		args = append(args, sqlmock.AnyArg())
	}
	return args
}

// A server-generated private key is encrypted bound to the agent's ID before
// the row is inserted, so Create must store the ID the caller assigned.
func TestAgentRepository_Create_KeepsCallerAssignedID(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewAgentRepository(db)

	assigned := uuid.New()
	mock.ExpectExec("INSERT INTO agents").WithArgs(agentInsertArgs(assigned)...).
		WillReturnResult(sqlmock.NewResult(0, 1))

	agent := &domain.Agent{ID: assigned, OrganizationID: uuid.New(), Name: "kept-id", AgentType: domain.AgentTypeAI}
	require.NoError(t, repo.Create(agent))
	require.NoError(t, mock.ExpectationsWereMet())
	assert.Equal(t, assigned, agent.ID)
}

func TestAgentRepository_Create_AssignsIDWhenUnset(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewAgentRepository(db)

	mock.ExpectExec("INSERT INTO agents").WithArgs(agentInsertArgs(sqlmock.AnyArg())...).
		WillReturnResult(sqlmock.NewResult(0, 1))

	agent := &domain.Agent{OrganizationID: uuid.New(), Name: "new-id", AgentType: domain.AgentTypeAI}
	require.NoError(t, repo.Create(agent))
	require.NoError(t, mock.ExpectationsWereMet())
	assert.NotEqual(t, uuid.Nil, agent.ID)
}
