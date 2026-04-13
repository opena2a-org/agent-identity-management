package middleware

import (
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v3"

	atcdomain "github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain/atc"
)

const (
	// maxATCHeaderSize limits the raw Authorization header token length before any decoding.
	// MaxATCSize is 8 KiB decoded; base64 expands ~4/3x, so 16 KiB encoded is generous.
	maxATCHeaderSize = 16 * 1024
)

// ATCAuthMiddleware verifies Authorization: ATC <base64url> headers.
// If the header is present, it verifies the ATC and sets agent context.
// If the header is absent, it passes through to the next middleware (e.g., PQC or JWT).
//
// On success, sets the following Fiber locals:
//   - "agent_id": uuid.UUID
//   - "authenticated_via": "atc"
//   - "auth_method": "atc"
//   - "atc_claims": *atcdomain.ATCClaims
//
// Emits response headers:
//   - X-ATC-Supported: true
//   - X-ATC-Version: 1.0
func ATCAuthMiddleware(verifier atcdomain.ATCVerifier) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Always signal ATC support (spec v1 Section 3.2)
		c.Set("X-ATC-Supported", "true")
		c.Set("X-ATC-Version", "1.0")

		authHeader := c.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "ATC ") {
			// Not an ATC request — let other middleware handle it
			return c.Next()
		}

		// Extract the base64url token
		rawToken := strings.TrimPrefix(authHeader, "ATC ")
		if rawToken == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error":   atcdomain.ErrCodeMalformed,
				"message": "empty ATC token",
			})
		}

		// Reject oversized tokens before any decoding to prevent allocation DoS
		if len(rawToken) > maxATCHeaderSize {
			return c.Status(fiber.StatusRequestEntityTooLarge).JSON(fiber.Map{
				"error":   atcdomain.ErrCodeOversized,
				"message": fmt.Sprintf("ATC token size %d exceeds maximum %d", len(rawToken), maxATCHeaderSize),
			})
		}

		// Verify the ATC
		claims, err := verifier.Verify(rawToken)
		if err != nil {
			status := fiber.StatusUnauthorized
			errCode := "atc_verification_failed"
			errMsg := err.Error()

			if atcErr, ok := err.(*atcdomain.ATCError); ok {
				errCode = atcErr.Code
				errMsg = atcErr.Message
				if atcErr.Code == atcdomain.ErrCodeInsufficientScope {
					status = fiber.StatusForbidden
				}
			}

			return c.Status(status).JSON(fiber.Map{
				"error":   errCode,
				"message": errMsg,
			})
		}

		// Check revocation (spec v1 Step 8)
		revoked, err := verifier.IsRevoked(claims.ATCID)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error":   atcdomain.ErrCodeRevoked,
				"message": "CRL check failed: " + err.Error(),
			})
		}
		if revoked {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error":   atcdomain.ErrCodeRevoked,
				"message": "ATC has been revoked",
			})
		}

		// ATC verified — set agent context
		c.Locals("agent_id", claims.AgentID)
		c.Locals("authenticated_via", "atc")
		c.Locals("auth_method", "atc")
		c.Locals("atc_claims", claims)

		return c.Next()
	}
}
