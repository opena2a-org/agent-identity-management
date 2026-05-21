package handlers

import (
	"context"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// ===========================
// A3d-vii.a: cross-tenant access on the 11 agent-scoped A2AHandler
// routes must return 404 with existence-secrecy body. The LoadOwned
// guard (via loadOwnedAgent helper) short-circuits BEFORE any
// a2aService dispatch; a2aService and auditService are nil here, so
// any bypass that reached them would panic and surface as a 500 —
// observably distinct from the 404 the tests assert. Same pattern as
// PR #185 (A3d-vi MCPAttestationHandler) and PR #150's panic-proof
// cross-org test on GetViolationsByAgent.
//
// Never use t.Fatalf inside a fiber app.Test mock goroutine — fiber
// v3 runs handlers on a separate goroutine and runtime.Goexit from a
// non-test goroutine does not fail the test (memory:
// feedback_fiber_app_test_goroutine_t_fatalf_race).
// ===========================

func TestA2AHandler_AgentScoped_CrossOrgReturns404(t *testing.T) {
	callerOrgID := uuid.New()
	callerUserID := uuid.New()
	callerAgentID := uuid.New()
	differentOrgID := uuid.New()
	pathID := uuid.New()

	agentSvc := &MockAgentServiceImpl{
		GetAgentFunc: func(_ context.Context, id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             id,
				OrganizationID: differentOrgID,
				Name:           "victim-agent",
			}, nil
		},
	}

	// a2aService and auditService are nil — any handler path that
	// reaches them after LoadOwned would panic and surface as a 500.
	handler := &A2AHandler{
		a2aService:   nil,
		agentService: agentSvc,
		auditService: nil,
	}

	setLocals := func(c fiber.Ctx) {
		c.Locals("organization_id", callerOrgID)
		c.Locals("user_id", callerUserID)
		c.Locals("agent_id", callerAgentID)
	}

	cases := []struct {
		name        string
		method      string
		mount       func(app *fiber.App)
		requestPath string
		body        string
	}{
		{
			name:   "GetAgentCard_CrossOrg",
			method: "GET",
			mount: func(app *fiber.App) {
				app.Get("/a2a/agents/:id/card", func(c fiber.Ctx) error {
					setLocals(c)
					return handler.GetAgentCard(c)
				})
			},
			requestPath: "/a2a/agents/" + pathID.String() + "/card",
		},
		{
			name:   "SignRequest_CrossOrg",
			method: "POST",
			mount: func(app *fiber.App) {
				app.Post("/a2a/agents/:id/sign", func(c fiber.Ctx) error {
					setLocals(c)
					return handler.SignRequest(c)
				})
			},
			requestPath: "/a2a/agents/" + pathID.String() + "/sign",
			body:        `{"method":"GET","path":"/x","body":""}`,
		},
		{
			name:   "GetA2ATrustScore_CrossOrg",
			method: "GET",
			mount: func(app *fiber.App) {
				app.Get("/a2a/agents/:id/trust-score", func(c fiber.Ctx) error {
					setLocals(c)
					return handler.GetA2ATrustScore(c)
				})
			},
			requestPath: "/a2a/agents/" + pathID.String() + "/trust-score",
		},
		{
			name:   "GetPeerTrustScore_CrossOrgPathAgent",
			method: "GET",
			mount: func(app *fiber.App) {
				app.Get("/a2a/agents/:id/peers/:peer_id/trust", func(c fiber.Ctx) error {
					setLocals(c)
					return handler.GetPeerTrustScore(c)
				})
			},
			requestPath: "/a2a/agents/" + pathID.String() + "/peers/" + uuid.New().String() + "/trust",
		},
		{
			name:   "GetAgentSkills_CrossOrg",
			method: "GET",
			mount: func(app *fiber.App) {
				app.Get("/a2a/agents/:id/skills", func(c fiber.Ctx) error {
					setLocals(c)
					return handler.GetAgentSkills(c)
				})
			},
			requestPath: "/a2a/agents/" + pathID.String() + "/skills",
		},
		{
			name:   "GetConsensusStatus_CrossOrgAgent_AltPath",
			method: "GET",
			mount: func(app *fiber.App) {
				app.Get("/a2a/agents/:id/skills/:skillId/consensus", func(c fiber.Ctx) error {
					setLocals(c)
					return handler.GetConsensusStatus(c)
				})
			},
			requestPath: "/a2a/agents/" + pathID.String() + "/skills/test-skill/consensus",
		},
		{
			name:   "GetConsensusStatus_CrossOrgAgent_AgentIdParam",
			method: "GET",
			mount: func(app *fiber.App) {
				app.Get("/a2a/consensus/:agentId/:skillId", func(c fiber.Ctx) error {
					setLocals(c)
					return handler.GetConsensusStatus(c)
				})
			},
			requestPath: "/a2a/consensus/" + pathID.String() + "/test-skill",
		},
		{
			name:   "GetAgentAttestations_CrossOrg",
			method: "GET",
			mount: func(app *fiber.App) {
				app.Get("/a2a/agents/:id/attestations", func(c fiber.Ctx) error {
					setLocals(c)
					return handler.GetAgentAttestations(c)
				})
			},
			requestPath: "/a2a/agents/" + pathID.String() + "/attestations",
		},
		{
			name:   "GetTrustScoreAlt_CrossOrg",
			method: "GET",
			mount: func(app *fiber.App) {
				app.Get("/a2a/trust/:id", func(c fiber.Ctx) error {
					setLocals(c)
					return handler.GetTrustScoreAlt(c)
				})
			},
			requestPath: "/a2a/trust/" + pathID.String(),
		},
		{
			name:   "UpdateTrustScore_CrossOrgTarget",
			method: "PUT",
			mount: func(app *fiber.App) {
				app.Put("/a2a/trust/:id", func(c fiber.Ctx) error {
					setLocals(c)
					return handler.UpdateTrustScore(c)
				})
			},
			requestPath: "/a2a/trust/" + pathID.String(),
			body:        `{"score":0.5,"confidence":0.5,"reason":"x"}`,
		},
		{
			name:   "RecordInteraction_CrossOrgTarget",
			method: "POST",
			mount: func(app *fiber.App) {
				app.Post("/a2a/trust/:id/interaction", func(c fiber.Ctx) error {
					setLocals(c)
					return handler.RecordInteraction(c)
				})
			},
			requestPath: "/a2a/trust/" + pathID.String() + "/interaction",
			body:        `{"taskType":"x","success":true,"durationMs":1}`,
		},
		{
			name:   "UpdateAgentCard_CrossOrg",
			method: "PUT",
			mount: func(app *fiber.App) {
				app.Put("/a2a/cards/:id", func(c fiber.Ctx) error {
					setLocals(c)
					return handler.UpdateAgentCard(c)
				})
			},
			requestPath: "/a2a/cards/" + pathID.String(),
			body:        `{"name":"x","description":"y","version":"z","endpoint":"e","skills":[]}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := fiber.New()
			tc.mount(app)

			r := httptest.NewRequest(tc.method, tc.requestPath, strings.NewReader(tc.body))
			if tc.body != "" {
				r.Header.Set("Content-Type", "application/json")
			}

			resp, err := app.Test(r)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, fiber.StatusNotFound, resp.StatusCode,
				"cross-org request must return 404 (existence-secrecy); 500 would mean loadOwnedAgent was bypassed and the nil a2aService was reached")
			body, _ := io.ReadAll(resp.Body)
			assert.JSONEq(t, `{"error":"not found"}`, string(body),
				"cross-org body must be the standard not-found shape")
		})
	}
}
