package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// POST /verification-events refuses every caller until event outcomes are
// derived by the server. These tests send the request body a caller could
// previously use to record an outcome it chose itself (status "success",
// result "verified", a signature and public key nobody checked) and assert the
// refusal: 403 with code verificationEventWriteNotAccepted, no row written, no
// agent looked up, and no attribute of any agent in the response.

// recordingEventRepo records every Create; nothing else is expected to run.
type recordingEventRepo struct {
	domain.VerificationEventRepository
	created []*domain.VerificationEvent
}

func (r *recordingEventRepo) Create(e *domain.VerificationEvent) error {
	e.ID = uuid.New()
	r.created = append(r.created, e)
	return nil
}

// recordingAgentRepo serves a fixed set of agents and counts lookups.
type recordingAgentRepo struct {
	domain.AgentRepository
	agents  map[uuid.UUID]*domain.Agent
	lookups int
}

func (r *recordingAgentRepo) GetByID(id uuid.UUID) (*domain.Agent, error) {
	r.lookups++
	if a, ok := r.agents[id]; ok {
		return a, nil
	}
	return nil, errors.New("sql: no rows in result set")
}

type createEventFixture struct {
	events *recordingEventRepo
	agents *recordingAgentRepo
	h      *VerificationEventHandler
}

func newCreateEventFixture(agents ...*domain.Agent) *createEventFixture {
	byID := map[uuid.UUID]*domain.Agent{}
	for _, a := range agents {
		byID[a.ID] = a
	}
	events := &recordingEventRepo{}
	agentRepo := &recordingAgentRepo{agents: byID}
	svc := application.NewVerificationEventService(events, agentRepo, nil)
	return &createEventFixture{events: events, agents: agentRepo, h: NewVerificationEventHandler(svc, agentRepo, nil)}
}

// callerContext stands in for AuthMiddleware: organization, user and role are
// everything it contributes to this route.
func callerContext(orgID uuid.UUID, role string) fiber.Handler {
	return func(c fiber.Ctx) error {
		c.Locals("organization_id", orgID)
		c.Locals("user_id", uuid.New())
		if role != "" {
			c.Locals("role", role)
		}
		return c.Next()
	}
}

func callerSuppliedOutcomeBody(agentID uuid.UUID) string {
	return `{"agentId":"` + agentID.String() + `","protocol":"A2A","verificationType":"identity",` +
		`"status":"success","result":"verified","signature":"AAAA-not-a-signature",` +
		`"publicKey":"BBBB-not-a-key","confidence":1.0,"initiatorType":"user"}`
}

type createEventResult struct {
	status int
	raw    string
	body   map[string]interface{}
}

func postCreateEvent(t *testing.T, app *fiber.App, body string) createEventResult {
	t.Helper()
	req := httptest.NewRequest("POST", "/api/v1/verification-events", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	out := map[string]interface{}{}
	_ = json.Unmarshal(raw, &out)
	return createEventResult{status: resp.StatusCode, raw: string(raw), body: out}
}

// assertCreateRefused is the safe outcome every test below requires.
func assertCreateRefused(t *testing.T, f *createEventFixture, res createEventResult, agents ...*domain.Agent) {
	t.Helper()
	assert.Equal(t, fiber.StatusForbidden, res.status, "response: %s", res.raw)
	assert.Equal(t, "verificationEventWriteNotAccepted", res.body["code"], "response: %s", res.raw)
	assert.Empty(t, f.events.created, "no verification-event row may be written")
	assert.Zero(t, f.agents.lookups, "the refusal must not depend on, or reveal, whether the agent exists")
	for _, key := range []string{"id", "agentId", "agentName", "trustScore", "organizationId", "status", "result", "signature", "publicKey", "confidence"} {
		assert.NotContains(t, res.body, key, "response must not carry %q: %s", key, res.raw)
	}
	for _, a := range agents {
		assert.NotContains(t, res.raw, a.DisplayName, "response must not carry the agent's display name")
	}
}

// --- The handler, called with an organization and no role gate in front ---

func handlerOnlyApp(f *createEventFixture, orgID uuid.UUID, role string) *fiber.App {
	app := fiber.New()
	app.Post("/api/v1/verification-events", callerContext(orgID, role), f.h.CreateVerificationEvent)
	return app
}

func TestCreateVerificationEvent_RefusesCallerSuppliedOutcomeForOwnOrganizationsAgent(t *testing.T) {
	org := uuid.New()
	agent := &domain.Agent{ID: uuid.New(), OrganizationID: org, DisplayName: "own-org-agent", TrustScore: 0.42}
	f := newCreateEventFixture(agent)

	res := postCreateEvent(t, handlerOnlyApp(f, org, ""), callerSuppliedOutcomeBody(agent.ID))

	assertCreateRefused(t, f, res, agent)
}

func TestCreateVerificationEvent_RefusesUnknownAgent(t *testing.T) {
	f := newCreateEventFixture()

	res := postCreateEvent(t, handlerOnlyApp(f, uuid.New(), ""), callerSuppliedOutcomeBody(uuid.New()))

	assertCreateRefused(t, f, res)
}

func TestCreateVerificationEvent_RefusesAgentOfAnotherOrganization(t *testing.T) {
	callerOrg, otherOrg := uuid.New(), uuid.New()
	agent := &domain.Agent{ID: uuid.New(), OrganizationID: otherOrg, DisplayName: "other-org-agent", TrustScore: 0.91}
	f := newCreateEventFixture(agent)

	res := postCreateEvent(t, handlerOnlyApp(f, callerOrg, ""), callerSuppliedOutcomeBody(agent.ID))

	assertCreateRefused(t, f, res, agent)
	assert.NotContains(t, res.raw, "0.91", "response must not carry the other organization's trust score")
}

// --- Mounted as setupRoutes mounts it: role gate, then the handler ---

func memberGatedApp(f *createEventFixture, orgID uuid.UUID, role string) *fiber.App {
	app := fiber.New()
	app.Post("/api/v1/verification-events", callerContext(orgID, role), middleware.MemberMiddleware(), f.h.CreateVerificationEvent)
	return app
}

func TestCreateVerificationEvent_MemberIsRefusedForOwnOrganizationsAgent(t *testing.T) {
	org := uuid.New()
	agent := &domain.Agent{ID: uuid.New(), OrganizationID: org, DisplayName: "own-org-agent", TrustScore: 0.42}
	f := newCreateEventFixture(agent)

	res := postCreateEvent(t, memberGatedApp(f, org, string(domain.RoleMember)), callerSuppliedOutcomeBody(agent.ID))

	assertCreateRefused(t, f, res, agent)
}

// A viewer is stopped by the role gate. The handler must refuse a viewer on its
// own as well, so the refusal does not depend on which gate a mount carries.
func TestCreateVerificationEvent_ViewerIsRefused(t *testing.T) {
	org := uuid.New()
	agent := &domain.Agent{ID: uuid.New(), OrganizationID: org, DisplayName: "own-org-agent", TrustScore: 0.42}

	t.Run("behind the member gate", func(t *testing.T) {
		f := newCreateEventFixture(agent)
		res := postCreateEvent(t, memberGatedApp(f, org, string(domain.RoleViewer)), callerSuppliedOutcomeBody(agent.ID))
		assert.Equal(t, fiber.StatusForbidden, res.status, "response: %s", res.raw)
		assert.Empty(t, f.events.created)
		assert.Zero(t, f.agents.lookups)
	})

	t.Run("handler without a gate", func(t *testing.T) {
		f := newCreateEventFixture(agent)
		res := postCreateEvent(t, handlerOnlyApp(f, org, string(domain.RoleViewer)), callerSuppliedOutcomeBody(agent.ID))
		assertCreateRefused(t, f, res, agent)
	})
}

func TestCreateVerificationEvent_MemberIsRefusedForAgentOfAnotherOrganization(t *testing.T) {
	callerOrg, otherOrg := uuid.New(), uuid.New()
	agent := &domain.Agent{ID: uuid.New(), OrganizationID: otherOrg, DisplayName: "other-org-agent", TrustScore: 0.91}
	f := newCreateEventFixture(agent)

	res := postCreateEvent(t, memberGatedApp(f, callerOrg, string(domain.RoleMember)), callerSuppliedOutcomeBody(agent.ID))

	assertCreateRefused(t, f, res, agent)
	assert.NotContains(t, res.raw, "0.91", "response must not carry the other organization's trust score")
}

func TestCreateVerificationEvent_MemberIsRefusedForUnknownAgent(t *testing.T) {
	f := newCreateEventFixture()

	res := postCreateEvent(t, memberGatedApp(f, uuid.New(), string(domain.RoleMember)), callerSuppliedOutcomeBody(uuid.New()))

	assertCreateRefused(t, f, res)
}
