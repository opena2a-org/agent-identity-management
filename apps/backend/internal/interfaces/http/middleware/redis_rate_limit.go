package middleware

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/cache"
)

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	// Default limits (can be overridden per-organization)
	DefaultMaxRequests int           // Max requests per window
	DefaultWindow      time.Duration // Time window

	// Strict limits for sensitive endpoints
	StrictMaxRequests int
	StrictWindow      time.Duration

	// Redis cache (nil = use in-memory fallback)
	Cache *cache.RedisCache
}

// RateLimitInfo contains current rate limit status
type RateLimitInfo struct {
	Limit     int64
	Remaining int64
	Reset     time.Time
}

// DefaultRateLimitConfig returns default configuration from environment
func DefaultRateLimitConfig() *RateLimitConfig {
	defaultMax, _ := strconv.Atoi(getEnvDefault("RATE_LIMIT_DEFAULT_MAX", "100"))
	defaultWindowSec, _ := strconv.Atoi(getEnvDefault("RATE_LIMIT_DEFAULT_WINDOW_SEC", "60"))
	strictMax, _ := strconv.Atoi(getEnvDefault("RATE_LIMIT_STRICT_MAX", "10"))
	strictWindowSec, _ := strconv.Atoi(getEnvDefault("RATE_LIMIT_STRICT_WINDOW_SEC", "60"))

	return &RateLimitConfig{
		DefaultMaxRequests: defaultMax,
		DefaultWindow:      time.Duration(defaultWindowSec) * time.Second,
		StrictMaxRequests:  strictMax,
		StrictWindow:       time.Duration(strictWindowSec) * time.Second,
	}
}

func getEnvDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// RedisRateLimiter provides Redis-based distributed rate limiting
type RedisRateLimiter struct {
	config *RateLimitConfig
}

// NewRedisRateLimiter creates a new Redis-based rate limiter
func NewRedisRateLimiter(config *RateLimitConfig) *RedisRateLimiter {
	return &RedisRateLimiter{config: config}
}

// checkLimit checks if the request should be allowed and returns rate limit info
func (r *RedisRateLimiter) checkLimit(ctx context.Context, key string, limit int64, window time.Duration) (*RateLimitInfo, bool, error) {
	if r.config.Cache == nil {
		// No Redis, always allow (will use in-memory fallback middleware)
		return nil, true, nil
	}

	fullKey := "ratelimit:" + key
	now := time.Now()
	resetTime := now.Add(window)

	// Increment counter
	count, err := r.config.Cache.Increment(ctx, fullKey)
	if err != nil {
		// Redis error - fail open (allow request)
		return nil, true, err
	}

	// Set expiration on first request
	if count == 1 {
		ttl := window
		// Use raw client for expire (we need to access it through the cache interface)
		r.config.Cache.Set(ctx, fullKey+"_reset", resetTime.Unix(), ttl)
	}

	remaining := limit - count
	if remaining < 0 {
		remaining = 0
	}

	info := &RateLimitInfo{
		Limit:     limit,
		Remaining: remaining,
		Reset:     resetTime,
	}

	return info, count <= limit, nil
}

// Middleware creates a Fiber middleware for rate limiting
func (r *RedisRateLimiter) Middleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		key := r.getKey(c)
		limit := int64(r.config.DefaultMaxRequests)
		window := r.config.DefaultWindow

		ctx := context.Background()
		info, allowed, err := r.checkLimit(ctx, key, limit, window)

		// Set rate limit headers
		if info != nil {
			c.Set("X-RateLimit-Limit", strconv.FormatInt(info.Limit, 10))
			c.Set("X-RateLimit-Remaining", strconv.FormatInt(info.Remaining, 10))
			c.Set("X-RateLimit-Reset", strconv.FormatInt(info.Reset.Unix(), 10))
		}

		if err != nil {
			// Log error but allow request (fail open)
			// In production, you might want to use a logger
			fmt.Printf("Rate limit check error: %v\n", err)
		}

		if !allowed {
			c.Set("Retry-After", strconv.FormatInt(int64(window.Seconds()), 10))
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error":      "Rate limit exceeded",
				"retryAfter": int64(window.Seconds()),
			})
		}

		return c.Next()
	}
}

// StrictMiddleware creates a stricter rate limiting middleware for sensitive endpoints
func (r *RedisRateLimiter) StrictMiddleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		key := "strict:" + r.getKey(c)
		limit := int64(r.config.StrictMaxRequests)
		window := r.config.StrictWindow

		ctx := context.Background()
		info, allowed, err := r.checkLimit(ctx, key, limit, window)

		// Set rate limit headers
		if info != nil {
			c.Set("X-RateLimit-Limit", strconv.FormatInt(info.Limit, 10))
			c.Set("X-RateLimit-Remaining", strconv.FormatInt(info.Remaining, 10))
			c.Set("X-RateLimit-Reset", strconv.FormatInt(info.Reset.Unix(), 10))
		}

		if err != nil {
			fmt.Printf("Strict rate limit check error: %v\n", err)
		}

		if !allowed {
			c.Set("Retry-After", strconv.FormatInt(int64(window.Seconds()), 10))
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error":      "Rate limit exceeded. Please try again later.",
				"retryAfter": int64(window.Seconds()),
			})
		}

		return c.Next()
	}
}

// getKey generates the rate limit key based on user/org/IP
func (r *RedisRateLimiter) getKey(c fiber.Ctx) string {
	// Priority: authenticated user > organization > IP

	// Check for authenticated user
	if userID := c.Locals("user_id"); userID != nil {
		if id, ok := userID.(uuid.UUID); ok {
			return "user:" + id.String()
		}
	}

	// Check for organization (from API key or JWT)
	if orgID := c.Locals("organization_id"); orgID != nil {
		if id, ok := orgID.(uuid.UUID); ok {
			return "org:" + id.String()
		}
	}

	// Fall back to IP
	return "ip:" + getClientIP(c)
}

// OrganizationLimitMiddleware creates per-organization rate limiting
// This checks organization-specific limits from settings
func (r *RedisRateLimiter) OrganizationLimitMiddleware(getOrgLimit func(orgID uuid.UUID) (int, time.Duration)) fiber.Handler {
	return func(c fiber.Ctx) error {
		var limit int64
		var window time.Duration

		// Check for organization-specific limits
		if orgID := c.Locals("organization_id"); orgID != nil {
			if id, ok := orgID.(uuid.UUID); ok {
				orgLimit, orgWindow := getOrgLimit(id)
				if orgLimit > 0 {
					limit = int64(orgLimit)
					window = orgWindow
				}
			}
		}

		// Use defaults if no org-specific limit
		if limit == 0 {
			limit = int64(r.config.DefaultMaxRequests)
			window = r.config.DefaultWindow
		}

		key := r.getKey(c)
		ctx := context.Background()
		info, allowed, err := r.checkLimit(ctx, key, limit, window)

		// Set rate limit headers
		if info != nil {
			c.Set("X-RateLimit-Limit", strconv.FormatInt(info.Limit, 10))
			c.Set("X-RateLimit-Remaining", strconv.FormatInt(info.Remaining, 10))
			c.Set("X-RateLimit-Reset", strconv.FormatInt(info.Reset.Unix(), 10))
		}

		if err != nil {
			fmt.Printf("Org rate limit check error: %v\n", err)
		}

		if !allowed {
			c.Set("Retry-After", strconv.FormatInt(int64(window.Seconds()), 10))
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error":      "Rate limit exceeded",
				"retryAfter": int64(window.Seconds()),
			})
		}

		return c.Next()
	}
}
