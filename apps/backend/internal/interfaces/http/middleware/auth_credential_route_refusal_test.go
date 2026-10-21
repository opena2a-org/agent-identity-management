package middleware

import (
	"context"
	"errors"
	"io"
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

// A credential route (an SDK download, a device sign-in approval, an SDK
// credential recovery, an API key's creation, an agent's key read, rotated or
// replaced, a secret's rotation, a webhook's creation, update or secret
// rotation, an alert channel's creation or update) records a refusal of a
// revoked session as credential_mint_refused.
// AuthMiddleware refuses a revoked session's access token before the route
// runs, so it records that refusal itself, under the route's label: one row
// in the JWTService's audit log and one SECURITY line, cause sessionRevoked.

const credentialRefusalUA = "credential-route-cell/1"

// credentialRefusalLog is an auth.AuditLogger that keeps every entry written.
type credentialRefusalLog struct {
	rows []credentialRefusalRow
}

type credentialRefusalRow struct {
	orgID        uuid.UUID
	userID       uuid.UUID
	action       domain.AuditAction
	resourceType string
	resourceID   uuid.UUID
	ip           string
	ua           string
	metadata     map[string]interface{}
}

func (l *credentialRefusalLog) LogAction(_ context.Context, orgID uuid.UUID, userID uuid.UUID, action domain.AuditAction, resourceType string, resourceID uuid.UUID, ipAddress string, userAgent string, metadata map[string]interface{}) error {
	l.rows = append(l.rows, credentialRefusalRow{orgID, userID, action, resourceType, resourceID, ipAddress, userAgent, metadata})
	return nil
}

// credentialRouteApp mounts a handler at the method and path pattern behind
// AuthMiddleware, as the server's groups mount a credential route; the
// handler counts the requests that reach it.
func credentialRouteApp(svc *auth.JWTService, method, pattern string, reached *int) *fiber.App {
	app := fiber.New()
	app.Use(AuthMiddleware(svc))
	app.Add([]string{method}, pattern, func(c fiber.Ctx) error {
		*reached++
		return c.SendStatus(fiber.StatusOK)
	})
	return app
}

// sendBearer sends one request with the bearer token and returns the status
// and the body.
func sendBearer(t *testing.T, app *fiber.App, method, path, token string) (int, []byte) {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", credentialRefusalUA)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, body
}

// R1: at each credential route a login access token whose session was
// revoked is refused with the body a revoked token gets, the route is not
// reached, and the refusal is recorded once under the route's label: one
// credential_mint_refused row with identifiers, the route and the cause
// sessionRevoked, and one SECURITY line. Before the revocation the same
// request reaches the route and records nothing (control).
func TestAuthMiddleware_RecordsARevokedSessionAtEachCredentialRoute(t *testing.T) {
	routes := CredentialRoutes()
	require.NotEmpty(t, routes)
	for _, route := range routes {
		t.Run(route.Label, func(t *testing.T) {
			svc := sessionFamilyService(t, &sessionFamilyStore{}, false)
			audit := &credentialRefusalLog{}
			svc.SetAuditLogger(audit)
			var reached int
			app := credentialRouteApp(svc, route.Method, route.Path, &reached)
			userID, orgID := uuid.New(), uuid.New()
			access, refresh, err := svc.GenerateTokenPair(userID.String(), orgID.String(), "r1@example.com", "admin")
			require.NoError(t, err)
			claims, err := svc.ValidateToken(access)
			require.NoError(t, err)
			require.NotEmpty(t, claims.AccessFamilyID(), "a login access token carries its session")
			path := concretePath(route.Path)

			var status int
			logged := captureLog(t, func() { status, _ = sendBearer(t, app, route.Method, path, access) })
			require.Equal(t, fiber.StatusOK, status, "control: a live session reaches the route")
			require.Equal(t, 1, reached)
			require.Empty(t, audit.rows, "control: a live session records nothing")
			require.NotContains(t, logged, "SECURITY")

			revokeSessionOf(t, svc, refresh)

			var body []byte
			logged = captureLog(t, func() { status, body = sendBearer(t, app, route.Method, path, access) })
			assert.Equal(t, fiber.StatusUnauthorized, status)
			assert.JSONEq(t, `{"error":"Token has been revoked"}`, string(body), "the body a revoked token gets")
			assert.Equal(t, 1, reached, "the route is not reached")
			require.Len(t, audit.rows, 1, "one row for the refused request")
			row := audit.rows[0]
			assert.Equal(t, domain.AuditActionCredentialMintRefused, row.action)
			assert.Equal(t, orgID, row.orgID)
			assert.Equal(t, userID, row.userID)
			assert.Equal(t, "user", row.resourceType)
			assert.Equal(t, userID, row.resourceID)
			assert.NotEmpty(t, row.ip)
			assert.Equal(t, credentialRefusalUA, row.ua)
			assert.Equal(t, map[string]interface{}{
				"familyId": claims.AccessFamilyID(),
				"jti":      claims.ID,
				"route":    route.Label,
				"cause":    "sessionRevoked",
			}, row.metadata)
			assert.Equal(t, 1, strings.Count(logged, "SECURITY credential_mint_refused route="+route.Label+" cause=sessionRevoked "), logged)
			assert.Contains(t, logged, "family="+claims.AccessFamilyID())
			assert.NotContains(t, logged, access, "no token in the line")
		})
	}
}

// R2: a revoked session's request at a route that is not a credential route
// is refused as before and records nothing: a read in a group that holds
// credential routes, another write there, a path that extends a credential
// route's pattern, and a credential route's path under another method.
func TestAuthMiddleware_RevokedSessionAtAnotherRouteRecordsNothing(t *testing.T) {
	svc := sessionFamilyService(t, &sessionFamilyStore{}, false)
	audit := &credentialRefusalLog{}
	svc.SetAuditLogger(audit)
	access, refresh, err := svc.GenerateTokenPair(uuid.New().String(), uuid.New().String(), "r2@example.com", "admin")
	require.NoError(t, err)
	revokeSessionOf(t, svc, refresh)

	for _, r := range []struct{ method, pattern string }{
		{"GET", "/api/v1/agents"},
		{"GET", "/api/v1/webhooks"},
		{"DELETE", "/api/v1/webhooks/:id"},
		{"POST", "/api/v1/webhooks/:id/test"},
		{"GET", "/api/v1/agents/:id/credentials/history"},
		{"POST", "/api/v1/sdk/download"},
	} {
		var reached int
		app := credentialRouteApp(svc, r.method, r.pattern, &reached)
		var status int
		var body []byte
		logged := captureLog(t, func() { status, body = sendBearer(t, app, r.method, concretePath(r.pattern), access) })
		assert.Equal(t, fiber.StatusUnauthorized, status, "%s %s", r.method, r.pattern)
		assert.JSONEq(t, `{"error":"Token has been revoked"}`, string(body), "%s %s", r.method, r.pattern)
		assert.Zero(t, reached, "%s %s", r.method, r.pattern)
		assert.NotContains(t, logged, "SECURITY", "%s %s", r.method, r.pattern)
	}
	assert.Empty(t, audit.rows)
}

// R3: at a credential route a refusal is recorded only when the store
// confirms the session's revocation. A token revoked by its own id while its
// session lives, a token with no session revoked by its own id, and a token
// whose session the store cannot read (the revoker fails closed) are each
// refused as before, with no row and no SECURITY line.
func TestAuthMiddleware_CredentialRouteRecordsOnlyAConfirmedSessionRevocation(t *testing.T) {
	const method, pattern = "GET", "/api/v1/agents/:id/credentials"
	ctx := context.Background()
	userID, orgID := uuid.New().String(), uuid.New().String()

	for _, tc := range []struct {
		name  string
		store *sessionFamilyStore
		token func(t *testing.T, svc *auth.JWTService) string
	}{
		{"revoked by its own id", &sessionFamilyStore{}, func(t *testing.T, svc *auth.JWTService) string {
			access, _, err := svc.GenerateTokenPair(userID, orgID, "r3@example.com", "admin")
			require.NoError(t, err)
			require.NoError(t, svc.RevokeToken(ctx, access))
			return access
		}},
		{"no session", &sessionFamilyStore{}, func(t *testing.T, svc *auth.JWTService) string {
			standalone, err := svc.GenerateAccessToken(userID, orgID, "r3@example.com", "admin")
			require.NoError(t, err)
			claims, err := svc.ValidateToken(standalone)
			require.NoError(t, err)
			require.Empty(t, claims.AccessFamilyID(), "the token carries no session")
			require.NoError(t, svc.RevokeToken(ctx, standalone))
			return standalone
		}},
		{"session read fails", &sessionFamilyStore{familyErr: errors.New("store down")}, func(t *testing.T, svc *auth.JWTService) string {
			access, _, err := svc.GenerateTokenPair(userID, orgID, "r3@example.com", "admin")
			require.NoError(t, err)
			return access
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := sessionFamilyService(t, tc.store, false)
			audit := &credentialRefusalLog{}
			svc.SetAuditLogger(audit)
			var reached int
			app := credentialRouteApp(svc, method, pattern, &reached)
			token := tc.token(t, svc)

			var status int
			var body []byte
			logged := captureLog(t, func() { status, body = sendBearer(t, app, method, concretePath(pattern), token) })
			assert.Equal(t, fiber.StatusUnauthorized, status)
			assert.JSONEq(t, `{"error":"Token has been revoked"}`, string(body))
			assert.Zero(t, reached)
			assert.Empty(t, audit.rows)
			assert.NotContains(t, logged, "SECURITY")
		})
	}
}

// R4: with no audit log attached, or with a nil *application.AuditService
// attached, the refusal at a credential route is still logged as its
// SECURITY line, and the request is refused as before.
func TestAuthMiddleware_CredentialRouteRefusalWithoutAnAuditLog(t *testing.T) {
	const method, pattern = "POST", "/api/v1/api-keys"
	for _, tc := range []struct {
		name   string
		attach func(svc *auth.JWTService)
	}{
		{"none attached", func(*auth.JWTService) {}},
		{"nil audit service", func(svc *auth.JWTService) {
			var nilService *application.AuditService
			svc.SetAuditLogger(nilService)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := sessionFamilyService(t, &sessionFamilyStore{}, false)
			tc.attach(svc)
			var reached int
			app := credentialRouteApp(svc, method, pattern, &reached)
			access, refresh, err := svc.GenerateTokenPair(uuid.New().String(), uuid.New().String(), "r4@example.com", "admin")
			require.NoError(t, err)
			revokeSessionOf(t, svc, refresh)

			var status int
			logged := captureLog(t, func() { status, _ = sendBearer(t, app, method, pattern, access) })
			assert.Equal(t, fiber.StatusUnauthorized, status)
			assert.Zero(t, reached)
			assert.Equal(t, 1, strings.Count(logged, "SECURITY credential_mint_refused route=api_key_create cause=sessionRevoked "), logged)
		})
	}
}

// R5: a credential route is recognised as the router matches it. A path in
// another case and with a trailing slash, which the router serves from the
// route, and a HEAD request, which the router answers with the GET route's
// handlers, each reach the route for a live session (control) and are
// recorded under the route's label once the session is revoked.
func TestAuthMiddleware_CredentialRouteIsRecognisedAsTheRouterMatchesIt(t *testing.T) {
	for _, tc := range []struct {
		name, routeMethod, pattern, method, path, label string
	}{
		{"case and trailing slash", "GET", "/api/v1/sdk/download", "GET", "/API/V1/SDK/Download/", "sdk_download"},
		{"HEAD of a GET route", "GET", "/api/v1/agents/:id/credentials", "HEAD", concretePath("/api/v1/agents/:id/credentials"), "agent_credentials"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := sessionFamilyService(t, &sessionFamilyStore{}, false)
			audit := &credentialRefusalLog{}
			svc.SetAuditLogger(audit)
			var reached int
			app := credentialRouteApp(svc, tc.routeMethod, tc.pattern, &reached)
			access, refresh, err := svc.GenerateTokenPair(uuid.New().String(), uuid.New().String(), "r5@example.com", "admin")
			require.NoError(t, err)

			status, _ := sendBearer(t, app, tc.method, tc.path, access)
			require.Equal(t, fiber.StatusOK, status, "control: the router serves the request from the route")
			require.Equal(t, 1, reached)

			revokeSessionOf(t, svc, refresh)
			var logged string
			logged = captureLog(t, func() { status, _ = sendBearer(t, app, tc.method, tc.path, access) })
			assert.Equal(t, fiber.StatusUnauthorized, status)
			assert.Equal(t, 1, reached)
			require.Len(t, audit.rows, 1)
			assert.Equal(t, tc.label, audit.rows[0].metadata["route"])
			assert.Equal(t, 1, strings.Count(logged, "SECURITY credential_mint_refused route="+tc.label+" "), logged)
		})
	}
}
