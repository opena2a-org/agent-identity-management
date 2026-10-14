package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RevokeFamilyFor is RevokeFamily for a caller that holds a family's id and
// its longest member's lifetime from a stored row (the dashboard's SDK token
// revoke) rather than a presented token: the key is written under the same
// rule, for max(JWT_REFRESH_TTL, lifetime) plus one minute.
func TestFamily_RevokeFamilyForWritesTheKeyUnderTheSameRule(t *testing.T) {
	ctx := context.Background()
	svc := familyTestService(t, "2h")
	store := &familyStore{}
	svc.SetRevoker(NewTokenRevoker(store, false))

	revoked, err := svc.RevokeFamilyFor(ctx, "fam-short", time.Hour)
	require.NoError(t, err)
	assert.True(t, revoked)
	ttl, ok := store.keys["revoked:fam:fam-short"]
	require.True(t, ok, "the key is written under the family's id")
	assert.InDelta(t, (2*time.Hour + time.Minute).Seconds(), ttl.Seconds(), 1, "the configured TTL wins when the lifetime is shorter")

	revoked, err = svc.RevokeFamilyFor(ctx, "fam-sdk", 90*24*time.Hour)
	require.NoError(t, err)
	assert.True(t, revoked)
	ttl, ok = store.keys["revoked:fam:fam-sdk"]
	require.True(t, ok)
	assert.InDelta(t, (90*24*time.Hour + time.Minute).Seconds(), ttl.Seconds(), 1, "the lifetime wins when it is longer")

	t.Run("the same key as RevokeFamily writes for the token", func(t *testing.T) {
		sdk, err := svc.GenerateSDKRefreshToken("user", "org", "for@example.com", "admin")
		require.NoError(t, err)
		sc := validated(t, svc, sdk)
		byClaims := &familyStore{}
		byRow := &familyStore{}
		svc.SetRevoker(NewTokenRevoker(byClaims, false))
		_, err = svc.RevokeFamily(ctx, sc)
		require.NoError(t, err)
		svc.SetRevoker(NewTokenRevoker(byRow, false))
		_, err = svc.RevokeFamilyFor(ctx, sc.FamilyID(), sc.ExpiresAt.Time.Sub(sc.IssuedAt.Time))
		require.NoError(t, err)
		assert.Equal(t, byClaims.keys, byRow.keys)
		svc.SetRevoker(NewTokenRevoker(store, false))
	})
	t.Run("no family: false, nil, no write", func(t *testing.T) {
		before := len(store.keys)
		revoked, err := svc.RevokeFamilyFor(ctx, "", time.Hour)
		assert.NoError(t, err)
		assert.False(t, revoked)
		assert.Len(t, store.keys, before)
	})
	t.Run("no revoker, or no service: false, nil", func(t *testing.T) {
		bare := familyTestService(t, "2h")
		revoked, err := bare.RevokeFamilyFor(ctx, "fam-bare", time.Hour)
		assert.NoError(t, err)
		assert.False(t, revoked)
		var none *JWTService
		revoked, err = none.RevokeFamilyFor(ctx, "fam-none", time.Hour)
		assert.NoError(t, err)
		assert.False(t, revoked)
	})
	t.Run("store refuses the write: false, err", func(t *testing.T) {
		failing := &familyStore{setErr: errors.New("store down")}
		svc2 := familyTestService(t, "2h")
		svc2.SetRevoker(NewTokenRevoker(failing, false))
		revoked, err := svc2.RevokeFamilyFor(ctx, "fam-failing", time.Hour)
		assert.Error(t, err)
		assert.False(t, revoked)
		assert.Empty(t, failing.keys)
	})
}
