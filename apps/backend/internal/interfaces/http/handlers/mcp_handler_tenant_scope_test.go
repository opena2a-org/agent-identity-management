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
	recoverer "github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// mcpServerRoute is one MCPHandler route that loads a server by its :id.
type mcpServerRoute struct {
	name    string
	method  string
	suffix  string
	body    string
	handler func(h *MCPHandler) fiber.Handler
}

func mcpServerRoutes() []mcpServerRoute {
	return []mcpServerRoute{
		{"GetMCPServer", "GET", "", "", func(h *MCPHandler) fiber.Handler { return h.GetMCPServer }},
		{"UpdateMCPServer", "PUT", "", "{}", func(h *MCPHandler) fiber.Handler { return h.UpdateMCPServer }},
		{"DeleteMCPServer", "DELETE", "", "", func(h *MCPHandler) fiber.Handler { return h.DeleteMCPServer }},
		{"VerifyMCPServer", "POST", "/verify", "", func(h *MCPHandler) fiber.Handler { return h.VerifyMCPServer }},
		{"AddPublicKey", "POST", "/keys", "{}", func(h *MCPHandler) fiber.Handler { return h.AddPublicKey }},
		{"GetVerificationStatus", "GET", "/verification-status", "", func(h *MCPHandler) fiber.Handler { return h.GetVerificationStatus }},
		{"GetMCPServerCapabilities", "GET", "/capabilities", "", func(h *MCPHandler) fiber.Handler { return h.GetMCPServerCapabilities }},
		{"DetectCapabilities", "POST", "/detect-capabilities", "", func(h *MCPHandler) fiber.Handler { return h.DetectCapabilities }},
		{"GetMCPServerAgents", "GET", "/agents", "", func(h *MCPHandler) fiber.Handler { return h.GetMCPServerAgents }},
		{"GetMCPVerificationEvents", "GET", "/verification-events", "", func(h *MCPHandler) fiber.Handler { return h.GetMCPVerificationEvents }},
		{"GetMCPServerAuditLogs", "GET", "/audit-logs", "", func(h *MCPHandler) fiber.Handler { return h.GetMCPServerAuditLogs }},
	}
}

// serveMCPRoute mounts one route for a caller in callerOrgID and returns the
// status and raw body of a single request for serverID. A recover middleware
// turns a handler panic into a 500 so a route that fails the check reports a
// status instead of killing the test binary.
func serveMCPRoute(t *testing.T, h *MCPHandler, r mcpServerRoute, callerOrgID, serverID uuid.UUID) (int, []byte) {
	t.Helper()
	app := fiber.New()
	app.Use(recoverer.New())
	app.Add([]string{r.method}, "/mcp-servers/:id"+r.suffix, func(c fiber.Ctx) error {
		c.Locals("organization_id", callerOrgID)
		c.Locals("user_id", uuid.New())
		return r.handler(h)(c)
	})

	var body io.Reader
	if r.body != "" {
		body = strings.NewReader(r.body)
	}
	req := httptest.NewRequest(r.method, "/mcp-servers/"+serverID.String()+r.suffix, body)
	if r.body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, raw
}

// An unknown server ID and another organization's server ID must produce the
// same response on every MCP server route, so a caller cannot tell which IDs
// exist outside their own organization.
func TestMCPHandler_ForeignAndUnknownServerAnswerIdentically(t *testing.T) {
	callerOrgID := uuid.New()
	otherOrgID := uuid.New()
	unknownID := uuid.New()
	foreignID := uuid.New()

	mcpSvc := &MockMCPServiceExtendedImpl{
		GetMCPServerFunc: func(ctx context.Context, id uuid.UUID) (*domain.MCPServer, error) {
			if id == foreignID {
				return &domain.MCPServer{ID: foreignID, OrganizationID: otherOrgID, Name: "other-org-server"}, nil
			}
			return nil, assert.AnError
		},
	}
	auditSvc := &MockAuditServiceImpl{
		GetAuditLogsFunc: func(ctx context.Context, orgID uuid.UUID, action string, entityType string, entityID *uuid.UUID, userID *uuid.UUID, startDate *time.Time, endDate *time.Time, limit int, offset int) ([]*domain.AuditLog, int, error) {
			t.Errorf("audit logs were read for server %v that the caller does not own", entityID)
			return nil, 0, nil
		},
	}
	h := createMCPHandlerWithMocks(mcpSvc, &MockMCPCapabilityServiceImpl{}, auditSvc, nil, nil, nil, &MockMCPAttestationServiceExtendedImpl{})

	for _, r := range mcpServerRoutes() {
		t.Run(r.name, func(t *testing.T) {
			unknownStatus, unknownBody := serveMCPRoute(t, h, r, callerOrgID, unknownID)
			foreignStatus, foreignBody := serveMCPRoute(t, h, r, callerOrgID, foreignID)

			assert.Equal(t, fiber.StatusNotFound, unknownStatus, "unknown server: %s", unknownBody)
			assert.Equal(t, fiber.StatusNotFound, foreignStatus, "other organization's server: %s", foreignBody)
			assert.Equal(t, string(unknownBody), string(foreignBody),
				"unknown and other-organization server IDs must give byte-equal bodies")
		})
	}
}

// Control: the caller's own server returns its audit timeline.
func TestMCPHandler_GetMCPServerAuditLogs_OwnServerReturnsLogs(t *testing.T) {
	orgID := uuid.New()
	serverID := uuid.New()
	logID := uuid.New()
	userID := uuid.New()

	mcpSvc := &MockMCPServiceExtendedImpl{
		GetMCPServerFunc: func(ctx context.Context, id uuid.UUID) (*domain.MCPServer, error) {
			return &domain.MCPServer{ID: id, OrganizationID: orgID, Name: "own-server"}, nil
		},
	}
	auditSvc := &MockAuditServiceImpl{
		GetAuditLogsFunc: func(ctx context.Context, gotOrgID uuid.UUID, action string, entityType string, entityID *uuid.UUID, gotUserID *uuid.UUID, startDate *time.Time, endDate *time.Time, limit int, offset int) ([]*domain.AuditLog, int, error) {
			assert.Equal(t, orgID, gotOrgID)
			assert.Equal(t, "mcp_server", entityType)
			require.NotNil(t, entityID)
			assert.Equal(t, serverID, *entityID)
			return []*domain.AuditLog{{
				ID:             logID,
				OrganizationID: orgID,
				UserID:         &userID,
				UserName:       "Alice",
				Action:         domain.AuditActionUpdate,
				ResourceType:   "mcp_server",
				ResourceID:     serverID,
				Timestamp:      time.Now(),
			}}, 1, nil
		},
	}
	h := createMCPHandlerWithMocks(mcpSvc, &MockMCPCapabilityServiceImpl{}, auditSvc, nil, nil, nil, &MockMCPAttestationServiceExtendedImpl{})

	route := mcpServerRoutes()[len(mcpServerRoutes())-1]
	require.Equal(t, "GetMCPServerAuditLogs", route.name)
	status, raw := serveMCPRoute(t, h, route, orgID, serverID)
	require.Equal(t, fiber.StatusOK, status, "body: %s", raw)

	var result struct {
		Logs []struct {
			ID        string `json:"id"`
			EventType string `json:"eventType"`
			Action    string `json:"action"`
			ActorName string `json:"actorName"`
		} `json:"logs"`
		Total int `json:"total"`
	}
	require.NoError(t, json.Unmarshal(raw, &result))
	assert.Equal(t, 1, result.Total)
	require.Len(t, result.Logs, 1)
	assert.Equal(t, logID.String(), result.Logs[0].ID)
	assert.Equal(t, "audit", result.Logs[0].EventType)
	assert.Equal(t, string(domain.AuditActionUpdate), result.Logs[0].Action)
	assert.Equal(t, "Alice", result.Logs[0].ActorName)
}
