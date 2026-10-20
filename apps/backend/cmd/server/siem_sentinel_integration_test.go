//go:build integration

package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
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
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/integrations/siem"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/integrations/siem/sentineltest"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/handlers"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/middleware"
)

// A failed sign-in sent through the server's route table against a migrated
// database reaches a loopback Logs Ingestion endpoint as one record, with a
// bearer token from a loopback Microsoft Entra token endpoint, when the
// SIEM_SENTINEL_* variables name them; /metrics counts it for the sentinel
// adapter. Started again with a client secret the token endpoint refuses,
// the next failed sign-in reaches no ingestion endpoint, is counted failed,
// and aim_siem_forwarding_failing reads 1. /metrics is read with the token;
// GET /health/ready carries no siem entry throughout.
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestSIEMSentinelRoundTrip ./cmd/server/
func TestSIEMSentinelRoundTrip(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping SIEM Sentinel integration test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping())

	withEnvironment(t, "", false)
	t.Setenv("JWT_SECRET", "siem-integration-secret-of-at-least-32-chars")
	schema := aim07LoadSchema(t)
	metricsToken := uuid.NewString() + uuid.NewString()
	metricsApp := newMetricsApp(metricsToken)

	tenant, clientID := uuid.NewString(), uuid.NewString()
	clientSecret := strings.Join([]string{"integration", uuid.NewString()}, "-")
	dcr, stream := "dcr-"+strings.ReplaceAll(uuid.NewString(), "-", ""), "Custom-AIMSecurityEvents"
	tokens := sentineltest.NewTokenServer(tenant, clientID, clientSecret)
	t.Cleanup(tokens.Close)
	ingestion := sentineltest.NewIngestionServer(dcr, stream, tokens)
	t.Cleanup(ingestion.Close)
	sentinelEnv := func(secret string) func(string) string {
		return siemEnv(map[string]string{
			"SIEM_SENTINEL_ENDPOINT":         ingestion.Endpoint(),
			"SIEM_SENTINEL_DCR_IMMUTABLE_ID": dcr,
			"SIEM_SENTINEL_STREAM_NAME":      stream,
			"SIEM_SENTINEL_TENANT_ID":        tenant,
			"SIEM_SENTINEL_CLIENT_ID":        clientID,
			"SIEM_SENTINEL_CLIENT_SECRET":    secret,
			"SIEM_SENTINEL_AUTHORITY_HOST":   tokens.AuthorityHost(),
		})
	}

	// The Logs Ingestion API accepts the forwarder's records.
	started := time.Now().UTC().Add(-time.Second)
	app, forwarder := sentinelRoundTripServer(t, db, sentinelEnv(clientSecret))
	sentinelSignInFails(t, app)
	sentinelFlush(t, forwarder)
	ended := time.Now().UTC().Add(time.Second)

	accepted := ingestion.Accepted()
	require.Len(t, accepted, 1, "one record for one failed sign-in; refusals: %v", ingestion.Refusals())
	eventType, ok := accepted[0].String("EventType")
	require.True(t, ok)
	assert.Equal(t, string(siem.EventSignInFailed), eventType)
	generated, ok := accepted[0].String("TimeGenerated")
	require.True(t, ok)
	at, err := time.Parse(time.RFC3339Nano, generated)
	require.NoError(t, err, "TimeGenerated is RFC 3339")
	assert.True(t, !at.Before(started) && !at.After(ended), "TimeGenerated %s is within %s and %s", generated, started, ended)
	assert.Equal(t, 1, tokens.Requests(), "one token for one batch")
	series := siemMetricSeries(t, metricsApp, metricsToken)
	assert.GreaterOrEqual(t, series[`aim_siem_events_sent_total{adapter="sentinel"}`], float64(1), "series: %v", series)
	require.Contains(t, series, "aim_siem_forwarding_failing", "series: %v", series)
	assert.Equal(t, float64(0), series["aim_siem_forwarding_failing"], "the last delivery was accepted")
	readiness := siemReadiness(t, app, schema)
	assert.NotContains(t, readiness["dependencies"], "siem")
	flushSIEMForwarding(forwarder)

	// Started again with a client secret the token endpoint refuses.
	ingestionRequests := ingestion.Requests()
	app, forwarder = sentinelRoundTripServer(t, db, sentinelEnv(clientSecret+"-wrong"))
	failedBefore := siemMetricSeries(t, metricsApp, metricsToken)[`aim_siem_events_failed_total{adapter="sentinel"}`]
	sentinelSignInFails(t, app)
	sentinelFlush(t, forwarder)

	assert.Equal(t, ingestionRequests, ingestion.Requests(), "no upload without a token")
	assert.Len(t, ingestion.Accepted(), 1)
	refusals := tokens.Refusals()
	require.Len(t, refusals, 1, "a refused token is not asked for again within the batch")
	assert.Equal(t, sentineltest.Refusal{Status: 401, Error: "invalid_client"}, refusals[0])
	series = siemMetricSeries(t, metricsApp, metricsToken)
	assert.Greater(t, series[`aim_siem_events_failed_total{adapter="sentinel"}`], failedBefore, "series: %v", series)
	assert.Equal(t, float64(0), series[`aim_siem_events_sent_total{adapter="sentinel"}`], "the restarted forwarder sent nothing")
	assert.Equal(t, float64(1), series["aim_siem_forwarding_failing"], "a refused token: series: %v", series)
	readiness = siemReadiness(t, app, schema)
	assert.Equal(t, false, readiness["degraded"], "SIEM delivery is not a readiness dependency")
	assert.NotContains(t, readiness["dependencies"], "siem")
	flushSIEMForwarding(forwarder)
}

// sentinelRoundTripServer starts SIEM forwarding from getenv and mounts the
// server's routes, with the sign-in observers of observeSecurityDecisions, on
// a fresh set of repositories.
func sentinelRoundTripServer(t *testing.T, db *sql.DB, getenv func(string) string) (*fiber.App, *siem.Forwarder) {
	t.Helper()
	repos, oauthRepo := initRepositories(db)
	forwarder, err := startSIEMForwarding(repos.AuditLog, getenv)
	require.NoError(t, err)
	require.NotNil(t, forwarder, "the SIEM_SENTINEL_* variables configure forwarding")
	t.Cleanup(func() { _ = forwarder.Close(context.Background()) })

	jwtService := auth.NewJWTService()
	auditService := application.NewAuditService(repos.AuditLog)
	authService := application.NewAuthService(repos.User, repos.Organization, repos.APIKey, repos.AuthFailure, nil, nil)
	agentService := application.NewAgentService(repos.Agent, nil, nil, nil, repos.Alert, nil, repos.Capability, nil, nil, repos.User, repos.Organization, nil)
	fga := application.NewFGAEngine(db, agentService, nil)
	t.Cleanup(func() { _ = fga.Shutdown(context.Background()) })
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
	return app, forwarder
}

// sentinelSignInFails sends a sign-in for an email no user or registration
// request has: the route answers 401 and forwards one event.
func sentinelSignInFails(t *testing.T, app *fiber.App) {
	t.Helper()
	raw, err := json.Marshal(map[string]string{
		"email":    "siem-sentinel-" + uuid.NewString()[:8] + "@example.invalid",
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

func sentinelFlush(t *testing.T, forwarder *siem.Forwarder) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	require.NoError(t, forwarder.Flush(ctx))
}
