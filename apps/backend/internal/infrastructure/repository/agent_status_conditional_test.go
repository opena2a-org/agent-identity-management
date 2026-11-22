package repository

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/require"
)

// SetAgentStatusTx writes only to an agent still in the status the caller
// read, so a revocation that came meanwhile is not overwritten.
func TestSetAgentStatusTx_IsConditionalOnTheStatusRead(t *testing.T) {
	id := uuid.New()
	where := regexp.QuoteMeta(`WHERE id = $1 AND status = $4`)

	run := func(t *testing.T, rows int64) error {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()
		mock.ExpectBegin()
		mock.ExpectExec(where).
			WithArgs(id, domain.AgentStatusSuspended, nil, domain.AgentStatusVerified).
			WillReturnResult(sqlmock.NewResult(0, rows))
		tx, err := db.Begin()
		require.NoError(t, err)
		err = SetAgentStatusTx(context.Background(), tx, id, domain.AgentStatusVerified, domain.AgentStatusSuspended, nil)
		require.NoError(t, mock.ExpectationsWereMet())
		return err
	}

	t.Run("agent still in the status read", func(t *testing.T) {
		require.NoError(t, run(t, 1))
	})
	t.Run("agent revoked since it was read", func(t *testing.T) {
		require.ErrorIs(t, run(t, 0), ErrAgentStatusChanged)
	})
}
