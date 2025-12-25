package middleware

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/opena2a/identity/backend/internal/domain"
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
