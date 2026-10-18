package repository

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

var agentStatusQuery = regexp.QuoteMeta(`SELECT status FROM agents WHERE id = $1`)

// AgentStatus reads one column of one row: the status, whether a row exists,
// a NULL status as the empty status, and a failed read as an error.
func TestAgentRepository_AgentStatus(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewAgentRepository(db)
	id := uuid.New()
	ctx := context.Background()

	mock.ExpectQuery(agentStatusQuery).WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow(string(domain.AgentStatusSuspended)))
	status, found, err := repo.AgentStatus(ctx, id)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, domain.AgentStatusSuspended, status)

	mock.ExpectQuery(agentStatusQuery).WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{"status"}))
	status, found, err = repo.AgentStatus(ctx, id)
	require.NoError(t, err)
	assert.False(t, found, "an id no agent has was reported found")
	assert.Empty(t, status)

	mock.ExpectQuery(agentStatusQuery).WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow(nil))
	status, found, err = repo.AgentStatus(ctx, id)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Empty(t, status, "a NULL status was read as %q", status)

	mock.ExpectQuery(agentStatusQuery).WithArgs(id).
		WillReturnError(errors.New("connection reset by peer"))
	_, found, err = repo.AgentStatus(ctx, id)
	assert.Error(t, err)
	assert.False(t, found)

	require.NoError(t, mock.ExpectationsWereMet())
}

// The read ends with its context: a store that has not answered when the
// context ends gives an error, not a status.
func TestAgentRepository_AgentStatus_EndsWithItsContext(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewAgentRepository(db)
	id := uuid.New()

	mock.ExpectQuery(agentStatusQuery).WithArgs(id).
		WillDelayFor(2 * time.Second).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow(string(domain.AgentStatusVerified)))

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	start := time.Now()
	status, found, err := repo.AgentStatus(ctx, id)
	assert.Error(t, err)
	assert.False(t, found)
	assert.Empty(t, status)
	assert.Less(t, time.Since(start), time.Second, "the read outlived its context")
}
