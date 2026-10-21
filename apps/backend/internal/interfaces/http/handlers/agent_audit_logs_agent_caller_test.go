package handlers

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/middleware"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/testutil/mocks"
)

// An agent reads its own audit log with the credentials the SDK holds: a
// request signed with its Ed25519 key, or its OAuth service token. Neither
// carries a user ID. The requests below pass through the authenticators the
// agents group mounts, in the order it mounts them, to the route as the server
// registers it (held, under AgentPathObservation). The API-key and ATC
// authenticators are left out: a request that carries neither credential
// passes them unchanged.

const agentAuditLogsPattern = "/api/v1/agents/:id/audit-logs"

// agentPathMismatchBody is the refusal a route bound to its agent gives,
// written out so this file states the contract on its own.
const agentPathMismatchBody = `{"error":"An agent may only act on its own agent ID on this route","reasonCode":"agent_path_mismatch"}`

type agentAuditLogsFixture struct {
	org     uuid.UUID
	caller  *domain.Agent // signs the requests, and is the subject of the service token
	sibling *domain.Agent // another agent of the caller's organization
	foreign *domain.Agent // an agent of another organization
	priv    ed25519.PrivateKey
	jwt     *auth.JWTService
	repo    *mocks.MockAgentRepository
	h       *AgentHandler
	app     *fiber.App

	// What the handler read from the audit service, and the actor of each
	// view it recorded there.
	reads      int
	readType   string
	readEntity uuid.UUID
	readLimit  int
	agentViews []uuid.UUID
	userViews  []uuid.UUID
}

func newAgentAuditLogsFixture(t *testing.T) *agentAuditLogsFixture {
	t.Helper()
	t.Setenv("JWT_SECRET", sessionAgeTestSecret)

	pub, priv, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	key := base64.StdEncoding.EncodeToString(pub)

	f := &agentAuditLogsFixture{org: uuid.New(), priv: priv, jwt: auth.NewJWTService()}
	f.caller = &domain.Agent{ID: uuid.New(), OrganizationID: f.org, Name: "billing-agent", Status: domain.AgentStatusVerified, PublicKey: &key}
	f.sibling = &domain.Agent{ID: uuid.New(), OrganizationID: f.org, Name: "sibling-agent", Status: domain.AgentStatusVerified}
	f.foreign = &domain.Agent{ID: uuid.New(), OrganizationID: uuid.New(), Name: "foreign-agent", Status: domain.AgentStatusVerified}

	f.repo = &mocks.MockAgentRepository{}
	for _, a := range []*domain.Agent{f.caller, f.sibling, f.foreign} {
		f.repo.On("GetByID", a.ID).Return(a, nil)
	}
	f.repo.On("GetByID", mock.Anything).Return(nil, errors.New("agent not found"))
	agents := application.NewAgentService(f.repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	audit := &MockAuditServiceImpl{
		GetAuditLogsFunc: func(_ context.Context, orgID uuid.UUID, _ string, entityType string, entityID *uuid.UUID, _ *uuid.UUID, _ *time.Time, _ *time.Time, limit int, _ int) ([]*domain.AuditLog, int, error) {
			f.reads++
			f.readType, f.readLimit = entityType, limit
			if entityID == nil {
				return []*domain.AuditLog{}, 0, nil
			}
			f.readEntity = *entityID
			return []*domain.AuditLog{canaryAuditLog(orgID, *entityID, entityType)}, 1, nil
		},
		LogAgentActionFunc: func(_ context.Context, _ uuid.UUID, agentID uuid.UUID, _ domain.AuditAction, _ string, _ uuid.UUID, _ string, _ string, _ map[string]interface{}) error {
			f.agentViews = append(f.agentViews, agentID)
			return nil
		},
		LogActionFunc: func(_ context.Context, _ uuid.UUID, userID uuid.UUID, _ domain.AuditAction, _ string, _ uuid.UUID, _ string, _ string, _ map[string]interface{}) error {
			f.userViews = append(f.userViews, userID)
			return nil
		},
	}
	f.h = NewAgentHandlerWithInterfaces(agents, nil, audit, nil, nil, nil, nil, nil, nil, nil, nil)

	f.app = fiber.New()
	group := f.app.Group("/api/v1/agents")
	group.Use(middleware.PQCAgentMiddleware(agents))
	group.Use(middleware.ServicePrincipalMiddleware(f.jwt, f.repo))
	group.Use(middleware.AuthMiddleware(f.jwt))
	group.Get("/:id/audit-logs", middleware.AgentPathObservation("GET "+agentAuditLogsPattern, "id"), f.h.GetAgentAuditLogs)
	return f
}

// signed sends GET target signed as the SDK signs it: the method, the path
// with its query, and the timestamp, joined by line feeds.
func (f *agentAuditLogsFixture) signed(t *testing.T, target string) (int, string) {
	t.Helper()
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	sig := ed25519.Sign(f.priv, []byte("GET\n"+target+"\n"+ts))
	req := httptest.NewRequest("GET", target, nil)
	req.Header.Set("X-Agent-ID", f.caller.ID.String())
	req.Header.Set("X-Timestamp", ts)
	req.Header.Set("X-Signature", base64.StdEncoding.EncodeToString(sig))
	req.Header.Set("X-Public-Key", *f.caller.PublicKey)
	return agentAuditLogsDo(t, f.app, req)
}

// serviceToken sends GET target with a service token minted for the caller.
func (f *agentAuditLogsFixture) serviceToken(t *testing.T, target string) (int, string) {
	t.Helper()
	token, err := f.jwt.GenerateServiceToken(f.caller.ID.String(), f.org.String())
	require.NoError(t, err)
	req := httptest.NewRequest("GET", target, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	return agentAuditLogsDo(t, f.app, req)
}

func agentAuditLogsDo(t *testing.T, app *fiber.App, req *http.Request) (int, string) {
	t.Helper()
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(body)
}

func agentAuditLogsTarget(agentID uuid.UUID) string {
	return "/api/v1/agents/" + agentID.String() + "/audit-logs"
}

type agentAuditLogsCredential struct {
	name string
	send func(*agentAuditLogsFixture, *testing.T, string) (int, string)
}

var agentAuditLogsCredentials = []agentAuditLogsCredential{
	{"Ed25519 signature", (*agentAuditLogsFixture).signed},
	{"service token", (*agentAuditLogsFixture).serviceToken},
}

// The agent gets its own audit log as every caller but an admin gets it, and
// its read is recorded with the agent as the actor.
func TestAgentAuditLogs_AnAgentReadsItsOwnLog(t *testing.T) {
	for _, cred := range agentAuditLogsCredentials {
		t.Run(cred.name, func(t *testing.T) {
			f := newAgentAuditLogsFixture(t)

			status, body := cred.send(f, t, agentAuditLogsTarget(f.caller.ID))
			require.Equal(t, fiber.StatusOK, status, body)

			var got struct {
				AgentID string                       `json:"agentId"`
				Logs    []map[string]json.RawMessage `json:"logs"`
				Total   int                          `json:"total"`
			}
			require.NoError(t, json.Unmarshal([]byte(body), &got))
			assert.Equal(t, f.caller.ID.String(), got.AgentID)
			assert.Equal(t, 1, got.Total)
			require.Len(t, got.Logs, 1)
			for _, member := range []string{"ipAddress", "userAgent", "metadata"} {
				assert.NotContains(t, got.Logs[0], member, "member %q must be absent, not empty", member)
			}
			for _, canary := range auditCanaries {
				assert.NotContains(t, body, canary)
			}

			assert.Equal(t, "agent", f.readType)
			assert.Equal(t, f.caller.ID, f.readEntity)
			assert.Equal(t, []uuid.UUID{f.caller.ID}, f.agentViews, "the read is recorded with the agent as its actor")
			assert.Empty(t, f.userViews)
		})
	}
}

// An agent naming any other agent ID gets the refusal of a route bound to its
// agent, before the named agent or any audit record is read, and nothing is
// written.
func TestAgentAuditLogs_AnAgentNamingAnotherAgentIsRefused(t *testing.T) {
	for _, cred := range agentAuditLogsCredentials {
		for _, target := range []struct {
			name string
			id   func(*agentAuditLogsFixture) uuid.UUID
		}{
			{"an agent of its organization", func(f *agentAuditLogsFixture) uuid.UUID { return f.sibling.ID }},
			{"an agent of another organization", func(f *agentAuditLogsFixture) uuid.UUID { return f.foreign.ID }},
			{"an ID that names no agent", func(*agentAuditLogsFixture) uuid.UUID { return uuid.New() }},
		} {
			t.Run(cred.name+"/"+target.name, func(t *testing.T) {
				f := newAgentAuditLogsFixture(t)
				named := target.id(f)

				status, body := cred.send(f, t, agentAuditLogsTarget(named))
				assert.Equal(t, fiber.StatusForbidden, status, body)
				assert.JSONEq(t, agentPathMismatchBody, body)
				f.repo.AssertNotCalled(t, "GetByID", named)
				assert.Zero(t, f.reads, "no audit record is read")
				assert.Empty(t, f.agentViews)
				assert.Empty(t, f.userViews)
			})
		}
	}
}

// The limit a signed request carries is verified with its path, and the
// handler reads it as it reads a user's: 50 without one, 100 at most.
func TestAgentAuditLogs_AnAgentsLimitIsReadAsAUsersIs(t *testing.T) {
	for _, tc := range []struct {
		query string
		limit int
	}{
		{"", 50},
		{"?limit=10", 10},
		{"?limit=500", 100},
	} {
		t.Run("query="+tc.query, func(t *testing.T) {
			f := newAgentAuditLogsFixture(t)

			status, body := f.signed(t, agentAuditLogsTarget(f.caller.ID)+tc.query)
			require.Equal(t, fiber.StatusOK, status, body)
			assert.Equal(t, tc.limit, f.readLimit)
		})
	}
}

// A user's session and an API key carry a user ID, and are served as before:
// an API key naming another agent of its organization is still answered while
// the route's binding is held, and each read is recorded with the user as its
// actor.
func TestAgentAuditLogs_AUserOrAnAPIKeyIsServedAsBefore(t *testing.T) {
	for _, tc := range []struct {
		name   string
		locals func(c fiber.Ctx, f *agentAuditLogsFixture, userID uuid.UUID)
	}{
		{"user session", func(c fiber.Ctx, _ *agentAuditLogsFixture, userID uuid.UUID) {
			c.Locals("user_id", userID)
			c.Locals("role", string(domain.RoleMember))
		}},
		{"API key", func(c fiber.Ctx, f *agentAuditLogsFixture, userID uuid.UUID) {
			c.Locals("user_id", userID)
			c.Locals("agent_id", f.caller.ID)
			c.Locals("auth_method", "api_key")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newAgentAuditLogsFixture(t)
			userID := uuid.New()

			// The authenticators' work, done in place: these principals need
			// a session store or a database the fixture does not have.
			app := fiber.New()
			observe := middleware.AgentPathObservation("GET "+agentAuditLogsPattern, "id")
			app.Get(agentAuditLogsPattern, func(c fiber.Ctx) error {
				c.Locals("organization_id", f.org)
				tc.locals(c, f, userID)
				return c.Next()
			}, observe, f.h.GetAgentAuditLogs)

			status, body := agentAuditLogsDo(t, app, httptest.NewRequest("GET", agentAuditLogsTarget(f.sibling.ID), nil))
			require.Equal(t, fiber.StatusOK, status, body)
			assert.Equal(t, f.sibling.ID, f.readEntity)
			assert.Equal(t, []uuid.UUID{userID}, f.userViews, "the read is recorded with the user as its actor")
			assert.Empty(t, f.agentViews)
		})
	}
}
