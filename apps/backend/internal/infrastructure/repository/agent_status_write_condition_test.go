package repository

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SetAgentStatusTx writes the new status only while the agent still has the status the
// caller read, so a revoke that lands between a reactivate's read and its write is not
// overwritten.

const conditionalAgentStatusUpdate = `UPDATE agents\s+SET status = \$2, verified_at = COALESCE\(\$3, verified_at\), updated_at = NOW\(\)\s+WHERE id = \$1 AND status = \$4`

func TestSetAgentStatusTx_WritesOnlyWhileTheStatusIsTheOneRead(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	id := uuid.New()

	mock.ExpectBegin()
	mock.ExpectExec(conditionalAgentStatusUpdate).
		WithArgs(id, domain.AgentStatusVerified, nil, domain.AgentStatusSuspended).
		WillReturnResult(sqlmock.NewResult(0, 1))
	tx, err := db.Begin()
	require.NoError(t, err)

	require.NoError(t, SetAgentStatusTx(context.Background(), tx, id,
		domain.AgentStatusSuspended, domain.AgentStatusVerified, nil))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSetAgentStatusTx_ReportsAStatusChangedSinceTheRead(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	id := uuid.New()

	mock.ExpectBegin()
	mock.ExpectExec(conditionalAgentStatusUpdate).
		WithArgs(id, domain.AgentStatusVerified, nil, domain.AgentStatusSuspended).
		WillReturnResult(sqlmock.NewResult(0, 0))
	tx, err := db.Begin()
	require.NoError(t, err)

	err = SetAgentStatusTx(context.Background(), tx, id,
		domain.AgentStatusSuspended, domain.AgentStatusVerified, nil)
	assert.ErrorIs(t, err, ErrAgentStatusChanged)
	assert.NoError(t, mock.ExpectationsWereMet())
}
