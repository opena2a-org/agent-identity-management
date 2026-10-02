package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/handlers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type routeTestEventRepo struct {
	domain.VerificationEventRepository
	events  map[uuid.UUID]*domain.VerificationEvent
	created int
	deleted []uuid.UUID
}

func (r *routeTestEventRepo) Create(e *domain.VerificationEvent) error {
	r.created++
	return nil
}

func (r *routeTestEventRepo) GetByID(id uuid.UUID) (*domain.VerificationEvent, error) {
	if e, ok := r.events[id]; ok {
		return e, nil
	}
	return nil, errors.New("sql: no rows in result set")
}

func (r *routeTestEventRepo) Delete(id uuid.UUID) error {
	r.deleted = append(r.deleted, id)
	return nil
}

type routeTestAgentRepo struct {
	domain.AgentRepository
	agents       map[uuid.UUID]*domain.Agent
	lookups      int
	scoreUpdates []float64
}

func (r *routeTestAgentRepo) GetByID(id uuid.UUID) (*domain.Agent, error) {
	r.lookups++
	if a, ok := r.agents[id]; ok {
		return a, nil
	}
	return nil, errors.New("sql: no rows in result set")
}

func (r *routeTestAgentRepo) UpdateTrustScore(id uuid.UUID, newScore float64) error {
	r.scoreUpdates = append(r.scoreUpdates, newScore)
	return nil
}

type routeTestAlertRepo struct {
	domain.AlertRepository
	created int
}

func (r *routeTestAlertRepo) Create(a *domain.Alert) error {
	r.created++
	return nil
}

// routeTestMCPServerRepo is the handler's MCP server repository; the routes
// exercised here call none of its methods.
type routeTestMCPServerRepo struct {
	domain.MCPServerRepository
}

// routeFixture builds the handler the way setupRoutes receives it
// (cmd/server/main.go): the service with drift detection and its alert
// repository, the handler with the agent and MCP server repositories. No
// collaborator is nil, so a request that reached drift detection would be
// counted as a score update and an alert.
type routeFixture struct {
	app    *fiber.App
	jwt    *auth.JWTService
	events *routeTestEventRepo
	agents *routeTestAgentRepo
	alerts *routeTestAlertRepo
}

func newRouteFixture(t *testing.T, agents []*domain.Agent, events []*domain.VerificationEvent) *routeFixture {
	t.Helper()
	t.Setenv("JWT_SECRET", "test-secret-key-for-testing-purposes-32chars")
	f := &routeFixture{
		jwt:    auth.NewJWTService(),
		events: &routeTestEventRepo{events: map[uuid.UUID]*domain.VerificationEvent{}},
		agents: &routeTestAgentRepo{agents: map[uuid.UUID]*domain.Agent{}},
		alerts: &routeTestAlertRepo{},
	}
	for _, a := range agents {
		f.agents.agents[a.ID] = a
	}
	for _, e := range events {
		f.events.events[e.ID] = e
	}
	drift := application.NewDriftDetectionService(f.agents, f.alerts)
	svc := application.NewVerificationEventService(f.events, f.agents, drift)
	h := handlers.NewVerificationEventHandler(svc, f.agents, &routeTestMCPServerRepo{})

	f.app = fiber.New()
	mountVerificationEventRoutes(f.app.Group("/api/v1"), h, f.jwt)
	return f
}

func (f *routeFixture) token(t *testing.T, org uuid.UUID, role domain.UserRole) string {
	t.Helper()
	token, err := f.jwt.GenerateAccessToken(uuid.New().String(), org.String(), string(role)+"@example.com", string(role))
	require.NoError(t, err)
	return token
}

func (f *routeFixture) do(t *testing.T, method, path, token, body string) (int, string, map[string]interface{}) {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := f.app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	out := map[string]interface{}{}
	_ = json.Unmarshal(raw, &out)
	return resp.StatusCode, string(raw), out
}

// TestVerificationEventsPostIsRefusedForEveryRole mounts /verification-events
// through mountVerificationEventRoutes — the function setupRoutes calls — with
// the real AuthMiddleware, and sends POST with a real access token per role.
// The body carries a runtime list that is not registered for the agent, so a
// request that reached the service would start drift detection. Every role is
// refused, no row is written, no agent is read, no trust score is updated and
// no alert is written, including for an admin.
func TestVerificationEventsPostIsRefusedForEveryRole(t *testing.T) {
	org := uuid.New()
	ownAgent := &domain.Agent{ID: uuid.New(), OrganizationID: org, DisplayName: "own-org-agent", TrustScore: 0.42,
		TalksTo: []string{"registered-server"}}
	otherAgent := &domain.Agent{ID: uuid.New(), OrganizationID: uuid.New(), DisplayName: "other-org-agent", TrustScore: 0.91,
		TalksTo: []string{"registered-server"}}
	f := newRouteFixture(t, []*domain.Agent{ownAgent, otherAgent}, nil)

	body := func(agentID uuid.UUID) string {
		return `{"agentId":"` + agentID.String() + `","protocol":"A2A","verificationType":"identity",` +
			`"status":"success","result":"verified","signature":"AAAA-not-a-signature",` +
			`"publicKey":"BBBB-not-a-key","confidence":1.0,"initiatorType":"user",` +
			`"currentMcpServers":["unregistered-server"]}`
	}
	const path = "/api/v1/verification-events"

	t.Run("no token", func(t *testing.T) {
		status, raw, _ := f.do(t, http.MethodPost, path, "", body(ownAgent.ID))
		assert.Equal(t, http.StatusUnauthorized, status, raw)
	})

	for _, role := range []domain.UserRole{domain.RoleMember, domain.RoleManager, domain.RoleAdmin} {
		for _, target := range []*domain.Agent{ownAgent, otherAgent} {
			t.Run(string(role)+"/"+target.DisplayName, func(t *testing.T) {
				status, raw, out := f.do(t, http.MethodPost, path, f.token(t, org, role), body(target.ID))

				assert.Equal(t, http.StatusForbidden, status, raw)
				assert.Equal(t, "verificationEventWriteNotAccepted", out["code"], raw)
				assert.NotContains(t, raw, target.DisplayName)
				assert.NotContains(t, out, "trustScore")
				assert.NotContains(t, out, "agentName")
			})
		}
	}

	t.Run("viewer", func(t *testing.T) {
		status, raw, _ := f.do(t, http.MethodPost, path, f.token(t, org, domain.RoleViewer), body(ownAgent.ID))
		assert.Equal(t, http.StatusForbidden, status, raw)
		assert.NotContains(t, raw, ownAgent.DisplayName)
	})

	assert.Zero(t, f.events.created, "no verification-event row may be written through the mounted route")
	assert.Zero(t, f.agents.lookups, "the refusal must not read the agent")
	assert.Empty(t, f.agents.scoreUpdates, "no trust score may be updated through the mounted route")
	assert.Zero(t, f.alerts.created, "no alert may be written through the mounted route")
	assert.Equal(t, 0.42, ownAgent.TrustScore)
	assert.Equal(t, 0.91, otherAgent.TrustScore)
}

// TestVerificationEventsReadRoutesStayMounted pins that only the POST writer
// changed: the read routes and the org-scoped DELETE are still served, by the
// handler the server constructs, behind the middleware setupRoutes applies.
func TestVerificationEventsReadRoutesStayMounted(t *testing.T) {
	org := uuid.New()
	ownEvent := &domain.VerificationEvent{ID: uuid.New(), OrganizationID: org}
	otherEvent := &domain.VerificationEvent{ID: uuid.New(), OrganizationID: uuid.New()}
	f := newRouteFixture(t, nil, []*domain.VerificationEvent{ownEvent, otherEvent})

	mounted := map[string]bool{}
	for _, r := range f.app.GetRoutes(true) {
		mounted[r.Method+" "+r.Path] = true
	}
	for _, want := range []string{
		"GET /api/v1/verification-events/",
		"GET /api/v1/verification-events/recent",
		"GET /api/v1/verification-events/statistics",
		"GET /api/v1/verification-events/stats",
		"GET /api/v1/verification-events/agent/:id",
		"GET /api/v1/verification-events/mcp/:id",
		"GET /api/v1/verification-events/:id",
		"POST /api/v1/verification-events/",
		"DELETE /api/v1/verification-events/:id",
	} {
		assert.True(t, mounted[want], "%s is not mounted; mounted: %v", want, mounted)
	}

	eventPath := func(e *domain.VerificationEvent) string { return "/api/v1/verification-events/" + e.ID.String() }

	t.Run("GET /:id requires a token", func(t *testing.T) {
		status, raw, _ := f.do(t, http.MethodGet, eventPath(ownEvent), "", "")
		assert.Equal(t, http.StatusUnauthorized, status, raw)
	})

	t.Run("GET /:id returns an event of the caller's organization to a viewer", func(t *testing.T) {
		status, raw, out := f.do(t, http.MethodGet, eventPath(ownEvent), f.token(t, org, domain.RoleViewer), "")
		assert.Equal(t, http.StatusOK, status, raw)
		assert.Equal(t, ownEvent.ID.String(), out["id"], raw)
	})

	t.Run("GET /:id answers 404 for another organization's event", func(t *testing.T) {
		status, raw, _ := f.do(t, http.MethodGet, eventPath(otherEvent), f.token(t, org, domain.RoleAdmin), "")
		assert.Equal(t, http.StatusNotFound, status, raw)
		assert.NotContains(t, raw, otherEvent.OrganizationID.String())
	})

	t.Run("DELETE /:id is refused below manager", func(t *testing.T) {
		for _, role := range []domain.UserRole{domain.RoleViewer, domain.RoleMember} {
			status, raw, _ := f.do(t, http.MethodDelete, eventPath(ownEvent), f.token(t, org, role), "")
			assert.Equal(t, http.StatusForbidden, status, "%s: %s", role, raw)
		}
		assert.Empty(t, f.events.deleted)
	})

	t.Run("DELETE /:id answers 404 for another organization's event", func(t *testing.T) {
		status, raw, _ := f.do(t, http.MethodDelete, eventPath(otherEvent), f.token(t, org, domain.RoleAdmin), "")
		assert.Equal(t, http.StatusNotFound, status, raw)
		assert.Empty(t, f.events.deleted)
	})

	t.Run("DELETE /:id deletes an event of the caller's organization for a manager", func(t *testing.T) {
		status, raw, _ := f.do(t, http.MethodDelete, eventPath(ownEvent), f.token(t, org, domain.RoleManager), "")
		assert.Equal(t, http.StatusNoContent, status, raw)
		assert.Equal(t, []uuid.UUID{ownEvent.ID}, f.events.deleted)
	})
}
