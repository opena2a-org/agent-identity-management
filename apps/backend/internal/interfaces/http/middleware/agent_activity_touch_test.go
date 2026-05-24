package middleware

import (
	"context"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type capturingToucher struct {
	mu      sync.Mutex
	calls   []uuid.UUID
	called  atomic.Int32
	err     error
}

func (c *capturingToucher) UpdateLastActive(_ context.Context, agentID uuid.UUID) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, agentID)
	c.called.Add(1)
	return c.err
}

// waitForCalls blocks until the toucher has been invoked `n` times
// or the deadline elapses. The middleware fires UpdateLastActive on
// a separate goroutine, so the test goroutine must synchronize.
func (c *capturingToucher) waitForCalls(n int32, deadline time.Duration) bool {
	end := time.Now().Add(deadline)
	for time.Now().Before(end) {
		if c.called.Load() >= n {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

func TestAgentActivityTouchMiddleware_FiresOnSdkAuth(t *testing.T) {
	toucher := &capturingToucher{}
	agentID := uuid.New()

	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		// Simulate an upstream agent-auth middleware (PQC, API key, ATC).
		c.Locals("agent_id", agentID)
		c.Locals("auth_method", "ed25519")
		return c.Next()
	})
	app.Use(AgentActivityTouchMiddleware(toucher))
	app.Get("/test", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	require.True(t, toucher.waitForCalls(1, 200*time.Millisecond),
		"middleware should call UpdateLastActive once after SDK-authed request")
	assert.Equal(t, []uuid.UUID{agentID}, toucher.calls)
}

func TestAgentActivityTouchMiddleware_SkipsJwtOnlyRequests(t *testing.T) {
	toucher := &capturingToucher{}

	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		// Simulate JWT user auth: only user_id and organization_id, no agent_id.
		c.Locals("user_id", uuid.New())
		c.Locals("organization_id", uuid.New())
		return c.Next()
	})
	app.Use(AgentActivityTouchMiddleware(toucher))
	app.Get("/test", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	// Give any stray goroutine time to fire if the gate is broken.
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int32(0), toucher.called.Load(),
		"JWT-only requests must not touch last_active")
}

func TestAgentActivityTouchMiddleware_SkipsErrorResponses(t *testing.T) {
	toucher := &capturingToucher{}
	agentID := uuid.New()

	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Locals("agent_id", agentID)
		return c.Next()
	})
	app.Use(AgentActivityTouchMiddleware(toucher))
	app.Get("/test", func(c fiber.Ctx) error {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "denied"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, fiber.StatusForbidden, resp.StatusCode)

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int32(0), toucher.called.Load(),
		"4xx responses must not extend last_active")
}

func TestAgentActivityTouchMiddleware_SkipsWhenAgentIDWrongType(t *testing.T) {
	toucher := &capturingToucher{}

	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		// Defensive: an upstream bug stores agent_id as a string. The
		// touch should not panic or fire.
		c.Locals("agent_id", "not-a-uuid")
		return c.Next()
	})
	app.Use(AgentActivityTouchMiddleware(toucher))
	app.Get("/test", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int32(0), toucher.called.Load(),
		"non-UUID agent_id must not fire the touch")
}

func TestAgentActivityTouchMiddleware_SwallowsServiceErrors(t *testing.T) {
	toucher := &capturingToucher{err: assertedError("db unavailable")}
	agentID := uuid.New()

	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Locals("agent_id", agentID)
		return c.Next()
	})
	app.Use(AgentActivityTouchMiddleware(toucher))
	app.Get("/test", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Handler must return success even if the touch fails. The
	// touch is fire-and-forget; an UPDATE error never escapes.
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	require.True(t, toucher.waitForCalls(1, 200*time.Millisecond),
		"middleware should have attempted the touch even though it will fail")
}

type assertedError string

func (e assertedError) Error() string { return string(e) }
