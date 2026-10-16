package main

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v3"
)

// statusRedisPingTimeout bounds the Redis ping of GET /api/v1/status.
const statusRedisPingTimeout = 2 * time.Second

// newStatusHandler builds GET /api/v1/status (no auth required). pingDB is
// the database ping; a nil pingRedis means no Redis client is in use;
// emailConfigured reports whether the email service started; environment is
// the ENVIRONMENT the server read at startup. The version is
// the stamped build version, reported exactly as GET /health/ready reports
// it (see reportedVersion), so the two routes never disagree.
func newStatusHandler(pingDB func() error, pingRedis func(context.Context) error, emailConfigured bool, environment string, startTime time.Time) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Check database status
		dbStatus := "healthy"
		if err := pingDB(); err != nil {
			dbStatus = "unavailable"
		}

		// Check Redis status (optional)
		redisStatus := "not configured"
		if pingRedis != nil {
			ctx, cancel := context.WithTimeout(context.Background(), statusRedisPingTimeout)
			defer cancel()
			if err := pingRedis(ctx); err != nil {
				redisStatus = "unavailable"
			} else {
				redisStatus = "healthy"
			}
		}

		// Check email service status
		emailStatus := "unavailable"
		if emailConfigured {
			emailStatus = "healthy"
		}

		return c.JSON(fiber.Map{
			"status":      "operational",
			"version":     stampedVersion(),
			"environment": environment,
			"uptime":      time.Since(startTime).Seconds(),
			"services": fiber.Map{
				"database": dbStatus,
				"redis":    redisStatus,
				"email":    emailStatus,
			},
			"features": fiber.Map{
				"email_registration": true,
				"mcp_auto_detection": true,
				"trust_scoring":      true,
			},
		})
	}
}
