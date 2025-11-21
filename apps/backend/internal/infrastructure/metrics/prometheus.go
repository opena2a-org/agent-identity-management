package metrics

import (
	"bytes"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/expfmt"
)

// Custom registry to avoid conflicts with default registry
var registry = prometheus.NewRegistry()

func init() {
	// Register all metrics with custom registry
	registry.MustRegister(
		httpRequestsTotal,
		httpRequestDuration,
		securityAlertsTotal,
		securityThreatsTotal,
		trustScoreGauge,
		trustScoreHistogram,
		agentOperationsTotal,
		activeAgentsGauge,
		mcpServersTotal,
		mcpAttestationsTotal,
		verificationEventsTotal,
		verificationDuration,
		complianceChecksTotal,
		complianceViolationsTotal,
		databaseConnectionsActive,
		databaseQueryDuration,
		apiKeyOperationsTotal,
		activeAPIKeysGauge,
		auditLogsTotal,
	)
}

var (
	// HTTP metrics
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "aim_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "aim_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "status"},
	)

	// Security metrics
	securityAlertsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "aim_security_alerts_total",
			Help: "Total number of security alerts",
		},
		[]string{"severity", "type"},
	)

	securityThreatsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "aim_security_threats_total",
			Help: "Total number of detected security threats",
		},
		[]string{"severity", "type"},
	)

	// Trust score metrics
	trustScoreGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "aim_trust_score",
			Help: "Current trust score of agents",
		},
		[]string{"agent_id", "agent_name"},
	)

	trustScoreHistogram = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "aim_trust_score_distribution",
			Help:    "Distribution of trust scores across all agents",
			Buckets: []float64{0, 10, 20, 30, 40, 50, 60, 70, 80, 90, 100},
		},
	)

	// Agent metrics
	agentOperationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "aim_agent_operations_total",
			Help: "Total number of agent operations",
		},
		[]string{"operation", "status"},
	)

	activeAgentsGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "aim_active_agents",
			Help: "Number of currently active agents",
		},
	)

	// MCP Server metrics
	mcpServersTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "aim_mcp_servers_total",
			Help: "Total number of registered MCP servers",
		},
	)

	mcpAttestationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "aim_mcp_attestations_total",
			Help: "Total number of MCP attestations",
		},
		[]string{"agent_id", "mcp_id", "status"},
	)

	// Verification metrics
	verificationEventsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "aim_verification_events_total",
			Help: "Total number of verification events",
		},
		[]string{"event_type", "status"},
	)

	verificationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "aim_verification_duration_seconds",
			Help:    "Duration of verification events in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"event_type"},
	)

	// Compliance metrics
	complianceChecksTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "aim_compliance_checks_total",
			Help: "Total number of compliance checks",
		},
		[]string{"check_type", "status"},
	)

	complianceViolationsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "aim_compliance_violations_total",
			Help: "Total number of compliance violations detected",
		},
	)

	// Database metrics
	databaseConnectionsActive = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "aim_database_connections_active",
			Help: "Number of active database connections",
		},
	)

	databaseQueryDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "aim_database_query_duration_seconds",
			Help:    "Database query duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"query_type"},
	)

	// API Key metrics
	apiKeyOperationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "aim_api_key_operations_total",
			Help: "Total number of API key operations",
		},
		[]string{"operation", "status"},
	)

	activeAPIKeysGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "aim_active_api_keys",
			Help: "Number of currently active API keys",
		},
	)

	// Audit log metrics
	auditLogsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "aim_audit_logs_total",
			Help: "Total number of audit log entries",
		},
		[]string{"action", "resource_type"},
	)
)

// PrometheusMiddleware collects HTTP metrics for all requests
func PrometheusMiddleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		// Skip metrics collection for the /metrics endpoint itself to avoid circular recording
		if c.Path() == "/metrics" {
			return c.Next()
		}

		start := time.Now()

		// Process request
		err := c.Next()

		// Record metrics
		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Response().StatusCode())
		method := c.Method()
		path := normalizePath(c.Path())

		httpRequestsTotal.WithLabelValues(method, path, status).Inc()
		httpRequestDuration.WithLabelValues(method, path, status).Observe(duration)

		return err
	}
}

// normalizePath normalizes request paths to prevent high cardinality in Prometheus
// Replaces UUIDs and dynamic IDs with placeholders to avoid label explosion
func normalizePath(path string) string {
	// Limit path length to prevent truncation issues
	if len(path) > 100 {
		path = path[:100]
	}

	// Replace common dynamic segments with placeholders
	// UUIDs: /agents/550e8400-e29b-41d4-a716-446655440000 -> /agents/:id
	parts := strings.Split(path, "/")
	for i, part := range parts {
		// Replace UUIDs (8-4-4-4-12 format)
		if len(part) == 36 && strings.Count(part, "-") == 4 {
			parts[i] = ":id"
		}
		// Replace numeric IDs
		if len(part) > 0 && isNumeric(part) {
			parts[i] = ":id"
		}
	}

	return strings.Join(parts, "/")
}

// isNumeric checks if a string contains only digits
func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// RecordSecurityAlert records a security alert
func RecordSecurityAlert(severity, alertType string) {
	securityAlertsTotal.WithLabelValues(severity, alertType).Inc()
}

// RecordSecurityThreat records a security threat
func RecordSecurityThreat(severity, threatType string) {
	securityThreatsTotal.WithLabelValues(severity, threatType).Inc()
}

// UpdateTrustScore updates the trust score for an agent
func UpdateTrustScore(agentID, agentName string, score float64) {
	trustScoreGauge.WithLabelValues(agentID, agentName).Set(score)
	trustScoreHistogram.Observe(score)
}

// RecordAgentOperation records an agent operation
func RecordAgentOperation(operation, status string) {
	agentOperationsTotal.WithLabelValues(operation, status).Inc()
}

// UpdateActiveAgents updates the count of active agents
func UpdateActiveAgents(count float64) {
	activeAgentsGauge.Set(count)
}

// UpdateMCPServersTotal updates the total number of MCP servers
func UpdateMCPServersTotal(count float64) {
	mcpServersTotal.Set(count)
}

// RecordMCPAttestation records an MCP attestation
func RecordMCPAttestation(agentID, mcpID, status string) {
	mcpAttestationsTotal.WithLabelValues(agentID, mcpID, status).Inc()
}

// RecordVerificationEvent records a verification event
func RecordVerificationEvent(eventType, status string) {
	verificationEventsTotal.WithLabelValues(eventType, status).Inc()
}

// ObserveVerificationDuration observes the duration of a verification event
func ObserveVerificationDuration(eventType string, duration float64) {
	verificationDuration.WithLabelValues(eventType).Observe(duration)
}

// RecordComplianceCheck records a compliance check
func RecordComplianceCheck(checkType, status string) {
	complianceChecksTotal.WithLabelValues(checkType, status).Inc()
}

// RecordComplianceViolation records a compliance violation
func RecordComplianceViolation() {
	complianceViolationsTotal.Inc()
}

// UpdateDatabaseConnections updates the count of active database connections
func UpdateDatabaseConnections(count float64) {
	databaseConnectionsActive.Set(count)
}

// ObserveDatabaseQueryDuration observes the duration of a database query
func ObserveDatabaseQueryDuration(queryType string, duration float64) {
	databaseQueryDuration.WithLabelValues(queryType).Observe(duration)
}

// RecordAPIKeyOperation records an API key operation
func RecordAPIKeyOperation(operation, status string) {
	apiKeyOperationsTotal.WithLabelValues(operation, status).Inc()
}

// UpdateActiveAPIKeys updates the count of active API keys
func UpdateActiveAPIKeys(count float64) {
	activeAPIKeysGauge.Set(count)
}

// RecordAuditLog records an audit log entry
func RecordAuditLog(action, resourceType string) {
	auditLogsTotal.WithLabelValues(action, resourceType).Inc()
}

// PrometheusHandler returns a Fiber handler that exposes Prometheus metrics
// Thread-safe implementation that gathers and encodes metrics on each request
func PrometheusHandler() fiber.Handler {
	return func(c fiber.Ctx) error {
		// Set response headers for Prometheus text format
		c.Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

		// Gather metrics from our custom registry
		metricFamilies, err := registry.Gather()
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("Error gathering metrics: " + err.Error())
		}

		// Encode metrics to a buffer first (thread-safe)
		var buf bytes.Buffer
		encoder := expfmt.NewEncoder(&buf, expfmt.NewFormat(expfmt.TypeTextPlain))

		for _, mf := range metricFamilies {
			if err := encoder.Encode(mf); err != nil {
				return c.Status(fiber.StatusInternalServerError).SendString("Error encoding metrics: " + err.Error())
			}
		}

		// Send the buffered metrics
		return c.SendString(buf.String())
	}
}
