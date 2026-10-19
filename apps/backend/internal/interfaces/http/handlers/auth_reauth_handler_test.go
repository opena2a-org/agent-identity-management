package handlers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/middleware"
)

// POST /auth/reauth: a signed-in user presents their password again with the
// refresh token of the session the bearer access token belongs to. The
// password is verified as at sign-in, the refresh token is checked and
// retired as at refresh, and the successor pair stays in the session with an
// auth_time of now. The route runs behind the real AuthMiddleware here, as in
// main.go.

const reauthWrongPassword = "not-the-password"

// reauthFailureRepo records the failed attempts the auth service writes and
// answers with the lockout the service created, as the repository does.
type reauthFailureRepo struct {
	domain.AuthFailureRepository
	failures []*domain.AuthFailure
	lockout  *domain.AuthLockout
}

func (r *reauthFailureRepo) RecordFailure(f *domain.AuthFailure) error {
	r.failures = append(r.failures, f)
	return nil
}
func (r *reauthFailureRepo) CountRecentFailures(string, int) (int, error) {
	return len(r.failures), nil
}
func (r *reauthFailureRepo) CreateLockout(l *domain.AuthLockout) error {
	r.lockout = l
	return nil
}
func (r *reauthFailureRepo) GetActiveLockout(string) (*domain.AuthLockout, error) {
	return r.lockout, nil
}
func (r *reauthFailureRepo) ClearFailures(string) error {
	r.failures = nil
	return nil
}

type reauthFixture struct {
	app      *fiber.App
	svc      *auth.JWTService
	audit    *familyAuditRepo
	failures *reauthFailureRepo
	users    *issuanceUserRepo
	user     *domain.User
	sid      string
	signedIn time.Time
	access   string // the session's access token, the bearer
	refresh  string // the session's current refresh token

	// authService checks the password; a test attaches a requirement of
	// provider sign-in to it.
	authService *application.AuthService
}

// newReauthFixture builds the re-authentication route and the refresh route
// on one JWT service, revocation store (none when store is nil), user record
// and audit log, and a session that signed in three hours ago and has been
// refreshed since.
func newReauthFixture(t *testing.T, store auth.RevocationStore, sdkRepo domain.SDKTokenRepository) *reauthFixture {
	t.Helper()
	t.Setenv("JWT_SECRET", sessionAgeTestSecret)
	svc := auth.NewJWTService()
	if store != nil {
		svc.SetRevoker(auth.NewTokenRevoker(store, false))
	}
	if sdkRepo == nil {
		sdkRepo = &refreshTestSDKRepo{}
	}
	user := issuanceUser(t, false)
	users := &issuanceUserRepo{user: user}
	failures := &reauthFailureRepo{}
	auditRepo := &familyAuditRepo{}
	audit := application.NewAuditService(auditRepo)
	sdkTokens := application.NewSDKTokenService(sdkRepo)
	authService := application.NewAuthService(users, nil, nil, failures, nil, nil)

	reauth := NewAuthReauthHandler(svc, sdkTokens, users, audit, authService)
	refresh := NewAuthRefreshHandler(svc, sdkTokens, users, audit)
	app := fiber.New()
	app.Post("/auth/reauth", middleware.AuthMiddleware(svc), reauth.Reauthenticate)
	app.Post("/auth/refresh", refresh.RefreshToken)
	app.Get("/auth/session", middleware.AuthMiddleware(svc), func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	now := time.Now()
	signedIn := now.Add(-3 * time.Hour)
	sid := uuid.New().String()
	first := signSessionClaims(t, loginTokenSignedIn(user.ID, user.OrganizationID, sid, signedIn, now.Add(-time.Hour)))
	access, current, err := svc.RefreshTokenPair(first, user.Email, string(user.Role))
	require.NoError(t, err)
	return &reauthFixture{
		app: app, svc: svc, audit: auditRepo, failures: failures, users: users, user: user, authService: authService,
		sid: sid, signedIn: signedIn, access: access, refresh: current,
	}
}

// getSession passes the bearer through the session middleware alone and
// returns the status; a pass leaves the middleware's "not revoked" answer for
// the bearer in its cache.
func getSession(t *testing.T, app *fiber.App, bearer string) int {
	t.Helper()
	req := httptest.NewRequest("GET", "/auth/session", nil)
	req.Header.Set("Authorization", "Bearer "+bearer)
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})
	require.NoError(t, err)
	defer resp.Body.Close()
	return resp.StatusCode
}

func postReauth(t *testing.T, app *fiber.App, bearer, password, refreshToken string) (int, map[string]any) {
	t.Helper()
	raw, err := json.Marshal(map[string]string{"password": password, "refreshToken": refreshToken})
	require.NoError(t, err)
	return postReauthBody(t, app, bearer, raw)
}

func postReauthBody(t *testing.T, app *fiber.App, bearer string, raw []byte) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest("POST", "/auth/reauth", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+bearer)
	req.Header.Set("User-Agent", familyTestUA)
	// The password check hashes; under -race it can take longer than the
	// default one-second test timeout.
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second})
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var out map[string]any
	require.NoError(t, json.Unmarshal(body, &out), string(body))
	return resp.StatusCode, out
}

func (f *reauthFixture) claimsOf(t *testing.T, token string) *auth.JWTClaims {
	t.Helper()
	claims, err := f.svc.ValidateToken(token)
	require.NoError(t, err)
	return claims
}

func auditRowsOf(rows []*domain.AuditLog, action domain.AuditAction) []*domain.AuditLog {
	var kept []*domain.AuditLog
	for _, row := range rows {
		if row.Action == action {
			kept = append(kept, row)
		}
	}
	return kept
}

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// assertNothingIssued checks a refused re-authentication: no token in the
// answer, no sign-in recorded, the sign-in time unchanged.
func (f *reauthFixture) assertNothingIssued(t *testing.T, body map[string]any) {
	t.Helper()
	assert.NotContains(t, body, "accessToken")
	assert.NotContains(t, body, "refreshToken")
	assert.Empty(t, auditRowsOf(f.audit.rows, domain.AuditActionLogin), "no sign-in recorded")
	assert.Nil(t, f.users.user.LastLoginAt, "the account's last sign-in is unchanged")
}

// RA1: the right password rotates the session's pair. The successor keeps the
// session, both tokens carry an auth_time of now, the successor refreshes with
// that auth_time, the presented refresh token is refused when presented
// again, and the re-authentication is recorded as a sign-in of the session.
func TestReauth_RotatesThePairWithANewAuthTime(t *testing.T) {
	store := &rotationStore{}
	f := newReauthFixture(t, store, nil)
	presentedJTI := f.claimsOf(t, f.refresh).ID
	start := time.Now().Truncate(time.Second)

	status, body := postReauth(t, f.app, f.access, issuancePassword, f.refresh)
	require.Equal(t, fiber.StatusOK, status, body)
	assert.Equal(t, "Bearer", body["tokenType"])
	assert.Equal(t, float64(f.svc.AccessTTLSeconds()), body["expiresIn"])
	assert.Equal(t, true, body["rotated"])
	next, _ := body["refreshToken"].(string)
	access, _ := body["accessToken"].(string)
	require.NotEmpty(t, next)
	require.NotEmpty(t, access)
	assert.NotEqual(t, f.refresh, next)

	r, a := f.claimsOf(t, next), f.claimsOf(t, access)
	assert.Equal(t, f.sid, r.SessionID, "the successor stays in the session")
	assert.Equal(t, f.sid, a.SessionID)
	require.NotNil(t, r.AuthTime)
	require.NotNil(t, a.AuthTime)
	assert.False(t, r.AuthTime.Time.Before(start), "auth_time is the re-authentication's")
	assert.Equal(t, r.AuthTime.Unix(), a.AuthTime.Unix())
	assert.Greater(t, r.AuthTime.Unix(), f.signedIn.Unix(), "auth_time moved past the sign-in")
	assert.Equal(t, f.user.Email, a.Email, "the email is read from the user record")
	assert.Equal(t, string(f.user.Role), a.Role, "the role is read from the user record")
	assert.True(t, store.data["revoked:jti:"+presentedJTI], "the presented refresh token was retired")

	refreshed, status := postRefresh(t, f.app, next)
	require.Equal(t, fiber.StatusOK, status)
	ra := f.claimsOf(t, refreshed.AccessToken)
	require.NotNil(t, ra.AuthTime)
	assert.Equal(t, r.AuthTime.Unix(), ra.AuthTime.Unix(), "a refresh of the successor keeps the new auth_time")

	reuse, status := postRefreshRaw(t, f.app, f.refresh)
	assert.Equal(t, fiber.StatusUnauthorized, status, "the presented refresh token is refused on reuse")
	assert.JSONEq(t, `{"error":"Token has been revoked or is invalid"}`, reuse)

	logins := auditRowsOf(f.audit.rows, domain.AuditActionLogin)
	require.Len(t, logins, 1, "one sign-in row for the re-authentication")
	assert.Equal(t, "reauthentication", logins[0].Metadata["method"])
	assert.Equal(t, f.sid, logins[0].Metadata["familyId"])
	require.NotNil(t, f.users.user.LastLoginAt, "the account's last sign-in is the re-authentication")
	assert.False(t, f.users.user.LastLoginAt.Before(start))
}

// RA2: a wrong password is refused with 401 and recorded as a failed attempt
// of the account; nothing is retired, issued or recorded as a sign-in. The
// same request with the right password then succeeds.
func TestReauth_WrongPasswordIsRefusedAndRecorded(t *testing.T) {
	store := &rotationStore{}
	f := newReauthFixture(t, store, nil)

	status, body := postReauth(t, f.app, f.access, reauthWrongPassword, f.refresh)
	assert.Equal(t, fiber.StatusUnauthorized, status)
	assert.Equal(t, map[string]any{"error": "Invalid password"}, body)
	f.assertNothingIssued(t, body)
	require.Len(t, f.failures.failures, 1, "one failed attempt recorded")
	failure := f.failures.failures[0]
	assert.Equal(t, domain.AuthFailureInvalidCredentials, failure.FailureType)
	assert.Equal(t, f.user.Email, failure.Email)
	require.NotNil(t, failure.UserID)
	assert.Equal(t, f.user.ID, *failure.UserID)
	assert.Equal(t, familyTestUA, failure.UserAgent)
	assert.Zero(t, store.writes, "nothing retired")

	status, _ = postReauth(t, f.app, f.access, issuancePassword, f.refresh)
	assert.Equal(t, fiber.StatusOK, status, "control: the same session with the right password")
}

// RA3: re-authentication counts toward the sign-in lockout, and a locked
// account is refused whatever password it presents.
func TestReauth_LockedAccountIsRefused(t *testing.T) {
	store := &rotationStore{}
	f := newReauthFixture(t, store, nil)
	attempts := domain.DefaultAuthFailureConfig().MaxAttempts
	for i := 0; i < attempts; i++ {
		status, _ := postReauth(t, f.app, f.access, reauthWrongPassword, f.refresh)
		require.Equal(t, fiber.StatusUnauthorized, status)
	}
	require.NotNil(t, f.failures.lockout, "the failed re-authentications locked the account")

	status, body := postReauth(t, f.app, f.access, issuancePassword, f.refresh)
	assert.Equal(t, fiber.StatusUnauthorized, status)
	assert.Equal(t, map[string]any{"error": "Invalid password"}, body)
	f.assertNothingIssued(t, body)
	assert.Zero(t, store.writes, "nothing retired")
}

// RA3b: an account without a password cannot re-authenticate; it is refused
// with the same line as a wrong password.
func TestReauth_AccountWithoutAPasswordIsRefused(t *testing.T) {
	store := &rotationStore{}
	f := newReauthFixture(t, store, nil)
	f.users.user.PasswordHash = nil

	status, body := postReauth(t, f.app, f.access, issuancePassword, f.refresh)
	assert.Equal(t, fiber.StatusUnauthorized, status)
	assert.Equal(t, map[string]any{"error": "Invalid password"}, body)
	f.assertNothingIssued(t, body)
	assert.Zero(t, store.writes, "nothing retired")
}

// RA3c: the maximum session age counts from the re-authentication. A session
// re-authenticated before the cap ends gets a pair that outlives the sign-in's
// cap, and its successor refreshes.
func TestReauth_MaximumSessionAgeCountsFromTheReauthentication(t *testing.T) {
	t.Setenv("JWT_SESSION_MAX_AGE", "4h")
	store := &rotationStore{}
	f := newReauthFixture(t, store, nil)

	// The fixture's session signed in three hours ago, so under a 4h cap it
	// ends in an hour; two hours from now it would be past the cap.
	later := time.Now().Add(2 * time.Hour)
	require.True(t, f.svc.SessionExpired(f.claimsOf(t, f.refresh), later), "control: the sign-in's refresh token is past the cap in two hours")
	require.True(t, f.svc.AccessSessionExpired(f.claimsOf(t, f.access), later), "control: the sign-in's access token is past the cap in two hours")

	status, body := postReauth(t, f.app, f.access, issuancePassword, f.refresh)
	require.Equal(t, fiber.StatusOK, status, body)
	next, _ := body["refreshToken"].(string)
	access, _ := body["accessToken"].(string)
	assert.False(t, f.svc.SessionExpired(f.claimsOf(t, next), later), "the re-authenticated refresh token is within the cap in two hours")
	assert.False(t, f.svc.AccessSessionExpired(f.claimsOf(t, access), later), "the re-authenticated access token is within the cap in two hours")

	refreshed, status := postRefresh(t, f.app, next)
	assert.Equal(t, fiber.StatusOK, status, "the re-authenticated session refreshes")
	assert.Equal(t, f.sid, f.claimsOf(t, refreshed.RefreshToken).SessionID)
}

// RA3d: a session already past JWT_SESSION_MAX_AGE is not re-authenticated:
// its access token is refused by AuthMiddleware like an expired one, before
// the password is read, and the user signs in again.
func TestReauth_SessionPastItsMaximumAgeIsRefused(t *testing.T) {
	t.Setenv("JWT_SESSION_MAX_AGE", "1h")
	store := &rotationStore{}
	f := newReauthFixture(t, store, nil)

	// The fixture's session signed in three hours ago; a copy of its refresh
	// token, in the session but not the one the session holds, shows the
	// refresh route refusing it.
	expired := signSessionClaims(t, loginTokenSignedIn(f.user.ID, f.user.OrganizationID, f.sid, f.signedIn, time.Now().Add(-time.Minute)))
	_, status := postRefreshRaw(t, f.app, expired)
	require.Equal(t, fiber.StatusUnauthorized, status, "control: past the maximum age the session cannot refresh")

	status, body := postReauth(t, f.app, f.access, issuancePassword, f.refresh)
	assert.Equal(t, fiber.StatusUnauthorized, status)
	assert.Equal(t, map[string]any{"error": "Invalid or expired token"}, body)
	f.assertNothingIssued(t, body)
	assert.Empty(t, f.failures.failures, "the password was not read")
	assert.Zero(t, store.writes, "nothing retired")
}

// RA4: a refresh token that is not the bearer session's own login refresh
// token is refused before the password is read: another sign-in of the same
// user, another user's sign-in, the session's access token and an
// SDK-download token.
func TestReauth_RefreshTokenOfAnotherSessionIsRefused(t *testing.T) {
	store := &rotationStore{}
	f := newReauthFixture(t, store, nil)
	userID, orgID := f.user.ID.String(), f.user.OrganizationID.String()
	_, otherSignIn, err := f.svc.GenerateTokenPair(userID, orgID, f.user.Email, string(f.user.Role))
	require.NoError(t, err)
	_, otherUser, err := f.svc.GenerateTokenPair(uuid.NewString(), orgID, "someone-else@example.com", "admin")
	require.NoError(t, err)
	sdk, err := f.svc.GenerateSDKRefreshToken(userID, orgID, f.user.Email, string(f.user.Role))
	require.NoError(t, err)

	for name, presented := range map[string]string{
		"another sign-in of the same user": otherSignIn,
		"another user's sign-in":           otherUser,
		"the session's access token":       f.access,
		"an SDK-download token":            sdk,
		"not a token":                      "not-a-real-token",
	} {
		status, body := postReauth(t, f.app, f.access, issuancePassword, presented)
		assert.Equal(t, fiber.StatusUnauthorized, status, name)
		assert.Equal(t, map[string]any{"error": "Invalid or expired refresh token"}, body, name)
	}
	assert.Empty(t, f.failures.failures, "the password was not read")
	assert.Empty(t, f.audit.rows)
	assert.Nil(t, f.users.user.LastLoginAt)
	assert.Zero(t, store.writes, "nothing retired")

	status, _ := postReauth(t, f.app, f.access, issuancePassword, f.refresh)
	assert.Equal(t, fiber.StatusOK, status, "control: the session's own refresh token")
}

// RA5: an access token minted from an SDK-download token cannot
// re-authenticate: its session is the download's, which no login refresh
// token belongs to.
func TestReauth_SDKDerivedAccessTokenIsRefused(t *testing.T) {
	store := &rotationStore{}
	f := newReauthFixture(t, store, nil)
	sdk, err := f.svc.GenerateSDKRefreshToken(f.user.ID.String(), f.user.OrganizationID.String(), f.user.Email, string(f.user.Role))
	require.NoError(t, err)
	sdkAccess, _, err := f.svc.RefreshTokenPair(sdk, f.user.Email, string(f.user.Role))
	require.NoError(t, err)
	require.NotEmpty(t, f.claimsOf(t, sdkAccess).SessionID, "control: the SDK-derived access token is in the download's session")

	for name, presented := range map[string]string{"the user's login refresh token": f.refresh, "the SDK-download token": sdk} {
		status, body := postReauth(t, f.app, sdkAccess, issuancePassword, presented)
		assert.Equal(t, fiber.StatusUnauthorized, status, name)
		assert.Equal(t, map[string]any{"error": "Invalid or expired refresh token"}, body, name)
	}
	assert.Empty(t, f.failures.failures, "the password was not read")
	assert.Zero(t, store.writes, "nothing retired")
}

// RA6: a refresh token already retired by a rotation is a reuse, handled as
// the refresh route handles one: the session is revoked and the reuse
// recorded. A member of a revoked session is refused by the session
// middleware before the route; when the middleware passes it on a cached
// answer (another instance revoked the session), the route refuses and
// records it. The password is read in no case.
func TestReauth_RevokedRefreshTokenIsRefused(t *testing.T) {
	t.Run("retired by a rotation", func(t *testing.T) {
		store := &rotationStore{}
		f := newReauthFixture(t, store, nil)
		refreshed, status := postRefresh(t, f.app, f.refresh)
		require.Equal(t, fiber.StatusOK, status)
		require.True(t, refreshed.Rotated)

		status, body := postReauth(t, f.app, refreshed.AccessToken, issuancePassword, f.refresh)
		assert.Equal(t, fiber.StatusUnauthorized, status)
		assert.Equal(t, map[string]any{"error": "Token has been revoked or is invalid"}, body)
		f.assertNothingIssued(t, body)
		assert.True(t, store.data["revoked:fam:"+f.sid], "the session was revoked")
		reuses := auditRowsOf(f.audit.rows, domain.AuditActionRefreshTokenReuse)
		require.Len(t, reuses, 1)
		assert.Equal(t, f.sid, reuses[0].Metadata["familyId"])
		assert.Empty(t, f.failures.failures, "the password was not read")
	})
	t.Run("member of a revoked session", func(t *testing.T) {
		store := &rotationStore{}
		f := newReauthFixture(t, store, nil)
		require.NoError(t, store.Set(context.Background(), "revoked:fam:"+f.sid, "1", time.Hour))

		status, body := postReauth(t, f.app, f.access, issuancePassword, f.refresh)
		assert.Equal(t, fiber.StatusUnauthorized, status)
		assert.Equal(t, map[string]any{"error": "Token has been revoked"}, body, "the session middleware's answer")
		f.assertNothingIssued(t, body)
		assert.Empty(t, auditRowsOf(f.audit.rows, domain.AuditActionRefreshSessionRevoked), "the request does not reach the route")
		assert.Empty(t, f.failures.failures, "the password was not read")
		assert.Equal(t, 1, store.writes, "nothing written beyond the test's own revocation")
	})
	t.Run("member of a session revoked after the middleware cached its answer", func(t *testing.T) {
		store := &rotationStore{}
		f := newReauthFixture(t, store, nil)
		require.Equal(t, fiber.StatusOK, getSession(t, f.app, f.access), "control: the bearer passes the session middleware")
		// Another instance revokes the session: this instance's cached answer
		// is not dropped, so the middleware passes the bearer to the route.
		require.NoError(t, store.Set(context.Background(), "revoked:fam:"+f.sid, "1", time.Hour))

		status, body := postReauth(t, f.app, f.access, issuancePassword, f.refresh)
		assert.Equal(t, fiber.StatusUnauthorized, status)
		assert.Equal(t, map[string]any{"error": "Token has been revoked or is invalid"}, body, "the route's answer")
		f.assertNothingIssued(t, body)
		require.Len(t, auditRowsOf(f.audit.rows, domain.AuditActionRefreshSessionRevoked), 1)
		assert.Empty(t, f.failures.failures, "the password was not read")
		assert.Equal(t, 1, store.writes, "nothing written beyond the test's own revocation")
	})
}

// RA7: an account that may no longer hold a session is refused with the
// refresh route's answer, before the password is read.
func TestReauth_InactiveAccountIsRefused(t *testing.T) {
	for name, deactivate := range map[string]func(u *domain.User){
		"deactivated": func(u *domain.User) { u.Status = domain.UserStatusDeactivated },
		"suspended":   func(u *domain.User) { u.Status = domain.UserStatusSuspended },
		"deleted": func(u *domain.User) {
			at := time.Now()
			u.DeletedAt = &at
		},
	} {
		t.Run(name, func(t *testing.T) {
			store := &rotationStore{}
			f := newReauthFixture(t, store, nil)
			deactivate(f.users.user)

			status, body := postReauth(t, f.app, f.access, issuancePassword, f.refresh)
			assert.Equal(t, fiber.StatusUnauthorized, status)
			assert.Equal(t, map[string]any{"error": "Account is not active"}, body)
			f.assertNothingIssued(t, body)
			assert.Zero(t, store.writes, "nothing retired")
		})
	}
}

// RA8: without a revocation store nothing can be retired, so, as at refresh,
// the presented refresh token comes back unchanged (rotated false) with an
// access token that carries the new auth_time.
func TestReauth_WithoutARevocationStoreTheRefreshTokenIsReturned(t *testing.T) {
	f := newReauthFixture(t, nil, nil)
	start := time.Now().Truncate(time.Second)

	status, body := postReauth(t, f.app, f.access, issuancePassword, f.refresh)
	require.Equal(t, fiber.StatusOK, status, body)
	assert.Equal(t, false, body["rotated"])
	assert.Equal(t, f.refresh, body["refreshToken"], "the presented refresh token comes back unchanged")
	access, _ := body["accessToken"].(string)
	a := f.claimsOf(t, access)
	assert.Equal(t, f.sid, a.SessionID)
	require.NotNil(t, a.AuthTime)
	assert.False(t, a.AuthTime.Time.Before(start), "the access token carries the re-authentication's auth_time")
	require.Len(t, auditRowsOf(f.audit.rows, domain.AuditActionLogin), 1)
}

// RA9: a re-authentication and a refresh that present the same refresh token
// at once both read it as live; the refresh retires it first, so the
// re-authentication loses the retirement and is a reuse. Its pair is not
// handed out, the session is revoked and the reuse recorded.
func TestReauth_LosingTheRetirementToARefreshIsAReuse(t *testing.T) {
	store := &raceStore{}
	f := newReauthFixture(t, store, nil)
	refreshed, status := postRefresh(t, f.app, f.refresh)
	require.Equal(t, fiber.StatusOK, status)
	require.True(t, refreshed.Rotated)

	// The re-authentication's three reads (the bearer, the refresh token, the
	// session) come back as they did before the refresh's write landed.
	store.blindReads = 3
	status, body := postReauth(t, f.app, f.access, issuancePassword, f.refresh)
	assert.Equal(t, fiber.StatusUnauthorized, status)
	assert.Equal(t, map[string]any{"error": "Token has been revoked or is invalid"}, body)
	assert.Zero(t, store.blindReads, "control: the re-authentication read the store three times before it retired")
	assert.NotContains(t, body, "accessToken")
	assert.True(t, store.data["revoked:fam:"+f.sid], "the session was revoked")
	require.Len(t, auditRowsOf(f.audit.rows, domain.AuditActionRefreshTokenReuse), 1)

	_, status = postRefreshRaw(t, f.app, refreshed.RefreshToken)
	assert.Equal(t, fiber.StatusUnauthorized, status, "the refreshed pair ended with the session")
}

// RA10: a login refresh token with an sdk_tokens row (a recovered SDK
// credential) keeps its row in step with the rotation, so the user's SDK token
// list names the live token.
func TestReauth_RecoveredSDKCredentialRowFollowsTheRotation(t *testing.T) {
	repo := &rotationSDKRepo{}
	f := newReauthFixture(t, &rotationStore{}, repo)
	repo.token = &domain.SDKToken{
		ID: uuid.New(), UserID: f.user.ID, OrganizationID: f.user.OrganizationID, ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	status, body := postReauth(t, f.app, f.access, issuancePassword, f.refresh)
	require.Equal(t, fiber.StatusOK, status, body)
	next, _ := body["refreshToken"].(string)
	require.Len(t, repo.revoked, 1)
	assert.Equal(t, tokenHash(f.refresh), repo.revoked[0].hash, "the presented token's row is retired")
	assert.Equal(t, "token_rotation", repo.revoked[0].reason)
	require.Len(t, repo.created, 1)
	assert.Equal(t, tokenHash(next), repo.created[0].TokenHash, "the successor has a row")
	assert.Equal(t, f.claimsOf(t, next).ID, repo.created[0].TokenID)
	assert.Equal(t, f.user.ID, repo.created[0].UserID)
}

// RA11: the body must carry both fields.
func TestReauth_RequiresPasswordAndRefreshToken(t *testing.T) {
	f := newReauthFixture(t, &rotationStore{}, nil)
	for _, tc := range []struct{ name, raw, want string }{
		{"not JSON", "password", "Invalid request body"},
		{"no password", `{"refreshToken":"x"}`, "password and refreshToken are required"},
		{"no refresh token", `{"password":"x"}`, "password and refreshToken are required"},
	} {
		status, body := postReauthBody(t, f.app, f.access, []byte(tc.raw))
		assert.Equal(t, fiber.StatusBadRequest, status, tc.name)
		assert.Equal(t, map[string]any{"error": tc.want}, body, tc.name)
	}
	assert.Empty(t, f.failures.failures)
}
