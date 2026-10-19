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

// RequestDeletion records the user's request for the deletion of their own
// account and deactivates the account, in one transaction: status becomes
// 'deactivated' and deletion_requested_at the time at, as stored.
//
// The organization's only active admin is refused with
// domain.ErrLastActiveAdmin while the organization has another user who is not
// deactivated. The organization's row is locked first (FOR NO KEY UPDATE, which
// does not wait on the rows other tables insert that reference it), so two
// admins who ask at the same time do not each count the other as the admin who
// stays: the second reads the first's request once it commits. An account that
// is already deactivated is refused with domain.ErrAccountDeactivated, and an
// unknown user with domain.ErrUserNotFound.
func (r *UserRepository) RequestDeletion(ctx context.Context, userID uuid.UUID, at time.Time) (domain.AccountDeletion, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.AccountDeletion{}, fmt.Errorf("account deletion request: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var orgID uuid.UUID
	err = tx.QueryRowContext(ctx, `SELECT organization_id FROM users WHERE id = $1`, userID).Scan(&orgID)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.AccountDeletion{}, domain.ErrUserNotFound
	}
	if err != nil {
		return domain.AccountDeletion{}, fmt.Errorf("account deletion request: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `SELECT 1 FROM organizations WHERE id = $1 FOR NO KEY UPDATE`, orgID); err != nil {
		return domain.AccountDeletion{}, fmt.Errorf("account deletion request: %w", err)
	}

	var role, status string
	var deletedAt sql.NullTime
	err = tx.QueryRowContext(ctx, `SELECT role, status, deleted_at FROM users WHERE id = $1 FOR UPDATE`, userID).
		Scan(&role, &status, &deletedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.AccountDeletion{}, domain.ErrUserNotFound
	}
	if err != nil {
		return domain.AccountDeletion{}, fmt.Errorf("account deletion request: %w", err)
	}
	if status == string(domain.UserStatusDeactivated) || deletedAt.Valid {
		return domain.AccountDeletion{}, domain.ErrAccountDeactivated
	}

	if role == string(domain.RoleAdmin) && status == string(domain.UserStatusActive) {
		var othersRemain, anotherAdmin bool
		err = tx.QueryRowContext(ctx, `
			SELECT
			    EXISTS (SELECT 1 FROM users
			            WHERE organization_id = $1 AND id <> $2
			              AND status <> 'deactivated' AND deleted_at IS NULL),
			    EXISTS (SELECT 1 FROM users
			            WHERE organization_id = $1 AND id <> $2
			              AND role = 'admin' AND status = 'active' AND deleted_at IS NULL)`,
			orgID, userID).Scan(&othersRemain, &anotherAdmin)
		if err != nil {
			return domain.AccountDeletion{}, fmt.Errorf("account deletion request: %w", err)
		}
		if othersRemain && !anotherAdmin {
			return domain.AccountDeletion{}, domain.ErrLastActiveAdmin
		}
	}

	// updated_at is stamped by the users table's BEFORE UPDATE trigger.
	var requestedAt time.Time
	err = tx.QueryRowContext(ctx, `
		UPDATE users
		SET status = 'deactivated', deletion_requested_at = $2
		WHERE id = $1
		RETURNING deletion_requested_at`, userID, at).Scan(&requestedAt)
	if err != nil {
		return domain.AccountDeletion{}, fmt.Errorf("account deletion request: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return domain.AccountDeletion{}, fmt.Errorf("account deletion request: %w", err)
	}
	return domain.AccountDeletion{UserID: userID, OrganizationID: orgID, RequestedAt: requestedAt}, nil
}

// PurgeDeletionRequests deletes at most limit accounts whose deletion request
// was made at or before requestedBy and that are still deactivated, the
// oldest request first, and returns them. It deletes with DELETE FROM users,
// as Delete does, so each reference to the user follows its ON DELETE rule and
// the AFTER DELETE triggers on users run. Rows another purge holds locked are
// skipped (FOR UPDATE SKIP LOCKED), so two servers that purge at once delete
// each account once.
func (r *UserRepository) PurgeDeletionRequests(ctx context.Context, requestedBy time.Time, limit int) ([]domain.AccountDeletion, error) {
	rows, err := r.db.QueryContext(ctx, `
		DELETE FROM users
		WHERE id IN (
		    SELECT id FROM users
		    WHERE deletion_requested_at <= $1
		      AND status = 'deactivated'
		    ORDER BY deletion_requested_at
		    LIMIT $2
		    FOR UPDATE SKIP LOCKED
		)
		RETURNING id, organization_id, deletion_requested_at`, requestedBy, limit)
	if err != nil {
		return nil, fmt.Errorf("account deletion purge: %w", err)
	}
	defer rows.Close()

	deleted := make([]domain.AccountDeletion, 0)
	for rows.Next() {
		var d domain.AccountDeletion
		if err := rows.Scan(&d.UserID, &d.OrganizationID, &d.RequestedAt); err != nil {
			return deleted, fmt.Errorf("account deletion purge: %w", err)
		}
		deleted = append(deleted, d)
	}
	if err := rows.Err(); err != nil {
		return deleted, fmt.Errorf("account deletion purge: %w", err)
	}
	return deleted, nil
}
