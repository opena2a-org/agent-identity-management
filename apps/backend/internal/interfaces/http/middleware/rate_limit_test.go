package middleware

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetClientIP_NoTrustedProxies(t *testing.T) {
	// Without trusted proxies configured, should use direct IP
	originalEnv := os.Getenv("TRUSTED_PROXIES")
	os.Unsetenv("TRUSTED_PROXIES")
	defer func() {
		if originalEnv != "" {
			os.Setenv("TRUSTED_PROXIES", originalEnv)
		}
	}()

	app := fiber.New()
	var capturedIP string

	app.Get("/test", func(c fiber.Ctx) error {
		capturedIP = getClientIP(c)
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Forwarded-For", "1.2.3.4, 5.6.7.8")
	req.Header.Set("X-Real-IP", "9.10.11.12")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should ignore X-Forwarded-For and X-Real-IP since no trusted proxies
	assert.NotEqual(t, "1.2.3.4", capturedIP)
	assert.NotEqual(t, "9.10.11.12", capturedIP)
}

func TestGetClientIP_WithTrustedProxy(t *testing.T) {
	// With trusted proxies, should respect X-Real-IP
	originalEnv := os.Getenv("TRUSTED_PROXIES")
	// Note: In test environment, the direct IP is typically "0.0.0.0"
	os.Setenv("TRUSTED_PROXIES", "0.0.0.0")
	defer func() {
		if originalEnv != "" {
			os.Setenv("TRUSTED_PROXIES", originalEnv)
		} else {
			os.Unsetenv("TRUSTED_PROXIES")
		}
	}()

	app := fiber.New()
	var capturedIP string

	app.Get("/test", func(c fiber.Ctx) error {
		capturedIP = getClientIP(c)
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Real-IP", "203.0.113.50")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Should use X-Real-IP when direct connection IP matches a trusted proxy
	assert.Equal(t, "203.0.113.50", capturedIP)
}

// setTrustedProxies points TRUSTED_PROXIES at the given list for one test.
//
// "0.0.0.0" is the direct connection IP fiber reports under app.Test, so including it
// is what opens the forwarded-header path at all — without a trusted direct peer
// getClientIP ignores the headers entirely, which is the correct default and is
// covered by TestGetClientIP_NoTrustedProxies. Anything listed AFTER it is a proxy
// trusted to appear inside X-Forwarded-For.
func setTrustedProxies(t *testing.T, value string) {
	t.Helper()
	t.Setenv("TRUSTED_PROXIES", value)
}

// clientIPFor runs getClientIP against a request carrying the given X-Forwarded-For.
func clientIPFor(t *testing.T, forwardedFor string) string {
	t.Helper()

	app := fiber.New()
	var capturedIP string
	app.Get("/test", func(c fiber.Ctx) error {
		capturedIP = getClientIP(c)
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Forwarded-For", forwardedFor)

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	return capturedIP
}

// TestGetClientIP_XForwardedFor pins the selection rule the limiter key is built from.
//
// This test previously asserted the opposite — that ips[0] is the client. Every hop
// APPENDS to X-Forwarded-For, so ips[0] is a value the client writes itself: a caller
// that varied it got a fresh rate-limit bucket per request, which is no rate limit at
// all on the unauthenticated DID resolver. The rule is now the rightmost entry that is
// not a trusted proxy. COUNCIL_LEDGER 2026-09-02, CHIEF-CISO.
func TestGetClientIP_XForwardedFor(t *testing.T) {
	t.Run("AIMC-06.AC1 with no trusted proxy in the chain the rightmost entry is the key", func(t *testing.T) {
		setTrustedProxies(t, "0.0.0.0")
		// 1.1.1.1 is client-written; 2.2.2.2 is the address that actually reached our
		// ingress. Neither is a trusted proxy, so the walk stops at the first one from
		// the right.
		assert.Equal(t, "2.2.2.2", clientIPFor(t, "1.1.1.1, 2.2.2.2"),
			"the leftmost X-Forwarded-For entry is attacker-controlled and must never select the limiter key")
	})

	t.Run("AIMC-06.AC1 a trusted proxy entry is skipped and the one before it is the key", func(t *testing.T) {
		setTrustedProxies(t, "0.0.0.0,2.2.2.2")
		// 2.2.2.2 is now infrastructure we run: the entry it appended describes the
		// hop before it, so 1.1.1.1 is the furthest-left address we have reason to
		// believe.
		assert.Equal(t, "1.1.1.1", clientIPFor(t, "1.1.1.1, 2.2.2.2"),
			"a trusted proxy in the chain must be walked past, not treated as the client")
	})

	t.Run("AIMC-06.AC1 a chain of nothing but trusted proxies falls back to the direct peer", func(t *testing.T) {
		setTrustedProxies(t, "0.0.0.0,1.1.1.1,2.2.2.2")
		// No entry identifies a client, so there is nothing in the header to key on.
		// Falling back to the direct connection is the safe answer; inventing a key
		// from a trusted proxy would pool every client behind it into one bucket.
		got := clientIPFor(t, "1.1.1.1, 2.2.2.2")
		assert.NotEqual(t, "1.1.1.1", got)
		assert.NotEqual(t, "2.2.2.2", got)
	})

	t.Run("AIMC-06.AC1 a longer chain still resolves to the rightmost untrusted entry", func(t *testing.T) {
		setTrustedProxies(t, "0.0.0.0,10.0.0.1")
		// 10.0.0.1 is our own ingress; 192.0.2.1 spoke to it; 198.51.100.178 is what
		// the caller claimed about itself.
		assert.Equal(t, "192.0.2.1", clientIPFor(t, "198.51.100.178, 192.0.2.1, 10.0.0.1"))
	})
}

// The selection rule only matters because of what it feeds. This asserts the
// consequence directly, through the real middleware: a caller that rewrites the
// leftmost entry on every request still spends one shared bucket.
func TestRateLimitMiddlewareKeyIgnoresClientChosenForwardedForEntries(t *testing.T) {
	t.Run("AIMC-06.AC1 varying the leftmost X-Forwarded-For entry does not buy extra quota", func(t *testing.T) {
		setTrustedProxies(t, "0.0.0.0")

		app := fiber.New()
		app.Use(RateLimitMiddleware())
		app.Get("/test", func(c fiber.Ctx) error {
			return c.JSON(fiber.Map{"message": "success"})
		})

		limit := rateLimitMax(100)
		var lastStatus int
		for i := 0; i <= limit; i++ {
			req := httptest.NewRequest("GET", "/test", nil)
			// A different client-chosen leftmost entry every time; the same real one.
			req.Header.Set("X-Forwarded-For", fmt.Sprintf("203.0.113.%d, 2.2.2.2", i%256))
			resp, err := app.Test(req)
			require.NoError(t, err)
			lastStatus = resp.StatusCode
			resp.Body.Close()
			if lastStatus == fiber.StatusTooManyRequests {
				break
			}
		}

		assert.Equal(t, fiber.StatusTooManyRequests, lastStatus,
			"all %d requests came from 2.2.2.2 and must share one limiter bucket; keying on the "+
				"leftmost entry gave the caller a fresh bucket per request", limit+1)
	})
}

func TestRateLimitMiddleware_WithAuthenticatedUser(t *testing.T) {
	app := fiber.New()

	userID := uuid.New()

	// Add authentication simulation before rate limiter
	app.Use(func(c fiber.Ctx) error {
		c.Locals("user_id", userID)
		return c.Next()
	})
	app.Use(RateLimitMiddleware())
	app.Get("/test", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "success"})
	})

	// First request should succeed
	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestRateLimitMiddleware_WithoutAuthentication(t *testing.T) {
	app := fiber.New()
	app.Use(RateLimitMiddleware())
	app.Get("/test", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "success"})
	})

	// Request without authentication
	req := httptest.NewRequest("GET", "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestStrictRateLimitMiddleware_LimitReached(t *testing.T) {
	app := fiber.New()
	app.Use(StrictRateLimitMiddleware())
	app.Get("/test", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "success"})
	})

	// Make 11 requests (strict limit is 10 per minute)
	hitLimit := false
	for i := 0; i < 11; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		if resp.StatusCode == fiber.StatusTooManyRequests {
			hitLimit = true
			body, _ := io.ReadAll(resp.Body)
			var result map[string]interface{}
			json.Unmarshal(body, &result)
			assert.Contains(t, result["error"], "Rate limit exceeded")
			resp.Body.Close()
			break
		}
		resp.Body.Close()
	}

	assert.True(t, hitLimit, "Should hit rate limit after 10 requests")
}

func TestRateLimitMiddleware_RateLimitExceeded(t *testing.T) {
	app := fiber.New()
	// Create a new rate limiter with very low limit for testing
	app.Use(RateLimitMiddleware())
	app.Get("/test", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"message": "success"})
	})

	// Make 101 requests (limit is 100 per minute)
	hitLimit := false
	for i := 0; i < 101; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)

		if resp.StatusCode == fiber.StatusTooManyRequests {
			hitLimit = true
			body, _ := io.ReadAll(resp.Body)
			var result map[string]interface{}
			json.Unmarshal(body, &result)
			assert.Contains(t, result["error"], "Rate limit exceeded")
			resp.Body.Close()
			break
		}
		resp.Body.Close()
	}

	assert.True(t, hitLimit, "Should hit rate limit after 100 requests")
}

func TestGetClientIP_SpecificTrustedProxy(t *testing.T) {
	originalEnv := os.Getenv("TRUSTED_PROXIES")
	os.Setenv("TRUSTED_PROXIES", "127.0.0.1, 10.0.0.1")
	defer func() {
		if originalEnv != "" {
			os.Setenv("TRUSTED_PROXIES", originalEnv)
		} else {
			os.Unsetenv("TRUSTED_PROXIES")
		}
	}()

	app := fiber.New()
	var capturedIP string

	app.Get("/test", func(c fiber.Ctx) error {
		capturedIP = getClientIP(c)
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	// Set some forwarded headers that should be ignored since direct IP isn't trusted
	req.Header.Set("X-Real-IP", "192.168.1.100")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Since the test connection doesn't come from 127.0.0.1 or 10.0.0.1,
	// headers should be ignored and direct IP should be used
	assert.NotEqual(t, "192.168.1.100", capturedIP)
}

func TestGetClientIP_XRealIPPreferredOverXForwardedFor(t *testing.T) {
	originalEnv := os.Getenv("TRUSTED_PROXIES")
	os.Setenv("TRUSTED_PROXIES", "0.0.0.0")
	defer func() {
		if originalEnv != "" {
			os.Setenv("TRUSTED_PROXIES", originalEnv)
		} else {
			os.Unsetenv("TRUSTED_PROXIES")
		}
	}()

	app := fiber.New()
	var capturedIP string

	app.Get("/test", func(c fiber.Ctx) error {
		capturedIP = getClientIP(c)
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Real-IP", "203.0.113.100")
	req.Header.Set("X-Forwarded-For", "198.51.100.1, 192.0.2.1")

	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// X-Real-IP should be preferred over X-Forwarded-For
	assert.Equal(t, "203.0.113.100", capturedIP)
}
