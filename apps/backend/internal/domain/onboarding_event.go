package domain

import (
	"context"
	"math"
	"sort"
	"time"

	"github.com/google/uuid"
)

// Onboarding telemetry records how an organization moves from sign-up to its
// first agent. An event carries the organization, the event name, an optional
// SDK tab and a timestamp. It never carries a user, an email address, a client
// address, a user agent or free text: the event name and tab are closed sets.
type OnboardingEventType string

const (
	OnboardingEventViewed               OnboardingEventType = "onboarding_viewed"
	OnboardingEventTabSelected          OnboardingEventType = "tab_selected"
	OnboardingEventTokenMinted          OnboardingEventType = "token_minted"
	OnboardingEventTokenExchanged       OnboardingEventType = "token_exchanged"
	OnboardingEventFirstAgentRegistered OnboardingEventType = "first_agent_registered"
	OnboardingEventCompleted            OnboardingEventType = "onboarding_completed"
	OnboardingEventSkipped              OnboardingEventType = "onboarding_skipped"
)

// OnboardingEventTypes lists every event in funnel order.
var OnboardingEventTypes = []OnboardingEventType{
	OnboardingEventViewed,
	OnboardingEventTabSelected,
	OnboardingEventTokenMinted,
	OnboardingEventTokenExchanged,
	OnboardingEventFirstAgentRegistered,
	OnboardingEventCompleted,
	OnboardingEventSkipped,
}

// IsValid reports whether e is a known event.
func (e OnboardingEventType) IsValid() bool {
	for _, t := range OnboardingEventTypes {
		if e == t {
			return true
		}
	}
	return false
}

// IsClientReported reports whether a dashboard may report e. Token and agent
// events are recorded by the server where they happen, so a client cannot add
// them to the funnel.
func (e OnboardingEventType) IsClientReported() bool {
	switch e {
	case OnboardingEventViewed, OnboardingEventTabSelected, OnboardingEventCompleted, OnboardingEventSkipped:
		return true
	}
	return false
}

// OnboardingTabs is the closed set of SDK tabs a tab_selected event may name.
var OnboardingTabs = []string{"python", "typescript", "java", "go", "cli", "mcp", "claude", "cursor"}

// IsValidOnboardingTab reports whether tab is in OnboardingTabs.
func IsValidOnboardingTab(tab string) bool {
	for _, t := range OnboardingTabs {
		if tab == t {
			return true
		}
	}
	return false
}

// OnboardingEvent is one recorded step. Tab is set only on tab_selected.
type OnboardingEvent struct {
	OrganizationID uuid.UUID
	Event          OnboardingEventType
	Tab            *string
	OccurredAt     time.Time
}

// OnboardingEventCount is how many events of one type were recorded in a
// window, and by how many distinct organizations.
type OnboardingEventCount struct {
	Event         OnboardingEventType `json:"event"`
	Organizations int                 `json:"organizations"`
	Total         int                 `json:"total"`
}

// TimeToFirstAgentSample is one organization's creation time and the creation
// time of its earliest agent, nil when it has none.
type TimeToFirstAgentSample struct {
	OrganizationID uuid.UUID
	OrgCreatedAt   time.Time
	FirstAgentAt   *time.Time
}

// OnboardingEventRepository stores onboarding events and reads the columns the
// time-to-first-agent metric is computed from.
type OnboardingEventRepository interface {
	// Record inserts one event.
	Record(ctx context.Context, e *OnboardingEvent) error
	// RecordFirstAgent records first_agent_registered for orgID, timestamped
	// with its earliest agent's created_at. It records nothing when the
	// organization has no agent or already has the event.
	RecordFirstAgent(ctx context.Context, orgID uuid.UUID) error
	// TimeToFirstAgentSamples returns one sample per organization created at
	// or after since, or per organization when since is nil. It reads only
	// organizations.created_at and agents.created_at, so it measures every
	// organization that exists, including those created before events were
	// recorded.
	TimeToFirstAgentSamples(ctx context.Context, since *time.Time) ([]TimeToFirstAgentSample, error)
	// CountEventsSince counts events per type recorded at or after since.
	CountEventsSince(ctx context.Context, since time.Time) ([]OnboardingEventCount, error)
}

// TimeToFirstAgentBucket counts organizations whose first agent arrived in
// [previous bucket's bound, UpperSeconds). UpperSeconds is nil on the last,
// open-ended bucket.
type TimeToFirstAgentBucket struct {
	Label        string   `json:"label"`
	UpperSeconds *float64 `json:"upperSeconds"`
	Count        int      `json:"count"`
}

// TimeToFirstAgentStats summarises time_to_first_agent, the first agent's
// created_at minus the organization's created_at, over a set of organizations.
// Percentiles use the nearest-rank method and are nil when no organization in
// the set has an agent.
type TimeToFirstAgentStats struct {
	Organizations          int                      `json:"organizations"`
	OrganizationsWithAgent int                      `json:"organizationsWithAgent"`
	ConversionRate         float64                  `json:"conversionRate"`
	MedianSeconds          *float64                 `json:"medianSeconds"`
	P75Seconds             *float64                 `json:"p75Seconds"`
	P90Seconds             *float64                 `json:"p90Seconds"`
	FastestSeconds         *float64                 `json:"fastestSeconds"`
	Buckets                []TimeToFirstAgentBucket `json:"buckets"`
	// ExcludedNegative counts organizations whose earliest agent predates the
	// organization (seeded or migrated data). They count as having an agent
	// but are left out of the percentiles and buckets.
	ExcludedNegative int `json:"excludedNegative"`
}

// timeToFirstAgentBounds are the bucket upper bounds, in seconds.
var timeToFirstAgentBounds = []struct {
	label string
	upper float64
}{
	{"under 1 minute", 60},
	{"1 to 5 minutes", 5 * 60},
	{"5 to 60 minutes", 60 * 60},
	{"1 to 24 hours", 24 * 60 * 60},
	{"1 to 7 days", 7 * 24 * 60 * 60},
}

// ComputeTimeToFirstAgent summarises samples.
func ComputeTimeToFirstAgent(samples []TimeToFirstAgentSample) TimeToFirstAgentStats {
	stats := TimeToFirstAgentStats{Organizations: len(samples)}

	durations := make([]float64, 0, len(samples))
	for _, s := range samples {
		if s.FirstAgentAt == nil {
			continue
		}
		stats.OrganizationsWithAgent++
		d := s.FirstAgentAt.Sub(s.OrgCreatedAt).Seconds()
		if d < 0 {
			stats.ExcludedNegative++
			continue
		}
		durations = append(durations, d)
	}
	if stats.Organizations > 0 {
		stats.ConversionRate = float64(stats.OrganizationsWithAgent) / float64(stats.Organizations)
	}

	stats.Buckets = make([]TimeToFirstAgentBucket, 0, len(timeToFirstAgentBounds)+1)
	for _, b := range timeToFirstAgentBounds {
		upper := b.upper
		stats.Buckets = append(stats.Buckets, TimeToFirstAgentBucket{Label: b.label, UpperSeconds: &upper})
	}
	stats.Buckets = append(stats.Buckets, TimeToFirstAgentBucket{Label: "over 7 days"})

	for _, d := range durations {
		i := 0
		for i < len(timeToFirstAgentBounds) && d >= timeToFirstAgentBounds[i].upper {
			i++
		}
		stats.Buckets[i].Count++
	}

	if len(durations) == 0 {
		return stats
	}
	sort.Float64s(durations)
	stats.MedianSeconds = nearestRank(durations, 50)
	stats.P75Seconds = nearestRank(durations, 75)
	stats.P90Seconds = nearestRank(durations, 90)
	stats.FastestSeconds = nearestRank(durations, 0)
	return stats
}

// nearestRank returns the p-th percentile of sorted, a non-empty ascending
// slice: the smallest value with at least p percent of values at or below it.
func nearestRank(sorted []float64, p float64) *float64 {
	rank := int(math.Ceil(p / 100 * float64(len(sorted))))
	if rank < 1 {
		rank = 1
	}
	v := sorted[rank-1]
	return &v
}
