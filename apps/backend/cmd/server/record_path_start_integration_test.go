//go:build integration

package main

import (
	"bytes"
	"database/sql"
	"io"
	"log"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/metrics"
)

// At boot the server reads the record path's start state once, writes its
// start line and serves its gauges on /metrics. On the shipped schema the
// line and the gauges name each foreign key that removes audit rows by
// cascade.
func TestServerReportsTheRecordPathStartState(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping record path start test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	var logs bytes.Buffer
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	reportRecordPathStart(db)

	shipped := []string{
		"audit_logs.audit_logs_organization_id_fkey",
		"audit_logs.audit_logs_user_id_fkey",
		"verification_events.verification_events_agent_id_fkey",
		"verification_events.verification_events_mcp_server_id_fkey",
		"verification_events.verification_events_organization_id_fkey",
	}
	line := strings.TrimSpace(logs.String())
	assert.Contains(t, line, "record_path_start state=")
	assert.Contains(t, line, "cascading_foreign_keys="+strings.Join(shipped, ","))

	app := fiber.New()
	app.Get("/metrics", metrics.PrometheusHandler())
	resp, err := app.Test(httptest.NewRequest("GET", "/metrics", nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), "\naim_record_path_pre_chain ")
	for _, name := range shipped {
		assert.Contains(t, string(body), `aim_record_cascading_foreign_keys{constraint="`+name+`"} 1`)
	}
}
