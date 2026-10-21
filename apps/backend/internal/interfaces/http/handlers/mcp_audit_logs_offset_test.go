package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/middleware"
)

// GET /api/v1/mcp-servers/:id/audit-logs pages an MCP server's timeline in
// memory, and reads a negative offset as 0. The route runs here behind the
// recovery middleware the server mounts, so a panic in the handler is answered
// 500 instead of ending the test binary.

const mcpAuditLogsPattern = "/api/v1/mcp-servers/:id/audit-logs"

func mcpAuditLogsTarget(serverID uuid.UUID) string {
	return "/api/v1/mcp-servers/" + serverID.String() + "/audit-logs"
}

// memberMCPAuditLogsApp serves the route to a member's session, for an MCP
// server of the organization whose timeline holds events events, a minute
// apart. The events take turns between the timeline's three sources: the
// newest is an audit record, the next an attestation, the next a detected
// capability, and so on.
func memberMCPAuditLogsApp(t *testing.T, events int) (*fiber.App, uuid.UUID) {
	t.Helper()
	org, serverID := uuid.New(), uuid.New()
	newest := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)

	var (
		records      []*domain.AuditLog
		attestations []*domain.AttestationWithAgentDetails
		capabilities []*domain.MCPServerCapability
	)
	for i := 0; i < events; i++ {
		at := newest.Add(-time.Duration(i) * time.Minute)
		switch i % 3 {
		case 0:
			records = append(records, &domain.AuditLog{
				ID:             uuid.New(),
				OrganizationID: org,
				Action:         domain.AuditActionUpdate,
				ResourceType:   "mcp_server",
				ResourceID:     serverID,
				Timestamp:      at,
			})
		case 1:
			attestations = append(attestations, &domain.AttestationWithAgentDetails{
				ID:         uuid.New(),
				AgentID:    uuid.New(),
				AgentName:  "billing-agent",
				VerifiedAt: at.Format(time.RFC3339),
				IsValid:    true,
			})
		case 2:
			capabilities = append(capabilities, &domain.MCPServerCapability{
				ID:             uuid.New(),
				MCPServerID:    serverID,
				Name:           "read_file",
				CapabilityType: domain.MCPCapabilityTypeTool,
				DetectedAt:     at,
				IsActive:       true,
			})
		}
	}

	mcpSvc := &MockMCPServiceExtendedImpl{
		GetMCPServerFunc: func(ctx context.Context, id uuid.UUID) (*domain.MCPServer, error) {
			if id != serverID {
				return nil, errors.New("mcp server not found")
			}
			return &domain.MCPServer{ID: serverID, OrganizationID: org, Name: "files-server"}, nil
		},
	}
	auditSvc := &MockAuditServiceImpl{
		GetAuditLogsFunc: func(ctx context.Context, orgID uuid.UUID, action string, entityType string, entityID *uuid.UUID, userID *uuid.UUID, startDate *time.Time, endDate *time.Time, limit int, offset int) ([]*domain.AuditLog, int, error) {
			return records, len(records), nil
		},
	}
	capSvc := &MockMCPCapabilityServiceImpl{
		GetCapabilitiesFunc: func(ctx context.Context, mcpServerID uuid.UUID) ([]*domain.MCPServerCapability, error) {
			return capabilities, nil
		},
	}
	attSvc := &MockMCPAttestationServiceExtendedImpl{
		GetMCPAttestationsFunc: func(ctx context.Context, mcpServerID uuid.UUID) ([]*domain.AttestationWithAgentDetails, float64, time.Time, error) {
			return attestations, 0, time.Time{}, nil
		},
	}
	h := createMCPHandlerWithMocks(mcpSvc, capSvc, auditSvc, nil, nil, nil, attSvc)

	app := fiber.New()
	app.Use(middleware.RecoveryMiddleware())
	app.Get(mcpAuditLogsPattern, func(c fiber.Ctx) error {
		c.Locals("organization_id", org)
		c.Locals("user_id", uuid.New())
		c.Locals("role", string(domain.RoleMember))
		return c.Next()
	}, h.GetMCPServerAuditLogs)
	return app, serverID
}

func mcpAuditLogsGet(t *testing.T, app *fiber.App, target string) (int, string) {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest("GET", target, nil))
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(body)
}

type mcpAuditLogsPage struct {
	Logs []struct {
		ID        string `json:"id"`
		EventType string `json:"eventType"`
	} `json:"logs"`
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

func decodeMCPAuditLogsPage(t *testing.T, status int, body string, msg string) mcpAuditLogsPage {
	t.Helper()
	require.Equal(t, fiber.StatusOK, status, "%s: %s", msg, body)
	var page mcpAuditLogsPage
	require.NoError(t, json.Unmarshal([]byte(body), &page), msg)
	return page
}

func (p mcpAuditLogsPage) eventTypes() []string {
	types := make([]string, 0, len(p.Logs))
	for _, l := range p.Logs {
		types = append(types, l.EventType)
	}
	return types
}

// A member's session with a negative offset gets the first page, the same
// body offset 0 gets, with zero events and with one event from each source.
func TestMCPServerAuditLogs_NegativeOffsetReadsAsZero(t *testing.T) {
	for _, events := range []int{0, 3} {
		for _, offset := range []string{"-1", "-100"} {
			msg := fmt.Sprintf("%d events, offset %s", events, offset)
			t.Run(msg, func(t *testing.T) {
				app, serverID := memberMCPAuditLogsApp(t, events)
				status, body := mcpAuditLogsGet(t, app, mcpAuditLogsTarget(serverID)+"?limit=2&offset="+offset)
				page := decodeMCPAuditLogsPage(t, status, body, msg)
				assert.Equal(t, 0, page.Offset, msg)
				assert.Equal(t, 2, page.Limit, msg)
				assert.Equal(t, events, page.Total, msg)
				assert.Len(t, page.Logs, min(events, 2), msg)

				firstStatus, firstPage := mcpAuditLogsGet(t, app, mcpAuditLogsTarget(serverID)+"?limit=2&offset=0")
				require.Equal(t, fiber.StatusOK, firstStatus, firstPage)
				assert.Equal(t, firstPage, body, "%s: the first page is served", msg)
			})
		}
	}
}

// A non-negative offset pages the timeline, newest first, as before.
func TestMCPServerAuditLogs_OffsetPagesTheTimeline(t *testing.T) {
	app, serverID := memberMCPAuditLogsApp(t, 3)
	cases := []struct {
		offset string
		want   []string
	}{
		{"0", []string{"audit", "attestation"}},
		{"2", []string{"capability"}},
		{"3", []string{}},
	}
	for _, tc := range cases {
		msg := "offset " + tc.offset
		status, body := mcpAuditLogsGet(t, app, mcpAuditLogsTarget(serverID)+"?limit=2&offset="+tc.offset)
		page := decodeMCPAuditLogsPage(t, status, body, msg)
		assert.Equal(t, tc.offset, fmt.Sprint(page.Offset), msg)
		assert.Equal(t, 3, page.Total, msg)
		assert.Equal(t, tc.want, page.eventTypes(), msg)
	}
}
