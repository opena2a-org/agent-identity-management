package repository

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Only a unique violation of the one-open-token index is reported as an
// open-token conflict, the one error a retry of the mint can resolve.
func TestBootstrapTokenRepository_CreateReplacingOpenReportsOnlyTheOpenTokenConflict(t *testing.T) {
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	newToken := func() *domain.BootstrapToken {
		return &domain.BootstrapToken{
			ID: uuid.New(), OrganizationID: uuid.New(), CreatedBy: uuid.New(),
			TokenHash: "hash", DisplayPrefix: "abcdefgh", Scope: domain.BootstrapTokenScopeAgentsRegister,
			CreatedAt: now, ExpiresAt: now.Add(domain.BootstrapTokenTTL),
		}
	}

	cases := []struct {
		name         string
		insertErr    error
		wantConflict bool
	}{
		{"open-token index violation", &pq.Error{Code: "23505", Constraint: openBootstrapTokenIndex}, true},
		{"token hash collision", &pq.Error{Code: "23505", Constraint: "idx_bootstrap_tokens_token_hash"}, false},
		{"foreign key violation", &pq.Error{Code: "23503", Constraint: openBootstrapTokenIndex}, false},
		{"connection lost", errors.New("driver: bad connection"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			repo := NewBootstrapTokenRepository(db)

			mock.ExpectBegin()
			mock.ExpectExec(regexp.QuoteMeta("UPDATE bootstrap_tokens")).WillReturnResult(sqlmock.NewResult(0, 0))
			mock.ExpectExec(regexp.QuoteMeta("INSERT INTO bootstrap_tokens")).WillReturnError(tc.insertErr)
			mock.ExpectRollback()

			err = repo.CreateReplacingOpen(context.Background(), newToken(), now)
			require.Error(t, err)
			assert.Equal(t, tc.wantConflict, errors.Is(err, domain.ErrBootstrapTokenOpenConflict), "%v", err)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}

	t.Run("database unreachable", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()
		repo := NewBootstrapTokenRepository(db)

		mock.ExpectBegin().WillReturnError(errors.New("dial tcp: connection refused"))

		err = repo.CreateReplacingOpen(context.Background(), newToken(), now)
		require.Error(t, err)
		assert.False(t, errors.Is(err, domain.ErrBootstrapTokenOpenConflict))
		require.NoError(t, mock.ExpectationsWereMet())
	})
}
