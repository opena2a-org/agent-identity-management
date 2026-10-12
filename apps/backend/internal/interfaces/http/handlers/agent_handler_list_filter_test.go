package handlers

// GET /api/v1/agents by the query that reaches the database: the handler, the
// agent service and the agent repository are the server's own, over a sqlmock
// connection that answers only the statement each case expects.

import (
	"database/sql/driver"
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
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
)

// agentListColumns are the columns the agent list selects, in order.
var agentListColumns = []string{
	"id", "organization_id", "name", "display_name", "description", "agent_type", "status", "version", "public_key",
	"certificate_url", "repository_url", "documentation_url", "trust_score", "verified_at",
	"talks_to", "metadata", "created_at", "updated_at", "created_by",
	"created_by_name", "created_by_email", "capability_violation_count", "is_compromised", "declared_purpose",
}

// agentListRow is one row of the agent list for an agent of the given status
// and type.
func agentListRow(rows *sqlmock.Rows, orgID uuid.UUID, name, agentType, status string) *sqlmock.Rows {
	now := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	return rows.AddRow(
		uuid.New().String(), orgID.String(), name, name, nil, agentType, status, nil, nil,
		nil, nil, nil, 0.5, nil,
		[]byte("[]"), []byte("{}"), now, now, uuid.New().String(),
		"", "", 0, false, nil,
	)
}

// listAgentsOverSQL sends one GET to the list handler, wired to the real
// service and repository over mock's connection, and returns the status and
// the decoded answer.
func listAgentsOverSQL(t *testing.T, orgID uuid.UUID, target string, expect func(mock sqlmock.Sqlmock)) (int, map[string]interface{}) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	expect(mock)

	agentService := application.NewAgentService(
		repository.NewAgentRepository(db),
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
	handler := NewAgentHandlerWithInterfaces(
		agentService,
		&MockMCPServiceImpl{},
		&MockAuditServiceImpl{},
		&MockAPIKeyServiceImpl{},
		nil,
		&MockAlertServiceImpl{},
		&MockVerificationEventServiceImpl{},
		&MockCapabilityServiceImpl{},
		&MockTagServiceImpl{},
		&MockOrganizationRepository{},
		&MockMCPAttestationServiceImpl{},
	)
	app := createTestAppWithAuth(handler, orgID, uuid.New())
	app.Get("/agents", handler.ListAgents)

	resp, err := app.Test(httptest.NewRequest("GET", target, nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &body), "the answer is JSON: %s", raw)

	require.NoError(t, mock.ExpectationsWereMet(), "every expected statement reached the database")
	return resp.StatusCode, body
}

// agentListSQL matches the list statement whose WHERE clause is exactly where.
func agentListSQL(where string) string {
	return `FROM agents\s+WHERE ` + regexp.QuoteMeta(where) + `\s+ORDER BY created_at DESC, id`
}

// agentCountSQL matches the count statement whose WHERE clause is exactly
// where.
func agentCountSQL(where string) string {
	return `^SELECT COUNT\(\*\) FROM agents WHERE ` + regexp.QuoteMeta(where) + `$`
}

// The request of the report: a status and an agent type no agent holds, with
// a limit. Both are predicates of the list statement and of its count, so the
// answer is 200 with no agent and a total of 0. The handler read only limit
// and offset before, so the same request listed every agent of the
// organization.
func TestListAgents_StatusAndAgentTypeNoAgentHoldsMatchNoAgent(t *testing.T) {
	orgID := uuid.New()
	const where = "organization_id = $1 AND status = $2 AND agent_type = $3"

	status, body := listAgentsOverSQL(t, orgID,
		"/agents?limit=100&offset=0&status=no-such-status&agent_type=no-such-type",
		func(mock sqlmock.Sqlmock) {
			mock.ExpectQuery(agentListSQL(where)+` LIMIT \$4 OFFSET \$5$`).
				WithArgs(orgID, "no-such-status", "no-such-type", 100, 0).
				WillReturnRows(sqlmock.NewRows(agentListColumns))
			mock.ExpectQuery(agentCountSQL(where)).
				WithArgs(orgID, "no-such-status", "no-such-type").
				WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
		})

	assert.Equal(t, fiber.StatusOK, status)
	assert.Empty(t, body["agents"], "no agent holds the status and the type")
	assert.Equal(t, float64(0), body["total"])
	assert.Equal(t, float64(100), body["limit"])
	assert.Equal(t, float64(0), body["offset"])
}

// Each filter is one equality predicate with its value as a parameter. The
// agent type is read from agentType and, when that is absent or empty, from
// agent_type; a parameter that is absent or empty is no predicate.
func TestListAgents_FilterParametersBecomePredicates(t *testing.T) {
	for _, tc := range []struct {
		name  string
		query string
		where string
		args  []string
	}{
		{name: "status", query: "?status=verified",
			where: "organization_id = $1 AND status = $2", args: []string{"verified"}},
		{name: "agentType", query: "?agentType=langchain",
			where: "organization_id = $1 AND agent_type = $2", args: []string{"langchain"}},
		{name: "agent_type", query: "?agent_type=langchain",
			where: "organization_id = $1 AND agent_type = $2", args: []string{"langchain"}},
		{name: "agentType comes before agent_type", query: "?agentType=langchain&agent_type=crewai",
			where: "organization_id = $1 AND agent_type = $2", args: []string{"langchain"}},
		{name: "an empty agentType leaves agent_type", query: "?agentType=&agent_type=crewai",
			where: "organization_id = $1 AND agent_type = $2", args: []string{"crewai"}},
		{name: "status and agentType", query: "?status=suspended&agentType=ai_agent",
			where: "organization_id = $1 AND status = $2 AND agent_type = $3", args: []string{"suspended", "ai_agent"}},
		{name: "a value with characters of SQL is a parameter", query: "?status=verified%27%20OR%20%271%27%3D%271",
			where: "organization_id = $1 AND status = $2", args: []string{"verified' OR '1'='1"}},
		{name: "no filter", query: "",
			where: "organization_id = $1"},
		{name: "empty filters", query: "?status=&agentType=&agent_type=",
			where: "organization_id = $1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			orgID := uuid.New()
			args := []driver.Value{orgID}
			for _, a := range tc.args {
				args = append(args, a)
			}

			status, body := listAgentsOverSQL(t, orgID, "/agents"+tc.query, func(mock sqlmock.Sqlmock) {
				rows := agentListRow(sqlmock.NewRows(agentListColumns), orgID, "matching-agent", "langchain", "verified")
				// No limit, so no LIMIT clause and no count statement: the
				// total is the number of rows.
				mock.ExpectQuery(agentListSQL(tc.where) + `\s*$`).
					WithArgs(args...).
					WillReturnRows(rows)
			})

			assert.Equal(t, fiber.StatusOK, status)
			agents, ok := body["agents"].([]interface{})
			require.True(t, ok, "agents is a list")
			require.Len(t, agents, 1)
			assert.Equal(t, "matching-agent", agents[0].(map[string]interface{})["name"])
			assert.Equal(t, float64(1), body["total"])
			_, hasLimit := body["limit"]
			assert.False(t, hasLimit, "with no limit the answer names none")
		})
	}
}

// With a limit, the total of a filtered list counts the agents that pass the
// filter, not the organization's agents and not the rows of the page.
func TestListAgents_FilteredTotalCountsTheAgentsThatPass(t *testing.T) {
	orgID := uuid.New()
	const where = "organization_id = $1 AND status = $2"

	status, body := listAgentsOverSQL(t, orgID, "/agents?limit=2&offset=4&status=verified", func(mock sqlmock.Sqlmock) {
		rows := sqlmock.NewRows(agentListColumns)
		rows = agentListRow(rows, orgID, "fifth-verified", "langchain", "verified")
		rows = agentListRow(rows, orgID, "sixth-verified", "crewai", "verified")
		mock.ExpectQuery(agentListSQL(where)+` LIMIT \$3 OFFSET \$4$`).
			WithArgs(orgID, "verified", 2, 4).
			WillReturnRows(rows)
		mock.ExpectQuery(agentCountSQL(where)).
			WithArgs(orgID, "verified").
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(7))
	})

	assert.Equal(t, fiber.StatusOK, status)
	assert.Len(t, body["agents"], 2)
	assert.Equal(t, float64(7), body["total"])
	assert.Equal(t, float64(2), body["limit"])
	assert.Equal(t, float64(4), body["offset"])
}
