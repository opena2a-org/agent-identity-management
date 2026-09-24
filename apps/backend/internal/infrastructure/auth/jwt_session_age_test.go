package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A sign-in ends a fixed time after it began, however often it is refreshed.
// A login refresh token carries the sign-in's time (the registered auth_time
// claim), every rotation copies it unchanged, and a token minted before the
// claim existed counts from its own iat. Only login refresh tokens have a
// maximum session age; access, SDK-download and service tokens do not.

func sessionAgeService(t *testing.T, maxAge string) *JWTService {
	t.Helper()
	t.Setenv("JWT_SESSION_MAX_AGE", maxAge)
	return familyTestService(t, "168h")
}

// loginRefreshClaims shapes a login refresh token that was signed in at
// signedIn and issued (or last rotated) at issued; a zero signedIn leaves the
// auth_time claim out, as a token minted before the claim existed.
func loginRefreshClaims(userID, orgID, sid string, signedIn, issued time.Time) JWTClaims {
	c := JWTClaims{
		UserID:         userID,
		OrganizationID: orgID,
		TokenType:      TokenTypeRefresh,
		SessionID:      sid,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(issued.Add(168 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(issued),
			NotBefore: jwt.NewNumericDate(issued),
			Issuer:    IssuerUser,
			Subject:   userID,
			ID:        uuid.New().String(),
		},
	}
	if !signedIn.IsZero() {
		c.AuthTime = jwt.NewNumericDate(signedIn)
	}
	return c
}

// A1: a login refresh token carries auth_time equal to its iat; no other kind does.
func TestSessionAge_LoginRefreshTokenCarriesAuthTime(t *testing.T) {
	svc := sessionAgeService(t, "8h")
	userID, orgID := uuid.New().String(), uuid.New().String()
	access, refresh, err := svc.GenerateTokenPair(userID, orgID, "age@example.com", "admin")
	require.NoError(t, err)

	p := payloadOf(t, refresh)
	at, ok := p["auth_time"].(float64)
	require.True(t, ok, "auth_time is a JSON number")
	assert.Equal(t, float64(int64(at)), at, "auth_time is whole seconds")
	assert.Equal(t, p["iat"], p["auth_time"], "at sign-in auth_time equals iat")

	_, has := payloadOf(t, access)["auth_time"]
	assert.False(t, has, "access tokens carry no auth_time")
	sdk, err := svc.GenerateSDKRefreshToken(userID, orgID, "age@example.com", "admin")
	require.NoError(t, err)
	_, has = payloadOf(t, sdk)["auth_time"]
	assert.False(t, has, "SDK-download tokens carry no auth_time")
	service, err := svc.GenerateServiceToken(uuid.New().String(), orgID)
	require.NoError(t, err)
	_, has = payloadOf(t, service)["auth_time"]
	assert.False(t, has, "service tokens carry no auth_time")
}

// A2: rotation copies auth_time unchanged, twice over, while iat advances.
func TestSessionAge_RotationCopiesAuthTimeUnchanged(t *testing.T) {
	svc := sessionAgeService(t, "8h")
	now := time.Now()
	signedIn := now.Add(-3 * time.Hour)
	sid := uuid.New().String()
	userID, orgID := uuid.New().String(), uuid.New().String()
	p1 := signClaims(t, loginRefreshClaims(userID, orgID, sid, signedIn, now.Add(-time.Hour)))

	_, p2, err := svc.RefreshTokenPair(p1, "age@example.com", "admin")
	require.NoError(t, err)
	_, p3, err := svc.RefreshTokenPair(p2, "age@example.com", "admin")
	require.NoError(t, err)
	for _, tok := range []string{p2, p3} {
		c := validated(t, svc, tok)
		require.NotNil(t, c.AuthTime, "the successor carries auth_time")
		assert.Equal(t, signedIn.Unix(), c.AuthTime.Unix())
		assert.Equal(t, sid, c.SessionID)
		assert.True(t, c.IssuedAt.After(now.Add(-time.Hour)), "iat advances")
	}
}

// A3: a token minted before the claim existed counts from its own iat, and
// its first rotation writes that value as auth_time.
func TestSessionAge_PreChangeTokenCountsFromItsIat(t *testing.T) {
	svc8 := sessionAgeService(t, "8h")
	iat := time.Now().Add(-2 * time.Hour)
	userID, orgID := uuid.New().String(), uuid.New().String()
	c := loginRefreshClaims(userID, orgID, "", time.Time{}, iat)
	c.SessionID = ""
	tok := signClaims(t, c)

	claims := validated(t, svc8, tok)
	assert.Equal(t, iat.Unix(), claims.SignedInAt().Unix())
	assert.False(t, svc8.SessionExpired(claims, time.Now()), "2h old is inside an 8h cap")
	svc1 := sessionAgeService(t, "1h")
	assert.True(t, svc1.SessionExpired(claims, time.Now()), "2h old is past a 1h cap")

	_, next, err := svc8.RefreshTokenPair(tok, "age@example.com", "admin")
	require.NoError(t, err)
	nc := validated(t, svc8, next)
	require.NotNil(t, nc.AuthTime)
	assert.Equal(t, iat.Unix(), nc.AuthTime.Unix(), "the rotation writes the fallback as auth_time")
}

// A4: only login refresh tokens have a maximum session age.
func TestSessionAge_OnlyLoginRefreshTokensHaveAMaxAge(t *testing.T) {
	svc := sessionAgeService(t, "8h")
	now := time.Now()
	maxAge := 8 * time.Hour
	userID, orgID := uuid.New().String(), uuid.New().String()
	login := func(signedIn time.Time) *JWTClaims {
		c := loginRefreshClaims(userID, orgID, "s", signedIn, now)
		return &c
	}
	assert.True(t, svc.SessionExpired(login(now.Add(-maxAge-time.Second)), now), "cap+1s is expired")
	assert.False(t, svc.SessionExpired(login(now.Add(-maxAge+time.Minute)), now), "cap-1min is not")

	old := jwt.NewNumericDate(now.Add(-48 * time.Hour))
	sdk := &JWTClaims{TokenType: TokenTypeSDK, AuthTime: old, RegisteredClaims: jwt.RegisteredClaims{Issuer: IssuerSDK, IssuedAt: old}}
	assert.False(t, svc.SessionExpired(sdk, now), "SDK-download tokens have no maximum session age")
	access := &JWTClaims{TokenType: TokenTypeAccess, AuthTime: old, RegisteredClaims: jwt.RegisteredClaims{Issuer: IssuerUser, IssuedAt: old}}
	assert.False(t, svc.SessionExpired(access, now), "access tokens are not checked here")
	bare := &JWTClaims{TokenType: TokenTypeRefresh, RegisteredClaims: jwt.RegisteredClaims{Issuer: IssuerUser}}
	assert.True(t, svc.SessionExpired(bare, now), "no auth_time and no iat fails closed")
	assert.False(t, svc.SessionExpired(nil, now))
}

// A5: the maximum session age comes from JWT_SESSION_MAX_AGE, 8h when unset or unreadable.
func TestSessionAge_MaxAgeFromEnvironment(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  time.Duration
	}{
		{"", 8 * time.Hour}, {"30m", 30 * time.Minute}, {"24h", 24 * time.Hour},
		{"abc", 8 * time.Hour}, {"0", 8 * time.Hour}, {"-1h", 8 * time.Hour},
	} {
		t.Run("value="+tc.value, func(t *testing.T) {
			assert.Equal(t, tc.want, sessionAgeService(t, tc.value).SessionMaxAge())
		})
	}
}
