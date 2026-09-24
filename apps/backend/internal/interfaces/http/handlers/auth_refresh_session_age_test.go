package handlers

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

// A sign-in ends a fixed time after it began, however often it is refreshed:
// the refresh route refuses a login refresh token whose sign-in is older than
// JWT_SESSION_MAX_AGE with the route's expiry answer, before the account
// lookup, logging identifiers only and writing no audit row. SDK-download
// tokens are not subject to it.

const sessionAgeTestSecret = "test-secret-key-for-unit-tests-32"
const sessionExpiredBody = `{"error":"Invalid or expired refresh token"}`

func signSessionClaims(t *testing.T, claims auth.JWTClaims) string {
	t.Helper()
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(sessionAgeTestSecret))
	require.NoError(t, err)
	return tok
}

// loginTokenSignedIn shapes a login refresh token signed in at signedIn and
// issued at issued; a zero signedIn leaves auth_time out (a pre-change token).
func loginTokenSignedIn(userID, orgID uuid.UUID, sid string, signedIn, issued time.Time) auth.JWTClaims {
	c := auth.JWTClaims{
		UserID:         userID.String(),
		OrganizationID: orgID.String(),
		TokenType:      auth.TokenTypeRefresh,
		SessionID:      sid,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(issued.Add(168 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(issued),
			NotBefore: jwt.NewNumericDate(issued),
			Issuer:    auth.IssuerUser,
			Subject:   userID.String(),
			ID:        uuid.New().String(),
		},
	}
	if !signedIn.IsZero() {
		c.AuthTime = jwt.NewNumericDate(signedIn)
	}
	return c
}

func sessionAgeApp(t *testing.T, store *rotationStore) *familyFixture {
	t.Helper()
	t.Setenv("JWT_SESSION_MAX_AGE", "1h")
	return familyApp(t, store, false)
}

// S1: a sign-in older than the maximum age is refused with the expiry answer,
// recorded in the log with identifiers only, and nothing else is written.
func TestRefreshToken_SignInOlderThanMaxAgeIsRefused(t *testing.T) {
	logs := captureLog(t)
	store := &rotationStore{}
	f := sessionAgeApp(t, store)
	now := time.Now()
	sid := uuid.New().String()
	claims := loginTokenSignedIn(f.userID, f.orgID, sid, now.Add(-2*time.Hour), now.Add(-10*time.Minute))
	tok := signSessionClaims(t, claims)

	body, status := postRefreshUA(t, f.app, tok)
	require.Equal(t, fiber.StatusUnauthorized, status)
	assert.JSONEq(t, sessionExpiredBody, body)
	assert.Empty(t, f.audit.rows, "no audit row")
	assert.Equal(t, 0, familyKeyWrites(store), "no family key is written")
	assert.False(t, f.svc.IsRevoked(context.Background(), claims.ID), "the jti is not denylisted")
	assert.Contains(t, logs.String(), "family="+sid)
	assert.NotContains(t, logs.String(), tok, "the token is never logged")
	assert.NotContains(t, logs.String(), "SECURITY")

	// Same-fixture control: a revoked family past the cap still gets the
	// revoked answer, one audit row and one SECURITY line.
	require.NoError(t, store.Set(context.Background(), "revoked:fam:"+sid, "1", time.Hour))
	logs.Reset()
	body, status = postRefreshUA(t, f.app, tok)
	assert.Equal(t, fiber.StatusUnauthorized, status)
	assert.Contains(t, body, familyRefusal)
	require.Len(t, f.audit.rows, 1)
	assert.Equal(t, domain.AuditActionRefreshSessionRevoked, f.audit.rows[0].Action)
	assert.Equal(t, 1, strings.Count(logs.String(), "SECURITY refresh_session_revoked"))
}

// S2: a sign-in inside the maximum age refreshes and its successor keeps auth_time and sid.
func TestRefreshToken_SignInInsideMaxAgeRefreshesAndKeepsItsAuthTime(t *testing.T) {
	f := sessionAgeApp(t, &rotationStore{})
	signedIn := time.Now().Add(-30 * time.Minute)
	sid := uuid.New().String()
	tok := signSessionClaims(t, loginTokenSignedIn(f.userID, f.orgID, sid, signedIn, signedIn))

	out, status := postRefresh(t, f.app, tok)
	require.Equal(t, fiber.StatusOK, status)
	assert.True(t, out.Rotated)
	c, err := f.svc.ValidateToken(out.RefreshToken)
	require.NoError(t, err)
	require.NotNil(t, c.AuthTime)
	assert.Equal(t, signedIn.Unix(), c.AuthTime.Unix())
	assert.Equal(t, sid, c.SessionID)
}

// S3: a token minted before the claim existed counts from its own iat.
func TestRefreshToken_PreChangeTokenCountsFromItsIat(t *testing.T) {
	f := sessionAgeApp(t, &rotationStore{})
	now := time.Now()
	oldClaims := loginTokenSignedIn(f.userID, f.orgID, "", time.Time{}, now.Add(-2*time.Hour))
	oldClaims.SessionID = ""
	body, status := postRefreshRaw(t, f.app, signSessionClaims(t, oldClaims))
	require.Equal(t, fiber.StatusUnauthorized, status)
	assert.JSONEq(t, sessionExpiredBody, body)

	iat := now.Add(-10 * time.Minute)
	youngClaims := loginTokenSignedIn(f.userID, f.orgID, "", time.Time{}, iat)
	youngClaims.SessionID = ""
	out, status := postRefresh(t, f.app, signSessionClaims(t, youngClaims))
	require.Equal(t, fiber.StatusOK, status)
	c, err := f.svc.ValidateToken(out.RefreshToken)
	require.NoError(t, err)
	require.NotNil(t, c.AuthTime)
	assert.Equal(t, iat.Unix(), c.AuthTime.Unix(), "the successor's auth_time is the pre-change token's iat")
}

// S4: an SDK-download token has no maximum session age; a login token of the
// same age on the same app is refused.
func TestRefreshToken_SDKDownloadTokenHasNoMaxAge(t *testing.T) {
	t.Setenv("JWT_SESSION_MAX_AGE", "1h")
	userID, orgID := uuid.New(), uuid.New()
	users := &refreshTestUserRepo{getByID: func(uuid.UUID) (*domain.User, error) {
		return activeUser(userID, orgID, domain.RoleAdmin, "sdk-age@example.com"), nil
	}}
	sdkRepo := &rotationSDKRepo{token: &domain.SDKToken{
		ID: uuid.New(), UserID: userID, OrganizationID: orgID, ExpiresAt: time.Now().Add(24 * time.Hour),
	}}
	app, svc := newRefreshTestAppAudit(t, users, sdkRepo, &familyAuditRepo{})
	svc.SetRevoker(auth.NewTokenRevoker(&rotationStore{}, false))
	now := time.Now()
	issued := now.Add(-3 * time.Hour)
	sdkClaims := auth.JWTClaims{
		UserID: userID.String(), OrganizationID: orgID.String(), Email: "sdk-age@example.com", Role: "admin",
		TokenType: auth.TokenTypeSDK,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(issued.Add(90 * 24 * time.Hour)), IssuedAt: jwt.NewNumericDate(issued),
			NotBefore: jwt.NewNumericDate(issued), Issuer: auth.IssuerSDK, Subject: userID.String(), ID: uuid.New().String(),
		},
	}
	_, status := postRefresh(t, app, signSessionClaims(t, sdkClaims))
	require.Equal(t, fiber.StatusOK, status, "an SDK-download token 3h old still refreshes under a 1h cap")

	// Same-app control: a login token of the same age is refused.
	sdkRepo.token = nil
	body, status := postRefreshRaw(t, app, signSessionClaims(t, loginTokenSignedIn(userID, orgID, uuid.New().String(), issued, issued)))
	require.Equal(t, fiber.StatusUnauthorized, status)
	assert.JSONEq(t, sessionExpiredBody, body)
}

// S5: a revoked session past the maximum age keeps its revoked answer and record.
func TestRefreshToken_RevokedSessionPastMaxAgeKeepsItsRecord(t *testing.T) {
	store := &rotationStore{}
	f := sessionAgeApp(t, store)
	now := time.Now()
	sid := uuid.New().String()
	require.NoError(t, store.Set(context.Background(), "revoked:fam:"+sid, "1", time.Hour))
	body, status := postRefreshUA(t, f.app, signSessionClaims(t, loginTokenSignedIn(f.userID, f.orgID, sid, now.Add(-2*time.Hour), now.Add(-10*time.Minute))))
	require.Equal(t, fiber.StatusUnauthorized, status)
	assert.Contains(t, body, familyRefusal)
	require.Len(t, f.audit.rows, 1)
	assert.Equal(t, domain.AuditActionRefreshSessionRevoked, f.audit.rows[0].Action)
}

// S6: the maximum age is checked before the account, so an expired sign-in
// never learns the account's state.
func TestRefreshToken_MaxAgeIsCheckedBeforeTheAccount(t *testing.T) {
	t.Setenv("JWT_SESSION_MAX_AGE", "1h")
	userID, orgID := uuid.New(), uuid.New()
	deactivated := activeUser(userID, orgID, domain.RoleAdmin, "gone@example.com")
	deactivated.Status = domain.UserStatusDeactivated
	users := &refreshTestUserRepo{getByID: func(uuid.UUID) (*domain.User, error) { return deactivated, nil }}
	app, _ := newRefreshTestAppAudit(t, users, nil, &familyAuditRepo{})
	now := time.Now()

	body, status := postRefreshRaw(t, app, signSessionClaims(t, loginTokenSignedIn(userID, orgID, uuid.New().String(), now.Add(-2*time.Hour), now.Add(-10*time.Minute))))
	require.Equal(t, fiber.StatusUnauthorized, status)
	assert.JSONEq(t, sessionExpiredBody, body, "past the cap the answer is expiry, not the account's state")

	body, status = postRefreshRaw(t, app, signSessionClaims(t, loginTokenSignedIn(userID, orgID, uuid.New().String(), now.Add(-30*time.Minute), now.Add(-30*time.Minute))))
	require.Equal(t, fiber.StatusUnauthorized, status)
	assert.Contains(t, body, "Account is not active", "control: inside the cap the account is looked up")
}

// S7: the maximum age holds without a revocation store.
func TestRefreshToken_MaxAgeHoldsWithoutARevocationStore(t *testing.T) {
	f := sessionAgeApp(t, nil)
	now := time.Now()
	body, status := postRefreshRaw(t, f.app, signSessionClaims(t, loginTokenSignedIn(f.userID, f.orgID, uuid.New().String(), now.Add(-2*time.Hour), now.Add(-10*time.Minute))))
	require.Equal(t, fiber.StatusUnauthorized, status)
	assert.JSONEq(t, sessionExpiredBody, body)

	young := signSessionClaims(t, loginTokenSignedIn(f.userID, f.orgID, uuid.New().String(), now.Add(-30*time.Minute), now.Add(-30*time.Minute)))
	out, status := postRefresh(t, f.app, young)
	require.Equal(t, fiber.StatusOK, status)
	assert.False(t, out.Rotated, "without a store nothing is retired, so nothing rotates")
	assert.Equal(t, young, out.RefreshToken)
}
