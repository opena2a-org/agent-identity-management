package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// SDKTokenTrackingMiddleware tracks SDK token usage automatically
// Extracts JWT token from Authorization header and tracks usage by JTI claim
type SDKTokenTrackingMiddleware struct {
	sdkTokenRepo domain.SDKTokenRepository
}

// NewSDKTokenTrackingMiddleware creates a new SDK token tracking middleware
func NewSDKTokenTrackingMiddleware(sdkTokenRepo domain.SDKTokenRepository) *SDKTokenTrackingMiddleware {
	return &SDKTokenTrackingMiddleware{
		sdkTokenRepo: sdkTokenRepo,
	}
}

// Handler returns the middleware handler function
func (m *SDKTokenTrackingMiddleware) Handler() fiber.Handler {
	return func(c fiber.Ctx) error {
		// ✅ Check for X-SDK-Token header (sent by Python and Java SDKs)
		// This is the reliable way to identify SDK registrations
		sdkTokenHeader := c.Get("X-SDK-Token", "")
		userAgent := c.Get("User-Agent", "")

		if sdkTokenHeader != "" {
			c.Locals("is_sdk_registration", true)
			c.Locals("sdk_token_id", sdkTokenHeader)

			// Look up SDK token to get the user who created it
			if sdkToken, err := m.sdkTokenRepo.GetByTokenID(sdkTokenHeader); err == nil && sdkToken != nil {
				c.Locals("sdk_token_user_id", sdkToken.UserID)
			}

			// Record usage asynchronously
			ipAddress := c.IP()
			go func(tokenID, ip string) {
				_ = m.sdkTokenRepo.RecordUsage(tokenID, ip)
			}(sdkTokenHeader, ipAddress)
		}

		// Also check User-Agent for SDK identification as a fallback
		if strings.Contains(userAgent, "AIM-Python-SDK") || strings.Contains(userAgent, "AIM-Java-SDK") {
			c.Locals("is_sdk_registration", true)
		}

		// Extract Authorization header for additional tracking
		authHeader := c.Get("Authorization", "")
		if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
			tokenString := strings.TrimPrefix(authHeader, "Bearer ")

			// Parse JWT without validation (we only need the JTI for tracking)
			token, _, err := jwt.NewParser().ParseUnverified(tokenString, jwt.MapClaims{})
			if err == nil {
				if claims, ok := token.Claims.(jwt.MapClaims); ok {
					if jti, ok := claims["jti"].(string); ok && jti != "" {
						// Store access token JTI (useful for logging)
						c.Locals("access_token_jti", jti)
					}
				}
			}
		}

		// Continue to next handler
		return c.Next()
	}
}
