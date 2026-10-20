//go:build integration

package main

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/integrations/siem/hectest"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/handlers"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/middleware"
)

// Failed sign-ins sent through the server's route table against a migrated
// database are forwarded to a loopback collector that refuses the token,
// then accepts it, then is closed. After each flush GET /metrics, read with
// the token, reports what became of the delivery: aim_siem_forwarding_failing
// is 1 while deliveries fail and 0 once one is accepted, and the counters
// count each event by outcome. GET /health/ready answers 200 throughout, not
// degraded and with no siem entry. With no SIEM configured /metrics carries no
// SIEM series.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestSIEMHealthRoundTrip ./cmd/server/
func TestSIEMHealthRoundTrip(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping SIEM health integration test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping())

	withEnvironment(t, "", false)
	t.Setenv("JWT_SECRET", "siem-integration-secret-of-at-least-32-chars")
	schema := aim07LoadSchema(t)

	hecToken := uuid.NewString()
	receiver := hectest.NewReceiver(hecToken)
	t.Cleanup(receiver.Close)

	repos, oauthRepo := initRepositories(db)
	forwarder, err := startSIEMForwarding(repos.AuditLog, siemEnv(map[string]string{
		"SIEM_SPLUNK_HEC_URL":   receiver.URL(),
		"SIEM_SPLUNK_HEC_TOKEN": hecToken,
	}))
	require.NoError(t, err)
	require.NotNil(t, forwarder)
	t.Cleanup(func() { flushSIEMForwarding(forwarder) })

	ctx := context.Background()
	jwtService := auth.NewJWTService()
	auditService := application.NewAuditService(repos.AuditLog)
	authService := application.NewAuthService(repos.User, repos.Organization, repos.APIKey, repos.AuthFailure, nil, nil)
	agentService := application.NewAgentService(repos.Agent, nil, nil, nil, repos.Alert, nil, repos.Capability, nil, nil, repos.User, repos.Organization, nil)
	fga := application.NewFGAEngine(db, agentService, nil)
	t.Cleanup(func() { _ = fga.Shutdown(ctx) })
	services := &Services{
		Auth:    authService,
		Audit:   auditService,
		Agent:   agentService,
		FGA:     fga,
		Secrets: &application.SecretsService{},
	}
	h := &Handlers{
		Auth: handlers.NewAuthHandler(authService, jwtService, repos.Organization, auditService, nil),
		PublicRegistration: handlers.NewPublicRegistrationHandler(
			application.NewRegistrationService(oauthRepo, repos.User, repos.Organization, auditService, nil),
			authService, jwtService, auditService),
		Authorize: handlers.NewAuthorizeHandler(fga, repos.Agent),
	}
	observeSecurityDecisions(forwarder, services, h)

	app := fiber.New(fiber.Config{ErrorHandler: customErrorHandler})
	app.Use(middleware.RecoveryMiddleware())
	app.Get("/health/ready", newHealthReadyHandler(db.PingContext, redisReadyCheck(false, nil), nil))
	mountAPIRoutes(app, h, services, jwtService, nil, db)
	metricsToken := uuid.NewString() + uuid.NewString()
	metricsApp := newMetricsApp(metricsToken)

	// signInFails sends a sign-in for an email no user or registration
	// request has: the route answers 401 and forwards one event.
	signInFails := func() {
		t.Helper()
		raw, err := json.Marshal(map[string]string{
			"email":    "siem-health-" + uuid.NewString()[:8] + "@example.invalid",
			"password": "Pw-" + uuid.NewString(),
		})
		require.NoError(t, err)
		req := httptest.NewRequest(fiber.MethodPost, "/api/v1/public/login", strings.NewReader(string(raw)))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, fiber.TestConfig{Timeout: 30 * time.Second, FailOnTimeout: true})
		require.NoError(t, err)
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		require.Equal(t, fiber.StatusUnauthorized, resp.StatusCode, string(body))
	}
	flush := func() {
		t.Helper()
		// The closed collector's batch takes three attempts, 1s and 2s apart.
		flushCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		require.NoError(t, forwarder.Flush(flushCtx))
	}

	// The collector refuses the token: 403, code 4, not retried.
	receiver.RefuseToken(true)
	signInFails()
	signInFails()
	flush()

	readiness := siemReadiness(t, app, schema)
	assert.Equal(t, false, readiness["degraded"], "SIEM delivery is not a readiness dependency")
	assert.Equal(t, true, readiness["ready"])
	assert.NotContains(t, readiness["dependencies"], "siem")
	series := siemMetricSeries(t, metricsApp, metricsToken)
	assert.Equal(t, map[string]float64{
		`aim_siem_events_sent_total{adapter="splunk"}`:   0,
		`aim_siem_events_failed_total{adapter="splunk"}`: 2,
		`aim_siem_events_dropped_total`:                  0,
		`aim_siem_forwarding_failing`:                    1,
	}, series, "after two refused events")
	refusals := receiver.Refusals()
	require.NotEmpty(t, refusals)
	for _, refusal := range refusals {
		assert.Equal(t, hectest.Refusal{Status: 403, Code: 4, Text: "Invalid token"}, refusal)
	}
	assert.Empty(t, receiver.Accepted())

	// The collector accepts again: the next delivered event returns the gauge to 0.
	receiver.RefuseToken(false)
	signInFails()
	flush()

	readiness = siemReadiness(t, app, schema)
	assert.Equal(t, false, readiness["degraded"])
	assert.NotContains(t, readiness["dependencies"], "siem")
	series = siemMetricSeries(t, metricsApp, metricsToken)
	assert.Equal(t, map[string]float64{
		`aim_siem_events_sent_total{adapter="splunk"}`:   1,
		`aim_siem_events_failed_total{adapter="splunk"}`: 2,
		`aim_siem_events_dropped_total`:                  0,
		`aim_siem_forwarding_failing`:                    0,
	}, series, "after one accepted event")
	accepted := receiver.Accepted()
	require.Len(t, accepted, 1)
	var event map[string]any
	require.NoError(t, accepted[0].Decode(&event))
	assert.Equal(t, "aim.auth.signin_failed", event["eventType"])

	// The collector is closed: every attempt is refused a connection.
	receiver.Close()
	started := time.Now()
	signInFails()
	flush()
	assert.GreaterOrEqual(t, time.Since(started), 3*time.Second, "the batch is tried three times, 1s and 2s apart")

	readiness = siemReadiness(t, app, schema)
	assert.Equal(t, false, readiness["degraded"])
	assert.NotContains(t, readiness["dependencies"], "siem")
	series = siemMetricSeries(t, metricsApp, metricsToken)
	assert.Equal(t, map[string]float64{
		`aim_siem_events_sent_total{adapter="splunk"}`:   1,
		`aim_siem_events_failed_total{adapter="splunk"}`: 3,
		`aim_siem_events_dropped_total`:                  0,
		`aim_siem_forwarding_failing`:                    1,
	}, series, "after one event the closed collector could not take")

	// No SIEM configured: no SIEM series, and readiness as before.
	flushSIEMForwarding(forwarder)
	none, err := startSIEMForwarding(repos.AuditLog, siemEnv(nil))
	require.NoError(t, err)
	require.Nil(t, none)

	readiness = siemReadiness(t, app, schema)
	assert.Equal(t, false, readiness["degraded"])
	assert.NotContains(t, readiness["dependencies"], "siem")
	assert.Empty(t, siemMetricSeries(t, metricsApp, metricsToken), "no SIEM series without a SIEM")
	assert.NotContains(t, metricsBody(t, metricsApp, metricsToken), "aim_siem_")
}

// siemReadiness reads GET /health/ready, which must answer 200 with a body
// the vendored contract accepts.
func siemReadiness(t *testing.T, app *fiber.App, schema *aim07Schema) map[string]any {
	t.Helper()
	status, _, body := aim07Get(t, app)
	require.Equal(t, fiber.StatusOK, status, string(body))
	aim07Validate(t, schema, body)
	return aim07TopLevel(t, body)
}

// siemMetricSeries returns each aim_siem_ sample of GET /metrics, read with
// token as the bearer, keyed by its name and labels as the exposition prints
// them.
func siemMetricSeries(t *testing.T, metricsApp *fiber.App, token string) map[string]float64 {
	t.Helper()
	series := map[string]float64{}
	scanner := bufio.NewScanner(strings.NewReader(metricsBody(t, metricsApp, token)))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "aim_siem_") {
			continue
		}
		at := strings.LastIndex(line, " ")
		require.NotEqual(t, -1, at, "a sample line carries a value: %q", line)
		value, err := strconv.ParseFloat(line[at+1:], 64)
		require.NoError(t, err, "sample %q", line)
		series[line[:at]] = value
	}
	require.NoError(t, scanner.Err())
	return series
}
