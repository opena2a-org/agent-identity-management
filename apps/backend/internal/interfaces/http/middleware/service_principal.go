package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

// ServicePrincipalMiddleware authenticates a machine principal — an agent that
// obtained a token from the OAuth token endpoint (RFC 7523 jwt-bearer).
//
// It is an OPTIONAL authenticator, in the same shape as OptionalAPIKeyMiddleware:
// a request without a service token passes straight through so the remaining
// middleware (PQC agent auth, then JWT) can handle it. Only a token minted with
// auth.IssuerService is claimed here.
//
// SECURITY: this deliberately does NOT set "role". A service principal is not a
// human and must not satisfy a human role gate. It sets "agent_id" so route gates
// can pin access to the caller's own agent — see MemberOrSelfServiceMiddleware.
//
// Must be registered BEFORE AuthMiddleware, which skips when auth_method is
// already "service".
func ServicePrincipalMiddleware(jwtService *auth.JWTService) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Leave already-authenticated requests alone.
		if c.Locals("auth_method") != nil || c.Locals("authenticated_via") != nil {
			return c.Next()
		}

		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Next()
		}
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			return c.Next() // let AuthMiddleware produce the format error
		}

		claims, err := jwtService.ValidateToken(parts[1])
		if err != nil {
			return c.Next() // not ours to reject; AuthMiddleware reports invalid tokens
		}

		// Only claim service-issuer tokens. Anything else belongs downstream.
		if claims.Issuer != auth.IssuerService {
			return c.Next()
		}

		// From here the token IS a service token, so failures are terminal —
		// falling through would hand it to AuthMiddleware, which rejects the
		// issuer outright and would mask the real reason.
		if claims.TokenType != auth.TokenTypeAccess {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "This token cannot be used for API access",
			})
		}

		if jwtService.IsRevoked(c.Context(), claims.ID) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Token has been revoked",
			})
		}

		agentID, err := uuid.Parse(claims.Subject)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid agent ID in token",
			})
		}

		orgID, err := uuid.Parse(claims.OrganizationID)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid organization ID in token",
			})
		}

		c.Locals("auth_method", "service")
		c.Locals("agent_id", agentID)
		c.Locals("organization_id", orgID)
		// Intentionally no "role" and no "user_id": a service principal is not a
		// user. Handlers calling RequireOrgAndUserID will fail closed, which is
		// correct — those handlers serve human flows.

		return c.Next()
	}
}

// MemberOrSelfServiceMiddleware allows a route to be reached EITHER by a human
// with at least member role (the JWT path) OR by a service principal acting on
// ITSELF — the ":id" path parameter must equal the agent ID the token was minted
// for.
//
// Shaped after MemberOrAPIKeyMiddleware and deliberately just as narrow. The
// self-scope check is what makes this safe to apply: a service principal cannot
// reach a sibling agent's resource through it by construction, so the route does
// not depend on an allowlist being kept up to date.
//
// Apply ONLY to routes an SDK legitimately calls on its own agent record.
//
// MUST be registered as a route handler — agents.Put("/:id", gate, handler) — not
// via .Use(). It reads c.Params("id"), which Fiber only populates after route
// matching; mounted with .Use() the param is empty and every service request 400s.
// That direction is fail-closed, but the route would be unusable.
func MemberOrSelfServiceMiddleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		if method, _ := c.Locals("auth_method").(string); method == "service" {
			callerAgentID, ok := c.Locals("agent_id").(uuid.UUID)
			if !ok {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"error": "Authentication required",
				})
			}

			targetID, err := uuid.Parse(c.Params("id"))
			if err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"error": "Invalid agent ID",
				})
			}

			if targetID != callerAgentID {
				return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
					"error": "Service principals may only act on their own agent",
				})
			}

			return c.Next()
		}

		// Human path — identical allow-list to MemberMiddleware.
		role, ok := c.Locals("role").(string)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Authentication required",
			})
		}

		if role != string(domain.RoleAdmin) &&
			role != string(domain.RoleManager) &&
			role != string(domain.RoleMember) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Member access required (viewers cannot perform this action)",
			})
		}

		return c.Next()
	}
}
