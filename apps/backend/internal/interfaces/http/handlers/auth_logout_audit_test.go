package handlers

import (
	"context"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

// The logout audit row. POST /api/v1/auth/logout sits in the public auth
// group, so no middleware sets the principal; the row is written from the
// presented token's own claims (the bearer access token, or the refresh token
// from the body or cookie), once per logout with a valid token, never for
// garbage tokens.

func logoutAuditFixture(t *testing.T) (*AuthHandler, *auth.JWTService, *familyAuditRepo, uuid.UUID, uuid.UUID) {
	t.Helper()
	svc := logoutTestService(t, true)
	repo := &familyAuditRepo{}
	userID, orgID := uuid.New(), uuid.New()
	h := &AuthHandler{jwtService: svc, auditService: application.NewAuditService(repo)}
	return h, svc, repo, userID, orgID
}

// LA1: a logout with a valid bearer records one row from the token's claims.
func TestAuthHandler_Logout_AuditRowFromTheBearer(t *testing.T) {
	h, svc, repo, userID, orgID := logoutAuditFixture(t)
	access, refresh, err := svc.GenerateTokenPair(userID.String(), orgID.String(), "audit@example.com", "admin")
	require.NoError(t, err)
	app := fiber.New()
	app.Post("/api/v1/auth/logout", h.Logout)
	_, _, out, status := postLogout(t, h, access, `{"refreshToken":"`+refresh+`"}`, "")
	require.Equal(t, fiber.StatusOK, status)
	assert.True(t, out.Revoked["accessToken"])
	require.Len(t, repo.rows, 1, "one logout row")
	row := repo.rows[0]
	assert.Equal(t, domain.AuditActionLogout, row.Action)
	require.NotNil(t, row.UserID)
	assert.Equal(t, userID, *row.UserID)
	assert.Equal(t, orgID, row.OrganizationID)
	assert.Equal(t, "logout-cell/1", row.UserAgent, "the request's user agent")
	assert.Equal(t, "access", row.Metadata["token"])
	assert.NotContains(t, marshalled(t, row.Metadata), "eyJ", "no token in the row")
}

// LA2: a logout with only a refresh token (the SDK's shape) records one row from it.
func TestAuthHandler_Logout_AuditRowFromTheRefreshToken(t *testing.T) {
	h, svc, repo, userID, orgID := logoutAuditFixture(t)
	_, refresh, err := svc.GenerateTokenPair(userID.String(), orgID.String(), "audit@example.com", "admin")
	require.NoError(t, err)
	_, _, _, status := postLogout(t, h, "", `{"refreshToken":"`+refresh+`"}`, "")
	require.Equal(t, fiber.StatusOK, status)
	require.Len(t, repo.rows, 1)
	assert.Equal(t, userID, *repo.rows[0].UserID)
	assert.Equal(t, orgID, repo.rows[0].OrganizationID)
	assert.Equal(t, "refresh", repo.rows[0].Metadata["token"])
	assert.True(t, svc.IsRevoked(context.Background(), jtiOf(t, svc, refresh)), "the revocation is unchanged")
}

// LA3: garbage tokens, or none, record nothing.
func TestAuthHandler_Logout_NoRowWithoutAValidToken(t *testing.T) {
	h, _, repo, _, _ := logoutAuditFixture(t)
	_, _, _, status := postLogout(t, h, "not.a.token", `{"refreshToken":"garbage"}`, "")
	assert.Equal(t, fiber.StatusOK, status)
	_, _, _, status = postLogout(t, h, "", "", "")
	assert.Equal(t, fiber.StatusOK, status)
	assert.Empty(t, repo.rows)
}
