package repository

import (
	"context"
	"database/sql/driver"
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

// users.password_changed_at records the instant the password last changed.
// The auth middleware reads it, with force_password_change, through
// GetSessionState on every request, and Update only ever moves it forward.

func TestUserRepository_GetSessionState(t *testing.T) {
	query := regexp.QuoteMeta(`SELECT password_changed_at, COALESCE(force_password_change, FALSE) FROM users WHERE id = $1`)
	id := uuid.New()

	t.Run("reads the change and the flag", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()
		changedAt := time.Date(2026, 10, 18, 12, 0, 0, 400_000_000, time.UTC)
		mock.ExpectQuery(query).WithArgs(id).
			WillReturnRows(sqlmock.NewRows([]string{"password_changed_at", "force_password_change"}).AddRow(changedAt, true))

		state, err := NewUserRepository(db).GetSessionState(context.Background(), id)
		require.NoError(t, err)
		require.NotNil(t, state.PasswordChangedAt)
		assert.True(t, changedAt.Equal(*state.PasswordChangedAt))
		assert.True(t, state.ForcePasswordChange)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("a password that has not changed reads as nil", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()
		mock.ExpectQuery(query).WithArgs(id).
			WillReturnRows(sqlmock.NewRows([]string{"password_changed_at", "force_password_change"}).AddRow(nil, false))

		state, err := NewUserRepository(db).GetSessionState(context.Background(), id)
		require.NoError(t, err)
		assert.Nil(t, state.PasswordChangedAt)
		assert.False(t, state.ForcePasswordChange)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("no such user", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()
		mock.ExpectQuery(query).WithArgs(id).
			WillReturnRows(sqlmock.NewRows([]string{"password_changed_at", "force_password_change"}))

		_, err = NewUserRepository(db).GetSessionState(context.Background(), id)
		assert.True(t, errors.Is(err, domain.ErrUserNotFound), "got %v", err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("a failed read is not a missing user", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()
		mock.ExpectQuery(query).WithArgs(id).WillReturnError(errors.New("connection refused"))

		_, err = NewUserRepository(db).GetSessionState(context.Background(), id)
		require.Error(t, err)
		assert.False(t, errors.Is(err, domain.ErrUserNotFound))
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// Update writes the user's password_changed_at through GREATEST, so a copy of
// the row that carries no change (nil) or an older one leaves the recorded
// change in place.
func TestUserRepository_Update_OnlyMovesThePasswordChangeForward(t *testing.T) {
	update := regexp.QuoteMeta(`password_changed_at = GREATEST(password_changed_at, $14)`) + `\s+WHERE id = \$13`
	changedAt := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)

	for name, value := range map[string]*time.Time{"a recorded change": &changedAt, "no change carried": nil} {
		t.Run(name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			user := &domain.User{ID: uuid.New(), Name: "n", Role: domain.RoleMember, Status: domain.UserStatusActive, PasswordChangedAt: value}
			args := make([]driver.Value, 14)
			for i := range args {
				args[i] = sqlmock.AnyArg()
			}
			args[12] = user.ID
			if value != nil {
				args[13] = *value
			} else {
				args[13] = nil
			}
			mock.ExpectExec(update).WithArgs(args...).WillReturnResult(sqlmock.NewResult(0, 1))

			require.NoError(t, NewUserRepository(db).Update(user))
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
