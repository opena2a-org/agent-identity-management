package repository

import (
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// EmailInUseOutsideOrganization compares the email without regard to case,
// counts the users of every organization but the one named, and counts a
// soft-deleted user, whose row still holds the email: the query has no
// deleted_at condition.
func TestUserRepository_EmailInUseOutsideOrganization(t *testing.T) {
	query := `SELECT EXISTS \(\s*SELECT 1 FROM users\s+WHERE LOWER\(email\) = LOWER\(\$1\)\s+AND organization_id <> \$2\s*\)`
	org := uuid.New()

	for _, inUse := range []bool{true, false} {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		mock.ExpectQuery(query).WithArgs("dave@example.com", org).
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(inUse))

		got, err := NewUserRepository(db).EmailInUseOutsideOrganization("dave@example.com", org)
		require.NoError(t, err)
		assert.Equal(t, inUse, got)
		assert.NoError(t, mock.ExpectationsWereMet())
		_ = db.Close()
	}

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	failure := errors.New("connection refused")
	mock.ExpectQuery(query).WithArgs("dave@example.com", org).WillReturnError(failure)
	got, err := NewUserRepository(db).EmailInUseOutsideOrganization("dave@example.com", org)
	assert.ErrorIs(t, err, failure)
	assert.False(t, got)
	assert.NoError(t, mock.ExpectationsWereMet())
}
