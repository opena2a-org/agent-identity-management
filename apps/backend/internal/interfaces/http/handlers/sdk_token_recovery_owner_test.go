package handlers

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

// The SDK credential recovery route mints only for the acting principal: the
// bearer's user and organisation (set by the auth middleware) must be the
// revoked token's owner. Any other account presenting that token is answered
// exactly as not-found, so the route is neither a way to resurrect another
// user's credential nor an oracle for which tokens exist.

type recoveryOwnerFixture struct {
	app   *fiber.App
	repo  *rotationSDKRepo
	svc   *auth.JWTService
	owner uuid.UUID
	org   uuid.UUID
	old   string
}

func newRecoveryOwnerFixture(t *testing.T, acting func(c fiber.Ctx)) *recoveryOwnerFixture {
	t.Helper()
	t.Setenv("JWT_SECRET", "test-secret-key-for-unit-tests-32")
	svc := auth.NewJWTService()
	owner, org := uuid.New(), uuid.New()
	repo := &rotationSDKRepo{token: revokedSDKToken(owner, org)}
	users := &refreshTestUserRepo{getByID: func(id uuid.UUID) (*domain.User, error) {
		return activeUser(owner, org, domain.RoleAdmin, "owner@example.com"), nil
	}}
	h := NewSDKTokenRecoveryHandler(application.NewSDKTokenService(repo), svc, users)
	app := fiber.New()
	app.Post("/auth/sdk/recover", func(c fiber.Ctx) error {
		if acting != nil {
			acting(c)
		}
		return h.RecoverRevokedToken(c)
	})
	old, err := svc.GenerateSDKRefreshToken(owner.String(), org.String(), "owner@example.com", "admin")
	require.NoError(t, err)
	return &recoveryOwnerFixture{app: app, repo: repo, svc: svc, owner: owner, org: org, old: old}
}

func (f *recoveryOwnerFixture) recover(t *testing.T) (int, string) {
	t.Helper()
	req := httptest.NewRequest("POST", "/auth/sdk/recover", strings.NewReader(`{"oldRefreshToken":"`+f.old+`"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := f.app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	buf := make([]byte, 4096)
	n, _ := resp.Body.Read(buf)
	return resp.StatusCode, string(buf[:n])
}

// O1: another account presenting the owner's revoked token is answered as not-found, with no pair and no row.
func TestRecoverRevokedToken_AnotherAccountIsAnsweredAsNotFound(t *testing.T) {
	other, otherOrg := uuid.New(), uuid.New()
	f := newRecoveryOwnerFixture(t, func(c fiber.Ctx) { c.Locals("user_id", other); c.Locals("organization_id", otherOrg) })
	status, body := f.recover(t)
	assert.Equal(t, fiber.StatusNotFound, status)
	assert.Contains(t, body, "Token not found")
	assert.NotContains(t, body, "refreshToken")
	assert.Empty(t, f.repo.created, "no recovered credential row")
}

// O2: the same user in another organisation is refused the same way.
func TestRecoverRevokedToken_SameUserOtherOrganisationIsNotFound(t *testing.T) {
	var f *recoveryOwnerFixture
	f = newRecoveryOwnerFixture(t, func(c fiber.Ctx) { c.Locals("user_id", f.owner); c.Locals("organization_id", uuid.New()) })
	status, _ := f.recover(t)
	assert.Equal(t, fiber.StatusNotFound, status)
	assert.Empty(t, f.repo.created)
}

// O3: no principal at all (the route reached without the middleware) mints nothing.
func TestRecoverRevokedToken_NoPrincipalIsNotFound(t *testing.T) {
	f := newRecoveryOwnerFixture(t, nil)
	status, _ := f.recover(t)
	assert.Equal(t, fiber.StatusNotFound, status)
	assert.Empty(t, f.repo.created)
}

// O4 (control): the owner recovers.
func TestRecoverRevokedToken_OwnerRecovers(t *testing.T) {
	var f *recoveryOwnerFixture
	f = newRecoveryOwnerFixture(t, func(c fiber.Ctx) { c.Locals("user_id", f.owner); c.Locals("organization_id", f.org) })
	status, body := f.recover(t)
	assert.Equal(t, fiber.StatusOK, status)
	assert.Contains(t, body, "refreshToken")
	assert.Len(t, f.repo.created, 1)
}
