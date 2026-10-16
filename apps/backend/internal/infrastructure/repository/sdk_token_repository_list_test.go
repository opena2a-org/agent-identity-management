package repository

import (
	"errors"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// GetByUserID and GetByOrganizationID list SDK token rows. A read that fails
// after the first rows gives its error: the rows read before the failure are
// not returned as the whole list, so a reader that decides from the list (a
// token family's end, for one) never decides from part of it.

var (
	tokenListSQL     = regexp.QuoteMeta(`FROM sdk_tokens`)
	tokenListColumns = []string{
		"id", "user_id", "organization_id", "token_hash", "token_id",
		"device_name", "device_fingerprint", "ip_address", "user_agent",
		"last_used_at", "last_ip_address", "usage_count",
		"created_at", "expires_at", "revoked_at", "revoke_reason", "metadata",
	}
)

// tokenListRows holds a live row and, after it, a row a logout revoked.
func tokenListRows(userID, organizationID uuid.UUID) *sqlmock.Rows {
	now := time.Now()
	return sqlmock.NewRows(tokenListColumns).
		AddRow(uuid.New().String(), userID.String(), organizationID.String(), "not-a-real-hash-1", "jti-1",
			nil, nil, nil, nil, nil, nil, 0, now, now.Add(time.Hour), nil, nil, []byte(`{}`)).
		AddRow(uuid.New().String(), userID.String(), organizationID.String(), "not-a-real-hash-2", "jti-2",
			nil, nil, nil, nil, nil, nil, 0, now, now.Add(time.Hour), now, "logout", []byte(`{}`))
}

func TestSDKTokenRepository_ListsGiveTheErrorOfAReadThatFailsAfterTheFirstRow(t *testing.T) {
	userID, organizationID := uuid.New(), uuid.New()
	readErr := errors.New("read: connection reset by peer")

	for _, includeRevoked := range []bool{true, false} {
		t.Run(fmt.Sprintf("GetByUserID/includeRevoked=%t", includeRevoked), func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			mock.ExpectQuery(tokenListSQL).
				WithArgs(userID).
				WillReturnRows(tokenListRows(userID, organizationID).RowError(1, readErr))

			tokens, err := NewSDKTokenRepository(db).GetByUserID(userID, includeRevoked)
			assert.ErrorIs(t, err, readErr)
			assert.Nil(t, tokens, "the row read before the failure is not returned as the list")
			assert.NoError(t, mock.ExpectationsWereMet())
		})

		t.Run(fmt.Sprintf("GetByOrganizationID/includeRevoked=%t", includeRevoked), func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			mock.ExpectQuery(tokenListSQL).
				WithArgs(organizationID).
				WillReturnRows(tokenListRows(userID, organizationID).RowError(1, readErr))

			tokens, err := NewSDKTokenRepository(db).GetByOrganizationID(organizationID, includeRevoked)
			assert.ErrorIs(t, err, readErr)
			assert.Nil(t, tokens, "the row read before the failure is not returned as the list")
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestSDKTokenRepository_ListsReturnEveryRowOfAReadThatCompletes(t *testing.T) {
	userID, organizationID := uuid.New(), uuid.New()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewSDKTokenRepository(db)

	mock.ExpectQuery(tokenListSQL).
		WithArgs(userID).
		WillReturnRows(tokenListRows(userID, organizationID))
	byUser, err := repo.GetByUserID(userID, true)
	require.NoError(t, err)
	require.Len(t, byUser, 2)
	require.NotNil(t, byUser[1].RevokeReason)
	assert.Equal(t, "logout", *byUser[1].RevokeReason)

	mock.ExpectQuery(tokenListSQL).
		WithArgs(organizationID).
		WillReturnRows(tokenListRows(userID, organizationID))
	byOrganization, err := repo.GetByOrganizationID(organizationID, true)
	require.NoError(t, err)
	require.Len(t, byOrganization, 2)
	require.NotNil(t, byOrganization[1].RevokeReason)
	assert.Equal(t, "logout", *byOrganization[1].RevokeReason)

	assert.NoError(t, mock.ExpectationsWereMet())
}
