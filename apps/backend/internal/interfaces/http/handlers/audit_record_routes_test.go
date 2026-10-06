package handlers

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// auditRouteRows is one row per actor type: a user's act with its address and
// user agent recorded, an agent's act with neither, an agent's act that also
// names its owner as user_id, and a system act with no actor id.
func auditRouteRows(orgID, agentID uuid.UUID) []*domain.AuditLog {
	user, owner := uuid.New(), uuid.New()
	at := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	return []*domain.AuditLog{
		{ID: uuid.New(), OrganizationID: orgID, UserID: &user, Action: domain.AuditActionUpdate,
			ResourceType: "agent", ResourceID: agentID, IPAddress: "192.0.2.10", UserAgent: "Mozilla/5.0", Timestamp: at},
		{ID: uuid.New(), OrganizationID: orgID, AgentID: &agentID, Action: domain.AuditActionHoneytokenTriggered,
			ResourceType: "agent", ResourceID: agentID, Timestamp: at},
		{ID: uuid.New(), OrganizationID: orgID, UserID: &owner, AgentID: &agentID, Action: domain.AuditActionVerify,
			ResourceType: "agent", ResourceID: agentID, Timestamp: at},
		{ID: uuid.New(), OrganizationID: orgID, Action: "capability_granted",
			ResourceType: "agent", ResourceID: agentID, Timestamp: at},
	}
}

var auditRouteActorTypes = []string{"user", "agent", "agent", "system"}

func auditRouteMock(rows []*domain.AuditLog) *MockAuditServiceImpl {
	byID := map[uuid.UUID]*domain.AuditLog{}
	for _, r := range rows {
		byID[r.ID] = r
	}
	return &MockAuditServiceImpl{
		GetAuditLogsFunc: func(context.Context, uuid.UUID, string, string, *uuid.UUID, *uuid.UUID, *time.Time, *time.Time, int, int) ([]*domain.AuditLog, int, error) {
			return rows, len(rows), nil
		},
		GetAgentActivityFunc: func(context.Context, uuid.UUID, uuid.UUID, int, int) ([]*domain.AuditLog, error) {
			return rows, nil
		},
		GetByIDFunc: func(_ context.Context, id uuid.UUID) (*domain.AuditLog, error) {
			return byID[id], nil
		},
	}
}

// auditRouteApp serves the routes as a caller of the given role. The empty
// role stands for a principal that authenticates without one.
func auditRouteApp(orgID uuid.UUID, role string, register func(app *fiber.App, scoped func(fiber.Handler) fiber.Handler)) *fiber.App {
	app := fiber.New()
	register(app, func(next fiber.Handler) fiber.Handler {
		return func(c fiber.Ctx) error {
			c.Locals("organization_id", orgID)
			c.Locals("user_id", uuid.New())
			if role != "" {
				c.Locals("role", role)
			}
			return next(c)
		}
	})
	return app
}

func auditRouteGet(t *testing.T, app *fiber.App, path string) (int, []byte) {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest("GET", path, nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, body
}

// assertAuditMembers checks one returned record per row: its actorType, and
// ipAddress and userAgent present only on the row that recorded them.
func assertAuditMembers(t *testing.T, route string, records []map[string]any) {
	t.Helper()
	require.Len(t, records, len(auditRouteActorTypes), route)
	for i, rec := range records {
		assert.Equal(t, auditRouteActorTypes[i], rec["actorType"], "%s record %d actorType", route, i)
		_, hasIP := rec["ipAddress"]
		_, hasUA := rec["userAgent"]
		if i == 0 {
			assert.Equal(t, "192.0.2.10", rec["ipAddress"], route)
			assert.Equal(t, "Mozilla/5.0", rec["userAgent"], route)
			continue
		}
		assert.False(t, hasIP, "%s record %d carries ipAddress %v it never recorded", route, i, rec["ipAddress"])
		assert.False(t, hasUA, "%s record %d carries userAgent %v it never recorded", route, i, rec["userAgent"])
	}
}

func TestAuditRecordRoutesCarryActorType(t *testing.T) {
	orgID, agentID := uuid.New(), uuid.New()
	rows := auditRouteRows(orgID, agentID)
	audit := auditRouteMock(rows)

	admin := NewAdminHandlerWithInterfaces(nil, nil, nil, nil, audit, nil, nil, nil, nil)
	agents := NewAgentHandlerWithInterfaces(&MockAgentServiceImpl{
		GetAgentFunc: func(_ context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{ID: id, OrganizationID: orgID, Name: "a"}, nil
		},
	}, nil, audit, nil, nil, nil, nil, nil, nil, nil, nil)

	// The admin routes admit only admins, and only an admin reads the request
	// detail of the agent routes, so every read here is an admin's.
	app := auditRouteApp(orgID, string(domain.RoleAdmin), func(app *fiber.App, scoped func(fiber.Handler) fiber.Handler) {
		app.Get("/admin/audit-logs", scoped(admin.GetAuditLogs))
		app.Get("/admin/audit-logs/export", scoped(admin.ExportAuditLogs))
		app.Get("/admin/audit-logs/:id", scoped(admin.GetAuditLogByID))
		app.Get("/agents/:id/activity", scoped(agents.GetAgentActivity))
		app.Get("/agents/:id/audit-logs", scoped(agents.GetAgentAuditLogs))
	})

	t.Run("GET /admin/audit-logs", func(t *testing.T) {
		status, body := auditRouteGet(t, app, "/admin/audit-logs")
		require.Equal(t, fiber.StatusOK, status, string(body))
		var got struct{ Logs []map[string]any }
		require.NoError(t, json.Unmarshal(body, &got))
		assertAuditMembers(t, "list", got.Logs)
	})

	t.Run("GET /admin/audit-logs/:id", func(t *testing.T) {
		got := make([]map[string]any, 0, len(rows))
		for _, r := range rows {
			status, body := auditRouteGet(t, app, "/admin/audit-logs/"+r.ID.String())
			require.Equal(t, fiber.StatusOK, status, string(body))
			var rec map[string]any
			require.NoError(t, json.Unmarshal(body, &rec))
			got = append(got, rec)
		}
		assertAuditMembers(t, "by id", got)
	})

	t.Run("GET /admin/audit-logs/export?format=json", func(t *testing.T) {
		status, body := auditRouteGet(t, app, "/admin/audit-logs/export?format=json")
		require.Equal(t, fiber.StatusOK, status, string(body))
		var got []map[string]any
		require.NoError(t, json.Unmarshal(body, &got))
		assertAuditMembers(t, "json export", got)
	})

	t.Run("GET /admin/audit-logs/export as CSV", func(t *testing.T) {
		status, body := auditRouteGet(t, app, "/admin/audit-logs/export")
		require.Equal(t, fiber.StatusOK, status, string(body))
		records, err := csv.NewReader(strings.NewReader(string(body))).ReadAll()
		require.NoError(t, err)
		require.Len(t, records, len(rows)+1)
		header := records[0]
		col := map[string]int{}
		for i, name := range header {
			col[name] = i
		}
		require.Contains(t, col, "ActorType", "header %v", header)
		for i, rec := range records[1:] {
			assert.Equal(t, auditRouteActorTypes[i], rec[col["ActorType"]], "csv row %d", i)
			if rows[i].UserID == nil {
				assert.Empty(t, rec[col["UserID"]], "csv row %d has no user id", i)
			} else {
				assert.Equal(t, rows[i].UserID.String(), rec[col["UserID"]], "csv row %d", i)
			}
		}
	})

	t.Run("GET /agents/:id/activity", func(t *testing.T) {
		status, body := auditRouteGet(t, app, "/agents/"+agentID.String()+"/activity")
		require.Equal(t, fiber.StatusOK, status, string(body))
		var got struct{ Activities []map[string]any }
		require.NoError(t, json.Unmarshal(body, &got))
		assertAuditMembers(t, "agent activity", got.Activities)
	})

	t.Run("GET /agents/:id/audit-logs", func(t *testing.T) {
		status, body := auditRouteGet(t, app, "/agents/"+agentID.String()+"/audit-logs")
		require.Equal(t, fiber.StatusOK, status, string(body))
		var got struct{ Logs []map[string]any }
		require.NoError(t, json.Unmarshal(body, &got))
		assertAuditMembers(t, "agent audit logs", got.Logs)
	})
}

// The agent routes are open to every principal in the organization, but only an
// admin reads a record's address, user agent and full metadata. Every other
// caller still gets each record's actorType, and none of those members.
func TestAuditRecordAgentRoutesCarryActorTypeButNoRequestDetailForNonAdmins(t *testing.T) {
	orgID, agentID := uuid.New(), uuid.New()
	rows := auditRouteRows(orgID, agentID)
	rows[0].Metadata = map[string]interface{}{"note": canaryMetadata}
	audit := auditRouteMock(rows)
	agents := NewAgentHandlerWithInterfaces(&MockAgentServiceImpl{
		GetAgentFunc: func(_ context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{ID: id, OrganizationID: orgID, Name: "a"}, nil
		},
	}, nil, audit, nil, nil, nil, nil, nil, nil, nil, nil)

	for _, role := range nonAdminRoles {
		app := auditRouteApp(orgID, role, func(app *fiber.App, scoped func(fiber.Handler) fiber.Handler) {
			app.Get("/agents/:id/activity", scoped(agents.GetAgentActivity))
			app.Get("/agents/:id/audit-logs", scoped(agents.GetAgentAuditLogs))
		})
		for _, route := range []struct{ path, list string }{
			{"/agents/" + agentID.String() + "/activity", "activities"},
			{"/agents/" + agentID.String() + "/audit-logs", "logs"},
		} {
			t.Run("role="+role+" "+route.path, func(t *testing.T) {
				status, body := auditRouteGet(t, app, route.path)
				require.Equal(t, fiber.StatusOK, status, string(body))
				for _, recorded := range []string{"192.0.2.10", "Mozilla/5.0", canaryMetadata} {
					assert.NotContains(t, string(body), recorded)
				}
				var got map[string]json.RawMessage
				require.NoError(t, json.Unmarshal(body, &got))
				var records []map[string]any
				require.NoError(t, json.Unmarshal(got[route.list], &records))
				require.Len(t, records, len(auditRouteActorTypes))
				for i, rec := range records {
					assert.Equal(t, auditRouteActorTypes[i], rec["actorType"], "record %d actorType", i)
					for _, member := range []string{"ipAddress", "userAgent", "metadata"} {
						assert.NotContains(t, rec, member, "record %d member %q must be absent", i, member)
					}
				}
			})
		}
	}
}
