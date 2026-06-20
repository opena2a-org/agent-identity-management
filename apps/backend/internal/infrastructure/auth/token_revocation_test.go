package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRevocationStore struct {
	data    map[string]bool
	failErr error
}

func (f *fakeRevocationStore) Exists(ctx context.Context, key string) (bool, error) {
	if f.failErr != nil {
		return false, f.failErr
	}
	return f.data[key], nil
}

func (f *fakeRevocationStore) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if f.failErr != nil {
		return f.failErr
	}
	if f.data == nil {
		f.data = map[string]bool{}
	}
	f.data[key] = true
	return nil
}

func TestTokenRevoker_RevokeThenRevoked(t *testing.T) {
	store := &fakeRevocationStore{data: map[string]bool{}}
	r := NewTokenRevoker(store, false)
	ctx := context.Background()

	assert.False(t, r.IsRevoked(ctx, "jti-1"), "fresh jti not revoked")
	require.NoError(t, r.Revoke(ctx, "jti-1", time.Minute))
	assert.True(t, r.IsRevoked(ctx, "jti-1"), "jti revoked after Revoke")
}

func TestTokenRevoker_NilAndNoStoreDisabled(t *testing.T) {
	ctx := context.Background()
	var nilRevoker *TokenRevoker
	assert.False(t, nilRevoker.IsRevoked(ctx, "x"), "nil revoker disables revocation")
	assert.NoError(t, nilRevoker.Revoke(ctx, "x", time.Minute))

	noStore := NewTokenRevoker(nil, false)
	assert.False(t, noStore.IsRevoked(ctx, "x"), "no store disables revocation")
}

func TestTokenRevoker_EmptyJTIIsNoOp(t *testing.T) {
	store := &fakeRevocationStore{data: map[string]bool{}}
	r := NewTokenRevoker(store, false) // fail-closed
	ctx := context.Background()

	// An empty jti must never be denylisted (would collide on key prefix) and
	// must not be treated as revoked even under fail-closed.
	require.NoError(t, r.Revoke(ctx, "", time.Minute))
	assert.False(t, r.IsRevoked(ctx, ""), "empty jti is a no-op, never revoked")
	_, stored := store.data[revokedKeyPrefix]
	assert.False(t, stored, "empty jti must not write a denylist key")
}

func TestTokenRevoker_FailClosedOnStoreError(t *testing.T) {
	store := &fakeRevocationStore{failErr: errors.New("redis down")}
	r := NewTokenRevoker(store, false) // fail-closed
	assert.True(t, r.IsRevoked(context.Background(), "jti-new"), "store error => fail closed (revoked)")
}

func TestTokenRevoker_FailOpenOnStoreError(t *testing.T) {
	store := &fakeRevocationStore{failErr: errors.New("redis down")}
	r := NewTokenRevoker(store, true) // fail-open
	assert.False(t, r.IsRevoked(context.Background(), "jti-new"), "store error => fail open (allowed)")
}

func TestTokenRevoker_NegativeCacheSurvivesOutage(t *testing.T) {
	store := &fakeRevocationStore{data: map[string]bool{}}
	r := NewTokenRevoker(store, false) // fail-closed
	ctx := context.Background()

	// First check populates the negative cache (not revoked).
	assert.False(t, r.IsRevoked(ctx, "jti-cached"))

	// Store now errors. A fail-closed revoker would normally reject, but the
	// cached "not revoked" result must still be served within the cache window,
	// bounding the blast radius of an outage.
	store.failErr = errors.New("redis down")
	assert.False(t, r.IsRevoked(ctx, "jti-cached"), "cached jti served despite outage")

	// An uncached jti still fails closed during the outage.
	assert.True(t, r.IsRevoked(ctx, "jti-uncached"), "uncached jti fails closed")
}
