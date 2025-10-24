package middleware

import (
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/limiter"
	"github.com/google/uuid"
)

// RateLimitMiddleware implements rate limiting
func RateLimitMiddleware() fiber.Handler {
	return limiter.New(limiter.Config{
		Max:        100,                  // 100 requests
		Expiration: 1 * time.Minute,      // per minute
		KeyGenerator: func(c fiber.Ctx) string {
			// Rate limit by user if authenticated, otherwise by IP
			if userID := c.Locals("user_id"); userID != nil {
				if id, ok := userID.(uuid.UUID); ok {
					return id.String()
				}
			}
			return c.IP()
		},
		LimitReached: func(c fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "Rate limit exceeded. Please try again later.",
			})
		},
	})
}

// StrictRateLimitMiddleware implements stricter rate limiting for sensitive endpoints
func StrictRateLimitMiddleware() fiber.Handler {
	return limiter.New(limiter.Config{
		Max:        10,                   // 10 requests
		Expiration: 1 * time.Minute,      // per minute
		KeyGenerator: func(c fiber.Ctx) string {
			if userID := c.Locals("user_id"); userID != nil {
				if id, ok := userID.(uuid.UUID); ok {
					return id.String()
				}
			}
			return c.IP()
		},
		LimitReached: func(c fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "Rate limit exceeded. Please try again later.",
			})
		},
	})
}
