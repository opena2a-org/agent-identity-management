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

// BootstrapTokenRepository implements domain.BootstrapTokenRepository using
// PostgreSQL. Every write that decides whether a token is usable does so in
// its WHERE clause, so the decision and the write are one statement.
type BootstrapTokenRepository struct {
	db *sql.DB
}

// NewBootstrapTokenRepository creates a new BootstrapTokenRepository.
func NewBootstrapTokenRepository(db *sql.DB) *BootstrapTokenRepository {
	return &BootstrapTokenRepository{db: db}
}

var _ domain.BootstrapTokenRepository = (*BootstrapTokenRepository)(nil)

// CreateReplacingOpen revokes the caller's open tokens and inserts the new one
// in one transaction.
func (r *BootstrapTokenRepository) CreateReplacingOpen(ctx context.Context, t *domain.BootstrapToken, now time.Time) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin bootstrap token mint: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		UPDATE bootstrap_tokens
		SET revoked_at = $3
		WHERE organization_id = $1 AND created_by = $2
		  AND used_at IS NULL AND revoked_at IS NULL
	`, t.OrganizationID, t.CreatedBy, now); err != nil {
		return fmt.Errorf("revoke previous bootstrap tokens: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO bootstrap_tokens (
			id, organization_id, created_by, token_hash, display_prefix,
			scope, created_at, expires_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, t.ID, t.OrganizationID, t.CreatedBy, t.TokenHash, t.DisplayPrefix,
		t.Scope, t.CreatedAt, t.ExpiresAt); err != nil {
		return fmt.Errorf("insert bootstrap token: %w", err)
	}

	return tx.Commit()
}

// GetByHash returns the token stored under hash.
func (r *BootstrapTokenRepository) GetByHash(ctx context.Context, hash string) (*domain.BootstrapToken, error) {
	t := &domain.BootstrapToken{}
	err := r.db.QueryRowContext(ctx, `
		SELECT id, organization_id, created_by, token_hash, display_prefix,
			scope, created_at, expires_at, used_at, revoked_at, agent_id
		FROM bootstrap_tokens
		WHERE token_hash = $1
	`, hash).Scan(
		&t.ID, &t.OrganizationID, &t.CreatedBy, &t.TokenHash, &t.DisplayPrefix,
		&t.Scope, &t.CreatedAt, &t.ExpiresAt, &t.UsedAt, &t.RevokedAt, &t.AgentID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrBootstrapTokenNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get bootstrap token: %w", err)
	}
	return t, nil
}

// Claim marks the token used only if it is still usable at now.
func (r *BootstrapTokenRepository) Claim(ctx context.Context, id uuid.UUID, now time.Time) (bool, error) {
	res, err := r.db.ExecContext(ctx, `
		UPDATE bootstrap_tokens
		SET used_at = $2
		WHERE id = $1 AND used_at IS NULL AND revoked_at IS NULL AND expires_at > $2
	`, id, now)
	if err != nil {
		return false, fmt.Errorf("claim bootstrap token: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("claim bootstrap token: %w", err)
	}
	return n == 1, nil
}

// ReleaseClaim reopens a token claimed at usedAt whose exchange failed before
// an agent was created.
func (r *BootstrapTokenRepository) ReleaseClaim(ctx context.Context, id uuid.UUID, usedAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE bootstrap_tokens
		SET used_at = NULL
		WHERE id = $1 AND used_at = $2 AND revoked_at IS NULL AND agent_id IS NULL
	`, id, usedAt)
	if err != nil {
		return fmt.Errorf("release bootstrap token claim: %w", err)
	}
	return nil
}

// SetAgent records the agent a claimed token registered.
func (r *BootstrapTokenRepository) SetAgent(ctx context.Context, id, agentID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE bootstrap_tokens SET agent_id = $2 WHERE id = $1 AND used_at IS NOT NULL
	`, id, agentID)
	if err != nil {
		return fmt.Errorf("record bootstrap token agent: %w", err)
	}
	return nil
}

// RevokeOpenForUser revokes the open tokens minted by userID in orgID.
func (r *BootstrapTokenRepository) RevokeOpenForUser(ctx context.Context, orgID, userID uuid.UUID, now time.Time) (int64, error) {
	res, err := r.db.ExecContext(ctx, `
		UPDATE bootstrap_tokens
		SET revoked_at = $3
		WHERE organization_id = $1 AND created_by = $2
		  AND used_at IS NULL AND revoked_at IS NULL
	`, orgID, userID, now)
	if err != nil {
		return 0, fmt.Errorf("revoke bootstrap tokens: %w", err)
	}
	return res.RowsAffected()
}

// RevokeByHash revokes the token stored under hash if it is still open.
func (r *BootstrapTokenRepository) RevokeByHash(ctx context.Context, hash string, now time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE bootstrap_tokens
		SET revoked_at = $2
		WHERE token_hash = $1 AND used_at IS NULL AND revoked_at IS NULL
	`, hash, now)
	if err != nil {
		return fmt.Errorf("revoke bootstrap token: %w", err)
	}
	return nil
}
