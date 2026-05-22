package handlers

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ===========================
// Cross-tenant existence-oracle regression tests for
// AgentHandler.GetAgentKeyVault and AgentHandler.GetAgentAuditLogs
// (agent_security_endpoints.go).
//
// Pre-fix shape: handler called h.agentService.GetAgent, returned 404
// "Agent not found" on loader error, then 403 "Access denied" when
// agent.OrganizationID != orgID. The distinct status + body let an
// attacker probe agentIDs and distinguish "no such agent system-wide"
// from "agent exists in another org" — an existence side channel that
// enables cross-tenant agent UUID enumeration.
//
// Post-fix: both branches collapse to a fixed 404
// {"error":"not found"} via LoadOwned.
//
// Regression-proofing: auditService is nil so a bypass on either
// handler would nil-deref on h.auditService.LogAction → fiber 500 →
// distinguishable from the secure 404.
// ===========================

func TestAgentHandler_SecurityEndpoints_CrossOrgReturns404(t *testing.T) {
	callerOrgID := uuid.New()
	differentOrgID := uuid.New()
	victimAgentID := uuid.New()

	// agentService returns the victim agent in a different org.
	leakySentinel := "VICTIM_PUBLIC_KEY_MUST_NOT_LEAK"
	mockAgentSvc := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             victimAgentID,
				OrganizationID: differentOrgID,
				Name:           "victim-agent",
				PublicKey:      &leakySentinel,
			}, nil
		},
	}

	// auditService intentionally nil — any path past LoadOwned would
	// call h.auditService.LogAction and nil-deref. Fiber 500 ≠ 404.
	handler := NewAgentHandlerWithInterfaces(
		mockAgentSvc, // agentServicer
		nil,          // mcpServicer
		nil,          // auditServicer (the bypass canary)
		nil,          // apiKeyServicer
		nil,          // trustScoreHandler
		nil,          // alertServicer
		nil,          // verificationEventServicer
		nil,          // capabilityServicer
		nil,          // tagServicer
		nil,          // orgRepo
		nil,          // attestationServicer
	)

	tests := []struct {
		name        string
		method      string
		requestPath string
		body        string
		setup       func(*fiber.App, *AgentHandler)
	}{
		{
			name:        "GetAgentKeyVault",
			method:      "GET",
			requestPath: "/agents/" + victimAgentID.String() + "/key-vault",
			body:        "",
			setup: func(app *fiber.App, h *AgentHandler) {
				app.Get("/agents/:id/key-vault", func(c fiber.Ctx) error {
					c.Locals("organization_id", callerOrgID)
					c.Locals("user_id", uuid.New())
					return h.GetAgentKeyVault(c)
				})
			},
		},
		{
			name:        "GetAgentAuditLogs",
			method:      "GET",
			requestPath: "/agents/" + victimAgentID.String() + "/audit-logs",
			body:        "",
			setup: func(app *fiber.App, h *AgentHandler) {
				app.Get("/agents/:id/audit-logs", func(c fiber.Ctx) error {
					c.Locals("organization_id", callerOrgID)
					c.Locals("user_id", uuid.New())
					return h.GetAgentAuditLogs(c)
				})
			},
		},
		{
			// SECURITY (HIGH IDOR): pre-fix VerifyCapability set
			// orgID := agent.OrganizationID, then wrote audit-log /
			// verification-event / alert rows into the VICTIM org
			// using attacker's IP + userID. The LoadOwned check now
			// blocks the cross-tenant path. nil-deref-as-proof: the
			// auditService is nil so a bypass would crash on
			// LogAction at line 591.
			name:        "VerifyCapability",
			method:      "POST",
			requestPath: "/agents/" + victimAgentID.String() + "/verify-capability",
			body:        `{"capability":"file:write","resource":"/sensitive"}`,
			setup: func(app *fiber.App, h *AgentHandler) {
				app.Post("/agents/:id/verify-capability", func(c fiber.Ctx) error {
					c.Locals("organization_id", callerOrgID)
					c.Locals("user_id", uuid.New())
					return h.VerifyCapability(c)
				})
			},
		},
		{
			// SECURITY: pre-fix LogCapabilityResult had no
			// caller-org check and was in the lint allowlist. The
			// service call would write a per-agent capability
			// result with attacker-supplied agentID + auditID.
			name:        "LogCapabilityResult",
			method:      "POST",
			requestPath: "/agents/" + victimAgentID.String() + "/log-capability/" + uuid.New().String(),
			body:        `{"success":true}`,
			setup: func(app *fiber.App, h *AgentHandler) {
				app.Post("/agents/:id/log-capability/:audit_id", func(c fiber.Ctx) error {
					c.Locals("organization_id", callerOrgID)
					c.Locals("user_id", uuid.New())
					return h.LogCapabilityResult(c)
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			tt.setup(app, handler)

			var req *http.Request
			if tt.body == "" {
				req = httptest.NewRequest(tt.method, tt.requestPath, nil)
			} else {
				req = httptest.NewRequest(tt.method, tt.requestPath, strings.NewReader(tt.body))
				req.Header.Set("Content-Type", "application/json")
			}
			resp, err := app.Test(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, fiber.StatusNotFound, resp.StatusCode,
				"%s: cross-tenant request must return 404, got %d", tt.name, resp.StatusCode)
			body, _ := io.ReadAll(resp.Body)
			bodyStr := string(body)
			assert.JSONEq(t, `{"error":"not found"}`, bodyStr,
				"%s: cross-tenant 404 body must be existence-secrecy shape, got %s", tt.name, bodyStr)
			// Defense-in-depth: confirm no fragment of the victim
			// agent's sensitive fields appears in the response.
			assert.NotContains(t, bodyStr, "VICTIM_PUBLIC_KEY",
				"%s: response must not leak any victim-agent field", tt.name)
			assert.NotContains(t, bodyStr, "victim-agent",
				"%s: response must not leak victim-agent name", tt.name)
		})
	}
}
