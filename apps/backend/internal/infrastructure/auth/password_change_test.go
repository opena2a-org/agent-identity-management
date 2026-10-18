package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// A password change recorded at an instant refuses every token whose sign-in
// time is before the first whole second after that instant: the token's
// auth_time, or its own iat when it carries none.

func TestPasswordChangeBoundary_IsTheFirstWholeSecondAfterTheChange(t *testing.T) {
	base := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	assert.Equal(t, base.Add(time.Second), PasswordChangeBoundary(base), "a change on a whole second")
	assert.Equal(t, base.Add(time.Second), PasswordChangeBoundary(base.Add(700*time.Millisecond)))
	assert.Equal(t, base.Add(time.Second), PasswordChangeBoundary(base.Add(999999*time.Microsecond)))
}

func TestSignedInBeforePasswordChange(t *testing.T) {
	changedAt := time.Date(2026, 10, 18, 12, 0, 0, 400_000_000, time.UTC)
	second := changedAt.Truncate(time.Second)
	at := func(t time.Time) *jwt.NumericDate { return jwt.NewNumericDate(t) }

	cases := []struct {
		name   string
		claims *JWTClaims
		want   bool
	}{
		{"auth_time a second before the change", &JWTClaims{AuthTime: at(second.Add(-time.Second))}, true},
		{"auth_time in the second of the change", &JWTClaims{AuthTime: at(second)}, true},
		{"auth_time at the boundary", &JWTClaims{AuthTime: at(second.Add(time.Second))}, false},
		{"auth_time after the boundary, iat before it", &JWTClaims{AuthTime: at(second.Add(2 * time.Second)), RegisteredClaims: jwt.RegisteredClaims{IssuedAt: at(second.Add(-time.Hour))}}, false},
		{"auth_time before the change, iat after it (a rotated login token)", &JWTClaims{AuthTime: at(second.Add(-time.Hour)), RegisteredClaims: jwt.RegisteredClaims{IssuedAt: at(second.Add(time.Hour))}}, true},
		{"no auth_time, iat in the second of the change (an SDK-download token)", &JWTClaims{RegisteredClaims: jwt.RegisteredClaims{IssuedAt: at(second)}}, true},
		{"no auth_time, iat at the boundary", &JWTClaims{RegisteredClaims: jwt.RegisteredClaims{IssuedAt: at(second.Add(time.Second))}}, false},
		{"neither claim", &JWTClaims{}, true},
		{"no claims at all", nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, SignedInBeforePasswordChange(tc.claims, &changedAt))
		})
	}

	t.Run("an account whose password has not changed refuses nothing", func(t *testing.T) {
		assert.False(t, SignedInBeforePasswordChange(&JWTClaims{}, nil))
		assert.False(t, SignedInBeforePasswordChange(nil, nil))
	})
}

// A pair minted for an account whose password changed in this second signs
// in at the change's boundary, so the change does not refuse it; a pair
// minted long after a change signs in now; with no change it is the pair
// GenerateTokenPair mints.
func TestGenerateTokenPairSince(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-key-for-testing-purposes-32chars")
	svc := NewJWTService()
	userID, orgID := uuid.New().String(), uuid.New().String()

	justNow := time.Now()
	access, refresh, err := svc.GenerateTokenPairSince(userID, orgID, "pc@example.com", "admin", &justNow)
	require.NoError(t, err)
	accessClaims, err := svc.ValidateToken(access)
	require.NoError(t, err)
	refreshClaims, err := svc.ValidateToken(refresh)
	require.NoError(t, err)
	boundary := PasswordChangeBoundary(justNow)
	require.NotNil(t, refreshClaims.AuthTime)
	assert.False(t, refreshClaims.AuthTime.Before(boundary), "the refresh token's sign-in is not before the boundary")
	assert.Equal(t, refreshClaims.AuthTime.Unix(), accessClaims.SignedInAt().Unix(), "the access token carries the pair's sign-in")
	assert.Equal(t, refreshClaims.FamilyID(), accessClaims.AccessFamilyID(), "one new family")
	assert.False(t, SignedInBeforePasswordChange(accessClaims, &justNow))
	assert.False(t, SignedInBeforePasswordChange(refreshClaims, &justNow))
	assert.False(t, svc.SessionExpired(refreshClaims, time.Now()), "a sign-in time up to a second ahead is not an expired session")
	assert.False(t, svc.AccessSessionExpired(accessClaims, time.Now()))

	longAgo := time.Now().Add(-time.Hour)
	_, refresh, err = svc.GenerateTokenPairSince(userID, orgID, "pc@example.com", "admin", &longAgo)
	require.NoError(t, err)
	refreshClaims, err = svc.ValidateToken(refresh)
	require.NoError(t, err)
	assert.Equal(t, refreshClaims.IssuedAt.Unix(), refreshClaims.AuthTime.Unix(), "a change long ago: the pair signs in now")

	_, refresh, err = svc.GenerateTokenPairSince(userID, orgID, "pc@example.com", "admin", nil)
	require.NoError(t, err)
	refreshClaims, err = svc.ValidateToken(refresh)
	require.NoError(t, err)
	assert.Equal(t, refreshClaims.IssuedAt.Unix(), refreshClaims.AuthTime.Unix(), "no change: the pair signs in now")
}

type sessionStateStub struct {
	state *domain.UserSessionState
	err   error
}

func (s sessionStateStub) GetSessionState(context.Context, uuid.UUID) (*domain.UserSessionState, error) {
	return s.state, s.err
}

func TestJWTService_SessionState(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-key-for-testing-purposes-32chars")
	svc := NewJWTService()
	state, attached, err := svc.SessionState(context.Background(), uuid.New())
	assert.False(t, attached, "no reader attached")
	assert.Nil(t, state)
	assert.NoError(t, err)

	want := &domain.UserSessionState{ForcePasswordChange: true}
	svc.SetSessionStateReader(sessionStateStub{state: want})
	state, attached, err = svc.SessionState(context.Background(), uuid.New())
	assert.True(t, attached)
	assert.Same(t, want, state)
	assert.NoError(t, err)

	svc.SetSessionStateReader(sessionStateStub{err: domain.ErrUserNotFound})
	_, attached, err = svc.SessionState(context.Background(), uuid.New())
	assert.True(t, attached)
	assert.True(t, errors.Is(err, domain.ErrUserNotFound))
}
