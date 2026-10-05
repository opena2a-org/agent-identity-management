package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// deviceCodeSink accepts the code the device service stores; nothing else on
// the interface is reached by InitiateDeviceAuth.
type deviceCodeSink struct{ domain.DeviceCodeRepository }

func (deviceCodeSink) Create(context.Context, *domain.DeviceCode) error { return nil }

// Every /oauth/device route sits behind the strict limiter. In its busiest
// minute a CLI login sends the code request, one poll per interval the server
// handed out, and, when the approving browser shares the CLI's address, the
// dashboard's verify and approve calls. In production that minute has to fit
// the limiter, or a login that is not approved at once is answered 429 by the
// same server that set the interval.
func TestStrictRateLimit_DeviceLoginBusiestMinute_IsNotThrottledInProduction(t *testing.T) {
	t.Setenv("ENVIRONMENT", "production")

	svc := application.NewDeviceAuthService(deviceCodeSink{}, nil, nil, "http://localhost:3000")
	start, err := svc.InitiateDeviceAuth(context.Background(), "aim-sdk", "", "127.0.0.1", "aim-sdk/test")
	require.NoError(t, err)
	require.Positive(t, start.Interval)

	app := fiber.New()
	device := app.Group("/api/v1/oauth/device")
	device.Use(StrictRateLimitMiddleware())
	ok := func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) }
	device.Post("/code", ok)
	device.Post("/token", ok)
	device.Get("/verify", ok)
	device.Post("/approve", ok)

	// The limiter counts in fixed 60-second windows, so the most polls one
	// window can hold is ceil(60 / interval).
	polls := (60 + start.Interval - 1) / start.Interval
	requests := []*http.Request{httptest.NewRequest(fiber.MethodPost, "/api/v1/oauth/device/code", nil)}
	for i := 0; i < polls; i++ {
		requests = append(requests, httptest.NewRequest(fiber.MethodPost, "/api/v1/oauth/device/token", nil))
	}
	requests = append(requests,
		httptest.NewRequest(fiber.MethodGet, "/api/v1/oauth/device/verify?user_code="+start.UserCode, nil),
		httptest.NewRequest(fiber.MethodPost, "/api/v1/oauth/device/approve", nil),
	)

	for i, req := range requests {
		resp, err := app.Test(req)
		require.NoError(t, err)
		resp.Body.Close()
		assert.NotEqual(t, fiber.StatusTooManyRequests, resp.StatusCode,
			fmt.Sprintf("request %d of %d (%s %s) was throttled: a %d s poll interval sends %d requests in one minute",
				i+1, len(requests), req.Method, req.URL.Path, start.Interval, len(requests)))
	}
}
