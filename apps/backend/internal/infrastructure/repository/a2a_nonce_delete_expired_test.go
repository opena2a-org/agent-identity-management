package repository

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The purge keeps a nonce until expires_at + skew and used_at + 2*skew have
// both passed. A statement that deletes at expires_at alone frees a nonce
// while a request carrying it can still pass the timestamp check, and fails
// these expectations. The rows themselves are planted against a real Postgres
// in cmd/server/nonce_cleanup_integration_test.go.

var deleteExpiredNonces = `^` + regexp.QuoteMeta(`DELETE FROM a2a_request_nonces
		WHERE expires_at < NOW() - make_interval(secs => $1)
		  AND used_at < NOW() - make_interval(secs => $2)`) + `$`

func nonceMock(t *testing.T) (*A2ARequestNonceRepository, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	t.Cleanup(func() { assert.NoError(t, mock.ExpectationsWereMet()) })
	return NewA2ARequestNonceRepository(db), mock
}

func TestA2ARequestNonceDeleteExpired_KeepsRowsForTheSkewBound(t *testing.T) {
	repo, mock := nonceMock(t)
	mock.ExpectExec(deleteExpiredNonces).
		WithArgs(float64(300), float64(600)).
		WillReturnResult(sqlmock.NewResult(0, 3))

	deleted, err := repo.DeleteExpired(context.Background(), 5*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, 3, deleted)
}

func TestA2ARequestNonceDeleteExpired_ReturnsTheStatementError(t *testing.T) {
	repo, mock := nonceMock(t)
	mock.ExpectExec(deleteExpiredNonces).
		WithArgs(float64(300), float64(600)).
		WillReturnError(errors.New("connection reset"))

	deleted, err := repo.DeleteExpired(context.Background(), 5*time.Minute)
	require.ErrorContains(t, err, "connection reset")
	assert.Zero(t, deleted)
}
