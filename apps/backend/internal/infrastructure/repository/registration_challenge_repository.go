package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// RegistrationChallengeRepository implements domain.RegistrationChallengeRepository
// with the agent_registration_challenges table (migration 142). The decision
// that a challenge is used for the first time is the WHERE clause of one UPDATE,
// so two registrations that present the same id cannot both win it.
type RegistrationChallengeRepository struct {
	db *sql.DB
}

// NewRegistrationChallengeRepository creates the repository.
func NewRegistrationChallengeRepository(db *sql.DB) *RegistrationChallengeRepository {
	return &RegistrationChallengeRepository{db: db}
}

var _ domain.RegistrationChallengeRepository = (*RegistrationChallengeRepository)(nil)

// Create stores a new challenge.
func (r *RegistrationChallengeRepository) Create(ctx context.Context, c *domain.RegistrationChallenge) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO agent_registration_challenges (id, organization_id, principal_id, challenge, issued_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, c.ID, c.OrganizationID, c.PrincipalID, c.Challenge, c.IssuedAt, c.ExpiresAt)
	if err != nil {
		return fmt.Errorf("insert registration challenge: %w", err)
	}
	return nil
}

// Consume marks the challenge used at now, once, and returns it.
func (r *RegistrationChallengeRepository) Consume(ctx context.Context, id uuid.UUID, now time.Time) (*domain.RegistrationChallenge, bool, error) {
	c := &domain.RegistrationChallenge{}
	err := r.db.QueryRowContext(ctx, `
		UPDATE agent_registration_challenges
		SET used_at = $2
		WHERE id = $1 AND used_at IS NULL
		RETURNING id, organization_id, principal_id, challenge, issued_at, expires_at
	`, id, now).Scan(&c.ID, &c.OrganizationID, &c.PrincipalID, &c.Challenge, &c.IssuedAt, &c.ExpiresAt)
	if err == nil {
		used := now
		c.UsedAt = &used
		return c, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, false, fmt.Errorf("use registration challenge: %w", err)
	}

	// No row was updated: the id names no challenge, or an earlier call used it.
	var usedAt sql.NullTime
	err = r.db.QueryRowContext(ctx, `
		SELECT id, organization_id, principal_id, challenge, issued_at, expires_at, used_at
		FROM agent_registration_challenges
		WHERE id = $1
	`, id).Scan(&c.ID, &c.OrganizationID, &c.PrincipalID, &c.Challenge, &c.IssuedAt, &c.ExpiresAt, &usedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, domain.ErrRegistrationChallengeNotFound
	}
	if err != nil {
		return nil, false, fmt.Errorf("read registration challenge: %w", err)
	}
	if usedAt.Valid {
		c.UsedAt = &usedAt.Time
	}
	return c, false, nil
}

// DeleteExpired deletes the challenges that expired at or before now.
func (r *RegistrationChallengeRepository) DeleteExpired(ctx context.Context, now time.Time) (int64, error) {
	result, err := r.db.ExecContext(ctx, `
		DELETE FROM agent_registration_challenges WHERE expires_at <= $1
	`, now)
	if err != nil {
		return 0, fmt.Errorf("delete expired registration challenges: %w", err)
	}
	return result.RowsAffected()
}
