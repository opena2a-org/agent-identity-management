package main

import (
	"context"
	"errors"
	"go/ast"
	"go/token"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/integrations/siem/hectest"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/handlers"
)

// main hands the SIEM forwarder, the services and the handlers to
// observeSecurityDecisions once the handlers exist, and only when a SIEM is
// configured. Without the call, failed sign-ins and FGA denials do not reach
// the SIEM.
func TestMain_ObservesSecurityDecisionsForTheSIEM(t *testing.T) {
	main := funcDecl(t, "main.go", "main")

	var handlersBuilt token.Pos
	ast.Inspect(main.Body, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok || len(assign.Lhs) != 1 || identName(assign.Lhs[0]) != "h" || len(assign.Rhs) != 1 {
			return true
		}
		if call, ok := assign.Rhs[0].(*ast.CallExpr); ok && identName(call.Fun) == "initHandlers" {
			handlersBuilt = assign.Pos()
		}
		return true
	})
	require.True(t, handlersBuilt.IsValid(), "main assigns h := initHandlers(...)")

	calls := callsIn(main.Body, "observeSecurityDecisions")
	require.Len(t, calls, 1, "main calls observeSecurityDecisions once")
	call := calls[0]
	require.Len(t, call.Args, 3)
	assert.Equal(t, "siemForwarder", identName(call.Args[0]))
	assert.Equal(t, "services", identName(call.Args[1]))
	assert.Equal(t, "h", identName(call.Args[2]))
	assert.Greater(t, call.Pos(), handlersBuilt, "the handlers exist before their observers are set")

	var guarded bool
	ast.Inspect(main.Body, func(n ast.Node) bool {
		ifStmt, ok := n.(*ast.IfStmt)
		if !ok {
			return true
		}
		cond, ok := ifStmt.Cond.(*ast.BinaryExpr)
		if ok && cond.Op == token.NEQ && identName(cond.X) == "siemForwarder" && identName(cond.Y) == "nil" &&
			ifStmt.Body.Pos() < call.Pos() && call.End() <= ifStmt.Body.End() {
			guarded = true
		}
		return true
	})
	assert.True(t, guarded, "observeSecurityDecisions is called inside if siemForwarder != nil")
}

type siemAgentRepo struct {
	domain.AgentRepository
	agent *domain.Agent
}

func (r *siemAgentRepo) GetByID(id uuid.UUID) (*domain.Agent, error) {
	if r.agent.ID == id {
		return r.agent, nil
	}
	return nil, errors.New("agent not found")
}

type siemCapabilityRepo struct {
	domain.CapabilityRepository
	granted string
}

func (r *siemCapabilityRepo) GetActiveCapabilitiesByAgentID(agentID uuid.UUID) ([]*domain.AgentCapability, error) {
	return []*domain.AgentCapability{{AgentID: agentID, CapabilityType: r.granted}}, nil
}

type siemNoUsers struct{ domain.UserRepository }

func (siemNoUsers) GetByEmail(string) (*domain.User, error) { return nil, errors.New("user not found") }

type siemNoRegistrations struct {
	application.RegistrationRepository
}

func (siemNoRegistrations) GetRegistrationRequestByEmailAnyStatus(context.Context, string) (*domain.UserRegistrationRequest, error) {
	return nil, errors.New("not found")
}

// The FGA denial and the refused sign-in on each password route reach the
// SIEM; the allowed decision does not.
func TestObserveSecurityDecisions_ForwardsDenialsAndRefusedSignIns(t *testing.T) {
	hecToken := uuid.NewString()
	receiver := hectest.NewReceiver(hecToken)
	t.Cleanup(receiver.Close)
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	forwarder, err := startSIEMForwarding(repository.NewAuditLogRepository(db), siemEnv(map[string]string{
		"SIEM_SPLUNK_HEC_URL":   receiver.URL(),
		"SIEM_SPLUNK_HEC_TOKEN": hecToken,
	}))
	require.NoError(t, err)

	agent := &domain.Agent{ID: uuid.New(), OrganizationID: uuid.New()}
	agentService := application.NewAgentService(&siemAgentRepo{agent: agent}, nil, nil, nil, nil, nil,
		&siemCapabilityRepo{granted: "files:read"}, nil, nil, nil, nil, nil)
	fga := application.NewFGAEngine(nil, agentService, nil)
	authService := application.NewAuthService(siemNoUsers{}, nil, nil, nil, nil, nil)
	h := &Handlers{
		Auth: handlers.NewAuthHandler(authService, nil, nil, nil, nil),
		PublicRegistration: handlers.NewPublicRegistrationHandler(
			application.NewRegistrationService(siemNoRegistrations{}, siemNoUsers{}, nil, nil, nil), authService, nil, nil),
	}
	observeSecurityDecisions(forwarder, &Services{FGA: fga}, h)

	allowed, err := fga.Authorize(context.Background(), &application.FGARequest{AgentID: agent.ID, Capability: "files:read"})
	require.NoError(t, err)
	require.True(t, allowed.Allowed)
	denied, err := fga.Authorize(context.Background(), &application.FGARequest{AgentID: agent.ID, Capability: "files:delete"})
	require.NoError(t, err)
	require.False(t, denied.Allowed)

	app := fiber.New()
	app.Post("/api/v1/public/login", h.PublicRegistration.Login)
	app.Post("/api/v1/auth/login/local", h.Auth.LocalLogin)
	for _, route := range []string{"/api/v1/public/login", "/api/v1/auth/login/local"} {
		req := httptest.NewRequest(fiber.MethodPost, route, strings.NewReader(`{"email":"nobody@example.invalid","password":"Not-The-Passw0rd"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second, FailOnTimeout: true})
		require.NoError(t, err)
		_ = resp.Body.Close()
		require.Equal(t, fiber.StatusUnauthorized, resp.StatusCode, route)
	}

	flushSIEMForwarding(forwarder)

	assert.Empty(t, receiver.Refusals())
	accepted := receiver.Accepted()
	require.Len(t, accepted, 3)
	routes := map[string]string{}
	var deniedEvents int
	for _, e := range accepted {
		assert.NotContains(t, string(e.Event), "nobody@example.invalid")
		assert.NotContains(t, string(e.Event), "Not-The-Passw0rd")
		var event map[string]interface{}
		require.NoError(t, e.Decode(&event))
		details := event["details"].(map[string]interface{})
		switch event["eventType"] {
		case "aim.fga.denied":
			deniedEvents++
			assert.Equal(t, agent.ID.String(), event["agentId"])
			assert.Equal(t, "files:delete", event["capability"])
			assert.Equal(t, "DENY", event["outcome"])
			assert.Equal(t, "medium", event["severity"])
			assert.Equal(t, agent.OrganizationID.String(), details["organizationId"])
			assert.Equal(t, "capability_check", details["deniedBy"])
		case "aim.auth.signin_failed":
			assert.Equal(t, "low", event["severity"])
			assert.NotContains(t, details, "userId")
			routes[details["route"].(string)] = details["reason"].(string)
		default:
			t.Errorf("unexpected event type %v", event["eventType"])
		}
	}
	assert.Equal(t, 1, deniedEvents)
	assert.Equal(t, map[string]string{
		"/api/v1/public/login":     "email_or_password_mismatch",
		"/api/v1/auth/login/local": "unknown_email",
	}, routes)
}
