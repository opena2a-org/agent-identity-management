package repository

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

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
//
// In the same transaction it deletes each organization of the deleted accounts
// that has no user left (deleteOrganizationLeftWithNoUser), with the rows that
// reference the organization by ON DELETE CASCADE, and sets OrganizationDeleted
// on the accounts of a deleted organization. An organization whose delete a
// reference refuses is kept, the accounts are deleted all the same, and the
// refusal is logged once the transaction commits. Any other failure rolls the
// whole batch back and returns no account, so the next purge takes the accounts
// and their organizations again.
func (r *UserRepository) PurgeDeletionRequests(ctx context.Context, requestedBy time.Time, limit int) ([]domain.AccountDeletion, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("account deletion purge: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	deleted, err := deleteDueAccounts(ctx, tx, requestedBy, limit)
	if err != nil {
		return nil, fmt.Errorf("account deletion purge: %w", err)
	}

	organizationDeleted := make(map[uuid.UUID]bool)
	refused := make(map[uuid.UUID]error)
	for _, orgID := range organizationsOf(deleted) {
		gone, refusal, err := deleteOrganizationLeftWithNoUser(ctx, tx, orgID)
		if err != nil {
			return nil, fmt.Errorf("account deletion purge: organization %s: %w", orgID, err)
		}
		organizationDeleted[orgID] = gone
		if refusal != nil {
			refused[orgID] = refusal
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("account deletion purge: %w", err)
	}

	for orgID, refusal := range refused {
		log.Printf("Account deletion purge: organization %s has no user left and is kept: its delete was refused: %v",
			orgID, refusal)
	}
	for i := range deleted {
		deleted[i].OrganizationDeleted = organizationDeleted[deleted[i].OrganizationID]
	}
	return deleted, nil
}

// deleteDueAccounts is the purge's DELETE FROM users. The accounts are chosen
// and locked once, in a MATERIALIZED common table expression. Written as
// DELETE ... WHERE id IN (SELECT ... LIMIT ... FOR UPDATE SKIP LOCKED), the
// statement was planned as a nested loop that ran the subquery again for each
// row of users, and with the oldest due account locked by another transaction
// it deleted two accounts under LIMIT 1.
func deleteDueAccounts(ctx context.Context, tx *sql.Tx, requestedBy time.Time, limit int) ([]domain.AccountDeletion, error) {
	rows, err := tx.QueryContext(ctx, `
		WITH due AS MATERIALIZED (
		    SELECT id FROM users
		    WHERE deletion_requested_at <= $1
		      AND status = 'deactivated'
		    ORDER BY deletion_requested_at
		    LIMIT $2
		    FOR UPDATE SKIP LOCKED
		)
		DELETE FROM users u
		USING due
		WHERE u.id = due.id
		RETURNING u.id, u.organization_id, u.deletion_requested_at`, requestedBy, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	deleted := make([]domain.AccountDeletion, 0)
	for rows.Next() {
		var d domain.AccountDeletion
		if err := rows.Scan(&d.UserID, &d.OrganizationID, &d.RequestedAt); err != nil {
			return nil, err
		}
		deleted = append(deleted, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return deleted, nil
}

// organizationsOf returns the organizations of the accounts, each once, in the
// order of their ids' bytes, so two purges lock the organizations they share in
// the same order.
func organizationsOf(deleted []domain.AccountDeletion) []uuid.UUID {
	orgs := make([]uuid.UUID, 0)
	for _, d := range deleted {
		if !slices.Contains(orgs, d.OrganizationID) {
			orgs = append(orgs, d.OrganizationID)
		}
	}
	slices.SortFunc(orgs, func(a, b uuid.UUID) int { return bytes.Compare(a[:], b[:]) })
	return orgs
}

// deleteOrganizationLeftWithNoUser deletes the organization if no user of it is
// left, inside the purge's transaction, and reports whether it did.
//
// The organization's row is locked (FOR UPDATE) by a statement of its own, and
// the users are counted by the next statement, which reads what was committed
// once the lock is held. A transaction that adds a user to the organization
// holds a lock on the organization's row until it ends (the foreign key check of
// users.organization_id takes it), so the purge waits for it and then counts
// that user, and the organization is kept. One DELETE that tested for users and
// deleted the row would test against what was committed when it started, and
// would delete that user by cascade.
//
// The delete runs under a savepoint. A delete that a reference to the
// organization refuses (SQLSTATE 23503, a reference with no ON DELETE action, or
// 23001, ON DELETE RESTRICT, as the signed audit record chain tables have) is
// rolled back to the savepoint and returned as refusal, and the organization is
// kept. Any other error is returned as err.
func deleteOrganizationLeftWithNoUser(ctx context.Context, tx *sql.Tx, orgID uuid.UUID) (deleted bool, refusal error, err error) {
	if _, err := tx.ExecContext(ctx, `SELECT 1 FROM organizations WHERE id = $1 FOR UPDATE`, orgID); err != nil {
		return false, nil, err
	}
	if _, err := tx.ExecContext(ctx, `SAVEPOINT organization_delete`); err != nil {
		return false, nil, err
	}
	res, err := tx.ExecContext(ctx, `
		DELETE FROM organizations o
		WHERE o.id = $1
		  AND NOT EXISTS (SELECT 1 FROM users u WHERE u.organization_id = o.id)`, orgID)
	if isReferenceRefusal(err) {
		if _, rbErr := tx.ExecContext(ctx, `ROLLBACK TO SAVEPOINT organization_delete`); rbErr != nil {
			return false, nil, rbErr
		}
		if _, relErr := tx.ExecContext(ctx, `RELEASE SAVEPOINT organization_delete`); relErr != nil {
			return false, nil, relErr
		}
		return false, err, nil
	}
	if err != nil {
		return false, nil, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, nil, err
	}
	if _, err := tx.ExecContext(ctx, `RELEASE SAVEPOINT organization_delete`); err != nil {
		return false, nil, err
	}
	return n == 1, nil, nil
}

// isReferenceRefusal is true for an error of a delete that a foreign key
// refused: foreign_key_violation (23503) or restrict_violation (23001).
func isReferenceRefusal(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && (pqErr.Code == "23503" || pqErr.Code == "23001")
}
