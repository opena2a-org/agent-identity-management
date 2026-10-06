package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// The per-agent and per-MCP-server audit routes are open to every principal
// in the organization, while the admin audit routes that return the same
// records admit only admins. A principal the admin routes refuse must not read
// the network address, user agent or metadata a colleague's request left in
// the record. The canaries below are written by another user; none of them may
// reach a non-admin, and every one of them must still reach an admin.
const (
	canaryIP       = "203.0.113.77"
	canaryUA       = "CanaryBrowser/9.9 (colleague-workstation)"
	canaryEmail    = "canary.colleague@example.test"
	canaryMetadata = "CANARY_METADATA_VALUE"
)

var auditCanaries = []string{canaryIP, canaryUA, canaryEmail, canaryMetadata}

func canaryAuditLog(orgID, resourceID uuid.UUID, resourceType string) *domain.AuditLog {
	writer := uuid.New()
	return &domain.AuditLog{
		ID:             uuid.New(),
		OrganizationID: orgID,
		UserID:         &writer,
		UserName:       "Colleague Admin",
		Action:         domain.AuditActionUpdate,
		ResourceType:   resourceType,
		ResourceID:     resourceID,
		IPAddress:      canaryIP,
		UserAgent:      canaryUA,
		Metadata: map[string]interface{}{
			"userEmail": canaryEmail,
			"note":      canaryMetadata,
		},
		Timestamp: time.Now(),
	}
}

// Roles the admin audit routes refuse. The empty role stands for a
// principal that authenticates without one (an API key or an agent).
var nonAdminRoles = []string{
	string(domain.RoleManager),
	string(domain.RoleMember),
	string(domain.RoleViewer),
	"",
}

func serveAuditRoute(t *testing.T, path, pattern string, role string, orgID uuid.UUID, h fiber.Handler) []byte {
	t.Helper()
	app := fiber.New()
	app.Get(pattern, func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", uuid.New())
		if role != "" {
			c.Locals("role", role)
		}
		return h(c)
	})
	resp, err := app.Test(httptest.NewRequest("GET", path, nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, fiber.StatusOK, resp.StatusCode, "body: %s", raw)
	return raw
}

// decodeLogs returns each record of the response's "logs" array as a member map.
func decodeLogs(t *testing.T, raw []byte) []map[string]json.RawMessage {
	t.Helper()
	var body struct {
		Logs []map[string]json.RawMessage `json:"logs"`
	}
	require.NoError(t, json.Unmarshal(raw, &body))
	return body.Logs
}

func newAgentAuditHandler(orgID, agentID uuid.UUID) *AgentHandler {
	agentSvc := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{ID: agentID, OrganizationID: orgID, Name: "billing-agent"}, nil
		},
	}
	auditSvc := &MockAuditServiceImpl{
		GetAuditLogsFunc: func(ctx context.Context, gotOrgID uuid.UUID, action string, entityType string, entityID *uuid.UUID, userID *uuid.UUID, startDate *time.Time, endDate *time.Time, limit int, offset int) ([]*domain.AuditLog, int, error) {
			return []*domain.AuditLog{canaryAuditLog(orgID, agentID, "agent")}, 1, nil
		},
	}
	return NewAgentHandlerWithInterfaces(agentSvc, nil, auditSvc, nil, nil, nil, nil, nil, nil, nil, nil)
}

func newMCPAuditHandler(orgID, serverID uuid.UUID) *MCPHandler {
	mcpSvc := &MockMCPServiceExtendedImpl{
		GetMCPServerFunc: func(ctx context.Context, id uuid.UUID) (*domain.MCPServer, error) {
			return &domain.MCPServer{ID: id, OrganizationID: orgID, Name: "files-server"}, nil
		},
	}
	auditSvc := &MockAuditServiceImpl{
		GetAuditLogsFunc: func(ctx context.Context, gotOrgID uuid.UUID, action string, entityType string, entityID *uuid.UUID, userID *uuid.UUID, startDate *time.Time, endDate *time.Time, limit int, offset int) ([]*domain.AuditLog, int, error) {
			return []*domain.AuditLog{canaryAuditLog(orgID, serverID, "mcp_server")}, 1, nil
		},
	}
	return createMCPHandlerWithMocks(mcpSvc, &MockMCPCapabilityServiceImpl{}, auditSvc, nil, nil, nil, &MockMCPAttestationServiceExtendedImpl{})
}

func TestAgentAuditLogs_NonAdminGetsNoNetworkAddressUserAgentOrMetadata(t *testing.T) {
	orgID, agentID := uuid.New(), uuid.New()
	h := newAgentAuditHandler(orgID, agentID)

	for _, role := range nonAdminRoles {
		t.Run("role="+role, func(t *testing.T) {
			raw := serveAuditRoute(t, "/agents/"+agentID.String()+"/audit-logs", "/agents/:id/audit-logs", role, orgID, h.GetAgentAuditLogs)
			for _, canary := range auditCanaries {
				assert.NotContains(t, string(raw), canary)
			}
			logs := decodeLogs(t, raw)
			require.Len(t, logs, 1)
			for _, member := range []string{"ipAddress", "userAgent", "metadata"} {
				assert.NotContains(t, logs[0], member, "member %q must be absent, not empty", member)
			}
			assert.JSONEq(t, `"Colleague Admin"`, string(logs[0]["userName"]))
			assert.JSONEq(t, `"update"`, string(logs[0]["action"]))
		})
	}
}

func TestAgentAuditLogs_AdminResponseUnchanged(t *testing.T) {
	orgID, agentID := uuid.New(), uuid.New()
	h := newAgentAuditHandler(orgID, agentID)

	raw := serveAuditRoute(t, "/agents/"+agentID.String()+"/audit-logs", "/agents/:id/audit-logs", string(domain.RoleAdmin), orgID, h.GetAgentAuditLogs)
	for _, canary := range auditCanaries {
		assert.Contains(t, string(raw), canary)
	}
	logs := decodeLogs(t, raw)
	require.Len(t, logs, 1)
	assert.JSONEq(t, `"`+canaryIP+`"`, string(logs[0]["ipAddress"]))
	assert.JSONEq(t, `"`+canaryUA+`"`, string(logs[0]["userAgent"]))
}

func TestMCPServerAuditLogs_NonAdminGetsNoNetworkAddressOrMetadata(t *testing.T) {
	orgID, serverID := uuid.New(), uuid.New()
	h := newMCPAuditHandler(orgID, serverID)

	for _, role := range nonAdminRoles {
		t.Run("role="+role, func(t *testing.T) {
			raw := serveAuditRoute(t, "/mcp-servers/"+serverID.String()+"/audit-logs", "/mcp-servers/:id/audit-logs", role, orgID, h.GetMCPServerAuditLogs)
			for _, canary := range auditCanaries {
				assert.NotContains(t, string(raw), canary)
			}
			logs := decodeLogs(t, raw)
			require.Len(t, logs, 1)
			for _, member := range []string{"ipAddress", "userAgent", "metadata"} {
				assert.NotContains(t, logs[0], member, "member %q must be absent, not empty", member)
			}
			assert.JSONEq(t, `"Colleague Admin"`, string(logs[0]["actorName"]))
		})
	}
}

func TestMCPServerAuditLogs_AdminResponseUnchanged(t *testing.T) {
	orgID, serverID := uuid.New(), uuid.New()
	h := newMCPAuditHandler(orgID, serverID)

	raw := serveAuditRoute(t, "/mcp-servers/"+serverID.String()+"/audit-logs", "/mcp-servers/:id/audit-logs", string(domain.RoleAdmin), orgID, h.GetMCPServerAuditLogs)
	// The MCP timeline has never carried a user agent; the address and the
	// metadata are what an admin keeps.
	for _, canary := range []string{canaryIP, canaryEmail, canaryMetadata} {
		assert.Contains(t, string(raw), canary)
	}
}

// Positive control on the admin route itself: the same record, read through
// GET /admin/audit-logs/:id, still carries every canary.
func TestAdminAuditLogByID_ReturnsCanaries(t *testing.T) {
	orgID := uuid.New()
	record := canaryAuditLog(orgID, uuid.New(), "agent")
	h := &AdminHandler{auditServicer: &MockAuditServiceImpl{
		GetByIDFunc: func(ctx context.Context, id uuid.UUID) (*domain.AuditLog, error) {
			return record, nil
		},
	}}

	raw := serveAuditRoute(t, "/admin/audit-logs/"+record.ID.String(), "/admin/audit-logs/:id", string(domain.RoleAdmin), orgID, h.GetAuditLogByID)
	for _, canary := range auditCanaries {
		assert.True(t, strings.Contains(string(raw), canary), "admin route must return %q", canary)
	}
}
