package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
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
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/integrations/siem/hectest"
)

func siemEntry(t *testing.T, body []byte) map[string]any {
	t.Helper()
	deps, ok := aim07TopLevel(t, body)["dependencies"].(map[string]any)
	require.True(t, ok)
	entry, ok := deps["siem"].(map[string]any)
	require.True(t, ok, "the body has no siem dependency: %s", body)
	return entry
}

// The SIEM entry of GET /health/ready is optional: notConfigured without a
// SIEM, unavailable and degraded while its check fails, ok otherwise. The
// response stays 200 and carries no latencyMs for it.
func TestHealthReadySIEMStatusByCheck(t *testing.T) {
	schema := aim07LoadSchema(t)
	for _, tc := range []struct {
		name      string
		checkSIEM func() error
		entry     map[string]any
		degraded  bool
	}{
		{"not configured", nil, map[string]any{"status": "notConfigured", "required": false}, false},
		{"delivering", func() error { return nil }, map[string]any{"status": "ok", "required": false}, false},
		{
			"failing",
			func() error { return errors.New("siem: 403 from the collector at splunk.example.invalid") },
			map[string]any{"status": "unavailable", "required": false, "reason": "dependency check failed"},
			true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := fiber.New()
			app.Get("/health/ready", newHealthReadyHandler(aim07CheckOK, nil, tc.checkSIEM, nil))
			status, _, body := aim07Get(t, app)

			require.Equal(t, http.StatusOK, status, "the SIEM is optional")
			aim07Validate(t, schema, body)
			assert.Equal(t, tc.entry, siemEntry(t, body))
			assert.Equal(t, tc.degraded, aim07TopLevel(t, body)["degraded"])
			assert.NotContains(t, string(body), "splunk.example.invalid", "error text never reaches the body")
		})
	}
}

// siemReadyCheck reads the forwarder's record of its deliveries: it fails
// after a refused delivery, passes after an accepted one, and sends nothing
// to the collector itself.
func TestSIEMReadyCheckReadsTheForwardersDeliveries(t *testing.T) {
	assert.Nil(t, siemReadyCheck(nil), "no SIEM configured")

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
	})
	check := siemReadyCheck(forwarder)
	require.NotNil(t, check)

	writeEntry := func() {
		t.Helper()
		mock.ExpectExec(`INSERT INTO audit_logs`).WillReturnResult(sqlmock.NewResult(0, 1))
		require.NoError(t, auditLogs.Create(&domain.AuditLog{OrganizationID: uuid.New(), Action: domain.AuditActionLogin}))
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, forwarder.Flush(ctx))
	}
	requests := func() int { return len(receiver.Accepted()) + len(receiver.Refusals()) }

	assert.NoError(t, check(), "before the first delivery")

	receiver.RefuseToken(true)
	writeEntry()
	assert.ErrorIs(t, check(), errSIEMDeliveryFailing)

	sent := requests()
	for i := 0; i < 3; i++ {
		_ = check()
	}
	assert.Equal(t, sent, requests(), "the check sends no request")

	receiver.RefuseToken(false)
	writeEntry()
	assert.NoError(t, check(), "an accepted delivery ends the failing state")
	require.NoError(t, mock.ExpectationsWereMet())
}

// startSIEMForwarding puts the forwarder's counters on the registry /metrics
// serves, and leaves none when no SIEM is configured.
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
		`aim_siem_events_sent_total{adapter="splunk"} 0`,
		`aim_siem_events_failed_total{adapter="splunk"} 0`,
		"aim_siem_events_dropped_total 0",
	} {
		assert.Contains(t, exposition, line+"\n")
	}

	none, err := startSIEMForwarding(auditLogs, siemEnv(nil))
	require.NoError(t, err)
	require.Nil(t, none)
	assert.False(t, strings.Contains(scrape(), "aim_siem_"), "no SIEM series without a SIEM")
}
