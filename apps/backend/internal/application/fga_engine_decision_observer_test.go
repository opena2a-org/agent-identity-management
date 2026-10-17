package application

import (
	"context"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// observerAgentRepo reads one agent, so the decision names its organization.
type observerAgentRepo struct {
	domain.AgentRepository
	agent *domain.Agent
}

func (r *observerAgentRepo) GetByID(id uuid.UUID) (*domain.Agent, error) {
	if r.agent != nil && r.agent.ID == id {
		return r.agent, nil
	}
	return nil, assert.AnError
}

// observerEngine is an engine with no policies (db nil), so a decision is the
// capability check alone: the agent holds files:read and nothing else.
func observerEngine(t *testing.T) (*FGAEngine, *domain.Agent) {
	t.Helper()
	agent := &domain.Agent{ID: uuid.New(), OrganizationID: uuid.New()}
	agentSvc := &AgentService{
		capabilityRepo: &aim08CapabilityRepo{capability: "files:read"},
		agentRepo:      &observerAgentRepo{agent: agent},
	}
	return NewFGAEngine(nil, agentSvc, slog.Default()), agent
}

func TestFGAEngine_DecisionObserverReceivesEveryFinalDecision(t *testing.T) {
	engine, agent := observerEngine(t)
	var observed []FGADecision
	engine.SetDecisionObserver(func(d FGADecision) { observed = append(observed, d) })

	denied, err := engine.Authorize(context.Background(), &FGARequest{AgentID: agent.ID, Capability: "files:delete"})
	require.NoError(t, err)
	require.False(t, denied.Allowed)
	allowed, err := engine.Authorize(context.Background(), &FGARequest{AgentID: agent.ID, Capability: "files:read"})
	require.NoError(t, err)
	require.True(t, allowed.Allowed)

	require.Len(t, observed, 2, "a DENY and an ALLOW are both observed; the observer decides what to forward")
	deny := observed[0]
	assert.Equal(t, agent.ID, deny.AgentID)
	assert.Equal(t, agent.OrganizationID, deny.OrganizationID)
	assert.Equal(t, "files:delete", deny.Capability)
	assert.False(t, deny.Allowed)
	assert.Equal(t, "DENY", deny.Outcome)
	assert.Equal(t, denied.Outcome, deny.Outcome)
	assert.Equal(t, denied.DeniedBy, deny.DeniedBy)
	assert.Equal(t, denied.DeniedReason, deny.DeniedReason)
	assert.Equal(t, "capability_check", deny.DeniedBy)
	assert.False(t, deny.Time.IsZero())

	allow := observed[1]
	assert.True(t, allow.Allowed)
	assert.Equal(t, "ALLOW", allow.Outcome)
	assert.Equal(t, "files:read", allow.Capability)
	assert.Empty(t, allow.DeniedBy)
}

func TestFGAEngine_DecisionObserverSeesTheAnswerAuthorizeReturns(t *testing.T) {
	engine, agent := observerEngine(t)
	var observed FGADecision
	engine.SetDecisionObserver(func(d FGADecision) {
		observed = d
		// A change to the observer's copy is not a change to the decision.
		d.Allowed = true
		d.Outcome = "ALLOW"
	})

	result, err := engine.Authorize(context.Background(), &FGARequest{AgentID: agent.ID, Capability: "files:delete"})
	require.NoError(t, err)
	assert.False(t, result.Allowed)
	assert.Equal(t, "DENY", result.Outcome)
	assert.Equal(t, "DENY", observed.Outcome)
}

func TestFGAEngine_WithoutADecisionObserver(t *testing.T) {
	engine, agent := observerEngine(t)

	result, err := engine.Authorize(context.Background(), &FGARequest{AgentID: agent.ID, Capability: "files:delete"})
	require.NoError(t, err)
	assert.Equal(t, "DENY", result.Outcome)

	engine.SetDecisionObserver(nil)
	result, err = engine.Authorize(context.Background(), &FGARequest{AgentID: agent.ID, Capability: "files:read"})
	require.NoError(t, err)
	assert.Equal(t, "ALLOW", result.Outcome)
}

func TestFGAEngine_DecisionObserverWhenTheAgentIsNotRead(t *testing.T) {
	engine, _ := observerEngine(t)
	var observed []FGADecision
	engine.SetDecisionObserver(func(d FGADecision) { observed = append(observed, d) })

	unknown := uuid.New()
	_, err := engine.Authorize(context.Background(), &FGARequest{AgentID: unknown, Capability: "files:delete"})
	require.NoError(t, err)
	require.Len(t, observed, 1)
	assert.Equal(t, unknown, observed[0].AgentID)
	assert.Equal(t, uuid.Nil, observed[0].OrganizationID)
}
