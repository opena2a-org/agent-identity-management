package handlers

import (
	"context"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockA2AReaderImpl implements the A2AReader interface for tests that
// need to exercise the tenant-ownership guard in A2AHandler.RevokeConsent
// and A2AHandler.UpdateTaskState without standing up an actual A2AService.
type MockA2AReaderImpl struct {
	GetConsentFunc func(ctx context.Context, consentID uuid.UUID) (*domain.A2AConsentRecord, error)
	GetA2ATaskFunc func(ctx context.Context, taskID uuid.UUID) (*domain.A2ATask, error)
}

func (m *MockA2AReaderImpl) GetConsent(ctx context.Context, consentID uuid.UUID) (*domain.A2AConsentRecord, error) {
	if m.GetConsentFunc != nil {
		return m.GetConsentFunc(ctx, consentID)
	}
	return nil, nil
}

func (m *MockA2AReaderImpl) GetA2ATask(ctx context.Context, taskID uuid.UUID) (*domain.A2ATask, error) {
	if m.GetA2ATaskFunc != nil {
		return m.GetA2ATaskFunc(ctx, taskID)
	}
	return nil, nil
}

// A3d-vii.b: a cross-tenant POST /a2a/consent/:id/revoke must return
// 404 with the existence-secrecy body, never disclose that the consent
// exists in another organization, and never invoke the mutation path.
//
// Captured-flag pattern (NOT t.Fatalf): fiber.App.Test runs the handler
// on a separate goroutine; t.Fatalf from the mock goroutine would call
// runtime.Goexit on THAT goroutine only and silently pass even if the
// LoadOwned guard were removed. Instead, the test leaves a2aService nil
// — the LoadOwned guard returns 404 before the handler can reach
// h.a2aService.RevokeConsent, so the nil pointer is never dereferenced.
// If a regression removes the guard, the test would either surface 200
// (mutation succeeded with no auth) or a 500 from the nil panic — never
// the 404 asserted here.
func TestA2AHandler_RevokeConsent_CrossTenantReturns404(t *testing.T) {
	consentID := uuid.New()
	callerOrgID := uuid.New()
	victimOrgID := uuid.New()

	loaderCalled := false
	mockReader := &MockA2AReaderImpl{
		GetConsentFunc: func(ctx context.Context, id uuid.UUID) (*domain.A2AConsentRecord, error) {
			loaderCalled = true
			return &domain.A2AConsentRecord{
				ID:             id,
				OrganizationID: &victimOrgID,
			}, nil
		},
	}

	handler := &A2AHandler{a2aReader: mockReader}

	app := fiber.New()
	app.Post("/a2a/consent/:id/revoke", func(c fiber.Ctx) error {
		c.Locals("organization_id", callerOrgID)
		c.Locals("user_id", uuid.New())
		return handler.RevokeConsent(c)
	})

	req := httptest.NewRequest("POST", "/a2a/consent/"+consentID.String()+"/revoke",
		strings.NewReader(`{"reason":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.JSONEq(t, `{"error":"not found"}`, string(body))
	assert.True(t, loaderCalled, "GetConsent must be called to load the consent record")
}

// A3d-vii.b: a consent record with no assigned OrganizationID
// (OrganizationID == nil) must NOT be revocable by any caller. The
// consentOrgID accessor returns uuid.Nil for an unassigned consent,
// which never matches a real caller's org → 404. This protects the
// unassigned-consent state from being weaponized as a wildcard mutation
// target.
func TestA2AHandler_RevokeConsent_UnassignedConsentReturns404(t *testing.T) {
	consentID := uuid.New()
	callerOrgID := uuid.New()

	mockReader := &MockA2AReaderImpl{
		GetConsentFunc: func(ctx context.Context, id uuid.UUID) (*domain.A2AConsentRecord, error) {
			return &domain.A2AConsentRecord{
				ID:             id,
				OrganizationID: nil, // unassigned
			}, nil
		},
	}

	handler := &A2AHandler{a2aReader: mockReader}

	app := fiber.New()
	app.Post("/a2a/consent/:id/revoke", func(c fiber.Ctx) error {
		c.Locals("organization_id", callerOrgID)
		c.Locals("user_id", uuid.New())
		return handler.RevokeConsent(c)
	})

	req := httptest.NewRequest("POST", "/a2a/consent/"+consentID.String()+"/revoke",
		strings.NewReader(`{"reason":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

// A3d-vii.b: a cross-tenant PUT /a2a/tasks/:id/state must return 404
// and never mutate the victim org's task. A2ATask has no direct
// OrganizationID; the LoadOwned guard runs on the task's ClientAgentID
// — if the client agent belongs to another org, the verdict is 404.
//
// RemoteAgentID is intentionally NOT scoped: A2A tasks span orgs by
// protocol design. The test reflects that by only mocking the
// ClientAgentID's owning org.
func TestA2AHandler_UpdateTaskState_CrossTenantReturns404(t *testing.T) {
	taskID := uuid.New()
	clientAgentID := uuid.New()
	callerOrgID := uuid.New()
	victimOrgID := uuid.New()

	taskLoaded := false
	mockReader := &MockA2AReaderImpl{
		GetA2ATaskFunc: func(ctx context.Context, id uuid.UUID) (*domain.A2ATask, error) {
			taskLoaded = true
			return &domain.A2ATask{
				ID:            id,
				ClientAgentID: clientAgentID,
				RemoteAgentID: uuid.New(),
			}, nil
		},
	}

	agentLoaded := false
	mockAgentService := &MockAgentServiceImpl{
		GetAgentFunc: func(ctx context.Context, id uuid.UUID) (*domain.Agent, error) {
			agentLoaded = true
			return &domain.Agent{ID: id, OrganizationID: victimOrgID}, nil
		},
	}

	handler := &A2AHandler{a2aReader: mockReader, agentService: mockAgentService}

	app := fiber.New()
	app.Put("/a2a/tasks/:id/state", func(c fiber.Ctx) error {
		c.Locals("organization_id", callerOrgID)
		return handler.UpdateTaskState(c)
	})

	req := httptest.NewRequest("PUT", "/a2a/tasks/"+taskID.String()+"/state",
		strings.NewReader(`{"state":"FAILED","errorCode":"AUTH","errorMessage":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.JSONEq(t, `{"error":"not found"}`, string(body))
	assert.True(t, taskLoaded, "GetA2ATask must be called to load the task")
	assert.True(t, agentLoaded, "agentService.GetAgent must be called to verify ClientAgentID ownership")
}

// A3d-vii.b defense-in-depth: a caller whose Locals("organization_id")
// somehow surfaces uuid.Nil (auth-chain bug, corrupted JWT claim, etc.)
// must NOT be able to revoke an unassigned consent. Both sides being
// uuid.Nil would otherwise match in a naive equality check; LoadOwned
// rejects either side being uuid.Nil up front. Phase 4.5 finding
// MEDIUM-1.
func TestA2AHandler_RevokeConsent_NilCallerOrgIsRejected(t *testing.T) {
	consentID := uuid.New()

	mockReader := &MockA2AReaderImpl{
		GetConsentFunc: func(ctx context.Context, id uuid.UUID) (*domain.A2AConsentRecord, error) {
			return &domain.A2AConsentRecord{
				ID:             id,
				OrganizationID: nil, // unassigned consent
			}, nil
		},
	}

	handler := &A2AHandler{a2aReader: mockReader}

	app := fiber.New()
	app.Post("/a2a/consent/:id/revoke", func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.Nil) // pathological caller org
		c.Locals("user_id", uuid.New())
		return handler.RevokeConsent(c)
	})

	req := httptest.NewRequest("POST", "/a2a/consent/"+consentID.String()+"/revoke",
		strings.NewReader(`{"reason":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
}

// A3d-vii.b: a PUT /a2a/tasks/:id/state for a taskID that does not
// exist must return 404. The handler must NOT continue past the
// load-or-404 fallback into the agent-ownership branch (which would
// hand a nil ClientAgentID to LoadOwned). The test confirms that
// not-found returns 404 cleanly.
func TestA2AHandler_UpdateTaskState_TaskNotFoundReturns404(t *testing.T) {
	taskID := uuid.New()
	callerOrgID := uuid.New()

	mockReader := &MockA2AReaderImpl{
		GetA2ATaskFunc: func(ctx context.Context, id uuid.UUID) (*domain.A2ATask, error) {
			return nil, nil
		},
	}

	handler := &A2AHandler{a2aReader: mockReader}

	app := fiber.New()
	app.Put("/a2a/tasks/:id/state", func(c fiber.Ctx) error {
		c.Locals("organization_id", callerOrgID)
		return handler.UpdateTaskState(c)
	})

	req := httptest.NewRequest("PUT", "/a2a/tasks/"+taskID.String()+"/state",
		strings.NewReader(`{"state":"FAILED"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.JSONEq(t, `{"error":"not found"}`, string(body))
}

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

// SECURITY (body-class #2, LogTask phantom-task IDOR): a POST
// /api/v1/a2a/tasks with body `{"_clientAgentId": "<victim-org-agent>",
// "remoteAgentId": "<attacker-org-agent>"}` from a token in the
// attacker's org must return 404, NOT create a phantom A2ATask row
// in the victim org. RemoteAgentID is intentionally NOT scoped (A2A
// tasks are cross-org by protocol design). a2aService is nil — any
// bypass would 500-panic rather than returning the asserted 404.
func TestA2AHandler_LogTask_CrossOrgClientAgentReturns404(t *testing.T) {
	callerOrgID := uuid.New()
	victimOrgID := uuid.New()
	clientAgentID := uuid.New() // victim's agent UUID (body-supplied)
	remoteAgentID := uuid.New() // attacker's own agent (legit cross-org)

	agentLoaded := false
	mockAgentService := &MockAgentServiceImpl{
		GetAgentFunc: func(_ context.Context, id uuid.UUID) (*domain.Agent, error) {
			agentLoaded = true
			return &domain.Agent{ID: id, OrganizationID: victimOrgID}, nil
		},
	}

	handler := &A2AHandler{agentService: mockAgentService}

	app := fiber.New()
	app.Post("/a2a/tasks", func(c fiber.Ctx) error {
		c.Locals("organization_id", callerOrgID)
		c.Locals("agent_id", uuid.New()) // attacker's own agent in Locals
		return handler.LogTask(c)
	})

	body := `{"externalTaskId":"phantom-1","_clientAgentId":"` + clientAgentID.String() +
		`","remoteAgentId":"` + remoteAgentID.String() + `","skillId":"exfiltrate"}`
	req := httptest.NewRequest("POST", "/a2a/tasks", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
	bodyBytes, _ := io.ReadAll(resp.Body)
	assert.JSONEq(t, `{"error":"not found"}`, string(bodyBytes))
	assert.True(t, agentLoaded, "agentService.GetAgent must be called to verify ClientAgentID ownership")
	assert.NotContains(t, string(bodyBytes), clientAgentID.String(), "response must not echo the supplied ClientAgentID")
	assert.NotContains(t, string(bodyBytes), victimOrgID.String(), "response must not echo the victim's org UUID")
}
