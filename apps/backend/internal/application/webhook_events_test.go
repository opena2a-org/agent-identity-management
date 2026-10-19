package application

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// receiveEvents takes n events from d, keyed by event name, and then checks
// that no further event arrives. The events of one call are dispatched on
// goroutines of their own, so their order is not part of the contract.
func receiveEvents(t *testing.T, d *recordingDispatcher, n int) map[domain.WebhookEvent]dispatchedEvent {
	t.Helper()
	got := make(map[domain.WebhookEvent]dispatchedEvent, n)
	for i := 0; i < n; i++ {
		e := d.next(t)
		_, repeated := got[e.event]
		require.False(t, repeated, "%s was sent twice", e.event)
		got[e.event] = e
	}
	d.assertNoMore(t)
	return got
}

func eventNames(events map[domain.WebhookEvent]dispatchedEvent) []domain.WebhookEvent {
	names := make([]domain.WebhookEvent, 0, len(events))
	for name := range events {
		names = append(names, name)
	}
	return names
}

// registeringAgentService is an AgentService that registers agents without a
// transition recorder, as the server does, with every repository mocked.
func registeringAgentService(t *testing.T) (*AgentService, *MockAgentRepository, *recordingDispatcher) {
	t.Helper()
	agents := new(MockAgentRepository)
	calc := new(AgentServiceMockTrustScoreCalculator)
	scores := new(AgentServiceMockTrustScoreRepository)
	keyVault, err := crypto.NewKeyVault(base64.StdEncoding.EncodeToString([]byte("test-master-key-32-bytes-long!!!")))
	require.NoError(t, err)

	calc.On("Calculate", mock.AnythingOfType("*domain.Agent")).Return(&domain.TrustScore{ID: uuid.New(), Score: 0.75}, nil)
	scores.On("Create", mock.AnythingOfType("*domain.TrustScore")).Return(nil)
	agents.On("Create", mock.AnythingOfType("*domain.Agent")).Return(nil)

	dispatcher := newRecordingDispatcher()
	s := &AgentService{
		agentRepo:      agents,
		trustCalc:      calc,
		trustScoreRepo: scores,
		keyVault:       keyVault,
		capabilityRepo: new(MockCapabilityRepository),
		webhooks:       dispatcher,
	}
	return s, agents, dispatcher
}

func registrationRequest() *CreateAgentRequest {
	return &CreateAgentRequest{
		Name:        "billing-agent",
		DisplayName: "Billing Agent",
		Description: "Reconciles invoices",
		AgentType:   domain.AgentTypeAI,
		Version:     "1.0.0",
	}
}

func TestAgentService_CreateAgent_SendsCreatedAndVerified(t *testing.T) {
	s, agents, dispatcher := registeringAgentService(t)
	agents.On("Update", mock.AnythingOfType("*domain.Agent")).Return(nil)

	orgID := uuid.New()
	agent, err := s.CreateAgent(context.Background(), registrationRequest(), orgID, uuid.New(), nil, nil, "")
	require.NoError(t, err)
	require.Equal(t, domain.AgentStatusVerified, agent.Status, "the agent qualifies for verification at registration")

	got := receiveEvents(t, dispatcher, 3)
	assert.ElementsMatch(t, []domain.WebhookEvent{domain.WebhookEventAgentCreated, domain.WebhookEventAgentVerified, domain.WebhookEventTrustScoreChanged}, eventNames(got))
	assertRegisteredTrustScore(t, got[domain.WebhookEventTrustScoreChanged], agent, 0.75)
	for _, name := range []domain.WebhookEvent{domain.WebhookEventAgentCreated, domain.WebhookEventAgentVerified} {
		e := got[name]
		assert.Equal(t, orgID, e.orgID)
		assert.NoError(t, e.ctxErr)
		assert.Equal(t, agent.ID.String(), e.data["agentId"])
		assert.Equal(t, "billing-agent", e.data["name"])
		assert.Equal(t, "Billing Agent", e.data["displayName"])
		assert.Equal(t, string(domain.AgentTypeAI), e.data["agentType"])
		assert.Equal(t, string(domain.AgentStatusVerified), e.data["status"])
		assert.Equal(t, 0.75, e.data["trustScore"])
	}
}

func TestAgentService_CreateAgent_SendsNoVerifiedWhenTheVerificationWasNotStored(t *testing.T) {
	s, agents, dispatcher := registeringAgentService(t)
	// The first Update stores the initial trust score, the second the
	// verification, the third the trust score recalculated after it.
	agents.On("Update", mock.AnythingOfType("*domain.Agent")).Return(nil).Once()
	agents.On("Update", mock.AnythingOfType("*domain.Agent")).Return(errors.New("connection reset")).Once()
	agents.On("Update", mock.AnythingOfType("*domain.Agent")).Return(nil).Once()

	agent, err := s.CreateAgent(context.Background(), registrationRequest(), uuid.New(), uuid.New(), nil, nil, "")
	require.NoError(t, err)

	got := receiveEvents(t, dispatcher, 2)
	assert.ElementsMatch(t, []domain.WebhookEvent{domain.WebhookEventAgentCreated, domain.WebhookEventTrustScoreChanged}, eventNames(got))
	assertRegisteredTrustScore(t, got[domain.WebhookEventTrustScoreChanged], agent, 0.75)
	agents.AssertNumberOfCalls(t, "Update", 3)
}

func TestAgentService_CreateAgent_InStrictModeSendsNoVerified(t *testing.T) {
	s, agents, dispatcher := registeringAgentService(t)
	agents.On("Update", mock.AnythingOfType("*domain.Agent")).Return(nil)
	orgID := uuid.New()
	orgs := new(SharedMockOrganizationRepository)
	orgs.On("GetByID", orgID).Return(&domain.Organization{ID: orgID, EnforcementMode: domain.EnforcementModeStrict}, nil)
	s.orgRepo = orgs

	agent, err := s.CreateAgent(context.Background(), registrationRequest(), orgID, uuid.New(), nil, nil, "")
	require.NoError(t, err)
	require.Equal(t, domain.AgentStatusPending, agent.Status, "strict mode leaves a new agent pending")

	got := receiveEvents(t, dispatcher, 2)
	assert.ElementsMatch(t, []domain.WebhookEvent{domain.WebhookEventAgentCreated, domain.WebhookEventTrustScoreChanged}, eventNames(got))
	assert.Equal(t, string(domain.AgentStatusPending), got[domain.WebhookEventAgentCreated].data["status"])
	assertRegisteredTrustScore(t, got[domain.WebhookEventTrustScoreChanged], agent, 0.75)
}

// lifecycleAgentService is an AgentService over one stored agent whose writes
// all succeed and whose trust score recalculation returns 0.9.
func lifecycleAgentService(agent *domain.Agent) (*AgentService, *MockAgentRepository, *recordingDispatcher) {
	agents := new(MockAgentRepository)
	calc := new(AgentServiceMockTrustScoreCalculator)
	scores := new(AgentServiceMockTrustScoreRepository)
	agents.On("GetByID", agent.ID).Return(agent, nil)
	agents.On("Update", mock.AnythingOfType("*domain.Agent")).Return(nil).Maybe()
	agents.On("Delete", agent.ID).Return(nil).Maybe()
	calc.On("Calculate", mock.AnythingOfType("*domain.Agent")).Return(&domain.TrustScore{ID: uuid.New(), Score: 0.9}, nil).Maybe()
	scores.On("Create", mock.AnythingOfType("*domain.TrustScore")).Return(nil).Maybe()

	dispatcher := newRecordingDispatcher()
	s := &AgentService{agentRepo: agents, trustCalc: calc, trustScoreRepo: scores, webhooks: dispatcher}
	return s, agents, dispatcher
}

func TestAgentService_LifecycleActsSendTheirEvent(t *testing.T) {
	for _, tc := range []struct {
		name       string
		from       domain.AgentStatus
		act        func(s *AgentService, ctx context.Context, a *domain.Agent) error
		event      domain.WebhookEvent
		wantStatus domain.AgentStatus
		// The act stores a recalculated trust score, 0.9 where the agent held
		// 0.85, and sends trust_score.changed for it as well.
		rescored bool
	}{
		{"update", domain.AgentStatusVerified, func(s *AgentService, ctx context.Context, a *domain.Agent) error {
			_, err := s.UpdateAgent(ctx, a.ID, &CreateAgentRequest{Description: "Reconciles invoices nightly"}, uuid.New())
			return err
		}, domain.WebhookEventAgentUpdated, domain.AgentStatusVerified, true},
		{"verify", domain.AgentStatusPending, func(s *AgentService, ctx context.Context, a *domain.Agent) error {
			return s.VerifyAgent(ctx, a.ID)
		}, domain.WebhookEventAgentVerified, domain.AgentStatusVerified, true},
		{"suspend", domain.AgentStatusVerified, func(s *AgentService, ctx context.Context, a *domain.Agent) error {
			return s.SuspendAgent(ctx, a.ID)
		}, domain.WebhookEventAgentSuspended, domain.AgentStatusSuspended, true},
		{"delete", domain.AgentStatusVerified, func(s *AgentService, ctx context.Context, a *domain.Agent) error {
			return s.DeleteAgent(ctx, a.ID)
		}, domain.WebhookEventAgentDeleted, domain.AgentStatusVerified, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			agent := createTestAgentForService()
			agent.Status = tc.from
			s, _, dispatcher := lifecycleAgentService(agent)

			// The request has ended by the time the event is dispatched.
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			require.NoError(t, tc.act(s, ctx, agent))

			want := []domain.WebhookEvent{tc.event}
			if tc.rescored {
				want = append(want, domain.WebhookEventTrustScoreChanged)
			}
			got := receiveEvents(t, dispatcher, len(want))
			assert.ElementsMatch(t, want, eventNames(got))
			e := got[tc.event]
			assert.NoError(t, e.ctxErr, "the event must not be dispatched under the request's context")
			assert.Equal(t, agent.OrganizationID, e.orgID)
			assert.Equal(t, agent.ID.String(), e.data["agentId"])
			assert.Equal(t, string(tc.wantStatus), e.data["status"])
			if tc.rescored {
				changed := got[domain.WebhookEventTrustScoreChanged]
				assert.NoError(t, changed.ctxErr, "the event must not be dispatched under the request's context")
				assert.Equal(t, agent.OrganizationID, changed.orgID)
				assert.Equal(t, 0.85, changed.data["previousScore"])
				assert.Equal(t, 0.9, changed.data["trustScore"])
			}
		})
	}
}

func TestAgentService_RefusedOrFailedActsSendNothing(t *testing.T) {
	t.Run("verify a revoked agent", func(t *testing.T) {
		agent := createTestAgentForService()
		agent.Status = domain.AgentStatusRevoked
		s, _, dispatcher := lifecycleAgentService(agent)
		var refusal *domain.AgentStatusTransitionError
		require.ErrorAs(t, s.VerifyAgent(context.Background(), agent.ID), &refusal)
		dispatcher.assertNoMore(t)
	})
	t.Run("suspend a suspended agent", func(t *testing.T) {
		agent := createTestAgentForService()
		agent.Status = domain.AgentStatusSuspended
		s, agents, dispatcher := lifecycleAgentService(agent)
		require.NoError(t, s.SuspendAgent(context.Background(), agent.ID))
		agents.AssertNotCalled(t, "Update", mock.Anything)
		dispatcher.assertNoMore(t)
	})
	t.Run("suspension not stored", func(t *testing.T) {
		agent := createTestAgentForService()
		agents := new(MockAgentRepository)
		agents.On("GetByID", agent.ID).Return(agent, nil)
		agents.On("Update", mock.AnythingOfType("*domain.Agent")).Return(errors.New("connection reset"))
		dispatcher := newRecordingDispatcher()
		s := &AgentService{agentRepo: agents, webhooks: dispatcher}
		require.Error(t, s.SuspendAgent(context.Background(), agent.ID))
		dispatcher.assertNoMore(t)
	})
	t.Run("deletion not stored", func(t *testing.T) {
		agent := createTestAgentForService()
		agents := new(MockAgentRepository)
		agents.On("GetByID", agent.ID).Return(agent, nil)
		agents.On("Delete", agent.ID).Return(errors.New("connection reset"))
		dispatcher := newRecordingDispatcher()
		s := &AgentService{agentRepo: agents, webhooks: dispatcher}
		require.Error(t, s.DeleteAgent(context.Background(), agent.ID))
		dispatcher.assertNoMore(t)
	})
}

// trustScoreAgentService is an AgentService with the real trust score policy
// over a verified agent stored at previousScore.
func trustScoreAgentService(agent *domain.Agent) (*AgentService, *MockAgentRepository, *recordingDispatcher) {
	agents := new(MockAgentRepository)
	alerts := new(MockAlertRepository)
	agents.On("GetByID", agent.ID).Return(agent, nil)
	agents.On("UpdateTrustScore", agent.ID, mock.AnythingOfType("float64")).Return(nil)
	agents.On("Update", mock.AnythingOfType("*domain.Agent")).Return(nil).Maybe()
	alerts.On("GetUnacknowledged", agent.OrganizationID).Return([]*domain.Alert{}, nil).Maybe()
	alerts.On("GetByOrganization", agent.OrganizationID, 50, 0).Return([]*domain.Alert{}, nil).Maybe()
	alerts.On("Create", mock.AnythingOfType("*domain.Alert")).Return(nil).Maybe()

	policy := NewSecurityPolicyService(nil, alerts, nil)
	policy.SetAgentRepository(agents)

	dispatcher := newRecordingDispatcher()
	s := &AgentService{agentRepo: agents, alertRepo: alerts, policyService: policy, webhooks: dispatcher}
	return s, agents, dispatcher
}

func TestAgentService_UpdateTrustScore_SendsTrustScoreEvents(t *testing.T) {
	for _, tc := range []struct {
		name         string
		from, to     float64
		want         []domain.WebhookEvent
		wantSeverity string
	}{
		{"rise", 0.60, 0.80, []domain.WebhookEvent{domain.WebhookEventTrustScoreChanged}, ""},
		{"no change", 0.80, 0.80, nil, ""},
		{"drop that stays above the warning threshold", 0.85, 0.75, []domain.WebhookEvent{domain.WebhookEventTrustScoreChanged}, ""},
		{"drop below the warning threshold", 0.85, 0.65,
			[]domain.WebhookEvent{domain.WebhookEventTrustScoreChanged, domain.WebhookEventTrustScoreLow}, string(domain.SeverityWarning)},
		{"drop below the critical threshold", 0.85, 0.40,
			[]domain.WebhookEvent{domain.WebhookEventTrustScoreChanged, domain.WebhookEventTrustScoreLow, domain.WebhookEventAgentSuspended},
			string(domain.SeverityCritical)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			agent := createTestAgentForService()
			agent.TrustScore = tc.from
			s, _, dispatcher := trustScoreAgentService(agent)

			require.NoError(t, s.UpdateTrustScore(context.Background(), agent.ID, tc.to))

			got := receiveEvents(t, dispatcher, len(tc.want))
			assert.ElementsMatch(t, tc.want, eventNames(got))
			for _, e := range got {
				assert.Equal(t, agent.OrganizationID, e.orgID)
				assert.Equal(t, agent.ID.String(), e.data["agentId"])
			}
			if changed, ok := got[domain.WebhookEventTrustScoreChanged]; ok {
				assert.Equal(t, tc.from, changed.data["previousScore"])
				assert.Equal(t, tc.to, changed.data["trustScore"])
			}
			if low, ok := got[domain.WebhookEventTrustScoreLow]; ok {
				assert.Equal(t, tc.to, low.data["trustScore"])
				assert.Equal(t, tc.wantSeverity, low.data["severity"])
			}
			if suspended, ok := got[domain.WebhookEventAgentSuspended]; ok {
				assert.Equal(t, string(domain.AgentStatusSuspended), suspended.data["status"])
			}
		})
	}
}

func TestAgentService_RecalculateTrustScore_SendsTrustScoreChanged(t *testing.T) {
	agent := createTestAgentForService()
	agent.TrustScore = 0.85
	s, _, dispatcher := lifecycleAgentService(agent)

	score, err := s.RecalculateTrustScore(context.Background(), agent.ID)
	require.NoError(t, err)

	got := receiveEvents(t, dispatcher, 1)
	require.Contains(t, got, domain.WebhookEventTrustScoreChanged)
	assert.Equal(t, 0.85, got[domain.WebhookEventTrustScoreChanged].data["previousScore"])
	assert.Equal(t, score.Score, got[domain.WebhookEventTrustScoreChanged].data["trustScore"])
}

// The trust re-evaluation job stores a recalculated score itself and asks the
// service to announce it. The two scores are compared, and sent, at the two
// decimal places agents.trust_score stores.
func TestAgentService_NotifyTrustScoreChanged_ComparesAtTheStoredPrecision(t *testing.T) {
	agent := createTestAgentForService()
	dispatcher := newRecordingDispatcher()
	s := &AgentService{webhooks: dispatcher}

	s.NotifyTrustScoreChanged(agent, 0.90, 0.9049)
	s.NotifyTrustScoreChanged(agent, 0.90, 0.8951)
	dispatcher.assertNoMore(t)

	s.NotifyTrustScoreChanged(agent, 0.90, 0.8949)
	got := receiveEvents(t, dispatcher, 1)
	changed, ok := got[domain.WebhookEventTrustScoreChanged]
	require.True(t, ok, "trust_score.changed is sent")
	assert.Equal(t, agent.OrganizationID, changed.orgID)
	assert.Equal(t, agent.ID.String(), changed.data["agentId"])
	assert.Equal(t, agent.Name, changed.data["agentName"])
	assert.Equal(t, 0.90, changed.data["previousScore"])
	assert.Equal(t, 0.89, changed.data["trustScore"])
}

func TestAgentService_NotifyTrustScoreChanged_WithoutAWebhookServiceSendsNothing(t *testing.T) {
	agent := createTestAgentForService()
	assert.NotPanics(t, func() { (&AgentService{}).NotifyTrustScoreChanged(agent, 0.50, 0.90) })
}

// A blocked capability violation lowers the stored score; when the score it
// leaves is below the critical threshold the policy suspends the agent, and
// each of the three changes is sent.
func TestAgentService_VerifyCapability_ViolationSendsTrustScoreEvents(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	mockTrustScoreRepo := new(AgentServiceMockTrustScoreRepository)
	mockAlertRepo := new(MockAlertRepository)
	mockCapabilityRepo := new(MockCapabilityRepository)
	mockPolicyRepo := new(AgentServiceMockSecurityPolicyRepository)
	mockOrgRepo := new(MockOrganizationRepository)

	agent := &domain.Agent{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		Name:           "report-agent",
		Status:         domain.AgentStatusVerified,
		TrustScore:     0.55,
		CreatedAt:      time.Now().Add(-time.Hour),
		UpdatedAt:      time.Now(),
	}

	policyService := NewSecurityPolicyService(mockPolicyRepo, mockAlertRepo, nil)
	policyService.SetAgentRepository(mockAgentRepo)

	dispatcher := newRecordingDispatcher()
	service := &AgentService{
		agentRepo:      mockAgentRepo,
		trustScoreRepo: mockTrustScoreRepo,
		alertRepo:      mockAlertRepo,
		policyService:  policyService,
		capabilityRepo: mockCapabilityRepo,
		orgRepo:        mockOrgRepo,
		webhooks:       dispatcher,
	}

	mockAgentRepo.On("GetByID", agent.ID).Return(agent, nil)
	mockCapabilityRepo.On("GetActiveCapabilitiesByAgentID", agent.ID).Return([]*domain.AgentCapability{
		{ID: uuid.New(), AgentID: agent.ID, CapabilityType: "db:read"},
	}, nil)
	mockOrgRepo.On("GetByID", agent.OrganizationID).Return(&domain.Organization{
		ID:              agent.OrganizationID,
		EnforcementMode: domain.EnforcementModeStrict,
	}, nil)
	mockPolicyRepo.On("GetByType", agent.OrganizationID, domain.PolicyTypeCapabilityViolation).
		Return([]*domain.SecurityPolicy{}, nil)
	mockAlertRepo.On("Create", mock.AnythingOfType("*domain.Alert")).Return(nil)
	mockAlertRepo.On("CreateCoalesced", mock.AnythingOfType("*domain.Alert"),
		mock.AnythingOfType("time.Time"), mock.AnythingOfType("time.Time")).Return(true, nil)
	mockAlertRepo.On("GetByOrganization", agent.OrganizationID, 50, 0).Return([]*domain.Alert{}, nil)
	mockCapabilityRepo.On("CreateViolation", mock.AnythingOfType("*domain.CapabilityViolation")).Return(nil)
	mockAgentRepo.On("UpdateTrustScore", agent.ID, mock.AnythingOfType("float64")).Return(nil)
	mockTrustScoreRepo.On("UpdateScore", agent.ID, mock.AnythingOfType("float64")).Return(nil)
	mockAgentRepo.On("Update", mock.AnythingOfType("*domain.Agent")).Return(nil)

	allowed, _, _, err := service.VerifyCapability(context.Background(), agent.ID, "db:write", "", nil, "")
	require.NoError(t, err)
	require.False(t, allowed)

	got := receiveEvents(t, dispatcher, 3)
	assert.ElementsMatch(t, []domain.WebhookEvent{
		domain.WebhookEventTrustScoreChanged, domain.WebhookEventTrustScoreLow, domain.WebhookEventAgentSuspended,
	}, eventNames(got))
	changed := got[domain.WebhookEventTrustScoreChanged]
	assert.Equal(t, 0.55, changed.data["previousScore"])
	assert.InDelta(t, 0.45, changed.data["trustScore"], 1e-9)
	assert.Equal(t, string(domain.SeverityCritical), got[domain.WebhookEventTrustScoreLow].data["severity"])
}

func TestAPIKeyService_SendsCreatedAndRevoked(t *testing.T) {
	orgID := uuid.New()
	agent := createTestAgentForAPIKey(orgID)

	t.Run("created", func(t *testing.T) {
		agents := new(MockAgentRepoForAPIKey)
		keys := new(MockAPIKeyRepoForAPIKey)
		agents.On("GetByID", agent.ID).Return(agent, nil)
		keys.On("Create", mock.AnythingOfType("*domain.APIKey")).Run(func(args mock.Arguments) {
			args.Get(0).(*domain.APIKey).ID = uuid.New() // the repository assigns the id
		}).Return(nil)
		dispatcher := newRecordingDispatcher()
		s := NewAPIKeyService(keys, agents)
		s.webhooks = dispatcher

		fullKey, key, err := s.GenerateAPIKey(context.Background(), agent.ID, orgID, uuid.New(), "ci", 30)
		require.NoError(t, err)

		got := receiveEvents(t, dispatcher, 1)
		require.Contains(t, got, domain.WebhookEventAPIKeyCreated)
		e := got[domain.WebhookEventAPIKeyCreated]
		assert.Equal(t, orgID, e.orgID)
		assert.Equal(t, key.ID.String(), e.data["apiKeyId"])
		assert.Equal(t, agent.ID.String(), e.data["agentId"])
		assert.Equal(t, "ci", e.data["name"])
		assert.Equal(t, key.Prefix, e.data["prefix"])
		assert.Equal(t, *key.ExpiresAt, e.data["expiresAt"])
		for field, value := range e.data {
			if str, ok := value.(string); ok {
				assert.NotEqual(t, fullKey, str, "field %s carries the key", field)
				assert.NotEqual(t, key.KeyHash, str, "field %s carries the key hash", field)
			}
		}
	})

	newKey := func(active bool) *domain.APIKey {
		return &domain.APIKey{ID: uuid.New(), OrganizationID: orgID, AgentID: agent.ID, Name: "ci", Prefix: "aim_live_abcdefg", IsActive: active}
	}

	t.Run("revoked", func(t *testing.T) {
		key := newKey(true)
		keys := new(MockAPIKeyRepoForAPIKey)
		keys.On("GetByID", key.ID).Return(key, nil)
		keys.On("Revoke", key.ID).Return(nil)
		dispatcher := newRecordingDispatcher()
		s := NewAPIKeyService(keys, new(MockAgentRepoForAPIKey))
		s.webhooks = dispatcher

		require.NoError(t, s.RevokeAPIKey(context.Background(), key.ID, orgID))

		got := receiveEvents(t, dispatcher, 1)
		require.Contains(t, got, domain.WebhookEventAPIKeyRevoked)
		assert.Equal(t, orgID, got[domain.WebhookEventAPIKeyRevoked].orgID)
		assert.Equal(t, key.ID.String(), got[domain.WebhookEventAPIKeyRevoked].data["apiKeyId"])
		_, hasExpiry := got[domain.WebhookEventAPIKeyRevoked].data["expiresAt"]
		assert.False(t, hasExpiry, "a key without an expiry sends no expiresAt")
	})

	t.Run("already revoked", func(t *testing.T) {
		key := newKey(false)
		keys := new(MockAPIKeyRepoForAPIKey)
		keys.On("GetByID", key.ID).Return(key, nil)
		keys.On("Revoke", key.ID).Return(nil)
		dispatcher := newRecordingDispatcher()
		s := NewAPIKeyService(keys, new(MockAgentRepoForAPIKey))
		s.webhooks = dispatcher

		require.NoError(t, s.RevokeAPIKey(context.Background(), key.ID, orgID))
		dispatcher.assertNoMore(t)
	})

	t.Run("another organization's key", func(t *testing.T) {
		key := newKey(true)
		keys := new(MockAPIKeyRepoForAPIKey)
		keys.On("GetByID", key.ID).Return(key, nil)
		dispatcher := newRecordingDispatcher()
		s := NewAPIKeyService(keys, new(MockAgentRepoForAPIKey))
		s.webhooks = dispatcher

		require.Error(t, s.RevokeAPIKey(context.Background(), key.ID, uuid.New()))
		keys.AssertNotCalled(t, "Revoke", mock.Anything)
		dispatcher.assertNoMore(t)
	})

	t.Run("revocation not stored", func(t *testing.T) {
		key := newKey(true)
		keys := new(MockAPIKeyRepoForAPIKey)
		keys.On("GetByID", key.ID).Return(key, nil)
		keys.On("Revoke", key.ID).Return(errors.New("connection reset"))
		dispatcher := newRecordingDispatcher()
		s := NewAPIKeyService(keys, new(MockAgentRepoForAPIKey))
		s.webhooks = dispatcher

		require.Error(t, s.RevokeAPIKey(context.Background(), key.ID, orgID))
		dispatcher.assertNoMore(t)
	})
}

func TestSetWebhookService_NilSetsNoDispatcher(t *testing.T) {
	agents := &AgentService{webhooks: newRecordingDispatcher()}
	agents.SetWebhookService(nil)
	assert.Nil(t, agents.webhooks, "a nil *WebhookService must not be stored as a non-nil dispatcher")

	keys := &APIKeyService{webhooks: newRecordingDispatcher()}
	keys.SetWebhookService(nil)
	assert.Nil(t, keys.webhooks)

	alerts := &AlertService{webhookService: newRecordingDispatcher()}
	alerts.SetWebhookService(nil)
	assert.Nil(t, alerts.webhookService)
}

// TestAPIKeyService_GenerateAPIKey_DeliversOneSignedAPIKeyCreated drives the
// real WebhookService against a recording endpoint: the delivered body is the
// api_key.created payload, signed, and carries neither the key nor its hash.
func TestAPIKeyService_GenerateAPIKey_DeliversOneSignedAPIKeyCreated(t *testing.T) {
	type received struct {
		header http.Header
		body   []byte
	}
	posts := make(chan received, 4)
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		posts <- received{header: r.Header.Clone(), body: body}
		w.WriteHeader(http.StatusOK)
	}))
	defer endpoint.Close()

	db, sqlMock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	orgID, webhookID := uuid.New(), uuid.New()
	secret := "test-webhook-secret"
	now := time.Now().UTC()
	sqlMock.ExpectQuery(`FROM webhooks`).WithArgs(orgID).WillReturnRows(
		sqlmock.NewRows([]string{
			"id", "organization_id", "name", "url", "events", "secret", "is_active",
			"timeout_seconds", "max_retries", "retry_delay_seconds",
			"last_triggered", "success_count", "failure_count", "created_by", "created_at", "updated_at",
		}).AddRow(
			webhookID.String(), orgID.String(), "keys", endpoint.URL,
			[]byte("{alert.created,api_key.created}"), secret, true,
			5, 1, 1,
			nil, 0, 0, uuid.New().String(), now, now,
		))
	sqlMock.ExpectExec(`INSERT INTO webhook_deliveries`).WillReturnResult(sqlmock.NewResult(0, 1))
	sqlMock.ExpectExec(`UPDATE webhooks`).WillReturnResult(sqlmock.NewResult(0, 1))

	webhookService := NewWebhookService(repository.NewWebhookRepository(db))
	// The endpoint is an httptest server on loopback, which the egress client refuses.
	webhookService.newHTTPClient = loopbackAdmittingClient

	agent := createTestAgentForAPIKey(orgID)
	agents := new(MockAgentRepoForAPIKey)
	keys := new(MockAPIKeyRepoForAPIKey)
	agents.On("GetByID", agent.ID).Return(agent, nil)
	keys.On("Create", mock.AnythingOfType("*domain.APIKey")).Run(func(args mock.Arguments) {
		args.Get(0).(*domain.APIKey).ID = uuid.New()
	}).Return(nil)
	service := NewAPIKeyService(keys, agents)
	service.SetWebhookService(webhookService)

	fullKey, key, err := service.GenerateAPIKey(context.Background(), agent.ID, orgID, uuid.New(), "ci", 0)
	require.NoError(t, err)

	var post received
	select {
	case post = <-posts:
	case <-time.After(10 * time.Second):
		t.Fatal("the subscribed endpoint received no api_key.created POST")
	}

	assert.Equal(t, "api_key.created", post.header.Get("X-Webhook-Event"))
	timestamp := post.header.Get("X-Webhook-Timestamp")
	require.NotEmpty(t, timestamp, "the delivery carries the time it was sent")
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "."))
	mac.Write(post.body)
	assert.Equal(t, "v1="+hex.EncodeToString(mac.Sum(nil)), post.header.Get("X-Webhook-Signature"))

	var payload domain.WebhookPayload
	require.NoError(t, json.Unmarshal(post.body, &payload))
	assert.Equal(t, domain.WebhookEventAPIKeyCreated, payload.Event)
	assert.Equal(t, orgID.String(), payload.OrganizationID)
	assert.Equal(t, key.ID.String(), payload.Data["apiKeyId"])
	assert.False(t, strings.Contains(string(post.body), fullKey), "the delivery must not carry the key")
	assert.False(t, strings.Contains(string(post.body), key.KeyHash), "the delivery must not carry the key hash")

	assert.Eventually(t, func() bool { return sqlMock.ExpectationsWereMet() == nil }, 5*time.Second, 10*time.Millisecond,
		"the delivery must be recorded as delivered")

	select {
	case extra := <-posts:
		t.Fatalf("expected exactly one POST, got another with event %q", extra.header.Get("X-Webhook-Event"))
	case <-time.After(200 * time.Millisecond):
	}
}
