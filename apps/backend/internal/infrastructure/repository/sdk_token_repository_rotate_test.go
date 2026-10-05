package repository

import (
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Rotate retires an SDK refresh token's row and stores its successor's row in
// one transaction: the revocation and the insert both commit or both roll
// back, so a rotation never leaves the old token live beside the new one, nor
// the new token without a row.

var (
	rotateRevokeSQL = regexp.QuoteMeta(`UPDATE sdk_tokens`)
	rotateInsertSQL = regexp.QuoteMeta(`INSERT INTO sdk_tokens`)
)

func rotateSuccessor() *domain.SDKToken {
	now := time.Now()
	return &domain.SDKToken{
		ID:             uuid.New(),
		UserID:         uuid.New(),
		OrganizationID: uuid.New(),
		TokenHash:      "new-hash",
		TokenID:        "new-jti",
		CreatedAt:      now,
		ExpiresAt:      now.Add(time.Hour),
		Metadata:       map[string]interface{}{"source": "token_rotation"},
	}
}

func TestSDKTokenRepository_RotateCommitsRevocationAndInsertTogether(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	next := rotateSuccessor()

	mock.ExpectBegin()
	mock.ExpectExec(rotateRevokeSQL).
		WithArgs(sqlmock.AnyArg(), "token_rotation", "old-hash").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(rotateInsertSQL).
		WithArgs(next.ID, next.UserID, next.OrganizationID, "new-hash", "new-jti",
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow(next.ID, next.CreatedAt))
	mock.ExpectCommit()

	require.NoError(t, NewSDKTokenRepository(db).Rotate("old-hash", "token_rotation", next))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSDKTokenRepository_RotateRollsBackWhenTheOldRowIsNotActive(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec(rotateRevokeSQL).
		WithArgs(sqlmock.AnyArg(), "token_rotation", "old-hash").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	err = NewSDKTokenRepository(db).Rotate("old-hash", "token_rotation", rotateSuccessor())
	assert.EqualError(t, err, "SDK token not found or already revoked")
	assert.NoError(t, mock.ExpectationsWereMet(), "no row is inserted for a token whose row was not revoked")
}

func TestSDKTokenRepository_RotateRollsBackTheRevocationWhenTheInsertFails(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec(rotateRevokeSQL).
		WithArgs(sqlmock.AnyArg(), "token_rotation", "old-hash").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(rotateInsertSQL).WillReturnError(errors.New("insert refused"))
	mock.ExpectRollback()

	err = NewSDKTokenRepository(db).Rotate("old-hash", "token_rotation", rotateSuccessor())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insert refused")
	assert.NoError(t, mock.ExpectationsWereMet(), "the old row's revocation is rolled back")
}

func TestSDKTokenRepository_RotateReportsAFailedCommit(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	next := rotateSuccessor()

	mock.ExpectBegin()
	mock.ExpectExec(rotateRevokeSQL).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(rotateInsertSQL).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow(next.ID, next.CreatedAt))
	mock.ExpectCommit().WillReturnError(errors.New("connection lost"))

	err = NewSDKTokenRepository(db).Rotate("old-hash", "token_rotation", next)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "connection lost")
	assert.NoError(t, mock.ExpectationsWereMet())
}
