package middleware

import (
	"github.com/gofiber/fiber/v3"
	"github.com/opena2a/identity/backend/internal/domain"
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
