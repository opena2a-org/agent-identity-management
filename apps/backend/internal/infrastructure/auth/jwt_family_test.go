package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Token families (RFC 9700 section 4.14.2): a login refresh token carries the
// registered "sid" claim naming the sign-in it belongs to (its own jti at
// login, copied on every rotation), so the refresh route can end the whole
// sign-in when a rotated-out member is presented again. Access, SDK-download
// and service tokens carry no family.

const familyTestSecret = "test-only-jwt-secret-not-a-real-value-0123456789"

// familyStore is a RevocationStore double that records the TTL of every write
// and whose reads and writes can be made to fail.
type familyStore struct {
	keys      map[string]time.Duration
	setErr    error
	existsErr error
}

func (s *familyStore) Exists(_ context.Context, key string) (bool, error) {
	if s.existsErr != nil {
		return false, s.existsErr
	}
	_, ok := s.keys[key]
	return ok, nil
}

func (s *familyStore) Set(_ context.Context, key string, _ interface{}, ttl time.Duration) error {
	if s.setErr != nil {
		return s.setErr
	}
	if s.keys == nil {
		s.keys = map[string]time.Duration{}
	}
	s.keys[key] = ttl
	return nil
}

func familyTestService(t *testing.T, refreshTTL string) *JWTService {
	t.Helper()
	t.Setenv("JWT_SECRET", familyTestSecret)
	t.Setenv("JWT_REFRESH_TTL", refreshTTL)
	return NewJWTService()
}

// payloadOf decodes the token's payload segment without verifying it: the
// tests read which claim KEYS are present, not their trustworthiness.
func payloadOf(t *testing.T, token string) map[string]interface{} {
	t.Helper()
	parts := strings.Split(token, ".")
	require.Len(t, parts, 3)
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &m))
	return m
}

func validated(t *testing.T, svc *JWTService, token string) *JWTClaims {
	t.Helper()
	claims, err := svc.ValidateToken(token)
	require.NoError(t, err)
	return claims
}

// signClaims mints a token from a claims literal with the test secret, so a
// test can shape a token the service itself would not mint (pre-change
// tokens, longer-lived tokens).
func signClaims(t *testing.T, claims JWTClaims) string {
	t.Helper()
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(familyTestSecret))
	require.NoError(t, err)
	return token
}

func preChangeRefreshToken(t *testing.T, userID, orgID string, lifetime time.Duration) string {
	t.Helper()
	now := time.Now()
	return signClaims(t, JWTClaims{
		UserID:         userID,
		OrganizationID: orgID,
		TokenType:      TokenTypeRefresh,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(lifetime)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    IssuerUser,
			Subject:   userID,
			ID:        uuid.New().String(),
		},
	})
}

// J1: a login refresh token names its own family; no other token kind does.
func TestFamily_LoginRefreshTokenCarriesSid(t *testing.T) {
	svc := familyTestService(t, "168h")
	userID, orgID := uuid.New().String(), uuid.New().String()
	access, refresh, err := svc.GenerateTokenPair(userID, orgID, "j1@example.com", "admin")
	require.NoError(t, err)

	claims := validated(t, svc, refresh)
	assert.NotEmpty(t, claims.ID)
	assert.Equal(t, claims.ID, claims.SessionID, "at login the family is the token's own jti")
	_, hasSid := payloadOf(t, refresh)["sid"]
	assert.True(t, hasSid, "the payload carries the sid key")

	_, hasSid = payloadOf(t, access)["sid"]
	assert.False(t, hasSid, "access tokens carry no sid")
	sdk, err := svc.GenerateSDKRefreshToken(userID, orgID, "j1@example.com", "admin")
	require.NoError(t, err)
	_, hasSid = payloadOf(t, sdk)["sid"]
	assert.False(t, hasSid, "SDK-download tokens carry no sid")
	service, err := svc.GenerateServiceToken(uuid.New().String(), orgID)
	require.NoError(t, err)
	_, hasSid = payloadOf(t, service)["sid"]
	assert.False(t, hasSid, "service tokens carry no sid")
}

// J2: rotation copies the family and draws a fresh jti; SDK rotation carries none.
func TestFamily_RotationCopiesTheSid(t *testing.T) {
	svc := familyTestService(t, "168h")
	userID, orgID := uuid.New().String(), uuid.New().String()
	p1, err := svc.GenerateRefreshToken(userID, orgID)
	require.NoError(t, err)
	_, p2, err := svc.RefreshTokenPair(p1, "j2@example.com", "admin")
	require.NoError(t, err)
	_, p3, err := svc.RefreshTokenPair(p2, "j2@example.com", "admin")
	require.NoError(t, err)

	c1, c2, c3 := validated(t, svc, p1), validated(t, svc, p2), validated(t, svc, p3)
	assert.Equal(t, c1.ID, c2.SessionID)
	assert.Equal(t, c1.ID, c3.SessionID)
	assert.NotEqual(t, c1.ID, c2.ID)
	assert.NotEqual(t, c2.ID, c3.ID)

	s1, err := svc.GenerateSDKRefreshToken(userID, orgID, "j2@example.com", "admin")
	require.NoError(t, err)
	_, s2, err := svc.RefreshTokenPair(s1, "j2@example.com", "admin")
	require.NoError(t, err)
	_, hasSid := payloadOf(t, s2)["sid"]
	assert.False(t, hasSid, "an SDK token rotated through RefreshTokenPair carries no sid")
}

// J3: a login refresh token minted before this change is the root of its own
// family, and its first rotation starts the family under that jti.
func TestFamily_PreChangeTokenIsItsOwnFamilyRoot(t *testing.T) {
	svc := familyTestService(t, "168h")
	userID, orgID := uuid.New().String(), uuid.New().String()
	l := preChangeRefreshToken(t, userID, orgID, time.Hour)
	cl := validated(t, svc, l)
	assert.Empty(t, cl.SessionID)
	assert.Equal(t, cl.ID, cl.FamilyID())

	_, n1, err := svc.RefreshTokenPair(l, "j3@example.com", "admin")
	require.NoError(t, err)
	assert.Equal(t, cl.ID, validated(t, svc, n1).SessionID, "the rotation carries the pre-change token's jti as the family")
}

// J4 (pin): no family for access, SDK-download and service tokens.
func TestFamily_OnlyLoginRefreshTokensHaveAFamily(t *testing.T) {
	svc := familyTestService(t, "168h")
	userID, orgID := uuid.New().String(), uuid.New().String()
	access, err := svc.GenerateAccessToken(userID, orgID, "j4@example.com", "admin")
	require.NoError(t, err)
	sdk, err := svc.GenerateSDKRefreshToken(userID, orgID, "j4@example.com", "admin")
	require.NoError(t, err)
	service, err := svc.GenerateServiceToken(uuid.New().String(), orgID)
	require.NoError(t, err)
	for name, token := range map[string]string{"access": access, "sdk": sdk, "service": service} {
		assert.Equal(t, "", validated(t, svc, token).FamilyID(), name)
	}
}

// J5 (pin): a verifier built before this change accepts a token that carries
// sid. Its claims struct has no field for it, and the jti and typ survive.
func TestFamily_OlderVerifierAcceptsATokenWithSid(t *testing.T) {
	type olderClaims struct {
		UserID    string `json:"user_id"`
		TokenType string `json:"typ,omitempty"`
		jwt.RegisteredClaims
	}
	svc := familyTestService(t, "168h")
	refresh, err := svc.GenerateRefreshToken(uuid.New().String(), uuid.New().String())
	require.NoError(t, err)
	_, hasSid := payloadOf(t, refresh)["sid"]
	require.True(t, hasSid)

	parse := func(key string) (*olderClaims, error) {
		out := &olderClaims{}
		_, err := jwt.ParseWithClaims(refresh, out, func(*jwt.Token) (interface{}, error) { return []byte(key), nil })
		return out, err
	}
	old, err := parse(familyTestSecret)
	require.NoError(t, err)
	assert.Equal(t, validated(t, svc, refresh).ID, old.ID, "the jti is preserved")
	assert.Equal(t, TokenTypeRefresh, old.TokenType)
	_, err = parse("another-secret-that-is-also-long-enough-0123456789")
	assert.Error(t, err, "control: the wrong key is refused")
}

// J6: RevokeFamily writes revoked:fam:<family> for max(JWT_REFRESH_TTL, the
// token's own lifetime) plus one minute, and writes nothing for tokens that
// have no family or when no revoker is attached.
func TestFamily_RevokeFamilyWritesTheKeyForTheLongerLifetime(t *testing.T) {
	svc := familyTestService(t, "2h")
	store := &familyStore{}
	svc.SetRevoker(NewTokenRevoker(store, false))
	userID, orgID := uuid.New().String(), uuid.New().String()
	ctx := context.Background()

	p1, err := svc.GenerateRefreshToken(userID, orgID)
	require.NoError(t, err)
	c1 := validated(t, svc, p1)
	revoked, err := svc.RevokeFamily(ctx, c1)
	require.NoError(t, err)
	assert.True(t, revoked)
	ttl, ok := store.keys["revoked:fam:"+c1.ID]
	require.True(t, ok, "the family key is written under the token's own jti")
	assert.InDelta(t, (2*time.Hour + time.Minute).Seconds(), ttl.Seconds(), 1, "configured TTL wins when the token is not longer-lived")

	longer := preChangeRefreshToken(t, userID, orgID, 5*time.Hour)
	cl := validated(t, svc, longer)
	revoked, err = svc.RevokeFamily(ctx, cl)
	require.NoError(t, err)
	assert.True(t, revoked)
	ttl, ok = store.keys["revoked:fam:"+cl.ID]
	require.True(t, ok)
	assert.InDelta(t, (5*time.Hour + time.Minute).Seconds(), ttl.Seconds(), 1, "the token's own lifetime wins when it is longer")

	t.Run("no revoker: false, nil, no write", func(t *testing.T) {
		bare := familyTestService(t, "2h")
		revoked, err := bare.RevokeFamily(ctx, c1)
		assert.NoError(t, err)
		assert.False(t, revoked)
	})
	t.Run("store refuses the write: false, err", func(t *testing.T) {
		failing := &familyStore{setErr: errors.New("store down")}
		svc2 := familyTestService(t, "2h")
		svc2.SetRevoker(NewTokenRevoker(failing, false))
		revoked, err := svc2.RevokeFamily(ctx, c1)
		assert.Error(t, err)
		assert.False(t, revoked)
		assert.Empty(t, failing.keys)
	})
	t.Run("tokens without a family: false, nil, no write", func(t *testing.T) {
		before := len(store.keys)
		access, err := svc.GenerateAccessToken(userID, orgID, "j6@example.com", "admin")
		require.NoError(t, err)
		sdk, err := svc.GenerateSDKRefreshToken(userID, orgID, "j6@example.com", "admin")
		require.NoError(t, err)
		for _, token := range []string{access, sdk} {
			revoked, err := svc.RevokeFamily(ctx, validated(t, svc, token))
			assert.NoError(t, err)
			assert.False(t, revoked)
		}
		assert.Equal(t, before, len(store.keys))
	})
}

// J7: the refresh route's reads bypass the in-process negative cache and say
// whether the store answered; the middleware's cached read is unchanged.
func TestFamily_CheckedReadsBypassTheCacheAndReportKnown(t *testing.T) {
	store := &familyStore{}
	a := familyTestService(t, "2h")
	a.SetRevoker(NewTokenRevoker(store, false))
	b := familyTestService(t, "2h")
	b.SetRevoker(NewTokenRevoker(store, false))
	ctx := context.Background()

	p1, err := a.GenerateRefreshToken(uuid.New().String(), uuid.New().String())
	require.NoError(t, err)
	c1 := validated(t, a, p1)

	assert.False(t, a.IsRevoked(ctx, c1.ID), "A caches 'not revoked'")
	revoked, known := a.CheckRevoked(ctx, c1.ID)
	assert.False(t, revoked)
	assert.True(t, known)
	revoked, known = a.CheckFamilyRevoked(ctx, c1.FamilyID())
	assert.False(t, revoked)
	assert.True(t, known)

	require.NoError(t, b.RevokeToken(ctx, p1))
	ok, err := b.RevokeFamily(ctx, c1)
	require.NoError(t, err)
	require.True(t, ok)

	assert.False(t, a.IsRevoked(ctx, c1.ID), "the cached middleware read still says not revoked (its contract is unchanged)")
	revoked, known = a.CheckRevoked(ctx, c1.ID)
	assert.True(t, revoked, "the uncached read sees B's write")
	assert.True(t, known)
	revoked, known = a.CheckFamilyRevoked(ctx, c1.FamilyID())
	assert.True(t, revoked)
	assert.True(t, known)

	store.existsErr = errors.New("store down")
	revoked, known = a.CheckRevoked(ctx, c1.ID)
	assert.True(t, revoked, "fail-closed: treated as revoked")
	assert.False(t, known, "but the store did not answer")
	revoked, known = a.CheckFamilyRevoked(ctx, c1.FamilyID())
	assert.True(t, revoked)
	assert.False(t, known)

	open := familyTestService(t, "2h")
	open.SetRevoker(NewTokenRevoker(store, true))
	revoked, known = open.CheckRevoked(ctx, c1.ID)
	assert.False(t, revoked, "fail-open: not treated as revoked")
	assert.False(t, known)

	bare := familyTestService(t, "2h")
	revoked, known = bare.CheckRevoked(ctx, c1.ID)
	assert.False(t, revoked)
	assert.False(t, known, "no revoker: nothing is known")
	revoked, known = bare.CheckFamilyRevoked(ctx, c1.FamilyID())
	assert.False(t, revoked)
	assert.False(t, known)
}
