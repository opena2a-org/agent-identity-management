package middleware

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/infrastructure/auth"
)

// AuthMiddleware validates JWT tokens and sets user context
func AuthMiddleware(jwtService *auth.JWTService) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Check if already authenticated by Ed25519 middleware
		authenticatedVia := c.Locals("authenticated_via")
		fmt.Printf("🔒 JWT middleware: authenticated_via = %v\n", authenticatedVia)
		if authenticatedVia == "ed25519" {
			// Already authenticated - skip JWT validation
			fmt.Printf("✅ JWT middleware: Skipping JWT - Ed25519 already authenticated\n")
			return c.Next()
		}

		// Try to get token from Authorization header first
		authHeader := c.Get("Authorization")
		var token string

		if authHeader != "" {
			// Expected format: "Bearer <token>"
			parts := strings.Split(authHeader, " ")
			if len(parts) == 2 && parts[0] == "Bearer" {
				token = parts[1]
			} else {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"error": "Invalid authorization header format",
				})
			}
		} else {
			// Fallback to cookie
			token = c.Cookies("access_token")
		}

		if token == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "No authentication token provided",
			})
		}

		// Validate token
		claims, err := jwtService.ValidateToken(token)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid or expired token",
			})
		}

		// Parse UUIDs from claims
		userID, err := uuid.Parse(claims.UserID)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid user ID in token",
			})
		}

		organizationID, err := uuid.Parse(claims.OrganizationID)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid organization ID in token",
			})
		}

		// Set user context for downstream handlers
		c.Locals("user_id", userID)
		c.Locals("organization_id", organizationID)
		c.Locals("email", claims.Email)
		c.Locals("role", claims.Role)

		return c.Next()
	}
}

// OptionalAuthMiddleware is like AuthMiddleware but doesn't fail if no token
// Useful for endpoints that work both authenticated and unauthenticated
func OptionalAuthMiddleware(jwtService *auth.JWTService) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Try to get token
		authHeader := c.Get("Authorization")
		var token string

		if authHeader != "" {
			parts := strings.Split(authHeader, " ")
			if len(parts) == 2 && parts[0] == "Bearer" {
				token = parts[1]
			}
		} else {
			token = c.Cookies("access_token")
		}

		// If no token, continue without setting context
		if token == "" {
			return c.Next()
		}

		// Validate token if present
		claims, err := jwtService.ValidateToken(token)
		if err != nil {
			// Invalid token, but don't fail - just continue without auth
			return c.Next()
		}

		// Parse UUIDs
		userID, err := uuid.Parse(claims.UserID)
		if err == nil {
			c.Locals("user_id", userID)
		}

		organizationID, err := uuid.Parse(claims.OrganizationID)
		if err == nil {
			c.Locals("organization_id", organizationID)
		}

		c.Locals("email", claims.Email)
		c.Locals("role", claims.Role)

		return c.Next()
	}
}
