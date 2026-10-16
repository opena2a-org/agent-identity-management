package handlers

// GET /api/v1/detection/agents/:id/capabilities/latest by the query that
// reaches the database: the handler and the detection service are the
// server's own, over a sqlmock connection that answers only the statements
// each case expects.

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
)

var (
	latestReportAgentCount = regexp.QuoteMeta(`SELECT COUNT(*) FROM agents`)
	latestReportSelect     = regexp.QuoteMeta(`FROM agent_capability_reports`)
)

// getLatestReport asks for agentID's latest capability report as a member of
// orgID, with mock holding the statements the case expects.
func getLatestReport(t *testing.T, orgID, agentID uuid.UUID, expect func(mock sqlmock.Sqlmock)) (int, []byte) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	expect(mock)

	handler := NewDetectionHandler(application.NewDetectionService(db, nil, nil), nil)
	app := fiber.New()
	app.Get("/api/v1/detection/agents/:id/capabilities/latest", func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		return handler.GetLatestCapabilityReport(c)
	})

	resp, err := app.Test(httptest.NewRequest("GET", "/api/v1/detection/agents/"+agentID.String()+"/capabilities/latest", nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
	return resp.StatusCode, body
}

// An agent of the caller's organization that has sent no capability report
// yet is answered 204 with no body: the dashboard reads every agent's latest
// report when its page opens, and a new agent has none.
func TestGetLatestCapabilityReport_AgentWithoutReportIs204(t *testing.T) {
	orgID, agentID := uuid.New(), uuid.New()
	status, body := getLatestReport(t, orgID, agentID, func(mock sqlmock.Sqlmock) {
		mock.ExpectQuery(latestReportAgentCount).WithArgs(agentID, orgID).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
		mock.ExpectQuery(latestReportSelect).WithArgs(agentID).
			WillReturnRows(sqlmock.NewRows([]string{"detected_at", "environment", "ai_models", "capabilities", "risk_assessment"}))
	})
	assert.Equal(t, fiber.StatusNoContent, status, "body: %s", body)
	assert.Empty(t, body)
}

// An agent the caller's organization does not hold stays 404, whether it does
// not exist or belongs to another organization: the count is scoped to the
// caller's organization, and no report is read.
func TestGetLatestCapabilityReport_AgentOutsideTheOrganizationIs404(t *testing.T) {
	orgID, agentID := uuid.New(), uuid.New()
	status, body := getLatestReport(t, orgID, agentID, func(mock sqlmock.Sqlmock) {
		mock.ExpectQuery(latestReportAgentCount).WithArgs(agentID, orgID).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	})
	assert.Equal(t, fiber.StatusNotFound, status)
	assert.JSONEq(t, `{"error":"agent not found"}`, string(body))
}

// An agent with a report is answered 200 with the latest one.
func TestGetLatestCapabilityReport_AgentWithReportIs200(t *testing.T) {
	orgID, agentID := uuid.New(), uuid.New()
	detectedAt := time.Date(2026, 10, 15, 8, 30, 0, 0, time.UTC)
	status, body := getLatestReport(t, orgID, agentID, func(mock sqlmock.Sqlmock) {
		mock.ExpectQuery(latestReportAgentCount).WithArgs(agentID, orgID).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
		mock.ExpectQuery(latestReportSelect).WithArgs(agentID).
			WillReturnRows(sqlmock.NewRows([]string{"detected_at", "environment", "ai_models", "capabilities", "risk_assessment"}).
				AddRow(detectedAt, []byte(`{"language":"python","version":"3.12"}`), []byte(`[]`), []byte(`{}`),
					[]byte(`{"overallRiskScore":20,"riskLevel":"LOW","trustScoreImpact":0,"alerts":[]}`)))
	})
	require.Equal(t, fiber.StatusOK, status, "body: %s", body)
	var report struct {
		DetectedAt     string                     `json:"detectedAt"`
		Environment    struct{ Language string }  `json:"environment"`
		RiskAssessment struct{ RiskLevel string } `json:"riskAssessment"`
	}
	require.NoError(t, json.Unmarshal(body, &report))
	assert.Equal(t, "2026-10-15T08:30:00Z", report.DetectedAt)
	assert.Equal(t, "python", report.Environment.Language)
	assert.Equal(t, "LOW", report.RiskAssessment.RiskLevel)
}
