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
	created int
}

func (r *routeTestEventRepo) Create(e *domain.VerificationEvent) error {
	r.created++
	return nil
}

type routeTestAgentRepo struct {
	domain.AgentRepository
	agents map[uuid.UUID]*domain.Agent
}

func (r *routeTestAgentRepo) GetByID(id uuid.UUID) (*domain.Agent, error) {
	if a, ok := r.agents[id]; ok {
		return a, nil
	}
	return nil, errors.New("sql: no rows in result set")
}

// TestVerificationEventsPostIsRefusedForEveryRole mounts /verification-events
// through mountVerificationEventRoutes — the function setupRoutes calls — with
// the real AuthMiddleware, and sends POST with a real access token per role.
// Every role is refused and no row is written, including for an admin.
func TestVerificationEventsPostIsRefusedForEveryRole(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-key-for-testing-purposes-32chars")
	jwtService := auth.NewJWTService()

	org := uuid.New()
	ownAgent := &domain.Agent{ID: uuid.New(), OrganizationID: org, DisplayName: "own-org-agent", TrustScore: 0.42}
	otherAgent := &domain.Agent{ID: uuid.New(), OrganizationID: uuid.New(), DisplayName: "other-org-agent", TrustScore: 0.91}
	events := &routeTestEventRepo{}
	agents := &routeTestAgentRepo{agents: map[uuid.UUID]*domain.Agent{ownAgent.ID: ownAgent, otherAgent.ID: otherAgent}}
	h := handlers.NewVerificationEventHandler(application.NewVerificationEventService(events, agents, nil), agents, nil)

	app := fiber.New()
	mountVerificationEventRoutes(app.Group("/api/v1"), h, jwtService)

	post := func(t *testing.T, token string, agentID uuid.UUID) (int, string, map[string]interface{}) {
		t.Helper()
		body := `{"agentId":"` + agentID.String() + `","protocol":"A2A","verificationType":"identity",` +
			`"status":"success","result":"verified","signature":"AAAA-not-a-signature",` +
			`"publicKey":"BBBB-not-a-key","confidence":1.0,"initiatorType":"user"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/verification-events", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := app.Test(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		raw, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		out := map[string]interface{}{}
		_ = json.Unmarshal(raw, &out)
		return resp.StatusCode, string(raw), out
	}

	t.Run("no token", func(t *testing.T) {
		status, raw, _ := post(t, "", ownAgent.ID)
		assert.Equal(t, http.StatusUnauthorized, status, raw)
	})

	for _, role := range []domain.UserRole{domain.RoleMember, domain.RoleManager, domain.RoleAdmin} {
		for _, target := range []*domain.Agent{ownAgent, otherAgent} {
			t.Run(string(role)+"/"+target.DisplayName, func(t *testing.T) {
				token, err := jwtService.GenerateAccessToken(uuid.New().String(), org.String(), string(role)+"@example.com", string(role))
				require.NoError(t, err)

				status, raw, out := post(t, token, target.ID)

				assert.Equal(t, http.StatusForbidden, status, raw)
				assert.Equal(t, "verificationEventWriteNotAccepted", out["code"], raw)
				assert.NotContains(t, raw, target.DisplayName)
				assert.NotContains(t, out, "trustScore")
				assert.NotContains(t, out, "agentName")
			})
		}
	}

	t.Run("viewer", func(t *testing.T) {
		token, err := jwtService.GenerateAccessToken(uuid.New().String(), org.String(), "viewer@example.com", string(domain.RoleViewer))
		require.NoError(t, err)
		status, raw, _ := post(t, token, ownAgent.ID)
		assert.Equal(t, http.StatusForbidden, status, raw)
		assert.NotContains(t, raw, ownAgent.DisplayName)
	})

	assert.Zero(t, events.created, "no verification-event row may be written through the mounted route")
}

// TestVerificationEventsReadRoutesStayMounted pins that only the POST writer
// changed: the read routes and the org-scoped DELETE are still served.
func TestVerificationEventsReadRoutesStayMounted(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-key-for-testing-purposes-32chars")
	app := fiber.New()
	mountVerificationEventRoutes(app.Group("/api/v1"), &handlers.VerificationEventHandler{}, auth.NewJWTService())

	mounted := map[string]bool{}
	for _, r := range app.GetRoutes(true) {
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
}
