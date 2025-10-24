package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/opena2a/identity/backend/internal/infrastructure/cache"
)

// CacheConfig holds cache middleware configuration
type CacheConfig struct {
	Cache      *cache.RedisCache
	Expiration time.Duration
	KeyGenerator func(fiber.Ctx) string
	Skip       func(fiber.Ctx) bool
}

// CacheMiddleware creates HTTP response caching middleware
func CacheMiddleware(config CacheConfig) fiber.Handler {
	if config.Expiration == 0 {
		config.Expiration = 5 * time.Minute
	}

	if config.KeyGenerator == nil {
		config.KeyGenerator = func(c fiber.Ctx) string {
			// Generate cache key from method + path + query
			key := c.Method() + ":" + c.Path()
			if queryString := c.Request().URI().QueryString(); len(queryString) > 0 {
				key += "?" + string(queryString)
			}

			// Add user context if available for user-specific caching
			if userID := c.Locals("user_id"); userID != nil {
				key += ":user:" + userID.(string)
			}

			// Hash the key to keep it short
			hash := sha256.Sum256([]byte(key))
			return "http:cache:" + hex.EncodeToString(hash[:])
		}
	}

	if config.Skip == nil {
		config.Skip = func(c fiber.Ctx) bool {
			// Skip caching for non-GET requests
			return c.Method() != fiber.MethodGet
		}
	}

	return func(c fiber.Ctx) error {
		if config.Cache == nil || config.Skip(c) {
			return c.Next()
		}

		key := config.KeyGenerator(c)

		// Try to get cached response
		var cachedResponse []byte
		err := config.Cache.Get(c.Context(), key, &cachedResponse)
		if err == nil {
			// Cache hit
			c.Set("X-Cache", "HIT")
			return c.Send(cachedResponse)
		}

		// Cache miss - capture response
		c.Set("X-Cache", "MISS")

		// Continue with request
		if err := c.Next(); err != nil {
			return err
		}

		// Only cache successful responses (200-299)
		if c.Response().StatusCode() >= 200 && c.Response().StatusCode() < 300 {
			// Cache the response body
			responseBody := c.Response().Body()
			if len(responseBody) > 0 {
				config.Cache.Set(c.Context(), key, responseBody, config.Expiration)
			}
		}

		return nil
	}
}

// AgentListCache creates caching for agent list endpoints
func AgentListCache(cache *cache.RedisCache) fiber.Handler {
	return CacheMiddleware(CacheConfig{
		Cache:      cache,
		Expiration: 2 * time.Minute,
		KeyGenerator: func(c fiber.Ctx) string {
			orgID := c.Locals("organization_id")
			return "agents:list:org:" + orgID.(string)
		},
	})
}

// TrustScoreCache creates caching for trust score endpoints
func TrustScoreCache(cache *cache.RedisCache) fiber.Handler {
	return CacheMiddleware(CacheConfig{
		Cache:      cache,
		Expiration: 15 * time.Minute,
		KeyGenerator: func(c fiber.Ctx) string {
			agentID := c.Params("id")
			return "trust:score:" + agentID
		},
	})
}

// InvalidateCache is a helper to invalidate cache patterns
func InvalidateCache(cache *cache.RedisCache, pattern string) error {
	return cache.DeletePattern(nil, pattern)
}
