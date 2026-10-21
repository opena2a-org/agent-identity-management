package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/middleware"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/testutil/mocks"
)

// GET /api/v1/agents/:id/audit-logs pages an agent's records through the
// audit service, and reads a negative offset as 0. The route runs here behind
// the recovery middleware the server mounts, so a panic is answered 500 as it
// is by the server.

// agentRecordsAuditRepo holds an agent's audit records and accepts the rows
// the route writes.
type agentRecordsAuditRepo struct {
	domain.AuditLogRepository
	records []*domain.AuditLog
}

func newAgentRecordsAuditRepo(org, agentID uuid.UUID, records int) *agentRecordsAuditRepo {
	r := &agentRecordsAuditRepo{}
	for i := 0; i < records; i++ {
		r.records = append(r.records, &domain.AuditLog{
			ID:             uuid.New(),
			OrganizationID: org,
			Action:         domain.AuditActionUpdate,
			ResourceType:   "agent",
			ResourceID:     agentID,
		})
	}
	return r
}

func (r *agentRecordsAuditRepo) GetByResource(orgID uuid.UUID, resourceType string, resourceID uuid.UUID) ([]*domain.AuditLog, error) {
	var out []*domain.AuditLog
	for _, rec := range r.records {
		if rec.OrganizationID == orgID && rec.ResourceType == resourceType && rec.ResourceID == resourceID {
			out = append(out, rec)
		}
	}
	return out, nil
}

func (r *agentRecordsAuditRepo) Create(*domain.AuditLog) error { return nil }

// memberAgentAuditLogsApp serves the route to a member's session, with
// records audit records for the agent it returns.
func memberAgentAuditLogsApp(t *testing.T, records int) (*fiber.App, uuid.UUID) {
	t.Helper()
	org := uuid.New()
	agent := &domain.Agent{ID: uuid.New(), OrganizationID: org, Name: "billing-agent", Status: domain.AgentStatusVerified}

	agentRepo := &mocks.MockAgentRepository{}
	agentRepo.On("GetByID", agent.ID).Return(agent, nil)
	agentRepo.On("GetByID", mock.Anything).Return(nil, errors.New("agent not found"))
	agents := application.NewAgentService(agentRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	audit := application.NewAuditService(newAgentRecordsAuditRepo(org, agent.ID, records))
	h := NewAgentHandlerWithInterfaces(agents, nil, audit, nil, nil, nil, nil, nil, nil, nil, nil)

	app := fiber.New()
	app.Use(middleware.RecoveryMiddleware())
	app.Get(agentAuditLogsPattern, func(c fiber.Ctx) error {
		c.Locals("organization_id", org)
		c.Locals("user_id", uuid.New())
		c.Locals("role", string(domain.RoleMember))
		return c.Next()
	}, h.GetAgentAuditLogs)
	return app, agent.ID
}

// serveCallerOwnRecords replaces the fixture's app with one whose audit
// service pages records audit records of the calling agent, behind the
// authenticators the agents group mounts.
func (f *agentAuditLogsFixture) serveCallerOwnRecords(records int) {
	agents := application.NewAgentService(f.repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	audit := application.NewAuditService(newAgentRecordsAuditRepo(f.org, f.caller.ID, records))
	h := NewAgentHandlerWithInterfaces(agents, nil, audit, nil, nil, nil, nil, nil, nil, nil, nil)

	f.app = fiber.New()
	f.app.Use(middleware.RecoveryMiddleware())
	group := f.app.Group("/api/v1/agents")
	group.Use(middleware.PQCAgentMiddleware(agents))
	group.Use(middleware.ServicePrincipalMiddleware(f.jwt, f.repo))
	group.Use(middleware.AuthMiddleware(f.jwt))
	group.Get("/:id/audit-logs", middleware.AgentPathObservation("GET "+agentAuditLogsPattern, "id"), h.GetAgentAuditLogs)
}

type agentAuditLogsPage struct {
	Logs   []map[string]json.RawMessage `json:"logs"`
	Total  int                          `json:"total"`
	Limit  int                          `json:"limit"`
	Offset int                          `json:"offset"`
}

func assertFirstPage(t *testing.T, status int, body string, records int, msg string) {
	t.Helper()
	require.Equal(t, fiber.StatusOK, status, "%s: %s", msg, body)
	var got agentAuditLogsPage
	require.NoError(t, json.Unmarshal([]byte(body), &got))
	assert.Equal(t, 0, got.Offset, msg)
	assert.Equal(t, 2, got.Limit, msg)
	assert.Equal(t, records, got.Total, msg)
	assert.Len(t, got.Logs, min(records, 2), "%s: the first page is served", msg)
}

// A member's session with a negative offset gets the first page.
func TestAgentAuditLogs_NegativeOffsetReadsAsZero(t *testing.T) {
	for _, records := range []int{0, 3} {
		for _, offset := range []string{"-1", "-100"} {
			app, agentID := memberAgentAuditLogsApp(t, records)
			req := httptest.NewRequest("GET", agentAuditLogsTarget(agentID)+"?limit=2&offset="+offset, nil)
			status, body := agentAuditLogsDo(t, app, req)
			assertFirstPage(t, status, body, records, fmt.Sprintf("%d records, offset %s", records, offset))
		}
	}
}

// The agent's own signed request and its service token with a negative
// offset get the first page of the agent's audit log.
func TestAgentAuditLogs_AnAgentsNegativeOffsetReadsAsZero(t *testing.T) {
	for _, cred := range agentAuditLogsCredentials {
		t.Run(cred.name, func(t *testing.T) {
			for _, records := range []int{0, 3} {
				for _, offset := range []string{"-1", "-100"} {
					f := newAgentAuditLogsFixture(t)
					f.serveCallerOwnRecords(records)
					status, body := cred.send(f, t, agentAuditLogsTarget(f.caller.ID)+"?limit=2&offset="+offset)
					assertFirstPage(t, status, body, records, fmt.Sprintf("%d records, offset %s", records, offset))
				}
			}
		})
	}
}

// A non-negative offset is served as before.
func TestAgentAuditLogs_OffsetPagesTheRecords(t *testing.T) {
	app, agentID := memberAgentAuditLogsApp(t, 3)
	req := httptest.NewRequest("GET", agentAuditLogsTarget(agentID)+"?limit=2&offset=2", nil)
	status, body := agentAuditLogsDo(t, app, req)
	require.Equal(t, fiber.StatusOK, status, body)

	var got agentAuditLogsPage
	require.NoError(t, json.Unmarshal([]byte(body), &got))
	assert.Equal(t, 2, got.Offset)
	assert.Equal(t, 3, got.Total)
	assert.Len(t, got.Logs, 1)
}
