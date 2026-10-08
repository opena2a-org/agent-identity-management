package domain

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var ttfaOrgCreated = time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)

// ttfaSample is an organization created at ttfaOrgCreated whose first agent
// arrived after d, or that has no agent when d is nil.
func ttfaSample(d *time.Duration) TimeToFirstAgentSample {
	s := TimeToFirstAgentSample{OrganizationID: uuid.New(), OrgCreatedAt: ttfaOrgCreated}
	if d != nil {
		at := ttfaOrgCreated.Add(*d)
		s.FirstAgentAt = &at
	}
	return s
}

func after(d time.Duration) *time.Duration { return &d }

func bucketCounts(stats TimeToFirstAgentStats) map[string]int {
	out := make(map[string]int, len(stats.Buckets))
	for _, b := range stats.Buckets {
		out[b.Label] = b.Count
	}
	return out
}

func TestComputeTimeToFirstAgent_NoOrganizations(t *testing.T) {
	stats := ComputeTimeToFirstAgent(nil)

	assert.Equal(t, 0, stats.Organizations)
	assert.Equal(t, 0, stats.OrganizationsWithAgent)
	assert.Equal(t, 0.0, stats.ConversionRate)
	assert.Nil(t, stats.MedianSeconds)
	assert.Nil(t, stats.P75Seconds)
	assert.Nil(t, stats.P90Seconds)
	assert.Nil(t, stats.FastestSeconds)
	assert.Len(t, stats.Buckets, 6, "every bucket is present even when empty")
}

func TestComputeTimeToFirstAgent_OrganizationsWithoutAgentsHaveNoPercentiles(t *testing.T) {
	stats := ComputeTimeToFirstAgent([]TimeToFirstAgentSample{ttfaSample(nil), ttfaSample(nil)})

	assert.Equal(t, 2, stats.Organizations)
	assert.Equal(t, 0, stats.OrganizationsWithAgent)
	assert.Equal(t, 0.0, stats.ConversionRate)
	assert.Nil(t, stats.MedianSeconds, "no agent means no measurement, not zero seconds")
}

func TestComputeTimeToFirstAgent_IsFirstAgentMinusOrgCreation(t *testing.T) {
	stats := ComputeTimeToFirstAgent([]TimeToFirstAgentSample{ttfaSample(after(90 * time.Second))})

	require.NotNil(t, stats.MedianSeconds)
	assert.Equal(t, 90.0, *stats.MedianSeconds)
	assert.Equal(t, 90.0, *stats.FastestSeconds)
	assert.Equal(t, 1.0, stats.ConversionRate)
}

func TestComputeTimeToFirstAgent_NearestRankPercentiles(t *testing.T) {
	// 10 organizations with agents after 1..10 minutes, 2 without.
	samples := []TimeToFirstAgentSample{ttfaSample(nil), ttfaSample(nil)}
	for i := 10; i >= 1; i-- { // unsorted input
		samples = append(samples, ttfaSample(after(time.Duration(i)*time.Minute)))
	}

	stats := ComputeTimeToFirstAgent(samples)

	assert.Equal(t, 12, stats.Organizations)
	assert.Equal(t, 10, stats.OrganizationsWithAgent)
	assert.InDelta(t, 10.0/12.0, stats.ConversionRate, 1e-9)
	// Nearest rank over n=10: p50 -> rank 5, p75 -> rank 8, p90 -> rank 9.
	assert.Equal(t, 5*60.0, *stats.MedianSeconds)
	assert.Equal(t, 8*60.0, *stats.P75Seconds)
	assert.Equal(t, 9*60.0, *stats.P90Seconds)
	assert.Equal(t, 60.0, *stats.FastestSeconds)
}

func TestComputeTimeToFirstAgent_BucketBoundsAreUpperExclusive(t *testing.T) {
	samples := []TimeToFirstAgentSample{
		ttfaSample(after(0)),                    // under 1 minute
		ttfaSample(after(59 * time.Second)),     // under 1 minute
		ttfaSample(after(60 * time.Second)),     // 1 to 5 minutes: 60 s is not under 1 minute
		ttfaSample(after(5 * time.Minute)),      // 5 to 60 minutes
		ttfaSample(after(59 * time.Minute)),     // 5 to 60 minutes
		ttfaSample(after(2 * time.Hour)),        // 1 to 24 hours
		ttfaSample(after(3 * 24 * time.Hour)),   // 1 to 7 days
		ttfaSample(after(7 * 24 * time.Hour)),   // over 7 days
		ttfaSample(after(400 * 24 * time.Hour)), // over 7 days
		ttfaSample(nil),                         // no agent: in no bucket
	}

	stats := ComputeTimeToFirstAgent(samples)

	assert.Equal(t, map[string]int{
		"under 1 minute":  2,
		"1 to 5 minutes":  1,
		"5 to 60 minutes": 2,
		"1 to 24 hours":   1,
		"1 to 7 days":     1,
		"over 7 days":     2,
	}, bucketCounts(stats))
	assert.Nil(t, stats.Buckets[len(stats.Buckets)-1].UpperSeconds, "the last bucket is open-ended")
}

func TestComputeTimeToFirstAgent_AgentOlderThanOrgIsCountedButNotMeasured(t *testing.T) {
	// Seeded or migrated data can hold an agent created before its organization.
	samples := []TimeToFirstAgentSample{
		ttfaSample(after(-time.Hour)),
		ttfaSample(after(2 * time.Minute)),
	}

	stats := ComputeTimeToFirstAgent(samples)

	assert.Equal(t, 2, stats.OrganizationsWithAgent)
	assert.Equal(t, 1, stats.ExcludedNegative)
	assert.Equal(t, 120.0, *stats.MedianSeconds, "the negative duration does not pull the median down")
	total := 0
	for _, b := range stats.Buckets {
		total += b.Count
	}
	assert.Equal(t, 1, total)
}

func TestTimeToFirstAgentStats_JSONIsCamelCaseAndNullWhenUnmeasured(t *testing.T) {
	raw, err := json.Marshal(ComputeTimeToFirstAgent([]TimeToFirstAgentSample{ttfaSample(nil)}))
	require.NoError(t, err)

	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &decoded))
	for _, key := range []string{"organizations", "organizationsWithAgent", "conversionRate", "medianSeconds", "p75Seconds", "p90Seconds", "fastestSeconds", "buckets", "excludedNegative"} {
		assert.Contains(t, decoded, key)
	}
	assert.Nil(t, decoded["medianSeconds"])
	assert.NotNil(t, decoded["buckets"], "buckets is an array, never null")
}

func TestOnboardingEventType_ClientCannotReportServerEvents(t *testing.T) {
	for _, e := range OnboardingEventTypes {
		assert.True(t, e.IsValid(), e)
	}
	assert.False(t, OnboardingEventType("agent_deleted").IsValid())
	assert.False(t, OnboardingEventType("").IsValid())

	assert.True(t, OnboardingEventViewed.IsClientReported())
	assert.True(t, OnboardingEventTabSelected.IsClientReported())
	assert.True(t, OnboardingEventCompleted.IsClientReported())
	assert.True(t, OnboardingEventSkipped.IsClientReported())
	assert.False(t, OnboardingEventTokenMinted.IsClientReported())
	assert.False(t, OnboardingEventTokenExchanged.IsClientReported())
	assert.False(t, OnboardingEventFirstAgentRegistered.IsClientReported())
}

func TestIsValidOnboardingTab_IsAClosedSet(t *testing.T) {
	assert.True(t, IsValidOnboardingTab("python"))
	assert.True(t, IsValidOnboardingTab("typescript"))
	assert.False(t, IsValidOnboardingTab("Python"))
	assert.False(t, IsValidOnboardingTab(""))
	assert.False(t, IsValidOnboardingTab("someone@example.com"))
}
