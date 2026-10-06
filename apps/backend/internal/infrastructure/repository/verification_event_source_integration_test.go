//go:build integration

package repository

import (
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// Trust scoring counts only events whose outcome the server observed: source
// service or system. An event a caller or the agent reported counts as zero
// observations in every statistic GetAgentStatistics returns. The filter is an
// allowlist, so a source value nobody has ruled on also counts nothing.
//
// The predicate lives in SQL, so these tests drive the real repository against
// a migrated Postgres:
//
//	TEST_DATABASE_URL=postgres://... go test -tags=integration \
//	  -run TestVerificationEventSource ./internal/infrastructure/repository/...

type sourceFixture struct {
	db      *sql.DB
	repo    *VerificationEventRepositorySimple
	orgID   uuid.UUID
	agentID uuid.UUID
	cutover time.Time
}

func newSourceFixture(t *testing.T) *sourceFixture {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping verification event source integration test")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping())

	f := &sourceFixture{db: db, repo: &VerificationEventRepositorySimple{db: db}, orgID: uuid.New(), agentID: uuid.New()}
	userID := uuid.New()
	tag := f.orgID.String()[:8]

	_, err = db.Exec(`
        INSERT INTO organizations
            (id, name, domain, plan_type, max_agents, max_users, is_active,
             created_at, updated_at, enforcement_mode,
             community_intelligence_enabled)
        VALUES ($1, $2, $3, 'free', 100, 100, true, NOW(), NOW(), 'monitoring', false)`,
		f.orgID, "event-source-"+tag, "event-source-"+tag+".invalid")
	require.NoError(t, err, "seed organization")

	_, err = db.Exec(`
        INSERT INTO users
            (id, organization_id, email, name, role, provider, provider_id,
             created_at, updated_at, status)
        VALUES ($1, $2, $3, 'Event Source Fixture', 'member', 'local', $4, NOW(), NOW(), 'active')`,
		userID, f.orgID, "event-source-"+tag+"@fixture.invalid", userID.String())
	require.NoError(t, err, "seed user")

	_, err = db.Exec(`
        INSERT INTO agents
            (id, organization_id, name, display_name, agent_type, status,
             created_at, updated_at, created_by)
        VALUES ($1, $2, $3, 'Event Source Fixture', 'service', 'active', NOW(), NOW(), $4)`,
		f.agentID, f.orgID, "event-source-agent-"+tag, userID)
	require.NoError(t, err, "seed agent")

	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM verification_events WHERE organization_id = $1`, f.orgID)
		_, _ = db.Exec(`DELETE FROM agents WHERE organization_id = $1`, f.orgID)
		_, _ = db.Exec(`DELETE FROM users WHERE organization_id = $1`, f.orgID)
		_, _ = db.Exec(`DELETE FROM organizations WHERE id = $1`, f.orgID)
	})

	require.NoError(t, db.QueryRow(
		`SELECT source_added_at FROM verification_event_source_cutover`).Scan(&f.cutover),
		"migration 124 records when the source column was added")
	return f
}

// create writes an event through the repository, as every server writer does.
func (f *sourceFixture) create(t *testing.T, source domain.VerificationEventSource, status domain.VerificationEventStatus, confidence float64, durationMs int) *domain.VerificationEvent {
	t.Helper()
	ev := &domain.VerificationEvent{
		OrganizationID:   f.orgID,
		AgentID:          &f.agentID,
		Protocol:         domain.VerificationProtocolA2A,
		VerificationType: domain.VerificationTypeCapability,
		Status:           status,
		Confidence:       confidence,
		DurationMs:       durationMs,
		InitiatorType:    domain.InitiatorTypeAgent,
		StartedAt:        time.Now(),
		Source:           source,
	}
	require.NoError(t, f.repo.Create(ev))
	return ev
}

// insertRaw writes a row the repository would refuse: a NULL source as rows
// written before the column existed carry, or a source value no writer sets.
func (f *sourceFixture) insertRaw(t *testing.T, source sql.NullString, status string, createdAt time.Time) {
	t.Helper()
	_, err := f.db.Exec(`
        INSERT INTO verification_events
            (organization_id, agent_id, protocol, verification_type, status,
             confidence, duration_ms, initiator_type, started_at, created_at, source)
        VALUES ($1, $2, 'A2A', 'capability', $3, 0.5, 10, 'agent', $4, $4, $5)`,
		f.orgID, f.agentID, status, createdAt, source)
	require.NoError(t, err)
}

func (f *sourceFixture) stats(t *testing.T) *domain.AgentVerificationStatistics {
	t.Helper()
	s, err := f.repo.GetAgentStatistics(f.agentID, f.cutover.Add(-2*time.Hour), time.Now().Add(time.Hour))
	require.NoError(t, err)
	return s
}

// Cells (a) and (b): an asserted success next to an observed one adds nothing
// to the total, the success count, or the averages.
func TestVerificationEventSource_AssertedEventsCountNothing(t *testing.T) {
	for _, asserted := range []domain.VerificationEventSource{
		domain.VerificationEventSourceCallerReported,
		domain.VerificationEventSourceAgentReported,
	} {
		t.Run(string(asserted), func(t *testing.T) {
			f := newSourceFixture(t)
			f.create(t, domain.VerificationEventSourceService, domain.VerificationEventStatusSuccess, 0.9, 100)
			f.create(t, asserted, domain.VerificationEventStatusSuccess, 0.1, 900)

			s := f.stats(t)
			assert.Equal(t, 1, s.TotalVerifications, "an asserted event is not an observation")
			assert.Equal(t, 1, s.SuccessCount)
			assert.Equal(t, 0, s.FailedCount)
			assert.InDelta(t, 0.9, s.AvgConfidence, 1e-9, "the asserted row's confidence is not averaged in")
			assert.InDelta(t, 100, s.AvgDurationMs, 1e-9, "the asserted row's duration is not averaged in")
		})
	}
}

// An asserted failure does not lower the ratio either: asserted rows are
// absent from the statistic, not counted against the agent.
func TestVerificationEventSource_AssertedFailureCountsNothing(t *testing.T) {
	f := newSourceFixture(t)
	f.create(t, domain.VerificationEventSourceSystem, domain.VerificationEventStatusSuccess, 1, 0)
	f.create(t, domain.VerificationEventSourceCallerReported, domain.VerificationEventStatusFailed, 0, 0)

	s := f.stats(t)
	assert.Equal(t, 1, s.TotalVerifications)
	assert.Equal(t, 0, s.FailedCount)
	assert.InDelta(t, 1.0, s.SuccessRate, 1e-9)
}

// Cell (c): a NULL-source row written before the column existed keeps
// counting; a NULL-source row written after it counts nothing.
func TestVerificationEventSource_NullSourceCountsOnlyBeforeCutover(t *testing.T) {
	f := newSourceFixture(t)
	f.insertRaw(t, sql.NullString{}, "success", f.cutover.Add(-time.Hour))
	f.insertRaw(t, sql.NullString{}, "failed", f.cutover.Add(time.Second))

	s := f.stats(t)
	assert.Equal(t, 1, s.TotalVerifications, "only the pre-cutover NULL row counts")
	assert.Equal(t, 1, s.SuccessCount)
	assert.Equal(t, 0, s.FailedCount, "a NULL row after the cutover is a write defect and counts nothing")
}

// Cell (d): a source value that is not on the allowlist counts nothing. A
// filter written as an exclusion of the asserted sources would count it.
func TestVerificationEventSource_UnknownSourceCountsNothing(t *testing.T) {
	f := newSourceFixture(t)
	f.insertRaw(t, sql.NullString{String: "future_source", Valid: true}, "success", time.Now())

	s := f.stats(t)
	assert.Equal(t, 0, s.TotalVerifications)
	assert.Equal(t, 0, s.SuccessCount)
}

// Cell (e): last_verification is the newest observed event, not a newer
// asserted one.
func TestVerificationEventSource_LastVerificationIgnoresAssertedEvents(t *testing.T) {
	f := newSourceFixture(t)
	observed := f.create(t, domain.VerificationEventSourceService, domain.VerificationEventStatusSuccess, 1, 0)
	f.create(t, domain.VerificationEventSourceCallerReported, domain.VerificationEventStatusSuccess, 1, 0)
	f.create(t, domain.VerificationEventSourceAgentReported, domain.VerificationEventStatusSuccess, 1, 0)

	s := f.stats(t)
	assert.True(t, s.LastVerification.Equal(observed.CreatedAt),
		"last_verification = %s, want the observed event's %s", s.LastVerification, observed.CreatedAt)
}

// Create stores the source it is given, and refuses an event without a known
// source rather than write a NULL-source row after the cutover.
func TestVerificationEventSource_CreateStoresAndRequiresSource(t *testing.T) {
	f := newSourceFixture(t)

	ev := f.create(t, domain.VerificationEventSourceAgentReported, domain.VerificationEventStatusSuccess, 1, 0)
	var stored sql.NullString
	require.NoError(t, f.db.QueryRow(`SELECT source FROM verification_events WHERE id = $1`, ev.ID).Scan(&stored))
	assert.Equal(t, "agent_reported", stored.String)

	for _, bad := range []domain.VerificationEventSource{"", "future_source"} {
		err := f.repo.Create(&domain.VerificationEvent{
			OrganizationID: f.orgID, AgentID: &f.agentID,
			Protocol: domain.VerificationProtocolA2A, VerificationType: domain.VerificationTypeCapability,
			Status: domain.VerificationEventStatusSuccess, InitiatorType: domain.InitiatorTypeAgent,
			StartedAt: time.Now(), Source: bad,
		})
		assert.ErrorIs(t, err, domain.ErrVerificationEventSourceRequired, "source %q", bad)
	}

	var n int
	require.NoError(t, f.db.QueryRow(
		`SELECT COUNT(*) FROM verification_events WHERE agent_id = $1`, f.agentID).Scan(&n))
	assert.Equal(t, 1, n, "a refused event writes no row")
}
