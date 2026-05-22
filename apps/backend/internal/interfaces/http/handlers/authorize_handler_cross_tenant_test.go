package handlers

import (
	"context"
	"io"
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

// crashingFGAAuthorizer is a sentinel that panics if Authorize is ever
// invoked. Any path that reaches FGAEngine.Authorize on a cross-tenant
// probe — i.e., a LoadOwned bypass — would call this and the test
// would crash with a recognizable panic instead of silently returning
// the FGA decision body.
type crashingFGAAuthorizer struct{}

func (crashingFGAAuthorizer) Authorize(ctx context.Context, req *application.FGARequest) (*application.FGAResult, error) {
	panic("FGAAuthorizer.Authorize must not be called on a cross-tenant probe — LoadOwned gate failed")
}

// authorizeForeignOrgAgentRepo returns a victim-org agent for any UUID
// the test asks about. The Name field carries a sentinel string the
// test asserts is NEVER present in the response body.
type authorizeForeignOrgAgentRepo struct {
	domain.AgentRepository
	victimOrgID uuid.UUID
}

func (r *authorizeForeignOrgAgentRepo) GetByID(id uuid.UUID) (*domain.Agent, error) {
	return &domain.Agent{
		ID:             id,
		OrganizationID: r.victimOrgID,
		Name:           "VICTIM_AGENT_NAME_MUST_NOT_LEAK",
		TrustScore:     0.99, // high score — pre-fix leak would surface this in OTel span
	}, nil
}

// ===========================
// AuthorizeHandler.Authorize cross-tenant regression.
//
// Pre-fix shape: the handler did not read c.Locals("organization_id")
// and FGAEngine itself is org-blind (FGARequest carries no caller-org
// field, FGAEngine.Authorize never reads any caller context — verified
// via grep across fga_engine.go: zero matches for OrganizationID /
// CallerOrgID / orgID). Any authenticated caller in org A could POST
// /agents/<victim-org-B-agent>/authorize and:
//   - receive the full 5-step FGA decision (allowed / outcome /
//     denied_by / denied_reason / steps_triggered) against the
//     victim's policies + trust score + ASC scan verdict
//   - pollute the fga.authorize OTel span tree with attributes from
//     the victim agent (agent.public_key.algorithm, agent.trust_score,
//     agent.scan_verdict per the SemConv slide-14 proposal)
//
// Post-fix: RequireOrganizationID + LoadOwned at the handler layer
// collapses cross-tenant AND not-found to a fixed 404.
//
// Regression-proofing: FGAAuthorizer is the crashing sentinel above.
// Any path that bypasses LoadOwned would panic in FGAEngine.Authorize
// and the test would fail with a recognizable error rather than
// silently returning the FGA decision.
// ===========================

func TestAuthorizeHandler_CrossOrgReturns404(t *testing.T) {
	callerOrgID := uuid.New()
	victimOrgID := uuid.New()
	victimAgentID := uuid.New()

	repo := &authorizeForeignOrgAgentRepo{victimOrgID: victimOrgID}
	h := NewAuthorizeHandlerWithInterface(crashingFGAAuthorizer{}, repo)

	app := fiber.New()
	app.Post("/api/v1/agents/:id/authorize", func(c fiber.Ctx) error {
		c.Locals("organization_id", callerOrgID)
		return h.Authorize(c)
	})

	body := strings.NewReader(`{"capability":"file:read","resource":"/sensitive"}`)
	req := httptest.NewRequest("POST", "/api/v1/agents/"+victimAgentID.String()+"/authorize", body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode,
		"cross-tenant authorize must return 404, got %d", resp.StatusCode)
	raw, _ := io.ReadAll(resp.Body)
	bodyStr := string(raw)
	assert.JSONEq(t, `{"error":"not found"}`, bodyStr,
		"cross-tenant 404 body must be existence-secrecy shape, got %s", bodyStr)
	// Defense-in-depth: none of the victim-agent's fields may appear in
	// the response. (The OTel span pollution is harder to assert from a
	// test, but if FGAEngine.Authorize is unreached we know the span
	// tree was not emitted against the victim.)
	assert.NotContains(t, bodyStr, "VICTIM_AGENT_NAME",
		"response must not leak any victim-agent field")
	assert.NotContains(t, bodyStr, "ALLOW",
		"response must not contain any FGA outcome — FGAEngine must not have been invoked")
	assert.NotContains(t, bodyStr, "DENY",
		"response must not contain any FGA outcome — FGAEngine must not have been invoked")
}

// Companion test: when the caller's org DOES match the agent's org,
// the gate passes through and the FGAEngine is invoked. This pins
// the legitimate path — a regression that hardens the gate too far
// (e.g., always returning 404) would be caught here.
func TestAuthorizeHandler_SameOrgPassesThroughToFGA(t *testing.T) {
	orgID := uuid.New()
	agentID := uuid.New()

	fake := &fakeAuthorizer{
		result: &application.FGAResult{
			Allowed: true,
			Outcome: "ALLOW",
		},
	}
	repo := &permissiveAuthorizeAgentRepo{orgID: orgID}
	h := NewAuthorizeHandlerWithInterface(fake, repo)

	app := newAuthorizeTestApp(h, orgID)
	body := strings.NewReader(`{"capability":"file:read"}`)
	req := httptest.NewRequest("POST", "/api/v1/agents/"+agentID.String()+"/authorize", body)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	require.NotNil(t, fake.gotRequest,
		"FGAEngine must be reached on the legitimate same-org path")
	assert.Equal(t, agentID, fake.gotRequest.AgentID)
}
