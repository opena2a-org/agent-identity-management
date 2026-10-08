package application

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

var (
	ErrOnboardingEventUnknown    = errors.New("unknown onboarding event")
	ErrOnboardingEventServerOnly = errors.New("this onboarding event is recorded by the server")
	ErrOnboardingTabInvalid      = errors.New("tab_selected needs a known tab, and only tab_selected takes one")
)

// OnboardingBaselineWindowDays is the recent window the platform panel reports
// next to the all-time baseline.
const OnboardingBaselineWindowDays = 30

// onboardingRecordTimeout bounds a server-side event write, which runs after
// the request that caused it has been answered.
const onboardingRecordTimeout = 5 * time.Second

// OnboardingTelemetryService records onboarding events and computes the
// time-to-first-agent baseline.
//
// Server-side events (token minted, token exchanged, first agent registered)
// are written off the request path, the same way the API call analytics are:
// telemetry is observability and never fails or slows the request it
// describes. A nil *OnboardingTelemetryService records nothing.
type OnboardingTelemetryService struct {
	repo     domain.OnboardingEventRepository
	now      func() time.Time
	dispatch func(func())
}

// NewOnboardingTelemetryService creates an OnboardingTelemetryService.
func NewOnboardingTelemetryService(repo domain.OnboardingEventRepository) *OnboardingTelemetryService {
	return &OnboardingTelemetryService{
		repo:     repo,
		now:      time.Now,
		dispatch: func(f func()) { go f() },
	}
}

// SetClock replaces the service clock. Tests only.
func (s *OnboardingTelemetryService) SetClock(now func() time.Time) {
	s.now = now
}

// SetDispatch replaces how server-side writes are scheduled. Tests pass a
// function that runs the write before returning.
func (s *OnboardingTelemetryService) SetDispatch(dispatch func(func())) {
	s.dispatch = dispatch
}

// RecordClientEvent records an event the dashboard reports for orgID. tab is
// required for tab_selected and refused for every other event.
func (s *OnboardingTelemetryService) RecordClientEvent(ctx context.Context, orgID uuid.UUID, event domain.OnboardingEventType, tab string) error {
	if orgID == uuid.Nil {
		return ErrInvalidOrgOrUser
	}
	if !event.IsValid() {
		return ErrOnboardingEventUnknown
	}
	if !event.IsClientReported() {
		return ErrOnboardingEventServerOnly
	}
	e := &domain.OnboardingEvent{OrganizationID: orgID, Event: event, OccurredAt: s.now().UTC()}
	if event == domain.OnboardingEventTabSelected {
		if !domain.IsValidOnboardingTab(tab) {
			return ErrOnboardingTabInvalid
		}
		e.Tab = &tab
	} else if tab != "" {
		return ErrOnboardingTabInvalid
	}
	return s.repo.Record(ctx, e)
}

// TokenMinted records token_minted for orgID.
func (s *OnboardingTelemetryService) TokenMinted(orgID uuid.UUID) {
	s.recordServerEvent(orgID, domain.OnboardingEventTokenMinted)
}

// TokenExchanged records token_exchanged for orgID.
func (s *OnboardingTelemetryService) TokenExchanged(orgID uuid.UUID) {
	s.recordServerEvent(orgID, domain.OnboardingEventTokenExchanged)
}

// FirstAgentRegistered records first_agent_registered for orgID unless it is
// already recorded. Called after every agent registration; only the first one
// writes a row.
func (s *OnboardingTelemetryService) FirstAgentRegistered(orgID uuid.UUID) {
	if s == nil || orgID == uuid.Nil {
		return
	}
	s.dispatch(func() {
		ctx, cancel := context.WithTimeout(context.Background(), onboardingRecordTimeout)
		defer cancel()
		if err := s.repo.RecordFirstAgent(ctx, orgID); err != nil {
			log.Printf("onboarding telemetry: first agent for organization %s: %v", orgID, err)
		}
	})
}

func (s *OnboardingTelemetryService) recordServerEvent(orgID uuid.UUID, event domain.OnboardingEventType) {
	if s == nil || orgID == uuid.Nil {
		return
	}
	occurredAt := s.now().UTC()
	s.dispatch(func() {
		ctx, cancel := context.WithTimeout(context.Background(), onboardingRecordTimeout)
		defer cancel()
		if err := s.repo.Record(ctx, &domain.OnboardingEvent{OrganizationID: orgID, Event: event, OccurredAt: occurredAt}); err != nil {
			log.Printf("onboarding telemetry: %s for organization %s: %v", event, orgID, err)
		}
	})
}

// OnboardingBaseline is what the platform panel shows: time to first agent
// over every organization and over organizations created in the recent
// window, and the onboarding events recorded in that window.
type OnboardingBaseline struct {
	GeneratedAt time.Time                     `json:"generatedAt"`
	WindowDays  int                           `json:"windowDays"`
	AllTime     domain.TimeToFirstAgentStats  `json:"allTime"`
	Recent      domain.TimeToFirstAgentStats  `json:"recent"`
	Events      []domain.OnboardingEventCount `json:"events"`
}

// Baseline computes the onboarding baseline. Events lists every event type in
// funnel order, with zero counts for types not seen in the window.
func (s *OnboardingTelemetryService) Baseline(ctx context.Context) (*OnboardingBaseline, error) {
	now := s.now().UTC()
	windowStart := now.AddDate(0, 0, -OnboardingBaselineWindowDays)

	samples, err := s.repo.TimeToFirstAgentSamples(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("onboarding baseline: %w", err)
	}
	recent := make([]domain.TimeToFirstAgentSample, 0, len(samples))
	for _, sample := range samples {
		if !sample.OrgCreatedAt.Before(windowStart) {
			recent = append(recent, sample)
		}
	}

	counts, err := s.repo.CountEventsSince(ctx, windowStart)
	if err != nil {
		return nil, fmt.Errorf("onboarding baseline: %w", err)
	}
	byEvent := make(map[domain.OnboardingEventType]domain.OnboardingEventCount, len(counts))
	for _, c := range counts {
		byEvent[c.Event] = c
	}
	events := make([]domain.OnboardingEventCount, 0, len(domain.OnboardingEventTypes))
	for _, t := range domain.OnboardingEventTypes {
		c := byEvent[t]
		c.Event = t
		events = append(events, c)
	}

	return &OnboardingBaseline{
		GeneratedAt: now,
		WindowDays:  OnboardingBaselineWindowDays,
		AllTime:     domain.ComputeTimeToFirstAgent(samples),
		Recent:      domain.ComputeTimeToFirstAgent(recent),
		Events:      events,
	}, nil
}
