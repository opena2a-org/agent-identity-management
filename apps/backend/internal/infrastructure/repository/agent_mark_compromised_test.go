package repository

import (
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/require"
)

// MarkAsCompromised writes suspended only to an agent that is neither suspended
// nor revoked, in the UPDATE itself: a revoked agent moved to suspended is one
// POST /api/v1/agents/:id/reactivate accepts.
func TestAgentRepository_MarkAsCompromised_LeavesARevokedAgentAlone(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewAgentRepository(db)

	id := uuid.New()
	mock.ExpectExec(regexp.QuoteMeta(`WHERE id = $3`)+`\s+`+regexp.QuoteMeta(`AND status NOT IN ($1, $4)`)).
		WithArgs(string(domain.AgentStatusSuspended), sqlmock.AnyArg(), id, string(domain.AgentStatusRevoked)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	require.NoError(t, repo.MarkAsCompromised(id))
	require.NoError(t, mock.ExpectationsWereMet())
}
