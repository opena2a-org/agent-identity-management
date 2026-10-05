package repository

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// Consume must test the status and change it in one conditional UPDATE inside
// the transaction that mints. A read of the status followed by a separate
// UPDATE lets two polls both see approved and both mint; that shape issues a
// SELECT first and fails these expectations. The race itself is driven against
// a real Postgres in device_code_consume_integration_test.go.

var consumeUpdate = regexp.QuoteMeta(`UPDATE device_codes
		SET status = $1
		WHERE device_code = $2 AND status = $3 AND expires_at > $4`)

func consumeMock(t *testing.T) (*DeviceCodeRepository, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return NewDeviceCodeRepository(db), mock
}

// The winning poll: the conditional UPDATE matches, the pair is minted inside
// the transaction, and the transaction commits.
func TestDeviceCodeConsume_ApprovedCode_MintsInsideTheTransactionAndCommits(t *testing.T) {
	repo, mock := consumeMock(t)
	mock.ExpectBegin()
	mock.ExpectExec(consumeUpdate).
		WithArgs(domain.DeviceCodeStatusConsumed, "dev-code", domain.DeviceCodeStatusApproved, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	issued := 0
	err := repo.Consume(context.Background(), "dev-code", func() error { issued++; return nil })
	require.NoError(t, err)
	assert.Equal(t, 1, issued)
	require.NoError(t, mock.ExpectationsWereMet())
}

// A code another poll already consumed (or one that is not approved) matches no
// row: nothing is minted and the transaction is rolled back.
func TestDeviceCodeConsume_CodeNotApproved_MintsNothing(t *testing.T) {
	repo, mock := consumeMock(t)
	mock.ExpectBegin()
	mock.ExpectExec(consumeUpdate).
		WithArgs(domain.DeviceCodeStatusConsumed, "dev-code", domain.DeviceCodeStatusApproved, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	issued := 0
	err := repo.Consume(context.Background(), "dev-code", func() error { issued++; return nil })
	assert.ErrorIs(t, err, domain.ErrDeviceCodeNotApproved)
	assert.Equal(t, 0, issued)
	require.NoError(t, mock.ExpectationsWereMet())
}

// A pair that fails to mint rolls the change back, so the code stays approved
// and the client's next poll can still receive its pair.
func TestDeviceCodeConsume_MintFails_RollsBackAndLeavesTheCodeApproved(t *testing.T) {
	repo, mock := consumeMock(t)
	mock.ExpectBegin()
	mock.ExpectExec(consumeUpdate).
		WithArgs(domain.DeviceCodeStatusConsumed, "dev-code", domain.DeviceCodeStatusApproved, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectRollback()

	mintErr := errors.New("signing unavailable")
	err := repo.Consume(context.Background(), "dev-code", func() error { return mintErr })
	assert.ErrorIs(t, err, mintErr)
	require.NoError(t, mock.ExpectationsWereMet())
}
