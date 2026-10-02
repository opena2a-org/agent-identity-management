package handlers

import (
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
)

// A refused POST /verification-events must not start drift detection: no
// trust score update and no alert, for any agent, in any organization.

func runtimeListBody(agentID uuid.UUID, servers string) string {
	body := `{"agentId":"` + agentID.String() + `","protocol":"A2A","verificationType":"identity",` +
		`"status":"success","initiatorType":"user"`
	if servers != "" {
		body += `,"currentMcpServers":[` + servers + `]`
	}
	return body + `}`
}

func driftAgent(org uuid.UUID, priorViolations int) *domain.Agent {
	return &domain.Agent{ID: uuid.New(), OrganizationID: org, DisplayName: "drift-agent", TrustScore: 0.91,
		TalksTo: []string{"registered-server"}, CapabilityViolationCount: priorViolations}
}

func TestCreateVerificationEvent_RuntimeListsStartNoDriftDetection(t *testing.T) {
	callerOrg := uuid.New()
	cases := []struct {
		name     string
		agentOrg uuid.UUID
		role     domain.UserRole
		prior    int
		servers  string
	}{
		{"member, other organization, list not registered", uuid.New(), domain.RoleMember, 0, `"unregistered-server"`},
		{"member, other organization, prior violations", uuid.New(), domain.RoleMember, 2, `"unregistered-server"`},
		{"member, other organization, no list", uuid.New(), domain.RoleMember, 0, ""},
		{"member, other organization, list equal to registration", uuid.New(), domain.RoleMember, 0, `"registered-server"`},
		{"member, own organization, list not registered", callerOrg, domain.RoleMember, 0, `"unregistered-server"`},
		{"manager, other organization, list not registered", uuid.New(), domain.RoleManager, 0, `"unregistered-server"`},
		{"admin, own organization, list not registered", callerOrg, domain.RoleAdmin, 0, `"unregistered-server"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			agent := driftAgent(tc.agentOrg, tc.prior)
			f := newCreateEventFixture(agent)

			res := postCreateEvent(t, memberGatedApp(f, callerOrg, string(tc.role)), runtimeListBody(agent.ID, tc.servers))

			assertCreateRefused(t, f, res, agent)
			assert.Equal(t, 0.91, agent.TrustScore)
		})
	}

	t.Run("viewer, other organization, list not registered", func(t *testing.T) {
		agent := driftAgent(uuid.New(), 0)
		f := newCreateEventFixture(agent)

		res := postCreateEvent(t, memberGatedApp(f, callerOrg, string(domain.RoleViewer)), runtimeListBody(agent.ID, `"unregistered-server"`))

		assert.Equal(t, fiber.StatusForbidden, res.status, "response: %s", res.raw)
		assert.Empty(t, f.events.created)
		assert.Empty(t, f.agents.scoreUpdates)
		assert.Empty(t, f.alerts.created)
		assert.Zero(t, f.agents.lookups)
	})
}

// An unknown agent and an agent of another organization get identical responses.
func TestCreateVerificationEvent_UnknownAndForeignAgentGetTheSameBody(t *testing.T) {
	callerOrg := uuid.New()
	foreign := driftAgent(uuid.New(), 0)

	fForeign := newCreateEventFixture(foreign)
	resForeign := postCreateEvent(t, memberGatedApp(fForeign, callerOrg, string(domain.RoleMember)), callerSuppliedOutcomeBody(foreign.ID))

	fUnknown := newCreateEventFixture()
	resUnknown := postCreateEvent(t, memberGatedApp(fUnknown, callerOrg, string(domain.RoleMember)), callerSuppliedOutcomeBody(uuid.New()))

	assert.Equal(t, resUnknown.status, resForeign.status)
	assert.Equal(t, resUnknown.raw, resForeign.raw)
}
