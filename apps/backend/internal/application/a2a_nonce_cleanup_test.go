package application

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
)

// The cleanup keeps each nonce for the signature timestamp tolerance past its
// expiry, the same bound VerifyA2ARequest accepts a timestamp within. The
// scheduled job and the admin maintenance route both reach the table through
// this call, so both purge with that bound.
func TestCleanupExpiredNonces_KeepsNoncesForTheTimestampTolerance(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	tolerance := float64(DefaultSignatureTimestampToleranceSeconds)
	mock.ExpectExec(`DELETE FROM a2a_request_nonces`).
		WithArgs(tolerance, 2*tolerance).
		WillReturnResult(sqlmock.NewResult(0, 4))

	svc := &A2AService{nonceRepo: repository.NewA2ARequestNonceRepository(db)}
	deleted, err := svc.CleanupExpiredNonces(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 4, deleted)
	require.NoError(t, mock.ExpectationsWereMet())
}

// The job runs once a minute; the interval is one constant.
func TestNonceCleanupInterval_IsOneMinute(t *testing.T) {
	assert.Equal(t, "1m0s", NonceCleanupInterval.String())
}
