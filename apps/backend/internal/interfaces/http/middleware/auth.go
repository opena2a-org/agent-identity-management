package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

// AuthMiddleware validates JWT tokens and sets user context
// SECURITY: Debug logging removed to prevent information leakage
func AuthMiddleware(jwtService *auth.JWTService) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Check if already authenticated by Ed25519 middleware
		authenticatedVia := c.Locals("authenticated_via")
		if authenticatedVia == "ed25519" {
			// Already authenticated - skip JWT validation
			return c.Next()
		}

		// Check if already authenticated by API key middleware
		authMethod := c.Locals("auth_method")
		if authMethod == "api_key" {
			// Already authenticated via API key - skip JWT validation
			return c.Next()
		}

		// Check if already authenticated by the service-principal middleware
		if authMethod == "service" {
			// Already authenticated as a machine principal - skip JWT validation.
			// No "role" was set, so human role gates still fail closed.
			return c.Next()
		}

		// Check if already authenticated by ATC middleware
		if authMethod == "atc" {
			// Already authenticated via ATC - skip JWT validation
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

		// SECURITY: SDK refresh tokens (90-day) must never be accepted as bearer
		// access tokens. They are only valid at the /auth/refresh endpoint, where
		// they are exchanged for a short-lived access token. Rejecting them here
		// stops a long-lived SDK token from being used as a session token.
		if claims.Issuer == auth.IssuerSDK {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "SDK tokens cannot be used for API access; exchange it at /auth/refresh",
			})
		}

		// SECURITY: service tokens (OAuth RFC 7523 jwt-bearer, minted to an agent)
		// are machine principals, not users. They must never traverse a human
		// auth path — this middleware populates "role", which the role gates read.
		// Service principals are authenticated by ServicePrincipalMiddleware
		// instead, which pins them to their own agent ID.
		if claims.Issuer == auth.IssuerService {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Service tokens cannot be used on user-authenticated routes",
			})
		}

		// SECURITY: only access tokens are valid bearers. A refresh token (or any
		// non-access type) must not be replayed as a session token. The empty-type
		// rollout grace has been retired on the access path: every live access token
		// carries typ="access" (issued post-rollout), so an empty type here can only
		// be a legacy refresh token replayed as access — reject it. (The refresh
		// endpoint keeps the empty-type grace until long-lived SDK tokens age out.)
		if claims.TokenType != auth.TokenTypeAccess {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "This token cannot be used for API access",
			})
		}

		// SECURITY: reject tokens revoked server-side (e.g. on logout). No-op when
		// revocation is not configured; fails closed on a store outage by default.
		if jwtService.IsRevoked(c.Context(), claims.ID) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Token has been revoked",
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

		// SECURITY: never treat an SDK refresh token as an authenticated bearer
		// (see AuthMiddleware). Continue unauthenticated instead.
		if claims.Issuer == auth.IssuerSDK {
			return c.Next()
		}

		// SECURITY: only access tokens authenticate. A non-access type (e.g. a
		// refresh token) must not set user context. The empty-type access grace is
		// retired (see AuthMiddleware): an empty type here is a legacy refresh token,
		// so continue unauthenticated rather than trusting it.
		if claims.TokenType != auth.TokenTypeAccess {
			return c.Next()
		}

		// SECURITY: a revoked token must not authenticate, even optionally.
		if jwtService.IsRevoked(c.Context(), claims.ID) {
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
