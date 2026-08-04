package middleware

import (
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

// The OAuth token endpoint (RFC 7523 jwt-bearer) mints an access token for an
// authenticated *agent* with role="service" — see oauth_token_handler.go:
//
//	GenerateAccessToken(clientID, agent.OrganizationID.String(), clientID, "service")
//
// "service" is not a member of domain.UserRole (admin|manager|member|viewer).
// MemberMiddleware rejects only RoleViewer, so every other non-empty role string
// — including "service" — reaches the handler. That puts an agent-authenticated
// token on all 35 MemberMiddleware-gated routes, among them
// GET /agents/:id/credentials, which returns a decrypted Ed25519 private key.
//
// Net effect: one compromised agent can read every sibling agent's private key in
// the same organization. These tests pin that shut.

// newServiceToken mints a token exactly the way the OAuth token endpoint does.
// Kept byte-identical to the production call so this test tracks the real mint
// site rather than a paraphrase of it.
func newServiceToken(t *testing.T, svc *auth.JWTService, agentID, orgID string) string {
	t.Helper()
	tok, err := svc.GenerateAccessToken(agentID, orgID, agentID, "service")
	require.NoError(t, err)
	return tok
}

func newRoleToken(t *testing.T, svc *auth.JWTService, orgID, role string) string {
	t.Helper()
	tok, err := svc.GenerateAccessToken(uuid.NewString(), orgID, "user@example.com", role)
	require.NoError(t, err)
	return tok
}

// callMemberGated wires the real AuthMiddleware + MemberMiddleware in front of a
// sentinel that echoes the resolved role. A 200 with an empty body would mean the
// request arrived with no principal, so asserting on the body — not just the status
// — is what keeps the positive control from being a tautology.
func callMemberGated(t *testing.T, svc *auth.JWTService, token string) (int, string) {
	t.Helper()
	app := fiber.New()
	app.Use(AuthMiddleware(svc))
	app.Use(MemberMiddleware())
	app.Get("/agents/:id/credentials", func(c fiber.Ctx) error {
		role, _ := c.Locals("role").(string)
		return c.Status(fiber.StatusOK).SendString(role)
	})

	req := httptest.NewRequest("GET", "/agents/"+uuid.NewString()+"/credentials", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(body)
}

func TestServiceTokenCannotReachMemberGatedCredentialRoute(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-at-least-32-characters-long!")
	svc := auth.NewJWTService()

	orgID := uuid.NewString()
	victimOrgID := orgID // same tenant: this is sibling-agent access, not cross-tenant

	t.Run("service token is refused", func(t *testing.T) {
		attackerAgentID := uuid.NewString()
		status, body := callMemberGated(t, svc, newServiceToken(t, svc, attackerAgentID, victimOrgID))

		// An agent-authenticated token must never traverse a human role gate.
		// Either layer may reject it (AuthMiddleware on issuer/type, or the role
		// gate on an unknown role) — what must not happen is reaching the handler.
		assert.NotEqual(t, fiber.StatusOK, status,
			"service token reached GET /agents/:id/credentials — an agent can read a sibling agent's private key (role echoed: %q)", body)
		assert.Contains(t, []int{fiber.StatusUnauthorized, fiber.StatusForbidden}, status,
			"expected 401 or 403 for a service token on a member-gated route, got %d", status)
	})

	// POSITIVE CONTROL — proves the harness can deliver a 200 at all. Without this,
	// the assertion above would also pass if every request 401'd for an unrelated
	// reason (bad secret, malformed token, middleware wiring error).
	t.Run("member token still reaches the handler", func(t *testing.T) {
		status, body := callMemberGated(t, svc, newRoleToken(t, svc, orgID, "member"))
		require.Equal(t, fiber.StatusOK, status, "member token must still reach the handler; harness is broken otherwise")
		require.Equal(t, "member", body, "handler reached but no role resolved — sentinel proves nothing")
	})

	// NEGATIVE CONTROL — proves MemberMiddleware is actually mounted and rejecting.
	// If this passed while the service-token case failed, the gate would be absent
	// rather than merely permissive.
	t.Run("viewer token is refused", func(t *testing.T) {
		status, _ := callMemberGated(t, svc, newRoleToken(t, svc, orgID, "viewer"))
		require.Equal(t, fiber.StatusForbidden, status, "viewer must be rejected by MemberMiddleware")
	})
}

// callSelfScoped wires the real service-principal authenticator + the
// MemberOrSelfService gate in front of a sentinel that echoes the resolved auth
// method, then addresses targetAgentID. This is the route shape of
// PUT /agents/:id, which the TypeScript SDK's updateAgent() calls.
func callSelfScoped(t *testing.T, svc *auth.JWTService, token, targetAgentID string) (int, string) {
	t.Helper()
	app := fiber.New()
	app.Use(ServicePrincipalMiddleware(svc))
	app.Use(AuthMiddleware(svc))
	// Registered as a ROUTE handler, not via app.Use — matching main.go. The gate
	// reads c.Params("id"), which is only populated after route matching, so
	// mounting it with .Use() would leave the param empty (it fails closed with a
	// 400, but the route would be unusable). Keep this mirroring production.
	app.Put("/agents/:id", MemberOrSelfServiceMiddleware(), func(c fiber.Ctx) error {
		am, _ := c.Locals("auth_method").(string)
		if am == "" {
			am, _ = c.Locals("role").(string)
		}
		return c.Status(fiber.StatusOK).SendString(am)
	})

	req := httptest.NewRequest("PUT", "/agents/"+targetAgentID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(body)
}

// The self-scope check is the security property that lets a service principal keep
// using PUT /agents/:id (the SDK needs it) without being able to reach a sibling.
// An allowlist alone would not give this — it has to be pinned to the token subject.
func TestServicePrincipalIsPinnedToItsOwnAgent(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-at-least-32-characters-long!")
	svc := auth.NewJWTService()

	orgID := uuid.NewString()
	callerAgentID := uuid.NewString()
	siblingAgentID := uuid.NewString()
	token, err := svc.GenerateServiceToken(callerAgentID, orgID)
	require.NoError(t, err)

	// POSITIVE CONTROL — without this, the sibling assertion below would also pass
	// if service tokens were rejected outright, which would silently break the SDK.
	t.Run("service principal may update its OWN agent", func(t *testing.T) {
		status, body := callSelfScoped(t, svc, token, callerAgentID)
		require.Equal(t, fiber.StatusOK, status, "SDK updateAgent() on its own record must keep working")
		require.Equal(t, "service", body, "reached the handler without resolving a service principal")
	})

	t.Run("service principal may NOT update a sibling agent", func(t *testing.T) {
		status, body := callSelfScoped(t, svc, token, siblingAgentID)
		assert.Equal(t, fiber.StatusForbidden, status,
			"service principal reached a sibling agent's record (auth echoed: %q)", body)
	})

	t.Run("member may still update any agent in the org", func(t *testing.T) {
		status, body := callSelfScoped(t, svc, newRoleToken(t, svc, orgID, "member"), siblingAgentID)
		require.Equal(t, fiber.StatusOK, status, "human member path must be unaffected")
		require.Equal(t, "member", body)
	})

	t.Run("viewer is still rejected", func(t *testing.T) {
		status, _ := callSelfScoped(t, svc, newRoleToken(t, svc, orgID, "viewer"), siblingAgentID)
		require.Equal(t, fiber.StatusForbidden, status)
	})
}

// A service token presented on a route that has NOT opted into service principals
// must be refused by AuthMiddleware on the issuer alone — defence in depth, so a
// route that forgets its gate still fails closed.
func TestServiceTokenIsRefusedOnRoutesWithoutTheServiceGate(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-at-least-32-characters-long!")
	svc := auth.NewJWTService()

	token, err := svc.GenerateServiceToken(uuid.NewString(), uuid.NewString())
	require.NoError(t, err)

	// No ServicePrincipalMiddleware in this chain, mirroring the other 34 routes.
	status, body := callMemberGated(t, svc, token)
	require.Equal(t, fiber.StatusUnauthorized, status,
		"service token was not refused by AuthMiddleware on an un-opted-in route (body: %q)", body)
}

// Unknown role strings must fail closed. This is the class fix, not the instance:
// "service" is today's escaping value, but any future non-enum role string would
// traverse a deny-list gate the same way.
func TestMemberGateFailsClosedOnUnknownRoles(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-at-least-32-characters-long!")
	svc := auth.NewJWTService()
	orgID := uuid.NewString()

	for _, role := range []string{"service", "root", "superuser", "system", "bot", "unknown"} {
		t.Run(role, func(t *testing.T) {
			status, body := callMemberGated(t, svc, newRoleToken(t, svc, orgID, role))
			assert.NotEqual(t, fiber.StatusOK, status,
				"unknown role %q traversed MemberMiddleware (role echoed: %q) — the gate is a deny-list, not an allow-list", role, body)
		})
	}
}
