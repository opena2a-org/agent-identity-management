package application

import (
	"context"
	"encoding/base64"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/crypto"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// fakeOnboardingRepo records what the service writes and serves canned reads.
type fakeOnboardingRepo struct {
	mu          sync.Mutex
	events      []domain.OnboardingEvent
	firstAgents []uuid.UUID
	recordErr   error

	samples      []domain.TimeToFirstAgentSample
	samplesErr   error
	counts       []domain.OnboardingEventCount
	countsErr    error
	countedSince time.Time
}

func (f *fakeOnboardingRepo) Record(_ context.Context, e *domain.OnboardingEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.recordErr != nil {
		return f.recordErr
	}
	f.events = append(f.events, *e)
	return nil
}

// RecordCapped applies the cap the SQL statement applies: same organization,
// event and tab, at or after since.
func (f *fakeOnboardingRepo) RecordCapped(_ context.Context, e *domain.OnboardingEvent, since time.Time, limit int) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.recordErr != nil {
		return false, f.recordErr
	}
	n := 0
	for _, got := range f.events {
		sameTab := (got.Tab == nil && e.Tab == nil) || (got.Tab != nil && e.Tab != nil && *got.Tab == *e.Tab)
		if got.OrganizationID == e.OrganizationID && got.Event == e.Event && sameTab && !got.OccurredAt.Before(since) {
			n++
		}
	}
	if n >= limit {
		return false, nil
	}
	f.events = append(f.events, *e)
	return true, nil
}

func (f *fakeOnboardingRepo) RecordFirstAgent(_ context.Context, orgID uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.firstAgents = append(f.firstAgents, orgID)
	return nil
}

func (f *fakeOnboardingRepo) TimeToFirstAgentSamples(_ context.Context, _ *time.Time) ([]domain.TimeToFirstAgentSample, error) {
	return f.samples, f.samplesErr
}

func (f *fakeOnboardingRepo) CountEventsSince(_ context.Context, since time.Time) ([]domain.OnboardingEventCount, error) {
	f.countedSince = since
	return f.counts, f.countsErr
}

var onboardingTestNow = time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)

// newOnboardingTestService runs server-side writes before returning, so a test
// can assert on them without waiting.
func newOnboardingTestService() (*OnboardingTelemetryService, *fakeOnboardingRepo) {
	repo := &fakeOnboardingRepo{}
	svc := NewOnboardingTelemetryService(repo)
	svc.SetClock(func() time.Time { return onboardingTestNow })
	svc.SetDispatch(func(f func()) { f() })
	return svc, repo
}

func TestOnboardingRecordClientEvent_StoresOrgEventAndTimeOnly(t *testing.T) {
	svc, repo := newOnboardingTestService()
	orgID := uuid.New()

	require.NoError(t, svc.RecordClientEvent(context.Background(), orgID, domain.OnboardingEventViewed, ""))
	require.NoError(t, svc.RecordClientEvent(context.Background(), orgID, domain.OnboardingEventTabSelected, "typescript"))

	require.Len(t, repo.events, 2)
	assert.Equal(t, domain.OnboardingEvent{OrganizationID: orgID, Event: domain.OnboardingEventViewed, OccurredAt: onboardingTestNow}, repo.events[0])
	require.NotNil(t, repo.events[1].Tab)
	assert.Equal(t, "typescript", *repo.events[1].Tab)
}

func TestOnboardingRecordClientEvent_CapsRepeatsPerOrganizationEventAndTab(t *testing.T) {
	svc, repo := newOnboardingTestService()
	ctx := context.Background()
	orgID, other := uuid.New(), uuid.New()

	for i := 0; i < OnboardingClientEventCap; i++ {
		require.NoError(t, svc.RecordClientEvent(ctx, orgID, domain.OnboardingEventViewed, ""))
	}
	// A loop of reports past the cap stores nothing more.
	for i := 0; i < 50; i++ {
		assert.ErrorIs(t, svc.RecordClientEvent(ctx, orgID, domain.OnboardingEventViewed, ""), ErrOnboardingEventCapped)
	}
	assert.Len(t, repo.events, OnboardingClientEventCap)

	// The cap is per organization, per event and per tab.
	require.NoError(t, svc.RecordClientEvent(ctx, other, domain.OnboardingEventViewed, ""))
	require.NoError(t, svc.RecordClientEvent(ctx, orgID, domain.OnboardingEventSkipped, ""))
	require.NoError(t, svc.RecordClientEvent(ctx, orgID, domain.OnboardingEventTabSelected, "python"))
	require.NoError(t, svc.RecordClientEvent(ctx, orgID, domain.OnboardingEventTabSelected, "go"))

	// The window includes its start; once it has passed, the organization can
	// report again.
	svc.SetClock(func() time.Time { return onboardingTestNow.Add(OnboardingClientEventCapWindow) })
	assert.ErrorIs(t, svc.RecordClientEvent(ctx, orgID, domain.OnboardingEventViewed, ""), ErrOnboardingEventCapped)
	svc.SetClock(func() time.Time { return onboardingTestNow.Add(OnboardingClientEventCapWindow + time.Second) })
	require.NoError(t, svc.RecordClientEvent(ctx, orgID, domain.OnboardingEventViewed, ""))
}

func TestOnboardingRecordClientEvent_RefusesServerEventsAndBadInput(t *testing.T) {
	svc, repo := newOnboardingTestService()
	orgID := uuid.New()
	ctx := context.Background()

	cases := []struct {
		name  string
		org   uuid.UUID
		event domain.OnboardingEventType
		tab   string
		want  error
	}{
		{"token minted is server-only", orgID, domain.OnboardingEventTokenMinted, "", ErrOnboardingEventServerOnly},
		{"token exchanged is server-only", orgID, domain.OnboardingEventTokenExchanged, "", ErrOnboardingEventServerOnly},
		{"first agent is server-only", orgID, domain.OnboardingEventFirstAgentRegistered, "", ErrOnboardingEventServerOnly},
		{"unknown event", orgID, "agent_deleted", "", ErrOnboardingEventUnknown},
		{"tab_selected without a tab", orgID, domain.OnboardingEventTabSelected, "", ErrOnboardingTabInvalid},
		{"tab_selected with an unknown tab", orgID, domain.OnboardingEventTabSelected, "someone@example.com", ErrOnboardingTabInvalid},
		{"tab on another event", orgID, domain.OnboardingEventSkipped, "python", ErrOnboardingTabInvalid},
		{"no organization", uuid.Nil, domain.OnboardingEventViewed, "", ErrInvalidOrgOrUser},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := svc.RecordClientEvent(ctx, tc.org, tc.event, tc.tab)
			assert.ErrorIs(t, err, tc.want)
		})
	}
	assert.Empty(t, repo.events, "a refused event writes nothing")
}

func TestOnboardingServerEvents_AreRecordedForTheOrganization(t *testing.T) {
	svc, repo := newOnboardingTestService()
	orgID := uuid.New()

	svc.TokenMinted(orgID)
	svc.TokenExchanged(orgID)
	svc.FirstAgentRegistered(orgID)
	svc.TokenMinted(uuid.Nil) // no organization, nothing to key on

	require.Len(t, repo.events, 2)
	assert.Equal(t, domain.OnboardingEventTokenMinted, repo.events[0].Event)
	assert.Equal(t, domain.OnboardingEventTokenExchanged, repo.events[1].Event)
	for _, e := range repo.events {
		assert.Equal(t, orgID, e.OrganizationID)
		assert.Equal(t, onboardingTestNow, e.OccurredAt)
		assert.Nil(t, e.Tab)
	}
	assert.Equal(t, []uuid.UUID{orgID}, repo.firstAgents)
}

func TestOnboardingServerEvents_RunOffTheCallersPath(t *testing.T) {
	repo := &fakeOnboardingRepo{recordErr: errors.New("database unavailable")}
	svc := NewOnboardingTelemetryService(repo)
	var scheduled []func()
	svc.SetDispatch(func(f func()) { scheduled = append(scheduled, f) })

	svc.TokenMinted(uuid.New())
	svc.FirstAgentRegistered(uuid.New())

	assert.Len(t, scheduled, 2, "both writes are handed to the dispatcher, not run inline")
	for _, f := range scheduled {
		f() // a failing write is logged, never raised
	}
}

func TestOnboardingServerEvents_NilServiceRecordsNothing(t *testing.T) {
	var svc *OnboardingTelemetryService
	assert.NotPanics(t, func() {
		svc.TokenMinted(uuid.New())
		svc.TokenExchanged(uuid.New())
		svc.FirstAgentRegistered(uuid.New())
	})
}

func TestOnboardingBaseline_AllTimeRecentWindowAndFunnel(t *testing.T) {
	svc, repo := newOnboardingTestService()
	at := func(d time.Duration) *time.Time { t := onboardingTestNow.Add(d); return &t }
	repo.samples = []domain.TimeToFirstAgentSample{
		// Created 100 days ago, first agent 2 days later: in all time only.
		{OrganizationID: uuid.New(), OrgCreatedAt: onboardingTestNow.AddDate(0, 0, -100), FirstAgentAt: at(-98 * 24 * time.Hour)},
		// Created exactly at the window start: in the window.
		{OrganizationID: uuid.New(), OrgCreatedAt: onboardingTestNow.AddDate(0, 0, -30), FirstAgentAt: at(-30*24*time.Hour + 3*time.Minute)},
		// Created 5 days ago, no agent.
		{OrganizationID: uuid.New(), OrgCreatedAt: onboardingTestNow.AddDate(0, 0, -5)},
	}
	repo.counts = []domain.OnboardingEventCount{
		{Event: domain.OnboardingEventTokenMinted, Organizations: 2, Total: 5},
		{Event: domain.OnboardingEventViewed, Organizations: 3, Total: 9},
	}

	b, err := svc.Baseline(context.Background())
	require.NoError(t, err)

	assert.Equal(t, onboardingTestNow, b.GeneratedAt)
	assert.Equal(t, 30, b.WindowDays)
	assert.Equal(t, onboardingTestNow.AddDate(0, 0, -30), repo.countedSince)

	assert.Equal(t, 3, b.AllTime.Organizations)
	assert.Equal(t, 2, b.AllTime.OrganizationsWithAgent)
	assert.Equal(t, 2, b.Recent.Organizations)
	assert.Equal(t, 1, b.Recent.OrganizationsWithAgent)
	require.NotNil(t, b.Recent.MedianSeconds)
	assert.Equal(t, 180.0, *b.Recent.MedianSeconds)

	// Every event type, in funnel order, zero-filled.
	require.Len(t, b.Events, len(domain.OnboardingEventTypes))
	for i, e := range b.Events {
		assert.Equal(t, domain.OnboardingEventTypes[i], e.Event)
	}
	assert.Equal(t, domain.OnboardingEventCount{Event: domain.OnboardingEventViewed, Organizations: 3, Total: 9}, b.Events[0])
	assert.Equal(t, domain.OnboardingEventCount{Event: domain.OnboardingEventTabSelected}, b.Events[1])
	assert.Equal(t, domain.OnboardingEventCount{Event: domain.OnboardingEventTokenMinted, Organizations: 2, Total: 5}, b.Events[2])
}

func TestOnboardingBaseline_PropagatesReadErrors(t *testing.T) {
	svc, repo := newOnboardingTestService()
	repo.samplesErr = errors.New("boom")
	_, err := svc.Baseline(context.Background())
	assert.Error(t, err)

	svc, repo = newOnboardingTestService()
	repo.countsErr = errors.New("boom")
	_, err = svc.Baseline(context.Background())
	assert.Error(t, err)
}

// The token service reports a mint and an exchange to the onboarding sink.
func TestBootstrapToken_MintAndExchangeAreRecordedAsOnboardingEvents(t *testing.T) {
	rig := newBootstrapTestRig(t)
	svc, repo := newOnboardingTestService()
	rig.svc.SetOnboardingEvents(svc)
	orgID, userID := uuid.New(), uuid.New()

	minted := rig.mint(t, orgID, userID)
	_, err := rig.exchange(minted.Plaintext, BootstrapExchangeRequest{Name: "first"})
	require.NoError(t, err)
	_, err = rig.exchange(minted.Plaintext, BootstrapExchangeRequest{Name: "second"})
	require.Error(t, err, "a used token is refused")

	var got []domain.OnboardingEventType
	for _, e := range repo.events {
		assert.Equal(t, orgID, e.OrganizationID)
		got = append(got, e.Event)
	}
	assert.Equal(t, []domain.OnboardingEventType{domain.OnboardingEventTokenMinted, domain.OnboardingEventTokenExchanged}, got,
		"a refused exchange records nothing")
}

type recordingFirstAgentSink struct{ orgs []uuid.UUID }

func (s *recordingFirstAgentSink) FirstAgentRegistered(orgID uuid.UUID) {
	s.orgs = append(s.orgs, orgID)
}

// Every successful registration tells the sink; the sink's single-row rule
// keeps only the first. A failed registration tells it nothing.
func TestAgentService_CreateAgent_ReportsRegistrationToOnboardingSink(t *testing.T) {
	masterKey := base64.StdEncoding.EncodeToString([]byte("test-master-key-32-bytes-long!!!"))
	keyVault, _ := crypto.NewKeyVault(masterKey)
	newService := func(createErr error) (*AgentService, *recordingFirstAgentSink) {
		repo := new(MockAgentRepository)
		repo.On("Create", mock.AnythingOfType("*domain.Agent")).Return(createErr)
		repo.On("Update", mock.AnythingOfType("*domain.Agent")).Return(nil)
		calc := new(AgentServiceMockTrustScoreCalculator)
		calc.On("Calculate", mock.AnythingOfType("*domain.Agent")).Return(&domain.TrustScore{ID: uuid.New(), Score: 0.5}, nil)
		scores := new(AgentServiceMockTrustScoreRepository)
		scores.On("Create", mock.AnythingOfType("*domain.TrustScore")).Return(nil)
		svc := &AgentService{agentRepo: repo, trustCalc: calc, trustScoreRepo: scores, keyVault: keyVault, capabilityRepo: new(MockCapabilityRepository)}
		sink := &recordingFirstAgentSink{}
		svc.SetOnboardingEvents(sink)
		return svc, sink
	}
	req := func() *CreateAgentRequest {
		return &CreateAgentRequest{Name: "agent", DisplayName: "Agent", AgentType: domain.AgentTypeAI, Version: "1.0.0"}
	}
	orgID := uuid.New()

	svc, sink := newService(nil)
	_, err := svc.CreateAgent(context.Background(), req(), orgID, uuid.New(), nil, nil, "")
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{orgID}, sink.orgs)

	svc, sink = newService(errors.New("database error"))
	_, err = svc.CreateAgent(context.Background(), req(), orgID, uuid.New(), nil, nil, "")
	require.Error(t, err)
	assert.Empty(t, sink.orgs)
}
