package repository

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// UpdateStatus writes a capability request's new status only while the request
// is in the one status that leads to it: a decision (approved, auto-approved or
// rejected) only while the request is pending, and pending, the undo of an
// approval whose grant failed, only while it is approved. Two administrators
// deciding one pending request at once both read it as pending, but only the
// first write finds it pending; the later one writes nothing and its error
// wraps ErrCapabilityRequestNotPending, which the decision routes answer 409.

const conditionalStatusUpdate = `UPDATE capability_requests\s+SET\s+status = \$1,\s+reviewed_by = \$2,\s+reviewed_at = \$3,\s+updated_at = \$4\s+WHERE id = \$5 AND status = \$6`

const capabilityRequestExists = `SELECT EXISTS \(SELECT 1 FROM capability_requests WHERE id = \$1\)`

func newCapabilityRequestRepoMock(t *testing.T) (domain.CapabilityRequestRepository, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return NewCapabilityRequestRepository(sqlx.NewDb(db, "postgres")), mock
}

func TestCapabilityRequestUpdateStatus_WritesADecisionOnlyWhileThePendingRequestIsPending(t *testing.T) {
	for _, status := range []domain.CapabilityRequestStatus{
		domain.CapabilityRequestStatusApproved,
		domain.CapabilityRequestStatusAutoApproved,
		domain.CapabilityRequestStatusRejected,
	} {
		t.Run(string(status), func(t *testing.T) {
			repo, mock := newCapabilityRequestRepoMock(t)
			id, reviewer := uuid.New(), uuid.New()
			mock.ExpectExec(conditionalStatusUpdate).
				WithArgs(status, reviewer, sqlmock.AnyArg(), sqlmock.AnyArg(), id, domain.CapabilityRequestStatusPending).
				WillReturnResult(sqlmock.NewResult(0, 1))

			require.NoError(t, repo.UpdateStatus(id, status, reviewer))
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestCapabilityRequestUpdateStatus_UndoesAnApprovalOnlyWhileTheRequestIsApproved(t *testing.T) {
	repo, mock := newCapabilityRequestRepoMock(t)
	id, reviewer := uuid.New(), uuid.New()
	mock.ExpectExec(conditionalStatusUpdate).
		WithArgs(domain.CapabilityRequestStatusPending, reviewer, sqlmock.AnyArg(), sqlmock.AnyArg(), id, domain.CapabilityRequestStatusApproved).
		WillReturnResult(sqlmock.NewResult(0, 1))

	require.NoError(t, repo.UpdateStatus(id, domain.CapabilityRequestStatusPending, reviewer))
	assert.NoError(t, mock.ExpectationsWereMet())
}

// The later of two decisions finds the request decided by the first.
func TestCapabilityRequestUpdateStatus_ADecisionOnARequestDecidedMeanwhileIsNotPending(t *testing.T) {
	for _, status := range []domain.CapabilityRequestStatus{
		domain.CapabilityRequestStatusApproved,
		domain.CapabilityRequestStatusRejected,
	} {
		t.Run(string(status), func(t *testing.T) {
			repo, mock := newCapabilityRequestRepoMock(t)
			id, reviewer := uuid.New(), uuid.New()
			mock.ExpectExec(conditionalStatusUpdate).
				WithArgs(status, reviewer, sqlmock.AnyArg(), sqlmock.AnyArg(), id, domain.CapabilityRequestStatusPending).
				WillReturnResult(sqlmock.NewResult(0, 0))
			mock.ExpectQuery(capabilityRequestExists).WithArgs(id).
				WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

			err := repo.UpdateStatus(id, status, reviewer)

			require.ErrorIs(t, err, ErrCapabilityRequestNotPending)
			assert.NotErrorIs(t, err, ErrCapabilityRequestNotFound)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestCapabilityRequestUpdateStatus_ADecisionOnARequestWithNoRowIsNotFound(t *testing.T) {
	repo, mock := newCapabilityRequestRepoMock(t)
	id, reviewer := uuid.New(), uuid.New()
	mock.ExpectExec(conditionalStatusUpdate).
		WithArgs(domain.CapabilityRequestStatusApproved, reviewer, sqlmock.AnyArg(), sqlmock.AnyArg(), id, domain.CapabilityRequestStatusPending).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(capabilityRequestExists).WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	err := repo.UpdateStatus(id, domain.CapabilityRequestStatusApproved, reviewer)

	require.ErrorIs(t, err, ErrCapabilityRequestNotFound)
	assert.NotErrorIs(t, err, ErrCapabilityRequestNotPending)
	assert.NoError(t, mock.ExpectationsWereMet())
}
