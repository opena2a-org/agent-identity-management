package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// OnboardingEventRepository implements domain.OnboardingEventRepository using
// PostgreSQL.
type OnboardingEventRepository struct {
	db *sql.DB
}

// NewOnboardingEventRepository creates a new OnboardingEventRepository.
func NewOnboardingEventRepository(db *sql.DB) *OnboardingEventRepository {
	return &OnboardingEventRepository{db: db}
}

var _ domain.OnboardingEventRepository = (*OnboardingEventRepository)(nil)

// Record inserts one event. A zero OccurredAt takes the database's NOW().
func (r *OnboardingEventRepository) Record(ctx context.Context, e *domain.OnboardingEvent) error {
	var occurredAt interface{}
	if !e.OccurredAt.IsZero() {
		occurredAt = e.OccurredAt
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO onboarding_events (organization_id, event, tab, occurred_at)
		VALUES ($1, $2, $3, COALESCE($4::timestamptz, NOW()))
	`, e.OrganizationID, string(e.Event), e.Tab, occurredAt)
	if err != nil {
		return fmt.Errorf("record onboarding event: %w", err)
	}
	return nil
}

// recordFirstAgentQuery stamps the event with the earliest agent's created_at,
// not the time of the call, so a late or repeated call records the same
// instant. An organization with no agent yields no row; one that already has
// the event hits the partial unique index and is left alone.
const recordFirstAgentQuery = `
	INSERT INTO onboarding_events (organization_id, event, occurred_at)
	SELECT organization_id, 'first_agent_registered', MIN(created_at)
	FROM agents
	WHERE organization_id = $1
	GROUP BY organization_id
	ON CONFLICT (organization_id) WHERE event = 'first_agent_registered' DO NOTHING
`

// RecordFirstAgent records first_agent_registered for orgID once.
func (r *OnboardingEventRepository) RecordFirstAgent(ctx context.Context, orgID uuid.UUID) error {
	if _, err := r.db.ExecContext(ctx, recordFirstAgentQuery, orgID); err != nil {
		return fmt.Errorf("record first agent: %w", err)
	}
	return nil
}

// timeToFirstAgentQuery is the baseline: it reads only columns every
// organization and agent already has, so it measures organizations created
// before any onboarding event was recorded. An agent deleted since registration
// no longer counts; the earliest remaining agent stands in for it.
const timeToFirstAgentQuery = `
	SELECT o.id, o.created_at, MIN(a.created_at)
	FROM organizations o
	LEFT JOIN agents a ON a.organization_id = o.id
	WHERE $1::timestamptz IS NULL OR o.created_at >= $1::timestamptz
	GROUP BY o.id, o.created_at
	ORDER BY o.created_at
`

// TimeToFirstAgentSamples returns one sample per organization.
func (r *OnboardingEventRepository) TimeToFirstAgentSamples(ctx context.Context, since *time.Time) ([]domain.TimeToFirstAgentSample, error) {
	var sinceArg interface{}
	if since != nil {
		sinceArg = *since
	}
	rows, err := r.db.QueryContext(ctx, timeToFirstAgentQuery, sinceArg)
	if err != nil {
		return nil, fmt.Errorf("query time to first agent: %w", err)
	}
	defer rows.Close()

	samples := make([]domain.TimeToFirstAgentSample, 0)
	for rows.Next() {
		var s domain.TimeToFirstAgentSample
		var firstAgentAt sql.NullTime
		if err := rows.Scan(&s.OrganizationID, &s.OrgCreatedAt, &firstAgentAt); err != nil {
			return nil, fmt.Errorf("scan time to first agent: %w", err)
		}
		if firstAgentAt.Valid {
			t := firstAgentAt.Time
			s.FirstAgentAt = &t
		}
		samples = append(samples, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read time to first agent: %w", err)
	}
	return samples, nil
}

// CountEventsSince counts events per type recorded at or after since.
func (r *OnboardingEventRepository) CountEventsSince(ctx context.Context, since time.Time) ([]domain.OnboardingEventCount, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT event, COUNT(DISTINCT organization_id), COUNT(*)
		FROM onboarding_events
		WHERE occurred_at >= $1
		GROUP BY event
	`, since)
	if err != nil {
		return nil, fmt.Errorf("count onboarding events: %w", err)
	}
	defer rows.Close()

	counts := make([]domain.OnboardingEventCount, 0)
	for rows.Next() {
		var c domain.OnboardingEventCount
		var event string
		if err := rows.Scan(&event, &c.Organizations, &c.Total); err != nil {
			return nil, fmt.Errorf("scan onboarding event count: %w", err)
		}
		c.Event = domain.OnboardingEventType(event)
		counts = append(counts, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read onboarding event counts: %w", err)
	}
	return counts, nil
}
