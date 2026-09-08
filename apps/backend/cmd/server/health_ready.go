package main

import (
	"context"
	"regexp"
	"time"

	"github.com/gofiber/fiber/v3"
)

// buildCommit is stamped at build time via
// -ldflags "-X main.buildCommit=<40-hex git commit>" (see
// infrastructure/docker/Dockerfile.backend). Empty in unstamped builds, which
// serialises as JSON null in the readiness body.
var buildCommit string

// buildVersion may be stamped the same way; empty serialises as JSON null.
var buildVersion string

const (
	healthReadyService = "agent-identity-management"

	// healthReadyDBTimeout bounds the database ping; the base handler used an
	// unbounded db.Ping().
	healthReadyDBTimeout = 2 * time.Second
	// healthReadyRedisTimeout preserves the pre-existing 2 s bounded Redis ping.
	healthReadyRedisTimeout = 2 * time.Second
	// healthReadyDeadline is the handler's own overall deadline.
	healthReadyDeadline = 5 * time.Second
)

// Dependency statuses per apps/backend/contracts/health-ready.schema.json.
const (
	depStatusOK            = "ok"
	depStatusUnavailable   = "unavailable"
	depStatusNotConfigured = "notConfigured"
)

// reason is always one of these fixed strings; driver error text (hosts,
// ports, dial errors) must never reach the response body.
const (
	reasonCheckFailed   = "dependency check failed"
	reasonCheckTimedOut = "dependency check timed out"
)

var buildCommitPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

type healthReadyDependency struct {
	Status    string `json:"status"`
	Required  bool   `json:"required"`
	LatencyMs *int64 `json:"latencyMs,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

type healthReadyBody struct {
	Ready        bool                             `json:"ready"`
	Service      string                           `json:"service"`
	Commit       *string                          `json:"commit"`
	Version      *string                          `json:"version"`
	CheckedAt    string                           `json:"checkedAt"`
	Degraded     bool                             `json:"degraded"`
	Dependencies map[string]healthReadyDependency `json:"dependencies"`
}

// runBoundedCheck runs check under a timeout and returns even when check
// ignores its context and never returns (the goroutine is abandoned; the
// select on ctx.Done() keeps the handler inside its deadline).
func runBoundedCheck(parent context.Context, timeout time.Duration, check func(context.Context) error) (status string, latencyMs int64, reason string) {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	start := time.Now()
	done := make(chan error, 1)
	go func() { done <- check(ctx) }()

	select {
	case err := <-done:
		if err != nil {
			return depStatusUnavailable, time.Since(start).Milliseconds(), reasonCheckFailed
		}
		return depStatusOK, time.Since(start).Milliseconds(), ""
	case <-ctx.Done():
		return depStatusUnavailable, time.Since(start).Milliseconds(), reasonCheckTimedOut
	}
}

// newHealthReadyHandler builds GET /health/ready. checkDB is the required
// database ping; a nil checkRedis means Redis is not configured (it is
// optional and a failed startup connection leaves the client nil). The
// response is 200 iff every required dependency is ok; an unavailable
// optional dependency only sets degraded.
func newHealthReadyHandler(checkDB, checkRedis func(context.Context) error) fiber.Handler {
	return func(c fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(context.Background(), healthReadyDeadline)
		defer cancel()

		deps := make(map[string]healthReadyDependency, 2)

		dbStatus, dbLatency, dbReason := runBoundedCheck(ctx, healthReadyDBTimeout, checkDB)
		deps["database"] = healthReadyDependency{
			Status:    dbStatus,
			Required:  true,
			LatencyMs: &dbLatency,
			Reason:    dbReason,
		}

		degraded := false
		if checkRedis == nil {
			deps["redis"] = healthReadyDependency{Status: depStatusNotConfigured, Required: false}
		} else {
			redisStatus, redisLatency, redisReason := runBoundedCheck(ctx, healthReadyRedisTimeout, checkRedis)
			deps["redis"] = healthReadyDependency{
				Status:    redisStatus,
				Required:  false,
				LatencyMs: &redisLatency,
				Reason:    redisReason,
			}
			degraded = redisStatus == depStatusUnavailable
		}

		ready := dbStatus == depStatusOK
		status := fiber.StatusOK
		if !ready {
			status = fiber.StatusServiceUnavailable
		}

		c.Set(fiber.HeaderCacheControl, "no-store")
		return c.Status(status).JSON(healthReadyBody{
			Ready:        ready,
			Service:      healthReadyService,
			Commit:       stampedCommit(),
			Version:      stampedVersion(),
			CheckedAt:    time.Now().UTC().Format(time.RFC3339),
			Degraded:     degraded,
			Dependencies: deps,
		})
	}
}

// stampedCommit returns the build-time commit when it is a full 40-hex SHA,
// nil (JSON null) otherwise.
func stampedCommit() *string {
	if !buildCommitPattern.MatchString(buildCommit) {
		return nil
	}
	commit := buildCommit
	return &commit
}

func stampedVersion() *string {
	if buildVersion == "" {
		return nil
	}
	version := buildVersion
	return &version
}
