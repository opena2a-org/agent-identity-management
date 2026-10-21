package handlers

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
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

// GET /api/v1/agents/:id/feedback returns the counts and the average of the
// ratings users have given an agent. The server registers it bound to its
// agent (bindAgentRoutes), so the requests below pass through the
// authenticators the agents group mounts, in the order it mounts them, and
// then through middleware.AgentPathBinding, as bindAgentRoutes puts it in
// front of the handler. The API-key and ATC authenticators are left out: a
// request that carries neither credential passes them unchanged.

const userFeedbackPattern = "/api/v1/agents/:id/feedback"

type userFeedbackFixture struct {
	org     uuid.UUID
	caller  *domain.Agent // signs the requests, and is the subject of the service token
	sibling *domain.Agent // another agent of the caller's organization
	foreign *domain.Agent // an agent of another organization
	priv    ed25519.PrivateKey
	jwt     *auth.JWTService
	repo    *mocks.MockAgentRepository
	trust   *MockTrustCalculatorServicerImpl
	h       *TrustScoreHandler
	app     *fiber.App

	stats    *domain.UserFeedbackStats
	statsErr error
	reads    []uuid.UUID // the agent IDs whose feedback the handler read
}

func newUserFeedbackFixture(t *testing.T) *userFeedbackFixture {
	t.Helper()
	t.Setenv("JWT_SECRET", sessionAgeTestSecret)

	pub, priv, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	key := base64.StdEncoding.EncodeToString(pub)

	f := &userFeedbackFixture{org: uuid.New(), priv: priv, jwt: auth.NewJWTService()}
	f.caller = &domain.Agent{ID: uuid.New(), OrganizationID: f.org, Name: "support-agent", Status: domain.AgentStatusVerified, PublicKey: &key}
	f.sibling = &domain.Agent{ID: uuid.New(), OrganizationID: f.org, Name: "sibling-agent", Status: domain.AgentStatusVerified}
	f.foreign = &domain.Agent{ID: uuid.New(), OrganizationID: uuid.New(), Name: "foreign-agent", Status: domain.AgentStatusVerified}
	// Ratings of 5, 5, 5, 4, 4, 3, 2 and 1.
	f.stats = &domain.UserFeedbackStats{Total: 8, PositiveCount: 5, NeutralCount: 1, NegativeCount: 2, AverageRating: 3.625}

	f.repo = &mocks.MockAgentRepository{}
	for _, a := range []*domain.Agent{f.caller, f.sibling, f.foreign} {
		f.repo.On("GetByID", a.ID).Return(a, nil)
	}
	f.repo.On("GetByID", mock.Anything).Return(nil, errors.New("agent not found"))
	agents := application.NewAgentService(f.repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	f.trust = &MockTrustCalculatorServicerImpl{
		GetUserFeedbackStatsFunc: func(_ context.Context, agentID uuid.UUID) (*domain.UserFeedbackStats, error) {
			f.reads = append(f.reads, agentID)
			return f.stats, f.statsErr
		},
	}
	f.h = NewTrustScoreHandlerWithInterfaces(f.trust, agents, &MockAuditServiceImpl{})

	f.app = fiber.New()
	group := f.app.Group("/api/v1/agents")
	group.Use(middleware.PQCAgentMiddleware(agents))
	group.Use(middleware.ServicePrincipalMiddleware(f.jwt, f.repo))
	group.Use(middleware.AuthMiddleware(f.jwt))
	group.Get("/:id/feedback", middleware.AgentPathBinding("GET "+userFeedbackPattern, "id", nil), f.h.GetUserFeedback)
	return f
}

// signed sends GET target signed as the SDK signs it: the method, the path
// with its query, and the timestamp, joined by line feeds.
func (f *userFeedbackFixture) signed(t *testing.T, target string) (int, string) {
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
func (f *userFeedbackFixture) serviceToken(t *testing.T, target string) (int, string) {
	t.Helper()
	token, err := f.jwt.GenerateServiceToken(f.caller.ID.String(), f.org.String())
	require.NoError(t, err)
	req := httptest.NewRequest("GET", target, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	return agentAuditLogsDo(t, f.app, req)
}

// withLocals sends GET target to the route with the locals an authenticator
// that needs a session store or a database (a user's session, an API key)
// would set, followed by the binding and the handler, as the server mounts
// them.
func (f *userFeedbackFixture) withLocals(t *testing.T, target string, locals func(c fiber.Ctx)) (int, string) {
	t.Helper()
	app := fiber.New()
	app.Get(userFeedbackPattern, func(c fiber.Ctx) error {
		c.Locals("organization_id", f.org)
		locals(c)
		return c.Next()
	}, middleware.AgentPathBinding("GET "+userFeedbackPattern, "id", nil), f.h.GetUserFeedback)
	return agentAuditLogsDo(t, app, httptest.NewRequest("GET", target, nil))
}

func userSession(userID uuid.UUID) func(c fiber.Ctx) {
	return func(c fiber.Ctx) {
		c.Locals("user_id", userID)
		c.Locals("role", string(domain.RoleViewer))
	}
}

func userFeedbackTarget(agentID string) string {
	return "/api/v1/agents/" + agentID + "/feedback"
}

type userFeedbackSummary struct {
	AgentID       string   `json:"agentId"`
	AgentName     string   `json:"agentName"`
	Total         int      `json:"total"`
	PositiveCount int      `json:"positiveCount"`
	NeutralCount  int      `json:"neutralCount"`
	NegativeCount int      `json:"negativeCount"`
	AverageRating *float64 `json:"averageRating"`
}

// userFeedbackMembers is every member the summary has, so a member added to
// it (a comment, the user who rated) fails the test that reads it.
var userFeedbackMembers = []string{"agentId", "agentName", "total", "positiveCount", "neutralCount", "negativeCount", "averageRating"}

func requireUserFeedbackMembers(t *testing.T, body string) {
	t.Helper()
	var members map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(body), &members))
	got := make([]string, 0, len(members))
	for name := range members {
		got = append(got, name)
	}
	assert.ElementsMatch(t, userFeedbackMembers, got)
}

// An agent reads the summary of its own ratings with each credential the SDK
// holds, and the handler reads the caller's feedback and no other agent's.
func TestUserFeedback_AnAgentReadsItsOwnSummary(t *testing.T) {
	for _, cred := range []struct {
		name string
		send func(*userFeedbackFixture, *testing.T, string) (int, string)
	}{
		{"Ed25519 signature", (*userFeedbackFixture).signed},
		{"service token", (*userFeedbackFixture).serviceToken},
	} {
		t.Run(cred.name, func(t *testing.T) {
			f := newUserFeedbackFixture(t)

			status, body := cred.send(f, t, userFeedbackTarget(f.caller.ID.String()))
			require.Equal(t, fiber.StatusOK, status, body)
			requireUserFeedbackMembers(t, body)

			var got userFeedbackSummary
			require.NoError(t, json.Unmarshal([]byte(body), &got))
			assert.Equal(t, f.caller.ID.String(), got.AgentID)
			assert.Equal(t, "support-agent", got.AgentName)
			assert.Equal(t, 8, got.Total)
			assert.Equal(t, 5, got.PositiveCount)
			assert.Equal(t, 1, got.NeutralCount)
			assert.Equal(t, 2, got.NegativeCount)
			require.NotNil(t, got.AverageRating)
			assert.Equal(t, 3.625, *got.AverageRating)
			assert.Equal(t, []uuid.UUID{f.caller.ID}, f.reads)
		})
	}
}

// An agent credential naming any other agent ID, or a value that is not an
// ID, gets the refusal of a route bound to its agent, before the named agent
// or any feedback is read.
func TestUserFeedback_AnAgentNamingAnotherAgentIsRefused(t *testing.T) {
	for _, cred := range []struct {
		name string
		send func(*userFeedbackFixture, *testing.T, string) (int, string)
	}{
		{"Ed25519 signature", (*userFeedbackFixture).signed},
		{"service token", (*userFeedbackFixture).serviceToken},
		{"API key", func(f *userFeedbackFixture, t *testing.T, target string) (int, string) {
			return f.withLocals(t, target, func(c fiber.Ctx) {
				c.Locals("user_id", uuid.New())
				c.Locals("agent_id", f.caller.ID)
				c.Locals("auth_method", "api_key")
			})
		}},
	} {
		for _, target := range []struct {
			name string
			id   func(*userFeedbackFixture) string
		}{
			{"an agent of its organization", func(f *userFeedbackFixture) string { return f.sibling.ID.String() }},
			{"an agent of another organization", func(f *userFeedbackFixture) string { return f.foreign.ID.String() }},
			{"an ID that names no agent", func(*userFeedbackFixture) string { return uuid.New().String() }},
			{"a value that is not an ID", func(*userFeedbackFixture) string { return "support-agent" }},
		} {
			t.Run(cred.name+"/"+target.name, func(t *testing.T) {
				f := newUserFeedbackFixture(t)
				named := target.id(f)

				status, body := cred.send(f, t, userFeedbackTarget(named))
				assert.Equal(t, fiber.StatusForbidden, status, body)
				assert.JSONEq(t, agentPathMismatchBody, body)
				if id, err := uuid.Parse(named); err == nil {
					f.repo.AssertNotCalled(t, "GetByID", id)
				}
				assert.Empty(t, f.reads, "no feedback is read")
			})
		}
	}
}

// An API key reads the summary of the agent it authenticates as.
func TestUserFeedback_AnAPIKeyReadsItsAgentsSummary(t *testing.T) {
	f := newUserFeedbackFixture(t)

	status, body := f.withLocals(t, userFeedbackTarget(f.caller.ID.String()), func(c fiber.Ctx) {
		c.Locals("user_id", uuid.New())
		c.Locals("agent_id", f.caller.ID)
		c.Locals("auth_method", "api_key")
	})
	require.Equal(t, fiber.StatusOK, status, body)
	assert.Equal(t, []uuid.UUID{f.caller.ID}, f.reads)
}

// A user's session reads the summary of any agent of the user's organization,
// with a viewer's role, as it reads the agent's trust score.
func TestUserFeedback_AUserReadsAnAgentOfTheirOrganization(t *testing.T) {
	f := newUserFeedbackFixture(t)

	status, body := f.withLocals(t, userFeedbackTarget(f.sibling.ID.String()), userSession(uuid.New()))
	require.Equal(t, fiber.StatusOK, status, body)
	requireUserFeedbackMembers(t, body)

	var got userFeedbackSummary
	require.NoError(t, json.Unmarshal([]byte(body), &got))
	assert.Equal(t, f.sibling.ID.String(), got.AgentID)
	assert.Equal(t, "sibling-agent", got.AgentName)
	assert.Equal(t, 8, got.Total)
	assert.Equal(t, []uuid.UUID{f.sibling.ID}, f.reads)
}

// A user's session is answered 403 for an agent of another organization, 404
// for an ID that names no agent and 400 for a value that is not an ID, and in
// each case no feedback is read.
func TestUserFeedback_AUserIsRefusedWhatTheirOrganizationDoesNotHold(t *testing.T) {
	for _, tc := range []struct {
		name   string
		id     func(*userFeedbackFixture) string
		status int
		body   string
	}{
		{"an agent of another organization", func(f *userFeedbackFixture) string { return f.foreign.ID.String() },
			fiber.StatusForbidden, `{"error":"Access denied"}`},
		{"an ID that names no agent", func(*userFeedbackFixture) string { return uuid.New().String() },
			fiber.StatusNotFound, `{"error":"Agent not found"}`},
		{"a value that is not an ID", func(*userFeedbackFixture) string { return "support-agent" },
			fiber.StatusBadRequest, `{"error":"Invalid agent ID"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newUserFeedbackFixture(t)

			status, body := f.withLocals(t, userFeedbackTarget(tc.id(f)), userSession(uuid.New()))
			assert.Equal(t, tc.status, status, body)
			assert.JSONEq(t, tc.body, body)
			assert.Empty(t, f.reads, "no feedback is read")
		})
	}
}

// With no ratings the counts are 0 and there is no average: averageRating is
// null, not the 0 the stats query returns, which is not a rating.
func TestUserFeedback_AnAgentWithNoRatingsHasNoAverage(t *testing.T) {
	f := newUserFeedbackFixture(t)
	f.stats = &domain.UserFeedbackStats{}

	status, body := f.signed(t, userFeedbackTarget(f.caller.ID.String()))
	require.Equal(t, fiber.StatusOK, status, body)
	requireUserFeedbackMembers(t, body)

	var members map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(body), &members))
	assert.Equal(t, "null", string(members["averageRating"]))
	for _, count := range []string{"total", "positiveCount", "neutralCount", "negativeCount"} {
		assert.Equal(t, "0", string(members[count]), count)
	}
}

// A failed read is answered 500 with a fixed message that does not carry the
// error.
func TestUserFeedback_AFailedReadIs500(t *testing.T) {
	f := newUserFeedbackFixture(t)
	f.stats, f.statsErr = nil, errors.New("pq: relation user_feedback does not exist")

	status, body := f.signed(t, userFeedbackTarget(f.caller.ID.String()))
	assert.Equal(t, fiber.StatusInternalServerError, status, body)
	assert.JSONEq(t, `{"error":"Failed to read user feedback"}`, body)
}
