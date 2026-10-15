package auth

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// C6: the auth middleware refuses an access token while its family's key
// exists, so the key must outlive every access token of the family. With
// JWT_ACCESS_TTL set above JWT_REFRESH_TTL the key is written for at least
// JWT_ACCESS_TTL plus a minute, whichever entry point writes it.
func TestFamily_KeyOutlivesTheFamilysAccessTokens(t *testing.T) {
	t.Setenv("JWT_ACCESS_TTL", "5h")
	svc := familyTestService(t, "1h")
	store := &familyStore{}
	svc.SetRevoker(NewTokenRevoker(store, false))
	ctx := context.Background()
	floor := 5*time.Hour + time.Minute

	_, refresh, err := svc.GenerateTokenPair(uuid.New().String(), uuid.New().String(), "c6@example.com", "admin")
	require.NoError(t, err)
	claims := validated(t, svc, refresh)
	written, err := svc.RevokeFamily(ctx, claims)
	require.NoError(t, err)
	require.True(t, written)
	ttl, ok := store.keys["revoked:fam:"+claims.FamilyID()]
	require.True(t, ok)
	assert.GreaterOrEqual(t, ttl, floor, "RevokeFamily: a login family's key outlives its access tokens")

	written, err = svc.RevokeFamilyFor(ctx, "fam-row", 30*time.Minute)
	require.NoError(t, err)
	require.True(t, written)
	assert.GreaterOrEqual(t, store.keys["revoked:fam:fam-row"], floor, "RevokeFamilyFor: the same floor")

	t.Run("control: a longer refresh lifetime still wins", func(t *testing.T) {
		t.Setenv("JWT_ACCESS_TTL", "2h")
		long := familyTestService(t, "168h")
		longStore := &familyStore{}
		long.SetRevoker(NewTokenRevoker(longStore, false))
		_, err := long.RevokeFamilyFor(ctx, "fam-long", time.Hour)
		require.NoError(t, err)
		assert.InDelta(t, (168*time.Hour + time.Minute).Seconds(), longStore.keys["revoked:fam:fam-long"].Seconds(), 1)
	})
}
