package main

import (
	"context"
	"errors"
	"regexp"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/redis/go-redis/v9"
)

// buildCommit is the commit the binary was built from, set by the linker:
// -ldflags "-X main.buildCommit=<40-hex git commit>". The Dockerfile of the
// published image (infrastructure/docker/Dockerfile.backend at the repository
// root) takes it as the COMMIT build argument; the quickstart Dockerfile under
// apps/backend takes it as GIT_COMMIT. A build that sets nothing, or anything
// other than a full lowercase 40-hex sha, reports "commit": null in the
// readiness body: the contract's value for a commit that is not known.
var buildCommit string

// buildVersion may be stamped the same way (-X main.buildVersion=<value>);
// the Dockerfile takes it as the VERSION build argument, which the publish
// workflow sets to the git ref name: the release tag, or the commit sha for a
// build that is not a tag. GET /health/ready and GET /api/v1/status report it
// through reportedVersion, so a release tag such as platform-v1.1.0 is
// reported as the bare semver 1.1.0, the version the image tag carries.
// Empty serialises as JSON null.
var buildVersion string

// buildSyncedFrom is set by -X main.buildSyncedFrom=<40-hex> and names the
// agent-identity-management commit the latest merged sync on the deploying
// branch was pulled from; protected paths may lag it, and the sync pull
// request's "Not carried" report is the record of what a sync did not carry.
// A build that sets nothing, or anything other than a full lowercase 40-hex
// sha, leaves "syncedFrom" out of the readiness body.
var buildSyncedFrom string

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
	SyncedFrom   *string                          `json:"syncedFrom,omitempty"`
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

// errRedisStartupFailed is what the readiness check reports for a configured
// Redis whose startup connection failed: the service runs without it until a
// restart, so it stays unavailable even if Redis comes back.
var errRedisStartupFailed = errors.New("redis: configured but the startup connection failed")

// redisReadyCheck returns the readiness check for Redis. A nil result means
// no Redis is configured (notConfigured). A configured Redis whose startup
// connection failed (client nil) always fails the check, so it reads
// unavailable and degraded rather than notConfigured.
func redisReadyCheck(configured bool, client *redis.Client) func(context.Context) error {
	if client != nil {
		return func(ctx context.Context) error {
			return client.Ping(ctx).Err()
		}
	}
	if configured {
		return func(context.Context) error { return errRedisStartupFailed }
	}
	return nil
}

// newHealthReadyHandler builds GET /health/ready. checkDB is the required
// database ping; a nil checkRedis means Redis is not configured (it is
// optional; see redisReadyCheck). The response is 200 iff every required
// dependency is ok; an unavailable optional dependency only sets degraded.
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
			SyncedFrom:   stampedSyncedFrom(),
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

// stampedSyncedFrom returns the build-time sync source when it is a full
// 40-hex SHA, nil otherwise; the field's omitempty then leaves the key out of
// the body rather than serialising null or an empty string.
func stampedSyncedFrom() *string {
	if !buildCommitPattern.MatchString(buildSyncedFrom) {
		return nil
	}
	syncedFrom := buildSyncedFrom
	return &syncedFrom
}

// releaseTagPattern matches a platform release tag (platform-v1.1.0, with an
// optional pre-release or build suffix) and the legacy bare v1.1.0 form, and
// captures the semver after the prefix. SDK tags (sdk-ts-v2.0.0) carry another
// prefix and do not match: they never build the platform images.
var releaseTagPattern = regexp.MustCompile(`^(?:platform-)?v([0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.+-]*)?)$`)

// reportedVersion is the version both routes report for a stamped
// buildVersion: the bare semver when the stamp is a release tag, the stamp as
// it is otherwise (a commit sha, the Dockerfile's "dev" default), nil (JSON
// null) for an unstamped build.
func reportedVersion(stamped string) *string {
	if stamped == "" {
		return nil
	}
	version := stamped
	if m := releaseTagPattern.FindStringSubmatch(stamped); m != nil {
		version = m[1]
	}
	return &version
}

func stampedVersion() *string {
	return reportedVersion(buildVersion)
}
