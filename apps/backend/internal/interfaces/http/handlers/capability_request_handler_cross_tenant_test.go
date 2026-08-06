package handlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// capRequestTestAgentRepo is a minimal domain.AgentRepository stub. Only
// GetByID is implemented; the other ~24 methods inherit from the embedded
// interface and will panic on access — that's the regression-proofing
// signal for handlers that read fields other than OrganizationID.
type capRequestTestAgentRepo struct {
	domain.AgentRepository
	getByID func(id uuid.UUID) (*domain.Agent, error)
}

func (r *capRequestTestAgentRepo) GetByID(id uuid.UUID) (*domain.Agent, error) {
	return r.getByID(id)
}

// capRequestTestRequestRepo is a stub domain.CapabilityRequestRepository.
// Only GetByID is used by the Reject path's LoadOwnedViaAgent loader;
// UpdateStatus is exposed via a captured flag so the test can prove the
// gate fires BEFORE any state mutation on a cross-tenant probe.
type capRequestTestRequestRepo struct {
	domain.CapabilityRequestRepository
	getByID        func(id uuid.UUID) (*domain.CapabilityRequestWithDetails, error)
	updateStatusCh chan struct{}
}

func (r *capRequestTestRequestRepo) GetByID(id uuid.UUID) (*domain.CapabilityRequestWithDetails, error) {
	return r.getByID(id)
}

func (r *capRequestTestRequestRepo) UpdateStatus(id uuid.UUID, status domain.CapabilityRequestStatus, reviewedBy uuid.UUID) error {
	// Bypass signal: if RejectRequest ever runs UpdateStatus on a
	// cross-tenant probe, this fires and the test fails.
	r.updateStatusCh <- struct{}{}
	return nil
}

// ===========================
// A3d-ix: cross-tenant access on the 3 CapabilityRequestHandlers
// routes must return 404 with the existence-secrecy body.
//
// PQC SDK auth middleware (pqc_agent_auth.go:151-155) authenticates
// via X-Agent-ID header; the affected handlers trust URL path :id.
// Pre-fix, an SDK client with X-Agent-ID = agent-in-org-A could send
// the body to a URL containing victim-agent-in-org-B and plant a
// phantom capability request (CreateCapabilityRequest), read the
// victim's pending queue (ListAgentCapabilityRequests), or reject a
// victim-org admin's pending request (RejectCapabilityRequest).
//
// Regression-proofing:
//   - CreateCapabilityRequest / ListAgentCapabilityRequests:
//     service intentionally nil — bypass nil-derefs on the
//     h.service.CreateRequest / h.service.ListRequests call →
//     fiber 500 → assert.Equal(404) fires.
//   - RejectCapabilityRequest: captured-flag pattern. service is
//     real CapabilityRequestService with a stub requestRepo. The
//     stub's UpdateStatus blocks on the channel; the test asserts
//     the channel is empty after the cross-tenant call.
// ===========================

func TestCapabilityRequestHandlers_SDKRoutes_CrossOrgReturns404(t *testing.T) {
	callerOrgID := uuid.New()
	differentOrgID := uuid.New()
	victimAgentID := uuid.New()

	agentRepoForeignOrg := &capRequestTestAgentRepo{
		getByID: func(id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             victimAgentID,
				OrganizationID: differentOrgID,
				CreatedBy:      uuid.New(), // victim org's owner — would leak via RequestedBy pre-fix
				Name:           "victim-agent",
			}, nil
		},
	}

	// service intentionally nil — bypass nil-derefs and surfaces as fiber 500.
	handler := &CapabilityRequestHandlers{
		service:   nil,
		agentRepo: agentRepoForeignOrg,
	}

	tests := []struct {
		name        string
		method      string
		requestPath string
		body        string
		setup       func(*fiber.App, *CapabilityRequestHandlers)
	}{
		{
			name:        "CreateCapabilityRequest",
			method:      "POST",
			requestPath: "/sdk-api/agents/" + victimAgentID.String() + "/capability-requests",
			body:        `{"capabilityType":"file_write","reason":"this is a long enough reason"}`,
			setup: func(app *fiber.App, h *CapabilityRequestHandlers) {
				app.Post("/sdk-api/agents/:id/capability-requests", func(c fiber.Ctx) error {
					c.Locals("organization_id", callerOrgID)
					c.Locals("user_id", uuid.New())
					return h.CreateCapabilityRequest(c)
				})
			},
		},
		{
			name:        "ListAgentCapabilityRequests",
			method:      "GET",
			requestPath: "/sdk-api/agents/" + victimAgentID.String() + "/capability-requests",
			body:        "",
			setup: func(app *fiber.App, h *CapabilityRequestHandlers) {
				app.Get("/sdk-api/agents/:id/capability-requests", func(c fiber.Ctx) error {
					c.Locals("organization_id", callerOrgID)
					c.Locals("user_id", uuid.New())
					return h.ListAgentCapabilityRequests(c)
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
			assert.JSONEq(t, `{"error":"not found"}`, string(body),
				"%s: cross-tenant 404 body must be existence-secrecy shape, got %s", tt.name, string(body))
		})
	}
}

// RejectCapabilityRequest cross-org test uses the captured-flag pattern.
// A real CapabilityRequestService is constructed with a stub requestRepo
// that returns a foreign request from GetByID; the stub also tracks
// whether UpdateStatus is ever called. The gate must fire before the
// service ever touches the rejection mutation.
func TestCapabilityRequestHandlers_RejectCapabilityRequest_CrossOrgReturns404(t *testing.T) {
	callerOrgID := uuid.New()
	differentOrgID := uuid.New()
	victimAgentID := uuid.New()
	requestID := uuid.New()

	// Buffer 1 so a bypass-write doesn't block; assert empty at end.
	updateStatusCh := make(chan struct{}, 1)

	requestRepoStub := &capRequestTestRequestRepo{
		getByID: func(id uuid.UUID) (*domain.CapabilityRequestWithDetails, error) {
			return &domain.CapabilityRequestWithDetails{
				CapabilityRequest: domain.CapabilityRequest{
					ID:             id,
					AgentID:        victimAgentID,
					CapabilityType: "file_write",
					Status:         domain.CapabilityRequestStatusPending,
				},
			}, nil
		},
		updateStatusCh: updateStatusCh,
	}

	agentRepoForeignOrg := &capRequestTestAgentRepo{
		getByID: func(id uuid.UUID) (*domain.Agent, error) {
			return &domain.Agent{
				ID:             victimAgentID,
				OrganizationID: differentOrgID,
				Name:           "victim-agent",
			}, nil
		},
	}

	// Real CapabilityRequestService built with stub requestRepo. The
	// other repo deps are nil because RejectRequest only touches
	// requestRepo.
	svc := application.NewCapabilityRequestService(requestRepoStub, nil, nil, nil)
	handler := &CapabilityRequestHandlers{
		service:   svc,
		agentRepo: agentRepoForeignOrg,
	}

	app := fiber.New()
	app.Post("/admin/capability-requests/:id/reject", func(c fiber.Ctx) error {
		c.Locals("organization_id", callerOrgID)
		c.Locals("user_id", uuid.New())
		return handler.RejectCapabilityRequest(c)
	})

	req := httptest.NewRequest("POST", "/admin/capability-requests/"+requestID.String()+"/reject", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode,
		"cross-tenant reject must return 404, got %d", resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	assert.JSONEq(t, `{"error":"not found"}`, string(body),
		"cross-tenant reject 404 body must be existence-secrecy shape, got %s", string(body))

	// Bypass detection: the gate must fire BEFORE RejectRequest's
	// UpdateStatus call. An empty channel means the mutation never ran.
	select {
	case <-updateStatusCh:
		t.Fatalf("RejectRequest.UpdateStatus was called on a cross-tenant probe — LoadOwnedViaAgent gate did not fire")
	default:
		// expected: gate fired, no mutation
	}
}

func (r *capRequestTestAgentRepo) ListRevokedIDs(limit, offset int) ([]uuid.UUID, error) {
	return nil, nil
}
