package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RevokeTokenChecked reports whether the token's jti was written to the
// denylist, so a caller can tell a revocation from a no-op. RevokeToken keeps
// its best-effort signature on top of it.
func TestJWTService_RevokeTokenChecked(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-only-jwt-secret-not-a-real-value-0123456789")
	ctx := context.Background()

	t.Run("no revoker configured is a no-op reported as not revoked", func(t *testing.T) {
		svc := NewJWTService()
		access, _, err := svc.GenerateTokenPair("u", "o", "dev@example.com", "admin")
		require.NoError(t, err)
		revoked, err := svc.RevokeTokenChecked(ctx, access)
		assert.NoError(t, err)
		assert.False(t, revoked)
	})

	t.Run("an invalid token is reported as not revoked", func(t *testing.T) {
		svc := NewJWTService()
		svc.SetRevoker(NewTokenRevoker(&fakeRevocationStore{data: map[string]bool{}}, false))
		revoked, err := svc.RevokeTokenChecked(ctx, "not.a.jwt")
		assert.NoError(t, err)
		assert.False(t, revoked)
	})

	t.Run("a store error is reported", func(t *testing.T) {
		svc := NewJWTService()
		svc.SetRevoker(NewTokenRevoker(&fakeRevocationStore{data: map[string]bool{}, failErr: errors.New("store down")}, false))
		access, _, err := svc.GenerateTokenPair("u", "o", "dev@example.com", "admin")
		require.NoError(t, err)
		revoked, err := svc.RevokeTokenChecked(ctx, access)
		assert.Error(t, err)
		assert.False(t, revoked)
	})

	t.Run("a written jti is reported as revoked and reads back revoked", func(t *testing.T) {
		svc := NewJWTService()
		svc.SetRevoker(NewTokenRevoker(&fakeRevocationStore{data: map[string]bool{}}, false))
		access, _, err := svc.GenerateTokenPair("u", "o", "dev@example.com", "admin")
		require.NoError(t, err)
		revoked, err := svc.RevokeTokenChecked(ctx, access)
		assert.NoError(t, err)
		assert.True(t, revoked)
		id, err := svc.GetTokenID(access)
		require.NoError(t, err)
		assert.True(t, svc.IsRevoked(ctx, id))
	})
}
