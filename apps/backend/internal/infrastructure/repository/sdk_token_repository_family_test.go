package repository

import (
	"database/sql/driver"
	"regexp"
	"sort"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RevokeFamily finds an SDK-download token family from the user's rows: the
// download's row, whose token_id is the family's id, and every row that
// descends from it by metadata parent_token. Only the family's active rows
// are revoked, in one statement.

var (
	familyReadSQL   = regexp.QuoteMeta(`FROM sdk_tokens`)
	familyRevokeSQL = regexp.QuoteMeta(`UPDATE sdk_tokens`)
	familyColumns   = []string{"id", "token_id", "parent_token", "active"}
)

// idSet matches the uuid[] argument of the revoking UPDATE in any order.
type idSet []uuid.UUID

func (s idSet) Match(v driver.Value) bool {
	var got pq.StringArray
	if err := got.Scan(v); err != nil {
		return false
	}
	want := make([]string, len(s))
	for i, id := range s {
		want[i] = id.String()
	}
	sort.Strings(got)
	sort.Strings(want)
	return assert.ObjectsAreEqual(want, []string(got))
}

func TestSDKTokenRepository_RevokeFamilyRevokesTheActiveRowsThatDescendFromTheDownload(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	userID := uuid.New()
	download, rotated, tip, sibling := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	otherDownload, otherTip := uuid.New(), uuid.New()
	mock.ExpectQuery(familyReadSQL).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows(familyColumns).
			AddRow(download, "family-jti", "", false).
			AddRow(rotated, "jti-2", download.String(), false).
			AddRow(tip, "jti-3", rotated.String(), true).
			// a second row rotated from the same parent
			AddRow(sibling, "jti-3b", rotated.String(), true).
			// another download of the same user and the row rotated from it
			AddRow(otherDownload, "other-family-jti", "", true).
			AddRow(otherTip, "other-jti-2", otherDownload.String(), true))
	mock.ExpectExec(familyRevokeSQL).
		WithArgs(sqlmock.AnyArg(), "token_family_revoked", userID, idSet{tip, sibling}).
		WillReturnResult(sqlmock.NewResult(0, 2))

	require.NoError(t, NewSDKTokenRepository(db).RevokeFamily(userID, "family-jti", "token_family_revoked"))
	assert.NoError(t, mock.ExpectationsWereMet(), "only the family's active rows are revoked")
}

func TestSDKTokenRepository_RevokeFamilyWithNoActiveRowWritesNothing(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	userID := uuid.New()
	download, rotated := uuid.New(), uuid.New()
	mock.ExpectQuery(familyReadSQL).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows(familyColumns).
			AddRow(download, "family-jti", "", false).
			AddRow(rotated, "jti-2", download.String(), false))

	require.NoError(t, NewSDKTokenRepository(db).RevokeFamily(userID, "family-jti", "token_family_revoked"))
	assert.NoError(t, mock.ExpectationsWereMet(), "no UPDATE is issued")

	// An id that names no download of the user reaches no row either.
	mock.ExpectQuery(familyReadSQL).
		WithArgs(userID).
		WillReturnRows(sqlmock.NewRows(familyColumns).
			AddRow(uuid.New(), "other-family-jti", "", true))
	require.NoError(t, NewSDKTokenRepository(db).RevokeFamily(userID, "family-jti", "token_family_revoked"))
	assert.NoError(t, mock.ExpectationsWereMet())
}
