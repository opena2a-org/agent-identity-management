package handlers

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/integrations/siem"
)

// The user agent a login, logout or refresh refusal row records is the one
// the request that wrote the row sent. A header value is a view of the
// request buffer, which the server reuses for later requests, and the event
// SIEM forwarding queues from a row is serialized after the request ended.

const (
	// rowUserAgent is the user agent postProxied sends.
	rowUserAgent = "mint-cell/1"
	// laterUserAgent has the length of rowUserAgent, so writing it over the
	// header reuses that value's bytes.
	laterUserAgent = "QQQQQQQQQQQ"
)

// eventUserAgent is the userAgent the serialized SIEM event carries.
func eventUserAgent(t *testing.T, event *siem.SIEMEvent) interface{} {
	t.Helper()
	raw, err := json.Marshal(event.Details)
	require.NoError(t, err)
	var details map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &details))
	return details["userAgent"]
}

// The reproduction from the report: a sign-in through POST /public/login, the
// SIEM event of its login row, then 50 further requests to the same app with
// another user agent. The serialized event names the sign-in's user agent.
func TestSignInAudit_SIEMEventKeepsTheUserAgentAfterLaterRequests(t *testing.T) {
	user := issuanceUser(t, false)
	app, _, audit := publicLoginApp(t, user)
	status, body := postProxied(t, app, "/public/login", signInLoginBody(), "", "")
	require.Equal(t, fiber.StatusOK, status, body)
	logins := rowsWithAction(audit.rows, domain.AuditActionLogin)
	require.Len(t, logins, 1, "exactly one login row")
	event := siem.EventFromAuditLog(logins[0])

	for i := 0; i < 50; i++ {
		req := httptest.NewRequest("GET", "/public/login", nil)
		req.Header.Set("User-Agent", laterUserAgent)
		req.Header.Set("Authorization", "Bearer QQQQQ")
		resp, err := app.Test(req)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
	}

	assert.Equal(t, rowUserAgent, eventUserAgent(t, event), "the queued event")
	assert.Equal(t, rowUserAgent, logins[0].UserAgent, "the audit row")
}

// The login and logout rows keep the user agent after the request buffer is
// written over.
func TestSignInAudit_RowsKeepTheUserAgentWhenTheRequestBufferIsReused(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "")
	t.Setenv("JWT_SECRET", "test-secret-key-for-unit-tests-32")

	user := issuanceUser(t, false)
	audit := &familyAuditRepo{}
	users := &issuanceUserRepo{user: user}
	h := NewPublicRegistrationHandler(application.NewRegistrationService(refusalRegistrationRepo{}, users, nil, nil, nil), application.NewAuthService(users, nil, nil, nil, nil, nil), auth.NewJWTService(), application.NewAuditService(audit))
	loginApp := fiber.New()
	loginApp.Use(overwriteAfter("User-Agent", laterUserAgent))
	loginApp.Post("/public/login", h.Login)
	status, body := postProxied(t, loginApp, "/public/login", signInLoginBody(), "", "")
	require.Equal(t, fiber.StatusOK, status, body)
	logins := rowsWithAction(audit.rows, domain.AuditActionLogin)
	require.Len(t, logins, 1, "exactly one login row")
	assert.Equal(t, rowUserAgent, logins[0].UserAgent, "login row")

	lh, svc, repo, userID, orgID := logoutAuditFixture(t)
	access, refresh, err := svc.GenerateTokenPair(userID.String(), orgID.String(), "audit@example.com", "admin")
	require.NoError(t, err)
	logoutApp := fiber.New()
	logoutApp.Use(overwriteAfter("User-Agent", laterUserAgent))
	logoutApp.Post("/api/v1/auth/logout", lh.Logout)
	status, body = postProxied(t, logoutApp, "/api/v1/auth/logout", `{"refreshToken":"`+refresh+`"}`, access, "")
	require.Equal(t, fiber.StatusOK, status, body)
	logouts := rowsWithAction(repo.rows, domain.AuditActionLogout)
	require.Len(t, logouts, 1, "exactly one logout row")
	assert.Equal(t, rowUserAgent, logouts[0].UserAgent, "logout row")
}

// The refresh route's reuse row and the refusal row of the family it revoked
// keep the user agent after the request buffer is written over.
func TestAuthRefresh_RefusalRowsKeepTheUserAgentWhenTheRequestBufferIsReused(t *testing.T) {
	t.Setenv("TRUSTED_PROXIES", "")
	t.Setenv("JWT_SECRET", "test-secret-key-for-unit-tests-32")

	userID, orgID := uuid.New(), uuid.New()
	users := &refreshTestUserRepo{getByID: func(uuid.UUID) (*domain.User, error) {
		return activeUser(userID, orgID, domain.RoleAdmin, "agent-cell@example.com"), nil
	}}
	audit := &familyAuditRepo{}
	svc := auth.NewJWTService()
	svc.SetRevoker(auth.NewTokenRevoker(&valueStore{}, false))
	h := NewAuthRefreshHandler(svc, application.NewSDKTokenService(&refreshTestSDKRepo{}), users, application.NewAuditService(audit))
	app := fiber.New()
	app.Use(overwriteAfter("User-Agent", laterUserAgent))
	app.Post("/auth/refresh", h.RefreshToken)

	_, p1, err := svc.GenerateTokenPair(userID.String(), orgID.String(), "agent-cell@example.com", "admin")
	require.NoError(t, err)
	refresh := func(token string) (string, int) {
		return cmPost(t, app, "/auth/refresh", fmt.Sprintf(`{"refreshToken":%q}`, token), cmAddrA, rowUserAgent)
	}

	body, status := refresh(p1)
	require.Equal(t, fiber.StatusOK, status, body)
	var rotated RefreshTokenResponse
	require.NoError(t, json.Unmarshal([]byte(body), &rotated))
	require.NotEmpty(t, rotated.RefreshToken)

	body, status = refresh(p1)
	require.Equal(t, fiber.StatusUnauthorized, status, body)
	body, status = refresh(rotated.RefreshToken)
	require.Equal(t, fiber.StatusUnauthorized, status, body)

	for _, action := range []domain.AuditAction{domain.AuditActionRefreshTokenReuse, domain.AuditActionRefreshSessionRevoked} {
		rows := rowsWithAction(audit.rows, action)
		require.Len(t, rows, 1, "exactly one %s row", action)
		assert.Equal(t, rowUserAgent, rows[0].UserAgent, "%s row", action)
	}
}
