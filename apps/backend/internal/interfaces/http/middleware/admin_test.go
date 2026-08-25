package middleware

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminMiddleware_NoRole(t *testing.T) {
	app := fiber.New()
	app.Use(AdminMiddleware())
	app.Get("/admin", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "admin only"})
	})

	req := httptest.NewRequest("GET", "/admin", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)
	assert.Equal(t, "Authentication required", result["error"])
}

func TestAdminMiddleware_NotAdmin(t *testing.T) {
	roles := []string{
		string(domain.RoleMember),
		string(domain.RoleManager),
		string(domain.RoleViewer),
	}

	for _, role := range roles {
		t.Run("role_"+role, func(t *testing.T) {
			app := fiber.New()
			app.Use(func(c fiber.Ctx) error {
				c.Locals("role", role)
				return c.Next()
			})
			app.Use(AdminMiddleware())
			app.Get("/admin", func(c fiber.Ctx) error {
				return c.JSON(fiber.Map{"message": "admin only"})
			})

			req := httptest.NewRequest("GET", "/admin", nil)
			resp, err := app.Test(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, fiber.StatusForbidden, resp.StatusCode)

			body, _ := io.ReadAll(resp.Body)
			var result map[string]interface{}
			json.Unmarshal(body, &result)
			assert.Equal(t, "Admin access required", result["error"])
		})
	}
}

func TestAdminMiddleware_IsAdmin(t *testing.T) {
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Locals("role", string(domain.RoleAdmin))
		return c.Next()
	})
	app.Use(AdminMiddleware())
	app.Get("/admin", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "admin only"})
	})

	req := httptest.NewRequest("GET", "/admin", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestManagerMiddleware_NoRole(t *testing.T) {
	app := fiber.New()
	app.Use(ManagerMiddleware())
	app.Get("/manage", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "managers only"})
	})

	req := httptest.NewRequest("GET", "/manage", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)
	assert.Equal(t, "Authentication required", result["error"])
}

func TestManagerMiddleware_NotManager(t *testing.T) {
	roles := []string{
		string(domain.RoleMember),
		string(domain.RoleViewer),
	}

	for _, role := range roles {
		t.Run("role_"+role, func(t *testing.T) {
			app := fiber.New()
			app.Use(func(c fiber.Ctx) error {
				c.Locals("role", role)
				return c.Next()
			})
			app.Use(ManagerMiddleware())
			app.Get("/manage", func(c fiber.Ctx) error {
				return c.JSON(fiber.Map{"message": "managers only"})
			})

			req := httptest.NewRequest("GET", "/manage", nil)
			resp, err := app.Test(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, fiber.StatusForbidden, resp.StatusCode)

			body, _ := io.ReadAll(resp.Body)
			var result map[string]interface{}
			json.Unmarshal(body, &result)
			assert.Equal(t, "Manager or admin access required", result["error"])
		})
	}
}

func TestManagerMiddleware_IsManager(t *testing.T) {
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Locals("role", string(domain.RoleManager))
		return c.Next()
	})
	app.Use(ManagerMiddleware())
	app.Get("/manage", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "managers only"})
	})

	req := httptest.NewRequest("GET", "/manage", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestManagerMiddleware_IsAdmin(t *testing.T) {
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Locals("role", string(domain.RoleAdmin))
		return c.Next()
	})
	app.Use(ManagerMiddleware())
	app.Get("/manage", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "managers only"})
	})

	req := httptest.NewRequest("GET", "/manage", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestMemberMiddleware_NoRole(t *testing.T) {
	app := fiber.New()
	app.Use(MemberMiddleware())
	app.Get("/member", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "members only"})
	})

	req := httptest.NewRequest("GET", "/member", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestMemberMiddleware_IsViewer(t *testing.T) {
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Locals("role", string(domain.RoleViewer))
		return c.Next()
	})
	app.Use(MemberMiddleware())
	app.Get("/member", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "members only"})
	})

	req := httptest.NewRequest("GET", "/member", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusForbidden, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)
	assert.Contains(t, result["error"], "viewers cannot perform this action")
}

func TestMemberMiddleware_IsMember(t *testing.T) {
	roles := []string{
		string(domain.RoleMember),
		string(domain.RoleManager),
		string(domain.RoleAdmin),
	}

	for _, role := range roles {
		t.Run("role_"+role, func(t *testing.T) {
			app := fiber.New()
			app.Use(func(c fiber.Ctx) error {
				c.Locals("role", role)
				return c.Next()
			})
			app.Use(MemberMiddleware())
			app.Get("/member", func(c fiber.Ctx) error {
				return c.JSON(fiber.Map{"message": "members only"})
			})

			req := httptest.NewRequest("GET", "/member", nil)
			resp, err := app.Test(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, fiber.StatusOK, resp.StatusCode)
		})
	}
}

// ===========================================================================
// MemberOrAPIKeyMiddleware — the narrow agent-registration gate.
// A machine API key (auth_method=api_key) may register agents; a member+ JWT
// may too; viewers and unauthenticated are rejected; agent-signature auth
// (ed25519/atc, no role) cannot register agents. The final test locks the
// invariant that a machine key must STILL be blocked by plain MemberMiddleware,
// so no other role-gated route (credential rotation, key replacement, pqc-key)
// opens to bearer API keys.
// ===========================================================================

// withLocals sets arbitrary Fiber locals before the middleware under test.
func withLocals(kv map[string]string) fiber.Handler {
	return func(c fiber.Ctx) error {
		for k, v := range kv {
			c.Locals(k, v)
		}
		return c.Next()
	}
}

func TestMemberOrAPIKey_APIKeyPasses(t *testing.T) {
	app := fiber.New()
	app.Use(withLocals(map[string]string{"auth_method": "api_key"})) // no role, like OptionalAPIKeyMiddleware
	app.Use(MemberOrAPIKeyMiddleware())
	app.Post("/agents", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusCreated) })

	req := httptest.NewRequest("POST", "/agents", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, fiber.StatusCreated, resp.StatusCode, "a valid API key must be allowed to register agents")
}

func TestMemberOrAPIKey_MemberJWTPasses(t *testing.T) {
	app := fiber.New()
	app.Use(withLocals(map[string]string{"role": string(domain.RoleMember)}))
	app.Use(MemberOrAPIKeyMiddleware())
	app.Post("/agents", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusCreated) })

	req := httptest.NewRequest("POST", "/agents", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, fiber.StatusCreated, resp.StatusCode)
}

func TestMemberOrAPIKey_ViewerBlocked(t *testing.T) {
	app := fiber.New()
	app.Use(withLocals(map[string]string{"role": string(domain.RoleViewer)}))
	app.Use(MemberOrAPIKeyMiddleware())
	app.Post("/agents", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusCreated) })

	req := httptest.NewRequest("POST", "/agents", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, fiber.StatusForbidden, resp.StatusCode)
}

func TestMemberOrAPIKey_UnauthenticatedBlocked(t *testing.T) {
	app := fiber.New()
	app.Use(MemberOrAPIKeyMiddleware()) // nothing set
	app.Post("/agents", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusCreated) })

	req := httptest.NewRequest("POST", "/agents", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

// Agent-signature auth (ed25519/mldsa/hybrid/atc) sets auth_method but no role
// and is not "api_key" — an already-registered agent must not be able to register
// agents. All such auth methods must be rejected by the gate.
func TestMemberOrAPIKey_AgentSignatureBlocked(t *testing.T) {
	for _, method := range []string{"ed25519", "mldsa", "hybrid", "atc"} {
		t.Run(method, func(t *testing.T) {
			app := fiber.New()
			app.Use(withLocals(map[string]string{"auth_method": method}))
			app.Use(MemberOrAPIKeyMiddleware())
			app.Post("/agents", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusCreated) })

			req := httptest.NewRequest("POST", "/agents", nil)
			resp, err := app.Test(req)
			require.NoError(t, err)
			defer resp.Body.Close()
			assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode, "agent-signature auth must not register agents")
		})
	}
}

// INVARIANT LOCK: a machine API key (auth_method=api_key, no role) must STILL be
// rejected by plain MemberMiddleware. This proves the fix opens ONLY the
// agent-registration route and no other role-gated route (rotate-credentials,
// key replacement, pqc-key) becomes reachable by a bearer API key.
func TestMemberMiddleware_StillBlocksAPIKey(t *testing.T) {
	app := fiber.New()
	app.Use(withLocals(map[string]string{"auth_method": "api_key"})) // key authed, but no role
	app.Use(MemberMiddleware())
	app.Post("/agents/x/rotate-credentials", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	req := httptest.NewRequest("POST", "/agents/x/rotate-credentials", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode,
		"API keys must NOT pass plain MemberMiddleware — key-material routes stay JWT-only")
}

// The registration guard counts users with domain.RoleAdmin as the approvers; the approval
// routes sit behind this middleware. The two must admit the same role set, or a deployment
// with a working approver is refused (or one without is not). Every other role is rejected.
func TestAdminMiddleware_AdmitsExactlyTheRoleTheRegistrationGuardCounts(t *testing.T) {
	admitted := map[domain.UserRole]bool{}
	for _, role := range []domain.UserRole{domain.RoleAdmin, domain.RoleManager, domain.RoleMember, domain.RoleViewer} {
		app := fiber.New()
		app.Use(func(c fiber.Ctx) error {
			c.Locals("role", string(role))
			return c.Next()
		})
		app.Use(AdminMiddleware())
		app.Get("/", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
		resp, err := app.Test(httptest.NewRequest("GET", "/", nil))
		assert.NoError(t, err)
		admitted[role] = resp.StatusCode == fiber.StatusOK
	}
	assert.Equal(t, map[domain.UserRole]bool{domain.RoleAdmin: true, domain.RoleManager: false, domain.RoleMember: false, domain.RoleViewer: false}, admitted)
}
