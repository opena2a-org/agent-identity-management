package handlers

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
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

// Another organization's agent and an ID that matches no agent get the same
// answer, the one GET /agents/:id gives for both, so the response does not
// reveal which IDs exist in other organizations. Both return before the
// issuance service is called.
func TestATCIssuanceHandler_OtherOrganizationsAgentAnswersLikeAMissingOne(t *testing.T) {
	other := &domain.Agent{ID: uuid.New(), OrganizationID: uuid.New(), Name: "other-org-agent"}
	agentService := application.NewAgentService(&oneAgentRepo{agent: other}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	h := NewATCIssuanceHandler(nil, agentService, nil)
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Locals("organization_id", uuid.New())
		c.Locals("user_id", uuid.New())
		return c.Next()
	})
	app.Post("/agents/:id/atc", h.IssueATC)

	issue := func(agentID uuid.UUID) (int, string) {
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
}
