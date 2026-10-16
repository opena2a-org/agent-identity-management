package repository

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// GetByParentID reads the row rotation created from a user's row: the row
// whose metadata parent_token names it, scoped to the user, the earliest
// created where more than one does.

var (
	successorReadSQL = regexp.QuoteMeta(`WHERE user_id = $1 AND metadata->>'parent_token' = $2
		ORDER BY created_at ASC
		LIMIT 1`)
	successorColumns = []string{
		"id", "user_id", "organization_id", "token_hash", "token_id",
		"device_name", "device_fingerprint", "ip_address", "user_agent",
		"last_used_at", "last_ip_address", "usage_count",
		"created_at", "expires_at", "revoked_at", "revoke_reason", "metadata",
	}
)

func TestSDKTokenRepository_GetByParentIDReadsTheRowRotatedFromTheParent(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	userID, orgID, parent, next := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	now := time.Now()
	mock.ExpectQuery(successorReadSQL).
		WithArgs(userID, parent.String()).
		WillReturnRows(sqlmock.NewRows(successorColumns).
			AddRow(next, userID, orgID, "hash-2", "jti-2",
				nil, nil, "203.0.113.7", "aim-sdk-python/1.0",
				nil, nil, 1,
				now, now.Add(time.Hour), nil, nil, []byte(`{"parent_token":"`+parent.String()+`","source":"token_rotation"}`)))

	row, err := NewSDKTokenRepository(db).GetByParentID(userID, parent)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet(), "the read is by user and parent_token")
	assert.Equal(t, next, row.ID)
	require.NotNil(t, row.IPAddress)
	require.NotNil(t, row.UserAgent)
	assert.Equal(t, "203.0.113.7", *row.IPAddress)
	assert.Equal(t, "aim-sdk-python/1.0", *row.UserAgent)
	assert.Equal(t, parent.String(), row.Metadata["parent_token"])
}

func TestSDKTokenRepository_GetByParentIDWithNoSuccessorIsAnError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	userID, parent := uuid.New(), uuid.New()
	mock.ExpectQuery(successorReadSQL).
		WithArgs(userID, parent.String()).
		WillReturnRows(sqlmock.NewRows(successorColumns))

	row, err := NewSDKTokenRepository(db).GetByParentID(userID, parent)
	assert.Nil(t, row)
	assert.EqualError(t, err, "SDK token not found")
	assert.NoError(t, mock.ExpectationsWereMet())
}
