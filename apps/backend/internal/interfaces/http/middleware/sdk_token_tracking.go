package middleware

import (
	"github.com/gofiber/fiber/v3"
	"github.com/opena2a/identity/backend/internal/domain"
)

// SDKTokenTrackingMiddleware tracks SDK token usage automatically
// Extracts X-SDK-Token header and increments usage count in background
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
		// Extract SDK token from header
		sdkTokenID := c.Get("X-SDK-Token", "")

		// If SDK token is present, record usage in background
		if sdkTokenID != "" {
			// Get client IP address
			ipAddress := c.IP()

			// Record usage asynchronously to avoid blocking the request
			go func(tokenID, ip string) {
				if err := m.sdkTokenRepo.RecordUsage(tokenID, ip); err != nil {
					// Log error but don't fail the request
					// In production, use proper logging
					// log.Printf("Failed to record SDK token usage: %v", err)
				}
			}(sdkTokenID, ipAddress)
		}

		// Continue to next handler
		return c.Next()
	}
}
