package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A re-authenticated pair carries auth_time, the time the user last presented
// their password in that login family: the time of the re-authentication, on
// both its refresh token and its access token. The sign-in pair and a
// refresh's access token are covered by jwt_access_session_age_test.go.

// T1: a re-authenticated pair stays in the presented token's family, and both
// tokens carry auth_time equal to the new refresh token's iat, the time of the
// re-authentication.
func TestAuthTime_ReauthenticatedPairCarriesTheReauthenticationTime(t *testing.T) {
	svc := sessionAgeService(t, "8h")
	now := time.Now()
	signedIn := now.Add(-3 * time.Hour)
	sid := uuid.New().String()
	userID, orgID := uuid.New().String(), uuid.New().String()
	presentedClaims := loginRefreshClaims(userID, orgID, sid, signedIn, now.Add(-time.Hour))
	presented := validated(t, svc, signClaims(t, presentedClaims))

	before := time.Now().Truncate(time.Second)
	access, refresh, err := svc.ReauthenticatedPair(presented, "current@example.com", "member")
	require.NoError(t, err)
	after := time.Now()

	r := validated(t, svc, refresh)
	assert.Equal(t, TokenTypeRefresh, r.TokenType)
	assert.Equal(t, IssuerUser, r.Issuer)
	assert.Equal(t, sid, r.SessionID, "the successor stays in the presented token's family")
	assert.NotEqual(t, presented.ID, r.ID, "the successor is a new token")
	require.NotNil(t, r.AuthTime)
	assert.Equal(t, r.IssuedAt.Unix(), r.AuthTime.Unix(), "auth_time equals the successor's iat")
	assert.False(t, r.AuthTime.Time.Before(before), "auth_time is the re-authentication's, not the sign-in's")
	assert.False(t, r.AuthTime.Time.After(after))
	assert.Empty(t, r.Email, "a login refresh token carries no email")
	assert.Empty(t, r.Role, "a login refresh token carries no role")

	a := validated(t, svc, access)
	assert.Equal(t, TokenTypeAccess, a.TokenType)
	assert.Equal(t, sid, a.SessionID)
	require.NotNil(t, a.AuthTime)
	assert.Equal(t, r.AuthTime.Unix(), a.AuthTime.Unix(), "the pair agrees on auth_time")
	assert.Equal(t, "current@example.com", a.Email, "the email is the caller's")
	assert.Equal(t, "member", a.Role, "the role is the caller's")
	assert.Equal(t, userID, a.UserID)
	assert.Equal(t, orgID, a.OrganizationID)
	assert.False(t, svc.AccessSessionExpired(a, time.Now()), "the session's maximum age counts from the re-authentication")
}

// T2: only a login refresh token has a re-authenticated successor.
func TestAuthTime_ReauthenticatedPairRefusesEveryOtherToken(t *testing.T) {
	svc := sessionAgeService(t, "8h")
	userID, orgID := uuid.New().String(), uuid.New().String()
	access, _, err := svc.GenerateTokenPair(userID, orgID, "auth-time@example.com", "admin")
	require.NoError(t, err)
	sdk, err := svc.GenerateSDKRefreshToken(userID, orgID, "auth-time@example.com", "admin")
	require.NoError(t, err)
	service, err := svc.GenerateServiceToken(uuid.New().String(), orgID)
	require.NoError(t, err)

	for name, claims := range map[string]*JWTClaims{
		"login access token": validated(t, svc, access),
		"SDK-download token": validated(t, svc, sdk),
		"service token":      validated(t, svc, service),
		"no token":           nil,
	} {
		a, r, err := svc.ReauthenticatedPair(claims, "auth-time@example.com", "admin")
		assert.Error(t, err, name)
		assert.Empty(t, a, name)
		assert.Empty(t, r, name)
	}
}
