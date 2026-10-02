package application

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// CreateVerificationEvent compares the agent's organization with the event's
// before it reads any attribute of the agent and before it calls drift
// detection or the event repository.
//
// The service is built the way cmd/server/main.go builds it: the event
// repository, the agent repository, and a DriftDetectionService constructed
// from the same agent repository and an alert repository. No collaborator is
// nil, so the drift path (score update and alert) is measured in every case.

type ownershipEventRepo struct {
	domain.VerificationEventRepository
	created []*domain.VerificationEvent
}

func (r *ownershipEventRepo) Create(e *domain.VerificationEvent) error {
	r.created = append(r.created, e)
	return nil
}

type ownershipAgentRepo struct {
	domain.AgentRepository
	agents       map[uuid.UUID]*domain.Agent
	lookups      int
	scoreUpdates []float64
}

func (r *ownershipAgentRepo) GetByID(id uuid.UUID) (*domain.Agent, error) {
	r.lookups++
	if a, ok := r.agents[id]; ok {
		return a, nil
	}
	return nil, errors.New("sql: no rows in result set")
}

func (r *ownershipAgentRepo) UpdateTrustScore(id uuid.UUID, newScore float64) error {
	r.scoreUpdates = append(r.scoreUpdates, newScore)
	return nil
}

type ownershipAlertRepo struct {
	domain.AlertRepository
	created []*domain.Alert
}

func (r *ownershipAlertRepo) Create(a *domain.Alert) error {
	r.created = append(r.created, a)
	return nil
}

type ownershipFixture struct {
	events *ownershipEventRepo
	agents *ownershipAgentRepo
	alerts *ownershipAlertRepo
	svc    *VerificationEventService
}

func newOwnershipFixture(agents ...*domain.Agent) *ownershipFixture {
	byID := map[uuid.UUID]*domain.Agent{}
	for _, a := range agents {
		byID[a.ID] = a
	}
	f := &ownershipFixture{
		events: &ownershipEventRepo{},
		agents: &ownershipAgentRepo{agents: byID},
		alerts: &ownershipAlertRepo{},
	}
	drift := NewDriftDetectionService(f.agents, f.alerts)
	f.svc = NewVerificationEventService(f.events, f.agents, drift)
	return f
}

func registeredAgent(org uuid.UUID, priorViolations int) *domain.Agent {
	return &domain.Agent{
		ID:                       uuid.New(),
		OrganizationID:           org,
		DisplayName:              "registered-agent",
		TrustScore:               0.91,
		TalksTo:                  []string{"registered-server"},
		CapabilityViolationCount: priorViolations,
	}
}

func eventRequest(org, agentID uuid.UUID, servers []string) *CreateVerificationEventRequest {
	sig, key := "AAAA-not-a-signature", "BBBB-not-a-key"
	verified := domain.VerificationResultVerified
	return &CreateVerificationEventRequest{
		OrganizationID:    org,
		AgentID:           agentID,
		Protocol:          domain.VerificationProtocolA2A,
		VerificationType:  domain.VerificationTypeIdentity,
		Status:            domain.VerificationEventStatusSuccess,
		Result:            &verified,
		Signature:         &sig,
		PublicKey:         &key,
		Confidence:        1.0,
		InitiatorType:     domain.InitiatorTypeUser,
		CurrentMCPServers: servers,
	}
}

// assertNothingChanged: no row, no score update, no alert, one lookup only.
func assertNothingChanged(t *testing.T, f *ownershipFixture, event *domain.VerificationEvent, err error) {
	t.Helper()
	assert.Nil(t, event, "no event may be returned")
	assert.ErrorIs(t, err, ErrVerificationEventAgentNotFound)
	assert.Empty(t, f.events.created, "no verification-event row may be written")
	assert.Empty(t, f.agents.scoreUpdates, "no trust score may be updated")
	assert.Empty(t, f.alerts.created, "no alert may be written")
	assert.LessOrEqual(t, f.agents.lookups, 1, "only the ownership lookup may run")
}

func TestCreateVerificationEvent_AgentOfAnotherOrganization(t *testing.T) {
	callerOrg := uuid.New()
	cases := []struct {
		name    string
		prior   int
		servers []string
	}{
		{"runtime list not in the registration", 0, []string{"unregistered-server"}},
		{"no runtime list", 0, nil},
		{"runtime list equal to the registration", 0, []string{"registered-server"}},
		{"prior violations and a runtime list not in the registration", 3, []string{"unregistered-server"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			agent := registeredAgent(uuid.New(), tc.prior)
			f := newOwnershipFixture(agent)

			event, err := f.svc.CreateVerificationEvent(context.Background(), eventRequest(callerOrg, agent.ID, tc.servers))

			assertNothingChanged(t, f, event, err)
			assert.Equal(t, 0.91, agent.TrustScore)
		})
	}
}

// An unknown agent and an agent of another organization are refused with the
// same error, so the result does not reveal whether the id exists elsewhere.
func TestCreateVerificationEvent_UnknownAndForeignAgentAreIndistinguishable(t *testing.T) {
	callerOrg := uuid.New()
	foreign := registeredAgent(uuid.New(), 0)

	fForeign := newOwnershipFixture(foreign)
	_, errForeign := fForeign.svc.CreateVerificationEvent(context.Background(), eventRequest(callerOrg, foreign.ID, nil))

	fUnknown := newOwnershipFixture()
	event, errUnknown := fUnknown.svc.CreateVerificationEvent(context.Background(), eventRequest(callerOrg, uuid.New(), nil))
	assertNothingChanged(t, fUnknown, event, errUnknown)

	require.Error(t, errForeign)
	require.Error(t, errUnknown)
	assert.Equal(t, errUnknown.Error(), errForeign.Error())
	assert.NotContains(t, errForeign.Error(), foreign.DisplayName)
}

// Controls: an agent of the event's own organization is still recorded, and
// drift detection still runs for it on the service's in-process callers.
func TestCreateVerificationEvent_AgentOfOwnOrganization(t *testing.T) {
	t.Run("runtime list equal to the registration", func(t *testing.T) {
		org := uuid.New()
		agent := registeredAgent(org, 0)
		f := newOwnershipFixture(agent)

		event, err := f.svc.CreateVerificationEvent(context.Background(), eventRequest(org, agent.ID, []string{"registered-server"}))

		require.NoError(t, err)
		require.NotNil(t, event)
		assert.Len(t, f.events.created, 1)
		assert.Empty(t, f.agents.scoreUpdates)
		assert.Empty(t, f.alerts.created)
	})

	t.Run("no runtime list", func(t *testing.T) {
		org := uuid.New()
		agent := registeredAgent(org, 0)
		f := newOwnershipFixture(agent)

		_, err := f.svc.CreateVerificationEvent(context.Background(), eventRequest(org, agent.ID, nil))

		require.NoError(t, err)
		assert.Len(t, f.events.created, 1)
		assert.Empty(t, f.agents.scoreUpdates)
		assert.Empty(t, f.alerts.created)
	})
}
