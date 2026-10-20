package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/integrations/siem"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/integrations/siem/hectest"
)

// loopbackSIEMForwarding starts SIEM forwarding as main does, to a loopback
// HTTP Event Collector, and returns the forwarder, the collector, and deliver,
// which writes one audit log entry and returns once its delivery has ended,
// accepted or not. The cleanup flushes the forwarder and leaves no SIEM series
// on /metrics.
func loopbackSIEMForwarding(t *testing.T) (*siem.Forwarder, *hectest.Receiver, func()) {
	t.Helper()
	hecToken := uuid.NewString()
	receiver := hectest.NewReceiver(hecToken)
	t.Cleanup(receiver.Close)
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	auditLogs := repository.NewAuditLogRepository(db)
	forwarder, err := startSIEMForwarding(auditLogs, siemEnv(map[string]string{
		"SIEM_SPLUNK_HEC_URL":   receiver.URL(),
		"SIEM_SPLUNK_HEC_TOKEN": hecToken,
	}))
	require.NoError(t, err)
	require.NotNil(t, forwarder)
	t.Cleanup(func() {
		flushSIEMForwarding(forwarder)
		_, _ = startSIEMForwarding(auditLogs, siemEnv(nil))
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	deliver := func() {
		t.Helper()
		mock.ExpectExec(`INSERT INTO audit_logs`).WillReturnResult(sqlmock.NewResult(0, 1))
		require.NoError(t, auditLogs.Create(&domain.AuditLog{OrganizationID: uuid.New(), Action: domain.AuditActionLogin}))
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, forwarder.Flush(ctx))
	}
	return forwarder, receiver, deliver
}

// readinessClockFields matches the values of a readiness body that change with
// the clock rather than with a dependency: checkedAt, and the latencyMs of each
// timed check.
var readinessClockFields = regexp.MustCompile(`"(checkedAt|latencyMs)":("[^"]*"|[0-9]+)`)

// GET /health/ready is served without authentication and carries only the
// dependencies the service needs to answer requests: whatever SIEM forwarding
// is doing, the body has no siem entry and its degraded flag does not move.
// With the database and Redis ok and no revocation list read, the body, its
// checkedAt and each latencyMs normalised, is byte-identical with no SIEM, with
// a SIEM whose last delivery was refused and with one whose last delivery was
// accepted, and each answer is 200. An unavailable Redis changes the same
// body, so the comparison sees a change when there is one. SIEM delivery is
// read on /metrics (TestSIEMReadyCheckReadsTheForwardersDeliveries).
func TestHealthReadySIEMStatusByCheck(t *testing.T) {
	schema := aim07LoadSchema(t)
	read := func(checkRedis func(context.Context) error) (int, []byte) {
		t.Helper()
		app := fiber.New()
		app.Get("/health/ready", newHealthReadyHandler(aim07CheckOK, checkRedis, nil))
		status, _, body := aim07Get(t, app)
		aim07Validate(t, schema, body)
		return status, readinessClockFields.ReplaceAll(body, []byte(`"$1":0`))
	}

	type answer struct {
		state  string
		status int
		body   []byte
	}
	var answers []answer

	status, body := read(aim07CheckOK)
	answers = append(answers, answer{"no SIEM", status, body})
	redisDownStatus, redisDown := read(func(context.Context) error { return errors.New("redis: connection refused") })

	forwarder, receiver, deliver := loopbackSIEMForwarding(t)
	receiver.RefuseToken(true)
	deliver()
	require.True(t, forwarder.Health().Failing, "the collector refused the last delivery")
	status, body = read(aim07CheckOK)
	answers = append(answers, answer{"a SIEM whose last delivery was refused", status, body})

	receiver.RefuseToken(false)
	deliver()
	require.False(t, forwarder.Health().Failing, "the collector accepted the last delivery")
	status, body = read(aim07CheckOK)
	answers = append(answers, answer{"a SIEM whose last delivery was accepted", status, body})

	t.Run("the body is the same whatever the SIEM state", func(t *testing.T) {
		for _, a := range answers {
			deps, ok := aim07TopLevel(t, a.body)["dependencies"].(map[string]any)
			require.True(t, ok, "%s: dependencies must be an object: %s", a.state, a.body)
			assert.NotContains(t, deps, "siem", "%s: the body carries no siem entry", a.state)
			assert.Equal(t, false, aim07TopLevel(t, a.body)["degraded"], a.state)
			assert.Equal(t, string(answers[0].body), string(a.body), "%s: the body is the one with no SIEM", a.state)
		}
	})

	t.Run("an unavailable Redis changes the body", func(t *testing.T) {
		assert.Equal(t, http.StatusOK, redisDownStatus, "Redis is optional")
		assert.Equal(t, true, aim07TopLevel(t, redisDown)["degraded"])
		assert.NotEqual(t, string(answers[0].body), string(redisDown))
	})

	t.Run("the answer is 200 whatever the SIEM state", func(t *testing.T) {
		for _, a := range answers {
			assert.Equal(t, http.StatusOK, a.status, a.state)
		}
	})
}

// metricsBody reads GET /metrics from metricsApp with token as the bearer.
func metricsBody(t *testing.T, metricsApp *fiber.App, token string) string {
	t.Helper()
	req := httptest.NewRequest(fiber.MethodGet, "/metrics", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := metricsApp.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return string(body)
}

// siemFailingGauge reads GET /metrics from metricsApp with token as the bearer
// and returns the value of aim_siem_forwarding_failing, and whether the series
// is there at all.
func siemFailingGauge(t *testing.T, metricsApp *fiber.App, token string) (string, bool) {
	t.Helper()
	for _, line := range strings.Split(metricsBody(t, metricsApp, token), "\n") {
		if value, found := strings.CutPrefix(line, "aim_siem_forwarding_failing "); found {
			return value, true
		}
	}
	return "", false
}

// SIEM delivery is read on /metrics, behind the bearer token: the gauge
// aim_siem_forwarding_failing reads the forwarder's record of its deliveries.
// It is absent without a SIEM, 0 before the first delivery, 1 after a delivery
// the collector refused and 0 again after one it accepted, and reading it
// sends nothing to the collector. Without the token, /metrics answers 401.
func TestSIEMReadyCheckReadsTheForwardersDeliveries(t *testing.T) {
	metricsToken := uuid.NewString() + uuid.NewString()
	metricsApp := newMetricsApp(metricsToken)
	gauge := func() (string, bool) {
		t.Helper()
		return siemFailingGauge(t, metricsApp, metricsToken)
	}

	none, err := startSIEMForwarding(nil, siemEnv(nil))
	require.NoError(t, err)
	require.Nil(t, none)
	_, present := gauge()
	assert.False(t, present, "no SIEM configured: the series is absent")

	forwarder, receiver, deliver := loopbackSIEMForwarding(t)
	requests := func() int { return len(receiver.Accepted()) + len(receiver.Refusals()) }
	value, present := gauge()
	require.True(t, present, "a SIEM is configured: the series is there")
	assert.Equal(t, "0", value, "before the first delivery")

	receiver.RefuseToken(true)
	deliver()
	require.True(t, forwarder.Health().Failing)
	value, _ = gauge()
	assert.Equal(t, "1", value, "the collector refused the last delivery")

	sent := requests()
	for i := 0; i < 3; i++ {
		_, _ = gauge()
	}
	assert.Equal(t, sent, requests(), "reading the gauge sends no request")

	receiver.RefuseToken(false)
	deliver()
	value, _ = gauge()
	assert.Equal(t, "0", value, "an accepted delivery ends the failing state")

	resp, err := metricsApp.Test(httptest.NewRequest(fiber.MethodGet, "/metrics", nil))
	require.NoError(t, err)
	_ = resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode, "without the token /metrics is refused")
}

// startSIEMForwarding puts the forwarder's series on the registry /metrics
// serves, the three counters and the gauge aim_siem_forwarding_failing, and
// leaves none when no SIEM is configured.
func TestStartSIEMForwarding_PutsTheCountersOnMetrics(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	auditLogs := repository.NewAuditLogRepository(db)
	metricsApp := newMetricsApp("")
	scrape := func() string {
		t.Helper()
		resp, err := metricsApp.Test(httptest.NewRequest(fiber.MethodGet, "/metrics", nil))
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		return string(body)
	}

	receiver := hectest.NewReceiver("fake-hec-token")
	t.Cleanup(receiver.Close)
	forwarder, err := startSIEMForwarding(auditLogs, siemEnv(map[string]string{
		"SIEM_SPLUNK_HEC_URL":   receiver.URL(),
		"SIEM_SPLUNK_HEC_TOKEN": "fake-hec-token",
	}))
	require.NoError(t, err)
	require.NotNil(t, forwarder)
	t.Cleanup(func() { flushSIEMForwarding(forwarder) })

	exposition := scrape()
	for _, line := range []string{
		"# TYPE aim_siem_events_sent_total counter",
		"# TYPE aim_siem_events_failed_total counter",
		"# TYPE aim_siem_events_dropped_total counter",
		"# TYPE aim_siem_forwarding_failing gauge",
		`aim_siem_events_sent_total{adapter="splunk"} 0`,
		`aim_siem_events_failed_total{adapter="splunk"} 0`,
		"aim_siem_events_dropped_total 0",
		"aim_siem_forwarding_failing 0",
	} {
		assert.Contains(t, exposition, line+"\n")
	}

	none, err := startSIEMForwarding(auditLogs, siemEnv(nil))
	require.NoError(t, err)
	require.Nil(t, none)
	assert.False(t, strings.Contains(scrape(), "aim_siem_"), "no SIEM series without a SIEM")
}
