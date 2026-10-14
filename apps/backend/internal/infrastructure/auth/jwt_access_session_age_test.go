package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A login access token carries the sign-in's time (auth_time) its login pair
// carries, so a bearer whose sign-in is past JWT_SESSION_MAX_AGE can be
// refused by comparison alone. Access tokens minted from an SDK-download
// token, standalone, or before the claim reached access tokens carry none
// and are not checked: they live at most JWT_ACCESS_TTL from their iat.

// B1: the access token minted with a login pair carries auth_time equal to
// its refresh token's, and a rotation's access token carries the sign-in's.
func TestAccessSessionAge_LoginAccessTokenCarriesItsSignInTime(t *testing.T) {
	svc := sessionAgeService(t, "8h")
	userID, orgID := uuid.New().String(), uuid.New().String()
	access, refresh, err := svc.GenerateTokenPair(userID, orgID, "age@example.com", "admin")
	require.NoError(t, err)

	ac, rc := validated(t, svc, access), validated(t, svc, refresh)
	require.NotNil(t, rc.AuthTime)
	require.NotNil(t, ac.AuthTime, "the login access token carries auth_time")
	assert.Equal(t, rc.AuthTime.Unix(), ac.AuthTime.Unix(), "equal to its refresh token's")
	assert.Equal(t, rc.SessionID, ac.SessionID, "and the same family")

	// A rotation three hours into the sign-in: the new access token carries
	// the sign-in's time, not the rotation's.
	now := time.Now()
	signedIn := now.Add(-3 * time.Hour)
	rotated, _, err := svc.RefreshTokenPair(signClaims(t, loginRefreshClaims(userID, orgID, "s", signedIn, now)), "age@example.com", "admin")
	require.NoError(t, err)
	c := validated(t, svc, rotated)
	require.NotNil(t, c.AuthTime)
	assert.Equal(t, signedIn.Unix(), c.AuthTime.Unix())
}

// B2: an access token minted from an SDK-download token or standalone
// carries no auth_time: neither has a maximum session age.
func TestAccessSessionAge_OtherAccessTokensCarryNoSignInTime(t *testing.T) {
	svc := sessionAgeService(t, "8h")
	userID, orgID := uuid.New().String(), uuid.New().String()

	sdk, err := svc.GenerateSDKRefreshToken(userID, orgID, "age@example.com", "admin")
	require.NoError(t, err)
	fromSDK, _, err := svc.RefreshTokenPair(sdk, "age@example.com", "admin")
	require.NoError(t, err)
	_, has := payloadOf(t, fromSDK)["auth_time"]
	assert.False(t, has, "an access token minted from an SDK-download token carries no auth_time")

	standalone, err := svc.GenerateAccessToken(userID, orgID, "age@example.com", "admin")
	require.NoError(t, err)
	_, has = payloadOf(t, standalone)["auth_time"]
	assert.False(t, has, "a standalone access token carries no auth_time")
}

// B3: AccessSessionExpired compares a login access token's auth_time with
// the cap and nothing else.
func TestAccessSessionAge_OnlyLoginAccessTokensWithAuthTimeAreChecked(t *testing.T) {
	svc := sessionAgeService(t, "8h")
	now := time.Now()
	maxAge := 8 * time.Hour
	at := func(signedIn time.Time) *jwt.NumericDate { return jwt.NewNumericDate(signedIn) }
	login := func(authTime *jwt.NumericDate) *JWTClaims {
		return &JWTClaims{TokenType: TokenTypeAccess, SessionID: "s", AuthTime: authTime,
			RegisteredClaims: jwt.RegisteredClaims{Issuer: IssuerUser, IssuedAt: jwt.NewNumericDate(now)}}
	}
	assert.True(t, svc.AccessSessionExpired(login(at(now.Add(-maxAge-time.Second))), now), "cap+1s is expired")
	assert.False(t, svc.AccessSessionExpired(login(at(now.Add(-maxAge+time.Minute))), now), "cap-1min is not")
	assert.False(t, svc.AccessSessionExpired(login(nil), now), "no auth_time is not checked")

	old := at(now.Add(-48 * time.Hour))
	refresh := &JWTClaims{TokenType: TokenTypeRefresh, AuthTime: old, RegisteredClaims: jwt.RegisteredClaims{Issuer: IssuerUser, IssuedAt: old}}
	assert.False(t, svc.AccessSessionExpired(refresh, now), "refresh tokens are checked by SessionExpired, not here")
	sdk := &JWTClaims{TokenType: TokenTypeSDK, AuthTime: old, RegisteredClaims: jwt.RegisteredClaims{Issuer: IssuerSDK, IssuedAt: old}}
	assert.False(t, svc.AccessSessionExpired(sdk, now), "SDK-download tokens have no maximum session age")
	service := &JWTClaims{TokenType: TokenTypeAccess, AuthTime: old, RegisteredClaims: jwt.RegisteredClaims{Issuer: IssuerService, IssuedAt: old}}
	assert.False(t, svc.AccessSessionExpired(service, now), "service tokens have no sign-in")
	assert.False(t, svc.AccessSessionExpired(nil, now))

	svc1 := sessionAgeService(t, "1h")
	assert.True(t, svc1.AccessSessionExpired(login(at(now.Add(-2*time.Hour))), now), "2h old is past a 1h cap")
}

// The check is a comparison of two claims already in memory.
func BenchmarkAccessSessionExpired(b *testing.B) {
	svc := &JWTService{sessionMaxAge: 8 * time.Hour}
	now := time.Now()
	c := &JWTClaims{TokenType: TokenTypeAccess, SessionID: "s", AuthTime: jwt.NewNumericDate(now.Add(-time.Hour)),
		RegisteredClaims: jwt.RegisteredClaims{Issuer: IssuerUser, IssuedAt: jwt.NewNumericDate(now)}}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if svc.AccessSessionExpired(c, now) {
			b.Fatal("an hour-old sign-in is inside an 8h cap")
		}
	}
}
