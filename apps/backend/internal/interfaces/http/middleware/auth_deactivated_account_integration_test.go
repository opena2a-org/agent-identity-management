//go:build integration

package middleware

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
)

// Deactivating an account ends its sessions, held against PostgreSQL with
// every migration applied. AuthMiddleware reads the user record behind a
// login access token on every request through UserRepository.GetSessionState.
// The admin's deactivation (POST /api/v1/admin/users/:id/deactivate) writes
// status 'deactivated' through UserRepository.Update, and sign-in and refresh
// refuse such an account, but its access tokens were admitted until they
// expired (JWT_ACCESS_TTL, 2h by default).
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestAuthMiddleware_DeactivatedAccount ./internal/interfaces/http/middleware/...

func TestAuthMiddleware_DeactivatedAccountsSessionIsRefusedOnItsNextRequest(t *testing.T) {
	db := revocationTestDB(t)
	ctx := context.Background()
	agentID := seedAgentWithStatus(t, db, ctx, "verified", "unused")
	var userID, orgID uuid.UUID
	var email string
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT u.id, u.organization_id, u.email FROM agents a JOIN users u ON u.id = a.created_by WHERE a.id = $1`,
		agentID).Scan(&userID, &orgID, &email))

	t.Setenv("JWT_SECRET", "test-secret-key-for-testing-purposes-32chars")
	svc := auth.NewJWTService()
	users := repository.NewUserRepository(db)
	svc.SetSessionStateReader(users)
	access, _, err := svc.GenerateTokenPair(userID.String(), orgID.String(), email, "admin")
	require.NoError(t, err)

	app := fiber.New()
	app.Use(AuthMiddleware(svc))
	app.Get("/api/v1/auth/me", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	get := func() int {
		req := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
		req.Header.Set("Authorization", "Bearer "+access)
		resp, err := app.Test(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		return resp.StatusCode
	}

	require.Equal(t, fiber.StatusOK, get(), "control: the session acts while the account is active")

	user, err := users.GetByID(userID)
	require.NoError(t, err)
	user.Status = "deactivated"
	require.NoError(t, users.Update(user), "the write the admin's deactivation makes")
	assert.Equal(t, fiber.StatusUnauthorized, get(), "a deactivated account's session is refused")

	user.Status = "active"
	require.NoError(t, users.Update(user), "the write the admin's activation makes")
	assert.Equal(t, fiber.StatusOK, get(), "the session acts again once the account is activated")
}
