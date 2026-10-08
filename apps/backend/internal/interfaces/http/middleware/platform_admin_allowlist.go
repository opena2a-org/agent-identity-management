package middleware

import (
	"github.com/gofiber/fiber/v3"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// PlatformAdminAllowlistMiddleware admits a request whose session belongs to
// an organization admin whose email isPlatformAdmin accepts (the
// AIM_PLATFORM_ADMINS allowlist in production). Routes behind it read data
// across every organization, so both conditions must hold: an org admin who
// is not on the list is refused, and so is a listed address holding any other
// role. Must be used AFTER AuthMiddleware.
func PlatformAdminAllowlistMiddleware(isPlatformAdmin func(email string) bool) fiber.Handler {
	return func(c fiber.Ctx) error {
		role, roleOK := c.Locals("role").(string)
		email, emailOK := c.Locals("email").(string)
		if !roleOK || !emailOK {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Authentication required",
			})
		}
		if role != string(domain.RoleAdmin) || email == "" || !isPlatformAdmin(email) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Platform admin access required",
			})
		}
		return c.Next()
	}
}
