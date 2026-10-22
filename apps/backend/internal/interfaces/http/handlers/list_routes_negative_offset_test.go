package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/middleware"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/testutil/mocks"
)

// GET /api/v1/agents/:id/alerts, GET /api/v1/agents/:id/violations,
// GET /api/v1/admin/alerts and GET /api/v1/a2a/security/violations page with
// a query that ends in LIMIT and OFFSET, and read a negative offset as 0.
// Each route runs here on its service and its repository, over a database
// that answers the page query only when it is sent the expected limit and
// offset, so a negative offset that reaches the query fails the request.
// list_routes_negative_limit_test.go runs the same routes with a negative
// limit.

// listOffsetRoute is one of the four routes, served over a database whose page
// query expects limit wantLimit and offset wantOffset.
type listOffsetRoute struct {
	name string
	// defaultLimit is the page size the route reads when the request names no
	// limit.
	defaultLimit int
	// serve sets the queries the route sends on the database and returns the
	// app, the request target without its query string and the ID of the one
	// record on the page.
	serve func(t *testing.T, store sqlmock.Sqlmock, db *sql.DB, wantLimit, wantOffset int) (*fiber.App, string, string)
	// page reads the IDs of the records served and the offset the body
	// echoes, nil when the route does not echo one, and checks that a route
	// that echoes its limit echoes wantLimit.
	page func(t *testing.T, body []byte, wantLimit int) ([]string, *int)
}

var listOffsetCreatedAt = time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)

var listOffsetAlertColumns = []string{
	"id", "organization_id", "alert_type", "severity", "title", "description", "resource_type", "resource_id",
	"audit_id", "agent_name", "source_ip", "metadata", "is_acknowledged", "acknowledged_by", "acknowledged_at",
	"created_at", "occurrence_count", "last_seen_at", "dedupe_key",
}

func listOffsetAlertRows(alertID, org, resourceID uuid.UUID) *sqlmock.Rows {
	return sqlmock.NewRows(listOffsetAlertColumns).AddRow(
		alertID.String(), org.String(), "trust_score_drop", "warning", "Trust score dropped", "", "agent",
		resourceID.String(), nil, "billing-agent", nil, []byte("{}"), false, nil, nil,
		listOffsetCreatedAt, int64(1), nil, "",
	)
}

// listOffsetMember marks the request as a member's session of org.
func listOffsetMember(org uuid.UUID) fiber.Handler {
	return func(c fiber.Ctx) error {
		c.Locals("organization_id", org)
		c.Locals("user_id", uuid.New())
		c.Locals("role", string(domain.RoleMember))
		return c.Next()
	}
}

func listOffsetAgent(org uuid.UUID) (*domain.Agent, *mocks.MockAgentRepository) {
	agent := &domain.Agent{ID: uuid.New(), OrganizationID: org, Name: "billing-agent", Status: domain.AgentStatusVerified}
	agentRepo := &mocks.MockAgentRepository{}
	agentRepo.On("GetByID", agent.ID).Return(agent, nil)
	agentRepo.On("GetByID", mock.Anything).Return(nil, fmt.Errorf("agent not found"))
	return agent, agentRepo
}

func listOffsetApp() *fiber.App {
	app := fiber.New()
	app.Use(middleware.RecoveryMiddleware())
	return app
}

// listOffsetAuditRepo accepts the audit rows the admin route writes.
type listOffsetAuditRepo struct{ domain.AuditLogRepository }

func (listOffsetAuditRepo) Create(*domain.AuditLog) error { return nil }

func listOffsetIDs(t *testing.T, items []map[string]json.RawMessage) []string {
	t.Helper()
	ids := make([]string, 0, len(items))
	for _, item := range items {
		var id string
		require.NoError(t, json.Unmarshal(item["id"], &id))
		ids = append(ids, id)
	}
	return ids
}

var listOffsetRoutes = []listOffsetRoute{
	{
		name:         "GET /api/v1/agents/:id/alerts",
		defaultLimit: 50,
		serve: func(t *testing.T, store sqlmock.Sqlmock, db *sql.DB, wantLimit, wantOffset int) (*fiber.App, string, string) {
			org := uuid.New()
			agent, agentRepo := listOffsetAgent(org)
			agents := application.NewAgentService(agentRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
			alerts := application.NewAlertService(repository.NewAlertRepository(db), agentRepo, db)
			h := NewAgentHandlerWithInterfaces(agents, nil, nil, nil, nil, alerts, nil, nil, nil, nil, nil)

			alertID := uuid.New()
			store.ExpectQuery(`FROM alerts\s+WHERE resource_id = \$1\s+ORDER BY created_at DESC\s+LIMIT \$2 OFFSET \$3`).
				WithArgs(agent.ID, wantLimit, wantOffset).
				WillReturnRows(listOffsetAlertRows(alertID, org, agent.ID))

			app := listOffsetApp()
			app.Get("/api/v1/agents/:id/alerts", listOffsetMember(org), h.GetAgentAlerts)
			return app, "/api/v1/agents/" + agent.ID.String() + "/alerts", alertID.String()
		},
		page: func(t *testing.T, body []byte, wantLimit int) ([]string, *int) {
			var got struct {
				Alerts []map[string]json.RawMessage `json:"alerts"`
				Limit  int                          `json:"limit"`
				Offset int                          `json:"offset"`
			}
			require.NoError(t, json.Unmarshal(body, &got))
			assert.Equal(t, wantLimit, got.Limit)
			return listOffsetIDs(t, got.Alerts), &got.Offset
		},
	},
	{
		name:         "GET /api/v1/agents/:id/violations",
		defaultLimit: 20,
		serve: func(t *testing.T, store sqlmock.Sqlmock, db *sql.DB, wantLimit, wantOffset int) (*fiber.App, string, string) {
			org := uuid.New()
			agent, agentRepo := listOffsetAgent(org)
			capabilities := application.NewCapabilityService(
				repository.NewCapabilityRepository(sqlx.NewDb(db, "postgres")), agentRepo, nil, nil, nil, nil)
			h := NewCapabilityHandler(capabilities, nil, agentRepo, nil)

			violationID := uuid.New()
			store.ExpectQuery(`SELECT COUNT\(\*\) FROM capability_violations WHERE agent_id = \$1`).
				WithArgs(agent.ID).
				WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
			store.ExpectQuery(`FROM capability_violations cv\s+LEFT JOIN agents a ON cv.agent_id = a.id\s+WHERE cv.agent_id = \$1\s+ORDER BY cv.created_at DESC\s+LIMIT \$2 OFFSET \$3`).
				WithArgs(agent.ID, wantLimit, wantOffset).
				WillReturnRows(sqlmock.NewRows([]string{
					"id", "agent_id", "agent_name", "attempted_capability", "registered_capabilities", "severity",
					"trust_score_impact", "is_blocked", "source_ip", "request_metadata", "created_at",
				}).AddRow(
					violationID.String(), agent.ID.String(), "billing-agent", "file:write", []byte("{}"), "high",
					int64(-5), true, nil, []byte("{}"), listOffsetCreatedAt,
				))

			app := listOffsetApp()
			app.Get("/api/v1/agents/:id/violations", listOffsetMember(org), h.GetViolationsByAgent)
			return app, "/api/v1/agents/" + agent.ID.String() + "/violations", violationID.String()
		},
		page: func(t *testing.T, body []byte, wantLimit int) ([]string, *int) {
			var got struct {
				Violations []map[string]json.RawMessage `json:"violations"`
				Total      int                          `json:"total"`
				Limit      int                          `json:"limit"`
				Offset     int                          `json:"offset"`
			}
			require.NoError(t, json.Unmarshal(body, &got))
			assert.Equal(t, 1, got.Total)
			assert.Equal(t, wantLimit, got.Limit)
			return listOffsetIDs(t, got.Violations), &got.Offset
		},
	},
	{
		name:         "GET /api/v1/admin/alerts",
		defaultLimit: 100,
		serve: func(t *testing.T, store sqlmock.Sqlmock, db *sql.DB, wantLimit, wantOffset int) (*fiber.App, string, string) {
			org := uuid.New()
			alerts := application.NewAlertService(repository.NewAlertRepository(db), nil, db)
			audit := application.NewAuditService(listOffsetAuditRepo{})
			h := NewAdminHandlerWithInterfaces(nil, nil, nil, nil, audit, alerts, nil, nil, nil)

			alertID := uuid.New()
			store.ExpectQuery(`FROM alerts\s+WHERE organization_id = \$1\s+ORDER BY created_at DESC\s+LIMIT \$2 OFFSET \$3`).
				WithArgs(org, wantLimit, wantOffset).
				WillReturnRows(listOffsetAlertRows(alertID, org, uuid.New()))
			for i := 0; i < 2; i++ { // the filtered count, then the count of every alert
				store.ExpectQuery(`^SELECT COUNT\(\*\) FROM alerts WHERE organization_id = \$1$`).
					WithArgs(org).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
			}
			store.ExpectQuery(`FROM alerts\s+WHERE organization_id = \$1 AND is_acknowledged = false`).
				WithArgs(org).
				WillReturnRows(sqlmock.NewRows(listOffsetAlertColumns))
			store.ExpectQuery(`COUNT\(\*\) FILTER`).
				WithArgs(org).
				WillReturnRows(sqlmock.NewRows([]string{"critical_count", "high_count", "warning_count", "info_count"}).
					AddRow(0, 0, 1, 0))

			app := listOffsetApp()
			app.Get("/api/v1/admin/alerts", listOffsetMember(org), h.GetAlerts)
			return app, "/api/v1/admin/alerts", alertID.String()
		},
		page: func(t *testing.T, body []byte, wantLimit int) ([]string, *int) {
			var got struct {
				Alerts []map[string]json.RawMessage `json:"alerts"`
				Total  int                          `json:"total"`
				Limit  int                          `json:"limit"`
				Offset int                          `json:"offset"`
			}
			require.NoError(t, json.Unmarshal(body, &got))
			assert.Equal(t, 1, got.Total)
			assert.Equal(t, wantLimit, got.Limit)
			return listOffsetIDs(t, got.Alerts), &got.Offset
		},
	},
	{
		name:         "GET /api/v1/a2a/security/violations",
		defaultLimit: 50,
		serve: func(t *testing.T, store sqlmock.Sqlmock, db *sql.DB, wantLimit, wantOffset int) (*fiber.App, string, string) {
			org := uuid.New()
			h := newA2AFailureHandler(t, db, a2aRecordOwned)

			violationID := uuid.New()
			store.ExpectQuery(`FROM a2a_security_violations WHERE organization_id = \$1\s+ORDER BY created_at DESC LIMIT \$2 OFFSET \$3`).
				WithArgs(org, wantLimit, wantOffset).
				WillReturnRows(sqlmock.NewRows([]string{
					"id", "organization_id", "requesting_agent_id", "target_agent_id", "skill_id", "task_id",
					"violation_type", "violation_details", "policy_id", "action_taken", "request_signature",
					"request_nonce", "request_timestamp", "created_at",
				}).AddRow(
					violationID.String(), org.String(), nil, nil, "summarize", nil, "consent_missing",
					[]byte("{}"), nil, "blocked", nil, nil, nil, listOffsetCreatedAt,
				))

			app := listOffsetApp()
			app.Get("/api/v1/a2a/security/violations", listOffsetMember(org), h.GetSecurityViolations)
			return app, "/api/v1/a2a/security/violations", violationID.String()
		},
		page: func(t *testing.T, body []byte, wantLimit int) ([]string, *int) {
			var got []map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(body, &got))
			return listOffsetIDs(t, got), nil
		},
	},
}

// listOffsetGet requests the route with the query string query, over a
// database whose page query expects wantLimit and wantOffset, and checks the
// store saw every query the route was expected to send.
func listOffsetGet(t *testing.T, route listOffsetRoute, query string, wantLimit, wantOffset int) (int, []byte, string) {
	t.Helper()
	db, store, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	app, target, recordID := route.serve(t, store, db, wantLimit, wantOffset)
	resp, err := app.Test(httptest.NewRequest("GET", target+"?"+query, nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.NoError(t, store.ExpectationsWereMet(),
		"the route sends its page query with limit %d and offset %d", wantLimit, wantOffset)
	return resp.StatusCode, body, recordID
}

// A negative offset reaches the page query as 0, and the first page is
// answered 200.
func TestListRoutes_NegativeOffsetReadsAsZero(t *testing.T) {
	for _, route := range listOffsetRoutes {
		for _, offset := range []string{"-1", "-100"} {
			t.Run(route.name+" offset="+offset, func(t *testing.T) {
				status, body, recordID := listOffsetGet(t, route, "limit=2&offset="+offset, 2, 0)
				require.Equal(t, fiber.StatusOK, status, string(body))
				ids, echoed := route.page(t, body, 2)
				assert.Equal(t, []string{recordID}, ids, "the first page is served")
				if echoed != nil {
					assert.Equal(t, 0, *echoed, "the body echoes the offset read")
				}
			})
		}
	}
}

// A non-negative offset reaches the page query as sent.
func TestListRoutes_NonNegativeOffsetIsSentAsIs(t *testing.T) {
	for _, route := range listOffsetRoutes {
		for _, offset := range []int{0, 2} {
			t.Run(fmt.Sprintf("%s offset=%d", route.name, offset), func(t *testing.T) {
				status, body, recordID := listOffsetGet(t, route, fmt.Sprintf("limit=2&offset=%d", offset), 2, offset)
				require.Equal(t, fiber.StatusOK, status, string(body))
				ids, echoed := route.page(t, body, 2)
				assert.Equal(t, []string{recordID}, ids)
				if echoed != nil {
					assert.Equal(t, offset, *echoed)
				}
			})
		}
	}
}
