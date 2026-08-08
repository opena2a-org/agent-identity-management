package middleware

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

// A service token must stop working the moment its agent stops being permitted to
// authenticate — not at the next issuance.
//
// The OAuth token endpoint refuses to MINT a token for a revoked agent, but a token minted
// a moment before the revocation stays cryptographically valid for its full hour. Before
// this check, revoking an agent left every already-issued service token working for up to
// an hour against the account you had just revoked. The token endpoint cannot close that;
// only the use path can.
//
// The statuses are written as literals rather than derived from the domain constants, so
// this table states the contract independently of the code under test. "deactivated" is
// not a status the system has — it stands for any value a future migration or a hand-run
// UPDATE could put in the column, which agents.status permits because it carries no CHECK
// constraint. A deny-list implementation authenticates it; an allow-list denies it.
func TestServicePrincipalGatesOnAgentStatus(t *testing.T) {
	cases := []struct {
		status     string
		shouldAuth bool
		why        string
	}{
		{"verified", true, "the ordinary case — also proves the request is otherwise well-formed"},
		{"pending", true, "registration default; denying it would break enrollment"},
		{"revoked", false, "RevokeAgent writes this and nothing else denied the request"},
		{"suspended", false, "SuspendAgent writes this"},
		{"deactivated", false, "unrecognised value must fail closed, not fall through"},
	}

	for _, tc := range cases {
		t.Run(tc.status, func(t *testing.T) {
			t.Setenv("JWT_SECRET", "test-secret-at-least-32-characters-long!")
			svc := auth.NewJWTService()
			agentID, orgID := uuid.New(), uuid.New()

			token, err := svc.GenerateServiceToken(agentID.String(), orgID.String())
			require.NoError(t, err)

			reader := &stubAgentStatusReader{
				agent: &domain.Agent{ID: agentID, OrganizationID: orgID, Status: domain.AgentStatus(tc.status)},
			}

			app := fiber.New()
			app.Use(ServicePrincipalMiddleware(svc, reader))
			app.Get("/probe", func(c fiber.Ctx) error {
				return c.JSON(fiber.Map{"reached": true})
			})

			req := httptest.NewRequest("GET", "/probe", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			resp, err := app.Test(req)
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()

			require.True(t, reader.called,
				"the middleware never read the agent, so it cannot have gated on status")

			if tc.shouldAuth {
				require.Equal(t, fiber.StatusOK, resp.StatusCode, tc.why)
			} else {
				require.Equal(t, fiber.StatusUnauthorized, resp.StatusCode, tc.why)
			}
		})
	}
}

// A nil reader must fail closed. The alternative is a wiring mistake that silently
// disables the check while every request still succeeds.
func TestServicePrincipalFailsClosedWithoutAReader(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-at-least-32-characters-long!")
	svc := auth.NewJWTService()
	token, err := svc.GenerateServiceToken(uuid.New().String(), uuid.New().String())
	require.NoError(t, err)

	app := fiber.New()
	app.Use(ServicePrincipalMiddleware(svc, nil))
	app.Get("/probe", func(c fiber.Ctx) error { return c.JSON(fiber.Map{"reached": true}) })

	req := httptest.NewRequest("GET", "/probe", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, fiber.StatusUnauthorized, resp.StatusCode,
		"a middleware that cannot check status must refuse, not wave the request through")
}

type stubAgentStatusReader struct {
	agent  *domain.Agent
	err    error
	called bool
}

func (s *stubAgentStatusReader) GetByID(id uuid.UUID) (*domain.Agent, error) {
	s.called = true
	if s.err != nil {
		return nil, s.err
	}
	if s.agent == nil || s.agent.ID != id {
		return nil, fmt.Errorf("agent not found")
	}
	return s.agent, nil
}

// anyVerifiedAgentReader resolves every id to a verified agent.
//
// For tests whose subject is authorization (role gates, self-scoping) rather than status:
// it keeps the status check satisfied without pinning the test to any particular agent, so
// those tests keep measuring what they were written to measure. Status itself is covered by
// TestServicePrincipalGatesOnAgentStatus.
type anyVerifiedAgentReader struct{}

func (anyVerifiedAgentReader) GetByID(id uuid.UUID) (*domain.Agent, error) {
	return &domain.Agent{ID: id, Status: domain.AgentStatusVerified}, nil
}
