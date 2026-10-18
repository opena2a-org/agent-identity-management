package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/registry"
)

// These tests are a no-DB route-registration smoke for POST /agents/:id/atc.
// They exercise the authz + validation paths that return before any service or
// repository call, so the handler can be wired with nil dependencies. Live
// end-to-end verification (against Postgres + the Registry) is Phase D.

func TestATCIssuanceHandler_MissingOrg_Unauthorized(t *testing.T) {
	h := NewATCIssuanceHandler(nil, nil, nil)
	app := fiber.New()
	app.Post("/agents/:id/atc", h.IssueATC)

	req := httptest.NewRequest("POST", "/agents/"+uuid.New().String()+"/atc", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 when organization_id is absent", resp.StatusCode)
	}
}

func TestATCIssuanceHandler_NilOrg_Unauthorized(t *testing.T) {
	h := NewATCIssuanceHandler(nil, nil, nil)
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		// A malformed token could set a zero-value org. It must not authorize: a
		// Nil orgID would compare equal to a Nil-org agent and collapse the tenant
		// check.
		c.Locals("organization_id", uuid.Nil)
		c.Locals("user_id", uuid.New())
		return c.Next()
	})
	app.Post("/agents/:id/atc", h.IssueATC)

	req := httptest.NewRequest("POST", "/agents/"+uuid.New().String()+"/atc", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 when organization_id is the zero UUID", resp.StatusCode)
	}
}

func TestATCIssuanceHandler_BadAgentID_BadRequest(t *testing.T) {
	h := NewATCIssuanceHandler(nil, nil, nil)
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", uuid.New())
		return c.Next()
	})
	app.Post("/agents/:id/atc", h.IssueATC)

	req := httptest.NewRequest("POST", "/agents/not-a-uuid/atc", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for an invalid agent ID", resp.StatusCode)
	}
}

// oneAgentRepo serves one agent by ID and reports every other ID as missing.
type oneAgentRepo struct {
	domain.AgentRepository
	agent *domain.Agent
}

func (r *oneAgentRepo) GetByID(id uuid.UUID) (*domain.Agent, error) {
	if id == r.agent.ID {
		return r.agent, nil
	}
	return nil, errors.New("agent not found")
}

// refusingIssuances is the agent reader of an issuance service that refuses
// every credential it is asked for and counts the requests. A handler that
// reaches the service answers 502.
type refusingIssuances struct{ asked int }

func (r *refusingIssuances) GetByID(uuid.UUID) (*domain.Agent, error) {
	r.asked++
	return nil, errors.New("issuance refused")
}

// atcIssuer answers POST /agents/:id/atc for a caller in callerOrg and returns
// the status and body. The handler reads its agents from a repository holding
// agent alone, and its issuance service is the stub issuances.
func atcIssuer(t *testing.T, callerOrg uuid.UUID, agent *domain.Agent, issuances *refusingIssuances) func(uuid.UUID) (int, string) {
	t.Helper()
	return atcIssuerWithService(t, callerOrg, agent, application.NewATCIssuanceService(issuances, nil, nil, nil, nil, "https://aim.example"))
}

// atcIssuerWithService is atcIssuer with the issuance service issuanceService.
func atcIssuerWithService(t *testing.T, callerOrg uuid.UUID, agent *domain.Agent, issuanceService *application.ATCIssuanceService) func(uuid.UUID) (int, string) {
	t.Helper()
	agentService := application.NewAgentService(&oneAgentRepo{agent: agent}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	h := NewATCIssuanceHandler(issuanceService, agentService, nil)
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Locals("organization_id", callerOrg)
		c.Locals("user_id", uuid.New())
		return c.Next()
	})
	app.Post("/agents/:id/atc", h.IssueATC)

	return func(agentID uuid.UUID) (int, string) {
		t.Helper()
		req := httptest.NewRequest("POST", "/agents/"+agentID.String()+"/atc", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test: %v", err)
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		return resp.StatusCode, string(body)
	}
}

// Another organization's agent and an ID that matches no agent get the same
// answer, the one GET /agents/:id gives for both, so the response does not
// reveal which IDs exist in other organizations. Neither request reaches the
// issuance service.
func TestATCIssuanceHandler_OtherOrganizationsAgentAnswersLikeAMissingOne(t *testing.T) {
	other := &domain.Agent{ID: uuid.New(), OrganizationID: uuid.New(), Name: "other-org-agent"}
	issuances := &refusingIssuances{}
	issue := atcIssuer(t, uuid.New(), other, issuances)

	crossStatus, crossBody := issue(other.ID)
	missingStatus, missingBody := issue(uuid.New())
	if crossStatus != http.StatusNotFound {
		t.Errorf("status = %d, want 404 for another organization's agent", crossStatus)
	}
	if missingStatus != http.StatusNotFound {
		t.Errorf("status = %d, want 404 for a missing agent", missingStatus)
	}
	if crossBody != missingBody {
		t.Errorf("another organization's agent answers %s, a missing agent %s; want the same body", crossBody, missingBody)
	}
	if issuances.asked != 0 {
		t.Errorf("the issuance service was asked for %d credential(s), want none", issuances.asked)
	}
}

// The caller's own agent passes the organization check and reaches the issuance
// service, whose refusal is answered 502. This is what a request for another
// organization's agent would get if it passed the check, so the 404 and the
// zero count asserted above can fail.
func TestATCIssuanceHandler_OwnAgentReachesTheIssuanceService(t *testing.T) {
	own := &domain.Agent{ID: uuid.New(), OrganizationID: uuid.New(), Name: "own-agent"}
	issuances := &refusingIssuances{}
	issue := atcIssuer(t, own.OrganizationID, own, issuances)

	status, body := issue(own.ID)
	if status != http.StatusBadGateway {
		t.Errorf("status = %d, want 502 when the issuance service refuses; body %s", status, body)
	}
	if issuances.asked != 1 {
		t.Errorf("the issuance service was asked for %d credential(s), want 1", issuances.asked)
	}
}

// agentRevokedAfterReads serves a copy of agent for its first reads reads and
// the agent revoked on every later one.
type agentRevokedAfterReads struct {
	agent *domain.Agent
	reads int
	read  int
}

func (r *agentRevokedAfterReads) GetByID(uuid.UUID) (*domain.Agent, error) {
	r.read++
	agent := *r.agent
	if r.read > r.reads {
		agent.Status = domain.AgentStatusRevoked
	}
	return &agent, nil
}

type noCapabilityGrants struct{}

func (noCapabilityGrants) GetActiveCapabilitiesByAgentID(uuid.UUID) ([]*domain.AgentCapability, error) {
	return nil, nil
}

type fixedTrustScore struct{}

func (fixedTrustScore) CalculateTrustScore(context.Context, uuid.UUID) (*domain.TrustScore, error) {
	return &domain.TrustScore{Score: 0.8}, nil
}

// countingRegistry answers each issuance with a credential for the requested
// agent and counts the issuances.
type countingRegistry struct{ issued int }

func (r *countingRegistry) IssueATC(_ context.Context, req registry.ATCIssuanceRequest) (*registry.AgentTrustCredential, error) {
	r.issued++
	return &registry.AgentTrustCredential{AgentID: req.AgentID, AgentDID: req.AgentDID}, nil
}

// The caller's own revoked agent is answered 409 with reasonCode agentRevoked,
// the line that says what to do instead, and no credential: when it was revoked
// before the request, and when it is revoked while the Registry is issuing.
func TestATCIssuanceHandler_RevokedAgentAnswersConflict(t *testing.T) {
	cases := []struct {
		name string
		// unrevokedReads is how many of the issuance service's reads of the
		// agent find it not revoked.
		unrevokedReads int
		wantIssued     int
	}{
		{name: "revoked before the request", unrevokedReads: 0, wantIssued: 0},
		{name: "revoked while the Registry issues", unrevokedReads: 1, wantIssued: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			agent := &domain.Agent{ID: uuid.New(), OrganizationID: uuid.New(), Name: "revoked-agent", Status: domain.AgentStatusVerified}
			reg := &countingRegistry{}
			issuanceService := application.NewATCIssuanceService(
				&agentRevokedAfterReads{agent: agent, reads: tc.unrevokedReads},
				noCapabilityGrants{}, nil, fixedTrustScore{}, reg, "https://aim.example")
			issue := atcIssuerWithService(t, agent.OrganizationID, agent, issuanceService)

			status, raw := issue(agent.ID)
			if status != http.StatusConflict {
				t.Fatalf("status = %d, want 409 for a revoked agent; body %s", status, raw)
			}
			var body map[string]interface{}
			if err := json.Unmarshal([]byte(raw), &body); err != nil {
				t.Fatalf("decode body %s: %v", raw, err)
			}
			assertReasonCode(t, body, "agentRevoked")
			if want := "This agent is revoked, and no Agent Trust Credential is issued for a revoked agent: register a new agent to replace it"; body["error"] != want {
				t.Errorf("error = %v, want %q", body["error"], want)
			}
			if _, ok := body["credential"]; ok {
				t.Errorf("the refusal carries a credential: %s", raw)
			}
			if reg.issued != tc.wantIssued {
				t.Errorf("the Registry was asked for %d credential(s), want %d", reg.issued, tc.wantIssued)
			}
		})
	}
}
