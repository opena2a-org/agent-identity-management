package metrics

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/prometheus/client_golang/prometheus/testutil/promlint"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The exposition PrometheusHandler serves, with every recorder in this
// package observed, passes the checks promtool check metrics runs: a gauge
// has no _total suffix, a counter has one, and so on
// (https://prometheus.io/docs/practices/naming/).
func TestServedMetricsPassPromlint(t *testing.T) {
	httpRequestsTotal.WithLabelValues("GET", "/api/v1/agents", "200").Inc()
	httpRequestDuration.WithLabelValues("GET", "/api/v1/agents", "200").Observe(0.01)
	RecordSecurityAlert("high", "brute_force")
	RecordSecurityThreat("high", "injection")
	UpdateTrustScore(80)
	RecordAgentOperation("create", "success")
	UpdateActiveAgents(3)
	UpdateMCPServersTotal(2)
	RecordVerificationEvent("signature", "success")
	ObserveVerificationDuration("signature", 0.01)
	RecordComplianceCheck("policy", "pass")
	RecordComplianceViolation()
	UpdateDatabaseConnections(4)
	ObserveDatabaseQueryDuration("select", 0.01)
	RecordAPIKeyOperation("create", "success")
	UpdateActiveAPIKeys(1)
	RecordAuditLog("create", "agent")
	RecordS1Refusal(S1ReasonInvalidTimestamp, "aim-sdk-python/1.0")
	RecordAgentRequestV2Refusal(AgentRequestV2ReasonSkewPast, "AIM-SDK-TypeScript/1.4.0")

	app := fiber.New()
	app.Get("/metrics", PrometheusHandler())
	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/metrics", nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, fiber.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	exposition := string(body)

	problems, err := promlint.New(strings.NewReader(exposition)).Lint()
	require.NoError(t, err)
	for _, p := range problems {
		t.Errorf("%s: %s", p.Metric, p.Text)
	}

	assert.True(t, strings.Contains(exposition, "# TYPE aim_mcp_servers gauge\n"), "positive control: the MCP server gauge is not served as aim_mcp_servers")
	assert.True(t, strings.Contains(exposition, "\naim_mcp_servers 2\n"), "the MCP server gauge does not keep its value")
	assert.False(t, strings.Contains(exposition, "aim_mcp_servers_total"), "the gauge is served under its old counter-style name")
}
