package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/opena2a/identity/backend/internal/domain"
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
		// Extract Authorization header
		authHeader := c.Get("Authorization", "")

		// Extract token ID (JTI) from JWT if present
		if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
			tokenString := strings.TrimPrefix(authHeader, "Bearer ")

			// Parse JWT without validation (we only need the JTI claim)
			// Note: We're NOT validating the token here - that's done by AuthMiddleware
			// We're just extracting the token ID for usage tracking
			token, _, err := jwt.NewParser().ParseUnverified(tokenString, jwt.MapClaims{})
			if err == nil {
				if claims, ok := token.Claims.(jwt.MapClaims); ok {
					if jti, ok := claims["jti"].(string); ok && jti != "" {
						// Get client IP address
						ipAddress := c.IP()

						// Record usage asynchronously to avoid blocking the request
						go func(tokenID, ip string) {
							if err := m.sdkTokenRepo.RecordUsage(tokenID, ip); err != nil {
								// Log error but don't fail the request
								// In production, use proper logging
								// log.Printf("Failed to record SDK token usage: %v", err)
							}
						}(jti, ipAddress)
					}
				}
			}
		}

		// Continue to next handler
		return c.Next()
	}
}
