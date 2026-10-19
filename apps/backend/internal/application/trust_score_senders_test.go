package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record/transition"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// The trust calculator, the drift detection service and the capability
// service each store trust scores of their own. These tests pin that each
// sends trust_score.changed when the score it stored differs, at the two
// decimal places agents.trust_score keeps, from the agent's score before, and
// sends nothing otherwise.

// assertTrustScoreChangedSent takes the one event the dispatcher received and
// checks that it is trust_score.changed for agent, from previous to current.
func assertTrustScoreChangedSent(t *testing.T, d *recordingDispatcher, agent *domain.Agent, previous, current float64) {
	t.Helper()
	got := receiveEvents(t, d, 1)
	changed, ok := got[domain.WebhookEventTrustScoreChanged]
	require.True(t, ok, "trust_score.changed is sent, got %v", eventNames(got))
	assert.Equal(t, agent.OrganizationID, changed.orgID)
	assert.Equal(t, agent.ID.String(), changed.data["agentId"])
	assert.Equal(t, agent.Name, changed.data["agentName"])
	assert.Equal(t, previous, changed.data["previousScore"])
	assert.Equal(t, current, changed.data["trustScore"])
	assert.NoError(t, changed.ctxErr, "the event is sent with a context that has not ended")
}

// calculatorAgent is an agent whose stored score is storedScore, with enough
// history for the calculation to give a score far above 0.01.
func calculatorAgent(storedScore float64) *domain.Agent {
	return &domain.Agent{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		Name:           "calculated-agent",
		Description:    "An agent whose score the trust calculator stores.",
		Version:        "1.0.0",
		Status:         domain.AgentStatusVerified,
		TrustScore:     storedScore,
		CreatedAt:      time.Now().Add(-200 * 24 * time.Hour),
		UpdatedAt:      time.Now().Add(-15 * 24 * time.Hour),
	}
}

// scoringTrustCalculator returns a trust calculator over mocked repositories
// that read agent and record nothing about it, sending to the returned
// dispatcher. The two writes of CalculateTrustScore are left to the caller.
func scoringTrustCalculator(agent *domain.Agent) (*TrustCalculator, *TrustCalcMockAgentRepository, *AgentServiceMockTrustScoreRepository, *recordingDispatcher) {
	agents := new(TrustCalcMockAgentRepository)
	scores := new(AgentServiceMockTrustScoreRepository)
	capabilities := new(MockCapabilityRepository)
	alerts := new(TrustCalcMockAlertRepository)

	agents.On("GetByID", agent.ID).Return(agent, nil)
	capabilities.On("GetActiveCapabilitiesByAgentID", agent.ID).Return([]*domain.AgentCapability{}, nil).Maybe()
	capabilities.On("GetViolationsByAgentID", agent.ID, 500, 0).Return([]*domain.CapabilityViolation{}, 0, nil).Maybe()
	alerts.On("GetUnacknowledgedByResourceID", agent.ID).Return([]*domain.Alert{}, nil).Maybe()
	alerts.On("GetByResourceID", agent.ID, 100, 0).Return([]*domain.Alert{}, nil).Maybe()

	calculator := NewTrustCalculator(scores, new(MockAPIKeyRepository), new(AgentServiceMockAuditLogRepository),
		capabilities, agents, alerts)
	dispatcher := newRecordingDispatcher()
	calculator.webhooks = dispatcher
	return calculator, agents, scores, dispatcher
}

// A calculation that stores a score other than the agent's is sent as
// trust_score.changed, with both scores at the stored precision.
func TestTrustCalculator_CalculateTrustScore_SendsTrustScoreChanged(t *testing.T) {
	agent := calculatorAgent(0.01)
	calculator, agents, scores, dispatcher := scoringTrustCalculator(agent)
	scores.On("Create", mock.AnythingOfType("*domain.TrustScore")).Return(nil)
	agents.On("UpdateTrustScore", agent.ID, mock.AnythingOfType("float64")).Return(nil)

	score, err := calculator.CalculateTrustScore(context.Background(), agent.ID)
	require.NoError(t, err)
	require.NotEqual(t, 0.01, storedTrustScore(score.Score), "the fixture must calculate a different score")

	assertTrustScoreChangedSent(t, dispatcher, agent, 0.01, storedTrustScore(score.Score))
	agents.AssertCalled(t, "UpdateTrustScore", agent.ID, score.Score)
}

// A calculation that lands on the agent's stored score sends nothing, and so
// does one whose score is not stored.
func TestTrustCalculator_CalculateTrustScore_SendsNothingForAnUnchangedOrUnstoredScore(t *testing.T) {
	t.Run("unchanged at the stored precision", func(t *testing.T) {
		agent := calculatorAgent(0.01)
		calculator, agents, scores, dispatcher := scoringTrustCalculator(agent)
		calculated, err := calculator.Calculate(agent)
		require.NoError(t, err)
		agent.TrustScore = storedTrustScore(calculated.Score)
		scores.On("Create", mock.AnythingOfType("*domain.TrustScore")).Return(nil)
		agents.On("UpdateTrustScore", agent.ID, mock.AnythingOfType("float64")).Return(nil)

		score, err := calculator.CalculateTrustScore(context.Background(), agent.ID)
		require.NoError(t, err)
		require.Equal(t, agent.TrustScore, storedTrustScore(score.Score))
		dispatcher.assertNoMore(t)
	})

	t.Run("history row not written", func(t *testing.T) {
		agent := calculatorAgent(0.01)
		calculator, agents, scores, dispatcher := scoringTrustCalculator(agent)
		scores.On("Create", mock.AnythingOfType("*domain.TrustScore")).Return(assert.AnError)

		_, err := calculator.CalculateTrustScore(context.Background(), agent.ID)
		require.Error(t, err)
		agents.AssertNotCalled(t, "UpdateTrustScore", mock.Anything, mock.Anything)
		dispatcher.assertNoMore(t)
	})

	t.Run("agent row not written", func(t *testing.T) {
		agent := calculatorAgent(0.01)
		calculator, agents, scores, dispatcher := scoringTrustCalculator(agent)
		scores.On("Create", mock.AnythingOfType("*domain.TrustScore")).Return(nil)
		agents.On("UpdateTrustScore", agent.ID, mock.AnythingOfType("float64")).Return(assert.AnError)

		_, err := calculator.CalculateTrustScore(context.Background(), agent.ID)
		require.Error(t, err)
		dispatcher.assertNoMore(t)
	})
}

// driftingAgent is an agent registered to talk to one MCP server, holding
// score on the scale the existing drift tests use.
func driftingAgent(score float64) *domain.Agent {
	return &domain.Agent{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		Name:           "drifting-agent",
		TalksTo:        []string{"filesystem-mcp"},
		TrustScore:     score,
	}
}

// The penalty DetectDrift applies for an unregistered MCP server is sent as
// trust_score.changed once it is stored.
func TestDriftDetectionService_DriftPenalty_SendsTrustScoreChanged(t *testing.T) {
	agent := driftingAgent(85.0)
	agents := new(MockAgentRepository)
	alerts := new(MockAlertRepository)
	agents.On("GetByID", agent.ID).Return(agent, nil)
	alerts.On("Create", mock.AnythingOfType("*domain.Alert")).Return(nil)
	agents.On("UpdateTrustScore", agent.ID, 85.0-FirstViolationPenalty).Return(nil)
	service := NewDriftDetectionService(agents, alerts)
	dispatcher := newRecordingDispatcher()
	service.webhooks = dispatcher

	result, err := service.DetectDrift(agent.ID, []string{"filesystem-mcp", "external-api-mcp"}, []string{})
	require.NoError(t, err)
	require.True(t, result.DriftDetected)

	assertTrustScoreChangedSent(t, dispatcher, agent, 85.0, 85.0-FirstViolationPenalty)
	agents.AssertExpectations(t)
}

// A penalty that is not stored, and a check that finds no drift, send nothing.
func TestDriftDetectionService_SendsNothingWithoutAStoredPenalty(t *testing.T) {
	t.Run("penalty not stored", func(t *testing.T) {
		agent := driftingAgent(85.0)
		agents := new(MockAgentRepository)
		alerts := new(MockAlertRepository)
		agents.On("GetByID", agent.ID).Return(agent, nil)
		alerts.On("Create", mock.AnythingOfType("*domain.Alert")).Return(nil)
		agents.On("UpdateTrustScore", agent.ID, 85.0-FirstViolationPenalty).Return(assert.AnError)
		service := NewDriftDetectionService(agents, alerts)
		dispatcher := newRecordingDispatcher()
		service.webhooks = dispatcher

		result, err := service.DetectDrift(agent.ID, []string{"filesystem-mcp", "external-api-mcp"}, []string{})
		require.NoError(t, err)
		require.True(t, result.DriftDetected)
		dispatcher.assertNoMore(t)
	})

	t.Run("no drift", func(t *testing.T) {
		agent := driftingAgent(85.0)
		agents := new(MockAgentRepository)
		agents.On("GetByID", agent.ID).Return(agent, nil)
		service := NewDriftDetectionService(agents, new(MockAlertRepository))
		dispatcher := newRecordingDispatcher()
		service.webhooks = dispatcher

		result, err := service.DetectDrift(agent.ID, []string{"filesystem-mcp"}, []string{})
		require.NoError(t, err)
		require.False(t, result.DriftDetected)
		dispatcher.assertNoMore(t)
		agents.AssertNotCalled(t, "UpdateTrustScore", mock.Anything, mock.Anything)
	})
}

// capabilityAgent is a verified agent holding score.
func capabilityAgent(score float64) *domain.Agent {
	return &domain.Agent{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		Name:           "capability-agent",
		DisplayName:    "Capability agent",
		Status:         domain.AgentStatusVerified,
		TrustScore:     score,
	}
}

// capabilityServiceWithMocks returns a capability service over mocked
// repositories, sending to the returned dispatcher.
func capabilityServiceWithMocks() (*CapabilityService, *MockAgentRepository, *MockCapabilityRepository, *AgentServiceMockAuditLogRepository, *AgentServiceMockTrustScoreCalculator, *AgentServiceMockTrustScoreRepository, *recordingDispatcher) {
	agents := new(MockAgentRepository)
	capabilities := new(MockCapabilityRepository)
	audit := new(AgentServiceMockAuditLogRepository)
	calc := new(AgentServiceMockTrustScoreCalculator)
	scores := new(AgentServiceMockTrustScoreRepository)
	service := NewCapabilityService(capabilities, agents, audit, new(MockAlertRepository), calc, scores)
	dispatcher := newRecordingDispatcher()
	service.webhooks = dispatcher
	return service, agents, capabilities, audit, calc, scores, dispatcher
}

// An action outside the agent's capabilities lowers its stored score, and the
// lowered score is sent as trust_score.changed at the stored precision.
func TestCapabilityService_VerifyAction_ViolationSendsTrustScoreChanged(t *testing.T) {
	agent := capabilityAgent(0.55)
	service, agents, capabilities, audit, _, _, dispatcher := capabilityServiceWithMocks()
	agents.On("GetByID", agent.ID).Return(agent, nil)
	capabilities.On("GetActiveCapabilitiesByAgentID", agent.ID).Return([]*domain.AgentCapability{
		{ID: uuid.New(), AgentID: agent.ID, CapabilityType: "db:read"},
	}, nil)
	capabilities.On("CreateViolation", mock.AnythingOfType("*domain.CapabilityViolation")).Return(nil)
	agents.On("UpdateTrustScore", agent.ID, mock.AnythingOfType("float64")).Return(nil)
	audit.On("Create", mock.AnythingOfType("*domain.AuditLog")).Return(nil)

	result, err := service.VerifyAction(context.Background(), agent.ID, "db:write", nil, nil, nil, nil)
	require.NoError(t, err)
	require.False(t, result.IsAuthorized)

	assertTrustScoreChangedSent(t, dispatcher, agent, 0.55, 0.45)
}

// A violation whose lowered score is not stored sends nothing.
func TestCapabilityService_VerifyAction_UnstoredViolationSendsNothing(t *testing.T) {
	agent := capabilityAgent(0.55)
	service, agents, capabilities, _, _, _, dispatcher := capabilityServiceWithMocks()
	agents.On("GetByID", agent.ID).Return(agent, nil)
	capabilities.On("GetActiveCapabilitiesByAgentID", agent.ID).Return([]*domain.AgentCapability{}, nil)
	capabilities.On("CreateViolation", mock.AnythingOfType("*domain.CapabilityViolation")).Return(nil)
	agents.On("UpdateTrustScore", agent.ID, mock.AnythingOfType("float64")).Return(assert.AnError)

	_, err := service.VerifyAction(context.Background(), agent.ID, "db:write", nil, nil, nil, nil)
	require.Error(t, err)
	dispatcher.assertNoMore(t)
}

// A grant and a revocation recalculate the agent's score, and a recalculated
// score that differs from the stored one is sent as trust_score.changed.
func TestCapabilityService_GrantAndRevoke_SendTrustScoreChanged(t *testing.T) {
	t.Run("grant", func(t *testing.T) {
		agent := capabilityAgent(0.60)
		service, agents, capabilities, audit, calc, scores, dispatcher := capabilityServiceWithMocks()
		grantedBy := uuid.New()
		agents.On("GetByID", agent.ID).Return(agent, nil)
		capabilities.On("CreateCapability", mock.AnythingOfType("*domain.AgentCapability")).Return(nil)
		audit.On("Create", mock.AnythingOfType("*domain.AuditLog")).Return(nil)
		calc.On("Calculate", agent).Return(&domain.TrustScore{ID: uuid.New(), AgentID: agent.ID, Score: 0.70}, nil)
		agents.On("Update", agent).Return(nil)
		scores.On("Create", mock.AnythingOfType("*domain.TrustScore")).Return(nil)

		_, err := service.GrantCapability(context.Background(), agent.ID, "db:write", nil, &grantedBy, domain.ExecutionModeAuto)
		require.NoError(t, err)

		assertTrustScoreChangedSent(t, dispatcher, agent, 0.60, 0.70)
	})

	t.Run("revocation", func(t *testing.T) {
		agent := capabilityAgent(0.60)
		service, agents, capabilities, audit, calc, scores, dispatcher := capabilityServiceWithMocks()
		capabilityID := uuid.New()
		revokedBy := uuid.New()
		capabilities.On("GetCapabilityByID", capabilityID).Return(&domain.AgentCapability{
			ID: capabilityID, AgentID: agent.ID, CapabilityType: "db:write",
		}, nil)
		agents.On("GetByID", agent.ID).Return(agent, nil)
		capabilities.On("RevokeCapability", capabilityID, mock.AnythingOfType("time.Time")).Return(nil)
		audit.On("Create", mock.AnythingOfType("*domain.AuditLog")).Return(nil)
		calc.On("Calculate", agent).Return(&domain.TrustScore{ID: uuid.New(), AgentID: agent.ID, Score: 0.50}, nil)
		agents.On("Update", agent).Return(nil)
		scores.On("Create", mock.AnythingOfType("*domain.TrustScore")).Return(nil)

		require.NoError(t, service.RevokeCapability(context.Background(), capabilityID, &revokedBy))

		assertTrustScoreChangedSent(t, dispatcher, agent, 0.60, 0.50)
	})
}

// A score recalculated after a grant or a revocation is stored by either of
// two writes: the agent's row, saved when there is no transition recorder, or
// the score's trust_scores row, whose insert sets agents.trust_score through
// the trigger of migration 093. trust_score.changed is sent when one of them
// stored a score that differs from the agent's at the stored precision.
func TestCapabilityService_StoreRecalculatedTrustScore_SendsWhenAWriteStoredAChange(t *testing.T) {
	for _, tc := range []struct {
		name       string
		recorded   bool
		rowErr     error
		historyErr error
		from, to   float64
		sent       bool
	}{
		{name: "both rows written", from: 0.60, to: 0.70, sent: true},
		{name: "only the history row written", rowErr: assert.AnError, from: 0.60, to: 0.70, sent: true},
		{name: "only the agent row written", historyErr: assert.AnError, from: 0.60, to: 0.70, sent: true},
		{name: "neither row written", rowErr: assert.AnError, historyErr: assert.AnError, from: 0.60, to: 0.70},
		{name: "recorded, history row written", recorded: true, from: 0.60, to: 0.70, sent: true},
		{name: "recorded, history row not written", recorded: true, historyErr: assert.AnError, from: 0.60, to: 0.70},
		{name: "unchanged at the stored precision", from: 0.60, to: 0.601},
	} {
		t.Run(tc.name, func(t *testing.T) {
			agent := capabilityAgent(tc.from)
			service, agents, _, _, _, scores, dispatcher := capabilityServiceWithMocks()
			if tc.recorded {
				service.transitions = &transition.Recorder{}
			} else {
				agents.On("Update", agent).Return(tc.rowErr)
			}
			scores.On("Create", mock.AnythingOfType("*domain.TrustScore")).Return(tc.historyErr)

			service.storeRecalculatedTrustScore(agent, &domain.TrustScore{ID: uuid.New(), AgentID: agent.ID, Score: tc.to})

			assert.Equal(t, tc.to, agent.TrustScore)
			if tc.sent {
				assertTrustScoreChangedSent(t, dispatcher, agent, tc.from, tc.to)
			} else {
				dispatcher.assertNoMore(t)
			}
			agents.AssertExpectations(t)
			if tc.recorded {
				agents.AssertNotCalled(t, "Update", mock.Anything)
			}
		})
	}
}

// SetWebhookService stores the webhook service as the dispatcher each of the
// three services sends through, and a nil service as no dispatcher, so a
// service built without webhooks sends nothing.
func TestTrustScoreSenders_SetWebhookService(t *testing.T) {
	webhooks := &WebhookService{}

	calculator := &TrustCalculator{}
	calculator.SetWebhookService(webhooks)
	assert.True(t, calculator.webhooks == webhookEventDispatcher(webhooks))
	calculator.SetWebhookService(nil)
	assert.True(t, calculator.webhooks == nil, "a nil *WebhookService must not be stored as a non-nil dispatcher")

	drift := &DriftDetectionService{}
	drift.SetWebhookService(webhooks)
	assert.True(t, drift.webhooks == webhookEventDispatcher(webhooks))
	drift.SetWebhookService(nil)
	assert.True(t, drift.webhooks == nil, "a nil *WebhookService must not be stored as a non-nil dispatcher")

	capabilities := &CapabilityService{}
	capabilities.SetWebhookService(webhooks)
	assert.True(t, capabilities.webhooks == webhookEventDispatcher(webhooks))
	capabilities.SetWebhookService(nil)
	assert.True(t, capabilities.webhooks == nil, "a nil *WebhookService must not be stored as a non-nil dispatcher")
}

// Without a webhook service each of the three stores its score as before.
func TestTrustScoreSenders_WithoutAWebhookServiceStoreAsBefore(t *testing.T) {
	agent := driftingAgent(85.0)
	agents := new(MockAgentRepository)
	alerts := new(MockAlertRepository)
	agents.On("GetByID", agent.ID).Return(agent, nil)
	alerts.On("Create", mock.AnythingOfType("*domain.Alert")).Return(nil)
	agents.On("UpdateTrustScore", agent.ID, 85.0-FirstViolationPenalty).Return(nil)
	drift := NewDriftDetectionService(agents, alerts)
	_, err := drift.DetectDrift(agent.ID, []string{"external-api-mcp"}, []string{})
	require.NoError(t, err)
	agents.AssertExpectations(t)

	scored := calculatorAgent(0.01)
	calculator, calculatorAgents, scores, _ := scoringTrustCalculator(scored)
	calculator.webhooks = nil
	scores.On("Create", mock.AnythingOfType("*domain.TrustScore")).Return(nil)
	calculatorAgents.On("UpdateTrustScore", scored.ID, mock.AnythingOfType("float64")).Return(nil)
	_, err = calculator.CalculateTrustScore(context.Background(), scored.ID)
	require.NoError(t, err)

	granted := capabilityAgent(0.60)
	service, capabilityAgents, _, _, _, capabilityScores, _ := capabilityServiceWithMocks()
	service.webhooks = nil
	capabilityAgents.On("Update", granted).Return(nil)
	capabilityScores.On("Create", mock.AnythingOfType("*domain.TrustScore")).Return(nil)
	assert.NotPanics(t, func() {
		service.storeRecalculatedTrustScore(granted, &domain.TrustScore{ID: uuid.New(), AgentID: granted.ID, Score: 0.70})
	})
	capabilityAgents.AssertExpectations(t)
	capabilityScores.AssertExpectations(t)
}
