package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/middleware"
)

// A password change ends every session signed in before it: the access token
// of an earlier sign-in is refused by the auth middleware, and its refresh
// token and an SDK-download token minted before the change are refused at the
// refresh route; the session that made the change is issued a new pair.
// While an account must change its password, the auth middleware admits its
// session only to GET /api/v1/auth/me and POST /api/v1/auth/change-password
// (logout reads no session through it); every other route behind it answers
// 403 with the code passwordChangeRequired, and the refresh route does too.

const (
	pwChangeOldPassword = "OldPassword1!"
	pwChangeNewPassword = "NewPassword2!"
)

// pwChangeUserRepo holds one user in memory, as the users table holds it. It
// is also the session-state reader the auth middleware reads the record
// through, as the user repository is in the server.
type pwChangeUserRepo struct {
	domain.UserRepository
	mu   sync.Mutex
	user domain.User
}

func (r *pwChangeUserRepo) GetSessionState(_ context.Context, id uuid.UUID) (*domain.UserSessionState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if id != r.user.ID {
		return nil, domain.ErrUserNotFound
	}
	return &domain.UserSessionState{PasswordChangedAt: r.user.PasswordChangedAt, ForcePasswordChange: r.user.ForcePasswordChange}, nil
}

func (r *pwChangeUserRepo) GetByID(id uuid.UUID) (*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if id != r.user.ID {
		return nil, errors.New("user not found")
	}
	u := r.user
	return &u, nil
}

func (r *pwChangeUserRepo) GetByEmail(email string) (*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if email != r.user.Email {
		return nil, errors.New("user not found")
	}
	u := r.user
	return &u, nil
}

func (r *pwChangeUserRepo) Update(u *domain.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if u.ID != r.user.ID {
		return errors.New("user not found")
	}
	r.user.PasswordHash = u.PasswordHash
	r.user.ForcePasswordChange = u.ForcePasswordChange
	// As UserRepository.Update's query: the recorded change only moves forward.
	if u.PasswordChangedAt != nil && (r.user.PasswordChangedAt == nil || u.PasswordChangedAt.After(*r.user.PasswordChangedAt)) {
		at := *u.PasswordChangedAt
		r.user.PasswordChangedAt = &at
	}
	return nil
}

func (r *pwChangeUserRepo) UpdateLastLogin(uuid.UUID, time.Time) error { return nil }

func (r *pwChangeUserRepo) passwordChangedAt() *time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.user.PasswordChangedAt
}

// pwChangeOrgRepo answers the organization lookup /auth/me makes best-effort.
type pwChangeOrgRepo struct {
	domain.OrganizationRepository
}

func (pwChangeOrgRepo) GetByID(uuid.UUID) (*domain.Organization, error) {
	return nil, errors.New("organization not found")
}

// newPasswordChangeUser is an active account whose password is
// pwChangeOldPassword, hashed at bcrypt's minimum cost (a verification reads
// the cost from the hash; see issuanceUser).
func newPasswordChangeUser(t *testing.T, forceChange bool) *pwChangeUserRepo {
	t.Helper()
	hashed, err := bcrypt.GenerateFromPassword([]byte(pwChangeOldPassword), bcrypt.MinCost)
	require.NoError(t, err)
	hash := string(hashed)
	return &pwChangeUserRepo{user: domain.User{
		ID:                  uuid.New(),
		OrganizationID:      uuid.New(),
		Email:               "pw-change@example.com",
		Name:                "Password Change",
		Role:                domain.RoleAdmin,
		Provider:            "local",
		Status:              domain.UserStatusActive,
		PasswordHash:        &hash,
		ForcePasswordChange: forceChange,
	}}
}

// pwChangeEnv is the server under test: the routes as cmd/server/main.go
// mounts them, with the user record attached as main.go attaches it.
type pwChangeEnv struct {
	app   *fiber.App
	jwt   *auth.JWTService
	users *pwChangeUserRepo
	// sdkRows maps the hash of each SDK-download token that has an active
	// sdk_tokens row to its family's birth, the row's created_at.
	sdkMu   sync.Mutex
	sdkRows map[string]time.Time
}

// newPasswordChangeApp mounts the refresh route and the public login and
// forced change without a session, /auth/me and /auth/change-password behind
// the auth middleware, and one route of another group behind it too.
func newPasswordChangeApp(t *testing.T, users *pwChangeUserRepo) *pwChangeEnv {
	t.Helper()
	t.Setenv("JWT_SECRET", "test-secret-key-for-unit-tests-32")
	env := &pwChangeEnv{jwt: auth.NewJWTService(), users: users, sdkRows: map[string]time.Time{}}
	env.jwt.SetSessionStateReader(users)
	authSvc := application.NewAuthService(users, pwChangeOrgRepo{}, nil, nil, nil, nil)
	authHandler := NewAuthHandler(authSvc, env.jwt, pwChangeOrgRepo{}, nil, nil)
	sdkRepo := &refreshTestSDKRepo{getByTokenHash: func(hash string) (*domain.SDKToken, error) {
		env.sdkMu.Lock()
		defer env.sdkMu.Unlock()
		born, ok := env.sdkRows[hash]
		if !ok {
			return nil, errors.New("not tracked")
		}
		return &domain.SDKToken{
			ID:             uuid.New(),
			UserID:         users.user.ID,
			OrganizationID: users.user.OrganizationID,
			CreatedAt:      born,
			ExpiresAt:      time.Now().Add(24 * time.Hour),
		}, nil
	}}
	refresh := NewAuthRefreshHandler(env.jwt, application.NewSDKTokenService(sdkRepo), users, application.NewAuditService(&familyAuditRepo{}))
	public := &PublicRegistrationHandler{authService: authSvc, jwtService: env.jwt}

	env.app = fiber.New()
	env.app.Post("/api/v1/public/login", public.Login)
	env.app.Post("/api/v1/public/change-password", public.ChangePassword)
	env.app.Post("/api/v1/auth/refresh", refresh.RefreshToken)
	authProtected := env.app.Group("/api/v1/auth")
	authProtected.Use(middleware.AuthMiddleware(env.jwt))
	authProtected.Get("/me", authHandler.Me)
	authProtected.Post("/change-password", authHandler.ChangePassword)
	authProtected.Post("/reauth", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	agents := env.app.Group("/api/v1/agents")
	agents.Use(middleware.AuthMiddleware(env.jwt))
	agents.Post("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusCreated) })
	return env
}

// sdkToken mints an SDK-download token for the user and gives it an active
// row, as a download does.
func (e *pwChangeEnv) sdkToken(t *testing.T) string {
	t.Helper()
	return e.sdkTokenOfFamilyBornAt(t, time.Now())
}

// sdkTokenOfFamilyBornAt mints an SDK-download token whose active row says its
// family was born at born, as a member a rotation minted carries its
// family's download time.
func (e *pwChangeEnv) sdkTokenOfFamilyBornAt(t *testing.T, born time.Time) string {
	t.Helper()
	token, err := e.jwt.GenerateSDKRefreshToken(e.users.user.ID.String(), e.users.user.OrganizationID.String(), e.users.user.Email, "admin")
	require.NoError(t, err)
	sum := sha256.Sum256([]byte(token))
	e.sdkMu.Lock()
	e.sdkRows[hex.EncodeToString(sum[:])] = born
	e.sdkMu.Unlock()
	return token
}

func (e *pwChangeEnv) call(t *testing.T, method, path, bearer, body string) (int, map[string]interface{}) {
	t.Helper()
	var req = httptest.NewRequest(method, path, nil)
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	// A password change hashes the new password at the production bcrypt
	// cost, which can outrun the default one-second test timeout under -race.
	resp, err := e.app.Test(req, fiber.TestConfig{Timeout: 30 * time.Second, FailOnTimeout: true})
	require.NoError(t, err)
	defer resp.Body.Close()
	out := map[string]interface{}{}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func (e *pwChangeEnv) refresh(t *testing.T, refreshToken string) (int, map[string]interface{}) {
	t.Helper()
	return e.call(t, "POST", "/api/v1/auth/refresh", "", fmt.Sprintf(`{"refreshToken":%q}`, refreshToken))
}

func changePasswordBody() string {
	return fmt.Sprintf(`{"currentPassword":%q,"newPassword":%q}`, pwChangeOldPassword, pwChangeNewPassword)
}

// waitForNextSecond returns at the start of the next wall-clock second, so a
// token minted after it signs in at or after the boundary of a change made
// before the call.
func waitForNextSecond() {
	time.Sleep(time.Until(time.Now().Truncate(time.Second).Add(time.Second)))
}

// An access token and a refresh token of a sign-in from before a password
// change are refused after it: 401 on GET /api/v1/auth/me, 401 at the refresh
// route. The session that made the change is refused too, and is answered
// with a new pair that is accepted at once.
func TestPasswordChange_EndsTheSessionsSignedInBeforeIt(t *testing.T) {
	users := newPasswordChangeUser(t, false)
	env := newPasswordChangeApp(t, users)
	id, org := users.user.ID.String(), users.user.OrganizationID.String()
	otherAccess, otherRefresh, err := env.jwt.GenerateTokenPair(id, org, users.user.Email, "admin")
	require.NoError(t, err)
	changingAccess, changingRefresh, err := env.jwt.GenerateTokenPair(id, org, users.user.Email, "admin")
	require.NoError(t, err)

	status, _ := env.call(t, "GET", "/api/v1/auth/me", otherAccess, "")
	require.Equal(t, fiber.StatusOK, status, "control: the session is valid before the change")

	status, body := env.call(t, "POST", "/api/v1/auth/change-password", changingAccess, changePasswordBody())
	require.Equal(t, fiber.StatusOK, status, "the change itself: %v", body)
	require.NotNil(t, users.passwordChangedAt(), "the change is recorded")

	status, body = env.call(t, "GET", "/api/v1/auth/me", otherAccess, "")
	assert.Equal(t, fiber.StatusUnauthorized, status, "an access token signed in before the change")
	assert.Contains(t, body["error"], "password changed")
	status, _ = env.refresh(t, otherRefresh)
	assert.Equal(t, fiber.StatusUnauthorized, status, "a refresh token signed in before the change")

	status, _ = env.call(t, "GET", "/api/v1/auth/me", changingAccess, "")
	assert.Equal(t, fiber.StatusUnauthorized, status, "the changing session's own access token")
	status, _ = env.refresh(t, changingRefresh)
	assert.Equal(t, fiber.StatusUnauthorized, status, "the changing session's own refresh token")
}

// The pair the change answers with is a new sign-in the change does not
// refuse: its access token is accepted on GET /api/v1/auth/me and on another
// group's route at once, in the second of the change, and its refresh token
// rotates at the refresh route.
func TestPasswordChange_TheChangingSessionIsIssuedANewPair(t *testing.T) {
	users := newPasswordChangeUser(t, false)
	env := newPasswordChangeApp(t, users)
	changingAccess, _, err := env.jwt.GenerateTokenPair(users.user.ID.String(), users.user.OrganizationID.String(), users.user.Email, "admin")
	require.NoError(t, err)

	status, body := env.call(t, "POST", "/api/v1/auth/change-password", changingAccess, changePasswordBody())
	require.Equal(t, fiber.StatusOK, status, "%v", body)
	assert.Equal(t, "Password changed successfully", body["message"])
	newAccess, _ := body["accessToken"].(string)
	newRefresh, _ := body["refreshToken"].(string)
	require.NotEmpty(t, newAccess, "the answer carries the new access token")
	require.NotEmpty(t, newRefresh, "the answer carries the new refresh token")

	claims, err := env.jwt.ValidateToken(newAccess)
	require.NoError(t, err)
	require.NotNil(t, claims.AuthTime)
	assert.False(t, claims.AuthTime.Before(auth.PasswordChangeBoundary(*users.passwordChangedAt())),
		"the pair's sign-in is not before the change's boundary")

	status, _ = env.call(t, "GET", "/api/v1/auth/me", newAccess, "")
	assert.Equal(t, fiber.StatusOK, status)
	status, _ = env.call(t, "POST", "/api/v1/agents", newAccess, `{"name":"a"}`)
	assert.Equal(t, fiber.StatusCreated, status)
	status, rotated := env.refresh(t, newRefresh)
	assert.Equal(t, fiber.StatusOK, status, "%v", rotated)
}

// An SDK-download token minted before the change is refused at the refresh
// route, and an access token minted from one before the change is refused
// on GET /api/v1/auth/me: neither carries a sign-in time, so its own iat is
// compared. A download after the change is not refused.
func TestPasswordChange_EndsTheSDKTokensMintedBeforeIt(t *testing.T) {
	users := newPasswordChangeUser(t, false)
	env := newPasswordChangeApp(t, users)
	control := env.sdkToken(t)
	status, out := env.refresh(t, control)
	require.Equal(t, fiber.StatusOK, status, "control: an SDK-download token refreshes before the change: %v", out)
	sdkAccess, _ := out["accessToken"].(string)
	require.NotEmpty(t, sdkAccess)
	before := env.sdkToken(t)
	changingAccess, _, err := env.jwt.GenerateTokenPair(users.user.ID.String(), users.user.OrganizationID.String(), users.user.Email, "admin")
	require.NoError(t, err)

	status, body := env.call(t, "POST", "/api/v1/auth/change-password", changingAccess, changePasswordBody())
	require.Equal(t, fiber.StatusOK, status, "%v", body)

	status, _ = env.refresh(t, before)
	assert.Equal(t, fiber.StatusUnauthorized, status, "an SDK-download token minted before the change")
	status, _ = env.call(t, "GET", "/api/v1/auth/me", sdkAccess, "")
	assert.Equal(t, fiber.StatusUnauthorized, status, "an access token minted from an SDK-download token before the change")

	waitForNextSecond()
	after := env.sdkToken(t)
	status, out = env.refresh(t, after)
	assert.Equal(t, fiber.StatusOK, status, "an SDK-download token minted after the change: %v", out)
}

// A member of an SDK-download family downloaded before the change is refused
// at the refresh route by its family's birth, even when its own iat is after
// the change, as a member minted by a rotation in flight at the change is.
func TestPasswordChange_EndsTheSDKFamiliesDownloadedBeforeIt(t *testing.T) {
	users := newPasswordChangeUser(t, false)
	env := newPasswordChangeApp(t, users)
	changingAccess, _, err := env.jwt.GenerateTokenPair(users.user.ID.String(), users.user.OrganizationID.String(), users.user.Email, "admin")
	require.NoError(t, err)
	status, body := env.call(t, "POST", "/api/v1/auth/change-password", changingAccess, changePasswordBody())
	require.Equal(t, fiber.StatusOK, status, "%v", body)
	changedAt := *users.passwordChangedAt()

	waitForNextSecond()
	rotatedMember := env.sdkTokenOfFamilyBornAt(t, changedAt.Add(-time.Minute))
	require.False(t, auth.SignedInBeforePasswordChange(mustClaims(t, env.jwt, rotatedMember), &changedAt),
		"the member's own iat is after the change")

	status, _ = env.refresh(t, rotatedMember)
	assert.Equal(t, fiber.StatusUnauthorized, status, "a member of a family downloaded before the change")

	downloadedAfter := env.sdkTokenOfFamilyBornAt(t, time.Now())
	status, out := env.refresh(t, downloadedAfter)
	assert.Equal(t, fiber.StatusOK, status, "control: a family downloaded after the change: %v", out)
}

func mustClaims(t *testing.T, jwtSvc *auth.JWTService, token string) *auth.JWTClaims {
	t.Helper()
	claims, err := jwtSvc.ValidateToken(token)
	require.NoError(t, err)
	return claims
}

// A token whose sign-in falls in the second of the change is refused: its
// whole-second times cannot tell a token minted just before the change from
// one minted just after it.
func TestPasswordChange_RefusesATokenOfTheSecondOfTheChange(t *testing.T) {
	users := newPasswordChangeUser(t, false)
	env := newPasswordChangeApp(t, users)
	sameSecond, _, err := env.jwt.GenerateTokenPair(users.user.ID.String(), users.user.OrganizationID.String(), users.user.Email, "admin")
	require.NoError(t, err)
	status, _ := env.call(t, "GET", "/api/v1/auth/me", sameSecond, "")
	require.Equal(t, fiber.StatusOK, status, "control: the token is valid before the change")

	at := mustSignedInAt(t, env.jwt, sameSecond).Add(500 * time.Millisecond)
	users.mu.Lock()
	users.user.PasswordChangedAt = &at
	users.mu.Unlock()

	status, _ = env.call(t, "GET", "/api/v1/auth/me", sameSecond, "")
	assert.Equal(t, fiber.StatusUnauthorized, status)
}

func mustSignedInAt(t *testing.T, jwtSvc *auth.JWTService, token string) time.Time {
	t.Helper()
	claims, err := jwtSvc.ValidateToken(token)
	require.NoError(t, err)
	return claims.SignedInAt()
}

// The forced change at first sign-in (POST /api/v1/public/change-password)
// answers with a pair that is accepted at once, and the token the forced
// sign-in issued is refused after it.
func TestForcedPasswordChange_ThePublicChangeIssuesAPairTheChangeKeeps(t *testing.T) {
	users := newPasswordChangeUser(t, true)
	env := newPasswordChangeApp(t, users)
	status, login := env.call(t, "POST", "/api/v1/public/login", "",
		fmt.Sprintf(`{"email":%q,"password":%q}`, users.user.Email, pwChangeOldPassword))
	require.Equal(t, fiber.StatusOK, status, "%v", login)
	restricted, _ := login["accessToken"].(string)
	require.NotEmpty(t, restricted)

	status, changed := env.call(t, "POST", "/api/v1/public/change-password", "",
		fmt.Sprintf(`{"email":%q,"oldPassword":%q,"newPassword":%q}`, users.user.Email, pwChangeOldPassword, pwChangeNewPassword))
	require.Equal(t, fiber.StatusOK, status, "%v", changed)
	access, _ := changed["accessToken"].(string)
	require.NotEmpty(t, access)

	status, _ = env.call(t, "POST", "/api/v1/agents", access, `{"name":"a"}`)
	assert.Equal(t, fiber.StatusCreated, status, "the pair the forced change issues")
	status, _ = env.call(t, "GET", "/api/v1/auth/me", restricted, "")
	assert.Equal(t, fiber.StatusUnauthorized, status, "the forced sign-in's token, signed in before the change")
}

// A sign-in that must change its password is admitted only to GET
// /api/v1/auth/me and POST /api/v1/auth/change-password: the token the public
// login issues for it is answered 403 with the code passwordChangeRequired on
// POST /api/v1/agents and on another route of the auth group, its refresh
// token is answered the same at the refresh route, and the two routes admit
// it.
func TestForcedPasswordChange_RefusesEveryOtherRoute(t *testing.T) {
	users := newPasswordChangeUser(t, true)
	env := newPasswordChangeApp(t, users)

	status, login := env.call(t, "POST", "/api/v1/public/login", "",
		fmt.Sprintf(`{"email":%q,"password":%q}`, users.user.Email, pwChangeOldPassword))
	require.Equal(t, fiber.StatusOK, status, "%v", login)
	require.Equal(t, true, login["requiresPasswordChange"])
	access, _ := login["accessToken"].(string)
	refresh, _ := login["refreshToken"].(string)
	require.NotEmpty(t, access)
	require.NotEmpty(t, refresh)

	status, body := env.call(t, "POST", "/api/v1/agents", access, `{"name":"a"}`)
	assert.Equal(t, fiber.StatusForbidden, status)
	assert.Equal(t, "passwordChangeRequired", body["code"])
	status, body = env.call(t, "POST", "/api/v1/auth/reauth", access, `{}`)
	assert.Equal(t, fiber.StatusForbidden, status, "another route of the auth group")
	assert.Equal(t, "passwordChangeRequired", body["code"])
	status, body = env.refresh(t, refresh)
	assert.Equal(t, fiber.StatusForbidden, status, "the refresh route")
	assert.Equal(t, "passwordChangeRequired", body["code"])

	status, _ = env.call(t, "GET", "/api/v1/auth/me", access, "")
	assert.Equal(t, fiber.StatusOK, status, "GET /api/v1/auth/me admits it")
	status, changed := env.call(t, "POST", "/api/v1/auth/change-password", access, changePasswordBody())
	require.Equal(t, fiber.StatusOK, status, "POST /api/v1/auth/change-password admits it: %v", changed)

	newAccess, _ := changed["accessToken"].(string)
	require.NotEmpty(t, newAccess)
	status, _ = env.call(t, "POST", "/api/v1/agents", newAccess, `{"name":"a"}`)
	assert.Equal(t, fiber.StatusCreated, status, "after the change, the new pair reaches every route")
}
