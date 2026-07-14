package middleware

import (
	"github.com/gofiber/fiber/v3"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// AdminMiddleware checks if user has admin role
// Must be used AFTER AuthMiddleware
func AdminMiddleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		role, ok := c.Locals("role").(string)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Authentication required",
			})
		}

		if role != string(domain.RoleAdmin) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Admin access required",
			})
		}

		return c.Next()
	}
}

// ManagerMiddleware checks if user has manager or admin role
// Must be used AFTER AuthMiddleware
func ManagerMiddleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		role, ok := c.Locals("role").(string)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Authentication required",
			})
		}

		if role != string(domain.RoleAdmin) && role != string(domain.RoleManager) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Manager or admin access required",
			})
		}

		return c.Next()
	}
}

// MemberMiddleware checks if user has at least member role (excludes viewers)
// Must be used AFTER AuthMiddleware
func MemberMiddleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		role, ok := c.Locals("role").(string)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Authentication required",
			})
		}

		if role == string(domain.RoleViewer) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Member access required (viewers cannot perform this action)",
			})
		}

		return c.Next()
	}
}

// MemberOrAPIKeyMiddleware allows a request authenticated EITHER by a valid API
// key (a machine principal, org derived from the key) OR by a human with at
// least member role (the JWT path). Must be used AFTER the auth middlewares that
// set "auth_method" / "role" (OptionalAPIKeyMiddleware + AuthMiddleware).
//
// It exists so machine API keys can register agents (POST /agents/) WITHOUT the
// blanket member surface that setting a role on the API-key path would grant.
// Deliberately narrow: it is applied ONLY to agent registration. Every other
// role-gated route keeps MemberMiddleware / ManagerMiddleware / AdminMiddleware,
// which require a JWT role — so key-material routes (credential rotation, agent
// key replacement, PQC key registration) stay unreachable by a bearer API key.
// Agent-signature auth (ed25519/mldsa/hybrid/atc) sets no role and is not
// "api_key", so an already-registered agent still cannot register other agents.
func MemberOrAPIKeyMiddleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		if method, _ := c.Locals("auth_method").(string); method == "api_key" {
			return c.Next()
		}

		role, ok := c.Locals("role").(string)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Authentication required",
			})
		}

		if role == string(domain.RoleViewer) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Member access required (viewers cannot perform this action)",
			})
		}

		return c.Next()
	}
}
