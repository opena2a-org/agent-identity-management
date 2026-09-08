package handlers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

// A refresh token is an identity handle: the minted pair's role and email come
// from the user record at refresh time, and an account that may not hold a
// session cannot refresh one. Before these tests, RefreshTokenPair copied the
// refresh token's own claims — login-issued tokens carry none, so every
// refreshed browser session became role-less and 403ed at the member gates
// (reproduced on production 2026-08-27).

// refreshTestUserRepo stubs domain.UserRepository; only GetByID is real.
type refreshTestUserRepo struct {
	domain.UserRepository
	getByID func(id uuid.UUID) (*domain.User, error)
}

func (r *refreshTestUserRepo) GetByID(id uuid.UUID) (*domain.User, error) {
	if r.getByID == nil {
		return nil, fmt.Errorf("unexpected GetByID")
	}
	return r.getByID(id)
}

// refreshTestSDKRepo stubs domain.SDKTokenRepository for the rotation path.
type refreshTestSDKRepo struct {
	domain.SDKTokenRepository
	getByTokenHash func(hash string) (*domain.SDKToken, error)
	created        []*domain.SDKToken
}

func (r *refreshTestSDKRepo) GetByTokenHash(hash string) (*domain.SDKToken, error) {
	if r.getByTokenHash == nil {
		return nil, fmt.Errorf("not tracked")
	}
	return r.getByTokenHash(hash)
}
func (r *refreshTestSDKRepo) RecordUsage(tokenID string, ipAddress string) error { return nil }
func (r *refreshTestSDKRepo) RevokeByTokenHash(tokenHash string, reason string) error {
	return nil
}
func (r *refreshTestSDKRepo) Create(token *domain.SDKToken) error {
	r.created = append(r.created, token)
	return nil
}

func decodeRoleEmail(t *testing.T, token string) (string, string) {
	t.Helper()
	parts := strings.Split(token, ".")
	require.Len(t, parts, 3)
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)
	var claims struct {
		Role  string `json:"role"`
		Email string `json:"email"`
	}
	require.NoError(t, json.Unmarshal(payload, &claims))
	return claims.Role, claims.Email
}

func newRefreshTestApp(t *testing.T, users domain.UserRepository, sdkRepo domain.SDKTokenRepository) (*fiber.App, *auth.JWTService) {
	t.Helper()
	t.Setenv("JWT_SECRET", "test-secret-key-for-unit-tests-32")
	jwtSvc := auth.NewJWTService()
	if sdkRepo == nil {
		sdkRepo = &refreshTestSDKRepo{}
	}
	h := NewAuthRefreshHandler(jwtSvc, application.NewSDKTokenService(sdkRepo), users)
	app := fiber.New()
	app.Post("/auth/refresh", h.RefreshToken)
	return app, jwtSvc
}

func postRefresh(t *testing.T, app *fiber.App, refreshToken string) (*RefreshTokenResponse, int) {
	t.Helper()
	body := fmt.Sprintf(`{"refreshToken":%q}`, refreshToken)
	req := httptest.NewRequest("POST", "/auth/refresh", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	var out RefreshTokenResponse
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return &out, resp.StatusCode
}

func activeUser(id, orgID uuid.UUID, role domain.UserRole, email string) *domain.User {
	return &domain.User{
		ID:             id,
		OrganizationID: orgID,
		Email:          email,
		Role:           role,
		Status:         domain.UserStatusActive,
	}
}

// H1: the login-issuer P0 regression — the refreshed pair carries the user's
// CURRENT role and email from the database (here: demoted admin -> member).
func TestRefreshToken_MintsRoleAndEmailFromUserRecord(t *testing.T) {
	userID, orgID := uuid.New(), uuid.New()
	users := &refreshTestUserRepo{getByID: func(id uuid.UUID) (*domain.User, error) {
		require.Equal(t, userID, id)
		return activeUser(userID, orgID, domain.RoleMember, "current@example.com"), nil
	}}
	app, jwtSvc := newRefreshTestApp(t, users, nil)
	_, refresh, err := jwtSvc.GenerateTokenPair(userID.String(), orgID.String(), "old@example.com", "admin")
	require.NoError(t, err)

	out, status := postRefresh(t, app, refresh)
	require.Equal(t, fiber.StatusOK, status)
	role, email := decodeRoleEmail(t, out.AccessToken)
	assert.Equal(t, "member", role, "role must come from the user record, not the old token")
	assert.Equal(t, "current@example.com", email)
}

// H2: SDK-issuer tokens embed a role, but the DB still wins.
func TestRefreshToken_SDKTokenRoleComesFromDB(t *testing.T) {
	userID, orgID := uuid.New(), uuid.New()
	users := &refreshTestUserRepo{getByID: func(uuid.UUID) (*domain.User, error) {
		return activeUser(userID, orgID, domain.RoleViewer, "db@example.com"), nil
	}}
	sdkRepo := &refreshTestSDKRepo{getByTokenHash: func(string) (*domain.SDKToken, error) {
		return &domain.SDKToken{
			ID:             uuid.New(),
			UserID:         userID,
			OrganizationID: orgID,
			ExpiresAt:      time.Now().Add(24 * time.Hour),
		}, nil
	}}
	app, jwtSvc := newRefreshTestApp(t, users, sdkRepo)
	refresh, err := jwtSvc.GenerateSDKRefreshToken(userID.String(), orgID.String(), "sdk@example.com", "admin")
	require.NoError(t, err)

	out, status := postRefresh(t, app, refresh)
	require.Equal(t, fiber.StatusOK, status)
	role, _ := decodeRoleEmail(t, out.AccessToken)
	assert.Equal(t, "viewer", role, "an embedded admin role must not survive when the DB says viewer")
}

// H3: accounts that may not hold a session cannot refresh one. The status set
// is an allow-list: an unrecognised value fails closed.
func TestRefreshToken_RefusedForAccountsThatCannotHoldASession(t *testing.T) {
	userID, orgID := uuid.New(), uuid.New()
	deleted := time.Now()
	cases := map[string]*domain.User{
		"suspended":      {ID: userID, OrganizationID: orgID, Role: domain.RoleAdmin, Status: domain.UserStatusSuspended},
		"deactivated":    {ID: userID, OrganizationID: orgID, Role: domain.RoleAdmin, Status: domain.UserStatusDeactivated},
		"soft-deleted":   {ID: userID, OrganizationID: orgID, Role: domain.RoleAdmin, Status: domain.UserStatusActive, DeletedAt: &deleted},
		"unknown-status": {ID: userID, OrganizationID: orgID, Role: domain.RoleAdmin, Status: domain.UserStatus("zombie")},
	}
	for name, user := range cases {
		t.Run(name, func(t *testing.T) {
			users := &refreshTestUserRepo{getByID: func(uuid.UUID) (*domain.User, error) { return user, nil }}
			app, jwtSvc := newRefreshTestApp(t, users, nil)
			_, refresh, err := jwtSvc.GenerateTokenPair(userID.String(), orgID.String(), "x@example.com", "admin")
			require.NoError(t, err)

			out, status := postRefresh(t, app, refresh)
			assert.Equal(t, fiber.StatusUnauthorized, status)
			assert.Empty(t, out.AccessToken, "a refused refresh must not return a token")
		})
	}
}

// H4: a lookup error is a refusal, never a fallback to the token's claims.
func TestRefreshToken_LookupErrorFailsClosed(t *testing.T) {
	userID, orgID := uuid.New(), uuid.New()
	users := &refreshTestUserRepo{getByID: func(uuid.UUID) (*domain.User, error) {
		return nil, fmt.Errorf("db unavailable")
	}}
	app, jwtSvc := newRefreshTestApp(t, users, nil)
	_, refresh, err := jwtSvc.GenerateTokenPair(userID.String(), orgID.String(), "x@example.com", "admin")
	require.NoError(t, err)

	out, status := postRefresh(t, app, refresh)
	assert.Equal(t, fiber.StatusUnauthorized, status)
	assert.Empty(t, out.AccessToken)
}

// H5: an unknown user is a refusal.
func TestRefreshToken_UnknownUserRefused(t *testing.T) {
	userID, orgID := uuid.New(), uuid.New()
	users := &refreshTestUserRepo{getByID: func(uuid.UUID) (*domain.User, error) { return nil, nil }}
	app, jwtSvc := newRefreshTestApp(t, users, nil)
	_, refresh, err := jwtSvc.GenerateTokenPair(userID.String(), orgID.String(), "x@example.com", "admin")
	require.NoError(t, err)

	_, status := postRefresh(t, app, refresh)
	assert.Equal(t, fiber.StatusUnauthorized, status)
}

// H6: the token's organization must match the user record's.
func TestRefreshToken_OrgMismatchRefused(t *testing.T) {
	userID, orgID := uuid.New(), uuid.New()
	users := &refreshTestUserRepo{getByID: func(uuid.UUID) (*domain.User, error) {
		return activeUser(userID, uuid.New(), domain.RoleAdmin, "x@example.com"), nil
	}}
	app, jwtSvc := newRefreshTestApp(t, users, nil)
	_, refresh, err := jwtSvc.GenerateTokenPair(userID.String(), orgID.String(), "x@example.com", "admin")
	require.NoError(t, err)

	_, status := postRefresh(t, app, refresh)
	assert.Equal(t, fiber.StatusUnauthorized, status)
}

// H7: a service-principal token can never traverse the human refresh path.
func TestRefreshToken_ServiceTokenRefused(t *testing.T) {
	users := &refreshTestUserRepo{getByID: func(uuid.UUID) (*domain.User, error) {
		t.Fatal("a service token must be refused before any user lookup")
		return nil, nil
	}}
	app, jwtSvc := newRefreshTestApp(t, users, nil)
	svcToken, err := jwtSvc.GenerateServiceToken(uuid.New().String(), uuid.New().String())
	require.NoError(t, err)

	_, status := postRefresh(t, app, svcToken)
	assert.Equal(t, fiber.StatusUnauthorized, status)
}
