package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/handlers"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const onboardingTestPlatformAdmin = "operator@example.com"

// onboardingFakeRepo records client events and serves an empty baseline.
type onboardingFakeRepo struct {
	recorded []domain.OnboardingEvent
}

func (f *onboardingFakeRepo) Record(_ context.Context, e *domain.OnboardingEvent) error {
	f.recorded = append(f.recorded, *e)
	return nil
}
func (f *onboardingFakeRepo) RecordCapped(_ context.Context, e *domain.OnboardingEvent, since time.Time, limit int) (bool, error) {
	n := 0
	for _, got := range f.recorded {
		sameTab := (got.Tab == nil) == (e.Tab == nil) && (got.Tab == nil || *got.Tab == *e.Tab)
		if got.OrganizationID == e.OrganizationID && got.Event == e.Event && sameTab && !got.OccurredAt.Before(since) {
			n++
		}
	}
	if n >= limit {
		return false, nil
	}
	f.recorded = append(f.recorded, *e)
	return true, nil
}
func (f *onboardingFakeRepo) RecordFirstAgent(_ context.Context, _ uuid.UUID) error { return nil }
func (f *onboardingFakeRepo) TimeToFirstAgentSamples(_ context.Context, _ *time.Time) ([]domain.TimeToFirstAgentSample, error) {
	return []domain.TimeToFirstAgentSample{}, nil
}
func (f *onboardingFakeRepo) CountEventsSince(_ context.Context, _ time.Time) ([]domain.OnboardingEventCount, error) {
	return []domain.OnboardingEventCount{}, nil
}

// onboardingRouteApp mounts the real route table, the real platform admin
// gate and the real handler. Authentication is a stand-in that admits a
// request carrying X-Test-Role as X-Test-Email in org X-Test-Org, and refuses
// any other.
func onboardingRouteApp(t *testing.T) (*fiber.App, *onboardingFakeRepo) {
	t.Helper()
	t.Setenv("ENVIRONMENT", "production")

	authenticate := func(c fiber.Ctx) error {
		role := c.Get("X-Test-Role")
		if role == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "session required"})
		}
		c.Locals("role", role)
		c.Locals("email", c.Get("X-Test-Email"))
		c.Locals("user_id", uuid.New())
		org, err := uuid.Parse(c.Get("X-Test-Org"))
		if err != nil {
			org = uuid.New()
		}
		c.Locals("organization_id", org)
		return c.Next()
	}

	repo := &onboardingFakeRepo{}
	svc := application.NewOnboardingTelemetryService(repo)
	h := handlers.NewOnboardingTelemetryHandler(svc)

	app := fiber.New()
	registerOnboardingTelemetryRoutes(app.Group("/api/v1"), onboardingTelemetryRouteDeps{
		Authenticate:         authenticate,
		RequirePlatformAdmin: middleware.PlatformAdminAllowlistMiddleware(func(email string) bool { return email == onboardingTestPlatformAdmin }),
		EventLimit:           middleware.RateLimitMiddleware(),
		MetricsLimit:         middleware.RateLimitMiddleware(),
		RecordEvent:          h.RecordEvent,
		Metrics:              h.GetBaseline,
	})
	return app, repo
}

type onboardingRequest struct {
	method, path, role, email, org, body string
}

func onboardingRouteDo(t *testing.T, app *fiber.App, r onboardingRequest) (int, []byte) {
	t.Helper()
	req := httptest.NewRequest(r.method, "/api/v1"+r.path, bytes.NewBufferString(r.body))
	req.Header.Set("Content-Type", "application/json")
	if r.role != "" {
		req.Header.Set("X-Test-Role", r.role)
	}
	if r.email != "" {
		req.Header.Set("X-Test-Email", r.email)
	}
	if r.org != "" {
		req.Header.Set("X-Test-Org", r.org)
	}
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(resp.Body)
	return resp.StatusCode, buf.Bytes()
}

// The metrics read every organization: only an org admin on the platform
// admin allowlist may read them.
func TestOnboardingRoutes_MetricsNeedAnAllowlistedAdmin(t *testing.T) {
	app, _ := onboardingRouteApp(t)
	get := func(role, email string) int {
		status, _ := onboardingRouteDo(t, app, onboardingRequest{method: http.MethodGet, path: onboardingMetricsPath, role: role, email: email})
		return status
	}

	assert.Equal(t, fiber.StatusUnauthorized, get("", ""))
	assert.Equal(t, fiber.StatusForbidden, get("admin", "customer-admin@example.com"), "an org admin is not a platform admin")
	assert.Equal(t, fiber.StatusForbidden, get("member", onboardingTestPlatformAdmin), "a listed address needs the admin role")
	assert.Equal(t, fiber.StatusForbidden, get("service", onboardingTestPlatformAdmin))
	assert.Equal(t, fiber.StatusForbidden, get("admin", ""))
	assert.Equal(t, fiber.StatusOK, get("admin", onboardingTestPlatformAdmin))
}

func TestOnboardingRoutes_MetricsResponseShape(t *testing.T) {
	app, _ := onboardingRouteApp(t)
	status, body := onboardingRouteDo(t, app, onboardingRequest{method: http.MethodGet, path: onboardingMetricsPath, role: "admin", email: onboardingTestPlatformAdmin})
	require.Equal(t, fiber.StatusOK, status)

	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &decoded))
	for _, key := range []string{"generatedAt", "windowDays", "allTime", "recent", "events"} {
		assert.Contains(t, decoded, key)
	}
	events, ok := decoded["events"].([]interface{})
	require.True(t, ok, "events is an array")
	assert.Len(t, events, len(domain.OnboardingEventTypes))
}

// Any signed-in role reports events for its own organization, and the stored
// row carries the organization from the session, never from the body.
func TestOnboardingRoutes_EventIsKeyedOnTheSessionOrganization(t *testing.T) {
	app, repo := onboardingRouteApp(t)
	org := uuid.New()

	status, _ := onboardingRouteDo(t, app, onboardingRequest{method: http.MethodPost, path: onboardingEventsPath, body: `{"event":"onboarding_viewed"}`})
	assert.Equal(t, fiber.StatusUnauthorized, status)

	status, _ = onboardingRouteDo(t, app, onboardingRequest{
		method: http.MethodPost, path: onboardingEventsPath, role: "viewer", email: "viewer@example.com", org: org.String(),
		body: `{"event":"tab_selected","tab":"python","organizationId":"` + uuid.New().String() + `","email":"viewer@example.com"}`,
	})
	assert.Equal(t, fiber.StatusAccepted, status)

	require.Len(t, repo.recorded, 1)
	assert.Equal(t, org, repo.recorded[0].OrganizationID)
	assert.Equal(t, domain.OnboardingEventTabSelected, repo.recorded[0].Event)
	assert.Equal(t, "python", *repo.recorded[0].Tab)
}

// A signed-in viewer posting the same event in a loop stops adding rows at
// the per-organization cap; the extra reports are accepted and not stored.
func TestOnboardingRoutes_EventLoopIsCappedPerOrganization(t *testing.T) {
	app, repo := onboardingRouteApp(t)
	org := uuid.New()

	for i := 0; i < application.OnboardingClientEventCap+10; i++ {
		status, body := onboardingRouteDo(t, app, onboardingRequest{
			method: http.MethodPost, path: onboardingEventsPath, role: "viewer", email: "viewer@example.com", org: org.String(),
			body: `{"event":"onboarding_viewed"}`,
		})
		require.Equal(t, fiber.StatusAccepted, status)
		want := `{"recorded":true}`
		if i >= application.OnboardingClientEventCap {
			want = `{"recorded":false}`
		}
		assert.JSONEq(t, want, string(body), "report %d", i+1)
	}
	assert.Len(t, repo.recorded, application.OnboardingClientEventCap)
}

func TestOnboardingRoutes_EventRefusesServerEventsAndBadBodies(t *testing.T) {
	app, repo := onboardingRouteApp(t)
	for _, body := range []string{
		`{"event":"token_minted"}`,
		`{"event":"first_agent_registered"}`,
		`{"event":"agent_deleted"}`,
		`{"event":"tab_selected","tab":"cobol"}`,
		`{"event":"onboarding_skipped","tab":"python"}`,
		`not json`,
	} {
		status, resp := onboardingRouteDo(t, app, onboardingRequest{method: http.MethodPost, path: onboardingEventsPath, role: "member", body: body})
		assert.Equal(t, fiber.StatusBadRequest, status, body)
		assert.False(t, strings.Contains(string(resp), "pq:"), body)
	}
	assert.Empty(t, repo.recorded)
}
