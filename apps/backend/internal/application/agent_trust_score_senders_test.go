package application

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record/transition"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// The agent service stores a trust score when it registers an agent, and
// stores a recalculated one after an agent is updated, verified, suspended,
// reactivated or revoked, and after MCP servers are added to or removed from
// its talks_to list. These tests pin that each sends trust_score.changed for
// the score it stored, and nothing when it stored none or the stored score did
// not change at the two decimal places agents.trust_score keeps.

var errWriteFailed = errors.New("connection reset")

// scoredRegistrar is registeringAgentService in an organization whose
// enforcement mode is mode, with a trust calculator that gives each of scores
// in turn (an error for a negative one), and a trust score history whose
// writes, like the agent row's updates, are left to the caller.
func scoredRegistrar(t *testing.T, orgID uuid.UUID, mode domain.EnforcementMode, scores ...float64) (*AgentService, *MockAgentRepository, *AgentServiceMockTrustScoreRepository, *recordingDispatcher) {
	t.Helper()
	s, agents, dispatcher := registeringAgentService(t)
	calc := new(AgentServiceMockTrustScoreCalculator)
	for _, score := range scores {
		if score < 0 {
			calc.On("Calculate", mock.AnythingOfType("*domain.Agent")).Return(nil, errors.New("no factors")).Once()
			continue
		}
		calc.On("Calculate", mock.AnythingOfType("*domain.Agent")).Return(&domain.TrustScore{ID: uuid.New(), Score: score}, nil).Once()
	}
	s.trustCalc = calc
	history := new(AgentServiceMockTrustScoreRepository)
	s.trustScoreRepo = history
	orgs := new(SharedMockOrganizationRepository)
	orgs.On("GetByID", orgID).Return(&domain.Organization{ID: orgID, EnforcementMode: mode}, nil)
	s.orgRepo = orgs
	return s, agents, history, dispatcher
}

// assertRegisteredTrustScore checks that event is trust_score.changed for
// agent, from no score (previousScore 0) to score.
func assertRegisteredTrustScore(t *testing.T, event dispatchedEvent, agent *domain.Agent, score float64) {
	t.Helper()
	assert.Equal(t, domain.WebhookEventTrustScoreChanged, event.event)
	assert.Equal(t, agent.OrganizationID, event.orgID)
	assert.NoError(t, event.ctxErr, "the event is sent with a context that has not ended")
	assert.Equal(t, agent.ID.String(), event.data["agentId"])
	assert.Equal(t, agent.Name, event.data["agentName"])
	assert.Equal(t, 0.0, event.data["previousScore"])
	assert.Equal(t, score, event.data["trustScore"])
}

// A registration sends trust_score.changed once, from no score to the last
// score it stored: for an agent verified at registration, the score
// recalculated after the verification. A score counts as stored when the
// agent's row or its trust score history row was written, as the history
// insert sets agents.trust_score through the trigger of migration 093. It is
// sent whatever the score, 0.5 included, the score a new agent's row holds
// until its first score is stored.
func TestAgentService_CreateAgent_SendsTheStoredTrustScoreOnce(t *testing.T) {
	for _, tc := range []struct {
		name        string
		mode        domain.EnforcementMode
		scores      []float64
		rowErrs     []error // the agent row updates, in order
		historyErrs []error // the trust score history inserts, in order
		events      []domain.WebhookEvent
		sent        float64
	}{
		{
			name: "verified at registration", mode: domain.EnforcementModeMonitoring,
			scores: []float64{0.5, 0.6543}, rowErrs: []error{nil, nil, nil}, historyErrs: []error{nil, nil},
			events: []domain.WebhookEvent{domain.WebhookEventAgentCreated, domain.WebhookEventAgentVerified, domain.WebhookEventTrustScoreChanged},
			sent:   0.65,
		},
		{
			name: "verified at registration, the recalculated score not stored", mode: domain.EnforcementModeMonitoring,
			scores: []float64{0.5, 0.6543}, rowErrs: []error{nil, nil, errWriteFailed}, historyErrs: []error{nil, errWriteFailed},
			events: []domain.WebhookEvent{domain.WebhookEventAgentCreated, domain.WebhookEventAgentVerified, domain.WebhookEventTrustScoreChanged},
			sent:   0.5,
		},
		{
			name: "pending", mode: domain.EnforcementModeStrict,
			scores: []float64{0.5}, rowErrs: []error{nil}, historyErrs: []error{nil},
			events: []domain.WebhookEvent{domain.WebhookEventAgentCreated, domain.WebhookEventTrustScoreChanged},
			sent:   0.5,
		},
		{
			name: "pending, only the history row written", mode: domain.EnforcementModeStrict,
			scores: []float64{0.4321}, rowErrs: []error{errWriteFailed}, historyErrs: []error{nil},
			events: []domain.WebhookEvent{domain.WebhookEventAgentCreated, domain.WebhookEventTrustScoreChanged},
			sent:   0.43,
		},
		{
			name: "pending, only the agent row written", mode: domain.EnforcementModeStrict,
			scores: []float64{0.4321}, rowErrs: []error{nil}, historyErrs: []error{errWriteFailed},
			events: []domain.WebhookEvent{domain.WebhookEventAgentCreated, domain.WebhookEventTrustScoreChanged},
			sent:   0.43,
		},
		{
			name: "pending, neither row written", mode: domain.EnforcementModeStrict,
			scores: []float64{0.4321}, rowErrs: []error{errWriteFailed}, historyErrs: []error{errWriteFailed},
			events: []domain.WebhookEvent{domain.WebhookEventAgentCreated},
		},
		{
			name: "pending, no score calculated", mode: domain.EnforcementModeStrict,
			scores: []float64{-1},
			events: []domain.WebhookEvent{domain.WebhookEventAgentCreated},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			orgID := uuid.New()
			s, agents, history, dispatcher := scoredRegistrar(t, orgID, tc.mode, tc.scores...)
			for _, err := range tc.rowErrs {
				agents.On("Update", mock.AnythingOfType("*domain.Agent")).Return(err).Once()
			}
			for _, err := range tc.historyErrs {
				history.On("Create", mock.AnythingOfType("*domain.TrustScore")).Return(err).Once()
			}

			agent, err := s.CreateAgent(context.Background(), registrationRequest(), orgID, uuid.New(), nil, nil, "")
			require.NoError(t, err)

			got := receiveEvents(t, dispatcher, len(tc.events))
			assert.ElementsMatch(t, tc.events, eventNames(got))
			if changed, ok := got[domain.WebhookEventTrustScoreChanged]; ok {
				assertRegisteredTrustScore(t, changed, agent, tc.sent)
			}
			agents.AssertNumberOfCalls(t, "Update", len(tc.rowErrs))
			history.AssertNumberOfCalls(t, "Create", len(tc.historyErrs))
		})
	}
}

// recalculatingAgentService is an AgentService over agent, without a
// transition recorder, as the server runs it, whose trust score
// recalculation gives score. The agent row updates and the trust score
// history inserts are left to the caller.
func recalculatingAgentService(agent *domain.Agent, score float64) (*AgentService, *MockAgentRepository, *AgentServiceMockTrustScoreRepository, *recordingDispatcher) {
	agents := new(MockAgentRepository)
	calc := new(AgentServiceMockTrustScoreCalculator)
	history := new(AgentServiceMockTrustScoreRepository)
	agents.On("GetByID", agent.ID).Return(agent, nil).Maybe()
	calc.On("Calculate", mock.AnythingOfType("*domain.Agent")).Return(&domain.TrustScore{ID: uuid.New(), AgentID: agent.ID, Score: score}, nil)
	dispatcher := newRecordingDispatcher()
	s := &AgentService{agentRepo: agents, trustCalc: calc, trustScoreRepo: history, webhooks: dispatcher}
	return s, agents, history, dispatcher
}

// recalculatingActs are the agent service's acts that recalculate the
// agent's trust score after they stored their change, with the status each
// starts from and the event each sends for the change itself, if any.
var recalculatingActs = []struct {
	name  string
	from  domain.AgentStatus
	act   func(s *AgentService, a *domain.Agent) error
	event domain.WebhookEvent
}{
	{"update", domain.AgentStatusVerified, func(s *AgentService, a *domain.Agent) error {
		_, err := s.UpdateAgent(context.Background(), a.ID, &CreateAgentRequest{Description: "Reconciles invoices nightly"}, uuid.New())
		return err
	}, domain.WebhookEventAgentUpdated},
	{"verify", domain.AgentStatusPending, func(s *AgentService, a *domain.Agent) error {
		return s.VerifyAgent(context.Background(), a.ID)
	}, domain.WebhookEventAgentVerified},
	{"suspend", domain.AgentStatusVerified, func(s *AgentService, a *domain.Agent) error {
		return s.SuspendAgent(context.Background(), a.ID)
	}, domain.WebhookEventAgentSuspended},
	{"reactivate", domain.AgentStatusSuspended, func(s *AgentService, a *domain.Agent) error {
		return s.ReactivateAgent(context.Background(), a.ID)
	}, ""},
	{"revoke", domain.AgentStatusVerified, func(s *AgentService, a *domain.Agent) error {
		return s.RevokeAgent(context.Background(), a.ID)
	}, ""},
	{"add MCP servers", domain.AgentStatusVerified, func(s *AgentService, a *domain.Agent) error {
		_, _, err := s.AddMCPServers(context.Background(), a.ID, []string{"mcp-server-2"})
		return err
	}, ""},
	{"remove an MCP server", domain.AgentStatusVerified, func(s *AgentService, a *domain.Agent) error {
		_, err := s.RemoveMCPServer(context.Background(), a.ID, "mcp-server-1")
		return err
	}, ""},
}

// expectedEvents is the act's own event, if it sends one, and trust_score.changed
// when sent is true.
func expectedEvents(own domain.WebhookEvent, sent bool) []domain.WebhookEvent {
	events := []domain.WebhookEvent{}
	if own != "" {
		events = append(events, own)
	}
	if sent {
		events = append(events, domain.WebhookEventTrustScoreChanged)
	}
	return events
}

// Each act that recalculates the agent's trust score sends trust_score.changed
// once the recalculated score is stored, from the score the agent held to the
// new one, both at the stored precision.
func TestAgentService_RecalculationsSendTrustScoreChanged(t *testing.T) {
	for _, tc := range recalculatingActs {
		t.Run(tc.name, func(t *testing.T) {
			agent := createTestAgentForService()
			agent.Status = tc.from
			s, agents, history, dispatcher := recalculatingAgentService(agent, 0.4321)
			agents.On("Update", mock.AnythingOfType("*domain.Agent")).Return(nil)
			history.On("Create", mock.AnythingOfType("*domain.TrustScore")).Return(nil).Once()

			require.NoError(t, tc.act(s, agent))

			want := expectedEvents(tc.event, true)
			got := receiveEvents(t, dispatcher, len(want))
			assert.ElementsMatch(t, want, eventNames(got))
			changed := got[domain.WebhookEventTrustScoreChanged]
			assert.Equal(t, agent.OrganizationID, changed.orgID)
			assert.NoError(t, changed.ctxErr, "the event is sent with a context that has not ended")
			assert.Equal(t, agent.ID.String(), changed.data["agentId"])
			assert.Equal(t, agent.Name, changed.data["agentName"])
			assert.Equal(t, 0.85, changed.data["previousScore"])
			assert.Equal(t, 0.43, changed.data["trustScore"])
			history.AssertExpectations(t)
		})
	}
}

// An act whose recalculated score lands on the stored one at the stored
// precision sends no trust_score.changed, and neither does one whose
// recalculated score was written to neither the agent's row nor its history.
// Each act still sends its own event.
func TestAgentService_RecalculationsSendNothingForAnUnchangedOrUnstoredScore(t *testing.T) {
	for _, tc := range recalculatingActs {
		t.Run(tc.name+", unchanged", func(t *testing.T) {
			agent := createTestAgentForService()
			agent.Status = tc.from
			s, agents, history, dispatcher := recalculatingAgentService(agent, 0.8549)
			agents.On("Update", mock.AnythingOfType("*domain.Agent")).Return(nil)
			history.On("Create", mock.AnythingOfType("*domain.TrustScore")).Return(nil).Once()

			require.NoError(t, tc.act(s, agent))

			want := expectedEvents(tc.event, false)
			assert.ElementsMatch(t, want, eventNames(receiveEvents(t, dispatcher, len(want))))
			assert.Equal(t, 0.8549, agent.TrustScore, "the recalculated score is the agent's")
		})
		t.Run(tc.name+", not stored", func(t *testing.T) {
			agent := createTestAgentForService()
			agent.Status = tc.from
			s, agents, history, dispatcher := recalculatingAgentService(agent, 0.4321)
			// Each act stores its change with one update of the agent's row
			// and then the recalculated score with a second.
			agents.On("Update", mock.AnythingOfType("*domain.Agent")).Return(nil).Once()
			agents.On("Update", mock.AnythingOfType("*domain.Agent")).Return(errWriteFailed).Once()
			history.On("Create", mock.AnythingOfType("*domain.TrustScore")).Return(errWriteFailed).Once()

			require.NoError(t, tc.act(s, agent))

			want := expectedEvents(tc.event, false)
			assert.ElementsMatch(t, want, eventNames(receiveEvents(t, dispatcher, len(want))))
			agents.AssertNumberOfCalls(t, "Update", 2)
			history.AssertExpectations(t)
		})
	}
}

// storeRecalculatedTrustScore stores a recalculated score through either of
// two writes: the agent's row, saved when there is no transition recorder, or
// the score's trust_scores row, whose insert sets agents.trust_score through
// the trigger of migration 093. trust_score.changed is sent when one of them
// stored a score that differs from the agent's at the stored precision.
func TestAgentService_StoreRecalculatedTrustScore_SendsWhenAWriteStoredAChange(t *testing.T) {
	for _, tc := range []struct {
		name       string
		recorded   bool
		noHistory  bool
		rowErr     error
		historyErr error
		from, to   float64
		sent       bool
	}{
		{name: "both rows written", from: 0.60, to: 0.70, sent: true},
		{name: "only the history row written", rowErr: errWriteFailed, from: 0.60, to: 0.70, sent: true},
		{name: "only the agent row written", historyErr: errWriteFailed, from: 0.60, to: 0.70, sent: true},
		{name: "neither row written", rowErr: errWriteFailed, historyErr: errWriteFailed, from: 0.60, to: 0.70},
		{name: "no trust score history, agent row written", noHistory: true, from: 0.60, to: 0.70, sent: true},
		{name: "no trust score history, agent row not written", noHistory: true, rowErr: errWriteFailed, from: 0.60, to: 0.70},
		{name: "recorded, history row written", recorded: true, from: 0.60, to: 0.70, sent: true},
		{name: "recorded, history row not written", recorded: true, historyErr: errWriteFailed, from: 0.60, to: 0.70},
		{name: "unchanged at the stored precision", from: 0.60, to: 0.601},
	} {
		t.Run(tc.name, func(t *testing.T) {
			agent := createTestAgentForService()
			agent.TrustScore = tc.from
			s, agents, history, dispatcher := recalculatingAgentService(agent, tc.to)
			if tc.recorded {
				s.transitions = &transition.Recorder{}
			} else {
				agents.On("Update", agent).Return(tc.rowErr).Once()
			}
			if tc.noHistory {
				s.trustScoreRepo = nil
			} else {
				history.On("Create", mock.AnythingOfType("*domain.TrustScore")).Return(tc.historyErr).Once()
			}

			s.storeRecalculatedTrustScore(agent, &domain.TrustScore{ID: uuid.New(), AgentID: agent.ID, Score: tc.to})

			assert.Equal(t, tc.to, agent.TrustScore)
			if tc.sent {
				got := receiveEvents(t, dispatcher, 1)
				changed, ok := got[domain.WebhookEventTrustScoreChanged]
				require.True(t, ok, "trust_score.changed is sent, got %v", eventNames(got))
				assert.Equal(t, storedTrustScore(tc.from), changed.data["previousScore"])
				assert.Equal(t, storedTrustScore(tc.to), changed.data["trustScore"])
			} else {
				dispatcher.assertNoMore(t)
			}
			agents.AssertExpectations(t)
			history.AssertExpectations(t)
			if tc.recorded {
				agents.AssertNotCalled(t, "Update", mock.Anything)
			}
		})
	}
}

// Without a webhook service a registration and a recalculation store their
// scores as before and send nothing.
func TestAgentService_TrustScoreWritesWithoutAWebhookService(t *testing.T) {
	orgID := uuid.New()
	registrar, agents, history, _ := scoredRegistrar(t, orgID, domain.EnforcementModeStrict, 0.4321)
	registrar.webhooks = nil
	agents.On("Update", mock.AnythingOfType("*domain.Agent")).Return(nil).Once()
	history.On("Create", mock.AnythingOfType("*domain.TrustScore")).Return(nil).Once()
	registered, err := registrar.CreateAgent(context.Background(), registrationRequest(), orgID, uuid.New(), nil, nil, "")
	require.NoError(t, err)
	assert.Equal(t, 0.4321, registered.TrustScore)
	agents.AssertExpectations(t)
	history.AssertExpectations(t)

	agent := createTestAgentForService()
	s, agents, history, _ := recalculatingAgentService(agent, 0.4321)
	s.webhooks = nil
	agents.On("Update", mock.AnythingOfType("*domain.Agent")).Return(nil).Twice()
	history.On("Create", mock.AnythingOfType("*domain.TrustScore")).Return(nil).Once()
	require.NoError(t, s.SuspendAgent(context.Background(), agent.ID))
	assert.Equal(t, 0.4321, agent.TrustScore)
	agents.AssertExpectations(t)
	history.AssertExpectations(t)
}
