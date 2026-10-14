package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The dashboard's revoke routes end a row's token family after revoking the
// row. The service gives them the row (or rows) it revoked and the family a
// row belongs to: the token ID of the download row its rotation lineage grew
// from.

func familyRow(userID uuid.UUID, tokenID string, parent *domain.SDKToken) *domain.SDKToken {
	row := &domain.SDKToken{
		ID:        uuid.New(),
		UserID:    userID,
		TokenHash: "hash-" + tokenID,
		TokenID:   tokenID,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(90 * 24 * time.Hour),
		Metadata:  map[string]interface{}{"source": "sdk_download"},
	}
	if parent != nil {
		row.Metadata = map[string]interface{}{
			"source":       "token_rotation",
			"parent_token": parent.ID.String(),
		}
	}
	return row
}

func TestSDKTokenService_FamilyOfWalksTheLineageToTheDownloadRow(t *testing.T) {
	repo := NewMockSDKTokenRepository()
	service := NewSDKTokenService(repo)
	ctx := context.Background()
	userID := uuid.New()

	root := familyRow(userID, "family-jti", nil)
	child := familyRow(userID, "child-jti", root)
	grandchild := familyRow(userID, "grandchild-jti", child)
	for _, row := range []*domain.SDKToken{root, child, grandchild} {
		require.NoError(t, repo.Create(row))
	}

	for name, row := range map[string]*domain.SDKToken{"the download row": root, "its child": child, "its grandchild": grandchild} {
		family, err := service.FamilyOf(ctx, row)
		require.NoError(t, err, name)
		assert.Equal(t, "family-jti", family, "%s: the family is the download row's token ID", name)
	}

	t.Run("a parent the repository no longer holds", func(t *testing.T) {
		orphan := familyRow(userID, "orphan-jti", familyRow(userID, "gone-jti", nil))
		_, err := service.FamilyOf(ctx, orphan)
		assert.ErrorIs(t, err, ErrSDKTokenFamilyUnknown)
	})
	t.Run("a parent id that is not a uuid", func(t *testing.T) {
		row := familyRow(userID, "bad-parent-jti", nil)
		row.Metadata["parent_token"] = "not-a-uuid"
		_, err := service.FamilyOf(ctx, row)
		assert.ErrorIs(t, err, ErrSDKTokenFamilyUnknown)
	})
	t.Run("a lineage that loops", func(t *testing.T) {
		a := familyRow(userID, "loop-a", nil)
		b := familyRow(userID, "loop-b", a)
		a.Metadata["parent_token"] = b.ID.String()
		require.NoError(t, repo.Create(a))
		require.NoError(t, repo.Create(b))
		_, err := service.FamilyOf(ctx, b)
		assert.ErrorIs(t, err, ErrSDKTokenFamilyUnknown)
	})
	t.Run("a root row with no token ID, or no row", func(t *testing.T) {
		row := familyRow(userID, "", nil)
		_, err := service.FamilyOf(ctx, row)
		assert.ErrorIs(t, err, ErrSDKTokenFamilyUnknown)
		_, err = service.FamilyOf(ctx, nil)
		assert.ErrorIs(t, err, ErrSDKTokenFamilyUnknown)
	})
}

func TestSDKTokenService_RevokeTokenRowReportsTheRowItRevoked(t *testing.T) {
	repo := NewMockSDKTokenRepository()
	service := NewSDKTokenService(repo)
	ctx := context.Background()
	userID := uuid.New()
	row := familyRow(userID, "family-jti", nil)
	require.NoError(t, repo.Create(row))

	got, err := service.RevokeTokenRow(ctx, row.ID, userID, "user revoked")
	require.NoError(t, err)
	assert.Equal(t, row.ID, got.ID)
	assert.Equal(t, "family-jti", got.TokenID)
	assert.NotNil(t, repo.tokens[row.ID].RevokedAt, "the row is revoked")

	t.Run("another user's row, or no row: ErrSDKTokenNotFound and no row", func(t *testing.T) {
		other := familyRow(uuid.New(), "other-jti", nil)
		require.NoError(t, repo.Create(other))
		got, err := service.RevokeTokenRow(ctx, other.ID, userID, "user revoked")
		assert.ErrorIs(t, err, ErrSDKTokenNotFound)
		assert.Nil(t, got)
		assert.Nil(t, repo.tokens[other.ID].RevokedAt)
		got, err = service.RevokeTokenRow(ctx, uuid.New(), userID, "user revoked")
		assert.ErrorIs(t, err, ErrSDKTokenNotFound)
		assert.Nil(t, got)
	})
}

func TestSDKTokenService_RevokeAllUserTokenRowsReportsTheUsersActiveRows(t *testing.T) {
	repo := NewMockSDKTokenRepository()
	service := NewSDKTokenService(repo)
	ctx := context.Background()
	userID := uuid.New()

	live1 := familyRow(userID, "live-1", nil)
	live2 := familyRow(userID, "live-2", nil)
	already := familyRow(userID, "already-revoked", nil)
	already.Revoke("earlier")
	other := familyRow(uuid.New(), "other-user", nil)
	for _, row := range []*domain.SDKToken{live1, live2, already, other} {
		require.NoError(t, repo.Create(row))
	}

	rows, err := service.RevokeAllUserTokenRows(ctx, userID, "revoke all")
	require.NoError(t, err)
	ids := map[string]bool{}
	for _, row := range rows {
		ids[row.TokenID] = true
	}
	assert.Equal(t, map[string]bool{"live-1": true, "live-2": true}, ids, "the user's rows that were active, and no other")
	assert.NotNil(t, repo.tokens[live1.ID].RevokedAt)
	assert.NotNil(t, repo.tokens[live2.ID].RevokedAt)
	assert.Nil(t, repo.tokens[other.ID].RevokedAt, "another user's row is untouched")
}
