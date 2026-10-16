package application

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SDK credential recovery decides whether a token family has ended from the
// family's own rows. The service gives it those rows: the presented row, the
// rows its rotation lineage grew from, and every row that grew from those.

func rowIDs(rows []*domain.SDKToken) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	return ids
}

func TestSDKTokenService_FamilyRowsReadsTheWholeLineage(t *testing.T) {
	repo := NewMockSDKTokenRepository()
	service := NewSDKTokenService(repo)
	ctx := context.Background()
	userID := uuid.New()

	root := familyRow(userID, "family-jti", nil)
	child := familyRow(userID, "child-jti", root)
	grandchild := familyRow(userID, "grandchild-jti", child)
	otherFamily := familyRow(userID, "other-jti", nil)
	otherChild := familyRow(userID, "other-child-jti", otherFamily)
	otherUser := familyRow(uuid.New(), "other-user-jti", nil)
	otherUser.Metadata["parent_token"] = root.ID.String()
	for _, row := range []*domain.SDKToken{root, child, grandchild, otherFamily, otherChild, otherUser} {
		require.NoError(t, repo.Create(row))
	}

	for name, row := range map[string]*domain.SDKToken{"the download row": root, "its child": child, "its grandchild": grandchild} {
		rows, err := service.FamilyRows(ctx, row)
		require.NoError(t, err, name)
		assert.ElementsMatch(t, []uuid.UUID{root.ID, child.ID, grandchild.ID}, rowIDs(rows),
			"%s: the family is the download row and every row rotated from it, no other family's and no other user's", name)
	}
}

func TestSDKTokenService_FamilyRowsEndsTheWalkTowardTheRootAtAGap(t *testing.T) {
	repo := NewMockSDKTokenRepository()
	service := NewSDKTokenService(repo)
	ctx := context.Background()
	userID := uuid.New()

	t.Run("a parent the repository no longer holds", func(t *testing.T) {
		gone := familyRow(userID, "gone-jti", nil)
		orphan := familyRow(userID, "orphan-jti", gone)
		orphanChild := familyRow(userID, "orphan-child-jti", orphan)
		require.NoError(t, repo.Create(orphan))
		require.NoError(t, repo.Create(orphanChild))

		rows, err := service.FamilyRows(ctx, orphanChild)
		require.NoError(t, err)
		assert.ElementsMatch(t, []uuid.UUID{orphan.ID, orphanChild.ID}, rowIDs(rows), "the rows below the gap are read")
	})
	t.Run("a parent id that is not a uuid", func(t *testing.T) {
		row := familyRow(userID, "bad-parent-jti", nil)
		row.Metadata["parent_token"] = "not-a-uuid"
		below := familyRow(userID, "below-bad-parent-jti", row)
		require.NoError(t, repo.Create(row))
		require.NoError(t, repo.Create(below))

		rows, err := service.FamilyRows(ctx, row)
		require.NoError(t, err)
		assert.ElementsMatch(t, []uuid.UUID{row.ID, below.ID}, rowIDs(rows))
	})
	t.Run("a lineage that loops", func(t *testing.T) {
		a := familyRow(userID, "loop-a", nil)
		b := familyRow(userID, "loop-b", a)
		a.Metadata["parent_token"] = b.ID.String()
		require.NoError(t, repo.Create(a))
		require.NoError(t, repo.Create(b))

		rows, err := service.FamilyRows(ctx, b)
		require.NoError(t, err)
		assert.ElementsMatch(t, []uuid.UUID{a.ID, b.ID}, rowIDs(rows), "each row is read once")
	})
	t.Run("a row the repository does not list", func(t *testing.T) {
		unlisted := familyRow(userID, "unlisted-jti", nil)
		rows, err := service.FamilyRows(ctx, unlisted)
		require.NoError(t, err)
		assert.Equal(t, []uuid.UUID{unlisted.ID}, rowIDs(rows), "the presented row is always a member")
	})
	t.Run("a listed parent of a row the repository does not list", func(t *testing.T) {
		parent := familyRow(userID, "listed-parent-jti", nil)
		require.NoError(t, repo.Create(parent))
		unlisted := familyRow(userID, "unlisted-child-jti", parent)

		rows, err := service.FamilyRows(ctx, unlisted)
		require.NoError(t, err)
		assert.ElementsMatch(t, []uuid.UUID{parent.ID, unlisted.ID}, rowIDs(rows))
	})
}

// unlistableSDKTokenRepository is the SDK token mock whose read of a user's
// rows fails.
type unlistableSDKTokenRepository struct{ *MockSDKTokenRepository }

func (r unlistableSDKTokenRepository) GetByUserID(uuid.UUID, bool) ([]*domain.SDKToken, error) {
	return nil, errors.New("connection refused")
}

func TestSDKTokenService_FamilyRowsWithoutARowOrARead(t *testing.T) {
	ctx := context.Background()

	_, err := NewSDKTokenService(NewMockSDKTokenRepository()).FamilyRows(ctx, nil)
	assert.ErrorIs(t, err, ErrSDKTokenFamilyUnknown, "no row")

	service := NewSDKTokenService(unlistableSDKTokenRepository{NewMockSDKTokenRepository()})
	rows, err := service.FamilyRows(ctx, familyRow(uuid.New(), "family-jti", nil))
	assert.EqualError(t, err, "connection refused", "a failed read gives its error")
	assert.Nil(t, rows)
}
