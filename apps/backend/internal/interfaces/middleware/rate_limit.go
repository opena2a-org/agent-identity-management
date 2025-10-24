package middleware

import (
	"fmt"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/opena2a/identity/backend/internal/infrastructure/cache"
)

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	Cache         *cache.RedisCache
	MaxRequests   int64
	Window        time.Duration
	KeyGenerator  func(fiber.Ctx) string
	LimitReached  fiber.Handler
}

// RateLimiter creates a rate limiting middleware
func RateLimiter(config RateLimitConfig) fiber.Handler {
	if config.MaxRequests == 0 {
		config.MaxRequests = 100 // Default: 100 requests
	}

	if config.Window == 0 {
		config.Window = 1 * time.Minute // Default: per minute
	}

	if config.KeyGenerator == nil {
		// Default: rate limit by IP
		config.KeyGenerator = func(c fiber.Ctx) string {
			return c.IP()
		}
	}

	if config.LimitReached == nil {
		config.LimitReached = func(c fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "Too many requests, please try again later",
			})
		}
	}

	return func(c fiber.Ctx) error {
		if config.Cache == nil {
			// If no cache, skip rate limiting
			return c.Next()
		}

		key := config.KeyGenerator(c)
		allowed, err := config.Cache.RateLimit(c.Context(), key, config.MaxRequests, config.Window)
		if err != nil {
			// On error, allow request to proceed but log error
			fmt.Printf("Rate limit error: %v\n", err)
			return c.Next()
		}

		if !allowed {
			return config.LimitReached(c)
		}

		return c.Next()
	}
}

// APIKeyRateLimiter creates rate limiting based on API key
func APIKeyRateLimiter(cache *cache.RedisCache) fiber.Handler {
	return RateLimiter(RateLimitConfig{
		Cache:       cache,
		MaxRequests: 1000,         // 1000 requests
		Window:      1 * time.Hour, // per hour
		KeyGenerator: func(c fiber.Ctx) string {
			apiKey := c.Get("X-API-Key")
			if apiKey == "" {
				// Fallback to IP-based rate limiting
				return "ip:" + c.IP()
			}
			return "apikey:" + apiKey
		},
	})
}

// IPRateLimiter creates IP-based rate limiting
func IPRateLimiter(cache *cache.RedisCache) fiber.Handler {
	return RateLimiter(RateLimitConfig{
		Cache:       cache,
		MaxRequests: 60,            // 60 requests
		Window:      1 * time.Minute, // per minute
		KeyGenerator: func(c fiber.Ctx) string {
			return "ip:" + c.IP()
		},
	})
}

// UserRateLimiter creates user-based rate limiting
func UserRateLimiter(cache *cache.RedisCache) fiber.Handler {
	return RateLimiter(RateLimitConfig{
		Cache:       cache,
		MaxRequests: 300,           // 300 requests
		Window:      1 * time.Minute, // per minute
		KeyGenerator: func(c fiber.Ctx) string {
			userID := c.Locals("user_id")
			if userID == nil {
				return "ip:" + c.IP()
			}
			return fmt.Sprintf("user:%v", userID)
		},
	})
}
